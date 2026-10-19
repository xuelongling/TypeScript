// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

// Recursive expression adapters share this small interface so callable facts
// also survive within scalar arguments and operators. Official diagnostics own
// compatibility; this module records each binding, use and resolved call shape.
type functionValueProjector func(*ast.Node, string) (*expression, *response)

func projectFunctionValueExpression(file *ast.SourceFile, check *checker.Checker, node *ast.Node, expected string,
	refs map[*ast.Symbol]symbol, functions map[*ast.Symbol]functionInfo, valueRange span, project functionValueProjector,
) (*expression, *response, bool) {
	if node == nil {
		return nil, nil, false
	}
	failureAt := func(code, category string, at *ast.Node) (*expression, *response, bool) {
		failure := failAt(code, category, file, at)
		return nil, &failure, true
	}
	finish := func(value *expression) (*expression, *response, bool) {
		if expected != "" && expected != value.Type {
			return failureAt("r01-d-scalar-type", "error", node)
		}
		return value, nil, true
	}
	if node.Kind == ast.KindIdentifier || moduleReference(check, node) && functions[check.GetSymbolAtLocation(node)].ModuleCallable {
		upstream := check.GetSymbolAtLocation(node)
		binding, bound := refs[upstream]
		fn, declared := functions[upstream]
		if !bound && !declared || bound && binding.Signature == nil {
			return nil, nil, false
		}
		// A reference has the official use-site type, which can differ from
		// its binding's annotation or its initializer after control-flow analysis.
		sig, valid := functionValueType(check, check.GetTypeAtLocation(node))
		if !valid || !bound && (fn.Conversion != nil || fn.Declaration == nil || fn.Declaration.Kind != ast.KindFunctionDeclaration) {
			return failureAt("r02b-d-unsupported", "unsupported", node)
		}
		value := &expression{Kind: "reference", Type: "function", Symbol: binding.ID, Signature: sig, Range: valueRange}
		if !bound {
			value.Kind, value.Symbol, value.Target = "functionReference", "", fn.ID
		}
		return finish(value)
	}
	if node.Kind == ast.KindParenthesizedExpression {
		sig, valid := functionValueType(check, check.GetTypeAtLocation(node))
		if !valid {
			return nil, nil, false
		}
		operand, failure := project(node.AsParenthesizedExpression().Expression, "function")
		if failure != nil {
			return nil, failure, true
		}
		return finish(&expression{Kind: "group", Type: "function", Operand: operand, Signature: sig, Range: expressionOffset(file, node)})
	}
	if node.Kind != ast.KindCallExpression {
		return nil, nil, false
	}
	call := node.AsCallExpression()
	leaf := ast.SkipParentheses(call.Expression)
	binding := refs[check.GetSymbolAtLocation(leaf)]
	fn, direct := functions[check.GetSymbolAtLocation(call.Expression)]
	direct = direct && moduleReference(check, call.Expression)
	computed := call.Expression.Kind == ast.KindCallExpression || call.Expression.Kind == ast.KindParenthesizedExpression
	if !direct && binding.Signature == nil && !computed || direct && !signatureHasCallable(fn.Signature) {
		return nil, nil, false
	}
	if call.QuestionDotToken != nil || call.TypeArguments != nil {
		return failureAt("r02b-d-unsupported", "unsupported", node)
	}
	sig, valid := functionValueSignature(check, check.GetResolvedSignature(node), map[*checker.Type]bool{})
	if !valid {
		return failureAt("r02b-d-unsupported", "unsupported", node)
	}
	if len(call.Arguments.Nodes) != len(sig.Parameters) {
		failure := failCallAt("r02b-d-type", file, node)
		return nil, &failure, true
	}
	value := &expression{Kind: "call", Type: sig.ReturnType, Target: fn.ID, Signature: sig, Range: valueRange}
	if !direct {
		callee, failure := project(call.Expression, "function")
		if failure != nil {
			return nil, failure, true
		}
		// Signature declarations describe callable types. They do not prove
		// which function value is stored in this independently owned binding.
		value.Kind, value.Target, value.Callee = "indirectCall", "", callee
	}
	arguments := make([]expression, 0, len(call.Arguments.Nodes))
	for index, arg := range call.Arguments.Nodes {
		owned, failure := project(arg, sig.Parameters[index])
		if failure != nil {
			return nil, failure, true
		}
		arguments = append(arguments, *owned)
	}
	value.Arguments = &arguments
	return finish(value)
}

func projectOrdinaryFunctionValue(file *ast.SourceFile, check *checker.Checker, node *ast.Node, expected string,
	refs map[*ast.Symbol]symbol, functions map[*ast.Symbol]functionInfo,
) (*expression, *response, bool) {
	valueRange := span{}
	if node != nil {
		valueRange = offset(file, node)
	}
	return projectFunctionValueExpression(file, check, node, expected, refs, functions, valueRange, func(child *ast.Node, want string) (*expression, *response) {
		value, failure := projectExpression(file, check, child, want, refs, functions)
		return &value, failure
	})
}
