// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/json"
)

type frontendCommandUnusedIO struct {
	reads, writes int
}

func (s *frontendCommandUnusedIO) Read([]byte) (int, error) {
	s.reads++
	return 0, errors.New("unexpected input consumption")
}

func (s *frontendCommandUnusedIO) Write([]byte) (int, error) {
	s.writes++
	return 0, errors.New("unexpected output emission")
}

func TestFrontendCommandRejectsArgvBeforeIO(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{""}, {"unknown"}, {"--"}, {"--help"},
		{"--generic-package-io-v1=true"}, {"--generic-package-io-v2"},
		{"--GENERIC-PACKAGE-IO-V1"}, {" --generic-package-io-v1"},
		{"--generic-package-io-v1 "}, {"--generic-package-io-v1", "extra"},
		{"extra", "--generic-package-io-v1"},
		{"--generic-package-io-v1", "--generic-package-io-v1"},
	} {
		t.Run(strings.Join(args, "/"), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			streams := &frontendCommandUnusedIO{}
			err := runFrontendCommand(ctx, args, streams, streams)
			if err == nil || err.Error() != "tsfg-frontend accepts one JSON request on stdin" || streams.reads != 0 || streams.writes != 0 {
				t.Fatalf("argv must fail with unchanged usage before context/input/output: err=%v streams=%+v", err, streams)
			}
		})
	}
}

func frontendCommandLegacyInputForTest(t *testing.T, source string) []byte {
	t.Helper()
	var input bytes.Buffer
	if err := json.NewLegacyEncoder(&input).Encode(scalarRequest(source)); err != nil {
		t.Fatal(err)
	}
	return input.Bytes()
}

func TestFrontendCommandDefaultPreservesLegacyAnalysis(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{nil, {}} {
		// The legacy command consumes one JSON value and has no context policy.
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		input := append(frontendCommandLegacyInputForTest(t, "export function answer(): bool { return true; }"), []byte(" {}")...)
		var output bytes.Buffer
		if err := runFrontendCommand(ctx, args, bytes.NewReader(input), &output); err != nil {
			t.Fatal(err)
		}
		var result response
		if err := json.NewLegacyDecoder(&output).Decode(&result); err != nil {
			t.Fatal(err)
		}
		if result.Status != "ok" || len(result.Diagnostics) != 0 || len(result.Functions) != 1 {
			t.Fatalf("default command lost real legacy analysis: %+v", result)
		}
		fn := result.Functions[0]
		if fn.Name != "answer" || fn.ReturnType != "bool" || len(fn.Body) != 1 || fn.Body[0].Kind != "return" || fn.Body[0].Expression == nil || fn.Body[0].Expression.Boolean == nil || !*fn.Body[0].Expression.Boolean {
			t.Fatalf("default command changed owned boolean facts: %+v", fn)
		}
	}
}

func TestFrontendCommandDefaultPreservesLegacyDiagnostics(t *testing.T) {
	t.Parallel()
	source := "/* 😀 */ export function answer(): i32 { const v: i32 = true; return v; }"
	var output bytes.Buffer
	if err := runFrontendCommand(t.Context(), nil, bytes.NewReader(frontendCommandLegacyInputForTest(t, source)), &output); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(output.Bytes(), []byte("\n")) || bytes.Contains(output.Bytes(), []byte(`"family"`)) {
		t.Fatal("legacy newline/response envelope changed")
	}
	var result response
	if err := json.NewLegacyDecoder(&output).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "error" || len(result.Functions) != 0 || len(result.Diagnostics) != 1 {
		t.Fatalf("default command lost official failure: %+v", result)
	}
	d := result.Diagnostics[0]
	if d.UpstreamCode != 2322 || d.Code != "r01-d-scalar-type" || d.File != "/src/main.ts" || d.Primary.SourceID != d.File || sourceText(t, source, d.Range) != "v" {
		t.Fatalf("default command changed diagnostic identity or source range: %+v", d)
	}
}

func TestFrontendCommandDefaultPreservesLegacyDecodeError(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	err := runFrontendCommand(t.Context(), nil, strings.NewReader(`{"unexpected":true}`), &output)
	if err == nil || err.Error() != `json: unknown field "unexpected"` || output.Len() != 0 {
		t.Fatalf("default command changed legacy decode failure: err=%v output=%q", err, output.String())
	}
	input := genericIOCanonicalForTest(t, genericIOExportInputForTest())
	err = runFrontendCommand(t.Context(), nil, bytes.NewReader(input), &output)
	if err == nil || !strings.Contains(err.Error(), "unknown field") || output.Len() != 0 {
		t.Fatalf("default command unexpectedly auto-selected package mode: err=%v output=%q", err, output.String())
	}
}
