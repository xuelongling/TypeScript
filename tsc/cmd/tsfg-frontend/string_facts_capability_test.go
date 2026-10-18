// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"github.com/microsoft/typescript-go/internal/ast"
	"testing"
)

func TestStringFactsUnsupportedAndDiagnosticPriority(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"template", "export function f(): string { return `ok`; }"},
		{"concatenation", "export function f(): string { return \"a\" + \"b\"; }"},
		{"localReference", "export function f(): string { const value = \"a\"; return value; }"},
		{"parameter", "export function f(value: string): string { return value; }"},
		{"unselectedDeclaration", "export function f(): string { return \"ok\"; } const unused = 1;"},
		{"script", "function f(): string { return \"ok\"; }"},
		{"controlFlow", "export function f(): string { if (true) return \"x\"; return \"y\"; }"},
		{"generic", "export function f<T>(): string { return \"ok\"; }"},
		{"arrow", "export const f = (): string => \"ok\";"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := stringBoundaryRequest24(t, tc.source)
			var r struct{ Input request }
			if err := json.Unmarshal(input, &r); err != nil {
				t.Fatal(err)
			}
			official := throwBoundaryProgramForTest(t, r.Input)
			throwBoundaryDiagnosticsForTest(t, official)
			failure := stringBoundaryFailure24(t, input, "capability")
			if failure.Code != "r03-string-facts-capability" || failure.RuleID != "r01-ts-baseline" || failure.Phase != "capability" || len(failure.Diagnostics) != 1 || failure.Diagnostics[0].Range.End <= failure.Diagnostics[0].Range.Start {
				t.Fatalf("capability identity/anchor: %+v", failure)
			}
		})
	}
	for _, tc := range []struct {
		name, source string
		code         int32
	}{
		{"returnType", "export function f(): string { return 1; }", 2322},
		{"unresolvedReturn", "export function f(): string { return missing; }", 2304},
		{"unselectedTypeError", "const unselected: string = 1; export function f(): string { return 'ok'; }", 2322},
		{"syntaxBeforeCapability", `export function f(): string { return "unterminated; }`, 1002},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := stringBoundaryRequest24(t, tc.source)
			var r struct{ Input request }
			if err := json.Unmarshal(input, &r); err != nil {
				t.Fatal(err)
			}
			official := throwBoundaryProgramForTest(t, r.Input)
			groups := [][]*ast.Diagnostic{official.GetSyntacticDiagnostics(t.Context(), nil), official.GetSemanticDiagnostics(t.Context(), nil), official.GetProgramDiagnostics(), official.GetGlobalDiagnostics(t.Context())}
			all := []*ast.Diagnostic{}
			found := false
			for _, group := range groups {
				all = append(all, group...)
				for _, d := range group {
					if d.Code() == tc.code {
						found = true
					}
				}
			}
			if !found {
				t.Fatalf("independent official diagnostic %d missing", tc.code)
			}
			failure := stringBoundaryFailure24(t, input, "diagnostic")
			if len(failure.Diagnostics) != len(all) {
				t.Fatal("official diagnostics were filtered")
			}
			for _, owned := range failure.Diagnostics {
				matched := false
				for _, d := range all {
					if d.File() != nil && owned.UpstreamCode == d.Code() && owned.File == d.File().FileName() {
						positions := d.File().GetPositionMap()
						if owned.Range.Start == positions.UTF8ToUTF16(d.Pos()) && owned.Range.End == positions.UTF8ToUTF16(d.End()) && owned.Primary.Start == owned.Range.Start && owned.Primary.End == owned.Range.End && owned.RuleID == "r01-ts-baseline" {
							matched = true
						}
					}
				}
				if !matched {
					t.Fatalf("official code/range/primary lost: %+v", owned)
				}
			}
		})
	}
}
