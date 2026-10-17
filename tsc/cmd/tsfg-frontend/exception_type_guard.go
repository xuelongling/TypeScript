// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

// Both ordinary and integer expression paths use their existing projector for
// the original operand. The guard records official syntax/type facts only.
func projectExceptionTypeTest(file *ast.SourceFile, check *checker.Checker, node *ast.Node,
	project func(*ast.Node, string) (*expression, *response),
) (*expression, *response, bool) {
	if node == nil || node.Kind != ast.KindBinaryExpression {
		return nil, nil, false
	}
	binary := node.AsBinaryExpression()
	typeofNode, tag := binary.Left, binary.Right
	if typeofNode.Kind != ast.KindTypeOfExpression {
		typeofNode, tag = tag, typeofNode
	}
	if typeofNode.Kind != ast.KindTypeOfExpression || tag.Kind != ast.KindStringLiteral {
		return nil, nil, false
	}
	operator := integerOperator(binary.OperatorToken.Kind)
	if operator != "===" && operator != "!==" {
		return nil, nil, false
	}
	unsupported := func() (*expression, *response, bool) {
		failure := failAt("r02b-d-unsupported", "unsupported", file, node)
		return nil, &failure, true
	}
	conditionType, tagType := check.GetTypeAtLocation(node), check.GetTypeAtLocation(tag)
	if tag.Text() != "boolean" || conditionType == nil || !conditionType.IsBooleanLike() ||
		tagType == nil || tagType.Flags()&checker.TypeFlagsStringLiteral == 0 {
		return unsupported()
	}
	operand := typeofNode.AsTypeOfExpression().Expression
	actual := check.GetTypeAtLocation(operand)
	want := "unknown"
	if actual != check.GetUnknownType() {
		if actual == nil || !actual.IsBooleanLike() {
			return unsupported()
		}
		want = "bool"
	}
	value, failure := project(operand, want)
	if failure != nil {
		return nil, failure, true
	}
	return &expression{
		Kind: "type-test", Type: "bool", Operator: operator, Operand: value,
		TypeTest: &typeTestFact{Tag: tag.Text(), TypeofRange: offset(file, typeofNode), TagRange: offset(file, tag)},
		Range:    expressionOffset(file, node),
	}, nil, true
}
