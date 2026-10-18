// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
)

func TestStringFactsV2UnsupportedAndDiagnosticPriority(t *testing.T) {
	data := stringConcatOracleRead24e(t, "boundaries.json", "385c5ee6d0078af1ce20e61c1b0c62428f69a4cc48925f5e8d0cfd06dfa3c3ab")
	var vectors struct {
		Unsupported []struct {
			Name, Source, AnchorSource string
			Anchor                     stringOracleRange24
		}
	}
	if json.Unmarshal(data, &vectors) != nil || len(vectors.Unsupported) != 32 {
		t.Fatal("independent unsupported corpus")
	}
	for _, tc := range vectors.Unsupported {
		t.Run(tc.Name, func(t *testing.T) {
			input := stringConcatRequest24e(t, tc.Source)
			r := stringConcatInput24e(t, input)
			official := throwBoundaryProgramForTest(t, r)
			throwBoundaryDiagnosticsForTest(t, official)
			failure := stringConcatFailure24e(t, input, "capability")
			if failure.RuleID != "r01-ts-baseline" || failure.Phase != "capability" || len(failure.Diagnostics) != 1 {
				t.Fatalf("capability identity: %+v", failure)
			}
			d := failure.Diagnostics[0]
			if d.Code != "r03-string-facts-capability" || d.RuleID != "r01-ts-baseline" || d.Phase != "capability" || d.File != r.RootFile || d.Range.Start != tc.Anchor.Start || d.Range.End != tc.Anchor.End || d.Primary.SourceID != r.RootFile || d.Primary.Start != tc.Anchor.Start || d.Primary.End != tc.Anchor.End || d.Range.End <= d.Range.Start || d.Primary != (primary{SourceID: r.RootFile, Start: tc.Anchor.Start, End: tc.Anchor.End, StartColumn: tc.Anchor.Start, EndColumn: tc.Anchor.End}) {
				t.Fatalf("actual unsupported owner anchor, want %s %+v: %+v", tc.AnchorSource, tc.Anchor, d)
			}
		})
	}
	for _, tc := range []struct {
		name, source string
		code         int32
	}{
		{"returnNumber", "export function f(): string { return 1 + 2; }", 2322},
		{"unresolvedOperand", "export function f(): string { return missing + \"b\"; }", 2304},
		{"unselectedTypeError", "const unselected: string = 1; export function f(): string { return \"a\" + \"b\"; }", 2322},
		{"syntaxBeforeCapability", "export function f(): string { return \"unterminated; }", 1002},
		{"returnNewline", "export function f(): string { return\n\"a\" + \"b\"; }", 2322},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := stringConcatRequest24e(t, tc.source)
			r := stringConcatInput24e(t, input)
			official := throwBoundaryProgramForTest(t, r)
			// Build the entire expectation directly from the four official groups.
			// No string-facts diagnostic converter supplies expected envelope fields.
			all := []diagnostic{}
			groups := []struct {
				phase string
				items []*ast.Diagnostic
			}{
				{"syntax", official.GetSyntacticDiagnostics(t.Context(), nil)},
				{"types", official.GetSemanticDiagnostics(t.Context(), nil)},
				{"types", official.GetProgramDiagnostics()},
				{"types", official.GetGlobalDiagnostics(t.Context())},
			}
			found := false
			for _, group := range groups {
				for _, d := range group.items {
					name, where, location := r.RootFile, span{}, primary{SourceID: r.RootFile}
					if file := d.File(); file != nil {
						name = file.FileName()
						positions := file.GetPositionMap()
						start, end := positions.UTF8ToUTF16(d.Pos()), positions.UTF8ToUTF16(d.End())
						where = span{Start: start, End: end}
						// These fixtures and the pinned DTS are ASCII with LF only.
						sp, ep := file.Text()[:d.Pos()], file.Text()[:d.End()]
						location = primary{SourceID: name, Start: start, End: end, StartLine: strings.Count(sp, "\n"), StartColumn: len(sp) - strings.LastIndex(sp, "\n") - 1, EndLine: strings.Count(ep, "\n"), EndColumn: len(ep) - strings.LastIndex(ep, "\n") - 1}
					}
					all = append(all, diagnostic{Code: "r03-string-facts-" + group.phase, RuleID: "r01-ts-baseline", RuleIDs: []string{"r01-ts-baseline"}, Category: "error", Phase: group.phase, Severity: "error", File: name, Range: where, Primary: location, UpstreamCode: d.Code()})
					if d.Code() == tc.code {
						found = true
					}
				}
			}
			if !found {
				t.Fatalf("independent official code %d missing: %+v", tc.code, all)
			}
			// The existing contract orders by file/start/phase/end/upstream code;
			// group collection order alone does not determine the envelope phase.
			slices.SortStableFunc(all, func(a, b diagnostic) int {
				if n := strings.Compare(a.File, b.File); n != 0 {
					return n
				}
				if a.Range.Start != b.Range.Start {
					return a.Range.Start - b.Range.Start
				}
				if n := strings.Compare(a.Phase, b.Phase); n != 0 {
					return n
				}
				if a.Range.End != b.Range.End {
					return a.Range.End - b.Range.End
				}
				return int(a.UpstreamCode - b.UpstreamCode)
			})
			failure := stringConcatFailure24e(t, input, "diagnostic")
			if failure.Code != "r03-string-facts-"+all[0].Phase || failure.Phase != all[0].Phase || failure.RuleID != all[0].RuleID || failure.RuleID != "r01-ts-baseline" || len(failure.Diagnostics) != len(all) {
				t.Fatalf("official first sorted phase/envelope or complete count lost: %+v", failure)
			}
			for index, expected := range all {
				if !reflect.DeepEqual(failure.Diagnostics[index], expected) {
					t.Fatalf("ordered complete official diagnostic %d mismatch\nactual=%+v\nexpected=%+v", index, failure.Diagnostics[index], expected)
				}
			}
		})
	}
}
