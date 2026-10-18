// SPDX-License-Identifier: Apache-2.0
// Exact function cardinality on small literal Source, unrelated to historical stress inputs.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestStringFactsFunctionLimit64(t *testing.T) {
	for _, count := range []int{64, 65} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			var source strings.Builder
			functions := []any{}
			for i := 0; i < count; i++ {
				name := fmt.Sprintf("limit%02d", i)
				declaration := "export function " + name + "(): string { return ''; }"
				at := source.Len()
				source.WriteString(declaration + "\n")
				statement := at + strings.Index(declaration, "return")
				literal := at + strings.Index(declaration, "''")
				functions = append(functions, map[string]any{"name": name, "sourcePath": "/owned/strings.ts", "parameters": []any{}, "returnType": "string", "declarationRange": map[string]any{"start": at, "end": at + len(declaration)},
					"body": []any{map[string]any{"kind": "return", "range": map[string]any{"start": statement, "end": statement + len("return '';")}, "value": map[string]any{"kind": "stringLiteral", "type": "string", "range": map[string]any{"start": literal, "end": literal + 2}, "codeUnits": []int32{}}}}})
			}
			if source.Len() > 4096 {
				t.Fatal("bounded small Source exceeded test premise")
			}
			input := stringBoundaryRequest24(t, source.String())
			var r struct{ Input request }
			if err := json.Unmarshal(input, &r); err != nil {
				t.Fatal(err)
			}
			official := throwBoundaryProgramForTest(t, r.Input)
			throwBoundaryDiagnosticsForTest(t, official)
			if len(official.GetSourceFile(r.Input.RootFile).Statements.Nodes) != count {
				t.Fatal("official complete declaration cardinality")
			}
			if count == 65 {
				stringBoundaryFailure24(t, input, "limit")
				return
			}
			expected := stringBoundaryPacket24(t, input, functions)
			var output bytes.Buffer
			if err := runFrontendCommand(t.Context(), []string{"--string-facts-io-v1"}, bytes.NewReader(input), &output); err != nil || !bytes.Equal(output.Bytes(), expected) {
				t.Fatalf("complete 64-function manual package mismatch: %v", err)
			}
		})
	}
}
