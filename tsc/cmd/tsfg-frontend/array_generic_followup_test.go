// SPDX-License-Identifier: Apache-2.0
// Incremental review counterexamples remain small at the owned source seam.
package main

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestArrayGenericV8ReviewClassAssertionRepresentation25f(t *testing.T) {
	source := "class A{n:i32=1;} class B{n:f64=1.0;} export function f(a:A[],i:u32):f64{return (a[i] as B).n;}"
	input := genericArrayRequest25f(t, source)
	var out bytes.Buffer
	err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(input), &out)
	if err == nil {
		genericArrayCapture25f(t, input, source, out.Bytes())
	}
	var failure *arrayFactsFailure
	if !errors.As(err, &failure) || failure.Code != "r01-d-scalar-type" || out.Len() != 0 {
		t.Fatalf("class assertion changed a settled field representation: err=%v outputBytes=%d", err, out.Len())
	}
}

func TestArrayGenericV8ReviewUnionExpansionWork25f(t *testing.T) {
	// Only three members repeated three times. The existing shared100000 budget
	// is nearly spent, so no large source or expanded temporary is ever needed.
	p := &arrayGenericProjection{ctx: t.Context(), annotationCalls: 99995, types: map[string]arrayGenericType{
		"U":                         {Kind: "union", Members: []string{"i32", "null", "undefined"}},
		"union<i32,null,undefined>": {ID: "union<i32,null,undefined>", Kind: "union", Members: []string{"i32", "null", "undefined"}},
	}}
	_, err := p.union([]string{"U", "U", "U"}, nil)
	var failure *arrayFactsFailure
	if !errors.As(err, &failure) || failure.Code != "r03-array-facts-limit" {
		t.Fatalf("cached union expansion edges must consume shared work before materialization: %v", err)
	}
}

func TestArrayGenericV8ReviewUnionExpansionCanceled25f(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	p := &arrayGenericProjection{ctx: ctx, types: map[string]arrayGenericType{"i32": {Kind: "scalar"}}}
	_, err := p.union([]string{"i32", "i32"}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("union expansion must preserve cancellation before copying edges: %v", err)
	}
}

func TestArrayGenericV8ReviewClassAssertionMembers25f(t *testing.T) {
	for _, source := range []string{
		"class A{n:i32=1;} class B{n:i32=2;} export function f(a:A[],i:u32):i32{return (a[i] as B).n;}",
		"class A{n:i32=1;} class B{n:i32=2;} export function f(a:(A|undefined)[],i:u32):i32{return (a[i] as B).n;}",
		"class A{n:i32=1;next:A[]=[];} class B{n:i32=2;next:B[]=[];} export function f(a:A[],i:u32):i32{return (a[i] as B).n;}",
	} {
		p := genericArrayPublic25f(t, source)
		cast := genericReturn25f(t, p, "f")["receiver"].(map[string]any)["expression"].(map[string]any)
		proof := genericRootProof25f(t, p, cast)
		if cast["kind"] != "as" || cast["expression"].(map[string]any)["kind"] != "checkedElementAccess" || proof["assignable"] != true {
			t.Fatalf("structural assertion lost original checked receiver or M12 proof: %v", cast)
		}
		members := proof["members"].([]any)
		if len(members) == 0 || proof["sourceType"] == proof["targetType"] {
			t.Fatalf("structural assertion must retain actual distinct class/member IDs: %v", proof)
		}
	}
}

func TestArrayGenericV8ReviewNestedClassAssertionRepresentation25f(t *testing.T) {
	for _, source := range []string{
		"class A{n:i32=1;} class B{n:f64=1.0;} class X{value:A=new A();} class Y{value:B=new B();} export function f(a:X[],i:u32):f64{return (a[i] as Y).value.n;}",
		"class A{n:i32[]=[];} class B{n:f64[]=[];} export function f(a:A[],i:u32):f64{return (a[i] as B).n[i];}",
		"class A{m():i32{return 1;}} class B{m():f64{return 1.0;}} export function f(a:A[],i:u32):f64{return (a[i] as B).m();}",
		"class A{m(x:i32):i32{return 1;}} class B{m(x:f64):i32{return 1;}} export function f(a:A[],i:u32):i32{return (a[i] as B).m(1.0);}",
	} {
		var out bytes.Buffer
		err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(genericArrayRequest25f(t, source)), &out)
		var failure *arrayFactsFailure
		if !errors.As(err, &failure) || failure.Code != "r01-d-scalar-type" || out.Len() != 0 {
			t.Errorf("member view changed a settled representation: err=%v outputBytes=%d source=%s", err, out.Len(), source)
		}
	}
}

func TestArrayGenericV8ReviewValueAssertionIdentity25f(t *testing.T) {
	genericArrayPublic25f(t, "@tsfgValue class V{n:i32=1;} export function f(a:(V|undefined)[],i:u32):i32{return (a[i] as V).n;}")
	genericArrayPublic25f(t, "class Box{n:i32=1;} @tsfgValue class Pair{n:i32|undefined=undefined;child:Box|undefined=undefined;} export function f(a:Pair[],i:u32):Pair{return (a[i] as Pair)!;}")
	for _, source := range []string{
		"@tsfgValue class V{n:i32=1;} class A{n:i32=1;} export function f(a:A[],i:u32):i32{return (a[i] as V).n;}",
		"@tsfgValue class V{n:i32=1;} class A{n:i32=1;} export function f(a:V[],i:u32):i32{return (a[i] as A).n;}",
	} {
		var out bytes.Buffer
		err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(genericArrayRequest25f(t, source)), &out)
		var failure *arrayFactsFailure
		if !errors.As(err, &failure) || failure.Code != "r03-array-facts-capability" || out.Len() != 0 {
			t.Errorf("value as must retain existing nominal representation identity: err=%v outputBytes=%d", err, out.Len())
		}
	}
}

func TestArrayGenericV8ReviewUnionCanonicalDedup25f(t *testing.T) {
	p := genericArrayPublic25f(t, "type U=undefined|i32|null;type V=U|U|U;export function f(a:V[],i:u32):V{return a[i];}")
	found := 0
	for _, raw := range p["types"].([]any) {
		typ := raw.(map[string]any)
		if typ["kind"] == "union" {
			found++
			if typ["id"] != "union<i32,null,undefined>" || len(typ["members"].([]any)) != 3 {
				t.Fatalf("logical union canonical output changed after edge budgeting: %v", typ)
			}
		}
	}
	if found != 1 {
		t.Fatalf("deduplicated aliases must retain one canonical catalog union: %d", found)
	}
}

type genericArrayCancelingContext25f struct {
	context.Context
	cancel context.CancelFunc
	checks int
}

func (c *genericArrayCancelingContext25f) Err() error {
	c.checks++
	if c.checks == 3 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestArrayGenericV8ReviewUnionSortBoundaries25f(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	controlled := &genericArrayCancelingContext25f{Context: ctx, cancel: cancel}
	p := &arrayGenericProjection{ctx: controlled}
	if err := p.sortUnion([]string{"z", "y", "x", "w"}); !errors.Is(err, context.Canceled) || controlled.checks != 3 {
		t.Fatalf("sort must stop at the first actual mid-copy cancellation: %v", err)
	}
	p = &arrayGenericProjection{ctx: t.Context(), annotationCalls: 99999}
	_, err := p.union([]string{"i32", "i32"}, nil)
	var failure *arrayFactsFailure
	if !errors.As(err, &failure) || failure.Code != "r03-array-facts-limit" {
		t.Fatalf("duplicate edges must consume work before copying: %v", err)
	}
	p = &arrayGenericProjection{ctx: t.Context(), annotationCalls: 99999}
	err = p.sortUnion([]string{"z", "a"})
	if !errors.As(err, &failure) || failure.Code != "r03-array-facts-limit" {
		t.Fatalf("sorting unique members must consume the same work budget: %v", err)
	}
}
