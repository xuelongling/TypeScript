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

func TestArrayGenericV8NarrowFieldCopyAfterWideWrite25f(t *testing.T) {
	source := "class Narrow{n:i32=1;} class Wide{n:i32|undefined=undefined;} export function run():i32|undefined{const narrow=new Narrow();const wide:Wide=narrow;wide.n=undefined;const copied:i32=narrow.n;narrow.n=copied;return narrow.n;}"
	p := genericArrayPublic25f(t, source)
	if len(p["diagnostics"].([]any)) != 0 {
		t.Fatal("official checked copy/store source must have zero diagnostics")
	}
	types := genericConstructionTypes25f(t, p)
	narrow, wide := types["Narrow"], types["Wide"]
	narrowField, wideField := (*narrow.Fields)[0], (*wide.Fields)[0]
	if narrowField.Type != "i32" || narrowField.Readonly || wideField.Type != "union<i32,undefined>" || wideField.Readonly {
		t.Fatal("original field identities, static types and write permissions changed")
	}
	fn := genericFindFunction25f(t, p, "run")
	body := fn["body"].([]any)
	if fn["returnType"] != wideField.Type || len(body) != 6 {
		t.Fatal("original allocation/view/write/copy/store/return order changed")
	}
	narrowBinding := body[0].(map[string]any)["binding"]
	wideBinding := body[1].(map[string]any)["binding"]
	wideStore := body[2].(map[string]any)["value"].(map[string]any)
	wideTarget := wideStore["target"].(map[string]any)
	if wideStore["kind"] != "fieldAssignment" || wideTarget["property"] != wideField.ID || wideTarget["type"] != wideField.Type || wideTarget["receiver"].(map[string]any)["binding"] != wideBinding || wideStore["value"].(map[string]any)["kind"] != "undefinedLiteral" {
		t.Fatal("original valid-undefined write through the wider static view changed")
	}
	local := body[3].(map[string]any)
	copyRead := local["initializer"].(map[string]any)
	bound := false
	for _, raw := range fn["bindings"].([]any) {
		binding := raw.(map[string]any)
		if binding["id"] == local["binding"] {
			bound = binding["name"] == "copied" && binding["declaredType"] == "i32"
		}
	}
	store := body[4].(map[string]any)["value"].(map[string]any)
	target := store["target"].(map[string]any)
	value := store["value"].(map[string]any)
	if !bound || local["kind"] != "constDeclaration" || store["kind"] != "fieldAssignment" || target["property"] != narrowField.ID || target["type"] != "i32" || target["receiver"].(map[string]any)["binding"] != narrowBinding || value["kind"] != "bindingReference" || value["binding"] != local["binding"] || value["type"] != "i32" || value["flowType"] != "i32" {
		t.Fatal("copy and subsequent statically permitted narrow field store must share the actual local binding")
	}
	result := body[5].(map[string]any)["value"].(map[string]any)
	for _, read := range []map[string]any{copyRead, result} {
		if read["kind"] != "fieldAccess" || read["property"] != narrowField.ID || read["type"] != "i32" || read["flowType"] != "i32" || read["receiver"].(map[string]any)["binding"] != narrowBinding {
			t.Fatal("both reads must preserve the original narrow alias and official i32 static facts")
		}
	}
	previous := -1
	for _, occurrence := range []struct {
		value map[string]any
		text  string
	}{{wideStore, "wide.n=undefined"}, {copyRead, "narrow.n"}, {store, "narrow.n=copied"}, {result, "narrow.n"}} {
		r := occurrence.value["range"].(map[string]any)
		start, end := int(r["start"].(float64)), int(r["end"].(float64))
		if start <= previous || source[start:end] != occurrence.text {
			t.Fatal("original alias write/copy/store/read occurrence order or anchor changed")
		}
		previous = start
	}
}
