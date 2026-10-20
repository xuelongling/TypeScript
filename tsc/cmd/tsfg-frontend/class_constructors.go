// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Constructor signatures precede bodies; initialization has an explicit boundary.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

func (p *classProjection) constructorHeader(index int, node *ast.Node) error {
	owner := &p.graph.Classes[index]
	var explicit *ast.Node
	for _, member := range node.Members() {
		if member.Kind != ast.KindConstructor {
			continue
		}
		if explicit != nil || member.Body() == nil || member.Body().Kind != ast.KindBlock {
			return classError("unsupported", "constructor overload or absent body", classNodeAnchor(member))
		}
		// Nonpublic construction/access semantics remain outside this candidate.
		if ast.HasSyntacticModifier(member, ast.ModifierFlagsPrivate|ast.ModifierFlagsProtected) {
			return classError("unsupported", "nonpublic constructor visibility is not yet represented", classNodeAnchor(member))
		}
		explicit = member
	}
	static := p.check.GetTypeOfSymbol(p.check.GetSymbolAtLocation(node.Name()))
	signatures := p.check.GetSignaturesOfType(static, checker.SignatureKindConstruct)
	if len(signatures) != 1 || p.types[p.check.GetReturnTypeOfSignature(signatures[0])] != owner.InstanceType {
		return classError("identity", "official constructor signature", classNodeAnchor(node))
	}
	owned := classConstructor{ID: owner.ID + ".ctor", Kind: "implicit-base",
		Signature: classSignature{Parameters: []classParameter{}, Return: owner.InstanceType}, Body: []classStatement{},
		InitializationPhase: "before-body", InitializationFields: []string{}, Origin: classNodeAnchor(node)}
	if owner.Base != "" {
		owned.InitializationPhase = "after-successful-super"
	}
	for _, field := range owner.Fields {
		owned.InitializationFields = append(owned.InitializationFields, field.ID)
	}
	if explicit != nil {
		// Official modifier flags above exclude nonpublic access; both an explicit
		// public keyword and omitted accessibility are the official public default.
		owned.Kind, owned.Access, owned.Origin = "explicit", "public", classNodeAnchor(explicit)
		parameters, err := p.requiredParameters(signatures[0], explicit.AsConstructorDeclaration().Parameters.Nodes, owned.ID, "constructor")
		if err != nil {
			return err
		}
		owned.Signature.Parameters = parameters
	} else if owner.Base != "" {
		if err := p.implicitConstructor(index, node, signatures[0], &owned); err != nil {
			return err
		}
	} else if len(signatures[0].Parameters()) != 0 {
		return classError("unsupported", "implicit base constructor has parameters", owned.Origin)
	} else {
		// No own constructor and no official base produce the official default
		// root construct signature. There is no inherited nonpublic declaration.
		owned.Access = "public"
	}
	p.constructorNodes[index] = explicit
	owner.Constructor = owned
	return nil
}

func (p *classProjection) constructorBody(index int, node *ast.Node) error {
	owner := &p.graph.Classes[index]
	ordinal := 0
	for _, member := range node.Members() {
		if member.Kind != ast.KindPropertyDeclaration {
			continue
		}
		if initializer := member.AsPropertyDeclaration().Initializer; initializer != nil {
			value, err := p.expression(initializer, index)
			if err != nil {
				return err
			}
			resolved := p.check.GetSymbolAtLocation(member.Name())
			target := p.check.GetTypeOfSymbolAtLocation(resolved, member)
			if err := p.literalContext(value, target, initializer); err != nil {
				return err
			}
			witness, err := p.compatibility(p.check.GetTypeAtLocation(initializer), target, initializer)
			if err != nil {
				return err
			}
			owner.Fields[ordinal].Initializer = value
			owner.Fields[ordinal].InitializerCompatibility = &witness
		}
		ordinal++
	}
	if explicit := p.constructorNodes[index]; explicit != nil {
		static := p.check.GetTypeOfSymbol(p.check.GetSymbolAtLocation(node.Name()))
		signatures := p.check.GetSignaturesOfType(static, checker.SignatureKindConstruct)
		if len(signatures) != 1 {
			return classError("identity", "official constructor return signature missing", classNodeAnchor(explicit))
		}
		body, err := p.statements(explicit.Body().AsBlock().Statements.Nodes, index, p.check.GetReturnTypeOfSignature(signatures[0]))
		if err != nil {
			return err
		}
		owner.Constructor.Body = body
	}
	supers := []string{}
	var visit func(*classExpression)
	visit = func(value *classExpression) {
		if value == nil {
			return
		}
		if value.Kind == "super-construct" {
			supers = append(supers, value.ID)
		}
		visit(value.Left)
		visit(value.Right)
		visit(value.Receiver)
		for _, argument := range value.Arguments {
			visit(argument)
		}
	}
	for _, statement := range owner.Constructor.Body {
		visit(statement.Expression)
	}
	if owner.Constructor.Kind == "implicit-derived" {
		if len(supers) != 0 || owner.Constructor.ImplicitForward == nil {
			return classError("identity", "implicit forwarding must not fabricate a source super call", owner.Constructor.Origin)
		}
	} else if owner.Base != "" {
		if len(supers) != 1 {
			return classError("unsupported", "derived constructor requires one explicit super initialization boundary", owner.Constructor.Origin)
		}
		owner.Constructor.SuperCall = supers[0]
	} else if len(supers) != 0 {
		return classError("identity", "base constructor unexpectedly resolves super", owner.Constructor.Origin)
	}
	return nil
}
