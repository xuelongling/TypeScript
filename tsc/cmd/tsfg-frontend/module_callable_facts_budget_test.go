// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestModuleCallableFactsPublicBudgets(t *testing.T) {
	t.Run("exactly-64-source-files", func(t *testing.T) {
		r := mcMutableRequest23d(t, "first-rr")
		for i := 0; i < 61; i++ {
			mcAppendSource23d(r, fmt.Sprintf("/src/extra%02d.ts", i), "export function extra(): bool { return true; }\n")
		}
		mcAcceptPublicBudget23d(t, mcRequestBytes23d(t, r), 64, 65, -1)
	})
	t.Run("exactly-128-source-functions", func(t *testing.T) {
		r := mcMutableRequest23d(t, "first-rr")
		mcReplaceSource23d(t, r, "/src/app.ts", func(s string) string {
			for i := 0; i < 124; i++ {
				s += fmt.Sprintf("export function extra%03d(): bool { return true; }\n", i)
			}
			return s
		})
		mcAcceptPublicBudget23d(t, mcRequestBytes23d(t, r), 3, 128, -1)
	})
	t.Run("exactly-32-parameters", func(t *testing.T) {
		r := mcMutableRequest23d(t, "first-rr")
		parameters := []string{}
		for i := 0; i < 32; i++ {
			parameters = append(parameters, fmt.Sprintf("p%d: bool", i))
		}
		mcReplaceSource23d(t, r, "/src/app.ts", func(s string) string {
			return s + "export function excess(" + strings.Join(parameters, ", ") + "): bool { return p0; }\n"
		})
		mcAcceptPublicBudget23d(t, mcRequestBytes23d(t, r), 3, 5, 32)
	})
	// Count callable signature nodes only: declaration root=1, each nested
	// function parameter/result adds one; the scalar bool leaf adds none.
	t.Run("exactly-16-callable-depth", func(t *testing.T) {
		r := mcMutableRequest23d(t, "first-rr")
		mcReplaceSource23d(t, r, "/src/app.ts", func(s string) string {
			return s + "export function deep(fn: " + strings.Repeat("() => ", 15) + "bool): bool { return true; }\n"
		})
		mcAcceptPublicBudget23d(t, mcRequestBytes23d(t, r), 3, 5, 1)
	})
	t.Run("adjacent-17-callable-depth", func(t *testing.T) {
		r := mcMutableRequest23d(t, "first-rr")
		mcReplaceSource23d(t, r, "/src/app.ts", func(s string) string {
			return s + "export function deep(fn: " + strings.Repeat("() => ", 16) + "bool): bool { return true; }\n"
		})
		mcRejectPublic23d(t, mcRequestBytes23d(t, r))
	})

	t.Run("more-than-64-source-files", func(t *testing.T) {
		r := mcMutableRequest23d(t, "first-rr")
		for i := 0; i < 62; i++ {
			mcAppendSource23d(r, fmt.Sprintf("/src/extra%02d.ts", i), "export function extra(): bool { return true; }\n")
		}
		mcRejectPublic23d(t, mcRequestBytes23d(t, r))
	})
	t.Run("more-than-128-source-functions", func(t *testing.T) {
		r := mcMutableRequest23d(t, "first-rr")
		mcReplaceSource23d(t, r, "/src/app.ts", func(s string) string {
			for i := 0; i < 125; i++ {
				s += fmt.Sprintf("export function extra%03d(): bool { return true; }\n", i)
			}
			return s
		})
		mcRejectPublic23d(t, mcRequestBytes23d(t, r))
	})
	t.Run("more-than-32-parameters", func(t *testing.T) {
		r := mcMutableRequest23d(t, "first-rr")
		parameters := []string{}
		for i := 0; i < 33; i++ {
			parameters = append(parameters, fmt.Sprintf("p%d: bool", i))
		}
		mcReplaceSource23d(t, r, "/src/app.ts", func(s string) string {
			return s + "export function excess(" + strings.Join(parameters, ", ") + "): bool { return p0; }\n"
		})
		mcRejectPublic23d(t, mcRequestBytes23d(t, r))
	})
	t.Run("deep-callable-signature", func(t *testing.T) {
		r := mcMutableRequest23d(t, "first-rr")
		mcReplaceSource23d(t, r, "/src/app.ts", func(s string) string {
			return s + "export function deep(fn: " + strings.Repeat("() => ", 17) + "bool): bool { return true; }\n"
		})
		mcRejectPublic23d(t, mcRequestBytes23d(t, r))
	})
	t.Run("more-than-8mib-request", func(t *testing.T) {
		r := mcMutableRequest23d(t, "first-rr")
		for i := 0; i < 9; i++ {
			mcAppendSource23d(r, fmt.Sprintf("/src/size%02d.ts", i), "/*"+strings.Repeat("a", 950000)+"*/\nexport function extra(): bool { return true; }\n")
		}
		input := mcRequestBytes23d(t, r)
		if len(input) <= 8388608 {
			t.Fatal("independent request-byte negative must exceed frozen 8MiB")
		}
		mcRejectPublic23d(t, input)
	})
}

// Boundary controls validate complete canonical publication and independent
// input/manifest identities; they do not sample output as a future whole oracle.
func mcAcceptPublicBudget23d(t *testing.T, input []byte, wantSources, wantFunctions, wantDeepParameters int) {
	t.Helper()
	before := bytes.Clone(input)
	output := &mcPublicWriter23d{}
	if err := runFrontendCommand(t.Context(), []string{"--module-callable-facts-io-v2"}, bytes.NewReader(input), output); err != nil {
		t.Fatalf("legal adjacent public boundary must succeed: %v", err)
	}
	if output.writes != 1 || !bytes.Equal(input, before) {
		t.Fatal("legal public boundary requires one complete write and unchanged input")
	}
	var requestObject, packet mcObject23d
	if err := json.Unmarshal(input, &requestObject); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
		t.Fatalf("legal boundary published incomplete JSON: %v", err)
	}
	if !bytes.Equal(output.Bytes(), append(mcCanonical23d(t, packet), '\n')) {
		t.Fatal("legal boundary packet must be complete canonical JSON plus one LF")
	}
	if len(packet) != 6 || packet["family"] != "tsfg.module-callable-facts-package-candidate" || packet["version"] != float64(2) || packet["schemaHash"] != mcHash23d(moduleCallableSchema23d) || packet["requestId"] != mcDomain23d(t, "tsfg.module-callable-facts.request.v2", requestObject) {
		t.Fatal("legal boundary envelope/request identity differs")
	}
	packageID := packet["packageId"]
	delete(packet, "packageId")
	if packageID != mcDomain23d(t, "tsfg.module-callable-facts.package.v2", packet) {
		t.Fatal("legal boundary complete package identity differs")
	}
	payload, ok := packet["payload"].(map[string]any)
	if !ok || len(payload) != 5 || payload["inputIdentity"] != mcDomain23d(t, "tsfg.module-callable-facts.input.v2", requestObject["input"]) {
		t.Fatal("legal boundary complete payload/input identity differs")
	}
	manifest, ok := payload["files"].([]any)
	if !ok || len(manifest) != wantSources+1 {
		t.Fatal("legal boundary complete manifest differs")
	}
	files := mcInputObject23d(requestObject)["files"].([]any)
	for index, item := range manifest {
		row := item.(map[string]any)
		file := files[index].(map[string]any)
		if row["path"] != file["path"] || row["digest"] != "sha256:"+file["sha256"].(string) || row["positionUnit"] != "utf16" {
			t.Fatal("legal boundary manifest file identity differs")
		}
	}
	facts, ok := payload["facts"].(map[string]any)
	if !ok || len(facts) != 5 || facts["status"] != "ok" {
		t.Fatal("legal boundary complete facts missing")
	}
	modules, ok := facts["modules"].([]any)
	if !ok || len(modules) != wantSources {
		t.Fatal("legal boundary dropped modules")
	}
	functions, ok := facts["functions"].([]any)
	if !ok || len(functions) != wantFunctions {
		t.Fatal("legal boundary dropped source functions")
	}
	annotations, ok := payload["annotations"].([]any)
	if !ok || len(annotations) != wantFunctions {
		t.Fatal("legal boundary dropped function annotations")
	}
	exports, ok := payload["exports"].([]any)
	if !ok || len(exports) != wantSources {
		t.Fatal("legal boundary dropped export groups")
	}
	if wantDeepParameters >= 0 {
		found := false
		for _, item := range functions {
			function := item.(map[string]any)
			if function["name"] == "excess" || function["name"] == "deep" {
				parameters, ok := function["parameters"].([]any)
				if !ok || len(parameters) != wantDeepParameters {
					t.Fatal("legal boundary function parameter count differs")
				}
				found = true
			}
		}
		if !found {
			t.Fatal("legal boundary declaration omitted")
		}
	}
	t.Logf("TSFG_MODULE_CALLABLE_23D_LEGAL_BOUNDARY_PASS sources=%d functions=%d bytes=%d digest=%s", wantSources, wantFunctions, output.Len(), mcHash23d(output.Bytes()))
}
