// SPDX-License-Identifier: Apache-2.0
// Exact unpublished common facts boundary, independent of formal Frontend admission.
package main

import (
	"bytes"
	"context"
	"io"

	"github.com/microsoft/typescript-go/internal/json"
)

func runCommonProgramFacts(ctx context.Context, input io.Reader, output io.Writer) error {
	data, err := io.ReadAll(io.LimitReader(arrayFactsReader{ctx, input}, arrayFactsMaxBytes+1))
	if err != nil {
		return arrayFactsError("io", "common Program request", err)
	}
	if err := genericJSONPreflight(data, arrayFactsPolicy(false)); err != nil {
		return arrayFactsJSONError(err)
	}
	if err := arrayFactsRequestArrayLimit(data, 128); err != nil {
		return err
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		return arrayFactsError("canonical", "common Program JCS plus LF", nil)
	}
	raw := data[:len(data)-1]
	value := json.Value(bytes.Clone(raw))
	if err := value.Canonicalize(); err != nil || !bytes.Equal(value, raw) {
		return arrayFactsError("canonical", "common Program exact JCS", err)
	}
	var r commonProgramRequest
	if err := json.Unmarshal(raw, &r); err != nil {
		return arrayFactsError("schema", "common Program exact request", err)
	}
	encoded, err := arrayFactsCanonical(r)
	schema := "sha256:" + arrayFactsSHA(commonProgramSchema)
	if err != nil || !bytes.Equal(encoded, raw) || r.Family != "tsfg.program-facts-request-candidate" || r.Version != 2 || r.SchemaHash != schema {
		return arrayFactsError("schema", "common Program identity/fields", err)
	}
	hostPaths, err := commonHostPaths(r.Input)
	if err != nil {
		return err
	}
	owned, err := arrayGenericInput(r.Input.arrayInput(), hostPaths...)
	if err != nil {
		return err
	}
	var target *integerTarget
	if raw := r.Input.Target; raw != nil {
		target = &integerTarget{Target: raw.Target, Triple: raw.Triple, LLVMDataLayout: raw.LLVMDataLayout, PointerBits: raw.PointerBits}
	}
	if !validIntegerTarget(target) || r.Input.CompilationClosure == nil || !exceptionFactsClosure(r.Input.CompilationClosure, owned.Files) {
		return arrayFactsError("input", "common Program target and complete original root tuples", nil)
	}
	for _, roots := range [][]closureRoot{r.Input.CompilationClosure.Roots, r.Input.CompilationClosure.RequiredRoots} {
		previous := ""
		for _, root := range roots {
			key := moduleCallableRootKey(root)
			if previous != "" && key <= previous {
				return arrayFactsError("input", "common Program strict original root order", nil)
			}
			previous = key
		}
	}
	owned.Target, owned.CompilationClosure = target, r.Input.CompilationClosure
	owned.Declarations.Paths = append(owned.Declarations.Paths, hostPaths...)
	projection := &commonProgramProjection{input: r.Input}
	payload, err := projectArrayGeneric(ctx, owned, r.Input.arrayInput(), projection)
	if err != nil {
		return err
	}
	complete := commonProgramPayloadFrom(payload, projection)
	payloadBytes, err := arrayFactsCanonical(complete)
	if err != nil {
		return err
	}
	packet := commonProgramPackage{Family: "tsfg.program-facts-package-candidate", Version: 2, SchemaHash: schema,
		RequestID: "sha256:" + arrayFactsSHA(append([]byte("tsfg.program-facts.request.v2\x00"), data...)),
		PackageID: "sha256:" + arrayFactsSHA(append([]byte("tsfg.program-facts.package.v2\x00"+schema+"\x00"), payloadBytes...)), Payload: complete}
	encoded, err = arrayFactsCanonical(packet)
	if err != nil {
		return err
	}
	if len(encoded) >= arrayFactsMaxBytes {
		return arrayFactsError("limit", "common Program complete packet", nil)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	n, err := output.Write(encoded)
	if err != nil {
		return arrayFactsError("io", "common Program packet write", err)
	}
	if n != len(encoded) {
		return io.ErrShortWrite
	}
	return ctx.Err()
}
