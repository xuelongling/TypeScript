// SPDX-License-Identifier: Apache-2.0

// This opt-in producer validates a complete package before any transport write.
package main

import (
	"bytes"
	"context"
	"io"
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
	return runNullableFactsVersion(ctx, input, output, nullableFactsV3)
}

func runNullablePresenceFactsCandidate(ctx context.Context, input io.Reader, output io.Writer) error {
	return runNullableFactsVersion(ctx, input, output, nullableFactsV4)
}

func runNullableFactsVersion(ctx context.Context, input io.Reader, output io.Writer, version nullableFactsVersion) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(nullableFactsReader{ctx: ctx, input: input}, nullableFactsMaxBytes+1))
	if err != nil {
		return err
	}
	encoded, err := handleNullableFactsCandidate(ctx, data, version)
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

func handleNullableFactsCandidate(ctx context.Context, data []byte, version nullableFactsVersion) (outputBytes []byte, err error) {
	defer func() {
		if canceled := ctx.Err(); canceled != nil {
			outputBytes, err = nil, canceled
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if version != nullableFactsV3 && version != nullableFactsV4 {
		return nil, version.failure("schema", "selected nullable protocol")
	}
	r, err := decodeNullableFactsRequest(data, version)
	if err != nil {
		return nil, err
	}
	owned, inputID, files, err := nullableFactsInputs(r.Input, version)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var exports []nullableFactsExport
	var projectionError error
	called := false
	facts := analyzeWithProjection(ctx, owned, func(checked ownedProjectionContext) {
		called = true
		exports, projectionError = projectNullableFacts(checked.Root, checked.Checker, checked.Functions, checked.Facts, version)
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// An earlier official/legacy projection rejection remains a Source failure,
	// not a successful private export or a later export-table rejection.
	if facts.Status != "ok" || len(facts.Diagnostics) != 0 {
		if len(facts.Diagnostics) != 0 {
			diagnostic := facts.Diagnostics[0]
			if version == nullableFactsV4 {
				return nil, &nullablePresenceFactsError{Kind: "source", Detail: diagnostic.Code, File: diagnostic.File, Range: diagnostic.Range}
			}
			return nil, nullableFactsError("source", diagnostic.Code+" at "+diagnostic.File)
		}
		return nil, version.failure("source", "owned projection did not succeed")
	}
	if projectionError != nil {
		return nil, projectionError
	}
	if !called || len(facts.Inputs) != len(files) {
		return nil, version.failure("input", "incomplete actual consumed source closure")
	}
	for i, file := range files {
		if facts.Inputs[i] != file.Path {
			return nil, version.failure("input", "actual consumed files differ from complete input closure")
		}
	}
	payload := nullableFactsPayload{InputIdentity: inputID, Files: files, Facts: facts, Exports: exports}
	payloadBytes, err := nullableFactsCanonical(payload, version)
	if err != nil {
		return nil, err
	}
	requestID := nullableFactsDigest(append([]byte(version.requestDomain()), data...))
	preimage := append([]byte(version.packageDomain()+version.schemaHash()+"\x00"), payloadBytes...)
	result := nullableFactsPackage{Family: "tsfg.nullable-facts-package-candidate", Version: int(version), SchemaHash: version.schemaHash(),
		RequestID: requestID, PackageID: nullableFactsDigest(preimage), Payload: payload}
	encoded, err := nullableFactsCanonical(result, version)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Retain no caller byte slice in the returned package.
	return bytes.Clone(encoded), nil
}
