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

// returnPresence records declaration members independently of parameter omission
// and default selection. Its anchors always belong to the original function.
type returnPresence struct {
	Undefined  bool `json:"undefined"`
	Null       bool `json:"null"`
	Range      span `json:"range"`
	Annotation span `json:"annotation"`
}

// Admit this first nullable return slice only from matching official annotation
// and signature facts. No synthesized union stands in for either type object.
func checkedNullableReturn(file *ast.SourceFile, check *checker.Checker, node *ast.Node) (string, *returnPresence, bool) {
	decl := node.AsFunctionDeclaration()
	if decl.Type == nil || decl.TypeParameters != nil || node.Name() == nil {
		return "", nil, false
	}
	payload, undefined, null, valid := finitePresence(check.GetTypeFromTypeNode(decl.Type))
	if !valid || payload != "i32" || !undefined && !null {
		return "", nil, false
	}
	resolved := check.GetSymbolAtLocation(node.Name())
	if resolved == nil {
		return "", nil, false
	}
	signatures := check.GetSignaturesOfType(check.GetTypeOfSymbol(resolved), checker.SignatureKindCall)
	if len(signatures) != 1 || signatures[0].Declaration() != node {
		return "", nil, false
	}
	checkedPayload, checkedUndefined, checkedNull, checkedValid := finitePresence(check.GetReturnTypeOfSignature(signatures[0]))
	if !checkedValid || checkedPayload != payload || checkedUndefined != undefined || checkedNull != null {
		return "", nil, false
	}
	return payload, &returnPresence{
		Undefined: undefined, Null: null, Range: offset(file, node), Annotation: expressionOffset(file, decl.Type),
	}, true
}

// Nullable returns share the integer expression projector, while their allowed
// members come from the return declaration rather than a parameter DTO.
func (p *projection) nullableReturnExpr(node *ast.Node, payload string, declared *returnPresence) (*expression, response) {
	if node == nil || p.integerTarget == nil {
		return nil, fail("r02b-d-unsupported", "unsupported", p.file.FileName(), span{})
	}
	checkedPayload, undefined, null, valid := finitePresence(p.check.GetTypeAtLocation(node))
	if !valid || checkedPayload != "" && checkedPayload != payload || undefined && !declared.Undefined || null && !declared.Null {
		return nil, failAt("r01-d-scalar-type", "error", p.file, node)
	}
	value, failure := projectIntegerExpression(p.file, p.check, node, payload, p.bindings, p.functions, p.integerTarget, false, true)
	if failure != nil {
		return nil, *failure
	}
	flow := expressionPresence(value)
	if flow != nil && (flow.Undefined && !declared.Undefined || flow.Null && !declared.Null) {
		return nil, failAt("r01-d-scalar-type", "error", p.file, node)
	}
	value.PresenceFlow = &presenceFlow{Undefined: undefined, Null: null}
	return value, response{}
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
	if value.Kind == "write" && (value.Operator == "&&=" || value.Operator == "||=") {
		return value.PresenceFlow
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
	return payload, &parameterPresence{
		Undefined: undefined || optional || hasDefault, Null: null, Optional: optional || hasDefault,
		Default: hasDefault, Range: offset(file, node), Annotation: expressionOffset(file, p.Type),
	}, true
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
	if resolved == nil || resolved.Declaration() == nil || len(resolved.Parameters()) != len(fn.Signature.Parameters) {
		return false
	}
	declaration := resolved.Declaration()
	if expected := fn.Signature.ReturnPresence; expected != nil {
		// Nullable ordinary calls require this exact source declaration. The
		// scalar ambient/conversion spelling exception below is not authority
		// for a different nullable declaration or return member set.
		if fn.Conversion != nil || declaration != fn.Declaration || len(declaration.AsFunctionDeclaration().Parameters.Nodes) != len(resolved.Parameters()) {
			return false
		}
		payload, undefined, null, valid := finitePresence(check.GetReturnTypeOfSignature(resolved))
		if !valid || payload != "i32" || payload != fn.Signature.ReturnType || undefined != expected.Undefined || null != expected.Null {
			return false
		}
		declaredPayload, declared, valid := checkedNullableReturn(ast.GetSourceFileOfNode(declaration), check, declaration)
		if !valid || declaredPayload != payload || *declared != *expected {
			return false
		}
	} else if scalar(check.GetReturnTypeOfSignature(resolved)) != fn.Signature.ReturnType {
		return false
	}
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
			left, right = ast.SkipParentheses(node.AsBinaryExpression().Left), node.AsBinaryExpression().Right
			// Binder retains the whole parenthesized target as Flow.Node.
			flow = node.AsBinaryExpression().Left
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
	leaf := ast.SkipParentheses(node)
	if leaf.Kind == ast.KindNullKeyword && t.Flags()&checker.TypeFlagsNull != 0 {
		return "null"
	}
	if leaf.Kind == ast.KindIdentifier && leaf.Text() == "undefined" && t.Flags()&checker.TypeFlagsUndefined != 0 {
		if resolved := check.GetSymbolAtLocation(leaf); resolved == nil || len(resolved.Declarations) != 0 {
			return ""
		}
		return "undefined"
	}
	return ""
}

// Keep ?? as one lazy, non-writing node. Each operand retains its official
// use-site type; literal context cannot remove absence from the left operand.
func projectNullishCoalesce(file *ast.SourceFile, check *checker.Checker, node *ast.Node,
	refs map[*ast.Symbol]symbol, functions map[*ast.Symbol]functionInfo, target *integerTarget,
) (*expression, *response) {
	binary := node.AsBinaryExpression()
	leftPayload, leftUndefined, leftNull, leftValid := finitePresence(check.GetTypeAtLocation(binary.Left))
	rightPayload, rightUndefined, rightNull, rightValid := finitePresence(check.GetTypeAtLocation(binary.Right))
	resultPayload, resultUndefined, resultNull, resultValid := finitePresence(check.GetTypeAtLocation(node))
	if !leftValid || !rightValid || !resultValid || leftPayload != "i32" || rightPayload != leftPayload || resultPayload != leftPayload ||
		!leftUndefined && !leftNull || rightUndefined || rightNull || resultUndefined || resultNull {
		failure := failAt("r02b-d-unsupported", "unsupported", file, node)
		return nil, &failure
	}
	left, failure := projectIntegerExpression(file, check, binary.Left, leftPayload, refs, functions, target, true, true)
	if failure != nil {
		return nil, failure
	}
	leftFlow := expressionPresence(left)
	if leftFlow == nil || leftFlow.Undefined != leftUndefined || leftFlow.Null != leftNull {
		failure := failAt("r01-d-scalar-type", "error", file, binary.Left)
		return nil, &failure
	}
	right, failure := projectIntegerExpression(file, check, binary.Right, rightPayload, refs, functions, target, true)
	if failure != nil {
		return nil, failure
	}
	return &expression{
		Kind: "shortCircuit", Type: resultPayload, Operator: "??", Left: left, Right: right,
		PresenceFlow: &presenceFlow{Undefined: resultUndefined, Null: resultNull}, Range: expressionOffset(file, node),
	}, nil
}
