// SPDX-License-Identifier: Apache-2.0
// One-shot command: validate before analysis and publish at most one complete value.
package main

import (
	"context"
	"errors"
	"io"
)

type stringFactsReader struct {
	ctx   context.Context
	input io.Reader
}

func (r stringFactsReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.input.Read(p)
	if canceled := r.ctx.Err(); canceled != nil {
		return n, errors.Join(err, canceled)
	}
	return n, err
}
func runStringFactsCandidate(ctx context.Context, input io.Reader, output io.Writer) error {
	if err := ctx.Err(); err != nil {
		return stringFactsError("canceled", "before read", err)
	}
	data, err := io.ReadAll(io.LimitReader(stringFactsReader{ctx, input}, stringFactsMaxBytes+1))
	if canceled := ctx.Err(); canceled != nil {
		if !errors.Is(err, canceled) {
			err = errors.Join(err, canceled)
		}
		return stringFactsError("canceled", "after read", err)
	}
	if err != nil {
		return stringFactsError("io", "request read", err)
	}
	r, err := decodeStringFactsRequest(data)
	if err != nil {
		return err
	}
	owned, err := stringFactsInputs(r.Input)
	if err != nil {
		return err
	}
	payload, err := projectStringFacts(ctx, owned, r.Input)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return stringFactsError("canceled", "after checking", err)
	}
	payloadBytes, err := stringFactsCanonical(payload)
	if err != nil {
		return err
	}
	packet := stringFactsPackage{Family: "tsfg.string-facts-package-candidate", Version: 1, SchemaHash: stringFactsSchema,
		RequestID: "sha256:" + stringFactsSHA(append([]byte("tsfg.string-facts.request.v1\x00"), data...)),
		PackageID: "sha256:" + stringFactsSHA(append([]byte("tsfg.string-facts.package.v1\x00"+stringFactsSchema+"\x00"), payloadBytes...)), Payload: payload}
	encoded, err := stringFactsCanonical(packet)
	if err != nil {
		return err
	}
	if len(encoded) >= stringFactsMaxBytes {
		return stringFactsError("limit", "output bytes", nil)
	}
	encoded = append(encoded, '\n')
	if err := ctx.Err(); err != nil {
		return stringFactsError("canceled", "before write", err)
	}
	n, err := output.Write(encoded)
	if err != nil {
		return stringFactsError("io", "response write", err)
	}
	if n != len(encoded) {
		return stringFactsError("io", "response short write", io.ErrShortWrite)
	}
	if err := ctx.Err(); err != nil {
		return stringFactsError("canceled", "after write", err)
	}
	return nil
}
