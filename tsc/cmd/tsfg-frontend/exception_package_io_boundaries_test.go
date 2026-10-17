// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"testing"
)

type exceptionBoundaryReadFuncForTest func([]byte) (int, error)

func (f exceptionBoundaryReadFuncForTest) Read(data []byte) (int, error) { return f(data) }

func TestExceptionFactsCommandV1IOAndCancellationBoundaries(t *testing.T) {
	f := exceptionBoundaryFirstForTest(t)
	if !f.control(t) {
		return
	}
	readFailure := errors.New("independent exception read failure")
	writeFailure := errors.New("independent exception writer failure")
	for _, name := range []string{"preCanceled", "readError", "cancelDuringRead", "cancelWithReadError"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			reads := 0
			input := exceptionBoundaryReadFuncForTest(func(data []byte) (int, error) {
				reads++
				n := copy(data, f.requestBytes)
				if name == "cancelDuringRead" || name == "cancelWithReadError" {
					cancel()
				}
				if name == "readError" || name == "cancelWithReadError" {
					return n, readFailure
				}
				return n, io.EOF
			})
			if name == "preCanceled" {
				cancel()
			}
			output := &exceptionBoundaryWriterForTest{limit: -1}
			err := runFrontendCommand(ctx, []string{"--exception-facts-io-v1"}, input, output)
			want := error(context.Canceled)
			if name == "readError" {
				want = readFailure
			}
			t.Logf("TSFG_EXCEPTION_BOUNDARY_IO_V1 case=%s error=%v reads=%d writes=%d bytes=%d", name, err, reads, output.calls, len(output.accepted))
			if !errors.Is(err, want) || output.calls != 0 || len(output.accepted) != 0 {
				t.Fatalf("real read/cancel error must precede success bytes: got=%v want=%v", err, want)
			}
			if name == "preCanceled" && reads != 0 {
				t.Fatal("already canceled input was read")
			}
			if name != "preCanceled" && reads != 1 {
				t.Fatal("failing read was retried")
			}
			if name == "cancelWithReadError" && errors.Is(err, readFailure) {
				t.Fatal("one-shot cancellation precedence changed")
			}
		})
	}
	for _, tc := range []struct {
		name          string
		limit         int
		failure, want error
		cancel        bool
	}{
		{"shortWrite", len(f.expected) - 1, nil, io.ErrShortWrite, false},
		{"writerErrorZero", 0, writeFailure, writeFailure, false},
		{"writerErrorPartial", 7, writeFailure, writeFailure, false},
		{"cancelDuringWrite", -1, nil, context.Canceled, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			output := &exceptionBoundaryWriterForTest{limit: tc.limit, failure: tc.failure}
			if tc.cancel {
				output.cancel = cancel
			}
			before := bytes.Clone(f.requestBytes)
			err := runFrontendCommand(ctx, []string{"--exception-facts-io-v1"}, bytes.NewReader(f.requestBytes), output)
			// Preserve actual physical bytes of failed transport. A validated full
			// argument is offered once; partial/full receipt never becomes success.
			t.Logf("TSFG_EXCEPTION_BOUNDARY_FAILED_WRITE_V1 case=%s error=%v writes=%d acceptedBytes=%d acceptedBase64=%s", tc.name, err, output.calls, len(output.accepted), base64.StdEncoding.EncodeToString(output.accepted))
			if !errors.Is(err, tc.want) || output.calls != 1 || !bytes.Equal(output.offered, f.expected) {
				t.Fatalf("writer failure/retry/complete encoding differs: got=%v want=%v writes=%d", err, tc.want, output.calls)
			}
			count := tc.limit
			if count < 0 {
				count = len(f.expected)
			}
			if !bytes.Equal(output.accepted, f.expected[:count]) {
				t.Fatal("failed transport did not retain its exact physical prefix")
			}
			if !bytes.Equal(before, f.requestBytes) {
				t.Fatal("IO failure changed request bytes")
			}
		})
	}
}

func TestExceptionFactsCommandV1AnalysisFailureNoSuccessBytes(t *testing.T) {
	for _, tc := range []struct {
		name, source, code string
		syntax             bool
	}{
		{"syntax", "export function entry(value: i32): i32 { throw value;", "r02b-d-syntax", true},
		{"unresolved", "export function entry(value: i32): i32 { throw missingName; }", "r01-d-unresolved-name", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := integerRequest(tc.source)
			program := throwBoundaryProgramForTest(t, r)
			syntax := program.GetSyntacticDiagnostics(t.Context(), nil)
			if tc.syntax && len(syntax) == 0 || !tc.syntax && len(syntax) != 0 {
				t.Fatal("fresh official syntax precondition differs")
			}
			if !tc.syntax {
				found := false
				for _, diagnostic := range program.GetSemanticDiagnostics(t.Context(), nil) {
					found = found || diagnostic.Code() == 2304
				}
				if !found {
					t.Fatal("fresh official unresolved-name reason missing")
				}
			}
			public := nullableFactsBoundaryLegacy(t, r)
			if public.Status != "error" || len(public.Diagnostics) == 0 || public.Diagnostics[0].Code != tc.code {
				t.Fatalf("actual public diagnostic precondition differs: %+v", public.Diagnostics)
			}
			data := nullableFactsBoundaryCanonical(t, map[string]any{"family": "tsfg.exception-facts-request-candidate", "version": 1, "schemaHash": exceptionBoundarySchemaForTest, "input": r})
			exceptionBoundaryRejectForTest(t, data, "source", "fresh official/public diagnostic: "+tc.code)
		})
	}
}
