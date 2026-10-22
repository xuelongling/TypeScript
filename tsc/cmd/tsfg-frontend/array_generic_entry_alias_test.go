// SPDX-License-Identifier: Apache-2.0
package main

import "testing"

func TestArrayGenericV8EntryArrayAlias25f(t *testing.T) {
	t.Run("compatible-parameters", func(t *testing.T) {
		source := "class Base{n:i32=1;} class Derived extends Base{extra:i32=2;} export function shared(a:Derived[],b:Base[]):Base{b[0]=new Base();return a[0];}"
		p := genericArrayPublic25f(t, source)
		types := genericConstructionTypes25f(t, p)
		base, derived := types["Base"].ID, types["Derived"].ID
		fn := genericFindFunction25f(t, p, "shared")
		params := fn["parameters"].([]any)
		a, b := params[0].(map[string]any), params[1].(map[string]any)
		if len(p["functions"].([]any)) != 1 || a["declaredType"] != "array<"+derived+">" || b["declaredType"] != "array<"+base+">" || a["id"] == b["id"] {
			t.Fatal("original export parameters or independent caller-free closure changed")
		}
		store := fn["body"].([]any)[0].(map[string]any)["value"].(map[string]any)
		target := store["target"].(map[string]any)
		value := store["value"].(map[string]any)
		if store["kind"] != "elementAssignment" || target["type"] != base || target["receiver"].(map[string]any)["binding"] != b["id"] || target["index"].(map[string]any)["decimal"] != "0" || value["kind"] != "new" || value["callee"] != base {
			t.Fatal("wider parameter store must retain its original target and allocation")
		}
		read := genericReturn25f(t, p, "shared")
		if read["kind"] != "checkedElementAccess" || read["type"] != derived || read["receiver"].(map[string]any)["binding"] != a["id"] || read["index"].(map[string]any)["decimal"] != "0" {
			t.Fatal("original narrow parameter checked read was replaced or retagged")
		}
		proof := genericRootProof25f(t, p, read)
		if proof["sourceType"] != derived || proof["targetType"] != base || len(proof["members"].([]any)) != 1 {
			t.Fatal("return contextual view requires the real Derived-to-Base member projection")
		}
		for _, occurrence := range []struct {
			value map[string]any
			text  string
		}{{store, "b[0]=new Base()"}, {read, "a[0]"}} {
			r := occurrence.value["range"].(map[string]any)
			if source[int(r["start"].(float64)):int(r["end"].(float64))] != occurrence.text {
				t.Fatal("original store/read anchor changed")
			}
		}
	})
	t.Run("distinct-native-elements", func(t *testing.T) {
		p := genericArrayPublic25f(t, "export function separated(a:i32[],b:u32[]):i32{a[0]+=1;return a[0];}")
		fn := genericFindFunction25f(t, p, "separated")
		params := fn["parameters"].([]any)
		a, b := params[0].(map[string]any), params[1].(map[string]any)
		if a["declaredType"] != "array<i32>" || b["declaredType"] != "array<u32>" || a["id"] == b["id"] || fn["returnType"] != "i32" {
			t.Fatal("unused u32 parameter and i32 return must retain distinct logical types")
		}
		update := fn["body"].([]any)[0].(map[string]any)["value"].(map[string]any)
		target := update["target"].(map[string]any)
		if update["kind"] != "elementCompoundAssignment" || update["operator"] != "+=" || update["flowType"] != "i32" || target["type"] != "i32" || target["receiver"].(map[string]any)["binding"] != a["id"] || update["value"].(map[string]any)["flowType"] != "i32" {
			t.Fatal("native i32 compound operation must not acquire a u32 conversion")
		}
		read := genericReturn25f(t, p, "separated")
		if read["kind"] != "checkedElementAccess" || read["flowType"] != "i32" || read["receiver"].(map[string]any)["binding"] != a["id"] {
			t.Fatal("native i32 checked return changed")
		}
	})
}
