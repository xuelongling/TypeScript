// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"
	"testing"
)

var integerOracle = []struct {
	kind, minimum, maximum, below, above string
}{
	{"i8", "-128", "127", "-129", "128"},
	{"i16", "-32768", "32767", "-32769", "32768"},
	{"i32", "-2147483648", "2147483647", "-2147483649", "2147483648"},
	{"i64", "-9223372036854775808", "9223372036854775807", "-9223372036854775809", "9223372036854775808"},
	{"u8", "0", "255", "-1", "256"},
	{"u16", "0", "65535", "-1", "65536"},
	{"u32", "0", "4294967295", "-1", "4294967296"},
	{"u64", "0", "18446744073709551615", "-1", "18446744073709551616"},
	{"isize", "-9223372036854775808", "9223372036854775807", "-9223372036854775809", "9223372036854775808"},
	{"usize", "0", "18446744073709551615", "-1", "18446744073709551616"},
}

func integerRequest(source string) request {
	r := scalarRequest(source)
	declarations := scalarDeclarations
	for _, tc := range integerOracle {
		if tc.kind != "i32" {
			declarations += fmt.Sprintf("declare const __tsfg_%s: unique symbol;\ntype %s = number & { readonly [__tsfg_%s]: true };\n", tc.kind, tc.kind, tc.kind)
		}
	}
	r.Files[1] = inputFile{Path: "/src/minimal-integers-v3.d.ts", Content: declarations}
	r.Declarations.Version = "minimal-integers-v3"
	r.Declarations.Paths = []string{r.Files[1].Path}
	r.Target = &integerTarget{Target: "windows-x86_64-msvc", Triple: "x86_64-pc-windows-msvc",
		LLVMDataLayout: "e-m:w-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128", PointerBits: "64"}
	return r
}

func TestIntegerFamilyExactMathematicalOracle(t *testing.T) {
	for _, tc := range integerOracle {
		t.Run(tc.kind, func(t *testing.T) {
			for _, value := range []string{tc.minimum, tc.maximum} {
				source := fmt.Sprintf("export function entry(): %s { return %s; }", tc.kind, value)
				result := analyze(integerRequest(source))
				if result.Status != "ok" || len(result.Functions) != 1 {
					t.Fatalf("mathematical boundary %s rejected: %+v", value, result)
				}
				e := result.Functions[0].Body[0].Expression
				if e.Type != tc.kind || e.Kind != "integer" || e.Literal == nil || e.Literal.Decimal != value || e.Literal.Token != value || sourceText(t, source, e.Range) != value {
					t.Fatalf("boundary rounded or mistyped: %+v", e)
				}
			}
			for _, value := range []string{tc.below, tc.above} {
				source := fmt.Sprintf("export function entry(): %s { return %s; }", tc.kind, value)
				result := analyze(integerRequest(source))
				if result.Status != "error" || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "r01-d-integer-range" || result.Diagnostics[0].UpstreamCode != 2322 {
					t.Fatalf("out-of-range %s not diagnosed precisely: %+v", value, result)
				}
				if sourceText(t, source, result.Diagnostics[0].Range) != "return" {
					t.Fatalf("official return diagnostic span altered: %+v", result.Diagnostics[0])
				}
			}
		})
	}
}

func TestIntegerFamilyContextsAndOperators(t *testing.T) {
	for _, tc := range integerOracle {
		t.Run(tc.kind, func(t *testing.T) {
			for _, operator := range []string{"+", "-", "*", "/", "%", "&", "|", "^", "<<", ">>", ">>>"} {
				countType := tc.kind
				if operator == "<<" || operator == ">>" || operator == ">>>" {
					countType = "i32"
				}
				source := fmt.Sprintf("export function entry(a: %s, b: %s): %s { const first: %s = a %s 1; let value: %s = identity(first %s 1); value = value %s 1; return value %s b; } function identity(n: %s): %s { return n; }", tc.kind, countType, tc.kind, tc.kind, operator, tc.kind, operator, operator, operator, tc.kind, tc.kind)
				result := analyze(integerRequest(source))
				if result.Status != "ok" || len(result.Functions) != 2 {
					t.Fatalf("operator %s context rejected: %+v", operator, result)
				}
				body := result.Functions[0].Body
				if body[0].Initializer.Type != tc.kind || body[1].Initializer.Type != tc.kind || body[1].Initializer.Kind != "call" || body[2].Value.Type != tc.kind || body[3].Expression.Type != tc.kind || body[3].Expression.Right.Type != countType {
					t.Fatalf("operator %s lost integer kind: %+v", operator, body)
				}
			}
			for _, operator := range []string{"<", "<=", ">", ">=", "===", "!=="} {
				source := fmt.Sprintf("export function entry(a: %s): bool { return 1 %s a; }", tc.kind, operator)
				result := analyze(integerRequest(source))
				if result.Status != "ok" || result.Functions[0].Body[0].Expression.Left.Type != tc.kind || result.Functions[0].Body[0].Expression.Type != "bool" {
					t.Fatalf("comparison %s context rejected: %+v", operator, result)
				}
			}
			for _, operator := range []string{"+", "-", "~"} {
				source := fmt.Sprintf("export function entry(a: %s): %s { return %sa; }", tc.kind, tc.kind, operator)
				result := analyze(integerRequest(source))
				if result.Status != "ok" || result.Functions[0].Body[0].Expression.Type != tc.kind {
					t.Fatalf("unary %s context rejected: %+v", operator, result)
				}
			}
		})
	}
}

func TestIntegerFamilyBoundaryContextMatrix(t *testing.T) {
	for _, tc := range integerOracle {
		t.Run(tc.kind, func(t *testing.T) {
			for _, value := range []string{tc.minimum, tc.maximum, tc.below, tc.above} {
				for _, body := range []string{
					"const v: %s = %s; return v;",
					"let v: %s = 0; v = %s; return v;",
					"return identity(%s);",
					"const v: %s = identity(%s); return v;",
					"return a + %s;",
				} {
					var rendered string
					if strings.Count(body, "%s") == 2 {
						rendered = fmt.Sprintf(body, tc.kind, value)
					} else {
						rendered = fmt.Sprintf(body, value)
					}
					source := fmt.Sprintf("export function entry(a: %s): %s { %s } function identity(n: %s): %s { return n; }", tc.kind, tc.kind, rendered, tc.kind, tc.kind)
					result := analyze(integerRequest(source))
					valid := value == tc.minimum || value == tc.maximum
					if valid != (result.Status == "ok") {
						t.Fatalf("context %s, value %s result: %+v", body, value, result)
					}
					if !valid && result.Diagnostics[0].Code != "r01-d-integer-range" {
						t.Fatalf("context range identity lost: %+v", result)
					}
				}
			}
		})
	}
}

func TestIntegerFamilyNoImplicitConversion(t *testing.T) {
	for _, tc := range integerOracle {
		t.Run(tc.kind, func(t *testing.T) {
			other := "i32"
			if tc.kind == "i32" {
				other = "u32"
			}
			for _, body := range []string{
				"return a + b;", "return identity(b);", "const v: " + tc.kind + " = b; return v;",
				"let v: " + tc.kind + " = a; v = b; return v;", "return b;",
			} {
				source := fmt.Sprintf("export function entry(a: %s, b: %s): %s { %s } function identity(n: %s): %s { return n; }", tc.kind, other, tc.kind, body, tc.kind, tc.kind)
				result := analyze(integerRequest(source))
				if result.Status != "error" || result.Diagnostics[0].Code != "r01-d-scalar-type" {
					t.Fatalf("implicit conversion admitted: %+v", result)
				}
			}
			if tc.kind != "i32" {
				for _, body := range []string{"const v = 1; return v;", "const v = 1; const copy = v; return copy;", "const v = 1; return identity(v);"} {
					source := fmt.Sprintf("export function entry(): %s { %s } function identity(n: %s): %s { return n; }", tc.kind, body, tc.kind, tc.kind)
					result := analyze(integerRequest(source))
					if result.Status != "error" || result.Diagnostics[0].Code != "r01-d-scalar-type" {
						t.Fatalf("default i32 implicitly converted: %+v", result)
					}
				}
			}
		})
	}
	result := analyze(integerRequest("export function entry(): i32 { const v = 1; const copy = v; return copy; }"))
	if result.Status != "ok" || result.Functions[0].Body[0].Symbol.Type != "i32" || result.Functions[0].Body[1].Symbol.Type != "i32" {
		t.Fatalf("default i32 fact lost: %+v", result)
	}
}

func TestIntegerFamilyShiftCountsAndMIN(t *testing.T) {
	for _, tc := range integerOracle {
		t.Run(tc.kind, func(t *testing.T) {
			for _, count := range []string{"1", "-1", "32", "64"} {
				result := analyze(integerRequest(fmt.Sprintf("export function entry(a: %s): %s { return a << %s; }", tc.kind, tc.kind, count)))
				if result.Status != "ok" || result.Functions[0].Body[0].Expression.Right.Type != "i32" {
					t.Fatalf("shift count not i32: %+v", result)
				}
			}
			result := analyze(integerRequest(fmt.Sprintf("export function entry(a: %s, count: u8): %s { return a << count; }", tc.kind, tc.kind)))
			if result.Status != "error" || result.Diagnostics[0].Code != "r01-d-shift-count-type" || result.Diagnostics[0].RuleID != "r01-shifts" {
				t.Fatalf("non-i32 count admitted: %+v", result)
			}
			if strings.HasPrefix(tc.kind, "i") {
				result := analyze(integerRequest(fmt.Sprintf("export function entry(): %s { const min: %s = %s; return -min; }", tc.kind, tc.kind, tc.minimum)))
				if result.Status != "ok" || result.Functions[0].Body[1].Expression.Kind != "unary" || result.Functions[0].Body[1].Expression.Type != tc.kind {
					t.Fatalf("MIN negation incorrectly range folded: %+v", result)
				}
			}
		})
	}
}

func TestIntegerFamilyIndependentOfficialDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		body, text, code string
		upstream         int32
	}{
		{"const v: u8 = identity(a + 1); const bad: bool = 42; return v;", "bad", "r01-d-scalar-type", 2322},
		{"const v: u8 = identity(a + 1); const bad: u8 = missing; return v;", "missing", "r01-d-unresolved-name", 2304},
		{"const v: u8 = identity(a + 1); v = a; return v;", "v", "r02b-d-type", 2588},
		{"return identity(a + true);", "a + true", "r02b-d-type", 2365},
		{"return identity(a, a);", "a", "r02b-d-type", 2554},
	} {
		t.Run(tc.code+tc.text, func(t *testing.T) {
			source := "/*😀*/ export function entry(a: u8): u8 { " + tc.body + " } function identity(a: u8): u8 { return a; }"
			result := analyze(integerRequest(source))
			found := false
			for _, d := range result.Diagnostics {
				if d.Code == tc.code && d.UpstreamCode == tc.upstream && sourceText(t, source, d.Range) == tc.text && d.Primary.Start == d.Range.Start && d.Primary.End == d.Range.End && d.Primary.SourceID == "/src/main.ts" {
					found = true
				}
			}
			if result.Status != "error" || !found {
				t.Fatalf("official identity/span lost: %+v", result)
			}
		})
	}
}

func TestIntegerFamilyTargetAdmissionAndUnsupported(t *testing.T) {
	for _, mutation := range []func(*request){
		func(r *request) { r.Target = nil }, func(r *request) { r.Target.PointerBits = "32" },
		func(r *request) { r.Target.LLVMDataLayout += "-p:32:32" }, func(r *request) { r.Target.Triple = "i386-pc-windows-msvc" },
		func(r *request) { r.Target.Target = "host" }, func(r *request) { r.Declarations.Version = "minimal-integers-v4" },
	} {
		r := integerRequest("export function entry(): usize { return 18446744073709551615; }")
		mutation(&r)
		result := analyze(r)
		if result.Status != "error" || result.Diagnostics[0].Code != "r02b-d-input" || result.Diagnostics[0].Category != "input" {
			t.Fatalf("wrong target/version admitted: %+v", result)
		}
	}
	for _, source := range []string{
		"export function entry(): f64 { return 1.5; }",
		"export function entry(a: f64): f64 { return a + a; }",
		"export function entry(a: u8): u8 { return a ** a; }",
		"export function entry(): bool { return !1; }",
		"export function entry(a: u64): bool { return !a; }",
		"export function entry(a: u64): bool { return a && a; }",
	} {
		result := analyze(integerRequest(source))
		if result.Status != "error" || result.Diagnostics[0].Category != "unsupported" || result.Diagnostics[0].Phase != "capability" {
			t.Fatalf("unsupported capability misclassified: %+v", result)
		}
	}
	result := analyze(integerRequest("export function entry(a: bool): bool { return !a || (a && true); }"))
	if result.Status != "ok" {
		t.Fatalf("boolean short circuit rejected: %+v", result)
	}
	result = analyze(integerRequest("export function entry(a: u64): bool { return (a < 1) && ((a > 0) === true); }"))
	if result.Status != "ok" {
		t.Fatalf("nested boolean integer comparison rejected: %+v", result)
	}
	for _, source := range []string{
		"export function entry(): i32 { return 0; } function unused(): u8 { return 256; }",
		"export function entry(): i32 { return 0; } function unused(): u64 { const v = 18446744073709551615; return v; }",
		"export function entry(): i32 { return 0; } function unused(): u8 { return 1.5; }",
	} {
		result := analyze(integerRequest(source))
		if result.Status != "error" || result.Diagnostics[0].Category == "unsupported" {
			t.Fatalf("unentered type failure not rejected: %+v", result)
		}
	}
}

func TestIntegerFamilyTokenPrecisionAndNestedCalls(t *testing.T) {
	source := "export function entry(a: u64): u64 { { const v: u64 = identity(identity(a + 0xffff_ffff_ffff_ffff)); return v; } } function identity(a: u64): u64 { return a; }"
	result := analyze(integerRequest(source))
	if result.Status != "ok" {
		t.Fatalf("nested call context rejected: %+v", result)
	}
	e := (*(*result.Functions[0].Body[0].Body[0].Initializer.Arguments)[0].Arguments)[0].Right
	if e.Type != "u64" || e.Literal.Decimal != "18446744073709551615" || e.Literal.Token != "0xffff_ffff_ffff_ffff" || strings.TrimSpace(sourceText(t, source, e.Range)) != e.Literal.Token {
		t.Fatalf("token precision lost: %+v", e)
	}
	for _, value := range []string{"1.0", "1e0", "1.5"} {
		result := analyze(integerRequest("export function entry(): u64 { return " + value + "; }"))
		if result.Status != "error" || result.Diagnostics[0].Code != "r01-d-scalar-type" {
			t.Fatalf("fraction/exponent token admitted: %+v", result)
		}
	}
}
