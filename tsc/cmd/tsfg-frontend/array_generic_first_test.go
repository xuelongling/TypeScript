// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	tsjson "github.com/microsoft/typescript-go/internal/json"
)

func genericArrayRequest25f(t *testing.T, source string) []byte {
	t.Helper()
	dts, err := os.ReadFile("testdata/array-facts-v8/minimal-arrays-v8.d.ts")
	if err != nil {
		t.Fatal(err)
	}
	r := arrayFactsRequest{Family: "tsfg.array-facts-request-candidate", Version: 8,
		Input: arrayFactsInput{SourceRoot: "/src", RootFile: "/src/main.ts", Config: arrayFactsConfig{Strict: true, NoEmit: true},
			Declarations: arrayFactsDeclarations{Version: "minimal-arrays-v8", Paths: []string{"/src/minimal-arrays-v8.d.ts"}},
			Files:        []arrayFactsInputFile{{Path: "/src/main.ts", Content: source, SHA256: arrayFactsSHA([]byte(source))}, {Path: "/src/minimal-arrays-v8.d.ts", Content: string(dts), SHA256: arrayFactsSHA(dts)}}}}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	v := tsjson.Value(b)
	if err := v.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	return append(v, '\n')
}
func genericArrayPublic25f(t *testing.T, source string) map[string]any {
	t.Helper()
	return genericArrayPublicRequest25f(t, genericArrayRequest25f(t, source), source)
}
func genericArrayPublicRequest25f(t *testing.T, input []byte, source string) map[string]any {
	t.Helper()
	var out bytes.Buffer
	if err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(input), &out); err != nil {
		t.Fatalf("actual v8 source failed: %v", err)
	}
	genericArrayCapture25f(t, input, source, out.Bytes())
	var packet map[string]any
	if err := json.Unmarshal(out.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	if packet["version"] != float64(8) {
		t.Fatalf("wrong version: %v", packet["version"])
	}
	return packet["payload"].(map[string]any)
}
func genericArrayCapture25f(t *testing.T, input []byte, source string, output []byte) {
	t.Helper()
	if root := os.Getenv("TSFG_ARRAY_V8_CAPTURE_ROOT"); root != "" {
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
		id := arrayFactsSHA([]byte(source))
		for name, data := range map[string][]byte{"source.ts": []byte(source), "request.json": input, "package.json": output} {
			if err := os.WriteFile(filepath.Join(root, id+"-"+name), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
}
func TestArrayGenericV8UndefinedDensePublic25f(t *testing.T) {
	p := genericArrayPublic25f(t, "export function probe(i:u32): i32 | undefined { const a: (i32 | undefined)[] = [undefined, 7]; a[i] = undefined; return a[i]; }")
	types := map[string]map[string]any{}
	for _, raw := range p["types"].([]any) {
		v := raw.(map[string]any)
		types[v["id"].(string)] = v
	}
	for _, id := range []string{"i32", "u32", "undefined", "union<i32,undefined>", "array<union<i32,undefined>>"} {
		if types[id] == nil {
			t.Fatalf("missing %s", id)
		}
	}
	fn := p["functions"].([]any)[0].(map[string]any)
	body := fn["body"].([]any)
	lit := body[0].(map[string]any)["initializer"].(map[string]any)
	if lit["elements"].([]any)[0].(map[string]any)["kind"] != "undefinedLiteral" {
		t.Fatalf("undefined slot lost: %v", lit)
	}
	store := body[1].(map[string]any)["value"].(map[string]any)
	if store["kind"] != "elementAssignment" || store["value"].(map[string]any)["kind"] != "undefinedLiteral" {
		t.Fatalf("store lost: %v", store)
	}
	read := body[2].(map[string]any)["value"].(map[string]any)
	if read["kind"] != "checkedElementAccess" || read["type"] != "union<i32,undefined>" {
		t.Fatalf("checked T read lost: %v", read)
	}
}
