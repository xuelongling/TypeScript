// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"
)

func floatRequest(source string) request {
	r := integerRequest(source)
	r.Files[1].Content += "declare const __tsfg_f32: unique symbol; type f32 = number & { readonly [__tsfg_f32]: true };\n"
	r.Files[1].Path = "/src/minimal-floats-v4.d.ts"
	r.Declarations.Version = "minimal-floats-v4"
	r.Declarations.Paths = []string{r.Files[1].Path}
	return r
}

func TestFloatNestedDefaultContexts(t *testing.T) {
	t.Parallel()
	for index, row := range []struct{ source, kind string }{
		{"function entry():f64{const a=0.1+0.2;return a;}", "f64"},
		{"function entry():f64{const a=(0.1)+(0.2);return a;}", "f64"},
		{"function entry():f64{const a=-(0.1);return a;}", "f64"},
		{"function entry():f64{const a=+((1e0));return a;}", "f64"},
		{"function entry():f64{const a=((1e0))+((2e0));return a;}", "f64"},
		{"function entry():f64{const a=-((1e0)+(2.0));return a;}", "f64"},
		{"function entry():f64{const a=((1)+(2))+((3e0));return a;}", "f64"},
		{"function entry():bool{const a=(0.1+0.2)===(0.3+0.0);return a;}", "bool"},
		{"function entry():bool{const a=((1e0)<(2e0))===((2.0)>(1.0));return a;}", "bool"},
		{"function entry(a:f32):f32{const b=(1.0)+(a);return b;}", "f32"},
		{"function entry(a:f32):f32{const b=-((1.0)+(a));return b;}", "f32"},
		{"function entry():f32{const a:f32=((1.0)+(2.0));return a;}", "f32"},
	} {
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			t.Parallel()
			result := analyze(floatRequest(row.source))
			if result.Status != "ok" {
				t.Fatalf("nested source rejected: %+v", result)
			}
			local := result.Functions[0].Body[0]
			if local.Symbol == nil || local.Symbol.Type != row.kind || local.Initializer == nil || local.Initializer.Type != row.kind {
				t.Fatalf("default/context type: %+v", local)
			}
		})
	}
}

func TestFloatNestedContextPrecedenceAndFirstFailure(t *testing.T) {
	t.Parallel()
	for index, row := range []struct {
		source, code, anchor string
		offset, length       int
	}{
		{"function entry():i32{const a:i32=(0.1)+(0.2);return a;}", "r01-d-scalar-type", "const a", 6, 1},
		{"function entry(a:i32):i32{const b=(0.1)+(a);return b;}", "r02b-d-type", "return", 0, 6},
		{"function entry(a:f32,b:f64):f32{const c=((a))+((b));return c;}", "r02b-d-type", "return", 0, 6},
		{"function entry():f32{const a=(1.0)+(2.0);return(a);}", "r01-d-scalar-type", "return", 0, 6},
		{"function entry(a:f32):f64{const b=(1e1000)+(a);return(b);}", "r01-d-float-range", "1e1000", 0, 6},
		{"function entry(a:f32,b:f64):f32{return((b)+(1e1000));}", "r01-d-scalar-type", "return", 0, 6},
		{"/*😀*/function entry():i32{const a:i32=(0.1)+(1e1000);return(a);}", "r01-d-scalar-type", "const a", 6, 1},
	} {
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			t.Parallel()
			result := analyze(floatRequest(row.source))
			if result.Status != "error" || len(result.Diagnostics) == 0 {
				t.Fatalf("typed conversion or invalid source accepted: %+v", result)
			}
			diagnostic := result.Diagnostics[0]
			phase, rule := "types", "r01-literals"
			if row.code == "r02b-d-type" {
				phase, rule = "syntax", "r01-ts-baseline"
			}
			if diagnostic.Code != row.code || diagnostic.Phase != phase || diagnostic.RuleID != rule || diagnostic.Category != "error" || diagnostic.Severity != "error" {
				t.Fatalf("first diagnostic identity: %+v", diagnostic)
			}
			// Preserve official declaration/return diagnostic spans. Exact
			// finite overflow alone retains its narrower owned literal span.
			position := strings.Index(row.source, row.anchor)
			start := len(utf16.Encode([]rune(row.source[:position]))) + row.offset
			end := start + row.length
			if diagnostic.Primary.Start != start || diagnostic.Primary.End != end {
				t.Fatalf("first diagnostic UTF16 anchor: %+v, expected %d..%d", diagnostic, start, end)
			}
		})
	}
}

func TestFloatDirectRounding(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ kind, token, bits string }{
		{"f32", "1.000000059604644775390625", "3f800000"},
		{"f32", "1.000000059604644775390625000000000001", "3f800001"},
		{"f32", "1.401298464324817070923729583289916131280e-45", "00000001"},
		{"f32", "-0.0", "80000000"},
		{"f32", "-1e-10000000000000000000000", "80000000"},
		{"f64", "9007199254740993", "4340000000000000"},
		{"f64", "-0.0", "8000000000000000"},
		{"f64", "5e-324", "0000000000000001"},
	} {
		t.Run(row.kind+row.token, func(t *testing.T) {
			t.Parallel()
			source := fmt.Sprintf("export function entry(): %s { return %s; }", row.kind, row.token)
			result := analyze(floatRequest(source))
			if result.Status != "ok" {
				t.Fatalf("literal rejected: %+v", result)
			}
			literal := result.Functions[0].Body[0].Expression
			if literal.Kind != "floating" || literal.Type != row.kind || literal.Float == nil || literal.Float.Token != row.token || literal.Float.Bits != row.bits {
				t.Fatalf("exact direct bits: %+v", literal)
			}
		})
	}
}

func TestFloatNominalContexts(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"f32", "f64"} {
		for _, op := range []string{"+", "-", "*", "/", "%", "<", "<=", ">", ">=", "===", "!=="} {
			ret := kind
			if strings.ContainsAny(op, "<>=!") {
				ret = "bool"
			}
			source := fmt.Sprintf("export function entry(a: %s, b: %s): %s { return a %s b; }", kind, kind, ret, op)
			if result := analyze(floatRequest(source)); result.Status != "ok" {
				t.Fatalf("%s %s: %+v", kind, op, result)
			}
		}
		for _, token := range []string{"1e1000000000000000000000", "1e400"} {
			result := analyze(floatRequest(fmt.Sprintf("export function entry(): %s { return %s; }", kind, token)))
			if result.Status != "error" || result.Diagnostics[0].Code != "r01-d-float-range" {
				t.Fatalf("finite overflow: %+v", result)
			}
		}
	}
	for _, source := range []string{
		"export function entry(a: f32, b: f64): f32 { return a + b; }",
		"export function entry(a: f32): f32 { return ~a; }",
		"export function entry(a: f32): f32 { return a << 1; }",
	} {
		if result := analyze(floatRequest(source)); result.Status != "error" {
			t.Fatalf("strict nominal operator accepted: %+v", result)
		}
	}
}

func TestFloatOverflowAnchors(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		"function entry(): f32 { return (1e1000); }",
		"function entry(): f32 { const value: f32 = 1e1000; return value; }",
		"function entry(): f32 { let value: f32 = 0.0; value = 1e1000; return value; }",
		"function entry(): f32 { return consume(1e1000); } function consume(a: f32): f32 { return a; }",
	} {
		result := analyze(floatRequest(source))
		if result.Status != "error" || len(result.Diagnostics) == 0 {
			t.Fatalf("overflow accepted: %+v", result)
		}
		diagnostic := result.Diagnostics[0]
		if diagnostic.Code != "r01-d-float-range" || diagnostic.RuleID != "r01-literals" || diagnostic.Phase != "types" {
			t.Fatalf("overflow identity: %+v", diagnostic)
		}
		if source[diagnostic.Primary.Start:diagnostic.Primary.End] != "1e1000" {
			t.Fatalf("overflow literal anchor: %+v", diagnostic)
		}
	}
}
