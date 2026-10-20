// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// All official method headers exist before any constructor or method body.
package main

import (
	"fmt"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

type classMethodLocation struct {
	owner int
	index int
}

func (p *classProjection) methodHeaders(index int, node *ast.Node) error {
	owner := &p.graph.Classes[index]
	instance := p.check.GetDeclaredTypeOfSymbol(p.check.GetSymbolAtLocation(node.Name()))
	for _, member := range node.Members() {
		if err := p.ctx.Err(); err != nil {
			return err
		}
		if member.Kind != ast.KindMethodDeclaration {
			continue
		}
		decl := member.AsMethodDeclaration()
		if ast.IsStatic(member) || ast.HasDecorators(member) || member.Name() == nil ||
			member.Name().Kind != ast.KindIdentifier || member.Body() == nil || member.Body().Kind != ast.KindBlock ||
			ast.GetFunctionFlags(member) != ast.FunctionFlagsNormal || decl.TypeParameters != nil || decl.PostfixToken != nil {
			return classError("unsupported", "ordinary non-generic instance method with body required", classNodeAnchor(member))
		}
		resolved := p.check.GetSymbolAtLocation(member.Name())
		if resolved == nil || len(resolved.Declarations) != 1 || resolved.Declarations[0] != member ||
			p.check.GetPropertyOfType(instance, member.Name().Text()) != resolved {
			return classError("identity", "method symbol differs from official instance member", classNodeAnchor(member))
		}
		signature := p.check.GetSignatureFromDeclaration(member)
		methodType := p.check.GetTypeOfSymbolAtLocation(resolved, member)
		signatures := p.check.GetSignaturesOfType(methodType, checker.SignatureKindCall)
		if signature == nil || signature.Declaration() != member || len(signatures) != 1 || signatures[0] != signature {
			return classError("unsupported", "overloaded method signature", classNodeAnchor(member))
		}
		resultType, err := p.typeRef(p.check.GetReturnTypeOfSignature(signature), member)
		if err != nil {
			return err
		}
		id := fmt.Sprintf("%s.m%d", owner.ID, len(owner.Methods))
		functionType := id + ".function"
		p.types[methodType] = functionType
		p.graph.Types = append(p.graph.Types, classType{ID: functionType, Kind: "method-function", Declaration: id})
		symbol := p.addSymbol(resolved, id+".symbol", "method", functionType, member.Name())
		parameters, err := p.requiredParameters(signature, decl.Parameters.Nodes, id, "method")
		if err != nil {
			return err
		}
		p.methodRefs[member] = classMethodLocation{owner: index, index: len(owner.Methods)}
		owner.Methods = append(owner.Methods, classMethod{ID: id, Name: member.Name().Text(), Symbol: symbol, Access: classMemberAccess(resolved),
			Signature: classSignature{Parameters: parameters, Return: resultType}, Body: []classStatement{}, Origin: classNodeAnchor(member)})
	}
	return nil
}

func (p *classProjection) methodBodies(index int, node *ast.Node) error {
	for _, member := range node.Members() {
		if member.Kind != ast.KindMethodDeclaration {
			continue
		}
		signature := p.check.GetSignatureFromDeclaration(member)
		if signature == nil {
			return classError("identity", "official method return signature missing", classNodeAnchor(member))
		}
		body, err := p.methodStatements(member.Body().AsBlock().Statements.Nodes, index, p.check.GetReturnTypeOfSignature(signature))
		if err != nil {
			return err
		}
		location := p.methodRefs[member]
		p.graph.Classes[index].Methods[location.index].Body = body
	}
	return nil
}

// Both real consumers now register required, non-property parameters from an
// official signature. Constructors keep defaults/property initialization separate.
func (p *classProjection) requiredParameters(signature *checker.Signature, nodes []*ast.Node, owner, kind string) ([]classParameter, error) {
	// The official checker stores an explicit this parameter separately from
	// runtime parameters. Its unsupported receiver annotation is not bad identity.
	if len(nodes) > 0 && nodes[0].Name() != nil && nodes[0].Name().Kind == ast.KindIdentifier {
		resolved := p.check.GetSymbolAtLocation(nodes[0].Name())
		if resolved != nil && resolved.Name == ast.InternalSymbolNameThis {
			return nil, classError("unsupported", "explicit this parameter is not projected", classNodeAnchor(nodes[0]))
		}
	}
	if len(nodes) != len(signature.Parameters()) {
		return nil, classError("identity", "official "+kind+" parameter count", classAnchor{})
	}
	parameters := make([]classParameter, 0, len(nodes))
	for ordinal, parameter := range nodes {
		decl := parameter.AsParameterDeclaration()
		if parameter.Name() == nil || parameter.Name().Kind != ast.KindIdentifier || parameter.Modifiers() != nil ||
			decl.Initializer != nil || decl.QuestionToken != nil || decl.DotDotDotToken != nil {
			return nil, classError("unsupported", "required "+kind+" parameter without default or property modifiers", classNodeAnchor(parameter))
		}
		resolved := p.check.GetSymbolAtLocation(parameter.Name())
		if resolved == nil || signature.Parameters()[ordinal] != resolved {
			return nil, classError("identity", "official "+kind+" parameter symbol", classNodeAnchor(parameter))
		}
		typeRef, err := p.typeRef(p.check.GetTypeOfSymbolAtLocation(resolved, parameter), parameter)
		if err != nil {
			return nil, err
		}
		id := fmt.Sprintf("%s.p%d", owner, ordinal)
		p.addSymbol(resolved, id, "parameter", typeRef, parameter.Name())
		parameters = append(parameters, classParameter{Symbol: id, Type: typeRef, Origin: classNodeAnchor(parameter)})
	}
	return parameters, nil
}
