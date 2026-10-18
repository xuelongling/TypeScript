// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestStringFactsV2IdentityAndCanonical(t *testing.T) {
	valid := stringConcatOracleRead24e(t, "request.json", "5815ef980b77978c732239d56b0a1e6966183e0d8710de16d3b9448d19586028")
	for _, tc := range []struct {
		name, kind string
		edit       func(map[string]any)
	}{
		{"unknownRoot", "schema", func(r map[string]any) { r["extra"] = true }},
		{"unknownInput", "schema", func(r map[string]any) { r["input"].(map[string]any)["target"] = "esnext" }},
		{"unknownConfig", "schema", func(r map[string]any) { r["input"].(map[string]any)["config"].(map[string]any)["extra"] = false }},
		{"unknownFile", "schema", func(r map[string]any) {
			r["input"].(map[string]any)["files"].([]any)[0].(map[string]any)["owner"] = "x"
		}},
		{"unknownDeclarations", "schema", func(r map[string]any) { r["input"].(map[string]any)["declarations"].(map[string]any)["extra"] = 0 }},
		{"family", "schema", func(r map[string]any) { r["family"] = "other" }},
		{"version", "schema", func(r map[string]any) { r["version"] = 3 }},
		{"versionString", "schema", func(r map[string]any) { r["version"] = "2" }},
		{"missingVersion", "schema", func(r map[string]any) { delete(r, "version") }},
		{"missingFalse", "schema", func(r map[string]any) { delete(r["input"].(map[string]any)["config"].(map[string]any), "allowJs") }},
		{"nullFiles", "schema", func(r map[string]any) { r["input"].(map[string]any)["files"] = nil }},
		{"nullDeclarationPaths", "schema", func(r map[string]any) { r["input"].(map[string]any)["declarations"].(map[string]any)["paths"] = nil }},
		{"strictFalse", "input", func(r map[string]any) { r["input"].(map[string]any)["config"].(map[string]any)["strict"] = false }},
		{"noEmitFalse", "input", func(r map[string]any) { r["input"].(map[string]any)["config"].(map[string]any)["noEmit"] = false }},
		{"allowJsTrue", "input", func(r map[string]any) { r["input"].(map[string]any)["config"].(map[string]any)["allowJs"] = true }},
		{"profile", "input", func(r map[string]any) {
			r["input"].(map[string]any)["declarations"].(map[string]any)["version"] = "minimal-integers-v3"
		}},
		{"noncanonicalRoot", "input", func(r map[string]any) { r["input"].(map[string]any)["sourceRoot"] = "/oracle24e/../oracle24e" }},
		{"outsideRoot", "input", func(r map[string]any) { r["input"].(map[string]any)["rootFile"] = "/outside/main.ts" }},
		{"nulPath", "input", func(r map[string]any) { r["input"].(map[string]any)["sourceRoot"] = "/oracle24e\x00" }},
		{"backslashPath", "input", func(r map[string]any) { r["input"].(map[string]any)["sourceRoot"] = "/oracle24e\\child" }},
		{"ancestorFiles", "input", func(r map[string]any) {
			i := r["input"].(map[string]any)
			child := "/oracle24e/minimal-scalars-v2.d.ts/main.ts"
			i["rootFile"] = child
			files := i["files"].([]any)
			files[0].(map[string]any)["path"] = child
			// Keep both original contents/digests and the exact DTS/profile;
			// only the TS path moves beneath the existing DTS file path.
			slices.SortFunc(files, func(a, b any) int {
				return strings.Compare(a.(map[string]any)["path"].(string), b.(map[string]any)["path"].(string))
			})
		}},
		{"missingFile", "input", func(r map[string]any) { i := r["input"].(map[string]any); i["files"] = i["files"].([]any)[:1] }},
		{"reversedFiles", "input", func(r map[string]any) { f := r["input"].(map[string]any)["files"].([]any); f[0], f[1] = f[1], f[0] }},
		{"duplicatePath", "input", func(r map[string]any) {
			f := r["input"].(map[string]any)["files"].([]any)
			f[1].(map[string]any)["path"] = f[0].(map[string]any)["path"]
		}},
		{"thirdFile", "limit", func(r map[string]any) {
			i := r["input"].(map[string]any)
			f := i["files"].([]any)
			i["files"] = append(f, f[0])
		}},
		{"badHash", "identity", func(r map[string]any) {
			r["input"].(map[string]any)["files"].([]any)[0].(map[string]any)["sha256"] = strings.Repeat("0", 64)
		}},
		{"prefixedHash", "identity", func(r map[string]any) {
			f := r["input"].(map[string]any)["files"].([]any)[0].(map[string]any)
			f["sha256"] = "sha256:" + f["sha256"].(string)
		}},
		{"upperHash", "identity", func(r map[string]any) {
			f := r["input"].(map[string]any)["files"].([]any)[0].(map[string]any)
			f["sha256"] = strings.ToUpper(f["sha256"].(string))
		}},
		{"staleSourceDigest", "identity", func(r map[string]any) {
			f := r["input"].(map[string]any)["files"].([]any)[0].(map[string]any)
			f["content"] = f["content"].(string) + "\n"
		}},
		{"rehashedChangedDTS", "identity", func(r map[string]any) {
			f := r["input"].(map[string]any)["files"].([]any)[1].(map[string]any)
			text := f["content"].(string) + "// changed\n"
			sum := sha256.Sum256([]byte(text))
			f["content"], f["sha256"] = text, hex.EncodeToString(sum[:])
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := stringConcatMutate24e(t, valid, tc.edit)
			if tc.name == "ancestorFiles" {
				r := stringConcatInput24e(t, input)
				if len(r.Files) != 2 || r.Files[0].Path != "/oracle24e/minimal-scalars-v2.d.ts" || r.Files[1].Path != r.RootFile || !strings.HasPrefix(r.RootFile, r.Files[0].Path+"/") {
					t.Fatal("ancestor fixture must contain the real parent and selected child in path order")
				}
			}
			failure := stringConcatFailure24e(t, input, tc.kind)
			if tc.name == "ancestorFiles" && failure.Detail != "input files must not be ancestors of another file" {
				t.Fatalf("the real ancestor boundary was masked: %+v", failure)
			}
		})
	}
	for _, tc := range []struct {
		name, kind string
		data       []byte
	}{
		{"noLF", "canonical", valid[:len(valid)-1]},
		{"twoLF", "canonical", append(bytes.Clone(valid), '\n')},
		{"CRLF", "canonical", append(bytes.Clone(valid[:len(valid)-1]), '\r', '\n')},
		{"leadingSpace", "canonical", append([]byte(" "), valid...)},
		{"trailingSpace", "canonical", append(bytes.Clone(valid[:len(valid)-1]), ' ', '\n')},
		{"trailingValue", "canonical", append(bytes.Clone(valid), []byte("{}")...)},
		{"duplicateKey", "json", bytes.Replace(valid, []byte("\"version\":2"), []byte("\"version\":2,\"version\":2"), 1)},
		{"escapeEquivalentDuplicate", "json", bytes.Replace(valid, []byte("\"version\":2"), []byte("\"version\":2,\"\\u0076ersion\":2"), 1)},
		{"escapedMemberSpelling", "canonical", bytes.Replace(valid, []byte("\"family\""), []byte("\"\\u0066amily\""), 1)},
		{"unpairedJSONHigh", "json", []byte("{\"x\":\"\\ud800\"}\n")},
		{"unpairedJSONLow", "json", []byte("{\"x\":\"\\udc00\"}\n")},
		{"invalidUTF8", "unicode", []byte{0xff, '\n'}},
		{"BOM", "json", append([]byte{0xef, 0xbb, 0xbf}, valid...)},
		{"nullRoot", "schema", []byte("null\n")},
		{"nonfinite", "json", []byte("{\"version\":1e400}\n")},
		{"unsafeInteger", "json", []byte("{\"version\":9007199254740992}\n")},
		{"negativeInteger", "json", []byte("{\"version\":-1}\n")},
		{"malformed", "json", []byte("{\n")},
	} {
		t.Run(tc.name, func(t *testing.T) { stringConcatFailure24e(t, tc.data, tc.kind) })
	}
	t.Run("versionCrossing", func(t *testing.T) {
		v1 := stringOracleRead24(t, "request.json", "153b9e72ac0dec01815f05218cad4bc7377a46456b5030e0659e4223474fdec5")
		for _, tc := range []struct {
			flag string
			data []byte
		}{{"--string-facts-io-v2", v1}, {"--string-facts-io-v1", valid}} {
			r := &stringReaderProbe24{input: bytes.NewReader(tc.data)}
			w := &stringWriterProbe24{}
			err := runFrontendCommand(t.Context(), []string{tc.flag}, r, w)
			var f *stringFactsFailure
			if !errors.As(err, &f) || f.Kind != "schema" || f.Code != "r03-string-facts-schema" || r.calls == 0 || w.calls != 0 {
				t.Fatalf("cross-version selection: %v reads=%d writes=%d", err, r.calls, w.calls)
			}
		}
		v1Expected := stringOracleRead24(t, "expected.json", "64e164fc00016eda6052453993c0eab1de9cac0f4879de3b79fe42df4b38a468")
		var output bytes.Buffer
		if err := runFrontendCommand(t.Context(), []string{"--string-facts-io-v1"}, bytes.NewReader(v1), &output); err != nil || !bytes.Equal(output.Bytes(), v1Expected) {
			t.Fatalf("v1 complete bytes changed: %v", err)
		}
	})
	t.Run("v1StillRejectsLegalConcat", func(t *testing.T) {
		input := stringConcatMutate24e(t, valid, func(r map[string]any) { r["version"] = 1 })
		r := stringConcatInput24e(t, input)
		official := throwBoundaryProgramForTest(t, r)
		throwBoundaryDiagnosticsForTest(t, official)
		w := &stringWriterProbe24{}
		err := runFrontendCommand(t.Context(), []string{"--string-facts-io-v1"}, bytes.NewReader(input), w)
		var f *stringFactsFailure
		if !errors.As(err, &f) || f.Kind != "capability" || f.RuleID != "r01-ts-baseline" || w.calls != 0 {
			t.Fatalf("v1 concat capability changed: %v", err)
		}
	})
}
