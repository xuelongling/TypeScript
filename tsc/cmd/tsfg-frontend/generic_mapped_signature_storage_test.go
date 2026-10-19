// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestOwnedGenericMappedSignatureRejectsMalformedColdTypes(t *testing.T) {
	t.Parallel()
	data := genericMappedSignatureColdBytes(t)
	for _, test := range []struct {
		name, kind string
		mutate     func(*genericPackageDefinitionDTO)
	}{
		{"missing-indexed", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.Indexed = nil }},
		{"missing-mapped", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.Indexed.Object.Mapped = nil }},
		{"nonzero-indexed", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.Parameter = 1 }},
		{"nonzero-mapped", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.Indexed.Object.Parameter = 1 }},
		{"wrong-object-kind", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.Indexed.Object.Kind = "parameter" }},
		{"standalone-mapped", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType = d.ReturnType.Indexed.Object }},
		{"extra-top-mapped", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.Mapped = &genericPackageMappedTypeDTO{} }},
		{"extra-object-indexed", "input", func(d *genericPackageDefinitionDTO) {
			d.ReturnType.Indexed.Object.Indexed = &genericPackageIndexedTypeDTO{}
		}},
		{"conditional-check-on-indexed", "input", func(d *genericPackageDefinitionDTO) {
			d.ReturnType.CheckType = &genericPackageTypeDTO{Kind: "parameter"}
		}},
		{"conditional-extends-on-indexed", "input", func(d *genericPackageDefinitionDTO) {
			d.ReturnType.ExtendsType = &genericPackageTypeDTO{Kind: "boolean"}
		}},
		{"conditional-true-on-indexed", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.TrueType = &genericPackageTypeDTO{Kind: "boolean"} }},
		{"conditional-false-on-indexed", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.FalseType = &genericPackageTypeDTO{Kind: "number"} }},
		{"conditional-child-on-mapped", "input", func(d *genericPackageDefinitionDTO) {
			d.ReturnType.Indexed.Object.CheckType = &genericPackageTypeDTO{Kind: "parameter"}
		}},
		{"hidden-scalar-wrapper", "input", func(d *genericPackageDefinitionDTO) { d.Parameters[0].Type.Kind = "boolean" }},
		{"hidden-value-mapped", "input", func(d *genericPackageDefinitionDTO) {
			d.ReturnType.Indexed.Object.Mapped.Value.Mapped = &genericPackageMappedTypeDTO{}
		}},
		{"hidden-value-indexed", "input", func(d *genericPackageDefinitionDTO) {
			d.ReturnType.Indexed.Object.Mapped.Value.Indexed = &genericPackageIndexedTypeDTO{}
		}},
		{"hidden-value-conditional", "input", func(d *genericPackageDefinitionDTO) {
			d.ReturnType.Indexed.Object.Mapped.Value.CheckType = &genericPackageTypeDTO{Kind: "number"}
		}},
		{"unbound-value", "package-reference", func(d *genericPackageDefinitionDTO) { d.ReturnType.Indexed.Object.Mapped.Value.Parameter = 1 }},
		{"inner-binder-value-role", "input", func(d *genericPackageDefinitionDTO) {
			d.ReturnType.Indexed.Object.Mapped.Value.Kind = "mapped-parameter"
		}},
		{"empty-binder", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.Indexed.Object.Mapped.Binder = "" }},
		{"keyword-binder", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.Indexed.Object.Mapped.Binder = "return" }},
		{"shadow-outer-binder", "unsupported", func(d *genericPackageDefinitionDTO) {
			d.ReturnType.Indexed.Object.Mapped.Binder = "T"
			d.Parameters[0].Type.Indexed.Object.Mapped.Binder = "T"
		}},
		{"unknown-key-kind", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.Indexed.Object.Mapped.Keys.Kind = "keyof" }},
		{"missing-domain-literals", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.Indexed.Object.Mapped.Keys.Literals = nil }},
		{"literal-two-items", "input", func(d *genericPackageDefinitionDTO) {
			d.ReturnType.Indexed.Object.Mapped.Keys.Literals = []string{"value", "other"}
		}},
		{"union-one-item", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.Indexed.Object.Mapped.Keys.Kind = "union" }},
		{"unknown-index-kind", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.Indexed.Index.Kind = "number" }},
		{"missing-index-literals", "input", func(d *genericPackageDefinitionDTO) { d.ReturnType.Indexed.Index.Literals = nil }},
		{"index-outside-domain", "package-reference", func(d *genericPackageDefinitionDTO) { d.ReturnType.Indexed.Index.Literals[0] = "other" }},
		{"different-parameter-signature", "unsupported", func(d *genericPackageDefinitionDTO) { d.Parameters[0].Type = genericPackageTypeDTO{Kind: "boolean"} }},
		{"two-type-parameters", "input", func(d *genericPackageDefinitionDTO) { d.TypeParameters = append(d.TypeParameters, "U") }},
		{"two-value-parameters", "unsupported", func(d *genericPackageDefinitionDTO) {
			p := d.Parameters[0]
			p.Name = "other"
			d.Parameters = append(d.Parameters, p)
		}},
		{"body-parameter1", "unsupported", func(d *genericPackageDefinitionDTO) { d.Body.Parameter = 1 }},
		{"body-call", "package-reference", func(d *genericPackageDefinitionDTO) { d.Body.Kind = "call"; d.Body.Definition = d.Key }},
		{"body-add", "unsupported", func(d *genericPackageDefinitionDTO) { d.Body.Kind = "number-add" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			wire := genericPackageWireForTest(t, data)
			test.mutate(&wire.Payload.Definitions[0])
			bad := genericPackageSignedBytesForTest(t, wire)
			graph, err := decodeGenericPackage(bad)
			requireGenericPackageFailureKind(t, err, test.kind)
			// Calls are admitted by private6; the unchanged zero-arity target is still rejected.
			if test.name == "body-call" {
				var failure *genericFailure
				if !errors.As(err, &failure) || failure.Code != 0 || failure.Detail != "dangling definition or call arity" {
					t.Fatalf("invalid mapped call lost its precise reference rejection: %v", err)
				}
			}
			if test.name == "shadow-outer-binder" {
				var failure *genericFailure
				if !errors.As(err, &failure) || failure.Detail != "mapped binder shadows outer type parameter" {
					t.Fatalf("matching signatures did not reject the actual cold shadow: %v", err)
				}
			}
			if !reflect.DeepEqual(graph, genericGraph{}) {
				t.Fatal("malformed mapped input exposed a partial graph")
			}
			facts, err := instantiateGenericPackageFacts(t.Context(), bad, []genericPackageRoot{{Definition: wire.Payload.Definitions[0].Key, TypeArguments: []genericPackageTypeDTO{{Kind: "boolean"}}}})
			requireGenericPackageFailureKind(t, err, test.kind)
			if !reflect.DeepEqual(facts, genericPackageFacts{}) {
				t.Fatal("malformed mapped input exposed partial facts")
			}
		})
	}
}

func TestOwnedGenericMappedSignatureRejectsNullExtraAndMixedClaims(t *testing.T) {
	t.Parallel()
	data := genericMappedSignatureColdBytes(t)
	wire := genericPackageWireForTest(t, data)
	indexed, err := genericPackageCanonical(wire.Payload.Definitions[0].ReturnType.Indexed)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ name, old, replacement, kind string }{
		{"null-indexed", `"indexed":` + string(indexed), `"indexed":null`, "package-schema"},
		{"null-mapped", `"mapped":{`, `"mapped":null,"discarded":{`, "package-schema"},
		{"null-keys", `"keys":{`, `"keys":null,"discarded":{`, "package-schema"},
		{"null-index", `"index":{`, `"index":null,"discarded":{`, "package-schema"},
		{"missing-index", `"index":{"kind":"string-literal","literals":["value"]},`, "", "input"},
		{"null-literals", `"literals":["value"]`, `"literals":null`, "package-schema"},
		{"null-value", `"value":{"kind":"parameter","parameter":0}`, `"value":null`, "package-schema"},
		{"missing-binder", `"binder":"K",`, "", "input"},
		{"missing-value", `,"value":{"kind":"parameter","parameter":0}`, "", "input"},
		{"missing-required-zero", `"kind":"indexed","parameter":0`, `"kind":"indexed"`, "package-canonical"},
		{"extra-indexed", `"indexed":{`, `"indexed":{"extra":true,`, "package-canonical"},
		{"extra-mapped", `"mapped":{`, `"mapped":{"extra":true,`, "package-canonical"},
		{"extra-key", `"keys":{`, `"keys":{"extra":true,`, "package-canonical"},
		{"duplicate-binder", `"binder":"K"`, `"binder":"K","binder":"K"`, "package-json"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			bad := bytes.Replace(data, []byte(test.old), []byte(test.replacement), 1)
			if bytes.Equal(data, bad) {
				t.Fatal("malformed mapped bytes were not changed")
			}
			_, err := decodeGenericPackage(bad)
			requireGenericPackageFailureKind(t, err, test.kind)
		})
	}
	for _, test := range []struct {
		name   string
		mutate func(*genericPackageDTO)
	}{
		{"version3", func(w *genericPackageDTO) { w.Version = 3 }},
		{"schema3", func(w *genericPackageDTO) {
			w.SchemaHash = "38fe7f94987ee409c7f86c6a38a148901cf79933aed1112286cc36d990da24ea"
		}},
		{"host3", func(w *genericPackageDTO) { w.Payload.SemanticProfile = "tsfg-owned-scalar-generics-host-v3" }},
		{"capabilities3", func(w *genericPackageDTO) {
			w.Payload.Capabilities = []string{"conditional-signature", "explicit-type-arguments", "number-add", "scalar-generics", "single-module"}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			w := genericPackageWireForTest(t, data)
			test.mutate(&w)
			_, err := decodeGenericPackage(genericPackageSignedBytesForTest(t, w))
			requireGenericPackageFailureKind(t, err, "package-schema")
		})
	}
	wire.PackageID = "sha256:" + genericPackageIdentity("tsfg-owned-generic-package/v3\n", wire.Payload)
	bad, err := genericPackageCanonical(wire)
	if err != nil {
		t.Fatal(err)
	}
	_, err = decodeGenericPackage(bad)
	requireGenericPackageFailureKind(t, err, "package-identity")
}

func TestOwnedGenericMappedSignatureRejectsRootAndCallWrappers(t *testing.T) {
	t.Parallel()
	data := genericMappedSignatureColdBytes(t)
	for _, name := range []string{"whole-indexed", "whole-mapped", "hidden-indexed", "hidden-mapped", "uniform-boolean-indexed"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			wire := genericPackageWireForTest(t, data)
			arg := wire.Payload.Definitions[0].ReturnType
			switch name {
			case "whole-mapped":
				arg = arg.Indexed.Object
			case "hidden-indexed":
				arg.Kind = "boolean"
			case "hidden-mapped":
				arg = arg.Indexed.Object
				arg.Kind = "number"
			case "uniform-boolean-indexed":
				arg.Indexed.Object.Mapped.Value = genericPackageTypeDTO{Kind: "boolean"}
			}
			root := genericPackageRoot{Definition: wire.Payload.Definitions[0].Key, TypeArguments: []genericPackageTypeDTO{arg}}
			facts, err := instantiateGenericPackageFacts(t.Context(), data, []genericPackageRoot{root})
			requireGenericPackageFailureKind(t, err, "package-reference")
			if !reflect.DeepEqual(facts, genericPackageFacts{}) {
				t.Fatal("root wrapper was erased or leaked partial facts")
			}
			g, err := decodeGenericPackage(data)
			if err != nil {
				t.Fatal(err)
			}
			_, err = instantiateGenericGraph(t.Context(), g, []genericInstanceRequest{g.request(root.Definition, genericPackageOwnedType(arg))})
			requireGenericPackageFailureKind(t, err, "unsupported")
			callData := genericMappedColdSourceForTest(t, "function identity<U>(x: U): U { return x; } export function route<T>(x: T): T { return identity<T>(x); }")
			callWire := genericPackageWireForTest(t, callData)
			for i := range callWire.Payload.Definitions {
				if callWire.Payload.Definitions[i].Name == "route" {
					callWire.Payload.Definitions[i].Body.TypeArguments[0] = arg
				}
			}
			_, err = decodeGenericPackage(genericPackageSignedBytesForTest(t, callWire))
			kind := "input"
			if arg.Kind == "indexed" {
				kind = "unsupported"
			}
			requireGenericPackageFailureKind(t, err, kind)
		})
	}
}

func TestOwnedGenericMappedSignatureOwnsWrappersAndCountsBudget(t *testing.T) {
	t.Parallel()
	data := genericMappedColdSourceForTest(t, genericMappedTypeSourceForTest("{ [K in 'z' | 'a' | 'z']: T }['z' | 'a' | 'z']"))
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
		t.Fatalf("complete wrapper/key bytes and tokens escaped the domain budget: bytes=%d/%d tokens=%d/%d error=%v", budget.bytes, len(payload), budget.tokens, stats.tokens, err)
	}
	for _, limit := range []string{"bytes", "tokens", "depth"} {
		policy := genericPackageJSONPolicy()
		switch limit {
		case "bytes":
			policy.maxBytes = len(payload) - 1
		case "tokens":
			policy.maxTokens = stats.tokens - 1
		case "depth":
			policy.maxDepth = stats.depth - 1
		}
		for _, value := range []any{second, wire.Payload} {
			limited := &genericCanonicalBudget{policy: policy}
			requireGenericPackageFailureKind(t, limited.walk(reflect.ValueOf(value), 0), "package-limit")
		}
	}
	first.Definitions[0].Return.Indexed.Object.Mapped.Keys.Literals[0] = "changed"
	if first.Definitions[0].Return.Indexed.Index.Literals[0] != "z" {
		t.Fatal("mapped domain and index shared a key array")
	}
	first.Definitions[0].Return.Indexed.Index.Literals[0] = "changed"
	first.Definitions[0].Return.Indexed.Object.Mapped.Value.Kind = "changed"
	first.Definitions[0].Return.Indexed.Object.Mapped.Binder = "changed"
	if !reflect.DeepEqual(first.Definitions[0].Parameters[0].Type, second.Definitions[0].Return) {
		t.Fatal("parameter and return shared nested mapped storage")
	}
	copied := genericPackagePayload(second)
	copied.Definitions[0].ReturnType.Indexed.Index.Literals[0] = "changed"
	copied.Definitions[0].ReturnType.Indexed.Object.Mapped.Keys.Literals[0] = "changed"
	copied.Definitions[0].ReturnType.Indexed.Object.Mapped.Value.Kind = "changed"
	owned := genericPackageOwnedGraph(wire.Payload)
	wire.Payload.Definitions[0].ReturnType.Indexed.Index.Literals[0] = "changed"
	wire.Payload.Definitions[0].ReturnType.Indexed.Object.Mapped.Keys.Literals[0] = "changed"
	wire.Payload.Definitions[0].ReturnType.Indexed.Object.Mapped.Value.Kind = "changed"
	before := bytes.Clone(data)
	clear(data)
	for _, graph := range []genericGraph{second, owned} {
		encoded, err := encodeGenericPackage(graph)
		if err != nil || !bytes.Equal(encoded, before) {
			t.Fatalf("independent decodes or domain/DTO conversion shared wrapper storage: %v", err)
		}
	}
	// A cyclic owned wrapper must hit the shared depth budget before any DTO
	// recursion or serialization; the public encoder returns a controlled error.
	second.Definitions[0].Return.Indexed.Object = second.Definitions[0].Return
	_, err = encodeGenericPackage(second)
	requireGenericPackageFailureKind(t, err, "package-limit")
}

func TestOwnedGenericMappedSignatureRejectsKeyBudgetsAndUnicode(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, keys, index, detail string }{
		{"domain-seventeen-duplicates", strings.Repeat("'value' | ", 16) + "'value'", "'value'", "key union members"},
		{"index-seventeen-duplicates", "'value'", strings.Repeat("'value' | ", 16) + "'value'", "key union members"},
		{"domain-257-bytes", "'" + strings.Repeat("x", 257) + "'", "'" + strings.Repeat("x", 257) + "'", "key bytes"},
		{"unicode-258-bytes", "'" + strings.Repeat("é", 129) + "'", "'" + strings.Repeat("é", 129) + "'", "key bytes"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			source := genericMappedTypeSourceForTest("{ [K in " + test.keys + "]: T }[" + test.index + "]")
			program, environment := genericProgramForTest(source)
			if err := checkGenericDiagnostics(t.Context(), program, nil); err != nil {
				t.Fatal(err)
			}
			_, err := exportGenericPackage(t.Context(), program, program.GetSourceFile("/original/main.ts"), "package:mapped-key-limit", environment)
			requireGenericPackageFailureKind(t, err, "package-limit")
			var failure *genericFailure
			if !errors.As(err, &failure) || failure.Detail != test.detail {
				t.Fatalf("wrong bounded key failure: %v", err)
			}
		})
	}
	data := genericMappedSignatureColdBytes(t)
	for _, role := range []string{"domain", "index"} {
		for _, invalid := range []string{"\x00", "\xff", strings.Repeat("x", 257)} {
			g, err := decodeGenericPackage(data)
			if err != nil {
				t.Fatal(err)
			}
			if role == "domain" {
				g.Definitions[0].Return.Indexed.Object.Mapped.Keys.Literals[0] = invalid
			} else {
				g.Definitions[0].Return.Indexed.Index.Literals[0] = invalid
			}
			if _, err := encodeGenericPackage(g); err == nil {
				t.Fatal("invalid owned key encoded")
			}
		}
		wire := genericPackageWireForTest(t, data)
		keys := &wire.Payload.Definitions[0].ReturnType.Indexed.Index
		if role == "domain" {
			keys = &wire.Payload.Definitions[0].ReturnType.Indexed.Object.Mapped.Keys
		}
		keys.Literals = []string{strings.Repeat("x", 257)}
		_, err := decodeGenericPackage(genericPackageSignedBytesForTest(t, wire))
		requireGenericPackageFailureKind(t, err, "package-limit")
		keys.Kind, keys.Literals = "union", strings.Split(strings.Repeat("value,", 16)+"value", ",")
		_, err = decodeGenericPackage(genericPackageSignedBytesForTest(t, wire))
		requireGenericPackageFailureKind(t, err, "package-limit")
	}
	for _, replacement := range []string{`"literals":["\ud800"]`, `"literals":["\udc00"]`, `"literals":["\u0000"]`} {
		bad := bytes.Replace(data, []byte(`"literals":["value"]`), []byte(replacement), 1)
		graph, err := decodeGenericPackage(bad)
		if err == nil || !reflect.DeepEqual(graph, genericGraph{}) {
			t.Fatal("invalid Unicode key exposed a graph")
		}
	}
}
