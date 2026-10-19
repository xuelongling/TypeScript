// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Model probes use complete independently checked Programs before the actual
// public seam. Captures are bounded inputs, not replacement whole oracles.
func mcModelRoots23d(n int) mcBudgetCase23d {
	var source strings.Builder
	roots := []closureRoot{}
	for i := 0; i < 128; i++ {
		name := fmt.Sprintf("f%03d", i)
		fmt.Fprintf(&source, "export function %s(): bool { return true; }\n", name)
		roots = append(roots, mcBudgetRoot23d(name))
	}
	if n == 129 {
		roots = append(roots, closureRoot{Kind: "application", SourcePath: "/src/app.ts", Name: "f000"})
	}
	return mcBudgetCase23d{name: fmt.Sprintf("model-roots-%d", n), kind: "model-roots", source: source.String(), roots: roots, functions: 128, scale: n}
}
func mcModelGlobals23d(n int) mcBudgetCase23d {
	var source strings.Builder
	// No annotation: each global costs three official statement/expression nodes,
	// so the4096/4097 symbol probes are not hidden by the official16384 AST cap.
	for i := 0; i < n; i++ {
		fmt.Fprintf(&source, "let g%04d = false;\n", i)
	}
	source.WriteString("export function entry(): bool { return true; }\n")
	return mcBudgetCase23d{name: fmt.Sprintf("model-symbols-%d", n+1), kind: "model-symbols", source: source.String(), roots: []closureRoot{mcBudgetRoot23d("entry")}, functions: 1, globals: n, scale: n + 1}
}
func mcModelBlocks23d(n int) mcBudgetCase23d {
	var source strings.Builder
	for i := 0; i < 4; i++ {
		fmt.Fprintf(&source, "export function f%03d(): bool {\n", i)
		count := n / 4
		if i < n%4 {
			count++
		}
		source.WriteString(strings.Repeat("{}\n", count))
		source.WriteString("return true;\n}\n")
	}
	return mcBudgetCase23d{name: "model-ast", kind: "model-ast", source: source.String(), roots: []closureRoot{mcBudgetRoot23d("f000")}, functions: 4}
}
func mcModelRun23d(t *testing.T, r mcObject23d, want string) []byte {
	t.Helper()
	input := mcRequestBytes23d(t, r)
	before := bytes.Clone(input)
	if len(input) > 8388608 {
		t.Fatal("model probe unexpectedly exceeded request bytes")
	}
	output := &mcPublicWriter23d{}
	err := runFrontendCommand(t.Context(), []string{"--module-callable-facts-io-v2"}, bytes.NewReader(input), output)
	if !bytes.Equal(input, before) {
		t.Fatal("model public request mutated")
	}
	if want != "" {
		if err == nil || err.Error() != "r03-module-callable-"+want+": "+map[string]string{"limit": "module callable candidate budget exceeded", "input": "invalid module callable candidate"}[want] || output.writes != 0 || output.Len() != 0 {
			t.Fatalf("model public %s expected no output: error=%v writes=%d bytes=%d", want, err, output.writes, output.Len())
		}
		return nil
	}
	if err != nil || output.writes != 1 {
		t.Fatalf("legal model public input must publish once: err=%v writes=%d", err, output.writes)
	}
	var packet mcObject23d
	if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), append(mcCanonical23d(t, packet), '\n')) || packet["requestId"] != mcDomain23d(t, "tsfg.module-callable-facts.request.v2", r) {
		t.Fatal("model full canonical publication/request differs")
	}
	id := packet["packageId"]
	delete(packet, "packageId")
	if id != mcDomain23d(t, "tsfg.module-callable-facts.package.v2", packet) {
		t.Fatal("model full package digest differs")
	}
	return bytes.Clone(output.Bytes())
}
func TestModuleCallableFactsBudgetModelRoots(t *testing.T) {
	tc := mcModelRoots23d(128)
	t.Run("roots128-capture", func(t *testing.T) { mcBudgetCapture23d(t, tc) })
	t.Run("required128", func(t *testing.T) {
		r := mcBudgetRequest23d(t, tc)
		closure := mcInputObject23d(r)["compilationClosure"].(map[string]any)
		closure["requiredRoots"] = closure["roots"]
		official := mcBudgetOfficial23d(t, mcRequestBytes23d(t, r))
		if official["functions"] != 128 {
			t.Fatal(official)
		}
		raw := mcModelRun23d(t, r, "")
		t.Logf("TSFG_MODULE_CALLABLE_23D_MODEL_REQUIRED_PASS roots=128 requiredRoots=128 bytes=%d digest=%s", len(raw), mcHash23d(raw))
	})
	t.Run("roots129", func(t *testing.T) {
		r := mcBudgetRequest23d(t, mcModelRoots23d(129))
		official := mcBudgetOfficial23d(t, mcRequestBytes23d(t, r))
		if official["functions"] != 128 {
			t.Fatal(official)
		}
		mcModelRun23d(t, r, "limit")
	})
	t.Run("required129", func(t *testing.T) {
		r := mcBudgetRequest23d(t, tc)
		other := mcBudgetRequest23d(t, mcModelRoots23d(129))
		mcInputObject23d(r)["compilationClosure"].(map[string]any)["requiredRoots"] = mcInputObject23d(other)["compilationClosure"].(map[string]any)["roots"]
		mcBudgetOfficial23d(t, mcRequestBytes23d(t, r))
		mcModelRun23d(t, r, "limit")
	})
}
func TestModuleCallableFactsBudgetModelSymbols(t *testing.T) {
	// Every non-function symbol costs>=58 JSON tokens across registry+embedded
	// declaration. With<=128 function symbols,4096 symbols cost>=232832 tokens:
	// the200000 package cap shadows a publishable4096-symbol maximum.
	t.Run("smaller-legal-control", func(t *testing.T) { mcBudgetCapture23d(t, mcModelGlobals23d(2048)) })
	for _, n := range []int{4095, 4096} {
		tc := mcModelGlobals23d(n)
		t.Run(tc.name, func(t *testing.T) {
			r := mcBudgetRequest23d(t, tc)
			official := mcBudgetOfficial23d(t, mcRequestBytes23d(t, r))
			if official["symbols"] != n+1 || official["ast"] > 16384 || official["functions"] != 1 || official["globals"] != n {
				t.Fatal(official)
			}
			mcModelRun23d(t, r, "limit")
			t.Logf("TSFG_MODULE_CALLABLE_23D_MODEL_SYMBOL_REJECT symbols=%d officialAST=%d packageTokenLowerBound=%d legalPublishedMaximum=false", n+1, official["ast"], 21+75*n)
		})
	}
}
func TestModuleCallableFactsBudgetModelAST(t *testing.T) {
	base := mcModelBlocks23d(0)
	baseCounts := mcBudgetOfficial23d(t, mcRequestBytes23d(t, mcBudgetRequest23d(t, base)))
	for _, target := range []int{16384, 16385} {
		tc := mcModelBlocks23d(target - baseCounts["ast"])
		tc.name = fmt.Sprintf("model-ast-%d", target)
		tc.scale = target
		t.Run(tc.name, func(t *testing.T) {
			r := mcBudgetRequest23d(t, tc)
			official := mcBudgetOfficial23d(t, mcRequestBytes23d(t, r))
			if official["ast"] != target || official["symbols"] != 4 || official["calls"] != 0 {
				t.Fatal(official)
			}
			if target == 16384 {
				mcBudgetCapture23d(t, tc)
			} else {
				mcModelRun23d(t, r, "limit")
			}
		})
	}
}
func TestModuleCallableFactsBudgetModelJSONTokens(t *testing.T) {
	tc := mcModelGlobals23d(0)
	for _, extra := range []int{0, 1} {
		t.Run(fmt.Sprintf("tokens-%d", 200000+extra), func(t *testing.T) {
			r := mcBudgetRequest23d(t, tc)
			mcBudgetOfficial23d(t, mcRequestBytes23d(t, r))
			r["extra"] = []any{}
			tokens, _, _ := mcBudgetJsonMetrics23d(r, 1)
			r["extra"] = make([]any, 200000+extra-tokens)
			tokens, depth, maxString := mcBudgetJsonMetrics23d(r, 1)
			if tokens != 200000+extra || depth > 128 || maxString > 1048576 || len(mcRequestBytes23d(t, r)) > 8388608 {
				t.Fatalf("isolated wire token probe differs: %d %d %d", tokens, depth, maxString)
			}
			kind := "input"
			if extra == 1 {
				kind = "limit"
			}
			mcModelRun23d(t, r, kind)
		})
	}
}
