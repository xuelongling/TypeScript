// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"
)

func runModuleCallableFactsCandidate(ctx context.Context, input io.Reader, output io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// The existing contextual reader preserves cancellation before and after Read.
	data, err := io.ReadAll(io.LimitReader(exceptionFactsReader{ctx: ctx, input: input}, moduleCallableMaxBytes+1))
	if err != nil {
		return err
	}
	encoded, err := handleModuleCallableFactsCandidate(ctx, data)
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

func handleModuleCallableFactsCandidate(ctx context.Context, data []byte) (result []byte, err error) {
	defer func() {
		if canceled := ctx.Err(); canceled != nil {
			result, err = nil, canceled
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	decoded, err := decodeModuleCallableRequest(data)
	if err != nil {
		return nil, err
	}
	owned, inputID, files, err := moduleCallableInputs(decoded.Input)
	if err != nil {
		return nil, err
	}
	calls := 0
	var projectionError error
	var annotations []moduleCallableAnnotation
	var exports []moduleCallableExportGroup
	facts := analyzeWithProjection(ctx, owned, func(checked ownedProjectionContext) {
		calls++
		if calls != 1 {
			projectionError = moduleCallableError("source")
			return
		}
		if len(checked.SourceFiles) != len(files) || len(checked.Facts.Inputs) != len(files) {
			projectionError = moduleCallableError("identity")
			return
		}
		for i, file := range files {
			actual := checked.SourceFiles[i]
			if actual.FileName() != file.Path || checked.Facts.Inputs[i] != file.Path || moduleCallableHash([]byte(actual.Text())) != file.Digest {
				projectionError = moduleCallableError("identity")
				return
			}
		}
		if err := moduleCallableRoots(checked, owned.CompilationClosure); err != nil {
			projectionError = moduleCallableError("identity")
			return
		}
		annotations, exports, projectionError = moduleCallableOrigins(checked)
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if facts.Status != "ok" || len(facts.Diagnostics) != 0 {
		if len(facts.Diagnostics) > 0 && facts.Diagnostics[0].Code == "r03-module-callable-limit" {
			return nil, moduleCallableError("limit")
		}
		return nil, moduleCallableError("source")
	}
	if projectionError != nil {
		return nil, projectionError
	}
	if calls != 1 || len(facts.Functions) == 0 {
		return nil, moduleCallableError("source")
	}
	if err := moduleCallableFactsBounds(facts); err != nil {
		return nil, err
	}
	requestID, err := moduleCallableDigest("tsfg.module-callable-facts.request.v1", decoded)
	if err != nil {
		return nil, err
	}
	packet := moduleCallablePacket{Family: "tsfg.module-callable-facts-package-candidate", Version: 1, SchemaHash: moduleCallableFactsSchema, RequestID: requestID, Payload: moduleCallablePayload{InputIdentity: inputID, Files: files, Facts: facts, Annotations: annotations, Exports: exports}}
	packet.PackageID, err = moduleCallableDigest("tsfg.module-callable-facts.package.v1", packet)
	if err != nil {
		return nil, err
	}
	encoded, err := moduleCallableCanonical(packet)
	if err != nil {
		return nil, err
	}
	if len(encoded)+1 > moduleCallableMaxBytes {
		return nil, moduleCallableError("limit")
	}
	return append(encoded, '\n'), nil
}
