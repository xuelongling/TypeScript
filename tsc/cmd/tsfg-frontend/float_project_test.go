// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"
	"testing"
)

func floatRequest(source string) request {
	r := integerRequest(source)
	r.Files[1].Content += "declare const __tsfg_f32: unique symbol; type f32 = number & { readonly [__tsfg_f32]: true };\n"
	r.Files[1].Path = "/src/minimal-floats-v4.d.ts"
	r.Declarations.Version = "minimal-floats-v4"
	r.Declarations.Paths = []string{r.Files[1].Path}
	return r
}

func TestFloatDirectRounding(t *testing.T) {
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
