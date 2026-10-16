// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/json"
)

func canonicalIORawForTest(t *testing.T, data []byte) []byte {
	t.Helper()
	raw := json.Value(bytes.Clone(data))
	if err := raw.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestGenericPackageIORejectsFramingWithoutOutput(t *testing.T) {
	t.Parallel()
	r := genericIOExportInputForTest()
	good := genericIOCanonicalForTest(t, r)
	r.Family = "wrong"
	wrongFamily := genericIOCanonicalForTest(t, r)
	r = genericIOExportInputForTest()
	r.Operation = "other"
	wrongOperation := genericIOCanonicalForTest(t, r)
	r = genericIOExportInputForTest()
	r.Version = 2
	wrongVersion := genericIOCanonicalForTest(t, r)
	for name, data := range map[string][]byte{
		"family": wrongFamily, "operation": wrongOperation, "version": wrongVersion,
		"BOM": append([]byte{0xef, 0xbb, 0xbf}, good...), "LF": append(bytes.Clone(good), '\n'),
		"second document":  append(bytes.Clone(good), []byte("{}")...),
		"case":             canonicalIORawForTest(t, bytes.Replace(good, []byte(`"family":`), []byte(`"Family":`), 1)),
		"unknown":          canonicalIORawForTest(t, append([]byte(`{"unexpected":true,`), good[1:]...)),
		"missing":          bytes.Replace(good, []byte(`"scope":"package:wire-candidate",`), nil, 1),
		"duplicate":        append([]byte(`{"family":"duplicate",`), good[1:]...),
		"escape duplicate": append([]byte(`{"f\u0061mily":"duplicate",`), good[1:]...),
		"lone surrogate":   bytes.Replace(good, []byte("package:wire-candidate"), []byte(`\ud800`), 1),
		"unsafe integer":   bytes.Replace(good, []byte(`"version":1`), []byte(`"version":9007199254740992`), 1),
		"exponent":         bytes.Replace(good, []byte(`"version":1`), []byte(`"version":1e0`), 1),
		"negative":         bytes.Replace(good, []byte(`"version":1`), []byte(`"version":-1`), 1),
		"raw bytes":        bytes.Repeat([]byte(" "), genericPackageIOMaxBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			result, err := handleGenericPackageCandidate(t.Context(), data)
			var failure *genericFailure
			if !errors.As(err, &failure) || result != nil {
				t.Fatalf("invalid framing produced a response: %v", err)
			}
			var output bytes.Buffer
			if err := runGenericPackageCandidate(t.Context(), bytes.NewReader(data), &output); err == nil || output.Len() != 0 {
				t.Fatal("stream wrapper wrote output for invalid framing")
			}
		})
	}
}

func TestGenericPackageIOAssociatesPayloadFailures(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		mutate func(*genericPackageIOExportRequest)
	}{
		{"scope NUL", func(r *genericPackageIOExportRequest) { r.Scope = "bad\x00" }},
		{"source path", func(r *genericPackageIOExportRequest) { r.Source.Path = "relative.ts" }},
		{"source identity", func(r *genericPackageIOExportRequest) { r.Source.Digest = strings.Repeat("0", 64) }},
		{"empty source", func(r *genericPackageIOExportRequest) { r.Source.Content = ""; r.Source.Digest = genericTextDigest("") }},
		{"source UTF16", func(r *genericPackageIOExportRequest) {
			r.Source.Content = strings.Repeat("a", (1<<20)+1)
			r.Source.Digest = genericTextDigest(r.Source.Content)
		}},
		{"profile", func(r *genericPackageIOExportRequest) { r.Environment.Profile = "other" }},
		{"missing declaration", func(r *genericPackageIOExportRequest) { r.Environment.Files = []genericPackageDeclarationDTO{} }},
		{"environment NUL", func(r *genericPackageIOExportRequest) {
			r.Environment.Files[0].Content = "bad\x00"
			r.Environment.Files[0].Digest = genericTextDigest("bad\x00")
		}},
		{"environment identity", func(r *genericPackageIOExportRequest) { r.Environment.Files[0].Digest = "wrong" }},
		{"duplicate declaration", func(r *genericPackageIOExportRequest) {
			r.Environment.Files = append(r.Environment.Files, r.Environment.Files[0])
		}},
		{"source declaration collision", func(r *genericPackageIOExportRequest) { r.Environment.Files[0].Path = r.Source.Path }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			r := genericIOExportInputForTest()
			test.mutate(&r)
			response := genericIOInvokeForTest(t, genericIOCanonicalForTest(t, r), "error")
			d := genericIODiagnosticForTest(t, response)
			if d.Kind == "" || d.Code != 0 || d.Primary != nil {
				t.Fatal("unowned payload failure must retain classification and null primary")
			}
		})
	}
	good := genericIOCanonicalForTest(t, genericIOExportInputForTest())
	for _, bad := range [][]byte{
		bytes.Replace(good, []byte(`"scope":"package:wire-candidate"`), []byte(`"scope":null`), 1),
		canonicalIORawForTest(t, bytes.Replace(good, []byte(`"source":{`), []byte(`"source":{"unexpected":true,`), 1)),
	} {
		genericIOInvokeForTest(t, bad, "error")
	}
	_, _, packageBytes := genericIOProducedPackageForTest(t)
	g, err := decodeGenericPackage(packageBytes)
	if err != nil {
		t.Fatal(err)
	}
	wire := genericPackageWireForTest(t, packageBytes)
	wire.PackageID = "sha256:" + strings.Repeat("0", 64)
	badPackage, err := genericPackageCanonical(wire)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name     string
		pkg      json.Value
		roots    []genericPackageRoot
		kind     string
		anchored bool
	}{
		{"inner identity", badPackage, []genericPackageRoot{genericPackageRootForTest(t, g, "route", "boolean")}, "package-identity", false},
		{"null package", json.Value("null"), []genericPackageRoot{genericPackageRootForTest(t, g, "route", "boolean")}, "package-schema", false},
		{"empty roots", packageBytes, []genericPackageRoot{}, "package-limit", false},
		{"nonconcrete root", packageBytes, []genericPackageRoot{genericPackageRootForTest(t, g, "route", "parameter")}, "package-reference", true},
		{"root arity", packageBytes, []genericPackageRoot{genericPackageRootForTest(t, g, "route")}, "package-reference", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := genericIOCanonicalForTest(t, genericPackageIOInstantiateRequest{Family: genericPackageIOFamily, Version: 1, Operation: "instantiate", Package: test.pkg, Roots: test.roots})
			d := genericIODiagnosticForTest(t, genericIOInvokeForTest(t, request, "error"))
			if d.Kind != test.kind || d.Code != 0 {
				t.Fatalf("payload error was swallowed or misclassified: %+v", d)
			}
			if test.anchored {
				if d.Primary == nil || !reflect.DeepEqual(d.Primary.Source, genericPackageSource(g.Source)) || d.Primary.Origin != genericPackageOrigin(genericDefinitionNamed(t, g, "route").Origin) {
					t.Fatal("owned root failure lost its exact source/declaration origin")
				}
			} else if d.Primary != nil {
				t.Fatal("unowned package failure invented a source anchor")
			}
		})
	}
}

func TestGenericPackageIOUnknownNamesFollowEnvelopeBoundary(t *testing.T) {
	t.Parallel()
	good := genericIOCanonicalForTest(t, genericIOExportInputForTest())
	for _, name := range []string{"__proto__", "constructor", "prototype"} {
		top := canonicalIORawForTest(t, append([]byte("{\""+name+"\":false,"), good[1:]...))
		if response, err := handleGenericPackageCandidate(t.Context(), top); err == nil || response != nil {
			t.Fatal("unknown top name produced response")
		}
		nested := canonicalIORawForTest(t, bytes.Replace(good, []byte(`"source":{`), []byte(`"source":{`+"\""+name+"\":false,"), 1))
		d := genericIODiagnosticForTest(t, genericIOInvokeForTest(t, nested, "error"))
		if d.Kind != "package-schema" || d.Primary != nil {
			t.Fatal("nested unknown name was not a linked payload error")
		}
	}
}
