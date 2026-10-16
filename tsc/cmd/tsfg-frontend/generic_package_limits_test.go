// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestOwnedGenericPackageRejectsBudgetsAndInvalidUnicode(t *testing.T) {
	t.Parallel()
	data := genericPackageBytesForTest(t)
	for name, bad := range map[string][]byte{
		"bytes":               bytes.Repeat([]byte(" "), genericPackageMaxBytes+1),
		"depth":               []byte(strings.Repeat("[", genericPackageMaxDepth+1) + "0" + strings.Repeat("]", genericPackageMaxDepth+1)),
		"tokens":              []byte("[" + strings.Repeat("0,", genericPackageMaxTokens-2) + "0]"),
		"string":              []byte("\"" + strings.Repeat("x", genericPackageMaxString+1) + "\""),
		"invalid UTF8":        bytes.Replace(data, []byte("package:wire-candidate"), []byte{0xff}, 1),
		"lone high surrogate": bytes.Replace(data, []byte("package:wire-candidate"), []byte("\\ud800"), 1),
		"lone low surrogate":  bytes.Replace(data, []byte("package:wire-candidate"), []byte("\\udc00"), 1),
		"NUL":                 bytes.Replace(data, []byte("package:wire-candidate"), []byte("\\u0000"), 1),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := decodeGenericPackage(bad); err == nil {
				t.Fatal("out-of-budget or invalid Unicode input accepted")
			}
		})
	}
	wire := genericPackageWireForTest(t, data)
	slices.Reverse(wire.Payload.Definitions)
	if _, err := decodeGenericPackage(genericPackageSignedBytesForTest(t, wire)); err == nil {
		t.Fatal("unordered definitions accepted")
	}
}

func genericPackageBudgetTree(depth int, origin genericPackageOriginDTO) genericPackageExpressionDTO {
	e := genericPackageExpressionDTO{Kind: "parameter", Origin: origin, Arguments: []genericPackageExpressionDTO{}, TypeArguments: []genericPackageTypeDTO{}}
	if depth > 0 {
		e.Kind = "call"
		e.Arguments = []genericPackageExpressionDTO{genericPackageBudgetTree(depth-1, origin), genericPackageBudgetTree(depth-1, origin)}
	}
	return e
}

func TestOwnedGenericPackageChecksTotalExpressionBudgetBeforeReferences(t *testing.T) {
	t.Parallel()
	wire := genericPackageWireForTest(t, genericPackageBytesForTest(t))
	base := wire.Payload.Definitions[0]
	wire.Payload.Definitions = make([]genericPackageDefinitionDTO, 33)
	for i := range wire.Payload.Definitions {
		wire.Payload.Definitions[i] = base
		// Each tree has 1023 nodes, so 33 roots exceed the total 32768 budget.
		wire.Payload.Definitions[i].Body = genericPackageBudgetTree(9, base.Body.Origin)
		wire.Payload.Definitions[i].Key = strings.Repeat("0", 62) + string([]byte{'a' + byte(i/16), 'a' + byte(i%16)})
	}
	bad := genericPackageSignedBytesForTest(t, wire)
	if len(bad) > genericPackageMaxBytes {
		t.Fatal("total-expression test exceeded byte budget first")
	}
	_, err := decodeGenericPackage(bad)
	var failure *genericFailure
	if !errors.As(err, &failure) || failure.Kind != "package-limit" || failure.Detail != "expression depth/count/arity" {
		t.Fatalf("expression budget did not reject before interpreting references: %v", err)
	}
}

func rekeyGenericPackageForTest(wire *genericPackageDTO) {
	keys := map[string]string{}
	for i, d := range wire.Payload.Definitions {
		key := genericPackageIdentity(genericDefinitionDomain, genericPackageDefinitionIdentityDTO{
			Name: d.Name, Origin: d.Origin, Scope: wire.Payload.Scope, Source: wire.Payload.Source})
		keys[d.Key] = key
		wire.Payload.Definitions[i].Key = key
	}
	var rewrite func(*genericPackageExpressionDTO)
	rewrite = func(e *genericPackageExpressionDTO) {
		if key, ok := keys[e.Definition]; ok {
			e.Definition = key
		}
		for i := range e.Arguments {
			rewrite(&e.Arguments[i])
		}
	}
	for i := range wire.Payload.Definitions {
		rewrite(&wire.Payload.Definitions[i].Body)
	}
	slices.SortFunc(wire.Payload.Definitions, func(a, b genericPackageDefinitionDTO) int { return strings.Compare(a.Key, b.Key) })
}

func TestOwnedGenericPackageRejectsSplitUTF16Origins(t *testing.T) {
	t.Parallel()
	data := genericPackageBytesForTest(t)
	for _, endpoint := range []string{"start", "end"} {
		t.Run(endpoint, func(t *testing.T) {
			t.Parallel()
			wire := genericPackageWireForTest(t, data)
			for _, d := range wire.Payload.Definitions {
				if d.Name != "identity" {
					continue
				}
				position := d.Body.Origin.Start
				if endpoint == "end" {
					position = d.Body.Origin.End
				}
				wire.Payload.Source.SurrogateStarts = []int{position - 1}
			}
			rekeyGenericPackageForTest(&wire)
			_, err := decodeGenericPackage(genericPackageSignedBytesForTest(t, wire))
			var failure *genericFailure
			if !errors.As(err, &failure) || !strings.Contains(failure.Detail, "origin") {
				t.Fatalf("split UTF16 endpoint accepted or rejected for wrong reason: %v", err)
			}
		})
	}
}

func TestOwnedGenericPackageMetadataErrorsReturnWithoutPanic(t *testing.T) {
	t.Parallel()
	data := genericPackageBytesForTest(t)
	for _, test := range []struct {
		name   string
		mutate func(*genericGraph)
	}{
		{"scope NUL", func(g *genericGraph) { g.Package = "package:bad\x00" }},
		{"scope invalid UTF8", func(g *genericGraph) { g.Package = "package:bad\xff" }},
		{"scope size", func(g *genericGraph) { g.Package = strings.Repeat("x", 257) }},
		{"source path NUL", func(g *genericGraph) { g.Source.Path = "/bad\x00.ts" }},
		{"surrogate negative", func(g *genericGraph) { g.Source.SurrogateStarts = []int{-1} }},
		{"origin negative", func(g *genericGraph) { g.Definitions[0].Origin.Start = -1 }},
		{"name size", func(g *genericGraph) { g.Definitions[0].Name = strings.Repeat("x", genericPackageMaxString+1) }},
		{"declaration UTF8", func(g *genericGraph) { g.Environment.Files[0].Content = "\xff" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g, err := decodeGenericPackage(data)
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(&g)
			if _, err := encodeGenericPackage(g); err == nil {
				t.Fatal("invalid metadata encoded")
			}
		})
	}
	for _, scope := range []string{"package:bad\x00", "package:bad\xff", strings.Repeat("x", 257)} {
		program, environment := genericProgramForTest("export function safe<T>(value: T): T { return value; }")
		if _, err := exportGenericPackage(t.Context(), program, program.GetSourceFile("/original/main.ts"), scope, environment); err == nil {
			t.Fatal("invalid external export scope accepted")
		}
	}
}

func TestOwnedGenericPackageUnicodeNamesAndNoNormalization(t *testing.T) {
	t.Parallel()
	program, environment := genericProgramForTest("/* 😀 */ export function 𝒜<𝒯>(𝒳: 𝒯): 𝒯 { return 𝒳; }")
	first, err := exportGenericPackage(t.Context(), program, program.GetSourceFile("/original/main.ts"), "package:e\u0301", environment)
	if err != nil {
		t.Fatal(err)
	}
	second, err := exportGenericPackage(t.Context(), program, program.GetSourceFile("/original/main.ts"), "package:é", environment)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("package scope was Unicode-normalized")
	}
	g, err := decodeGenericPackage(first)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Source.SurrogateStarts) < 4 {
		t.Fatal("non-BMP source boundaries omitted")
	}
	d := genericDefinitionNamed(t, g, "𝒜")
	result, err := instantiateGenericGraph(t.Context(), g, []genericInstanceRequest{
		g.request(d.Key, genericType{Kind: "boolean"}), g.request(d.Key, genericType{Kind: "number"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Instances[result.Roots[0]].Body.Type != "bool" || result.Instances[result.Roots[1]].Body.Type != "f64" {
		t.Fatal("Unicode binder names changed official projection")
	}
}

func TestOwnedGenericPackageAggregateBudgetBeforeSerialization(t *testing.T) {
	t.Parallel()
	data := genericPackageBytesForTest(t)
	for _, name := range []string{"aggregate", "escaped expansion"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g, err := decodeGenericPackage(data)
			if err != nil {
				t.Fatal(err)
			}
			content, count := strings.Repeat("x", genericPackageMaxString), 8
			if name == "escaped expansion" {
				content, count = strings.Repeat("\x01", genericPackageMaxString), 2
			}
			g.Environment.Files = make([]genericDeclaration, count)
			digest := genericTextDigest(content)
			for i := range g.Environment.Files {
				g.Environment.Files[i] = genericDeclaration{
					Path: "/original/budget" + string(rune('a'+i)) + ".d.ts", Content: content, Digest: digest,
				}
			}
			// Every source string fits individually. The corrected public producer
			// must reject cumulative canonical bytes before digest/Marshal/DTO copy.
			_, err = encodeGenericPackage(g)
			var failure *genericFailure
			if !errors.As(err, &failure) || failure.Kind != "package-limit" || failure.Detail != "canonical bytes" {
				t.Fatalf("aggregate canonical budget did not return a controlled error: %v", err)
			}
		})
	}
}
