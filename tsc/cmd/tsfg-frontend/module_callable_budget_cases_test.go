// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

type mcBudgetCase23d struct {
	name, kind, source        string
	roots                     []closureRoot
	functions, globals, scale int
}

func mcBudgetRoot23d(name string) closureRoot {
	return closureRoot{Kind: "kernel", SourcePath: "/src/app.ts", Name: name}
}
func mcBudgetGlobals23d(n int) string {
	var out strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&out, "let g%04d: bool = false;\n", i)
	}
	return out.String()
}
func mcBudgetBoolID23d(t *testing.T, name string) string {
	signature := mcDomain23d(t, "tsfg.module-callable-signature.candidate.v1", mcObject23d{"parameters": []any{}, "returns": "bool"})
	return mcDomain23d(t, "tsfg.module-callable-function.candidate.v1", mcObject23d{"path": "/src/app.ts", "name": name, "signatureHash": signature})
}
func mcBudgetAggregationWork23d(t *testing.T, edges int) mcBudgetCase23d {
	names := []string{}
	for i := 0; i <= edges; i++ {
		names = append(names, fmt.Sprintf("f%03d", i))
	}
	sort.Slice(names, func(i, j int) bool { return mcBudgetBoolID23d(t, names[i]) < mcBudgetBoolID23d(t, names[j]) })
	next := map[string]string{}
	for i := 0; i < edges; i++ {
		next[names[i]] = names[i+1]
	}
	leaf := names[edges]
	source := mcBudgetGlobals23d(256)
	declared := append([]string{}, names...)
	sort.Strings(declared)
	for _, name := range declared {
		source += "export function " + name + "(): bool {\n"
		if name == leaf {
			for i := 0; i < 256; i++ {
				source += fmt.Sprintf("g%04d;\n", i)
			}
		} else {
			source += next[name] + "();\n"
		}
		source += "return true;\n}\n"
	}
	return mcBudgetCase23d{name: fmt.Sprintf("aggregation-work-%d", edges), kind: "aggregation-work", source: source, roots: []closureRoot{mcBudgetRoot23d(names[0])}, functions: edges + 1, globals: 256, scale: edges}
}
func mcBudgetAggregationMembers23d(callers int) mcBudgetCase23d {
	source := mcBudgetGlobals23d(1024) + "export function callee(): bool { return true; }\n"
	roots := []closureRoot{}
	for i := 0; i < callers; i++ {
		name := fmt.Sprintf("c%03d", i)
		source += "export function " + name + "(): bool { return callee(); }\n"
		roots = append(roots, mcBudgetRoot23d(name))
	}
	return mcBudgetCase23d{name: fmt.Sprintf("aggregation-members-%d", callers), kind: "aggregation-members", source: source, roots: roots, functions: callers + 1, globals: 1024, scale: callers}
}
func mcBudgetSolverGrowth23d(sites int) mcBudgetCase23d {
	source := ""
	params := "a: () => bool, b: () => bool, c: () => bool, d: () => bool"
	candidate := "(" + params + ") => bool"
	callback := "(candidate: " + candidate + ") => bool"
	for i := 0; i < 124; i++ {
		source += fmt.Sprintf("export function f%03d(%s): bool { return true; }\n", i, params)
	}
	source += "export function leaf(): bool { return true; }\nexport function invoke(fn: " + callback + ", candidate: " + candidate + "): bool {\n"
	for i := 0; i < sites; i++ {
		source += "fn(candidate);\n"
	}
	source += "return true;\n}\nexport function entry(fn: " + callback + "): bool { return invoke(fn, f000); }\n"
	return mcBudgetCase23d{name: fmt.Sprintf("solver-growth-top-%d", sites), kind: "solver-growth-top", source: source, roots: []closureRoot{mcBudgetRoot23d("entry")}, functions: 127, scale: sites}
}
func mcBudgetSolverWork23d(padding int) mcBudgetCase23d {
	source := "export function leafA(): bool { return true; }\nexport function leafB(): bool { return false; }\nexport function entry(flag: bool): bool {\n"
	for i := 0; i < 256; i++ {
		target := "leafA"
		if i == 0 {
			target = "leafB"
		}
		source += fmt.Sprintf("let s%03d: () => bool = %s;\n", i, target)
	}
	source += "while (flag) {\n"
	for i := 255; i > 0; i-- {
		source += fmt.Sprintf("s%03d = s%03d;\n", i, i-1)
	}
	source += strings.Repeat("true;\n", padding) + "}\nreturn true;\n}\n"
	return mcBudgetCase23d{name: fmt.Sprintf("solver-work-256-%d", padding), kind: "solver-work-256", source: source, roots: []closureRoot{mcBudgetRoot23d("entry")}, functions: 3, scale: padding}
}
func mcBudgetTypes23d(lastDepth int) mcBudgetCase23d {
	params := []string{}
	for i := 0; i < 17; i++ {
		depth := 15
		if i == 16 {
			depth = lastDepth
		}
		typ := "bool"
		args := []string{}
		for j := 0; j < i; j++ {
			args = append(args, fmt.Sprintf("a%d: bool", j))
		}
		for j := 0; j < depth; j++ {
			typ = "(" + strings.Join(args, ", ") + ") => " + typ
		}
		params = append(params, fmt.Sprintf("p%d: %s", i, typ))
	}
	return mcBudgetCase23d{name: fmt.Sprintf("types-%d", 244+lastDepth), kind: "types", source: "export function entry(" + strings.Join(params, ", ") + "): bool { return true; }\n", roots: []closureRoot{mcBudgetRoot23d("entry")}, functions: 1, scale: 244 + lastDepth}
}
func TestModuleCallableFactsBudgetAggregationWork(t *testing.T) {
	for _, n := range []int{86, 87} {
		tc := mcBudgetAggregationWork23d(t, n)
		t.Run(tc.name, func(t *testing.T) { mcBudgetCapture23d(t, tc) })
	}
}
func TestModuleCallableFactsBudgetAggregationMembers(t *testing.T) {
	for _, n := range []int{62, 63} {
		tc := mcBudgetAggregationMembers23d(n)
		t.Run(tc.name, func(t *testing.T) { mcBudgetCapture23d(t, tc) })
	}
}
func TestModuleCallableFactsBudgetSolverGrowth(t *testing.T) {
	for _, n := range []int{401, 402} {
		tc := mcBudgetSolverGrowth23d(n)
		t.Run(tc.name, func(t *testing.T) { mcBudgetCapture23d(t, tc) })
	}
}
func TestModuleCallableFactsBudgetSolverWork(t *testing.T) {
	for _, n := range []int{1568, 1569} {
		tc := mcBudgetSolverWork23d(n)
		t.Run(tc.name, func(t *testing.T) { mcBudgetCapture23d(t, tc) })
	}
}
func TestModuleCallableFactsBudgetTypes(t *testing.T) {
	for _, n := range []int{12, 13} {
		tc := mcBudgetTypes23d(n)
		t.Run(tc.name, func(t *testing.T) { mcBudgetCapture23d(t, tc) })
	}
}
