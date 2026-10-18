// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

// These probes model IO contracts only; no producer or official checker is replaced.
type stringConcatReadProbe24e struct {
	data             []byte
	calls, delivered int
	err              error
	cancel           context.CancelFunc
}

func (r *stringConcatReadProbe24e) Read(p []byte) (int, error) {
	r.calls++
	n := copy(p, r.data)
	r.data = r.data[n:]
	r.delivered += n
	if r.cancel != nil {
		r.cancel()
	}
	if r.err != nil {
		return n, r.err
	}
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

type stringConcatWriteProbe24e struct {
	bytes.Buffer
	calls, accept int
	partial       bool
	err           error
	cancel        context.CancelFunc
}

func (w *stringConcatWriteProbe24e) Write(p []byte) (int, error) {
	w.calls++
	n := len(p)
	if w.partial {
		n = w.accept
		if n > len(p) {
			n = len(p)
		}
	}
	_, _ = w.Buffer.Write(p[:n])
	if w.cancel != nil {
		w.cancel()
	}
	return n, w.err
}

func TestStringFactsV2IOCancellationAndDispatch(t *testing.T) {
	input := stringConcatOracleRead24e(t, "request.json", "5815ef980b77978c732239d56b0a1e6966183e0d8710de16d3b9448d19586028")
	expected := stringConcatOracleRead24e(t, "expected.json", "801792594c3f3b81d7d0a726bf99ea33772425f65abe3552236068d848c4498a")
	sentinel := errors.New("independent v2 IO sentinel")
	for _, expired := range []bool{false, true} {
		t.Run(map[bool]string{false: "preCanceled", true: "expiredDeadline"}[expired], func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			if expired {
				cancel()
				ctx, cancel = context.WithDeadline(t.Context(), time.Unix(0, 0))
			} else {
				cancel()
			}
			defer cancel()
			r := &stringConcatReadProbe24e{data: bytes.Clone(input)}
			w := &stringConcatWriteProbe24e{}
			err := runFrontendCommand(ctx, []string{"--string-facts-io-v2"}, r, w)
			want := context.Canceled
			if expired {
				want = context.DeadlineExceeded
			}
			if !errors.Is(err, want) || r.calls != 0 || w.calls != 0 {
				t.Fatalf("pre-read cancellation: %v reads=%d writes=%d", err, r.calls, w.calls)
			}
		})
	}
	for _, tc := range []struct {
		name    string
		data    []byte
		readErr error
		cancel  bool
	}{
		{"readFailure", nil, sentinel, false}, {"readBytesAndFailure", input[:17], sentinel, false}, {"cancelDuringRead", input, nil, true}, {"readFailureAndCancellation", input[:17], sentinel, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			r := &stringConcatReadProbe24e{data: bytes.Clone(tc.data), err: tc.readErr}
			if tc.cancel {
				r.cancel = cancel
			}
			w := &stringConcatWriteProbe24e{}
			err := runFrontendCommand(ctx, []string{"--string-facts-io-v2"}, r, w)
			var f *stringFactsFailure
			if !errors.As(err, &f) || w.calls != 0 || r.calls != 1 || tc.readErr != nil && !errors.Is(err, tc.readErr) || tc.cancel && !errors.Is(err, context.Canceled) {
				t.Fatalf("read causal/zero-publication boundary: %v reads=%d writes=%d", err, r.calls, w.calls)
			}
			want := "io"
			if tc.cancel {
				want = "canceled"
			}
			if f.Kind != want {
				t.Fatalf("read boundary kind: %+v", f)
			}
		})
	}
	for _, tc := range []struct {
		name     string
		partial  bool
		n        int
		writeErr error
		cancel   bool
		want     error
	}{
		{"writeFailure", true, 0, sentinel, false, sentinel},
		{"partialWriteFailure", true, 23, sentinel, false, sentinel},
		{"shortWrite", true, len(expected) - 1, nil, false, io.ErrShortWrite},
		{"cancelAfterFullWrite", false, 0, nil, true, context.Canceled},
		{"writeFailureAndCancel", true, 23, sentinel, true, sentinel},
		{"shortWriteAndCancel", true, 23, nil, true, io.ErrShortWrite},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			w := &stringConcatWriteProbe24e{partial: tc.partial, accept: tc.n, err: tc.writeErr}
			if tc.cancel {
				w.cancel = cancel
			}
			before := bytes.Clone(input)
			err := runFrontendCommand(ctx, []string{"--string-facts-io-v2"}, bytes.NewReader(input), w)
			n := len(expected)
			if tc.partial {
				n = tc.n
			}
			if !errors.Is(err, tc.want) || w.calls != 1 || !bytes.Equal(w.Bytes(), expected[:n]) || !bytes.Equal(input, before) {
				t.Fatalf("one-write delivered-prefix/cause: %v writes=%d delivered=%d", err, w.calls, w.Len())
			}
			var f *stringFactsFailure
			if !errors.As(err, &f) {
				t.Fatalf("structured IO/cancel boundary: %v", err)
			}
			kind := "io"
			if tc.want == context.Canceled {
				kind = "canceled"
			}
			if f.Kind != kind {
				t.Fatalf("writer error priority: %+v", f)
			}
		})
	}
	for _, args := range [][]string{{"--string-facts-io-v3"}, {"--string-facts-io-v2", "extra"}, {"--string-facts-io-v2", "--string-facts-io-v2"}, {"--string-facts-io-v2=true"}, {" --string-facts-io-v2"}, {"--string-facts-io-v2 "}} {
		t.Run(args[0], func(t *testing.T) {
			r := &stringConcatReadProbe24e{data: bytes.Clone(input)}
			w := &stringConcatWriteProbe24e{}
			if err := runFrontendCommand(t.Context(), args, r, w); err == nil || r.calls != 0 || w.calls != 0 {
				t.Fatalf("exact argv must reject before IO: %v reads=%d writes=%d", err, r.calls, w.calls)
			}
		})
	}
	for _, flag := range []string{"--exception-facts-io-v1", "--nullable-facts-io-v3", "--generic-package-io-v1"} {
		t.Run(flag, func(t *testing.T) {
			w := &stringConcatWriteProbe24e{}
			if err := runFrontendCommand(t.Context(), []string{flag}, bytes.NewReader(input), w); err == nil || w.calls != 0 {
				t.Fatalf("other family accepted v2 envelope: %v writes=%d", err, w.calls)
			}
		})
	}
	t.Run("publisherPreCanceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		w := &stringConcatWriteProbe24e{}
		if err := publishStringFactsPacket(ctx, w, []string{"owned"}); !errors.Is(err, context.Canceled) || w.calls != 0 {
			t.Fatalf("actual shared publisher write-before-cancel: %v", err)
		}
	})
}
