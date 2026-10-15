// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"fmt"
	"github.com/microsoft/typescript-go/internal/json"
	"slices"
	"strings"
	"testing"
)

func moduleRequest(files []inputFile, entry string) request {
	r := scalarRequest("")
	r.RootFile = entry
	r.Files = append(slices.Clone(files), inputFile{Path: "/src/minimal-scalars-v2.d.ts", Content: scalarDeclarations})
	r.CompilationClosure = &closureConfiguration{Roots: []closureRoot{{Kind: "application", SourcePath: entry, Name: "entry"}}, RequiredRoots: []closureRoot{}}
	return r
}

func TestModuleFactsStrictFloatComposition(t *testing.T) {
	for _, kind := range []string{"f32", "f64"} {
		t.Run(kind, func(t *testing.T) {
			r := floatRequest("")
			r.Files[0].Content = fmt.Sprintf("import * as state from './barrel'; export function entry(): %s { return state.step(); }", kind)
			r.Files = append(r.Files,
				inputFile{Path: "/src/state.ts", Content: fmt.Sprintf("export let count: %s = 1.0; export function increment(): %s { count = count + 1.0; return count; }", kind, kind)},
				inputFile{Path: "/src/barrel.ts", Content: "export { increment as step, count } from './state';"})
			r.CompilationClosure = &closureConfiguration{Roots: []closureRoot{{Kind: "application", SourcePath: r.RootFile, Name: "entry"}}, RequiredRoots: []closureRoot{}}
			result := analyze(r)
			if result.Status != "ok" || len(result.Modules) != 3 || len(result.Functions) != 2 {
				t.Fatalf("float module composition rejected: %+v", result)
			}
			state := result.Modules[2]
			literal := state.Initialization[0].Initializer
			wantBits := "3f800000"
			if kind == "f64" {
				wantBits = "3ff0000000000000"
			}
			if literal.Kind != "floating" || literal.Type != kind || literal.Float.Bits != wantBits {
				t.Fatalf("float initialization facts changed: %+v", literal)
			}
			if result.Functions[0].Body[0].Expression.Name != "state.step" || result.Functions[0].Body[0].Expression.Target != result.Functions[1].Symbol {
				t.Fatal("namespace float call lost checker target or source spelling")
			}
		})
	}
	for _, row := range []struct{ source, code string }{
		{"export let count: f32 = 1e1000; export function entry(): f32 { return count; }", "r01-d-float-range"},
		{"export let count: i32 = 0.1; export function entry(): i32 { return count; }", "r01-d-scalar-type"},
	} {
		r := floatRequest(row.source)
		r.CompilationClosure = &closureConfiguration{Roots: []closureRoot{{Kind: "application", SourcePath: r.RootFile, Name: "entry"}}, RequiredRoots: []closureRoot{}}
		result := analyze(r)
		if result.Status != "error" || len(result.Diagnostics) == 0 || result.Diagnostics[0].Code != row.code {
			t.Fatalf("module float boundary changed: want %s: %+v", row.code, result)
		}
	}
}

func TestModuleFactsLiveBindingAndForwardCall(t *testing.T) {
	files := []inputFile{{Path: "/src/app.ts", Content: "import { count, increment } from './state'; export function entry(): i32 { increment(); return count; }"}, {Path: "/src/state.ts", Content: "export let count: i32 = 1; export function increment(): i32 { count = count + 1; return count; }"}}
	r := moduleRequest(files, "/src/app.ts")
	result := analyze(r)
	if result.Status != "ok" {
		t.Fatalf("module source rejected: %+v", result.Diagnostics)
	}
	if len(result.Modules) != 2 || len(result.Functions) != 2 || len(result.Modules[0].Imports) != 1 || len(result.Modules[1].Globals) != 1 || len(result.Modules[1].Initialization) != 1 {
		t.Fatalf("module facts absent: %+v", result)
	}
	imported := result.Modules[0].Imports[0].Bindings[0].Target
	read := result.Functions[0].Body[1].Expression
	if read.Symbol != imported || read.Name != "count" || result.Functions[0].SourcePath != "/src/app.ts" {
		t.Fatalf("live binding lost: %+v", read)
	}
	if result.Functions[0].Body[0].Expression.Target != result.Functions[1].Symbol {
		t.Fatal("forward imported call has wrong official target")
	}
	var first bytes.Buffer
	if err := json.NewLegacyEncoder(&first).Encode(result); err != nil {
		t.Fatal(err)
	}
	slices.Reverse(r.Files)
	var second bytes.Buffer
	if err := json.NewLegacyEncoder(&second).Encode(analyze(r)); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() {
		t.Fatal("input enumeration changed owned facts")
	}
}

func TestModuleFactsImportedAliasAndDeclarationCycle(t *testing.T) {
	r := moduleRequest([]inputFile{{Path: "/src/a.ts", Content: "import { odd as other } from './b'; export function even(n: i32): bool { if (n === 0) return true; return other(n - 1); } export function entry(): bool { return even(4); }"}, {Path: "/src/b.ts", Content: "import { even } from './a'; export function odd(n: i32): bool { if (n === 0) return false; return even(n - 1); }"}}, "/src/a.ts")
	result := analyze(r)
	if result.Status != "ok" {
		t.Fatalf("legal declaration cycle rejected: %+v", result.Diagnostics)
	}
	call := result.Functions[0].Body[1].Expression
	if call.Name != "other" || call.Target != result.Functions[2].Symbol {
		t.Fatalf("alias target/spelling lost: %+v", call)
	}
}

func TestModuleFactsNormalCheckingAndForbiddenDependencies(t *testing.T) {
	for _, files := range [][]inputFile{
		{{Path: "/src/app.ts", Content: "export function entry(): i32 { return 42; }"}, {Path: "/src/unentered.ts", Content: "export function unused(): i32 { return absentName; }"}},
		{{Path: "/src/app.ts", Content: "import { value } from './legacy.js'; export function entry(): i32 { return value; }"}, {Path: "/src/legacy.js", Content: "export const value = 42;"}},
	} {
		result := analyze(moduleRequest(files, "/src/app.ts"))
		if result.Status != "error" || len(result.Diagnostics) == 0 {
			t.Fatalf("invalid unentered/JS source accepted: %+v", result)
		}
	}
}

func TestModuleFactsGlobalInitializationAndScriptJudgement(t *testing.T) {
	r := moduleRequest([]inputFile{{Path: "/src/app.ts", Content: "import { observed } from './init'; export function entry(): i32 { return observed; }"}, {Path: "/src/init.ts", Content: "import { count } from './state'; export let observed: i32 = count + 1;"}, {Path: "/src/state.ts", Content: "export let count: i32 = 1;"}, {Path: "/src/script.ts", Content: "function scriptValue(): i32 { return 7; }"}}, "/src/app.ts")
	result := analyze(r)
	if result.Status != "ok" {
		t.Fatalf("global dependency init rejected: %+v", result.Diagnostics)
	}
	for _, m := range result.Modules {
		if m.SourcePath == "/src/script.ts" && m.External {
			t.Fatal("script was forced into module")
		}
	}
}

func TestModuleFactsActualHostCapabilityDeclaration(t *testing.T) {
	r := moduleRequest([]inputFile{{Path: "/src/app.ts", Content: "export let first: i32 = record(7); export let fail: i32 = 1 / 0; export function entry(): i32 { return first; }"}, {Path: "/src/host.d.ts", Content: "declare function record(value: i32): i32;"}}, "/src/app.ts")
	r.Declarations.Paths = append(r.Declarations.Paths, "/src/host.d.ts")
	r.HostImports = []hostImport{{SourcePath: "/src/host.d.ts", Name: "record", CapabilityID: 31, ParameterTypes: []string{"i32"}, ReturnType: "i32"}}
	result := analyze(r)
	if result.Status != "ok" || len(result.HostImports) != 1 {
		t.Fatalf("explicit checked host import rejected: %+v", result)
	}
	if result.Modules[0].Initialization[0].Initializer.Target != result.HostImports[0].Symbol {
		t.Fatal("host call identity lost")
	}
}

func TestModuleFactsIntegerFamilyAndNominalChecks(t *testing.T) {
	for _, tc := range integerOracle {
		t.Run(tc.kind, func(t *testing.T) {
			r := integerRequest("")
			r.RootFile = "/src/app.ts"
			r.Files = append([]inputFile{{Path: "/src/app.ts", Content: fmt.Sprintf("import { value, read } from './state'; export function entry(): %s { read(); return value; }", tc.kind)}, {Path: "/src/state.ts", Content: fmt.Sprintf("export let value: %s = %s; export function read(): %s { return value; }", tc.kind, tc.maximum, tc.kind)}}, r.Files[1])
			r.CompilationClosure = &closureConfiguration{Roots: []closureRoot{{Kind: "application", SourcePath: r.RootFile, Name: "entry"}}, RequiredRoots: []closureRoot{}}
			result := analyze(r)
			if result.Status != "ok" {
				t.Fatalf("module integer nominal facts rejected: %+v", result.Diagnostics)
			}
			init := result.Modules[1].Initialization[0].Initializer
			if init.Type != tc.kind || init.Literal.Decimal != tc.maximum {
				t.Fatalf("module integer was narrowed: %+v", init)
			}
			r.Files[1].Content = fmt.Sprintf("export let value: %s = %s; export function read(): %s { return value; }", tc.kind, tc.above, tc.kind)
			bad := analyze(r)
			if bad.Status != "error" || len(bad.Diagnostics) == 0 || bad.Diagnostics[0].Code != "r01-d-integer-range" {
				t.Fatalf("global range error absent: %+v", bad)
			}
		})
	}
}

func TestModuleFactsSourceAnchorAndNormalCheckerNegatives(t *testing.T) {
	for _, source := range []string{
		"export let count: i32 = 1; export function entry(): i32 { const frozen: i32 = 2; frozen = 3; return count; }",
		"export let count: bool = true; export function entry(): i32 { return count; }",
		"export function entry(): i32 { return 42; } export function unentered(): i32 { return absent; }",
	} {
		r := moduleRequest([]inputFile{{Path: "/src/app.ts", Content: source}}, "/src/app.ts")
		bad := analyze(r)
		if bad.Status != "error" || len(bad.Diagnostics) == 0 || bad.Diagnostics[0].UpstreamCode == 0 {
			t.Fatalf("normal official checking bypassed: %+v", bad)
		}
	}
	source := "/*😀*/ export let count = 1; export function entry(): i32 { return count; }"
	good := analyze(moduleRequest([]inputFile{{Path: "/src/app.ts", Content: source}}, "/src/app.ts"))
	if good.Status != "ok" {
		t.Fatalf("default exact i32 module literal rejected: %+v", good.Diagnostics)
	}
	s := good.Symbols[0]
	if s.Type != "i32" || sourceText(t, source, s.Range) != "count" {
		t.Fatalf("global nominal type/UTF16 anchor lost: %+v", s)
	}
}

func TestModuleFactsBarrelStarNamespaceLiveIdentity(t *testing.T) {
	for _, barrel := range []string{"export {count, increment} from './state';", "export * from './state';"} {
		r := moduleRequest([]inputFile{
			{Path: "/src/app.ts", Content: "import * as state from './barrel'; import './effects'; export function entry(): i32 { state.increment(); return state.count; }"},
			{Path: "/src/barrel.ts", Content: barrel},
			{Path: "/src/state.ts", Content: "export let count: i32 = 2; export function increment(): i32 { count = count + 1; return count; }"},
			{Path: "/src/effects.ts", Content: "export let observed: i32 = 7;"},
		}, "/src/app.ts")
		got := analyze(r)
		if got.Status != "ok" {
			t.Fatalf("static barrel rejected: %+v", got.Diagnostics)
		}
		entry := got.Functions[0]
		if entry.Body[0].Expression.Name != "state.increment" || entry.Body[1].Expression.Name != "state.count" {
			t.Fatalf("namespace spelling lost: %+v", entry)
		}
		var globalID, functionID string
		for _, s := range got.Symbols {
			if s.SourcePath == "/src/state.ts" {
				if s.Name == "count" {
					globalID = s.ID
				}
				if s.Name == "increment" {
					functionID = s.ID
				}
			}
		}
		if entry.Body[0].Expression.Target != functionID || entry.Body[1].Expression.Symbol != globalID {
			t.Fatal("namespace access copied instead of resolving live origin")
		}
		if len(got.Modules[0].Imports) != 2 || len(got.Modules[1].Imports) != 1 || got.Modules[1].Imports[0].Kind != "export" {
			t.Fatal("static initialization dependencies lost")
		}
		var first bytes.Buffer
		if err := json.NewLegacyEncoder(&first).Encode(got); err != nil {
			t.Fatal(err)
		}
		slices.Reverse(r.Files)
		var second bytes.Buffer
		if err := json.NewLegacyEncoder(&second).Encode(analyze(r)); err != nil {
			t.Fatal(err)
		}
		if first.String() != second.String() {
			t.Fatal("namespace facts depend on source enumeration")
		}
	}
}

func TestModuleFactsDefaultAndLocalExportAlias(t *testing.T) {
	for _, provider := range []string{
		"export default function value(): i32 { return 42; }",
		"export default function (): i32 { return 42; }",
	} {
		got := analyze(moduleRequest([]inputFile{{Path: "/src/app.ts", Content: "import read from './value'; export function entry(): i32 { return read(); }"}, {Path: "/src/value.ts", Content: provider}}, "/src/app.ts"))
		if got.Status != "ok" {
			t.Fatalf("default function rejected: %+v", got.Diagnostics)
		}
		if got.Functions[0].Body[0].Expression.Target != got.Functions[1].Symbol || got.Modules[1].Exports[0].Name != "default" {
			t.Fatal("default official function identity lost")
		}
	}
	anonymousArgs := analyze(moduleRequest([]inputFile{{Path: "/src/app.ts", Content: "import helper from './helper'; export function entry(): i32 { return helper(40) + 2; }"}, {Path: "/src/helper.ts", Content: "export default function(value: i32): i32 { return value; }"}}, "/src/app.ts"))
	if anonymousArgs.Status != "ok" || anonymousArgs.Functions[0].Body[0].Expression.Left.Arguments == nil {
		t.Fatalf("anonymous default contextual literal call rejected: %+v", anonymousArgs.Diagnostics)
	}
	got := analyze(moduleRequest([]inputFile{{Path: "/src/app.ts", Content: "import value from './value'; import {renamed} from './alias'; export function entry(): i32 { return value + renamed; }"}, {Path: "/src/value.ts", Content: "export default 40;"}, {Path: "/src/alias.ts", Content: "let original: i32 = 2; export {original as renamed};"}}, "/src/app.ts"))
	if got.Status != "ok" {
		t.Fatalf("default scalar/local alias rejected: %+v", got.Diagnostics)
	}
	var synthesized symbol
	for _, s := range got.Symbols {
		if s.SourcePath == "/src/value.ts" {
			synthesized = s
		}
	}
	if synthesized.Name != "$default" || synthesized.Writable || sourceText(t, "export default 40;", synthesized.Range) != "40" {
		t.Fatal("default value lacks original one-time readonly anchor")
	}
	copyDefault := analyze(moduleRequest([]inputFile{{Path: "/src/app.ts", Content: "import value from './value'; export function entry(): i32 { return value; }"}, {Path: "/src/value.ts", Content: "let original: i32 = 2; export default original; original = 3;"}}, "/src/app.ts"))
	if copyDefault.Status != "ok" {
		t.Fatalf("default expression rejected: %+v", copyDefault.Diagnostics)
	}
	var defaultID string
	for _, s := range copyDefault.Symbols {
		if s.Name == "$default" {
			defaultID = s.ID
		}
	}
	if defaultID == "" || copyDefault.Functions[0].Body[0].Expression.Symbol != defaultID || copyDefault.Modules[1].Exports[0].Target != defaultID {
		t.Fatalf("export default expression incorrectly became live alias: %+v", copyDefault)
	}
}

func TestModuleFactsOfficialAmbiguousAndReadonlyNamespaceErrors(t *testing.T) {
	for _, source := range []string{"import {count} from './barrel'; export function entry(): i32 { return count; }", "import * as state from './a'; export function entry(): i32 { state.count = 7; return state.count; }"} {
		got := analyze(moduleRequest([]inputFile{{Path: "/src/app.ts", Content: source}, {Path: "/src/barrel.ts", Content: "export * from './a'; export * from './b';"}, {Path: "/src/a.ts", Content: "export let count: i32 = 2;"}, {Path: "/src/b.ts", Content: "export let count: i32 = 3;"}}, "/src/app.ts"))
		if got.Status != "error" || len(got.Diagnostics) == 0 || got.Diagnostics[0].UpstreamCode == 0 {
			t.Fatalf("official ambiguous/readonly error bypassed: %+v", got)
		}
	}
	got := analyze(moduleRequest([]inputFile{{Path: "/src/app.ts", Content: "export function entry(): i32 { return 42; }"}, {Path: "/src/unentered.ts", Content: "const obj = {count: 1}; export function invalid(): i32 { return obj.count; }"}}, "/src/app.ts"))
	if got.Status != "error" {
		t.Fatal("ordinary object property was treated as namespace binding")
	}
}

func TestModuleFactsPreserveUnenteredF64CapabilityFacts(t *testing.T) {
	r := integerRequest("")
	r.Files[0].Content = "export function entry(): i32 { return 42; } export function unused(value: f64): f64 { return value; }"
	r.CompilationClosure = &closureConfiguration{Roots: []closureRoot{{Kind: "application", SourcePath: r.RootFile, Name: "entry"}}, RequiredRoots: []closureRoot{}}
	got := analyze(r)
	if got.Status != "ok" || len(got.Functions) != 2 {
		t.Fatalf("unentered f64 fact was capability-rejected in source: %+v", got)
	}
	fn := got.Functions[1]
	if fn.ReturnType != "f64" || fn.Parameters[0].Type != "f64" || fn.Body[0].Expression.Type != "f64" || fn.Body[0].Expression.Symbol != fn.Parameters[0].ID {
		t.Fatal("official f64 signature/reference fact lost")
	}
	start := strings.LastIndex(r.Files[0].Content, " value;")
	if fn.Body[0].Expression.Range != (span{Start: start, End: start + 6}) {
		t.Fatal("f64 original trivia-inclusive range changed")
	}
}

func TestModuleFactsTypeOnlyEdgesAndVarCapability(t *testing.T) {
	for _, source := range []string{"import type {read} from './effects'; export function entry(): i32 { return 42; }", "import {type read} from './effects'; export function entry(): i32 { return 42; }", "export {type read} from './effects'; export function entry(): i32 { return 42; }", "import {type read, count} from './effects'; export function entry(): i32 { return count; }"} {
		got := analyze(moduleRequest([]inputFile{{Path: "/src/app.ts", Content: source}, {Path: "/src/effects.ts", Content: "export let count: i32 = record(7); export function read(): i32 { return count; }"}, {Path: "/src/host.d.ts", Content: "declare function record(value: i32): i32;"}}, "/src/app.ts"))
		// First demonstrate that official checking still sees the supplied effect
		// source: an unconfigured host declaration cannot be silently executed.
		if got.Status != "error" {
			t.Fatal("unconfigured host effect was admitted")
		}
		r := moduleRequest([]inputFile{{Path: "/src/app.ts", Content: source}, {Path: "/src/effects.ts", Content: "export let count: i32 = record(7); export function read(): i32 { return count; }"}, {Path: "/src/host.d.ts", Content: "declare function record(value: i32): i32;"}}, "/src/app.ts")
		r.Declarations.Paths = append(r.Declarations.Paths, "/src/host.d.ts")
		r.HostImports = []hostImport{{SourcePath: "/src/host.d.ts", Name: "record", CapabilityID: 31, ParameterTypes: []string{"i32"}, ReturnType: "i32"}}
		got = analyze(r)
		if got.Status != "ok" {
			t.Fatalf("type edge source rejected: %+v", got.Diagnostics)
		}
		if !strings.Contains(source, "type read, count") {
			if len(got.Modules[0].Imports) != 0 {
				t.Fatal("type-only edge gained initialization dependency")
			}
		} else {
			if len(got.Modules[0].Imports) != 1 || len(got.Modules[0].Imports[0].Bindings) != 1 || got.Modules[0].Imports[0].Bindings[0].Name != "count" {
				t.Fatal("mixed import lost value edge or retained erased type binding")
			}
		}
	}
	bad := analyze(moduleRequest([]inputFile{{Path: "/src/app.ts", Content: "export var count: i32 = 1; export function entry(): i32 { return count; }"}}, "/src/app.ts"))
	if bad.Status != "error" || bad.Diagnostics[0].Code != "r02b-d-unsupported" {
		t.Fatal("var was silently converted to readonly lexical storage")
	}
}

func TestModuleDefaultIntegerRangeBeforeCapability(t *testing.T) {
	for _, literal := range []string{"2147483648", "-2147483649", "0x80000000"} {
		r := integerRequest("")
		r.Files[0].Content = "export let count = " + literal + "; export function entry(): i32 { return count; }"
		r.CompilationClosure = &closureConfiguration{Roots: []closureRoot{{Kind: "application", SourcePath: r.RootFile, Name: "entry"}}, RequiredRoots: []closureRoot{}}
		got := analyze(r)
		if got.Status != "error" || len(got.Diagnostics) != 1 || got.Diagnostics[0].Code != "r01-d-integer-range" {
			t.Fatalf("module default integer lost its range context: %+v", got)
		}
	}
}

func TestModuleLegacyDiagnosticWithoutTargetKeepsUpstream(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Errorf("analyze panicked at the nil-target module diagnostic boundary: %v", recovered)
		}
	}()
	source := "export function entry(): bool { let x = 1 + 2; return 1; }"
	r := moduleRequest([]inputFile{{Path: "/src/app.ts", Content: source}}, "/src/app.ts")
	if r.Target != nil || r.Declarations.Version != "minimal-scalars-v2" {
		t.Fatal("legacy request fixture no longer represents the nil-target contract")
	}
	got := analyze(r)
	if got.Status != "error" || len(got.Diagnostics) == 0 {
		t.Fatalf("legacy module type error was lost: %+v", got)
	}
	found := false
	for _, d := range got.Diagnostics {
		if d.UpstreamCode == 2322 && d.File == r.RootFile &&
			d.Primary.SourceID == r.RootFile && d.Primary.Start == d.Range.Start && d.Primary.End == d.Range.End {
			found = true
		}
	}
	if !found {
		t.Fatalf("normal upstream diagnostic and source anchor were lost: %+v", got.Diagnostics)
	}
}

func TestModuleFloatOverflowRetainsExactLiteralAnchor(t *testing.T) {
	source := "export let count:f32=1e1000; export function entry():f32{return count;}"
	r := floatRequest(source)
	r.CompilationClosure = &closureConfiguration{Roots: []closureRoot{
		{Kind: "application", SourcePath: r.RootFile, Name: "entry"},
	}, RequiredRoots: []closureRoot{}}
	got := analyze(r)
	if got.Status != "error" {
		t.Fatalf("finite overflow accepted: %+v", got)
	}
	start := strings.Index(source, "1e1000")
	want := span{Start: start, End: start + len("1e1000")}
	found := false
	for _, d := range got.Diagnostics {
		if d.Code == "r01-d-float-range" {
			found = true
			if d.Range != want || d.Primary.Start != want.Start || d.Primary.End != want.End ||
				d.Primary.SourceID != r.RootFile || d.File != r.RootFile || d.UpstreamCode != 2322 ||
				d.RuleID != "r01-literals" || d.Phase != "types" {
				t.Fatalf("float literal source anchor or diagnostic identity changed: %+v", d)
			}
		}
	}
	if !found {
		t.Fatalf("owned float range diagnostic missing: %+v", got.Diagnostics)
	}
}
