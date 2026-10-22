// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"testing"
)

func TestArrayGenericV8NarrowArrayCopyAfterWideWrite25f(t *testing.T) {
	source := "export function run():i32{const narrow:i32[]=[1];const wide:(i32|undefined)[]=narrow;wide[0]=undefined;const copied:i32=narrow[0];narrow[0]=copied;return narrow[0];}"
	p := genericArrayPublic25f(t, source)
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var payload arrayGenericPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Diagnostics) != 0 || payload.PositionUnit != "utf16" || len(payload.Functions) != 1 {
		t.Fatal("actual checked source must retain zero diagnostics and UTF16 provenance")
	}
	types := map[string]arrayGenericType{}
	for _, typ := range payload.Types {
		types[typ.ID] = typ
	}
	narrowType, wideType := "array<i32>", "array<union<i32,undefined>>"
	for _, test := range []struct{ id, element string }{{narrowType, "i32"}, {wideType, "union<i32,undefined>"}} {
		typ := types[test.id]
		if typ.Kind != "array" || typ.ElementType != test.element || typ.SourcePath != "/src/minimal-arrays-v8.d.ts" || typ.DeclarationRange == nil || typ.NameRange == nil || typ.IndexSignatureRange == nil {
			t.Fatal("both concrete arrays must retain their actual Array owner and element relation")
		}
		if string(arrayGenericDeclarations[typ.NameRange.Start:typ.NameRange.End]) != "Array" || string(arrayGenericDeclarations[typ.IndexSignatureRange.Start:typ.IndexSignatureRange.End]) != "[n: number]: T;" {
			t.Fatal("original generic owner and index-signature declaration anchors changed")
		}
	}
	if types["undefined"].Kind != "undefined" || len(types["union<i32,undefined>"].Members) != 2 || types["union<i32,undefined>"].Members[0] != "i32" || types["union<i32,undefined>"].Members[1] != "undefined" {
		t.Fatal("valid undefined must remain a concrete member of the wider element union")
	}
	fn := payload.Functions[0]
	if fn.Name != "run" || fn.ReturnType != "i32" || len(fn.Body) != 6 || len(fn.Bindings) != 3 {
		t.Fatal("original allocation/view/write/copy/store/return order changed")
	}
	body := fn.Body
	bindings := map[string]arrayFactsBinding{}
	for _, binding := range fn.Bindings {
		bindings[binding.Name] = binding
	}
	if bindings["narrow"].DeclaredType != narrowType || bindings["wide"].DeclaredType != wideType || bindings["copied"].DeclaredType != "i32" || body[0].Binding != bindings["narrow"].ID || body[1].Binding != bindings["wide"].ID || body[3].Binding != bindings["copied"].ID {
		t.Fatal("original local declarations and binding identities changed")
	}
	allocation, view := body[0].Initializer, body[1].Initializer
	if allocation == nil || allocation.Kind != "arrayLiteral" || allocation.Type != narrowType || allocation.AllocationSite == "" || allocation.Elements == nil || len(*allocation.Elements) != 1 || (*allocation.Elements)[0].Type != "i32" || (*allocation.Elements)[0].Decimal != "1" || view == nil || view.Kind != "bindingReference" || view.Binding != bindings["narrow"].ID || view.Type != narrowType || view.FlowType != narrowType || view.AllocationSite != "" {
		t.Fatal("the wider view must alias the sole original narrow Array allocation")
	}
	wideStore, copyRead, narrowStore, result := body[2].Value, body[3].Initializer, body[4].Value, body[5].Value
	if wideStore == nil || wideStore.Kind != "elementAssignment" || wideStore.Target == nil || wideStore.Target.Kind != "elementTarget" || wideStore.Target.Type != "union<i32,undefined>" || wideStore.Target.Receiver.Binding != bindings["wide"].ID || wideStore.Target.Receiver.Type != wideType || wideStore.Value.Kind != "undefinedLiteral" || wideStore.Value.Type != "undefined" || wideStore.FlowType != "undefined" {
		t.Fatal("original writable wider array target and valid undefined RHS changed")
	}
	// These i32 facts are static read/store permissions, not an actual payload bound.
	for _, read := range []*arrayGenericExpression{copyRead, result} {
		if read == nil || read.Kind != "checkedElementAccess" || read.Type != "i32" || read.FlowType != "i32" || read.Receiver.Binding != bindings["narrow"].ID || read.Receiver.Type != narrowType || read.Index.Type != "u32" || read.Index.Decimal != "0" {
			t.Fatal("both checked reads must retain the original narrow alias and official i32 facts")
		}
	}
	if body[3].Kind != "constDeclaration" || body[5].Kind != "return" || narrowStore == nil || narrowStore.Kind != "elementAssignment" || narrowStore.Target == nil || narrowStore.Target.Kind != "elementTarget" || narrowStore.Target.Type != "i32" || narrowStore.Target.Receiver.Binding != bindings["narrow"].ID || narrowStore.Target.Receiver.Type != narrowType || narrowStore.Value.Kind != "bindingReference" || narrowStore.Value.Binding != bindings["copied"].ID || narrowStore.Value.Type != "i32" || narrowStore.Value.FlowType != "i32" {
		t.Fatal("statically permitted copy and narrow store must preserve the same local value binding")
	}
	for _, target := range []*arrayGenericTarget{wideStore.Target, narrowStore.Target} {
		if target.Index == nil || target.Index.Type != "u32" || target.Index.Decimal != "0" {
			t.Fatal("original checked array targets must preserve their exact index")
		}
	}
	previous := -1
	for _, occurrence := range []struct {
		value *arrayGenericExpression
		text  string
	}{{allocation, "[1]"}, {view, "narrow"}, {wideStore, "wide[0]=undefined"}, {copyRead, "narrow[0]"}, {narrowStore, "narrow[0]=copied"}, {result, "narrow[0]"}} {
		if occurrence.value.Range.Start <= previous || source[occurrence.value.Range.Start:occurrence.value.Range.End] != occurrence.text {
			t.Fatal("original alias/write/copy/store/read UTF16 occurrence order or anchor changed")
		}
		previous = occurrence.value.Range.Start
	}
}
