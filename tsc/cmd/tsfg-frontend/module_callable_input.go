// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/json"
)

func moduleCallableRootKey(root closureRoot) string {
	return root.Kind + "\x00" + root.SourcePath + "\x00" + root.Name
}

func moduleCallableInputs(input moduleCallableInput) (request, string, []moduleCallableFile, error) {
	bad := func(kind string) (request, string, []moduleCallableFile, error) {
		return request{}, "", nil, moduleCallableError(kind)
	}
	if len(input.Files) > 65 || input.CompilationClosure != nil && (len(input.CompilationClosure.Roots) > 128 || len(input.CompilationClosure.RequiredRoots) > 128) {
		return bad("limit")
	}
	if len(input.Files) < 2 || input.Files == nil || input.HostImports == nil || len(input.HostImports) != 0 || input.CompilationClosure == nil || input.CompilationClosure.Roots == nil || input.CompilationClosure.RequiredRoots == nil || !exceptionFactsPath(input.SourceRoot) || !exceptionFactsPath(input.RootFile) || input.Declarations.Version != "minimal-integers-v3" || len(input.Declarations.Paths) != 1 || input.Declarations.Paths[0] != input.SourceRoot+"/minimal-integers-v3.d.ts" || !validIntegerTarget(input.Target) {
		return bad("input")
	}
	data, err := moduleCallableCanonical(input)
	if err != nil {
		return request{}, "", nil, err
	}
	var owned request
	if err := json.Unmarshal(data, &owned); err != nil || !valid(owned) || !exceptionFactsClosure(input.CompilationClosure, input.Files) {
		return bad("input")
	}
	for _, roots := range [][]closureRoot{input.CompilationClosure.Roots, input.CompilationClosure.RequiredRoots} {
		previous := ""
		for _, root := range roots {
			key := moduleCallableRootKey(root)
			if previous != "" && key <= previous {
				return bad("input")
			}
			previous = key
		}
	}
	files := make([]moduleCallableFile, 0, len(input.Files))
	declarationCount := 0
	for i, file := range input.Files {
		if !exceptionFactsPath(file.Path) || !strings.HasPrefix(file.Path, input.SourceRoot+"/") || i > 0 && file.Path <= input.Files[i-1].Path {
			return bad("input")
		}
		digest := moduleCallableHash([]byte(file.Content))
		if len(file.SHA256) != 64 || digest != "sha256:"+file.SHA256 {
			return bad("identity")
		}
		if strings.HasSuffix(file.Path, ".d.ts") {
			declarationCount++
			if file.Path != input.Declarations.Paths[0] || digest != "sha256:9fc1955899e303b5815590f7150f60662f3cece0a633c94a2a06669cf509a96e" {
				return bad("identity")
			}
		} else if !strings.HasSuffix(file.Path, ".ts") {
			return bad("input")
		}
		files = append(files, moduleCallableFile{Path: file.Path, Digest: digest, ByteLength: len(file.Content), Length: len(utf16.Encode([]rune(file.Content))), PositionUnit: "utf16"})
	}
	if declarationCount != 1 {
		return bad("input")
	}
	identity, err := moduleCallableDigest("tsfg.module-callable-facts.input.v2", input)
	if err != nil {
		return request{}, "", nil, err
	}
	owned.moduleCallable = true
	return owned, identity, files, nil
}
