// SPDX-License-Identifier: Apache-2.0
// Finite values retain lexical spelling and official symbol/type provenance.
package main

import (
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

func (p *arrayFactsProjection) reference(node *ast.Node, bindings map[*ast.Symbol]arrayFactsBinding) (*arrayFactsValue, error) {
	if node == nil || node.Kind != ast.KindIdentifier {
		return nil, arrayFactsCapability(p.file, node, "direct prior Array binding required")
	}
	binding, ok := bindings[p.check.GetSymbolAtLocation(node)]
	if !ok || !p.arrayType(p.check.GetTypeAtLocation(node), false) {
		return nil, arrayFactsCapability(p.file, node, "original prior f64 Array symbol/use type required")
	}
	return &arrayFactsValue{Kind: "reference", Binding: binding.ID, FlowType: "array.f64", Range: expressionOffset(p.file, node)}, nil
}
func (p *arrayFactsProjection) scalarReference(node *ast.Node, kind string, state *arrayFactsFunctionState) (*arrayFactsValue, error) {
	if node == nil || node.Kind != ast.KindIdentifier {
		return nil, arrayFactsCapability(p.file, node, "direct required scalar parameter reference required")
	}
	parameter, ok := state.parameters[p.check.GetSymbolAtLocation(node)]
	if !ok || parameter.DeclaredType != kind || p.check.GetTypeAtLocation(node) != p.check.GetNumberType() {
		return nil, arrayFactsCapability(p.file, node, "original same-function pinned scalar parameter/use required")
	}
	return &arrayFactsValue{Kind: "reference", Binding: parameter.ID, FlowType: kind, Range: expressionOffset(p.file, node)}, nil
}
func (p *arrayFactsProjection) floating(node *ast.Node) (*arrayFactsValue, error) {
	if node == nil {
		return nil, arrayFactsCapability(p.file, node, "finite numeric literal required")
	}
	token, valid := floatingNode(p.file, node)
	if !valid {
		return nil, arrayFactsCapability(p.file, node, "finite numeric literal required")
	}
	typ := p.check.GetTypeAtLocation(node)
	if typ == nil || typ.Flags()&checker.TypeFlagsNumberLike == 0 {
		return nil, arrayFactsCapability(p.file, node, "official numeric literal type required")
	}
	bits, valid := exactFloatBits(token, "f64")
	if !valid {
		return nil, arrayFactsCapability(p.file, node, "finite f64 literal required")
	}
	return &arrayFactsValue{Kind: "numericLiteral", Type: "f64", Range: expressionOffset(p.file, node), Token: strings.Clone(token), Bits: bits}, nil
}
func (p *arrayFactsProjection) literal(node *ast.Node, declared *checker.Type) (*arrayFactsValue, error) {
	elements := node.AsArrayLiteralExpression().Elements.Nodes
	actual := p.check.GetTypeAtLocation(node)
	if !p.arrayType(actual, len(elements) == 0) || !p.check.IsTypeAssignableTo(actual, declared) {
		return nil, arrayFactsCapability(p.file, node, "official dense f64 Array literal context required")
	}
	site, err := arrayFactsID("tsfg.array-facts.allocation-site.v1\x00", struct {
		SourcePath string `json:"sourcePath"`
		Range      span   `json:"range"`
	}{p.file.FileName(), expressionOffset(p.file, node)})
	if err != nil {
		return nil, err
	}
	values := make([]arrayFactsValue, 0, len(elements))
	for _, element := range elements {
		if err := p.ctx.Err(); err != nil {
			return nil, arrayFactsError("canceled", "literal elements", err)
		}
		value, err := p.floating(element)
		if err != nil {
			return nil, err
		}
		values = append(values, *value)
	}
	return &arrayFactsValue{Kind: "arrayLiteral", Type: "array.f64", Range: expressionOffset(p.file, node), AllocationSite: site, Elements: &values}, nil
}
func (p *arrayFactsProjection) unsigned(node *ast.Node, length bool, state *arrayFactsFunctionState) (*arrayFactsValue, error) {
	if node != nil && node.Kind == ast.KindIdentifier {
		return p.scalarReference(node, "u32", state)
	}
	if node == nil || node.Kind != ast.KindNumericLiteral {
		return nil, arrayFactsCapability(p.file, node, "original NumericLiteral or required u32 parameter required")
	}
	token, valid := floatingNode(p.file, node)
	maximum, kind := uint64(4294967294), "indexLiteral"
	if length {
		maximum, kind = 4294967295, "lengthLiteral"
	}
	decimal, exact := arrayFactsUnsigned(token, maximum)
	typ := p.check.GetTypeAtLocation(node)
	if !valid || !exact || typ == nil || typ.Flags()&checker.TypeFlagsNumberLike == 0 {
		return nil, arrayFactsCapability(p.file, node, "exact integral Array index/length domain required")
	}
	return &arrayFactsValue{Kind: kind, Type: "u32", Range: expressionOffset(p.file, node), Token: strings.Clone(token), Decimal: decimal}, nil
}
func (p *arrayFactsProjection) read(node *ast.Node, state *arrayFactsFunctionState) (*arrayFactsValue, error) {
	if node == nil || node.Kind != ast.KindElementAccessExpression {
		return nil, arrayFactsCapability(p.file, node, "ordinary named Array checked read required")
	}
	access := node.AsElementAccessExpression()
	if access.QuestionDotToken != nil {
		return nil, arrayFactsCapability(p.file, node, "ordinary Array element access required")
	}
	array, err := p.reference(access.Expression, state.bindings)
	if err != nil {
		return nil, err
	}
	index, err := p.unsigned(access.ArgumentExpression, false, state)
	if err != nil {
		return nil, err
	}
	if p.check.GetTypeAtLocation(node) != p.check.GetNumberType() {
		return nil, arrayFactsCapability(p.file, node, "official f64 Array element result required")
	}
	return &arrayFactsValue{Kind: "checkedElementAccess", Type: "f64", Range: expressionOffset(p.file, node), Array: array, Index: index}, nil
}
func (p *arrayFactsProjection) rhs(node *ast.Node, state *arrayFactsFunctionState) (*arrayFactsValue, error) {
	if node != nil && node.Kind == ast.KindIdentifier {
		return p.scalarReference(node, "f64", state)
	}
	if node != nil && node.Kind == ast.KindElementAccessExpression {
		return p.read(node, state)
	}
	return p.floating(node)
}
