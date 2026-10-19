// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"sort"
)

// Complete runtime exports are independently selected from the official module
// symbols, then checked against hand-owned original declaration targets.
func (o *mcIndependent23d) modules(recipes []mcFileRecipe23d) ([]any, []any) {
	modules, exports := []any{}, []any{}
	for _, recipe := range recipes {
		file := o.files[recipe.Path]
		imported, initialized, globalIDs := []any{}, []any{}, []any{}
		importIndex, globalIndex := 0, 0
		for _, node := range file.Statements.Nodes {
			switch node.Kind {
			case ast.KindImportDeclaration, ast.KindExportDeclaration:
				if importIndex >= len(recipe.Imports) {
					o.t.Fatal("unlisted original static dependency")
				}
				expected := recipe.Imports[importIndex]
				importIndex++
				var moduleSpecifier *ast.Node
				bindings := []*ast.Node{}
				kind := "import"
				if node.Kind == ast.KindImportDeclaration {
					declaration := node.AsImportDeclaration()
					moduleSpecifier = declaration.ModuleSpecifier
					if declaration.ImportClause == nil {
						o.t.Fatal("frozen import clause missing")
					}
					clause := declaration.ImportClause.AsImportClause()
					if clause.Name() != nil || clause.NamedBindings == nil || clause.NamedBindings.Kind != ast.KindNamedImports {
						o.t.Fatal("independent named import shape differs")
					}
					for _, element := range clause.NamedBindings.AsNamedImports().Elements.Nodes {
						bindings = append(bindings, element.Name())
					}
				} else {
					kind = "export"
					moduleSpecifier = node.AsExportDeclaration().ModuleSpecifier
					if (node.AsExportDeclaration().ExportClause == nil) != expected.Star {
						o.t.Fatal("original star/named reexport syntax differs")
					}
				}
				if kind != expected.Kind || moduleSpecifier == nil || moduleSpecifier.Text() != expected.Specifier || len(bindings) != len(expected.Bindings) {
					o.t.Fatal("independent original import/reexport differs")
				}
				resolved := o.check.ResolveExternalModuleName(moduleSpecifier)
				if resolved == nil || len(resolved.Declarations) == 0 || ast.GetSourceFileOfNode(resolved.Declarations[0]).FileName() != expected.SourcePath {
					o.t.Fatal("official module dependency differs from frozen original target")
				}
				rows := []any{}
				for index, binding := range bindings {
					expectedBinding := expected.Bindings[index]
					if o.spelling(file, binding) != expectedBinding.Name {
						o.t.Fatal("original import binding spelling differs")
					}
					original := o.lookup(binding, expectedBinding.Target)
					rows = append(rows, mcObject23d{"name": expectedBinding.Name, "target": original["id"], "range": o.anchor(file, binding, true)})
				}
				imported = append(imported, mcObject23d{"kind": kind, "specifier": expected.Specifier, "sourcePath": expected.SourcePath, "range": o.anchor(file, node, false), "bindings": rows})
			case ast.KindVariableStatement:
				declarations := node.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
				if len(declarations) != 1 || globalIndex >= len(recipe.Globals) {
					o.t.Fatal("complete global initializer recipe differs")
				}
				expected := recipe.Globals[globalIndex]
				globalIndex++
				declaration := declarations[0]
				symbol := o.lookup(declaration.Name(), recipe.Path+"#"+expected.Name)
				globalIDs = append(globalIDs, symbol["id"])
				initialized = append(initialized, mcObject23d{"kind": "initialize", "range": o.anchor(file, declaration, false), "symbol": symbol, "initializer": o.expression(file, declaration.AsVariableDeclaration().Initializer, expected.Value)})
			case ast.KindFunctionDeclaration:
			default:
				o.t.Fatal("unlisted original complete module statement")
			}
		}
		if importIndex != len(recipe.Imports) || globalIndex != len(recipe.Globals) {
			o.t.Fatal("original module coverage differs")
		}
		expectedExports := map[string]string{}
		for _, export := range recipe.Exports {
			if _, exists := expectedExports[export.Name]; exists {
				o.t.Fatal("independent recipe repeats export")
			}
			expectedExports[export.Name] = export.Target
		}
		officialExports := o.check.GetExportsOfModule(file.AsNode().Symbol())
		if len(officialExports) != len(expectedExports) {
			o.t.Fatal("complete official runtime export set differs")
		}
		sort.Slice(officialExports, func(i, j int) bool { return officialExports[i].Name < officialExports[j].Name })
		rawExports, exportOrigins := []any{}, []any{}
		for _, export := range officialExports {
			expectedTarget, exists := expectedExports[export.Name]
			if !exists {
				o.t.Fatal("unlisted original official export")
			}
			original := o.original(export)
			symbol := o.rows[original]
			declaration := o.declarations[original]
			if symbol == nil || declaration == nil || o.keys[original] != expectedTarget {
				o.t.Fatal("official reexport lost original declaration identity")
			}
			var site *ast.Node
			for _, candidate := range export.Declarations {
				if ast.GetSourceFileOfNode(candidate) == file {
					site = candidate
					break
				}
			}
			originSite := site
			starCount := 0
			for _, dependency := range recipe.Imports {
				if dependency.Star {
					starCount++
				}
			}
			if starCount > 0 {
				originSite = o.firstResolvingStar(file, export.Name, original, starCount)
				if site == nil {
					site = file.AsNode()
				} // Existing raw module facts retain their whole-source fallback.
			}
			if site == nil || originSite == nil {
				o.t.Fatal("independent original exporting site missing")
			}
			rawExports = append(rawExports, mcObject23d{"name": export.Name, "target": symbol["id"], "range": o.anchor(file, site, false)})
			originFile := ast.GetSourceFileOfNode(declaration)
			exportOrigins = append(exportOrigins, mcObject23d{"name": export.Name, "target": symbol["id"], "range": o.anchor(file, originSite, true), "originSourcePath": originFile.FileName(), "definitionRange": o.anchor(originFile, declaration, false), "nameRange": o.anchor(originFile, declaration.Name(), false)})
		}
		external := ast.IsExternalModule(file)
		if !external {
			o.t.Fatal("frozen source must remain an external module")
		}
		modules = append(modules, mcObject23d{"sourcePath": recipe.Path, "external": external, "imports": imported, "exports": rawExports, "globals": globalIDs, "initialization": initialized})
		exports = append(exports, mcObject23d{"sourcePath": recipe.Path, "external": external, "bindings": exportOrigins})
	}
	return modules, exports
}
