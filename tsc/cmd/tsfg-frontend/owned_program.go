// SPDX-License-Identifier: Apache-2.0
// Shared official Program bootstrap. Checker admission and projection remain caller-owned.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
	"slices"
)

func newOwnedProgram(r request) (*compiler.Program, *ast.SourceFile, []string, *response) {
	contents := map[string]string{}
	for _, file := range r.Files {
		contents[file.Path] = file.Content
	}
	fileNames := append([]string{r.RootFile}, r.Declarations.Paths...)
	if r.CompilationClosure != nil {
		fileNames = nil
		for _, input := range r.Files {
			fileNames = append(fileNames, input.Path)
		}
	}
	slices.Sort(fileNames)
	fs := vfstest.FromMap(contents, true)
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
			FileNames:       fileNames,
			CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext},
		}},
		Host:           compiler.NewCompilerHost(r.SourceRoot, fs, r.SourceRoot, nil, nil),
		SingleThreaded: core.TSTrue,
	})
	file := program.GetSourceFile(r.RootFile)
	if file == nil {
		failure := fail("r02b-d-input", "input", r.RootFile, span{})
		return nil, nil, nil, &failure
	}
	approved := map[string]bool{}
	for name := range contents {
		approved[name] = true
	}
	consumed := []string{}
	for _, source := range program.GetSourceFiles() {
		if !approved[source.FileName()] {
			failure := fail("r02b-d-input", "input", source.FileName(), span{})
			return nil, nil, nil, &failure
		}
		consumed = append(consumed, source.FileName())
		delete(approved, source.FileName())
	}
	slices.Sort(consumed)
	return program, file, consumed, nil
}
