// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"testing"
)

func TestArrayGenericV8CallerWriteBeforeCalleeRead25f(t *testing.T) {
	source := "class Box{n:i32|undefined=undefined;} function read(b:Box):i32|undefined{return b.n;} export function run(b:Box):i32|undefined{b.n=1;return read(b);}"
	p := genericArrayPublic25f(t, source)
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var payload arrayGenericPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Diagnostics) != 0 || payload.PositionUnit != "utf16" || len(payload.Functions) != 2 || len(payload.Exports) != 1 {
		t.Fatal("original checked caller/callee closure and export must retain zero diagnostics")
	}
	box := genericConstructionTypes25f(t, p)["Box"]
	field := (*box.Fields)[0]
	if field.Type != "union<i32,undefined>" || field.Readonly || field.Initializer.Kind != "undefinedLiteral" {
		t.Fatal("original mutable presence field changed")
	}
	functions := map[string]arrayGenericFunction{}
	for _, fn := range payload.Functions {
		functions[fn.Name] = fn
	}
	read, run := functions["read"], functions["run"]
	if read.ReturnType != field.Type || run.ReturnType != field.Type || len(read.Parameters) != 1 || len(run.Parameters) != 1 || len(read.Body) != 1 || len(run.Body) != 2 {
		t.Fatal("original internal read and caller signatures/body order changed")
	}
	readParam, runParam := read.Parameters[0], run.Parameters[0]
	fieldRead, store, call := read.Body[0].Value, run.Body[0].Value, run.Body[1].Value
	// Internal entry memory comes from the caller; official flow remains union.
	if readParam.ID == runParam.ID || readParam.DeclaredType != box.ID || runParam.DeclaredType != box.ID || read.Body[0].Kind != "return" || fieldRead == nil || fieldRead.Kind != "fieldAccess" || fieldRead.Property != field.ID || fieldRead.Type != field.Type || fieldRead.FlowType != field.Type || fieldRead.Receiver.Binding != readParam.ID || fieldRead.Receiver.Type != box.ID {
		t.Fatal("callee field read must retain its own original parameter and official union type/flow")
	}
	if store == nil || store.Kind != "fieldAssignment" || store.Target == nil || store.Target.Kind != "fieldTarget" || store.Target.Property != field.ID || store.Target.Type != field.Type || store.Target.Receiver.Binding != runParam.ID || store.Target.Receiver.Type != box.ID || store.Value.Kind != "numericLiteral" || store.Value.Type != "i32" || store.Value.Decimal != "1" || store.FlowType != "i32" {
		t.Fatal("caller must write the same original Box field through its own parameter before calling")
	}
	if run.Body[1].Kind != "return" || call == nil || call.Kind != "call" || call.Callee != read.ID || call.Type != field.Type || call.FlowType != field.Type || call.Arguments == nil || len(*call.Arguments) != 1 || (*call.Arguments)[0].Kind != "bindingReference" || (*call.Arguments)[0].Binding != runParam.ID || (*call.Arguments)[0].Type != box.ID || call.Signature == nil || call.Signature.SourcePath != read.SourcePath || call.Signature.DeclarationRange != read.DeclarationRange || call.Signature.ReturnType != field.Type || len(call.Signature.Parameters) != 1 || call.Signature.Parameters[0] != box.ID {
		t.Fatal("actual call must join the selected read declaration/signature and same caller parameter argument")
	}
	for _, occurrence := range []struct {
		rangeValue span
		text       string
	}{{read.DeclarationRange, "function read(b:Box):i32|undefined{return b.n;}"}, {readParam.DeclarationRange, "b:Box"}, {runParam.DeclarationRange, "b:Box"}, {fieldRead.Range, "b.n"}, {*fieldRead.NameRange, "n"}, {store.Range, "b.n=1"}, {*store.Target.NameRange, "n"}, {*call.CalleeRange, "read"}, {call.Range, "read(b)"}, {(*call.Arguments)[0].Range, "b"}} {
		if source[occurrence.rangeValue.Start:occurrence.rangeValue.End] != occurrence.text {
			t.Fatal("original caller/callee/parameter/field UTF16 anchor changed")
		}
	}
	if store.Range.End >= call.Range.Start {
		t.Fatal("original caller field write must precede the internal read call")
	}
}
