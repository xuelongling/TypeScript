// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// The candidate stream boundary is callable without changing native argv modes.
package main

import (
	"bytes"
	"context"
	"io"
)

type genericPackageIOReader struct {
	ctx   context.Context
	input io.Reader
}

func (r genericPackageIOReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.input.Read(p)
	if err != nil && err != io.EOF {
		return n, err
	}
	if contextErr := r.ctx.Err(); contextErr != nil {
		return n, contextErr
	}
	return n, err
}

func runGenericPackageCandidate(ctx context.Context, input io.Reader, output io.Writer) error {
	data, err := io.ReadAll(io.LimitReader(genericPackageIOReader{ctx: ctx, input: input}, genericPackageIOMaxBytes+1))
	if err != nil {
		return err
	}
	result, err := handleGenericPackageCandidate(ctx, data)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := io.Copy(output, bytes.NewReader(result))
	if err != nil {
		return err
	}
	if n != int64(len(result)) {
		return io.ErrShortWrite
	}
	return ctx.Err()
}
