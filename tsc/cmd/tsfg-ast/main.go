// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// tsfg-ast is a compiler-side source intake probe, not a compiler or JS runner.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/microsoft/typescript-go/internal/json"
	"github.com/microsoft/typescript-go/internal/tspath"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/locale"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs"
)

type statement struct {
	Kind  string `json:"kind"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

type typeObservation struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type diagnosticObservation struct {
	Code     int32  `json:"code"`
	Category string `json:"category"`
	Message  string `json:"message"`
	File     string `json:"file"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
}

func observeDiagnostics(diagnostics []*ast.Diagnostic) []diagnosticObservation {
	result := []diagnosticObservation{}
	for _, d := range diagnostics {
		name := ""
		if d.File() != nil {
			name = tspath.GetBaseFileName(d.File().FileName())
		}
		result = append(result, diagnosticObservation{d.Code(), d.Category().Name(), d.Localize(locale.Default), name, d.Pos(), d.End()})
	}
	return result
}

func runProbe(fs vfs.FS, name string, output io.Writer) error {
	libs := []string{"/internal/bundled/libs/lib.es5.d.ts", "/internal/bundled/libs/lib.decorators.d.ts", "/internal/bundled/libs/lib.decorators.legacy.d.ts"}
	for _, input := range append([]string{name}, libs...) {
		if !fs.FileExists(input) {
			return fmt.Errorf("missing required input: %s", input)
		}
	}
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
			FileNames:       append([]string{name}, libs...),
			CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext},
		}},
		Host:           compiler.NewCompilerHost("/", fs, "/internal/bundled/libs", nil, nil),
		SingleThreaded: core.TSTrue,
	})
	approved := map[string]bool{name: true}
	for _, input := range libs {
		approved[input] = true
	}
	sourceFiles := []string{}
	for _, source := range program.GetSourceFiles() {
		input := source.FileName()
		if !approved[input] {
			return fmt.Errorf("unapproved Program input: %s", input)
		}
		delete(approved, input)
		sourceFiles = append(sourceFiles, input)
	}
	if len(approved) != 0 {
		return errors.New("Program omitted required inputs")
	}
	sort.Strings(sourceFiles)
	file := program.GetSourceFile(name)
	if file == nil {
		return errors.New("source was not parsed")
	}
	statements := []statement{}
	for _, node := range file.Statements.Nodes {
		statements = append(statements, statement{node.Kind.String(), node.Pos(), node.End()})
	}
	identifiers := []string{}
	types := []typeObservation{}
	syntax := observeDiagnostics(program.GetSyntacticDiagnostics(context.Background(), nil))
	semantic := observeDiagnostics(program.GetSemanticDiagnostics(context.Background(), nil))
	programDiagnostics := observeDiagnostics(program.GetProgramDiagnostics())
	global := observeDiagnostics(program.GetGlobalDiagnostics(context.Background()))
	// Convert types while their owning checker is exclusively borrowed. Only values escape.
	checker, release := program.GetTypeCheckerForFileExclusive(context.Background(), file)
	defer release()
	var visit ast.Visitor
	visit = func(node *ast.Node) bool {
		if node.Kind == ast.KindIdentifier {
			identifiers = append(identifiers, node.Text())
		}
		if node.Kind == ast.KindVariableDeclaration && node.Name().Kind == ast.KindIdentifier {
			types = append(types, typeObservation{node.Name().Text(), checker.TypeToString(checker.GetTypeAtLocation(node.Name()))})
		}
		node.ForEachChild(visit)
		return false
	}
	file.ForEachChild(visit)
	result := struct {
		Kind                 string                  `json:"kind"`
		Statements           []statement             `json:"statements"`
		Identifiers          []string                `json:"identifiers"`
		ParseDiagnosticCount int                     `json:"parseDiagnosticCount"`
		Types                []typeObservation       `json:"types"`
		SyntaxDiagnostics    []diagnosticObservation `json:"syntaxDiagnostics"`
		SemanticDiagnostics  []diagnosticObservation `json:"semanticDiagnostics"`
		ProgramDiagnostics   []diagnosticObservation `json:"programDiagnostics"`
		GlobalDiagnostics    []diagnosticObservation `json:"globalDiagnostics"`
		SourceFiles          []string                `json:"sourceFiles"`
	}{file.AsNode().Kind.String(), statements, identifiers, len(file.Diagnostics()), types, syntax, semantic, programDiagnostics, global, sourceFiles}
	return json.NewLegacyEncoder(output).Encode(result)
}
