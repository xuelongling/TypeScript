// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

// These bytes came from the pre-frozen independent fresh official Program
// oracle, before Source candidate implementation. Never update from its output.
//
//go:embed testdata/module-callable-facts-v1/expected/*.json testdata/module-callable-facts-v1/legacy-first-response.json
var mcPublicExpected23d embed.FS

func mcPublicFixture23d(t *testing.T, name string) (mcCase23d, []byte, []byte) {
	t.Helper()
	var corpus struct{ Cases []mcCase23d }
	if err := json.Unmarshal(moduleCallableScenarios23d, &corpus); err != nil {
		t.Fatal(err)
	}
	for _, tc := range corpus.Cases {
		if tc.Name == name {
			var requestObject mcObject23d
			if err := json.Unmarshal(tc.Request, &requestObject); err != nil {
				t.Fatal(err)
			}
			input := append(mcCanonical23d(t, requestObject), '\n')
			expected, err := mcPublicExpected23d.ReadFile("testdata/module-callable-facts-v1/expected/" + name + ".json")
			if err != nil {
				t.Fatal(err)
			}
			return tc, input, expected
		}
	}
	t.Fatalf("missing independently frozen Source case %s", name)
	return mcCase23d{}, nil, nil
}

type mcPublicWriter23d struct {
	bytes.Buffer
	writes  int
	failure error
}

func (w *mcPublicWriter23d) Write(data []byte) (int, error) {
	w.writes++
	if w.failure != nil {
		return 0, w.failure
	}
	return w.Buffer.Write(data)
}

func TestModuleCallableFactsPublicWholeCorpus(t *testing.T) {
	var corpus struct{ Cases []mcCase23d }
	if err := json.Unmarshal(moduleCallableScenarios23d, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 8 {
		t.Fatal("complete independently frozen corpus required")
	}
	for _, row := range corpus.Cases {
		t.Run(row.Name, func(t *testing.T) {
			_, input, expected := mcPublicFixture23d(t, row.Name)
			before := bytes.Clone(input)
			output := &mcPublicWriter23d{}
			if err := runFrontendCommand(t.Context(), []string{"--module-callable-facts-io-v2"}, bytes.NewReader(input), output); err != nil {
				t.Fatalf("ordinary official-checked module source must publish complete facts: %v", err)
			}
			if !bytes.Equal(input, before) {
				t.Fatal("public command mutated frozen request bytes")
			}
			t.Logf("TSFG_MODULE_CALLABLE_23D_PUBLIC_ACTUAL name=%s bytes=%d digest=%s base64=%s", row.Name, output.Len(), mcHash23d(output.Bytes()), base64.StdEncoding.EncodeToString(output.Bytes()))
			if output.writes != 1 || !bytes.Equal(output.Bytes(), expected) {
				t.Fatalf("complete independent facts differ for %s; writes=%d got=%s want=%s", row.Name, output.writes, mcHash23d(output.Bytes()), mcHash23d(expected))
			}
			t.Logf("TSFG_MODULE_CALLABLE_23D_PUBLIC_WHOLE_PASS name=%s bytes=%d digest=%s", row.Name, output.Len(), mcHash23d(expected))
		})
	}
}

func TestModuleCallableFactsDefaultWirePreserved(t *testing.T) {
	expected, err := mcPublicExpected23d.ReadFile("testdata/module-callable-facts-v1/legacy-first-response.json")
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runFrontendCommand(t.Context(), nil, bytes.NewBufferString(moduleCallableFirstInput23d), &output); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), expected) {
		t.Fatal("legacy default wire changed from the retained original causal FIRST response")
	}
}

func TestModuleCallableFactsCancellationAndSingleWrite(t *testing.T) {
	_, input, expected := mcPublicFixture23d(t, "first-rr")
	t.Run("cancel-before-read", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		output := &mcPublicWriter23d{}
		err := runFrontendCommand(ctx, []string{"--module-callable-facts-io-v2"}, bytes.NewReader(input), output)
		if !errors.Is(err, context.Canceled) || output.writes != 0 {
			t.Fatalf("cancelled request must produce no package: err=%v writes=%d", err, output.writes)
		}
	})
	t.Run("cancel-after-input", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		reader := &mcCancelReader23d{reader: bytes.NewReader(input), cancel: cancel}
		output := &mcPublicWriter23d{}
		err := runFrontendCommand(ctx, []string{"--module-callable-facts-io-v2"}, reader, output)
		if !errors.Is(err, context.Canceled) || output.writes != 0 {
			t.Fatalf("input-time cancellation must publish no partial package: err=%v writes=%d", err, output.writes)
		}
	})
	t.Run("terminal-writer-failure", func(t *testing.T) {
		sentinel := errors.New("independent terminal writer failure")
		output := &mcPublicWriter23d{failure: sentinel}
		err := runFrontendCommand(t.Context(), []string{"--module-callable-facts-io-v2"}, bytes.NewReader(input), output)
		if !errors.Is(err, sentinel) || output.writes != 1 || output.Len() != 0 {
			t.Fatalf("terminal write failure must be returned once: err=%v writes=%d", err, output.writes)
		}
	})
	t.Run("one-complete-write", func(t *testing.T) {
		output := &mcPublicWriter23d{}
		if err := runFrontendCommand(t.Context(), []string{"--module-callable-facts-io-v2"}, bytes.NewReader(input), output); err != nil {
			t.Fatal(err)
		}
		if output.writes != 1 || !bytes.Equal(output.Bytes(), expected) {
			t.Fatal("success must publish the independently frozen complete package in one write")
		}
	})
}

type mcCancelReader23d struct {
	reader *bytes.Reader
	cancel context.CancelFunc
}

func (r *mcCancelReader23d) Read(data []byte) (int, error) {
	n, err := r.reader.Read(data)
	r.cancel()
	return n, err
}

func TestModuleCallableFactsSelectorBeforeIO(t *testing.T) {
	for index, args := range [][]string{{"--module-callable-facts-io-v2", "extra"}, {"--module-callable-facts-io-v1"}, {"--module-callable-facts-io-v2=true"}, {" --module-callable-facts-io-v2"}} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			streams := &mcUntouchedIO23d{}
			if err := runFrontendCommand(t.Context(), args, streams, streams); err == nil || streams.reads != 0 || streams.writes != 0 {
				t.Fatalf("wrong selector must reject before I/O: err=%v streams=%+v", err, streams)
			}
		})
	}
}

type mcUntouchedIO23d struct{ reads, writes int }

func (s *mcUntouchedIO23d) Read([]byte) (int, error) {
	s.reads++
	return 0, errors.New("unexpected independent input read")
}
func (s *mcUntouchedIO23d) Write([]byte) (int, error) {
	s.writes++
	return 0, errors.New("unexpected independent output write")
}
