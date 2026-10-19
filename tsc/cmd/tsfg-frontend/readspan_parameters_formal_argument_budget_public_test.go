// SPDX-License-Identifier: Apache-2.0
// Exact pre-producer official packets test the public boundary; Product owns work refusal.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

func TestReadSpan26bV6FormalArgumentBudgetPublic(t *testing.T) {
	const path = "testdata/readspan-parameters-v6/formal-argument-budget-expected-facts.json"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 503470 || readSpanParameterPublicHash26b(raw) != "09c8cd7e3db0c7e7a9fb67be3323056092b895521057dd22f3c23b9a996d80c3" {
		t.Fatal("independent formal/argument budget fixture changed")
	}
	frozen := bytes.Clone(raw)
	t.Cleanup(func() {
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(after, frozen) || !bytes.Equal(raw, frozen) {
			t.Errorf("frozen budget fixture changed: %v", err)
		}
	})
	var fixture struct {
		Cases []struct {
			Name        string
			RequestText string
			PackageText string
		}
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 2 {
		t.Fatal("exact inclusive and one-over pair required")
	}
	for i, item := range fixture.Cases {
		if item.Name != []string{"work1024", "work1025"}[i] {
			t.Fatal("frozen budget case order changed")
		}
		t.Run(item.Name, func(t *testing.T) {
			request, expected := []byte(item.RequestText), []byte(item.PackageText)
			before := bytes.Clone(request)
			var output readSpanParameterPublicWriter26b
			err := runArrayFactsCandidate(t.Context(), bytes.NewReader(request), &output)
			t.Logf("TSFG_READSPAN_26B_BUDGET_PUBLIC_IO name=%s requestBytes=%d requestSHA256=%s expectedBytes=%d expectedSHA256=%s actualBytes=%d actualSHA256=%s writes=%d", item.Name, len(request), readSpanParameterPublicHash26b(request), len(expected), readSpanParameterPublicHash26b(expected), output.Len(), readSpanParameterPublicHash26b(output.Bytes()), output.writes)
			t.Logf("TSFG_READSPAN_26B_BUDGET_ACTUAL_PACKET name=%s bytes=%d sha256=%s base64=%s", item.Name, output.Len(), readSpanParameterPublicHash26b(output.Bytes()), base64.StdEncoding.EncodeToString(output.Bytes()))
			if !bytes.Equal(request, before) {
				t.Error("public handler mutated budget request")
			}
			if err != nil {
				if output.Len() != 0 || output.writes != 0 {
					t.Fatalf("failure emitted partial packet: %s", readSpanParameterPublicError26b(err))
				}
				t.Fatalf("official-valid budget source must publish facts before Product work accounting: %s", readSpanParameterPublicError26b(err))
			}
			if output.writes != 1 || !bytes.Equal(output.Bytes(), expected) {
				t.Fatalf("full budget packet differs: writes=%d actualBase64=%s", output.writes, base64.StdEncoding.EncodeToString(output.Bytes()))
			}
		})
	}
}
