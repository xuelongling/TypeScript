// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"errors"
	"testing"
)

func TestArrayGenericV8EntryCrossExport25f(t *testing.T) {
	p := genericArrayPublic25f(t, "class Base{n:i32=1;} class Derived extends Base{extra:i32=2;} export function widen(b:Base[]):void{b[0]=new Base();} export function read(a:Derived[]):Base{return a[0];}")
	types := genericConstructionTypes25f(t, p)
	if len(p["functions"].([]any)) != 2 || len(p["entryCompatibilityRoots"].([]any)) != 2 {
		t.Fatal("both host exports must retain their parameter entries without a fabricated internal caller")
	}
	widen := genericFindFunction25f(t, p, "widen")
	read := genericFindFunction25f(t, p, "read")
	wp, rp := widen["parameters"].([]any)[0].(map[string]any), read["parameters"].([]any)[0].(map[string]any)
	store := widen["body"].([]any)[0].(map[string]any)["value"].(map[string]any)
	value := genericReturn25f(t, p, "read")
	if store["target"].(map[string]any)["receiver"].(map[string]any)["binding"] != wp["id"] || store["value"].(map[string]any)["callee"] != types["Base"].ID || value["receiver"].(map[string]any)["binding"] != rp["id"] || value["type"] != types["Derived"].ID {
		t.Fatal("separate export store and original narrow checked read must keep real bindings")
	}
	proof := genericRootProof25f(t, p, value)
	if proof["sourceType"] != types["Derived"].ID || proof["targetType"] != types["Base"].ID {
		t.Fatal("read contextual view lost its actual member authority")
	}
	for _, raw := range p["entryCompatibilityRoots"].([]any) {
		entry := raw.(map[string]any)
		roots := entry["roots"].([]any)
		if entry["functionId"] == read["id"] && len(roots) != 0 {
			t.Fatal("inherited base members must not admit Base as a Derived entry payload")
		}
		if entry["functionId"] == widen["id"] && (len(roots) != 1 || roots[0].(map[string]any)["sourceType"] != types["Derived"].ID || roots[0].(map[string]any)["targetType"] != types["Base"].ID) {
			t.Fatal("wider export entry lost its actual Derived candidate")
		}
	}
}

func TestArrayGenericV8EntryNestedClosure25f(t *testing.T) {
	prefix := "class X{n:i32=1;next:X|undefined=undefined;} class Y{n:i32=2;next:Y|undefined=undefined;} "
	for _, source := range []string{
		prefix + "@tsfgValue class V{items!:ReadSpan<X[]>;} export function f(a:ReadSpan<V|undefined>,n:u32):void{}",
		prefix + "class Holder{get(x:X):X{return x;}} export function f(a:Holder,n:u32):void{}",
	} {
		p := genericArrayPublic25f(t, source)
		types := genericConstructionTypes25f(t, p)
		entries := p["entryCompatibilityRoots"].([]any)
		if len(entries) != 2 {
			t.Fatal("every original parameter, including native u32, needs one entry")
		}
		roots := 0
		for _, raw := range entries {
			entry := raw.(map[string]any)
			pairs := entry["roots"].([]any)
			if entry["viewType"] == "u32" && len(pairs) != 0 {
				t.Fatal("native entry invented a reference proof")
			}
			for _, raw := range pairs {
				pair := raw.(map[string]any)
				if pair["sourceType"] != types["Y"].ID || pair["targetType"] != types["X"].ID {
					t.Fatal("nested type/method closure admitted the wrong reference payload")
				}
				roots++
			}
		}
		if roots != 1 {
			t.Fatal("recursive nested entry must publish exactly its real Y-to-X root")
		}
		for _, raw := range p["compatibilityWitnesses"].([]any) {
			record := raw.(map[string]any)
			deps := record["dependencies"].([]any)
			if len(deps) != 1 || deps[0].(map[string]any)["sourceType"] != types["Y"].ID || deps[0].(map[string]any)["targetType"] != types["X"].ID {
				t.Fatal("cyclic M12 member proof must retain its explicit same-pair back edge")
			}
		}
	}
}

func TestArrayGenericV8EntryWorkBoundaries25f(t *testing.T) {
	fields := []arrayGenericField{{Type: "array<A>"}}
	for _, cause := range []string{"cycle", "work", "cancel"} {
		t.Run(cause, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			p := &arrayGenericProjection{ctx: ctx, types: map[string]arrayGenericType{"A": {ID: "A", Kind: "reference", Fields: &fields}, "array<A>": {ID: "array<A>", Kind: "array", ElementType: "A"}}}
			if cause == "work" {
				p.annotationCalls = 99999
			} else if cause == "cancel" {
				cancel()
			}
			targets, err := p.entryReferenceTargets("A")
			if cause == "cycle" {
				if err != nil || len(targets) != 1 || targets[0] != "A" || p.annotationCalls > 20 {
					t.Fatalf("recursive reference closure must visit each type once: %v %v", targets, err)
				}
			} else if cause == "cancel" {
				if !errors.Is(err, context.Canceled) || targets != nil {
					t.Fatalf("canceled entry walk exposed a partial target set: %v %v", targets, err)
				}
			} else {
				var failure *arrayFactsFailure
				if !errors.As(err, &failure) || failure.Code != "r03-array-facts-limit" || targets != nil || p.annotationCalls != 100000 {
					t.Fatalf("entry walk must charge before expansion and publish no partial targets: %v %v", targets, err)
				}
			}
		})
	}
}

// Expression-only regression counts remain scoped to expression occurrences;
// entry-role proofs have their own independent origin and closure assertions.
func genericExpressionWitnesses25f(p map[string]any) []any {
	entries := map[classAnchor]bool{}
	for _, raw := range p["entryCompatibilityRoots"].([]any) {
		v := raw.(map[string]any)["origin"].(map[string]any)
		entries[classAnchor{Source: v["source"].(string), Start: int(v["start"].(float64)), End: int(v["end"].(float64))}] = true
	}
	result := []any{}
	for _, raw := range p["compatibilityWitnesses"].([]any) {
		v := raw.(map[string]any)["compatibility"].(map[string]any)["origin"].(map[string]any)
		if !entries[classAnchor{Source: v["source"].(string), Start: int(v["start"].(float64)), End: int(v["end"].(float64))}] {
			result = append(result, raw)
		}
	}
	return result
}
