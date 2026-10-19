// SPDX-License-Identifier: Apache-2.0
// Real public Source grammar, exact dense proof and raw AST budget boundaries.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestArrayDenseMutation25dStaticDensity(t *testing.T) {
	for _, tc := range []struct{ name, statements, token string }{
		{"gap", "a[3] = 9.0;", "3"},
		{"growth", "a.length = 3;", "3"},
		{"aliasGapAfterShrink", "const same = a; a.length = 1; same[2] = 3.0;", "2"},
		{"aliasGrowthAfterShrink", "const same = a; a.length = 0; same.length = 1;", "1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prefix := "export function density(): f64 { const a: f64[] = [1.0, 2.0]; "
			source := prefix + tc.statements + " return a[0]; }"
			input := arrayBoundarySource25(t, source)
			arrayDomainOfficialZero25(t, input)
			_, owned := arrayBoundaryCall25(t, input)
			at := len(prefix) + strings.LastIndex(tc.statements, tc.token)
			if owned == nil || owned.Code != "r03-array-facts-density" || owned.RuleID != "r01-ts-baseline" || owned.Phase != "semantic" || len(owned.Diagnostics) != 1 {
				t.Fatalf("located private dense invariant failure: %+v", owned)
			}
			d := owned.Diagnostics[0]
			if d.Category != "error" || d.Severity != "error" || d.UpstreamCode != 0 || d.File != "/src/main.ts" || d.Range != (span{Start: at, End: at + len(tc.token)}) || d.Primary.Start != at || d.Primary.End != at+len(tc.token) {
				t.Fatalf("exact original index/newLength density anchor: %+v", d)
			}
		})
	}
}

func TestArrayDenseMutation25dExcludedGrammar(t *testing.T) {
	base := func(parameters, declaration, statement string) string {
		return "export function excluded(" + parameters + "): f64 { " + declaration + " " + statement + " return a[0]; }"
	}
	decl := "const a: f64[] = [1.0, 2.0];"
	for _, tc := range []struct{ name, source string }{
		{"compound", base("", decl, "a[0] += 1.0;")},
		{"update", base("", decl, "a[0]++;")},
		{"nestedAssignment", base("", decl, "a[0] = (a[1] = 3.0);")},
		{"delete", base("", decl, "delete a[0];")},
		{"let", base("", "let a: f64[] = [1.0, 2.0];", "a[0] = 3.0;")},
		{"numberIndexParameter", base("i: number", decl, "a[i] = 3.0;")},
		{"f64IndexParameter", base("i: f64", decl, "a[i] = 3.0;")},
		{"f64LengthParameter", base("n: f64", decl, "a.length = n;")},
		{"u32RHSParameter", base("n: u32", decl, "a[0] = n;")},
		{"optionalParameter", base("i?: u32", decl, "a[0] = 3.0;")},
		{"defaultParameter", base("i: u32 = 0", decl, "a[0] = 3.0;")},
		{"restParameter", base("...i: u32[]", decl, "a[0] = 3.0;")},
		{"numberArrayParameter", base("foreign: number[]", decl, "foreign[0] = 3.0;")},
		{"stringLengthTarget", base("", decl, "a[\"length\"] = 0;")},
		{"fractionIndex", base("", decl, "a[1.0000000000000001] = 3.0;")},
		{"fractionLength", base("", decl, "a.length = 1.0000000000000001;")},
		{"maxIndexIsNotArrayIndex", base("", decl, "a[4294967295] = 3.0;")},
		{"lengthOverflow", base("", decl, "a.length = 4294967296;")},
		{"negativeLength", base("", decl, "a.length = -1;")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := arrayBoundarySource25(t, tc.source)
			arrayDomainOfficialZero25(t, input)
			_, owned := arrayBoundaryCall25(t, input)
			if owned == nil || owned.Code != "r03-array-facts-capability" || owned.RuleID != "r01-ts-baseline" || owned.Phase != "capability" || len(owned.Diagnostics) != 1 || owned.Diagnostics[0].UpstreamCode != 0 {
				t.Fatalf("official-valid private grammar exclusion: %+v", owned)
			}
		})
	}
}

func TestArrayDenseMutation25dCheckedRHSAndInterleaving(t *testing.T) {
	source := "export function ordered(): f64 { const a: f64[] = [1.0]; a[0] = a[9]; const same = a; return same[0]; }"
	input := arrayBoundarySource25(t, source)
	arrayDomainOfficialZero25(t, input)
	output, owned := arrayBoundaryCall25(t, input)
	if owned != nil {
		t.Fatalf("OOB RHS remains a reached check and late alias is supported: %+v", owned)
	}
	var packet arrayFirstPackage25
	if err := json.Unmarshal(output, &packet); err != nil {
		t.Fatal(err)
	}
	if len(packet.Payload.Functions) != 1 {
		t.Fatal("complete ordered function collection")
	}
	fn := packet.Payload.Functions[0]
	if len(fn.Bindings) != 2 || len(fn.Body) != 4 || fn.Body[0].Kind != "variable" || fn.Body[1].Kind != "elementAssignment" || fn.Body[2].Kind != "variable" || fn.Body[3].Kind != "return" {
		t.Fatal("all original interleaved statements retained")
	}
	store := fn.Body[1]
	if store.Target == nil || store.Target.Array == nil || store.Target.Array.Binding != fn.Bindings[0].ID || store.Target.Index == nil || store.Target.Index.Decimal != "0" || store.Value == nil || store.Value.Kind != "checkedElementAccess" || store.Value.Array == nil || store.Value.Array.Binding != fn.Bindings[0].ID || store.Value.Index == nil || store.Value.Index.Decimal != "9" {
		t.Fatal("selected target and original OOB RHS obligation retained")
	}
	alias := fn.Body[2]
	if alias.Binding != fn.Bindings[1].ID || alias.Initializer == nil || alias.Initializer.Binding != fn.Bindings[0].ID || fn.Body[3].Value == nil || fn.Body[3].Value.Array == nil || fn.Body[3].Value.Array.Binding != fn.Bindings[1].ID {
		t.Fatal("late alias shares original binding; checked return retained")
	}
}

func arrayMutationBudgetSource25d(parameters, locals, mutations int, unsupported bool, extraEmpty bool) string {
	args := []string{}
	for i := 0; i < parameters; i++ {
		args = append(args, fmt.Sprintf("p%d: u32", i))
	}
	source := "export function budget(" + strings.Join(args, ", ") + "): f64 {\n"
	for i := 0; i < locals; i++ {
		source += fmt.Sprintf("const a%d: f64[] = [1.0];\n", i)
	}
	op := "="
	if unsupported {
		op = "+="
	}
	for i := 0; i < mutations; i++ {
		source += "a0[0] " + op + " 1.0;\n"
	}
	if extraEmpty {
		source += ";\n"
	}
	return source + "return a0[0];\n}\n"
}

func TestArrayDenseMutation25dSourceBudgets(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		params, locals, mutations  int
		unsupported, extra, accept bool
	}{
		{"params8", 8, 1, 1, false, false, true},
		{"params9", 9, 1, 1, false, false, false},
		{"mutations32", 0, 1, 32, false, false, true},
		{"mutations33", 0, 1, 33, false, false, false},
		{"body50", 8, 17, 32, false, false, true},
		{"body51Unsupported", 0, 17, 32, false, true, false},
		{"rawUnsupportedExpr33", 0, 1, 33, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := arrayMutationBudgetSource25d(tc.params, tc.locals, tc.mutations, tc.unsupported, tc.extra)
			input := arrayBoundarySource25(t, source)
			arrayDomainOfficialZero25(t, input)
			var output arrayBoundaryWriter25
			err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v6"}, bytes.NewReader(input), &output)
			if !tc.accept {
				owned := arrayBoundaryFailure25(t, err)
				if owned.Code != "r03-array-facts-limit" || owned.Phase != "input" || owned.RuleID != "r01-input-boundary" || output.calls != 0 || output.Len() != 0 {
					t.Fatalf("raw AST budget precedes unsupported grammar and publication: %+v", owned)
				}
				return
			}
			if err != nil || output.calls != 1 {
				t.Fatalf("inclusive domain budget: %v", err)
			}
			var packet arrayFirstPackage25
			if json.Unmarshal(output.Bytes(), &packet) != nil || len(packet.Payload.Functions) != 1 {
				t.Fatal("complete legal budget packet")
			}
			fn := packet.Payload.Functions[0]
			if len(fn.Parameters) != tc.params || len(fn.Bindings) != tc.locals || len(fn.Body) != tc.locals+tc.mutations+1 {
				t.Fatal("legal boundary lost parameters, locals or statements")
			}
		})
	}
}
