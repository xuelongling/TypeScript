// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/json"
)

const genericConditionalCallSource = "// 😀\nexport function leaf<U>(value: U extends boolean ? boolean : number): U extends boolean ? boolean : number { return value; }\n" +
	"export function route<T>(value: T extends boolean ? boolean : number): T extends boolean ? boolean : number { return leaf<T>(value); }"

// Only canonical bytes escape. The original Program and checker are not reused
// by the cold importer, and their actual VFS inputs are cleared first.
func genericConditionalCallColdBytes(t *testing.T) []byte {
	t.Helper()
	program, environment := genericProgramForTest(genericConditionalCallSource)
	if err := checkGenericDiagnostics(t.Context(), program, nil); err != nil {
		t.Fatalf("conditional-call fixture is not an officially valid Program: %v", err)
	}
	t.Log("TSFG_GENERIC_CONDITIONAL_CALL_OFFICIAL_DIAGNOSTICS_PASS")
	data, err := exportGenericPackage(t.Context(), program, program.GetSourceFile("/original/main.ts"),
		"package:conditional-call", environment)
	if err != nil {
		var failure *genericFailure
		if errors.As(err, &failure) && failure.Kind == "unsupported" && failure.Code == 0 &&
			failure.Detail == "conditional signature requires one type parameter and one directly returned parameter with the identical conditional type" &&
			failure.Origin == (genericOrigin{Start: 131, End: 265}) {
			t.Fatalf("TSFG_GENERIC_CONDITIONAL_CALL_CAPABILITY_RED public export rejected the real conditional call body at declaration UTF16 131..265: %v", err)
		}
		t.Fatalf("conditional-call export failed outside the expected semantic capability boundary: %v", err)
	}
	for _, file := range program.GetSourceFiles() {
		if err := program.Host().FS().WriteFile(file.FileName(), ""); err != nil {
			t.Fatal(err)
		}
		if text, ok := program.Host().FS().ReadFile(file.FileName()); !ok || text != "" {
			t.Fatal("original conditional-call Program input was not cleared")
		}
	}
	return data
}

// The oracle is one conditional call(p0) to a direct-return leaf: two requested
// scalar roots, four complete instances, and their real call/operand anchors.
// No body is run; conditional chains and recursion are outside this tracer.
func TestOwnedGenericConditionalCallColdInstances(t *testing.T) {
	t.Parallel()
	data := genericConditionalCallColdBytes(t)
	originalData := bytes.Clone(data)
	g, err := decodeGenericPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	again, err := encodeGenericPackage(g)
	if err != nil || !bytes.Equal(data, again) {
		t.Fatalf("conditional-call cold package roundtrip changed bytes: %v", err)
	}
	wire := genericPackageWireForTest(t, data)
	// The precise descriptor digest is independently pinned by the later schema
	// boundary test. These assertions follow export so version drift is not RED.
	schema, err := hex.DecodeString(wire.SchemaHash)
	if err != nil || len(schema) != 32 || wire.SchemaHash != strings.ToLower(wire.SchemaHash) ||
		wire.Family != "tsfg-owned-generic-package" || wire.Version != 5 ||
		wire.Payload.SemanticProfile != "tsfg-owned-scalar-generics-host-v5" ||
		g.Semantics != "tsfg-owned-scalar-generics-host-v5" {
		t.Fatal("conditional call was not versioned as a private5 owned package")
	}
	payload, err := genericPackageCanonical(wire.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if wire.PackageID != "sha256:"+genericTextDigest("tsfg-owned-generic-package/v5\n"+string(payload)) ||
		wire.PackageID != "sha256:"+g.Digest || g.Package != "package:conditional-call" ||
		g.Source.Path != "/original/main.ts" || g.Source.PositionUnit != "utf16" ||
		g.Source.Digest != "0f7d41e4184bea4af390fd1e4671be1eee5847fc4fa092e95a39dbce55297fda" ||
		g.Source.Length != 265 || g.Source.Length != len(utf16.Encode([]rune(genericConditionalCallSource))) ||
		!reflect.DeepEqual(g.Source.SurrogateStarts, []int{3}) || len(g.Definitions) != 2 {
		t.Fatal("conditional-call package lost its complete identity, source or UTF16 metadata")
	}
	if g.Environment.Profile != "minimal-scalars-v2" || len(g.Environment.Files) != 1 {
		t.Fatal("conditional-call declaration environment differs")
	}
	declaration := g.Environment.Files[0]
	if declaration.Path != "/original/minimal-scalars-v2.d.ts" || declaration.Content != scalarDeclarations ||
		declaration.Digest != genericTextDigest(scalarDeclarations) {
		t.Fatal("conditional-call package substituted its checked declaration input")
	}
	if bytes.Contains(data, []byte("export function")) || bytes.Contains(data, []byte("return leaf")) ||
		bytes.Contains(data, []byte("\"instances\"")) {
		t.Fatal("conditional-call package retained source text or precomputed instances")
	}
	conditional := genericType{Kind: "conditional", CheckType: &genericType{Kind: "parameter"},
		ExtendsType: &genericType{Kind: "boolean"}, TrueType: &genericType{Kind: "boolean"},
		FalseType: &genericType{Kind: "number"}}
	leaf := genericDefinitionNamed(t, g, "leaf")
	route := genericDefinitionNamed(t, g, "route")
	for _, test := range []struct {
		name, parameter string
		start, end      int
		paramStart      int
		paramEnd        int
		bodyStart       int
		bodyEnd         int
	}{
		{"leaf", "U", 6, 130, 30, 73, 122, 127},
		{"route", "T", 131, 265, 156, 199, 248, 262},
	} {
		d := genericDefinitionNamed(t, g, test.name)
		if !d.Exported || !reflect.DeepEqual(d.TypeParameters, []string{test.parameter}) || len(d.Parameters) != 1 ||
			!reflect.DeepEqual(d.Return, conditional) || !reflect.DeepEqual(d.Parameters[0].Type, conditional) ||
			d.Origin != (genericOrigin{Start: test.start, End: test.end}) || d.Parameters[0].Name != "value" ||
			d.Parameters[0].Origin != (genericOrigin{Start: test.paramStart, End: test.paramEnd}) ||
			d.Body.Origin != (genericOrigin{Start: test.bodyStart, End: test.bodyEnd}) {
			t.Fatalf("%s lost its bound conditional signature or original declaration/parameter/body anchors", test.name)
		}
		identity, err := genericPackageCanonical(genericPackageDefinitionIdentityDTO{
			Name: test.name, Origin: genericPackageOriginDTO{Start: test.start, End: test.end},
			Scope: "package:conditional-call", Source: genericPackageSource(g.Source)})
		if err != nil || d.Key != genericTextDigest("tsfg:generic-definition:v1\n"+string(identity)) {
			t.Fatalf("%s complete definition identity changed: %v", test.name, err)
		}
	}
	if leaf.Key == route.Key || leaf.Body.Kind != "parameter" || leaf.Body.Parameter != 0 || leaf.Body.Definition != "" ||
		len(leaf.Body.TypeArguments) != 0 || len(leaf.Body.Arguments) != 0 ||
		route.Body.Kind != "call" || route.Body.Parameter != 0 || route.Body.Definition != leaf.Key ||
		!reflect.DeepEqual(route.Body.TypeArguments, []genericType{{Kind: "parameter"}}) || len(route.Body.Arguments) != 1 {
		t.Fatal("owned conditional call did not retain the real target and caller type parameter")
	}
	argument := route.Body.Arguments[0]
	if argument.Kind != "parameter" || argument.Parameter != 0 || argument.Definition != "" ||
		len(argument.TypeArguments) != 0 || len(argument.Arguments) != 0 ||
		argument.Origin != (genericOrigin{Start: 256, End: 261}) {
		t.Fatal("owned conditional call replaced its original bound parameter0 argument")
	}
	requests := []genericPackageRoot{
		genericPackageRootForTest(t, g, "route", "boolean"), genericPackageRootForTest(t, g, "route", "number"),
	}
	requestBytes, err := genericPackageCanonical(requests)
	if err != nil {
		t.Fatal(err)
	}
	facts, err := instantiateGenericPackageFacts(t.Context(), data, requests)
	if err != nil {
		t.Fatal(err)
	}
	if facts.PackageID != wire.PackageID || facts.SemanticProfile != "tsfg-owned-scalar-generics-host-v5" ||
		len(facts.Roots) != 2 || len(facts.Instances) != 4 || facts.Roots[0] == facts.Roots[1] {
		t.Fatal("cold conditional calls lost two distinct roots or the exact four-instance closure")
	}
	for i, instance := range facts.Instances {
		if i > 0 && facts.Instances[i-1].Key >= instance.Key {
			t.Fatal("conditional-call instances are not strictly sorted by full key")
		}
	}
	keys := map[string]bool{}
	for i, scalar := range []string{"bool", "f64"} {
		var leafKey string
		for _, d := range []genericDefinition{leaf, route} {
			identity, err := genericPackageCanonical(genericPackageInstanceIdentityDTO{Definition: d.Key, GraphDigest: g.Digest,
				Scope: "package:conditional-call", SemanticProfile: "tsfg-owned-scalar-generics-host-v5", TypeArguments: requests[i].TypeArguments})
			if err != nil {
				t.Fatal(err)
			}
			key := genericTextDigest("tsfg:generic-instance:v1\n" + string(identity))
			instance := genericPackageFactForTest(t, facts, key)
			if keys[key] || instance.Key != key || instance.Definition != d.Key ||
				!reflect.DeepEqual(instance.TypeArguments, requests[i].TypeArguments) ||
				instance.Origin != genericPackageOrigin(d.Origin) || len(instance.Parameters) != 1 ||
				instance.Parameters[0].Name != "value" || instance.Parameters[0].Type != scalar || instance.ReturnType != scalar ||
				instance.Parameters[0].Origin != genericPackageOrigin(d.Parameters[0].Origin) ||
				instance.Body.Origin != genericPackageOrigin(d.Body.Origin) || instance.Body.Type != scalar || instance.Body.Parameter != 0 {
				t.Fatalf("fresh official %s/%s instance lost full identity, signature or original anchors", d.Name, scalar)
			}
			keys[key] = true
			if d.Name == "leaf" {
				leafKey = key
				if instance.Body.Kind != "parameter" || instance.Body.Callee != "" || len(instance.Body.Arguments) != 0 {
					t.Fatal("leaf instance no longer returns its original parameter0")
				}
			} else {
				if facts.Roots[i] != key || instance.Body.Kind != "call" || instance.Body.Callee != leafKey || len(instance.Body.Arguments) != 1 {
					t.Fatal("route instance calls the wrong full leaf identity or lost root order")
				}
				a := instance.Body.Arguments[0]
				if a.Kind != "parameter" || a.Parameter != 0 || a.Type != scalar || a.Callee != "" || len(a.Arguments) != 0 ||
					a.Origin != (genericPackageOriginDTO{Start: 256, End: 261}) {
					t.Fatal("official conditional call argument lost its type, original parameter binding or UTF16 anchor")
				}
			}
		}
	}
	if len(keys) != 4 {
		t.Fatal("conditional-call closure contains aliased instance identities")
	}
	fixture, err := genericPackageCanonical(struct {
		Requests []genericPackageRoot `json:"requests"`
		Result   genericPackageFacts  `json:"result"`
	}{Requests: requests, Result: facts})
	if err != nil {
		t.Fatal(err)
	}
	var cold struct {
		Requests []genericPackageRoot `json:"requests"`
		Result   genericPackageFacts  `json:"result"`
	}
	if err := json.Unmarshal(fixture, &cold); err != nil || !reflect.DeepEqual(cold.Requests, requests) || !reflect.DeepEqual(cold.Result, facts) {
		t.Fatalf("actual conditional-call requests/results did not survive cold persistence: %v", err)
	}
	requestAgain, err := genericPackageCanonical(requests)
	if err != nil || !bytes.Equal(requestBytes, requestAgain) || !bytes.Equal(originalData, data) {
		t.Fatal("cold conditional-call instantiation mutated its package or requests")
	}
	originals := []genericPackageIOSource{
		{Path: "/original/main.ts", Content: genericConditionalCallSource, Digest: genericTextDigest(genericConditionalCallSource)},
		{Path: declaration.Path, Content: declaration.Content, Digest: declaration.Digest},
	}
	// Emit only actual public-seam bytes after all assertions. The instance
	// record includes the exact requests and real official result together.
	t.Logf("TSFG_GENERIC_CONDITIONAL_CALL_PACKAGE_V5 %s", base64.StdEncoding.EncodeToString(data))
	t.Logf("TSFG_GENERIC_CONDITIONAL_CALL_INSTANCE_V5 %s", base64.StdEncoding.EncodeToString(fixture))
	for _, original := range originals {
		record, err := genericPackageCanonical(original)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("TSFG_GENERIC_CONDITIONAL_CALL_ORIGINAL_V5 %s", base64.StdEncoding.EncodeToString(record))
	}
}
