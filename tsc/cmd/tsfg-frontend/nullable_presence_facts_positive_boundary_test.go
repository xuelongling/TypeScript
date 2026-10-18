// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"testing"
)

func TestNullableFactsPresenceNewScalarAndMixedPackets(t *testing.T) {
	for _, row := range nullablePresenceBoundaryPositives(t) {
		t.Run(row.Key, func(t *testing.T) {
			for _, version := range []string{"v3", "v4"} {
				wire, ok := row.Wires[version]
				if !ok {
					continue
				}
				t.Run(version, func(t *testing.T) {
					raw := []byte(wire.Request)
					nullablePresenceBoundaryOfficialAndLegacy(t, raw)
					facts := nullablePresenceBoundaryLegacy(t, raw, []byte(row.Legacy))
					if facts.Status != "ok" {
						t.Fatal("independent complete legacy prerequisite")
					}
					nullablePresenceBoundarySuccess(t, "--nullable-facts-io-"+version, raw, []byte(wire.Packet))
				})
			}
		})
	}
}

func TestNullableFactsPresenceOwnedInputsAndColdCalls(t *testing.T) {
	for _, f := range nullablePresenceFirstFixtures(t) {
		t.Run(f.name, func(t *testing.T) {
			v := nullablePresenceBoundaryObject(t, f.requestBytes)
			files := v["input"].(map[string]any)["files"].([]any)
			for _, value := range files {
				file := value.(map[string]any)
				file["sha256"] = nullablePresenceBoundaryDigest([]byte(file["content"].(string)))
			}
			files[0], files[1] = files[1], files[0]
			raw := nullablePresenceBoundaryJSON(t, v)
			expected := nullablePresenceBoundaryObject(t, f.packageBytes)
			expected["requestId"] = nullablePresenceBoundaryDigest(append([]byte("tsfg.nullable-facts.request.v4\x00"), raw...))
			complete := nullablePresenceBoundaryJSON(t, expected)
			// Sorted input identity, complete facts/file/export table and package ID stay fixed;
			// only the request ID changes with the complete order/checksum-bearing request.
			nullablePresenceBoundarySuccess(t, "--nullable-facts-io-v4", raw, complete)
			_, w, err := nullablePresenceBoundaryCommand(t.Context(), []string{"--nullable-facts-io-v4"}, raw)
			if err != nil || len(w.attempts) != 1 || !bytes.Equal(w.attempts[0], complete) {
				t.Fatalf("owned output: %v", err)
			}
			for i := range w.retained {
				w.retained[i] = 0
			}
			nullablePresenceBoundarySuccess(t, "--nullable-facts-io-v4", f.requestBytes, f.packageBytes)
			nullablePresenceBoundarySuccess(t, "--nullable-facts-io-v4", raw, complete)
		})
	}
	// An invalid cold request between two different sources must not select cached facts.
	fixtures := nullablePresenceFirstFixtures(t)
	for _, i := range []int{3, 0, 2, 1} {
		f := fixtures[i]
		nullablePresenceBoundaryReject(t, []byte("{}"), "schema", nil, 0)
		nullablePresenceBoundarySuccess(t, "--nullable-facts-io-v4", f.requestBytes, f.packageBytes)
	}
}

func TestNullableFactsPresenceExplicitVersionSeparation(t *testing.T) {
	ordinary := nullablePresenceBoundaryPositives(t)[0]
	for _, row := range []struct{ name, flag, request string }{
		{"v3-request-at-v4", "--nullable-facts-io-v4", ordinary.Wires["v3"].Request},
		{"v4-request-at-v3", "--nullable-facts-io-v3", ordinary.Wires["v4"].Request},
	} {
		t.Run(row.name, func(t *testing.T) {
			_, w, err := nullablePresenceBoundaryCommand(t.Context(), []string{row.flag}, []byte(row.request))
			if err == nil || w.writes != 0 {
				t.Fatalf("cross-version request accepted: %v", err)
			}
			if row.flag == "--nullable-facts-io-v4" {
				nullablePresenceBoundaryError(t, err, "schema", nil, 0)
			}
		})
	}
	// These are the four newly authorized guard sources, never an old nullable group.
	for _, f := range nullablePresenceFirstFixtures(t) {
		t.Run("new-guard-stays-outside-v3-"+f.name, func(t *testing.T) {
			v := nullablePresenceBoundaryObject(t, f.requestBytes)
			v["version"], v["schemaHash"] = 3, nullablePresenceBoundaryS3
			raw := nullablePresenceBoundaryJSON(t, v)
			_, w, err := nullablePresenceBoundaryCommand(t.Context(), []string{"--nullable-facts-io-v3"}, raw)
			if err == nil || w.writes != 0 {
				t.Fatalf("v3 widened to guard profile: %v", err)
			}
		})
	}
	input := nullablePresenceBoundaryObject(t, []byte(ordinary.Wires["v3"].Request))["input"]
	nullablePresenceBoundaryReject(t, nullablePresenceBoundaryJSON(t, input), "schema", nil, 0)
}
