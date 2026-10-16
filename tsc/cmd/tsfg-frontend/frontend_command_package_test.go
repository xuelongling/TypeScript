// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/microsoft/typescript-go/internal/json"
)

func frontendCommandPackageResponseForTest(t *testing.T, request []byte, operation, status string) map[string]json.Value {
	t.Helper()
	var output bytes.Buffer
	if err := runFrontendCommand(t.Context(), []string{"--generic-package-io-v1"}, bytes.NewReader(request), &output); err != nil {
		t.Fatalf("explicit package command failed: %v", err)
	}
	fields := genericIOFieldsForTest(t, output.Bytes())
	// The independent expectation uses the protocol's literal LF-terminated domain.
	preimage := append([]byte("tsfg:generic-package-io-request:v1\n"), request...)
	digest := sha256.Sum256(preimage)
	wantID := "sha256:" + hex.EncodeToString(digest[:])
	var gotID string
	if err := json.Unmarshal(fields["requestId"], &gotID); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 6 || string(fields["family"]) != `"tsfg.generic-package-io-candidate"` || string(fields["version"]) != "1" || string(fields["operation"]) != `"`+operation+`"` || string(fields["status"]) != `"`+status+`"` || gotID != wantID {
		t.Fatalf("package command changed response association: fields=%s requestId=%q want=%q", output.Bytes(), gotID, wantID)
	}
	return fields
}

// This scope retains only package bytes. No original request, Program, checker,
// AST or VFS is available to the subsequent command invocation.
func frontendCommandExportForTest(t *testing.T) []byte {
	t.Helper()
	input := genericIOCanonicalForTest(t, genericIOExportInputForTest())
	fields := frontendCommandPackageResponseForTest(t, input, "export", "ok")
	packageBytes := bytes.Clone(fields["value"])
	if genericTextDigest(string(packageBytes)) != "16feab4fa89d4baf5dea88d1f1c6e5aa11d56db4c07852d8264d9face05aba2c" {
		t.Fatal("command did not export the independently checked real package")
	}
	return packageBytes
}

func TestFrontendCommandGenericPackageExportAndFreshInstantiate(t *testing.T) {
	t.Parallel()
	packageBytes := frontendCommandExportForTest(t)
	g, err := decodeGenericPackage(packageBytes)
	if err != nil {
		t.Fatal(err)
	}
	roots := genericPackageFactRequestsForTest(t, g)
	input := genericIOCanonicalForTest(t, genericPackageIOInstantiateRequest{Family: genericPackageIOFamily, Version: 1,
		Operation: "instantiate", Package: packageBytes, Roots: roots})
	requestFields := genericIOFieldsForTest(t, input)
	if len(requestFields) != 5 || requestFields["source"] != nil || requestFields["files"] != nil || bytes.Contains(input, []byte("export function")) {
		t.Fatal("fresh import must carry only package and roots")
	}
	fields := frontendCommandPackageResponseForTest(t, input, "instantiate", "ok")
	var facts genericPackageFacts
	if err := json.Unmarshal(fields["value"], &facts); err != nil {
		t.Fatal(err)
	}
	if facts.PackageID != "sha256:0aa2f1e26d8f88707880513f49f62a3ffbbe0a18ba47e7a5be0ec6385bba8250" || facts.SemanticProfile != genericSemantics || len(facts.Roots) != 5 || len(facts.Instances) != 6 || facts.Roots[0] != facts.Roots[3] || facts.Roots[1] == facts.Roots[2] {
		t.Fatalf("fresh package command lost identity, closure, phantom args or duplicate roots: %+v", facts)
	}
	for _, test := range []struct {
		index int
		typ   string
	}{{0, "bool"}, {4, "f64"}} {
		route := genericPackageFactForTest(t, facts, facts.Roots[test.index])
		callee := genericPackageFactForTest(t, facts, route.Body.Callee)
		if route.ReturnType != test.typ || route.Parameters[0].Type != test.typ || route.Body.Kind != "call" || route.Body.Type != test.typ || callee.ReturnType != test.typ || callee.Body.Kind != "parameter" || callee.Body.Type != test.typ {
			t.Fatalf("command lost real fresh checker/body facts: %+v / %+v", route, callee)
		}
	}
}

func TestFrontendCommandGenericPackageRejectsFramingWithoutOutput(t *testing.T) {
	t.Parallel()
	valid := genericIOCanonicalForTest(t, genericIOExportInputForTest())
	for _, input := range [][]byte{
		nil, []byte(`{}`), append(bytes.Clone(valid), '\n'),
		append(bytes.Clone(valid), valid...),
		bytes.Replace(valid, []byte(`"operation":"export"`), []byte(`"operation":"other"`), 1),
		frontendCommandLegacyInputForTest(t, "export function answer(): bool { return true; }"),
	} {
		var output bytes.Buffer
		err := runFrontendCommand(t.Context(), []string{"--generic-package-io-v1"}, bytes.NewReader(input), &output)
		var failure *genericFailure
		if !errors.As(err, &failure) || failure.Kind == "" || output.Len() != 0 {
			t.Fatalf("package framing failure became usage, legacy fallback or output: err=%v output=%q", err, output.String())
		}
	}
}

func TestFrontendCommandGenericPackagePreservesLinkedDomainErrors(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, kind string
		code       int32
		anchored   bool
		mutate     func(*genericPackageIOExportRequest)
	}{
		{"bad digest", "package-identity", 0, false, func(r *genericPackageIOExportRequest) { r.Source.Digest = strings.Repeat("0", 64) }},
		{"official type", "official-type", 2322, true, func(r *genericPackageIOExportRequest) {
			r.Source.Content = "/* bad 😀 */ export function broken<T>(value: T): T { return false; }"
			r.Source.Digest = genericTextDigest(r.Source.Content)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			r := genericIOExportInputForTest()
			test.mutate(&r)
			input := genericIOCanonicalForTest(t, r)
			fields := frontendCommandPackageResponseForTest(t, input, "export", "error")
			var d genericPackageIODiagnostic
			if err := json.Unmarshal(fields["diagnostic"], &d); err != nil {
				t.Fatal(err)
			}
			if d.Kind != test.kind || d.Code != test.code || d.Detail == "" || (d.Primary != nil) != test.anchored || fields["value"] != nil {
				t.Fatalf("domain error lost classification/ownership: %+v", d)
			}
			if test.anchored && (!reflect.DeepEqual(d.Primary.Source, expectedGenericIOSource(r.Source.Path, r.Source.Content)) || d.Primary.Origin.Start < 0 || d.Primary.Origin.End <= d.Primary.Origin.Start || d.Primary.Origin.End > d.Primary.Source.Length) {
				t.Fatalf("official failure lost owned UTF16 source/range: %+v", d)
			}
		})
	}
}

func TestFrontendCommandGenericPackagePreservesContextAndIOErrors(t *testing.T) {
	t.Parallel()
	input := genericIOCanonicalForTest(t, genericIOExportInputForTest())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	deadline, stop := context.WithDeadline(t.Context(), time.Unix(0, 0))
	defer stop()
	for _, test := range []struct {
		ctx context.Context
		err error
	}{{ctx, context.Canceled}, {deadline, context.DeadlineExceeded}} {
		streams := &frontendCommandUnusedIO{}
		if err := runFrontendCommand(test.ctx, []string{"--generic-package-io-v1"}, streams, streams); err != test.err || streams.reads != 0 || streams.writes != 0 {
			t.Fatalf("candidate context error changed or accessed streams: err=%v streams=%+v", err, streams)
		}
	}
	sentinel := errors.New("owned command IO failure")
	var output bytes.Buffer
	if err := runFrontendCommand(t.Context(), []string{"--generic-package-io-v1"}, genericIOFailReader{err: sentinel}, &output); err != sentinel || output.Len() != 0 {
		t.Fatalf("candidate read error changed or emitted output: err=%v output=%q", err, output.String())
	}
	if err := runFrontendCommand(t.Context(), []string{"--generic-package-io-v1"}, bytes.NewReader(input), genericIOFailWriter{err: sentinel}); err != sentinel {
		t.Fatalf("candidate write error changed: %v", err)
	}
	if err := runFrontendCommand(t.Context(), []string{"--generic-package-io-v1"}, bytes.NewReader(input), genericIOFailWriter{short: true}); err != io.ErrShortWrite {
		t.Fatalf("candidate short write error changed: %v", err)
	}
}
