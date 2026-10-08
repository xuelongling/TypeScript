// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

// This finite slice takes every member from the official checker. It does not
// parse a type spelling or widen a mixed scalar union into a common payload.
func finitePresence(t *checker.Type) (payload string, undefined, null, ok bool) {
	if t == nil {
		return "", false, false, false
	}
	if t.IsUnion() {
		for _, member := range t.Types() {
			name, u, n, valid := finitePresence(member)
			if !valid || name != "" && payload != "" && name != payload {
				return "", false, false, false
			}
			if name != "" {
				payload = name
			}
			undefined, null = undefined || u, null || n
		}
		return payload, undefined, null, true
	}
	if t.Flags()&checker.TypeFlagsUndefined != 0 {
		return "", true, false, true
	}
	if t.Flags()&checker.TypeFlagsNull != 0 {
		return "", false, true, true
	}
	payload = scalar(t)
	return payload, false, false, payload != ""
}

type parameterPresence struct {
	Undefined  bool `json:"undefined"`
	Null       bool `json:"null"`
	Optional   bool `json:"optional"`
	Default    bool `json:"default"`
	Range      span `json:"range"`
	Annotation span `json:"annotation"`
}

type presenceFlow struct {
	Undefined bool `json:"undefined"`
	Null      bool `json:"null"`
}

func expressionPresence(value *expression) *presenceFlow {
	if value == nil {
		return nil
	}
	if value.Kind == "group" {
		return expressionPresence(value.Operand)
	}
	if value.Kind == "write" {
		if value.Operator == "??=" {
			left := expressionPresence(value.Left)
			if left == nil || !left.Undefined && !left.Null {
				return nil
			}
		}
		return expressionPresence(value.Right)
	}
	if value.Kind == "absence" {
		return &presenceFlow{Undefined: value.Absence == "undefined", Null: value.Absence == "null"}
	}
	return value.PresenceFlow
}

func permittedPresence(value *expression, shape *parameterPresence) bool {
	flow := expressionPresence(value)
	return flow == nil || (!flow.Undefined || shape != nil && shape.Undefined) && (!flow.Null || shape != nil && shape.Null)
}

func checkedParameter(file *ast.SourceFile, check *checker.Checker, node *ast.Node) (string, *parameterPresence, bool) {
	p := node.AsParameterDeclaration()
	if node.Name() == nil || node.Name().Kind != ast.KindIdentifier || p.Type == nil || p.DotDotDotToken != nil {
		return "", nil, false
	}
	payload, undefined, null, ok := finitePresence(check.GetTypeFromTypeNode(p.Type))
	bodyPayload, _, _, bodyOK := finitePresence(check.GetTypeAtLocation(node.Name()))
	if !ok || !bodyOK || payload == "" || payload != bodyPayload {
		return "", nil, false
	}
	optional, hasDefault := p.QuestionToken != nil, p.Initializer != nil
	return payload, &parameterPresence{Undefined: undefined || optional || hasDefault, Null: null, Optional: optional || hasDefault,
		Default: hasDefault, Range: offset(file, node), Annotation: expressionOffset(file, p.Type)}, true
}

// The incoming parameter admits undefined for default selection, but the
// initializer itself is checked against the annotation's declared members.
func checkedInitializerPresence(check *checker.Checker, node *ast.Node) *parameterPresence {
	_, undefined, null, _ := finitePresence(check.GetTypeFromTypeNode(node.AsParameterDeclaration().Type))
	return &parameterPresence{Undefined: undefined, Null: null}
}

func checkedLocalPresence(file *ast.SourceFile, check *checker.Checker, node *ast.Node) (string, *parameterPresence, bool) {
	declaration := node.AsVariableDeclaration()
	if declaration.Type == nil {
		return "", nil, false
	}
	payload, undefined, null, valid := finitePresence(check.GetTypeFromTypeNode(declaration.Type))
	if !valid || payload == "" {
		return "", nil, false
	}
	if !undefined && !null {
		return payload, nil, true
	}
	return payload, &parameterPresence{Undefined: undefined, Null: null, Range: offset(file, node), Annotation: expressionOffset(file, declaration.Type)}, true
}

func resolvedCallMatches(file *ast.SourceFile, check *checker.Checker, resolved *checker.Signature, fn functionInfo) bool {
	if resolved == nil || resolved.Declaration() == nil || scalar(check.GetReturnTypeOfSignature(resolved)) != fn.Signature.ReturnType || len(resolved.Parameters()) != len(fn.Signature.Parameters) {
		return false
	}
	declaration := resolved.Declaration()
	if declaration != fn.Declaration {
		// An already admitted ambient conversion spelling may share its official
		// symbol with a user implementation. Match the real symbol and complete
		// signature; it remains an ordinary call to the owned source body.
		if fn.Conversion != nil || declaration.Name() == nil || fn.Declaration.Name() == nil || check.GetSymbolAtLocation(declaration.Name()) != check.GetSymbolAtLocation(fn.Declaration.Name()) {
			return false
		}
	}
	for index, parameter := range declaration.AsFunctionDeclaration().Parameters.Nodes {
		payload, shape, valid := checkedParameter(ast.GetSourceFileOfNode(parameter), check, parameter)
		if !valid || payload != fn.Signature.Parameters[index] {
			return false
		}
		var expected *parameterPresence
		if len(fn.Signature.Presence) > index {
			expected = fn.Signature.Presence[index]
		}
		if expected != nil && (shape.Undefined != expected.Undefined || shape.Null != expected.Null || shape.Optional != expected.Optional || shape.Default != expected.Default) {
			return false
		}
		if expected == nil && (shape.Undefined || shape.Null || shape.Optional || shape.Default) {
			return false
		}
	}
	return true
}

func nominalPayload(t *checker.Type) *checker.Type {
	if t == nil {
		return nil
	}
	if !t.IsUnion() {
		if scalar(t) != "" {
			return t
		}
		return nil
	}
	var payload *checker.Type
	for _, member := range t.Types() {
		if member.Flags()&(checker.TypeFlagsUndefined|checker.TypeFlagsNull) != 0 {
			continue
		}
		if payload != nil || scalar(member) == "" {
			return nil
		}
		payload = member
	}
	return payload
}

// Register complete, present scalar RHS expressions against official AST and
// nominal type objects before the official checker's assignment-flow analysis.
// No spelling or synthesized union stands in for an official type object.
func scalarAssignmentContexts(file *ast.SourceFile, check *checker.Checker, functions map[*ast.Symbol]functionInfo, target *integerTarget) map[*ast.Node]*checker.Type {
	contexts := map[*ast.Node]*checker.Type{}
	refs := map[*ast.Symbol]symbol{}
	payloads := map[*ast.Symbol]*checker.Type{}
	var declarations func(*ast.Node) bool
	declarations = func(node *ast.Node) bool {
		var annotation *ast.Node
		if node.Kind == ast.KindParameter {
			annotation = node.AsParameterDeclaration().Type
		}
		if node.Kind == ast.KindVariableDeclaration {
			annotation = node.AsVariableDeclaration().Type
		}
		if annotation != nil && node.Name() != nil && node.Name().Kind == ast.KindIdentifier {
			resolved := check.GetSymbolAtLocation(node.Name())
			official := check.GetTypeFromTypeNode(annotation)
			payload, u, n, valid := finitePresence(official)
			if resolved != nil && valid && payload != "" {
				refs[resolved] = symbol{Type: payload, Writable: true, Presence: &parameterPresence{Undefined: u, Null: n}}
				payloads[resolved] = nominalPayload(official)
			}
		}
		return node.ForEachChild(declarations)
	}
	file.AsNode().ForEachChild(declarations)
	var containsCarrier func(*ast.Node) bool
	containsCarrier = func(node *ast.Node) bool {
		if node.Kind == ast.KindIdentifier {
			if binding, ok := refs[check.GetSymbolAtLocation(node)]; ok && binding.Presence != nil && (binding.Presence.Undefined || binding.Presence.Null) {
				return true
			}
		}
		return node.ForEachChild(containsCarrier)
	}
	var assignments func(*ast.Node) bool
	assignments = func(node *ast.Node) bool {
		var left, right, flow *ast.Node
		if node.Kind == ast.KindVariableDeclaration {
			left, right, flow = node.Name(), node.AsVariableDeclaration().Initializer, node
		}
		if node.Kind == ast.KindBinaryExpression && (node.AsBinaryExpression().OperatorToken.Kind == ast.KindEqualsToken || node.AsBinaryExpression().OperatorToken.Kind == ast.KindQuestionQuestionEqualsToken) {
			left, right = node.AsBinaryExpression().Left, node.AsBinaryExpression().Right
			flow = left
		}
		if left != nil && left.Kind == ast.KindIdentifier && right != nil && !containsCarrier(right) {
			resolved := check.GetSymbolAtLocation(left)
			binding, ok := refs[resolved]
			if ok && payloads[resolved] != nil {
				projected, failure := projectIntegerExpression(file, check, right, binding.Type, refs, functions, target)
				if failure == nil && permittedPresence(projected, nil) {
					contexts[flow] = payloads[resolved]
				}
			}
		}
		return node.ForEachChild(assignments)
	}
	file.AsNode().ForEachChild(assignments)
	return contexts
}

func missingState(check *checker.Checker, node *ast.Node) string {
	if node == nil {
		return ""
	}
	t := check.GetTypeAtLocation(node)
	if node.Kind == ast.KindNullKeyword && t.Flags()&checker.TypeFlagsNull != 0 {
		return "null"
	}
	if node.Kind == ast.KindIdentifier && node.Text() == "undefined" && t.Flags()&checker.TypeFlagsUndefined != 0 {
		if resolved := check.GetSymbolAtLocation(node); resolved == nil || len(resolved.Declarations) != 0 {
			return ""
		}
		return "undefined"
	}
	return ""
}
