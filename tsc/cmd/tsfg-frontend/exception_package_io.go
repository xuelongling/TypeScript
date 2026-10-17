// SPDX-License-Identifier: Apache-2.0

// This one-shot private producer finishes validation before its single write.
package main

import (
	"context"
	"io"
)

type exceptionFactsReader struct {
	ctx   context.Context
	input io.Reader
}

func (r exceptionFactsReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.input.Read(p)
	if canceled := r.ctx.Err(); canceled != nil {
		return n, canceled
	}
	return n, err
}

func runExceptionFactsCandidate(ctx context.Context, input io.Reader, output io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(exceptionFactsReader{ctx: ctx, input: input}, exceptionFactsMaxBytes+1))
	if err != nil {
		return err
	}
	encoded, err := handleExceptionFactsCandidate(ctx, data)
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

func handleExceptionFactsCandidate(ctx context.Context, data []byte) (outputBytes []byte, err error) {
	defer func() {
		if canceled := ctx.Err(); canceled != nil {
			outputBytes, err = nil, canceled
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r, err := decodeExceptionFactsRequest(data)
	if err != nil {
		return nil, err
	}
	owned, inputID, files, err := exceptionFactsInputs(r.Input)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var exports []exceptionFactsExportGroup
	var projectionError error
	var actualFiles []string
	calls := 0
	facts := analyzeWithProjection(ctx, owned, func(checked ownedProjectionContext) {
		calls++
		if calls != 1 {
			projectionError = exceptionFactsError("input", "repeated terminal callback")
			return
		}
		actualFiles = make([]string, 0, len(checked.SourceFiles))
		for _, file := range checked.SourceFiles {
			actualFiles = append(actualFiles, file.FileName())
		}
		if projectionError = exceptionFactsRoots(checked, owned.CompilationClosure); projectionError != nil {
			return
		}
		exports, projectionError = projectExceptionFactsExports(checked)
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Official/owned failures take precedence over a terminal export rejection.
	if facts.Status != "ok" || len(facts.Diagnostics) != 0 {
		if len(facts.Diagnostics) != 0 {
			diagnostic := facts.Diagnostics[0]
			return nil, exceptionFactsError("source", diagnostic.Code+" at "+diagnostic.File)
		}
		return nil, exceptionFactsError("source", "owned projection did not succeed")
	}
	if projectionError != nil {
		return nil, projectionError
	}
	if calls != 1 || len(facts.Inputs) != len(files) || len(actualFiles) != len(files) {
		return nil, exceptionFactsError("input", "incomplete actual consumed source closure")
	}
	for i, file := range files {
		if facts.Inputs[i] != file.Path || actualFiles[i] != file.Path {
			return nil, exceptionFactsError("input", "actual consumed files differ from complete input closure")
		}
	}
	if len(facts.Functions) > 512 || len(facts.Symbols) > 4096 || len(facts.HostImports) != 0 {
		return nil, exceptionFactsError("limit", "owned function/symbol/import bounds")
	}
	if err := exceptionFactsCallableBounds(facts); err != nil {
		return nil, err
	}
	encodedFacts, err := exceptionFactsEncodeResponse(facts)
	if err != nil {
		return nil, err
	}
	payload := exceptionFactsPayload{InputIdentity: inputID, Files: files, Facts: encodedFacts, Exports: exports}
	payloadBytes, err := exceptionFactsCanonical(payload)
	if err != nil {
		return nil, err
	}
	requestID := exceptionFactsDigest(append([]byte("tsfg.exception-facts.request.v1\x00"), data...))
	preimage := append([]byte("tsfg.exception-facts.package.v1\x00"+exceptionFactsSchema+"\x00"), payloadBytes...)
	result := exceptionFactsPackage{Family: "tsfg.exception-facts-package-candidate", Version: 1, SchemaHash: exceptionFactsSchema,
		RequestID: requestID, PackageID: exceptionFactsDigest(preimage), Payload: payload}
	encoded, err := exceptionFactsCanonical(result)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return encoded, nil
}
