// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Method declarations and direct base constructor values keep distinct targets.
package main

import (
	"fmt"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

func (p *classProjection) expressionID() string {
	p.expressions++
	return fmt.Sprintf("e%d", p.expressions)
}

func (p *classProjection) call(node *ast.Node, owner int) (*classExpression, error) {
	call := node.AsCallExpression()
	if call.QuestionDotToken != nil || call.TypeArguments != nil {
		return nil, classError("unsupported", "optional or generic class invocation", classNodeAnchor(node))
	}
	signature := p.check.GetResolvedSignature(node)
	if signature == nil {
		return nil, classError("identity", "official resolved call signature missing", classNodeAnchor(node))
	}
	typeRef, err := p.typeRef(p.check.GetTypeAtLocation(node), node)
	if err != nil {
		return nil, err
	}
	out := &classExpression{ID: p.expressionID(), Type: typeRef, Origin: classNodeAnchor(node),
		Arguments: []*classExpression{}, ArgumentCompatibility: []classCompatibility{}}
	var selected classSignature
	if call.Expression.Kind == ast.KindSuperKeyword {
		base := p.graph.Classes[owner].Base
		var target *classDeclaration
		for index := range p.graph.Classes {
			if p.graph.Classes[index].ID == base {
				target = &p.graph.Classes[index]
				break
			}
		}
		if target == nil {
			return nil, classError("identity", "super direct base is outside the owned class closure", out.Origin)
		}
		selected, err = p.constructorSignature(signature, p.check.GetTypeAtLocation(call.Expression), *target, node)
		if err != nil {
			return nil, err
		}
		out.Kind, out.Dispatch, out.StaticTarget = "super-construct", "direct-base-constructor", target.Constructor.ID
		out.Receiver, err = p.expression(call.Expression, owner)
		if err != nil {
			return nil, err
		}
	} else {
		if call.Expression.Kind != ast.KindPropertyAccessExpression || call.Expression.AsPropertyAccessExpression().QuestionDotToken != nil {
			return nil, classError("unsupported", "only resolved instance member calls are currently projected", out.Origin)
		}
		if signature.Declaration() == nil {
			return nil, classError("identity", "official resolved method declaration missing", out.Origin)
		}
		property := call.Expression
		facts, ok := p.check.TSFGClassPropertyAccess(property)
		location, exists := p.methodRefs[signature.Declaration()]
		if !ok || !exists || !facts.Readable {
			return nil, classError("identity", "official call member or declaration is outside the graph", out.Origin)
		}
		target := p.graph.Classes[location.owner].Methods[location.index]
		if p.symbolRef(facts.Property) != target.Symbol {
			return nil, classError("identity", "official resolved member differs from call declaration", out.Origin)
		}
		out.Kind, out.Dispatch, out.StaticTarget, out.Symbol = "method-call", "virtual", target.ID, target.Symbol
		if property.Expression().Kind == ast.KindSuperKeyword {
			out.Dispatch = "direct-base-method"
		}
		out.Permission = &classPermission{Readable: facts.Readable, Writable: facts.Writable, Readonly: facts.Readonly, WriteUse: facts.IsWrite}
		out.Receiver, err = p.expression(property.Expression(), owner)
		if err != nil {
			return nil, err
		}
		selected, err = p.callSignature(signature, node)
		if err != nil {
			return nil, err
		}
	}
	out.ResolvedSignature = &selected
	if err := p.callArguments(out, signature, call.Arguments.Nodes, node, owner); err != nil {
		return nil, err
	}
	return out, nil
}

// Method/super calls and construction share the official argument relation.
func (p *classProjection) callArguments(out *classExpression, signature *checker.Signature, nodes []*ast.Node, node *ast.Node, owner int) error {
	if len(nodes) != len(signature.Parameters()) || signature.HasRestParameter() {
		return classError("unsupported", "call requires the supported required-parameter arity", out.Origin)
	}
	for index, argument := range nodes {
		value, err := p.expression(argument, owner)
		if err != nil {
			return err
		}
		parameterType := p.check.GetTypeOfSymbolAtLocation(signature.Parameters()[index], node)
		if err := p.literalContext(value, parameterType, argument); err != nil {
			return err
		}
		witness, err := p.compatibility(p.check.GetTypeAtLocation(argument), parameterType, argument)
		if err != nil {
			return err
		}
		out.Arguments = append(out.Arguments, value)
		out.ArgumentCompatibility = append(out.ArgumentCompatibility, witness)
	}
	return nil
}

func (p *classProjection) callSignature(signature *checker.Signature, node *ast.Node) (classSignature, error) {
	result, err := p.typeRef(p.check.GetReturnTypeOfSignature(signature), node)
	if err != nil {
		return classSignature{}, err
	}
	owned := classSignature{Parameters: []classParameter{}, Return: result}
	for _, parameter := range signature.Parameters() {
		if len(parameter.Declarations) != 1 || parameter.Declarations[0].Name() == nil {
			return classSignature{}, classError("unsupported", "resolved parameter declaration is not a source parameter", classNodeAnchor(node))
		}
		resolved := parameter
		if parameter.Declarations[0].Name() != nil {
			resolved = p.check.GetSymbolAtLocation(parameter.Declarations[0].Name())
		}
		id := p.bindings[resolved]
		if id == "" {
			return classSignature{}, classError("identity", "resolved call parameter is not in the owned closure", classNodeAnchor(node))
		}
		typeRef, err := p.typeRef(p.check.GetTypeOfSymbolAtLocation(parameter, node), node)
		if err != nil {
			return classSignature{}, err
		}
		origin := classNodeAnchor(parameter.Declarations[0])
		owned.Parameters = append(owned.Parameters, classParameter{Symbol: id, Type: typeRef, Origin: origin})
	}
	return owned, nil
}
