// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

func TestDefaultIntegerContextRange(t *testing.T) {
	for name, source := range map[string]string{
		"bare":       "export function entry(): i32 { let a = 2147483648; return a; }",
		"group":      "export function entry(): i32 { let a = (2147483648); return a; }",
		"unary":      "export function entry(): i32 { let a = -2147483649; return a; }",
		"arithmetic": "export function entry(): i32 { let a = 2147483648 + 0; return a; }",
		"alias":      "export function entry(): i32 { let a = 2147483648; let b = a; return b; }",
		"call":       "export function entry(): i32 { let a = 2147483648; return identity(a); } function identity(a: i32): i32 { return a; }",
		"unentered":  "export function entry(): i32 { return 0; } function bad(): i32 { let a = 2147483648; return a; }",
		"unused":     "export function entry(): i32 { let a = 2147483648; return 0; }",
	} {
		t.Run(name, func(t *testing.T) {
			result := analyze(integerRequest(source))
			if result.Status != "error" || len(result.Diagnostics) != 1 {
				t.Fatalf("default integer range must reject: %+v", result)
			}
			d := result.Diagnostics[0]
			if d.Code != "r01-d-integer-range" || d.RuleID != "r01-literals" || d.Phase != "types" || d.Category != "error" {
				t.Fatalf("default integer range masked by dependent checker context: %+v", d)
			}
		})
	}
}

func TestDefaultIntegerContextFits(t *testing.T) {
	for _, expression := range []string{"2147483647", "-2147483648", "(2147483647)", "2147483647 + 0"} {
		t.Run(expression, func(t *testing.T) {
			result := analyze(integerRequest("export function entry(): i32 { let a = " + expression + "; let b = a; return b; }"))
			if result.Status != "ok" || len(result.Diagnostics) != 0 || len(result.Symbols) != 3 || result.Symbols[1].Type != "i32" || result.Symbols[2].Type != "i32" {
				t.Fatalf("valid default integer lost nominal i32 context: %+v", result)
			}
		})
	}
}

func TestDefaultIntegerContextKeepsOtherErrors(t *testing.T) {
	for _, tc := range []struct {
		name, source, code string
		upstream           int32
	}{
		{"bool", "export function entry(a: bool): i32 { return a; }", "r01-d-scalar-type", 2322},
		{"f64", "export function entry(a: f64): i32 { return a; }", "r01-d-scalar-type", 2322},
		{"unresolved", "export function entry(): i32 { return missing; }", "r01-d-unresolved-name", 2304},
		{"typed-u64", "export function entry(a: u64): i32 { return a; }", "r01-d-scalar-type", 2322},
		{"default-no-conversion", "export function entry(): u64 { let a = 42; return a; }", "r01-d-scalar-type", 2322},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := analyze(integerRequest(tc.source))
			if result.Status != "error" || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != tc.code || result.Diagnostics[0].UpstreamCode != tc.upstream {
				t.Fatalf("unrelated official diagnostic changed: %+v", result)
			}
		})
	}
}
