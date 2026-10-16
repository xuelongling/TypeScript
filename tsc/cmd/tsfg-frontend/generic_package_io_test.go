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

func genericIOExportInputForTest() genericPackageIOExportRequest {
	source := "/* wire 😀 origin */\nexport function identity<T>(value: T): T { return value; }\n" +
		"export function route<U>(input: U): U { return identity<U>(input); }\n" +
		"export function unused<V>(flag: boolean): boolean { return flag; }\n"
	return genericPackageIOExportRequest{Family: genericPackageIOFamily, Version: 1, Operation: "export", Scope: "package:wire-candidate",
		Source: genericPackageIOSource{Path: "/original/main.ts", Content: source, Digest: genericTextDigest(source)},
		Environment: genericPackageEnvironmentDTO{Profile: genericDeclarationProfile, Files: []genericPackageDeclarationDTO{{
			Path: "/original/minimal-scalars-v2.d.ts", Content: scalarDeclarations, Digest: genericTextDigest(scalarDeclarations)}}}}
}

func genericIOCanonicalForTest(t *testing.T, value any) []byte {
	t.Helper()
	data, err := genericJSONCanonical(value, genericPackageIOPolicy())
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func genericIOFieldsForTest(t *testing.T, data []byte) map[string]json.Value {
	t.Helper()
	var fields map[string]json.Value
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	canonical := json.Value(bytes.Clone(data))
	if err := canonical.Canonicalize(); err != nil || !bytes.Equal(canonical, data) {
		t.Fatal("noncanonical handler output")
	}
	return fields
}

func genericIOInvokeForTest(t *testing.T, request []byte, status string) []byte {
	t.Helper()
	response, err := handleGenericPackageCandidate(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	fields := genericIOFieldsForTest(t, response)
	var gotStatus, gotID, operation string
	var requestFields map[string]json.Value
	if err := json.Unmarshal(request, &requestFields); err != nil {
		t.Fatalf("test request fields: %v", err)
	}
	if err := json.Unmarshal(requestFields["operation"], &operation); err != nil {
		t.Fatalf("test request operation: %v", err)
	}
	if err := json.Unmarshal(fields["status"], &gotStatus); err != nil {
		t.Fatalf("response status: %v", err)
	}
	if err := json.Unmarshal(fields["requestId"], &gotID); err != nil {
		t.Fatalf("response requestId: %v", err)
	}
	if gotStatus != status || gotID != genericPackageIORequestID(request) || !bytes.Equal(fields["operation"], requestFields["operation"]) ||
		string(fields["family"]) != "\""+genericPackageIOFamily+"\"" || string(fields["version"]) != "1" || len(fields) != 6 {
		t.Fatalf("handler response mismatch: operation=%q status=%q want=%q requestId=%q want=%q family=%s version=%s operationField=%s fieldCount=%d diagnostic=%s",
			operation, gotStatus, status, gotID, genericPackageIORequestID(request), fields["family"], fields["version"], fields["operation"], len(fields), fields["diagnostic"])
	}
	return response
}

func expectedGenericIOSource(path, text string) genericPackageSourceDTO {
	s := genericPackageSourceDTO{Path: path, Digest: genericTextDigest(text), PositionUnit: "utf16", Length: len(utf16.Encode([]rune(text))), SurrogateStarts: []int{}}
	position := 0
	for _, ch := range text {
		if ch > 0xffff {
			s.SurrogateStarts = append(s.SurrogateStarts, position)
			position++
		}
		position++
	}
	return s
}

func genericIODiagnosticForTest(t *testing.T, response []byte) genericPackageIODiagnostic {
	t.Helper()
	fields := genericIOFieldsForTest(t, response)
	var diagnostic genericPackageIODiagnostic
	if err := json.Unmarshal(fields["diagnostic"], &diagnostic); err != nil {
		t.Fatal(err)
	}
	return diagnostic
}

// This producer scope returns only request/response/package bytes. No Program,
// checker, AST, original-source VFS or precomputed instance escapes.
func genericIOProducedPackageForTest(t *testing.T) ([]byte, []byte, []byte) {
	t.Helper()
	request := genericIOCanonicalForTest(t, genericIOExportInputForTest())
	response := genericIOInvokeForTest(t, request, "ok")
	packageBytes := bytes.Clone(genericIOFieldsForTest(t, response)["value"])
	if genericTextDigest(string(packageBytes)) != "16feab4fa89d4baf5dea88d1f1c6e5aa11d56db4c07852d8264d9face05aba2c" {
		t.Fatal("production factory changed the real package source/environment")
	}
	return request, response, packageBytes
}

func TestGenericPackageIORealProducerConsumerFixture(t *testing.T) {
	t.Parallel()
	exportRequest, exportResponse, packageBytes := genericIOProducedPackageForTest(t)
	g, err := decodeGenericPackage(packageBytes)
	if err != nil {
		t.Fatal(err)
	}
	requests := genericPackageFactRequestsForTest(t, g)
	request := genericIOCanonicalForTest(t, genericPackageIOInstantiateRequest{Family: genericPackageIOFamily, Version: 1,
		Operation: "instantiate", Package: json.Value(packageBytes), Roots: requests})
	response := genericIOInvokeForTest(t, request, "ok")
	var facts genericPackageFacts
	if err := json.Unmarshal(genericIOFieldsForTest(t, response)["value"], &facts); err != nil {
		t.Fatal(err)
	}
	if len(facts.Instances) != 6 || facts.Roots[0] != facts.Roots[3] || facts.Roots[1] == facts.Roots[2] {
		t.Fatal("single-request import lost fresh closure, phantom identity or deduplication")
	}
	for _, test := range []struct {
		index int
		typ   string
	}{{0, "bool"}, {4, "f64"}} {
		instance := genericPackageFactForTest(t, facts, facts.Roots[test.index])
		if instance.ReturnType != test.typ || instance.Body.Type != test.typ || instance.Body.Kind != "call" ||
			genericPackageFactForTest(t, facts, instance.Body.Callee).ReturnType != test.typ {
			t.Fatal("fresh official body fact lost")
		}
	}
	type fixtureCase struct {
		Name     string     `json:"name"`
		Request  json.Value `json:"request"`
		Response json.Value `json:"response"`
	}
	cases := []fixtureCase{{Name: "export-ok", Request: exportRequest, Response: exportResponse}, {Name: "instantiate-ok", Request: request, Response: response}}
	for _, name := range []string{"official-error", "declaration-error", "source-eof-error", "declaration-eof-error"} {
		r := genericIOExportInputForTest()
		path, content := r.Source.Path, "/* bad 😀 */ export function broken<T>(value: T): T { return false; }"
		if name == "declaration-error" || name == "declaration-eof-error" {
			path, content = r.Environment.Files[0].Path, "/* declaration 😀 */ interface {"
			if name == "declaration-eof-error" {
				content = "/* declaration 😀 */ interface Missing {"
			}
			r.Environment.Files[0].Content, r.Environment.Files[0].Digest = content, genericTextDigest(content)
		} else {
			if name == "source-eof-error" {
				content = "/* source 😀 */ export function missing<T>(value: T): T { return value;"
			}
			r.Source.Content, r.Source.Digest = content, genericTextDigest(content)
		}
		request := genericIOCanonicalForTest(t, r)
		response := genericIOInvokeForTest(t, request, "error")
		d := genericIODiagnosticForTest(t, response)
		if d.Kind != "official-type" || d.Code == 0 || d.Primary == nil || !reflect.DeepEqual(d.Primary.Source, expectedGenericIOSource(path, content)) ||
			d.Primary.Origin.Start < 0 || d.Primary.Origin.End < d.Primary.Origin.Start || d.Primary.Origin.End > d.Primary.Source.Length {
			t.Fatalf("official declaration/source diagnostic lost identity or origin: %+v", d)
		}
		if strings.Contains(name, "eof") && (d.Code != 1005 || d.Primary.Origin.Start != d.Primary.Source.Length || d.Primary.Origin.End != d.Primary.Source.Length) {
			t.Fatalf("real EOF diagnostic was widened or moved: %+v", d)
		}
		if name == "declaration-error" && d.Primary.Origin.Start <= 0 {
			t.Fatal("declaration diagnostic lost its real location")
		}
		if name == "official-error" && (d.Code != 2322 || d.Primary.Origin.End <= d.Primary.Origin.Start) {
			t.Fatal("expected original TS2322 anchor")
		}
		cases = append(cases, fixtureCase{Name: name, Request: request, Response: response})
	}
	fixture := genericIOCanonicalForTest(t, struct {
		Cases []fixtureCase `json:"cases"`
	}{Cases: cases})
	t.Log("TSFG_GENERIC_IO_FIXTURE=" + base64.StdEncoding.EncodeToString(fixture))
}

func TestGenericPackageIOPreservesSourceAndEnvironment(t *testing.T) {
	t.Parallel()
	r := genericIOExportInputForTest()
	r.Source.Path = "/colon:segment/main.ts"
	r.Source.Content = "/*\x00" + strings.Repeat("界", 350000) + "*/ export function legal<T>(value: T): T { return value; }"
	r.Source.Digest = genericTextDigest(r.Source.Content)
	r.Environment.Files[0].Path = "/colon:segment/actual.d.ts"
	r.Environment.Files[0].Content = "/* actual source declaration bytes */\n" + r.Environment.Files[0].Content
	r.Environment.Files[0].Digest = genericTextDigest(r.Environment.Files[0].Content)
	if len(r.Source.Content) <= genericPackageMaxString {
		t.Fatal("regression source must exceed old UTF8 string cap")
	}
	response := genericIOInvokeForTest(t, genericIOCanonicalForTest(t, r), "ok")
	g, err := decodeGenericPackage(genericIOFieldsForTest(t, response)["value"])
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(genericPackageSource(g.Source), expectedGenericIOSource(r.Source.Path, r.Source.Content)) ||
		g.Environment.Files[0].Path != r.Environment.Files[0].Path || g.Environment.Files[0].Content != r.Environment.Files[0].Content ||
		g.Environment.Files[0].Digest != r.Environment.Files[0].Digest {
		t.Fatal("source controls/UTF16 or actual env was replaced")
	}
}

func TestGenericPackageIOGlobalDiagnosticHasNoInventedSource(t *testing.T) {
	t.Parallel()
	_, _, packageBytes := genericIOProducedPackageForTest(t)
	g, err := decodeGenericPackage(packageBytes)
	if err != nil {
		t.Fatal(err)
	}
	content := strings.ReplaceAll(g.Environment.Files[0].Content, "interface Array<T> {}", "")
	g.Environment.Files[0].Content = content
	g.Environment.Files[0].Digest = genericTextDigest(content)
	g.Digest = genericGraphDigest(g)
	packageBytes, err = encodeGenericPackage(g)
	if err != nil {
		t.Fatal(err)
	}
	input := genericIOCanonicalForTest(t, genericPackageIOInstantiateRequest{Family: genericPackageIOFamily, Version: 1, Operation: "instantiate", Package: packageBytes, Roots: []genericPackageRoot{genericPackageRootForTest(t, g, "route", "boolean")}})
	d := genericIODiagnosticForTest(t, genericIOInvokeForTest(t, input, "error"))
	if d.Kind != "official-type" || d.Code != 2318 || d.Primary != nil {
		t.Fatalf("global diagnostic acquired an invented definition source: %+v", d)
	}
}
