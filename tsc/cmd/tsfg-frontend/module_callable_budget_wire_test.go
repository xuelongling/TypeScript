// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// Exact request-wire probes are JSON-valid but deliberately have an extra
// envelope field. They establish byte/error priority, not a legal program max.
func mcWireBoundaryReject23d(t *testing.T, input []byte, kind string) {
	t.Helper()
	before := bytes.Clone(input)
	output := &mcPublicWriter23d{}
	err := runFrontendCommand(t.Context(), []string{"--module-callable-facts-io-v2"}, bytes.NewReader(input), output)
	message := map[string]string{"input": "invalid module callable candidate", "limit": "module callable candidate budget exceeded"}[kind]
	if err == nil || err.Error() != "r03-module-callable-"+kind+": "+message || output.writes != 0 || output.Len() != 0 {
		t.Fatalf("public wire boundary expected %s and no output: err=%v writes=%d bytes=%d", kind, err, output.writes, output.Len())
	}
	if !bytes.Equal(input, before) {
		t.Fatal("public wire boundary mutated input")
	}
	t.Logf("TSFG_MODULE_CALLABLE_23D_WIRE_EDGE_REJECT bytes=%d digest=%s kind=%s", len(input), mcHash23d(input), kind)
}
func TestModuleCallableFactsBudgetWireDeclarationCounts(t *testing.T) {
	t.Run("one-original-declaration-control", func(t *testing.T) {
		_, input, _ := mcPublicFixture23d(t, "first-rr")
		mcAcceptPublicBudget23d(t, input, 3, 4, -1)
	})
	for _, n := range []int{0, 2} {
		t.Run(fmt.Sprintf("declarations-%d", n), func(t *testing.T) {
			r := mcMutableRequest23d(t, "first-rr")
			input := mcInputObject23d(r)
			declarations := input["declarations"].(map[string]any)
			if n == 0 {
				files := []any{}
				for _, item := range input["files"].([]any) {
					if !strings.HasSuffix(item.(map[string]any)["path"].(string), ".d.ts") {
						files = append(files, item)
					}
				}
				input["files"] = files
				declarations["paths"] = []any{}
			} else {
				mcAppendSource23d(r, "/src/second.d.ts", "// extra declaration cardinality fixture\n")
				declarations["paths"] = []any{"/src/minimal-integers-v3.d.ts", "/src/second.d.ts"}
			}
			actual := 0
			for _, item := range input["files"].([]any) {
				if strings.HasSuffix(item.(map[string]any)["path"].(string), ".d.ts") {
					actual++
				}
			}
			if actual != n || len(declarations["paths"].([]any)) != n {
				t.Fatal("exact declaration cardinality probe differs")
			}
			// Header cardinality rejects before creating/checking an official Program.
			// No syntax/typing failure or missing-DTS case is counted as semantic RED.
			mcWireBoundaryReject23d(t, mcRequestBytes23d(t, r), "input")
		})
	}
}
func TestModuleCallableFactsBudgetWireExactRequestBytes(t *testing.T) {
	const maximum = 8388608
	for _, extra := range []int{0, 1} {
		t.Run(fmt.Sprintf("request-bytes-%d", maximum+extra), func(t *testing.T) {
			r := mcMutableRequest23d(t, "first-rr")
			padding := []any{}
			for i := 0; i < 7; i++ {
				padding = append(padding, strings.Repeat("\"", 524288))
			}
			r["extra"] = padding
			// Adding one array element adds comma+two quote bytes; the final LF is
			// included independently. Every decoded string stays below its1MiB cap.
			remaining := maximum + extra - len(mcRequestBytes23d(t, r)) - 3
			if remaining < 0 || remaining > 1048576 {
				t.Fatal("request-byte padding would hit a string limit")
			}
			r["extra"] = append(padding, strings.Repeat("x", remaining))
			raw := mcRequestBytes23d(t, r)
			tokens, depth, maxString := mcBudgetJsonMetrics23d(r, 1)
			if len(raw) != maximum+extra || raw[len(raw)-1] != '\n' || tokens > 200000 || depth > 128 || maxString > 1048576 {
				t.Fatalf("request-byte independent metrics differ bytes=%d tokens=%d depth=%d maxString=%d", len(raw), tokens, depth, maxString)
			}
			kind := "input"
			if extra == 1 {
				kind = "limit"
			}
			mcWireBoundaryReject23d(t, raw, kind)
		})
	}
}
