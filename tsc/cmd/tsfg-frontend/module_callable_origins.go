// SPDX-License-Identifier: Apache-2.0

package main

import (
	"slices"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
)

type moduleCallableOrigin struct {
	id   string
	node *ast.Node
	file *ast.SourceFile
}

func moduleCallableOrigins(checked ownedProjectionContext) ([]moduleCallableAnnotation, []moduleCallableExportGroup, error) {
	registry := map[*ast.Symbol]moduleCallableOrigin{}
	byID := map[string]moduleCallableOrigin{}
	bindings := map[string]symbol{}
	for _, binding := range checked.Facts.Symbols {
		bindings[binding.ID] = binding
	}
	for _, file := range checked.SourceFiles {
		if strings.HasSuffix(file.FileName(), ".d.ts") {
			continue
		}
		for _, statement := range file.Statements.Nodes {
			declarations := []*ast.Node{}
			switch statement.Kind {
			case ast.KindFunctionDeclaration:
				declarations = append(declarations, statement)
			case ast.KindVariableStatement:
				declarations = localDeclarations(statement)
			}
			for _, declaration := range declarations {
				actual := checked.Checker.GetSymbolAtLocation(declaration.Name())
				if actual == nil {
					return nil, nil, moduleCallableError("source")
				}
				id := ""
				if declaration.Kind == ast.KindFunctionDeclaration {
					id = checked.Functions[actual].ID
				} else {
					// Original declaration/name ranges join the producer-owned registry. A
					// same-spelled alias elsewhere is never sufficient to select this binding.
					for _, binding := range checked.Facts.Symbols {
						if binding.Kind == "global" && binding.SourcePath == file.FileName() && binding.Range == offset(file, declaration.Name()) {
							if id != "" {
								return nil, nil, moduleCallableError("source")
							}
							id = binding.ID
						}
					}
				}
				binding, found := bindings[id]
				if id == "" || !found || binding.SourcePath != file.FileName() || binding.Range != offset(file, declaration.Name()) || binding.Name != declaration.Name().Text() {
					return nil, nil, moduleCallableError("source")
				}
				origin := moduleCallableOrigin{id: id, node: declaration, file: file}
				actual = checked.Checker.GetExportSymbolOfSymbol(actual)
				if _, duplicate := registry[actual]; duplicate {
					return nil, nil, moduleCallableError("source")
				}
				registry[actual] = origin
				byID[id] = origin
			}
		}
	}
	annotations := make([]moduleCallableAnnotation, 0, len(checked.Facts.Functions))
	for _, function := range checked.Facts.Functions {
		origin, found := byID[function.Symbol]
		if !found || origin.node.Kind != ast.KindFunctionDeclaration || function.Range != offset(origin.file, origin.node) {
			return nil, nil, moduleCallableError("source")
		}
		declaration := origin.node.AsFunctionDeclaration()
		if len(function.Parameters) != len(declaration.Parameters.Nodes) {
			return nil, nil, moduleCallableError("source")
		}
		row := moduleCallableAnnotation{Symbol: function.Symbol, SourcePath: origin.file.FileName(), DeclarationRange: offset(origin.file, origin.node), NameRange: offset(origin.file, origin.node.Name()), ReturnAnnotationRange: expressionOffset(origin.file, declaration.Type), Parameters: []moduleCallableParameterAnnotation{}}
		for i, node := range declaration.Parameters.Nodes {
			parameter := function.Parameters[i]
			if parameter.Range != offset(origin.file, node.Name()) || parameter.SourcePath != origin.file.FileName() {
				return nil, nil, moduleCallableError("source")
			}
			row.Parameters = append(row.Parameters, moduleCallableParameterAnnotation{Symbol: parameter.ID, DeclarationRange: offset(origin.file, node), NameRange: offset(origin.file, node.Name()), AnnotationRange: expressionOffset(origin.file, node.AsParameterDeclaration().Type)})
		}
		annotations = append(annotations, row)
	}
	exports := []moduleCallableExportGroup{}
	for _, file := range checked.SourceFiles {
		if strings.HasSuffix(file.FileName(), ".d.ts") {
			continue
		}
		group := moduleCallableExportGroup{SourcePath: file.FileName(), External: ast.IsExternalModule(file), Bindings: []moduleCallableExport{}}
		if group.External {
			for _, exported := range checked.Checker.GetExportsOfModule(file.AsNode().Symbol()) {
				actual := moduleAliasTarget(checked.Checker, exported)
				if actual == nil || checked.Checker.GetTypeOnlyAliasDeclaration(exported) != nil {
					return nil, nil, moduleCallableError("source")
				}
				actual = checked.Checker.GetExportSymbolOfSymbol(actual)
				origin, found := registry[actual]
				if !found {
					return nil, nil, moduleCallableError("source")
				}
				site := moduleCallableExportSite(checked, file, exported, actual)
				if site == nil {
					return nil, nil, moduleCallableError("source")
				}
				group.Bindings = append(group.Bindings, moduleCallableExport{Name: exported.Name, Target: origin.id, Range: expressionOffset(file, site), OriginSourcePath: origin.file.FileName(), DefinitionRange: offset(origin.file, origin.node), NameRange: offset(origin.file, origin.node.Name())})
			}
			slices.SortFunc(group.Bindings, func(a, b moduleCallableExport) int { return strings.Compare(a.Name, b.Name) })
		}
		exports = append(exports, group)
	}
	return annotations, exports, nil
}

func moduleCallableExportSite(checked ownedProjectionContext, file *ast.SourceFile, exported, actual *ast.Symbol) *ast.Node {
	for _, declaration := range exported.Declarations {
		if ast.GetSourceFileOfNode(declaration) == file {
			return declaration
		}
	}
	// Star reexports can retain only the originating declaration on the official
	// symbol. Choose the first source-order star edge that resolves this identity.
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindExportDeclaration {
			continue
		}
		declaration := node.AsExportDeclaration()
		if declaration.ExportClause != nil || declaration.ModuleSpecifier == nil || declaration.IsTypeOnly {
			continue
		}
		module := checked.Checker.ResolveExternalModuleName(declaration.ModuleSpecifier)
		if module == nil {
			continue
		}
		for _, candidate := range checked.Checker.GetExportsOfModule(module) {
			if candidate.Name != exported.Name {
				continue
			}
			target := moduleAliasTarget(checked.Checker, candidate)
			if target != nil && checked.Checker.GetExportSymbolOfSymbol(target) == actual {
				return node
			}
		}
	}
	return nil
}
