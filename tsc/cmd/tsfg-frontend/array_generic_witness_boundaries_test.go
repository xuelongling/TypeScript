// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"errors"
	"testing"
)

func TestArrayGenericV8CompatibilityOrigins25f(t *testing.T) {
	p := genericArrayPublic25f(t, "class X{n:i32=1;} class Y{n:i32=2;} class A{child:X=new X();} class B{child:Y=new Y();} export function f(a:A):B{return a;} export function g(a:A):B{return a;}")
	records := genericExpressionWitnesses25f(p)
	if len(records) != 4 {
		t.Fatalf("two actual occurrences require separate root/nested proofs: %v", records)
	}
	counts := map[float64]int{}
	for _, name := range []string{"f", "g"} {
		proof := genericRootProof25f(t, p, genericReturn25f(t, p, name))
		counts[proof["origin"].(map[string]any)["start"].(float64)] = 0
	}
	previous := float64(-1)
	for _, raw := range records {
		proof := raw.(map[string]any)["compatibility"].(map[string]any)
		start := proof["origin"].(map[string]any)["start"].(float64)
		if _, root := counts[start]; !root || start < previous {
			t.Fatalf("directory leaked a different occurrence or lost numeric origin order: %v", proof)
		}
		counts[start]++
		previous = start
	}
	for _, count := range counts {
		if count != 2 {
			t.Fatalf("origin must inherit exactly its own selected root closure: %v", counts)
		}
	}
}

// These fixtures test owned graph publication, not checker assignability.
func witnessGraph25f(ctx context.Context) (*arrayGenericViews, *classCompatibility) {
	root := classCompatibility{SourceType: "A", TargetType: "B", Assignable: true, Origin: classAnchor{Source: "/src/main.ts", Start: 1, End: 2}, Members: []classMemberProjection{}}
	r := &arrayGenericViews{p: &arrayGenericProjection{ctx: ctx}, proofs: map[[2]string]arrayGenericCompatibilityWitness{
		{"A", "B"}: {Compatibility: root, Dependencies: []arrayGenericCompatibilityDependency{}},
	}}
	return r, &root
}

func witnessFailure25f(t *testing.T, r *arrayGenericViews, root *classCompatibility, code string) {
	t.Helper()
	err := r.publishWitnesses(root)
	var failure *arrayFactsFailure
	if !errors.As(err, &failure) || failure.Code != code || len(r.p.witnesses) != 0 {
		t.Fatalf("unqualified closure must not publish a partial graph: err=%v records=%v", err, r.p.witnesses)
	}
}

func TestArrayGenericV8CompatibilityGraphBoundaries25f(t *testing.T) {
	t.Run("work", func(t *testing.T) {
		r, root := witnessGraph25f(t.Context())
		r.p.annotationCalls = 99999
		witnessFailure25f(t, r, root, "r03-array-facts-limit")
	})
	t.Run("cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		r, root := witnessGraph25f(ctx)
		err := r.publishWitnesses(root)
		if !errors.Is(err, context.Canceled) || len(r.p.witnesses) != 0 {
			t.Fatalf("canceled closure publication must stop before merge: %v", err)
		}
	})
	for _, differentOrigin := range []bool{false, true} {
		r, root := witnessGraph25f(t.Context())
		record := r.proofs[[2]string{"A", "B"}]
		record.Dependencies = []arrayGenericCompatibilityDependency{{SourceType: "X", TargetType: "Y"}}
		r.proofs[[2]string{"A", "B"}] = record
		if differentOrigin {
			child := *root
			child.SourceType, child.TargetType, child.Origin.Start = "X", "Y", 3
			r.proofs[[2]string{"X", "Y"}] = arrayGenericCompatibilityWitness{Compatibility: child, Dependencies: []arrayGenericCompatibilityDependency{}}
		}
		witnessFailure25f(t, r, root, "r03-array-facts-identity")
	}
}

func TestArrayGenericV8CompatibilitySelectedRoots25f(t *testing.T) {
	r, root := witnessGraph25f(t.Context())
	orphan := *root
	orphan.SourceType, orphan.TargetType = "unused", "candidate"
	r.proofs[[2]string{"unused", "candidate"}] = arrayGenericCompatibilityWitness{Compatibility: orphan, Dependencies: []arrayGenericCompatibilityDependency{}}
	for i := 0; i < 2; i++ {
		if err := r.publishWitnesses(root); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.p.finishWitnesses(); err != nil || len(r.p.payload.CompatibilityWitnesses) != 1 || r.p.payload.CompatibilityWitnesses[0].Compatibility.SourceType != "A" {
		t.Fatalf("failed/unselected candidates must not enter the deduplicated reachable directory: %v", err)
	}
}
