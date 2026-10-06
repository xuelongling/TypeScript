// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestIntegerFamilyPreservesOfficialF64SignatureFacts(t *testing.T) {
	for _, source := range []string{
		"export function answer(value: f64): i32 { return 42; }",
		"export function answer(): i32 { return 42; } export function second(value: f64): f64 { return value; }",
	} {
		t.Run(source, func(t *testing.T) {
			baseline := analyze(scalarRequest(source))
			if baseline.Status != "ok" || len(baseline.Diagnostics) != 0 {
				t.Fatalf("existing officially checked scalar facts rejected: %+v", baseline)
			}
			current := analyze(integerRequest(source))
			if current.Status != "ok" || len(current.Diagnostics) != 0 {
				t.Fatalf("integer declarations changed the source-to-HIR capability seam: %+v", current)
			}
			// These are already checked signatures and direct references. The
			// owned HIR consumer still rejects their unsupported f64 signature;
			// the producer must not replace that later capability decision.
			if len(current.Functions) != len(baseline.Functions) || !reflect.DeepEqual(current.Symbols, baseline.Symbols) {
				t.Fatalf("official function/symbol facts changed: current=%+v baseline=%+v", current, baseline)
			}
			for i, fn := range current.Functions {
				old := baseline.Functions[i]
				if fn.Name != old.Name || fn.Symbol != old.Symbol || fn.ReturnType != old.ReturnType || fn.Range != old.Range || !reflect.DeepEqual(fn.Parameters, old.Parameters) || len(fn.Body) != 1 || len(old.Body) != 1 || fn.Body[0].Kind != old.Body[0].Kind || fn.Body[0].Range != old.Body[0].Range {
					t.Fatalf("official signature/statement anchors changed: current=%+v baseline=%+v", fn, old)
				}
				expr, prior := fn.Body[0].Expression, old.Body[0].Expression
				if expr == nil || prior == nil || !reflect.DeepEqual(expr, prior) {
					t.Fatalf("official literal/reference facts changed: current=%+v baseline=%+v", expr, prior)
				}
			}
		})
	}
}

func TestIntegerFamilyOwnedLiteralNodeRanges(t *testing.T) {
	for _, source := range []string{
		"export function answer(): i32 { return 42; }",
		"export function answer(): u64 { return /*😀*/ 0xffff_ffff_ffff_ffff; }",
	} {
		result := analyze(integerRequest(source))
		if result.Status != "ok" {
			t.Fatalf("checked literal facts rejected: %+v", result)
		}
		expr := result.Functions[0].Body[0].Expression
		start := strings.Index(source, "return") + len("return")
		end := len(utf16.Encode([]rune(source[:strings.Index(source, ";")])))
		if expr.Range != (span{Start: start, End: end}) || expr.Literal == nil || strings.Contains(expr.Literal.Token, "/*") || strings.HasPrefix(expr.Literal.Token, " ") {
			t.Fatalf("owned node.Pos span and exact lexical token were conflated: %+v", expr)
		}
	}
}

func TestIntegerFamilyPreservesLegacyExpressionRangeContexts(t *testing.T) {
	for _, source := range []string{
		"export function entry(): i32 { return 42; }",
		"export function entry(a: i32): i32 { return a; }",
		"export function entry(a: i32, b: i32): i32 { return a / b; }",
		"export function entry(a: i32): i32 { return -a; }",
		"export function entry(a: i32): i32 { return (a); }",
		"export function entry(): i32 { return -2147483648; }",
		"export function entry(a: i32): i32 { return identity(a); } function identity(a: i32): i32 { return a; }",
		"export function entry(a: i32): i32 { return identity(a) / 1; } function identity(a: i32): i32 { return a; }",
		"export function entry(a: i32): i32 { return identity(a / 1); } function identity(a: i32): i32 { return a; }",
		"export function entry(a: i32, b: i32): i32 { return -((a / b) + 1); }",
	} {
		t.Run(source, func(t *testing.T) {
			prior, current := analyze(scalarRequest(source)), analyze(integerRequest(source))
			if prior.Status != "ok" || current.Status != "ok" || !reflect.DeepEqual(prior.Functions, current.Functions) {
				t.Fatalf("owned expression range context changed: prior=%+v current=%+v", prior, current)
			}
		})
	}
}

func TestIntegerFamilyF64FactsPreserveOfficialNameError(t *testing.T) {
	source := "export function answer(): i32 { return 42; } export function second(value: f64): f64 { return missing; }"
	result := analyze(integerRequest(source))
	if result.Status != "error" || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "r01-d-unresolved-name" || result.Diagnostics[0].UpstreamCode != 2304 || sourceText(t, source, result.Diagnostics[0].Range) != "missing" {
		t.Fatalf("signature capability hid or changed the official checker error: %+v", result)
	}
}
