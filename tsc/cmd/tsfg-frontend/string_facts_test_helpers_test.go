// SPDX-License-Identifier: Apache-2.0
// Independent request/expected builders use only literal source and the frozen schema.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf16"
)

func stringBoundaryRequest24(t *testing.T, source string) []byte {
	t.Helper()
	data := stringOracleRead24(t, "request.json", "153b9e72ac0dec01815f05218cad4bc7377a46456b5030e0659e4223474fdec5")
	var r map[string]any
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	input := r["input"].(map[string]any)
	input["sourceRoot"] = "/owned"
	input["rootFile"] = "/owned/strings.ts"
	input["declarations"].(map[string]any)["paths"] = []string{"/owned/minimal-scalars-v2.d.ts"}
	var dts string
	for _, raw := range input["files"].([]any) {
		file := raw.(map[string]any)
		if strings.HasSuffix(file["path"].(string), ".d.ts") {
			dts = file["content"].(string)
		}
	}
	sum := sha256.Sum256([]byte(source))
	input["files"] = []any{
		map[string]any{"path": "/owned/minimal-scalars-v2.d.ts", "content": dts, "sha256": "d629059f5175a580a8a7b19531c7d8cff1bfa7b61320f6638cf4ee25fbf5a141"},
		map[string]any{"path": "/owned/strings.ts", "content": source, "sha256": hex.EncodeToString(sum[:])},
	}
	return append(stringOracleCanonical24(t, r), '\n')
}
func stringBoundaryMutate24(t *testing.T, data []byte, edit func(map[string]any)) []byte {
	t.Helper()
	var r map[string]any
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	edit(r)
	return append(stringOracleCanonical24(t, r), '\n')
}
func stringBoundaryRange24(t *testing.T, source, token string) map[string]any {
	t.Helper()
	at := strings.Index(source, token)
	if at < 0 {
		t.Fatal("manual token absent")
	}
	return map[string]any{"start": len(utf16.Encode([]rune(source[:at]))), "end": len(utf16.Encode([]rune(source[:at+len(token)])))}
}
func stringBoundaryExpected24(t *testing.T, data []byte, source string) []byte {
	t.Helper()
	functions := []any{}
	for _, tc := range []struct{ name, declaration, statement, literal string }{
		{"other", "export function other(): string { return 'same'; }", "return 'same';", "'same'"},
		{"again", `export function again(): string { return "same"; }`, `return "same";`, `"same"`},
	} {
		functions = append(functions, map[string]any{"name": tc.name, "sourcePath": "/owned/strings.ts", "declarationRange": stringBoundaryRange24(t, source, tc.declaration), "parameters": []any{}, "returnType": "string",
			"body": []any{map[string]any{"kind": "return", "range": stringBoundaryRange24(t, source, tc.statement), "value": map[string]any{"kind": "stringLiteral", "type": "string", "range": stringBoundaryRange24(t, source, tc.literal), "codeUnits": []int32{115, 97, 109, 101}}}}})
	}
	return stringBoundaryPacket24(t, data, functions)
}
func stringBoundaryPacket24(t *testing.T, data []byte, functions []any) []byte {
	t.Helper()
	var request map[string]any
	if err := json.Unmarshal(data, &request); err != nil {
		t.Fatal(err)
	}
	input := request["input"].(map[string]any)
	files := []any{}
	for _, raw := range input["files"].([]any) {
		f := raw.(map[string]any)
		text := f["content"].(string)
		files = append(files, map[string]any{"path": f["path"], "content": text, "sha256": f["sha256"], "byteLength": len(text), "utf16Length": len(utf16.Encode([]rune(text)))})
	}
	payload := map[string]any{"sourceRoot": input["sourceRoot"], "rootFile": input["rootFile"], "config": input["config"], "declarations": input["declarations"], "positionUnit": "utf16", "files": files, "functions": functions}
	const schema = "sha256:6dfed46326c15a81aeb6a1ea9b5ed2fc10db7bb1bf27733ac3df530d78d5cc62"
	packet := map[string]any{"family": "tsfg.string-facts-package-candidate", "version": 1, "schemaHash": schema, "requestId": stringOracleHash24("tsfg.string-facts.request.v1\x00", data), "packageId": stringOracleHash24("tsfg.string-facts.package.v1\x00"+schema+"\x00", stringOracleCanonical24(t, payload)), "payload": payload}
	return append(stringOracleCanonical24(t, packet), '\n')
}
func stringBoundaryFailure24(t *testing.T, data []byte, kind string) *stringFactsFailure {
	t.Helper()
	var output bytes.Buffer
	err := runFrontendCommand(t.Context(), []string{"--string-facts-io-v1"}, bytes.NewReader(data), &output)
	failure, ok := err.(*stringFactsFailure)
	if !ok || failure.Kind != kind || output.Len() != 0 {
		t.Fatalf("wanted %s / zero output, got %+v / %d", kind, err, output.Len())
	}
	return failure
}
