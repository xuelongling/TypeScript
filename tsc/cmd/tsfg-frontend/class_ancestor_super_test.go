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
	"github.com/microsoft/typescript-go/internal/json"
)

const classAncestorSuperSource = `// 😀
export class Root {
  touch(seed: number): number { return seed; }
}
export class Middle extends Root {}
export class Leaf extends Middle {
  call(seed: number): number { return super.touch(seed); }
}`

// A super receiver is the direct base, while the resolved implementation can
// belong to an ancestor. Preserve both official facts without executing a call.
func TestOwnedClassSourceSuperMethodKeepsDirectBaseReceiverAndAncestorTarget(t *testing.T) {
	t.Parallel()
	project := func() classGraph {
		program, _ := genericProgramForTest(classAncestorSuperSource)
		if err := checkClassDiagnostics(t.Context(), program); err != nil {
			t.Fatalf("ancestor super fixture is not an officially valid Program: %v", err)
		}
		inputs := []classInput{}
		for _, file := range program.GetSourceFiles() {
			inputs = append(inputs, classInput{Path: file.FileName(), SHA256: classTextSHA(file.Text()), Declaration: file.IsDeclarationFile})
		}
		graph, err := exportClassGraph(t.Context(), program, []*ast.SourceFile{program.GetSourceFile("/original/main.ts")}, classExportOptions{
			Scope: "class:ancestor-super-method", Inputs: inputs, MaxNodes: 4096, MaxDepth: 64,
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
	canonical := classMultiCanonical(t, graph)
	var cold classGraph
	if err := json.Unmarshal(canonical, &cold); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canonical, classMultiCanonical(t, cold)) {
		t.Fatal("independent cold serialization changed the owned ancestor super graph")
	}
	view := classMultiView(t, cold)
	if view["semanticProfile"] != "tsfg-owned-class-facts-candidate-v5" || view["scope"] != "class:ancestor-super-method" {
		t.Fatal("ancestor super facts changed the existing Source5 profile or scope")
	}
	classes := classOwnedArray(t, view["classes"])
	if len(classes) != 3 {
		t.Fatal("actual Root/Middle/Leaf class closure differs")
	}
	root, middle, leaf := classOwnedNamed(t, classes, "Root"), classOwnedNamed(t, classes, "Middle"), classOwnedNamed(t, classes, "Leaf")
	if root["base"] != "" || middle["base"] != root["id"] || leaf["base"] != middle["id"] ||
		len(classOwnedArray(t, root["methods"])) != 1 || len(classOwnedArray(t, middle["methods"])) != 0 ||
		len(classOwnedArray(t, leaf["methods"])) != 1 {
		t.Fatal("ancestor method ownership was confused with direct inheritance")
	}
	for _, declaration := range []map[string]any{root, middle, leaf} {
		constructor := classOwnedObject(t, declaration["constructor"])
		if constructor["access"] != "public" || len(classOwnedArray(t, declaration["fields"])) != 0 ||
			len(classOwnedArray(t, constructor["body"])) != 0 {
			t.Fatal("actual public default constructors or empty field closure differ")
		}
		if declaration["runtimeKey"] != classMultiExpectedKey(t, view, declaration) {
			t.Fatal("ancestor super lost independently checked complete v2 allocation provenance")
		}
	}
	touch, callMethod := classOwnedNamed(t, root["methods"], "touch"), classOwnedNamed(t, leaf["methods"], "call")
	rootParameters := classOwnedArray(t, classOwnedObject(t, touch["signature"])["parameters"])
	leafParameters := classOwnedArray(t, classOwnedObject(t, callMethod["signature"])["parameters"])
	if len(rootParameters) != 1 || len(leafParameters) != 1 ||
		classOwnedObject(t, rootParameters[0])["symbol"] == classOwnedObject(t, leafParameters[0])["symbol"] {
		t.Fatal("actual ancestor and caller parameter bindings were merged")
	}
	call := classOwnedReturnedExpression(t, callMethod)
	receiver := classOwnedObject(t, call["receiver"])
	if call["kind"] != "method-call" || call["dispatch"] != "direct-base-method" ||
		call["staticTarget"] != touch["id"] || call["symbol"] != touch["symbol"] || call["type"] != "t:number" ||
		receiver["kind"] != "super" || receiver["symbol"] != middle["symbol"] {
		t.Fatal("official ancestor target or direct Middle super receiver was replaced")
	}
	var receiverType map[string]any
	for _, item := range classOwnedArray(t, view["types"]) {
		candidate := classOwnedObject(t, item)
		if candidate["id"] == receiver["type"] {
			receiverType = candidate
		}
	}
	if receiverType == nil || receiverType["declaration"] != middle["id"] {
		t.Fatal("super instance type must belong to direct Middle, not the Root method owner")
	}
	// Official super instance checking may retain its polymorphic Leaf this
	// argument. Both existing owned forms must still name exactly direct Middle.
	switch receiverType["kind"] {
	case "class-instance":
		if receiver["type"] != middle["instanceType"] {
			t.Fatal("direct super instance identity differs")
		}
	case "class-this-view":
		if receiverType["target"] != middle["instanceType"] || receiverType["thisArgument"] != leaf["thisType"] {
			t.Fatal("super view lost its actual direct base or Leaf this argument")
		}
	default:
		t.Fatal("super receiver is not an owned direct-base instance view")
	}
	permission := classOwnedObject(t, call["permission"])
	if permission["readable"] != true || permission["writeUse"] != false {
		t.Fatal("official super method access permission differs")
	}
	selected := classOwnedObject(t, call["resolvedSignature"])
	selectedParameters := classOwnedArray(t, selected["parameters"])
	arguments, witnesses := classOwnedArray(t, call["arguments"]), classOwnedArray(t, call["argumentCompatibility"])
	if selected["return"] != "t:number" || len(selectedParameters) != 1 || len(arguments) != 1 || len(witnesses) != 1 ||
		classOwnedObject(t, selectedParameters[0])["symbol"] != classOwnedObject(t, rootParameters[0])["symbol"] ||
		classOwnedObject(t, arguments[0])["symbol"] != classOwnedObject(t, leafParameters[0])["symbol"] {
		t.Fatal("official Root signature/caller Leaf operand provenance differs")
	}
	classLocalWitness(t, witnesses[0], "t:number", "t:number", nil)
	classLocalWitness(t, classOwnedObject(t, classOwnedArray(t, callMethod["body"])[0])["compatibility"], "t:number", "t:number", nil)
	rootReturned := classOwnedReturnedExpression(t, touch)
	if rootReturned["kind"] != "reference" || rootReturned["symbol"] != classOwnedObject(t, rootParameters[0])["symbol"] {
		t.Fatal("Root.touch lost its complete actual parameter return body")
	}
	from := strings.Index(classAncestorSuperSource, "call(")
	classLocalOrigin(t, touch["origin"], classAncestorSuperSource, "touch(seed: number): number { return seed; }", 0)
	classLocalOrigin(t, callMethod["origin"], classAncestorSuperSource, "call(seed: number): number { return super.touch(seed); }", from)
	classLocalOrigin(t, call["origin"], classAncestorSuperSource, "super.touch(seed)", from)
	classLocalOrigin(t, receiver["origin"], classAncestorSuperSource, "super", from)
	classLocalOrigin(t, classOwnedObject(t, selectedParameters[0])["origin"], classAncestorSuperSource, "seed: number", 0)
	classLocalOrigin(t, classOwnedObject(t, leafParameters[0])["origin"], classAncestorSuperSource, "seed: number", from)
	classLocalOrigin(t, classOwnedObject(t, arguments[0])["origin"], classAncestorSuperSource, "seed",
		strings.Index(classAncestorSuperSource, "super.touch(seed)"))
	classLocalOrigin(t, classOwnedObject(t, witnesses[0])["origin"], classAncestorSuperSource, "seed",
		strings.Index(classAncestorSuperSource, "super.touch(seed)"))
	if !bytes.Equal(canonical, classMultiCanonical(t, project())) {
		t.Fatal("ancestor super facts changed across fresh official Programs")
	}
	originals := map[string]string{"/original/main.ts": classAncestorSuperSource, "/original/minimal-scalars-v2.d.ts": scalarDeclarations}
	if len(cold.Sources) != len(originals) {
		t.Fatal("ancestor super source/declaration closure differs")
	}
	sourceRecords := [][]byte{}
	for _, source := range cold.Sources {
		text, known := originals[source.Path]
		if !known || source.SHA256 != classTextSHA(text) || source.UTF16Length != len(utf16.Encode([]rune(text))) ||
			source.Declaration != (source.Path != "/original/main.ts") {
			t.Fatal("ancestor super lost complete original source identity")
		}
		if source.Path == "/original/main.ts" {
			if len(source.SurrogateStarts) != 1 || source.SurrogateStarts[0] != 3 {
				t.Fatal("ancestor super lost its original surrogate occurrence")
			}
		} else if len(source.SurrogateStarts) != 0 {
			t.Fatal("declaration acquired an invented surrogate occurrence")
		}
		sourceRecords = append(sourceRecords, classMultiCanonical(t, map[string]any{"path": source.Path, "sha256": source.SHA256, "text": text}))
	}
	bindingBytes := classMultiCanonical(t, map[string]any{"semanticProfile": cold.SemanticProfile,
		"scope": cold.Scope, "sources": cold.Sources, "options": cold.Options})
	// Independent actual candidate records on stdout only, after all checks.
	fmt.Printf("TSFG_CLASS_OWNED_CANDIDATE_V5 ancestor-super-method %s %s\n",
		classTextSHA(string(canonical)), base64.StdEncoding.EncodeToString(canonical))
	fmt.Printf("TSFG_CLASS_OWNED_CANDIDATE_BINDING_V5 ancestor-super-method %s %s\n",
		classTextSHA(string(bindingBytes)), base64.StdEncoding.EncodeToString(bindingBytes))
	for _, record := range sourceRecords {
		fmt.Printf("TSFG_CLASS_OWNED_CANDIDATE_SOURCE_V5 ancestor-super-method %s %s\n",
			classTextSHA(string(record)), base64.StdEncoding.EncodeToString(record))
	}
}
