// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// The candidate projection owns its output after official checking is released.
package main

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
)

type classProjection struct {
	importTargets     map[*ast.Symbol]*ast.Symbol
	localSlots        map[*ast.Symbol]classLocalSlot
	localOrdinals     map[*ast.Node]int
	scalars           *scalarAdmissionFacts
	scalarKinds       map[string]string
	literalTypes      int
	methodRefs        map[*ast.Node]classMethodLocation
	constructorNodes  map[int]*ast.Node
	expressions       int
	ctx               context.Context
	program           *compiler.Program
	check             *checker.Checker
	graph             classGraph
	classes           map[*ast.Symbol]int
	types             map[*checker.Type]string
	bindings          map[*ast.Symbol]string
	resources         *classResources
	environmentDigest string
}

// exportClassGraph is the Source-to-owned seam. It does not publish a contract,
// admit a binary, or change the existing stdin/stdout command.
func exportClassGraph(ctx context.Context, program *compiler.Program, files []*ast.SourceFile, options classExportOptions) (classGraph, error) {
	if err := ctx.Err(); err != nil {
		return classGraph{}, err
	}
	if program == nil || options.Scope == "" || len(options.Scope) > 256 ||
		!utf8.ValidString(options.Scope) || strings.ContainsRune(options.Scope, 0) ||
		options.MaxNodes < 1 || options.MaxNodes > 100000 || options.MaxDepth < 1 || options.MaxDepth > 128 {
		return classGraph{}, classError("input", "scope or explicit projection budget", classAnchor{})
	}
	originals, err := classOriginalFiles(program, files)
	if err != nil {
		return classGraph{}, err
	}
	file := originals[0]
	config := program.Options()
	if config.Strict != core.TSTrue || config.StrictNullChecks == core.TSFalse || config.NoImplicitAny == core.TSFalse ||
		config.NoEmit != core.TSTrue || config.NoLib != core.TSTrue || config.NoCheck == core.TSTrue ||
		config.SkipLibCheck == core.TSTrue || config.SkipDefaultLibCheck == core.TSTrue || config.AllowJs == core.TSTrue ||
		config.Target != core.ScriptTargetESNext || config.Module != core.ModuleKindESNext {
		return classGraph{}, classError("input", "official options differ from class projection semantics", classAnchor{})
	}
	resources := &classResources{ctx: ctx, maxNodes: options.MaxNodes, maxDepth: options.MaxDepth}
	ownedOptions, err := classOfficialOptions(resources, config)
	if err != nil {
		return classGraph{}, err
	}
	p := &classProjection{ctx: ctx, resources: resources, program: program, classes: map[*ast.Symbol]int{},
		importTargets: map[*ast.Symbol]*ast.Symbol{},
		localSlots:    map[*ast.Symbol]classLocalSlot{}, localOrdinals: map[*ast.Node]int{},
		scalarKinds:      map[string]string{"t:number": "f64", "t:boolean": "bool"},
		methodRefs:       map[*ast.Node]classMethodLocation{},
		constructorNodes: map[int]*ast.Node{},
		types:            map[*checker.Type]string{}, bindings: map[*ast.Symbol]string{},
		graph: classGraph{Scope: options.Scope, Sources: []classSource{}, Types: []classType{}, Symbols: []classSymbol{}, Classes: []classDeclaration{},
			SemanticProfile: classSemanticProfile, Options: ownedOptions},
	}
	if err := p.inputs(options.Inputs); err != nil {
		return classGraph{}, err
	}
	if err := p.initializeEnvironment(); err != nil {
		return classGraph{}, err
	}
	if err := checkClassDiagnostics(ctx, program); err != nil {
		return classGraph{}, err
	}
	check, release := program.GetTypeCheckerForFileExclusive(ctx, file)
	defer release()
	if err := ctx.Err(); err != nil {
		return classGraph{}, err
	}
	if check.WasCanceled() {
		return classGraph{}, context.Canceled
	}
	p.check = check
	for _, scalar := range []struct {
		name  string
		value *checker.Type
	}{
		{"number", check.GetNumberType()}, {"boolean", check.GetBooleanType()}, {"void", check.GetVoidType()},
	} {
		id := "t:" + scalar.name
		p.types[scalar.value] = id
		p.graph.Types = append(p.graph.Types, classType{ID: id, Kind: scalar.name, Scalar: p.scalarKinds[id]})
	}
	if err := p.admitClassScalars(originals); err != nil {
		return classGraph{}, err
	}
	if err := p.declarations(originals); err != nil {
		return classGraph{}, err
	}
	if err := ctx.Err(); err != nil {
		return classGraph{}, err
	}
	if check.WasCanceled() {
		return classGraph{}, context.Canceled
	}
	if _, err := p.resources.canonicalJSON(&p.graph, "owned graph", classGraphByteLimit, classAnchor{}); err != nil {
		return classGraph{}, err
	}
	return p.graph, nil
}

func classTextSHA(text string) string { return "sha256:" + genericTextDigest(text) }

func classNodeAnchor(node *ast.Node) classAnchor {
	file := ast.GetSourceFileOfNode(node)
	origin := genericNodeOrigin(node)
	return classAnchor{Source: file.FileName(), Start: origin.Start, End: origin.End}
}

func (p *classProjection) inputs(inputs []classInput) error {
	if len(inputs) == 0 || len(inputs) > 16 || len(inputs) != len(p.program.GetSourceFiles()) {
		return classError("input", "explicit Program input set", classAnchor{})
	}
	claimed := map[string]classInput{}
	for _, input := range inputs {
		if _, exists := claimed[input.Path]; exists {
			return classError("input", "duplicate input", classAnchor{Source: input.Path})
		}
		claimed[input.Path] = input
	}
	files := slices.Clone(p.program.GetSourceFiles())
	slices.SortFunc(files, func(a, b *ast.SourceFile) int { return strings.Compare(a.FileName(), b.FileName()) })
	for _, file := range files {
		if err := p.ctx.Err(); err != nil {
			return err
		}
		input, ok := claimed[file.FileName()]
		if !ok || len(file.Text()) > 1<<20 || !utf8.ValidString(file.Text()) ||
			input.SHA256 != classTextSHA(file.Text()) || input.Declaration != file.IsDeclarationFile {
			return classError("input", "Program input identity or source closure differs", classAnchor{Source: file.FileName()})
		}
		source := genericSourceForFile(file)
		owned := classSource{classInput: input, UTF16Length: source.Length, SurrogateStarts: append([]int{}, source.SurrogateStarts...)}
		p.graph.Sources = append(p.graph.Sources, owned)
		var failure error
		var visit func(*ast.Node, int) bool
		visit = func(node *ast.Node, depth int) bool {
			if err := p.resources.inputNode(node, depth); err != nil {
				failure = err
				return true
			}
			return node.ForEachChild(func(child *ast.Node) bool { return visit(child, depth+1) })
		}
		visit(file.AsNode(), 0)
		if failure != nil {
			return failure
		}
	}
	return nil
}

func checkClassDiagnostics(ctx context.Context, program *compiler.Program) error {
	fail := func(items []*ast.Diagnostic) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		d := items[0]
		anchor := classAnchor{}
		if d.File() != nil {
			anchor = classAnchor{Source: d.File().FileName(), Start: d.File().GetPositionMap().UTF8ToUTF16(d.Pos()), End: d.File().GetPositionMap().UTF8ToUTF16(d.End())}
		}
		return &classFailure{Kind: "official", Detail: "official compiler diagnostic", Code: d.Code(), Origin: anchor}
	}
	for _, file := range program.GetSourceFiles() {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := fail(program.GetSyntacticDiagnostics(ctx, file)); err != nil {
			return err
		}
	}
	for _, file := range program.GetSourceFiles() {
		if err := ctx.Err(); err != nil {
			return err
		}
		check, release := program.GetTypeCheckerForFileExclusive(ctx, file)
		canceled := check.WasCanceled()
		release()
		if canceled {
			return context.Canceled
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := fail(program.GetSemanticDiagnostics(ctx, file)); err != nil {
			return err
		}
	}
	if err := fail(program.GetProgramDiagnostics()); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return fail(program.GetGlobalDiagnostics(ctx))
}
