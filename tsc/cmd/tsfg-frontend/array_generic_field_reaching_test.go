// SPDX-License-Identifier: Apache-2.0
package main

import "testing"

func TestArrayGenericV8FieldReadBeforeLaterStore25f(t *testing.T) {
	source := "class Box{n:i32|undefined=undefined;} export function f(b:Box):i32{b.n=1;const after:i32=b.n;b.n=undefined;return after;}"
	p := genericArrayPublic25f(t, source)
	box := genericConstructionTypes25f(t, p)["Box"]
	field := (*box.Fields)[0]
	if field.Type != "union<i32,undefined>" || field.Readonly || field.Initializer.Kind != "undefinedLiteral" {
		t.Fatal("original mutable presence field and initializer changed")
	}
	fn := genericFindFunction25f(t, p, "f")
	param := fn["parameters"].([]any)[0].(map[string]any)
	body := fn["body"].([]any)
	if len(body) != 4 {
		t.Fatal("original store/read/store/return order changed")
	}
	first := body[0].(map[string]any)["value"].(map[string]any)
	local := body[1].(map[string]any)
	read := local["initializer"].(map[string]any)
	later := body[2].(map[string]any)["value"].(map[string]any)
	for _, store := range []map[string]any{first, later} {
		target := store["target"].(map[string]any)
		if store["kind"] != "fieldAssignment" || target["kind"] != "fieldTarget" || target["property"] != field.ID || target["type"] != field.Type || target["receiver"].(map[string]any)["binding"] != param["id"] {
			t.Fatal("both writes must retain their original receiver, field and declared slot")
		}
	}
	if first["flowType"] != "i32" || first["value"].(map[string]any)["decimal"] != "1" || later["flowType"] != "undefined" || later["value"].(map[string]any)["kind"] != "undefinedLiteral" {
		t.Fatal("separate numeric and valid-undefined write results changed")
	}
	if read["kind"] != "fieldAccess" || read["property"] != field.ID || read["type"] != field.Type || read["flowType"] != "i32" || read["receiver"].(map[string]any)["binding"] != param["id"] {
		t.Fatal("the earlier read must retain its declared union and original checker narrowing")
	}
	result := genericReturn25f(t, p, "f")
	if result["kind"] != "bindingReference" || result["binding"] != local["binding"] || result["type"] != "i32" || result["flowType"] != "i32" {
		t.Fatal("return must reference the captured local, not reload the later field value")
	}
	for _, occurrence := range []struct {
		value map[string]any
		text  string
	}{{first, "b.n=1"}, {read, "b.n"}, {later, "b.n=undefined"}, {result, "after"}} {
		r := occurrence.value["range"].(map[string]any)
		if source[int(r["start"].(float64)):int(r["end"].(float64))] != occurrence.text {
			t.Fatal("original field reaching occurrence anchor changed")
		}
	}
}
