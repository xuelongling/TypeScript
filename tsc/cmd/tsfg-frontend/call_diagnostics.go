// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Preserves source-anchored call diagnostics and scalar call context.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"math/big"
)

func supportedOperatorReturn(file *ast.SourceFile, check *checker.Checker, d *ast.Diagnostic, functions map[*ast.Symbol]functionInfo) bool {
	if d.File() != file || d.Code() != 2322 {
		return false
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			continue
		}
		decl := node.AsFunctionDeclaration()
		if decl.Type == nil || scalar(check.GetTypeFromTypeNode(decl.Type)) != "i32" {
			continue
		}
		bindings := operatorBindings(file, check, decl)
		if visitStatements(decl.Body.AsBlock().Statements.Nodes, func(item *ast.Node) bool {
			if item.Kind != ast.KindReturnStatement || d.Pos() < item.Pos() || d.End() > item.End() {
				return false
			}
			expr := item.AsReturnStatement().Expression
			if expr != nil && (expr.Kind == ast.KindBinaryExpression || expr.Kind == ast.KindPrefixUnaryExpression || expr.Kind == ast.KindParenthesizedExpression) {
				candidate := expr
				for candidate.Kind == ast.KindParenthesizedExpression {
					candidate = candidate.AsParenthesizedExpression().Expression
				}
				if candidate.Kind == ast.KindBinaryExpression && candidate.AsBinaryExpression().OperatorToken.Kind == ast.KindAsteriskAsteriskToken {
					binary := candidate.AsBinaryExpression()
					left, leftFailure := projectOperator(file, check, binary.Left, bindings, functions)
					right, rightFailure := projectOperator(file, check, binary.Right, bindings, functions)
					if leftFailure == nil && rightFailure == nil && left.Type == "i32" && right.Type == "i32" {
						return true
					}
				}
				projected, failure := projectOperator(file, check, expr, bindings, functions)
				return failure == nil && projected.Type == "i32"
			}
			return false
		}) {
			return true
		}
	}
	return false
}

func supportedOperatorCallArgument(file *ast.SourceFile, check *checker.Checker, d *ast.Diagnostic, functions map[*ast.Symbol]functionInfo) bool {
	if d.File() != file || d.Code() != 2345 {
		return false
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			continue
		}
		bindings := operatorBindings(file, check, node.AsFunctionDeclaration())
		var visit func(*ast.Node) bool
		visit = func(candidate *ast.Node) bool {
			if candidate.Kind == ast.KindCallExpression {
				call := candidate.AsCallExpression()
				if moduleReference(check, call.Expression) && call.QuestionDotToken == nil && call.TypeArguments == nil {
					target, ok := functions[check.GetSymbolAtLocation(call.Expression)]
					if ok && len(call.Arguments.Nodes) == len(target.Signature.Parameters) {
						for index, argument := range call.Arguments.Nodes {
							if target.Signature.Parameters[index] == "i32" && d.Pos() >= argument.Pos() && d.End() <= argument.End() {
								projected, failure := projectExpression(file, check, argument, "i32", bindings, functions)
								if failure == nil && projected.Type == "i32" {
									return true
								}
							}
						}
					}
				}
			}
			return candidate.ForEachChild(visit)
		}
		if node.Body().ForEachChild(visit) {
			return true
		}
	}
	return false
}

func callDiagnostic(file *ast.SourceFile, d *ast.Diagnostic) (string, bool) {
	if d.File() != file {
		return "", false
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			continue
		}
		for _, item := range node.Body().AsBlock().Statements.Nodes {
			if item.Kind != ast.KindReturnStatement {
				continue
			}
			expr := item.AsReturnStatement().Expression
			if expr == nil || expr.Kind != ast.KindCallExpression || d.Pos() < item.Pos() || d.End() > item.End() {
				continue
			}
			if d.Code() != 2322 && (d.Pos() < expr.Pos() || d.End() > expr.End()) {
				continue
			}
			switch d.Code() {
			case 2304:
				return "r01-d-unresolved-name", true
			case 2345, 2322:
				return "r01-d-scalar-type", true
			case 2554:
				return "r02b-d-type", true
			}
		}
	}
	return "", false
}

func i32LiteralCallArgument(file *ast.SourceFile, check *checker.Checker, d *ast.Diagnostic) (*big.Int, bool) {
	if d.File() != file || d.Code() != 2345 {
		return nil, false
	}
	var inspectCall func(*ast.Node) (*big.Int, bool)
	inspectCall = func(expr *ast.Node) (*big.Int, bool) {
		if expr == nil || expr.Kind != ast.KindCallExpression {
			return nil, false
		}
		call := expr.AsCallExpression()
		for _, argument := range call.Arguments.Nodes {
			if d.Pos() >= argument.Pos() && d.End() <= argument.End() {
				if value, ok := inspectCall(argument); ok {
					return value, true
				}
			}
		}
		if !moduleReference(check, call.Expression) {
			return nil, false
		}
		callee := check.GetSymbolAtLocation(call.Expression)
		for _, declaration := range file.Statements.Nodes {
			if declaration.Kind != ast.KindFunctionDeclaration || declaration.Name() == nil ||
				check.GetSymbolAtLocation(declaration.Name()) != callee {
				continue
			}
			parameters := declaration.AsFunctionDeclaration().Parameters.Nodes
			for index, argument := range call.Arguments.Nodes {
				if index >= len(parameters) || d.Pos() < argument.Pos() || d.End() > argument.End() {
					continue
				}
				paramType := parameters[index].AsParameterDeclaration().Type
				if paramType != nil && scalar(check.GetTypeFromTypeNode(paramType)) == "i32" {
					value, _, ok := exactInteger(file, argument)
					return value, ok
				}
			}
		}
		return nil, false
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			continue
		}
		for _, item := range node.Body().AsBlock().Statements.Nodes {
			if item.Kind != ast.KindReturnStatement {
				continue
			}
			if value, ok := inspectCall(item.AsReturnStatement().Expression); ok {
				return value, true
			}
		}
	}
	return nil, false
}
