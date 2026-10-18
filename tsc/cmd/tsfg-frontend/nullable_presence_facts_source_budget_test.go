// SPDX-License-Identifier: Apache-2.0
// These three tops require Root static qualification before selection. They use
// only the exact fresh scalar-only literals frozen in source-domain.json.
package main

import (
	"encoding/json"
	"testing"
)

type nullablePresenceBoundaryDomain struct {
	Key, Qualification, Source, SourceSha256, Request, Legacy, Packet, ExpectedKind string
	SourceUtf8Bytes, SourceUtf16Length, ExpectedCause, PayloadJSONTokens            int
	Anchor                                                                          *span
	Counts                                                                          struct{ Functions, Symbols, ParametersPerFunction, Guards, ExtraLocals int }
}

func nullablePresenceBoundaryDomainRows(t *testing.T) []nullablePresenceBoundaryDomain {
	t.Helper()
	var rows []nullablePresenceBoundaryDomain
	if err := json.Unmarshal(nullablePresenceBoundaryRead(t, "source-domain.json"), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 6 {
		t.Fatal("independent domain corpus changed")
	}
	return rows
}
func nullablePresenceBoundaryDomainCases(t *testing.T, first int) {
	t.Helper()
	rows := nullablePresenceBoundaryDomainRows(t)
	for _, row := range rows[first : first+2] {
		t.Run(row.Key, func(t *testing.T) {
			if row.Qualification != "NOT_SELECTED_PENDING_ROOT_STATIC_QUALIFICATION" {
				t.Fatal("preparation metadata changed")
			}
			if len(row.Source) != row.SourceUtf8Bytes || len(row.Source) != row.SourceUtf16Length || nullablePresenceBoundaryDigest([]byte(row.Source)) != row.SourceSha256 {
				t.Fatal("exact ASCII Source identity changed")
			}
			raw := []byte(row.Request)
			if nullablePresenceBoundaryInput(t, raw).Files[0].Content != row.Source {
				t.Fatal("request does not contain selected complete Source")
			}
			nullablePresenceBoundaryOfficialAndLegacy(t, raw)
			actual := nullablePresenceBoundaryLegacy(t, raw, []byte(row.Legacy))
			if len(actual.Functions) != row.Counts.Functions || len(actual.Symbols) != row.Counts.Symbols {
				t.Fatal("independent exact domain counts differ")
			}
			for _, fn := range actual.Functions {
				if len(fn.Parameters) != row.Counts.ParametersPerFunction {
					t.Fatal("independent parameter count differs")
				}
			}
			if row.ExpectedKind == "" {
				if row.Packet == "" {
					t.Fatal("positive domain boundary requires complete independent packet")
				}
				nullablePresenceBoundarySuccess(t, "--nullable-facts-io-v4", raw, []byte(row.Packet))
			} else {
				nullablePresenceBoundaryReject(t, raw, row.ExpectedKind, row.Anchor, row.ExpectedCause)
			}
		})
	}
}
func TestNullableFactsPresenceSourceFunctionBudget(t *testing.T) {
	nullablePresenceBoundaryDomainCases(t, 0)
}
func TestNullableFactsPresenceSourceParameterBudget(t *testing.T) {
	nullablePresenceBoundaryDomainCases(t, 2)
}
func TestNullableFactsPresenceSourceSymbolBudget(t *testing.T) {
	// At 4096 symbols, the separately counted complete payload has 363400 JSON
	// tokens. Domain count admits it; downstream JSON budget rejects with cause.
	// At 4097, the domain rejects first without an upstream cause. No successful
	// 4096-symbol public packet is claimed, and no test-only helper is introduced.
	nullablePresenceBoundaryDomainCases(t, 4)
}
