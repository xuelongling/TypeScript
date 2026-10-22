// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"testing"
)

func TestArrayGenericV8WideFieldWriteThroughAlias25f(t *testing.T) {
	source := "class Narrow{n:i32=1;} class Wide{n:i32|undefined=undefined;} export function run():i32|undefined{const narrow=new Narrow();const wide:Wide=narrow;wide.n=undefined;return narrow.n;}"
	p := genericArrayPublic25f(t, source)
	if len(p["diagnostics"].([]any)) != 0 {
		t.Fatal("official checked structural alias source must have zero diagnostics")
	}
	types := genericConstructionTypes25f(t, p)
	narrow, wide := types["Narrow"], types["Wide"]
	narrowField, wideField := (*narrow.Fields)[0], (*wide.Fields)[0]
	if narrowField.Type != "i32" || wideField.Type != "union<i32,undefined>" || narrowField.Readonly || wideField.Readonly || narrowField.Initializer.Decimal != "1" || wideField.Initializer.Kind != "undefinedLiteral" {
		t.Fatal("original narrow and writable wider field declarations changed")
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var payload arrayGenericPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Functions) != 1 || payload.Functions[0].Name != "run" || payload.Functions[0].ReturnType != wideField.Type {
		t.Fatal("original export signature and closure changed")
	}
	body := payload.Functions[0].Body
	if len(body) != 4 {
		t.Fatal("original allocation/view/store/read order changed")
	}
	allocation, view := body[0].Initializer, body[1].Initializer
	if allocation == nil || allocation.Kind != "new" || allocation.Callee != narrow.ID || view == nil || view.Kind != "bindingReference" || view.Binding != body[0].Binding || len(view.CompatibilityRoots) != 1 || view.CompatibilityRoots[0].SourceType != narrow.ID || view.CompatibilityRoots[0].TargetType != wide.ID {
		t.Fatal("wide local must retain its real narrow allocation alias and authority root")
	}
	found := false
	for _, record := range payload.CompatibilityWitnesses {
		proof := record.Compatibility
		if proof.SourceType != narrow.ID || proof.TargetType != wide.ID || proof.Origin.Start != view.Range.Start || proof.Origin.End != view.Range.End {
			continue
		}
		if !proof.Assignable || len(proof.Members) != 1 || proof.Members[0].Source != narrowField.ID || proof.Members[0].Target != wideField.ID {
			t.Fatal("official M12 narrow-to-wide field projection changed")
		}
		found = true
	}
	if !found {
		t.Fatal("missing original same-origin official field compatibility witness")
	}
	store := body[2].Value
	if store == nil || store.Kind != "fieldAssignment" || store.Target == nil || store.Target.Type != wideField.Type || store.Target.Property != wideField.ID || store.Target.Receiver.Binding != body[1].Binding || store.Target.Receiver.Type != wide.ID || store.Value.Kind != "undefinedLiteral" || store.FlowType != "undefined" {
		t.Fatal("write must retain its writable static union view and valid undefined RHS")
	}
	read := body[3].Value
	if body[3].Kind != "return" || read == nil || read.Kind != "fieldAccess" || read.Property != narrowField.ID || read.Type != "i32" || read.FlowType != "i32" || read.Receiver.Binding != body[0].Binding || read.Receiver.Type != narrow.ID {
		t.Fatal("read must retain the original narrow alias and official static field facts")
	}
	for _, occurrence := range []struct {
		value *arrayGenericExpression
		text  string
	}{{allocation, "new Narrow()"}, {view, "narrow"}, {store, "wide.n=undefined"}, {read, "narrow.n"}} {
		if source[occurrence.value.Range.Start:occurrence.value.Range.End] != occurrence.text {
			t.Fatal("original structural alias/write/read source anchor changed")
		}
	}
	if !(allocation.Range.Start < view.Range.Start && view.Range.Start < store.Range.Start && store.Range.Start < read.Range.Start) {
		t.Fatal("original alias/write/read evaluation sequence changed")
	}
}
