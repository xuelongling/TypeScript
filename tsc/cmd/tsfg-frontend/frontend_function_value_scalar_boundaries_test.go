// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"testing"
)

func TestFrontendCommandCallableScalarFamilyFacts(t *testing.T) {
	for _, kind := range []string{"i8", "i16", "i32", "i64", "u8", "u16", "u32", "u64", "isize", "usize", "bool", "f32", "f64"} {
		t.Run(kind, func(t *testing.T) {
			source := fmt.Sprintf("export function entry(seed: %[1]s): %[1]s { function identity(value: %[1]s): %[1]s { return value; } function apply(fn: (value: %[1]s) => %[1]s, value: %[1]s): %[1]s { return fn(value); } function make(): (value: %[1]s) => %[1]s { return identity; } const selected = make(); return apply(selected, seed); }", kind)
			result := functionValueBoundaryRun(t, floatRequest(source), []closureBoundaryIdentity{
				{"seed", [][]int{{0, 1}}}, {"identity", [][]int{{0, 1}}}, {"fn", [][]int{{0, 1}}}, {"selected", [][]int{{0, 1}}}, {"make", [][]int{{0, 1}}},
			}, map[string]functionValueCallWant{"fn(value)": {1, 1, kind, ""}, "make()": {1, 0, "function", ""}, "apply(selected, seed)": {1, 2, kind, ""}})
			entry := functionValueBoundaryFunctionNamed(t, result, "entry")
			identity := functionValueBoundaryFunctionNamed(t, result, "identity")
			apply := functionValueBoundaryFunctionNamed(t, result, "apply")
			makeFn := functionValueBoundaryFunctionNamed(t, result, "make")
			selected := functionValueBoundaryBinding(t, result, "selected", entry.Symbol)
			leaf := functionValueBoundaryLeaf(kind, kind)
			functionValueBoundarySignature(t, selected.Signature, leaf)
			functionValueBoundarySignature(t, makeFn.ReturnSignature, leaf)
			if len(apply.Parameters) != 2 || apply.Parameters[0].Type != "function" || apply.Parameters[1].Type != kind || apply.Parameters[1].Signature != nil {
				t.Fatalf("mixed callable/scalar parameters lost types: %+v", apply.Parameters)
			}
			functionValueBoundarySignature(t, apply.Parameters[0].Signature, leaf)
			call := functionValueBoundaryExpressionAt(t, source, result, "call", "apply(selected, seed)")
			if call.Type != kind || call.Signature == nil || len(call.Signature.Parameters) != 2 || call.Signature.Parameters[0] != "function" || call.Signature.Parameters[1] != kind || len(call.Signature.ParameterSignatures) != 2 || call.Signature.ParameterSignatures[1] != nil || len(call.Arguments) != 2 || call.Arguments[0].Symbol != selected.ID || call.Arguments[1].Symbol != entry.Parameters[0].ID {
				t.Fatalf("scalar-family call lost aligned signatures or ordered arguments: %+v", call)
			}
			functionValueBoundarySignature(t, call.Signature.ParameterSignatures[0], leaf)
			indirect := functionValueBoundaryExpressionAt(t, source, result, "indirectCall", "fn(value)")
			functionValueBoundaryIndirect(t, indirect, apply.Parameters[0].ID)
			functionValueBoundarySignature(t, indirect.Signature, leaf)
			if indirect.Type != kind || len(indirect.Arguments) != 1 || indirect.Arguments[0].Type != kind || indirect.Arguments[0].Symbol != apply.Parameters[1].ID {
				t.Fatalf("callable scalar pass-through argument was changed: %+v", indirect)
			}
			if len(identity.Parameters) != 1 || identity.Parameters[0].Type != kind || identity.Body[0].Expression == nil || identity.Body[0].Expression.Kind != "reference" || identity.Body[0].Expression.Type != kind || identity.Body[0].Expression.Symbol != identity.Parameters[0].ID {
				t.Fatalf("scalar pass-through body lost original binding: %+v", identity)
			}
			returned := makeFn.Body[0].Expression
			if returned == nil || returned.Kind != "functionReference" || returned.Target == nil || *returned.Target != identity.Symbol || sourceText(t, source, returned.Range) != "identity" {
				t.Fatalf("scalar-family returned callable lost declaration identity: %+v", returned)
			}
			functionValueBoundarySignature(t, returned.Signature, leaf)
		})
	}
}

func TestFrontendCommandF64ArithmeticInsideCallableArgument(t *testing.T) {
	const source = "export function entry(seed: f64): f64 { function identity(value: f64): f64 { return value; } function apply(fn: (value: f64) => f64): f64 { return fn(seed + seed); } return apply(identity); }"
	result := functionValueBoundaryRun(t, floatRequest(source), []closureBoundaryIdentity{{"seed", [][]int{{0, 1, 2}}}, {"identity", [][]int{{0, 1}}}, {"fn", [][]int{{0, 1}}}}, map[string]functionValueCallWant{"fn(seed + seed)": {1, 1, "f64", ""}, "apply(identity)": {1, 1, "f64", ""}})
	entry := functionValueBoundaryFunctionNamed(t, result, "entry")
	apply := functionValueBoundaryFunctionNamed(t, result, "apply")
	indirect := functionValueBoundaryExpressionAt(t, source, result, "indirectCall", "fn(seed + seed)")
	functionValueBoundaryIndirect(t, indirect, apply.Parameters[0].ID)
	if len(indirect.Arguments) != 1 {
		t.Fatalf("callable lost arithmetic argument: %+v", indirect)
	}
	argument := indirect.Arguments[0]
	if argument.Kind != "binary" || argument.Type != "f64" || argument.Operator != "+" || argument.Left == nil || argument.Right == nil || argument.Left.Kind != "reference" || argument.Right.Kind != "reference" || argument.Left.Type != "f64" || argument.Right.Type != "f64" || argument.Left.Symbol != entry.Parameters[0].ID || argument.Right.Symbol != entry.Parameters[0].ID || sourceText(t, source, argument.Range) != "seed + seed" {
		t.Fatalf("float target or source evaluation structure was lost inside callable argument: %+v", argument)
	}
	functionValueBoundaryCapture(t, apply, entry.Parameters[0].ID)
}

func TestFrontendCommandFunctionValueCapabilityBoundaries(t *testing.T) {
	for _, tc := range []struct{ name, source, anchor, declaration string }{
		{"anonymous", "export function entry(seed: bool): bool { const selected = (): bool => seed; return selected(); }", "(): bool => seed", ""},
		{"generic", "export function entry(seed: bool): bool { function read<T>(): bool { return seed; } const selected = read; return selected(); }", "read", ""},
		{"rest", "export function entry(seed: bool): bool { function read(...values: bool[]): bool { return seed; } const selected = read; return selected(); }", "...values: bool[]", ""},
		// Recursive callable types are valid official source. The current owned
		// finite description rejects the cycle rather than recursing forever.
		{"cyclicCallable", "export function entry(seed: bool): bool { function invoke(fn: Loop): bool { return seed; } return seed; }", "fn: Loop", "interface Loop { (): Loop; }\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := integerRequest(tc.source)
			r.Files[1].Content += tc.declaration
			functionValueBoundaryOfficial(t, r, nil, nil, 0)
			result := functionValueBoundaryCommand(t, r)
			if result.Status != "error" || len(result.Diagnostics) != 1 {
				t.Fatalf("unsupported function-value form lost concrete capability diagnostic: %+v", result)
			}
			d := result.Diagnostics[0]
			if d.Code != "r02b-d-unsupported" || d.RuleID != "r01-ts-baseline" || d.Category != "unsupported" || d.Phase != "capability" || d.Severity != "error" || d.UpstreamCode != 0 || sourceText(t, tc.source, d.Range) != tc.anchor {
				t.Fatalf("unsupported form changed machine identity or source anchor: %+v", d)
			}
		})
	}
}

func TestFrontendCommandFunctionValueWrongArgumentKeepsOfficialDiagnostic(t *testing.T) {
	const source = "export function entry(seed: bool): bool { function read(value: bool): bool { return value; } const selected = read; return selected(1); }"
	r := scalarRequest(source)
	functionValueBoundaryOfficial(t, r, nil, nil, 2345)
	result := functionValueBoundaryCommand(t, r)
	if result.Status != "error" || len(result.Diagnostics) != 1 {
		t.Fatalf("invalid actual argument was accepted: %+v", result)
	}
	d := result.Diagnostics[0]
	if d.Code != "r01-d-scalar-type" || d.RuleID != "r01-calls" || d.Category != "error" || d.Phase != "types" || d.Severity != "error" || d.UpstreamCode != 2345 || sourceText(t, source, d.Range) != "1" {
		t.Fatalf("official argument error lost identity or source anchor: %+v", d)
	}
}
