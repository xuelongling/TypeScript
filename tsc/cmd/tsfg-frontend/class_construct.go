// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Construction records allocation and selected signature without running a ctor.
package main

import "github.com/microsoft/typescript-go/internal/ast"

func (p *classProjection) construct(node *ast.Node, owner int) (*classExpression, error) {
	construct := node.AsNewExpression()
	if construct.TypeArguments != nil || construct.Expression.Kind != ast.KindIdentifier {
		return nil, classError("unsupported", "new requires a direct non-generic owned class constructor value", classNodeAnchor(node))
	}
	resolved := p.valueSymbol(construct.Expression)
	index, ok := p.classes[resolved]
	if !ok {
		return nil, classError("unsupported", "constructor value is outside the owned direct class closure", classNodeAnchor(node))
	}
	declaration := p.graph.Classes[index]
	static := p.check.GetTypeAtLocation(construct.Expression)
	signature := p.check.GetResolvedSignature(node)
	selected, err := p.constructorSignature(signature, static, declaration, node)
	if err != nil {
		return nil, err
	}
	typeRef, err := p.typeRef(p.check.GetTypeAtLocation(node), node)
	if err != nil {
		return nil, err
	}
	if typeRef != declaration.InstanceType {
		return nil, classError("identity", "official new result differs from its owned allocation class", classNodeAnchor(node))
	}
	receiver, err := p.expression(construct.Expression, owner)
	if err != nil {
		return nil, err
	}
	out := &classExpression{ID: p.expressionID(), Kind: "construct", Dispatch: "direct-constructor",
		StaticTarget: declaration.Constructor.ID, Type: typeRef, Receiver: receiver, ResolvedSignature: &selected,
		Arguments: []*classExpression{}, ArgumentCompatibility: []classCompatibility{}, Origin: classNodeAnchor(node)}
	arguments := []*ast.Node{}
	if construct.Arguments != nil {
		arguments = construct.Arguments.Nodes
	}
	if err := p.callArguments(out, signature, arguments, node, owner); err != nil {
		return nil, err
	}
	return out, nil
}
