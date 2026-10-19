// SPDX-License-Identifier: Apache-2.0

package main

import (
	_ "embed"
	"encoding/json"
	"testing"
)

//go:embed testdata/module-callable-facts-v1/i32-scenario.json
var mcI32Scenario23d []byte

// The nominal i32 branch is independently identified from the pinned original
// DTS TypeAlias/GetDeclaredTypeOfSymbol. Zero call signatures never imply that
// a scalar is unsupported, and no producer scalar adapter is used.
func TestModuleCallable23dIndependentI32Oracle(t *testing.T) {
	var fixture struct{ Case mcCase23d }
	if err := json.Unmarshal(mcI32Scenario23d, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Case.Name != "i32-callback" || len(fixture.Case.Recipe) != 1 || len(fixture.Case.Recipe[0].Functions) != 2 {
		t.Fatal("complete independent nominal i32 case required")
	}
	mcGenerateIndependent23d(t, fixture.Case)
}
