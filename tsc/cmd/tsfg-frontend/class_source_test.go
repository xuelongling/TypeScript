// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
)

const classCounterSource = "export class Counter { value: number = 1; constructor(seed: number) { this.value = seed; } }"

func classSourceForTest(t *testing.T, source string) classGraph {
	t.Helper()
	// The same real official Program host used by the existing owned source
	// exporter; no successful checker result or graph is mocked.
	program, environment := genericProgramForTest(source)
	file := program.GetSourceFile("/original/main.ts")
	inputs := []classInput{{Path: "/original/main.ts", SHA256: classTextSHA(source)}}
	for _, declaration := range environment.Files {
		inputs = append(inputs, classInput{Path: declaration.Path, SHA256: "sha256:" + declaration.Digest, Declaration: true})
	}
	graph, err := exportClassGraph(t.Context(), program, []*ast.SourceFile{file}, classExportOptions{
		Scope: "class:test", Inputs: inputs, MaxNodes: 4096, MaxDepth: 64,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := program.Host().FS().WriteFile("/original/main.ts", ""); err != nil {
		t.Fatal(err)
	}
	return graph
}

func TestOwnedClassSourceConstructorOwnsCheckedFieldsAndBody(t *testing.T) {
	t.Parallel()
	graph := classSourceForTest(t, classCounterSource)
	if len(graph.Classes) != 1 {
		t.Fatalf("want one owned class, got %d", len(graph.Classes))
	}
	counter := graph.Classes[0]
	if counter.Name != "Counter" || counter.InstanceType == counter.ConstructorType ||
		counter.RuntimeKey == counter.InstanceType || !strings.HasPrefix(counter.RuntimeKey, "sha256:") {
		t.Fatalf("class allocation and official static identities differ: %+v", counter)
	}
	if len(counter.Fields) != 1 {
		t.Fatalf("want one field, got %+v", counter.Fields)
	}
	field := counter.Fields[0]
	if field.Ordinal != 0 || field.Name != "value" || field.Type != "t:number" || field.Access != "public" || field.Readonly ||
		field.Initializer == nil || field.Initializer.Kind != "number" || field.Initializer.Token != "1" {
		t.Fatalf("checked field/initializer was not owned: %+v", field)
	}
	constructor := counter.Constructor
	if constructor.Kind != "explicit" || constructor.Signature.Return != counter.InstanceType ||
		len(constructor.Signature.Parameters) != 1 || constructor.Signature.Parameters[0].Type != "t:number" ||
		constructor.InitializationPhase != "before-body" || len(constructor.InitializationFields) != 1 ||
		constructor.InitializationFields[0] != field.ID || !graph.Options.UseDefineForClassFields {
		t.Fatalf("checked constructor signature/initialization was not owned: %+v", constructor)
	}
	if len(constructor.Body) != 1 {
		t.Fatalf("complete constructor body missing: %+v", constructor.Body)
	}
	store := constructor.Body[0].Expression
	if store == nil || store.Kind != "assignment" || store.Operator != "=" || store.Left == nil || store.Right == nil ||
		store.Left.Kind != "field" || store.Left.Symbol != field.Symbol || store.Left.Receiver == nil ||
		store.Left.Receiver.Kind != "this" || store.Left.Receiver.Symbol != counter.Symbol ||
		store.Left.Permission == nil || !store.Left.Permission.Writable || !store.Left.Permission.WriteUse ||
		store.Right.Kind != "reference" || store.Right.Symbol != constructor.Signature.Parameters[0].Symbol {
		t.Fatalf("constructor store does not retain official this/field/parameter resolution: %+v", store)
	}
	if store.Origin.Source != "/original/main.ts" || store.Origin.Start != strings.Index(classCounterSource, "this.value") ||
		store.Origin.End != strings.Index(classCounterSource, "; }") {
		t.Fatalf("constructor expression source anchor differs: %+v", store.Origin)
	}
}

func TestOwnedClassSourceMethodOwnsCompleteBodyAndThisResolution(t *testing.T) {
	t.Parallel()
	source := strings.TrimSuffix(classCounterSource, " }") +
		" add(delta: number): number { this.value = this.value + delta; return this.value; } }"
	graph := classSourceForTest(t, source)
	if len(graph.Classes) != 1 || len(graph.Classes[0].Methods) != 1 {
		t.Fatalf("complete method missing: %+v", graph.Classes)
	}
	counter := graph.Classes[0]
	method := counter.Methods[0]
	if method.Name != "add" || method.Signature.Return != "t:number" || len(method.Signature.Parameters) != 1 ||
		method.Signature.Parameters[0].Type != "t:number" || len(method.Body) != 2 {
		t.Fatalf("method signature or complete body missing: %+v", method)
	}
	store, returned := method.Body[0].Expression, method.Body[1].Expression
	if store == nil || store.Kind != "assignment" || store.Left == nil || store.Right == nil ||
		store.Left.Symbol != counter.Fields[0].Symbol || store.Left.Receiver == nil || store.Left.Receiver.Symbol != counter.Symbol ||
		store.Left.Permission == nil || !store.Left.Permission.Writable || !store.Left.Permission.WriteUse ||
		store.Right.Kind != "binary" || store.Right.Operator != "+" || store.Right.Type != "t:number" ||
		store.Right.Left == nil || store.Right.Left.Symbol != counter.Fields[0].Symbol ||
		store.Right.Right == nil || store.Right.Right.Symbol != method.Signature.Parameters[0].Symbol ||
		method.Body[1].Kind != "return" || returned == nil || returned.Kind != "field" || returned.Symbol != counter.Fields[0].Symbol ||
		returned.Receiver == nil || returned.Receiver.Symbol != counter.Symbol || returned.Permission == nil || !returned.Permission.Readable || returned.Permission.WriteUse {
		t.Fatalf("method body lost this/member resolution: %+v", method.Body)
	}
}
