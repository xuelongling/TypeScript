// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/json"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

const classScalarSource = `export class Scalars {
  i: i32;
  f: f32;
  d: number;
  b: bool;
  constructor(i: i32, f: f32, d: number, b: bool) {
    this.i = i;
    this.f = f;
    this.d = d;
    this.b = b;
  }
  readI32(): i32 { return this.i; }
  readF32(): f32 { return this.f; }
  readNumber(): number { return this.d; }
  readBool(): bool { return this.b; }
}`

// scalar is the owned R01 meaning, independent of the local type-reference ID.
// A nominal record contains source/sha256/version plus the actual alias and
// unique-brand declaration anchors. A spelling alone never admits i32 or f32.
// This first fixture has no numeric literals or numeric conversion to obscure
// the real declared-type/body capability boundary.
func TestOwnedClassSourceOwnsAdmittedScalarTypesAndTypedBodies(t *testing.T) {
	t.Parallel()
	graph, declaration := classScalarGraphForTest(t)
	const declarationPath = "/original/minimal-conversions-v5.d.ts"
	data, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}
	var view map[string]any
	if err := json.Unmarshal(data, &view); err != nil {
		t.Fatal(err)
	}
	declarations := 0
	for _, item := range classOwnedArray(t, view["sources"]) {
		source := classOwnedObject(t, item)
		if source["path"] == declarationPath && source["declaration"] == true && source["sha256"] == conversionDeclarationHash {
			declarations++
		}
	}
	if declarations != 1 {
		t.Fatal("the actual canonical declaration dependency is not owned exactly once")
	}
	classes := classOwnedArray(t, view["classes"])
	if len(classes) != 1 {
		t.Fatalf("complete Scalars class closure differs: %d classes", len(classes))
	}
	scalars := classOwnedNamed(t, classes, "Scalars")
	fields, methods := classOwnedArray(t, scalars["fields"]), classOwnedArray(t, scalars["methods"])
	if len(fields) != 4 || len(methods) != 4 {
		t.Fatalf("complete scalar members differ: %d fields, %d methods", len(fields), len(methods))
	}
	types := map[any]map[string]any{}
	for _, item := range classOwnedArray(t, view["types"]) {
		record := classOwnedObject(t, item)
		types[record["id"]] = record
	}
	constructor := classOwnedObject(t, scalars["constructor"])
	parameters := classOwnedArray(t, classOwnedObject(t, constructor["signature"])["parameters"])
	stores := classOwnedArray(t, constructor["body"])
	if len(parameters) != 4 || len(stores) != 4 {
		t.Fatal("complete same-type scalar constructor parameters/assignments are missing")
	}
	used := map[any]bool{}
	for index, expected := range []struct{ field, method, scalar string }{
		{"i", "readI32", "i32"}, {"f", "readF32", "f32"},
		{"d", "readNumber", "f64"}, {"b", "readBool", "bool"},
	} {
		field := classOwnedNamed(t, fields, expected.field)
		typeID := field["type"]
		record := types[typeID]
		if record == nil || record["scalar"] != expected.scalar || used[typeID] || field["initializer"] != nil {
			t.Fatalf("actual declared scalar semantics were collapsed or initialized: field=%+v type=%+v", field, record)
		}
		used[typeID] = true
		if expected.scalar == "i32" || expected.scalar == "f32" {
			nominal := classOwnedObject(t, record["nominal"])
			alias, brand := classOwnedObject(t, nominal["alias"]), classOwnedObject(t, nominal["brand"])
			aliasText := "type " + expected.scalar + " = number & { readonly [__tsfg_" + expected.scalar + "]: true };"
			aliasStart := strings.Index(string(declaration), aliasText)
			brandText := "__tsfg_" + expected.scalar
			brandStart := strings.Index(string(declaration), brandText)
			if nominal["source"] != declarationPath || nominal["sha256"] != conversionDeclarationHash || nominal["version"] != conversionVersion ||
				alias["source"] != declarationPath || alias["start"] != float64(aliasStart) || alias["end"] != float64(aliasStart+len(aliasText)) ||
				brand["source"] != declarationPath || brand["start"] != float64(brandStart) || brand["end"] != float64(brandStart+len(brandText)) {
				t.Fatalf("nominal scalar lost the exact admitted alias/unique-brand declaration: %+v", nominal)
			}
		}
		parameter := classOwnedObject(t, parameters[index])
		store := classOwnedExpression(t, stores[index])
		left, right := classOwnedObject(t, store["left"]), classOwnedObject(t, store["right"])
		if parameter["type"] != typeID || store["kind"] != "assignment" || left["symbol"] != field["symbol"] ||
			left["type"] != typeID || right["symbol"] != parameter["symbol"] || right["type"] != typeID {
			t.Fatalf("typed scalar constructor assignment lost actual bindings/types: %+v", store)
		}
		method := classOwnedNamed(t, methods, expected.method)
		returned := classOwnedReturnedExpression(t, method)
		if classOwnedObject(t, method["signature"])["return"] != typeID || returned["kind"] != "field" ||
			returned["symbol"] != field["symbol"] || returned["type"] != typeID {
			t.Fatalf("typed scalar return lost its actual field/type: %+v", method)
		}
		returnStatement := classOwnedObject(t, classOwnedArray(t, method["body"])[0])
		for _, value := range []any{store["compatibility"], returnStatement["compatibility"]} {
			witness := classOwnedObject(t, value)
			if witness["assignable"] != true || witness["sourceType"] != typeID || witness["targetType"] != typeID ||
				len(classOwnedArray(t, witness["members"])) != 0 {
				t.Fatalf("same scalar conversion was replaced by a reference/member projection: %+v", witness)
			}
		}
	}
}

// Each use builds a fresh official Program in the same canonical environment.
// Return only owned facts and declaration bytes after the source VFS is cleared.
func classScalarGraphForTest(t *testing.T) (classGraph, []byte) {
	t.Helper()
	declaration, err := os.ReadFile("testdata/minimal-conversions-v5.d.ts")
	if err != nil {
		t.Fatal(err)
	}
	if classTextSHA(string(declaration)) != conversionDeclarationHash {
		t.Fatal("canonical scalar declaration bytes differ from the admitted environment")
	}
	const sourcePath = "/original/main.ts"
	const declarationPath = "/original/minimal-conversions-v5.d.ts"
	fs := vfstest.FromMap(map[string]string{sourcePath: classScalarSource, declarationPath: string(declaration)}, true)
	options := &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue,
		Target: core.ScriptTargetESNext, Module: core.ModuleKindESNext}
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
			FileNames: []string{sourcePath, declarationPath}, CompilerOptions: options,
		}}, Host: compiler.NewCompilerHost("/original", fs, "/original", nil, nil), SingleThreaded: core.TSTrue,
	})
	graph, err := exportClassGraph(t.Context(), program, []*ast.SourceFile{program.GetSourceFile(sourcePath)}, classExportOptions{
		Scope: "class:admitted-scalars", Inputs: []classInput{
			{Path: sourcePath, SHA256: classTextSHA(classScalarSource)},
			{Path: declarationPath, SHA256: conversionDeclarationHash, Declaration: true},
		}, MaxNodes: 8192, MaxDepth: 64,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := fs.WriteFile(sourcePath, ""); err != nil {
		t.Fatal(err)
	}
	return graph, declaration
}
