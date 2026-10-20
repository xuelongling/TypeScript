// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Ordinary reference compatibility is an official relation, not runtime identity.
package main

import (
	"slices"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

func (p *classProjection) compatibility(source, target *checker.Type, node *ast.Node) (classCompatibility, error) {
	sourceRef, err := p.typeRef(source, node)
	if err != nil {
		return classCompatibility{}, err
	}
	targetRef, err := p.typeRef(target, node)
	if err != nil {
		return classCompatibility{}, err
	}
	witness := classCompatibility{SourceType: sourceRef, TargetType: targetRef,
		Assignable: p.check.IsTypeAssignableTo(source, target), Members: []classMemberProjection{}, Origin: classNodeAnchor(node)}
	// R01 scalar typing adds no implicit conversion between already typed
	// scalar values. An unresolved literal has no settled scalar and is distinct.
	if from, to := p.scalarKinds[sourceRef], p.scalarKinds[targetRef]; from != "" && to != "" && from != to {
		return classCompatibility{}, classError("scalar", "r01-d-scalar-type: typed scalar values do not implicitly convert", witness.Origin)
	}
	if !witness.Assignable {
		return classCompatibility{}, classError("identity", "official compatibility differs from checked Program", witness.Origin)
	}
	if target.Flags()&checker.TypeFlagsObject == 0 && !(target.IsTypeParameter() && target.AsTypeParameter().IsThisType()) {
		return witness, nil
	}
	members := p.check.GetPropertiesOfType(target)
	if err := p.ctx.Err(); err != nil {
		return classCompatibility{}, err
	}
	// Input visits and additional owned relation edges share the explicit
	// projection budget. Reserve the complete witness before copying any edge.
	if err := p.resources.memberEdges(len(members), witness.Origin); err != nil {
		return classCompatibility{}, err
	}
	for _, member := range members {
		if err := p.ctx.Err(); err != nil {
			return classCompatibility{}, err
		}
		actual := p.check.GetPropertyOfType(source, member.Name)
		targetSymbol, sourceSymbol := p.symbolRef(member), p.symbolRef(actual)
		if actual == nil || targetSymbol == "" || sourceSymbol == "" {
			return classCompatibility{}, classError("unsupported", "official member projection lies outside the owned member closure", witness.Origin)
		}
		witness.Members = append(witness.Members, classMemberProjection{Target: targetSymbol, Source: sourceSymbol})
	}
	slices.SortFunc(witness.Members, func(a, b classMemberProjection) int { return strings.Compare(a.Target, b.Target) })
	return witness, nil
}

// Official instantiation may wrap a declaration's symbol for a polymorphic this
// receiver. Follow official root symbols; a spelling never establishes identity.
func (p *classProjection) symbolRef(symbol *ast.Symbol) string {
	if symbol == nil {
		return ""
	}
	if id := p.bindings[symbol]; id != "" {
		return id
	}
	roots := p.check.GetRootSymbols(symbol)
	if len(roots) == 1 {
		return p.bindings[roots[0]]
	}
	return ""
}
