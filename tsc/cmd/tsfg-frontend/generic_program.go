// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// A fresh official Program consumes factory definitions and explicit declarations.
package main

import (
	"context"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

type genericHost struct {
	compiler.CompilerHost
	tree           *genericTree
	diagnosticText string
	declarations   map[string]bool
	file           *ast.SourceFile
}

var _ compiler.CompilerHost = (*genericHost)(nil)

func (h *genericHost) GetSourceFile(opts ast.SourceFileParseOptions) *ast.SourceFile {
	if opts.FileName != genericFileName {
		if h.declarations[opts.FileName] {
			return h.CompilerHost.GetSourceFile(opts)
		}
		return nil
	}
	if h.file == nil {
		h.file = h.tree.sourceFile(opts, h.diagnosticText)
	}
	return h.file
}

func newGenericProgram(g genericGraph, tree *genericTree, diagnosticText string) *compiler.Program {
	// Only declaration files carry text. Definition source is absent even from
	// the VFS; the custom GetSourceFile is its sole semantic input.
	contents := map[string]string{genericFileName: ""}
	declarations := map[string]bool{}
	fileNames := []string{genericFileName}
	for _, d := range g.Environment.Files {
		contents[d.Path] = d.Content
		declarations[d.Path] = true
		fileNames = append(fileNames, d.Path)
	}
	fs := vfstest.FromMap(contents, true)
	host := &genericHost{CompilerHost: compiler.NewCompilerHost("/owned", fs, "/owned", nil, nil),
		tree: tree, diagnosticText: diagnosticText, declarations: declarations}
	return compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
			FileNames: fileNames,
			CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue,
				Strict: core.TSTrue, Target: core.ScriptTargetESNext, Module: core.ModuleKindESNext},
		}}, Host: host, SingleThreaded: core.TSTrue,
	})
}

func checkGenericProgram(ctx context.Context, g genericGraph, program *compiler.Program, tree *genericTree) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	expected := map[string]bool{genericFileName: true}
	for _, declaration := range g.Environment.Files {
		expected[declaration.Path] = true
	}
	for _, file := range program.GetSourceFiles() {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !expected[file.FileName()] {
			return genericError("input", "unexpected compiler input", genericOrigin{})
		}
		delete(expected, file.FileName())
	}
	if len(expected) != 0 {
		return genericError("input", "owned host/declaration not consumed", genericOrigin{})
	}
	return checkGenericDiagnostics(ctx, program, tree)
}

// Never ask an official checker to process another file after cancellation.
// The Program's nil-file semantic collector iterates its checker group without
// this boundary, so each file is requested separately and cancellation is final.
func checkGenericDiagnostics(ctx context.Context, program *compiler.Program, tree *genericTree) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	files := program.GetSourceFiles()
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		diagnostics := program.GetSyntacticDiagnostics(ctx, file)
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(diagnostics) != 0 {
			return genericOfficialFailure(diagnostics[0], tree)
		}
	}
	for _, file := range files {
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
		diagnostics := program.GetSemanticDiagnostics(ctx, file)
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(diagnostics) != 0 {
			return genericOfficialFailure(diagnostics[0], tree)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	programDiagnostics := program.GetProgramDiagnostics()
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(programDiagnostics) != 0 {
		return genericOfficialFailure(programDiagnostics[0], tree)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	globalDiagnostics := program.GetGlobalDiagnostics(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(globalDiagnostics) != 0 {
		return genericOfficialFailure(globalDiagnostics[0], tree)
	}
	return nil
}

func genericOfficialFailure(d *ast.Diagnostic, tree *genericTree) error {
	origin := genericOrigin{}
	if d.File() != nil {
		origin = genericOrigin{Start: d.File().GetPositionMap().UTF8ToUTF16(d.Pos()), End: d.File().GetPositionMap().UTF8ToUTF16(d.End())}
	}
	if tree != nil && d.File() != nil && d.File().FileName() == genericFileName {
		width := int(^uint(0) >> 1)
		origin = genericOrigin{}
		for n, anchor := range tree.origins {
			if n.Pos() <= d.Pos() && n.End() >= d.End() && n.End()-n.Pos() < width {
				origin, width = anchor, n.End()-n.Pos()
			}
		}
	}
	source := genericSource{}
	if d.File() != nil {
		source = genericSource{Path: d.File().FileName(), Digest: genericTextDigest(d.File().Text()), PositionUnit: "utf16", Length: d.File().GetPositionMap().UTF8ToUTF16(len(d.File().Text()))}
	}
	return &genericFailure{Kind: "official-type", Code: d.Code(), Source: source, Origin: origin, Detail: "official compiler diagnostic"}
}

func projectCheckedGenericInstance(ctx context.Context, g genericGraph, r genericInstanceRequest, tree *genericTree, program *compiler.Program) (genericInstance, []genericInstanceRequest, error) {
	if err := ctx.Err(); err != nil {
		return genericInstance{}, nil, err
	}
	file := program.GetSourceFile(genericFileName)
	if file == nil {
		return genericInstance{}, nil, genericError("input", "missing owned module", genericOrigin{})
	}
	check, release := program.GetTypeCheckerForFileExclusive(ctx, file)
	defer release()
	if err := ctx.Err(); err != nil {
		return genericInstance{}, nil, err
	}
	if check.WasCanceled() {
		return genericInstance{}, nil, context.Canceled
	}
	return makeGenericInstance(ctx, g, r, tree, check)
}
