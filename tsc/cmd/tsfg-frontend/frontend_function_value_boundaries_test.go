// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

func functionValueBoundaryRun(t *testing.T, r request, identities []closureBoundaryIdentity, calls map[string]functionValueCallWant) functionValueBoundaryResponse {
	t.Helper()
	functionValueBoundaryOfficial(t, r, identities, calls, 0)
	result := functionValueBoundaryCommand(t, r)
	functionValueBoundaryOK(t, result)
	return result
}

func TestFrontendCommandFunctionValueRebindingAndAssignmentFacts(t *testing.T) {
	const source = "export function entry(seed: bool): bool { function first(): bool { return seed; } function other(): bool { return false; } let selected: () => bool = first; const alias = selected; selected = other; const snapshot = (selected = first); return alias() && snapshot(); }"
	result := functionValueBoundaryRun(t, integerRequest(source), []closureBoundaryIdentity{
		{"seed", [][]int{{0, 1}}}, {"first", [][]int{{0, 1, 2}}}, {"selected", [][]int{{0, 1, 2, 3}}}, {"alias", [][]int{{0, 1}}}, {"other", [][]int{{0, 1}}}, {"snapshot", [][]int{{0, 1}}},
	}, map[string]functionValueCallWant{"alias()": {1, 0, "bool", ""}, "snapshot()": {1, 0, "bool", ""}})
	entry := functionValueBoundaryFunctionNamed(t, result, "entry")
	first, other := functionValueBoundaryFunctionNamed(t, result, "first"), functionValueBoundaryFunctionNamed(t, result, "other")
	selected := functionValueBoundaryBinding(t, result, "selected", entry.Symbol)
	alias := functionValueBoundaryBinding(t, result, "alias", entry.Symbol)
	snapshot := functionValueBoundaryBinding(t, result, "snapshot", entry.Symbol)
	leaf := functionValueBoundaryLeaf("bool")
	for _, s := range []functionValueWireSymbol{selected, alias, snapshot} {
		functionValueBoundarySignature(t, s.Signature, leaf)
	}
	if !selected.Writable || alias.Writable || snapshot.Writable || selected.ID == alias.ID || alias.ID == snapshot.ID {
		t.Fatal("rebindable slot and independent value bindings were merged")
	}
	if len(entry.Body) != 7 || entry.Body[2].Initializer == nil || entry.Body[3].Initializer == nil {
		t.Fatal("original ordered declarations were lost")
	}
	initial := entry.Body[2].Initializer
	if initial.Kind != "functionReference" || initial.Target == nil || *initial.Target != first.Symbol || entry.Body[3].Initializer.Kind != "reference" || entry.Body[3].Initializer.Symbol != selected.ID {
		t.Fatal("alias initializer was replaced by a guessed static target")
	}
	assignment := entry.Body[4]
	if assignment.Kind != "assignment" || assignment.Target == nil || assignment.Target.Symbol != selected.ID || assignment.Value == nil || assignment.Value.Kind != "functionReference" || assignment.Value.Target == nil || *assignment.Value.Target != other.Symbol || sourceText(t, source, assignment.Range) != "selected = other;" {
		t.Fatalf("callable rebinding lost original slot or RHS: %+v", assignment)
	}
	group := entry.Body[5].Initializer
	if group == nil || group.Kind != "group" || group.Operand == nil || sourceText(t, source, group.Range) != "(selected = first)" {
		t.Fatalf("assignment-expression grouping was lost: %+v", group)
	}
	write := group.Operand
	if write.Kind != "write" || write.Operator != "=" || write.Left == nil || write.Left.Symbol != selected.ID || write.Right == nil || write.Right.Target == nil || *write.Right.Target != first.Symbol || sourceText(t, source, write.Range) != "selected = first" {
		t.Fatalf("assignment expression lost its ordered slot/value facts: %+v", write)
	}
	functionValueBoundarySignature(t, write.Signature, leaf)
	functionValueBoundaryIndirect(t, functionValueBoundaryExpressionAt(t, source, result, "indirectCall", "alias()"), alias.ID)
	functionValueBoundaryIndirect(t, functionValueBoundaryExpressionAt(t, source, result, "indirectCall", "snapshot()"), snapshot.ID)
}

func TestFrontendCommandFunctionValueCapturesAndShadowFacts(t *testing.T) {
	const source = "export function entry(seed: bool): bool { function read(): bool { return seed; } const selected = read; function invoke(): bool { return selected(); } function pick(): () => bool { return read; } function shadow(selected: () => bool): bool { return selected(); } const returned = pick(); return invoke() && shadow(returned); }"
	result := functionValueBoundaryRun(t, integerRequest(source), []closureBoundaryIdentity{
		{"seed", [][]int{{0, 1}}}, {"read", [][]int{{0, 1, 2}}}, {"selected", [][]int{{0, 1}, {2, 3}}}, {"returned", [][]int{{0, 1}}},
	}, map[string]functionValueCallWant{"selected()": {2, 0, "bool", ""}, "pick()": {1, 0, "function", ""}, "invoke()": {1, 0, "bool", ""}, "shadow(returned)": {1, 1, "bool", ""}})
	entry := functionValueBoundaryFunctionNamed(t, result, "entry")
	read := functionValueBoundaryFunctionNamed(t, result, "read")
	invoke := functionValueBoundaryFunctionNamed(t, result, "invoke")
	pick := functionValueBoundaryFunctionNamed(t, result, "pick")
	shadow := functionValueBoundaryFunctionNamed(t, result, "shadow")
	seed := functionValueBoundaryBinding(t, result, "seed", entry.Symbol)
	selected := functionValueBoundaryBinding(t, result, "selected", entry.Symbol)
	shadowed := functionValueBoundaryBinding(t, result, "selected", shadow.Symbol)
	if selected.ID == shadowed.ID {
		t.Fatal("outer callable and shadow parameter share an owned binding")
	}
	functionValueBoundaryCapture(t, read, seed.ID)
	functionValueBoundaryCapture(t, invoke, selected.ID)
	functionValueBoundaryCapture(t, pick, read.Symbol)
	functionValueBoundaryCapture(t, shadow)
	functionValueBoundaryIndirect(t, invoke.Body[0].Expression, selected.ID)
	functionValueBoundaryIndirect(t, shadow.Body[0].Expression, shadowed.ID)
	returned := pick.Body[0].Expression
	if returned == nil || returned.Kind != "functionReference" || returned.Target == nil || *returned.Target != read.Symbol || sourceText(t, source, returned.Range) != "read" {
		t.Fatalf("named outer binding value was copied or lost: %+v", returned)
	}
	for _, fn := range []functionValueBoundaryFunction{read, invoke, pick, shadow} {
		if fn.LexicalParent != entry.Symbol {
			t.Fatalf("nested declaration lost original parent: %+v", fn)
		}
	}
}

func TestFrontendCommandComputedAndParenthesizedFunctionCalls(t *testing.T) {
	const source = "export function entry(seed: bool): bool { function make(): () => bool { function read(): bool { return seed; } return read; } const value = make()(); return (make())() && value; }"
	result := functionValueBoundaryRun(t, integerRequest(source), []closureBoundaryIdentity{{"seed", [][]int{{0, 1}}}, {"make", [][]int{{0, 1, 2}}}, {"read", [][]int{{0, 1}}}, {"value", [][]int{{0, 1}}}}, map[string]functionValueCallWant{
		"make()": {2, 0, "function", ""}, "make()()": {1, 0, "bool", ""}, "(make())()": {1, 0, "bool", ""},
	})
	factory := functionValueBoundaryFunctionNamed(t, result, "make")
	computed := functionValueBoundaryExpressionAt(t, source, result, "indirectCall", "make()()")
	parenthesized := functionValueBoundaryExpressionAt(t, source, result, "indirectCall", "(make())()")
	if computed.Target != nil || computed.Callee == nil || computed.Callee.Kind != "call" || computed.Callee.Target == nil || *computed.Callee.Target != factory.Symbol || sourceText(t, source, computed.Callee.Range) != "make()" {
		t.Fatalf("computed callee lost factory evaluation: %+v", computed)
	}
	if parenthesized.Target != nil || parenthesized.Callee == nil || parenthesized.Callee.Kind != "group" || parenthesized.Callee.Operand == nil || parenthesized.Callee.Operand.Kind != "call" || parenthesized.Callee.Operand.Target == nil || *parenthesized.Callee.Operand.Target != factory.Symbol || sourceText(t, source, parenthesized.Callee.Range) != "(make())" {
		t.Fatalf("parenthesized callee lost original grouping/evaluation: %+v", parenthesized)
	}
	functionValueBoundarySignature(t, computed.Signature, functionValueBoundaryLeaf("bool"))
	functionValueBoundarySignature(t, parenthesized.Signature, functionValueBoundaryLeaf("bool"))
}

func TestFrontendCommandStructurallyCompatibleCallableSignatures(t *testing.T) {
	const source = "export function entry(seed: bool): bool { function read(): bool { return seed; } function invoke(fn: (ignored: bool) => bool): bool { return fn(seed); } const annotated: (unused: bool) => bool = read; return invoke(read) && annotated(seed); }"
	result := functionValueBoundaryRun(t, integerRequest(source), []closureBoundaryIdentity{{"seed", [][]int{{0, 1, 2, 3}}}, {"read", [][]int{{0, 1, 2}}}, {"fn", [][]int{{0, 1}}}, {"annotated", [][]int{{0, 1}}}}, map[string]functionValueCallWant{
		"fn(seed)": {1, 1, "bool", "(ignored: bool) => bool"}, "invoke(read)": {1, 1, "bool", ""}, "annotated(seed)": {1, 1, "bool", "(unused: bool) => bool"},
	})
	entry := functionValueBoundaryFunctionNamed(t, result, "entry")
	read := functionValueBoundaryFunctionNamed(t, result, "read")
	invoke := functionValueBoundaryFunctionNamed(t, result, "invoke")
	annotated := functionValueBoundaryBinding(t, result, "annotated", entry.Symbol)
	actual, expected := functionValueBoundaryLeaf("bool"), functionValueBoundaryLeaf("bool", "bool")
	functionValueBoundarySignature(t, annotated.Signature, expected)
	functionValueBoundarySignature(t, invoke.Parameters[0].Signature, expected)
	initializer := entry.Body[2].Initializer
	if initializer == nil || initializer.Kind != "functionReference" || initializer.Target == nil || *initializer.Target != read.Symbol {
		t.Fatalf("compatible callable annotation lost actual initializer: %+v", initializer)
	}
	functionValueBoundarySignature(t, initializer.Signature, actual)
	call := functionValueBoundaryExpressionAt(t, source, result, "call", "invoke(read)")
	if call.Signature == nil || len(call.Signature.ParameterSignatures) != 1 || len(call.Arguments) != 1 {
		t.Fatalf("consumer lost its expected callable parameter: %+v", call)
	}
	functionValueBoundarySignature(t, call.Signature.ParameterSignatures[0], expected)
	functionValueBoundarySignature(t, call.Arguments[0].Signature, actual)
	if call.Arguments[0].Kind != "functionReference" || call.Arguments[0].Target == nil || *call.Arguments[0].Target != read.Symbol {
		t.Fatal("structurally compatible argument was replaced with its expected type")
	}
	functionValueBoundarySignature(t, functionValueBoundaryExpressionAt(t, source, result, "indirectCall", "annotated(seed)").Signature, expected)
	// The official checker accepts fewer parameters on the supplied function.
	// Its actual description remains distinct from the receiving callable type.
}

func TestFrontendCommandRecursiveFunctionValueAliasFacts(t *testing.T) {
	const source = "export function entry(seed: bool): bool { function recur(value: bool): bool { const alias = recur; if (value) { return value; } return alias(seed); } return recur(seed); }"
	result := functionValueBoundaryRun(t, integerRequest(source), []closureBoundaryIdentity{{"seed", [][]int{{0, 1, 2}}}, {"recur", [][]int{{0, 1, 2}}}, {"value", [][]int{{0, 1, 2}}}, {"alias", [][]int{{0, 1}}}}, map[string]functionValueCallWant{"alias(seed)": {1, 1, "bool", ""}, "recur(seed)": {1, 1, "bool", ""}})
	entry := functionValueBoundaryFunctionNamed(t, result, "entry")
	recur := functionValueBoundaryFunctionNamed(t, result, "recur")
	alias := functionValueBoundaryBinding(t, result, "alias", recur.Symbol)
	seed := functionValueBoundaryBinding(t, result, "seed", entry.Symbol)
	initializer := recur.Body[0].Initializer
	if initializer == nil || initializer.Kind != "functionReference" || initializer.Target == nil || *initializer.Target != recur.Symbol || sourceText(t, source, initializer.Range) != "recur" {
		t.Fatalf("recursive value lost its original declaration identity: %+v", initializer)
	}
	functionValueBoundaryIndirect(t, functionValueBoundaryExpressionAt(t, source, result, "indirectCall", "alias(seed)"), alias.ID)
	functionValueBoundaryCapture(t, recur, recur.Symbol, seed.ID)
	// This records an ordinary recursive source edge, not Runtime execution.
}

func TestFrontendCommandOrdinaryFunctionValueExpressions(t *testing.T) {
	for _, tc := range []struct{ name, returned string }{
		{"nestedScalarArgument", "identity(selected())"},
		{"shortCircuitAndNegation", "selected() && !selected()"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "export function entry(seed: bool): bool { function read(): bool { return seed; } function identity(value: bool): bool { return value; } const selected = read; return " + tc.returned + "; }"
			count := 1
			calls := map[string]functionValueCallWant{}
			if tc.name == "shortCircuitAndNegation" {
				count = 2
			} else {
				calls[tc.returned] = functionValueCallWant{1, 1, "bool", ""}
			}
			calls["selected()"] = functionValueCallWant{count, 0, "bool", ""}
			selectedSites := []int{0, 1}
			if count == 2 {
				selectedSites = append(selectedSites, 2)
			}
			result := functionValueBoundaryRun(t, scalarRequest(source), []closureBoundaryIdentity{{"seed", [][]int{{0, 1}}}, {"read", [][]int{{0, 1}}}, {"selected", [][]int{selectedSites}}}, calls)
			entry := functionValueBoundaryFunctionNamed(t, result, "entry")
			selected := functionValueBoundaryBinding(t, result, "selected", entry.Symbol)
			returned := entry.Body[len(entry.Body)-1].Expression
			if sourceText(t, source, returned.Range) != tc.returned {
				t.Fatalf("ordinary expression lost source anchor: %+v", returned)
			}
			if tc.name == "nestedScalarArgument" {
				if returned.Kind != "call" || len(returned.Arguments) != 1 {
					t.Fatalf("ordinary scalar call lost argument: %+v", returned)
				}
				functionValueBoundaryIndirect(t, &returned.Arguments[0], selected.ID)
			} else {
				if returned.Kind != "shortCircuit" || returned.Operator != "&&" || returned.Right == nil || returned.Right.Kind != "unary" || returned.Right.Operator != "!" {
					t.Fatalf("ordinary short circuit/negation structure was lost: %+v", returned)
				}
				functionValueBoundaryIndirect(t, returned.Left, selected.ID)
				functionValueBoundaryIndirect(t, returned.Right.Operand, selected.ID)
			}
		})
	}
}

func functionValueMigratedClosureFacts(t *testing.T, source string, r request, returned bool) {
	t.Helper()
	functionValueBoundaryOfficial(t, r, nil, nil, 0)
	result := functionValueBoundaryCommand(t, r)
	functionValueBoundaryOK(t, result)
	entry, read := functionValueBoundaryFunctionNamed(t, result, "entry"), functionValueBoundaryFunctionNamed(t, result, "read")
	seed := functionValueBoundaryBinding(t, result, "seed", entry.Symbol)
	functionValueBoundaryCapture(t, read, seed.ID)
	if returned {
		functionValueBoundarySignature(t, entry.ReturnSignature, functionValueBoundaryLeaf("bool"))
		e := entry.Body[len(entry.Body)-1].Expression
		if e == nil || e.Kind != "functionReference" || e.Target == nil || *e.Target != read.Symbol || sourceText(t, source, e.Range) != "read" {
			t.Fatalf("returned closure lost named declaration value: %+v", e)
		}
	} else {
		selected := functionValueBoundaryBinding(t, result, "selected", entry.Symbol)
		functionValueBoundarySignature(t, selected.Signature, functionValueBoundaryLeaf("bool"))
		init := entry.Body[1].Initializer
		if init == nil || init.Kind != "functionReference" || init.Target == nil || *init.Target != read.Symbol {
			t.Fatalf("alias lost original function initializer: %+v", init)
		}
		functionValueBoundaryIndirect(t, entry.Body[2].Expression, selected.ID)
	}
	// These former capability rejections now prove owned facts. Escape,
	// borrowed storage, GC lifetime and real consumer execution remain open.
}
