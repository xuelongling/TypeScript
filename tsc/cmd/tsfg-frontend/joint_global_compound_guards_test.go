// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

// Reject through the same public command seam. These controls distinguish the
// supported source-owned writable global from const/import/nominal violations;
// every diagnostic must retain its exact upstream identity, rule and anchor.
func TestJointGlobalCompoundKeepsOfficialGuards(t *testing.T) {
	cases := []struct {
		name, app, state, marker string
		upstream                 int32
		code, rule               string
	}{
		{"const_global", "export const count:i32=1; export function entry():i32 { count += 2; return count; }", "", "count += 2", 2588, "r02b-d-type", "r01-ts-baseline"},
		{"named_import", "import {count} from './state'; export function entry():i32 { count += 2; return count; }", "export let count:i32=1;", "count += 2", 2632, "r02b-d-type", "r01-ts-baseline"},
		{"namespace_readonly", "import * as state from './state'; export function entry():i32 { state.count += 2; return state.count; }", "export let count:i32=1;", "count += 2", 2540, "r02b-d-type", "r01-ts-baseline"},
		{"nominal_i64_rhs", "export let count:i32=1; export function entry(delta:i64):i32 { count += delta; return count; }", "", "count += delta", 2322, "r01-d-scalar-type", "r01-literals"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sources := []inputFile{{Path: "/src/app.ts", Content: tc.app}}
			if tc.state != "" {
				sources = append(sources, inputFile{Path: "/src/state.ts", Content: tc.state})
			}
			result, raw := jointCommandForTest(t, integerRequest(""), sources, "/src/app.ts")
			if result.Status != "error" || len(result.Diagnostics) != 1 {
				t.Fatalf("expected one preserved official guard, got: %s", raw)
			}
			start := strings.Index(tc.app, tc.marker)
			if start < 0 {
				t.Fatal("test-owned diagnostic marker is missing")
			}
			d := result.Diagnostics[0]
			if d.UpstreamCode != tc.upstream || d.Code != tc.code || d.RuleID != tc.rule || len(d.RuleIDs) != 1 || d.RuleIDs[0] != tc.rule || d.Phase != "types" || d.Category != "error" || d.Severity != "error" || d.File != "/src/app.ts" || d.Range.Start != start || d.Range.End != start+len("count") || d.Primary.SourceID != "/src/app.ts" || d.Primary.Start != start || d.Primary.End != start+len("count") || d.Primary.StartLine != 0 || d.Primary.EndLine != 0 || d.Primary.StartColumn != start || d.Primary.EndColumn != start+len("count") {
				t.Fatalf("official guard identity/rule/count anchor changed: %s", raw)
			}
		})
	}
}

// Shared non-external scripts use one official global binding domain. This
// pair distinguishes that domain from imported aliases and external modules.
func TestJointSharedScriptGlobalCompound(t *testing.T) {
	cases := []struct {
		name, app, state, owner string
		modules                 int
	}{
		{"same_script", "let count:i32=1; function entry():i32 { count += 2; return count; }", "", "/src/app.ts", 1},
		{"shared_scripts", "function entry():i32 { count += 2; return count; }", "let count:i32=1;", "/src/state.ts", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sources := []inputFile{{Path: "/src/app.ts", Content: tc.app}}
			if tc.state != "" {
				sources = append(sources, inputFile{Path: "/src/state.ts", Content: tc.state})
			}
			result, raw := jointCommandForTest(t, integerRequest(""), sources, "/src/app.ts")
			if result.Status != "ok" || len(result.Diagnostics) != 0 || len(result.Modules) != tc.modules || len(result.Functions) != 1 {
				t.Fatalf("official shared-script global compound rejected: %s", raw)
			}
			global := ""
			for _, s := range result.Symbols {
				if s.Name == "count" && s.SourcePath == tc.owner && s.BindingKind == "source" {
					global = s.ID
				}
			}
			if global == "" {
				t.Fatal("shared-script global lost its declaring source owner")
			}
			ownerHasGlobal := false
			for _, m := range result.Modules {
				if m.SourcePath == tc.owner {
					for _, id := range m.Globals {
						ownerHasGlobal = ownerHasGlobal || id == global
					}
				}
			}
			fn := result.Functions[0]
			if !ownerHasGlobal || fn.Name != "entry" || fn.SourcePath != "/src/app.ts" || len(fn.Parameters) != 0 || len(fn.Body) != 2 || fn.Body[0].Expression == nil || fn.Body[1].Expression == nil {
				t.Fatal("script/function/declaration ownership changed")
			}
			write, returned := fn.Body[0].Expression, fn.Body[1].Expression
			if fn.Body[0].Kind != "expression" || fn.Body[1].Kind != "return" || write.Kind != "write" || write.Operator != "+=" || write.Type != "i32" || write.Left == nil || write.Left.Kind != "reference" || write.Left.Symbol != global || write.Left.Type != "i32" || write.Right == nil || write.Right.Type != "i32" || write.Right.Literal == nil || write.Right.Literal.Decimal != "2" || returned.Kind != "reference" || returned.Symbol != global || returned.Type != "i32" {
				t.Fatal("shared-script write/return no longer targets the one owned global and integer2")
			}
		})
	}
}
