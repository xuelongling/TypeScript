// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Class declarations preserve official symbol/type identity and source order.
package main

import (
	"fmt"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

func (p *classProjection) declarations(files []*ast.SourceFile) error {
	// IDs follow sorted original paths and source declaration order. Dependency
	// order is a separate concern and must not change those source identities.
	nodes := []*ast.Node{}
	for _, file := range files {
		for _, node := range file.Statements.Nodes {
			if err := p.ctx.Err(); err != nil {
				return err
			}
			if node.Kind == ast.KindImportDeclaration {
				continue
			}
			if node.Kind == ast.KindImportEqualsDeclaration || node.Kind == ast.KindExportDeclaration {
				return classError("unsupported", "import/re-export form is outside ordinary named value imports", classNodeAnchor(node))
			}
			nodes = append(nodes, node)
		}
	}
	for _, node := range nodes {
		if err := p.ctx.Err(); err != nil {
			return err
		}
		if node.Kind != ast.KindClassDeclaration || node.Name() == nil ||
			node.Name().Kind != ast.KindIdentifier || node.AsClassDeclaration().TypeParameters != nil ||
			ast.HasDecorators(node) || ast.HasSyntacticModifier(node, ast.ModifierFlagsAbstract|ast.ModifierFlagsAmbient|ast.ModifierFlagsDefault) {
			return classError("unsupported", "ordinary named non-generic class declaration required", classNodeAnchor(node))
		}
		resolved := p.check.GetSymbolAtLocation(node.Name())
		if resolved == nil || len(resolved.Declarations) != 1 || resolved.Declarations[0] != node {
			return classError("identity", "class declaration symbol", classNodeAnchor(node))
		}
		instance, constructor := p.check.GetDeclaredTypeOfSymbol(resolved), p.check.GetTypeOfSymbol(resolved)
		if instance == nil || !instance.IsClass() || constructor == nil || instance == constructor {
			return classError("identity", "class instance and constructor types", classNodeAnchor(node))
		}
		id := fmt.Sprintf("c%d", len(p.graph.Classes)+1)
		symbol := p.addSymbol(resolved, id+".symbol", "class", id+".constructor", node.Name())
		p.classes[resolved] = len(p.graph.Classes)
		p.types[instance], p.types[constructor] = id+".instance", id+".constructor"
		p.graph.Types = append(p.graph.Types, classType{ID: id + ".instance", Kind: "class-instance", Declaration: id},
			classType{ID: id + ".constructor", Kind: "class-constructor", Declaration: id}, classType{ID: id + ".this", Kind: "class-this", Declaration: id})
		origin := classNodeAnchor(node)
		// This candidate key identifies allocation provenance, never assignability.
		runtimeKey, err := p.runtimeKey(origin)
		if err != nil {
			return err
		}
		p.graph.Classes = append(p.graph.Classes, classDeclaration{ID: id, Name: node.Name().Text(), Symbol: symbol,
			RuntimeKey: runtimeKey, InstanceType: id + ".instance", ConstructorType: id + ".constructor", ThisType: id + ".this",
			Fields: []classField{}, Methods: []classMethod{}, Origin: origin})
	}
	if len(p.graph.Classes) == 0 {
		return classError("unsupported", "no ordinary class declaration", classAnchor{Source: files[0].FileName()})
	}
	if err := p.importDeclarations(files); err != nil {
		return err
	}
	// All actual class identities and bases precede every member/header. Bodies
	// are last so legal construction-time calls can resolve later declarations.
	for _, node := range nodes {
		index := p.classes[p.check.GetSymbolAtLocation(node.Name())]
		if err := p.baseEdge(index, node); err != nil {
			return err
		}
	}
	for _, node := range nodes {
		index := p.classes[p.check.GetSymbolAtLocation(node.Name())]
		if err := p.fields(index, node); err != nil {
			return err
		}
		if err := p.methodHeaders(index, node); err != nil {
			return err
		}
	}
	// Forwarding uses complete members and the actual base's constructed header.
	if err := p.constructorHeaders(nodes); err != nil {
		return err
	}
	for _, node := range nodes {
		index := p.classes[p.check.GetSymbolAtLocation(node.Name())]
		if err := p.overrideTargets(index, node); err != nil {
			return err
		}
	}
	for _, node := range nodes {
		index := p.classes[p.check.GetSymbolAtLocation(node.Name())]
		if err := p.constructorBody(index, node); err != nil {
			return err
		}
		if err := p.methodBodies(index, node); err != nil {
			return err
		}
	}
	return nil
}

func (p *classProjection) addSymbol(symbol *ast.Symbol, id, kind, typeRef string, node *ast.Node) string {
	p.bindings[symbol] = id
	p.graph.Symbols = append(p.graph.Symbols, classSymbol{ID: id, Name: node.Text(), Kind: kind, Type: typeRef, Declaration: classNodeAnchor(node)})
	return id
}

func (p *classProjection) fields(index int, node *ast.Node) error {
	owner := &p.graph.Classes[index]
	instance := p.check.GetDeclaredTypeOfSymbol(p.check.GetSymbolAtLocation(node.Name()))
	for _, member := range node.Members() {
		if err := p.ctx.Err(); err != nil {
			return err
		}
		if member.Kind == ast.KindConstructor || member.Kind == ast.KindMethodDeclaration {
			continue
		}
		if member.Kind != ast.KindPropertyDeclaration || ast.IsStatic(member) || ast.HasDecorators(member) ||
			ast.HasSyntacticModifier(member, ast.ModifierFlagsAccessor|ast.ModifierFlagsAmbient|ast.ModifierFlagsAbstract) ||
			member.Name() == nil || member.Name().Kind != ast.KindIdentifier {
			return classError("unsupported", "ordinary instance field required", classNodeAnchor(member))
		}
		field := member.AsPropertyDeclaration()
		if field.PostfixToken != nil && field.PostfixToken.Kind == ast.KindQuestionToken {
			return classError("unsupported", "optional class field", classNodeAnchor(member))
		}
		resolved := p.check.GetSymbolAtLocation(member.Name())
		if resolved == nil || len(resolved.Declarations) != 1 || resolved.Declarations[0] != member ||
			p.check.GetPropertyOfType(instance, member.Name().Text()) != resolved {
			return classError("identity", "field symbol differs from official instance member", classNodeAnchor(member))
		}
		typeRef, err := p.typeRef(p.check.GetTypeOfSymbolAtLocation(resolved, member), member)
		if err != nil {
			return err
		}
		ordinal := len(owner.Fields)
		id := fmt.Sprintf("%s.f%d", owner.ID, ordinal)
		flags := checker.GetDeclarationModifierFlagsFromSymbol(resolved)
		access := "public"
		if flags&ast.ModifierFlagsPrivate != 0 {
			access = "private"
		} else if flags&ast.ModifierFlagsProtected != 0 {
			access = "protected"
		}
		symbol := p.addSymbol(resolved, id+".symbol", "field", typeRef, member.Name())
		owner.Fields = append(owner.Fields, classField{ID: id, Symbol: symbol, Name: member.Name().Text(), Ordinal: ordinal,
			Type: typeRef, Access: access, Readonly: flags&ast.ModifierFlagsReadonly != 0,
			DefiniteAssertion: field.PostfixToken != nil && field.PostfixToken.Kind == ast.KindExclamationToken, Origin: classNodeAnchor(member)})
	}
	return nil
}

func (p *classProjection) typeRef(value *checker.Type, node *ast.Node) (string, error) {
	if value != nil {
		if id := p.types[value]; id != "" {
			return id, nil
		}
		if id, scalar, err := p.admittedScalarRef(value, node); scalar || err != nil {
			return id, err
		}
		if value.Flags()&checker.TypeFlagsNumberLiteral != 0 {
			return p.numberLiteralRef(value, node)
		}
		if value.Flags()&checker.TypeFlagsBooleanLike != 0 {
			return "t:boolean", nil
		}
		if value.ObjectFlags()&checker.ObjectFlagsReference != 0 {
			index, known := p.classes[value.Symbol()]
			if known {
				arguments := p.check.GetTypeArguments(value)
				if len(arguments) == 1 && arguments[0].IsTypeParameter() && arguments[0].AsTypeParameter().IsThisType() {
					argumentRef, err := p.typeRef(arguments[0], node)
					if err != nil {
						return "", err
					}
					owner := p.graph.Classes[index]
					id := owner.ID + ".this-view:" + argumentRef
					p.types[value] = id
					found := false
					for _, existing := range p.graph.Types {
						if existing.ID == id {
							found = true
						}
					}
					if !found {
						p.graph.Types = append(p.graph.Types, classType{ID: id, Kind: "class-this-view", Declaration: owner.ID, Target: owner.InstanceType, ThisArgument: argumentRef})
					}
					return id, nil
				}
			}
		}
		if value.IsTypeParameter() && value.AsTypeParameter().IsThisType() {
			if base := p.check.GetBaseConstraintOfType(value); base != nil {
				if symbol := base.Symbol(); symbol != nil {
					if index, ok := p.classes[symbol]; ok {
						return p.graph.Classes[index].ThisType, nil
					}
				}
			}
		}
	}
	return "", classError("unsupported", "type outside current class/admitted scalar/number-literal/void projection", classNodeAnchor(node))
}
