// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// This internal candidate imports canonical packages without source-file intake.
package main

import "context"

type genericPackageRoot struct {
	Definition    string                  `json:"definition"`
	TypeArguments []genericPackageTypeDTO `json:"typeArguments"`
}

type genericPackageFacts struct {
	PackageID       string                       `json:"packageId"`
	SemanticProfile string                       `json:"semanticProfile"`
	Roots           []string                     `json:"roots"`
	Instances       []genericPackageInstanceFact `json:"instances"`
}

type genericPackageInstanceFact struct {
	Key           string                        `json:"key"`
	Definition    string                        `json:"definition"`
	TypeArguments []genericPackageTypeDTO       `json:"typeArguments"`
	Origin        genericPackageOriginDTO       `json:"origin"`
	Parameters    []genericPackageParameterFact `json:"parameters"`
	ReturnType    string                        `json:"returnType"`
	Body          genericPackageBodyFact        `json:"body"`
}

type genericPackageParameterFact struct {
	Name   string                  `json:"name"`
	Type   string                  `json:"type"`
	Origin genericPackageOriginDTO `json:"origin"`
}

type genericPackageBodyFact struct {
	Kind      string                   `json:"kind"`
	Type      string                   `json:"type"`
	Parameter int                      `json:"parameter"`
	Callee    string                   `json:"callee"`
	Arguments []genericPackageBodyFact `json:"arguments"`
	Origin    genericPackageOriginDTO  `json:"origin"`
}

func instantiateGenericPackageFacts(ctx context.Context, canonicalPackage []byte, roots []genericPackageRoot) (genericPackageFacts, error) {
	if err := ctx.Err(); err != nil {
		return genericPackageFacts{}, err
	}
	if len(roots) == 0 || len(roots) > 256 {
		return genericPackageFacts{}, genericPackageError("limit", "root count")
	}
	for _, root := range roots {
		if len(root.TypeArguments) > 16 {
			return genericPackageFacts{}, genericPackageError("limit", "root type argument count")
		}
	}
	if err := genericPackageBudget(roots); err != nil {
		return genericPackageFacts{}, err
	}
	g, err := decodeGenericPackage(canonicalPackage)
	if err != nil {
		return genericPackageFacts{}, err
	}
	if err := ctx.Err(); err != nil {
		return genericPackageFacts{}, err
	}
	definitions := make(map[string]genericDefinition, len(g.Definitions))
	for _, d := range g.Definitions {
		definitions[d.Key] = d
	}
	requests := make([]genericInstanceRequest, len(roots))
	for i, root := range roots {
		if err := ctx.Err(); err != nil {
			return genericPackageFacts{}, err
		}
		d, found := definitions[root.Definition]
		if !found || !d.Exported {
			return genericPackageFacts{}, genericPackageError("reference", "root must name an exported definition")
		}
		if len(root.TypeArguments) != len(d.TypeParameters) {
			return genericPackageFacts{}, &genericFailure{Kind: "package-reference", Detail: "root type argument arity", Origin: d.Origin, Source: cloneGenericSource(g.Source)}
		}
		args := make([]genericType, len(root.TypeArguments))
		for j, arg := range root.TypeArguments {
			args[j] = genericType{Kind: arg.Kind, Parameter: arg.Parameter}
			if !genericValidType(args[j], 0) {
				return genericPackageFacts{}, &genericFailure{Kind: "package-reference", Detail: "root requires concrete boolean/number arguments", Origin: d.Origin, Source: cloneGenericSource(g.Source)}
			}
		}
		requests[i] = g.request(d.Key, args...)
	}
	instances, err := instantiateGenericGraph(ctx, g, requests)
	if err != nil {
		return genericPackageFacts{}, err
	}
	return mapGenericPackageFacts(ctx, g, definitions, instances)
}
