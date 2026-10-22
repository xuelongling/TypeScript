// SPDX-License-Identifier: Apache-2.0
// Both inferred and annotated literals share density and allocation provenance.
package main

import "github.com/microsoft/typescript-go/internal/ast"

func (p *arrayGenericProjection) arrayElements(node *ast.Node, context string, state *arrayGenericState, depth int) ([]arrayGenericExpression, []string, error) {
	elements := []arrayGenericExpression{}
	types := []string{}
	for _, element := range node.AsArrayLiteralExpression().Elements.Nodes {
		if element.Kind == ast.KindOmittedExpression || element.Kind == ast.KindSpreadElement {
			return nil, nil, arrayFactsDensity(ast.GetSourceFileOfNode(node), element, "dense literal cannot contain holes or spreads")
		}
		value, err := p.expression(element, context, state, depth+1)
		if err != nil {
			return nil, nil, err
		}
		elements = append(elements, *value)
		types = append(types, value.FlowType)
	}
	return elements, types, nil
}
func (p *arrayGenericProjection) arrayValue(node *ast.Node, typ string, elements []arrayGenericExpression) (*arrayGenericExpression, error) {
	id, err := genericNodeID("tsfg.array-facts.allocation.v8\x00", node)
	if err != nil {
		return nil, err
	}
	return &arrayGenericExpression{Kind: "arrayLiteral", Type: typ, Range: *genericSpan(node), Elements: &elements, AllocationSite: id}, nil
}
