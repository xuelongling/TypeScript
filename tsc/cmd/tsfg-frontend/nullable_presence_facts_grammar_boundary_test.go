// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

type nullablePresenceBoundaryGrammar struct {
	Key, Source, Kind, AnchorText, LegacyCode string
	Anchor                                    *span
}

func nullablePresenceBoundaryGrammarRows(t *testing.T) []nullablePresenceBoundaryGrammar {
	t.Helper()
	var rows []nullablePresenceBoundaryGrammar
	if err := json.Unmarshal(nullablePresenceBoundaryRead(t, "grammar.json"), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 22 {
		t.Fatal("independent grammar corpus changed")
	}
	return rows
}

func TestNullableFactsPresenceGuardAndExportBoundary(t *testing.T) {
	for _, row := range nullablePresenceBoundaryGrammarRows(t) {
		if row.Kind == "source" {
			continue
		}
		t.Run(row.Key, func(t *testing.T) {
			raw := nullablePresenceBoundaryRequest(t, row.Source, 4)
			// Admission is real: a private grammar rejection must follow successful
			// complete official diagnostics and successful unchanged legacy projection.
			nullablePresenceBoundaryOfficialAndLegacy(t, raw)
			if row.Anchor != nil {
				text := nullablePresenceFirstSourceSlice(t, row.Source, [2]int{row.Anchor.Start, row.Anchor.End})
				if text != row.AnchorText {
					t.Fatal("independent raw UTF16 anchor changed")
				}
			}
			nullablePresenceBoundaryReject(t, raw, row.Kind, row.Anchor, 0)
		})
	}
}

func TestNullableFactsPresenceSourceDiagnosticPriority(t *testing.T) {
	for _, row := range nullablePresenceBoundaryGrammarRows(t) {
		if row.Kind != "source" {
			continue
		}
		t.Run(row.Key, func(t *testing.T) {
			raw := nullablePresenceBoundaryRequest(t, row.Source, 4)
			r := nullablePresenceBoundaryInput(t, raw)
			program := nullablePresenceFirstProgram(t, r)
			if len(program.GetSyntacticDiagnostics(t.Context(), nil)) != 0 || len(program.GetProgramDiagnostics()) != 0 || len(program.GetGlobalDiagnostics(t.Context())) != 0 {
				t.Fatal("unexpected official nonsemantic diagnostics")
			}
			semantic := program.GetSemanticDiagnostics(t.Context(), nil)
			if row.LegacyCode == "r01-d-scalar-type" {
				if len(semantic) != 1 || semantic[0].Code() != 2322 {
					t.Fatalf("official unsafe return prerequisite changed: %+v", semantic)
				}
			} else if len(semantic) != 0 {
				t.Fatalf("legacy operator admission case acquired official diagnostic: %+v", semantic)
			}
			legacy := nullablePresenceBoundaryLegacy(t, raw, nil)
			if legacy.Status == "ok" || len(legacy.Diagnostics) == 0 || legacy.Diagnostics[0].Code != row.LegacyCode {
				t.Fatalf("original legacy rejection changed: %+v", legacy)
			}
			first := legacy.Diagnostics[0]
			if first.File != "/src/main.ts" || first.Range.End <= first.Range.Start {
				t.Fatal("legacy diagnostic lost original source anchor")
			}
			if row.Anchor != nil && first.Range != *row.Anchor {
				t.Fatalf("literal operator anchor differs: got=%+v expected=%+v", first.Range, *row.Anchor)
			}
			if row.LegacyCode == "r01-d-scalar-type" {
				ds := semantic[0]
				file := ds.File()
				if file == nil || first.UpstreamCode != ds.Code() || first.Range.Start != file.GetPositionMap().UTF8ToUTF16(ds.Pos()) || first.Range.End != file.GetPositionMap().UTF8ToUTF16(ds.End()) {
					t.Fatal("legacy diagnostic no longer preserves independent official code/range")
				}
			}
			got := nullablePresenceBoundaryReject(t, raw, "source", &first.Range, 0)
			if !strings.Contains(got.Detail, first.Code) {
				t.Fatal("Source error lost trusted first legacy Code")
			}
		})
	}
}
