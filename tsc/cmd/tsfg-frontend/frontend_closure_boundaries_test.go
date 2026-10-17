// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

func TestFrontendCommandClosureCaptureBoundaries(t *testing.T) {
	const i32Source = "export function entry(seed: i32): i32 { let state: i32 = seed; function read(): i32 { return state; } return read(); }"
	const constSource = "export function entry(seed: bool): bool { const frozen: bool = seed; function observe(): bool { return frozen; } return observe(); }"
	const parameterSource = "export function entry(seed: bool): bool { function read(): bool { return seed; } return read(); }"
	const shadowSource = "export function entry(seed: bool): bool { let state: bool = seed; function read(state: bool): bool { return state; } return read(state); }"
	const writeOnlySource = "export function entry(seed: bool): bool { let state: bool = seed; function write(value: bool): bool { state = value; return value; } return write(seed); }"
	const ancestorSource = "export function entry(seed: bool): bool { const state: bool = seed; function middle(): bool { function read(): bool { return state; } return read(); } return middle(); }"
	for _, tc := range []struct {
		name       string
		source     string
		payload    string
		identities []closureBoundaryIdentity
		functions  []closureBoundaryFunctionWant
		bindings   []closureBoundaryBinding
	}{
		{
			name: "existingI32Binding", source: i32Source, payload: "i32",
			identities: []closureBoundaryIdentity{{"seed", [][]int{{0, 1}}}, {"state", [][]int{{0, 1}}}, {"read", [][]int{{0, 1}}}},
			functions: []closureBoundaryFunctionWant{
				{Name: "entry", Text: i32Source, ReturnTarget: "read"},
				{Name: "read", Parent: "entry", Text: "function read(): i32 { return state; }", ReturnBinding: "state", BindingOwner: "entry", Capture: &closureBoundaryCaptureWant{"state", "entry", true, false}},
			},
			bindings: []closureBoundaryBinding{{"seed", "entry", "parameter", true}, {"state", "entry", "local", true}},
		},
		{
			// Deliberately rename both the captured binding and its reader: the
			// official identity and owned facts must not rely on tracer names.
			name: "readonlyConstWithRenamedBinding", source: constSource, payload: "bool",
			identities: []closureBoundaryIdentity{{"seed", [][]int{{0, 1}}}, {"frozen", [][]int{{0, 1}}}, {"observe", [][]int{{0, 1}}}},
			functions: []closureBoundaryFunctionWant{
				{Name: "entry", Text: constSource, ReturnTarget: "observe"},
				{Name: "observe", Parent: "entry", Text: "function observe(): bool { return frozen; }", ReturnBinding: "frozen", BindingOwner: "entry", Capture: &closureBoundaryCaptureWant{"frozen", "entry", true, false}},
			},
			bindings: []closureBoundaryBinding{{"seed", "entry", "parameter", true}, {"frozen", "entry", "local", false}},
		},
		{
			name: "outerParameter", source: parameterSource, payload: "bool",
			identities: []closureBoundaryIdentity{{"seed", [][]int{{0, 1}}}, {"read", [][]int{{0, 1}}}},
			functions: []closureBoundaryFunctionWant{
				{Name: "entry", Text: parameterSource, ReturnTarget: "read"},
				{Name: "read", Parent: "entry", Text: "function read(): bool { return seed; }", ReturnBinding: "seed", BindingOwner: "entry", Capture: &closureBoundaryCaptureWant{"seed", "entry", true, false}},
			},
			bindings: []closureBoundaryBinding{{"seed", "entry", "parameter", true}},
		},
		{
			name: "shadowParameterIsNotCapture", source: shadowSource, payload: "bool",
			identities: []closureBoundaryIdentity{{"seed", [][]int{{0, 1}}}, {"state", [][]int{{0, 3}, {1, 2}}}, {"read", [][]int{{0, 1}}}},
			functions: []closureBoundaryFunctionWant{
				{Name: "entry", Text: shadowSource, ReturnTarget: "read", ReturnText: "return read(state);", ArgumentBinding: "state", ArgumentOwner: "entry"},
				{Name: "read", Parent: "entry", Text: "function read(state: bool): bool { return state; }", ReturnBinding: "state", BindingOwner: "read"},
			},
			bindings: []closureBoundaryBinding{{"seed", "entry", "parameter", true}, {"state", "entry", "local", true}, {"state", "read", "parameter", true}},
		},
		{
			name: "writeOnlyCaptureHasExplicitFalseRead", source: writeOnlySource, payload: "bool",
			identities: []closureBoundaryIdentity{{"seed", [][]int{{0, 1, 2}}}, {"state", [][]int{{0, 1}}}, {"value", [][]int{{0, 1, 2}}}, {"write", [][]int{{0, 1}}}},
			functions: []closureBoundaryFunctionWant{
				{Name: "entry", Text: writeOnlySource, ReturnTarget: "write", ReturnText: "return write(seed);", ArgumentBinding: "seed", ArgumentOwner: "entry"},
				{Name: "write", Parent: "entry", Text: "function write(value: bool): bool { state = value; return value; }", ReturnBinding: "value", BindingOwner: "write", Capture: &closureBoundaryCaptureWant{"state", "entry", false, true}},
			},
			bindings: []closureBoundaryBinding{{"seed", "entry", "parameter", true}, {"state", "entry", "local", true}, {"value", "write", "parameter", true}},
		},
		{
			name: "ancestorBindingWithoutMiddleDirectAccess", source: ancestorSource, payload: "bool",
			identities: []closureBoundaryIdentity{{"seed", [][]int{{0, 1}}}, {"state", [][]int{{0, 1}}}, {"middle", [][]int{{0, 1}}}, {"read", [][]int{{0, 1}}}},
			functions: []closureBoundaryFunctionWant{
				{Name: "entry", Text: ancestorSource, ReturnTarget: "middle"},
				{Name: "middle", Parent: "entry", Text: "function middle(): bool { function read(): bool { return state; } return read(); }", ReturnTarget: "read"},
				{Name: "read", Parent: "middle", Text: "function read(): bool { return state; }", ReturnBinding: "state", BindingOwner: "entry", Capture: &closureBoundaryCaptureWant{"state", "entry", true, false}},
			},
			bindings: []closureBoundaryBinding{{"seed", "entry", "parameter", true}, {"state", "entry", "local", false}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := integerRequest(tc.source)
			closureBoundaryOfficial(t, r, tc.identities)
			closureBoundaryFacts(t, tc.source, tc.payload, closureBoundaryCommand(t, r), tc.functions, tc.bindings)
		})
	}
	for _, tc := range []struct {
		name   string
		source string
		code   string
		anchor string
	}{
		{
			name:   "functionValueStillUnsupported",
			source: "export function entry(seed: bool): bool { function read(): bool { return seed; } const selected = read; return selected(); }",
			code:   "r02b-d-type", anchor: "read",
		},
		{
			name:   "returnedClosureStillUnsupported",
			source: "export function entry(seed: bool): () => bool { function read(): bool { return seed; } return read; }",
			code:   "r02b-d-unsupported", anchor: "() => bool",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := integerRequest(tc.source)
			closureBoundaryOfficial(t, r, []closureBoundaryIdentity{{"seed", [][]int{{0, 1}}}, {"read", [][]int{{0, 1}}}})
			result := closureBoundaryCommand(t, r)
			if result.Status == "ok" || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != tc.code || sourceText(t, tc.source, result.Diagnostics[0].Range) != tc.anchor {
				t.Fatalf("unsupported function-value boundary lost its concrete rejection and source anchor: %+v", result)
			}
			// These are capability boundaries, not a proof of escape safety.
		})
	}
}
