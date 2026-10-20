// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Flat method locals own declarations and values without copying a scope checker.
package main

import (
	"fmt"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

type classLocalStatement struct {
	Kind     string              `json:"kind"`
	Bindings []classLocalBinding `json:"bindings"`
}

type classLocalBinding struct {
	Symbol        string             `json:"symbol"`
	Type          string             `json:"type"`
	Initializer   *classExpression   `json:"initializer"`
	Compatibility classCompatibility `json:"compatibility"`
	Origin        classAnchor        `json:"origin"`
}

// These pointers remain inside this projection and never enter the owned graph.
// The declared slot is independent of official flow types at later occurrences.
type classLocalSlot struct {
	Method *ast.Node
	Kind   string
	Type   *checker.Type
}

func (p *classProjection) localStatement(node *ast.Node, owner int) (*classLocalStatement, error) {
	method := ast.GetContainingFunction(node)
	location, known := p.methodRefs[method]
	if !known || location.owner != owner || node.Parent != method.Body() {
		return nil, classError("unsupported", "local declarations require a flat owned ordinary method body", classNodeAnchor(node))
	}
	statement := node.AsVariableStatement()
	if node.Modifiers() != nil || statement.DeclarationList == nil {
		return nil, classError("unsupported", "ordinary initialized const/let statement required", classNodeAnchor(node))
	}
	kind := ""
	if ast.IsVarConst(statement.DeclarationList) {
		kind = "const"
	} else if ast.IsVarLet(statement.DeclarationList) {
		kind = "let"
	} else {
		return nil, classError("unsupported", "only initialized const/let local declarations are projected", classNodeAnchor(node))
	}
	declarations := statement.DeclarationList.AsVariableDeclarationList().Declarations.Nodes
	if len(declarations) == 0 {
		return nil, classError("identity", "official local declaration list is empty", classNodeAnchor(node))
	}
	local := &classLocalStatement{Kind: kind, Bindings: []classLocalBinding{}}
	for _, declaration := range declarations {
		if err := p.ctx.Err(); err != nil {
			return nil, err
		}
		if declaration.Kind != ast.KindVariableDeclaration || declaration.Name() == nil || declaration.Name().Kind != ast.KindIdentifier {
			return nil, classError("unsupported", "local declaration requires a simple identifier binding", classNodeAnchor(declaration))
		}
		decl := declaration.AsVariableDeclaration()
		if decl.Initializer == nil || decl.ExclamationToken != nil {
			return nil, classError("unsupported", "local binding requires an explicit initializer", classNodeAnchor(declaration))
		}
		symbol := p.check.GetSymbolAtLocation(declaration.Name())
		if declaration.Parent != statement.DeclarationList || ast.GetContainingFunction(declaration) != method ||
			symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != declaration || p.bindings[symbol] != "" {
			return nil, classError("identity", "official local symbol/declaration ownership", classNodeAnchor(declaration))
		}
		// Passing the declaration, not a reference occurrence, asks the official
		// checker for the declared slot without location-specific flow narrowing.
		slot := p.check.GetTypeOfSymbolAtLocation(symbol, declaration)
		typeRef, err := p.typeRef(slot, declaration)
		if err != nil {
			return nil, err
		}
		initializer, err := p.expression(decl.Initializer, owner)
		if err != nil {
			return nil, err
		}
		if err := p.literalContext(initializer, slot, decl.Initializer); err != nil {
			return nil, err
		}
		witness, err := p.compatibility(p.check.GetTypeAtLocation(decl.Initializer), slot, decl.Initializer)
		if err != nil {
			return nil, err
		}
		// Register only after this binding's initializer and witness succeed.
		// The next initializer can then resolve this actual symbol in order.
		methodID := p.graph.Classes[location.owner].Methods[location.index].ID
		id := fmt.Sprintf("%s.l%d", methodID, p.localOrdinals[method])
		p.localOrdinals[method]++
		p.addSymbol(symbol, id, "local", typeRef, declaration.Name())
		p.localSlots[symbol] = classLocalSlot{Method: method, Kind: kind, Type: slot}
		local.Bindings = append(local.Bindings, classLocalBinding{Symbol: id, Type: typeRef,
			Initializer: initializer, Compatibility: witness, Origin: classNodeAnchor(declaration)})
	}
	return local, nil
}

// A local's declaration does not replace the official type of a read. Until
// explicit owned flow-view facts exist, differing official flow types are a
// precise unsupported capability rather than a fabricated declared-type read.
func (p *classProjection) localUse(node *ast.Node) error {
	symbol := p.check.GetSymbolAtLocation(node)
	slot, known := p.localSlots[symbol]
	if !known {
		return nil
	}
	if ast.GetContainingFunction(node) != slot.Method {
		return classError("unsupported", "local reference leaves its owned flat method scope", classNodeAnchor(node))
	}
	if p.check.GetTypeAtLocation(node) != slot.Type {
		return classError("unsupported", "local reference requires an owned official flow-type view", classNodeAnchor(node))
	}
	return nil
}

func (p *classProjection) localAssignment(node *ast.Node, owner int) (*classExpression, error) {
	binary := node.AsBinaryExpression()
	symbol := p.check.GetSymbolAtLocation(binary.Left)
	slot, known := p.localSlots[symbol]
	if !known {
		return nil, classError("unsupported", "identifier assignment requires an owned let local binding", classNodeAnchor(binary.Left))
	}
	location, methodKnown := p.methodRefs[slot.Method]
	if !methodKnown || location.owner != owner || ast.GetContainingFunction(node) != slot.Method {
		return nil, classError("unsupported", "local assignment leaves its owned flat method scope", classNodeAnchor(node))
	}
	if slot.Kind != "let" {
		return nil, classError("identity", "official checked Program assigns to a const local", classNodeAnchor(binary.Left))
	}
	// The assignment's value type comes from the official expression. Its
	// compatibility target remains the original declaration slot, not that type.
	result, err := p.typeRef(p.check.GetTypeAtLocation(node), node)
	if err != nil {
		return nil, err
	}
	out := &classExpression{ID: p.expressionID(), Kind: "local-assignment", Operator: "=", Type: result, Origin: classNodeAnchor(node)}
	out.Left, err = p.expression(binary.Left, owner)
	if err != nil {
		return nil, err
	}
	out.Right, err = p.expression(binary.Right, owner)
	if err != nil {
		return nil, err
	}
	if err := p.literalContext(out.Right, slot.Type, binary.Right); err != nil {
		return nil, err
	}
	witness, err := p.compatibility(p.check.GetTypeAtLocation(binary.Right), slot.Type, binary.Right)
	if err != nil {
		return nil, err
	}
	out.Compatibility = &witness
	return out, nil
}
