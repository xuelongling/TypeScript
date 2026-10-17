// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"
)

const exceptionBoundarySchemaForTest = "sha256:5221b50822965c633418462b28a9b58bfbc41b01e0fa4b6d31c50ab83805424d"
const exceptionBoundarySourceForTest = "// 😀\r\nexport function entry(value: i32): i32 {\r\n  try {\r\n    throw value;\r\n  } catch (caught) {\r\n    throw caught;\r\n  } finally {\r\n    const copied: i32 = value;\r\n  }\r\n}"

type exceptionBoundaryFixtureForTest struct {
	legacy                 request
	envelope               map[string]any
	requestBytes, expected []byte
	facts, exports         any
}

// This is the original independent first oracle, not a captured producer frame.
// Existing test-only JCS/digest/packet controls remain the shared mechanics.
func exceptionBoundaryFirstForTest(t *testing.T) exceptionBoundaryFixtureForTest {
	t.Helper()
	r := integerRequest(exceptionBoundarySourceForTest)
	symbol := func(id, name, kind, typ string, writable bool, start, end int) map[string]any {
		return map[string]any{"id": id, "name": name, "kind": kind, "type": typ, "writable": writable, "range": span{start, end}}
	}
	fn := symbol("s1", "entry", "function", "i32", false, 22, 28)
	parameter := symbol("s2", "value", "parameter", "i32", true, 29, 34)
	parameter["presence"] = map[string]any{"undefined": false, "null": false, "optional": false, "default": false, "range": span{29, 39}, "annotation": span{36, 39}}
	caught := symbol("s3", "caught", "local", "unknown", true, 87, 93)
	copied := symbol("s4", "copied", "local", "i32", false, 141, 148)
	reference := func(typ, id string, start, end int) map[string]any {
		return map[string]any{"kind": "reference", "type": typ, "symbol": id, "range": span{start, end}}
	}
	facts := map[string]any{"status": "ok", "inputs": []string{"/src/main.ts", "/src/minimal-integers-v3.d.ts"}, "symbols": []any{fn, parameter, caught, copied}, "functions": []any{map[string]any{
		"name": "entry", "symbol": "s1", "parameters": []any{parameter}, "returnType": "i32", "parameterInitializers": []any{nil}, "range": span{0, 170},
		"body": []any{map[string]any{"kind": "try", "range": span{47, 167}, "exception": map[string]any{
			"tryBlock":     map[string]any{"range": span{54, 79}, "body": []any{map[string]any{"kind": "throw", "range": span{56, 74}, "expression": reference("i32", "s2", 67, 73)}}},
			"catchClause":  map[string]any{"binding": caught, "declarationRange": span{87, 93}, "range": span{79, 120}, "block": map[string]any{"range": span{94, 120}, "body": []any{map[string]any{"kind": "throw", "range": span{96, 115}, "expression": reference("unknown", "s3", 107, 114)}}}},
			"finallyBlock": map[string]any{"range": span{128, 167}, "body": []any{map[string]any{"kind": "variable", "symbol": copied, "range": span{141, 161}, "initializer": reference("i32", "s2", 155, 161)}}},
		}}},
	}}}
	exports := []any{map[string]any{"sourcePath": r.RootFile, "external": true, "bindings": []any{map[string]any{"name": "entry", "target": "s1", "sourcePath": r.RootFile, "range": span{0, 170}, "definition": span{22, 28}}}}}
	envelope, requestBytes, expected := exceptionBoundaryPacketForTest(t, r, facts, exports)
	return exceptionBoundaryFixtureForTest{r, envelope, requestBytes, expected, facts, exports}
}

// Shared by the first golden and the second real, small 64/65-parameter case.
func exceptionBoundaryPacketForTest(t *testing.T, r request, facts, exports any) (map[string]any, []byte, []byte) {
	t.Helper()
	inputBytes, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var input map[string]any
	if err := json.Unmarshal(inputBytes, &input); err != nil {
		t.Fatal(err)
	}
	envelope := map[string]any{"family": "tsfg.exception-facts-request-candidate", "version": 1, "schemaHash": exceptionBoundarySchemaForTest, "input": input}
	requestBytes := nullableFactsBoundaryCanonical(t, envelope)
	files := slices.Clone(r.Files)
	slices.SortFunc(files, func(a, b inputFile) int { return strings.Compare(a.Path, b.Path) })
	identityFiles, manifest := []any{}, []any{}
	for _, file := range files {
		identityFiles = append(identityFiles, map[string]any{"path": file.Path, "content": file.Content})
		manifest = append(manifest, map[string]any{"path": file.Path, "digest": exceptionPackageExportsDigestForTest([]byte(file.Content)), "byteLength": len(file.Content), "length": len(utf16.Encode([]rune(file.Content))), "positionUnit": "utf16"})
	}
	normalized := exceptionBoundaryCloneForTest(t, input)
	normalized["files"] = identityFiles
	payload := map[string]any{"inputIdentity": exceptionPackageExportsDigestForTest(nullableFactsBoundaryCanonical(t, normalized)), "files": manifest, "facts": facts, "exports": exports}
	requestID := exceptionPackageExportsDigestForTest(append([]byte("tsfg.exception-facts.request.v1\x00"), requestBytes...))
	packageID := exceptionPackageExportsDigestForTest(append([]byte("tsfg.exception-facts.package.v1\x00"+exceptionBoundarySchemaForTest+"\x00"), nullableFactsBoundaryCanonical(t, payload)...))
	expected := nullableFactsBoundaryCanonical(t, map[string]any{"family": "tsfg.exception-facts-package-candidate", "version": 1, "schemaHash": exceptionBoundarySchemaForTest, "requestId": requestID, "packageId": packageID, "payload": payload})
	return envelope, requestBytes, expected
}

func (f exceptionBoundaryFixtureForTest) control(t *testing.T) bool {
	t.Helper()
	return t.Run("independentFirstControl", func(t *testing.T) {
		input := exceptionPackageExportsRequestForTest(t, "boundaryFirst", f.legacy)
		exceptionPackageExportsPacketsForTest(t, "boundaryFirst", f.legacy, input, f.facts, f.exports)
	})
}

func exceptionBoundaryCloneForTest(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var copy map[string]any
	if err := json.Unmarshal(encoded, &copy); err != nil {
		t.Fatal(err)
	}
	return copy
}

func exceptionBoundaryInputForTest(value map[string]any) map[string]any {
	return value["input"].(map[string]any)
}
func exceptionBoundaryFilesForTest(value map[string]any) []any {
	return exceptionBoundaryInputForTest(value)["files"].([]any)
}

type exceptionBoundaryWriterForTest struct {
	calls, limit      int
	failure           error
	cancel            context.CancelFunc
	offered, accepted []byte
}

func (w *exceptionBoundaryWriterForTest) Write(data []byte) (int, error) {
	w.calls++
	w.offered = append(w.offered, data...)
	n := len(data)
	if w.limit >= 0 && w.limit < n {
		n = w.limit
	}
	w.accepted = append(w.accepted, data[:n]...)
	if w.cancel != nil {
		w.cancel()
	}
	return n, w.failure
}

// Malformed wire and invalid input must fail before Write. IO failures are tested
// separately: physical partial/full bytes cannot be rolled back or qualified.
func exceptionBoundaryRejectForTest(t *testing.T, data []byte, category, reason string) {
	t.Helper()
	before := bytes.Clone(data)
	output := &exceptionBoundaryWriterForTest{limit: -1}
	err := runFrontendCommand(t.Context(), []string{"--exception-facts-io-v1"}, bytes.NewReader(data), output)
	t.Logf("TSFG_EXCEPTION_BOUNDARY_REJECT_V1 reason=%s error=%v writes=%d bytes=%d", reason, err, output.calls, len(output.accepted))
	if err == nil || !strings.Contains(err.Error(), "[r03-exception-facts-"+category+"]") || output.calls != 0 || len(output.accepted) != 0 {
		t.Fatalf("want %s failure before any success bytes (%s), got err=%v writes=%d bytes=%d", category, reason, err, output.calls, len(output.accepted))
	}
	if !bytes.Equal(data, before) {
		t.Fatal("rejection mutated caller request bytes")
	}
}
