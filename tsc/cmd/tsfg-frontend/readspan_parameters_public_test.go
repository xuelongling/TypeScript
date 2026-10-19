// SPDX-License-Identifier: Apache-2.0
// Frozen independent facts5 bytes exercise the public IO boundary, not a CLI child.
// Both source variants must publish facts; Product alone decides borrow lifetime.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
)

type readSpanParameterPublicWriter26b struct {
	bytes.Buffer
	writes int
}

func (w *readSpanParameterPublicWriter26b) Write(data []byte) (int, error) {
	w.writes++
	return w.Buffer.Write(data)
}

func readSpanParameterPublicHash26b(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func readSpanParameterPublicError26b(err error) string {
	var failure *arrayFactsFailure
	if errors.As(err, &failure) {
		return fmt.Sprintf("type=%T code=%s kind=%s rule=%s phase=%s detail=%q diagnostics=%+v cause=%v error=%v",
			err, failure.Code, failure.Kind, failure.RuleID, failure.Phase, failure.Detail, failure.Diagnostics, failure.Cause, err)
	}
	return fmt.Sprintf("type=%T code=<unclassified> error=%v", err, err)
}

func TestReadSpan26bV5PublicWholeFrozenPair(t *testing.T) {
	const path = "testdata/readspan-parameters-v5/first-expected-facts.json"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 42525 || readSpanParameterPublicHash26b(raw) != "ea68589da3764b0ba2db32dd40feaad5cfd14d7ad177b28eeca58b195389fc67" {
		t.Fatal("complete independent pre-production facts5 fixture changed")
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
		{"before", "d8d54f5b164973c1201b7b967bec9876fb4419020a1bd2e00afd844fd5844698", "aa13e92406fdd678f6320865e8d2be4176b8a6de7650bbadb573fba8b76c71e7"},
		{"after", "dfe738186620a18cb2bb2025287b954e83d44b7e685895078e33dc51588aa054", "d18c50260681941d699d4387b5fd87e7455ce94d5b407323a8af221351f4da03"},
	} {
		t.Run(pin.name, func(t *testing.T) {
			item := fixture.Cases[i]
			request, expected := []byte(item.RequestText), []byte(item.PackageText)
			if item.Name != pin.name || len(request) != 1332 || len(expected) != 9593 ||
				readSpanParameterPublicHash26b(request) != pin.requestHash || readSpanParameterPublicHash26b(expected) != pin.packageHash {
				t.Fatal("exact independent request/package pair changed")
			}
			inputBefore := bytes.Clone(request)
			var output readSpanParameterPublicWriter26b
			// This is the existing IO handler directly. BASE4 must reject this exact
			// request as InputRED before a protocol-only migration reveals capabilityRED.
			err := runArrayFactsCandidate(t.Context(), bytes.NewReader(request), &output)
			t.Logf("TSFG_READSPAN_26B_PUBLIC_IO name=%s requestBytes=%d requestSHA256=%s expectedBytes=%d expectedSHA256=%s actualBytes=%d actualSHA256=%s writes=%d",
				pin.name, len(request), readSpanParameterPublicHash26b(request), len(expected), readSpanParameterPublicHash26b(expected),
				output.Len(), readSpanParameterPublicHash26b(output.Bytes()), output.writes)
			t.Logf("TSFG_READSPAN_26B_ACTUAL_PACKET name=%s bytes=%d sha256=%s base64=%s", pin.name, output.Len(), readSpanParameterPublicHash26b(output.Bytes()), base64.StdEncoding.EncodeToString(output.Bytes()))
			if !bytes.Equal(request, inputBefore) {
				t.Errorf("caller request mutated: %s", readSpanParameterPublicError26b(err))
			}
			if err != nil {
				if output.Len() != 0 || output.writes != 0 {
					t.Fatalf("failure published partial facts: writes=%d bytes=%d %s actualBase64=%s",
						output.writes, output.Len(), readSpanParameterPublicError26b(err), base64.StdEncoding.EncodeToString(output.Bytes()))
				}
				t.Fatalf("official-valid required-view source must publish the exact facts5 packet; IO rejection is not a CLI invocation or borrow decision: %s",
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

type readSpanParameterNoIO26b struct {
	reads, writes int
}

func (s *readSpanParameterNoIO26b) Read([]byte) (int, error) {
	s.reads++
	return 0, errors.New("unexpected mode-test input consumption")
}

func (s *readSpanParameterNoIO26b) Write([]byte) (int, error) {
	s.writes++
	return 0, errors.New("unexpected mode-test output emission")
}

func TestReadSpan26bV5ModeSelection(t *testing.T) {
	// The native main delegates to this command selector. This tests in-process
	// argv routing only; no main/os.Args/stdin/stdout/exit or child is exercised.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	t.Run("selected-v5", func(t *testing.T) {
		streams := &readSpanParameterNoIO26b{}
		err := runFrontendCommand(ctx, []string{"--array-facts-io-v5"}, streams, streams)
		var failure *arrayFactsFailure
		if !errors.As(err, &failure) || failure.Code != "r03-array-facts-canceled" || !errors.Is(err, context.Canceled) ||
			streams.reads != 0 || streams.writes != 0 {
			t.Fatalf("exact v5 mode must reach Array IO cancellation before streams: %s reads=%d writes=%d",
				readSpanParameterPublicError26b(err), streams.reads, streams.writes)
		}
	})
	t.Run("retired-v4", func(t *testing.T) {
		streams := &readSpanParameterNoIO26b{}
		err := runFrontendCommand(ctx, []string{"--array-facts-io-v4"}, streams, streams)
		if err == nil || err.Error() != "tsfg-frontend accepts one JSON request on stdin" || streams.reads != 0 || streams.writes != 0 {
			t.Fatalf("retired v4 mode must reject before context/IO: %s reads=%d writes=%d",
				readSpanParameterPublicError26b(err), streams.reads, streams.writes)
		}
	})
}
