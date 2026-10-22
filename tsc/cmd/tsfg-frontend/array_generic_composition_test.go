// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func genericFindFunction25f(t *testing.T, p map[string]any, name string) map[string]any {
	t.Helper()
	for _, fn := range p["functions"].([]any) {
		v := fn.(map[string]any)
		if v["name"] == name {
			return v
		}
	}
	t.Fatalf("missing function %s", name)
	return nil
}
func genericReturn25f(t *testing.T, p map[string]any, name string) map[string]any {
	t.Helper()
	fn := genericFindFunction25f(t, p, name)
	body := fn["body"].([]any)
	return body[len(body)-1].(map[string]any)["value"].(map[string]any)
}

func TestArrayGenericV8CompoundCallsPublic25f(t *testing.T) {
	source := "function receiver(a:i32[]):i32[] { return a; } function index(i:u32):u32 { return i; } function rhs(v:i32):i32 { return v; } export function update(a:i32[],i:u32):i32 { return receiver(a)[index(i)] += rhs(2); }"
	p := genericArrayPublic25f(t, source)
	v := genericReturn25f(t, p, "update")
	if v["kind"] != "elementCompoundAssignment" || v["operator"] != "+=" {
		t.Fatalf("compound lost: %v", v)
	}
	target := v["target"].(map[string]any)
	for _, item := range []struct {
		v    any
		name string
	}{{target["receiver"], "receiver"}, {target["index"], "index"}, {v["value"], "rhs"}} {
		call := item.v.(map[string]any)
		if call["kind"] != "call" || call["callee"] != genericFindFunction25f(t, p, item.name)["id"] || call["signature"] == nil {
			t.Fatalf("actual %s call lost: %v", item.name, call)
		}
	}
	if v["receiver"] != nil || v["index"] != nil {
		t.Fatalf("duplicated target expressions: %v", v)
	}
}
func TestArrayGenericV8UpdateWrappersPublic25f(t *testing.T) {
	for _, tc := range []struct {
		expression string
		prefix     bool
	}{{"a[i]++", false}, {"++a[i]", true}, {"a[i]--", false}, {"--a[i]", true}} {
		p := genericArrayPublic25f(t, "export function update(a:i32[],i:u32):i32 { return "+tc.expression+"; }")
		v := genericReturn25f(t, p, "update")
		if v["kind"] != "elementUpdate" || v["prefix"] != tc.prefix || v["target"] == nil {
			t.Fatalf("update result semantics lost: %v", v)
		}
	}
	p := genericArrayPublic25f(t, "export function read(a:(i32|undefined)[],i:u32):i32 { return (a[i] as i32)!; }")
	v := genericReturn25f(t, p, "read")
	if v["kind"] != "nonNull" {
		t.Fatal(v)
	}
	v = v["expression"].(map[string]any)["expression"].(map[string]any)
	if v["kind"] != "as" || v["expression"].(map[string]any)["kind"] != "checkedElementAccess" {
		t.Fatalf("assertion erased checked access: %v", v)
	}
}
func TestArrayGenericV8StringAndAbsencePublic25f(t *testing.T) {
	p := genericArrayPublic25f(t, "export function read(i:u32):string|undefined { const a:(string|undefined)[]=[\"a\",\"\\ud800\",undefined]; a[i]=\"b\"+\"\\u0000\"; return a[i]; }")
	fn := genericFindFunction25f(t, p, "read")
	body := fn["body"].([]any)
	literal := body[0].(map[string]any)["initializer"].(map[string]any)
	units := literal["elements"].([]any)[1].(map[string]any)["codeUnits"].([]any)
	if len(units) != 1 || units[0] != float64(0xd800) {
		t.Fatalf("M24 unpaired surrogate changed: %v", units)
	}
	if body[1].(map[string]any)["value"].(map[string]any)["value"].(map[string]any)["kind"] != "stringConcat" {
		t.Fatal("String concat tree lost")
	}
	genericArrayPublic25f(t, "export function read(i:u32):i32|null|undefined { const a:(i32|null|undefined)[]=[null,undefined,7]; return a[i]; }")
}
func TestArrayGenericV8NominalAndMethodsPublic25f(t *testing.T) {
	for _, source := range []string{
		"class Box { constructor(public n:i32) {} } export function read(a:Box[],i:u32):Box { return a[i]!; }",
		"@tsfgValue class Value { n:i32=2; name:string=\"v\"; } export function read(i:u32):Value { const a:Value[]=[new Value()]; return a[i]; }",
		"class Box { constructor(public n:i32) {} next(i:u32):u32 { this.n=this.n+1; return i; } } export function update(b:Box,a:i32[],i:u32):i32 { return a[b.next(i)] += b.n; }",
	} {
		p := genericArrayPublic25f(t, source)
		found := false
		for _, raw := range p["types"].([]any) {
			typ := raw.(map[string]any)
			if typ["kind"] == "reference" || typ["kind"] == "value" {
				found = true
				if typ["nominalKey"] == nil || typ["runtimeKey"] == nil || typ["fields"] == nil {
					t.Fatal("nominal original facts missing")
				}
			}
		}
		if !found {
			t.Fatal("no nominal T catalog")
		}
	}
}
func TestArrayGenericV8ReadSpanFlowPublic25f(t *testing.T) {
	p := genericArrayPublic25f(t, "export function read(a:(string|undefined)[],i:u32):string { const s:ReadSpan<string|undefined>=readSpan(a,0,2); const v:string|undefined=s[i]; if(v!==undefined){return v;} return \"missing\"; }")
	fn := genericFindFunction25f(t, p, "read")
	body := fn["body"].([]any)
	branch := body[2].(map[string]any)
	ret := branch["then"].([]any)[0].(map[string]any)["value"].(map[string]any)
	if ret["type"] != "union<string,undefined>" || ret["flowType"] != "string" {
		t.Fatalf("declared/flow identities conflated: %v", ret)
	}
}
func TestArrayGenericV8RejectsDensityAndVersionPublic25f(t *testing.T) {
	for _, source := range []string{
		"export function bad():i32 { const a:i32[]=[1,2]; a[4]=3; return 0; }",
		"export function bad():i32 { const a:i32[]=[1,2]; a.length=3; return 0; }",
		"export function bad():i32 { const a:i32[]=[1,2]; delete a[0]; return 0; }",
	} {
		var out bytes.Buffer
		err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(genericArrayRequest25f(t, source)), &out)
		var failure *arrayFactsFailure
		if !errors.As(err, &failure) || failure.Code != "r03-array-facts-density" || out.Len() != 0 {
			t.Fatalf("density expected: %v %s", err, out.String())
		}
	}
	input := genericArrayRequest25f(t, "export function read(i:u32):i32 { const a:i32[]=[1]; return a[i]; }")
	for _, raw := range []string{strings.Replace(string(input), "\"version\":8", "\"version\":7", 1), strings.TrimSuffix(string(input), "\n"), strings.Replace(string(input), "\"version\":8", "\"extra\":0,\"version\":8", 1)} {
		var out bytes.Buffer
		if err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, strings.NewReader(raw), &out); err == nil || out.Len() != 0 {
			t.Fatal("malformed v8 package accepted")
		}
	}
}
