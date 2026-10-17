// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/json"
)

func TestOwnedGenericPackageCanonicalIdentityAndOrdering(t *testing.T) {
	t.Parallel()
	g, err := decodeGenericPackage(genericPackageBytesForTest(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/original/\ue000.d.ts", "/original/😀.d.ts"} {
		content := "// auxiliary declaration\n"
		g.Environment.Files = append(g.Environment.Files, genericDeclaration{Path: path, Digest: genericTextDigest(content), Content: content})
	}
	g.Digest = genericGraphDigest(g)
	before, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := encodeGenericPackage(g)
	if err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(g)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("encoding mutated its domain input")
	}
	slices.Reverse(g.Definitions)
	slices.Reverse(g.Environment.Files)
	if genericGraphDigest(g) != g.Digest {
		t.Fatal("unordered declaration/definition arrays affected identity")
	}
	second, err := encodeGenericPackage(g)
	if err != nil || !bytes.Equal(encoded, second) {
		t.Fatal("canonical sorting depends on internal array order")
	}
	wire := genericPackageWireForTest(t, encoded)
	payload, err := genericPackageCanonical(wire.Payload)
	if err != nil {
		t.Fatal(err)
	}
	budget := &genericCanonicalBudget{policy: genericPackageJSONPolicy()}
	if err := budget.walk(reflect.ValueOf(g), 0); err != nil || budget.bytes != len(payload) {
		t.Fatalf("domain and DTO canonical budget disagree: counted=%d actual=%d error=%v", budget.bytes, len(payload), err)
	}
	sum := sha256.Sum256(append([]byte("tsfg-owned-generic-package/v4\n"), payload...))
	if wire.PackageID != "sha256:"+hex.EncodeToString(sum[:]) {
		t.Fatal("package preimage/domain includes wrong bytes or self identity")
	}
	files := wire.Payload.Environment.Files
	if files[1].Path != "/original/😀.d.ts" || files[2].Path != "/original/\ue000.d.ts" {
		t.Fatal("declaration ordering used UTF8 instead of UTF16")
	}
}

func TestOwnedGenericPackageCanonicalStringsAndStrictShape(t *testing.T) {
	t.Parallel()
	chars := "<>&/\u2028\u2029\n\t\r\b\f\"\\"
	encoded, err := genericPackageCanonical(genericPackageDeclarationDTO{Content: chars, Digest: strings.Repeat("a", 64), Path: "/x.d.ts"})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"\\u003c", "\\u003e", "\\u0026", "\\u2028", "\\u2029", "\\/"} {
		if bytes.Contains(encoded, []byte(forbidden)) {
			t.Fatalf("non-JCS escaping: %s", encoded)
		}
	}
	if !bytes.Contains(encoded, []byte("<>&/\u2028\u2029")) || !bytes.Contains(encoded, []byte("\\n\\t\\r\\b\\f")) {
		t.Fatalf("canonical Unicode/control serialization: %s", encoded)
	}
	data := genericPackageBytesForTest(t)
	cases := map[string][]byte{
		"BOM":                    append([]byte{0xef, 0xbb, 0xbf}, data...),
		"whitespace":             append([]byte(" "), data...),
		"trailing value":         append(append([]byte{}, data...), []byte("{}")...),
		"unknown envelope":       bytes.Replace(data, []byte("{\"family\":"), []byte("{\"sessionId\":\"forbidden\",\"family\":"), 1),
		"unknown nested":         bytes.Replace(data, []byte("\"positionUnit\":\"utf16\""), []byte("\"positionUnit\":\"utf16\",\"sessionId\":\"forbidden\""), 1),
		"dangerous key":          bytes.Replace(data, []byte("{\"family\":"), []byte("{\"__proto__\":{},\"family\":"), 1),
		"field case":             bytes.Replace(data, []byte("\"family\":"), []byte("\"Family\":"), 1),
		"missing required empty": bytes.Replace(data, []byte("\"arguments\":[],"), nil, 1),
		"null array":             bytes.Replace(data, []byte("\"arguments\":[]"), []byte("\"arguments\":null"), 1),
		"duplicate key":          bytes.Replace(data, []byte("{\"family\":"), []byte("{\"family\":\"tsfg-owned-generic-package\",\"family\":"), 1),
		"escaped duplicate key":  bytes.Replace(data, []byte("{\"family\":"), []byte("{\"\\u0066amily\":\"tsfg-owned-generic-package\",\"family\":"), 1),
		"exponent":               bytes.Replace(data, []byte("\"version\":4"), []byte("\"version\":4e0"), 1),
		"negative zero":          bytes.Replace(data, []byte("\"parameter\":0"), []byte("\"parameter\":-0"), 1),
		"unsafe integer":         bytes.Replace(data, []byte("\"version\":4"), []byte("\"version\":9007199254740992"), 1),
		"alternate escape":       bytes.Replace(data, []byte("package:wire-candidate"), []byte("package:wire-candid\\u0061te"), 1),
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if bytes.Equal(bad, data) {
				t.Fatal("malformed-input test did not change bytes")
			}
			if _, err := decodeGenericPackage(bad); err == nil {
				t.Fatal("noncanonical or unknown package accepted")
			}
		})
	}
}

func TestOwnedGenericPackageDecodedValuesOwnStorage(t *testing.T) {
	t.Parallel()
	data := genericPackageBytesForTest(t)
	first, err := decodeGenericPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	second, err := decodeGenericPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	original := bytes.Clone(data)
	clear(data)
	recovered, err := encodeGenericPackage(second)
	if err != nil || !bytes.Equal(original, recovered) {
		t.Fatal("decode retained caller byte storage")
	}
	first.Source.SurrogateStarts[0]++
	first.Environment.Files[0].Content = "changed"
	first.Definitions[0].TypeParameters[0] = "changed"
	if second.Source.SurrogateStarts[0] == first.Source.SurrogateStarts[0] || second.Environment.Files[0].Content == "changed" || second.Definitions[0].TypeParameters[0] == "changed" {
		t.Fatal("separate decodes shared mutable data")
	}
	d := genericDefinitionNamed(t, second, "identity")
	result, err := instantiateGenericGraph(t.Context(), second, []genericInstanceRequest{second.request(d.Key, genericType{Kind: "boolean"})})
	if err != nil {
		t.Fatal(err)
	}
	second.Source.SurrogateStarts[0]++
	if result.Instances[result.Roots[0]].Source.SurrogateStarts[0] == second.Source.SurrogateStarts[0] {
		t.Fatal("projected instance retained source boundary storage")
	}
}
