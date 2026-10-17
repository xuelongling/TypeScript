// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

func (a *aggregateProjection) parameter(file *ast.SourceFile, check *checker.Checker, node *ast.Node) (string, *parameterPresence, bool) {
	parameter := node.AsParameterDeclaration()
	if parameter.Type != nil && parameter.DotDotDotToken == nil && node.Name() != nil && node.Name().Kind == ast.KindIdentifier && !ast.IsParameterPropertyDeclaration(node, node.Parent) {
		if t := a.typeName(check.GetTypeFromTypeNode(parameter.Type)); t != "" {
			// Aggregate parameters are required and present in this profile. Keep
			// their official declaration ranges just like scalar parameters;
			// optional/default aggregate payloads remain outside the profile.
			if parameter.QuestionToken != nil || parameter.Initializer != nil {
				return "", nil, false
			}
			return t, &parameterPresence{
				Undefined: false, Null: false, Optional: false, Default: false,
				Range: offset(file, node), Annotation: expressionOffset(file, parameter.Type),
			}, true
		}
	}
	return checkedParameter(file, check, node)
}

// Mixed aggregate/scalar signatures need their scalar argument contexts before
// official diagnostics. Keep the same complete source declaration and checked
// signature identity requirements as the ordinary scalar context registry.
func (a *aggregateProjection) functionContexts(file *ast.SourceFile, check *checker.Checker) map[*ast.Symbol]functionInfo {
	functions := map[*ast.Symbol]functionInfo{}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Name() == nil || node.Name().Kind != ast.KindIdentifier || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			continue
		}
		decl := node.AsFunctionDeclaration()
		if decl.Type == nil || decl.TypeParameters != nil {
			continue
		}
		resolved := check.GetSymbolAtLocation(node.Name())
		if resolved == nil {
			continue
		}
		signatures := check.GetSignaturesOfType(check.GetTypeOfSymbol(resolved), checker.SignatureKindCall)
		if len(signatures) != 1 || signatures[0].Declaration() != node {
			continue
		}
		checked := signatures[0]
		returnType := a.typeName(check.GetReturnTypeOfSignature(checked))
		hasAggregate := returnType != ""
		if returnType == "" {
			returnType = scalar(check.GetReturnTypeOfSignature(checked))
		}
		if returnType == "" || len(checked.Parameters()) != len(decl.Parameters.Nodes) {
			continue
		}
		parameters := make([]string, 0, len(checked.Parameters()))
		presence := make([]*parameterPresence, 0, len(checked.Parameters()))
		hasPresence := false
		for index, node := range decl.Parameters.Nodes {
			t, shape, valid := a.parameter(file, check, node)
			checkedType := check.GetTypeOfSymbol(checked.Parameters()[index])
			ownedType := a.typeName(checkedType)
			if ownedType != "" {
				hasAggregate = true
			} else {
				var validType bool
				ownedType, _, _, validType = finitePresence(checkedType)
				valid = valid && validType
			}
			if !valid || ownedType != t || check.GetSymbolAtLocation(node.Name()) != checked.Parameters()[index] {
				break
			}
			parameters = append(parameters, t)
			presence = append(presence, shape)
			hasPresence = hasPresence || shape != nil
		}
		if hasAggregate && len(parameters) == len(decl.Parameters.Nodes) {
			if !hasPresence {
				presence = nil
			}
			functions[resolved] = functionInfo{Declaration: node, Signature: signature{Parameters: parameters, ReturnType: returnType, Presence: presence}}
		}
	}
	return functions
}

func (a *aggregateProjection) constructorHeaders(file *ast.SourceFile, check *checker.Checker, result *response) *response {
	for _, info := range a.ordered() {
		decl := &a.facts.Declarations[info.index]
		ctor := aggregateConstructor{ID: fmt.Sprintf("c%d", info.index+1), Kind: "implicitDefault", ClassRef: decl.ID, Parameters: []symbol{}, Signature: signature{Parameters: []string{}, ReturnType: decl.ID}, ResultType: decl.ID, Body: []statement{}, ParameterInitializers: []*expression{}, Range: decl.Range}
		implementation := ast.GetFirstConstructorWithBody(info.node)
		if implementation == nil {
			if info.checked.Declaration() != nil || len(info.checked.Parameters()) != 0 {
				failure := failAt("r01-d-aggregate-identity", "error", file, info.node)
				return &failure
			}
		} else {
			if info.checked.Declaration() != implementation || implementation.Body().Kind != ast.KindBlock {
				failure := failAt("r02b-d-unsupported", "unsupported", file, implementation)
				return &failure
			}
			ctor.Kind = "explicit"
			location := expressionOffset(file, implementation)
			body := expressionOffset(file, implementation.Body())
			ctor.SignatureDeclaration = &location
			ctor.Implementation = &location
			ctor.BodyRange = &body
			parameters := implementation.AsConstructorDeclaration().Parameters.Nodes
			if len(parameters) != len(info.checked.Parameters()) {
				failure := failAt("r01-d-aggregate-identity", "error", file, implementation)
				return &failure
			}
			hasPresence := false
			for index, node := range parameters {
				t, presence, valid := a.parameter(file, check, node)
				if !valid || check.GetSymbolAtLocation(node.Name()) != info.checked.Parameters()[index] {
					failure := failAt("r02b-d-unsupported", "unsupported", file, node)
					return &failure
				}
				s := symbol{ID: fmt.Sprintf("s%d", len(result.Symbols)+1), Name: node.Name().Text(), Kind: "parameter", Type: t, Writable: true, Presence: presence, Range: offset(file, node.Name())}
				result.Symbols = append(result.Symbols, s)
				ctor.Parameters = append(ctor.Parameters, s)
				ctor.Signature.Parameters = append(ctor.Signature.Parameters, t)
				ctor.Signature.Presence = append(ctor.Signature.Presence, presence)
				ctor.ParameterInitializers = append(ctor.ParameterInitializers, nil)
				hasPresence = hasPresence || presence != nil
			}
			if !hasPresence {
				ctor.Signature.Presence = nil
			}
		}
		decl.Constructors = append(decl.Constructors, ctor)
	}
	return nil
}

func (a *aggregateProjection) constructorBodies(file *ast.SourceFile, check *checker.Checker, result *response, functions map[*ast.Symbol]functionInfo, target *integerTarget) *response {
	for _, info := range a.ordered() {
		decl := &a.facts.Declarations[info.index]
		ctor := &decl.Constructors[0]
		project := projection{file: file, check: check, result: result, bindings: map[*ast.Symbol]symbol{}, functions: functions, integerTarget: target, aggregates: a, self: info}
		implementation := ast.GetFirstConstructorWithBody(info.node)
		if implementation != nil {
			for index, node := range implementation.AsConstructorDeclaration().Parameters.Nodes {
				project.bindings[check.GetSymbolAtLocation(node.Name())] = ctor.Parameters[index]
			}
			for index, node := range implementation.AsConstructorDeclaration().Parameters.Nodes {
				if initializer := node.AsParameterDeclaration().Initializer; initializer != nil {
					value, failure := project.expr(initializer, ctor.Parameters[index].Type, checkedInitializerPresence(check, node))
					if failure.Status != "" {
						return &failure
					}
					ctor.ParameterInitializers[index] = value
				}
			}
		}
		for _, member := range info.node.Members() {
			if member.Kind != ast.KindPropertyDeclaration || member.AsPropertyDeclaration().Initializer == nil {
				continue
			}
			ordinal := info.fields[check.GetSymbolAtLocation(member.Name())]
			value, failure := project.expr(member.AsPropertyDeclaration().Initializer, decl.Fields[ordinal].Type)
			if failure.Status != "" {
				return &failure
			}
			decl.Fields[ordinal].Initializer = value
		}
		if implementation != nil {
			body, failure := project.statements(implementation.Body().AsBlock().Statements.Nodes, decl.ID)
			if failure.Status != "" {
				return &failure
			}
			ctor.Body = body
		}
	}
	return nil
}
