// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
)

// Root tuples select actual checked declarations, not similarly named DTOs.
// This runs while the single terminal callback still borrows its Program.
func exceptionFactsRoots(checked ownedProjectionContext, closure *closureConfiguration) error {
	if closure == nil {
		return nil
	}
	files := map[string]*ast.SourceFile{}
	for _, file := range checked.SourceFiles {
		if !strings.HasSuffix(file.FileName(), ".d.ts") {
			files[file.FileName()] = file
		}
	}
	modules := map[string]moduleFact{}
	for _, module := range checked.Facts.Modules {
		if _, duplicate := modules[module.SourcePath]; duplicate {
			return exceptionFactsError("input", "duplicate actual root source module")
		}
		modules[module.SourcePath] = module
	}
	functions := map[string]function{}
	for _, fn := range checked.Facts.Functions {
		functions[fn.Symbol] = fn
	}
	bindings := map[string]symbol{}
	for _, binding := range checked.Facts.Symbols {
		bindings[binding.ID] = binding
	}
	for _, root := range closure.Roots {
		file := files[root.SourcePath]
		if file == nil {
			return exceptionFactsError("input", "closure root has no actual consumed source")
		}
		if root.Kind == "module-initialization" {
			module, found := modules[root.SourcePath]
			if root.Name != "$moduleInit" || !found || !exceptionFactsInitialization(file, module) {
				return exceptionFactsError("input", "closure root has no actual source initialization")
			}
			continue
		}
		actual := exceptionFactsRootSymbol(checked, file, root.Name)
		info, found := checked.Functions[actual]
		fn, hasFunction := functions[info.ID]
		binding, hasBinding := bindings[info.ID]
		declaration := info.Declaration
		if actual == nil || !found || !hasFunction || !hasBinding || declaration == nil ||
			declaration.Kind != ast.KindFunctionDeclaration || declaration.Body() == nil {
			return exceptionFactsError("input", "closure root has no actual owned function declaration")
		}
		owner := ast.GetSourceFileOfNode(declaration)
		if owner == nil || files[owner.FileName()] != owner || declaration.Parent != owner.AsNode() ||
			moduleFunctionSymbol(checked.Checker, declaration) != actual || info.ID == "" ||
			fn.SourcePath != owner.FileName() || binding.SourcePath != owner.FileName() ||
			fn.Name != moduleFunctionName(declaration) || binding.Name != fn.Name ||
			binding.Kind != "function" || binding.Writable || binding.Type != fn.ReturnType ||
			fn.Range != offset(owner, declaration) || binding.Range != moduleFunctionAnchor(owner, declaration) {
			return exceptionFactsError("input", "closure root function binding has foreign declaration ownership")
		}
	}
	return nil
}

// The existing root contract prefers a module's actual export binding, then a
// named source function. Alias targets come from the same official checker.
func exceptionFactsRootSymbol(checked ownedProjectionContext, file *ast.SourceFile, name string) *ast.Symbol {
	if ast.IsExternalModule(file) && file.AsNode().Symbol() != nil {
		for _, exported := range checked.Checker.GetExportsOfModule(file.AsNode().Symbol()) {
			if exported.Name == name {
				if checked.Checker.GetTypeOnlyAliasDeclaration(exported) != nil {
					return nil
				}
				return moduleAliasTarget(checked.Checker, exported)
			}
		}
	}
	for _, declaration := range file.Statements.Nodes {
		if declaration.Kind == ast.KindFunctionDeclaration && declaration.Name() != nil && declaration.Name().Text() == name {
			return moduleFunctionSymbol(checked.Checker, declaration)
		}
	}
	return nil
}

// Module initialization is owned by this exact actual source, even when its
// initialization is empty. Every emitted top-level statement retains an anchor
// of that source's own node (variable statements emit declaration anchors).
func exceptionFactsInitialization(file *ast.SourceFile, module moduleFact) bool {
	if module.SourcePath != file.FileName() || module.External != ast.IsExternalModule(file) {
		return false
	}
	anchors := map[span]bool{}
	for _, node := range file.Statements.Nodes {
		switch node.Kind {
		case ast.KindFunctionDeclaration, ast.KindImportDeclaration, ast.KindExportDeclaration:
			continue
		case ast.KindVariableStatement:
			for _, declaration := range localDeclarations(node) {
				anchors[offset(file, declaration)] = true
			}
		default:
			anchors[offset(file, node)] = true
			anchors[expressionOffset(file, node)] = true
		}
	}
	for _, item := range module.Initialization {
		if !anchors[item.Range] {
			return false
		}
	}
	return true
}
