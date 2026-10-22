// SPDX-License-Identifier: Apache-2.0
// Cold consumers require actual nested member projections, including back-edges.
package main

import "testing"

func TestArrayGenericV8CompatibilityClosure25f(t *testing.T) {
	for _, test := range []struct {
		name, source string
		pairs        map[string][]string
	}{
		{"nested", "class X{n:i32=1;} class Y{n:i32=2;} class A{child:X=new X();} class B{child:Y=new Y();} export function f(a:A):B{return a;}", map[string][]string{"A>B": {"X>Y"}, "X>Y": {}}},
		{"method", "class X{n:i32=1;} class Y{n:i32=2;} class A{m(x:X):X{return new X();}} class B{m(x:Y):Y{return new Y();}} export function f(a:A):B{return a;}", map[string][]string{"A>B": {"X>Y", "Y>X"}, "X>Y": {}, "Y>X": {}}},
		{"union", "class X{n:i32=1;} class Y{n:i32=2;} class A{constructor(public child:X|undefined){}} class B{constructor(public child:Y|undefined){}} export function f(a:A):B{return a;}", map[string][]string{"A>B": {"X>Y"}, "X>Y": {}}},
		{"cycle", "class X{n:i32=1;next:X[]=[];} class Y{n:i32=2;next:Y[]=[];} export function f(a:X):Y{return a;}", map[string][]string{"X>Y": {"X>Y"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := genericArrayPublic25f(t, test.source)
			genericWitnessClosure25f(t, p, test.pairs)
		})
	}
}

func genericWitnessClosure25f(t *testing.T, p map[string]any, expected map[string][]string) {
	t.Helper()
	records, ok := p["compatibilityWitnesses"].([]any)
	if !ok || len(records) != len(expected) {
		t.Fatalf("actual exported facts lack the closed nested witness directory: got=%v expectedPairs=%v", p["compatibilityWitnesses"], expected)
	}
	names := map[string]string{}
	for _, raw := range p["types"].([]any) {
		typ := raw.(map[string]any)
		if name, ok := typ["name"].(string); ok {
			names[typ["id"].(string)] = name
		}
	}
	root := genericReturn25f(t, p, "f")["compatibility"].(map[string]any)
	origin := root["origin"].(map[string]any)
	seen, previous := map[string]bool{}, ""
	for _, raw := range records {
		record := raw.(map[string]any)
		proof := record["compatibility"].(map[string]any)
		a, b := proof["sourceType"].(string), proof["targetType"].(string)
		key, order := names[a]+">"+names[b], a+"\x00"+b
		where := proof["origin"].(map[string]any)
		if proof["assignable"] != true || where["source"] != origin["source"] || where["start"] != origin["start"] || where["end"] != origin["end"] || seen[key] || previous != "" && previous >= order {
			t.Fatalf("witness identity/origin/order is not its actual unique class pair: %v", proof)
		}
		seen[key], previous = true, order
		wanted, exists := expected[key]
		deps := record["dependencies"].([]any)
		if !exists || len(deps) != len(wanted) || len(proof["members"].([]any)) == 0 {
			t.Fatalf("wrong actual member/dependency closure for %s: %v", key, record)
		}
		remaining, before := map[string]bool{}, ""
		for _, pair := range wanted {
			remaining[pair] = true
		}
		for _, raw := range deps {
			dep := raw.(map[string]any)
			x, y := dep["sourceType"].(string), dep["targetType"].(string)
			pair, order := names[x]+">"+names[y], x+"\x00"+y
			if !remaining[pair] || before != "" && before >= order {
				t.Fatalf("dependency must be the selected original pair, sorted and unique: %v", dep)
			}
			delete(remaining, pair)
			before = order
		}
	}
}
