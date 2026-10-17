// SPDX-License-Identifier: Apache-2.0

package main

import (
	stdjson "encoding/json"
	"strings"
	"testing"
)

// This explicitly agreed existing typed budget seam receives only owned DTOs.
// The names/ranges below follow the independent callable-throw fixture's DTO
// shape, but the signature chains are synthetic budget inputs, not captured
// producer responses. No source, Program, checker, parser or executable is used.
// Count signature objects along each path: a root is depth 1; null is no node.
func TestExceptionFactsOwnedSignatureDepthBudget(t *testing.T) {
	for _, tc := range []struct {
		name         string
		depth        int
		sharedPrefix bool
		wantLimit    bool
	}{
		{"signature64Control", 64, false, false},
		{"signature65Limit", 65, false, true},
		{"sharedChain63ThenPrefix64Control", 63, true, false},
		{"sharedChain64ThenPrefix65Limit", 64, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chain := &signature{Parameters: []string{}, ReturnType: "bool"}
			for layer := 1; layer < tc.depth; layer++ {
				chain = &signature{Parameters: []string{}, ReturnType: "function", ReturnSignature: chain}
			}
			returned, maximumDepth := chain, tc.depth
			if tc.sharedPrefix {
				returned = &signature{Parameters: []string{}, ReturnType: "function", ReturnSignature: chain}
				maximumDepth++
			}
			// The real DTO walk reaches function.Parameters before ReturnSignature.
			// Thus parameter.Signature visits chain first at its shallow depth;
			// returned reuses that identical pointer below a fresh prefix. Symbols
			// and the throw occurrence also retain the original shared chain.
			fn := symbol{ID: "s1", Name: "entry", Kind: "function", Type: "function", Writable: false, Range: span{22, 28}}
			parameter := symbol{ID: "s2", Name: "payload", Kind: "parameter", Type: "function", Writable: true, Range: span{29, 36}, Signature: chain}
			facts := response{Status: "ok", Symbols: []symbol{fn, parameter}, Functions: []function{{
				Name: "entry", Symbol: "s1", Parameters: []symbol{parameter}, ReturnType: "function", ReturnSignature: returned, Range: span{0, 80},
				Body: []statement{{Kind: "throw", Range: span{63, 78}, Expression: &expression{
					Kind: "reference", Type: "function", Symbol: "s2", Signature: chain, Range: span{69, 77},
				}}},
			}}}
			if tc.sharedPrefix && (returned.ReturnSignature != facts.Symbols[1].Signature || returned.ReturnSignature != facts.Functions[0].Parameters[0].Signature) {
				t.Fatal("shared-pointer precondition lost the identical shallow signature chain")
			}
			encoded, err := stdjson.Marshal(facts)
			if err != nil || len(encoded) >= 32*1024 {
				t.Fatalf("small complete typed DTO precondition failed: bytes=%d error=%v", len(encoded), err)
			}
			// All signature Parameters are required present empty arrays; the one
			// function parameter is below the existing arity cap. Only signature
			// maximum path depth changes, and encoding stays below JSON depth128.
			err = exceptionFactsCallableBounds(facts)
			t.Logf("TSFG_EXCEPTION_OWNED_SIGNATURE_BUDGET_V1 case=%s chainDepth=%d sharedPrefix=%t maximumSignatureDepth=%d dtoBytes=%d wantLimit=%t error=%v", tc.name, tc.depth, tc.sharedPrefix, maximumDepth, len(encoded), tc.wantLimit, err)
			if tc.wantLimit {
				if err == nil || !strings.HasPrefix(err.Error(), "[r03-exception-facts-limit] ") {
					t.Fatalf("maximum path depth %d must reach the existing typed budget helper and reject with r03-exception-facts-limit: %v", maximumDepth, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("maximum path depth %d control failed: %v", maximumDepth, err)
			}
		})
	}
}
