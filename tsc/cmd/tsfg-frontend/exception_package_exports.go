// SPDX-License-Identifier: Apache-2.0

package main

import (
	"slices"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
)

func projectExceptionFactsExports(checked ownedProjectionContext) ([]exceptionFactsExportGroup, error) {
	functions := map[string]function{}
	for _, fn := range checked.Facts.Functions {
		functions[fn.Symbol] = fn
	}
	symbols := map[string]symbol{}
	for _, binding := range checked.Facts.Symbols {
		symbols[binding.ID] = binding
	}
	groups := []exceptionFactsExportGroup{}
	for _, file := range checked.SourceFiles {
		if strings.HasSuffix(file.FileName(), ".d.ts") {
			continue
		}
		group := exceptionFactsExportGroup{SourcePath: file.FileName(), External: ast.IsExternalModule(file), Bindings: []exceptionFactsExport{}}
		if !group.External {
			groups = append(groups, group)
			continue
		}
		if file.AsNode().Symbol() == nil {
			return nil, exceptionFactsError("export", "missing actual module symbol")
		}
		topLevel := map[*ast.Node]bool{}
		for _, declaration := range file.Statements.Nodes {
			topLevel[declaration] = true
		}
		names := map[string]bool{}
		for _, exported := range checked.Checker.GetExportsOfModule(file.AsNode().Symbol()) {
			if exported == nil || exported.Flags&ast.SymbolFlagsAlias != 0 || len(exported.Declarations) != 1 ||
				checked.Checker.GetTypeOnlyAliasDeclaration(exported) != nil {
				return nil, exceptionFactsError("export", "alias/type-only/ambiguous export")
			}
			declaration := exported.Declarations[0]
			if !topLevel[declaration] || ast.GetSourceFileOfNode(declaration) != file || declaration.Kind != ast.KindFunctionDeclaration ||
				declaration.Name() == nil || exported.Name != declaration.Name().Text() || names[exported.Name] {
				return nil, exceptionFactsError("export", "foreign/nonfunction/renamed export declaration")
			}
			// The actual declaration joins export and local symbols; a same-spelled
			// lookup or the public filtered module export table cannot prove identity.
			actual := checked.Checker.GetSymbolAtLocation(declaration.Name())
			info, found := checked.Functions[actual]
			fn, hasFunction := functions[info.ID]
			binding, hasSymbol := symbols[info.ID]
			if actual == nil || !found || info.ID == "" || info.Declaration != declaration || !hasFunction || !hasSymbol ||
				binding.Kind != "function" || binding.Name != exported.Name || binding.Writable || binding.Type != fn.ReturnType ||
				fn.Name != exported.Name || fn.Range != offset(file, declaration) || binding.Range != offset(file, declaration.Name()) ||
				fn.SourcePath != "" && fn.SourcePath != file.FileName() || binding.SourcePath != "" && binding.SourcePath != file.FileName() {
				return nil, exceptionFactsError("export", "missing actual owned function/symbol declaration")
			}
			names[exported.Name] = true
			group.Bindings = append(group.Bindings, exceptionFactsExport{Name: exported.Name, Target: info.ID,
				SourcePath: file.FileName(), Range: offset(file, declaration), Definition: offset(file, declaration.Name())})
		}
		slices.SortFunc(group.Bindings, func(a, b exceptionFactsExport) int { return strings.Compare(a.Name, b.Name) })
		groups = append(groups, group)
	}
	if len(groups) == 0 {
		return nil, exceptionFactsError("input", "no actual non-DTS source")
	}
	slices.SortFunc(groups, func(a, b exceptionFactsExportGroup) int { return strings.Compare(a.SourcePath, b.SourcePath) })
	return groups, nil
}
