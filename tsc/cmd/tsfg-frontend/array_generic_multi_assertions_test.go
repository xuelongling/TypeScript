// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestArrayGenericV8MultiReferenceAssertions25f(t *testing.T) {
	prefix := "class Base{n:i32=1;} class Left extends Base{l:i32=2;} class Right extends Base{r:i32=3;} "
	for _, test := range []struct{ name, tail string }{
		{"slot", "export function f(x:Left|Right,i:u32):Base{const a:Base[]=[x as Base];return a[i];}"},
		{"array", "export function f(a:(Left|Right)[],i:u32):Base{return (a as Base[])[i];}"},
		{"parentheses", "export function f(x:Left|Right,i:u32):Base{const a:Base[]=[(x) as Base];return a[i];}"},
		{"presence", "export function f(a:(Left|Right|undefined)[],i:u32):Base{return (a as Base[])[i];}"},
		{"container-union", "export function f(a:Left[]|Right[],i:u32):Base{return (a as Base[])[i];}"},
		{"readspan-union", "export function f(a:ReadSpan<Left>|ReadSpan<Right>,i:u32):Base{return (a as ReadSpan<Base>)[i];}"},
		{"target-union", "class OnlyLeftView{r:i32=4;} export function f(x:Left|Right):Base|OnlyLeftView{return x as Base|OnlyLeftView;}"},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := genericArrayPublic25f(t, prefix+test.tail)
			var find func(any) map[string]any
			find = func(value any) map[string]any {
				switch v := value.(type) {
				case map[string]any:
					if v["kind"] == "as" {
						return v
					}
					for _, child := range v {
						if found := find(child); found != nil {
							return found
						}
					}
				case []any:
					for _, child := range v {
						if found := find(child); found != nil {
							return found
						}
					}
				}
				return nil
			}
			cast := find(genericFindFunction25f(t, p, "f"))
			if cast == nil || len(cast["compatibilityRoots"].([]any)) != 2 || len(genericExpressionWitnesses25f(p)) != 2 {
				t.Fatal("multi-reference structural assertion lost selected authoritative roots")
			}
		})
	}
}

func TestArrayGenericV8MultiReferenceAssertionBounds25f(t *testing.T) {
	t.Run("work", func(t *testing.T) {
		p := &arrayGenericProjection{ctx: t.Context(), annotationCalls: 99999, types: map[string]arrayGenericType{"A": {Kind: "array", ElementType: "R"}, "R": {Kind: "reference"}}}
		_, err := p.assertionContainsNominal("A")
		var failure *arrayFactsFailure
		if !errors.As(err, &failure) || failure.Code != "r03-array-facts-limit" {
			t.Fatalf("finite nominal traversal must charge nested work: %v", err)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		p := &arrayGenericProjection{ctx: ctx, types: map[string]arrayGenericType{"R": {Kind: "reference"}}}
		_, err := p.assertionContainsNominal("R")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("nominal assertion traversal must retain cancellation: %v", err)
		}
	})
}

func TestArrayGenericV8MultiReferenceAssertionRepresentations25f(t *testing.T) {
	for _, test := range []struct{ source, code string }{
		{"class Base{n:i32=1;} class A{n:i32=2;} class B{n:f64=1.0;} export function f(x:A|B):Base{return x as Base;}", "r01-d-scalar-type"},
		{"class Base{n:i32=1;} class A{n:i32=2;} class B{n:f64=1.0;} export function f(a:(A|B)[],i:u32):i32{return (a as Base[])[i].n;}", "r01-d-scalar-type"},
		{"@tsfgValue class V{n:i32=1;} @tsfgValue class W{n:i32=2;} class Base{v:V=new V();} class A{v:V=new V();} class B{v:W=new W();} export function f(x:A|B):Base{return x as Base;}", "r03-array-facts-capability"},
	} {
		var out bytes.Buffer
		err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(genericArrayRequest25f(t, test.source)), &out)
		var failure *arrayFactsFailure
		if !errors.As(err, &failure) || failure.Code != test.code || out.Len() != 0 {
			t.Fatalf("multi-reference assertion changed representation: %v bytes=%d", err, out.Len())
		}
	}
}
