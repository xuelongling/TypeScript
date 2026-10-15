// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"testing"

	"github.com/microsoft/typescript-go/internal/json"
)

// These test-owned inputs use the same public runCommand boundary as the
// original namespace/default regression. Each parent edge changes one semantic
// factor. Root runs each subtest separately before selecting a production fix.
func TestJointDefaultCompoundTightLoop(t *testing.T) {
	const state = "export let count: i32 = 1; export function increment(delta: i32 = 2): i32 { count += delta; return count; }"
	const namespace = "import * as state from './state'; export function entry(): i32 { return state.increment(); }"
	const named = "import {increment as step} from './state'; export function entry(): i32 { return step(); }"
	const oneGlobal = state + " export function entry(): i32 { return increment(); }"
	const oneLocal = "export function increment(delta: i32 = 2): i32 { let count: i32 = 1; count += delta; return count; } export function entry(): i32 { return increment(); }"
	files := func(app, stateSource string) []inputFile {
		return []inputFile{{Path: "/src/app.ts", Content: app}, {Path: "/src/state.ts", Content: stateSource}}
	}
	cases := []struct {
		name, parent, change string
		sources              []inputFile
		singleSource         bool
		defaultParameter     bool
		compound             bool
	}{
		{
			name: "01_original_namespace_barrel", parent: "original regression", change: "none",
			sources: []inputFile{
				{Path: "/src/app.ts", Content: "import * as state from './barrel'; export function entry(): i32 { return state.step(); }"},
				{Path: "/src/state.ts", Content: state},
				{Path: "/src/barrel.ts", Content: "export {increment as step, count} from './state';"},
			},
			defaultParameter: true, compound: true,
		},
		{name: "02_direct_namespace", parent: "01", change: "remove reexport indirection", sources: files(namespace, state), defaultParameter: true, compound: true},
		{name: "03_direct_named", parent: "02", change: "namespace reference becomes a named import", sources: files(named, state), defaultParameter: true, compound: true},
		{name: "04_one_module_global", parent: "03", change: "co-locate source-owned declarations and call, removing the import edge", sources: []inputFile{{Path: "/src/app.ts", Content: oneGlobal}}, defaultParameter: true, compound: true},
		{name: "05_explicit_argument", parent: "03", change: "supply the existing default value explicitly", sources: files("import {increment as step} from './state'; export function entry(): i32 { return step(2); }", state), defaultParameter: true, compound: true},
		{name: "06_required_parameter", parent: "05", change: "remove only the parameter initializer", sources: files("import {increment as step} from './state'; export function entry(): i32 { return step(2); }", "export let count: i32 = 1; export function increment(delta: i32): i32 { count += delta; return count; }"), compound: true},
		{name: "07_literal_rhs", parent: "03", change: "replace the compound RHS parameter reference with literal 2", sources: files(named, "export let count: i32 = 1; export function increment(delta: i32 = 2): i32 { count += 2; return count; }"), defaultParameter: true, compound: true},
		{name: "08_simple_assignment", parent: "03", change: "spell the same write as simple assignment and addition", sources: files(named, "export let count: i32 = 1; export function increment(delta: i32 = 2): i32 { count = count + delta; return count; }"), defaultParameter: true},
		{name: "09_one_module_local", parent: "04", change: "move count from module global to function local", sources: []inputFile{{Path: "/src/app.ts", Content: oneLocal}}, defaultParameter: true, compound: true},
		{name: "10_single_source_local", parent: "09", change: "omit only compilationClosure from the same one-file request", sources: []inputFile{{Path: "/src/app.ts", Content: oneLocal}}, singleSource: true, defaultParameter: true, compound: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := integerRequest("")
			var result jointFactsForTest
			var raw []byte
			if tc.singleSource {
				r.RootFile, r.Files[0] = tc.sources[0].Path, tc.sources[0]
				result, raw = defaultCompoundSingleCommandForTest(t, r)
			} else {
				result, raw = jointCommandForTest(t, r, tc.sources, "/src/app.ts")
			}
			if result.Status != "ok" || len(result.Diagnostics) != 0 {
				t.Fatalf("compound/default regression parent=%s change=%s response=%s", tc.parent, tc.change, raw)
			}
			found := false
			for _, fn := range result.Functions {
				if fn.Name != "increment" {
					continue
				}
				found = true
				if len(fn.Parameters) != 1 || fn.Parameters[0].Name != "delta" || fn.Parameters[0].Type != "i32" || fn.Parameters[0].Presence == nil {
					t.Fatal("checked i32 parameter ownership/presence was lost")
				}
				presence := fn.Parameters[0].Presence
				if presence.Default != tc.defaultParameter || presence.Optional != tc.defaultParameter {
					t.Fatal("the parameter default/presence meaning changed")
				}
				if tc.defaultParameter && (len(fn.ParameterInitializers) != 1 || fn.ParameterInitializers[0] == nil || fn.ParameterInitializers[0].Literal == nil || fn.ParameterInitializers[0].Literal.Decimal != "2") {
					t.Fatal("the owned default initializer is not the original integer 2")
				}
				if tc.compound {
					ownedWrite := false
					for _, st := range fn.Body {
						if st.Kind == "expression" && st.Expression != nil && st.Expression.Kind == "write" {
							value := st.Expression
							ownedWrite = value.Operator == "+=" && value.Type == "i32" && value.Left != nil && value.Left.Kind == "reference" && value.Left.Type == "i32" && value.Right != nil && value.Right.Type == "i32"
						}
					}
					if !ownedWrite {
						t.Fatal("the accepted compound write lost its owned i32 operands")
					}
				}
			}
			if !found {
				t.Fatal("the checked increment function disappeared")
			}
		})
	}
}

// Unlike jointCommandForTest this adds no module closure. The JSON request and
// response still cross runCommand; no projection function or DTO is mocked.
func defaultCompoundSingleCommandForTest(t *testing.T, r request) (jointFactsForTest, []byte) {
	t.Helper()
	var input, output bytes.Buffer
	if err := json.NewLegacyEncoder(&input).Encode(r); err != nil {
		t.Fatal(err)
	}
	if err := runCommand(bytes.NewReader(input.Bytes()), &output); err != nil {
		t.Fatalf("single-source request rejected before analysis: %v", err)
	}
	var result jointFactsForTest
	if err := json.NewLegacyDecoder(bytes.NewReader(output.Bytes())).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result, bytes.Clone(output.Bytes())
}

// Phase2 continuation: remove the remaining non-load-bearing parameter and
// forwarding call after the original10 matrix ruled out namespace/defaults.
// These cases keep the same module public command seam and do not replace or
// weaken any original matrix input or assertion above.
func TestJointGlobalCompoundMinimal(t *testing.T) {
	cases := []struct {
		name, source, functionName    string
		functionCount, parameterCount int
	}{
		{"11_one_module_literal_rhs", "export let count:i32=1; export function increment(delta:i32=2):i32 { count += 2; return count; } export function entry():i32 { return increment(); }", "increment", 2, 1},
		{"12_no_parameter", "export let count:i32=1; export function increment():i32 { count += 2; return count; } export function entry():i32 { return increment(); }", "increment", 2, 0},
		{"13_single_entry_minimal", "export let count:i32=1; export function entry():i32 { count += 2; return count; }", "entry", 1, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, raw := jointCommandForTest(t, integerRequest(""), []inputFile{{Path: "/src/app.ts", Content: tc.source}}, "/src/app.ts")
			if result.Status != "ok" || len(result.Diagnostics) != 0 || len(result.Modules) != 1 || len(result.Functions) != tc.functionCount {
				t.Fatalf("minimal source-owned global compound write rejected: %s", raw)
			}
			global := ""
			for _, s := range result.Symbols {
				if s.Name == "count" && s.SourcePath == "/src/app.ts" && s.BindingKind == "source" {
					global = s.ID
				}
			}
			if global == "" {
				t.Fatal("the source-owned global disappeared")
			}
			found := false
			for _, fn := range result.Functions {
				if fn.Name != tc.functionName {
					continue
				}
				found = true
				if fn.SourcePath != "/src/app.ts" || len(fn.Parameters) != tc.parameterCount {
					t.Fatal("function source ownership or unused-parameter deletion changed")
				}
				if tc.parameterCount == 1 && (fn.Parameters[0].Presence == nil || !fn.Parameters[0].Presence.Default || !fn.Parameters[0].Presence.Optional || len(fn.ParameterInitializers) != 1 || fn.ParameterInitializers[0] == nil || fn.ParameterInitializers[0].Literal == nil || fn.ParameterInitializers[0].Literal.Decimal != "2") {
					t.Fatal("the control case lost its existing default facts")
				}
				if len(fn.Body) != 2 || fn.Body[0].Kind != "expression" || fn.Body[0].Expression == nil || fn.Body[1].Kind != "return" || fn.Body[1].Expression == nil {
					t.Fatal("compound statement or return disappeared")
				}
				write, returned := fn.Body[0].Expression, fn.Body[1].Expression
				if write.Kind != "write" || write.Operator != "+=" || write.Type != "i32" || write.Left == nil || write.Left.Kind != "reference" || write.Left.Symbol != global || write.Left.Type != "i32" || write.Right == nil || write.Right.Type != "i32" || write.Right.Literal == nil || write.Right.Literal.Decimal != "2" || returned.Kind != "reference" || returned.Symbol != global || returned.Type != "i32" {
					t.Fatal("compound write/return no longer owns the original global and integer 2")
				}
			}
			if !found {
				t.Fatal("the expected function disappeared")
			}
		})
	}
}
