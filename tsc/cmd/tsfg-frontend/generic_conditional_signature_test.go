// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/json"
)

const genericConditionalSignatureSource = "// 😀\nexport function select<T>(value: T extends boolean ? boolean : number): T extends boolean ? boolean : number { return value; }"

// Only canonical owned bytes leave this scope. The original Program's VFS is
// cleared before returning; neither a checker nor a concrete instance escapes.
func genericConditionalSignatureColdBytes(t *testing.T) []byte {
	t.Helper()
	program, environment := genericProgramForTest(genericConditionalSignatureSource)
	if err := checkGenericDiagnostics(t.Context(), program, nil); err != nil {
		t.Fatalf("conditional-signature fixture is not an officially valid Program: %v", err)
	}
	t.Log("TSFG_GENERIC_CONDITIONAL_SIGNATURE_OFFICIAL_DIAGNOSTICS_PASS")
	data, err := exportGenericPackage(t.Context(), program, program.GetSourceFile("/original/main.ts"),
		"package:conditional-signature", environment)
	if err != nil {
		var failure *genericFailure
		if errors.As(err, &failure) && failure.Kind == "unsupported" && failure.Code == 0 &&
			failure.Detail == "type outside scalar/type-parameter slice" && failure.Origin == (genericOrigin{Start: 39, End: 75}) {
			t.Fatalf("TSFG_GENERIC_CONDITIONAL_SIGNATURE_CAPABILITY_RED public export rejected actual conditional type at UTF16 39..75: %v", err)
		}
		t.Fatalf("conditional-signature public export failed outside the expected capability boundary: %v", err)
	}
	for _, file := range program.GetSourceFiles() {
		if err := program.Host().FS().WriteFile(file.FileName(), ""); err != nil {
			t.Fatal(err)
		}
		if text, ok := program.Host().FS().ReadFile(file.FileName()); !ok || text != "" {
			t.Fatal("original conditional-signature Program input was not cleared")
		}
	}
	return data
}

// The independent expected signatures are (bool)->bool and (f64)->f64. The
// package importer must obtain them from fresh official checkers after the
// original Program leaves scope, retaining the same real parameter return.
func TestOwnedGenericPackageConditionalSignatureColdInstances(t *testing.T) {
	t.Parallel()
	data := genericConditionalSignatureColdBytes(t)
	g, err := decodeGenericPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	again, err := encodeGenericPackage(g)
	if err != nil || !bytes.Equal(data, again) {
		t.Fatalf("conditional-signature package changed on cold canonical roundtrip: %v", err)
	}
	wire := genericPackageWireForTest(t, data)
	if wire.Family != "tsfg-owned-generic-package" || wire.Version != 6 ||
		wire.Payload.SemanticProfile != "tsfg-owned-scalar-generics-host-v6" ||
		wire.SchemaHash != "c6b6baa76be25f6ac4f619867273208c9adb595030f12e7c54a200ed8c677789" {
		t.Fatal("conditional signature was mixed into the predecessor package contract")
	}
	payload, err := genericPackageCanonical(wire.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if wire.PackageID != "sha256:"+genericTextDigest("tsfg-owned-generic-package/v6\n"+string(payload)) ||
		wire.PackageID != "sha256:"+g.Digest || g.Package != "package:conditional-signature" ||
		g.Semantics != "tsfg-owned-scalar-generics-host-v6" || g.Source.Path != "/original/main.ts" ||
		g.Source.Digest != genericTextDigest(genericConditionalSignatureSource) || g.Source.PositionUnit != "utf16" ||
		g.Source.Length != 132 || g.Source.Length != len(utf16.Encode([]rune(genericConditionalSignatureSource))) ||
		!reflect.DeepEqual(g.Source.SurrogateStarts, []int{3}) || len(g.Definitions) != 1 {
		t.Fatal("conditional-signature package lost source, scope, v6 domain or complete identity")
	}
	if g.Environment.Profile != "minimal-scalars-v2" || len(g.Environment.Files) != 1 {
		t.Fatal("conditional-signature declaration environment differs")
	}
	declaration := g.Environment.Files[0]
	if declaration.Path != "/original/minimal-scalars-v2.d.ts" || declaration.Content != scalarDeclarations ||
		declaration.Digest != genericTextDigest(scalarDeclarations) {
		t.Fatal("conditional-signature package substituted its actual checked declaration input")
	}
	if bytes.Contains(data, []byte("return value")) || bytes.Contains(data, []byte("\"instances\"")) ||
		bytes.Contains(data, []byte("export function")) {
		t.Fatal("package retained original definition text or precomputed concrete instances")
	}
	d := genericDefinitionNamed(t, g, "select")
	if !d.Exported || !reflect.DeepEqual(d.TypeParameters, []string{"T"}) || len(d.Parameters) != 1 ||
		d.Return.Kind != "conditional" || !reflect.DeepEqual(d.Parameters[0].Type, d.Return) ||
		d.Origin != (genericOrigin{Start: 6, End: 132}) || d.Parameters[0].Name != "value" ||
		d.Parameters[0].Origin != (genericOrigin{Start: 32, End: 75}) ||
		d.Body.Kind != "parameter" || d.Body.Parameter != 0 || d.Body.Definition != "" ||
		len(d.Body.TypeArguments) != 0 || len(d.Body.Arguments) != 0 || d.Body.Origin != (genericOrigin{Start: 124, End: 129}) {
		t.Fatal("conditional signature, bound parameter return or original UTF16 anchors were dropped/replaced")
	}
	definitionIdentity, err := genericPackageCanonical(genericPackageDefinitionIdentityDTO{
		Name: "select", Origin: genericPackageOriginDTO{Start: 6, End: 132}, Scope: "package:conditional-signature",
		Source: genericPackageSource(g.Source)})
	if err != nil {
		t.Fatal(err)
	}
	if d.Key != genericTextDigest("tsfg:generic-definition:v1\n"+string(definitionIdentity)) {
		t.Fatal("conditional definition changed the complete v1 definition identity")
	}
	requests := []genericPackageRoot{
		genericPackageRootForTest(t, g, "select", "boolean"), genericPackageRootForTest(t, g, "select", "number"),
	}
	facts, err := instantiateGenericPackageFacts(t.Context(), data, requests)
	if err != nil {
		t.Fatal(err)
	}
	if facts.PackageID != wire.PackageID || facts.SemanticProfile != "tsfg-owned-scalar-generics-host-v6" ||
		len(facts.Roots) != 2 || len(facts.Instances) != 2 || facts.Roots[0] == facts.Roots[1] ||
		facts.Instances[0].Key >= facts.Instances[1].Key {
		t.Fatal("cold conditional requests lost full type arguments, distinct identity or sorted closure")
	}
	for i, want := range []string{"bool", "f64"} {
		request := requests[i]
		instance := genericPackageFactForTest(t, facts, facts.Roots[i])
		identity, err := genericPackageCanonical(genericPackageInstanceIdentityDTO{Definition: d.Key, GraphDigest: g.Digest,
			Scope: g.Package, SemanticProfile: g.Semantics, TypeArguments: request.TypeArguments})
		if err != nil {
			t.Fatal(err)
		}
		if facts.Roots[i] != genericTextDigest("tsfg:generic-instance:v1\n"+string(identity)) || instance.Key != facts.Roots[i] ||
			instance.Definition != d.Key || !reflect.DeepEqual(instance.TypeArguments, request.TypeArguments) ||
			instance.Origin != (genericPackageOriginDTO{Start: 6, End: 132}) || len(instance.Parameters) != 1 ||
			instance.Parameters[0].Name != "value" || instance.Parameters[0].Type != want ||
			instance.Parameters[0].Origin != (genericPackageOriginDTO{Start: 32, End: 75}) || instance.ReturnType != want ||
			instance.Body.Kind != "parameter" || instance.Body.Parameter != 0 || instance.Body.Type != want ||
			instance.Body.Callee != "" || len(instance.Body.Arguments) != 0 ||
			instance.Body.Origin != (genericPackageOriginDTO{Start: 124, End: 129}) {
			t.Fatalf("fresh official checker lost fixed %s signature, real parameter binding, anchors or complete instance identity: %+v", want, instance)
		}
	}
	factsBytes, err := genericPackageCanonical(facts)
	if err != nil {
		t.Fatal(err)
	}
	var coldFacts genericPackageFacts
	if err := json.Unmarshal(factsBytes, &coldFacts); err != nil || !reflect.DeepEqual(coldFacts, facts) {
		t.Fatalf("actual conditional instance DTO did not survive independent cold persistence: %v", err)
	}
	if strings.Contains(string(factsBytes), "export function") || bytes.Contains(factsBytes, []byte("\"files\"")) {
		t.Fatal("concrete conditional instance result imitated original source intake")
	}
	fixture, err := genericPackageCanonical(struct {
		Requests []genericPackageRoot `json:"requests"`
		Result   genericPackageFacts  `json:"result"`
	}{Requests: requests, Result: facts})
	if err != nil {
		t.Fatal(err)
	}
	originals := []genericPackageIOSource{
		{Path: "/original/main.ts", Content: genericConditionalSignatureSource, Digest: genericTextDigest(genericConditionalSignatureSource)},
		{Path: declaration.Path, Content: declaration.Content, Digest: declaration.Digest},
	}
	// Records describe actual checked bytes and facts only after every assertion.
	// No SourceEXE, VM or host-calculated runtime result is involved.
	t.Logf("TSFG_GENERIC_CONDITIONAL_SIGNATURE_PACKAGE_V6 %s", base64.StdEncoding.EncodeToString(data))
	t.Logf("TSFG_GENERIC_CONDITIONAL_SIGNATURE_INSTANCE_V6 %s", base64.StdEncoding.EncodeToString(fixture))
	for _, original := range originals {
		record, err := genericPackageCanonical(original)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("TSFG_GENERIC_CONDITIONAL_SIGNATURE_ORIGINAL_V6 %s", base64.StdEncoding.EncodeToString(record))
	}
}
