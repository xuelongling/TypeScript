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

	"github.com/microsoft/typescript-go/internal/ast"
)

func TestOwnedGenericConditionalSignatureRejectsOldAndMixedPackageClaims(t *testing.T) {
	t.Parallel()
	data := genericConditionalSignatureColdBytes(t)
	for _, test := range []struct {
		name   string
		mutate func(*genericPackageDTO)
	}{
		{"old-package2", func(w *genericPackageDTO) { w.Version = 2 }},
		{"old-schema2", func(w *genericPackageDTO) {
			w.SchemaHash = "7afee9732817ce2eca33d8aeb6f710cb2bc302eca5a183b64e7774e0e1e81f3a"
		}},
		{"old-host2", func(w *genericPackageDTO) { w.Payload.SemanticProfile = "tsfg-owned-scalar-generics-host-v2" }},
		{"old-capabilities2", func(w *genericPackageDTO) {
			w.Payload.Capabilities = []string{"explicit-type-arguments", "number-add", "scalar-generics", "single-module"}
		}},
		{"missing-capability", func(w *genericPackageDTO) { w.Payload.Capabilities = w.Payload.Capabilities[1:] }},
		{"extra-capability", func(w *genericPackageDTO) { w.Payload.Capabilities = append(w.Payload.Capabilities, "infer") }},
		{"duplicate-capability", func(w *genericPackageDTO) { w.Payload.Capabilities[1] = w.Payload.Capabilities[0] }},
		{"capability-order", func(w *genericPackageDTO) {
			w.Payload.Capabilities[0], w.Payload.Capabilities[1] = w.Payload.Capabilities[1], w.Payload.Capabilities[0]
		}},
		{"unknown-schema", func(w *genericPackageDTO) { w.SchemaHash = strings.Repeat("0", 64) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			wire := genericPackageWireForTest(t, data)
			test.mutate(&wire)
			graph, err := decodeGenericPackage(genericPackageSignedBytesForTest(t, wire))
			requireGenericPackageFailureKind(t, err, "package-schema")
			if !reflect.DeepEqual(graph, genericGraph{}) {
				t.Fatal("rejected contract exposed a partial graph")
			}
		})
	}
	// This body and its leaf signatures are valid in the complete predecessor
	// contract; only its actual package contract is now obsolete.
	t.Run("complete-predecessor2", func(t *testing.T) {
		t.Parallel()
		wire := genericPackageWireForTest(t, genericNumberAddColdBytes(t))
		wire.Version = 2
		wire.SchemaHash = "7afee9732817ce2eca33d8aeb6f710cb2bc302eca5a183b64e7774e0e1e81f3a"
		wire.Payload.SemanticProfile = "tsfg-owned-scalar-generics-host-v2"
		wire.Payload.Capabilities = []string{"explicit-type-arguments", "number-add", "scalar-generics", "single-module"}
		wire.PackageID = "sha256:" + genericPackageIdentity("tsfg-owned-generic-package/v2\n", wire.Payload)
		bad, err := genericPackageCanonical(wire)
		if err != nil {
			t.Fatal(err)
		}
		_, err = decodeGenericPackage(bad)
		requireGenericPackageFailureKind(t, err, "package-schema")
	})
	for _, domain := range []string{"tsfg-owned-generic-package/v2\n", "tsfg-owned-generic-package/v5\n"} {
		t.Run("identity-"+strings.TrimSpace(domain), func(t *testing.T) {
			t.Parallel()
			wire := genericPackageWireForTest(t, data)
			wire.PackageID = "sha256:" + genericPackageIdentity(domain, wire.Payload)
			if strings.Contains(domain, "/v5") {
				wire.PackageID = "sha256:" + strings.Repeat("0", 64)
			}
			bad, err := genericPackageCanonical(wire)
			if err != nil {
				t.Fatal(err)
			}
			_, err = decodeGenericPackage(bad)
			requireGenericPackageFailureKind(t, err, "package-identity")
		})
	}
}

// Every mutation starts with actual exported bytes. Re-signing preserves the
// package identity check so the specific owned restriction must reject.
func TestOwnedGenericConditionalSignatureRejectsMalformedColdTypesAndBodies(t *testing.T) {
	t.Parallel()
	data := genericConditionalSignatureColdBytes(t)
	for _, test := range []struct {
		name, kind string
		mutate     func(*genericPackageDefinitionDTO)
	}{
		{"missing-check", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.CheckType = nil }},
		{"missing-extends", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.ExtendsType = nil }},
		{"missing-true", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.TrueType = nil }},
		{"missing-false", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.FalseType = nil }},
		{"nonzero-conditional-parameter", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.Parameter = 1 }},
		{"unbound-check", "package-reference", func(d *genericPackageDefinitionDTO) { d.ReturnType.CheckType.Parameter = 1 }},
		{"nonparameter-check", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.CheckType.Kind = "number" }},
		{"wrong-extends-role", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.ExtendsType.Kind = "number" }},
		{"wrong-true-role", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.TrueType.Kind = "number" }},
		{"wrong-false-role", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.FalseType.Kind = "boolean" }},
		{"nonzero-leaf-parameter", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.TrueType.Parameter = 1 }},
		{"nested-check-child", "input", func(d *genericPackageDefinitionDTO) {
			d.ReturnType.CheckType.TrueType = &genericPackageTypeDTO{Kind: "boolean"}
		}},
		{"nested-scalar-child", "input", func(d *genericPackageDefinitionDTO) {
			d.ReturnType.ExtendsType.CheckType = &genericPackageTypeDTO{Kind: "parameter"}
		}},
		{"unknown-type-role", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.FalseType.Kind = "unknown" }},
		{"hidden-scalar-children", "input", func(d *genericPackageDefinitionDTO) { d.Parameters[0].Type.Kind = "boolean" }},
		{"different-parameter-type", "unsupported", func(d *genericPackageDefinitionDTO) { d.Parameters[0].Type = genericPackageTypeDTO{Kind: "boolean"} }},
		{"different-return-type", "unsupported", func(d *genericPackageDefinitionDTO) { d.ReturnType = genericPackageTypeDTO{Kind: "boolean"} }},
		{"two-type-parameters", "unsupported", func(d *genericPackageDefinitionDTO) { d.TypeParameters = append(d.TypeParameters, "U") }},
		{"two-value-parameters", "unsupported", func(d *genericPackageDefinitionDTO) {
			p := d.Parameters[0]
			p.Name = "other"
			p.Type = genericPackageTypeDTO{Kind: "number"}
			d.Parameters = append(d.Parameters, p)
		}},
		{"body-parameter1", "unsupported", func(d *genericPackageDefinitionDTO) { d.Body.Parameter = 1 }},
		{"body-call", "package-reference", func(d *genericPackageDefinitionDTO) {
			d.Body.Kind = "call"
			d.Body.Definition = d.Key
			d.Body.TypeArguments = []genericPackageTypeDTO{{Kind: "parameter"}}
		}},
		{"body-number-add", "unsupported", func(d *genericPackageDefinitionDTO) { d.Body.Kind = "number-add" }},
		{"body-hidden-target", "input", func(d *genericPackageDefinitionDTO) { d.Body.Definition = d.Key }},
		{"conditional-type-argument", "unsupported", func(d *genericPackageDefinitionDTO) { d.Body.TypeArguments = []genericPackageTypeDTO{d.ReturnType} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			wire := genericPackageWireForTest(t, data)
			test.mutate(&wire.Payload.Definitions[0])
			bad := genericPackageSignedBytesForTest(t, wire)
			graph, err := decodeGenericPackage(bad)
			requireGenericPackageFailureKind(t, err, test.kind)
			var failure *genericFailure
			wantOrigin := genericOrigin{Start: 6, End: 132}
			if test.name == "hidden-scalar-children" {
				wantOrigin = genericOrigin{Start: 32, End: 75}
			}
			if test.name == "conditional-type-argument" || test.name == "body-call" {
				wantOrigin = genericOrigin{Start: 124, End: 129}
			}
			// Existing expression-metadata failures have no source occurrence.
			if test.name == "body-hidden-target" {
				wantOrigin = genericOrigin{}
			}
			if !errors.As(err, &failure) || failure.Code != 0 || failure.Origin != wantOrigin ||
				!reflect.DeepEqual(graph, genericGraph{}) {
				t.Fatalf("cold rejection lost its expected original anchor or exposed a graph: %v", err)
			}
			facts, err := instantiateGenericPackageFacts(t.Context(), bad, []genericPackageRoot{{Definition: wire.Payload.Definitions[0].Key,
				TypeArguments: []genericPackageTypeDTO{{Kind: "boolean"}}}})
			requireGenericPackageFailureKind(t, err, test.kind)
			if !reflect.DeepEqual(facts, genericPackageFacts{}) {
				t.Fatal("malformed conditional package exposed partial facts")
			}
		})
	}
}

func TestOwnedGenericConditionalSignatureRejectsHiddenRootTypeFields(t *testing.T) {
	t.Parallel()
	data := genericConditionalSignatureColdBytes(t)
	wire := genericPackageWireForTest(t, data)
	for _, field := range []string{"check", "extends", "true", "false", "whole-conditional"} {
		t.Run(field, func(t *testing.T) {
			t.Parallel()
			argument := genericPackageTypeDTO{Kind: "boolean"}
			child := &genericPackageTypeDTO{Kind: "number"}
			switch field {
			case "check":
				argument.CheckType = child
			case "extends":
				argument.ExtendsType = child
			case "true":
				argument.TrueType = child
			case "false":
				argument.FalseType = child
			case "whole-conditional":
				argument = wire.Payload.Definitions[0].ReturnType
			}
			facts, err := instantiateGenericPackageFacts(t.Context(), data, []genericPackageRoot{{Definition: wire.Payload.Definitions[0].Key,
				TypeArguments: []genericPackageTypeDTO{argument}}})
			requireGenericPackageFailureKind(t, err, "package-reference")
			if !reflect.DeepEqual(facts, genericPackageFacts{}) {
				t.Fatal("root type fields were discarded or exposed partial facts")
			}
		})
	}
}

func TestOwnedGenericConditionalSignatureRejectsNullAndUnknownTypeFields(t *testing.T) {
	t.Parallel()
	data := genericConditionalSignatureColdBytes(t)
	for _, test := range []struct{ name, old, replacement, kind string }{
		{"null-check", `"checkType":{"kind":"parameter","parameter":0}`, `"checkType":null`, "package-schema"},
		{"null-extends", `"extendsType":{"kind":"boolean","parameter":0}`, `"extendsType":null`, "package-schema"},
		{"null-true", `"trueType":{"kind":"boolean","parameter":0}`, `"trueType":null`, "package-schema"},
		{"null-false", `"falseType":{"kind":"number","parameter":0}`, `"falseType":null`, "package-schema"},
		{"unknown-child-field", `"checkType":{"kind":`, `"checkType":{"extra":true,"kind":`, "package-canonical"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			bad := bytes.Replace(data, []byte(test.old), []byte(test.replacement), 1)
			if bytes.Equal(data, bad) {
				t.Fatal("malformed conditional bytes were not changed")
			}
			_, err := decodeGenericPackage(bad)
			requireGenericPackageFailureKind(t, err, test.kind)
		})
	}
}

func TestOwnedGenericConditionalSignatureOwnsTypeStorageAndCountsExactBudget(t *testing.T) {
	t.Parallel()
	data := genericConditionalSignatureColdBytes(t)
	first, err := decodeGenericPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	second, err := decodeGenericPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	wire := genericPackageWireForTest(t, data)
	payload, err := genericPackageCanonical(wire.Payload)
	if err != nil {
		t.Fatal(err)
	}
	stats, err := inspectGenericJSON(payload, genericPackageJSONPolicy())
	if err != nil {
		t.Fatal(err)
	}
	budget := &genericCanonicalBudget{policy: genericPackageJSONPolicy()}
	if err := budget.walk(reflect.ValueOf(second), 0); err != nil || budget.bytes != len(payload) || budget.tokens != stats.tokens {
		t.Fatalf("optional conditional fields or shared capabilities escaped the exact budget: bytes=%d/%d tokens=%d/%d error=%v", budget.bytes, len(payload), budget.tokens, stats.tokens, err)
	}
	for _, limit := range []string{"bytes", "tokens"} {
		policy := genericPackageJSONPolicy()
		if limit == "bytes" {
			policy.maxBytes = len(payload) - 1
		} else {
			policy.maxTokens = stats.tokens - 1
		}
		for _, value := range []any{second, wire.Payload} {
			limited := &genericCanonicalBudget{policy: policy}
			requireGenericPackageFailureKind(t, limited.walk(reflect.ValueOf(value), 0), "package-limit")
		}
	}
	children := []*genericType{first.Definitions[0].Return.CheckType, first.Definitions[0].Return.ExtendsType,
		first.Definitions[0].Return.TrueType, first.Definitions[0].Return.FalseType}
	for _, child := range children {
		child.Kind = "changed"
	}
	if !reflect.DeepEqual(first.Definitions[0].Parameters[0].Type, second.Definitions[0].Return) {
		t.Fatal("conditional parameter and return shared child storage")
	}
	copied := genericPackagePayload(second)
	for _, child := range []*genericPackageTypeDTO{copied.Definitions[0].ReturnType.CheckType, copied.Definitions[0].ReturnType.ExtendsType,
		copied.Definitions[0].ReturnType.TrueType, copied.Definitions[0].ReturnType.FalseType} {
		child.Kind = "changed"
	}
	owned := genericPackageOwnedGraph(wire.Payload)
	for _, child := range []*genericPackageTypeDTO{wire.Payload.Definitions[0].ReturnType.CheckType, wire.Payload.Definitions[0].ReturnType.ExtendsType,
		wire.Payload.Definitions[0].ReturnType.TrueType, wire.Payload.Definitions[0].ReturnType.FalseType} {
		child.Kind = "changed"
	}
	clear(data)
	encoded, err := encodeGenericPackage(second)
	if err != nil {
		t.Fatal(err)
	}
	other, err := encodeGenericPackage(owned)
	if err != nil || !bytes.Equal(encoded, other) {
		t.Fatalf("protocol/domain or independent decodes shared conditional children: %v", err)
	}
	caps := genericPackageCapabilities()
	caps[0] = "changed"
	if genericPackageCapabilities()[0] != "conditional-generic-call" {
		t.Fatal("shared capability table escaped caller ownership")
	}
}

func TestOwnedGenericConditionalSignatureRejectsUnsupportedOfficialSource(t *testing.T) {
	t.Parallel()
	conditional := "T extends boolean ? boolean : number"
	nested := "T extends boolean ? boolean : T extends boolean ? boolean : number"
	different := "T extends boolean ? number : boolean"
	alias := "type Choice<T> = " + conditional + ";"
	for _, test := range []struct {
		name, source, rejected string
		last                   bool
	}{
		{"alias", "// 😀\n" + alias + "\nexport function select<T>(value: Choice<T>): Choice<T> { return value; }", alias, false},
		{"nested", strings.ReplaceAll(genericConditionalSignatureSource, conditional, nested), nested, false},
		{"different-branches", strings.ReplaceAll(genericConditionalSignatureSource, conditional, different), different, false},
		{"two-type-parameters", strings.Replace(genericConditionalSignatureSource, "select<T>", "select<T, U>", 1), "", false},
		{"two-value-parameters", strings.Replace(genericConditionalSignatureSource, "): T", ", other: number): T", 1), "", false},
		{"recursive-call-body", strings.Replace(genericConditionalSignatureSource, "return value;", "return select<T>(value);", 1), "", false},
		{"conditional-call-argument", "// 😀\nfunction identity<U>(value: U): U { return value; }\n" +
			strings.TrimPrefix(strings.Replace(genericConditionalSignatureSource, "return value;", "return identity<"+conditional+">(value);", 1), "// 😀\n"), conditional, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			program, environment := genericProgramForTest(test.source)
			if err := checkGenericDiagnostics(t.Context(), program, nil); err != nil {
				t.Fatalf("unsupported conditional fixture is not officially valid: %v", err)
			}
			data, err := exportGenericPackage(t.Context(), program, program.GetSourceFile("/original/main.ts"), "package:conditional-boundary", environment)
			requireGenericPackageFailureKind(t, err, "unsupported")
			rejected := test.rejected
			if rejected == "" {
				rejected = strings.TrimPrefix(test.source, "// 😀\n")
			}
			start := strings.Index(test.source, rejected)
			if test.last {
				start = strings.LastIndex(test.source, rejected)
			}
			if start < 0 {
				t.Fatal("rejected original conditional occurrence is missing")
			}
			end := start + len(rejected)
			var failure *genericFailure
			if !errors.As(err, &failure) || failure.Code != 0 || len(data) != 0 ||
				failure.Origin != (genericOrigin{Start: len(utf16.Encode([]rune(test.source[:start]))), End: len(utf16.Encode([]rune(test.source[:end])))}) {
				t.Fatalf("unsupported conditional source lost exact original UTF16 anchor or exposed bytes: %v", err)
			}
		})
	}
}

func TestOwnedGenericConditionalSignatureKeepsOfficialTypeFailures(t *testing.T) {
	t.Parallel()
	source := strings.Replace(genericConditionalSignatureSource, "return value;", "return false;", 1)
	program, environment := genericProgramForTest(source)
	data, err := exportGenericPackage(t.Context(), program, program.GetSourceFile("/original/main.ts"), "package:conditional-official-error", environment)
	var failure *genericFailure
	if !errors.As(err, &failure) || failure.Kind != "official-type" || failure.Code != 2322 || len(data) != 0 ||
		failure.Source.Path != "/original/main.ts" || failure.Source.Digest != genericTextDigest(source) ||
		failure.Source.Length != len(utf16.Encode([]rune(source))) || !reflect.DeepEqual(failure.Source.SurrogateStarts, []int{3}) ||
		failure.Origin.Start < 6 || failure.Origin.End <= failure.Origin.Start || failure.Origin.End > failure.Source.Length {
		t.Fatalf("conditional source lost its real official TS2322/source/UTF16 origin: %v", err)
	}
}

func TestOwnedGenericConditionalSignatureFreshCheckerIgnoresDiagnosticText(t *testing.T) {
	t.Parallel()
	g, err := decodeGenericPackage(genericConditionalSignatureColdBytes(t))
	if err != nil {
		t.Fatal(err)
	}
	d := genericDefinitionNamed(t, g, "select")
	for i, kind := range []string{"boolean", "number"} {
		request := g.request(d.Key, genericType{Kind: kind})
		normalTree, blankTree := genericTreeForTest(t, g, request), genericTreeForTest(t, g, request)
		if !reflect.DeepEqual(genericSyntaxFacts(normalTree), genericSyntaxFacts(blankTree)) {
			t.Fatal("conditional factory syntax changed before metamorphic checking")
		}
		conditionals := 0
		for _, fact := range genericSyntaxFacts(normalTree) {
			if fact.Kind == ast.KindConditionalType {
				conditionals++
			}
		}
		if conditionals != 3 {
			t.Fatal("factory or driver selected a conditional branch instead of retaining all three conditional syntax nodes")
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
		want := []string{"bool", "f64"}[i]
		if !reflect.DeepEqual(a, b) || len(ac) != 0 || len(bc) != 0 || a.Return != want ||
			!reflect.DeepEqual(a.Parameters, []string{want}) || a.Body.Kind != "parameter" || a.Body.Parameter != 0 ||
			a.Body.Type != want || a.Body.Origin != (genericOrigin{Start: 124, End: 129}) {
			t.Fatal("diagnostic carrier text changed fresh official conditional facts")
		}
	}
}
