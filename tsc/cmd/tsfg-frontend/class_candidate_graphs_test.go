// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/json"
)

// This produces candidate bytes from the real official Program-to-owned seam.
// The helper clears the original VFS before returning only the owned graph.
// It does not claim an adopted RPC, admitted SourceEXE, or process-closure proof.
func TestOwnedClassSourceCanonicalCandidateGraphs(t *testing.T) {
	for _, fixture := range []struct{ name, source string }{
		{"constructor", strings.TrimSuffix(classCounterSource, " }") + " add(delta: number): number { this.value = this.value + delta; return this.value; } }"},
		{"inheritance", classInheritanceSource},
		{"readonly", classReadonlySource},
		{"implicit-derived", classImplicitConstructorSource},
		{"construct", classConstructSource},
		{"reference-identity", classReferenceIdentitySource},
		{"scalars", classScalarSource},
		{"literal-facts", classLiteralFactsSource},
		{"implicit-middle", classDefaultMiddleSource},
		{"locals-const", classLocalConstSource},
		{"locals-let", classLocalLetSource},
		{"multiple-sources", classMultiMainSource},
	} {
		project := func() classGraph {
			if fixture.name == "multiple-sources" {
				graph, err := classMultiSourceExport(t, fixture.source, classMultiBaseSource, false)
				if err != nil {
					t.Fatal(err)
				}
				return graph
			}
			if fixture.name == "scalars" {
				graph, _ := classScalarGraphForTest(t)
				return graph
			}
			return classSourceForTest(t, fixture.source)
		}
		first, err := json.Marshal(project())
		if err != nil {
			t.Fatal(err)
		}
		canonicalFirst := json.Value(first)
		if err := canonicalFirst.Canonicalize(); err != nil {
			t.Fatal(err)
		}
		second, err := json.Marshal(project())
		if err != nil {
			t.Fatal(err)
		}
		canonicalSecond := json.Value(second)
		if err := canonicalSecond.Canonicalize(); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(canonicalFirst, canonicalSecond) {
			t.Fatalf("%s owned facts differ across fresh official Programs", fixture.name)
		}
		fmt.Printf("TSFG_CLASS_OWNED_CANDIDATE_V5 %s %s %s\n", fixture.name,
			classTextSHA(string(canonicalFirst)), base64.StdEncoding.EncodeToString(canonicalFirst))
	}
}
