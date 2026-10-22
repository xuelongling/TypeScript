// SPDX-License-Identifier: Apache-2.0
// A super invocation initializes the current object; it allocates no new owner.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

func (p *arrayGenericProjection) superConstruct(node *ast.Node, state *arrayGenericState, depth int) (*arrayGenericExpression, error) {
	if state.header == nil || state.header.node.Kind != ast.KindConstructor || state.ownerType == "" {
		return nil, p.fail(node, "super must retain its actual explicit constructor owner")
	}
	base := p.types[state.ownerType].Base
	if base == "" || p.types[base].Construction == nil {
		return nil, p.fail(node, "super requires its closed actual direct base")
	}
	bases := p.check.GetBaseTypes(p.actual[state.ownerType])
	if len(bases) != 1 || p.classes[bases[0].Symbol()] != base {
		return nil, p.fail(node, "super direct base differs from original checker edge")
	}
	call := node.AsCallExpression()
	actual := p.check.GetTypeAtLocation(node)
	if actual == nil || actual.Flags()&checker.TypeFlagsVoid == 0 {
		return nil, p.fail(node, "official super expression must retain void")
	}
	resolved := p.check.GetResolvedSignature(node)
	if resolved == nil {
		return nil, p.fail(node, "super requires its actual selected construct signature")
	}
	signature, err := p.constructorSignature(base, resolved, p.check.GetTypeAtLocation(call.Expression), node)
	if err != nil {
		return nil, err
	}
	values, err := p.argumentValues(call.Arguments.Nodes, resolved, signature.Parameters, state, depth, nil)
	if err != nil {
		return nil, err
	}
	typ := p.addType(arrayGenericType{ID: "void", Kind: "void"}, actual)
	state.invalidateLengths()
	return &arrayGenericExpression{Kind: "superConstruct", Type: typ, FlowType: typ, Range: *genericSpan(node), Callee: base, CalleeRange: genericSpan(call.Expression), Signature: &signature, Arguments: &values}, nil
}
