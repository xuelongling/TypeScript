// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/json"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

// Options are applied before the real official Program is constructed. No
// existing Program/checker is mutated, and no projection result is mocked.
func classIdentityExport(t *testing.T, source string, configure func(*core.CompilerOptions)) (classGraph, error) {
	t.Helper()
	options := &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue,
		Target: core.ScriptTargetESNext, Module: core.ModuleKindESNext}
	if configure != nil {
		configure(options)
	}
	declaration := "/original/minimal-scalars-v2.d.ts"
	fs := vfstest.FromMap(map[string]string{"/original/main.ts": source, declaration: scalarDeclarations}, true)
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
			FileNames: []string{"/original/main.ts", declaration}, CompilerOptions: options,
		}}, Host: compiler.NewCompilerHost("/original", fs, "/original", nil, nil), SingleThreaded: core.TSTrue,
	})
	return exportClassGraph(t.Context(), program, []*ast.SourceFile{program.GetSourceFile("/original/main.ts")}, classExportOptions{
		Scope: "class:identity", Inputs: []classInput{
			{Path: "/original/main.ts", SHA256: classTextSHA(source)},
			{Path: declaration, SHA256: classTextSHA(scalarDeclarations), Declaration: true},
		}, MaxNodes: 4096, MaxDepth: 64,
	})
}

func classIdentityGraph(t *testing.T, source string, configure func(*core.CompilerOptions)) classGraph {
	t.Helper()
	graph, err := classIdentityExport(t, source, configure)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Classes) != 1 {
		t.Fatalf("want one class, got %+v", graph.Classes)
	}
	return graph
}

func classIdentitySourceSHA(t *testing.T, graph classGraph) string {
	t.Helper()
	for _, source := range graph.Sources {
		if source.Path == "/original/main.ts" {
			return source.SHA256
		}
	}
	t.Fatal("owned original source identity missing")
	return ""
}

func TestOwnedClassSourceRuntimeKeyBindsSourceAndOptions(t *testing.T) {
	t.Parallel()
	t.Run("source-content", func(t *testing.T) {
		t.Parallel()
		first := classIdentityGraph(t, classCounterSource, nil)
		changed := classIdentityGraph(t, strings.Replace(classCounterSource, "= 1;", "= 2;", 1), nil)
		if classIdentitySourceSHA(t, first) == classIdentitySourceSHA(t, changed) {
			t.Fatal("source mutation did not reach the official owned source identity")
		}
		if first.Classes[0].RuntimeKey == changed.Classes[0].RuntimeKey {
			t.Fatal("candidate allocation provenance reused a runtime key after source content changed at the same path and declaration position")
		}
	})
	t.Run("effective-field-initialization", func(t *testing.T) {
		t.Parallel()
		define := classIdentityGraph(t, classCounterSource, func(options *core.CompilerOptions) { options.UseDefineForClassFields = core.TSTrue })
		assign := classIdentityGraph(t, classCounterSource, func(options *core.CompilerOptions) { options.UseDefineForClassFields = core.TSFalse })
		if classIdentitySourceSHA(t, define) != classIdentitySourceSHA(t, assign) || !define.Options.UseDefineForClassFields || assign.Options.UseDefineForClassFields {
			t.Fatal("test did not vary only the actual official field-initialization option")
		}
		if define.Classes[0].RuntimeKey == assign.Classes[0].RuntimeKey {
			t.Fatal("candidate allocation provenance reused a runtime key across effective field-initialization semantics")
		}
	})
	t.Run("same-input-fresh-programs", func(t *testing.T) {
		t.Parallel()
		first, second := classIdentityGraph(t, classCounterSource, nil), classIdentityGraph(t, classCounterSource, nil)
		if first.Classes[0].RuntimeKey == "" || first.Classes[0].RuntimeKey != second.Classes[0].RuntimeKey {
			t.Fatal("same owned inputs did not preserve allocation provenance across fresh official Programs")
		}
		// These candidate keys describe owned provenance; they do not adopt a
		// contract family or make runtime identity an assignability constraint.
		bytes, err := json.Marshal(first)
		if err != nil {
			t.Fatal(err)
		}
		var view map[string]any
		if err := json.Unmarshal(bytes, &view); err != nil {
			t.Fatal(err)
		}
		profile, ok := view["semanticProfile"].(string)
		if !ok || profile == "" {
			t.Fatal("owned candidate semantic profile missing")
		}
		options := classOwnedObject(t, view["options"])
		digest, ok := options["officialOptionsSha256"].(string)
		if !ok || !strings.HasPrefix(digest, "sha256:") || len(digest) != 71 {
			t.Fatal("owned full official raw-options identity missing")
		}
		strict := classOwnedObject(t, options["effectiveStrict"])
		for _, name := range []string{"strictNullChecks", "strictFunctionTypes", "strictBindCallApply", "strictPropertyInitialization", "strictBuiltinIteratorReturn", "noImplicitAny", "noImplicitThis", "useUnknownInCatchVariables"} {
			if strict[name] != true {
				t.Fatalf("effective official strict option %s was not owned: %+v", name, strict)
			}
		}
	})
}

func TestOwnedClassSourceRejectsDisabledEffectiveStrictChecks(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		configure func(*core.CompilerOptions)
	}{
		{"strictFunctionTypes", func(options *core.CompilerOptions) { options.StrictFunctionTypes = core.TSFalse }},
		{"strictPropertyInitialization", func(options *core.CompilerOptions) { options.StrictPropertyInitialization = core.TSFalse }},
		{"strictBindCallApply", func(options *core.CompilerOptions) { options.StrictBindCallApply = core.TSFalse }},
		{"strictBuiltinIteratorReturn", func(options *core.CompilerOptions) { options.StrictBuiltinIteratorReturn = core.TSFalse }},
		{"noImplicitThis", func(options *core.CompilerOptions) { options.NoImplicitThis = core.TSFalse }},
		{"useUnknownInCatchVariables", func(options *core.CompilerOptions) { options.UseUnknownInCatchVariables = core.TSFalse }},
		// These two are existing admission controls, retained as positive checks
		// on the same effective strict profile rather than new capability credit.
		{"noImplicitAny", func(options *core.CompilerOptions) { options.NoImplicitAny = core.TSFalse }},
		{"strictNullChecks", func(options *core.CompilerOptions) { options.StrictNullChecks = core.TSFalse }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := classIdentityExport(t, classCounterSource, test.configure)
			var failure *classFailure
			if !errors.As(err, &failure) || failure.Kind != "input" {
				t.Fatalf("disabled effective strict check %s was not rejected by the candidate input profile: %v", test.name, err)
			}
		})
	}
}
