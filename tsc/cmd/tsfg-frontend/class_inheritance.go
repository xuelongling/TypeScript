// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Inheritance edges are resolved official types and symbols, never source names.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

func (p *classProjection) baseEdge(index int, node *ast.Node) error {
	instance := p.check.GetDeclaredTypeOfSymbol(p.check.GetSymbolAtLocation(node.Name()))
	bases := p.check.GetBaseTypes(instance)
	if len(bases) > 1 {
		return classError("unsupported", "only one ordinary class base is projected", classNodeAnchor(node))
	}
	if len(bases) == 0 {
		return nil
	}
	base := bases[0]
	baseIndex, ok := p.classes[base.Symbol()]
	if !ok || !base.IsClass() {
		return classError("unsupported", "base class is outside the owned source closure", classNodeAnchor(node))
	}
	p.graph.Classes[index].Base = p.graph.Classes[baseIndex].ID
	return nil
}

func (p *classProjection) overrideTargets(index int, node *ast.Node) error {
	instance := p.check.GetDeclaredTypeOfSymbol(p.check.GetSymbolAtLocation(node.Name()))
	bases := p.check.GetBaseTypes(instance)
	if len(bases) == 0 {
		return nil
	}
	for _, member := range node.Members() {
		if err := p.ctx.Err(); err != nil {
			return err
		}
		if member.Name() == nil {
			continue
		}
		baseMember := p.check.GetPropertyOfType(bases[0], member.Name().Text())
		if baseMember == nil {
			continue
		}
		if member.Kind != ast.KindMethodDeclaration || baseMember.Flags&ast.SymbolFlagsMethod == 0 {
			return classError("unsupported", "inherited non-method member replacement", classNodeAnchor(member))
		}
		// Full official semantic diagnostics already validated the override's
		// signature, accessibility, and readonly constraints. This copies its edge.
		location, ok := p.methodRefs[baseMember.ValueDeclaration]
		if !ok {
			return classError("identity", "official overridden method is not in the graph", classNodeAnchor(member))
		}
		own, ok := p.methodRefs[member]
		if !ok || own.owner != index {
			return classError("identity", "own method header missing", classNodeAnchor(member))
		}
		p.graph.Classes[index].Methods[own.index].OverrideTarget = p.graph.Classes[location.owner].Methods[location.index].ID
	}
	return nil
}

func classMemberAccess(symbol *ast.Symbol) string {
	flags := checker.GetDeclarationModifierFlagsFromSymbol(symbol)
	if flags&ast.ModifierFlagsPrivate != 0 {
		return "private"
	}
	if flags&ast.ModifierFlagsProtected != 0 {
		return "protected"
	}
	return "public"
}

// Constructor dependencies follow official base edges, not source IDs. A bounded
// iterative walk also handles a derived source sorting before its base source.
func (p *classProjection) constructorHeaders(nodes []*ast.Node) error {
	indices := map[string]int{}
	for index, declaration := range p.graph.Classes {
		indices[declaration.ID] = index
	}
	states := make([]uint8, len(nodes))
	for start := range nodes {
		chain := []int{}
		index := start
		for states[index] != 2 {
			if err := p.ctx.Err(); err != nil {
				return err
			}
			if states[index] == 1 {
				return classError("identity", "official base dependency is cyclic", classNodeAnchor(nodes[index]))
			}
			states[index] = 1
			chain = append(chain, index)
			base := p.graph.Classes[index].Base
			if base == "" {
				break
			}
			next, known := indices[base]
			if !known {
				return classError("identity", "official base header is outside the owned closure", classNodeAnchor(nodes[index]))
			}
			index = next
		}
		for offset := len(chain) - 1; offset >= 0; offset-- {
			index := chain[offset]
			if err := p.ctx.Err(); err != nil {
				return err
			}
			if err := p.constructorHeader(index, nodes[index]); err != nil {
				return err
			}
			states[index] = 2
		}
	}
	return nil
}
