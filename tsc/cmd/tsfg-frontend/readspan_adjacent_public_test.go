// SPDX-License-Identifier: Apache-2.0
// Actual v4 Source packets must equal frozen official-only whole packets, not summaries.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

type readSpanAdjacentPublicWriter26 struct {
	bytes.Buffer
	writes int
}

func (w *readSpanAdjacentPublicWriter26) Write(data []byte) (int, error) {
	w.writes++
	return w.Buffer.Write(data)
}
func TestReadSpan26aAdjacentV4PublicWholeFrozenPackets(t *testing.T) {
	raw, err := os.ReadFile("testdata/readspan-last-use-v4/adjacent-official-packages.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Family, ContractSHA256 string
		Version                int
		Cases                  []struct {
			Name, RequestText, PackageText string
			ExpectedSourceError            *struct{ Code, RuleID, Phase string }
		}
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Family != "tsfg.readspan-adjacent-source-independent-oracle" || corpus.Version != 4 || corpus.ContractSHA256 != rs26FrozenPin || len(corpus.Cases) != 18 {
		t.Fatal("complete pre-production independent adjacent Source corpus")
	}
	legal, refusal := 0, 0
	names := map[string]bool{}
	for _, item := range corpus.Cases {
		if names[item.Name] {
			t.Fatal("duplicate frozen adjacent case")
		}
		names[item.Name] = true
		t.Run(item.Name, func(t *testing.T) {
			request := []byte(item.RequestText)
			before := bytes.Clone(request)
			var output readSpanAdjacentPublicWriter26
			if len(request) == 0 || item.ExpectedSourceError == nil && len(item.PackageText) == 0 {
				t.Fatal("complete frozen request and expected packet required")
			}
			err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v4"}, bytes.NewReader(request), &output)
			t.Logf("TSFG_READSPAN_26A_ADJACENT_PUBLIC_REQUEST name=%s bytes=%d sha256=%s base64=%s", item.Name, len(request), ao23Hash(request), base64.StdEncoding.EncodeToString(request))
			t.Logf("TSFG_READSPAN_26A_ADJACENT_PUBLIC_ACTUAL name=%s bytes=%d sha256=%s writes=%d base64=%s", item.Name, output.Len(), ao23Hash(output.Bytes()), output.writes, base64.StdEncoding.EncodeToString(output.Bytes()))
			if !bytes.Equal(request, before) {
				t.Fatal("caller input mutated")
			}
			if item.ExpectedSourceError != nil {
				refusal++
				if item.Name != "raw-calls-33" || item.PackageText != "" || err == nil || output.Len() != 0 || output.writes != 0 {
					t.Fatalf("official-valid raw33 must reject without packet/partial write: %v", err)
				}
				failure := arrayBoundaryFailure25(t, err)
				if failure.Code != item.ExpectedSourceError.Code || failure.RuleID != item.ExpectedSourceError.RuleID || failure.Phase != item.ExpectedSourceError.Phase {
					t.Fatalf("exact Source raw-call budget refusal differs: %+v", failure)
				}
				return
			}
			legal++
			if err != nil {
				if output.Len() != 0 || output.writes != 0 {
					t.Fatal("failed publication wrote partial packet")
				}
				t.Fatalf("official-valid bounded Source must publish complete v4 facts: %v", err)
			}
			if output.writes != 1 || !bytes.Equal(output.Bytes(), []byte(item.PackageText)) {
				t.Fatalf("whole pre-production independent packet differs or publication not atomic: writes=%d", output.writes)
			}
		})
	}
	if legal != 17 || refusal != 1 {
		t.Fatalf("all17 complete packets and raw33 refusal required: legal=%d refusal=%d", legal, refusal)
	}
}
