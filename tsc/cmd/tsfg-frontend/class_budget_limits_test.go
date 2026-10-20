// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/json"
)

// These are public resource regressions added after the causal wire/constructor
// REDs. Every input is a real Program; no over-limit successful graph is mocked.
func classBudgetLimitsExport(t *testing.T, source string) (classGraph, error) {
	t.Helper()
	program, environment := genericProgramForTest(source)
	inputs := []classInput{{Path: "/original/main.ts", SHA256: classTextSHA(source)}}
	for _, declaration := range environment.Files {
		inputs = append(inputs, classInput{Path: declaration.Path, SHA256: "sha256:" + declaration.Digest, Declaration: true})
	}
	graph, err := exportClassGraph(t.Context(), program, []*ast.SourceFile{program.GetSourceFile("/original/main.ts")}, classExportOptions{
		Scope: "class:wire-shape-budget", Inputs: inputs, MaxNodes: 100000, MaxDepth: 128,
	})
	for _, input := range inputs {
		if writeErr := program.Host().FS().WriteFile(input.Path, ""); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	return graph, err
}

func classBudgetWireCounts(t *testing.T, graph classGraph) (int, int, int) {
	t.Helper()
	data, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	tokens, depth, maximum := 0, 0, 0
	for {
		token, err := decoder.ReadToken()
		if errors.Is(err, io.EOF) {
			return len(data), tokens, maximum
		}
		if err != nil {
			t.Fatal(err)
		}
		tokens++
		switch token.Kind() {
		case '{', '[':
			depth++
			if depth > maximum {
				maximum = depth
			}
		case '}', ']':
			depth--
		}
	}
}

func classBudgetLimitsRejected(t *testing.T, graph classGraph, err error, axis string) {
	t.Helper()
	if err == nil {
		wireBytes, tokens, depth := classBudgetWireCounts(t, graph)
		t.Fatalf("public export accepted over-budget owned graph: bytes=%d tokens=%d depth=%d expected=%s", wireBytes, tokens, depth, axis)
	}
	var failure *classFailure
	if !errors.As(err, &failure) || failure.Kind != "limit" ||
		!strings.Contains(failure.Detail, "owned graph") || !strings.Contains(failure.Detail, axis) {
		t.Fatalf("fixture did not reach the owned graph %s boundary: %v", axis, err)
	}
	if !reflect.DeepEqual(graph, classGraph{}) {
		t.Fatal("wire shape rejection exposed a partial owned graph")
	}
}

func classBudgetFieldsSource(count int) string {
	var source strings.Builder
	source.WriteString("export class Fields {\n")
	for index := range count {
		fmt.Fprintf(&source, "  f%d: number = 1;\n", index)
	}
	source.WriteString("}\n")
	return source.String()
}

func TestOwnedClassSourceBudgetLimitsBoundGraphTokens(t *testing.T) {
	t.Parallel()
	t.Run("complete-ten-fields", func(t *testing.T) {
		t.Parallel()
		graph, err := classBudgetLimitsExport(t, classBudgetFieldsSource(10))
		if err != nil {
			t.Fatal(err)
		}
		if len(graph.Classes) != 1 || len(graph.Classes[0].Fields) != 10 || len(graph.Classes[0].Methods) != 0 {
			t.Fatal("within-budget field fixture lost complete class members")
		}
		owner := graph.Classes[0]
		if owner.Constructor.Kind != "implicit-base" || owner.Constructor.InitializationPhase != "before-body" ||
			len(owner.Constructor.InitializationFields) != 10 || len(owner.Constructor.Body) != 0 {
			t.Fatal("within-budget fields lost their actual default initialization order")
		}
		for index, field := range owner.Fields {
			if field.Name != fmt.Sprintf("f%d", index) || field.Ordinal != index || field.Symbol != field.ID+".symbol" ||
				field.Initializer == nil || field.Initializer.Kind != "number" || field.Initializer.Token != "1" ||
				field.InitializerCompatibility == nil || !field.InitializerCompatibility.Assignable ||
				field.InitializerCompatibility.SourceType != field.Initializer.Type || field.InitializerCompatibility.TargetType != field.Type ||
				owner.Constructor.InitializationFields[index] != field.ID {
				t.Fatalf("within-budget field lost actual initializer/binding/witness: %+v", field)
			}
		}
		wireBytes, tokens, depth := classBudgetWireCounts(t, graph)
		if wireBytes >= 8<<20 || tokens >= 200000 || depth >= 128 {
			t.Fatalf("positive field fixture exceeds a wire boundary: bytes=%d tokens=%d depth=%d", wireBytes, tokens, depth)
		}
	})
	t.Run("reject-many-short-fields", func(t *testing.T) {
		t.Parallel()
		// The actual emitted field plus symbol uses roughly 124 JSON tokens.
		// 1500 fields fall below 200k; 1700 exceed tokens while remaining small
		// in source AST depth, input nodes, paths and expected wire bytes.
		graph, err := classBudgetLimitsExport(t, classBudgetFieldsSource(1700))
		classBudgetLimitsRejected(t, graph, err, "token")
	})
}

func classBudgetNestedConstructSource(count int) string {
	expression := "seed"
	for range count {
		expression = "new Box(" + expression + ")"
	}
	return "export class Box { next: Box; constructor(next: Box) { this.next = next; } wrap(seed: Box): Box { return " + expression + "; } }"
}

func TestOwnedClassSourceBudgetLimitsBoundGraphDepth(t *testing.T) {
	t.Parallel()
	t.Run("complete-four-allocations", func(t *testing.T) {
		t.Parallel()
		graph, err := classBudgetLimitsExport(t, classBudgetNestedConstructSource(4))
		if err != nil {
			t.Fatal(err)
		}
		if len(graph.Classes) != 1 || len(graph.Classes[0].Fields) != 1 || len(graph.Classes[0].Methods) != 1 {
			t.Fatal("within-budget nested allocation fixture lost Box members")
		}
		owner := graph.Classes[0]
		method := owner.Methods[0]
		if method.Name != "wrap" || len(method.Signature.Parameters) != 1 || len(method.Body) != 1 || method.Body[0].Kind != "return" {
			t.Fatal("within-budget nested allocation fixture lost wrap's complete signature/body")
		}
		expression := method.Body[0].Expression
		seen := map[string]bool{}
		for range 4 {
			if expression == nil || expression.Kind != "construct" || expression.Dispatch != "direct-constructor" ||
				expression.StaticTarget != owner.Constructor.ID || expression.Type != owner.InstanceType ||
				expression.ResolvedSignature == nil || !reflect.DeepEqual(*expression.ResolvedSignature, owner.Constructor.Signature) ||
				len(expression.Arguments) != 1 || len(expression.ArgumentCompatibility) != 1 || seen[expression.ID] {
				t.Fatalf("within-budget nested allocation lost a real operation/target/signature: %+v", expression)
			}
			seen[expression.ID] = true
			argument, witness := expression.Arguments[0], expression.ArgumentCompatibility[0]
			if argument == nil || !witness.Assignable || witness.SourceType != argument.Type || witness.TargetType != owner.InstanceType ||
				len(witness.Members) != len(owner.Fields)+len(owner.Methods) {
				t.Fatal("within-budget nested allocation lost its complete official reference witness")
			}
			expression = argument
		}
		if expression == nil || expression.Kind != "reference" || expression.Symbol != method.Signature.Parameters[0].Symbol ||
			expression.Type != owner.InstanceType || len(seen) != 4 {
			t.Fatal("nested allocations lost their four distinct operations or original seed binding")
		}
		wireBytes, tokens, depth := classBudgetWireCounts(t, graph)
		if wireBytes >= 8<<20 || tokens >= 200000 || depth >= 128 {
			t.Fatalf("positive allocation fixture exceeds a wire boundary: bytes=%d tokens=%d depth=%d", wireBytes, tokens, depth)
		}
	})
	t.Run("reject-owned-array-depth-expansion", func(t *testing.T) {
		t.Parallel()
		// New-expression AST nesting stays below 128, but each owned argument
		// introduces an expression object plus an arguments array.
		graph, err := classBudgetLimitsExport(t, classBudgetNestedConstructSource(64))
		classBudgetLimitsRejected(t, graph, err, "depth")
	})
}
