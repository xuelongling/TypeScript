// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
)

type stringReaderProbe24 struct {
	input  io.Reader
	calls  int
	err    error
	cancel context.CancelFunc
}

func (r *stringReaderProbe24) Read(p []byte) (int, error) {
	r.calls++
	if r.err != nil {
		return 0, r.err
	}
	n, err := r.input.Read(p)
	if r.cancel != nil {
		r.cancel()
	}
	return n, err
}

type stringWriterProbe24 struct {
	bytes.Buffer
	calls  int
	err    error
	short  bool
	cancel context.CancelFunc
}

func (w *stringWriterProbe24) Write(p []byte) (int, error) {
	w.calls++
	if w.err != nil {
		return 0, w.err
	}
	if w.short {
		return len(p) - 1, nil
	}
	n, err := w.Buffer.Write(p)
	if w.cancel != nil {
		w.cancel()
	}
	return n, err
}

func TestStringFactsIOCancellationAndDispatch(t *testing.T) {
	input := stringBoundaryRequest24(t, "export function ioCase(): string { return 'x'; }")
	sentinel := errors.New("independent IO sentinel")
	t.Run("preCanceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		r := &stringReaderProbe24{input: bytes.NewReader(input)}
		w := &stringWriterProbe24{}
		err := runFrontendCommand(ctx, []string{"--string-facts-io-v1"}, r, w)
		if !errors.Is(err, context.Canceled) || r.calls != 0 || w.calls != 0 {
			t.Fatalf("pre-cancel custody: %v", err)
		}
	})
	t.Run("cancelDuringRead", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		r := &stringReaderProbe24{input: bytes.NewReader(input), cancel: cancel}
		w := &stringWriterProbe24{}
		err := runFrontendCommand(ctx, []string{"--string-facts-io-v1"}, r, w)
		if !errors.Is(err, context.Canceled) || w.calls != 0 {
			t.Fatalf("read cancellation: %v", err)
		}
	})
	t.Run("readFailure", func(t *testing.T) {
		r := &stringReaderProbe24{err: sentinel}
		w := &stringWriterProbe24{}
		err := runFrontendCommand(t.Context(), []string{"--string-facts-io-v1"}, r, w)
		if !errors.Is(err, sentinel) || w.calls != 0 {
			t.Fatal("read error identity/zero writes")
		}
	})
	t.Run("writeFailure", func(t *testing.T) {
		w := &stringWriterProbe24{err: sentinel}
		err := runFrontendCommand(t.Context(), []string{"--string-facts-io-v1"}, bytes.NewReader(input), w)
		if !errors.Is(err, sentinel) || w.calls != 1 {
			t.Fatal("write error identity/single write")
		}
	})
	t.Run("shortWrite", func(t *testing.T) {
		w := &stringWriterProbe24{short: true}
		err := runFrontendCommand(t.Context(), []string{"--string-facts-io-v1"}, bytes.NewReader(input), w)
		if !errors.Is(err, io.ErrShortWrite) || w.calls != 1 {
			t.Fatal("short write was retried or hidden")
		}
	})
	t.Run("cancelAfterSingleWrite", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		w := &stringWriterProbe24{cancel: cancel}
		err := runFrontendCommand(ctx, []string{"--string-facts-io-v1"}, bytes.NewReader(input), w)
		if !errors.Is(err, context.Canceled) || w.calls != 1 || w.Len() == 0 {
			t.Fatal("one-shot post-write cancellation")
		}
	})
	for _, args := range [][]string{{"--string-facts-io-v1", "extra"}, {"--string-facts-io-v3"}} {
		t.Run(args[0]+"invalidArgs", func(t *testing.T) {
			r := &stringReaderProbe24{input: bytes.NewReader(input)}
			w := &stringWriterProbe24{}
			if err := runFrontendCommand(t.Context(), args, r, w); err == nil || r.calls != 0 || w.calls != 0 {
				t.Fatal("argv selection touched IO")
			}
		})
	}
	for _, flag := range []string{"--exception-facts-io-v1", "--nullable-facts-io-v3", "--generic-package-io-v1"} {
		t.Run(flag, func(t *testing.T) {
			w := &stringWriterProbe24{}
			if err := runFrontendCommand(t.Context(), []string{flag}, bytes.NewReader(input), w); err == nil || w.calls != 0 {
				t.Fatal("old family accepted string envelope")
			}
		})
	}
	t.Run("legacyScalarStillOwned", func(t *testing.T) {
		r := scalarRequest("export function old(value: i32): i32 { return value; }")
		raw, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		if err = runFrontendCommand(t.Context(), nil, bytes.NewReader(raw), &output); err != nil {
			t.Fatal(err)
		}
		var result response
		if err = json.Unmarshal(output.Bytes(), &result); err != nil || result.Status != "ok" || len(result.Functions) != 1 || result.Functions[0].ReturnType != "i32" {
			t.Fatalf("legacy scalar regression: %v %+v", err, result)
		}
	})
}
