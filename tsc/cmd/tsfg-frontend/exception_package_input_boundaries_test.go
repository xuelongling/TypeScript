// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestExceptionFactsCommandV1InputBoundaries(t *testing.T) {
	f := exceptionBoundaryFirstForTest(t)
	if !f.control(t) {
		return
	}
	mutated := func(change func(map[string]any)) []byte {
		value := exceptionBoundaryCloneForTest(t, f.envelope)
		change(exceptionBoundaryInputForTest(value))
		return nullableFactsBoundaryCanonical(t, value)
	}
	cases := []struct {
		name, category, reason string
		change                 func(map[string]any)
	}{
		{"emptyFiles", "input", "empty supplied closure", func(v map[string]any) { v["files"] = []any{} }},
		{"missingRoot", "input", "root is not supplied", func(v map[string]any) { v["rootFile"] = "/src/missing.ts" }},
		{"rootDTS", "input", "root cannot be DTS", func(v map[string]any) { v["rootFile"] = "/src/minimal-integers-v3.d.ts" }},
		{"sourceRootSlash", "input", "source root must be a canonical non-root path", func(v map[string]any) { v["sourceRoot"] = "/" }},
		{"sourceRootTrailing", "input", "source root trailing slash", func(v map[string]any) { v["sourceRoot"] = "/src/" }},
		{"rootEscape", "input", "root path cannot escape sourceRoot", func(v map[string]any) { v["rootFile"] = "/src/../main.ts" }},
		{"fileEscape", "input", "file path cannot escape sourceRoot", func(v map[string]any) { v["files"].([]any)[0].(map[string]any)["path"] = "/else/main.ts" }},
		{"fileBackslash", "input", "file path cannot use host separators", func(v map[string]any) { v["files"].([]any)[0].(map[string]any)["path"] = "/src/part\\main.ts" }},
		{"duplicateFile", "input", "duplicate supplied file", func(v map[string]any) { files := v["files"].([]any); v["files"] = append(files, files[0]) }},
		{"extraOrdinaryTS", "input", "non-closure cannot contain extra TS", func(v map[string]any) {
			v["files"] = append(v["files"].([]any), map[string]any{"path": "/src/extra.ts", "content": "export function extra(value: i32): i32 { throw value; }"})
		}},
		{"undeclaredDTS", "input", "extra DTS is not a pinned profile member", func(v map[string]any) {
			v["files"] = append(v["files"].([]any), map[string]any{"path": "/src/extra.d.ts", "content": ""})
		}},
		{"missingDTS", "input", "profile DTS is not supplied", func(v map[string]any) { v["files"] = v["files"].([]any)[:1] }},
		{"wrongDTSBytes", "identity", "pinned declaration bytes differ", func(v map[string]any) {
			file := v["files"].([]any)[1].(map[string]any)
			file["content"] = file["content"].(string) + "\n"
		}},
		{"declarationsEmpty", "input", "profile declaration paths cannot be empty", func(v map[string]any) { v["declarations"].(map[string]any)["paths"] = []any{} }},
		{"declarationsDuplicate", "input", "profile declaration paths cannot repeat", func(v map[string]any) {
			v["declarations"].(map[string]any)["paths"] = []any{"/src/minimal-integers-v3.d.ts", "/src/minimal-integers-v3.d.ts"}
		}},
		{"unsupportedProfile", "input", "minimal-bool-v1 is outside private v1", func(v map[string]any) { v["declarations"].(map[string]any)["version"] = "minimal-bool-v1" }},
		{"profileDTSMismatch", "input", "float profile needs its own primary DTS path", func(v map[string]any) { v["declarations"].(map[string]any)["version"] = "minimal-floats-v4" }},
		{"scalarWithTarget", "input", "scalar profile must omit target", func(v map[string]any) { v["declarations"].(map[string]any)["version"] = "minimal-scalars-v2" }},
		{"missingTarget", "input", "integer profile requires target", func(v map[string]any) { delete(v, "target") }},
		{"nullTarget", "schema", "null target is distinct from omission", func(v map[string]any) { v["target"] = nil }},
		{"pointerBits", "input", "required target pointer width differs", func(v map[string]any) { v["target"].(map[string]any)["pointerBits"] = "32" }},
		{"triple", "input", "required target triple differs", func(v map[string]any) { v["target"].(map[string]any)["triple"] = "aarch64-unknown-linux-gnu" }},
		{"strict", "input", "strict must be true", func(v map[string]any) { v["config"].(map[string]any)["strict"] = false }},
		{"noEmit", "input", "noEmit must be true", func(v map[string]any) { v["config"].(map[string]any)["noEmit"] = false }},
		{"allowJs", "input", "allowJs must be false", func(v map[string]any) { v["config"].(map[string]any)["allowJs"] = true }},
		{"decoratorMode", "input", "marker-free integer input cannot enable decorators", func(v map[string]any) { v["config"].(map[string]any)["decoratorMode"] = "standard" }},
		{"shaEmpty", "schema", "explicit empty optional sha256 cannot equal absence", func(v map[string]any) { v["files"].([]any)[0].(map[string]any)["sha256"] = "" }},
		{"shaNonDigest", "identity", "optional sha256 must be complete content digest", func(v map[string]any) { v["files"].([]any)[0].(map[string]any)["sha256"] = "not-a-sha256" }},
		{"shaMismatch", "identity", "optional sha256 must match original content", func(v map[string]any) {
			v["files"].([]any)[0].(map[string]any)["sha256"] = "sha256:" + strings.Repeat("0", 64)
		}},
		{"shaNull", "schema", "optional sha256 null cannot equal absence", func(v map[string]any) { v["files"].([]any)[0].(map[string]any)["sha256"] = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { exceptionBoundaryRejectForTest(t, mutated(tc.change), tc.category, tc.reason) })
	}
	// Both controls change exact requestId while preserving independently expected
	// normalized input/payload. Expected is the test oracle, never producer output.
	for _, name := range []string{"matchingOptionalSHA", "reorderedFiles"} {
		t.Run(name, func(t *testing.T) {
			data := mutated(func(v map[string]any) {
				files := v["files"].([]any)
				if name == "reorderedFiles" {
					files[0], files[1] = files[1], files[0]
				} else {
					for _, item := range files {
						file := item.(map[string]any)
						file["sha256"] = exceptionPackageExportsDigestForTest([]byte(file["content"].(string)))
					}
				}
			})
			var expected map[string]any
			if err := json.Unmarshal(f.expected, &expected); err != nil {
				t.Fatal(err)
			}
			expected["requestId"] = exceptionPackageExportsDigestForTest(append([]byte("tsfg.exception-facts.request.v1\x00"), data...))
			before := bytes.Clone(data)
			output := &exceptionBoundaryWriterForTest{limit: -1}
			err := runFrontendCommand(t.Context(), []string{"--exception-facts-io-v1"}, bytes.NewReader(data), output)
			if err != nil || output.calls != 1 || !bytes.Equal(output.accepted, nullableFactsBoundaryCanonical(t, expected)) || !bytes.Equal(data, before) {
				t.Fatalf("normalized positive control failed: err=%v writes=%d", err, output.calls)
			}
		})
	}
}

func TestExceptionFactsCommandV1ClosureInputBoundaries(t *testing.T) {
	f := exceptionBoundaryFirstForTest(t)
	base := exceptionBoundaryCloneForTest(t, f.envelope)
	root := map[string]any{"kind": "application", "sourcePath": "/src/main.ts", "name": "entry"}
	exceptionBoundaryInputForTest(base)["compilationClosure"] = map[string]any{"roots": []any{root}, "requiredRoots": []any{}}
	data := nullableFactsBoundaryCanonical(t, base)
	// Input-mode control only. Full module facts/export semantics retain their
	// independent first module oracle; no expected field is taken from this result.
	output := &exceptionBoundaryWriterForTest{limit: -1}
	err := runFrontendCommand(t.Context(), []string{"--exception-facts-io-v1"}, bytes.NewReader(data), output)
	if err != nil || output.calls != 1 || len(output.accepted) == 0 {
		t.Fatalf("actual named closure-root control failed before input boundaries: %v", err)
	}
	cases := []struct {
		name   string
		change func(map[string]any)
	}{
		{"emptyRoots", func(v map[string]any) { v["roots"] = []any{} }},
		{"rootKind", func(v map[string]any) { v["roots"].([]any)[0].(map[string]any)["kind"] = "not-a-root-kind" }},
		{"foreignRootSource", func(v map[string]any) { v["roots"].([]any)[0].(map[string]any)["sourcePath"] = "/src/missing.ts" }},
		{"emptyRootName", func(v map[string]any) { v["roots"].([]any)[0].(map[string]any)["name"] = "" }},
		{"moduleInitializationName", func(v map[string]any) {
			r := v["roots"].([]any)[0].(map[string]any)
			r["kind"] = "module-initialization"
			r["name"] = "entry"
		}},
		{"duplicateRoot", func(v map[string]any) { roots := v["roots"].([]any); v["roots"] = append(roots, roots[0]) }},
		{"foreignRequiredRoot", func(v map[string]any) {
			v["requiredRoots"] = []any{map[string]any{"kind": "application", "sourcePath": "/src/main.ts", "name": "absent"}}
		}},
		{"duplicateRequiredRoot", func(v map[string]any) { r := v["roots"].([]any)[0]; v["requiredRoots"] = []any{r, r} }},
		{"absentActualNamedBinding", func(v map[string]any) { v["roots"].([]any)[0].(map[string]any)["name"] = "absent" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			value := exceptionBoundaryCloneForTest(t, base)
			tc.change(exceptionBoundaryInputForTest(value)["compilationClosure"].(map[string]any))
			exceptionBoundaryRejectForTest(t, nullableFactsBoundaryCanonical(t, value), "input", "closure tuple/subset/actual binding: "+tc.name)
		})
	}
}
