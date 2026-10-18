// SPDX-License-Identifier: Apache-2.0
// Frozen FIRST uses only BASE APIs. Missing v4 selection is an API gap, never semantic RED.
package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestNullableFactsCommandPresenceGuardColdPackages(t *testing.T) {
	fixtures := nullablePresenceFirstFixtures(t)
	prerequisites, ranges := true, 0
	// Finish every official and complete legacy control for all four cases before
	// the first public v4 invocation. A missing selector cannot hide later controls.
	for _, f := range fixtures {
		ok := t.Run("prerequisites/"+f.name, func(t *testing.T) {
			program := nullablePresenceFirstProgram(t, f.r)
			if !t.Run("officialDiagnostics", func(t *testing.T) {
				nullablePresenceFirstDiagnostics(t, program)
			}) {
				return
			}
			if !t.Run("officialTypesSymbolsExportsRanges", func(t *testing.T) {
				ranges += nullablePresenceFirstOfficial(t, program, f)
			}) {
				return
			}
			t.Run("legacyCompleteOwnedFacts", func(t *testing.T) {
				nullablePresenceFirstLegacy(t, f)
			})
		})
		prerequisites = prerequisites && ok
	}
	if !prerequisites {
		return
	}
	if ranges != 256 {
		t.Fatalf("complete official range gate: got=%d want=256", ranges)
	}
	t.Log("TSFG_20K_FIRST_PUBLIC_GATE four official diagnostic/type/binding/export/range and complete legacy controls passed")
	for _, f := range fixtures {
		t.Run("publicV4/"+f.name, func(t *testing.T) {
			input := bytes.Clone(f.requestBytes)
			before := bytes.Clone(input)
			reader := bytes.NewReader(input)
			var output bytes.Buffer
			t.Logf("TSFG_20K_FIRST_REQUEST_V4 %s %s", f.name, base64.StdEncoding.EncodeToString(input))
			err := runFrontendCommand(t.Context(), []string{"--nullable-facts-io-v4"}, reader, &output)
			if !bytes.Equal(input, before) {
				t.Fatal("public v4 command mutated original request bytes")
			}
			if err != nil {
				t.Logf("TSFG_20K_FIRST_PUBLIC_FAILURE %s err=%v outputBytes=%d unreadRequestBytes=%d originalRequestBytes=%d", f.name, err, output.Len(), reader.Len(), len(input))
				if err.Error() == "tsfg-frontend accepts one JSON request on stdin" {
					if output.Len() != 0 || reader.Len() != len(input) {
						t.Fatal("unknown selector must reject before input Read/output Write")
					}
					t.Log("TSFG_20K_FIRST_CLASSIFICATION PUBLIC_V4_SELECTOR/API_MECHANICAL_GAP; no guard semantic RED")
				}
				t.Fatalf("real public v4 command refused after all complete prerequisites: %v; rawOutput=%s", err, output.Bytes())
			}
			actual := bytes.Clone(output.Bytes())
			t.Logf("TSFG_20K_FIRST_PACKAGE_V4 %s %s", f.name, base64.StdEncoding.EncodeToString(actual))
			if !bytes.Equal(actual, f.packageBytes) {
				t.Fatalf("complete independent v4 packet differs: actualBytes=%d actualSHA=%s expectedBytes=%d expectedSHA=%s actual=%s expected=%s", len(actual), nullablePresenceFirstDigest(actual), len(f.packageBytes), nullablePresenceFirstDigest(f.packageBytes), actual, f.packageBytes)
			}
		})
	}
}
