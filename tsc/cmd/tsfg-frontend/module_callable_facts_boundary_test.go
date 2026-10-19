// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

func mcMutableRequest23d(t *testing.T, name string) mcObject23d {
	t.Helper()
	_, input, _ := mcPublicFixture23d(t, name)
	var object mcObject23d
	if err := json.Unmarshal(input, &object); err != nil {
		t.Fatal(err)
	}
	return object
}
func mcInputObject23d(object mcObject23d) mcObject23d { return object["input"].(map[string]any) }
func mcRequestBytes23d(t *testing.T, object mcObject23d) []byte {
	return append(mcCanonical23d(t, object), '\n')
}
func mcReplaceSource23d(t *testing.T, object mcObject23d, path string, change func(string) string) {
	t.Helper()
	for _, file := range mcInputObject23d(object)["files"].([]any) {
		row := file.(map[string]any)
		if row["path"] == path {
			content := change(row["content"].(string))
			row["content"] = content
			row["sha256"] = strings.TrimPrefix(mcHash23d([]byte(content)), "sha256:")
			return
		}
	}
	t.Fatalf("original source %s missing", path)
}
func mcAppendSource23d(object mcObject23d, path, content string) {
	input := mcInputObject23d(object)
	files := input["files"].([]any)
	files = append(files, mcObject23d{"path": path, "content": content, "sha256": strings.TrimPrefix(mcHash23d([]byte(content)), "sha256:")})
	sort.Slice(files, func(i, j int) bool {
		return files[i].(map[string]any)["path"].(string) < files[j].(map[string]any)["path"].(string)
	})
	input["files"] = files
}
func mcRejectPublic23d(t *testing.T, input []byte) {
	t.Helper()
	before := bytes.Clone(input)
	output := &mcPublicWriter23d{}
	err := runFrontendCommand(t.Context(), []string{"--module-callable-facts-io-v1"}, bytes.NewReader(input), output)
	if err == nil || output.writes != 0 || output.Len() != 0 {
		t.Fatalf("invalid complete candidate must fail before publication: err=%v writes=%d bytes=%d", err, output.writes, output.Len())
	}
	if !bytes.Equal(input, before) {
		t.Fatal("failed public candidate mutated caller input")
	}
	t.Logf("TSFG_MODULE_CALLABLE_23D_BOUNDARY_REJECT bytes=%d digest=%s error=%v", len(input), mcHash23d(input), err)
}

func TestModuleCallableFactsStrictWire(t *testing.T) {
	_, input, _ := mcPublicFixture23d(t, "first-rr")
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, input, "", "  "); err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"missing-lf":         bytes.TrimSuffix(bytes.Clone(input), []byte("\n")),
		"double-lf":          append(bytes.Clone(input), '\n'),
		"crlf":               append(bytes.TrimSuffix(bytes.Clone(input), []byte("\n")), '\r', '\n'),
		"leading-space":      append([]byte(" "), input...),
		"pretty-json":        pretty.Bytes(),
		"bom":                append([]byte{0xef, 0xbb, 0xbf}, input...),
		"trailing-value":     append(bytes.Clone(input), []byte("{}")...),
		"duplicate-top":      bytes.Replace(input, []byte("\"version\":1"), []byte("\"version\":1,\"version\":1"), 1),
		"duplicate-config":   bytes.Replace(input, []byte("\"strict\":true"), []byte("\"strict\":true,\"strict\":true"), 1),
		"negative-zero":      bytes.Replace(input, []byte("\"version\":1"), []byte("\"version\":-0"), 1),
		"unsafe-integer":     bytes.Replace(input, []byte("\"version\":1"), []byte("\"version\":9007199254740992"), 1),
		"unpaired-surrogate": bytes.Replace(input, []byte("\"sourceRoot\":\"/src\""), []byte("\"sourceRoot\":\"/src/\\ud800\""), 1),
		"invalid-utf8":       append([]byte{0xff}, input...),
	}
	names := []string{}
	for name := range cases {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			if bytes.Equal(cases[name], input) {
				t.Fatal("strict-wire negative failed to mutate frozen input")
			}
			mcRejectPublic23d(t, cases[name])
		})
	}
}

func TestModuleCallableFactsIdentityTypesAndClosure(t *testing.T) {
	mutations := []struct {
		name   string
		change func(mcObject23d)
	}{
		{"extra-envelope", func(r mcObject23d) { r["extra"] = true }},
		{"extra-input", func(r mcObject23d) { mcInputObject23d(r)["extra"] = true }},
		{"wrong-schema", func(r mcObject23d) { r["schemaHash"] = "sha256:" + strings.Repeat("0", 64) }},
		{"wrong-target", func(r mcObject23d) { mcInputObject23d(r)["target"].(map[string]any)["pointerBits"] = "32" }},
		{"wrong-file-digest", func(r mcObject23d) {
			mcInputObject23d(r)["files"].([]any)[0].(map[string]any)["sha256"] = strings.Repeat("0", 64)
		}},
		{"missing-import-file", func(r mcObject23d) {
			input := mcInputObject23d(r)
			files := []any{}
			for _, file := range input["files"].([]any) {
				if file.(map[string]any)["path"] != "/src/state.ts" {
					files = append(files, file)
				}
			}
			input["files"] = files
		}},
		{"duplicate-file", func(r mcObject23d) {
			input := mcInputObject23d(r)
			input["files"] = append(input["files"].([]any), input["files"].([]any)[0])
		}},
		{"unsorted-files", func(r mcObject23d) {
			files := mcInputObject23d(r)["files"].([]any)
			files[0], files[1] = files[1], files[0]
		}},
		{"foreign-root-file", func(r mcObject23d) { mcInputObject23d(r)["rootFile"] = "/src/missing.ts" }},
		{"foreign-closure-target", func(r mcObject23d) {
			root := mcInputObject23d(r)["compilationClosure"].(map[string]any)["roots"].([]any)[0].(map[string]any)
			root["name"] = "missing"
		}},
		{"duplicate-closure-root", func(r mcObject23d) {
			closure := mcInputObject23d(r)["compilationClosure"].(map[string]any)
			closure["roots"] = append(closure["roots"].([]any), closure["roots"].([]any)[0])
		}},
		{"required-root-missing", func(r mcObject23d) {
			closure := mcInputObject23d(r)["compilationClosure"].(map[string]any)
			closure["requiredRoots"] = []any{mcObject23d{"kind": "kernel", "sourcePath": "/src/app.ts", "name": "missing"}}
		}},
		{"declaration-tamper", func(r mcObject23d) {
			mcReplaceSource23d(t, r, "/src/minimal-integers-v3.d.ts", func(s string) string { return s + "\n" })
		}},
		{"official-type-error", func(r mcObject23d) {
			mcReplaceSource23d(t, r, "/src/state.ts", func(s string) string { return strings.Replace(s, "return shared;", "return missing;", 1) })
		}},
		{"unentered-type-error", func(r mcObject23d) {
			mcAppendSource23d(r, "/src/unentered.ts", "export function unentered(): bool { return missing; }\n")
		}},
		{"callable-global", func(r mcObject23d) {
			mcReplaceSource23d(t, r, "/src/app.ts", func(s string) string { return s + "export const stored: () => bool = chosen;\n" })
		}},
		{"nested-function", func(r mcObject23d) {
			mcReplaceSource23d(t, r, "/src/app.ts", func(s string) string {
				return strings.Replace(s, "const selected = identity(chosen);", "function local(): bool { return true; } const selected = identity(chosen);", 1)
			})
		}},
		{"optional-parameter", func(r mcObject23d) {
			mcReplaceSource23d(t, r, "/src/app.ts", func(s string) string { return s + "export function optional(value?: bool): bool { return true; }\n" })
		}},
		{"host-extension", func(r mcObject23d) { mcInputObject23d(r)["hostImports"] = []any{mcObject23d{"name": "untrusted"}} }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			r := mcMutableRequest23d(t, "first-rr")
			mutation.change(r)
			mcRejectPublic23d(t, mcRequestBytes23d(t, r))
		})
	}
}
