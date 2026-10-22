// SPDX-License-Identifier: Apache-2.0
// Ordinary members cover every source variant; assertions select separately.
package main

import (
	"bytes"
	"errors"
	"testing"
)

func TestArrayGenericV8ReviewOrdinaryVariantCoverage25f(t *testing.T) {
	source := "class A{constructor(public v:i32|f64){}} class B{constructor(public v:i32|f32){}} export function f(a:A):B{return a;}"
	input := genericArrayRequest25f(t, source)
	var out bytes.Buffer
	err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(input), &out)
	if err == nil {
		genericArrayCapture25f(t, input, source, out.Bytes())
	}
	var failure *arrayFactsFailure
	if !errors.As(err, &failure) || failure.Code != "r01-d-scalar-type" || out.Len() != 0 {
		t.Fatalf("ordinary member relation dropped source f64 variant: err=%v outputBytes=%d", err, out.Len())
	}
}

func TestArrayGenericV8ReviewValuePresenceWidening25f(t *testing.T) {
	genericArrayPublic25f(t, "@tsfgValue class V{n:i32=1;} class A{v:V=new V();} class B{v:V|undefined=undefined;} export function f(a:A[],i:u32):B{return a[i] as B;}")
}

func TestArrayGenericV8ReviewOrdinaryVariantSubsets25f(t *testing.T) {
	for _, source := range []string{
		"class A{constructor(public v:i32){}} class B{constructor(public v:i32|f64){}} export function f(a:A):B{return a;}",
		"class A{constructor(public v:i32|f64){}} class B{constructor(public v:i32|f32|f64){}} export function f(a:A):B{return a;}",
		"class A{constructor(public v:i32|undefined){}} class B{constructor(public v:i32|f64|undefined){}} export function f(a:A):B{return a;}",
	} {
		genericArrayPublic25f(t, source)
	}
}

func genericArrayRelationReject25f(t *testing.T, source, code string) {
	t.Helper()
	var out bytes.Buffer
	err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(genericArrayRequest25f(t, source)), &out)
	var failure *arrayFactsFailure
	if !errors.As(err, &failure) || failure.Code != code || out.Len() != 0 {
		t.Errorf("ordinary relation changed a source representation: err=%v outputBytes=%d source=%s", err, out.Len(), source)
	}
}

func TestArrayGenericV8ReviewDistinctValueUnionIdentity25f(t *testing.T) {
	for _, source := range []string{
		"@tsfgValue class V{n:i32=1;} @tsfgValue class W{n:i32=1;} class A{v:V=new V();} class B{v:W|undefined=undefined;} export function f(a:A[],i:u32):B{return a[i] as B;}",
		"@tsfgValue class V{n:i32=1;} @tsfgValue class W{n:i32=1;} class A{m():V{return new V();}} class B{m():W|undefined{return undefined;}} export function f(a:A):B{return a;}",
	} {
		genericArrayRelationReject25f(t, source, "r03-array-facts-capability")
	}
}

func TestArrayGenericV8ReviewMethodVariantDirections25f(t *testing.T) {
	for _, source := range []string{
		"class A{m():i32{return 0;}} class B{m():i32|f64{return 0 as i32;}} export function f(a:A):B{return a;}",
		"class A{m(x:i32|f32|f64):i32{return 0;}} class B{m(x:i32|f32):i32{return 0;}} export function f(a:A):B{return a;}",
		"class A{m():i32{return 0;}} class B{m():void{}} export function f(a:A):B{return a;}",
		"@tsfgValue class V{n:i32=1;} class A{m():V{return new V();}} class B{m():V|undefined{return undefined;}} export function f(a:A):B{return a;}",
		"@tsfgValue class V{n:i32=1;} class A{m(x:V|undefined):i32{return 0;}} class B{m(x:V):i32{return 0;}} export function f(a:A):B{return a;}",
		"class A{m(x:i32|f32|f64):i32{return 0;}} class B{m(x:i32|f32):i32{return 0;}} function view(a:A):B{return a;} export function f(a:A):i32{return view(a).m(0 as i32);}",
		"class A{m():i32{return 0;}} class B{m():i32|f64{return 0 as i32;}} function view(a:A):B{return a;} export function f(a:A):i32|f64{return view(a).m();}",
	} {
		genericArrayPublic25f(t, source)
	}
	for _, source := range []string{
		"class A{m():i32|f64{return 0 as i32;}} class B{m():i32|f32{return 0 as i32;}} export function f(a:A):B{return a;}",
		"class A{m(x:i32|f64):i32{return 0;}} class B{m(x:i32|f32):i32{return 0;}} export function f(a:A):B{return a;}",
	} {
		genericArrayRelationReject25f(t, source, "r01-d-scalar-type")
	}
}
