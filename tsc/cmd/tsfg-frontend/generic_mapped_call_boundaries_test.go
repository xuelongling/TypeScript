// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
)

func mappedCallSignatureTypes14(d *genericPackageDefinitionDTO) []*genericPackageTypeDTO {
	return []*genericPackageTypeDTO{&d.ReturnType, &d.Parameters[0].Type}
}

// Equal domains provide a control for negatives that change only raw order or
// duplicate occurrences, rather than relying on TypeScript's structural equality.
func mappedCallTwoKeyControl14(leaf, route *genericPackageDefinitionDTO) {
	for _, d := range []*genericPackageDefinitionDTO{leaf, route} {
		for _, typ := range mappedCallSignatureTypes14(d) {
			typ.Indexed.Object.Mapped.Keys = genericPackageKeyTypeDTO{Kind: "union", Literals: []string{"a", "b"}}
			typ.Indexed.Index = genericPackageKeyTypeDTO{Kind: "union", Literals: []string{"a", "b"}}
		}
	}
}

func TestOwnedGenericMappedCallContractsAndUnsafeBytes(t *testing.T) {
	t.Parallel()
	data := mappedCallColdBytes14(t, mappedCallFixture14(t))
	wire := genericPackageWireForTest(t, data)
	_, route := genericConditionalCallDefinitionsForTest(t, &wire)
	requests := []genericPackageRoot{{Definition: route.Key, TypeArguments: []genericPackageTypeDTO{{Kind: "boolean"}}}}
	for _, test := range []struct {
		name   string
		mutate func(*genericPackageDTO)
	}{
		{"version5-claim", func(w *genericPackageDTO) { w.Version = 5 }},
		{"schema5-claim", func(w *genericPackageDTO) {
			w.SchemaHash = "da7eb72ae306eb88863a44c945df77d572a96443a005fefbd787267fe90658bf"
		}},
		{"profile5-claim", func(w *genericPackageDTO) { w.Payload.SemanticProfile = "tsfg-owned-scalar-generics-host-v5" }},
		{"missing-new-capability", func(w *genericPackageDTO) {
			w.Payload.Capabilities = append(w.Payload.Capabilities[:3], w.Payload.Capabilities[4:]...)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			w := genericPackageWireForTest(t, data)
			test.mutate(&w)
			genericConditionalCallRejectForTest(t, genericPackageSignedBytesForTest(t, w), requests, "package-schema", genericOrigin{})
		})
	}
	wire.PackageID = "sha256:" + genericPackageIdentity("tsfg-owned-generic-package/v5\n", wire.Payload)
	bad, err := genericPackageCanonical(wire)
	if err != nil {
		t.Fatal(err)
	}
	genericConditionalCallRejectForTest(t, bad, requests, "package-identity", genericOrigin{})
	for _, test := range []struct{ name, from, to, kind string }{
		{"duplicate-key", "{\"family\":", "{\"family\":\"x\",\"family\":", "package-json"},
		{"unknown-field", "{\"family\":", "{\"privateSession\":0,\"family\":", "package-canonical"},
		{"null-wrapper", "\"indexed\":{", "\"indexed\":null,\"discarded\":{", "package-schema"},
		{"exponent", "\"version\":6", "\"version\":6e0", "package-json"},
		{"whitespace", "{\"family\":", " {\"family\":", "package-canonical"},
	} {
		t.Run(test.name, func(t *testing.T) {
			bad := bytes.Replace(data, []byte(test.from), []byte(test.to), 1)
			if bytes.Equal(bad, data) {
				t.Fatal("unsafe fixture did not change")
			}
			genericConditionalCallRejectForTest(t, bad, requests, test.kind, genericOrigin{})
		})
	}
}

func TestOwnedGenericMappedCallRejectsMalformedColdRelations(t *testing.T) {
	t.Parallel()
	data := mappedCallColdBytes14(t, mappedCallFixture14(t))
	leafOrigin, routeOrigin := genericOrigin{Start: 6, End: 118}, genericOrigin{Start: 119, End: 241}
	callOrigin := genericOrigin{Start: 224, End: 238}
	for _, test := range []struct {
		name, kind string
		origin     genericOrigin
		mutate     func(*genericPackageDefinitionDTO, *genericPackageDefinitionDTO)
	}{
		{"caller-extra-slot", "unsupported", routeOrigin, func(leaf, route *genericPackageDefinitionDTO) {
			p := route.Parameters[0]
			p.Name = "extra"
			route.Parameters = append(route.Parameters, p)
		}},
		{"caller-parameter-signature", "unsupported", routeOrigin, func(leaf, route *genericPackageDefinitionDTO) {
			route.Parameters[0].Type = genericPackageTypeDTO{Kind: "boolean"}
		}},
		{"caller-original-type-arity", "input", routeOrigin, func(leaf, route *genericPackageDefinitionDTO) { route.TypeParameters = []string{"T", "V"} }},
		{"callee-original-type-arity", "input", leafOrigin, func(leaf, route *genericPackageDefinitionDTO) { leaf.TypeParameters = []string{"U", "V"} }},
		{"callee-shadowed-by-value-slot", "package-reference", callOrigin, func(leaf, route *genericPackageDefinitionDTO) { route.Parameters[0].Name = "leaf" }},
		{"number-add-body", "unsupported", routeOrigin, func(leaf, route *genericPackageDefinitionDTO) { route.Body.Kind = "number-add" }},
		{"caller-type-slot1", "package-reference", callOrigin, func(leaf, route *genericPackageDefinitionDTO) { route.Body.TypeArguments[0].Parameter = 1 }},
		{"caller-value-slot1", "unsupported", routeOrigin, func(leaf, route *genericPackageDefinitionDTO) { route.Body.Arguments[0].Parameter = 1 }},
		{"callee-value-slot1", "unsupported", leafOrigin, func(leaf, route *genericPackageDefinitionDTO) { leaf.Body.Parameter = 1 }},
		{"missing-type-argument", "package-reference", callOrigin, func(leaf, route *genericPackageDefinitionDTO) { route.Body.TypeArguments = []genericPackageTypeDTO{} }},
		{"extra-type-argument", "package-reference", callOrigin, func(leaf, route *genericPackageDefinitionDTO) {
			route.Body.TypeArguments = append(route.Body.TypeArguments, genericPackageTypeDTO{Kind: "parameter"})
		}},
		{"missing-value-argument", "package-reference", callOrigin, func(leaf, route *genericPackageDefinitionDTO) { route.Body.Arguments = []genericPackageExpressionDTO{} }},
		{"extra-value-argument", "package-reference", callOrigin, func(leaf, route *genericPackageDefinitionDTO) {
			route.Body.Arguments = append(route.Body.Arguments, route.Body.Arguments[0])
		}},
		{"concrete-type-boolean", "unsupported", routeOrigin, func(leaf, route *genericPackageDefinitionDTO) {
			route.Body.TypeArguments[0] = genericPackageTypeDTO{Kind: "boolean"}
		}},
		{"concrete-type-number", "unsupported", routeOrigin, func(leaf, route *genericPackageDefinitionDTO) {
			route.Body.TypeArguments[0] = genericPackageTypeDTO{Kind: "number"}
		}},
		{"missing-target", "package-reference", callOrigin, func(leaf, route *genericPackageDefinitionDTO) { route.Body.Definition = strings.Repeat("0", 64) }},
		{"self", "unsupported", routeOrigin, func(leaf, route *genericPackageDefinitionDTO) { route.Body.Definition = route.Key }},
		{"mutual", "unsupported", leafOrigin, func(leaf, route *genericPackageDefinitionDTO) {
			argument := leaf.Body
			leaf.Body = genericPackageExpressionDTO{Kind: "call", Definition: route.Key, Origin: argument.Origin, TypeArguments: []genericPackageTypeDTO{{Kind: "parameter"}}, Arguments: []genericPackageExpressionDTO{argument}}
		}},
		{"nested-call-argument", "unsupported", routeOrigin, func(leaf, route *genericPackageDefinitionDTO) {
			argument := route.Body.Arguments[0]
			route.Body.Arguments[0] = genericPackageExpressionDTO{Kind: "call", Definition: leaf.Key, Origin: argument.Origin, TypeArguments: []genericPackageTypeDTO{{Kind: "parameter"}}, Arguments: []genericPackageExpressionDTO{argument}}
		}},
		{"call-metadata", "input", genericOrigin{}, func(leaf, route *genericPackageDefinitionDTO) { route.Body.Parameter = 1 }},
		{"callee-binder-text", "unsupported", routeOrigin, func(leaf, route *genericPackageDefinitionDTO) {
			for _, typ := range mappedCallSignatureTypes14(leaf) {
				typ.Indexed.Object.Mapped.Binder = "J"
			}
		}},
		{"binder-shadows-caller", "unsupported", routeOrigin, func(leaf, route *genericPackageDefinitionDTO) { route.TypeParameters[0] = "K" }},
		{"key-order", "unsupported", routeOrigin, func(leaf, route *genericPackageDefinitionDTO) {
			mappedCallTwoKeyControl14(leaf, route)
			for _, typ := range mappedCallSignatureTypes14(leaf) {
				typ.Indexed.Object.Mapped.Keys.Literals = []string{"b", "a"}
			}
		}},
		{"key-occurrences", "unsupported", routeOrigin, func(leaf, route *genericPackageDefinitionDTO) {
			mappedCallTwoKeyControl14(leaf, route)
			for _, typ := range mappedCallSignatureTypes14(leaf) {
				typ.Indexed.Object.Mapped.Keys.Literals = []string{"a", "b", "a"}
			}
		}},
		{"index-order", "unsupported", routeOrigin, func(leaf, route *genericPackageDefinitionDTO) {
			mappedCallTwoKeyControl14(leaf, route)
			for _, typ := range mappedCallSignatureTypes14(leaf) {
				typ.Indexed.Index.Literals = []string{"b", "a"}
			}
		}},
		{"index-occurrences", "unsupported", routeOrigin, func(leaf, route *genericPackageDefinitionDTO) {
			mappedCallTwoKeyControl14(leaf, route)
			for _, typ := range mappedCallSignatureTypes14(leaf) {
				typ.Indexed.Index.Literals = []string{"a", "b", "a"}
			}
		}},
		{"mapped-value", "unsupported", routeOrigin, func(leaf, route *genericPackageDefinitionDTO) {
			for _, typ := range mappedCallSignatureTypes14(leaf) {
				typ.Indexed.Object.Mapped.Value = genericPackageTypeDTO{Kind: "boolean"}
			}
		}},
		{"conditional-callee", "unsupported", routeOrigin, func(leaf, route *genericPackageDefinitionDTO) {
			p, b, n := genericPackageTypeDTO{Kind: "parameter"}, genericPackageTypeDTO{Kind: "boolean"}, genericPackageTypeDTO{Kind: "number"}
			for _, typ := range mappedCallSignatureTypes14(leaf) {
				*typ = genericPackageTypeDTO{Kind: "conditional", CheckType: &p, ExtendsType: &b, TrueType: &b, FalseType: &n}
			}
		}},
		{"definition-key", "package-identity", leafOrigin, func(leaf, route *genericPackageDefinitionDTO) { leaf.Name = "renamed" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			wire := genericPackageWireForTest(t, data)
			leaf, route := genericConditionalCallDefinitionsForTest(t, &wire)
			requests := []genericPackageRoot{{Definition: route.Key, TypeArguments: []genericPackageTypeDTO{{Kind: "boolean"}}}}
			test.mutate(leaf, route)
			// The existing rejection helper checks both decoder and facts adapter,
			// exact source anchors and zero partial results; it has no conditional gate.
			genericConditionalCallRejectForTest(t, genericPackageSignedBytesForTest(t, wire), requests, test.kind, test.origin)
		})
	}
}

func TestOwnedGenericMappedCallOwnsStorageAndRoots(t *testing.T) {
	t.Parallel()
	source := strings.Replace(mappedCallSourceForTest14("'value'", "'value'", "parameter"), "export function leaf", "function leaf", 1)
	data := genericMappedColdSourceForTest(t, source)
	g, err := decodeGenericPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	leaf := genericDefinitionNamed(t, g, "leaf")
	if leaf.Exported {
		t.Fatal("nonexported callee was made an exported root")
	}
	requests := []genericPackageRoot{genericPackageRootForTest(t, g, "route", "number"), genericPackageRootForTest(t, g, "route", "boolean"), genericPackageRootForTest(t, g, "route", "number")}
	before, requestBefore := bytes.Clone(data), mappedCallJSON14(t, requests)
	facts, err := instantiateGenericPackageFacts(t.Context(), data, requests)
	if err != nil {
		t.Fatal(err)
	}
	second, err := instantiateGenericPackageFacts(t.Context(), data, requests)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.Instances) != 4 || len(facts.Roots) != 3 || facts.Roots[0] != facts.Roots[2] || facts.Roots[0] == facts.Roots[1] {
		t.Fatal("ordered repeated roots or same-module nonexported callee closure changed")
	}
	for i, want := range []string{"f64", "bool"} {
		caller := genericPackageFactForTest(t, facts, facts.Roots[i])
		callee := genericPackageFactForTest(t, facts, caller.Body.Callee)
		if callee.Definition != leaf.Key || caller.Body.Type != want || callee.ReturnType != want || caller.Body.Arguments[0].Parameter != 0 {
			t.Fatal("fresh official same-module forwarding was lost")
		}
	}
	if !bytes.Equal(before, data) || !bytes.Equal(requestBefore, mappedCallJSON14(t, requests)) || !bytes.Equal(mappedCallJSON14(t, facts), mappedCallJSON14(t, second)) {
		t.Fatal("inputs changed or fresh results are nondeterministic")
	}
	want := mappedCallJSON14(t, second)
	facts.Roots[0] = "changed"
	facts.Instances[0].Parameters[0].Name = "changed"
	requests[0].TypeArguments[0].Kind = "boolean"
	clear(data)
	g.Definitions[0].Return.Indexed.Object.Mapped.Keys.Literals[0] = "changed"
	if !bytes.Equal(want, mappedCallJSON14(t, second)) {
		t.Fatal("cold results retained input or independent result storage")
	}
	_, err = instantiateGenericPackageFacts(t.Context(), before, []genericPackageRoot{{Definition: leaf.Key, TypeArguments: []genericPackageTypeDTO{{Kind: "boolean"}}}})
	requireGenericPackageFailureKind(t, err, "package-reference")
}

func TestOwnedGenericMappedCallRootAndKeyBudgets(t *testing.T) {
	t.Parallel()
	sixteen := strings.Repeat("'value' | ", 15) + "'value'"
	for _, test := range []struct {
		keys, index string
		occurrences int
	}{
		{sixteen, sixteen, 16},
		{"'" + strings.Repeat("é", 128) + "'", "'" + strings.Repeat("é", 128) + "'", 1},
	} {
		data := genericMappedColdSourceForTest(t, mappedCallSourceForTest14(test.keys, test.index, "parameter"))
		g, err := decodeGenericPackage(data)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range g.Definitions {
			if len(d.Return.Indexed.Object.Mapped.Keys.Literals) != test.occurrences || len(d.Return.Indexed.Index.Literals) != test.occurrences {
				t.Fatal("exact key occurrences were not retained")
			}
		}
		facts, err := instantiateGenericPackageFacts(t.Context(), data, []genericPackageRoot{genericPackageRootForTest(t, g, "route", "boolean"), genericPackageRootForTest(t, g, "route", "number")})
		if err != nil || len(facts.Instances) != 4 {
			t.Fatalf("exact16/256-byte mapped-call key control failed: %v", err)
		}
	}
	for _, test := range []struct{ name, keys, index, rejected string }{
		{"domain17", sixteen + " | 'value'", "'value'", sixteen + " | 'value'"},
		{"index17", "'value'", sixteen + " | 'value'", sixteen + " | 'value'"},
		{"key257-utf8", "'" + strings.Repeat("é", 128) + "x'", "'" + strings.Repeat("é", 128) + "x'", "'" + strings.Repeat("é", 128) + "x'"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := mappedCallSourceForTest14(test.keys, test.index, "parameter")
			program, environment := genericProgramForTest(source)
			if err := checkGenericDiagnostics(t.Context(), program, nil); err != nil {
				t.Fatal(err)
			}
			data, err := exportGenericPackage(t.Context(), program, program.GetSourceFile("/original/main.ts"), "package:mapped-call-budget", environment)
			requireGenericPackageFailureKind(t, err, "package-limit")
			start := strings.Index(source, test.rejected)
			want := genericOrigin{Start: len(utf16.Encode([]rune(source[:start]))), End: len(utf16.Encode([]rune(source[:start+len(test.rejected)])))}
			var failure *genericFailure
			if !errors.As(err, &failure) || failure.Origin != want || len(data) != 0 {
				t.Fatalf("raw key budget lost its exact original type-node anchor: %v", err)
			}
		})
	}
	data := mappedCallColdBytes14(t, mappedCallFixture14(t))
	g, err := decodeGenericPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	root := genericPackageRootForTest(t, g, "route", "boolean")
	roots := make([]genericPackageRoot, 256)
	for i := range roots {
		roots[i] = root
	}
	facts, err := instantiateGenericPackageFacts(t.Context(), data, roots)
	if err != nil || len(facts.Roots) != 256 || len(facts.Instances) != 2 {
		t.Fatalf("exact256 repeated roots should preserve order and deduplicate only instances: %v", err)
	}
	for _, key := range facts.Roots {
		if key != facts.Roots[0] {
			t.Fatal("repeated root order changed")
		}
	}
	badFacts, err := instantiateGenericPackageFacts(t.Context(), data, append(roots, root))
	requireGenericPackageFailureKind(t, err, "package-limit")
	if !reflect.DeepEqual(badFacts, genericPackageFacts{}) {
		t.Fatal("257 roots exposed partial facts")
	}
	wire := genericPackageWireForTest(t, data)
	wire.Payload.Definitions = make([]genericPackageDefinitionDTO, 257)
	_, err = decodeGenericPackage(genericPackageSignedBytesForTest(t, wire))
	requireGenericPackageFailureKind(t, err, "package-limit")
	// Budget a cyclic caller-owned graph before interpreting its wrapper role.
	cycle := &genericPackageIndexedTypeDTO{}
	typ := genericPackageTypeDTO{Kind: "indexed", Indexed: cycle}
	cycle.Object = typ
	_, err = instantiateGenericPackageFacts(t.Context(), data, []genericPackageRoot{{Definition: root.Definition, TypeArguments: []genericPackageTypeDTO{typ}}})
	requireGenericPackageFailureKind(t, err, "package-limit")
}
