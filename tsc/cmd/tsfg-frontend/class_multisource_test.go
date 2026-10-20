// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/json"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

const classMultiMainPath = "/original/a-main.ts"
const classMultiBasePath = "/original/z-base.ts"
const classMultiDeclarationPath = "/original/minimal-scalars-v2.d.ts"

const classMultiBaseSource = `// 😀 base
export class Base {
  value: number;
  constructor(seed: number) { this.value = seed; }
  touch(seed: number): number { this.value = seed; return this.value; }
}`

const classMultiMainSource = `// 🧭 main
import { Base as Ancestor } from "./z-base";
export class Derived extends Ancestor {
  override touch(seed: number): number { return this.value; }
  make(seed: number): Ancestor {
    const made: Ancestor = new Ancestor(seed);
    made.touch(seed);
    return made;
  }
  rebind(other: Ancestor, replacement: Derived): void {
    let alias: Ancestor = other;
    alias = replacement;
  }
}`

// The host contains only explicit test VFS entries. Before projection, the
// actual official diagnostics must confirm a legal complete Program; a module
// resolution or syntax error can never substitute for the intended behavior RED.
func classMultiSourceExport(t *testing.T, main, base string, reverse bool) (classGraph, error) {
	t.Helper()
	fs := vfstest.FromMap(map[string]string{
		classMultiMainPath: main, classMultiBasePath: base, classMultiDeclarationPath: scalarDeclarations,
		"/original/external.d.ts": "export declare class Base { value: number; constructor(seed: number); touch(seed: number): number; }",
	}, true)
	roots := []string{classMultiMainPath, classMultiDeclarationPath}
	if reverse {
		slices.Reverse(roots)
	}
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
			FileNames: roots, CompilerOptions: &core.CompilerOptions{
				NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue,
				Target: core.ScriptTargetESNext, Module: core.ModuleKindESNext,
			},
		}}, Host: compiler.NewCompilerHost("/original", fs, "/original", nil, nil), SingleThreaded: core.TSTrue,
	})
	if err := checkClassDiagnostics(t.Context(), program); err != nil {
		t.Fatalf("multi-source fixture is not an officially valid Program: %v", err)
	}
	files := []*ast.SourceFile{}
	inputs := []classInput{}
	for _, file := range program.GetSourceFiles() {
		inputs = append(inputs, classInput{Path: file.FileName(), SHA256: classTextSHA(file.Text()), Declaration: file.IsDeclarationFile})
		if !file.IsDeclarationFile {
			files = append(files, file)
		}
	}
	slices.SortFunc(files, func(a, b *ast.SourceFile) int { return strings.Compare(a.FileName(), b.FileName()) })
	slices.SortFunc(inputs, func(a, b classInput) int { return strings.Compare(a.Path, b.Path) })
	if reverse {
		slices.Reverse(files)
		slices.Reverse(inputs)
	}
	graph, err := exportClassGraph(t.Context(), program, files, classExportOptions{
		Scope: "class:explicit-multiple-sources", Inputs: inputs, MaxNodes: 8192, MaxDepth: 64,
	})
	for _, input := range inputs {
		if writeErr := fs.WriteFile(input.Path, ""); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	return graph, err
}

func classMultiCanonical(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	canonical := json.Value(data)
	if err := canonical.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	return canonical
}

func classMultiView(t *testing.T, graph classGraph) map[string]any {
	t.Helper()
	var view map[string]any
	if err := json.Unmarshal(classMultiCanonical(t, graph), &view); err != nil {
		t.Fatal(err)
	}
	return view
}

func classMultiOrigin(t *testing.T, value any, path, source, token string, from int) {
	t.Helper()
	origin := classOwnedObject(t, value)
	offset := strings.Index(source[from:], token)
	if offset < 0 {
		t.Fatalf("multi-source fixture token missing: %q", token)
	}
	start, end := from+offset, from+offset+len(token)
	if origin["source"] != path || origin["start"] != float64(len(utf16.Encode([]rune(source[:start])))) ||
		origin["end"] != float64(len(utf16.Encode([]rune(source[:end])))) {
		t.Fatalf("multi-source original UTF16 anchor differs: token=%q origin=%+v", token, origin)
	}
}

// Independently cold-compute the already locked v2 provenance algorithm from
// owned metadata. No producer cache, checker pointer or private method is read.
func classMultiExpectedKey(t *testing.T, view, declaration map[string]any) string {
	t.Helper()
	hash := func(domain string, value any) string {
		canonical := classMultiCanonical(t, value)
		return fmt.Sprintf("sha256:%x", sha256.Sum256(append([]byte(domain), canonical...)))
	}
	environment := hash("tsfg.candidate.class-environment.v2\x00", map[string]any{
		"profile": view["semanticProfile"], "scope": view["scope"], "sources": view["sources"], "options": view["options"],
	})
	return hash("tsfg.candidate.class-allocation-provenance.v2\x00", map[string]any{
		"environmentDigest": environment, "declaration": declaration["origin"],
	})
}

func TestOwnedClassSourceOwnsExplicitMultipleSourceClassClosure(t *testing.T) {
	t.Parallel()
	graph, err := classMultiSourceExport(t, classMultiMainSource, classMultiBaseSource, false)
	if err != nil {
		t.Fatal(err)
	}
	view := classMultiView(t, graph)
	if view["semanticProfile"] != "tsfg-owned-class-facts-candidate-v5" || view["scope"] != "class:explicit-multiple-sources" {
		t.Fatal("multiple originals changed the existing candidate profile or closure scope")
	}
	sources := classOwnedArray(t, view["sources"])
	expected := map[string]string{
		classMultiMainPath: classMultiMainSource, classMultiBasePath: classMultiBaseSource, classMultiDeclarationPath: scalarDeclarations,
	}
	if len(sources) != len(expected) {
		t.Fatal("explicit complete original/declaration input closure differs")
	}
	previous := ""
	for _, item := range sources {
		source := classOwnedObject(t, item)
		path, ok := source["path"].(string)
		text, known := expected[path]
		if !ok || !known || path <= previous || source["sha256"] != classTextSHA(text) ||
			source["declaration"] != (path == classMultiDeclarationPath) ||
			source["utf16Length"] != float64(len(utf16.Encode([]rune(text)))) {
			t.Fatalf("owned explicit source identity/order differs: %+v", source)
		}
		previous = path
	}
	classes := classOwnedArray(t, view["classes"])
	if len(classes) != 2 {
		t.Fatal("complete original class closure lost or duplicated the imported Base")
	}
	derived, base := classOwnedNamed(t, classes, "Derived"), classOwnedNamed(t, classes, "Base")
	if derived["id"] != "c1" || base["id"] != "c2" || derived["base"] != base["id"] ||
		classOwnedObject(t, derived["origin"])["source"] != classMultiMainPath ||
		classOwnedObject(t, base["origin"])["source"] != classMultiBasePath {
		t.Fatal("source-sorted class identities were confused with base-before-derived dependency order")
	}
	for _, declaration := range []map[string]any{base, derived} {
		if declaration["runtimeKey"] != classMultiExpectedKey(t, view, declaration) {
			t.Fatal("candidate v2 key did not bind the complete multi-source environment")
		}
	}
	baseFields, derivedFields := classOwnedArray(t, base["fields"]), classOwnedArray(t, derived["fields"])
	if len(baseFields) != 1 || len(derivedFields) != 0 || len(classOwnedArray(t, base["methods"])) != 1 ||
		len(classOwnedArray(t, derived["methods"])) != 3 {
		t.Fatal("own class members were mixed with inherited or imported declarations")
	}
	field := classOwnedObject(t, baseFields[0])
	classMultiOrigin(t, field["origin"], classMultiBasePath, classMultiBaseSource, "value: number;", 0)
	baseTouch, derivedTouch := classOwnedNamed(t, base["methods"], "touch"), classOwnedNamed(t, derived["methods"], "touch")
	if derivedTouch["overrideTarget"] != baseTouch["id"] {
		t.Fatal("cross-source official override target was lost")
	}
	inherited := classOwnedReturnedExpression(t, derivedTouch)
	if inherited["symbol"] != field["symbol"] || classOwnedObject(t, inherited["receiver"])["symbol"] != derived["symbol"] {
		t.Fatal("Derived this.value did not retain the actual Base field symbol")
	}
	classMultiOrigin(t, inherited["origin"], classMultiMainPath, classMultiMainSource, "this.value", 0)
	baseCtor, derivedCtor := classOwnedObject(t, base["constructor"]), classOwnedObject(t, derived["constructor"])
	baseParameters := classOwnedArray(t, classOwnedObject(t, baseCtor["signature"])["parameters"])
	derivedParameters := classOwnedArray(t, classOwnedObject(t, derivedCtor["signature"])["parameters"])
	if len(baseParameters) != 1 || len(derivedParameters) != 1 || derivedCtor["kind"] != "implicit-derived" ||
		len(classOwnedArray(t, derivedCtor["body"])) != 0 || derivedCtor["superCall"] != "" ||
		derivedCtor["initializationPhase"] != "after-successful-super" {
		t.Fatal("cross-source implicit constructor lost its own logical forwarding or fabricated source body")
	}
	baseParameter, derivedParameter := classOwnedObject(t, baseParameters[0]), classOwnedObject(t, derivedParameters[0])
	if baseParameter["symbol"] == derivedParameter["symbol"] || derivedParameter["inheritedFrom"] != baseParameter["symbol"] {
		t.Fatal("source ordering overwrote the Base parameter with the Derived logical binding")
	}
	classMultiOrigin(t, baseParameter["origin"], classMultiBasePath, classMultiBaseSource, "seed: number", strings.Index(classMultiBaseSource, "constructor("))
	forward := classOwnedObject(t, derivedCtor["implicitForward"])
	mapping := classOwnedArray(t, forward["parameters"])
	if forward["target"] != baseCtor["id"] || len(mapping) != 1 ||
		classOwnedObject(t, mapping[0])["source"] != derivedParameter["symbol"] ||
		classOwnedObject(t, mapping[0])["target"] != baseParameter["symbol"] {
		t.Fatal("actual cross-source base signature/forward parameter edges differ")
	}
	classLocalWitness(t, classOwnedObject(t, mapping[0])["compatibility"], "t:number", "t:number", nil)
	baseBody := classOwnedArray(t, baseCtor["body"])
	if len(baseBody) != 1 || classOwnedObject(t, classOwnedExpression(t, baseBody[0])["right"])["symbol"] != baseParameter["symbol"] {
		t.Fatal("Base source constructor no longer owns its real seed reference")
	}
	makeMethod := classOwnedNamed(t, derived["methods"], "make")
	makeParameters := classOwnedArray(t, classOwnedObject(t, makeMethod["signature"])["parameters"])
	makeBody := classOwnedArray(t, makeMethod["body"])
	if len(makeParameters) != 1 || len(makeBody) != 3 || classOwnedObject(t, makeMethod["signature"])["return"] != base["instanceType"] {
		t.Fatal("complete new/local/call/return method is missing")
	}
	makeParameter := classOwnedObject(t, makeParameters[0])
	first := classOwnedObject(t, makeBody[0])
	local := classOwnedObject(t, first["local"])
	bindings := classOwnedArray(t, local["bindings"])
	if first["kind"] != "local" || local["kind"] != "const" || len(bindings) != 1 {
		t.Fatal("actual imported-constructor local binding is missing")
	}
	made := classOwnedObject(t, bindings[0])
	construct := classOwnedObject(t, made["initializer"])
	receiver := classOwnedObject(t, construct["receiver"])
	resolved := classOwnedObject(t, construct["resolvedSignature"])
	resolvedParameters := classOwnedArray(t, resolved["parameters"])
	arguments, witnesses := classOwnedArray(t, construct["arguments"]), classOwnedArray(t, construct["argumentCompatibility"])
	if made["type"] != base["instanceType"] || construct["kind"] != "construct" || construct["dispatch"] != "direct-constructor" ||
		construct["staticTarget"] != baseCtor["id"] || construct["type"] != base["instanceType"] ||
		receiver["kind"] != "reference" || receiver["symbol"] != base["symbol"] || receiver["type"] != base["constructorType"] ||
		resolved["return"] != base["instanceType"] || len(resolvedParameters) != 1 ||
		classOwnedObject(t, resolvedParameters[0])["symbol"] != baseParameter["symbol"] || len(arguments) != 1 || len(witnesses) != 1 ||
		classOwnedObject(t, arguments[0])["symbol"] != makeParameter["symbol"] {
		t.Fatal("import alias did not resolve to the real owned Base constructor/signature/caller argument")
	}
	classMultiOrigin(t, construct["origin"], classMultiMainPath, classMultiMainSource, "new Ancestor(seed)", 0)
	classMultiOrigin(t, receiver["origin"], classMultiMainPath, classMultiMainSource, "Ancestor", strings.Index(classMultiMainSource, "new Ancestor"))
	classLocalWitness(t, witnesses[0], "t:number", "t:number", nil)
	members := map[any]bool{field["symbol"]: true, baseTouch["symbol"]: true}
	classLocalWitness(t, made["compatibility"], base["instanceType"], base["instanceType"], members)
	call := classOwnedExpression(t, makeBody[1])
	callArguments, callWitnesses := classOwnedArray(t, call["arguments"]), classOwnedArray(t, call["argumentCompatibility"])
	touchParameters := classOwnedArray(t, classOwnedObject(t, baseTouch["signature"])["parameters"])
	callParameters := classOwnedArray(t, classOwnedObject(t, call["resolvedSignature"])["parameters"])
	if call["kind"] != "method-call" || call["dispatch"] != "virtual" || call["staticTarget"] != baseTouch["id"] ||
		classOwnedObject(t, call["receiver"])["symbol"] != made["symbol"] || len(callArguments) != 1 || len(callWitnesses) != 1 ||
		classOwnedObject(t, callArguments[0])["symbol"] != makeParameter["symbol"] || len(touchParameters) != 1 ||
		len(callParameters) != 1 || classOwnedObject(t, callParameters[0])["symbol"] != classOwnedObject(t, touchParameters[0])["symbol"] {
		t.Fatal("local alias call lost its actual Base method/parameter and caller binding")
	}
	classLocalWitness(t, callWitnesses[0], "t:number", "t:number", nil)
	returned := classOwnedExpression(t, makeBody[2])
	if returned["kind"] != "reference" || returned["symbol"] != made["symbol"] || returned["type"] != base["instanceType"] {
		t.Fatal("typed return replaced the actual constructed local object reference")
	}
	classLocalWitness(t, classOwnedObject(t, makeBody[2])["compatibility"], base["instanceType"], base["instanceType"], members)
	// Simple assignment has the official RHS result type, independently of the
	// declared Ancestor slot. Do not read alias after rebind: that needs flow-view.
	rebindMethod := classOwnedNamed(t, derived["methods"], "rebind")
	rebindSignature := classOwnedObject(t, rebindMethod["signature"])
	rebindParameters := classOwnedArray(t, rebindSignature["parameters"])
	rebindBody := classOwnedArray(t, rebindMethod["body"])
	if rebindSignature["return"] != "t:void" || len(rebindParameters) != 2 || len(rebindBody) != 2 {
		t.Fatal("typed rebind method lost its complete signature/body")
	}
	rebindLocal := classOwnedObject(t, classOwnedObject(t, rebindBody[0])["local"])
	rebindBindings := classOwnedArray(t, rebindLocal["bindings"])
	if rebindLocal["kind"] != "let" || len(rebindBindings) != 1 {
		t.Fatal("cross-type rebind lost its declared local slot")
	}
	alias := classOwnedObject(t, rebindBindings[0])
	aliasInitializer := classOwnedObject(t, alias["initializer"])
	if alias["type"] != base["instanceType"] || aliasInitializer["symbol"] != classOwnedObject(t, rebindParameters[0])["symbol"] ||
		aliasInitializer["type"] != base["instanceType"] {
		t.Fatal("Ancestor local initializer lost its actual declared slot/input")
	}
	classLocalWitness(t, alias["compatibility"], base["instanceType"], base["instanceType"], members)
	rebind := classOwnedExpression(t, rebindBody[1])
	rebindLeft, rebindRight := classOwnedObject(t, rebind["left"]), classOwnedObject(t, rebind["right"])
	if rebind["kind"] != "local-assignment" || rebind["operator"] != "=" || rebind["type"] != derived["instanceType"] ||
		rebindLeft["kind"] != "reference" || rebindLeft["symbol"] != alias["symbol"] || rebindLeft["type"] != base["instanceType"] ||
		rebindRight["kind"] != "reference" || rebindRight["symbol"] != classOwnedObject(t, rebindParameters[1])["symbol"] ||
		rebindRight["type"] != derived["instanceType"] {
		t.Fatal("assignment result/RHS was confused with the declared Ancestor LHS slot")
	}
	rebindWitness := classOwnedObject(t, rebind["compatibility"])
	rebindEdges := classOwnedArray(t, rebindWitness["members"])
	if rebindWitness["assignable"] != true || rebindWitness["sourceType"] != derived["instanceType"] ||
		rebindWitness["targetType"] != base["instanceType"] || len(rebindEdges) != 2 {
		t.Fatal("cross-type local assignment lost its official RHS-to-declared-slot relation")
	}
	expectedEdges := map[any]any{field["symbol"]: field["symbol"], baseTouch["symbol"]: derivedTouch["symbol"]}
	for _, item := range rebindEdges {
		edge := classOwnedObject(t, item)
		expectedSource, known := expectedEdges[edge["target"]]
		if !known || edge["source"] != expectedSource {
			t.Fatal("Derived-to-Ancestor relation lost its inherited field or actual override projection")
		}
		delete(expectedEdges, edge["target"])
	}
	if len(expectedEdges) != 0 {
		t.Fatal("Derived-to-Ancestor member relation is incomplete")
	}
	classMultiOrigin(t, rebind["origin"], classMultiMainPath, classMultiMainSource, "alias = replacement", 0)
	classMultiOrigin(t, rebindWitness["origin"], classMultiMainPath, classMultiMainSource, "replacement", strings.Index(classMultiMainSource, "alias = replacement"))
	reordered, err := classMultiSourceExport(t, classMultiMainSource, classMultiBaseSource, true)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(classMultiCanonical(t, graph), classMultiCanonical(t, reordered)) {
		t.Fatal("fresh Program/root/original/input enumeration changed canonical owned closure")
	}
	changedBase := strings.Replace(classMultiBaseSource, "return this.value;", "return this.value + this.value;", 1)
	changed, err := classMultiSourceExport(t, classMultiMainSource, changedBase, false)
	if err != nil {
		t.Fatal(err)
	}
	changedView := classMultiView(t, changed)
	for _, name := range []string{"Base", "Derived"} {
		before := classOwnedNamed(t, classes, name)
		after := classOwnedNamed(t, changedView["classes"], name)
		if before["runtimeKey"] == after["runtimeKey"] || after["runtimeKey"] != classMultiExpectedKey(t, changedView, after) {
			t.Fatal("changing only Base body failed to change the entire closure's source-bound provenance")
		}
	}
	for _, item := range classOwnedArray(t, changedView["sources"]) {
		source := classOwnedObject(t, item)
		if source["path"] == classMultiMainPath && source["sha256"] != classTextSHA(classMultiMainSource) {
			t.Fatal("base-only mutation changed the actual main source identity")
		}
	}
}

func TestOwnedClassSourceRejectsUnsupportedMultiSourceImports(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source string }{
		{"namespace", "// 🧭\nimport * as Env from \"./z-base\"; export class Derived extends Env.Base {}"},
		{"side-effect", "// 🧭\nimport \"./z-base\"; export class Derived {}"},
		{"outside-original", "// 🧭\nimport { Base as Ancestor } from \"./external\"; export class Derived extends Ancestor {}"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			graph, err := classMultiSourceExport(t, test.source, classMultiBaseSource, false)
			var failure *classFailure
			if !errors.As(err, &failure) || failure.Kind != "unsupported" || !strings.Contains(failure.Detail, "import") {
				t.Fatalf("legal import outside the supported boundary lost its specific capability diagnostic: %v", err)
			}
			start := strings.Index(test.source, "import")
			end := start + strings.Index(test.source[start:], ";") + 1
			if failure.Origin.Source != classMultiMainPath ||
				failure.Origin.Start != len(utf16.Encode([]rune(test.source[:start]))) ||
				failure.Origin.End != len(utf16.Encode([]rune(test.source[:end]))) {
				t.Fatal("unsupported import lost its complete original UTF16 statement anchor")
			}
			if !reflect.DeepEqual(graph, classGraph{}) {
				t.Fatal("unsupported import exposed a partial owned closure")
			}
		})
	}
}
