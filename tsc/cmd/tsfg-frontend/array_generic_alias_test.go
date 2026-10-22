// SPDX-License-Identifier: Apache-2.0
package main

import "testing"

func TestArrayGenericV8WritableCovariantAlias25f(t *testing.T) {
	prefix := "class Base{n:i32=1;} class Derived extends Base{extra:i32=2;} "
	for _, test := range []struct{ name, helpers, alias, store string }{
		{"direct", "", "narrow", "wide[0]=replacement;"},
		{"parameter-return", "function pass(a:Base[]):Base[]{return a;} function replace(a:Base[],v:Base):void{a[0]=v;} ", "pass(narrow)", "replace(wide,replacement);"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := prefix + test.helpers + "export function alias():bool{const narrow:Derived[]=[new Derived()];const wide:Base[]=" + test.alias + ";const replacement:Base=new Base();" + test.store + "const observed:Base=narrow[0];return observed===replacement;}"
			p := genericArrayPublic25f(t, source)
			types := genericConstructionTypes25f(t, p)
			base, derived := types["Base"].ID, types["Derived"].ID
			fn := genericFindFunction25f(t, p, "alias")
			bindings := map[string]map[string]any{}
			for _, value := range fn["bindings"].([]any) {
				binding := value.(map[string]any)
				bindings[binding["name"].(string)] = binding
			}
			if bindings["narrow"]["declaredType"] != "array<"+derived+">" || bindings["wide"]["declaredType"] != "array<"+base+">" || bindings["observed"]["declaredType"] != base {
				t.Fatal("original covariant alias and read-view annotations changed")
			}
			body := fn["body"].([]any)
			allocation := body[0].(map[string]any)["initializer"].(map[string]any)
			alias := body[1].(map[string]any)["initializer"].(map[string]any)
			aliasSource := alias
			store := body[3].(map[string]any)["value"].(map[string]any)
			if test.name == "parameter-return" {
				if alias["kind"] != "call" || alias["callee"] != genericFindFunction25f(t, p, "pass")["id"] || store["kind"] != "call" || store["callee"] != genericFindFunction25f(t, p, "replace")["id"] {
					t.Fatal("alias call signatures and source order lost")
				}
				aliasSource = alias["arguments"].([]any)[0].(map[string]any)
				args := store["arguments"].([]any)
				if args[0].(map[string]any)["binding"] != bindings["wide"]["id"] || args[1].(map[string]any)["binding"] != bindings["replacement"]["id"] {
					t.Fatal("ordered store-call arguments lost")
				}
				pass := genericFindFunction25f(t, p, "pass")
				if genericReturn25f(t, p, "pass")["binding"] != pass["parameters"].([]any)[0].(map[string]any)["id"] {
					t.Fatal("actual return aliases its original parameter")
				}
				replace := genericFindFunction25f(t, p, "replace")
				params := replace["parameters"].([]any)
				store = replace["body"].([]any)[0].(map[string]any)["value"].(map[string]any)
				if store["target"].(map[string]any)["receiver"].(map[string]any)["binding"] != params[0].(map[string]any)["id"] || store["value"].(map[string]any)["binding"] != params[1].(map[string]any)["id"] {
					t.Fatal("callee must retain parameter-based store target and value")
				}
			} else if store["target"].(map[string]any)["receiver"].(map[string]any)["binding"] != bindings["wide"]["id"] || store["value"].(map[string]any)["binding"] != bindings["replacement"]["id"] {
				t.Fatal("direct store must retain the wider alias and replacement binding")
			}
			if aliasSource["kind"] != "bindingReference" || aliasSource["binding"] != bindings["narrow"]["id"] || alias["allocationSite"] != nil || len(allocation["elements"].([]any)) != 1 || store["kind"] != "elementAssignment" || store["target"].(map[string]any)["type"] != base {
				t.Fatal("aliasing must preserve the source reference and checked wider store")
			}
			proof := genericRootProof25f(t, p, aliasSource)
			if proof["sourceType"] != derived || proof["targetType"] != base {
				t.Fatal("covariant array view requires the actual member projection")
			}
			replacement := body[2].(map[string]any)["initializer"].(map[string]any)
			if replacement["kind"] != "new" || replacement["callee"] != base || replacement["allocationSite"] == allocation["elements"].([]any)[0].(map[string]any)["allocationSite"] {
				t.Fatal("new Base must retain its distinct original allocation")
			}
			read := body[4].(map[string]any)["initializer"].(map[string]any)
			if read["kind"] != "checkedElementAccess" || read["type"] != derived || read["receiver"].(map[string]any)["binding"] != bindings["narrow"]["id"] {
				t.Fatal("read through original narrow alias was erased or retagged")
			}
			proof = genericRootProof25f(t, p, read)
			if proof["sourceType"] != derived || proof["targetType"] != base {
				t.Fatal("observed Base view must keep original read and actual member proof")
			}
			identity := genericReturn25f(t, p, "alias")
			if identity["kind"] != "binary" || identity["operator"] != "===" || identity["type"] != "bool" || identity["left"].(map[string]any)["binding"] != bindings["observed"]["id"] || identity["right"].(map[string]any)["binding"] != bindings["replacement"]["id"] {
				t.Fatal("reference identity comparison tree changed")
			}
			for _, value := range []map[string]any{aliasSource, store, read, identity} {
				rangeValue := value["range"].(map[string]any)
				if int(rangeValue["end"].(float64)) <= int(rangeValue["start"].(float64)) {
					t.Fatal("original occurrence anchor lost")
				}
			}
		})
	}
}
