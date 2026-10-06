// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"reflect"
	"strings"
	"testing"
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
				if expr == nil || prior == nil || expr.Kind != prior.Kind || expr.Type != prior.Type || expr.Range.End != prior.Range.End || strings.TrimSpace(sourceText(t, source, expr.Range)) != strings.TrimSpace(sourceText(t, source, prior.Range)) || expr.Symbol != prior.Symbol || !reflect.DeepEqual(expr.Literal, prior.Literal) {
					t.Fatalf("official literal/reference facts changed: current=%+v baseline=%+v", expr, prior)
				}
				if expr.Type == "f64" && !reflect.DeepEqual(expr, prior) {
					t.Fatalf("f64 reference facts changed: current=%+v baseline=%+v", expr, prior)
				}
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
