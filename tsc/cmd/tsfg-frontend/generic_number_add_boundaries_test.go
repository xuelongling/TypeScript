// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestOwnedGenericNumberAddRejectsOldAndMixedPackageClaims(t *testing.T) {
	t.Parallel()
	data := genericNumberAddColdBytes(t)
	for _, test := range []struct {
		name   string
		mutate func(*genericPackageDTO)
	}{
		{"old-package1", func(w *genericPackageDTO) { w.Version = 1 }},
		{"unknown-package999", func(w *genericPackageDTO) { w.Version = 999 }},
		{"old-schema", func(w *genericPackageDTO) {
			w.SchemaHash = "5f3e8a6e8e3b49dcbdf507db643c538a5810cc3a09f75ea7dc4db560086e9e4b"
		}},
		{"unknown-schema", func(w *genericPackageDTO) { w.SchemaHash = strings.Repeat("0", 64) }},
		{"old-host-profile", func(w *genericPackageDTO) { w.Payload.SemanticProfile = "tsfg-owned-scalar-generics-host-v1" }},
		{"old-capabilities", func(w *genericPackageDTO) {
			w.Payload.Capabilities = []string{"explicit-type-arguments", "scalar-generics", "single-module"}
		}},
		{"capability-order", func(w *genericPackageDTO) {
			w.Payload.Capabilities[0], w.Payload.Capabilities[1] = w.Payload.Capabilities[1], w.Payload.Capabilities[0]
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			wire := genericPackageWireForTest(t, data)
			test.mutate(&wire)
			graph, err := decodeGenericPackage(genericPackageSignedBytesForTest(t, wire))
			requireGenericPackageFailureKind(t, err, "package-schema")
			if !reflect.DeepEqual(graph, genericGraph{}) {
				t.Fatal("rejected package contract exposed a partial owned graph")
			}
		})
	}
	t.Run("old-packageId-domain", func(t *testing.T) {
		t.Parallel()
		wire := genericPackageWireForTest(t, data)
		wire.PackageID = "sha256:" + genericPackageIdentity("tsfg-owned-generic-package/v1\n", wire.Payload)
		bad, err := genericPackageCanonical(wire)
		if err != nil {
			t.Fatal(err)
		}
		_, err = decodeGenericPackage(bad)
		requireGenericPackageFailureKind(t, err, "package-identity")
	})
}

// These are deliberately malformed cold DTOs derived from actual bytes, never
// replacement Source/checker success facts. Re-signing only the outer package
// ensures each owned body restriction rejects before fresh semantic checking.
func TestOwnedGenericNumberAddRejectsMalformedColdBodies(t *testing.T) {
	t.Parallel()
	data := genericNumberAddColdBytes(t)
	for _, test := range []struct {
		name, kind string
		mutate     func(*genericPackageDefinitionDTO)
	}{
		{"one-operand", "input", func(d *genericPackageDefinitionDTO) { d.Body.Arguments = d.Body.Arguments[:1] }},
		{"three-operands", "input", func(d *genericPackageDefinitionDTO) { d.Body.Arguments = append(d.Body.Arguments, d.Body.Arguments[0]) }},
		{"foreign-parameter", "package-reference", func(d *genericPackageDefinitionDTO) { d.Body.Arguments[0].Parameter = 2 }},
		{"boolean-parameter", "package-reference", func(d *genericPackageDefinitionDTO) { d.Parameters[0].Type = genericPackageTypeDTO{Kind: "boolean"} }},
		{"type-parameter-operand", "package-reference", func(d *genericPackageDefinitionDTO) { d.Parameters[0].Type = genericPackageTypeDTO{Kind: "parameter"} }},
		{"nonzero-add-parameter", "input", func(d *genericPackageDefinitionDTO) { d.Body.Parameter = 1 }},
		{"spurious-target", "input", func(d *genericPackageDefinitionDTO) { d.Body.Definition = d.Key }},
		{"spurious-type-arguments", "input", func(d *genericPackageDefinitionDTO) { d.Body.TypeArguments = []genericPackageTypeDTO{{Kind: "number"}} }},
		{"nonnumber-return-slot", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType = genericPackageTypeDTO{Kind: "boolean"} }},
		{"nested-add", "package-reference", func(d *genericPackageDefinitionDTO) { d.Body.Arguments[0].Kind = "number-add" }},
		{"literal-operand", "package-reference", func(d *genericPackageDefinitionDTO) { d.Body.Arguments[1].Kind = "number" }},
		{"call-operand", "package-reference", func(d *genericPackageDefinitionDTO) { d.Body.Arguments[0].Kind = "call" }},
		{"operand-before-add", "input", func(d *genericPackageDefinitionDTO) { d.Body.Arguments[0].Origin.Start = 74 }},
		{"operand-after-add", "input", func(d *genericPackageDefinitionDTO) { d.Body.Arguments[1].Origin.End = 88 }},
		{"reversed-occurrences", "input", func(d *genericPackageDefinitionDTO) {
			d.Body.Arguments[0], d.Body.Arguments[1] = d.Body.Arguments[1], d.Body.Arguments[0]
		}},
		{"overlapping-occurrences", "input", func(d *genericPackageDefinitionDTO) { d.Body.Arguments[1].Origin.Start = 78 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			wire := genericPackageWireForTest(t, data)
			if len(wire.Payload.Definitions) != 1 || wire.Payload.Definitions[0].Body.Kind != "number-add" {
				t.Fatal("actual number-add fixture changed before mutation")
			}
			test.mutate(&wire.Payload.Definitions[0])
			bad := genericPackageSignedBytesForTest(t, wire)
			graph, err := decodeGenericPackage(bad)
			requireGenericPackageFailureKind(t, err, test.kind)
			if !reflect.DeepEqual(graph, genericGraph{}) {
				t.Fatal("malformed body exposed a partial owned graph")
			}
			roots := []genericPackageRoot{{Definition: wire.Payload.Definitions[0].Key,
				TypeArguments: []genericPackageTypeDTO{{Kind: "number"}}}}
			facts, err := instantiateGenericPackageFacts(t.Context(), bad, roots)
			requireGenericPackageFailureKind(t, err, test.kind)
			if !reflect.DeepEqual(facts, genericPackageFacts{}) {
				t.Fatal("malformed package reached a successful fresh instance result")
			}
		})
	}
}

func TestOwnedGenericNumberAddRejectsUnsupportedOfficialSourceBodies(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, replacement, rejected string }{
		{"non-plus", "left - right", "left - right"},
		{"nested-add", "left + (right + left)", "(right + left)"},
		{"literal-operand", "left + 1", "1"},
		{"grouped-parameter", "(left) + right", "(left)"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			source := strings.Replace(genericNumberAddSource, "left + right", test.replacement, 1)
			program, environment := genericProgramForTest(source)
			if err := checkGenericDiagnostics(t.Context(), program, nil); err != nil {
				t.Fatalf("number-add capability fixture is not officially valid: %v", err)
			}
			data, err := exportGenericPackage(t.Context(), program, program.GetSourceFile("/original/main.ts"),
				"package:number-add-boundary", environment)
			requireGenericPackageFailureKind(t, err, "unsupported")
			var failure *genericFailure
			if !errors.As(err, &failure) || !strings.Contains(failure.Detail, "number-add") || len(data) != 0 {
				t.Fatalf("unsupported body lost its capability cause or returned partial bytes: %v", err)
			}
			start := strings.Index(source, test.rejected)
			if start < 0 {
				t.Fatal("unsupported source occurrence is missing")
			}
			end := start + len(test.rejected)
			if failure.Origin != (genericOrigin{Start: len(utf16.Encode([]rune(source[:start]))), End: len(utf16.Encode([]rune(source[:end])))}) {
				t.Fatalf("unsupported body moved its original UTF16 occurrence: %+v", failure.Origin)
			}
		})
	}
}

// Invalid TypeScript remains an official diagnostic, distinct from the owned
// number-only shape restriction. No hand-built graph serves as its oracle.
func TestOwnedGenericNumberAddKeepsOfficialTypeFailures(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, source string
		code         int32
	}{
		{"wrong-return", strings.Replace(genericNumberAddSource, "): number {", "): boolean {", 1), 2322},
		{"unconstrained-T-plus-T", strings.Replace(genericNumberAddSource, "left: number, right: number", "left: T, right: T", 1), 2365},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			program, environment := genericProgramForTest(test.source)
			data, err := exportGenericPackage(t.Context(), program, program.GetSourceFile("/original/main.ts"),
				"package:number-add-official-error", environment)
			var failure *genericFailure
			if !errors.As(err, &failure) || failure.Kind != "official-type" || failure.Code != test.code || len(data) != 0 ||
				failure.Source.Path != "/original/main.ts" || failure.Source.Digest != genericTextDigest(test.source) ||
				failure.Source.Length != len(utf16.Encode([]rune(test.source))) ||
				!reflect.DeepEqual(failure.Source.SurrogateStarts, []int{3}) || failure.Origin.Start < 6 ||
				failure.Origin.End <= failure.Origin.Start || failure.Origin.End > failure.Source.Length {
				t.Fatalf("number-add error lost real official code/source/origin or exposed bytes: %v", err)
			}
		})
	}
}

// The two new binary factory trees must remain semantic when diagnostic text
// is blank. The retained package provides only metadata and declaration text.
func TestOwnedGenericNumberAddFreshCheckerIgnoresDiagnosticText(t *testing.T) {
	t.Parallel()
	g, err := decodeGenericPackage(genericNumberAddColdBytes(t))
	if err != nil {
		t.Fatal(err)
	}
	d := genericDefinitionNamed(t, g, "sum")
	for _, kind := range []string{"boolean", "number"} {
		request := g.request(d.Key, genericType{Kind: kind})
		normalTree, blankTree := genericTreeForTest(t, g, request), genericTreeForTest(t, g, request)
		if !reflect.DeepEqual(genericSyntaxFacts(normalTree), genericSyntaxFacts(blankTree)) {
			t.Fatal("number-add factory syntax changed before metamorphic checking")
		}
		normal := newGenericProgram(g, normalTree, normalTree.text.String())
		blank := newGenericProgram(g, blankTree, strings.Repeat(" ", blankTree.text.Len()))
		if err := checkGenericProgram(t.Context(), g, normal, normalTree); err != nil {
			t.Fatal(err)
		}
		if err := checkGenericProgram(t.Context(), g, blank, blankTree); err != nil {
			t.Fatal(err)
		}
		a, ac, err := projectCheckedGenericInstance(t.Context(), g, request, normalTree, normal)
		if err != nil {
			t.Fatal(err)
		}
		b, bc, err := projectCheckedGenericInstance(t.Context(), g, request, blankTree, blank)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(a, b) || len(ac) != 0 || len(bc) != 0 || a.Return != "f64" ||
			!reflect.DeepEqual(a.Parameters, []string{"f64", "f64"}) || a.Body.Kind != "number-add" || a.Body.Type != "f64" ||
			a.Body.Origin != (genericOrigin{Start: 75, End: 87}) || len(a.Body.Arguments) != 2 {
			t.Fatal("diagnostic carrier text changed real official mapped number-add facts")
		}
	}
}
