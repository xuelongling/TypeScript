// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Adapts only exact module scalar contexts; every other official diagnostic survives.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
)

func moduleContextDiagnostic(program *compiler.Program, check *checker.Checker, d *ast.Diagnostic, functions map[*ast.Symbol]functionInfo, target *integerTarget) (bool, *response) {
	file := d.File()
	if file == nil || (d.Code() != 2322 && d.Code() != 2345) {
		return false, nil
	}
	globals := moduleGlobals(program, check)
	moduleAliases(program, check, functions, globals)
	for _, top := range file.Statements.Nodes {
		if d.Pos() < top.Pos() || d.End() > top.End() {
			continue
		}
		refs := map[*ast.Symbol]symbol{}
		for s, b := range globals {
			refs[s] = b
		}
		returnType := ""
		if top.Kind == ast.KindFunctionDeclaration && top.Body() != nil {
			decl := top.AsFunctionDeclaration()
			if decl.Type != nil {
				returnType = scalar(check.GetTypeFromTypeNode(decl.Type))
			}
			var contextBindings map[*ast.Symbol]symbol
			if target == nil {
				contextBindings = operatorBindings(file, check, decl)
			} else {
				contextBindings = integerContextBindings(file, check, decl, functions, target, globals)
			}
			for s, b := range contextBindings {
				refs[s] = b
			}
		}
		var matched bool
		var failure *response
		var visit func(*ast.Node) bool
		visit = func(node *ast.Node) bool {
			if d.Pos() < node.Pos() || d.End() > node.End() {
				return false
			}
			if node.ForEachChild(visit) {
				return true
			}
			var expressionNode *ast.Node
			expected := ""
			if d.Code() == 2345 && node.Kind == ast.KindCallExpression {
				call := node.AsCallExpression()
				callee, ok := functions[check.GetSymbolAtLocation(call.Expression)]
				if ok && call.Arguments != nil && len(call.Arguments.Nodes) == len(callee.Signature.Parameters) {
					for i, arg := range call.Arguments.Nodes {
						if d.Pos() >= arg.Pos() && d.End() <= arg.End() {
							expressionNode, expected = arg, callee.Signature.Parameters[i]
							break
						}
					}
				}
			}
			if d.Code() == 2322 {
				switch node.Kind {
				case ast.KindVariableDeclaration:
					local := node.AsVariableDeclaration()
					if local.Type != nil {
						expressionNode, expected = local.Initializer, scalar(check.GetTypeFromTypeNode(local.Type))
					}
				case ast.KindParameter:
					parameter := node.AsParameterDeclaration()
					if parameter.Type != nil {
						expressionNode = parameter.Initializer
						expected, _, _, _ = finitePresence(check.GetTypeFromTypeNode(parameter.Type))
					}
				case ast.KindReturnStatement:
					expressionNode, expected = node.AsReturnStatement().Expression, returnType
				case ast.KindExpressionStatement:
					candidate := node.AsExpressionStatement().Expression
					if candidate.Kind == ast.KindBinaryExpression {
						binary := candidate.AsBinaryExpression()
						if binary.OperatorToken.Kind == ast.KindEqualsToken {
							expressionNode, expected = binary.Right, refs[check.GetSymbolAtLocation(binary.Left)].Type
						} else if target != nil && ast.IsCompoundAssignment(binary.OperatorToken.Kind) && ordinaryAssignmentOperator(binary.OperatorToken.Kind) != "" {
							// Recheck the whole write in its official binding domain.
							// Two nonexternal scripts share globals; imports and foreign
							// external-module bindings do not grant writable targets.
							leaf := ast.SkipParentheses(binary.Left)
							resolved := check.GetSymbolAtLocation(leaf)
							binding, owned := refs[resolved]
							if leaf.Kind == ast.KindIdentifier && resolved != nil && resolved.Flags&ast.SymbolFlagsAlias == 0 && len(resolved.Declarations) > 0 &&
								owned && binding.Kind == "global" && binding.BindingKind == "source" && binding.Writable &&
								d.Pos() >= binary.Left.Pos() && d.End() <= binary.Left.End() {
								owner := ast.GetSourceFileOfNode(resolved.Declarations[0])
								if owner != nil && owner.FileName() == binding.SourcePath && program.GetSourceFile(binding.SourcePath) == owner &&
									(owner == file || !ast.IsExternalModule(owner) && !ast.IsExternalModule(file)) {
									expressionNode, expected = candidate, binding.Type
								}
							}
						}
					}
				}
			}
			if expressionNode == nil || (!integerKind(expected) && expected != "bool" && !(target != nil && target.floats && floatKind(expected))) {
				return false
			}
			if target != nil {
				_, failure = projectIntegerExpression(file, check, expressionNode, expected, refs, functions, target)
			} else {
				_, failure = projectExpression(file, check, expressionNode, expected, refs, functions)
			}
			if failure != nil {
				failure.Diagnostics[0].UpstreamCode = d.Code()
				if failure.Diagnostics[0].Code != "r01-d-float-range" {
					failure.Diagnostics[0].Range = span{file.GetPositionMap().UTF8ToUTF16(d.Pos()), file.GetPositionMap().UTF8ToUTF16(d.End())}
					failure.Diagnostics[0].Primary = sourcePosition(file, d.Pos(), d.End())
				}
			}
			matched = true
			return true
		}
		visit(top)
		if matched {
			return true, failure
		}
	}
	return false, nil
}
