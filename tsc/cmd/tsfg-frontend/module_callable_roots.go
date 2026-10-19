// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
)

func moduleCallableRoots(checked ownedProjectionContext, closure *closureConfiguration) error {
	files := map[string]*ast.SourceFile{}
	for _, file := range checked.SourceFiles {
		if !strings.HasSuffix(file.FileName(), ".d.ts") {
			files[file.FileName()] = file
		}
	}
	modules := map[string]moduleFact{}
	for _, module := range checked.Facts.Modules {
		modules[module.SourcePath] = module
	}
	functions := map[string]function{}
	for _, function := range checked.Facts.Functions {
		functions[function.Symbol] = function
	}
	for _, root := range closure.Roots {
		file := files[root.SourcePath]
		if file == nil {
			return moduleCallableError("identity")
		}
		if root.Kind == "module-initialization" {
			module, found := modules[root.SourcePath]
			if !found || root.Name != "$moduleInit" || !exceptionFactsInitialization(file, module) {
				return moduleCallableError("identity")
			}
			continue
		}
		actual := exceptionFactsRootSymbol(checked, file, root.Name)
		if actual == nil {
			return moduleCallableError("identity")
		}
		canonical := checked.Checker.GetExportSymbolOfSymbol(actual)
		info, found := checked.Functions[actual]
		if !found {
			info, found = checked.Functions[canonical]
		}
		function, owned := functions[info.ID]
		if !found || !owned || info.Declaration == nil || info.Declaration.Kind != ast.KindFunctionDeclaration {
			return moduleCallableError("identity")
		}
		declaration := info.Declaration
		owner := ast.GetSourceFileOfNode(declaration)
		declared := moduleFunctionSymbol(checked.Checker, declaration)
		if owner == nil || files[owner.FileName()] != owner || declaration.Parent != owner.AsNode() || checked.Checker.GetExportSymbolOfSymbol(declared) != canonical || function.SourcePath != owner.FileName() || function.Range != offset(owner, declaration) {
			return moduleCallableError("identity")
		}
	}
	return nil
}
