// SPDX-License-Identifier: Apache-2.0
// R01 assertions preserve representation; numeric conversions use owned entries.
package main

import (
	"bytes"
	"errors"
	"testing"
)

func TestArrayGenericV8AssertionCannotConvertNumeric25f(t *testing.T) {
	for _, source := range []string{
		"export function f(a:i32[],i:u32):f64{return a[i] as f64;}",
		"export function f(a:f64[],i:u32):i32{return a[i] as i32;}",
		"export function f(a:f32[],i:u32):f64{return a[i] as f64;}",
		"export function f(a:i32[],i:u32):f64{return (a as f64[])[i];}",
		"export function f(a:ReadSpan<i32>,i:u32):f64{return (a as ReadSpan<f64>)[i];}",
		"export function f(a:i32[][],i:u32):f64{return (a as f64[][])[i][i];}",
		"export function f():f64{const x=1;return x as f64;}",
		"export function f():f64{return (1 as i32) as f64;}",
	} {
		var out bytes.Buffer
		input := genericArrayRequest25f(t, source)
		err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(input), &out)
		if err == nil {
			genericArrayCapture25f(t, input, source, out.Bytes())
		}
		var failure *arrayFactsFailure
		if !errors.As(err, &failure) || failure.Code != "r01-d-scalar-type" || out.Len() != 0 {
			t.Errorf("pure assertion changed a typed numeric representation: err=%v outputBytes=%d source=%s", err, out.Len(), source)
		}
	}
}

func TestArrayGenericV8AssertionPreservesSameTypePresence25f(t *testing.T) {
	for _, source := range []string{
		"export function f(a:i32[],i:u32):i32{return (a[i] as i32)!;}",
		"export function f(a:(i32|undefined)[],i:u32):i32{return (a[i] as i32)!;}",
		"export function f(a:i32[],i:u32):i32|undefined{return a[i] as (i32|undefined);}",
		"class Box{n:i32=1;} export function f(a:Box[],i:u32):Box{return (a[i] as Box)!;}",
		"export function f():f64{return 1 as f64;}",
		"export function f():f64{return (-1) as f64;}",
		"export function f():f64{return (1) as f64;}",
		"export function f(i:u32):f64{return ([1,2] as f64[])[i];}",
		"export function f(a:(i32|undefined)[],i:u32):undefined{return a[i] as undefined;}",
		"export function f(a:(i32|f64)[],i:u32):i32{return a[i] as i32;}",
		"export function f(a:ReadSpan<i32>,i:u32):i32{return (a as ReadSpan<i32>)[i];}",
	} {
		genericArrayPublic25f(t, source)
	}
}
