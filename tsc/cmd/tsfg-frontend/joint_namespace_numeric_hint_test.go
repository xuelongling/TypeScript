// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

// The same public-command test bytes run against REV2 and the repaired overlay.
// Each operand must retain its declared kind and the official imported binding.
func TestJointNamespaceNumericHintPreservesDeclaredKinds(t *testing.T) {
	for _, tc := range []struct {
		kind     string
		floating bool
	}{
		{"i8", false}, {"u8", false}, {"i16", false}, {"u16", false},
		{"i32", false}, {"u32", false}, {"i64", false}, {"u64", false},
		{"isize", false}, {"usize", false}, {"f32", true}, {"f64", true},
	} {
		for _, mode := range []string{"comparison", "uncontextual-arithmetic"} {
			t.Run(tc.kind+"/"+mode, func(t *testing.T) {
				r := integerRequest("")
				if tc.floating {
					r = floatRequest("")
				}
				returnType, body := "bool", "return values.amount < 2;"
				if mode == "uncontextual-arithmetic" {
					returnType, body = tc.kind, "const local = values.amount + 1; return local;"
				}
				result, _ := jointCommandForTest(t, r, []inputFile{
					{Path: "/src/app.ts", Content: "import * as values from './state'; export function entry(): " + returnType + " { " + body + " }"},
					{Path: "/src/state.ts", Content: "export const amount: " + tc.kind + " = 1;"},
				}, "/src/app.ts")
				if result.Status != "ok" || len(result.Functions) != 1 {
					t.Fatalf("namespace %s lost its declared kind in %s: %+v", tc.kind, mode, result)
				}
				global := ""
				for _, s := range result.Symbols {
					if s.Name == "amount" && s.SourcePath == "/src/state.ts" && s.BindingKind == "source" {
						global = s.ID
					}
				}
				if global == "" {
					t.Fatal("namespace numeric operand has no official owned global")
				}
				fn := result.Functions[0]
				if fn.SourcePath != "/src/app.ts" || len(fn.Body) == 0 {
					t.Fatal("entry facts lost their source owner or body")
				}
				value, literal := fn.Body[0].Expression, "2"
				if mode == "uncontextual-arithmetic" {
					if len(fn.Body) != 2 || fn.Body[0].Symbol == nil || fn.Body[0].Symbol.Type != tc.kind || fn.Body[0].Initializer == nil || fn.Body[1].Expression == nil || fn.Body[1].Expression.Type != tc.kind {
						t.Fatal("unannotated local or returned reference defaulted to another kind")
					}
					value, literal = fn.Body[0].Initializer, "1"
				}
				if value == nil || value.Kind != "binary" || value.Type != returnType || value.Left == nil || value.Right == nil || value.Left.Type != tc.kind || value.Right.Type != tc.kind || value.Left.Kind != "reference" || value.Left.Symbol != global {
					t.Fatalf("operator operands lost their own kind or namespace identity: %+v", value)
				}
				if tc.floating {
					if value.Right.Kind != "floating" || value.Right.Float == nil {
						t.Fatal("contextual numeric literal did not use the admitted float route")
					}
				} else if value.Right.Kind != "integer" || value.Right.Literal == nil || value.Right.Literal.Decimal != literal {
					t.Fatal("contextual integer literal changed its exact value")
				}
			})
		}
	}
}
