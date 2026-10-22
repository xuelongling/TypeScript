// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"testing"
)

func TestArrayGenericV8ReturnedParameterFieldAlias25f(t *testing.T) {
	source := "class Box{n:i32|undefined=undefined;} function same(b:Box):Box{return b;} export function run(b:Box):i32|undefined{const alias=same(b);alias.n=1;return b.n;}"
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
	same, run := functions["same"], functions["run"]
	if same.ReturnType != box.ID || len(same.Parameters) != 1 || len(same.Body) != 1 || run.ReturnType != field.Type || len(run.Parameters) != 1 || len(run.Bindings) != 1 || len(run.Body) != 3 {
		t.Fatal("original identity function and caller signature/body changed")
	}
	sameParam, runParam, alias := same.Parameters[0], run.Parameters[0], run.Bindings[0]
	returned := same.Body[0].Value
	if sameParam.ID == runParam.ID || sameParam.DeclaredType != box.ID || runParam.DeclaredType != box.ID || alias.Name != "alias" || alias.DeclaredType != box.ID || alias.AnnotationRange != nil || same.Body[0].Kind != "return" || returned == nil || returned.Kind != "bindingReference" || returned.Binding != sameParam.ID || returned.Type != box.ID || returned.FlowType != box.ID {
		t.Fatal("identity return must reference its own original parameter; inferred alias type must remain Box")
	}
	call, store, read := run.Body[0].Initializer, run.Body[1].Value, run.Body[2].Value
	if run.Body[0].Kind != "constDeclaration" || run.Body[0].Binding != alias.ID || call == nil || call.Kind != "call" || call.Callee != same.ID || call.Type != box.ID || call.FlowType != box.ID || call.Arguments == nil || len(*call.Arguments) != 1 || (*call.Arguments)[0].Kind != "bindingReference" || (*call.Arguments)[0].Binding != runParam.ID || (*call.Arguments)[0].Type != box.ID || call.Signature == nil || call.Signature.SourcePath != same.SourcePath || call.Signature.DeclarationRange != same.DeclarationRange || call.Signature.ReturnType != box.ID || len(call.Signature.Parameters) != 1 || call.Signature.Parameters[0] != box.ID {
		t.Fatal("actual call must join the original same declaration/signature and caller parameter argument")
	}
	if store == nil || store.Kind != "fieldAssignment" || store.Target == nil || store.Target.Kind != "fieldTarget" || store.Target.Property != field.ID || store.Target.Type != field.Type || store.Target.Receiver.Binding != alias.ID || store.Target.Receiver.Type != box.ID || store.Value.Kind != "numericLiteral" || store.Value.Type != "i32" || store.Value.Decimal != "1" || store.FlowType != "i32" {
		t.Fatal("field write must use the returned call-result alias and original mutable property")
	}
	// The checker does not narrow the original parameter field through this alias.
	if run.Body[2].Kind != "return" || read == nil || read.Kind != "fieldAccess" || read.Property != field.ID || read.Type != field.Type || read.FlowType != field.Type || read.Receiver.Binding != runParam.ID || read.Receiver.Type != box.ID {
		t.Fatal("original parameter read must preserve the real union field type and checker flow")
	}
	for _, occurrence := range []struct {
		rangeValue span
		text       string
	}{{same.DeclarationRange, "function same(b:Box):Box{return b;}"}, {sameParam.DeclarationRange, "b:Box"}, {runParam.DeclarationRange, "b:Box"}, {returned.Range, "b"}, {*call.CalleeRange, "same"}, {call.Range, "same(b)"}, {store.Range, "alias.n=1"}, {read.Range, "b.n"}, {*store.Target.NameRange, "n"}, {*read.NameRange, "n"}} {
		if source[occurrence.rangeValue.Start:occurrence.rangeValue.End] != occurrence.text {
			t.Fatal("original caller/callee/parameter/field UTF16 anchor changed")
		}
	}
	if !(returned.Range.Start < call.Range.Start && call.Range.Start < store.Range.Start && store.Range.Start < read.Range.Start) {
		t.Fatal("original identity return then caller call/store/read sequence changed")
	}
}
