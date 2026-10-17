// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Instantiation identity is independent of the ordinary one-symbol function map.
package main

import (
	"context"
	"slices"
)

type genericInstanceRequest struct {
	Package       string
	Semantics     string
	GraphDigest   string
	Definition    string
	TypeArguments []genericType
}

type genericConcreteExpression struct {
	Kind      string
	Type      string
	Parameter int
	Callee    string
	Arguments []genericConcreteExpression
	Origin    genericOrigin
}

type genericInstance struct {
	Key           string
	Definition    string
	TypeArguments []genericType
	Source        genericSource
	Origin        genericOrigin
	Parameters    []string
	Return        string
	Body          genericConcreteExpression
}

type genericInstanceSet struct {
	Roots     []string
	Instances map[string]genericInstance
}

func (g genericGraph) request(key string, args ...genericType) genericInstanceRequest {
	return genericInstanceRequest{Package: g.Package, Semantics: g.Semantics, GraphDigest: g.Digest,
		Definition: key, TypeArguments: slices.Clone(args)}
}

func genericInstanceKey(r genericInstanceRequest) string {
	// Include phantom arguments even when they do not appear in the signature.
	return genericPackageIdentity(genericInstanceDomain, genericPackageInstanceIdentityDTO{
		Definition: r.Definition, GraphDigest: r.GraphDigest, Scope: r.Package, SemanticProfile: r.Semantics,
		TypeArguments: genericPackageTypes(r.TypeArguments)})
}

func validateGenericRequest(g genericGraph, r genericInstanceRequest) error {
	if r.Package != g.Package || r.Semantics != g.Semantics || r.GraphDigest != g.Digest {
		return genericError("input", "request package/semantics/graph identity", genericOrigin{})
	}
	for _, d := range g.Definitions {
		if d.Key != r.Definition {
			continue
		}
		if len(r.TypeArguments) != len(d.TypeParameters) {
			break
		}
		for _, a := range r.TypeArguments {
			if !genericTypeLeaf(a) || a.Kind != "boolean" && a.Kind != "number" || !genericValidType(a, 0) {
				return genericError("unsupported", "only concrete boolean/number arguments", d.Origin)
			}
		}
		return nil
	}
	return genericError("input", "missing definition or type argument arity", genericOrigin{})
}

func instantiateGenericGraph(ctx context.Context, g genericGraph, requests []genericInstanceRequest) (genericInstanceSet, error) {
	if err := ctx.Err(); err != nil {
		return genericInstanceSet{}, err
	}
	if err := g.validate(); err != nil {
		return genericInstanceSet{}, err
	}
	if len(requests) == 0 || len(requests) > 256 {
		return genericInstanceSet{}, genericError("input", "request count", genericOrigin{})
	}
	result := genericInstanceSet{Instances: map[string]genericInstance{}}
	pending := make([]genericInstanceRequest, 0, len(requests))
	queued := map[string]bool{}
	for _, r := range requests {
		if err := ctx.Err(); err != nil {
			return genericInstanceSet{}, err
		}
		if err := validateGenericRequest(g, r); err != nil {
			return genericInstanceSet{}, err
		}
		key := genericInstanceKey(r)
		result.Roots = append(result.Roots, key)
		if !queued[key] {
			pending = append(pending, r)
			queued[key] = true
		}
	}
	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return genericInstanceSet{}, err
		}
		r := pending[0]
		pending = pending[1:]
		instance, calls, err := projectGenericInstance(ctx, g, r)
		if err != nil {
			return genericInstanceSet{}, err
		}
		result.Instances[instance.Key] = instance
		for _, call := range calls {
			if err := validateGenericRequest(g, call); err != nil {
				return genericInstanceSet{}, err
			}
			key := genericInstanceKey(call)
			if !queued[key] {
				if len(queued) >= 512 {
					return genericInstanceSet{}, genericError("unsupported", "instance closure limit", instance.Origin)
				}
				queued[key] = true
				pending = append(pending, call)
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return genericInstanceSet{}, err
	}
	return result, nil
}
