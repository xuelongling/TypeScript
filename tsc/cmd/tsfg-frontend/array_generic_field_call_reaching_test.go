// SPDX-License-Identifier: Apache-2.0
package main

import "testing"

func TestArrayGenericV8FieldReadAcrossCall25f(t *testing.T) {
	source := "class Box{n:i32|undefined=undefined;} function clear(b:Box):void{b.n=undefined;} export function captured(b:Box):i32{b.n=1;const before:i32=b.n;clear(b);return before;} export function reread(b:Box):i32|undefined{b.n=1;clear(b);return b.n;}"
	p := genericArrayPublic25f(t, source)
	if len(p["diagnostics"].([]any)) != 0 || len(p["functions"].([]any)) != 3 || len(p["exports"].([]any)) != 2 {
		t.Fatal("official checked whole function closure and both exports must remain")
	}
	box := genericConstructionTypes25f(t, p)["Box"]
	field := (*box.Fields)[0]
	if field.Type != "union<i32,undefined>" || field.Readonly || field.Initializer.Kind != "undefinedLiteral" {
		t.Fatal("original mutable presence field changed")
	}
	clear := genericFindFunction25f(t, p, "clear")
	clearParam := clear["parameters"].([]any)[0].(map[string]any)
	clearBody := clear["body"].([]any)
	if clear["returnType"] != "void" || len(clearBody) != 1 {
		t.Fatal("original direct clear callee changed")
	}
	clearStore := clearBody[0].(map[string]any)["value"].(map[string]any)
	clearTarget := clearStore["target"].(map[string]any)
	if clearStore["kind"] != "fieldAssignment" || clearStore["flowType"] != "undefined" || clearTarget["property"] != field.ID || clearTarget["receiver"].(map[string]any)["binding"] != clearParam["id"] || clearStore["value"].(map[string]any)["kind"] != "undefinedLiteral" {
		t.Fatal("clear must retain the original same-receiver valid-undefined store")
	}
	anchor := func(value map[string]any, text string) int {
		t.Helper()
		r := value["range"].(map[string]any)
		start, end := int(r["start"].(float64)), int(r["end"].(float64))
		if source[start:end] != text {
			t.Fatal("original field/call/return source anchor changed")
		}
		return start
	}
	anchor(clearStore, "b.n=undefined")
	for _, name := range []string{"captured", "reread"} {
		fn := genericFindFunction25f(t, p, name)
		param := fn["parameters"].([]any)[0].(map[string]any)
		body := fn["body"].([]any)
		callIndex := 1
		if name == "captured" {
			callIndex = 2
		}
		if len(body) != callIndex+2 {
			t.Fatal("original store/read/call/return statement order changed")
		}
		store := body[0].(map[string]any)["value"].(map[string]any)
		target := store["target"].(map[string]any)
		if store["kind"] != "fieldAssignment" || target["property"] != field.ID || target["receiver"].(map[string]any)["binding"] != param["id"] || store["value"].(map[string]any)["decimal"] != "1" {
			t.Fatal("initial numeric field store changed")
		}
		call := body[callIndex].(map[string]any)["value"].(map[string]any)
		args := call["arguments"].([]any)
		if call["kind"] != "call" || call["callee"] != clear["id"] || call["type"] != "void" || call["flowType"] != "void" || len(args) != 1 || args[0].(map[string]any)["binding"] != param["id"] {
			t.Fatal("original direct callee and same-receiver argument changed")
		}
		result := body[callIndex+1].(map[string]any)["value"].(map[string]any)
		read := result
		if name == "captured" {
			local := body[1].(map[string]any)
			read = local["initializer"].(map[string]any)
			if result["kind"] != "bindingReference" || result["binding"] != local["binding"] || result["flowType"] != "i32" || !(anchor(read, "b.n") < anchor(call, "clear(b)") && anchor(call, "clear(b)") < anchor(result, "before")) {
				t.Fatal("return must retain the pre-call captured local rather than reload the field")
			}
		} else if fn["returnType"] != field.Type || anchor(call, "clear(b)") >= anchor(read, "b.n") {
			t.Fatal("reread must retain its original post-call field access and return annotation")
		}
		// The official checker retains this narrowing across the call. It is
		// not proof of the actual field version after the callee's mutation.
		if read["kind"] != "fieldAccess" || read["property"] != field.ID || read["type"] != field.Type || read["flowType"] != "i32" || read["receiver"].(map[string]any)["binding"] != param["id"] {
			t.Fatal("declared field type and original checker flow facts changed")
		}
		if anchor(store, "b.n=1") >= anchor(call, "clear(b)") {
			t.Fatal("call must remain after the original numeric store")
		}
	}
}
