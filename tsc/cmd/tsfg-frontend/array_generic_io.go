// SPDX-License-Identifier: Apache-2.0
// Strict v8 boundary does not reinterpret or forward to the retired profile.
package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/microsoft/typescript-go/internal/json"
)

func decodeArrayGenericRequest(data []byte) (arrayFactsRequest, error) {
	var r arrayFactsRequest
	if err := genericJSONPreflight(data, arrayFactsPolicy(false)); err != nil {
		return r, arrayFactsJSONError(err)
	}
	if err := arrayFactsRequestArrayLimit(data, 64); err != nil {
		return r, err
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		return r, arrayFactsError("canonical", "v8 JCS plus one LF required", nil)
	}
	raw := data[:len(data)-1]
	canonical := json.Value(bytes.Clone(raw))
	if err := canonical.Canonicalize(); err != nil || !bytes.Equal(raw, canonical) {
		return r, arrayFactsError("canonical", "v8 exact JCS required", err)
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return r, arrayFactsError("schema", "v8 request DTO", err)
	}
	encoded, err := arrayFactsCanonical(r)
	if err != nil {
		return r, err
	}
	if !bytes.Equal(encoded, raw) || r.Family != "tsfg.array-facts-request-candidate" || r.Version != 8 {
		return r, arrayFactsError("schema", "v8 exact fields and version", nil)
	}
	return r, nil
}
func arrayGenericInput(input arrayFactsInput) (request, error) {
	var r request
	root, dts := input.SourceRoot, input.SourceRoot+"/minimal-arrays-v8.d.ts"
	if !arrayFactsPath(root) || !arrayFactsPath(input.RootFile) || !strings.HasPrefix(input.RootFile, root+"/") || strings.HasSuffix(input.RootFile, ".d.ts") || !strings.HasSuffix(input.RootFile, ".ts") || len(input.Files) < 2 || len(input.Files) > 64 || input.Declarations.Version != "minimal-arrays-v8" || len(input.Declarations.Paths) != 1 || input.Declarations.Paths[0] != dts || !input.Config.Strict || !input.Config.NoEmit || input.Config.AllowJS {
		return r, arrayFactsError("input", "v8 source/config/declarations closure", nil)
	}
	seen := map[string]bool{}
	for i, f := range input.Files {
		if !arrayFactsPath(f.Path) || !strings.HasPrefix(f.Path, root+"/") || !strings.HasSuffix(f.Path, ".ts") || f.Path != dts && strings.HasSuffix(f.Path, ".d.ts") || i > 0 && input.Files[i-1].Path >= f.Path || !utf8.ValidString(f.Content) || len(f.Content) > 1048576 || f.SHA256 != arrayFactsSHA([]byte(f.Content)) {
			return r, arrayFactsError("input", "v8 exact sorted original files", nil)
		}
		for prior := range seen {
			if strings.HasPrefix(f.Path, prior+"/") || strings.HasPrefix(prior, f.Path+"/") {
				return r, arrayFactsError("input", "source path ancestry", nil)
			}
		}
		if f.Path == dts && !bytes.Equal([]byte(f.Content), arrayGenericDeclarations) {
			return r, arrayFactsError("identity", "v8 pinned declaration bytes", nil)
		}
		seen[f.Path] = true
		r.Files = append(r.Files, inputFile{Path: f.Path, Content: f.Content})
	}
	if !seen[input.RootFile] || !seen[dts] {
		return r, arrayFactsError("input", "v8 missing root/declarations", nil)
	}
	r.SourceRoot, r.RootFile = root, input.RootFile
	r.Declarations.Version = input.Declarations.Version
	r.Declarations.Paths = append([]string{}, input.Declarations.Paths...)
	r.Config.Strict, r.Config.NoEmit = true, true
	return r, nil
}
func runArrayGenericCandidate(ctx context.Context, input io.Reader, output io.Writer) error {
	data, err := io.ReadAll(io.LimitReader(arrayFactsReader{ctx, input}, arrayFactsMaxBytes+1))
	if err != nil {
		return arrayFactsError("io", "v8 request read", err)
	}
	if err := ctx.Err(); err != nil {
		return arrayFactsError("canceled", "v8 request read", err)
	}
	r, err := decodeArrayGenericRequest(data)
	if err != nil {
		return err
	}
	owned, err := arrayGenericInput(r.Input)
	if err != nil {
		return err
	}
	payload, err := projectArrayGeneric(ctx, owned, r.Input)
	if err != nil {
		return err
	}
	schema := "sha256:" + arrayFactsSHA(arrayGenericSchema)
	canonical, err := arrayFactsCanonical(payload)
	if err != nil {
		return err
	}
	packet := arrayGenericPackage{Family: "tsfg.array-facts-package-candidate", Version: 8, SchemaHash: schema, RequestID: "sha256:" + arrayFactsSHA(append([]byte("tsfg.array-facts.request.v8\x00"), data...)), PackageID: "sha256:" + arrayFactsSHA(append([]byte("tsfg.array-facts.package.v8\x00"+schema+"\x00"), canonical...)), Payload: payload}
	encoded, err := arrayFactsCanonical(packet)
	if err != nil {
		return err
	}
	if len(encoded) >= arrayFactsMaxBytes {
		return arrayFactsError("limit", "v8 complete packet bytes", nil)
	}
	if err := ctx.Err(); err != nil {
		return arrayFactsError("canceled", "v8 before write", err)
	}
	encoded = append(encoded, '\n')
	n, err := output.Write(encoded)
	if err != nil {
		return arrayFactsError("io", "v8 packet write", err)
	}
	if n != len(encoded) {
		return arrayFactsError("io", "v8 short packet write", io.ErrShortWrite)
	}
	if err := ctx.Err(); err != nil {
		return arrayFactsError("canceled", "v8 after write", err)
	}
	return nil
}
