// SPDX-License-Identifier: Apache-2.0
// Exact independent facts exercise the existing in-process public IO boundary.
package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"testing"
)

func scc23fPublicWhole(t *testing.T, first bool) {
	t.Helper()
	for index, item := range scc23fReadWhole(t).Cases {
		if (index < 3) != first {
			continue
		}
		t.Run(item.Name, func(t *testing.T) {
			input := []byte(item.RequestText)
			before := bytes.Clone(input)
			var output readSpanParameterPublicWriter26b
			err := runArrayFactsCandidate(t.Context(), bytes.NewReader(input), &output)
			t.Logf("TSFG_ARRAY_OWNER_23F_PUBLIC_ACTUAL name=%s bytes=%d sha256=%s writes=%d base64=%s", item.Name, output.Len(), ao23Hash(output.Bytes()), output.writes, base64.StdEncoding.EncodeToString(output.Bytes()))
			ao23Want(t, bytes.Equal(input, before), "entire original private7 request unchanged")
			if !item.SourceShouldPublish {
				var failure *arrayFactsFailure
				ao23Want(t, errors.As(err, &failure) && failure.Code == "r03-array-facts-limit" && failure.Kind == "limit" && failure.RuleID == "r01-input-boundary" && failure.Phase == "input", "raw481 exact original facts-limit boundary")
				ao23Want(t, output.Len() == 0 && output.writes == 0, "raw481 never publishes partial facts")
				return
			}
			if err != nil {
				ao23Want(t, output.Len() == 0 && output.writes == 0, "rejected publication remains atomic")
				t.Fatalf("ordinary official-valid SCC must publish full independent private7 facts: %s", readSpanParameterPublicError26b(err))
			}
			ao23Want(t, output.writes == 1 && bytes.Equal(output.Bytes(), []byte(item.ExpectedText)), "one whole publication equals exact independent packet")
		})
	}
}

// Phase1 accepts joint7 but retains the original complete cycle guard;
// this exact unchanged assertion yields semantic Source RED, then Phase2 GREEN.
func TestArrayOwner23fV7PublicWholeFirstPair(t *testing.T) { scc23fPublicWhole(t, true) }
func TestArrayOwner23fV7PublicWholeBounds(t *testing.T)    { scc23fPublicWhole(t, false) }
