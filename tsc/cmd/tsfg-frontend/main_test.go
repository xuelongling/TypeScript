// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
	"unicode/utf16"
)

const scalarDeclarations = `interface Object {}
interface Function {}
interface CallableFunction {}
interface NewableFunction {}
interface IArguments {}
interface String {}
interface Number {}
interface Boolean {}
interface RegExp {}
interface Array<T> {}
type bool = boolean;
declare const __tsfg_i32: unique symbol;
type i32 = number & { readonly [__tsfg_i32]: true };
type f64 = number;
`

func scalarRequest(source string) request {
	r := request{SourceRoot: "/src", RootFile: "/src/main.ts", Files: []inputFile{
		{Path: "/src/main.ts", Content: source},
		{Path: "/src/minimal-scalars-v2.d.ts", Content: scalarDeclarations},
	}}
	r.Declarations.Version = "minimal-scalars-v2"
	r.Declarations.Paths = []string{"/src/minimal-scalars-v2.d.ts"}
	r.Config.Strict, r.Config.NoEmit = true, true
	return r
}

func TestAnalyzeI32LocalInitializerOriginalFailures(t *testing.T) {
	for _, body := range []string{
		"const seed: i32 = 20 + 1; const copy: i32 = seed; return seed + copy;",
		"const a: i32 = 6; const b: i32 = 7; let value: i32 = a * b; value = value + 0; return value;",
		"const unused: i32 = 1 / 0; let value: i32 = 1 % 0; value = 42; return value;",
		"let value: i32 = 1 / 0; value = 42; return value;",
	} {
		t.Run(body, func(t *testing.T) {
			result := analyze(scalarRequest("export function answer(): i32 { " + body + " }"))
			if result.Status != "ok" {
				t.Fatalf("checked i32 initializer rejected: %+v", result.Diagnostics)
			}
			if len(result.Functions) != 1 || len(result.Functions[0].Body) == 0 {
				t.Fatalf("missing owned function body: %+v", result)
			}
		})
	}
}

func sourceText(t *testing.T, source string, location span) string {
	t.Helper()
	units := utf16.Encode([]rune(source))
	if location.Start < 0 || location.End <= location.Start || location.End > len(units) {
		t.Fatalf("invalid source range: %+v", location)
	}
	return strings.TrimSpace(string(utf16.Decode(units[location.Start:location.End])))
}

func TestAnalyzeI32LocalInitializerOwnedFacts(t *testing.T) {
	source := "/*😀*/ export function answer(seed: i32): i32 { const a: i32 = (seed + 1), b: i32 = -a; let c: i32 = ~b, d: i32 = c * 2; return d; }"
	result := analyze(scalarRequest(source))
	if result.Status != "ok" || len(result.Functions) != 1 {
		t.Fatalf("initializer forms rejected: %+v", result)
	}
	fn := result.Functions[0]
	if len(fn.Parameters) != 1 || fn.Parameters[0].Type != "i32" || len(fn.Body) != 5 {
		t.Fatalf("missing owned parameter or statements: %+v", fn)
	}
	symbols := map[string]symbol{}
	for _, s := range result.Symbols {
		symbols[s.ID] = s
	}
	for i, name := range []string{"a", "b", "c", "d"} {
		local := fn.Body[i]
		if local.Kind != "variable" || local.Symbol == nil || local.Initializer == nil {
			t.Fatalf("missing owned initializer: %+v", local)
		}
		s := *local.Symbol
		if s.Name != name || s.Kind != "local" || s.Type != "i32" || s.Writable != (i >= 2) || symbols[s.ID] != s || sourceText(t, source, s.Range) != name {
			t.Fatalf("incorrect local symbol facts: %+v", s)
		}
		if !strings.HasPrefix(sourceText(t, source, local.Range), name+": i32 = ") {
			t.Fatalf("incorrect declaration source: %+v", local.Range)
		}
	}
	type expectedExpression struct {
		kind, operator, text, symbolName, decimal string
		left, right, operand                      *expectedExpression
	}
	ref := func(name string) *expectedExpression {
		return &expectedExpression{kind: "reference", text: name, symbolName: name}
	}
	literal := func(text string) *expectedExpression {
		return &expectedExpression{kind: "integer", text: text, decimal: text}
	}
	want := []*expectedExpression{
		{kind: "group", text: "(seed + 1)", operand: &expectedExpression{kind: "binary", operator: "+", text: "seed + 1", left: ref("seed"), right: literal("1")}},
		{kind: "unary", operator: "-", text: "-a", operand: ref("a")},
		{kind: "unary", operator: "~", text: "~b", operand: ref("b")},
		{kind: "binary", operator: "*", text: "c * 2", left: ref("c"), right: literal("2")},
	}
	var checkExpression func(*expression, *expectedExpression)
	checkExpression = func(got *expression, expected *expectedExpression) {
		t.Helper()
		if got == nil || expected == nil {
			if got != nil || expected != nil {
				t.Fatalf("incorrect operand presence: %+v / %+v", got, expected)
			}
			return
		}
		if got.Kind != expected.kind || got.Type != "i32" || got.Operator != expected.operator || sourceText(t, source, got.Range) != expected.text {
			t.Fatalf("incorrect owned expression: %+v / %+v", got, expected)
		}
		if expected.symbolName != "" && (got.Symbol == "" || symbols[got.Symbol].Name != expected.symbolName) {
			t.Fatalf("incorrect operand symbol: %+v", got)
		}
		if expected.decimal != "" && (got.Literal == nil || got.Literal.Decimal != expected.decimal || got.Literal.Token != expected.text) {
			t.Fatalf("incorrect integer operand: %+v", got)
		}
		checkExpression(got.Left, expected.left)
		checkExpression(got.Right, expected.right)
		checkExpression(got.Operand, expected.operand)
	}
	for i := range want {
		checkExpression(fn.Body[i].Initializer, want[i])
	}
	checkExpression(fn.Body[4].Expression, ref("d"))
}

func TestAnalyzeI32LocalInitializerRejectsInvalidSources(t *testing.T) {
	for _, tc := range []struct {
		name, body, code, rule, phase, primary string
		upstream                               int32
	}{
		{"mixed f64 and i32", "const a: f64 = 1.5; const b: i32 = 2; const v: i32 = a + b; return v;", "r01-d-scalar-type", "r01-literals", "types", "v", 2322},
		{"bool mismatch", "const v: i32 = true; return v;", "r01-d-scalar-type", "r01-literals", "types", "v", 2322},
		{"out of range", "const v: i32 = 2147483648; return v;", "r01-d-integer-range", "r01-literals", "types", "v", 2322},
		{"const write", "const v: i32 = 42; v = 7; return v;", "r02b-d-type", "r01-ts-baseline", "types", "v", 2588},
		{"unresolved name", "const v: i32 = missingName + 1; return v;", "r01-d-unresolved-name", "r01-ts-baseline", "types", "missingName", 2304},
		{"malformed source", "const v: i32 = 1 + ; return v;", "r02b-d-syntax", "r01-ts-baseline", "syntax", ";", 1109},
		{"comma sibling mismatch", "const a: i32 = 20 + 1, bad: i32 = true; return a;", "r01-d-scalar-type", "r01-literals", "types", "bad", 2322},
		{"unrelated d2322", "const a: i32 = 20 + 1; const bad: bool = 42; return a;", "r01-d-scalar-type", "r01-literals", "types", "bad", 2322},
		{"foreign symbol", "const v: i32 = __tsfg_i32; return v;", "r02b-d-type", "r01-ts-baseline", "types", "v", 2322},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "export function answer(): i32 { " + tc.body + " }"
			result := analyze(scalarRequest(source))
			if result.Status != "error" || len(result.Diagnostics) != 1 || len(result.Functions) != 0 {
				t.Fatalf("invalid source admitted or incorrect diagnostics: %+v", result)
			}
			d := result.Diagnostics[0]
			if d.Code != tc.code || d.RuleID != tc.rule || len(d.RuleIDs) != 1 || d.RuleIDs[0] != tc.rule || d.Phase != tc.phase || d.Category != "error" || d.Severity != "error" || d.UpstreamCode != tc.upstream || d.File != "/src/main.ts" || d.Primary.SourceID != d.File || d.Primary.Start != d.Range.Start || d.Primary.End != d.Range.End || sourceText(t, source, d.Range) != tc.primary {
				t.Fatalf("diagnostic identity changed: %+v", d)
			}
		})
	}
}

func TestAnalyzeI32LocalInitializerPreservesUnevaluatedTraps(t *testing.T) {
	source := "export function answer(): i32 { const unused: i32 = 1 / 0; let value: i32 = 1 % 0; value = 42; return value; }"
	result := analyze(scalarRequest(source))
	if result.Status != "ok" || len(result.Functions) != 1 || len(result.Functions[0].Body) != 4 {
		t.Fatalf("trap-bearing initializers rejected: %+v", result)
	}
	body := result.Functions[0].Body
	for i, operator := range []string{"/", "%"} {
		e := body[i].Initializer
		if body[i].Kind != "variable" || e == nil || e.Kind != "binary" || e.Type != "i32" || e.Operator != operator || e.Left == nil || e.Right == nil || e.Left.Literal == nil || e.Right.Literal == nil || e.Left.Literal.Decimal != "1" || e.Right.Literal.Decimal != "0" || sourceText(t, source, e.Range) != "1 "+operator+" 0" {
			t.Fatalf("initializer was folded or reordered: %+v", body[i])
		}
	}
	if body[2].Kind != "assignment" || body[3].Kind != "return" || body[2].Target == nil || body[3].Expression == nil || body[2].Target.Symbol != body[1].Symbol.ID || body[3].Expression.Symbol != body[1].Symbol.ID {
		t.Fatalf("later statements lost their original order or binding: %+v", body)
	}
}

func TestAnalyzeI32LocalInitializerKeepsPendingFormsRejected(t *testing.T) {
	for _, body := range []string{
		"let v: i32; v = 42; return v;",
		"let v: i32 = 1; const copy: i32 = v++; return copy;",
		"let v: i32 = 1; const copy: i32 = (v = 2); return copy;",
		"let v: i32 = 1; const copy: i32 = (v += 2); return copy;",
	} {
		t.Run(body, func(t *testing.T) {
			result := analyze(scalarRequest("export function answer(): i32 { " + body + " }"))
			if result.Status != "error" || len(result.Diagnostics) == 0 || len(result.Functions) != 0 {
				t.Fatalf("pending initializer form admitted: %+v", result)
			}
		})
	}
}
