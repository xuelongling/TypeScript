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

const genericMappedSignatureSource = "// 😀\nexport function mapped<T>(value: { [K in 'value']: T }['value']): { [K in 'value']: T }['value'] { return value; }"

// Only canonical owned bytes leave this scope. The original Program's VFS is
// cleared before returning; neither a checker nor a concrete instance escapes.
func genericMappedSignatureColdBytes(t *testing.T) []byte {
	t.Helper()
	program, environment := genericProgramForTest(genericMappedSignatureSource)
	if err := checkGenericDiagnostics(t.Context(), program, nil); err != nil {
		t.Fatalf("mapped-signature fixture is not an officially valid Program: %v", err)
	}
	t.Log("TSFG_GENERIC_MAPPED_SIGNATURE_OFFICIAL_DIAGNOSTICS_PASS")
	data, err := exportGenericPackage(t.Context(), program, program.GetSourceFile("/original/main.ts"),
		"package:mapped-signature", environment)
	if err != nil {
		var failure *genericFailure
		if errors.As(err, &failure) && failure.Kind == "unsupported" && failure.Code == 0 &&
			failure.Detail == "type outside scalar/type-parameter slice" && failure.Origin == (genericOrigin{Start: 39, End: 69}) {
			t.Fatalf("TSFG_GENERIC_MAPPED_SIGNATURE_CAPABILITY_RED public export rejected actual mapped/indexed type at UTF16 39..69: %v", err)
		}
		t.Fatalf("mapped-signature public export failed outside the expected capability boundary: %v", err)
	}
	for _, file := range program.GetSourceFiles() {
		if err := program.Host().FS().WriteFile(file.FileName(), ""); err != nil {
			t.Fatal(err)
		}
		if text, ok := program.Host().FS().ReadFile(file.FileName()); !ok || text != "" {
			t.Fatal("original mapped-signature Program input was not cleared")
		}
	}
	return data
}

// This first one-key slice expects (bool)->bool and (f64)->f64. The
// package importer must obtain them from fresh official checkers after the
// original Program leaves scope, retaining the same real parameter return.
func TestOwnedGenericPackageMappedSignatureColdInstances(t *testing.T) {
	t.Parallel()
	data := genericMappedSignatureColdBytes(t)
	g, err := decodeGenericPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	again, err := encodeGenericPackage(g)
	if err != nil || !bytes.Equal(data, again) {
		t.Fatalf("mapped-signature package changed on cold canonical roundtrip: %v", err)
	}
	wire := genericPackageWireForTest(t, data)
	if wire.Family != "tsfg-owned-generic-package" || wire.Version != 7 ||
		wire.Payload.SemanticProfile != "tsfg-owned-scalar-generics-host-v7" ||
		wire.SchemaHash != "2e9d1e07016b6f8ce5f48bf8a3508491eaf1ce7e6fd43bf497c133f65c8dff94" ||
		!reflect.DeepEqual(wire.Payload.Capabilities, []string{"conditional-generic-call", "conditional-signature", "explicit-type-arguments", "mapped-indexed-generic-call", "mapped-indexed-signature", "number-add", "scalar-generics", "single-module"}) {
		t.Fatal("mapped signature was mixed into the predecessor package contract")
	}
	payload, err := genericPackageCanonical(wire.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if wire.PackageID != "sha256:"+genericTextDigest("tsfg-owned-generic-package/v7\n"+string(payload)) ||
		wire.PackageID != "sha256:"+g.Digest || g.Package != "package:mapped-signature" ||
		g.Semantics != "tsfg-owned-scalar-generics-host-v7" || g.Source.Path != "/original/main.ts" ||
		g.Source.Digest != "eae49419fbca1d64bc6c55bb871da4b0d6eb942bdcd8cf14dbb09da903d6c356" ||
		g.Source.Digest != genericTextDigest(genericMappedSignatureSource) || g.Source.PositionUnit != "utf16" ||
		g.Source.Length != 120 || g.Source.Length != len(utf16.Encode([]rune(genericMappedSignatureSource))) ||
		len(genericMappedSignatureSource) != 122 || !reflect.DeepEqual(g.Source.SurrogateStarts, []int{3}) || len(g.Definitions) != 1 {
		t.Fatal("mapped-signature package lost source, scope, v6 domain or complete identity")
	}
	if g.Environment.Profile != "minimal-scalars-v2" || len(g.Environment.Files) != 1 {
		t.Fatal("mapped-signature declaration environment differs")
	}
	declaration := g.Environment.Files[0]
	if declaration.Path != "/original/minimal-scalars-v2.d.ts" || declaration.Content != scalarDeclarations ||
		declaration.Digest != genericTextDigest(scalarDeclarations) {
		t.Fatal("mapped-signature package substituted its actual checked declaration input")
	}
	if bytes.Contains(data, []byte("return value")) || bytes.Contains(data, []byte("\"instances\"")) ||
		bytes.Contains(data, []byte("export function")) {
		t.Fatal("package retained original definition text or precomputed concrete instances")
	}
	d := genericDefinitionNamed(t, g, "mapped")
	if !d.Exported || !reflect.DeepEqual(d.TypeParameters, []string{"T"}) || len(d.Parameters) != 1 ||
		d.Return.Kind != "indexed" || !reflect.DeepEqual(d.Parameters[0].Type, d.Return) ||
		d.Origin != (genericOrigin{Start: 6, End: 120}) || d.Parameters[0].Name != "value" ||
		d.Parameters[0].Origin != (genericOrigin{Start: 32, End: 69}) ||
		d.Body.Kind != "parameter" || d.Body.Parameter != 0 || d.Body.Definition != "" ||
		len(d.Body.TypeArguments) != 0 || len(d.Body.Arguments) != 0 || d.Body.Origin != (genericOrigin{Start: 112, End: 117}) {
		t.Fatal("mapped signature, bound parameter return or original UTF16 anchors were dropped/replaced")
	}
	ownedType, err := genericPackageCanonical(wire.Payload.Definitions[0].ReturnType)
	if err != nil {
		t.Fatal(err)
	}
	const wantOwnedType = `{"indexed":{"index":{"kind":"string-literal","literals":["value"]},"object":{"kind":"mapped","mapped":{"binder":"K","keys":{"kind":"string-literal","literals":["value"]},"value":{"kind":"parameter","parameter":0}},"parameter":0}},"kind":"indexed","parameter":0}`
	if string(ownedType) != wantOwnedType ||
		!reflect.DeepEqual(wire.Payload.Definitions[0].Parameters[0].Type, wire.Payload.Definitions[0].ReturnType) {
		t.Fatalf("cold package dropped or replaced mapped binder, key domain, value binding or separate index syntax: %s", ownedType)
	}
	definitionIdentity, err := genericPackageCanonical(genericPackageDefinitionIdentityDTO{
		Name: "mapped", Origin: genericPackageOriginDTO{Start: 6, End: 120}, Scope: "package:mapped-signature",
		Source: genericPackageSource(g.Source)})
	if err != nil {
		t.Fatal(err)
	}
	if d.Key != genericTextDigest("tsfg:generic-definition:v1\n"+string(definitionIdentity)) {
		t.Fatal("mapped definition changed the complete v1 definition identity")
	}
	requests := []genericPackageRoot{
		genericPackageRootForTest(t, g, "mapped", "boolean"), genericPackageRootForTest(t, g, "mapped", "number"),
	}
	facts, err := instantiateGenericPackageFacts(t.Context(), data, requests)
	if err != nil {
		t.Fatal(err)
	}
	if facts.PackageID != wire.PackageID || facts.SemanticProfile != "tsfg-owned-scalar-generics-host-v7" ||
		len(facts.Roots) != 2 || len(facts.Instances) != 2 || facts.Roots[0] == facts.Roots[1] ||
		facts.Instances[0].Key >= facts.Instances[1].Key {
		t.Fatal("cold mapped requests lost full type arguments, distinct identity or sorted closure")
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
			instance.Origin != (genericPackageOriginDTO{Start: 6, End: 120}) || len(instance.Parameters) != 1 ||
			instance.Parameters[0].Name != "value" || instance.Parameters[0].Type != want ||
			instance.Parameters[0].Origin != (genericPackageOriginDTO{Start: 32, End: 69}) || instance.ReturnType != want ||
			instance.Body.Kind != "parameter" || instance.Body.Parameter != 0 || instance.Body.Type != want ||
			instance.Body.Callee != "" || len(instance.Body.Arguments) != 0 ||
			instance.Body.Origin != (genericPackageOriginDTO{Start: 112, End: 117}) {
			t.Fatalf("fresh official checker lost fixed %s signature, real parameter binding, anchors or complete instance identity: %+v", want, instance)
		}
	}
	factsBytes, err := genericPackageCanonical(facts)
	if err != nil {
		t.Fatal(err)
	}
	var coldFacts genericPackageFacts
	if err := json.Unmarshal(factsBytes, &coldFacts); err != nil || !reflect.DeepEqual(coldFacts, facts) {
		t.Fatalf("actual mapped instance DTO did not survive independent cold persistence: %v", err)
	}
	if strings.Contains(string(factsBytes), "export function") || bytes.Contains(factsBytes, []byte("\"files\"")) {
		t.Fatal("concrete mapped instance result imitated original source intake")
	}
	fixture, err := genericPackageCanonical(struct {
		Requests []genericPackageRoot `json:"requests"`
		Result   genericPackageFacts  `json:"result"`
	}{Requests: requests, Result: facts})
	if err != nil {
		t.Fatal(err)
	}
	originals := []genericPackageIOSource{
		{Path: "/original/main.ts", Content: genericMappedSignatureSource, Digest: genericTextDigest(genericMappedSignatureSource)},
		{Path: declaration.Path, Content: declaration.Content, Digest: declaration.Digest},
	}
	// Records describe actual checked bytes and facts only after every assertion.
	// No SourceEXE, VM or host-calculated runtime result is involved.
	t.Logf("TSFG_GENERIC_MAPPED_SIGNATURE_PACKAGE_V7 %s", base64.StdEncoding.EncodeToString(data))
	t.Logf("TSFG_GENERIC_MAPPED_SIGNATURE_INSTANCE_V7 %s", base64.StdEncoding.EncodeToString(fixture))
	for _, original := range originals {
		record, err := genericPackageCanonical(original)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("TSFG_GENERIC_MAPPED_SIGNATURE_ORIGINAL_V7 %s", base64.StdEncoding.EncodeToString(record))
	}
}
