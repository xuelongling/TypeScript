// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestArrayGenericV8CompatibilityCandidateIsolation25f(t *testing.T) {
	// Exactly three ASCII spaces retain the original field-ID traversal order.
	source := "   class X{constructor(public q:A){}} class Y{constructor(public q:Bad|Good){}} class A{constructor(public child:X,public n:i32){}} class Bad{constructor(public child:Y,public n:f64){}} class Good{constructor(public n:i32){}} class Root{constructor(public x:X,public a:A){}} class View{constructor(public x:Y,public a:Bad|Good){}} export function f(r:Root):View{return r;}"
	input := genericArrayRequest25f(t, source)
	if root := os.Getenv("TSFG_ARRAY_V8_CAPTURE_ROOT"); root != "" {
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string][]byte{"candidate-source.ts": []byte(source), "candidate-request.json": input} {
			if err := os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	p := genericArrayPublicRequest25f(t, input, source)
	genericWitnessClosure25f(t, p, map[string][]string{"Root>View": {"X>Y", "A>Good"}, "X>Y": {"A>Good"}, "A>Good": {}})
	ids, fields := map[string]string{}, map[string]map[string]string{}
	for _, raw := range p["types"].([]any) {
		typ := raw.(map[string]any)
		name, ok := typ["name"].(string)
		if !ok || typ["kind"] != "reference" {
			continue
		}
		ids[name], fields[name] = typ["id"].(string), map[string]string{}
		for _, raw := range typ["fields"].([]any) {
			field := raw.(map[string]any)
			fields[name][field["name"].(string)] = field["id"].(string)
		}
	}
	if ids["Bad"] >= ids["Good"] || fields["View"]["x"] >= fields["View"]["a"] || fields["Bad"]["child"] >= fields["Bad"]["n"] {
		t.Fatal("counterexample type/member order changed; do not evade candidate rollback")
	}
}

func TestArrayGenericV8CompatibilityCandidateBoundaries25f(t *testing.T) {
	for _, cause := range []string{"mismatch", "cancel", "work"} {
		t.Run(cause, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			r, root := witnessGraph25f(ctx)
			outer := r.beginProofCandidate()
			changed := *root
			changed.Origin.Start = 3
			if err := r.storeProof([2]string{"A", "B"}, arrayGenericCompatibilityWitness{Compatibility: changed}); err != nil {
				t.Fatal(err)
			}
			inner := r.beginProofCandidate()
			if err := r.storeProof([2]string{"X", "Y"}, arrayGenericCompatibilityWitness{Compatibility: changed}); err != nil {
				t.Fatal(err)
			}
			r.finishProofCandidate(inner, true)
			var err error = &arrayGenericRelationMismatch{numeric: true}
			if cause == "cancel" {
				cancel()
				err = r.p.annotationWork()
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("actual cancellation not reached: %v", err)
				}
			} else if cause == "work" {
				r.p.annotationCalls = 100000
				err = r.storeProof([2]string{"denied", "write"}, arrayGenericCompatibilityWitness{Compatibility: changed})
				var failure *arrayFactsFailure
				if !errors.As(err, &failure) || failure.Code != "r03-array-facts-limit" {
					t.Fatalf("actual work boundary not reached: %v", err)
				}
			}
			r.finishProofCandidate(outer, err == nil)
			if len(r.proofs) != 1 || r.proofs[[2]string{"A", "B"}].Compatibility.Origin != root.Origin || r.proofScopes != 0 || len(r.proofUndo) != 0 {
				t.Fatalf("failed parent leaked committed child or overwritten prior proof: %+v", r)
			}
			if r.p.annotationCalls < 2 {
				t.Fatal("rollback must not refund charged candidate work")
			}
		})
	}
	t.Run("selected-parent-rejected-child", func(t *testing.T) {
		r, root := witnessGraph25f(t.Context())
		outer := r.beginProofCandidate()
		selected := *root
		selected.Origin.Start = 3
		if err := r.storeProof([2]string{"A", "B"}, arrayGenericCompatibilityWitness{Compatibility: selected}); err != nil {
			t.Fatal(err)
		}
		inner := r.beginProofCandidate()
		if err := r.storeProof([2]string{"A", "B"}, arrayGenericCompatibilityWitness{Compatibility: *root}); err != nil {
			t.Fatal(err)
		}
		r.finishProofCandidate(inner, false)
		r.finishProofCandidate(outer, true)
		if len(r.proofs) != 1 || r.proofs[[2]string{"A", "B"}].Compatibility.Origin != selected.Origin || r.proofScopes != 0 || len(r.proofUndo) != 0 {
			t.Fatal("rejected child lost selected parent or successful scope leaked undo state")
		}
	})
}
