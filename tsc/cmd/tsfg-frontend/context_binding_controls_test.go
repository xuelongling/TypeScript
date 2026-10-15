// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

func TestSingleSourceContextParametersKeepOfficialIdentities(t *testing.T) {
	result := analyze(integerRequest("export function entry(amount: i64): i64 { const local = amount + 1; return local; } function other(amount: u64): u64 { const local = amount + 1; return local; }"))
	if result.Status != "ok" || len(result.Functions) != 2 {
		t.Fatalf("single-source checked parameter inference changed: %+v", result)
	}
	parameterIDs, localIDs := map[string]bool{}, map[string]bool{}
	for index, kind := range []string{"i64", "u64"} {
		fn := result.Functions[index]
		if len(fn.Parameters) != 1 || len(fn.Body) != 2 || fn.Body[0].Symbol == nil {
			t.Fatal("single-source parameter or local facts are incomplete")
		}
		parameter, local := fn.Parameters[0], fn.Body[0].Symbol
		value, returned := fn.Body[0].Initializer, fn.Body[1].Expression
		if parameter.ID == "" || local.ID == "" || parameterIDs[parameter.ID] || localIDs[local.ID] {
			t.Fatal("same spelling merged independent function bindings")
		}
		parameterIDs[parameter.ID], localIDs[local.ID] = true, true
		if parameter.Type != kind || local.Type != kind || value == nil || value.Type != kind || value.Left == nil || value.Left.Symbol != parameter.ID || value.Left.Type != kind || value.Right == nil || value.Right.Type != kind || returned == nil || returned.Symbol != local.ID || returned.Type != kind {
			t.Fatal("single-source local inference lost its own official parameter context")
		}
	}
}

func TestSingleSourceContextUnannotatedArithmeticStillDefaultsI32(t *testing.T) {
	ok := analyze(integerRequest("export function entry(): i32 { const local = 1 + 2; return local; }"))
	if ok.Status != "ok" || len(ok.Functions) != 1 || len(ok.Functions[0].Body) != 2 {
		t.Fatalf("unannotated default-i32 arithmetic rejected: %+v", ok)
	}
	local := ok.Functions[0].Body[0]
	if local.Symbol == nil || local.Symbol.Type != "i32" || local.Initializer == nil || local.Initializer.Type != "i32" {
		t.Fatal("unannotated arithmetic acquired a non-i32 kind")
	}
	rejected := analyze(integerRequest("export function entry(): i64 { const local = 1 + 2; return local; }"))
	if rejected.Status != "error" || len(rejected.Diagnostics) != 1 || rejected.Diagnostics[0].Code != "r01-d-scalar-type" || rejected.Diagnostics[0].UpstreamCode != 2322 {
		t.Fatalf("return annotation promoted a default-i32 local or changed the official diagnostic: %+v", rejected)
	}
}
