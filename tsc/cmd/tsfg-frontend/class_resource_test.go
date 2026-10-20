// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/json"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

// The virtual path is valid UTF-8 and contains ordinary path segments. It is
// intentionally long so complete source anchors expand the owned wire graph
// independently of the small source AST and explicit input node/depth budget.
func classResourceExport(t *testing.T, count int) (classGraph, error, string) {
	t.Helper()
	path := "/original/" + strings.Repeat("segment/", 4096) + "main.ts"
	var source strings.Builder
	for index := range count {
		fmt.Fprintf(&source, "export class Empty%d {}\n", index)
	}
	text := source.String()
	const declaration = "/original/minimal-scalars-v2.d.ts"
	fs := vfstest.FromMap(map[string]string{path: text, declaration: scalarDeclarations}, true)
	options := &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue,
		Target: core.ScriptTargetESNext, Module: core.ModuleKindESNext}
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
			FileNames: []string{path, declaration}, CompilerOptions: options,
		}}, Host: compiler.NewCompilerHost("/original", fs, "/original", nil, nil), SingleThreaded: core.TSTrue,
	})
	graph, err := exportClassGraph(t.Context(), program, []*ast.SourceFile{program.GetSourceFile(path)}, classExportOptions{
		Scope: "class:graph-byte-budget", Inputs: []classInput{
			{Path: path, SHA256: classTextSHA(text)},
			{Path: declaration, SHA256: classTextSHA(scalarDeclarations), Declaration: true},
		}, MaxNodes: 100000, MaxDepth: 128,
	})
	for _, input := range []string{path, declaration} {
		if writeErr := fs.WriteFile(input, ""); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	return graph, err, path
}

func TestOwnedClassSourceBoundsCompleteGraphWireBytes(t *testing.T) {
	t.Parallel()
	t.Run("complete-small-graph", func(t *testing.T) {
		t.Parallel()
		graph, err, path := classResourceExport(t, 1)
		if err != nil {
			t.Fatalf("small official class Program did not project: %v", err)
		}
		if len(graph.Classes) != 1 || graph.Classes[0].Name != "Empty0" ||
			graph.Classes[0].Origin.Source != path || graph.Classes[0].Constructor.Origin.Source != path ||
			len(graph.Classes[0].Fields) != 0 || len(graph.Classes[0].Methods) != 0 ||
			len(graph.Symbols) != 1 || graph.Symbols[0].Declaration.Source != path {
			t.Fatal("small graph lost its complete class, constructor or original symbol anchors")
		}
		data, err := json.Marshal(graph)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) >= 8<<20 {
			t.Fatalf("small graph is not a valid byte-budget positive control: bytes=%d", len(data))
		}
	})
	t.Run("reject-owned-wire-expansion", func(t *testing.T) {
		t.Parallel()
		graph, err, _ := classResourceExport(t, 128)
		if err == nil {
			data, marshalErr := json.Marshal(graph)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			if len(graph.Classes) != 128 || len(data) <= 8<<20 {
				t.Fatalf("fixture did not produce the complete over-budget graph: classes=%d bytes=%d", len(graph.Classes), len(data))
			}
			// Report counts only. The expanded graph is never printed.
			t.Fatalf("complete owned graph exceeded 8MiB without a controlled limit: classes=%d bytes=%d", len(graph.Classes), len(data))
		}
		var failure *classFailure
		if !errors.As(err, &failure) || failure.Kind != "limit" {
			t.Fatal("owned wire expansion was not rejected by a controlled class limit")
		}
		// An earlier AST/input rejection cannot prove this owned wire boundary.
		if !strings.Contains(failure.Detail, "owned") || !strings.Contains(failure.Detail, "byte") {
			t.Fatalf("fixture did not reach the owned graph byte limit: kind=%s detail=%s", failure.Kind, failure.Detail)
		}
		if !reflect.DeepEqual(graph, classGraph{}) {
			t.Fatal("exhausted owned wire budget exposed a partial graph")
		}
	})
}
