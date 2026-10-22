// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestArrayGenericV8ConsumerBorrowSemanticEdges25f(t *testing.T) {
	raw, err := os.ReadFile("testdata/array-facts-v8/consumer-borrow-edges-inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct{ Name, Source string }
	}
	if err := json.Unmarshal(raw, &fixture); err != nil || len(fixture.Cases) != 5 {
		t.Fatalf("five exact authorized sources required: %v", err)
	}
	for _, row := range fixture.Cases {
		t.Run(row.Name, func(t *testing.T) {
			input := genericArrayRequest25f(t, row.Source)
			// This independent batch preserves failed inputs without changing the
			// already qualified thirteen-input regression files or capture helpers.
			if root := os.Getenv("TSFG_ARRAY_V8_CAPTURE_ROOT"); root != "" {
				if err := os.MkdirAll(root, 0700); err != nil {
					t.Fatal(err)
				}
				for name, data := range map[string][]byte{"source.ts": []byte(row.Source), "request.json": input} {
					if err := os.WriteFile(filepath.Join(root, arrayFactsSHA([]byte(row.Source))+"-"+name), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			p := genericArrayPublicRequest25f(t, input, row.Source)
			encoded, err := json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			var payload arrayGenericPayload
			if err := json.Unmarshal(encoded, &payload); err != nil {
				t.Fatal(err)
			}
			if len(payload.Diagnostics) != 0 || len(payload.Exports) != 1 {
				t.Fatal("actual Source must retain a legal complete export with zero diagnostics")
			}
			functions := map[string]arrayGenericFunction{}
			for _, fn := range payload.Functions {
				functions[fn.Name] = fn
			}
			read := functions["read"]
			body := read.Body
			element := "union<i32,undefined>"
			if row.Name == "value-view-return" {
				pair := genericConstructionTypes25f(t, p)["Pair"]
				element = pair.ID
				if pair.Kind != "value" || pair.MarkerOrigin == nil || (*pair.Fields)[0].Type != "i32" {
					t.Fatal("real tsfgValue Pair identity/marker/field type must remain nominal value facts")
				}
			}
			if read.ReturnType != element {
				t.Fatal("complete original result element type changed")
			}
			last := body[len(body)-1].Value
			switch row.Name {
			case "fixed-call-upper":
				touch, call := functions["touch"], body[1].Value
				if len(body) != 3 || len(touch.Body) != 1 || touch.Body[0].Value.Decimal != "0" || call.Kind != "call" || call.Callee != touch.ID || call.Signature.DeclarationRange != touch.DeclarationRange || (*call.Arguments)[0].Binding != read.Parameters[0].ID || !(body[0].Initializer.Range.End < call.Range.Start && call.Range.End < last.Range.Start) {
					t.Fatal("no-op helper body must not erase the actual ordered caller call and selected signature")
				}
			case "after-last-use":
				captured, store := body[1].Initializer, body[2].Value
				if len(body) != 4 || captured.Kind != "checkedElementAccess" || captured.Receiver.Binding != body[0].Binding || store.Kind != "elementAssignment" || store.Target.Receiver.Binding != read.Parameters[0].ID || store.Target.Index.Decimal != "1" || store.Value.Kind != "undefinedLiteral" || last.Kind != "bindingReference" || last.Binding != body[1].Binding || !(captured.Range.End < store.Range.Start && store.Range.End < last.Range.Start) {
					t.Fatal("view last read, later structural write and captured local return order changed")
				}
			case "view-parameter-alias":
				identity, call, store := functions["identity"], body[1].Initializer, body[2].Value
				if len(body) != 4 || identity.ReturnType != "readSpan<"+element+">" || identity.Body[0].Value.Kind != "bindingReference" || identity.Body[0].Value.Binding != identity.Parameters[0].ID || call.Callee != identity.ID || (*call.Arguments)[0].Binding != body[0].Binding || store.Kind != "elementAssignment" || store.Value.Kind != "undefinedLiteral" || last.Kind != "checkedElementAccess" || last.Receiver.Binding != body[1].Binding || !(call.Range.End < store.Range.Start && store.Range.End < last.Range.Start) {
					t.Fatal("identity must preserve its own view parameter and actual alias/store/read sequence")
				}
			case "serial-overwrite":
				allocation, creation, store := body[0].Initializer, body[1].Initializer, body[2].Value
				if len(body) != 4 || allocation.Kind != "arrayLiteral" || allocation.Type != "array<"+element+">" || len(*allocation.Elements) != 1 || creation.Callee != payload.ReadSpanIntrinsic.ID || (*creation.Arguments)[0].Binding != body[0].Binding || store.Kind != "elementAssignment" || store.Target.Receiver.Binding != body[0].Binding || store.Target.Index.Decimal != "0" || store.Target.Type != element || store.Value.Kind != "undefinedLiteral" || last.Kind != "checkedElementAccess" || last.Receiver.Binding != body[1].Binding || last.Index.Decimal != "0" {
					t.Fatal("one-element allocation, existing-index valid undefined store and checked view read changed")
				}
			case "value-view-return":
				view, call := functions["view"], body[0].Initializer
				if len(body) != 2 || view.Parameters[0].DeclaredType != "array<"+element+">" || view.ReturnType != "readSpan<"+element+">" || view.Body[0].Value.Callee != payload.ReadSpanIntrinsic.ID || call.Callee != view.ID || call.Type != view.ReturnType || last.Kind != "checkedElementAccess" || last.Type != element || last.Receiver.Binding != body[0].Binding {
					t.Fatal("Array-origin view return and checked read must retain the same nominal value element")
				}
			default:
				t.Fatal("unknown exact input")
			}
			// Source preserves these actual operations; fixed publication and borrow
			// lifetime decisions belong to Product, not checker flow narrowing.
			if last.Type != element || last.FlowType != element {
				t.Fatal("original checked or captured result type/flow changed")
			}
		})
	}
}
