// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"testing"
)

func TestArrayGenericV8EarlyVirtualFieldRead25f(t *testing.T) {
	source := "class Base{constructor(){this.observe();} observe():i32|undefined{return undefined;}} class Derived extends Base{n:i32=1; observe():i32|undefined{return this.n;}} export function make():Base{return new Derived();}"
	p := genericArrayPublic25f(t, source)
	types := genericConstructionTypes25f(t, p)
	base, derived := types["Base"], types["Derived"]
	if base.Constructor == nil || derived.Base != base.ID || len(*base.Fields) != 0 || len(*derived.Fields) != 1 {
		t.Fatal("original base constructor and derived own field closure changed")
	}
	genericForward25f(t, derived, base)
	field := (*derived.Fields)[0]
	if field.Name != "n" || field.Type != "i32" || field.Initializer == nil || field.Initializer.Kind != "numericLiteral" || field.Initializer.Decimal != "1" ||
		derived.Construction.InitializationPhase != "after-successful-super" || len(derived.Construction.InitializationFields) != 1 || derived.Construction.InitializationFields[0] != field.ID {
		t.Fatal("derived own initializer must remain after successful base construction")
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var payload arrayGenericPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	method := func(owner string) arrayGenericFunction {
		for _, fn := range payload.Functions {
			if fn.OwnerType == owner && fn.Name == "observe" {
				return fn
			}
		}
		t.Fatal("missing actual observe declaration for owner", owner)
		return arrayGenericFunction{}
	}
	baseMethod, derivedMethod := method(base.ID), method(derived.ID)
	if baseMethod.OverrideTarget != "" || derivedMethod.OverrideTarget != baseMethod.ID || baseMethod.ReturnType != "union<i32,undefined>" || derivedMethod.ReturnType != baseMethod.ReturnType ||
		len(baseMethod.Body) != 1 || baseMethod.Body[0].Value.Kind != "undefinedLiteral" || len(derivedMethod.Body) != 1 {
		t.Fatal("actual virtual override and both method bodies must remain in the closure")
	}
	if len(base.Constructor.Body) != 1 {
		t.Fatal("base constructor call order changed")
	}
	call := base.Constructor.Body[0].Value
	if call == nil || call.Kind != "call" || call.Callee != baseMethod.ID || call.Signature == nil || call.Signature.DeclarationRange != baseMethod.DeclarationRange ||
		call.Receiver == nil || call.Receiver.Kind != "thisReference" || call.Receiver.Type != base.ID || call.Arguments == nil || len(*call.Arguments) != 0 {
		t.Fatal("base constructor must retain its original virtual call and current receiver")
	}
	read := derivedMethod.Body[0].Value
	if read == nil || read.Kind != "fieldAccess" || read.Property != field.ID || read.Type != "i32" || read.FlowType != "i32" ||
		read.Receiver == nil || read.Receiver.Kind != "thisReference" || read.Receiver.Type != derived.ID {
		t.Fatal("override must retain the real checked derived field read, without fabricating its runtime value")
	}
	allocation := genericReturn25f(t, p, "make")
	if allocation["kind"] != "new" || allocation["callee"] != derived.ID || allocation["allocationSite"] == "" {
		t.Fatal("export must retain its real derived allocation")
	}
	for _, occurrence := range []struct {
		value *arrayGenericExpression
		text  string
	}{{call, "this.observe()"}, {read, "this.n"}, {field.Initializer, "1"}} {
		if source[occurrence.value.Range.Start:occurrence.value.Range.End] != occurrence.text {
			t.Fatal("original constructor/call/read/initializer source anchor changed")
		}
	}
}
