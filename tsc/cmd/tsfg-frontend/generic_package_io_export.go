// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Production export owns explicit VFS inputs and uses the official checker.
package main

import (
	"context"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/tspath"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

func exportGenericPackageFromInputs(ctx context.Context, r genericPackageIOExportRequest) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := (&genericCanonicalBudget{policy: genericPackageIOPolicy()}).walk(reflect.ValueOf(r), 0); err != nil {
		return nil, err
	}
	if r.Scope == "" || len(r.Scope) > 256 || !genericPackageString(r.Scope) {
		return nil, genericPackageError("input", "scope")
	}
	source := r.Source
	if !genericPackagePath(source.Path) || !strings.HasSuffix(source.Path, ".ts") || strings.HasSuffix(source.Path, ".d.ts") ||
		source.Content == "" || len(source.Content) > 3<<20 || !utf8.ValidString(source.Content) {
		return nil, genericPackageError("input", "original source metadata")
	}
	units := 0
	for _, ch := range source.Content {
		units++
		if ch > 0xffff {
			units++
		}
	}
	if units > 1<<20 {
		return nil, genericPackageError("limit", "original source UTF16 length")
	}
	if !genericDigest(source.Digest) || source.Digest != genericTextDigest(source.Content) {
		return nil, genericPackageError("identity", "original source content digest")
	}
	if len(r.Environment.Files) == 0 || len(r.Environment.Files) > 8 {
		return nil, genericPackageError("limit", "declaration count")
	}
	environment := genericDeclarationEnvironment{Profile: r.Environment.Profile, Files: make([]genericDeclaration, len(r.Environment.Files))}
	contents := map[string]string{source.Path: source.Content}
	fileNames := []string{source.Path}
	for i, d := range r.Environment.Files {
		if d.Path == source.Path {
			return nil, genericPackageError("input", "source/declaration path collision")
		}
		environment.Files[i] = genericDeclaration{Path: d.Path, Digest: d.Digest, Content: d.Content}
		contents[d.Path] = d.Content
		fileNames = append(fileNames, d.Path)
	}
	if err := environment.validate(); err != nil {
		return nil, err
	}
	if err := validateGenericPackageFileSet(source.Path, environment.Files); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cwd := tspath.GetDirectoryPath(source.Path)
	fs := vfstest.FromMap(contents, true)
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{FileNames: fileNames,
			CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue,
				Target: core.ScriptTargetESNext, Module: core.ModuleKindESNext}}},
		Host: compiler.NewCompilerHost(cwd, fs, cwd, nil, nil), SingleThreaded: core.TSTrue,
	})
	return exportGenericPackage(ctx, program, program.GetSourceFile(source.Path), r.Scope, environment)
}
