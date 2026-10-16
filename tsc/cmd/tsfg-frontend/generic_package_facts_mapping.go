// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Mapping copies already checked concrete values; it never infers a type.
package main

import (
	"context"
	"reflect"
	"slices"
)

type genericPackageFactMapper struct {
	ctx       context.Context
	instances map[string]genericInstance
	nodes     int
}

func mapGenericPackageFacts(ctx context.Context, g genericGraph, definitions map[string]genericDefinition, set genericInstanceSet) (genericPackageFacts, error) {
	if err := ctx.Err(); err != nil {
		return genericPackageFacts{}, err
	}
	if len(set.Instances) == 0 || len(set.Instances) > 512 {
		return genericPackageFacts{}, genericPackageError("limit", "instance count")
	}
	result := genericPackageFacts{PackageID: "sha256:" + g.Digest, SemanticProfile: g.Semantics,
		Roots: append([]string{}, set.Roots...), Instances: []genericPackageInstanceFact{}}
	// Count the top-level object and its empty instances array once. Each actual
	// instance then extends that array using the same canonical budget walker.
	budget := genericCanonicalBudget{}
	if err := budget.walk(reflect.ValueOf(result), 0); err != nil {
		return genericPackageFacts{}, err
	}
	keys := make([]string, 0, len(set.Instances))
	for key := range set.Instances {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	mapper := genericPackageFactMapper{ctx: ctx, instances: set.Instances}
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return genericPackageFacts{}, err
		}
		instance := set.Instances[key]
		d, found := definitions[instance.Definition]
		if !found || instance.Key != key || len(instance.Parameters) != len(d.Parameters) {
			return genericPackageFacts{}, genericError("input", "inconsistent checked instance", instance.Origin)
		}
		out := genericPackageInstanceFact{Key: key, Definition: instance.Definition,
			TypeArguments: genericPackageTypes(instance.TypeArguments), Origin: genericPackageOrigin(instance.Origin),
			ReturnType: instance.Return, Parameters: make([]genericPackageParameterFact, len(d.Parameters))}
		for i, parameter := range d.Parameters {
			out.Parameters[i] = genericPackageParameterFact{Name: parameter.Name, Type: instance.Parameters[i],
				Origin: genericPackageOrigin(parameter.Origin)}
		}
		var err error
		out.Body, err = mapper.expression(instance.Body, len(instance.Parameters), 0)
		if err != nil {
			return genericPackageFacts{}, err
		}
		// Only one bounded definition is mapped ahead of this cumulative check;
		// no unbounded full-result copy or serialization precedes the budget.
		if len(result.Instances) != 0 {
			if err := budget.add(1, 0); err != nil {
				return genericPackageFacts{}, err
			}
		}
		if err := budget.walk(reflect.ValueOf(out), 2); err != nil {
			return genericPackageFacts{}, err
		}
		result.Instances = append(result.Instances, out)
	}
	if err := ctx.Err(); err != nil {
		return genericPackageFacts{}, err
	}
	return result, nil
}

func (m *genericPackageFactMapper) expression(e genericConcreteExpression, parameterCount, depth int) (genericPackageBodyFact, error) {
	if err := m.ctx.Err(); err != nil {
		return genericPackageBodyFact{}, err
	}
	m.nodes++
	if depth > 64 || m.nodes > genericPackageMaxExpressions {
		return genericPackageBodyFact{}, genericPackageError("limit", "result expression depth/count")
	}
	out := genericPackageBodyFact{Kind: e.Kind, Type: e.Type, Origin: genericPackageOrigin(e.Origin),
		Arguments: []genericPackageBodyFact{}}
	switch e.Kind {
	case "parameter":
		if e.Parameter < 0 || e.Parameter >= parameterCount || e.Callee != "" || len(e.Arguments) != 0 {
			return genericPackageBodyFact{}, genericError("package-reference", "checked parameter reference", e.Origin)
		}
		out.Parameter = e.Parameter
	case "call":
		if _, found := m.instances[e.Callee]; !found {
			return genericPackageBodyFact{}, genericError("package-reference", "checked call closure", e.Origin)
		}
		out.Callee = e.Callee
		for _, arg := range e.Arguments {
			mapped, err := m.expression(arg, parameterCount, depth+1)
			if err != nil {
				return genericPackageBodyFact{}, err
			}
			out.Arguments = append(out.Arguments, mapped)
		}
	default:
		return genericPackageBodyFact{}, genericError("unsupported", "checked body kind", e.Origin)
	}
	return out, nil
}
