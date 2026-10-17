// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	stdjson "encoding/json"
	"strings"
	"testing"
)

// This is the explicitly agreed existing any-to-bounded-JCS seam only.
// It creates no Source Program/checker and grants no source-token, signature,
// whole-program, executable or Runtime resource-limit credit. The literal
// descriptor bound is arrayItems=4096 under schema sha256:5221b50822965c633418462b28a9b58bfbc41b01e0fa4b6d31c50ab83805424d.
func TestExceptionFactsCanonicalArrayItemBudget(t *testing.T) {
	for _, tc := range []struct {
		name      string
		items     int
		wantLimit bool
	}{
		{"array4096Control", 4096, false},
		{"array4097Limit", 4097, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := make([]any, tc.items)
			for i := range value {
				if i%2 == 0 {
					value[i] = true
				} // Odd entries remain literal JSON null.
			}
			// Independent worked literal: 2048 true/null pairs, then one extra
			// true only in the rejecting case. No producer/JSON encoder builds it.
			literal := "[" + strings.TrimSuffix(strings.Repeat("true,null,", 2048), ",")
			if tc.items == 4097 {
				literal += ",true"
			}
			literal += "]"
			want := []byte(literal)
			// Both valid JSON values have depth 1, no string payload, 4098/4099
			// JSON tokens and 20481/20486 bytes: all global budgets are well below
			// depth128/tokens200000/bytes8388608. Only per-array count differs.
			if !stdjson.Valid(want) || strings.Count(literal, ",")+1 != tc.items || len(want) >= 32*1024 {
				t.Fatal("independent small literal precondition failed")
			}
			got, err := exceptionFactsCanonical(value)
			t.Logf("TSFG_EXCEPTION_CANONICAL_ARRAY_BUDGET_V1 case=%s items=%d inputBytes=%d wantLimit=%t outputBytes=%d error=%v", tc.name, tc.items, len(want), tc.wantLimit, len(got), err)
			if tc.wantLimit {
				if err == nil || !strings.HasPrefix(err.Error(), "[r03-exception-facts-limit] ") {
					t.Fatalf("4097 items must reach the existing helper and reject with r03-exception-facts-limit: err=%v outputBytes=%d", err, len(got))
				}
				if len(got) != 0 {
					t.Fatal("rejected canonical value exposed output bytes")
				}
				return
			}
			if err != nil {
				t.Fatalf("4096-item exact-bound control failed: %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("exact canonical control differs: got=%s want=%s", got, want)
			}
		})
	}
}
