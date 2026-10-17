// SPDX-License-Identifier: Apache-2.0

package main

import (
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/json"
)

func exceptionFactsProfile(version string) (string, string) {
	switch version {
	case "minimal-scalars-v2":
		return "minimal-scalars-v2.d.ts", "sha256:d629059f5175a580a8a7b19531c7d8cff1bfa7b61320f6638cf4ee25fbf5a141"
	case "minimal-integers-v3":
		return "minimal-integers-v3.d.ts", "sha256:9fc1955899e303b5815590f7150f60662f3cece0a633c94a2a06669cf509a96e"
	case "minimal-floats-v4":
		return "minimal-floats-v4.d.ts", "sha256:edb7bfe80718ba47175a87a65f2688f1b91420995e7c118dd2c3e4c85c193693"
	case "minimal-conversions-v5":
		return "minimal-conversions-v5.d.ts", "sha256:60974c1fa82cc549da3a9268f5d82ab10b58f52e2612a377ee4577f05af52c86"
	}
	return "", ""
}

func exceptionFactsPath(name string) bool {
	return strings.HasPrefix(name, "/") && canonicalProjectPath(name) == name &&
		!strings.ContainsAny(name, "\x00\\") && name != "/"
}

func exceptionFactsClosure(closure *closureConfiguration, files []inputFile) bool {
	if closure == nil {
		return true
	}
	if len(closure.Roots) == 0 || len(closure.Roots) > 128 || len(closure.RequiredRoots) > 128 {
		return false
	}
	sources := map[string]bool{}
	for _, file := range files {
		if !strings.HasSuffix(file.Path, ".d.ts") {
			sources[file.Path] = true
		}
	}
	roots := map[closureRoot]bool{}
	for _, root := range closure.Roots {
		switch root.Kind {
		case "application", "host-export", "host-callback", "hot-function", "kernel":
		case "module-initialization":
			if root.Name != "$moduleInit" {
				return false
			}
		default:
			return false
		}
		if !sources[root.SourcePath] || root.Name == "" || strings.ContainsRune(root.Name, 0) || roots[root] {
			return false
		}
		roots[root] = true
	}
	required := map[closureRoot]bool{}
	for _, root := range closure.RequiredRoots {
		if !roots[root] || required[root] {
			return false
		}
		required[root] = true
	}
	return true
}

func exceptionFactsInputs(input exceptionFactsInput) (request, string, []exceptionFactsFile, error) {
	data, err := exceptionFactsCanonical(input)
	if err != nil {
		return request{}, "", nil, err
	}
	// The analyzer mutates Target.floats; its complete request and all pointer/
	// slice members must be independently owned before any checker is created.
	var owned request
	if err := json.Unmarshal(data, &owned); err != nil {
		return request{}, "", nil, exceptionFactsError("schema", "legacy input conversion")
	}
	primary, primaryDigest := exceptionFactsProfile(input.Declarations.Version)
	if primary == "" || !valid(owned) || len(input.Files) > 256 || !exceptionFactsPath(input.SourceRoot) ||
		!exceptionFactsPath(input.RootFile) || !exceptionFactsClosure(input.CompilationClosure, input.Files) ||
		input.Declarations.Version == "minimal-scalars-v2" && input.Target != nil {
		return request{}, "", nil, exceptionFactsError("input", "profile/target/config/source closure")
	}
	primaryPath := input.SourceRoot + "/" + primary
	markerPath := input.SourceRoot + "/minimal-aggregates-v1.d.ts"
	declarationCount := 1
	if input.Declarations.MarkerProfile != nil {
		declarationCount++
	}
	if len(input.Declarations.Paths) != declarationCount || input.Declarations.Paths[0] != primaryPath ||
		declarationCount == 2 && input.Declarations.Paths[1] != markerPath {
		return request{}, "", nil, exceptionFactsError("input", "complete pinned declaration paths")
	}
	identity := input
	identity.Files = slices.Clone(input.Files)
	slices.SortFunc(identity.Files, func(a, b inputFile) int { return strings.Compare(a.Path, b.Path) })
	files := make([]exceptionFactsFile, 0, len(identity.Files))
	for i, file := range identity.Files {
		if !exceptionFactsPath(file.Path) {
			return request{}, "", nil, exceptionFactsError("input", "canonical logical file path")
		}
		digest := exceptionFactsDigest([]byte(file.Content))
		if file.SHA256 != "" && file.SHA256 != digest {
			return request{}, "", nil, exceptionFactsError("identity", "content checksum")
		}
		if strings.HasSuffix(file.Path, ".d.ts") {
			expected := primaryDigest
			if file.Path == markerPath && declarationCount == 2 {
				expected = markerHash()
			} else if file.Path != primaryPath {
				return request{}, "", nil, exceptionFactsError("input", "undeclared DTS input")
			}
			if digest != expected {
				return request{}, "", nil, exceptionFactsError("identity", "pinned declaration bytes")
			}
		} else if exceptionFactsSourceTokenCount(file.Content) > exceptionFactsSourceTokenLimit {
			return request{}, "", nil, exceptionFactsError("limit", "source tokens per file")
		}
		identity.Files[i].SHA256 = ""
		files = append(files, exceptionFactsFile{Path: file.Path, Digest: digest, ByteLength: len(file.Content),
			Length: len(utf16.Encode([]rune(file.Content))), PositionUnit: "utf16"})
	}
	identityBytes, err := exceptionFactsCanonical(identity)
	if err != nil {
		return request{}, "", nil, err
	}
	return owned, exceptionFactsDigest(identityBytes), files, nil
}
