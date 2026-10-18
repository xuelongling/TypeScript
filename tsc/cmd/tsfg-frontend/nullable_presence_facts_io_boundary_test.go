// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

type nullablePresenceBoundaryReadFunc func([]byte) (int, error)

func (f nullablePresenceBoundaryReadFunc) Read(p []byte) (int, error) { return f(p) }

func TestNullableFactsPresenceArgvBeforeIO(t *testing.T) {
	raw := nullablePresenceFirstRead(t, "mask2-null.request.json")
	for _, args := range [][]string{{"--nullable-facts-io-v5"}, {"--nullable-facts-io-v4", "extra"}, {"extra", "--nullable-facts-io-v4"}, {"--nullable-facts-io-v4", "--nullable-facts-io-v3"}, {"--nullable-facts-io-v4", "--nullable-facts-io-v4"}} {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		r, w, err := nullablePresenceBoundaryCommand(ctx, args, raw)
		if err == nil || errors.Is(err, context.Canceled) || r.calls != 0 || r.delivered != 0 || w.writes != 0 {
			t.Fatalf("argv precedence violated: args=%v err=%v reads=%d writes=%d", args, err, r.calls, w.writes)
		}
	}
}

func TestNullableFactsPresenceCancellationAndReadIdentity(t *testing.T) {
	raw := nullablePresenceFirstRead(t, "mask2-null.request.json")
	expected := nullablePresenceFirstRead(t, "mask2-null.package.json")
	t.Run("entry", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		r, w, err := nullablePresenceBoundaryCommand(ctx, []string{"--nullable-facts-io-v4"}, raw)
		if err != context.Canceled || r.calls != 0 || w.writes != 0 {
			t.Fatalf("entry cancellation identity: %v", err)
		}
	})
	sentinel := errors.New("CALLER_SECRET_20K reader identity")
	for _, delivered := range []int{0, 7, 512} {
		t.Run("read-error-"+string(rune('A'+delivered%26)), func(t *testing.T) {
			reader := &nullablePresenceBoundaryReader{reader: nullablePresenceBoundaryReadFunc(func(p []byte) (int, error) { n := copy(p, raw[:delivered]); return n, sentinel })}
			writer := &nullablePresenceBoundaryWriter{}
			err := runFrontendCommand(t.Context(), []string{"--nullable-facts-io-v4"}, reader, writer)
			if err != sentinel || reader.calls != 1 || writer.writes != 0 {
				t.Fatalf("read cause wrapped, retried or published: %v", err)
			}
		})
	}
	t.Run("cancel-wins-over-read-error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		reader := &nullablePresenceBoundaryReader{reader: nullablePresenceBoundaryReadFunc(func(p []byte) (int, error) { return copy(p, raw), sentinel }), after: cancel}
		writer := &nullablePresenceBoundaryWriter{}
		err := runFrontendCommand(ctx, []string{"--nullable-facts-io-v4"}, reader, writer)
		if err != context.Canceled || writer.writes != 0 || reader.calls != 1 {
			t.Fatalf("read-post cancellation precedence changed: %v", err)
		}
	})
	t.Run("cancel-after-successful-read", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		reader := &nullablePresenceBoundaryReader{reader: bytes.NewReader(raw), after: cancel}
		writer := &nullablePresenceBoundaryWriter{}
		if err := runFrontendCommand(ctx, []string{"--nullable-facts-io-v4"}, reader, writer); err != context.Canceled || writer.writes != 0 {
			t.Fatalf("read cancellation published: %v", err)
		}
	})
	t.Run("data-plus-EOF", func(t *testing.T) {
		cursor := 0
		reader := nullablePresenceBoundaryReadFunc(func(p []byte) (int, error) {
			n := copy(p, raw[cursor:])
			cursor += n
			if cursor == len(raw) {
				return n, io.EOF
			}
			return n, nil
		})
		writer := &nullablePresenceBoundaryWriter{}
		if err := runFrontendCommand(t.Context(), []string{"--nullable-facts-io-v4"}, reader, writer); err != nil || writer.writes != 1 || !bytes.Equal(writer.attempts[0], expected) {
			t.Fatalf("valid data+EOF changed: %v", err)
		}
	})
}

func TestNullableFactsPresenceSinglePublishFailures(t *testing.T) {
	raw := nullablePresenceFirstRead(t, "mask2-null.request.json")
	expected := nullablePresenceFirstRead(t, "mask2-null.package.json")
	sentinel := errors.New("CALLER_SECRET_20K writer identity")
	for _, row := range []struct {
		name    string
		n       int
		failure error
		cancel  bool
		want    error
	}{
		{"zero-short", 0, nil, false, io.ErrShortWrite},
		{"partial-short", 17, nil, false, io.ErrShortWrite},
		{"zero-error", 0, sentinel, false, sentinel},
		{"partial-error", 17, sentinel, false, sentinel},
		{"full-error", len(expected), sentinel, false, sentinel},
		{"postwrite-cancel", len(expected), nil, true, context.Canceled},
		{"write-error-before-postcancel", 17, sentinel, true, sentinel},
		{"short-before-postcancel", 17, nil, true, io.ErrShortWrite},
	} {
		t.Run(row.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			physical := bytes.Buffer{}
			writer := &nullablePresenceBoundaryWriter{hook: func(p []byte) (int, error) {
				if !bytes.Equal(p, expected) {
					t.Fatal("writer received an incomplete/unverified packet")
				}
				physical.Write(p[:row.n])
				if row.cancel {
					cancel()
				}
				return row.n, row.failure
			}}
			err := runFrontendCommand(ctx, []string{"--nullable-facts-io-v4"}, bytes.NewReader(raw), writer)
			if err != row.want || writer.writes != 1 || len(writer.attempts) != 1 || !bytes.Equal(writer.attempts[0], expected) || !bytes.Equal(physical.Bytes(), expected[:row.n]) {
				t.Fatalf("single publish transport semantics: %v", err)
			}
			var structured *nullablePresenceFactsError
			if errors.As(err, &structured) {
				t.Fatal("raw transport/cancellation identity became semantic error")
			}
		})
	}
}
