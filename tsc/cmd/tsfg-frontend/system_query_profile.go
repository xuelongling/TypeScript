// SPDX-License-Identifier: Apache-2.0
package main

import (
	_ "embed"
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
)

const systemQueryProfilePath = "/src/system-query-context.d.ts"

//go:embed profiles/system-query-v6.d.ts
var systemQueryDeclarations string

func newOwnedSystemProgram(r request) (*compiler.Program, *ast.SourceFile, []string, *response) {
	found := false
	for _, file := range r.Files {
		if file.Path == systemQueryProfilePath {
			if found || file.Content != systemQueryDeclarations {
				failure := fail("r03-system-query-profile", "input", file.Path, span{})
				return nil, nil, nil, &failure
			}
			found = true
		}
	}
	if !found {
		failure := fail("r03-system-query-profile", "input", systemQueryProfilePath, span{})
		return nil, nil, nil, &failure
	}
	return newOwnedProgramProfile(r, systemQueryProfilePath)
}
