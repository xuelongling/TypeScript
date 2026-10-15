// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Builds owned function statements and expressions; compiler objects stay within this package.
package main

import (
	"fmt"
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

type projection struct {
	aggregates    *aggregateProjection
	self          *aggregateInfo
	integerTarget *integerTarget
	file          *ast.SourceFile
	check         *checker.Checker
	result        *response
	bindings      map[*ast.Symbol]symbol
	functions     map[*ast.Symbol]functionInfo
}

func (p *projection) addSymbol(node *ast.Node, kind, t string, writable bool) (symbol, bool) {
	upstream := p.check.GetSymbolAtLocation(node)
	if upstream == nil {
		return symbol{}, false
	}
	s := symbol{
		ID: fmt.Sprintf("s%d", len(p.result.Symbols)+1), Name: node.Text(),
		Kind: kind, Type: t, Writable: writable, Range: offset(p.file, node),
	}
	if p.result.Modules != nil {
		s.SourcePath = p.file.FileName()
	}
	p.result.Symbols = append(p.result.Symbols, s)
	p.bindings[upstream] = s
	return s, true
}

// parameters installs owned bindings and projects defaults for both the single-
// source and module paths. Aggregate parameter checks remain with their state.
func (p *projection) parameters(fn *function, nodes []*ast.Node, hasPresence bool) response {
	for _, node := range nodes {
		t, shape, valid := checkedParameter(p.file, p.check, node)
		if p.aggregates != nil && !valid {
			t, shape, valid = p.aggregates.parameter(p.file, p.check, node)
		}
		if !valid {
			return failAt("r02b-d-unsupported", "unsupported", p.file, node)
		}
		s, ok := p.addSymbol(node.Name(), "parameter", t, true)
		if !ok {
			// Preserve the existing module whole-parameter diagnostic anchor.
			anchor := node.Name()
			if p.result.Modules != nil {
				anchor = node
			}
			return failAt("r02b-d-type", "error", p.file, anchor)
		}
		s.Presence = shape
		p.result.Symbols[len(p.result.Symbols)-1] = s
		p.bindings[p.check.GetSymbolAtLocation(node.Name())] = s
		fn.Parameters = append(fn.Parameters, s)
	}
	if hasPresence {
		fn.ParameterInitializers = make([]*expression, len(nodes))
		for index, node := range nodes {
			if node.AsParameterDeclaration().Initializer == nil {
				continue
			}
			value, failure := p.expr(node.AsParameterDeclaration().Initializer, fn.Parameters[index].Type, checkedInitializerPresence(p.check, node))
			if failure.Status != "" {
				return failure
			}
			fn.ParameterInitializers[index] = value
		}
	}
	return response{}
}

func (p *projection) expr(node *ast.Node, expected string, presence ...*parameterPresence) (*expression, response) {
	if p.aggregates != nil {
		if projected, failure, handled := p.aggregateExpr(node, expected, presence...); handled {
			return projected, failure
		}
	}
	if p.integerTarget != nil {
		var shape *parameterPresence
		if len(presence) != 0 {
			shape = presence[0]
		}
		projected, failure := projectIntegerExpression(p.file, p.check, node, expected, p.bindings, p.functions, p.integerTarget, false, shape != nil && (shape.Undefined || shape.Null))
		if failure != nil {
			return nil, *failure
		}
		if !permittedPresence(projected, shape) {
			return nil, failAt("r01-d-scalar-type", "error", p.file, node)
		}
		return projected, response{}
	}
	if node == nil {
		return nil, fail("r02b-d-unsupported", "unsupported", p.file.FileName(), span{})
	}
	if node.Kind == ast.KindCallExpression {
		if moduleReference(p.check, node.AsCallExpression().Expression) {
			if target, ok := p.functions[p.check.GetSymbolAtLocation(node.AsCallExpression().Expression)]; ok {
				if expected == "" {
					expected = target.Signature.ReturnType
				}
			}
		}
		projected, failure := projectExpression(p.file, p.check, node, expected, p.bindings, p.functions)
		if failure != nil {
			return nil, *failure
		}
		return &projected, response{}
	}
	if moduleReference(p.check, node) {
		s, ok := p.bindings[p.check.GetSymbolAtLocation(node)]
		if !ok {
			return nil, failAt("r02b-d-type", "error", p.file, node)
		}
		if expected != "" && s.Type != expected {
			return nil, failAt("r01-d-scalar-type", "error", p.file, node)
		}
		return &expression{Kind: "reference", Type: s.Type, Symbol: s.ID, Range: offset(p.file, node)}, response{}
	}
	if node.Kind == ast.KindTrueKeyword || node.Kind == ast.KindFalseKeyword {
		if expected != "" && expected != "bool" {
			return nil, failAt("r01-d-scalar-type", "error", p.file, node)
		}
		value := node.Kind == ast.KindTrueKeyword
		return &expression{Kind: "boolean", Type: "bool", Boolean: &value, Range: offset(p.file, node)}, response{}
	}
	if value, literal, ok := exactInteger(p.file, node); ok {
		if expected != "" && expected != "i32" {
			return nil, failAt("r01-d-scalar-type", "error", p.file, node)
		}
		if !i32InRange(value) {
			return nil, failAt("r01-d-integer-range", "error", p.file, node)
		}
		return &expression{Kind: "integer", Type: "i32", Literal: literal, Range: offset(p.file, node)}, response{}
	}
	projected, failure := projectOperator(p.file, p.check, node, p.bindings, p.functions)
	if failure != nil {
		return nil, *failure
	}
	if expected != "" && projected.Type != expected {
		return nil, failAt("r01-d-scalar-type", "error", p.file, node)
	}
	return projected, response{}
}

func (p *projection) statements(nodes []*ast.Node, returnType string) ([]statement, response) {
	out := make([]statement, 0, len(nodes))
	for _, node := range nodes {
		switch node.Kind {
		case ast.KindVariableStatement:
			declarations := localDeclarations(node)
			if len(declarations) == 0 {
				return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node)
			}
			for _, decl := range declarations {
				if decl.Kind != ast.KindVariableDeclaration || decl.Name() == nil || decl.Name().Kind != ast.KindIdentifier {
					return nil, failAt("r02b-d-unsupported", "unsupported", p.file, decl)
				}
				local := decl.AsVariableDeclaration()
				if local.Initializer == nil && local.Type == nil {
					return nil, failAt("r02b-d-unsupported", "unsupported", p.file, decl)
				}
				t := ""
				var shape *parameterPresence
				if local.Type != nil {
					var valid bool
					if p.aggregates != nil {
						t = p.aggregates.typeName(p.check.GetTypeFromTypeNode(local.Type))
					}
					if t != "" {
						valid = true
					} else {
						t, shape, valid = checkedLocalPresence(p.file, p.check, decl)
					}
					if !valid {
						return nil, failAt("r02b-d-unsupported", "unsupported", p.file, local.Type)
					}
				}
				var init *expression
				if local.Initializer != nil {
					var failure response
					init, failure = p.expr(local.Initializer, t, shape)
					if failure.Status != "" {
						return nil, failure
					}
				}
				if t == "" {
					t = init.Type
				}
				writable := node.AsVariableStatement().DeclarationList.Flags&ast.NodeFlagsLet != 0
				if !writable && node.AsVariableStatement().DeclarationList.Flags&ast.NodeFlagsConst == 0 {
					return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node)
				}
				s, ok := p.addSymbol(decl.Name(), "local", t, writable)
				if !ok {
					return nil, failAt("r02b-d-type", "error", p.file, decl.Name())
				}
				if shape != nil {
					s.Presence = shape
					p.result.Symbols[len(p.result.Symbols)-1] = s
					p.bindings[p.check.GetSymbolAtLocation(decl.Name())] = s
				}
				out = append(out, statement{Kind: "variable", Symbol: &s, Initializer: init, Range: offset(p.file, decl)})
			}
		case ast.KindExpressionStatement:
			expr := node.AsExpressionStatement().Expression
			if (p.result.Modules != nil && expr.Kind == ast.KindCallExpression) || (p.aggregates != nil && aggregateFieldMutation(expr)) {
				value, failure := p.expr(expr, "")
				if failure.Status != "" {
					return nil, failure
				}
				out = append(out, statement{Kind: "expression", Expression: value, Range: offset(p.file, node)})
				continue
			}
			if expr.Kind != ast.KindBinaryExpression || expr.AsBinaryExpression().OperatorToken.Kind != ast.KindEqualsToken || p.integerTarget != nil && expr.AsBinaryExpression().Left.Kind == ast.KindParenthesizedExpression {
				var discardedPresence *parameterPresence
				candidate := ast.SkipParentheses(expr)
				if p.integerTarget != nil && candidate.Kind == ast.KindBinaryExpression && (candidate.AsBinaryExpression().OperatorToken.Kind == ast.KindAmpersandAmpersandEqualsToken || candidate.AsBinaryExpression().OperatorToken.Kind == ast.KindBarBarEqualsToken) {
					leaf := ast.SkipParentheses(candidate.AsBinaryExpression().Left)
					if leaf.Kind == ast.KindIdentifier {
						discardedPresence = p.bindings[p.check.GetSymbolAtLocation(leaf)].Presence
					}
				}
				value, failure := p.expr(expr, "", discardedPresence)
				if failure.Status != "" {
					return nil, failure
				}
				out = append(out, statement{Kind: "expression", Expression: value, Range: offset(p.file, node)})
				continue
			}
			binary := expr.AsBinaryExpression()
			if !moduleReference(p.check, binary.Left) {
				return nil, failAt("r02b-d-unsupported", "unsupported", p.file, binary.Left)
			}
			s := p.bindings[p.check.GetSymbolAtLocation(binary.Left)]
			target, failure := p.expr(binary.Left, "", s.Presence)
			if failure.Status != "" {
				return nil, failure
			}
			if !s.Writable {
				return nil, failAt("r01-d-scalar-type", "error", p.file, binary.Left)
			}
			value, failure := p.expr(binary.Right, target.Type, s.Presence)
			if failure.Status != "" {
				return nil, failure
			}
			out = append(out, statement{Kind: "assignment", Target: target, Value: value, Range: offset(p.file, node)})
		case ast.KindReturnStatement:
			if p.self != nil && node.AsReturnStatement().Expression == nil {
				out = append(out, statement{Kind: "return", Range: offset(p.file, node)})
				continue
			}
			expr, failure := p.expr(node.AsReturnStatement().Expression, returnType)
			if failure.Status != "" {
				return nil, failure
			}
			out = append(out, statement{Kind: "return", Expression: expr, Range: offset(p.file, node)})
		case ast.KindBlock:
			body, failure := p.statements(node.AsBlock().Statements.Nodes, returnType)
			if failure.Status != "" {
				return nil, failure
			}
			out = append(out, statement{Kind: "block", Body: body, Range: offset(p.file, node)})
		case ast.KindIfStatement:
			branch := node.AsIfStatement()
			condition, failure := p.expr(branch.Expression, "bool")
			if failure.Status != "" {
				return nil, failure
			}
			thenBody, failure := p.statements([]*ast.Node{branch.ThenStatement}, returnType)
			if failure.Status != "" {
				return nil, failure
			}
			projected := statement{Kind: "if", Condition: condition, ThenBody: thenBody, Range: expressionOffset(p.file, node)}
			if branch.ElseStatement != nil {
				projected.ElseBody, failure = p.statements([]*ast.Node{branch.ElseStatement}, returnType)
				if failure.Status != "" {
					return nil, failure
				}
			}
			out = append(out, projected)
		case ast.KindWhileStatement:
			loop := node.AsWhileStatement()
			condition, failure := p.expr(loop.Expression, "bool")
			if failure.Status != "" {
				return nil, failure
			}
			body, failure := p.statements([]*ast.Node{loop.Statement}, returnType)
			if failure.Status != "" {
				return nil, failure
			}
			out = append(out, statement{Kind: "while", Condition: condition, Body: body, Range: expressionOffset(p.file, node)})
		default:
			return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node)
		}
	}
	return out, response{}
}

func projectExpression(file *ast.SourceFile, check *checker.Checker, node *ast.Node, expected string,
	refs map[*ast.Symbol]symbol, functions map[*ast.Symbol]functionInfo,
) (expression, *response) {
	if value, literal, ok := exactInteger(file, node); ok && expected == "i32" {
		if !i32InRange(value) {
			failure := failAt("r01-d-integer-range", "error", file, node)
			return expression{}, &failure
		}
		return expression{Kind: "integer", Type: "i32", Literal: literal, Range: offset(file, node)}, nil
	}
	if node.Kind == ast.KindTrueKeyword || node.Kind == ast.KindFalseKeyword {
		if expected != "bool" {
			failure := failAt("r01-d-scalar-type", "error", file, node)
			return expression{}, &failure
		}
		value := node.Kind == ast.KindTrueKeyword
		return expression{Kind: "boolean", Type: "bool", Boolean: &value, Range: offset(file, node)}, nil
	}
	if moduleReference(check, node) {
		resolved := check.GetSymbolAtLocation(node)
		ref, ok := refs[resolved]
		if !ok {
			failure := failAt("r02b-d-type", "error", file, node)
			return expression{}, &failure
		}
		if ref.Type != expected {
			failure := failAt("r01-d-scalar-type", "error", file, node)
			return expression{}, &failure
		}
		return expression{Kind: "reference", Type: ref.Type, Symbol: ref.ID, Range: offset(file, node)}, nil
	}
	if node.Kind == ast.KindCallExpression {
		call := node.AsCallExpression()
		if !moduleReference(check, call.Expression) || call.QuestionDotToken != nil || call.TypeArguments != nil {
			failure := failAt("r02b-d-unsupported", "unsupported", file, node)
			return expression{}, &failure
		}
		target, ok := functions[check.GetSymbolAtLocation(call.Expression)]
		if !ok {
			failure := failCallAt("r01-d-unresolved-name", file, call.Expression)
			return expression{}, &failure
		}
		if target.Signature.ReturnType != expected {
			failure := failCallAt("r01-d-scalar-type", file, node)
			return expression{}, &failure
		}
		if len(call.Arguments.Nodes) != len(target.Signature.Parameters) {
			failure := failCallAt("r02b-d-type", file, node)
			return expression{}, &failure
		}
		arguments := make([]expression, 0, len(call.Arguments.Nodes))
		for index, argument := range call.Arguments.Nodes {
			projected, failure := projectExpression(file, check, argument, target.Signature.Parameters[index], refs, functions)
			if failure != nil {
				if failure.Diagnostics[0].Code == "r01-d-scalar-type" {
					failure.Diagnostics[0].RuleID = "r01-calls"
					failure.Diagnostics[0].RuleIDs = []string{"r01-calls"}
				}
				return expression{}, failure
			}
			arguments = append(arguments, projected)
		}
		sig := target.Signature
		return expression{
			Kind: "call", Type: sig.ReturnType, Target: target.ID,
			Arguments: &arguments, Signature: &sig, Range: offset(file, node),
		}, nil
	}
	if node.Kind == ast.KindBinaryExpression || node.Kind == ast.KindPrefixUnaryExpression || node.Kind == ast.KindParenthesizedExpression {
		projected, failure := projectOperator(file, check, node, refs, functions)
		if failure != nil {
			return expression{}, failure
		}
		if projected.Type != expected {
			failure := failAt("r01-d-scalar-type", "error", file, node)
			return expression{}, &failure
		}
		return *projected, nil
	}
	failure := failAt("r02b-d-unsupported", "unsupported", file, node)
	return expression{}, &failure
}
