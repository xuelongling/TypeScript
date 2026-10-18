// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestNullableFactsPresenceJSONBudgets(t *testing.T) {
	for _, row := range []struct {
		name  string
		count int
		kind  string
	}{
		{"string-at", 1048576, "schema"}, {"string-over", 1048577, "limit"},
	} {
		t.Run(row.name, func(t *testing.T) {
			nullablePresenceBoundaryReject(t, []byte("{\"x\":\""+strings.Repeat("a", row.count)+"\"}"), row.kind, nil, map[string]int{"schema": 0, "limit": 1}[row.kind])
		})
	}
	for _, row := range []struct {
		name   string
		arrays int
		kind   string
	}{
		{"depth-at", 127, "schema"}, {"depth-over", 128, "limit"},
	} {
		t.Run(row.name, func(t *testing.T) {
			nullablePresenceBoundaryReject(t, []byte("{\"x\":"+strings.Repeat("[", row.arrays)+"0"+strings.Repeat("]", row.arrays)+"}"), row.kind, nil, map[string]int{"schema": 0, "limit": 1}[row.kind])
		})
	}
	// object open/key/close + array open/close + N primitives = N+5 tokens.
	for _, row := range []struct {
		name   string
		values int
		kind   string
	}{
		{"tokens-at", 199995, "schema"}, {"tokens-over", 199996, "limit"},
	} {
		t.Run(row.name, func(t *testing.T) {
			nullablePresenceBoundaryReject(t, []byte("{\"x\":["+strings.Repeat("0,", row.values-1)+"0]}"), row.kind, nil, map[string]int{"schema": 0, "limit": 1}[row.kind])
		})
	}
	// Every string stays <=1 MiB, with only 18 tokens and depth 1.
	// The final string absorbs exact frame overhead; no Source Program is created.
	frame := func(size int) []byte {
		value := map[string]any{}
		for _, key := range []string{"a", "b", "c", "d", "e", "f", "g"} {
			value[key] = strings.Repeat("a", 1048576)
		}
		value["h"] = ""
		overhead := len(nullablePresenceBoundaryJSON(t, value))
		value["h"] = strings.Repeat("a", size-overhead)
		raw := nullablePresenceBoundaryJSON(t, value)
		if len(raw) != size {
			t.Fatal("independent frame arithmetic")
		}
		return raw
	}
	for _, row := range []struct {
		name string
		size int
		kind string
	}{
		{"bytes-below", 8388607, "schema"}, {"bytes-at", 8388608, "schema"}, {"bytes-over", 8388609, "limit"},
	} {
		t.Run(row.name, func(t *testing.T) {
			nullablePresenceBoundaryReject(t, frame(row.size), row.kind, nil, map[string]int{"schema": 0, "limit": 1}[row.kind])
		})
	}
	t.Run("read-is-bounded-before-preflight", func(t *testing.T) {
		raw := bytes.Repeat([]byte{'x'}, 8388608+100)
		r, w, err := nullablePresenceBoundaryCommand(t.Context(), []string{"--nullable-facts-io-v4"}, raw)
		if r.delivered != 8388609 || w.writes != 0 {
			t.Fatalf("unbounded read or premature write: delivered=%d", r.delivered)
		}
		nullablePresenceBoundaryError(t, err, "limit", nil, 1)
	})
	t.Run("domain-limit-before-noncanonical", func(t *testing.T) {
		raw := []byte(" {\"x\":\"" + strings.Repeat("a", 1048577) + "\"}")
		nullablePresenceBoundaryReject(t, raw, "limit", nil, 1)
	})
}
