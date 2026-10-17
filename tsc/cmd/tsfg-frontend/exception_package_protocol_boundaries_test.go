// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestExceptionFactsCommandV1StrictRequestBoundaries(t *testing.T) {
	f := exceptionBoundaryFirstForTest(t)
	if !f.control(t) {
		return
	}
	mutated := func(change func(map[string]any)) []byte {
		value := exceptionBoundaryCloneForTest(t, f.envelope)
		change(value)
		return nullableFactsBoundaryCanonical(t, value)
	}
	cases := []struct {
		name, category, reason string
		data                   []byte
	}{
		{"family", "schema", "foreign private family", mutated(func(v map[string]any) { v["family"] = "tsfg.nullable-facts-request-candidate" })},
		{"version", "schema", "unsupported private version", mutated(func(v map[string]any) { v["version"] = 2 })},
		{"schemaHash", "schema", "foreign descriptor identity", mutated(func(v map[string]any) { v["schemaHash"] = "sha256:" + strings.Repeat("0", 64) })},
		{"familyType", "schema", "family must be string", mutated(func(v map[string]any) { v["family"] = false })},
		{"versionType", "schema", "version must be integer", mutated(func(v map[string]any) { v["version"] = true })},
		{"unknownEnvelope", "schema", "unknown envelope key", mutated(func(v map[string]any) { v["unknown"] = true })},
		{"unknownInput", "schema", "unknown input key", mutated(func(v map[string]any) { exceptionBoundaryInputForTest(v)["unknown"] = true })},
		{"unknownFile", "schema", "unknown file key", mutated(func(v map[string]any) { exceptionBoundaryFilesForTest(v)[0].(map[string]any)["unknown"] = true })},
		{"unknownConfig", "schema", "unknown config key", mutated(func(v map[string]any) { exceptionBoundaryInputForTest(v)["config"].(map[string]any)["unknown"] = true })},
		{"unknownTarget", "schema", "unknown target key", mutated(func(v map[string]any) { exceptionBoundaryInputForTest(v)["target"].(map[string]any)["unknown"] = true })},
		{"unknownDeclarations", "schema", "unknown declarations key", mutated(func(v map[string]any) {
			exceptionBoundaryInputForTest(v)["declarations"].(map[string]any)["unknown"] = true
		})},
		{"hostImports", "schema", "unsupported hostImports input", mutated(func(v map[string]any) { exceptionBoundaryInputForTest(v)["hostImports"] = []any{} })},
		{"caseVariant", "schema", "case variant cannot stand for family", mutated(func(v map[string]any) { v["Family"] = v["family"]; delete(v, "family") })},
		{"mixedCase", "schema", "duplicate semantic key with different case", mutated(func(v map[string]any) { v["Family"] = v["family"] })},
		{"nullInput", "schema", "required input cannot be null", mutated(func(v map[string]any) { v["input"] = nil })},
		{"arrayInput", "schema", "required input must be record", mutated(func(v map[string]any) { v["input"] = []any{} })},
		{"prototypeKey", "schema", "prototype key is not a DTO field", mutated(func(v map[string]any) { v["__proto__"] = true })},
		{"leadingSpace", "canonical", "leading whitespace is not exact JCS", append([]byte(" "), f.requestBytes...)},
		{"trailingLF", "canonical", "final LF is not exact JCS", append(bytes.Clone(f.requestBytes), '\n')},
		{"escapedSlash", "canonical", "equivalent slash escape is not exact JCS", []byte(strings.Replace(string(f.requestBytes), "\"/src/main.ts\"", "\"\\/src/main.ts\"", 1))},
		{"duplicateFamily", "json", "duplicate envelope key", append([]byte("{\"family\":\"tsfg.exception-facts-request-candidate\","), f.requestBytes[1:]...)},
		{"escapedDuplicateFamily", "json", "escape-equivalent duplicate key", append([]byte("{\"\\u0066amily\":\"tsfg.exception-facts-request-candidate\","), f.requestBytes[1:]...)},
		{"duplicateRoot", "json", "duplicate nested rootFile key", []byte(strings.Replace(string(f.requestBytes), "\"rootFile\":", "\"rootFile\":\"/src/foreign.ts\",\"rootFile\":", 1))},
		{"invalidUTF8", "unicode", "raw invalid UTF8", append([]byte{0xff}, f.requestBytes...)},
		{"BOM", "json", "UTF8 BOM is not a JSON token", append([]byte{0xef, 0xbb, 0xbf}, f.requestBytes...)},
		{"loneSurrogate", "json", "JSON escape is not a Unicode scalar", []byte(strings.Replace(string(f.requestBytes), "\"tsfg.exception-facts-request-candidate\"", "\"\\ud800\"", 1))},
		{"negativeVersion", "json", "negative wire number", mutated(func(v map[string]any) { v["version"] = -1 })},
		{"unsafeInteger", "json", "unsafe wire integer", mutated(func(v map[string]any) { v["version"] = uint64(9007199254740992) })},
		{"negativeZero", "canonical", "negative zero changes under JCS", []byte(strings.Replace(string(f.requestBytes), "\"version\":1", "\"version\":-0", 1))},
		{"truncated", "json", "unterminated request object", bytes.Clone(f.requestBytes[:len(f.requestBytes)-1])},
		{"empty", "limit", "empty frame violates byte minimum", nil},
	}
	for _, key := range []string{"family", "version", "schemaHash", "input"} {
		key := key
		cases = append(cases, struct {
			name, category, reason string
			data                   []byte
		}{"missing-" + key, "schema", "missing required " + key, mutated(func(v map[string]any) { delete(v, key) })})
	}
	// These depth controls are only about 2.4KiB; no token/string/byte stress input.
	for _, depth := range []int{128, 129} {
		category := "schema"
		if depth == 129 {
			category = "limit"
		}
		data := []byte("{\"a\":" + strings.Repeat("[", depth-1) + "0" + strings.Repeat("]", depth-1) + "," + string(f.requestBytes[1:]))
		cases = append(cases, struct {
			name, category, reason string
			data                   []byte
		}{map[int]string{128: "depth128ReachesSchema", 129: "depth129Limit"}[depth], category, "exact JSON depth boundary", data})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if bytes.Equal(tc.data, f.requestBytes) {
				t.Fatal("invalid control is a no-op")
			}
			exceptionBoundaryRejectForTest(t, tc.data, tc.category, tc.reason)
		})
	}
}
