// SPDX-License-Identifier: Apache-2.0
// Frozen independent facts6 bytes exercise the public IO boundary, not a CLI child.
// Both source variants must publish facts; Product alone decides borrow lifetime.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

func TestReadSpan26cV6PublicWholeFrozenPair(t *testing.T) {
	const path = "testdata/readspan-returns-v6/first-expected-facts.json"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 26697 || readSpanParameterPublicHash26b(raw) != "f993d9de204a720b4dbba4985f3a76533b47e4c1ed5355456907eb782f7f435a" {
		t.Fatal("complete independent pre-production facts6 fixture changed")
	}
	fixtureBefore := bytes.Clone(raw)
	t.Cleanup(func() {
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(after, fixtureBefore) || !bytes.Equal(raw, fixtureBefore) {
			t.Errorf("frozen fixture changed during public IO call: err=%v", err)
		}
	})
	var fixture struct {
		Cases []struct {
			Name        string `json:"name"`
			RequestText string `json:"requestText"`
			PackageText string `json:"packageText"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 2 {
		t.Fatal("frozen before/after cases must be complete")
	}
	for i, pin := range []struct{ name, requestHash, packageHash string }{
		{"before", "85650f89a5e220e8158c32fd586b222985f8ebcf9bef3afaa8cc76bacff7736a", "67fdfc86a9032b1601b8b540ac6c8b3132b5e871d93ade1f207c654684661621"},
		{"after", "d6662f6412c390dc7aa1808340f4ee2a9d1d765e25361a8b1aa0b3fe026b3556", "c34ff85000bc424e7499840ea44b8eb22cadf293616eab4e2c36086588abac0a"},
	} {
		t.Run(pin.name, func(t *testing.T) {
			item := fixture.Cases[i]
			request, expected := []byte(item.RequestText), []byte(item.PackageText)
			if item.Name != pin.name || len(request) != 1344 || len(expected) != 10517 ||
				readSpanParameterPublicHash26b(request) != pin.requestHash || readSpanParameterPublicHash26b(expected) != pin.packageHash {
				t.Fatal("exact independent request/package pair changed")
			}
			inputBefore := bytes.Clone(request)
			var output readSpanParameterPublicWriter26b
			// This is the existing IO handler directly. BASE5 must reject this exact
			// request as InputRED before a protocol-only migration reveals capabilityRED.
			err := runArrayFactsCandidate(t.Context(), bytes.NewReader(request), &output)
			t.Logf("TSFG_READSPAN_26C_PUBLIC_IO name=%s requestBytes=%d requestSHA256=%s expectedBytes=%d expectedSHA256=%s actualBytes=%d actualSHA256=%s writes=%d",
				pin.name, len(request), readSpanParameterPublicHash26b(request), len(expected), readSpanParameterPublicHash26b(expected),
				output.Len(), readSpanParameterPublicHash26b(output.Bytes()), output.writes)
			t.Logf("TSFG_READSPAN_26C_ACTUAL_PACKET name=%s bytes=%d sha256=%s base64=%s", pin.name, output.Len(), readSpanParameterPublicHash26b(output.Bytes()), base64.StdEncoding.EncodeToString(output.Bytes()))
			if !bytes.Equal(request, inputBefore) {
				t.Errorf("caller request mutated: %s", readSpanParameterPublicError26b(err))
			}
			if err != nil {
				if output.Len() != 0 || output.writes != 0 {
					t.Fatalf("failure published partial facts: writes=%d bytes=%d %s actualBase64=%s",
						output.writes, output.Len(), readSpanParameterPublicError26b(err), base64.StdEncoding.EncodeToString(output.Bytes()))
				}
				t.Fatalf("official-valid required-view source must publish the exact facts6 packet; IO rejection is not a CLI invocation or borrow decision: %s",
					readSpanParameterPublicError26b(err))
			}
			if output.writes != 1 {
				t.Fatalf("whole facts publication requires one complete write, got %d", output.writes)
			}
			if !bytes.Equal(output.Bytes(), expected) {
				t.Fatalf("whole output differs from frozen independent packet: actualBase64=%s", base64.StdEncoding.EncodeToString(output.Bytes()))
			}
		})
	}
}
