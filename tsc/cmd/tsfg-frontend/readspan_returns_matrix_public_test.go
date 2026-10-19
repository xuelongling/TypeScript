// SPDX-License-Identifier: Apache-2.0
// Source publishes full frozen facts; Product alone classifies lifetime and budgets.
package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func rs26cPublicFrozenMatrix(t *testing.T, pins []rs26cFrozenFixturePin) {
	t.Helper()
	for _, pin := range pins {
		t.Run(pin.name, func(t *testing.T) {
			fixture := rs26cReadFrozenFixture(t, pin)
			for _, item := range fixture.Cases {
				t.Run(item.Name, func(t *testing.T) {
					input := []byte(item.RequestText)
					before := bytes.Clone(input)
					var output readSpanParameterPublicWriter26b
					err := runArrayFactsCandidate(t.Context(), bytes.NewReader(input), &output)
					t.Logf("TSFG_READSPAN_26C_MATRIX_PUBLIC_ACTUAL name=%s bytes=%d sha256=%s writes=%d base64=%s", item.Name, output.Len(), readSpanParameterPublicHash26b(output.Bytes()), output.writes, base64.StdEncoding.EncodeToString(output.Bytes()))
					ao23Want(t, bytes.Equal(input, before), "original caller request remains unchanged")
					if err != nil {
						if output.Len() != 0 || output.writes != 0 {
							t.Fatalf("failed publication wrote partial facts: %s", readSpanParameterPublicError26b(err))
						}
						t.Fatalf("ordinary legal Source must publish whole facts before Product classification: %s", readSpanParameterPublicError26b(err))
					}
					ao23Want(t, output.writes == 1 && bytes.Equal(output.Bytes(), []byte(item.ExpectedText)), "one atomic complete publication equals independent literal packet")
				})
			}
		})
	}
}

func TestReadSpan26cV7ReturnBudgetsPublicWholeFrozen(t *testing.T) {
	rs26cPublicFrozenMatrix(t, rs26cFrozenFixturePins[:2])
}

func TestReadSpan26cV7ReturnAdjacentPublicWholeFrozen(t *testing.T) {
	rs26cPublicFrozenMatrix(t, rs26cFrozenFixturePins[2:])
}
