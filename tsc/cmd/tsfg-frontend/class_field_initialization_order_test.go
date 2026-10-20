// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/json"
)

const classFieldInitializationOrderSource = `// 😀
export class Base {
  baseValue: number = 1;
}
export class Derived extends Base {
  derivedValue: number = 2;
  constructor() { super(); }
}`

// Source owns the two initialization phases and the actual super occurrence.
// It preserves unresolved literals; no constructor or numeric value is executed.
func TestOwnedClassSourceOwnsInheritedFieldInitializationOrder(t *testing.T) {
	t.Parallel()
	project := func() classGraph {
		program, _ := genericProgramForTest(classFieldInitializationOrderSource)
		if err := checkClassDiagnostics(t.Context(), program); err != nil {
			t.Fatalf("field initialization fixture is not an officially valid Program: %v", err)
		}
		inputs := []classInput{}
		for _, file := range program.GetSourceFiles() {
			inputs = append(inputs, classInput{Path: file.FileName(), SHA256: classTextSHA(file.Text()), Declaration: file.IsDeclarationFile})
		}
		graph, err := exportClassGraph(t.Context(), program, []*ast.SourceFile{program.GetSourceFile("/original/main.ts")}, classExportOptions{
			Scope: "class:field-initialization-order", Inputs: inputs, MaxNodes: 4096, MaxDepth: 64,
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
		t.Fatal("cold DTO serialization changed the owned field initialization graph")
	}
	view := classMultiView(t, cold)
	if view["semanticProfile"] != "tsfg-owned-class-facts-candidate-v5" || view["scope"] != "class:field-initialization-order" {
		t.Fatal("initialization order changed the existing Source5 profile or scope")
	}
	classes := classOwnedArray(t, view["classes"])
	if len(classes) != 2 {
		t.Fatal("actual Base/Derived closure differs")
	}
	base, derived := classOwnedNamed(t, classes, "Base"), classOwnedNamed(t, classes, "Derived")
	if base["base"] != "" || derived["base"] != base["id"] || base["id"] == derived["id"] ||
		base["symbol"] == derived["symbol"] || base["runtimeKey"] == derived["runtimeKey"] {
		t.Fatal("distinct official classes or their direct base relation differ")
	}
	types := map[any]map[string]any{}
	for _, item := range classOwnedArray(t, view["types"]) {
		record := classOwnedObject(t, item)
		if types[record["id"]] != nil {
			t.Fatal("owned type identity collided")
		}
		types[record["id"]] = record
	}
	symbols := map[any]map[string]any{}
	for _, item := range classOwnedArray(t, view["symbols"]) {
		record := classOwnedObject(t, item)
		if symbols[record["id"]] != nil {
			t.Fatal("owned symbol identity collided")
		}
		symbols[record["id"]] = record
	}
	if len(symbols) != 4 {
		t.Fatal("initialization fixture lost its complete two-class/two-field symbol closure")
	}
	baseCtor, derivedCtor := classOwnedObject(t, base["constructor"]), classOwnedObject(t, derived["constructor"])
	if baseCtor["id"] == derivedCtor["id"] || baseCtor["kind"] != "implicit-base" || derivedCtor["kind"] != "explicit" ||
		baseCtor["superCall"] != "" || baseCtor["initializationPhase"] != "before-body" ||
		derivedCtor["initializationPhase"] != "after-successful-super" ||
		baseCtor["implicitForward"] != nil || derivedCtor["implicitForward"] != nil ||
		len(classOwnedArray(t, baseCtor["body"])) != 0 {
		t.Fatal("entry and explicit after-super boundaries were replaced by invented constructor bodies")
	}
	baseText := "export class Base {\n  baseValue: number = 1;\n}"
	derivedText := "export class Derived extends Base {\n  derivedValue: number = 2;\n  constructor() { super(); }\n}"
	classLocalOrigin(t, base["origin"], classFieldInitializationOrderSource, baseText, 0)
	classLocalOrigin(t, derived["origin"], classFieldInitializationOrderSource, derivedText, 0)
	if !reflect.DeepEqual(baseCtor["origin"], base["origin"]) {
		t.Fatal("implicit root constructor fabricated a source occurrence")
	}
	classLocalOrigin(t, derivedCtor["origin"], classFieldInitializationOrderSource, "constructor() { super(); }", 0)
	seenExpressions := map[any]bool{}
	for index, expected := range []struct{ name, field, token string }{
		{"Base", "baseValue", "1"}, {"Derived", "derivedValue", "2"},
	} {
		declaration := []map[string]any{base, derived}[index]
		constructor := classOwnedObject(t, declaration["constructor"])
		signature := classOwnedObject(t, constructor["signature"])
		fields := classOwnedArray(t, declaration["fields"])
		if constructor["access"] != "public" || len(classOwnedArray(t, signature["parameters"])) != 0 ||
			signature["return"] != declaration["instanceType"] || len(fields) != 1 || len(classOwnedArray(t, declaration["methods"])) != 0 {
			t.Fatal("public zero-argument constructor or own field closure differs")
		}
		if declaration["runtimeKey"] != classMultiExpectedKey(t, view, declaration) {
			t.Fatal("field initialization lost independently checked full v2 allocation provenance")
		}
		for _, role := range []struct{ key, kind string }{
			{"instanceType", "class-instance"}, {"constructorType", "class-constructor"}, {"thisType", "class-this"},
		} {
			record := types[declaration[role.key]]
			if record == nil || record["kind"] != role.kind || record["declaration"] != declaration["id"] {
				t.Fatal("class type roles were merged or detached from their official declaration")
			}
		}
		classSymbol := symbols[declaration["symbol"]]
		if classSymbol == nil || classSymbol["kind"] != "class" || classSymbol["name"] != expected.name ||
			classSymbol["type"] != declaration["constructorType"] {
			t.Fatal("official class symbol lost its constructor-value type")
		}
		classLocalOrigin(t, classSymbol["declaration"], classFieldInitializationOrderSource, expected.name,
			strings.Index(classFieldInitializationOrderSource, "class "+expected.name))
		field := classOwnedNamed(t, fields, expected.field)
		initializationFields := classOwnedArray(t, constructor["initializationFields"])
		if len(initializationFields) != 1 || initializationFields[0] != field["id"] || field["ordinal"] != float64(0) ||
			field["access"] != "public" || field["readonly"] != false || field["definiteAssertion"] != false {
			t.Fatal("constructor initialization order mixed inherited fields with the actual own field")
		}
		fieldSymbol := symbols[field["symbol"]]
		if fieldSymbol == nil || fieldSymbol["kind"] != "field" || fieldSymbol["type"] != field["type"] || fieldSymbol["name"] != expected.field {
			t.Fatal("actual field lost its owned symbol/type binding")
		}
		fieldText := expected.field + ": number = " + expected.token + ";"
		classLocalOrigin(t, field["origin"], classFieldInitializationOrderSource, fieldText, 0)
		classLocalOrigin(t, fieldSymbol["declaration"], classFieldInitializationOrderSource, expected.field, 0)
		value := classOwnedObject(t, field["initializer"])
		literal := classOwnedObject(t, value["numericLiteral"])
		narrow, context := types[value["type"]], types[field["type"]]
		if value["id"] == nil || value["id"] == "" || seenExpressions[value["id"]] ||
			value["kind"] != "number" || value["token"] != expected.token || narrow == nil ||
			narrow["kind"] != "number-literal" || narrow["literal"] != expected.token || narrow["scalar"] != "" ||
			context == nil || context["kind"] != "number" || context["scalar"] != "f64" || value["type"] == field["type"] ||
			literal["state"] != "unresolved" || literal["officialType"] != value["type"] ||
			literal["defaultScalar"] != "i32" || literal["contextType"] != field["type"] {
			t.Fatal("initializer lost its exact token, unresolved narrow type or actual number slot context")
		}
		seenExpressions[value["id"]] = true
		from := strings.Index(classFieldInitializationOrderSource, fieldText)
		classLocalOrigin(t, value["origin"], classFieldInitializationOrderSource, expected.token,
			from+strings.Index(fieldText, "= ")+2)
		classLocalWitness(t, field["initializerCompatibility"], value["type"], field["type"], nil)
		if !reflect.DeepEqual(classOwnedObject(t, field["initializerCompatibility"])["origin"], value["origin"]) {
			t.Fatal("initializer witness points to a different occurrence")
		}
	}
	body := classOwnedArray(t, derivedCtor["body"])
	if len(body) != 1 {
		t.Fatal("derived constructor must retain only its actual super statement")
	}
	statement := classOwnedObject(t, body[0])
	call := classOwnedExpression(t, body[0])
	receiver := classOwnedObject(t, call["receiver"])
	selected := classOwnedObject(t, call["resolvedSignature"])
	if statement["kind"] != "expression" || statement["compatibility"] != nil || statement["local"] != nil ||
		call["kind"] != "super-construct" || call["dispatch"] != "direct-base-constructor" ||
		call["id"] == nil || call["id"] == "" || seenExpressions[call["id"]] || derivedCtor["superCall"] != call["id"] ||
		call["staticTarget"] != baseCtor["id"] || call["type"] != "t:void" ||
		receiver["kind"] != "super" || receiver["symbol"] != base["symbol"] || receiver["type"] != base["constructorType"] ||
		selected["return"] != base["instanceType"] || len(classOwnedArray(t, selected["parameters"])) != 0 ||
		len(classOwnedArray(t, call["arguments"])) != 0 || len(classOwnedArray(t, call["argumentCompatibility"])) != 0 {
		t.Fatal("successful-super boundary lost its real direct Base target, constructor receiver or zero-argument signature")
	}
	classLocalOrigin(t, statement["origin"], classFieldInitializationOrderSource, "super();", 0)
	classLocalOrigin(t, call["origin"], classFieldInitializationOrderSource, "super()", 0)
	classLocalOrigin(t, receiver["origin"], classFieldInitializationOrderSource, "super", 0)
	if !bytes.Equal(canonical, classMultiCanonical(t, project())) {
		t.Fatal("field initialization facts changed across fresh official Programs")
	}
	originals := map[string]string{"/original/main.ts": classFieldInitializationOrderSource, "/original/minimal-scalars-v2.d.ts": scalarDeclarations}
	if len(cold.Sources) != len(originals) {
		t.Fatal("field initialization lost its complete original source/declaration closure")
	}
	sourceRecords := [][]byte{}
	for _, source := range cold.Sources {
		text, known := originals[source.Path]
		if !known || source.SHA256 != classTextSHA(text) || source.UTF16Length != len(utf16.Encode([]rune(text))) ||
			source.Declaration != (source.Path != "/original/main.ts") {
			t.Fatal("field initialization lost complete original source identity")
		}
		if source.Path == "/original/main.ts" {
			if len(source.SurrogateStarts) != 1 || source.SurrogateStarts[0] != 3 {
				t.Fatal("field initialization lost its original surrogate occurrence")
			}
		} else if len(source.SurrogateStarts) != 0 {
			t.Fatal("declaration acquired an invented surrogate occurrence")
		}
		sourceRecords = append(sourceRecords, classMultiCanonical(t, map[string]any{"path": source.Path, "sha256": source.SHA256, "text": text}))
	}
	bindingBytes := classMultiCanonical(t, map[string]any{"semanticProfile": cold.SemanticProfile,
		"scope": cold.Scope, "sources": cold.Sources, "options": cold.Options})
	// Actual V5 records only after every check; no repository artifact is written.
	fmt.Printf("TSFG_CLASS_OWNED_CANDIDATE_V5 field-initialization-order %s %s\n",
		classTextSHA(string(canonical)), base64.StdEncoding.EncodeToString(canonical))
	fmt.Printf("TSFG_CLASS_OWNED_CANDIDATE_BINDING_V5 field-initialization-order %s %s\n",
		classTextSHA(string(bindingBytes)), base64.StdEncoding.EncodeToString(bindingBytes))
	for _, record := range sourceRecords {
		fmt.Printf("TSFG_CLASS_OWNED_CANDIDATE_SOURCE_V5 field-initialization-order %s %s\n",
			classTextSHA(string(record)), base64.StdEncoding.EncodeToString(record))
	}
}
