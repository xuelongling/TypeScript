// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"github.com/microsoft/typescript-go/internal/json"
	"strings"
	"testing"
)

// Decode through the public command seam and independent test-owned module data.
// This test file also compiles against Source0a3: its behavior RED is unknown
// compilationClosure input, rather than a test referring to missing Go fields.
type jointFactsForTest struct {
	Status      string
	Inputs      []string
	Diagnostics []diagnostic
	Modules     []struct {
		SourcePath string
		Globals    []string
	}
	Symbols   []struct{ ID, Name, SourcePath, BindingKind string }
	Functions []struct {
		Name, SourcePath, Symbol string
		Parameters               []symbol
		ParameterInitializers    []*expression
		Body                     []statement
	}
}

func jointCommandForTest(t *testing.T, r request, sources []inputFile, entry string) (jointFactsForTest, []byte) {
	t.Helper()
	declarations := []inputFile{}
	for _, file := range r.Files {
		if strings.HasSuffix(file.Path, ".d.ts") {
			declarations = append(declarations, file)
		}
	}
	r.RootFile = entry
	r.Files = append(append([]inputFile{}, sources...), declarations...)
	var data bytes.Buffer
	if err := json.NewLegacyEncoder(&data).Encode(r); err != nil {
		t.Fatal(err)
	}
	var input map[string]any
	if err := json.NewLegacyDecoder(bytes.NewReader(data.Bytes())).Decode(&input); err != nil {
		t.Fatal(err)
	}
	input["compilationClosure"] = map[string]any{
		"roots":         []map[string]string{{"kind": "application", "sourcePath": entry, "name": "entry"}},
		"requiredRoots": []any{},
	}
	data.Reset()
	if err := json.NewLegacyEncoder(&data).Encode(input); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runCommand(bytes.NewReader(data.Bytes()), &output); err != nil {
		t.Fatalf("owned joint request rejected before analysis: %v", err)
	}
	var result jointFactsForTest
	if err := json.NewLegacyDecoder(bytes.NewReader(output.Bytes())).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result, bytes.Clone(output.Bytes())
}

func TestJointNamespaceLiveBindingAndOrdinaryDefault(t *testing.T) {
	sources := []inputFile{
		{Path: "/src/app.ts", Content: "import * as state from './barrel'; export function entry(): i32 { return state.step(); }"},
		{Path: "/src/state.ts", Content: "export let count: i32 = 1; export function increment(delta: i32 = 2): i32 { count += delta; return count; }"},
		{Path: "/src/barrel.ts", Content: "export {increment as step, count} from './state';"},
	}
	result, raw := jointCommandForTest(t, integerRequest(""), sources, "/src/app.ts")
	if result.Status != "ok" || len(result.Modules) != 3 || len(result.Functions) != 2 {
		t.Fatalf("namespace/default composition rejected: %+v", result)
	}
	entry, increment := result.Functions[0], result.Functions[1]
	if entry.SourcePath != "/src/app.ts" || increment.SourcePath != "/src/state.ts" || entry.Body[0].Expression.Target != increment.Symbol {
		t.Fatal("namespace alias lost its official function target or source owner")
	}
	if len(increment.ParameterInitializers) != 1 || increment.ParameterInitializers[0] == nil || increment.ParameterInitializers[0].Literal == nil || increment.ParameterInitializers[0].Literal.Decimal != "2" {
		t.Fatal("ordinary parameter initializer was omitted or changed")
	}
	if increment.Parameters[0].Presence == nil || !increment.Parameters[0].Presence.Default || !increment.Parameters[0].Presence.Optional {
		t.Fatal("default selection presence was lost across a module call")
	}
	count := ""
	for _, s := range result.Symbols {
		if s.Name == "count" && s.SourcePath == "/src/state.ts" && s.BindingKind == "source" {
			count = s.ID
		}
	}
	if count == "" || increment.Body[1].Expression.Symbol != count {
		t.Fatal("module scalar read no longer refers to the live global binding")
	}
	sources[0], sources[2] = sources[2], sources[0]
	_, reordered := jointCommandForTest(t, integerRequest(""), sources, "/src/app.ts")
	if !bytes.Equal(raw, reordered) {
		t.Fatal("input order changed canonical owned module facts")
	}
}

func TestJointChecksUnenteredSourceBeforeProjection(t *testing.T) {
	result, _ := jointCommandForTest(t, integerRequest(""), []inputFile{
		{Path: "/src/app.ts", Content: "export function entry(): i32 { return 1; }"},
		{Path: "/src/unentered.ts", Content: "export function ignored(): bool { return 1; }"},
	}, "/src/app.ts")
	if result.Status != "error" {
		t.Fatal("unentered source escaped normal checking")
	}
	found := false
	for _, d := range result.Diagnostics {
		found = found || d.Primary.SourceID == "/src/unentered.ts" && d.UpstreamCode != 0
	}
	if !found {
		t.Fatalf("official unentered diagnostic lost source identity: %+v", result.Diagnostics)
	}
}

func TestJointPreservesNamespaceImportWriteDiagnostic(t *testing.T) {
	result, _ := jointCommandForTest(t, integerRequest(""), []inputFile{
		{Path: "/src/app.ts", Content: "import * as state from './state'; export function entry(): i32 { state.count = 4; return state.count; }"},
		{Path: "/src/state.ts", Content: "export let count: i32 = 1;"},
	}, "/src/app.ts")
	if result.Status != "error" {
		t.Fatal("namespace import write bypassed official readonly checking")
	}
	found := false
	for _, d := range result.Diagnostics {
		found = found || d.Primary.SourceID == "/src/app.ts" && d.UpstreamCode != 0
	}
	if !found {
		t.Fatal("namespace write error was replaced by unanchored capability rejection")
	}
}

func TestJointConversionOriginAcrossModules(t *testing.T) {
	result, _ := jointCommandForTest(t, conversionRequest(t, ""), []inputFile{
		{Path: "/src/app.ts", Content: "import {converted} from './state'; export function entry(): i32 { return converted(); }"},
		{Path: "/src/state.ts", Content: "export function converted(): i32 { return checked_i32_from_f64(4.75); }"},
	}, "/src/app.ts")
	if result.Status != "ok" || len(result.Functions) != 2 {
		t.Fatalf("conversion/module composition rejected: %+v", result)
	}
	value := result.Functions[1].Body[0].Expression
	if value.Kind != "conversion" || value.Type != "i32" || value.Conversion == nil || value.Conversion.DeclarationPath != "/src/minimal-conversions-v5.d.ts" || value.Conversion.EntryID == "" {
		t.Fatal("module conversion lost admitted declaration origin")
	}
	if value.Operand == nil || value.Operand.Type != "f64" || value.Operand.Float == nil || value.Operand.Float.Bits != "4013000000000000" {
		t.Fatal("module conversion changed the independent 4.75 f64 literal bits")
	}
}

func TestJointCommandStillRejectsUnknownFields(t *testing.T) {
	var out bytes.Buffer
	if err := runCommand(strings.NewReader("{\"unapprovedJointFlag\":true}"), &out); err == nil {
		t.Fatal("joint request widened arbitrary field admission")
	}
}
