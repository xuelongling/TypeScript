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

const classTerminalIfSource = `// 😀
export class Box {
  choose(flag: bool, other: Box): Box {
    if (flag) { return this; } else { return other; }
  }
}`

// Cold JSON checks use no new DTO members: baseline rejection must occur in
// projection after official Program diagnostics, rather than during compilation.
func TestOwnedClassSourceOwnsTerminalIfReferenceReturns(t *testing.T) {
	t.Parallel()
	project := func() classGraph {
		program, _ := genericProgramForTest(classTerminalIfSource)
		if err := checkClassDiagnostics(t.Context(), program); err != nil {
			t.Fatalf("terminal if fixture is not an officially valid Program: %v", err)
		}
		inputs := []classInput{}
		for _, file := range program.GetSourceFiles() {
			inputs = append(inputs, classInput{Path: file.FileName(), SHA256: classTextSHA(file.Text()), Declaration: file.IsDeclarationFile})
		}
		graph, err := exportClassGraph(t.Context(), program, []*ast.SourceFile{program.GetSourceFile("/original/main.ts")}, classExportOptions{
			Scope: "class:terminal-reference-if", Inputs: inputs, MaxNodes: 4096, MaxDepth: 64,
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
	canonical := classMultiCanonical(t, project())
	var cold classGraph
	if err := json.Unmarshal(canonical, &cold); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canonical, classMultiCanonical(t, cold)) {
		t.Fatal("independent cold serialization changed the terminal if graph")
	}
	view := classMultiView(t, cold)
	if view["semanticProfile"] != "tsfg-owned-class-facts-candidate-v5" || view["scope"] != "class:terminal-reference-if" {
		t.Fatal("terminal if requires its explicit candidate profile and scope")
	}
	classes := classOwnedArray(t, view["classes"])
	if len(classes) != 1 {
		t.Fatal("complete actual Box closure differs")
	}
	box := classOwnedNamed(t, classes, "Box")
	constructor := classOwnedObject(t, box["constructor"])
	if box["base"] != "" || len(classOwnedArray(t, box["fields"])) != 0 ||
		constructor["access"] != "public" || constructor["kind"] != "implicit-base" ||
		len(classOwnedArray(t, constructor["body"])) != 0 ||
		len(classOwnedArray(t, classOwnedObject(t, constructor["signature"])["parameters"])) != 0 {
		t.Fatal("terminal branch fixture changed its empty public default constructor")
	}
	if box["runtimeKey"] != classMultiExpectedKey(t, view, box) {
		t.Fatal("terminal branch lost independently checked complete v2 provenance")
	}
	methods := classOwnedArray(t, box["methods"])
	if len(methods) != 1 {
		t.Fatal("terminal branch method was dropped or invented")
	}
	method := classOwnedNamed(t, methods, "choose")
	signature := classOwnedObject(t, method["signature"])
	parameters := classOwnedArray(t, signature["parameters"])
	if method["access"] != "public" || len(parameters) != 2 || signature["return"] != box["instanceType"] {
		t.Fatal("actual choose signature differs")
	}
	flag, other := classOwnedObject(t, parameters[0]), classOwnedObject(t, parameters[1])
	if flag["type"] != "t:boolean" || other["type"] != box["instanceType"] || flag["symbol"] == other["symbol"] {
		t.Fatal("official boolean and reference parameters were confused")
	}
	body := classOwnedArray(t, method["body"])
	if len(body) != 1 {
		t.Fatal("terminal if must remain the sole actual method statement")
	}
	statement := classOwnedObject(t, body[0])
	if len(statement) != 6 || statement["kind"] != "if" || statement["local"] != nil ||
		statement["expression"] != nil || statement["compatibility"] != nil {
		t.Fatal("if was flattened or lost its exclusive branch payload")
	}
	branch := classOwnedObject(t, statement["branch"])
	if len(branch) != 4 {
		t.Fatal("terminal branch payload is incomplete or contains unknown fields")
	}
	condition := classOwnedObject(t, branch["condition"])
	if condition["id"] == nil || condition["id"] == "" || condition["kind"] != "reference" ||
		condition["symbol"] != flag["symbol"] || condition["type"] != flag["type"] {
		t.Fatal("condition lost its official boolean parameter occurrence")
	}
	classLocalWitness(t, branch["conditionCompatibility"], flag["type"], "t:boolean", nil)
	from := strings.Index(classTerminalIfSource, "if (flag)")
	classLocalOrigin(t, statement["origin"], classTerminalIfSource, "if (flag) { return this; } else { return other; }", from)
	classLocalOrigin(t, condition["origin"], classTerminalIfSource, "flag", from)
	classLocalOrigin(t, classOwnedObject(t, branch["conditionCompatibility"])["origin"], classTerminalIfSource, "flag", from)
	seenExpressions := map[any]bool{condition["id"]: true}
	for _, test := range []struct {
		side, token, expression, kind string
		symbol                        any
	}{
		{"then", "{ return this; }", "this", "this", box["symbol"]},
		{"else", "{ return other; }", "other", "reference", other["symbol"]},
	} {
		block := classOwnedObject(t, branch[test.side])
		blockBody := classOwnedArray(t, block["body"])
		if len(block) != 2 || len(blockBody) != 1 {
			t.Fatalf("%s lost its explicit block and single return", test.side)
		}
		returnedStatement := classOwnedObject(t, blockBody[0])
		returned := classOwnedObject(t, returnedStatement["expression"])
		if len(returnedStatement) != 5 || returnedStatement["kind"] != "return" || returnedStatement["local"] != nil ||
			returned["kind"] != test.kind || returned["symbol"] != test.symbol ||
			returned["id"] == nil || returned["id"] == "" || seenExpressions[returned["id"]] {
			t.Fatalf("%s lost its independent actual reference return", test.side)
		}
		seenExpressions[returned["id"]] = true
		if test.side == "then" {
			if returned["type"] != box["thisType"] && returned["type"] != box["instanceType"] {
				t.Fatal("then return lost the containing-class this type")
			}
		} else if returned["type"] != box["instanceType"] {
			t.Fatal("else return lost the actual other parameter type")
		}
		classLocalWitness(t, returnedStatement["compatibility"], returned["type"], box["instanceType"], map[any]bool{method["symbol"]: true})
		classLocalOrigin(t, block["origin"], classTerminalIfSource, test.token, from)
		returnFrom := strings.Index(classTerminalIfSource, "return "+test.expression+";")
		classLocalOrigin(t, returnedStatement["origin"], classTerminalIfSource, "return "+test.expression+";", returnFrom)
		classLocalOrigin(t, returned["origin"], classTerminalIfSource, test.expression, returnFrom)
		classLocalOrigin(t, classOwnedObject(t, returnedStatement["compatibility"])["origin"], classTerminalIfSource, test.expression, returnFrom)
	}
	classLocalOrigin(t, method["origin"], classTerminalIfSource,
		"choose(flag: bool, other: Box): Box {\n    if (flag) { return this; } else { return other; }\n  }", 0)
	classLocalOrigin(t, flag["origin"], classTerminalIfSource, "flag: bool", 0)
	classLocalOrigin(t, other["origin"], classTerminalIfSource, "other: Box", 0)
	if !bytes.Equal(canonical, classMultiCanonical(t, project())) {
		t.Fatal("terminal branch facts changed across fresh official Programs")
	}
	originals := map[string]string{"/original/main.ts": classTerminalIfSource, "/original/minimal-scalars-v2.d.ts": scalarDeclarations}
	if len(cold.Sources) != len(originals) {
		t.Fatal("terminal branch source/declaration closure differs")
	}
	sourceRecords := [][]byte{}
	for _, source := range cold.Sources {
		text, known := originals[source.Path]
		if !known || source.SHA256 != classTextSHA(text) || source.UTF16Length != len(utf16.Encode([]rune(text))) ||
			source.Declaration != (source.Path != "/original/main.ts") {
			t.Fatal("terminal branch lost complete original source identity")
		}
		if source.Path == "/original/main.ts" {
			if len(source.SurrogateStarts) != 1 || source.SurrogateStarts[0] != 3 {
				t.Fatal("terminal branch lost its original surrogate occurrence")
			}
		} else if len(source.SurrogateStarts) != 0 {
			t.Fatal("declaration acquired an invented surrogate occurrence")
		}
		sourceRecords = append(sourceRecords, classMultiCanonical(t, map[string]any{"path": source.Path, "sha256": source.SHA256, "text": text}))
	}
	bindingBytes := classMultiCanonical(t, map[string]any{"semanticProfile": cold.SemanticProfile,
		"scope": cold.Scope, "sources": cold.Sources, "options": cold.Options})
	// Actual candidate records follow all checks and confer no RPC/VM credit.
	fmt.Printf("TSFG_CLASS_OWNED_CANDIDATE_V5 terminal-reference-if %s %s\n",
		classTextSHA(string(canonical)), base64.StdEncoding.EncodeToString(canonical))
	fmt.Printf("TSFG_CLASS_OWNED_CANDIDATE_BINDING_V5 terminal-reference-if %s %s\n",
		classTextSHA(string(bindingBytes)), base64.StdEncoding.EncodeToString(bindingBytes))
	for _, record := range sourceRecords {
		fmt.Printf("TSFG_CLASS_OWNED_CANDIDATE_SOURCE_V5 terminal-reference-if %s %s\n",
			classTextSHA(string(record)), base64.StdEncoding.EncodeToString(record))
	}
}
