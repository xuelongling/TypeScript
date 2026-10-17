// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

// This first vertical slice owns its original source and complete JSON preimage.
// Existing test-only Program/UTF16 helpers do not invoke owned projection. No new
// exception DTO type or producer normalization is used to manufacture expectations.
func TestFrontendExceptionStructureOwnedFacts(t *testing.T) {
	const source = "// 😀\r\nexport function entry(value: i32): i32 {\r\n  try {\r\n    throw value;\r\n  } catch (caught) {\r\n    throw caught;\r\n  } finally {\r\n    const copied: i32 = value;\r\n  }\r\n}"
	t.Logf("TSFG_EXCEPTION_STRUCTURE_SOURCE_V1 %s", base64.StdEncoding.EncodeToString([]byte(source)))
	r := integerRequest(source)
	requestBytes, err := json.Marshal(r)
	t.Logf("TSFG_EXCEPTION_STRUCTURE_REQUEST_V1 %s", base64.StdEncoding.EncodeToString(requestBytes))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(source))
	if len(source) != 172 || len(utf16.Encode([]rune(source))) != 170 || hex.EncodeToString(sum[:]) != "4c5cfdea7fa418d1b0eaeda99b01cb1e749266ecd8ecb9c0154df4cb7219c8c2" {
		t.Fatal("independent original CRLF/non-BMP source changed")
	}
	defer func() {
		after, marshalErr := json.Marshal(r)
		if marshalErr != nil || !bytes.Equal(requestBytes, after) {
			t.Fatal("official oracle or public projection mutated the original request")
		}
	}()
	program := throwBoundaryProgramForTest(t, r)
	if !t.Run("officialDiagnostics", func(t *testing.T) {
		throwBoundaryDiagnosticsForTest(t, program)
	}) {
		return
	}
	anchors := map[string]span{}
	tokens := map[string]span{}
	if !t.Run("officialStructureSymbolsTypesExportsUTF16", func(t *testing.T) {
		file := program.GetSourceFile(r.RootFile)
		if file == nil || file.Text() != source || len(file.Statements.Nodes) != 1 {
			t.Fatal("official original root/function table changed")
		}
		fnNode := file.Statements.Nodes[0]
		if fnNode.Kind != ast.KindFunctionDeclaration {
			t.Fatal("one official function required")
		}
		fn := fnNode.AsFunctionDeclaration()
		if fn.Name().Text() != "entry" || fn.Type == nil || len(fn.Parameters.Nodes) != 1 || fn.Body == nil || fn.Body.Kind != ast.KindBlock {
			t.Fatal("official scalar entry declaration changed")
		}
		statements := fn.Body.AsBlock().Statements.Nodes
		if len(statements) != 1 || statements[0].Kind != ast.KindTryStatement {
			t.Fatal("one original try statement required")
		}
		tryNode := statements[0]
		attempt := tryNode.AsTryStatement()
		if attempt.TryBlock == nil || attempt.TryBlock.Kind != ast.KindBlock || attempt.CatchClause == nil || attempt.CatchClause.Kind != ast.KindCatchClause || attempt.FinallyBlock == nil || attempt.FinallyBlock.Kind != ast.KindBlock {
			t.Fatal("official try/catch/finally presence lost")
		}
		clause := attempt.CatchClause.AsCatchClause()
		if clause.VariableDeclaration == nil || clause.Block == nil || clause.Block.Kind != ast.KindBlock {
			t.Fatal("nonempty catch binding/block missing")
		}
		caught := clause.VariableDeclaration.AsVariableDeclaration()
		if caught.Name() == nil || caught.Name().Kind != ast.KindIdentifier || caught.Name().Text() != "caught" || caught.Type != nil || caught.Initializer != nil {
			t.Fatal("catch must remain an unannotated ordinary binding")
		}
		tryBody := attempt.TryBlock.AsBlock().Statements.Nodes
		catchBody := clause.Block.AsBlock().Statements.Nodes
		finallyBody := attempt.FinallyBlock.AsBlock().Statements.Nodes
		if len(tryBody) != 1 || len(catchBody) != 1 || len(finallyBody) != 1 || tryBody[0].Kind != ast.KindThrowStatement || catchBody[0].Kind != ast.KindThrowStatement || finallyBody[0].Kind != ast.KindVariableStatement {
			t.Fatal("all three original exception regions must be represented and nonempty")
		}
		tryValue := tryBody[0].AsThrowStatement().Expression
		catchValue := catchBody[0].AsThrowStatement().Expression
		listNode := finallyBody[0].AsVariableStatement().DeclarationList
		declarations := listNode.AsVariableDeclarationList().Declarations.Nodes
		if listNode.Flags&ast.NodeFlagsConst == 0 || len(declarations) != 1 || declarations[0].Kind != ast.KindVariableDeclaration {
			t.Fatal("finally must retain one explicit immutable const declaration")
		}
		localNode := declarations[0]
		local := localNode.AsVariableDeclaration()
		if tryValue == nil || tryValue.Kind != ast.KindIdentifier || tryValue.Text() != "value" || catchValue == nil || catchValue.Kind != ast.KindIdentifier || catchValue.Text() != "caught" || local.Name() == nil || local.Name().Text() != "copied" || local.Type == nil || local.Initializer == nil || local.Initializer.Kind != ast.KindIdentifier || local.Initializer.Text() != "value" {
			t.Fatal("single try/rethrow payload or finally initializer syntax changed")
		}
		check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
		defer release()
		parameter := fn.Parameters.Nodes[0]
		if parameter.Name() == nil || parameter.Name().Text() != "value" || parameter.AsParameterDeclaration().Type == nil {
			t.Fatal("one declared i32 parameter required")
		}
		fnSymbol := check.GetSymbolAtLocation(fn.Name())
		parameterSymbol := check.GetSymbolAtLocation(parameter.Name())
		catchSymbol := check.GetSymbolAtLocation(caught.Name())
		localSymbol := check.GetSymbolAtLocation(local.Name())
		symbols := []*ast.Symbol{fnSymbol, parameterSymbol, catchSymbol, localSymbol}
		seen := map[*ast.Symbol]bool{}
		for _, symbol := range symbols {
			if symbol == nil || seen[symbol] {
				t.Fatal("function/parameter/catch/finally-local must own four distinct official symbols")
			}
			seen[symbol] = true
		}
		if len(catchSymbol.Declarations) != 1 || catchSymbol.Declarations[0] != clause.VariableDeclaration || check.GetSymbolAtLocation(catchValue) != catchSymbol || check.GetSymbolAtLocation(tryValue) != parameterSymbol || check.GetSymbolAtLocation(local.Initializer) != parameterSymbol {
			t.Fatal("ordinary rethrow or finally reference lost actual defining symbol identity")
		}
		i32 := check.GetTypeFromTypeNode(parameter.AsParameterDeclaration().Type)
		unknown := check.GetTypeAtLocation(caught.Name())
		if i32 == nil || i32.Flags()&(checker.TypeFlagsNull|checker.TypeFlagsUndefined) != 0 || check.GetTypeAtLocation(parameter.Name()) != i32 || check.GetTypeFromTypeNode(fn.Type) != i32 || check.GetTypeAtLocation(tryValue) != i32 || check.GetTypeFromTypeNode(local.Type) != i32 || check.GetTypeAtLocation(local.Initializer) != i32 || check.GetTypeAtLocation(local.Name()) != i32 {
			t.Fatal("official scalar signature/try payload/finally local did not retain the same nominal i32")
		}
		if unknown == nil || unknown.Flags() != checker.TypeFlagsUnknown || unknown == i32 || check.GetTypeOfSymbol(catchSymbol) != unknown || check.GetTypeAtLocation(catchValue) != unknown {
			t.Fatal("catch must remain official unknown at declaration and rethrow use, never inferred from try payload")
		}
		exports := check.GetExportsOfModule(file.AsNode().Symbol())
		if len(exports) != 1 || exports[0] != fnSymbol || exports[0].Name != "entry" || len(exports[0].Declarations) != 1 || exports[0].Declarations[0] != fnNode || ast.GetSourceFileOfNode(exports[0].Declarations[0]) != file {
			t.Fatal("complete official export set must be the same actual root scalar entry")
		}
		// Raw ranges come only from official nodes and an independent UTF8-prefix to
		// UTF16 conversion. Token preimages come from explicit original spellings.
		remember := func(label string, node *ast.Node, token string, at int) {
			t.Helper()
			if at < 0 || !strings.HasPrefix(source[at:], token) {
				t.Fatalf("original token preimage missing for %s", label)
			}
			raw := throwBoundaryNodeForTest(t, file, node)
			tokenSpan := throwBoundarySpanForTest(source, at, at+len(token))
			if raw.Start > tokenSpan.Start || raw.End != tokenSpan.End || node.Pos() > at || node.End() != at+len(token) {
				t.Fatalf("official raw/token anchor mismatch for %s: raw=%+v token=%+v", label, raw, tokenSpan)
			}
			anchors[label], tokens[label] = raw, tokenSpan
		}
		functionStart := strings.Index(source, "export function")
		remember("function", fnNode, source[functionStart:], functionStart)
		remember("definition", fn.Name(), "entry", strings.Index(source, "entry("))
		paramStart := strings.Index(source, "value: i32")
		remember("parameter", parameter, "value: i32", paramStart)
		remember("parameterName", parameter.Name(), "value", paramStart)
		remember("parameterAnnotation", parameter.AsParameterDeclaration().Type, "i32", paramStart+len("value: "))
		tryStart := strings.Index(source, "try {")
		tryEnd := strings.LastIndex(source, "\r\n}")
		remember("try", tryNode, source[tryStart:tryEnd], tryStart)
		tryBlockStart := strings.Index(source, "{\r\n    throw value;")
		tryBlockText := "{\r\n    throw value;\r\n  }"
		remember("tryBlock", attempt.TryBlock, tryBlockText, tryBlockStart)
		throwStart := strings.Index(source, "throw value;")
		remember("tryThrow", tryBody[0], "throw value;", throwStart)
		remember("tryPayload", tryValue, "value", throwStart+len("throw "))
		catchStart := strings.Index(source, "catch (caught)")
		catchText := "catch (caught) {\r\n    throw caught;\r\n  }"
		remember("catchClause", attempt.CatchClause, catchText, catchStart)
		caughtStart := strings.Index(source, "(caught)") + 1
		remember("catchDeclaration", clause.VariableDeclaration, "caught", caughtStart)
		remember("catchName", caught.Name(), "caught", caughtStart)
		catchBlockStart := strings.Index(source, "{\r\n    throw caught;")
		remember("catchBlock", clause.Block, "{\r\n    throw caught;\r\n  }", catchBlockStart)
		rethrowStart := strings.Index(source, "throw caught;")
		remember("catchThrow", catchBody[0], "throw caught;", rethrowStart)
		remember("catchPayload", catchValue, "caught", rethrowStart+len("throw "))
		finallyBlockStart := strings.Index(source, "{\r\n    const copied")
		remember("finallyBlock", attempt.FinallyBlock, "{\r\n    const copied: i32 = value;\r\n  }", finallyBlockStart)
		localStart := strings.Index(source, "copied: i32 = value")
		remember("finallyDeclaration", localNode, "copied: i32 = value", localStart)
		remember("finallyName", local.Name(), "copied", localStart)
		initializerStart := strings.LastIndex(source, "value;")
		remember("finallyInitializer", local.Initializer, "value", initializerStart)
		if anchors["function"] != (span{0, 170}) || tokens["function"].Start != 7 || anchors["catchDeclaration"] != anchors["catchName"] {
			t.Fatal("original CRLF/non-BMP or unannotated catch declaration anchors changed")
		}
		oracle, marshalErr := json.Marshal(map[string]any{
			"kind": "test-only-official-exception-structure-oracle", "sourcePath": r.RootFile,
			"raw": anchors, "token": tokens, "catchDeclarationType": "official-unknown", "catchUseType": "same-official-unknown",
			"symbols": []string{"entry", "value", "caught", "copied"}, "exports": []map[string]any{{"name": "entry", "range": anchors["function"], "definition": anchors["definition"]}},
		})
		t.Logf("TSFG_EXCEPTION_STRUCTURE_OFFICIAL_ORACLE_V1 %s", base64.StdEncoding.EncodeToString(oracle))
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
	}) {
		return
	}
	t.Run("ownedProjection", func(t *testing.T) {
		var output bytes.Buffer
		commandErr := runFrontendCommand(t.Context(), nil, bytes.NewReader(requestBytes), &output)
		raw := output.Bytes()
		t.Logf("TSFG_EXCEPTION_STRUCTURE_RESPONSE_V1 %s", base64.StdEncoding.EncodeToString(raw))
		if commandErr != nil {
			t.Fatalf("public command failed after official oracle: %v; raw=%s", commandErr, raw)
		}
		if !bytes.HasSuffix(raw, []byte("\n")) {
			t.Fatal("legacy public response lost its existing final newline")
		}
		symbol := func(id, name, kind, typ string, writable bool, where span) map[string]any {
			return map[string]any{"id": id, "name": name, "kind": kind, "type": typ, "writable": writable, "range": where}
		}
		functionSymbol := symbol("s1", "entry", "function", "i32", false, anchors["definition"])
		parameter := symbol("s2", "value", "parameter", "i32", true, anchors["parameterName"])
		parameter["presence"] = map[string]any{"undefined": false, "null": false, "optional": false, "default": false,
			"range": anchors["parameter"], "annotation": tokens["parameterAnnotation"]}
		caught := symbol("s3", "caught", "local", "unknown", true, anchors["catchName"])
		copied := symbol("s4", "copied", "local", "i32", false, anchors["finallyName"])
		reference := func(typ, id, label string) map[string]any {
			return map[string]any{"kind": "reference", "type": typ, "symbol": id, "range": anchors[label]}
		}
		expected := map[string]any{
			"status": "ok", "inputs": []string{"/src/main.ts", "/src/minimal-integers-v3.d.ts"},
			"symbols": []any{functionSymbol, parameter, caught, copied},
			"functions": []any{map[string]any{
				"name": "entry", "symbol": "s1", "parameters": []any{parameter}, "returnType": "i32",
				"parameterInitializers": []any{nil}, "range": anchors["function"],
				"body": []any{map[string]any{"kind": "try", "range": anchors["try"], "exception": map[string]any{
					"tryBlock": map[string]any{"range": anchors["tryBlock"], "body": []any{map[string]any{
						"kind": "throw", "range": anchors["tryThrow"], "expression": reference("i32", "s2", "tryPayload")}}},
					"catchClause": map[string]any{
						"binding": caught, "declarationRange": anchors["catchDeclaration"], "range": anchors["catchClause"],
						"block": map[string]any{"range": anchors["catchBlock"], "body": []any{map[string]any{
							"kind": "throw", "range": anchors["catchThrow"], "expression": reference("unknown", "s3", "catchPayload")}}},
					},
					"finallyBlock": map[string]any{"range": anchors["finallyBlock"], "body": []any{map[string]any{
						"kind": "variable", "range": anchors["finallyDeclaration"], "symbol": copied, "initializer": reference("i32", "s2", "finallyInitializer")}}},
				}}},
			}},
		}
		expectedBytes, marshalErr := json.Marshal(expected)
		t.Logf("TSFG_EXCEPTION_STRUCTURE_EXPECTED_V1 %s", base64.StdEncoding.EncodeToString(expectedBytes))
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		var gotJSON, wantJSON any
		if decodeErr := json.Unmarshal(raw, &gotJSON); decodeErr != nil {
			t.Fatalf("public response is not complete JSON: %v; raw=%s", decodeErr, raw)
		}
		if decodeErr := json.Unmarshal(expectedBytes, &wantJSON); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if !reflect.DeepEqual(gotJSON, wantJSON) {
			t.Fatalf("official exception oracle passed, but owned complete structure differs: got=%s want=%s", raw, expectedBytes)
		}
	})
}
