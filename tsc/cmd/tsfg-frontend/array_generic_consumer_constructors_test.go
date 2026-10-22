// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestArrayGenericV8ConsumerConstructorActiveViews25f(t *testing.T) {
	raw, err := os.ReadFile("testdata/array-facts-v8/consumer-constructor-active-view-inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct{ Name, Source string }
	}
	if err := json.Unmarshal(raw, &fixture); err != nil || len(fixture.Cases) != 2 {
		t.Fatalf("two exact constructor sources required: %v", err)
	}
	for _, row := range fixture.Cases {
		t.Run(row.Name, func(t *testing.T) {
			input := genericArrayRequest25f(t, row.Source)
			// Keep this batch independent of the nine already staged paths, and
			// preserve original failed inputs without publishing an error as facts.
			capture := func(name string, data []byte) {
				t.Helper()
				if root := os.Getenv("TSFG_ARRAY_V8_CAPTURE_ROOT"); root != "" {
					if err := os.MkdirAll(root, 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(root, arrayFactsSHA([]byte(row.Source))+"-"+name), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			capture("source.ts", []byte(row.Source))
			capture("request.json", input)
			var output bytes.Buffer
			if err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(input), &output); err != nil {
				var failure *arrayFactsFailure
				errors.As(err, &failure)
				record, encodeErr := json.MarshalIndent(struct {
					Error   string             `json:"error"`
					Failure *arrayFactsFailure `json:"failure"`
				}{err.Error(), failure}, "", "  ")
				if encodeErr != nil {
					t.Fatal(encodeErr)
				}
				capture("error.json", record)
				capture("failed-output.bin", output.Bytes())
				t.Fatalf("actual original constructor Source rejection: %s", record)
			}
			genericArrayCapture25f(t, input, row.Source, output.Bytes())
			var packet arrayGenericPackage
			if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}
			payload := packet.Payload
			if packet.Version != 8 || len(payload.Diagnostics) != 0 || len(payload.Functions) != 1 || len(payload.Exports) != 1 {
				t.Fatal("actual checked constructor source must preserve the complete export with zero diagnostics")
			}
			p := map[string]any{}
			if err := json.Unmarshal(output.Bytes(), &p); err != nil {
				t.Fatal(err)
			}
			types := genericConstructionTypes25f(t, p["payload"].(map[string]any))
			base, target := types["C"], types["C"]
			if row.Name == "implicit-constructor-forward-active-view" {
				target = types["D"]
				genericForward25f(t, target, base)
			}
			constructor := base.Constructor
			if constructor == nil || base.Construction.Kind != "explicit" || len(constructor.Parameters) != 1 || constructor.Parameters[0].DeclaredType != "array<i32>" || len(constructor.Body) != 1 || base.Construction.Parameters[0].Symbol != constructor.Parameters[0].ID {
				t.Fatal("original explicit constructor and actual array parameter identity changed")
			}
			store := constructor.Body[0].Value
			if store.Kind != "lengthAssignment" || store.Target.Kind != "lengthTarget" || store.Target.Property != payload.ArrayLength.ID || store.Target.Receiver.Binding != constructor.Parameters[0].ID || store.Target.Receiver.Type != "array<i32>" || store.Value.Type != "u32" || store.Value.Decimal != "0" {
				t.Fatal("constructor must retain its original same-array length truncation")
			}
			read := payload.Functions[0]
			if read.Name != "read" || len(read.Body) != 3 || read.ReturnType != "i32" {
				t.Fatal("caller view/create/read order changed")
			}
			view, allocation, result := read.Body[0].Initializer, read.Body[1].Value, read.Body[2].Value
			if view.Callee != payload.ReadSpanIntrinsic.ID || (*view.Arguments)[0].Binding != read.Parameters[0].ID || allocation.Kind != "new" || allocation.Callee != target.ID || allocation.Type != target.ID || allocation.AllocationSite == "" || allocation.Signature.ID != target.Construction.Signature.ID || (*allocation.Arguments)[0].Binding != read.Parameters[0].ID || result.Kind != "checkedElementAccess" || result.Type != "i32" || result.Receiver.Binding != read.Body[0].Binding || !(view.Range.End < allocation.Range.Start && allocation.Range.End < result.Range.Start) {
				t.Fatal("constructor call must retain its selected construction/signature, argument and active-view read")
			}
			for _, occurrence := range []struct {
				value *arrayGenericExpression
				text  string
			}{{store, "a.length=0"}, {view, "readSpan(a,0,1)"}, {allocation, "new " + target.Name + "(a)"}, {result, "s[i]"}} {
				if row.Source[occurrence.value.Range.Start:occurrence.value.Range.End] != occurrence.text {
					t.Fatal("original constructor/view/store/read UTF16 anchor changed")
				}
			}
		})
	}
}
