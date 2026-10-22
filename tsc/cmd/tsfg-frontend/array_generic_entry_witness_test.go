// SPDX-License-Identifier: Apache-2.0
package main

import "testing"

func TestArrayGenericV8EntryReadonly25f(t *testing.T) {
	source := "class Frozen{readonly n:i32=1;} class Writable{n:i32=1;} export function f(a:Frozen):i32{const b:Writable=a;b.n=7;return a.n;}"
	p := genericArrayPublic25f(t, source)
	entries, ok := p["entryCompatibilityRoots"].([]any)
	if !ok || len(entries) != 1 {
		t.Fatal("original readonly export needs an authoritative parameter entry record")
	}
	types := genericConstructionTypes25f(t, p)
	frozen, writable := types["Frozen"], types["Writable"]
	fn := genericFindFunction25f(t, p, "f")
	param := fn["parameters"].([]any)[0].(map[string]any)
	entry := entries[0].(map[string]any)
	origin := entry["origin"].(map[string]any)
	if entry["functionId"] != fn["id"] || entry["parameterBinding"] != param["id"] || entry["viewType"] != frozen.ID || origin["source"] != "/src/main.ts" || source[int(origin["start"].(float64)):int(origin["end"].(float64))] != "a:Frozen" {
		t.Fatal("entry must join the actual export parameter declaration")
	}
	roots := entry["roots"].([]any)
	if len(roots) != 1 || roots[0].(map[string]any)["sourceType"] != writable.ID || roots[0].(map[string]any)["targetType"] != frozen.ID {
		t.Fatal("readonly entry needs the real reverse structural candidate, without an identity proof")
	}
	for _, raw := range p["compatibilityWitnesses"].([]any) {
		proof := raw.(map[string]any)["compatibility"].(map[string]any)
		if proof["sourceType"] != writable.ID || proof["targetType"] != frozen.ID {
			continue
		}
		anchor := proof["origin"].(map[string]any)
		members := proof["members"].([]any)
		if anchor["start"] != origin["start"] || anchor["end"] != origin["end"] || proof["assignable"] != true || len(members) != 1 || members[0].(map[string]any)["source"] != (*writable.Fields)[0].ID || members[0].(map[string]any)["target"] != (*frozen.Fields)[0].ID {
			t.Fatal("reverse entry member authority must come from the original M12 checker relation")
		}
		alias := fn["body"].([]any)[0].(map[string]any)["initializer"].(map[string]any)
		forward := genericRootProof25f(t, p, alias)
		if forward["sourceType"] != frozen.ID || forward["targetType"] != writable.ID {
			t.Fatal("the original expression view remains a distinct occurrence")
		}
		return
	}
	t.Fatal("entry root is missing its closed real M12 directory record")
}
