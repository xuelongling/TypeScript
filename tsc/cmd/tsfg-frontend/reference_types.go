// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

// Unknown bindings retain their declaration type. Only an actual occurrence of
// that same official symbol supplies a narrowed reference type; hints never do.
func referenceUseType(check *checker.Checker, node *ast.Node, binding symbol) string {
	if binding.Type != "unknown" {
		return binding.Type
	}
	upstream := check.GetSymbolAtLocation(node)
	if upstream == nil || check.GetTypeOfSymbol(upstream) != check.GetUnknownType() {
		return ""
	}
	actual := check.GetTypeAtLocation(node)
	if actual == check.GetUnknownType() {
		return "unknown"
	}
	if actual != nil && actual.IsBooleanLike() {
		return "bool"
	}
	return ""
}

// A plain write to unknown uses the current RHS value without a numeric hint.
// Both recursive expression profiles share this rule; it never updates the
// binding's declared type or substitutes a guessed type for a later read.
func projectUnknownWrite(file *ast.SourceFile, check *checker.Checker, node *ast.Node,
	refs map[*ast.Symbol]symbol, project func(*ast.Node) (*expression, *response),
) (*expression, *response, bool) {
	if node == nil || node.Kind != ast.KindBinaryExpression || node.AsBinaryExpression().OperatorToken.Kind != ast.KindEqualsToken {
		return nil, nil, false
	}
	binary := node.AsBinaryExpression()
	leaf := ast.SkipParentheses(binary.Left)
	if !moduleReference(check, leaf) {
		return nil, nil, false
	}
	upstream := check.GetSymbolAtLocation(leaf)
	binding, exists := refs[upstream]
	if !exists || binding.Type != "unknown" {
		return nil, nil, false
	}
	if !binding.Writable {
		failure := failAt("r01-d-scalar-type", "error", file, binary.Left)
		return nil, &failure, true
	}
	actual := check.GetTypeAtLocation(node)
	if upstream == nil || check.GetTypeOfSymbol(upstream) != check.GetUnknownType() ||
		check.GetTypeAtLocation(binary.Left) != check.GetUnknownType() || actual == nil || actual != check.GetTypeAtLocation(binary.Right) {
		failure := failAt("r02b-d-unsupported", "unsupported", file, node)
		return nil, &failure, true
	}
	left, failure := project(binary.Left)
	if failure != nil {
		return nil, failure, true
	}
	right, failure := project(binary.Right)
	if failure != nil {
		return nil, failure, true
	}
	return &expression{
		Kind: "write", Type: right.Type, Operator: "=", Left: left, Right: right,
		Signature: right.Signature, Range: expressionOffset(file, node),
	}, nil, true
}
