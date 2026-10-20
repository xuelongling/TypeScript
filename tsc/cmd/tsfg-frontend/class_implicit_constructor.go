// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// An implicit derived constructor owns logical forwarding, never a synthetic AST.
package main

import (
	"fmt"
	"slices"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

func (p *classProjection) implicitConstructor(index int, node *ast.Node, signature *checker.Signature, owned *classConstructor) error {
	instance := p.check.GetDeclaredTypeOfSymbol(p.check.GetSymbolAtLocation(node.Name()))
	bases := p.check.GetBaseTypes(instance)
	if len(bases) != 1 {
		return classError("identity", "implicit constructor requires its official direct base", classNodeAnchor(node))
	}
	baseIndex, ok := p.classes[bases[0].Symbol()]
	if !ok || p.graph.Classes[baseIndex].ID != p.graph.Classes[index].Base {
		return classError("identity", "implicit constructor direct base is outside its owned edge", classNodeAnchor(node))
	}
	base := p.graph.Classes[baseIndex]
	baseSignatures := p.check.GetSignaturesOfType(p.check.GetTypeOfSymbol(bases[0].Symbol()), checker.SignatureKindConstruct)
	if base.Constructor.ID == "" || len(baseSignatures) != 1 {
		return classError("identity", "implicit constructor base header/signature missing", classNodeAnchor(node))
	}
	// Official default signatures may retain an ancestor declaration. Absence
	// of an own constructor cannot establish accessibility by itself. Headers
	// are built base-first, so this exact fact validates the entire base chain.
	if base.Constructor.Access != "public" {
		return classError("identity", "implicit constructor visibility requires an explicit public base header", classNodeAnchor(node))
	}
	baseSignature := baseSignatures[0]
	parameters, baseParameters := signature.Parameters(), baseSignature.Parameters()
	if signature.HasRestParameter() || len(signature.TypeParameters()) != 0 ||
		signature.MinArgumentCount() != len(parameters) || len(parameters) != len(baseParameters) ||
		len(parameters) != len(base.Constructor.Signature.Parameters) {
		return classError("unsupported", "implicit forwarding requires the official required non-generic base parameters", classNodeAnchor(node))
	}
	forward := classImplicitForward{
		Target:        base.Constructor.ID,
		BaseSignature: classSignature{Parameters: slices.Clone(base.Constructor.Signature.Parameters), Return: base.Constructor.Signature.Return},
		Parameters:    []classImplicitParameter{}, Origin: classNodeAnchor(node),
	}
	for ordinal, parameter := range parameters {
		if err := p.ctx.Err(); err != nil {
			return err
		}
		// Non-generic inherited signatures can share the actual Base symbol.
		// Register an owned logical parameter without rebinding that symbol.
		declaration := parameter.ValueDeclaration
		if parameter != baseParameters[ordinal] || declaration == nil ||
			declaration.Kind != ast.KindParameter || declaration.Name() == nil ||
			declaration.Name().Kind != ast.KindIdentifier ||
			p.check.GetSymbolAtLocation(declaration.Name()) != parameter {
			return classError("identity", "implicit parameter differs from the official inherited base parameter", classNodeAnchor(node))
		}
		inherited := base.Constructor.Signature.Parameters[ordinal]
		sourceType := p.check.GetTypeOfSymbolAtLocation(parameter, declaration)
		targetType := p.check.GetTypeOfSymbolAtLocation(baseParameters[ordinal], declaration)
		typeRef, err := p.typeRef(sourceType, declaration)
		if err != nil {
			return err
		}
		witness, err := p.compatibility(sourceType, targetType, node)
		if err != nil {
			return err
		}
		if inherited.Type != witness.TargetType {
			return classError("identity", "implicit base parameter differs from its owned signature", classNodeAnchor(node))
		}
		id := fmt.Sprintf("%s.p%d", owned.ID, ordinal)
		p.graph.Symbols = append(p.graph.Symbols, classSymbol{ID: id, Name: parameter.Name, Kind: "implicit-parameter",
			Type: typeRef, Declaration: classNodeAnchor(declaration.Name())})
		owned.Signature.Parameters = append(owned.Signature.Parameters, classParameter{Symbol: id, Type: typeRef,
			InheritedFrom: inherited.Symbol, Origin: classNodeAnchor(declaration)})
		forward.Parameters = append(forward.Parameters, classImplicitParameter{Source: id, Target: inherited.Symbol, Compatibility: witness})
	}
	owned.Kind, owned.Access, owned.ImplicitForward = "implicit-derived", base.Constructor.Access, &forward
	return nil
}
