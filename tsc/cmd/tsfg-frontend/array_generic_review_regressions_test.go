// SPDX-License-Identifier: Apache-2.0
// Review findings exercise the unchanged public owned-facts seam.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestArrayGenericV8ReviewLiteralFlow25f(t *testing.T) {
	p := genericArrayPublic25f(t, "export function f(v:i32|undefined):i32 { if(v===1){return v;} return 0; }")
	branch := genericFindFunction25f(t, p, "f")["body"].([]any)[0].(map[string]any)
	v := branch["then"].([]any)[0].(map[string]any)["value"].(map[string]any)
	if v["type"] != "union<i32,undefined>" || v["flowType"] != "i32" {
		t.Fatalf("literal narrowing lost declared logical member: %v", v)
	}
}

func TestArrayGenericV8ReviewCheckedReadFlow25f(t *testing.T) {
	p := genericArrayPublic25f(t, "export function f(a:(i32|undefined)[]):i32 { if(a[0]!==undefined){return a[0];} return 0; }")
	branch := genericFindFunction25f(t, p, "f")["body"].([]any)[0].(map[string]any)
	v := branch["then"].([]any)[0].(map[string]any)["value"].(map[string]any)
	if v["kind"] != "checkedElementAccess" || v["type"] != "union<i32,undefined>" || v["flowType"] != "i32" || v["receiver"] == nil || v["index"] == nil {
		t.Fatalf("flow narrowing erased checked T read: %v", v)
	}
}

func TestArrayGenericV8ReviewRebindingAliases25f(t *testing.T) {
	p := genericArrayPublic25f(t, "export function f():i32 { const a:i32[]=[1]; let b:i32[]=[]; b=a; b[1]=2; a[2]=3; return 0; }")
	body := genericFindFunction25f(t, p, "f")["body"].([]any)
	for _, i := range []int{3, 4} {
		v := body[i].(map[string]any)["value"].(map[string]any)
		if v["kind"] != "elementAssignment" || v["target"] == nil {
			t.Fatalf("alias append must retain checked target: %v", v)
		}
	}
}

func TestArrayGenericV8ReviewBoundedAliasExpansion25f(t *testing.T) {
	// Forty distinct shallow alias declarations safely exercise expansion depth.
	// The unsafe cyclic source is deliberately tested only after guards exist.
	var source strings.Builder
	source.WriteString("type A0=i32;")
	for i := 1; i <= 40; i++ {
		fmt.Fprintf(&source, "type A%d=A%d[];", i, i-1)
	}
	source.WriteString("export function f(a:A40):void {}")
	var out bytes.Buffer
	err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(genericArrayRequest25f(t, source.String())), &out)
	var failure *arrayFactsFailure
	if !errors.As(err, &failure) || failure.Code != "r03-array-facts-limit" || out.Len() != 0 {
		t.Fatalf("bounded alias expansion should stop at the source projection limit: err=%v outputBytes=%d", err, out.Len())
	}
}

func TestArrayGenericV8ReviewRecursiveAliasTerminates25f(t *testing.T) {
	for _, source := range []string{"type A=A[];export function f(a:A):void{}", "type A=ReadSpan<A>;export function f(a:A):void{}", "type A=B[];type B=A[];export function f(a:A):void{}"} {
		var out bytes.Buffer
		err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(genericArrayRequest25f(t, source)), &out)
		var failure *arrayFactsFailure
		if !errors.As(err, &failure) || failure.Code != "r03-array-facts-capability" || !strings.Contains(failure.Detail, "finite back-reference type identity") || out.Len() != 0 {
			t.Fatalf("recursive alias must terminate at the explicit v8 catalog capability boundary: %v", err)
		}
	}
	genericArrayPublic25f(t, "class Link { children:Link[]=[]; } export function f(a:Link[],i:u32):Link{return a[i];}")
}

func TestArrayGenericV8ReviewUnknownNestedAlias25f(t *testing.T) {
	genericArrayPublic25f(t, "export function f():i32 { const a:i32[]=[1]; const outer:i32[][]=[a]; const b:i32[]=outer[0]; b[1]=2; a[2]=3; return 0; }")
}

func TestArrayGenericV8ReviewExpandedAliasKeyBudget25f(t *testing.T) {
	// Warm shallow aliases individually, so memoization cannot evade the total
	// key-byte limit with a finite DAG whose concrete IDs double at every level.
	var source strings.Builder
	source.WriteString("type A0=i32;function f0(a:A0):void{}")
	for i := 1; i <= 22; i++ {
		fmt.Fprintf(&source, "type A%d=A%d[]|ReadSpan<A%d>;function f%d(a:A%d):void{}", i, i-1, i-1, i, i)
	}
	source.WriteString("export function f(a:A22):void{}")
	var out bytes.Buffer
	err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(genericArrayRequest25f(t, source.String())), &out)
	var failure *arrayFactsFailure
	if !errors.As(err, &failure) || failure.Code != "r03-array-facts-limit" || !strings.Contains(failure.Detail, "concrete type key bytes") || out.Len() != 0 {
		t.Fatalf("expanded alias keys must stop within the concrete catalog byte budget: %v", err)
	}
}
