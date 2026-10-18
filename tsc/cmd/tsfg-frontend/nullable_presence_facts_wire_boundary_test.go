// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"errors"
	"testing"
)

func TestNullableFactsPresenceStrictWireBoundary(t *testing.T) {
	base := nullablePresenceFirstRead(t, "mask2-null.request.json")
	rows := []struct {
		name  string
		raw   []byte
		kind  string
		cause int
	}{
		{"empty", nil, "limit", 1},
		{"malformed", []byte("{CALLER_SECRET_20K"), "json", 1},
		{"BOM", append([]byte{239, 187, 191}, base...), "json", 1},
		{"invalid-UTF8", []byte{'{', '"', 'x', '"', ':', '"', 255, '"', '}'}, "json", 1},
		{"unpaired-high", []byte("{\"x\":\"\\ud800\"}"), "json", 1},
		{"unpaired-low", []byte("{\"x\":\"\\udfff\"}"), "json", 1},
		{"duplicate", []byte("{\"family\":1,\"family\":2}"), "json", 1},
		{"escape-duplicate", []byte("{\"family\":1,\"\\u0066amily\":2}"), "json", 1},
		{"null-policy", []byte("null"), "schema", 1},
		{"nested-null-policy", []byte("{\"input\":null}"), "schema", 1},
		{"negative-number", []byte("{\"version\":-1}"), "json", 1},
		{"unsafe-number", []byte("{\"version\":9007199254740992}"), "json", 1},
		{"fraction", []byte("{\"version\":1.5}"), "json", 1},
		{"leading-space", append([]byte(" "), base...), "canonical", 0},
		{"final-LF", append(bytes.Clone(base), '\n'), "canonical", 0},
		{"noncanonical-key-escape", bytes.Replace(base, []byte("family"), []byte("\\u0066amily"), 1), "canonical", 0},
		{"unordered-keys", []byte("{\"version\":4,\"family\":\"x\"}"), "canonical", 0},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			got := nullablePresenceBoundaryReject(t, row.raw, row.kind, nil, row.cause)
			if row.cause == 1 {
				var upstream *genericFailure
				if !errors.As(got, &upstream) || upstream == nil {
					t.Fatal("strict preflight cause lost its original generic error")
				}
			}
		})
	}
}

func TestNullableFactsPresenceExactSchemaBoundary(t *testing.T) {
	base := nullablePresenceFirstRead(t, "mask2-null.request.json")
	rows := []struct {
		name   string
		change func(map[string]any)
		cause  int
	}{
		{"family", func(v map[string]any) { v["family"] = "tsfg.nullable-facts-package-candidate" }, 0},
		{"version-3", func(v map[string]any) { v["version"] = 3 }, 0},
		{"version-5", func(v map[string]any) { v["version"] = 5 }, 0},
		{"schema-S3", func(v map[string]any) { v["schemaHash"] = nullablePresenceBoundaryS3 }, 0},
		{"schema-unknown", func(v map[string]any) {
			v["schemaHash"] = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
		}, 0},
		{"extra-top", func(v map[string]any) { v["extra"] = true }, 0},
		{"missing-version", func(v map[string]any) { delete(v, "version") }, 0},
		{"case-variant", func(v map[string]any) { v["Family"] = v["family"]; delete(v, "family") }, 0},
		{"wrong-type-version", func(v map[string]any) { v["version"] = "4" }, 1},
		{"extra-input", func(v map[string]any) { v["input"].(map[string]any)["hostImports"] = []any{} }, 0},
		{"extra-target", func(v map[string]any) { v["input"].(map[string]any)["target"].(map[string]any)["extra"] = true }, 0},
		{"missing-target", func(v map[string]any) { delete(v["input"].(map[string]any), "target") }, 0},
		{"missing-false-config", func(v map[string]any) { delete(v["input"].(map[string]any)["config"].(map[string]any), "allowJs") }, 0},
		{"wrong-bool-config", func(v map[string]any) { v["input"].(map[string]any)["config"].(map[string]any)["strict"] = "true" }, 1},
		{"extra-file", func(v map[string]any) {
			v["input"].(map[string]any)["files"].([]any)[0].(map[string]any)["metadata"] = "CALLER_SECRET_20K"
		}, 0},
		{"explicit-empty-checksum", func(v map[string]any) {
			v["input"].(map[string]any)["files"].([]any)[0].(map[string]any)["sha256"] = ""
		}, 0},
		{"extra-declarations", func(v map[string]any) {
			v["input"].(map[string]any)["declarations"].(map[string]any)["markerProfile"] = map[string]any{}
		}, 0},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			v := nullablePresenceBoundaryObject(t, base)
			row.change(v)
			nullablePresenceBoundaryReject(t, nullablePresenceBoundaryJSON(t, v), "schema", nil, row.cause)
		})
	}
	// NUL is allowed by this protocol's preflight, then the unknown field is rejected.
	nullablePresenceBoundaryReject(t, []byte("{\"x\":\"\\u0000\"}"), "schema", nil, 0)
	// A complete package contains genuine null initializer slots. Request null-policy
	// wins before DTO/schema checks; a package is never auto-detected as a request.
	nullablePresenceBoundaryReject(t, nullablePresenceFirstRead(t, "mask2-null.package.json"), "schema", nil, 1)
}

func TestNullableFactsPresenceInputClosureBoundary(t *testing.T) {
	rows := []struct {
		name, kind string
		change     func(map[string]any)
	}{
		{"relative-root", "input", func(v map[string]any) { v["sourceRoot"] = "src" }},
		{"noncanonical-root", "input", func(v map[string]any) { v["sourceRoot"] = "/src/." }},
		{"root-not-supplied", "input", func(v map[string]any) { v["rootFile"] = "/src/other.ts" }},
		{"root-case-mismatch", "input", func(v map[string]any) { v["rootFile"] = "/src/Main.ts" }},
		{"strict-false", "input", func(v map[string]any) { v["config"].(map[string]any)["strict"] = false }},
		{"emit-enabled", "input", func(v map[string]any) { v["config"].(map[string]any)["noEmit"] = false }},
		{"allow-js", "input", func(v map[string]any) { v["config"].(map[string]any)["allowJs"] = true }},
		{"target-triple", "input", func(v map[string]any) { v["target"].(map[string]any)["triple"] = "wrong" }},
		{"target-pointer", "input", func(v map[string]any) { v["target"].(map[string]any)["pointerBits"] = "32" }},
		{"target-layout", "input", func(v map[string]any) { v["target"].(map[string]any)["llvmDataLayout"] = "wrong" }},
		{"declaration-version", "input", func(v map[string]any) { v["declarations"].(map[string]any)["version"] = "minimal-scalars-v2" }},
		{"no-declaration", "input", func(v map[string]any) { v["declarations"].(map[string]any)["paths"] = []any{} }},
		{"missing-file", "input", func(v map[string]any) { v["files"] = v["files"].([]any)[:1] }},
		{"duplicate-path", "input", func(v map[string]any) { v["files"].([]any)[1].(map[string]any)["path"] = "/src/main.ts" }},
		{"dot-file-path", "input", func(v map[string]any) { v["files"].([]any)[0].(map[string]any)["path"] = "/src/./main.ts" }},
		{"outside-root", "input", func(v map[string]any) {
			v["files"].([]any)[1].(map[string]any)["path"] = "/outside/minimal-integers-v3.d.ts"
		}},
		{"undeclared-file", "input", func(v map[string]any) { v["files"].([]any)[1].(map[string]any)["path"] = "/src/extra.d.ts" }},
		{"extra-file-count", "input", func(v map[string]any) {
			v["files"] = append(v["files"].([]any), map[string]any{"path": "/src/extra.d.ts", "content": ""})
		}},
		{"source-checksum", "identity", func(v map[string]any) {
			v["files"].([]any)[0].(map[string]any)["sha256"] = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
		}},
		{"declaration-checksum", "identity", func(v map[string]any) { v["files"].([]any)[1].(map[string]any)["sha256"] = "CALLER_SECRET_20K" }},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			v := nullablePresenceBoundaryObject(t, nullablePresenceFirstRead(t, "mask2-null.request.json"))
			row.change(v["input"].(map[string]any))
			nullablePresenceBoundaryReject(t, nullablePresenceBoundaryJSON(t, v), row.kind, nil, 0)
		})
	}
}
