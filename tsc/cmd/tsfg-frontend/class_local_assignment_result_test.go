// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
)

const classLocalAssignmentResultSource = `// 😀
export class Base {
  value: number;
  constructor(seed: number) { this.value = seed; }
}
export class Derived extends Base {
  replace(other: Base, replacement: Derived): Derived {
    let alias: Base = other;
    return alias = replacement;
  }
}`

// This consumes the assignment value with a real typed return. It is a regression
// for already supported facts, not invented Source success or numeric settlement.
func TestOwnedClassSourceOwnsConsumedLocalAssignmentResult(t *testing.T) {
	t.Parallel()
	project := func() classGraph {
		program, _ := genericProgramForTest(classLocalAssignmentResultSource)
		if err := checkClassDiagnostics(t.Context(), program); err != nil {
			t.Fatalf("consumed assignment fixture is not an officially valid Program: %v", err)
		}
		inputs := []classInput{}
		for _, file := range program.GetSourceFiles() {
			inputs = append(inputs, classInput{Path: file.FileName(), SHA256: classTextSHA(file.Text()), Declaration: file.IsDeclarationFile})
		}
		graph, err := exportClassGraph(t.Context(), program, []*ast.SourceFile{program.GetSourceFile("/original/main.ts")}, classExportOptions{
			Scope: "class:local-assignment-result", Inputs: inputs, MaxNodes: 4096, MaxDepth: 64,
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, input := range inputs {
			if err := program.Host().FS().WriteFile(input.Path, ""); err != nil {
				t.Fatal(err)
			}
		}
		return graph
	}
	graph := project()
	view := classMultiView(t, graph)
	if view["semanticProfile"] != "tsfg-owned-class-facts-candidate-v5" || view["scope"] != "class:local-assignment-result" {
		t.Fatal("consumed local result changed the existing candidate profile or scope")
	}
	classes := classOwnedArray(t, view["classes"])
	if len(classes) != 2 {
		t.Fatal("consumed local result lost its complete Base/Derived closure")
	}
	base, derived := classOwnedNamed(t, classes, "Base"), classOwnedNamed(t, classes, "Derived")
	if derived["base"] != base["id"] || base["runtimeKey"] == derived["runtimeKey"] {
		t.Fatal("distinct actual classes or their official base relation differ")
	}
	for _, declaration := range []map[string]any{base, derived} {
		if declaration["runtimeKey"] != classMultiExpectedKey(t, view, declaration) {
			t.Fatal("consumed assignment class key lost independently computed full v2 provenance")
		}
	}
	fields, methods := classOwnedArray(t, base["fields"]), classOwnedArray(t, derived["methods"])
	if len(fields) != 1 || len(methods) != 1 || len(classOwnedArray(t, derived["fields"])) != 0 {
		t.Fatal("consumed assignment fixture changed its own member closure")
	}
	field, method := classOwnedObject(t, fields[0]), classOwnedNamed(t, methods, "replace")
	if field["initializer"] != nil {
		t.Fatal("fixture must not depend on unresolved literal initialization")
	}
	signature := classOwnedObject(t, method["signature"])
	parameters := classOwnedArray(t, signature["parameters"])
	body := classOwnedArray(t, method["body"])
	if len(parameters) != 2 || len(body) != 2 || signature["return"] != derived["instanceType"] ||
		classOwnedObject(t, parameters[0])["type"] != base["instanceType"] ||
		classOwnedObject(t, parameters[1])["type"] != derived["instanceType"] {
		t.Fatal("actual replace signature/body is incomplete")
	}
	first := classOwnedObject(t, body[0])
	local := classOwnedObject(t, first["local"])
	bindings := classOwnedArray(t, local["bindings"])
	if first["kind"] != "local" || local["kind"] != "let" || len(bindings) != 1 {
		t.Fatal("declared mutable Base slot was lost")
	}
	binding := classOwnedObject(t, bindings[0])
	initializer := classOwnedObject(t, binding["initializer"])
	if binding["type"] != base["instanceType"] || initializer["kind"] != "reference" ||
		initializer["symbol"] != classOwnedObject(t, parameters[0])["symbol"] || initializer["type"] != base["instanceType"] {
		t.Fatal("Base local initializer differs from the real incoming Base parameter")
	}
	baseMembers := map[any]bool{field["symbol"]: true}
	classLocalWitness(t, binding["compatibility"], base["instanceType"], base["instanceType"], baseMembers)
	returned := classOwnedObject(t, body[1])
	assignment := classOwnedExpression(t, body[1])
	left, right := classOwnedObject(t, assignment["left"]), classOwnedObject(t, assignment["right"])
	if returned["kind"] != "return" || assignment["kind"] != "local-assignment" || assignment["operator"] != "=" ||
		assignment["type"] != derived["instanceType"] || left["type"] != base["instanceType"] ||
		left["kind"] != "reference" || left["symbol"] != binding["symbol"] ||
		right["kind"] != "reference" || right["symbol"] != classOwnedObject(t, parameters[1])["symbol"] ||
		right["type"] != derived["instanceType"] {
		t.Fatal("consumed assignment replaced the Derived RHS/result with its declared Base storage slot")
	}
	classLocalWitness(t, assignment["compatibility"], derived["instanceType"], base["instanceType"], baseMembers)
	derivedMembers := map[any]bool{field["symbol"]: true, method["symbol"]: true}
	classLocalWitness(t, returned["compatibility"], derived["instanceType"], derived["instanceType"], derivedMembers)
	from := strings.Index(classLocalAssignmentResultSource, "replace(")
	classLocalOrigin(t, binding["origin"], classLocalAssignmentResultSource, "alias: Base = other", from)
	classLocalOrigin(t, assignment["origin"], classLocalAssignmentResultSource, "alias = replacement", from)
	classLocalOrigin(t, classOwnedObject(t, assignment["compatibility"])["origin"],
		classLocalAssignmentResultSource, "replacement", strings.Index(classLocalAssignmentResultSource, "alias = replacement"))
	classLocalOrigin(t, classOwnedObject(t, returned["compatibility"])["origin"],
		classLocalAssignmentResultSource, "alias = replacement", from)
	symbols := map[any]bool{}
	for _, item := range classOwnedArray(t, view["symbols"]) {
		symbol := classOwnedObject(t, item)
		if symbols[symbol["id"]] {
			t.Fatal("owned symbol identities collided")
		}
		symbols[symbol["id"]] = true
	}
	for _, id := range []any{field["symbol"], method["symbol"], binding["symbol"], left["symbol"], right["symbol"]} {
		if !symbols[id] {
			t.Fatal("consumed assignment/member projection contains an unowned symbol")
		}
	}
	canonical := classMultiCanonical(t, graph)
	if !bytes.Equal(canonical, classMultiCanonical(t, project())) {
		t.Fatal("consumed assignment graph changed across fresh official Programs")
	}
	originals := map[string]string{"/original/main.ts": classLocalAssignmentResultSource, "/original/minimal-scalars-v2.d.ts": scalarDeclarations}
	if len(graph.Sources) != len(originals) {
		t.Fatal("actual fixture source/declaration closure differs")
	}
	sourceRecords := [][]byte{}
	for _, source := range graph.Sources {
		text, known := originals[source.Path]
		if !known || source.SHA256 != classTextSHA(text) || source.UTF16Length != len(utf16.Encode([]rune(text))) ||
			source.Declaration != (source.Path != "/original/main.ts") {
			t.Fatal("actual source text cannot be paired with its complete owned identity")
		}
		if source.Path == "/original/main.ts" {
			if len(source.SurrogateStarts) != 1 || source.SurrogateStarts[0] != 3 {
				t.Fatal("actual original source lost its surrogate occurrence")
			}
		} else if len(source.SurrogateStarts) != 0 {
			t.Fatal("scalar declaration acquired an invented surrogate occurrence")
		}
		sourceRecords = append(sourceRecords, classMultiCanonical(t, map[string]any{"path": source.Path, "sha256": source.SHA256, "text": text}))
	}
	bindingBytes := classMultiCanonical(t, map[string]any{"semanticProfile": graph.SemanticProfile,
		"scope": graph.Scope, "sources": graph.Sources, "options": graph.Options})
	// Candidate bytes only: Root captures these stdout records after every check.
	// No fixture, Source executable, RPC or repository artifact is written here.
	fmt.Printf("TSFG_CLASS_OWNED_CANDIDATE_V5 local-assignment-result %s %s\n",
		classTextSHA(string(canonical)), base64.StdEncoding.EncodeToString(canonical))
	fmt.Printf("TSFG_CLASS_OWNED_CANDIDATE_BINDING_V5 local-assignment-result %s %s\n",
		classTextSHA(string(bindingBytes)), base64.StdEncoding.EncodeToString(bindingBytes))
	for _, record := range sourceRecords {
		fmt.Printf("TSFG_CLASS_OWNED_CANDIDATE_SOURCE_V5 local-assignment-result %s %s\n",
			classTextSHA(string(record)), base64.StdEncoding.EncodeToString(record))
	}
}
