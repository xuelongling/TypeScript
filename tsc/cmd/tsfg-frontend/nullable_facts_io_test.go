// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/json"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

func TestNullableFactsCommandScalarEntry(t *testing.T) {
	const source = "// 😀\nfunction echo(value: i32 | null): i32 | null { return value; }\nexport function entry(value: i32): i32 { const result: i32 | null = echo(value); return value; }"
	const schema = "sha256:24a04b0b595cae59ccf52a939f4671cea0369c46bd705f56726d028ab1df5a71"
	r := integerRequest(source)
	if len(source) != 167 || len(utf16.Encode([]rune(source))) != 165 || len(r.Files) != 2 || r.Files[0].Content != source {
		t.Fatal("fixed original source/declaration request changed")
	}
	if !t.Run("officialDiagnostics", func(t *testing.T) { nullableBoundaryOfficialForTest(t, r, 0) }) {
		return
	}
	legacy, legacyRaw := nullableBoundaryCommandForTest(t, r)
	if legacy.Status != "ok" || len(legacy.Diagnostics) != 0 || len(legacy.Functions) != 2 || len(legacy.Symbols) != 5 {
		t.Fatalf("legacy public acquisition failed before private seam: %+v; raw=%s", legacy, legacyRaw)
	}
	// Keep this oracle independent of the private producer and of the frozen
	// acquisition test; neither test calls the other or uses private helpers.
	contents := map[string]string{}
	for _, file := range r.Files {
		contents[file.Path] = file.Content
	}
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
			FileNames: append([]string{r.RootFile}, r.Declarations.Paths...),
			CompilerOptions: &core.CompilerOptions{
				NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext,
			},
		}},
		Host:           compiler.NewCompilerHost(r.SourceRoot, vfstest.FromMap(contents, true), r.SourceRoot, nil, nil),
		SingleThreaded: core.TSTrue,
	})
	file := program.GetSourceFile(r.RootFile)
	if file == nil || len(file.Statements.Nodes) != 2 {
		t.Fatal("fresh official program lost the two original functions")
	}
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	exports := check.GetExportsOfModule(file.AsNode().Symbol())
	if len(exports) != 1 || exports[0].Name != "entry" || len(exports[0].Declarations) != 1 {
		t.Fatal("official export set is not exactly the direct scalar entry")
	}
	declaration := exports[0].Declarations[0]
	if declaration != file.Statements.Nodes[1] || declaration.Kind != ast.KindFunctionDeclaration || ast.GetSourceFileOfNode(declaration) != file || declaration.Name() == nil || check.GetSymbolAtLocation(declaration.Name()) == nil {
		t.Fatal("official export is not the actual root entry declaration/binding")
	}
	positions := file.GetPositionMap()
	declRange := span{Start: positions.UTF8ToUTF16(declaration.Pos()), End: positions.UTF8ToUTF16(declaration.End())}
	nameRange := span{Start: positions.UTF8ToUTF16(declaration.Name().Pos()), End: positions.UTF8ToUTF16(declaration.Name().End())}
	if declRange != (span{68, 165}) || nameRange != (span{84, 90}) {
		t.Fatal("official export lost original UTF16 anchors after emoji")
	}
	entry := legacy.Functions[1]
	if entry.Name != exports[0].Name || entry.Symbol == "" || entry.Range != declRange || entry.ReturnType != "i32" || entry.ReturnPresence != nil || len(entry.Parameters) != 1 || entry.Parameters[0].Type != "i32" {
		t.Fatal("legacy entry differs from the independently exported declaration")
	}
	found := false
	for _, symbol := range legacy.Symbols {
		if symbol.ID == entry.Symbol {
			found = symbol.Name == exports[0].Name && symbol.Kind == "function" && symbol.Type == "i32" && symbol.Range == nameRange && symbol.ReturnPresence == nil
		}
	}
	if !found {
		t.Fatal("official defining declaration does not bind the owned entry symbol")
	}

	// These fixed test values use official JSON canonicalization; nullable
	// identities below have their own domains and no generic DTO budget walker.
	canonical := func(value any) []byte {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		canonical := json.Value(data)
		if err := canonical.Canonicalize(); err != nil {
			t.Fatal(err)
		}
		return canonical
	}
	digest := func(data []byte) string {
		sum := sha256.Sum256(data)
		return "sha256:" + hex.EncodeToString(sum[:])
	}
	inputJSON, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	input := json.Value(bytes.Clone(inputJSON))
	if err := input.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	requestBytes := canonical(map[string]any{
		"family": "tsfg.nullable-facts-request-candidate", "version": 3, "schemaHash": schema, "input": input,
	})
	requestBefore := bytes.Clone(requestBytes)
	requestID := digest(append([]byte("tsfg.nullable-facts.request.v3\x00"), requestBytes...))
	// The first command failure is an absent private command, not evidence of
	// unsupported nullable semantics. Preserve the actual request before calling.
	t.Logf("TSFG_NULLABLE_FACTS_REQUEST_V1 %s", base64.StdEncoding.EncodeToString(requestBytes))
	t.Logf("TSFG_NULLABLE_FACTS_MAIN_V1 %s", base64.StdEncoding.EncodeToString([]byte(r.Files[0].Content)))
	t.Logf("TSFG_NULLABLE_FACTS_DTS_V1 %s", base64.StdEncoding.EncodeToString([]byte(r.Files[1].Content)))
	var output bytes.Buffer
	if err := runFrontendCommand(t.Context(), []string{"--nullable-facts-io-v3"}, bytes.NewReader(requestBytes), &output); err != nil {
		t.Fatalf("private nullable command is not available or rejected this request: %v; rawResponse=%s", err, output.Bytes())
	}
	if !bytes.Equal(requestBefore, requestBytes) {
		t.Fatal("private command mutated caller request bytes")
	}
	fields := func(data []byte, keys ...string) map[string]json.Value {
		t.Helper()
		var value map[string]json.Value
		if err := json.Unmarshal(data, &value); err != nil || len(value) != len(keys) {
			t.Fatalf("wrong exact private object keys: keys=%v raw=%s err=%v", keys, data, err)
		}
		for _, key := range keys {
			if _, ok := value[key]; !ok {
				t.Fatalf("private object omitted %s: %s", key, data)
			}
		}
		copy := json.Value(bytes.Clone(data))
		if err := copy.Canonicalize(); err != nil || !bytes.Equal(copy, data) {
			t.Fatalf("private object is not exact JCS without trailing bytes: %s", data)
		}
		return value
	}
	raw := bytes.Clone(output.Bytes())
	t.Logf("TSFG_NULLABLE_FACTS_RESPONSE_V1 %s", base64.StdEncoding.EncodeToString(raw))
	wire := fields(raw, "family", "version", "schemaHash", "requestId", "packageId", "payload")
	if string(wire["family"]) != "\"tsfg.nullable-facts-package-candidate\"" || string(wire["version"]) != "3" || !bytes.Equal(wire["schemaHash"], canonical(schema)) || !bytes.Equal(wire["requestId"], canonical(requestID)) {
		t.Fatalf("private family/version/schema/request identity differs: %s", raw)
	}
	payload := fields(wire["payload"], "inputIdentity", "files", "facts", "exports")
	preimage := append([]byte("tsfg.nullable-facts.package.v3\x00"+schema+"\x00"), wire["payload"]...)
	if !bytes.Equal(wire["packageId"], canonical(digest(preimage))) {
		t.Fatal("package ID does not bind schema and complete canonical payload")
	}
	var inputFields map[string]json.Value
	if err := json.Unmarshal(input, &inputFields); err != nil {
		t.Fatal(err)
	}
	var sourceFiles []struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(inputFields["files"], &sourceFiles); err != nil {
		t.Fatal(err)
	}
	slices.SortFunc(sourceFiles, func(a, b struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}) int {
		return strings.Compare(a.Path, b.Path)
	})
	identityFiles := []map[string]any{}
	fileTable := []map[string]any{}
	for _, original := range sourceFiles {
		identityFiles = append(identityFiles, map[string]any{"path": original.Path, "content": original.Content})
		fileTable = append(fileTable, map[string]any{
			"path": original.Path, "digest": digest([]byte(original.Content)), "byteLength": len(original.Content),
			"length": len(utf16.Encode([]rune(original.Content))), "positionUnit": "utf16",
		})
	}
	inputFields["files"] = json.Value(canonical(identityFiles))
	if !bytes.Equal(payload["inputIdentity"], canonical(digest(canonical(inputFields)))) || !bytes.Equal(payload["files"], canonical(fileTable)) {
		t.Fatal("input/file identities differ from complete sorted original request files")
	}
	legacyCanonical := json.Value(bytes.Clone(legacyRaw))
	if err := legacyCanonical.Canonicalize(); err != nil || !bytes.Equal(payload["facts"], legacyCanonical) {
		t.Fatalf("private package changed or filtered existing owned response facts: payload=%s legacy=%s", payload["facts"], legacyRaw)
	}
	wantExports := []map[string]any{{
		"name": exports[0].Name, "target": entry.Symbol, "sourcePath": r.RootFile, "range": declRange, "definition": nameRange,
	}}
	if !bytes.Equal(payload["exports"], canonical(wantExports)) {
		t.Fatalf("complete private export table differs from fresh official declaration/symbol oracle: %s", payload["exports"])
	}
	// The capture owner must supply actual producer OID/dirty/input-set pins.
	// Log only hashes known here; this is not a fabricated production origin.
	captureHashes := canonical(map[string]any{
		"kind": "test-only-capture-hashes", "requestSha256": digest(requestBytes), "responseSha256": digest(raw),
	})
	t.Logf("TSFG_NULLABLE_FACTS_CAPTURE_HASHES_V1 %s", base64.StdEncoding.EncodeToString(captureHashes))
}
