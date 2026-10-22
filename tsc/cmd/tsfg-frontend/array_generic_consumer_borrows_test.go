// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestArrayGenericV8ConsumerBorrowOriginalSources25f(t *testing.T) {
	for _, row := range []struct{ name, source string }{
		{"union-view-return", "function view(a:(i32|undefined)[]):ReadSpan<i32|undefined>{return readSpan(a,0,1);} export function read(a:(i32|undefined)[],i:u32):i32|undefined{const s:ReadSpan<i32|undefined>=view(a);return s[i];}"},
		{"union-active-structural-write", "function view(a:(i32|undefined)[]):ReadSpan<i32|undefined>{return readSpan(a,0,1);} export function read(a:(i32|undefined)[],i:u32):i32|undefined{const s:ReadSpan<i32|undefined>=view(a);a[1]=undefined;return s[i];}"},
		{"box-view-return", "class Box{n:i32=1;} function view(a:Box[]):ReadSpan<Box>{return readSpan(a,0,1);} export function read(a:Box[],i:u32):Box{const s:ReadSpan<Box>=view(a);return s[i];}"},
	} {
		t.Run(row.name, func(t *testing.T) {
			input := genericArrayRequest25f(t, row.source)
			// Preserve failed inputs without changing the frozen ten-input batch.
			if root := os.Getenv("TSFG_ARRAY_V8_CAPTURE_ROOT"); root != "" {
				if err := os.MkdirAll(root, 0700); err != nil {
					t.Fatal(err)
				}
				for name, data := range map[string][]byte{"source.ts": []byte(row.source), "request.json": input} {
					if err := os.WriteFile(filepath.Join(root, arrayFactsSHA([]byte(row.source))+"-"+name), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			p := genericArrayPublicRequest25f(t, input, row.source)
			raw, err := json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			var payload arrayGenericPayload
			if err := json.Unmarshal(raw, &payload); err != nil {
				t.Fatal(err)
			}
			if len(payload.Diagnostics) != 0 || len(payload.Functions) != 2 || len(payload.Exports) != 1 {
				t.Fatal("officially legal source must retain complete functions/export with zero diagnostics")
			}
			element := "union<i32,undefined>"
			if row.name == "box-view-return" {
				box := genericConstructionTypes25f(t, p)["Box"]
				element = box.ID
				if box.Kind != "reference" || len(*box.Fields) != 1 || (*box.Fields)[0].Type != "i32" {
					t.Fatal("original nominal Box element identity and field representation changed")
				}
			}
			types := map[string]arrayGenericType{}
			for _, typ := range payload.Types {
				types[typ.ID] = typ
			}
			arrayType, viewType := "array<"+element+">", "readSpan<"+element+">"
			if types[arrayType].ElementType != element || types[viewType].ElementType != element {
				t.Fatal("Array and ReadSpan must preserve their full original element type")
			}
			functions := map[string]arrayGenericFunction{}
			for _, fn := range payload.Functions {
				functions[fn.Name] = fn
			}
			view, read := functions["view"], functions["read"]
			if view.ReturnType != viewType || view.Parameters[0].DeclaredType != arrayType || len(view.Body) != 1 || read.ReturnType != element || read.Parameters[0].DeclaredType != arrayType {
				t.Fatal("original helper/caller signature element types changed")
			}
			intrinsic, call, result := view.Body[0].Value, read.Body[0].Initializer, read.Body[len(read.Body)-1].Value
			if intrinsic.Kind != "call" || intrinsic.Callee != payload.ReadSpanIntrinsic.ID || intrinsic.Type != viewType || (*intrinsic.Arguments)[0].Binding != view.Parameters[0].ID || call.Kind != "call" || call.Callee != view.ID || call.Type != viewType || (*call.Arguments)[0].Binding != read.Parameters[0].ID || result.Kind != "checkedElementAccess" || result.Type != element || result.FlowType != element || result.Receiver.Binding != read.Body[0].Binding {
				t.Fatal("direct intrinsic return, original call/parameter binding or checked view read changed")
			}
			if row.name == "union-active-structural-write" {
				store := read.Body[1].Value
				if len(read.Body) != 3 || store.Kind != "elementAssignment" || store.Target.Type != element || store.Target.Receiver.Binding != read.Parameters[0].ID || store.Target.Index.Decimal != "1" || store.Value.Kind != "undefinedLiteral" || !(call.Range.End < store.Range.Start && store.Range.End < result.Range.Start) {
					t.Fatal("legal Source write must remain between view creation and last read; Product owns the lifetime decision")
				}
			} else if len(read.Body) != 2 {
				t.Fatal("original caller view/read order changed")
			}
		})
	}
}
