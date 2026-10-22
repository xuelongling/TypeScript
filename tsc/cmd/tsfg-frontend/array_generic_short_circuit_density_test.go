// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"errors"
	"testing"
)

func TestArrayGenericV8ShortCircuitDensityMeet25f(t *testing.T) {
	// The || neighbor has the same skipped/evaluated length alternatives as &&.
	source := "export function read(flag:boolean):i32{const a:i32[]=[1];const alias:i32[]=a;flag||((alias.length=0)===0);a[1]=2;return a[0];}"
	p := genericArrayPublic25f(t, source)
	read := genericFindFunction25f(t, p, "read")
	body := read["body"].([]any)
	logic := body[2].(map[string]any)["value"].(map[string]any)
	result := genericReturn25f(t, p, "read")
	if logic["operator"] != "||" || result["kind"] != "checkedElementAccess" || result["type"] != "i32" {
		t.Fatal("conditional || shrink must retain a checked result")
	}
	// Conditional array rebinding must not restore proofs invalidated by its RHS.
	genericArrayPublic25f(t, "export function read(flag:boolean):i32{const a:i32[]=[1];let b:i32[]=a;const c:i32[]=[1];flag&&((b=c).length===1);a[2]=2;return a[0];}")
	for _, source := range []string{
		"export function read():i32{const a:i32[]=[1];a.length=0;a[1]=2;return a[0];}",
		"export function read():i32{const a:i32[]=[1];a.length=0;a[0]+=2;return a[0];}",
		"export function read(flag:boolean):i32{const a:i32[]=[1];const b:i32[]=[1];flag&&((b.length=0)===0);a[2]=2;return a[0];}",
		"export function read(flag:boolean):i32{const a:i32[]=[1];flag&&((a.length=1)===1);a[2]=2;return a[0];}",
	} {
		var output bytes.Buffer
		err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(genericArrayRequest25f(t, source)), &output)
		var failure *arrayFactsFailure
		if !errors.As(err, &failure) || failure.Kind != "density" || failure.Code != "r03-array-facts-density" || output.Len() != 0 {
			t.Fatalf("unchanged must-gap and missing old-element facts must remain rejected: %v", err)
		}
	}
	t.Log("|| conditional positive; unconditional gap, missing old element, unrelated and equal-length must proofs retained")
}
