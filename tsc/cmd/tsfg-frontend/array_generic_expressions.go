// SPDX-License-Identifier: Apache-2.0
// Recursive source expressions preserve checked accesses through all wrappers.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/scanner"
	"strings"
)

func genericText(node *ast.Node) string {
	file := ast.GetSourceFileOfNode(node)
	return strings.Clone(file.Text()[scanner.SkipTrivia(file.Text(), node.Pos()):node.End()])
}
func genericTokenRange(node *ast.Node, start, end int) *span {
	file := ast.GetSourceFileOfNode(node)
	v := span{file.GetPositionMap().UTF8ToUTF16(start), file.GetPositionMap().UTF8ToUTF16(end)}
	return &v
}
func (p *arrayGenericProjection) scalarType(name string) string {
	for _, node := range p.scalars {
		if node.Name().Text() == name {
			return p.addType(arrayGenericType{ID: name, Kind: "scalar", Name: name, SourcePath: p.dts.FileName(), DeclarationRange: genericSpan(node), NameRange: genericSpan(node.Name())}, p.check.GetTypeFromTypeNode(node.AsTypeAliasDeclaration().Type))
		}
	}
	return ""
}
func (p *arrayGenericProjection) fits(actual, expected string) bool {
	if expected == "" || actual == expected {
		return true
	}
	if p.types[expected].Kind == "union" {
		for _, member := range p.types[expected].Members {
			if p.fits(actual, member) {
				return true
			}
		}
		if p.types[actual].Kind == "union" {
			for _, member := range p.types[actual].Members {
				if !p.fits(member, expected) {
					return false
				}
			}
			return true
		}
	}
	for base := p.types[actual].Base; base != ""; base = p.types[base].Base {
		if base == expected {
			return true
		}
	}
	return false
}
func (p *arrayGenericProjection) expression(node *ast.Node, expected string, state *arrayGenericState, depth int) (*arrayGenericExpression, error) {
	if node == nil {
		return nil, arrayFactsError("capability", "original expression required", nil)
	}
	if depth > 48 {
		return nil, arrayFactsError("limit", "v8 expression depth", nil)
	}
	if err := p.ctx.Err(); err != nil {
		return nil, err
	}
	actual := p.check.GetTypeAtLocation(node)
	if actual == nil {
		return nil, p.fail(node, "actual checked expression type required")
	}
	v := &arrayGenericExpression{Range: *genericSpan(node)}
	var err error
	switch node.Kind {
	case ast.KindIdentifier:
		symbol := p.check.GetSymbolAtLocation(node)
		b, ok := state.bindings[symbol]
		if ok {
			v.Kind, v.Type, v.Binding = "bindingReference", b.DeclaredType, b.ID
			v.FlowType, err = p.flow(actual, v.Type)
		} else if actual.Flags()&checker.TypeFlagsUndefined != 0 {
			v.Kind, v.Type = "undefinedLiteral", p.addType(arrayGenericType{ID: "undefined", Kind: "undefined"}, actual)
		} else {
			return nil, p.fail(node, "reference must join an actual in-scope binding")
		}
	case ast.KindNullKeyword:
		v.Kind, v.Type = "nullLiteral", p.addType(arrayGenericType{ID: "null", Kind: "null"}, actual)
	case ast.KindTrueKeyword, ast.KindFalseKeyword:
		if !actual.IsBooleanLike() {
			return nil, p.fail(node, "official boolean literal required")
		}
		v.Kind, v.Type, v.Token = "booleanLiteral", p.scalarType("bool"), genericText(node)
	case ast.KindStringLiteral:
		value, failure := stringFactsLiteralValue(ast.GetSourceFileOfNode(node), p.check, node)
		if failure != nil {
			return nil, failure
		}
		units, failure := stringFactsValueUnits(value)
		if failure != nil {
			return nil, failure
		}
		p.stringUnits += len(units)
		if p.stringUnits > stringFactsPackageUnitLimit {
			return nil, arrayFactsError("limit", "v8 String package code units", nil)
		}
		v.Kind, v.Type, v.CodeUnits = "stringLiteral", p.stringType(), &units
	case ast.KindNumericLiteral:
		v, err = p.number(node, expected)
	case ast.KindArrayLiteralExpression:
		if expected == "" {
			args := p.check.GetTypeArguments(actual)
			if !p.check.IsArrayType(actual) || len(args) != 1 {
				return nil, p.fail(node, "original inferred Array<T> required")
			}
			elements, ids, failure := p.arrayElements(node, "", state, depth)
			if failure != nil {
				return nil, failure
			}
			if len(ids) == 0 {
				return nil, p.fail(node, "empty Array literal requires concrete T annotation")
			}
			element, failure := p.union(ids, args[0])
			if failure != nil {
				return nil, failure
			}
			typ, failure := p.composite("array", element, actual)
			if failure != nil {
				return nil, failure
			}
			v, err = p.arrayValue(node, typ, elements)
			break
		}
		typ := p.types[expected]
		if typ.Kind != "array" {
			return nil, p.fail(node, "array literal requires its concrete declared Array<T>")
		}
		if !p.check.IsTypeAssignableTo(actual, p.actual[expected]) {
			return nil, p.fail(node, "literal must be assignable to actual Array<T>")
		}
		elements, _, failure := p.arrayElements(node, typ.ElementType, state, depth)
		if failure != nil {
			return nil, failure
		}
		v, err = p.arrayValue(node, expected, elements)
	case ast.KindCallExpression, ast.KindNewExpression:
		v, err = p.call(node, state, depth+1)
	case ast.KindElementAccessExpression:
		access := node.AsElementAccessExpression()
		if access.QuestionDotToken != nil {
			return nil, p.fail(node, "ordinary checked element access required")
		}
		receiver, failure := p.expression(access.Expression, "", state, depth+1)
		if failure != nil {
			return nil, failure
		}
		owner := p.types[receiver.FlowType]
		if owner.Kind != "array" && owner.Kind != "readSpan" {
			return nil, p.fail(node, "actual Array<T>/ReadSpan<T> receiver required")
		}
		index, failure := p.index(access.ArgumentExpression, false, state, depth+1)
		if failure != nil {
			return nil, failure
		}
		if !p.check.IsTypeAssignableTo(actual, p.actual[owner.ElementType]) {
			return nil, p.fail(node, "original index result must retain concrete T")
		}
		v.Kind, v.Type, v.Receiver, v.Index = "checkedElementAccess", owner.ElementType, receiver, index
		v.FlowType, err = p.flow(actual, owner.ElementType)
	case ast.KindPropertyAccessExpression:
		v, err = p.property(node, state, depth+1)
	case ast.KindParenthesizedExpression:
		inner, failure := p.expression(node.AsParenthesizedExpression().Expression, expected, state, depth+1)
		if failure != nil {
			return nil, failure
		}
		v.Kind, v.Type, v.FlowType, v.Expression = "parentheses", inner.Type, inner.FlowType, inner
	case ast.KindAsExpression:
		cast := node.AsAsExpression()
		typ, failure := p.annotation(cast.Type)
		if failure != nil {
			return nil, failure
		}
		context := p.assertionLiteralContext(cast.Expression, typ)
		inner, failure := p.expression(cast.Expression, context, state, depth+1)
		if failure != nil {
			return nil, failure
		}
		if failure := p.assertionRepresentation(node, inner.FlowType, typ); failure != nil {
			return nil, failure
		}
		v.CompatibilityRoots, failure = p.assertionClassView(inner.FlowType, typ, node)
		if failure != nil {
			return nil, failure
		}
		start := scanner.SkipTrivia(ast.GetSourceFileOfNode(node).Text(), cast.Expression.End())
		v.Kind, v.Type, v.Expression, v.AnnotationRange, v.OperatorRange = "as", typ, inner, genericSpan(cast.Type), genericTokenRange(node, start, start+2)
	case ast.KindNonNullExpression:
		innerNode := node.AsNonNullExpression().Expression
		inner, failure := p.expression(innerNode, "", state, depth+1)
		if failure != nil {
			return nil, failure
		}
		typ, failure := p.flow(actual, inner.FlowType)
		if failure != nil {
			return nil, failure
		}
		start := scanner.SkipTrivia(ast.GetSourceFileOfNode(node).Text(), innerNode.End())
		v.Kind, v.Type, v.Expression, v.OperatorRange = "nonNull", typ, inner, genericTokenRange(node, start, start+1)
	case ast.KindThisKeyword:
		if state.ownerType == "" {
			return nil, p.fail(node, "this requires its original constructor receiver")
		}
		v.Kind, v.Type = "thisReference", state.ownerType
	case ast.KindBinaryExpression:
		v, err = p.binary(node, expected, state, depth+1)
	case ast.KindPrefixUnaryExpression:
		prefix := node.AsPrefixUnaryExpression()
		if prefix.Operator == ast.KindPlusPlusToken || prefix.Operator == ast.KindMinusMinusToken {
			v, err = p.update(node, prefix.Operand, prefix.Operator, true, state, depth+1)
		} else if prefix.Operator == ast.KindMinusToken && prefix.Operand.Kind == ast.KindNumericLiteral {
			v, err = p.number(node, expected)
		} else {
			inner, failure := p.expression(prefix.Operand, expected, state, depth+1)
			if failure != nil {
				return nil, failure
			}
			start := scanner.SkipTrivia(ast.GetSourceFileOfNode(node).Text(), node.Pos())
			op := scanner.TokenToString(prefix.Operator)
			typ := inner.Type
			if prefix.Operator == ast.KindExclamationToken {
				typ = p.scalarType("bool")
			}
			v.Kind, v.Type, v.Expression, v.Operator, v.OperatorRange = "unary", typ, inner, op, genericTokenRange(node, start, start+len(op))
		}
	case ast.KindPostfixUnaryExpression:
		post := node.AsPostfixUnaryExpression()
		v, err = p.update(node, post.Operand, post.Operator, false, state, depth+1)
	case ast.KindDeleteExpression:
		return nil, arrayFactsDensity(ast.GetSourceFileOfNode(node), node, "delete cannot preserve dense Array storage")
	default:
		return nil, p.fail(node, "expression has no owned v8 representation")
	}
	if err != nil {
		return nil, err
	}
	if v.FlowType == "" {
		v.FlowType = v.Type
	}
	if !p.fits(v.FlowType, expected) || p.referenceViewRequired(v.FlowType, expected) {
		roots, failure := p.referenceCompatibility(v.FlowType, expected, node)
		if failure != nil {
			return nil, failure
		}
		v.CompatibilityRoots, failure = p.mergeCompatibilityRoots(v.CompatibilityRoots, roots)
		if failure != nil {
			return nil, failure
		}
	}
	if expected != "" && !p.check.IsTypeAssignableTo(actual, p.actual[expected]) {
		return nil, p.fail(node, "original checker assignment relation rejected expression")
	}
	return v, nil
}

func (p *arrayGenericProjection) referenceViewRequired(source, target string) bool {
	if p.types[source].Kind == "union" {
		for _, id := range p.types[source].Members {
			if p.referenceViewRequired(id, target) {
				return true
			}
		}
		return false
	}
	if p.types[source].Kind != "reference" || source == target {
		return false
	}
	if p.types[target].Kind == "reference" {
		return true
	}
	if p.types[target].Kind == "union" {
		for _, id := range p.types[target].Members {
			if id == source {
				return false
			}
		}
		for _, id := range p.types[target].Members {
			if p.types[id].Kind == "reference" && p.check.IsTypeAssignableTo(p.actual[source], p.actual[id]) {
				return true
			}
		}
	}
	return false
}

func (p *arrayGenericProjection) number(node *ast.Node, expected string) (*arrayGenericExpression, error) {
	actual := p.check.GetTypeAtLocation(node)
	if actual == nil || actual.Flags()&checker.TypeFlagsNumberLike == 0 {
		return nil, p.fail(node, "official numeric expression required")
	}
	kind := expected
	if p.types[kind].Kind == "union" {
		kind = ""
		for _, member := range p.types[expected].Members {
			if integerKind(member) || floatKind(member) {
				if kind != "" {
					return nil, p.fail(node, "literal requires one unambiguous numeric union constituent")
				}
				kind = member
			}
		}
	}
	if kind == "" {
		kind = "i32"
		if hint := floatDefaultHint(ast.GetSourceFileOfNode(node), node); hint != "" {
			kind = hint
		}
	}
	if !integerKind(kind) && !floatKind(kind) {
		return nil, p.fail(node, "numeric literal requires checked scalar context")
	}
	v := &arrayGenericExpression{Kind: "numericLiteral", Type: p.scalarType(kind), Range: *genericSpan(node)}
	if integerKind(kind) {
		value, literal, ok := exactInteger(ast.GetSourceFileOfNode(node), node)
		if !ok || kind != "isize" && kind != "usize" && !integerRange(value, kind, nil) {
			return nil, p.fail(node, "exact literal outside scalar integer domain")
		}
		v.Token, v.Decimal = literal.Token, literal.Decimal
	} else {
		token, ok := floatingNode(ast.GetSourceFileOfNode(node), node)
		bits, valid := exactFloatBits(token, kind)
		if !ok || !valid {
			return nil, p.fail(node, "exact finite floating literal required")
		}
		v.Token, v.Bits = token, bits
	}
	return v, nil
}
