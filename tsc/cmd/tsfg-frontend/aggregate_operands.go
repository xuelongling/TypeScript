// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
)

func aggregateFieldMutation(node *ast.Node) bool {
	node = ast.SkipParentheses(node)
	if node.Kind == ast.KindBinaryExpression {
		b := node.AsBinaryExpression()
		return ordinaryAssignmentOperator(b.OperatorToken.Kind) != "" && ast.SkipParentheses(b.Left).Kind == ast.KindPropertyAccessExpression
	}
	if node.Kind == ast.KindPrefixUnaryExpression {
		return ast.SkipParentheses(node.AsPrefixUnaryExpression().Operand).Kind == ast.KindPropertyAccessExpression && (node.AsPrefixUnaryExpression().Operator == ast.KindPlusPlusToken || node.AsPrefixUnaryExpression().Operator == ast.KindMinusMinusToken)
	}
	if node.Kind == ast.KindPostfixUnaryExpression {
		return ast.SkipParentheses(node.AsPostfixUnaryExpression().Operand).Kind == ast.KindPropertyAccessExpression
	}
	return false
}

// fieldExpr owns the actual resolved field symbols along an ordered path.
// A this read denotes a location; it does not assert initialized storage.
func (p *projection) fieldExpr(node *ast.Node, write bool) (*expression, response) {
	access := node.AsPropertyAccessExpression()
	if access.QuestionDotToken != nil {
		return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node)
	}
	var receiver *expression
	var failure response
	if access.Expression.Kind == ast.KindPropertyAccessExpression {
		receiver, failure = p.fieldExpr(access.Expression, false)
	} else {
		receiver, failure = p.expr(access.Expression, "")
	}
	if failure.Status != "" {
		return nil, failure
	}
	info := p.aggregates.types[p.check.GetTypeAtLocation(access.Expression)]
	if access.Expression.Kind == ast.KindThisKeyword {
		info = p.self
	}
	if info == nil {
		return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node)
	}
	resolved := p.check.GetSymbolAtLocation(access.Name())
	ordinal, exists := info.fields[resolved]
	if !exists {
		return nil, failAt("r01-d-aggregate-identity", "error", p.file, node)
	}
	decl := p.aggregates.facts.Declarations[info.index]
	field := decl.Fields[ordinal]
	self := p.self == info && access.Expression.Kind == ast.KindThisKeyword
	readable := p.check.IsPropertyAccessible(node, false, false, info.instance, resolved)
	writable := p.check.IsPropertyAccessible(node, false, true, info.instance, resolved) && (!field.Readonly || self)
	if !readable || write && !writable {
		return nil, failAt("r01-d-aggregate-write", "error", p.file, node)
	}
	path := []string{field.ID}
	if receiver.Kind == "field" {
		path = append(append([]string{}, receiver.FieldPath...), field.ID)
		receiver = receiver.Receiver
	}
	return &expression{Kind: "field", Type: field.Type, Receiver: receiver, FieldPath: path, Symbol: field.Symbol, Permission: &fieldPermission{Readable: readable, Writable: writable, ConstructorSelf: self, ClassRef: decl.ID}, Range: expressionOffset(p.file, node)}, response{}
}

// An aggregate expression uses lexical token spans. When it delegates an
// operand subtree to the ordinary scalar projector, retain that projector's
// checked facts and normalize only this owned subtree against its actual AST.
func (p *projection) aggregateOperand(node *ast.Node, expected string, presence ...*parameterPresence) (*expression, response) {
	value, failure := p.expr(node, expected, presence...)
	if failure.Status != "" {
		return nil, failure
	}
	if !p.involvesAggregate(node) {
		p.aggregateOperandRanges(value, node)
	}
	return value, response{}
}

func (p *projection) aggregateOperandRanges(value *expression, node *ast.Node) {
	if value == nil || node == nil {
		return
	}
	value.Range = expressionOffset(p.file, node)
	switch node.Kind {
	case ast.KindParenthesizedExpression:
		p.aggregateOperandRanges(value.Operand, node.AsParenthesizedExpression().Expression)
	case ast.KindBinaryExpression:
		binary := node.AsBinaryExpression()
		p.aggregateOperandRanges(value.Left, binary.Left)
		p.aggregateOperandRanges(value.Right, binary.Right)
	case ast.KindPrefixUnaryExpression:
		p.aggregateOperandRanges(value.Operand, node.AsPrefixUnaryExpression().Operand)
	case ast.KindPostfixUnaryExpression:
		p.aggregateOperandRanges(value.Operand, node.AsPostfixUnaryExpression().Operand)
	case ast.KindCallExpression:
		call := node.AsCallExpression()
		if call.Arguments != nil {
			if value.Kind == "conversion" && len(call.Arguments.Nodes) == 1 {
				p.aggregateOperandRanges(value.Operand, call.Arguments.Nodes[0])
			}
			if value.Arguments != nil {
				for index, argument := range call.Arguments.Nodes {
					if index < len(*value.Arguments) {
						p.aggregateOperandRanges(&(*value.Arguments)[index], argument)
					}
				}
			}
		}
	}
}
