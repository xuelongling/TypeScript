// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"reflect"
	"testing"

	"github.com/microsoft/typescript-go/internal/json"
)

// Only package bytes escape this official-source scope. The old Program and
// its checker are discarded, and its original in-memory source is overwritten.
func genericPackageBytesForTest(t *testing.T) []byte {
	t.Helper()
	source := "/* wire 😀 origin */\nexport function identity<T>(value: T): T { return value; }\n" +
		"export function route<U>(input: U): U { return identity<U>(input); }\n" +
		"export function unused<V>(flag: boolean): boolean { return flag; }\n"
	program, environment := genericProgramForTest(source)
	data, err := exportGenericPackage(t.Context(), program, program.GetSourceFile("/original/main.ts"), "package:wire-candidate", environment)
	if err != nil {
		t.Fatal(err)
	}
	if err := program.Host().FS().WriteFile("/original/main.ts", ""); err != nil {
		t.Fatal(err)
	}
	if text, ok := program.Host().FS().ReadFile("/original/main.ts"); !ok || text != "" {
		t.Fatal("original source was not cleared")
	}
	return data
}

func genericPackageWireForTest(t *testing.T, data []byte) genericPackageDTO {
	t.Helper()
	var wire genericPackageDTO
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	return wire
}

func genericPackageSignedBytesForTest(t *testing.T, wire genericPackageDTO) []byte {
	t.Helper()
	wire.PackageID = "sha256:" + genericPackageIdentity(genericPackageDomain, wire.Payload)
	data, err := genericPackageCanonical(wire)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestOwnedGenericPackageRoundTripFreshInstances(t *testing.T) {
	t.Parallel()
	data := genericPackageBytesForTest(t)
	g, err := decodeGenericPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	again, err := encodeGenericPackage(g)
	if err != nil || !bytes.Equal(data, again) {
		t.Fatalf("canonical package roundtrip: %v", err)
	}
	if len(g.Source.SurrogateStarts) != 1 || g.Source.PositionUnit != "utf16" {
		t.Fatal("Unicode source boundary metadata lost")
	}
	route := genericDefinitionNamed(t, g, "route")
	identity := genericDefinitionNamed(t, g, "identity")
	unused := genericDefinitionNamed(t, g, "unused")
	boolean, number := genericType{Kind: "boolean"}, genericType{Kind: "number"}
	requests := []genericInstanceRequest{g.request(route.Key, boolean), g.request(route.Key, number),
		g.request(unused.Key, boolean), g.request(unused.Key, number), g.request(route.Key, boolean)}
	result, err := instantiateGenericGraph(t.Context(), g, requests)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Instances) != 6 || result.Roots[0] != result.Roots[4] || result.Roots[2] == result.Roots[3] {
		t.Fatal("package did not retain closed definitions or complete instance identities")
	}
	for i, want := range []string{"bool", "f64"} {
		fn := result.Instances[result.Roots[i]]
		idKey := genericInstanceKey(g.request(identity.Key, requests[i].TypeArguments...))
		if fn.Return != want || fn.Body.Type != want || fn.Body.Callee != idKey || result.Instances[idKey].Body.Type != want ||
			fn.Body.Origin != route.Body.Origin || !reflect.DeepEqual(fn.Source, g.Source) {
			t.Fatalf("decoded definitions did not drive official body projection: %+v", fn)
		}
	}
	wire := genericPackageWireForTest(t, data)
	if wire.PackageID != "sha256:"+g.Digest {
		t.Fatal("package and internal graph identity diverged")
	}
	t.Logf("TSFG_GENERIC_PACKAGE_FIXTURE %s", base64.StdEncoding.EncodeToString(data))
	t.Logf("TSFG_GENERIC_PACKAGE_EXPECTED packageId=%s instances=%d bool=%s f64=%s", wire.PackageID, len(result.Instances), result.Roots[0], result.Roots[1])
}

func TestOwnedGenericPackageDecodedBadTypeUsesOfficialDiagnostic(t *testing.T) {
	t.Parallel()
	g, err := decodeGenericPackage(genericPackageBytesForTest(t))
	if err != nil {
		t.Fatal(err)
	}
	bad := genericDefinitionNamed(t, g, "unused")
	for i := range g.Definitions {
		if g.Definitions[i].Key == bad.Key {
			g.Definitions[i].Return = genericType{Kind: "number"}
		}
	}
	g.Digest = genericGraphDigest(g)
	data, err := encodeGenericPackage(g)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeGenericPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	_, err = instantiateGenericGraph(t.Context(), decoded, []genericInstanceRequest{decoded.request(bad.Key, genericType{Kind: "boolean"})})
	var failure *genericFailure
	if !errors.As(err, &failure) || failure.Code != 2322 || failure.Kind != "official-type" || !reflect.DeepEqual(failure.Source, g.Source) ||
		failure.Origin.Start < bad.Origin.Start || failure.Origin.End > bad.Origin.End || failure.Origin.End <= failure.Origin.Start {
		t.Fatalf("decoded invalid body did not preserve official diagnostic: %v", err)
	}
}

func TestOwnedGenericPackageRejectsCorruptMetadataAndReferences(t *testing.T) {
	t.Parallel()
	data := genericPackageBytesForTest(t)
	for _, test := range []struct {
		name   string
		mutate func(*genericPackageDTO)
	}{
		{"family", func(w *genericPackageDTO) { w.Family = "other-family" }},
		{"old version", func(w *genericPackageDTO) { w.Version = 1 }},
		{"unknown version", func(w *genericPackageDTO) { w.Version = 999 }},
		{"schema", func(w *genericPackageDTO) { w.SchemaHash = "wrong" }},
		{"capability", func(w *genericPackageDTO) { w.Payload.Capabilities = []string{"cross-package"} }},
		{"semantics", func(w *genericPackageDTO) { w.Payload.SemanticProfile = "different" }},
		{"declaration profile", func(w *genericPackageDTO) { w.Payload.Environment.Profile = "different" }},
		{"content hash", func(w *genericPackageDTO) { w.Payload.Environment.Files[0].Digest = "bad" }},
		{"source hash", func(w *genericPackageDTO) { w.Payload.Source.Digest = "bad" }},
		{"source unit", func(w *genericPackageDTO) { w.Payload.Source.PositionUnit = "utf8" }},
		{"source range", func(w *genericPackageDTO) { w.Payload.Source.Length = 1 }},
		{"definition key", func(w *genericPackageDTO) { w.Payload.Definitions[0].Key = "bad" }},
		{"missing definition", func(w *genericPackageDTO) {
			for i, d := range w.Payload.Definitions {
				if d.Name == "identity" {
					w.Payload.Definitions = append(w.Payload.Definitions[:i], w.Payload.Definitions[i+1:]...)
					break
				}
			}
		}},
		{"foreign reference", func(w *genericPackageDTO) {
			for i, d := range w.Payload.Definitions {
				if d.Name == "route" {
					w.Payload.Definitions[i].Body.Definition = "foreign"
				}
			}
		}},
		{"type parameter index", func(w *genericPackageDTO) {
			w.Payload.Definitions[0].ReturnType = genericPackageTypeDTO{Kind: "parameter", Parameter: 16}
		}},
		{"parameter index", func(w *genericPackageDTO) {
			for i, d := range w.Payload.Definitions {
				if d.Name == "identity" {
					w.Payload.Definitions[i].Body.Parameter = 64
				}
			}
		}},
		{"duplicate type namespace", func(w *genericPackageDTO) { w.Payload.Definitions[0].TypeParameters = []string{"T", "T"} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			wire := genericPackageWireForTest(t, data)
			test.mutate(&wire)
			if _, err := decodeGenericPackage(genericPackageSignedBytesForTest(t, wire)); err == nil {
				t.Fatal("invalid owned package accepted")
			}
		})
	}
	wire := genericPackageWireForTest(t, data)
	wire.PackageID = "sha256:wrong"
	corrupt, err := genericPackageCanonical(wire)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeGenericPackage(corrupt); err == nil {
		t.Fatal("corrupt package hash accepted")
	}
}
