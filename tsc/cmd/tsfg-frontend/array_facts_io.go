// SPDX-License-Identifier: Apache-2.0
// Closed input identity and one-shot publication bracket the real official projection.
package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/microsoft/typescript-go/internal/json"
)

// Count legal request arrays before the decoder allocates typed file/path slices.
func arrayFactsRequestArrays(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	containers := []int{}
	for {
		token, err := decoder.ReadToken()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return arrayFactsError("json", "request array boundary", err)
		}
		kind, last := token.Kind(), len(containers)-1
		if kind != '}' && kind != ']' && last >= 0 && containers[last] >= 0 {
			containers[last]++
			if containers[last] > 2 {
				return arrayFactsError("limit", "closed request array cardinality", nil)
			}
		}
		switch kind {
		case '[':
			containers = append(containers, 0)
		case '{':
			containers = append(containers, -1)
		case ']', '}':
			if last < 0 {
				return arrayFactsError("json", "request container boundary", nil)
			}
			containers = containers[:last]
		}
	}
}
func arrayFactsPath(path string) bool {
	return strings.HasPrefix(path, "/") && path != "/" && canonicalProjectPath(path) == path && !strings.ContainsAny(path, "\x00\\")
}
func arrayFactsInputs(input arrayFactsInput) (request, error) {
	var owned request
	root, dts := input.SourceRoot, input.SourceRoot+"/minimal-arrays-v3.d.ts"
	if !arrayFactsPath(root) || !arrayFactsPath(input.RootFile) || !strings.HasPrefix(input.RootFile, root+"/") || !strings.HasSuffix(input.RootFile, ".ts") || strings.HasSuffix(input.RootFile, ".d.ts") || len(input.Files) != 2 || input.Declarations.Version != "minimal-arrays-v3" || len(input.Declarations.Paths) != 1 || input.Declarations.Paths[0] != dts || !input.Config.Strict || !input.Config.NoEmit || input.Config.AllowJS {
		return owned, arrayFactsError("input", "closed source/profile/config", nil)
	}
	if strings.HasPrefix(input.RootFile, dts+"/") || strings.HasPrefix(dts, input.RootFile+"/") {
		return owned, arrayFactsError("input", "input paths must not be ancestors", nil)
	}
	seen := map[string]bool{}
	for i, file := range input.Files {
		if !arrayFactsPath(file.Path) || !strings.HasPrefix(file.Path, root+"/") || seen[file.Path] || i > 0 && strings.Compare(input.Files[i-1].Path, file.Path) >= 0 || file.Path != input.RootFile && file.Path != dts {
			return owned, arrayFactsError("input", "complete distinct files in UTF8 path order", nil)
		}
		if !utf8.ValidString(file.Content) {
			return owned, arrayFactsError("json", "source must be valid UTF8", nil)
		}
		if len(file.Content) > 65536 {
			return owned, arrayFactsError("limit", "source UTF8 bytes", nil)
		}
		if len(file.SHA256) != 64 || file.SHA256 != arrayFactsSHA([]byte(file.Content)) {
			return owned, arrayFactsError("identity", "exact source SHA256", nil)
		}
		if file.Path == dts && (file.SHA256 != arrayFactsDtsSHA || file.Content != string(arrayFactsDtsBytes)) {
			return owned, arrayFactsError("identity", "exact pinned Array declarations", nil)
		}
		seen[file.Path] = true
		owned.Files = append(owned.Files, inputFile{Path: file.Path, Content: file.Content})
	}
	if !seen[input.RootFile] || !seen[dts] {
		return owned, arrayFactsError("input", "missing root or declaration", nil)
	}
	owned.SourceRoot, owned.RootFile = root, input.RootFile
	owned.Declarations.Version = input.Declarations.Version
	owned.Declarations.Paths = append([]string(nil), input.Declarations.Paths...)
	owned.Config.Strict, owned.Config.NoEmit, owned.Config.AllowJS = true, true, false
	return owned, nil
}

type arrayFactsReader struct {
	ctx   context.Context
	input io.Reader
}

func (r arrayFactsReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.input.Read(p)
	if canceled := r.ctx.Err(); canceled != nil {
		return n, errors.Join(err, canceled)
	}
	return n, err
}
func runArrayFactsCandidate(ctx context.Context, input io.Reader, output io.Writer) error {
	if err := ctx.Err(); err != nil {
		return arrayFactsError("canceled", "before read", err)
	}
	data, err := io.ReadAll(io.LimitReader(arrayFactsReader{ctx, input}, arrayFactsMaxBytes+1))
	if canceled := ctx.Err(); canceled != nil {
		return arrayFactsError("canceled", "after read", errors.Join(err, canceled))
	}
	if err != nil {
		return arrayFactsError("io", "request read", err)
	}
	r, err := decodeArrayFactsRequest(data)
	if err != nil {
		return err
	}
	owned, err := arrayFactsInputs(r.Input)
	if err != nil {
		return err
	}
	payload, err := projectArrayFacts(ctx, owned, r.Input)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return arrayFactsError("canceled", "after checking", err)
	}
	// Check the entire future packet before either canonicalization or identity hashing.
	packet := arrayFactsPackage{Family: "tsfg.array-facts-package-candidate", Version: 6, SchemaHash: arrayFactsSchema, RequestID: "sha256:" + strings.Repeat("0", 64), PackageID: "sha256:" + strings.Repeat("0", 64), Payload: payload}
	if err := arrayFactsOutputBudget(packet); err != nil {
		return err
	}
	payloadBytes, err := arrayFactsCanonical(payload)
	if err != nil {
		return err
	}
	packet.RequestID = "sha256:" + arrayFactsSHA(append([]byte("tsfg.array-facts.request.v6\x00"), data...))
	packet.PackageID = "sha256:" + arrayFactsSHA(append([]byte("tsfg.array-facts.package.v6\x00"+arrayFactsSchema+"\x00"), payloadBytes...))
	encoded, err := arrayFactsCanonical(packet)
	if err != nil {
		return err
	}
	if len(encoded) >= arrayFactsMaxBytes {
		return arrayFactsError("limit", "complete output bytes", nil)
	}
	encoded = append(encoded, '\n')
	if err := ctx.Err(); err != nil {
		return arrayFactsError("canceled", "before write", err)
	}
	n, err := output.Write(encoded)
	if err != nil {
		return arrayFactsError("io", "response write", err)
	}
	if n != len(encoded) {
		return arrayFactsError("io", "response short write", io.ErrShortWrite)
	}
	if err := ctx.Err(); err != nil {
		return arrayFactsError("canceled", "after write", err)
	}
	return nil
}
