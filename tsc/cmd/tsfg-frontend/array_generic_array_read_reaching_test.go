// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"testing"
)

func TestArrayGenericV8CapturedArrayReadBeforeWideWrite25f(t *testing.T) {
	source := "export function run():i32{const narrow:i32[]=[1];const before:i32=narrow[0];const wide:(i32|undefined)[]=narrow;wide[0]=undefined;return before;}"
	p := genericArrayPublic25f(t, source)
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var payload arrayGenericPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Diagnostics) != 0 || payload.PositionUnit != "utf16" || len(payload.Functions) != 1 || len(payload.Exports) != 1 {
		t.Fatal("original checked source closure and export must retain zero diagnostics")
	}
	types := map[string]arrayGenericType{}
	for _, typ := range payload.Types {
		types[typ.ID] = typ
	}
	if types["array<i32>"].Kind != "array" || types["array<i32>"].ElementType != "i32" || types["array<union<i32,undefined>>"].Kind != "array" || types["array<union<i32,undefined>>"].ElementType != "union<i32,undefined>" || types["undefined"].Kind != "undefined" || len(types["union<i32,undefined>"].Members) != 2 || types["union<i32,undefined>"].Members[0] != "i32" || types["union<i32,undefined>"].Members[1] != "undefined" {
		t.Fatal("original narrow/wide Array element types and valid absence union changed")
	}
	fn := payload.Functions[0]
	if fn.Name != "run" || fn.ReturnType != "i32" || len(fn.Body) != 5 || len(fn.Bindings) != 3 {
		t.Fatal("original allocation/read/view/write/return order changed")
	}
	bindings := map[string]arrayFactsBinding{}
	for _, binding := range fn.Bindings {
		bindings[binding.Name] = binding
	}
	narrow, before, wide := bindings["narrow"], bindings["before"], bindings["wide"]
	body := fn.Body
	allocation, read, view, store, result := body[0].Initializer, body[1].Initializer, body[2].Initializer, body[3].Value, body[4].Value
	if narrow.DeclaredType != "array<i32>" || before.DeclaredType != "i32" || before.Writable || wide.DeclaredType != "array<union<i32,undefined>>" || body[0].Binding != narrow.ID || body[1].Binding != before.ID || body[2].Binding != wide.ID || body[1].Kind != "constDeclaration" || allocation == nil || allocation.Kind != "arrayLiteral" || allocation.Type != narrow.DeclaredType || allocation.AllocationSite == "" || allocation.Elements == nil || len(*allocation.Elements) != 1 || (*allocation.Elements)[0].Type != "i32" || (*allocation.Elements)[0].Decimal != "1" {
		t.Fatal("original narrow allocation and captured immutable local binding changed")
	}
	if read == nil || read.Kind != "checkedElementAccess" || read.Type != "i32" || read.FlowType != "i32" || read.Receiver.Binding != narrow.ID || read.Receiver.Type != narrow.DeclaredType || read.Index.Type != "u32" || read.Index.Decimal != "0" || view == nil || view.Kind != "bindingReference" || view.Binding != narrow.ID || view.Type != narrow.DeclaredType || view.FlowType != narrow.DeclaredType || view.AllocationSite != "" {
		t.Fatal("pre-write read and subsequent wider view must retain the same original array binding")
	}
	if store == nil || store.Kind != "elementAssignment" || store.Target == nil || store.Target.Kind != "elementTarget" || store.Target.Type != "union<i32,undefined>" || store.Target.Receiver.Binding != wide.ID || store.Target.Receiver.Type != wide.DeclaredType || store.Target.Index.Type != "u32" || store.Target.Index.Decimal != "0" || store.Value.Kind != "undefinedLiteral" || store.Value.Type != "undefined" || store.FlowType != "undefined" {
		t.Fatal("later wider alias store must retain its valid undefined RHS and original index")
	}
	// Return the captured local value; a later alias store cannot turn it into a reload.
	if body[4].Kind != "return" || result == nil || result.Kind != "bindingReference" || result.Binding != before.ID || result.Type != "i32" || result.FlowType != "i32" {
		t.Fatal("return must reference the already captured local rather than the later array element")
	}
	previous := -1
	for _, occurrence := range []struct {
		value *arrayGenericExpression
		text  string
	}{{allocation, "[1]"}, {read, "narrow[0]"}, {view, "narrow"}, {store, "wide[0]=undefined"}, {result, "before"}} {
		if occurrence.value.Range.Start <= previous || source[occurrence.value.Range.Start:occurrence.value.Range.End] != occurrence.text {
			t.Fatal("original array read/view/write/captured-return UTF16 order or anchor changed")
		}
		previous = occurrence.value.Range.Start
	}
}
