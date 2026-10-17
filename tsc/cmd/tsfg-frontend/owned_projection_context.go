// SPDX-License-Identifier: Apache-2.0

package main

import (
	"slices"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
)

// Terminal consumers borrow the same checked objects only for this callback.
// SourceFiles includes every consumed TS/DTS file; no compiler object enters a DTO.
type ownedProjectionContext struct {
	Root        *ast.SourceFile
	SourceFiles []*ast.SourceFile
	Checker     *checker.Checker
	Functions   map[*ast.Symbol]functionInfo
	Facts       response
}

type ownedProjectionHook func(ownedProjectionContext)

func checkedProjectionContext(root *ast.SourceFile, program *compiler.Program, check *checker.Checker, functions map[*ast.Symbol]functionInfo, facts response) ownedProjectionContext {
	files := slices.Clone(program.GetSourceFiles())
	slices.SortFunc(files, func(a, b *ast.SourceFile) int { return strings.Compare(a.FileName(), b.FileName()) })
	return ownedProjectionContext{Root: root, SourceFiles: files, Checker: check, Functions: functions, Facts: facts}
}
