// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"reflect"
	"testing"
)

func genericPackageFactsBytesFromSource(t *testing.T, source string) []byte {
	t.Helper()
	program, environment := genericProgramForTest(source)
	data, err := exportGenericPackage(t.Context(), program, program.GetSourceFile("/original/main.ts"), "package:facts-tests", environment)
	if err != nil {
		t.Fatal(err)
	}
	if err := program.Host().FS().WriteFile("/original/main.ts", ""); err != nil {
		t.Fatal(err)
	}
	if text, ok := program.Host().FS().ReadFile("/original/main.ts"); !ok || text != "" {
		t.Fatal("original in-memory source was not cleared")
	}
	// The old Program, checker, VFS and source text do not escape this helper.
	return data
}

func genericPackageRootForTest(t *testing.T, g genericGraph, name string, kinds ...string) genericPackageRoot {
	t.Helper()
	root := genericPackageRoot{Definition: genericDefinitionNamed(t, g, name).Key,
		TypeArguments: make([]genericPackageTypeDTO, len(kinds))}
	for i, kind := range kinds {
		root.TypeArguments[i] = genericPackageTypeDTO{Kind: kind}
	}
	return root
}

func genericPackageFactForTest(t *testing.T, facts genericPackageFacts, key string) genericPackageInstanceFact {
	t.Helper()
	for _, instance := range facts.Instances {
		if instance.Key == key {
			return instance
		}
	}
	t.Fatalf("missing concrete instance %q", key)
	return genericPackageInstanceFact{}
}

func genericPackageFactRequestsForTest(t *testing.T, g genericGraph) []genericPackageRoot {
	t.Helper()
	return []genericPackageRoot{
		genericPackageRootForTest(t, g, "route", "boolean"),
		genericPackageRootForTest(t, g, "unused", "number"),
		genericPackageRootForTest(t, g, "unused", "boolean"),
		genericPackageRootForTest(t, g, "route", "boolean"),
		genericPackageRootForTest(t, g, "route", "number"),
	}
}

func TestOwnedGenericPackageFactsFreshClosure(t *testing.T) {
	t.Parallel()
	data := genericPackageBytesForTest(t)
	// Independently rehashed from the qualified v4 package under exact private5.
	if genericTextDigest(string(data)) != "30b0c9f5063de2fd2e12662e6ad0d9d96718edab5185f563a4fcd34695a7190d" {
		t.Fatal("instance fixture no longer matches the independently checked package fixture")
	}
	g, err := decodeGenericPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	requests := genericPackageFactRequestsForTest(t, g)
	facts, err := instantiateGenericPackageFacts(t.Context(), data, requests)
	if err != nil {
		t.Fatal(err)
	}
	if facts.PackageID != "sha256:"+g.Digest || facts.SemanticProfile != genericSemantics ||
		len(facts.Roots) != 5 || len(facts.Instances) != 6 || facts.Roots[0] != facts.Roots[3] || facts.Roots[1] == facts.Roots[2] {
		t.Fatalf("lost package, deduplication, or phantom instance identity: %+v", facts)
	}
	for i, request := range requests {
		args := make([]genericType, len(request.TypeArguments))
		for j, arg := range request.TypeArguments {
			args[j] = genericType{Kind: arg.Kind, Parameter: arg.Parameter}
		}
		key := genericInstanceKey(g.request(request.Definition, args...))
		instance := genericPackageFactForTest(t, facts, facts.Roots[i])
		if facts.Roots[i] != key || instance.Definition != request.Definition || !reflect.DeepEqual(instance.TypeArguments, request.TypeArguments) {
			t.Fatal("root order or complete type arguments changed at the adapter")
		}
	}
	for i, instance := range facts.Instances {
		if i > 0 && facts.Instances[i-1].Key >= instance.Key {
			t.Fatal("instances are not strictly sorted by key")
		}
		d := genericDefinitionNamed(t, g, "identity")
		for _, candidate := range g.Definitions {
			if candidate.Key == instance.Definition {
				d = candidate
			}
		}
		if instance.Origin != genericPackageOrigin(d.Origin) || len(instance.Parameters) != len(d.Parameters) ||
			instance.Body.Origin != genericPackageOrigin(d.Body.Origin) || instance.Body.Type != instance.ReturnType {
			t.Fatal("original UTF16 declaration/body origins or concrete type lost")
		}
		for j, parameter := range instance.Parameters {
			if parameter.Name != d.Parameters[j].Name || parameter.Origin != genericPackageOrigin(d.Parameters[j].Origin) {
				t.Fatal("owned parameter metadata lost")
			}
		}
	}
	identity := genericDefinitionNamed(t, g, "identity")
	for _, test := range []struct {
		index        int
		kind, scalar string
	}{{0, "boolean", "bool"}, {4, "number", "f64"}} {
		route := genericPackageFactForTest(t, facts, facts.Roots[test.index])
		callee := genericInstanceKey(g.request(identity.Key, genericType{Kind: test.kind}))
		id := genericPackageFactForTest(t, facts, callee)
		if route.ReturnType != test.scalar || route.Parameters[0].Type != test.scalar || route.Body.Kind != "call" ||
			route.Body.Parameter != 0 || route.Body.Callee != callee || len(route.Body.Arguments) != 1 ||
			route.Body.Arguments[0].Kind != "parameter" || route.Body.Arguments[0].Parameter != 0 ||
			route.Body.Arguments[0].Type != test.scalar || route.Body.Arguments[0].Callee != "" ||
			len(route.Body.Arguments[0].Arguments) != 0 || id.Body.Kind != "parameter" || id.Body.Type != test.scalar {
			t.Fatalf("fresh official parameter/call facts changed: %+v", route)
		}
	}
	for _, index := range []int{1, 2} {
		phantom := genericPackageFactForTest(t, facts, facts.Roots[index])
		if phantom.ReturnType != "bool" || phantom.Parameters[0].Type != "bool" || phantom.Body.Type != "bool" {
			t.Fatal("phantom argument changed an unrelated official bool signature")
		}
	}
	fixture, err := genericPackageCanonical(struct {
		Requests []genericPackageRoot `json:"requests"`
		Result   genericPackageFacts  `json:"result"`
	}{Requests: requests, Result: facts})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("TSFG_GENERIC_INSTANCE_FIXTURE_V5 " + base64.StdEncoding.EncodeToString(fixture))
}

func TestOwnedGenericPackageFactsRecursiveInstances(t *testing.T) {
	t.Parallel()
	data := genericPackageFactsBytesFromSource(t, "/* recursive 😀 */ export function cycle<T>(value: T): T { return hidden<T>(value); }\n"+
		"function hidden<U>(item: U): U { return cycle<U>(item); }\n"+
		"export function self<V>(value: V): V { return self<V>(value); }")
	g, err := decodeGenericPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	requests := []genericPackageRoot{
		genericPackageRootForTest(t, g, "cycle", "boolean"), genericPackageRootForTest(t, g, "cycle", "number"),
		genericPackageRootForTest(t, g, "self", "boolean"), genericPackageRootForTest(t, g, "self", "number"),
		genericPackageRootForTest(t, g, "cycle", "boolean"),
	}
	facts, err := instantiateGenericPackageFacts(t.Context(), data, requests)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.Instances) != 6 || facts.Roots[0] != facts.Roots[4] {
		t.Fatal("recursive closure was not finite or deduplicated")
	}
	for i, want := range []string{"bool", "f64"} {
		cycle := genericPackageFactForTest(t, facts, facts.Roots[i])
		hidden := genericPackageFactForTest(t, facts, cycle.Body.Callee)
		self := genericPackageFactForTest(t, facts, facts.Roots[i+2])
		if cycle.Body.Type != want || hidden.Body.Callee != cycle.Key || hidden.Body.Type != want ||
			self.Body.Callee != self.Key || self.Body.Type != want || hidden.Definition != genericDefinitionNamed(t, g, "hidden").Key {
			t.Fatal("mutual/self recursive instance identity or scalar fact lost")
		}
	}
}

func TestOwnedGenericPackageFactsOwnStorageAndCanonicalOutput(t *testing.T) {
	t.Parallel()
	data := genericPackageBytesForTest(t)
	g, err := decodeGenericPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	requests := genericPackageFactRequestsForTest(t, g)
	first, err := instantiateGenericPackageFacts(t.Context(), data, requests)
	if err != nil {
		t.Fatal(err)
	}
	second, err := instantiateGenericPackageFacts(t.Context(), data, requests)
	if err != nil {
		t.Fatal(err)
	}
	before, err := genericPackageCanonical(second)
	if err != nil {
		t.Fatal(err)
	}
	requests[0].TypeArguments[0].Kind = "number"
	requests[0].Definition = "changed"
	clear(data)
	after, err := genericPackageCanonical(first)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("result retained mutable request or package storage")
	}
	if !bytes.Contains(after, []byte(`"arguments":[]`)) || !bytes.Contains(after, []byte(`"callee":""`)) ||
		!bytes.Contains(after, []byte(`"parameter":0`)) || bytes.Contains(after, []byte("null")) ||
		bytes.Contains(after, []byte(`"files"`)) || bytes.Contains(after, []byte(`"inputs"`)) || bytes.Contains(after, []byte("export function")) {
		t.Fatal("required expression fields absent or result imitated source intake")
	}
	phantomKey := first.Roots[2]
	otherPhantom := genericPackageFactForTest(t, first, first.Roots[1])
	for i := range first.Instances {
		instance := &first.Instances[i]
		if instance.Key == phantomKey {
			instance.Parameters[0].Name = "changed"
			instance.TypeArguments[0].Kind = "number"
		}
		if instance.Key == first.Roots[0] {
			instance.Body.Arguments[0].Type = "changed"
		}
	}
	first.Roots[0] = "changed"
	if otherPhantom.Parameters[0].Name != "flag" || otherPhantom.TypeArguments[0].Kind != "number" {
		t.Fatal("distinct instances shared mutable parameter or type-argument arrays")
	}
	after, err = genericPackageCanonical(second)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("independent adapter results shared mutable nested arrays")
	}
}
