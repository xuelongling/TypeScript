// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/json"
)

const genericNumberAddSource = "// 😀\nexport function sum<T>(left: number, right: number): number { return left + right; }"

// Only canonical bytes leave this scope. No instance is requested until the
// original Program and checker leave scope and its VFS inputs are overwritten.
// Declaration text is intentionally owned by the package; original function
// text is never supplied to the fresh checker Host.
func genericNumberAddColdBytes(t *testing.T) []byte {
	t.Helper()
	program, environment := genericProgramForTest(genericNumberAddSource)
	if err := checkGenericDiagnostics(t.Context(), program, nil); err != nil {
		t.Fatalf("number-add fixture is not an officially valid Program: %v", err)
	}
	data, err := exportGenericPackage(t.Context(), program, program.GetSourceFile("/original/main.ts"),
		"package:number-add", environment)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range program.GetSourceFiles() {
		if err := program.Host().FS().WriteFile(file.FileName(), ""); err != nil {
			t.Fatal(err)
		}
		if text, ok := program.Host().FS().ReadFile(file.FileName()); !ok || text != "" {
			t.Fatal("original Program input was not cleared")
		}
	}
	return data
}

// Baseline public export must reject the actual binary at 75..87 after valid
// official diagnostics; neither compilation nor package version is that RED.
func TestOwnedGenericPackageNumberAddColdInstances(t *testing.T) {
	t.Parallel()
	data := genericNumberAddColdBytes(t)
	g, err := decodeGenericPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	again, err := encodeGenericPackage(g)
	if err != nil || !bytes.Equal(data, again) {
		t.Fatalf("number-add package changed on cold canonical roundtrip: %v", err)
	}
	wire := genericPackageWireForTest(t, data)
	if wire.Family != "tsfg-owned-generic-package" || wire.Version != 5 ||
		wire.Payload.SemanticProfile != "tsfg-owned-scalar-generics-host-v5" ||
		wire.SchemaHash != "da7eb72ae306eb88863a44c945df77d572a96443a005fefbd787267fe90658bf" ||
		!reflect.DeepEqual(wire.Payload.Capabilities, []string{"conditional-generic-call", "conditional-signature", "explicit-type-arguments", "mapped-indexed-signature", "number-add", "scalar-generics", "single-module"}) {
		t.Fatal("new number body was mixed into the old candidate package contract")
	}
	if g.Package != "package:number-add" || g.Semantics != "tsfg-owned-scalar-generics-host-v5" ||
		g.Source.Path != "/original/main.ts" || g.Source.Digest != genericTextDigest(genericNumberAddSource) ||
		g.Source.PositionUnit != "utf16" || g.Source.Length != 90 ||
		g.Source.Length != len(utf16.Encode([]rune(genericNumberAddSource))) ||
		!reflect.DeepEqual(g.Source.SurrogateStarts, []int{3}) || len(g.Definitions) != 1 ||
		wire.PackageID != "sha256:"+g.Digest {
		t.Fatal("number-add package lost its actual source, scope or complete identity")
	}
	if g.Environment.Profile != "minimal-scalars-v2" || len(g.Environment.Files) != 1 {
		t.Fatal("number-add declaration environment differs")
	}
	declaration := g.Environment.Files[0]
	if declaration.Path != "/original/minimal-scalars-v2.d.ts" || declaration.Content != scalarDeclarations ||
		declaration.Digest != genericTextDigest(scalarDeclarations) {
		t.Fatal("number-add package substituted its actual checked declaration input")
	}
	if bytes.Contains(data, []byte("return left + right")) || bytes.Contains(data, []byte("\"instances\"")) {
		t.Fatal("package retained original body source or precomputed concrete instances")
	}
	d := genericDefinitionNamed(t, g, "sum")
	if !d.Exported || !reflect.DeepEqual(d.TypeParameters, []string{"T"}) || len(d.Parameters) != 2 ||
		d.Return != (genericType{Kind: "number"}) || d.Origin != (genericOrigin{Start: 6, End: 90}) ||
		d.Body.Kind != "number-add" || d.Body.Parameter != 0 || d.Body.Definition != "" ||
		len(d.Body.TypeArguments) != 0 || len(d.Body.Arguments) != 2 || d.Body.Origin != (genericOrigin{Start: 75, End: 87}) {
		t.Fatal("actual phantom declaration or number-add body was dropped/replaced")
	}
	for i, want := range []struct {
		name             string
		declaration, use genericOrigin
	}{
		{"left", genericOrigin{Start: 29, End: 41}, genericOrigin{Start: 75, End: 79}},
		{"right", genericOrigin{Start: 43, End: 56}, genericOrigin{Start: 82, End: 87}},
	} {
		parameter, operand := d.Parameters[i], d.Body.Arguments[i]
		if parameter.Name != want.name || parameter.Type != (genericType{Kind: "number"}) || parameter.Origin != want.declaration ||
			operand.Kind != "parameter" || operand.Parameter != i || operand.Definition != "" ||
			len(operand.TypeArguments) != 0 || len(operand.Arguments) != 0 || operand.Origin != want.use {
			t.Fatalf("number-add operand order, official binding or UTF16 occurrence differs: %d", i)
		}
	}
	requests := []genericPackageRoot{
		genericPackageRootForTest(t, g, "sum", "boolean"), genericPackageRootForTest(t, g, "sum", "number"),
	}
	facts, err := instantiateGenericPackageFacts(t.Context(), data, requests)
	if err != nil {
		t.Fatal(err)
	}
	if facts.PackageID != wire.PackageID || facts.SemanticProfile != "tsfg-owned-scalar-generics-host-v5" ||
		len(facts.Roots) != 2 || len(facts.Instances) != 2 || facts.Roots[0] == facts.Roots[1] {
		t.Fatal("cold phantom requests lost full type arguments or merged distinct instances")
	}
	for i, request := range requests {
		instance := genericPackageFactForTest(t, facts, facts.Roots[i])
		identity, err := genericPackageCanonical(genericPackageInstanceIdentityDTO{Definition: d.Key, GraphDigest: g.Digest,
			Scope: g.Package, SemanticProfile: g.Semantics, TypeArguments: request.TypeArguments})
		if err != nil {
			t.Fatal(err)
		}
		if facts.Roots[i] != genericTextDigest("tsfg:generic-instance:v1\n"+string(identity)) ||
			instance.Definition != d.Key || !reflect.DeepEqual(instance.TypeArguments, request.TypeArguments) ||
			instance.Origin != genericPackageOrigin(d.Origin) || instance.ReturnType != "f64" || len(instance.Parameters) != 2 ||
			instance.Body.Kind != "number-add" || instance.Body.Type != "f64" || instance.Body.Callee != "" ||
			instance.Body.Parameter != 0 || len(instance.Body.Arguments) != 2 || instance.Body.Origin != genericPackageOrigin(d.Body.Origin) {
			t.Fatalf("fresh official mapper lost f64 add or full phantom identity: %+v", instance)
		}
		for j, operand := range instance.Body.Arguments {
			if instance.Parameters[j].Name != d.Parameters[j].Name || instance.Parameters[j].Type != "f64" ||
				instance.Parameters[j].Origin != genericPackageOrigin(d.Parameters[j].Origin) ||
				operand.Kind != "parameter" || operand.Parameter != j || operand.Type != "f64" || operand.Callee != "" ||
				len(operand.Arguments) != 0 || operand.Origin != genericPackageOrigin(d.Body.Arguments[j].Origin) {
				t.Fatalf("fresh official operand mapping changed order/type/anchor: instance=%d operand=%d", i, j)
			}
		}
	}
	factsBytes, err := genericPackageCanonical(facts)
	if err != nil {
		t.Fatal(err)
	}
	var coldFacts genericPackageFacts
	if err := json.Unmarshal(factsBytes, &coldFacts); err != nil || !reflect.DeepEqual(coldFacts, facts) {
		t.Fatalf("actual instance DTO did not survive independent cold persistence: %v", err)
	}
	if strings.Contains(string(factsBytes), "export function") || bytes.Contains(factsBytes, []byte("\"files\"")) {
		t.Fatal("concrete instance result imitated original source intake")
	}
	fixture, err := genericPackageCanonical(struct {
		Requests []genericPackageRoot `json:"requests"`
		Result   genericPackageFacts  `json:"result"`
	}{Requests: requests, Result: facts})
	if err != nil {
		t.Fatal(err)
	}
	originals := []genericPackageIOSource{
		{Path: "/original/main.ts", Content: genericNumberAddSource, Digest: genericTextDigest(genericNumberAddSource)},
		{Path: declaration.Path, Content: declaration.Content, Digest: declaration.Digest},
	}
	// All records follow every check. No product file is written or host numeric
	// result calculated; neither SourceEXE nor a VM is executed.
	t.Logf("TSFG_GENERIC_NUMBER_ADD_PACKAGE_V5 %s", base64.StdEncoding.EncodeToString(data))
	t.Logf("TSFG_GENERIC_NUMBER_ADD_INSTANCE_V5 %s", base64.StdEncoding.EncodeToString(fixture))
	for _, original := range originals {
		record, err := genericPackageCanonical(original)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("TSFG_GENERIC_NUMBER_ADD_ORIGINAL_V5 %s", base64.StdEncoding.EncodeToString(record))
	}
}
