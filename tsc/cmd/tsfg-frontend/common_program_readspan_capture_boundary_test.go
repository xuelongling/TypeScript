// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestCommonProgramReadSpanCaptureBoundary26i(t *testing.T) {
	local, err := os.ReadFile("testdata/common-program-v2/readspan-capture/safe-lastuse.ts")
	if err != nil {
		t.Fatal(err)
	}
	parameter, err := os.ReadFile("testdata/common-program-v2/readspan-capture/parameter-safe.ts")
	if err != nil {
		t.Fatal(err)
	}
	for _, sample := range []struct {
		name    string
		source  []byte
		changes [][2]string
	}{
		{"uninitialized-local", local, [][2]string{{"const r: ReadSpan<i32> = readSpan(a, 0, 1);", "let r: ReadSpan<i32>; r = readSpan(a, 0, 1);"}}},
		{"array-local", local, [][2]string{{"const r: ReadSpan<i32> = readSpan(a, 0, 1);", "const r: i32[] = a;"}}},
		{"reference-local", local, [][2]string{{"const r: ReadSpan<i32> = readSpan(a, 0, 1);", "const r: Box = new Box();"}, {"return r[0];", "return r.n;"}}},
		{"value-local", local, [][2]string{{"const r: ReadSpan<i32> = readSpan(a, 0, 1);", "const r: Delta = new Delta();"}, {"return r[0];", "return r.n;"}}},
		{"function-local", local, [][2]string{{"const r: ReadSpan<i32> = readSpan(a, 0, 1);", "const r: Unary = leafLeft;"}, {"return r[0];", "return r(0);"}}},
		{"array-parameter", parameter, [][2]string{{"invokeCapturedSpan(r: ReadSpan<i32>)", "invokeCapturedSpan(r: i32[])"}, {"invokeCapturedSpan(readSpan(a, 0, 1))", "invokeCapturedSpan(a)"}}},
	} {
		t.Run(sample.name, func(t *testing.T) {
			source := string(sample.source)
			for _, change := range sample.changes {
				if strings.Count(source, change[0]) != 1 {
					t.Fatal("negative full-Program mutation is not unique")
				}
				source = strings.Replace(source, change[0], change[1], 1)
			}
			// These complete Programs remain TS-valid. The boundary is an explicit
			// producer capability, not a Source borrow verdict or language prohibition.
			commonMethodProgram37q(t, source)
			var output bytes.Buffer
			err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(readSpanCaptureRequest26i(t, []byte(source))), &output)
			var failure *arrayFactsFailure
			if !errors.As(err, &failure) || failure.Code != "r03-array-facts-capability" || output.Len() != 0 {
				t.Fatalf("bounded capture must reject atomically: %v/%d", err, output.Len())
			}
		})
	}
}
