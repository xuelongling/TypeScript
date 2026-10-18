// SPDX-License-Identifier: Apache-2.0
// Canonical framing owner unit evidence only; no public Source reachability claim.
package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestNullableFactsPresenceCanonicalOutputByteBudget(t *testing.T) {
	for _, row := range []struct {
		name string
		size int
		sha  string
	}{
		{"below", 8388607, "sha256:52af84ae3562e62f38c2c4eee430e1500307048d29facc9b0c9baad316709049"},
		{"at", 8388608, "sha256:7c95fea8f71074ef86cd7916344a9c90b862a4031823d132681814079db3bf45"},
		{"over", 8388609, "sha256:d3e6d2e512b0e2832725847fad650c86c19f1fd9c2e35cbf8b052bf1f48c6bdc"},
	} {
		t.Run(row.name, func(t *testing.T) {
			graph := map[string]any{}
			for _, key := range []string{"a", "b", "c", "d", "e", "f", "g"} {
				graph[key] = strings.Repeat("a", 1048576)
			}
			graph["h"] = ""
			overhead := len(nullablePresenceBoundaryJSON(t, graph))
			graph["h"] = strings.Repeat("a", row.size-overhead)
			expected := nullablePresenceBoundaryJSON(t, graph)
			if len(expected) != row.size || nullablePresenceBoundaryDigest(expected) != row.sha {
				t.Fatal("independent full frame identity changed")
			}
			// This is the current natural shared encoder called by both real runners.
			actual, err := nullableFactsCanonical(graph, nullableFactsV4)
			if row.size <= 8388608 {
				if err != nil || !bytes.Equal(actual, expected) {
					t.Fatalf("canonical exact byte boundary: %v bytes=%d", err, len(actual))
				}
			} else {
				if len(actual) != 0 {
					t.Fatal("canonical owner returned a partial over-budget frame")
				}
				got := nullablePresenceBoundaryError(t, err, "limit", nil, 1)
				var cause *genericFailure
				if !errors.As(got, &cause) || cause.Kind != "package-limit" {
					t.Fatal("canonical byte limit lost actual generic cause")
				}
			}
		})
	}
}
