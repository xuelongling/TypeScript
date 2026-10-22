// SPDX-License-Identifier: Apache-2.0
// Source targets occur once. Runtime owns materialization, checks and old values.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/scanner"
	"strconv"
)

func (p *arrayGenericProjection) index(node *ast.Node, length bool, state *arrayGenericState, depth int) (*arrayGenericExpression, error) {
	if node.Kind == ast.KindNumericLiteral {
		maximum := uint64(4294967294)
		if length {
			maximum = 4294967295
		}
		decimal, ok := arrayFactsUnsigned(genericText(node), maximum)
		if !ok {
			return nil, p.fail(node, "exact u32 index/length domain required")
		}
		actual := p.check.GetTypeAtLocation(node)
		if actual == nil || actual.Flags()&checker.TypeFlagsNumberLike == 0 {
			return nil, p.fail(node, "actual NumericLiteral index type required")
		}
		return &arrayGenericExpression{Kind: "numericLiteral", Type: p.scalarType("u32"), FlowType: "u32", Range: *genericSpan(node), Token: genericText(node), Decimal: decimal}, nil
	}
	if node.Kind == ast.KindParenthesizedExpression {
		inner, err := p.index(node.AsParenthesizedExpression().Expression, length, state, depth+1)
		if err != nil {
			return nil, err
		}
		return &arrayGenericExpression{Kind: "parentheses", Type: "u32", FlowType: "u32", Range: *genericSpan(node), Expression: inner}, nil
	}
	return p.expression(node, p.scalarType("u32"), state, depth+1)
}
func (p *arrayGenericProjection) target(node *ast.Node, state *arrayGenericState, depth int) (*arrayGenericTarget, error) {
	if node.Kind == ast.KindParenthesizedExpression {
		v, err := p.target(node.AsParenthesizedExpression().Expression, state, depth+1)
		if err == nil {
			v.Range = *genericSpan(node)
		}
		return v, err
	}
	if node.Kind == ast.KindIdentifier {
		binding, ok := state.bindings[p.check.GetSymbolAtLocation(node)]
		if !ok || !binding.Writable {
			return nil, p.fail(node, "actual writable binding required")
		}
		return &arrayGenericTarget{Kind: "bindingTarget", Type: binding.DeclaredType, Range: *genericSpan(node), Binding: binding.ID}, nil
	}
	value, err := p.expression(node, "", state, depth+1)
	if err != nil {
		return nil, err
	}
	v := &arrayGenericTarget{Type: value.Type, Range: value.Range, Receiver: value.Receiver, Index: value.Index, Property: value.Property, NameRange: value.NameRange}
	switch value.Kind {
	case "checkedElementAccess":
		if p.types[value.Receiver.FlowType].Kind != "array" {
			return nil, arrayFactsDensity(ast.GetSourceFileOfNode(node), node, "ReadSpan element is readonly")
		}
		v.Kind = "elementTarget"
	case "arrayLength":
		v.Kind = "lengthTarget"
	case "readSpanLength":
		return nil, arrayFactsDensity(ast.GetSourceFileOfNode(node), node, "ReadSpan length is readonly")
	case "fieldAccess":
		field := p.fields[p.check.GetSymbolAtLocation(node.Name())]
		if field.Readonly && (state.header == nil || state.header.node.Kind != ast.KindConstructor) {
			return nil, p.fail(node, "field is readonly")
		}
		v.Kind = "fieldTarget"
	default:
		return nil, p.fail(node, "checked mutation target required")
	}
	return v, nil
}
func (p *arrayGenericProjection) mutationDensity(target *arrayGenericTarget, value *arrayGenericExpression, compound bool, node *ast.Node, state *arrayGenericState) error {
	if target.Kind != "elementTarget" && target.Kind != "lengthTarget" {
		if target.Kind == "bindingTarget" && p.types[target.Type].Kind == "array" {
			// Rebinding may connect aliases whose object identity is intentionally
			// not evaluated here. Discard every static length proof; runtime checked
			// targets remain intact and no stale root can reject a legal append.
			state.invalidateLengths()
			state.lengths[target.Binding] = &arrayFactsLengthState{}
		}
		return nil
	}
	length := state.lengths[genericOwnerBinding(target.Receiver)]
	if length == nil || !length.known {
		// A receiver reached through a field/element/unknown alias can denote an
		// existing local root. Its write invalidates those static proofs too.
		state.invalidateLengths()
		return nil
	}
	if target.Kind == "lengthTarget" {
		if value.Kind == "numericLiteral" && value.Decimal != "" {
			n, err := strconv.ParseUint(value.Decimal, 10, 32)
			if err == nil {
				if n > length.length {
					return arrayFactsDensity(ast.GetSourceFileOfNode(node), node, "known Array length growth would create holes")
				}
				length.length = n
				return nil
			}
		}
		length.known = false
		return nil
	}
	if target.Index.Kind != "numericLiteral" || target.Index.Decimal == "" {
		length.known = false
		return nil
	}
	index, err := strconv.ParseUint(target.Index.Decimal, 10, 32)
	if err != nil {
		return err
	}
	if index > length.length || compound && index >= length.length {
		return arrayFactsDensity(ast.GetSourceFileOfNode(node), node, "known Array gap or missing old element")
	}
	if index == length.length {
		length.length++
	}
	return nil
}

func (state *arrayGenericState) invalidateLengths() {
	for _, length := range state.lengths {
		length.known = false
	}
}

func genericOwnerBinding(value *arrayGenericExpression) string {
	if value == nil {
		return ""
	}
	if value.Binding != "" {
		return value.Binding
	}
	if value.Expression != nil {
		return genericOwnerBinding(value.Expression)
	}
	return ""
}
func (p *arrayGenericProjection) binary(node *ast.Node, expected string, state *arrayGenericState, depth int) (*arrayGenericExpression, error) {
	b := node.AsBinaryExpression()
	op := b.OperatorToken.Kind
	operator := genericText(b.OperatorToken)
	if op >= ast.KindFirstAssignment && op <= ast.KindLastAssignment {
		target, err := p.target(b.Left, state, depth+1)
		if err != nil {
			return nil, err
		}
		compound := op != ast.KindEqualsToken
		if target.Kind == "lengthTarget" && compound {
			return nil, arrayFactsDensity(ast.GetSourceFileOfNode(node), node, "Array length only supports checked direct truncation")
		}
		var value *arrayGenericExpression
		if target.Kind == "lengthTarget" {
			value, err = p.index(b.Right, true, state, depth+1)
		} else {
			value, err = p.expression(b.Right, target.Type, state, depth+1)
		}
		if err != nil {
			return nil, err
		}
		if compound && !integerKind(target.Type) && !floatKind(target.Type) {
			return nil, p.fail(node, "compound operation requires numeric concrete T")
		}
		if err := p.mutationDensity(target, value, compound, b.Left, state); err != nil {
			return nil, err
		}
		kind := "bindingAssignment"
		switch target.Kind {
		case "elementTarget":
			kind = "elementAssignment"
			if compound {
				kind = "elementCompoundAssignment"
			}
		case "lengthTarget":
			kind = "lengthAssignment"
		case "fieldTarget":
			kind = "fieldAssignment"
		}
		flow := target.Type
		if !compound {
			flow = value.FlowType
			if !p.check.IsTypeAssignableTo(p.check.GetTypeAtLocation(node), p.actual[flow]) {
				return nil, p.fail(node, "original assignment result must preserve RHS flow type")
			}
		}
		return &arrayGenericExpression{Kind: kind, Type: target.Type, FlowType: flow, Range: *genericSpan(node), Operator: operator, OperatorRange: genericSpan(b.OperatorToken), Target: target, Value: value}, nil
	}
	left, err := p.expression(b.Left, "", state, depth+1)
	if err != nil {
		return nil, err
	}
	context := left.FlowType
	if op == ast.KindQuestionQuestionToken && p.types[context].Kind == "union" {
		context = ""
		for _, id := range p.types[left.FlowType].Members {
			if id != "undefined" && id != "null" {
				if context != "" {
					return nil, p.fail(node, "coalesce requires one concrete payload")
				}
				context = id
			}
		}
	}
	right, err := p.expression(b.Right, context, state, depth+1)
	if err != nil {
		return nil, err
	}
	typ, kind := left.FlowType, "binary"
	actual := p.check.GetTypeAtLocation(node)
	if actual.IsBooleanLike() {
		typ = p.scalarType("bool")
	} else if actual.Flags()&checker.TypeFlagsStringLike != 0 {
		typ = p.stringType()
		if op == ast.KindPlusToken {
			kind = "stringConcat"
		}
	} else if op == ast.KindQuestionQuestionToken {
		typ = right.FlowType
	}
	return &arrayGenericExpression{Kind: kind, Type: typ, Range: *genericSpan(node), Left: left, Right: right, Operator: operator, OperatorRange: genericSpan(b.OperatorToken)}, nil
}
func (p *arrayGenericProjection) update(node, operand *ast.Node, operator ast.Kind, prefix bool, state *arrayGenericState, depth int) (*arrayGenericExpression, error) {
	target, err := p.target(operand, state, depth+1)
	if err != nil {
		return nil, err
	}
	if !integerKind(target.Type) && !floatKind(target.Type) {
		return nil, p.fail(node, "update requires numeric concrete T")
	}
	kind := "elementUpdate"
	if target.Kind == "bindingTarget" {
		kind = "bindingUpdate"
	}
	if target.Kind == "fieldTarget" {
		kind = "fieldUpdate"
	}
	if target.Kind == "lengthTarget" {
		return nil, arrayFactsDensity(ast.GetSourceFileOfNode(node), node, "Array length update would change density")
	}
	if err := p.mutationDensity(target, nil, true, operand, state); err != nil {
		return nil, err
	}
	file := ast.GetSourceFileOfNode(node)
	start := scanner.SkipTrivia(file.Text(), node.Pos())
	if !prefix {
		start = scanner.SkipTrivia(file.Text(), operand.End())
	}
	return &arrayGenericExpression{Kind: kind, Type: target.Type, Range: *genericSpan(node), Target: target, Operator: scanner.TokenToString(operator), OperatorRange: genericTokenRange(node, start, start+2), Prefix: &prefix}, nil
}
