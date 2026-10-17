// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

// Only a fully projected, present scalar expression qualifies an upstream
// branded-numeric assignment diagnostic. Other diagnostics, particularly
// readonly/visibility/initialization errors, remain official errors.
func (a *aggregateProjection) contextDiagnostic(file *ast.SourceFile, check *checker.Checker, d *ast.Diagnostic, result *response, functions map[*ast.Symbol]functionInfo, target *integerTarget) (bool, *response) {
	if d.File() != file || d.Code() != 2322 && d.Code() != 2345 {
		return false, nil
	}
	bindings := map[*ast.Symbol]symbol{}
	var bind func(*ast.Node) bool
	bind = func(node *ast.Node) bool {
		var annotation *ast.Node
		if node.Kind == ast.KindParameter {
			annotation = node.AsParameterDeclaration().Type
		}
		if node.Kind == ast.KindVariableDeclaration {
			annotation = node.AsVariableDeclaration().Type
		}
		if node.Name() != nil && (annotation != nil || node.Kind == ast.KindVariableDeclaration) {
			var checked *checker.Type
			if annotation != nil {
				checked = check.GetTypeFromTypeNode(annotation)
			} else {
				checked = check.GetTypeAtLocation(node.Name())
			}
			t := a.typeName(checked)
			var shape *parameterPresence
			if t == "" {
				t = scalar(checked)
				if node.Kind == ast.KindParameter {
					if payload, presence, ok := checkedParameter(file, check, node); ok {
						t, shape = payload, presence
					}
				} else if node.Kind == ast.KindVariableDeclaration && annotation != nil {
					if payload, presence, ok := checkedLocalPresence(file, check, node); ok {
						t, shape = payload, presence
					}
				}
			}
			if annotation == nil && node.Kind == ast.KindVariableDeclaration && node.AsVariableDeclaration().Initializer != nil {
				// The checker sees numeric initializers as number. Diagnostic
				// qualification must retain the owned local's default i32 (or
				// other inferred owned type), exactly as body projection does.
				project := projection{file: file, check: check, result: result, bindings: bindings, functions: functions, integerTarget: target, aggregates: a}
				if value, failure := project.expr(node.AsVariableDeclaration().Initializer, ""); failure.Status == "" {
					t = value.Type
				}
			}
			if t != "" {
				bindings[check.GetSymbolAtLocation(node.Name())] = symbol{Type: t, Writable: true, Presence: shape}
			}
		}
		return node.ForEachChild(bind)
	}
	file.AsNode().ForEachChild(bind)
	var handled bool
	var failure *response
	var visit func(*ast.Node) bool
	visit = func(node *ast.Node) bool {
		if d.Pos() < node.Pos() || d.End() > node.End() {
			return false
		}
		var candidate *ast.Node
		expected := ""
		if d.Code() == 2322 && node.Kind == ast.KindPropertyDeclaration {
			candidate = node.AsPropertyDeclaration().Initializer
			expected = scalar(check.GetTypeOfSymbol(check.GetSymbolAtLocation(node.Name())))
		}
		if d.Code() == 2322 && node.Kind == ast.KindVariableDeclaration && node.AsVariableDeclaration().Type != nil {
			candidate = node.AsVariableDeclaration().Initializer
			expected = scalar(check.GetTypeFromTypeNode(node.AsVariableDeclaration().Type))
		}
		if d.Code() == 2322 && node.Kind == ast.KindReturnStatement {
			if function := ast.FindAncestor(node, ast.IsFunctionDeclaration); function != nil && function.AsFunctionDeclaration().Type != nil {
				candidate = node.AsReturnStatement().Expression
				expected = scalar(check.GetTypeFromTypeNode(function.AsFunctionDeclaration().Type))
			}
		}
		if d.Code() == 2322 && node.Kind == ast.KindBinaryExpression && ordinaryAssignmentOperator(node.AsBinaryExpression().OperatorToken.Kind) != "" {
			leaf := ast.SkipParentheses(node.AsBinaryExpression().Left)
			if leaf.Kind == ast.KindIdentifier {
				if binding, exists := bindings[check.GetSymbolAtLocation(leaf)]; exists && (strings.HasPrefix(binding.Type, "a") || (&projection{aggregates: a, check: check}).involvesAggregate(node)) {
					candidate = node
					expected = binding.Type
				}
			}
		}
		if d.Code() == 2322 && aggregateFieldMutation(node) {
			var operand *ast.Node
			switch node.Kind {
			case ast.KindBinaryExpression:
				operand = node.AsBinaryExpression().Left
			case ast.KindPrefixUnaryExpression:
				operand = node.AsPrefixUnaryExpression().Operand
			case ast.KindPostfixUnaryExpression:
				operand = node.AsPostfixUnaryExpression().Operand
			}
			if operand != nil {
				// Qualify the complete owned mutation, including its resolved
				// field identity, write permission and operator operand types.
				candidate = node
				expected = scalar(check.GetTypeAtLocation(operand))
			}
		}
		if d.Code() == 2322 && node.Kind == ast.KindParameter && ast.FindAncestor(node, ast.IsConstructorDeclaration) != nil {
			candidate = node.AsParameterDeclaration().Initializer
			expected = scalar(check.GetTypeOfSymbol(check.GetSymbolAtLocation(node.Name())))
		}
		if d.Code() == 2345 && node.Kind == ast.KindNewExpression {
			call := node.AsNewExpression()
			info := a.classes[check.GetSymbolAtLocation(call.Expression)]
			if info != nil && call.Arguments != nil {
				for index, arg := range call.Arguments.Nodes {
					if index < len(info.checked.Parameters()) && d.Pos() >= arg.Pos() && d.End() <= arg.End() {
						candidate = arg
						expected = scalar(check.GetTypeOfSymbol(info.checked.Parameters()[index]))
						break
					}
				}
			}
		}
		if d.Code() == 2345 && node.Kind == ast.KindCallExpression {
			call := node.AsCallExpression()
			if call.Expression.Kind == ast.KindIdentifier {
				if target, exists := functions[check.GetSymbolAtLocation(call.Expression)]; exists && call.Arguments != nil {
					for index, arg := range call.Arguments.Nodes {
						if index < len(target.Signature.Parameters) && !strings.HasPrefix(target.Signature.Parameters[index], "a") && d.Pos() >= arg.Pos() && d.End() <= arg.End() {
							candidate = arg
							expected = target.Signature.Parameters[index]
							break
						}
					}
				}
			}
		}
		if candidate != nil && expected != "" {
			var self *aggregateInfo
			if class := ast.FindAncestor(node, ast.IsClassLike); class != nil {
				self = a.classes[check.GetSymbolAtLocation(class.Name())]
			}
			p := projection{file: file, check: check, result: result, bindings: bindings, functions: functions, integerTarget: target, aggregates: a, self: self}
			value, owned := p.expr(candidate, expected)
			if owned.Status != "" {
				handled = true
				failure = &owned
				return true
			}
			if permittedPresence(value, nil) {
				handled = true
				return true
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
	return handled, failure
}

func (a *aggregateProjection) coverage(file *ast.SourceFile, check *checker.Checker, functions map[*ast.Symbol]functionInfo) *response {
	for _, node := range file.Statements.Nodes {
		if node.Kind == ast.KindClassDeclaration {
			info := a.classes[check.GetSymbolAtLocation(node.Name())]
			if info == nil {
				failure := failAt("r01-d-aggregate-identity", "error", file, node)
				return &failure
			}
			decl := a.facts.Declarations[info.index]
			body := decl.BodyRange
			a.facts.SourceCoverage = append(a.facts.SourceCoverage, sourceCoverage{Kind: "class", Name: decl.Name, Symbol: decl.Symbol, Range: decl.Range, BodyRange: &body})
			continue
		}
		if node.Kind != ast.KindFunctionDeclaration || node.Name() == nil || node.Body() == nil {
			failure := failAt("r02b-d-unsupported", "unsupported", file, node)
			return &failure
		}
		info, exists := functions[check.GetSymbolAtLocation(node.Name())]
		if !exists || info.Declaration != node || info.Conversion != nil || info.ID == "" {
			failure := failAt("r01-d-aggregate-identity", "error", file, node)
			return &failure
		}
		body := expressionOffset(file, node.Body())
		a.facts.SourceCoverage = append(a.facts.SourceCoverage, sourceCoverage{Kind: "function", Name: node.Name().Text(), Symbol: info.ID, Range: offset(file, node), BodyRange: &body})
	}
	return nil
}
