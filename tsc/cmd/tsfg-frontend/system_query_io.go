// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	_ "embed"
	"github.com/microsoft/typescript-go/internal/json"
	"io"
	"slices"
)

//go:embed testdata/system-query-v5/schema.json
var systemQuerySchema []byte

type systemQueryInput struct {
	SourceRoot   string                 `json:"sourceRoot"`
	RootFile     string                 `json:"rootFile"`
	Files        []arrayFactsInputFile  `json:"files"`
	Declarations arrayFactsDeclarations `json:"declarations"`
	Config       arrayFactsConfig       `json:"config"`
	Target       *commonProgramTarget   `json:"target"`
}
type systemQueryRequest struct {
	Family     string           `json:"family"`
	Version    int              `json:"version"`
	SchemaHash string           `json:"schemaHash"`
	Input      systemQueryInput `json:"input"`
}
type systemQueryPackage struct {
	Family     string             `json:"family"`
	Version    int                `json:"version"`
	SchemaHash string             `json:"schemaHash"`
	RequestID  string             `json:"requestId"`
	PackageID  string             `json:"packageId"`
	Payload    systemQueryPayload `json:"payload"`
}

func systemQuerySchemaID() (string, error) {
	value := json.Value(bytes.Clone(systemQuerySchema))
	if e := value.Canonicalize(); e != nil {
		return "", e
	}
	return "sha256:" + arrayFactsSHA(append(value, '\n')), nil
}
func runSystemQueryFacts(ctx context.Context, input io.Reader, output io.Writer) error {
	data, e := io.ReadAll(io.LimitReader(arrayFactsReader{ctx, input}, arrayFactsMaxBytes+1))
	if e != nil {
		return arrayFactsError("io", "system Query input", e)
	}
	if e = genericJSONPreflight(data, arrayFactsPolicy(false)); e != nil {
		return arrayFactsJSONError(e)
	}
	if e = arrayFactsRequestArrayLimit(data, 64); e != nil {
		return e
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		return arrayFactsError("canonical", "system Query JCS+LF", nil)
	}
	raw := data[:len(data)-1]
	canonical := json.Value(bytes.Clone(raw))
	if e = canonical.Canonicalize(); e != nil || !bytes.Equal(raw, canonical) {
		return arrayFactsError("canonical", "system Query exact JCS", e)
	}
	var request systemQueryRequest
	if e = json.Unmarshal(raw, &request); e != nil {
		return arrayFactsError("schema", "system Query exact fields", e)
	}
	encoded, e := arrayFactsCanonical(request)
	schema, e2 := systemQuerySchemaID()
	if e != nil || e2 != nil || !bytes.Equal(raw, encoded) || request.Family != "tsfg.system-query-facts-request-candidate" || request.Version != 5 || request.SchemaHash != schema {
		return arrayFactsError("schema", "system Query identity/fields", e)
	}
	in := request.Input
	expected := []string{in.SourceRoot + "/minimal-arrays-v8.d.ts", systemQueryProfilePath}
	if in.SourceRoot != "/src" || !slices.Equal(in.Declarations.Paths, expected) {
		return arrayFactsError("input", "complete exact query declaration context", nil)
	}
	// Reuse the base validator for every original file/hash/config. The extra
	// declaration path was checked above and is restored before the one Program.
	base := arrayFactsInput{SourceRoot: in.SourceRoot, RootFile: in.RootFile, Files: in.Files, Declarations: arrayFactsDeclarations{Version: in.Declarations.Version, Paths: expected[:1]}, Config: in.Config}
	owned, e := arrayGenericInput(base, systemQueryProfilePath)
	if e != nil {
		return e
	}
	owned.Declarations.Paths = append([]string{}, in.Declarations.Paths...)
	owned.CompilationClosure = &closureConfiguration{}
	if in.Target != nil {
		owned.Target = &integerTarget{Target: in.Target.Target, Triple: in.Target.Triple, LLVMDataLayout: in.Target.LLVMDataLayout, PointerBits: in.Target.PointerBits}
	}
	if !validIntegerTarget(owned.Target) {
		return arrayFactsError("input", "system Query target", nil)
	}
	payload, e := projectSystemQueries(ctx, owned)
	if e != nil {
		return e
	}
	payloadBytes, e := arrayFactsCanonical(payload)
	if e != nil {
		return e
	}
	packet := systemQueryPackage{Family: "tsfg.system-query-facts-package-candidate", Version: 5, SchemaHash: schema, RequestID: "sha256:" + arrayFactsSHA(append([]byte("tsfg.system-query.request.v5\x00"), data...)), PackageID: "sha256:" + arrayFactsSHA(append([]byte("tsfg.system-query.package.v5\x00"+schema+"\x00"), payloadBytes...)), Payload: payload}
	encoded, e = arrayFactsCanonical(packet)
	if e != nil {
		return e
	}
	if len(encoded) >= arrayFactsMaxBytes {
		return arrayFactsError("limit", "complete system Query packet", nil)
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	encoded = append(encoded, '\n')
	n, e := output.Write(encoded)
	if e != nil {
		return arrayFactsError("io", "system Query output", e)
	}
	if n != len(encoded) {
		return io.ErrShortWrite
	}
	return ctx.Err()
}
