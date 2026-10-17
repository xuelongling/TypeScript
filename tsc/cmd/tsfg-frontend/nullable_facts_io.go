// SPDX-License-Identifier: Apache-2.0

// This opt-in producer validates a complete package before any transport write.
package main

import (
	"bytes"
	"context"
	"io"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

// Cancellation wins even over a simultaneous non-EOF read error here. The
// independently versioned generic stream preserves that read error instead;
// keep this one-shot producer policy local rather than coupling the protocols.
type nullableFactsReader struct {
	ctx   context.Context
	input io.Reader
}

func (r nullableFactsReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.input.Read(p)
	if canceled := r.ctx.Err(); canceled != nil {
		return n, canceled
	}
	return n, err
}

func runNullableFactsCandidate(ctx context.Context, input io.Reader, output io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(nullableFactsReader{ctx: ctx, input: input}, nullableFactsMaxBytes+1))
	if err != nil {
		return err
	}
	encoded, err := handleNullableFactsCandidate(ctx, data)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := output.Write(encoded)
	if err != nil {
		return err
	}
	if n != len(encoded) {
		return io.ErrShortWrite
	}
	return ctx.Err()
}

func handleNullableFactsCandidate(ctx context.Context, data []byte) (outputBytes []byte, err error) {
	defer func() {
		if canceled := ctx.Err(); canceled != nil {
			outputBytes, err = nil, canceled
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r, err := decodeNullableFactsRequest(data)
	if err != nil {
		return nil, err
	}
	owned, inputID, files, err := nullableFactsInputs(r.Input)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var exports []nullableFactsExport
	var projectionError error
	called := false
	facts := analyzeWithProjection(ctx, owned, func(file *ast.SourceFile, check *checker.Checker, functions map[*ast.Symbol]functionInfo, result response) {
		called = true
		exports, projectionError = projectNullableFacts(file, check, functions, result)
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// An earlier official/legacy projection rejection remains a Source failure,
	// not a successful private export or a later export-table rejection.
	if facts.Status != "ok" || len(facts.Diagnostics) != 0 {
		if len(facts.Diagnostics) != 0 {
			diagnostic := facts.Diagnostics[0]
			return nil, nullableFactsError("source", diagnostic.Code+" at "+diagnostic.File)
		}
		return nil, nullableFactsError("source", "owned projection did not succeed")
	}
	if projectionError != nil {
		return nil, projectionError
	}
	if !called || len(facts.Inputs) != len(files) {
		return nil, nullableFactsError("input", "incomplete actual consumed source closure")
	}
	for i, file := range files {
		if facts.Inputs[i] != file.Path {
			return nil, nullableFactsError("input", "actual consumed files differ from complete input closure")
		}
	}
	payload := nullableFactsPayload{InputIdentity: inputID, Files: files, Facts: facts, Exports: exports}
	payloadBytes, err := nullableFactsCanonical(payload)
	if err != nil {
		return nil, err
	}
	requestID := nullableFactsDigest(append([]byte("tsfg.nullable-facts.request.v2\x00"), data...))
	preimage := append([]byte("tsfg.nullable-facts.package.v2\x00"+nullableFactsSchema+"\x00"), payloadBytes...)
	result := nullableFactsPackage{Family: "tsfg.nullable-facts-package-candidate", Version: 2, SchemaHash: nullableFactsSchema,
		RequestID: requestID, PackageID: nullableFactsDigest(preimage), Payload: payload}
	encoded, err := nullableFactsCanonical(result)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Retain no caller byte slice in the returned package.
	return bytes.Clone(encoded), nil
}
