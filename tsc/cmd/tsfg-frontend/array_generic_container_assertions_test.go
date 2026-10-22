// SPDX-License-Identifier: Apache-2.0
package main

import (
	"strings"
	"testing"
)

const genericContainerAssertionPrefix25f = "class Left{constructor(public n:i32,public l:i32){}} class Base{constructor(public n:i32){}} class OnlyLeftView{constructor(public l:i32){}} "

func TestArrayGenericV8ContainerTargetUnionAssertions25f(t *testing.T) {
	for _, test := range []struct{ name, tail string }{
		{"array", "export function f(a:Left[]):Base[]|OnlyLeftView[]{return a as (Base[]|OnlyLeftView[]);}"},
		{"readspan", "export function f(a:ReadSpan<Left>):ReadSpan<Base>|ReadSpan<OnlyLeftView>{return a as (ReadSpan<Base>|ReadSpan<OnlyLeftView>);}"},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := genericArrayPublic25f(t, genericContainerAssertionPrefix25f+test.tail)
			cast := genericReturn25f(t, p, "f")
			proof := genericRootProof25f(t, p, cast)
			types := map[string]map[string]any{}
			for _, raw := range p["types"].([]any) {
				typ := raw.(map[string]any)
				types[typ["id"].(string)] = typ
			}
			operand := cast["expression"].(map[string]any)
			source := types[operand["flowType"].(string)]
			target := types[cast["flowType"].(string)]
			members := target["members"].([]any)
			first := types[members[0].(string)]
			if cast["kind"] != "as" || operand["kind"] != "bindingReference" || proof["sourceType"] != source["elementType"] || proof["targetType"] != first["elementType"] || proof["assignable"] != true || len(proof["members"].([]any)) != 1 || len(genericExpressionWitnesses25f(p)) != 1 {
				t.Fatal("container target-union lost the actual first representation-preserving member proof or original operand")
			}
			if first["kind"] != test.name && !(test.name == "readspan" && first["kind"] == "readSpan") {
				t.Fatal("target container family changed")
			}
		})
	}
}

func TestArrayGenericV8ContainerTargetUnionSeed25f(t *testing.T) {
	main := genericContainerAssertionPrefix25f + "export function f(a:Left[]):Base[]|OnlyLeftView[]{return a as (Base[]|OnlyLeftView[]);}"
	seed := strings.Replace(main, "(Base[]|OnlyLeftView[])", "(Base[]               )", 1)
	if len(main) != len(seed) {
		t.Fatal("seed must retain exact original as occurrence width")
	}
	p := genericArrayPublic25f(t, seed)
	cast := genericReturn25f(t, p, "f")
	proof := genericRootProof25f(t, p, cast)
	where := cast["range"].(map[string]any)
	if where["start"] != float64(strings.Index(main, "a as (")) || where["end"] != float64(strings.LastIndex(main, ";")) {
		t.Fatal("seed moved the actual source occurrence")
	}
	for _, raw := range p["types"].([]any) {
		typ := raw.(map[string]any)
		if typ["name"] == "Base" && proof["targetType"] != typ["id"] {
			t.Fatal("seed must retain the actual Base-only authority")
		}
	}
}
