// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

func TestPresenceBooleanLogicalAssignmentFacts(t *testing.T) {
	for _, source := range []string{
		"export function entry(value?: bool): bool { if (value === undefined) { value = false; } value ||= true; return value; }",
		"export function entry(value?: bool): bool { if (value === undefined) { return false; } value &&= false; return value; }",
		"export function entry(value?: bool): bool { value ||= true; return value; }",
		"export function entry(value?: bool): bool { value &&= false; if (value === undefined) { return true; } return value; }",
		"export function entry(value: bool | null = null): bool { value &&= true; if (value === null) { return true; } return value; }",
	} {
		result := analyze(integerRequest(source))
		if result.Status != "ok" {
			t.Fatalf("official logical presence source rejected: %s: %+v", source, result.Diagnostics)
		}
	}
	source := "export function entry(value?: bool): bool { value &&= false; if (value === undefined) { return true; } return value; }"
	result := analyze(integerRequest(source))
	write := result.Functions[0].Body[0].Expression
	if write.Left.PresenceFlow == nil || !write.Left.PresenceFlow.Undefined || write.PresenceFlow == nil || !write.PresenceFlow.Undefined || write.PresenceFlow.Null {
		t.Fatalf("lost actual declared target or whole expression presence: %+v", write)
	}
}

func TestPresenceBooleanLogicalResultDoesNotEraseOfficialErrors(t *testing.T) {
	for _, tc := range []struct {
		source string
		code   int32
	}{
		{"export function entry(value?: bool): bool { return value &&= false; }", 2322},
		{"export function entry(value: bool | null): bool { return value &&= true; }", 2322},
		{"export function entry(value?: bool): bool { value ||= 0; return value; }", 2322},
		{"export function entry(): bool { const value: bool = false; value ||= true; return value; }", 2588},
	} {
		result := analyze(integerRequest(tc.source))
		found := false
		for _, diagnostic := range result.Diagnostics {
			found = found || diagnostic.UpstreamCode == tc.code
		}
		if result.Status != "error" || !found {
			t.Fatalf("lost official logical error %d: %+v", tc.code, result.Diagnostics)
		}
	}
}

func TestPresenceParenthesizedLocalTargets(t *testing.T) {
	for _, source := range []string{
		"export function entry(): i32 { let value: i32; ((value)) ??= 7; return (value)++; }",
		"export function entry(): i32 { let value: i32 | undefined; ((value)) = 7; return ++(value); }",
		"export function entry(): i32 { let value: i32 = 3; (value) += ((value) = 4); return value; }",
	} {
		result := analyze(integerRequest(source))
		if result.Status != "ok" {
			t.Fatalf("official parenthesized local target rejected: %s: %+v", source, result.Diagnostics)
		}
	}
	source := "export function entry(): i32 { let value: i32; ((value)) ??= 7; return (value)++; }"
	result := analyze(integerRequest(source))
	write := result.Functions[0].Body[1].Expression
	if write.Kind != "write" || write.Left.Kind != "group" || write.Left.Operand.Kind != "group" || write.Left.Operand.Operand.Kind != "reference" || sourceText(t, source, write.Left.Range) != "((value))" {
		t.Fatalf("parenthesized target lost its whole and leaf AST facts: %+v", write)
	}
}

func TestPresenceParenthesizedTargetsRetainOfficialReadErrors(t *testing.T) {
	for _, tc := range []struct {
		source string
		code   int32
	}{
		{"export function entry(): i32 { let value: i32; return (value)++; }", 2454},
		{"export function entry(): i32 { const value: i32 = 1; (value)++; return value; }", 2588},
	} {
		result := analyze(integerRequest(tc.source))
		found := false
		for _, diagnostic := range result.Diagnostics {
			found = found || diagnostic.UpstreamCode == tc.code
		}
		if result.Status != "error" || !found {
			t.Fatalf("lost official read error %d: %+v", tc.code, result.Diagnostics)
		}
	}
}

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

func TestPresenceNullishAssignmentOfficialFacts(t *testing.T) {
	for _, source := range []string{
		"export function entry(value?: i32): i32 { return value ??= 4; }",
		"export function entry(): i32 { let value: i32 | null = null; value ??= 5; return value; }",
		"export function entry(value: bool | null): bool { return value ??= false; }",
	} {
		result := analyze(integerRequest(source))
		if result.Status != "ok" {
			t.Fatalf("official nullish assignment rejected: %s: %+v", source, result.Diagnostics)
		}
		body := result.Functions[0].Body
		var write *expression
		if body[0].Kind == "return" {
			write = body[0].Expression
		} else {
			write = body[1].Expression
		}
		if write == nil || write.Kind != "write" || write.Operator != "??=" || write.Left.PresenceFlow == nil {
			t.Fatalf("official nullish write facts lost: %+v", body)
		}
	}
}

func TestPresenceNullishAssignmentBareLocalsAndTargetType(t *testing.T) {
	for _, source := range []string{
		"export function entry(): i32 { let value: i32; return value ??= 7; }",
		"export function entry(): i32 { let value: i32 | null; return value ??= 7; }",
		"export function entry(): i32 { let value: i32 | undefined; return value ??= 7; }",
		"export function entry(flag: bool): i32 { let value: i32; if (flag) { value = 8; } value ??= 7; return value; }",
		"function choose(value: i32 = 7): i32 { return value ??= 9; } export function entry(): i32 { return choose(); }",
		"function choose(value: i32 | undefined = 7): i32 { return value ??= 9; } export function entry(): i32 { return choose(); }",
	} {
		result := analyze(integerRequest(source))
		if result.Status != "ok" {
			t.Fatalf("official definite nullish target rejected: %s: %+v", source, result.Diagnostics)
		}
	}
	source := "export function entry(): i32 { let value: i32 | undefined = 0; value ??= 7; return value; }"
	result := analyze(integerRequest(source))
	if result.Status != "ok" {
		t.Fatalf("official nullish flow rejected: %+v", result.Diagnostics)
	}
	write, read := result.Functions[0].Body[1].Expression, result.Functions[0].Body[2].Expression
	if write.Left.PresenceFlow == nil || !write.Left.PresenceFlow.Undefined || read.PresenceFlow == nil || read.PresenceFlow.Undefined || read.PresenceFlow.Null {
		t.Fatalf("official definite target/read distinction lost: target=%+v read=%+v", write.Left, read)
	}
}

func TestPresenceNullishDoesNotEraseUninitializedReadDiagnostics(t *testing.T) {
	for _, source := range []string{
		"export function entry(): i32 { let value: i32; return value; }",
		"export function entry(): i32 { let value: i32; return value += 7; }",
		"export function entry(): i32 { let value: i32 | null; if (value === null) { return 2; } return value; }",
	} {
		result := analyze(integerRequest(source))
		found := false
		for _, diagnostic := range result.Diagnostics {
			if diagnostic.UpstreamCode == 2454 {
				found = true
			}
		}
		if result.Status != "error" || !found {
			t.Fatalf("official uninitialized read diagnostic missing: %s: %+v", source, result.Diagnostics)
		}
	}
}
