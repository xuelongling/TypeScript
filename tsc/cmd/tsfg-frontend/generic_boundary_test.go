// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

func genericProgramForTest(source string) (*compiler.Program, genericDeclarationEnvironment) {
	environment := genericDeclarationEnvironment{Profile: genericDeclarationProfile, Files: []genericDeclaration{{
		Path: "/original/minimal-scalars-v2.d.ts", Digest: genericTextDigest(scalarDeclarations), Content: scalarDeclarations,
	}}}
	fs := vfstest.FromMap(map[string]string{"/original/main.ts": source, environment.Files[0].Path: scalarDeclarations}, true)
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
			FileNames: []string{"/original/main.ts", environment.Files[0].Path},
			CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue,
				Strict: core.TSTrue, Target: core.ScriptTargetESNext, Module: core.ModuleKindESNext},
		}}, Host: compiler.NewCompilerHost("/original", fs, "/original", nil, nil), SingleThreaded: core.TSTrue,
	})
	return program, environment
}

func exportGenericSourceForTest(t *testing.T, source string) (genericGraph, genericDeclarationEnvironment) {
	t.Helper()
	program, environment := genericProgramForTest(source)
	graph, err := exportGenericGraph(t.Context(), program, program.GetSourceFile("/original/main.ts"), "package:test", environment)
	if err != nil {
		t.Fatal(err)
	}
	if err := program.Host().FS().WriteFile("/original/main.ts", ""); err != nil {
		t.Fatal(err)
	}
	if text, ok := program.Host().FS().ReadFile("/original/main.ts"); !ok || text != "" {
		t.Fatal("original in-memory source not cleared")
	}
	return graph, environment
}

func TestOwnedGenericHostLegalNamesUseSeparateNamespaces(t *testing.T) {
	t.Parallel()
	longName := strings.Repeat("long", 33)
	for _, test := range []struct{ name, source string }{
		{"same", "export function same<T>(T: T): T { return T; }"},
		{"T", "export function T<T>(value: T): T { return value; }"},
		{"__tsfg_request", "export function __tsfg_request<__tsfg_request>(__tsfg_arg0: __tsfg_request): __tsfg_request { return __tsfg_arg0; }"},
		{"__tsfg_arg0", "export function __tsfg_arg0<T>(__tsfg_request_: T): T { return __tsfg_request_; }"},
		{"type", "export function type<T>(value: T): T { return value; }"},
		{longName, "export function " + longName + "<T>(value: T): T { return value; }"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g, _ := exportGenericSourceForTest(t, test.source)
			d := genericDefinitionNamed(t, g, test.name)
			result, err := instantiateGenericGraph(t.Context(), g, []genericInstanceRequest{
				g.request(d.Key, genericType{Kind: "boolean"}), g.request(d.Key, genericType{Kind: "number"}),
			})
			if err != nil {
				t.Fatal(err)
			}
			for i, want := range []string{"bool", "f64"} {
				instance := result.Instances[result.Roots[i]]
				if instance.Return != want || len(instance.Parameters) != 1 || instance.Parameters[0] != want || instance.Body.Type != want {
					t.Fatalf("legal source name changed official instantiation: %+v", instance)
				}
			}
		})
	}
}

func TestOwnedGenericHostExportOwnsEnvironmentAndInstanceArguments(t *testing.T) {
	t.Parallel()
	g, environment := exportGenericSourceForTest(t, "export function owned<T>(value: T): T { return value; }")
	digest := g.Digest
	declaration := g.Environment.Files[0]
	environment.Files[0] = genericDeclaration{Path: "/mutated.d.ts", Content: "invalid", Digest: "changed"}
	if g.Environment.Files[0] != declaration || g.Digest != digest || genericGraphDigest(g) != digest {
		t.Fatal("export shared the caller's declaration slice")
	}
	arguments := []genericType{{Kind: "boolean"}}
	request := g.request(g.Definitions[0].Key, arguments...)
	arguments[0] = genericType{Kind: "number"}
	if request.TypeArguments[0].Kind != "boolean" {
		t.Fatal("request shared caller's type arguments")
	}
	result, err := instantiateGenericGraph(t.Context(), g, []genericInstanceRequest{request,
		g.request(g.Definitions[0].Key, genericType{Kind: "number"})})
	if err != nil {
		t.Fatal(err)
	}
	request.TypeArguments[0] = genericType{Kind: "number"}
	first := result.Instances[result.Roots[0]]
	second := result.Instances[result.Roots[1]]
	if first.TypeArguments[0].Kind != "boolean" || first.Return != "bool" || second.Return != "f64" {
		t.Fatal("instance retained mutable input arguments or declaration environment")
	}
	first.TypeArguments[0] = genericType{Kind: "boolean", Parameter: 1}
	if second.TypeArguments[0] != (genericType{Kind: "number"}) {
		t.Fatal("independent instances shared argument storage")
	}
}

func TestOwnedGenericHostPreCanceledReturns(t *testing.T) {
	t.Parallel()
	g := ownedGenericFixture(t)
	r := g.request(g.Definitions[0].Key, genericType{Kind: "boolean"})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := instantiateGenericGraph(ctx, g, []genericInstanceRequest{r}); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled instantiation: %v", err)
	}
	program, environment := genericProgramForTest("export function canceled<T>(value: T): T { return value; }")
	if _, err := exportGenericGraph(ctx, program, program.GetSourceFile("/original/main.ts"), "package:test", environment); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled export: %v", err)
	}
}

// Cancellation is monotonic and triggered by a known Err poll rather than a
// timer. The test below verifies that it occurred inside real official checking
// by observing WasCanceled without asking that checker for further diagnostics.
type genericCancelOnPoll struct {
	context.Context
	remaining atomic.Int32
	cancel    context.CancelFunc
}

func (c *genericCancelOnPoll) Err() error {
	if c.remaining.Add(-1) == 0 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestOwnedGenericHostCancellationDuringCheckingReturns(t *testing.T) {
	t.Parallel()
	g := ownedGenericFixture(t)
	r := g.request(g.Definitions[0].Key, genericType{Kind: "boolean"})
	tree := genericTreeForTest(t, g, r)
	program := newGenericProgram(g, tree, tree.text.String())
	inner, cancel := context.WithCancel(t.Context())
	defer cancel()
	ctx := &genericCancelOnPoll{Context: inner, cancel: cancel}
	// Diagnostics check once at entry, twice per syntactic file, twice before
	// the first semantic file, then the official checker performs the next poll.
	ctx.remaining.Store(int32(2*len(program.GetSourceFiles()) + 4))
	if err := checkGenericDiagnostics(ctx, program, tree); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation during official checking: %v", err)
	}
	wasCanceled := false
	for _, file := range program.GetSourceFiles() {
		check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
		wasCanceled = wasCanceled || check.WasCanceled()
		release()
	}
	if !wasCanceled {
		t.Fatal("controlled cancellation did not occur inside official checking")
	}
}

func genericTreeForTest(t *testing.T, g genericGraph, request genericInstanceRequest) *genericTree {
	t.Helper()
	tree, err := buildGenericTree(g, request)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}
