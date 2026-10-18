// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"testing"
)

func TestStringFactsRequestBoundaries(t *testing.T) {
	valid := stringBoundaryRequest24(t, "export function original(): string { return 'ok'; }")
	cases := []struct {
		name, kind string
		edit       func(map[string]any)
	}{
		{"unknownRoot", "schema", func(r map[string]any) { r["extra"] = true }},
		{"family", "schema", func(r map[string]any) { r["family"] = "other" }},
		{"version", "schema", func(r map[string]any) { r["version"] = 2 }},
		{"missingVersion", "schema", func(r map[string]any) { delete(r, "version") }},
		{"missingFalse", "schema", func(r map[string]any) { delete(r["input"].(map[string]any)["config"].(map[string]any), "allowJs") }},
		{"unknownInput", "schema", func(r map[string]any) { r["input"].(map[string]any)["target"] = map[string]any{} }},
		{"strictFalse", "input", func(r map[string]any) { r["input"].(map[string]any)["config"].(map[string]any)["strict"] = false }},
		{"allowJsTrue", "input", func(r map[string]any) { r["input"].(map[string]any)["config"].(map[string]any)["allowJs"] = true }},
		{"profile", "input", func(r map[string]any) {
			r["input"].(map[string]any)["declarations"].(map[string]any)["version"] = "minimal-integers-v3"
		}},
		{"rootPath", "input", func(r map[string]any) { r["input"].(map[string]any)["sourceRoot"] = "/owned/../owned" }},
		{"filesMissing", "input", func(r map[string]any) { i := r["input"].(map[string]any); i["files"] = i["files"].([]any)[:1] }},
		{"filesReverse", "input", func(r map[string]any) { f := r["input"].(map[string]any)["files"].([]any); f[0], f[1] = f[1], f[0] }},
		{"duplicatePath", "input", func(r map[string]any) {
			f := r["input"].(map[string]any)["files"].([]any)
			f[1].(map[string]any)["path"] = f[0].(map[string]any)["path"]
		}},
		{"filesCardinality", "limit", func(r map[string]any) {
			i := r["input"].(map[string]any)
			f := i["files"].([]any)
			i["files"] = append(f, f[0])
		}},
		{"badHash", "identity", func(r map[string]any) {
			r["input"].(map[string]any)["files"].([]any)[1].(map[string]any)["sha256"] = "0000000000000000000000000000000000000000000000000000000000000000"
		}},
		{"prefixedHash", "identity", func(r map[string]any) {
			f := r["input"].(map[string]any)["files"].([]any)[1].(map[string]any)
			f["sha256"] = "sha256:" + f["sha256"].(string)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { stringBoundaryFailure24(t, stringBoundaryMutate24(t, valid, tc.edit), tc.kind) })
	}
	for _, tc := range []struct {
		name, kind string
		data       []byte
	}{
		{"noLF", "canonical", valid[:len(valid)-1]},
		{"twoLF", "canonical", append(bytes.Clone(valid), '\n')},
		{"leadingSpace", "canonical", append([]byte(" "), valid...)},
		{"trailingValue", "canonical", append(bytes.Clone(valid), []byte("{}")...)},
		{"duplicateKey", "json", bytes.Replace(valid, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1)},
		{"nullRoot", "schema", []byte("null\n")},
		{"unpairedJSONSurrogate", "json", []byte(`{"x":"\ud800"}` + "\n")},
		{"invalidRawUTF8", "unicode", []byte{0xff, '\n'}},
		{"nonfiniteNumber", "json", []byte(`{"version":1e400}` + "\n")},
		// Raw JSON byte limit only, not a Source/Program stress test.
		{"inputByteLimit", "limit", bytes.Repeat([]byte{' '}, 2097153)},
	} {
		t.Run(tc.name, func(t *testing.T) { stringBoundaryFailure24(t, tc.data, tc.kind) })
	}
}
