// SPDX-License-Identifier: Apache-2.0
// Complete input validation precedes creation of the sole official Program.
package main

import (
	"strings"
	"unicode/utf8"
)

func stringFactsPath(name string) bool {
	return strings.HasPrefix(name, "/") && name != "/" && canonicalProjectPath(name) == name && !strings.ContainsAny(name, "\x00\\")
}
func stringFactsInputs(input stringFactsInput) (request, error) {
	var owned request
	root := input.SourceRoot
	dts := root + "/minimal-scalars-v2.d.ts"
	if !stringFactsPath(root) || !stringFactsPath(input.RootFile) || !strings.HasPrefix(input.RootFile, root+"/") ||
		!strings.HasSuffix(input.RootFile, ".ts") || strings.HasSuffix(input.RootFile, ".d.ts") ||
		len(input.Files) != 2 || input.Declarations.Version != "minimal-scalars-v2" || len(input.Declarations.Paths) != 1 || input.Declarations.Paths[0] != dts ||
		!input.Config.Strict || !input.Config.NoEmit || input.Config.AllowJS {
		return owned, stringFactsError("input", "closed source/profile/config", nil)
	}
	if strings.HasPrefix(input.RootFile, dts+"/") || strings.HasPrefix(dts, input.RootFile+"/") {
		return owned, stringFactsError("input", "input files must not be ancestors of another file", nil)
	}
	seen := map[string]bool{}
	for i, file := range input.Files {
		if !stringFactsPath(file.Path) || !strings.HasPrefix(file.Path, root+"/") || seen[file.Path] || i > 0 && strings.Compare(input.Files[i-1].Path, file.Path) >= 0 ||
			file.Path != input.RootFile && file.Path != dts {
			return owned, stringFactsError("input", "distinct complete files in UTF8 path order", nil)
		}
		if !utf8.ValidString(file.Content) {
			return owned, stringFactsError("unicode", "source must be valid UTF8", nil)
		}
		if len(file.Content) > 65536 {
			return owned, stringFactsError("limit", "source UTF8 bytes", nil)
		}
		if len(file.SHA256) != 64 || file.SHA256 != stringFactsSHA([]byte(file.Content)) {
			return owned, stringFactsError("identity", "exact file SHA256", nil)
		}
		if file.Path == dts && file.SHA256 != "d629059f5175a580a8a7b19531c7d8cff1bfa7b61320f6638cf4ee25fbf5a141" {
			return owned, stringFactsError("identity", "pinned scalar DTS", nil)
		}
		seen[file.Path] = true
		owned.Files = append(owned.Files, inputFile{Path: file.Path, Content: file.Content})
	}
	if !seen[input.RootFile] || !seen[dts] {
		return owned, stringFactsError("input", "missing root or declarations", nil)
	}
	owned.SourceRoot, owned.RootFile = root, input.RootFile
	owned.Declarations.Version = input.Declarations.Version
	owned.Declarations.Paths = append([]string(nil), input.Declarations.Paths...)
	owned.Config.Strict, owned.Config.NoEmit, owned.Config.AllowJS = input.Config.Strict, input.Config.NoEmit, input.Config.AllowJS
	return owned, nil
}
