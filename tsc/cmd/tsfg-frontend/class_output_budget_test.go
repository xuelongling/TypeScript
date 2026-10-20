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
	"github.com/microsoft/typescript-go/internal/json"
)

func classMemberBudgetSource() string {
	var source strings.Builder
	source.WriteString("export class Dense {\n")
	for index := range 20 {
		fmt.Fprintf(&source, "  f%d: number = %d;\n", index, index)
	}
	for index := range 30 {
		fmt.Fprintf(&source, "  copy%d(): Dense { return this; }\n", index)
	}
	source.WriteString("}\n")
	return source.String()
}

// Each budget is exercised on a fresh official Program through the production
// Source-to-owned seam. Clearing the VFS leaves only the returned owned facts.
func classMemberBudgetExport(t *testing.T, source string, maxNodes int) (classGraph, error) {
	t.Helper()
	program, environment := genericProgramForTest(source)
	inputs := []classInput{{Path: "/original/main.ts", SHA256: classTextSHA(source)}}
	for _, declaration := range environment.Files {
		inputs = append(inputs, classInput{Path: declaration.Path, SHA256: "sha256:" + declaration.Digest, Declaration: true})
	}
	graph, err := exportClassGraph(t.Context(), program, []*ast.SourceFile{program.GetSourceFile("/original/main.ts")}, classExportOptions{
		Scope: "class:member-budget", Inputs: inputs, MaxNodes: maxNodes, MaxDepth: 64,
	})
	if writeErr := program.Host().FS().WriteFile("/original/main.ts", ""); writeErr != nil {
		t.Fatal(writeErr)
	}
	return graph, err
}

func TestOwnedClassSourceBoundsOwnedMemberProjectionOutput(t *testing.T) {
	t.Parallel()
	source := classMemberBudgetSource()
	t.Run("complete-within-budget", func(t *testing.T) {
		t.Parallel()
		graph, err := classMemberBudgetExport(t, source, 4096)
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(graph)
		if err != nil {
			t.Fatal(err)
		}
		var view map[string]any
		if err := json.Unmarshal(data, &view); err != nil {
			t.Fatal(err)
		}
		classes := classOwnedArray(t, view["classes"])
		if len(classes) != 1 {
			t.Fatalf("complete Dense closure differs: %d classes", len(classes))
		}
		dense := classOwnedNamed(t, classes, "Dense")
		fields, methods := classOwnedArray(t, dense["fields"]), classOwnedArray(t, dense["methods"])
		if len(fields) != 20 || len(methods) != 30 {
			t.Fatalf("complete Dense member set differs: %d fields, %d methods", len(fields), len(methods))
		}
		expected := map[any]bool{}
		for _, members := range [][]any{fields, methods} {
			for _, member := range members {
				expected[classOwnedObject(t, member)["symbol"]] = true
			}
		}
		if len(expected) != 50 {
			t.Fatal("actual Dense member symbols are not independent")
		}
		total := 0
		for _, item := range methods {
			method := classOwnedObject(t, item)
			returned := classOwnedReturnedExpression(t, method)
			if returned["kind"] != "this" || returned["symbol"] != dense["symbol"] || returned["type"] != dense["thisType"] {
				t.Fatalf("copy method lost the same real this reference: %+v", returned)
			}
			body := classOwnedArray(t, method["body"])
			witness := classOwnedObject(t, classOwnedObject(t, body[0])["compatibility"])
			projection := classOwnedArray(t, witness["members"])
			if witness["sourceType"] != dense["thisType"] || witness["targetType"] != dense["instanceType"] ||
				witness["assignable"] != true || len(projection) != 50 {
				t.Fatalf("complete official this-to-Dense relation differs: %d members", len(projection))
			}
			seen := map[any]bool{}
			for _, item := range projection {
				edge := classOwnedObject(t, item)
				if edge["source"] != edge["target"] || !expected[edge["target"]] || seen[edge["target"]] {
					t.Fatalf("official member projection lost/duplicated an actual Dense member: %+v", edge)
				}
				seen[edge["target"]] = true
			}
			total += len(projection)
		}
		if total != 1500 {
			t.Fatalf("expected thirty complete fifty-member witnesses, got %d", total)
		}
	})
	t.Run("reject-owned-output-expansion", func(t *testing.T) {
		t.Parallel()
		graph, err := classMemberBudgetExport(t, source, 1024)
		var failure *classFailure
		if !errors.As(err, &failure) || failure.Kind != "limit" {
			t.Fatalf("owned member projection expansion exceeded the explicit budget without a controlled limit: %v", err)
		}
		// An earlier AST/input rejection is not evidence for the output budget.
		if !strings.Contains(failure.Detail, "owned") || failure.Origin.Source != "/original/main.ts" {
			t.Fatalf("fixture did not reach a concrete owned-output limit: %v", err)
		}
		if !reflect.DeepEqual(graph, classGraph{}) {
			t.Fatal("exhausted output budget exposed a partial owned graph")
		}
	})
}
