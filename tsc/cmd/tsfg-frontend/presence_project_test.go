// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

func TestPresenceOfficialParameterAndFlowFacts(t *testing.T) {
	source := "function choose(value: i32 | null = 7): i32 { if (value === null) { return 3; } return value; } export function entry(): i32 { return choose(undefined) + choose(null) + choose(0) + choose(); }"
	result := analyze(integerRequest(source))
	if result.Status != "ok" {
		t.Fatalf("official nullable/default facts rejected: %+v", result.Diagnostics)
	}
	fn := result.Functions[0]
	shape := fn.Parameters[0].Presence
	if shape == nil || !shape.Undefined || !shape.Null || !shape.Optional || !shape.Default || sourceText(t, source, shape.Annotation) != "i32 | null" {
		t.Fatalf("incorrect checked declaration facts: %+v", shape)
	}
	guard := fn.Body[0].Condition
	if guard.Kind != "presenceTest" || guard.Operator != "===" || guard.Left.PresenceFlow == nil || guard.Left.PresenceFlow.Undefined || !guard.Left.PresenceFlow.Null || guard.Right.Absence != "null" {
		t.Fatalf("guard lost official body flow: %+v", guard)
	}
	value := fn.Body[1].Expression
	if value.PresenceFlow == nil || value.PresenceFlow.Undefined || value.PresenceFlow.Null {
		t.Fatalf("post-return narrowing lost: %+v", value)
	}
}

func TestPresenceOfficialOptionalNarrowingAndCarrierForwarding(t *testing.T) {
	for _, source := range []string{
		"function choose(value?: i32): i32 { if (value === undefined) { return 8; } return value; } export function entry(): i32 { return choose(); }",
		"function choose(value: i32 | null = 7): i32 { if (value === null) { return 8; } return value; } function relay(value?: i32): i32 { return choose((value)); } export function entry(): i32 { return relay(); }",
		"function choose(a: i32 = 2, b: i32 = a++): i32 { return a * 100 + b; } export function entry(): i32 { return choose(4); }",
		"export function entry(): i32 { let value: i32 | undefined = undefined; value = 4; return value; }",
		"export function entry(): i32 { let value: i32 | null = null; if (value === null) { value = 5; } return value; }",
		"function choose(value?: i32): i32 { if (value === undefined) { return 8; } return value; } export function entry(): i32 { let value: i32 | undefined = undefined; return choose(value); }",
	} {
		result := analyze(integerRequest(source))
		if result.Status != "ok" {
			t.Fatalf("official finite presence source rejected: %s: %+v", source, result.Diagnostics)
		}
	}
}

func TestPresenceDoesNotEraseOfficialInvalidReadsOrLateDefaults(t *testing.T) {
	for _, source := range []string{
		"export function entry(value?: i32): i32 { return value; }",
		"export function entry(value: i32 | null): i32 { return value + 1; }",
		"function choose(a: i32 = b, b: i32 = 3): i32 { return a; } export function entry(): i32 { return choose(); }",
		"function choose(a: i32 = undefined): i32 { return a; } export function entry(): i32 { return choose(); }",
		"export function entry(): i32 { let value: i32 | undefined = undefined; return value; }",
		"export function entry(): i32 { let value: i32 | null = null; return value + 1; }",
	} {
		result := analyze(integerRequest(source))
		if result.Status == "ok" {
			t.Fatalf("invalid official presence source admitted: %s", source)
		}
	}
}

func TestPresenceLocalAssignmentUsesOfficialNarrowedType(t *testing.T) {
	source := "export function entry(): i32 { let value: i32 | undefined = undefined; value = 4; return value; }"
	result := analyze(integerRequest(source))
	if result.Status != "ok" {
		t.Fatalf("official contextual assignment rejected: %+v", result.Diagnostics)
	}
	local := result.Functions[0].Body[0]
	read := result.Functions[0].Body[2].Expression
	if local.Symbol.Presence == nil || !local.Symbol.Presence.Undefined || local.Initializer.Absence != "undefined" || read.PresenceFlow == nil || read.PresenceFlow.Undefined || read.PresenceFlow.Null {
		t.Fatalf("actual GetTypeAtLocation flow failed to narrow: local=%+v read=%+v", local, read)
	}
}
