// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Explicit originals and imports use only the already-built official Program.
package main

import (
	"slices"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tspath"
)

func classOriginalFiles(program *compiler.Program, files []*ast.SourceFile) ([]*ast.SourceFile, error) {
	if len(files) == 0 || len(files) > 16 || len(program.GetSourceFiles()) > 16 {
		return nil, classError("input", "explicit original source set requires one to sixteen files", classAnchor{})
	}
	owned := map[*ast.SourceFile]bool{}
	for _, file := range files {
		if file == nil || program.GetSourceFile(file.FileName()) != file || file.IsDeclarationFile ||
			file.ScriptKind != core.ScriptKindTS || owned[file] {
			return nil, classError("input", "original source identity, kind or uniqueness", classAnchor{})
		}
		owned[file] = true
	}
	for _, file := range program.GetSourceFiles() {
		if !file.IsDeclarationFile && !owned[file] {
			return nil, classError("input", "explicit originals omit an official Program source", classAnchor{Source: file.FileName()})
		}
	}
	sorted := slices.Clone(files)
	slices.SortFunc(sorted, func(a, b *ast.SourceFile) int { return strings.Compare(a.FileName(), b.FileName()) })
	return sorted, nil
}

func (p *classProjection) importDeclarations(files []*ast.SourceFile) error {
	owned := map[*ast.SourceFile]bool{}
	for _, file := range files {
		owned[file] = true
	}
	for _, file := range files {
		for _, node := range file.Statements.Nodes {
			if err := p.ctx.Err(); err != nil {
				return err
			}
			if node.Kind != ast.KindImportDeclaration {
				continue
			}
			if err := p.namedImport(file, node, owned); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *classProjection) namedImport(file *ast.SourceFile, node *ast.Node, owned map[*ast.SourceFile]bool) error {
	unsupported := func(detail string) error {
		return classError("unsupported", "import "+detail, classNodeAnchor(node))
	}
	declaration := node.AsImportDeclaration()
	if declaration.ImportClause == nil || declaration.Attributes != nil || node.Modifiers() != nil ||
		declaration.ModuleSpecifier == nil || declaration.ModuleSpecifier.Kind != ast.KindStringLiteral {
		return unsupported("requires a static relative named value clause without attributes")
	}
	clause := declaration.ImportClause.AsImportClause()
	if clause.Name() != nil || clause.PhaseModifier != 0 || clause.NamedBindings == nil ||
		clause.NamedBindings.Kind != ast.KindNamedImports {
		return unsupported("requires only named value bindings")
	}
	name := declaration.ModuleSpecifier.Text()
	if !tspath.PathIsRelative(name) {
		return unsupported("requires a relative original source module")
	}
	// This reads the Program's existing resolution cache; it never resolves a
	// path independently or asks a host to search for additional source files.
	resolution := p.program.GetResolvedModuleFromModuleSpecifier(file, declaration.ModuleSpecifier)
	if resolution == nil || !resolution.IsResolved() || resolution.IsExternalLibraryImport {
		return unsupported("has no owned official module resolution")
	}
	targetFile := p.program.GetSourceFile(resolution.ResolvedFileName)
	if targetFile == nil || !owned[targetFile] {
		return unsupported("target has no explicit original source body")
	}
	bindings := clause.NamedBindings.AsNamedImports().Elements
	if bindings == nil || len(bindings.Nodes) == 0 {
		return unsupported("requires at least one named class value binding")
	}
	for _, binding := range bindings.Nodes {
		if err := p.ctx.Err(); err != nil {
			return err
		}
		specifier := binding.AsImportSpecifier()
		if specifier.IsTypeOnly || binding.Name() == nil || binding.Name().Kind != ast.KindIdentifier ||
			specifier.PropertyName != nil && specifier.PropertyName.Kind != ast.KindIdentifier {
			return unsupported("requires ordinary named class value specifiers")
		}
		alias := p.check.GetSymbolAtLocation(binding.Name())
		if alias == nil || alias.Flags&ast.SymbolFlagsAlias == 0 || len(alias.Declarations) != 1 || alias.Declarations[0] != binding {
			return classError("identity", "official import alias declaration differs", classNodeAnchor(node))
		}
		target := p.check.GetAliasedSymbol(alias)
		if target == nil || len(target.Declarations) != 1 || !ast.IsClassDeclaration(target.Declarations[0]) ||
			ast.GetSourceFileOfNode(target.Declarations[0]) != targetFile {
			return unsupported("alias target is not a direct owned class declaration")
		}
		if _, known := p.classes[target]; !known {
			return unsupported("alias target lies outside the owned class closure")
		}
		p.importTargets[alias] = target
	}
	return nil
}

// Only previously validated official import aliases can resolve to a class.
// The canonical class symbol owns the value; the expression keeps its own anchor.
func (p *classProjection) valueSymbol(node *ast.Node) *ast.Symbol {
	symbol := p.check.GetSymbolAtLocation(node)
	if target := p.importTargets[symbol]; target != nil {
		return target
	}
	return symbol
}
