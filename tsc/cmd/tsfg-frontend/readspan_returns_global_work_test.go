// SPDX-License-Identifier: Apache-2.0
// Global work boundaries reuse the same independent official and public checks.
package main

import (
	"encoding/base64"
	"testing"
)

var rs26cReturnGlobalWorkPin = rs26cFrozenFixturePin{
	"global1024-1025-expected-facts.json",
	"5b37a5009347bd364f0451cbc7af2371af09cbe1eaa594c56d04f52e47b19a94",
	1989039,
	[]string{"return-global-1024", "return-global-1025"},
}

func TestReadSpan26cOfficialFreshWholeReturnGlobalWork(t *testing.T) {
	fixture := rs26cReadFrozenFixture(t, rs26cReturnGlobalWorkPin)
	for _, item := range fixture.Cases {
		t.Run(item.Name, func(t *testing.T) {
			witness := ao23JCS(t, rs26cOfficialFixtureWitness(t, item))
			t.Logf("TSFG_READSPAN_26C_OFFICIAL_WHOLE_GLOBAL_WORK name=%s bytes=%d sha256=%s base64=%s", item.Name, len(witness), ao23Hash(witness), base64.StdEncoding.EncodeToString(witness))
		})
	}
}

func TestReadSpan26cV7ReturnGlobalWorkPublicWholeFrozen(t *testing.T) {
	rs26cPublicFrozenMatrix(t, []rs26cFrozenFixturePin{rs26cReturnGlobalWorkPin})
}
