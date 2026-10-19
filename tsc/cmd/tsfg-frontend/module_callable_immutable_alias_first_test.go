// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"testing"
)

// This typed seam deliberately bypasses private wire/version admission. A v2
// command decoder failure is not the original callable-global capability RED.
// The complete success literal is frozen before any owned projection is run.
func TestModuleCallableImmutableAliasCausalProjectionFirst(t *testing.T) {
	before := bytes.Clone(immutableAliasWhole23g)
	var fixture immutableAliasWholeFixture23g
	if err := json.Unmarshal(immutableAliasWhole23g, &fixture); err != nil {
		t.Fatal(err)
	}
	r := fixture.Input
	original, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	r.moduleCallable = true
	calls := 0
	got := analyzeWithProjection(t.Context(), r, func(ctx ownedProjectionContext) {
		calls++
		if ctx.Root == nil || ctx.Root.FileName() != "/src/app.ts" || ctx.Checker == nil || len(ctx.SourceFiles) != 2 {
			t.Fatal("one original checked callback and exact function/source closure required")
		}
		if !reflect.DeepEqual(ctx.Facts, fixture.ExpectedFacts) {
			t.Fatal("callback must carry the complete independently frozen facts")
		}
		for i, file := range ctx.SourceFiles {
			if file.FileName() != r.Files[i].Path || file.Text() != r.Files[i].Content {
				t.Fatal("callback original TS/DTS closure differs")
			}
		}
		op := ctx.Root.Statements.Nodes[2].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0]
		if _, exists := ctx.Functions[ctx.Checker.GetSymbolAtLocation(op.Name())]; exists {
			t.Fatal("op original variable symbol must not be replaced by a function-map entry")
		}
	})
	actual, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("TSFG_M23G_TYPED_FIRST_ACTUAL_BASE64=%s callback=%d", base64.StdEncoding.EncodeToString(actual), calls)
	if !reflect.DeepEqual(got, fixture.ExpectedFacts) {
		want, _ := json.Marshal(fixture.ExpectedFacts)
		t.Fatalf("complete raw whole differs: got=%s want=%s", actual, want)
	}
	if calls != 1 {
		t.Fatalf("checked callback count=%d want=1", calls)
	}
	after, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, after) || !bytes.Equal(before, immutableAliasWhole23g) {
		t.Fatal("typed input or frozen literal data was mutated")
	}
	t.Log("TSFG_M23G_TYPED_FIRST_WHOLE_PASS rawSymbols8 functions3 modules1 globals2 originalOpRetained=true")
}
