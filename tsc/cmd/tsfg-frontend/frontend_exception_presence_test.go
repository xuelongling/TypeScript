// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
)

// The original three boundary sources now have owned exception structures.
// Expectations come from their original spellings and a fresh official checker,
// never exception DTOs, producer offsets or the actual public response.
func throwExceptionClausePresenceForTest(t *testing.T, body string) {
	t.Helper()
	source := "export function fail(value: i32): i32 { " + body + " }"
	hasCatch, hasFinally := strings.Contains(body, " catch "), strings.Contains(body, " finally ")
	r := integerRequest(source)
	requestBytes, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		after, marshalErr := json.Marshal(r)
		if marshalErr != nil || !bytes.Equal(requestBytes, after) {
			t.Fatal("official/public exception presence checks changed the original request")
		}
	}()
	program := throwBoundaryProgramForTest(t, r)
	if !t.Run("officialDiagnostics", func(t *testing.T) { throwBoundaryDiagnosticsForTest(t, program) }) {
		return
	}
	anchors, tokens := map[string]span{}, map[string]span{}
	if !t.Run("officialStructureSymbolsTypesExportsUTF16", func(t *testing.T) {
		file := program.GetSourceFile(r.RootFile)
		if file == nil || file.Text() != source || len(file.Statements.Nodes) != 1 {
			t.Fatal("official original root must contain exactly one function")
		}
		fnNode := file.Statements.Nodes[0]
		if fnNode.Kind != ast.KindFunctionDeclaration || fnNode.Name() == nil || fnNode.Name().Text() != "fail" {
			t.Fatal("official fail declaration changed")
		}
		fn := fnNode.AsFunctionDeclaration()
		if fn.Type == nil || fn.TypeParameters != nil || len(fn.Parameters.Nodes) != 1 || fn.Body == nil || fn.Body.Kind != ast.KindBlock {
			t.Fatal("official typed function shape changed")
		}
		statements := fn.Body.AsBlock().Statements.Nodes
		if len(statements) != 1 || statements[0].Kind != ast.KindTryStatement {
			t.Fatal("official function must retain exactly one original try")
		}
		tryNode := statements[0]
		attempt := tryNode.AsTryStatement()
		if attempt.TryBlock == nil || attempt.TryBlock.Kind != ast.KindBlock || (attempt.CatchClause != nil) != hasCatch || (attempt.FinallyBlock != nil) != hasFinally {
			t.Fatal("official catch/finally presence differs from the original source")
		}
		tryBody := attempt.TryBlock.AsBlock().Statements.Nodes
		if len(tryBody) != 1 || tryBody[0].Kind != ast.KindThrowStatement {
			t.Fatal("official try body must contain exactly the original throw")
		}
		payload := tryBody[0].AsThrowStatement().Expression
		parameter := fn.Parameters.Nodes[0]
		decl := parameter.AsParameterDeclaration()
		if parameter.Name() == nil || parameter.Name().Text() != "value" || decl.Type == nil || decl.QuestionToken != nil || decl.Initializer != nil || decl.DotDotDotToken != nil || payload == nil || payload.Kind != ast.KindIdentifier || payload.Text() != "value" {
			t.Fatal("official required parameter or single throw payload changed")
		}
		check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
		defer release()
		fnSymbol := check.GetSymbolAtLocation(fnNode.Name())
		parameterSymbol := check.GetSymbolAtLocation(parameter.Name())
		i32 := check.GetTypeFromTypeNode(decl.Type)
		if fnSymbol == nil || parameterSymbol == nil || fnSymbol == parameterSymbol || len(fnSymbol.Declarations) != 1 || fnSymbol.Declarations[0] != fnNode || len(parameterSymbol.Declarations) != 1 || parameterSymbol.Declarations[0] != parameter {
			t.Fatal("official function and parameter must have distinct actual declarations")
		}
		if i32 == nil || i32.Alias() == nil || i32.Alias().Symbol() == nil || i32.Alias().Symbol().Name != "i32" || check.GetTypeFromTypeNode(fn.Type) != i32 || check.GetTypeOfSymbol(parameterSymbol) != i32 || check.GetTypeAtLocation(payload) != i32 || check.GetSymbolAtLocation(payload) != parameterSymbol {
			t.Fatal("official throw must use the original nominal i32 parameter")
		}
		exports := check.GetExportsOfModule(file.AsNode().Symbol())
		if len(exports) != 1 || exports[0] != fnSymbol || exports[0].Name != "fail" || len(exports[0].Declarations) != 1 || exports[0].Declarations[0] != fnNode || ast.GetSourceFileOfNode(exports[0].Declarations[0]) != file {
			t.Fatal("complete official exports must contain exactly the original fail")
		}
		remember := func(label string, node *ast.Node, spelling string, at int) {
			t.Helper()
			if at < 0 || at > len(source) || !strings.HasPrefix(source[at:], spelling) {
				t.Fatalf("original spelling preimage missing for %s", label)
			}
			raw := throwBoundaryNodeForTest(t, file, node)
			token := throwBoundarySpanForTest(source, at, at+len(spelling))
			if node.Pos() > at || node.End() != at+len(spelling) || raw.Start > token.Start || raw.End != token.End {
				t.Fatalf("independent official raw/token UTF16 anchor differs for %s", label)
			}
			anchors[label], tokens[label] = raw, token
		}
		remember("function", fnNode, source, 0)
		remember("definition", fnNode.Name(), "fail", strings.Index(source, "fail("))
		parameterStart := strings.Index(source, "value: i32")
		remember("parameter", parameter, "value: i32", parameterStart)
		remember("parameterName", parameter.Name(), "value", parameterStart)
		remember("parameterAnnotation", decl.Type, "i32", parameterStart+len("value: "))
		remember("try", tryNode, body, strings.Index(source, body))
		remember("tryBlock", attempt.TryBlock, "{ throw value; }", strings.Index(source, "{ throw value; }"))
		throwStart := strings.Index(source, "throw value;")
		remember("tryThrow", tryBody[0], "throw value;", throwStart)
		remember("tryPayload", payload, "value", throwStart+len("throw "))
		if hasCatch {
			if attempt.CatchClause.Kind != ast.KindCatchClause {
				t.Fatal("official catch clause kind changed")
			}
			clause := attempt.CatchClause.AsCatchClause()
			if clause.VariableDeclaration != nil || clause.Block == nil || clause.Block.Kind != ast.KindBlock {
				t.Fatal("original binding-free catch must remain binding-free")
			}
			catchBody := clause.Block.AsBlock().Statements.Nodes
			if len(catchBody) != 1 || catchBody[0].Kind != ast.KindReturnStatement {
				t.Fatal("official catch must contain exactly one original return")
			}
			value := catchBody[0].AsReturnStatement().Expression
			if value == nil || value.Kind != ast.KindIdentifier || value.Text() != "value" || check.GetSymbolAtLocation(value) != parameterSymbol || check.GetTypeAtLocation(value) != i32 {
				t.Fatal("catch return lost the original nominal i32 parameter reference")
			}
			remember("catchClause", attempt.CatchClause, "catch { return value; }", strings.Index(source, "catch { return value; }"))
			remember("catchBlock", clause.Block, "{ return value; }", strings.Index(source, "{ return value; }"))
			returnStart := strings.Index(source, "return value;")
			remember("catchReturn", catchBody[0], "return value;", returnStart)
			remember("catchPayload", value, "value", returnStart+len("return "))
		}
		if hasFinally {
			if attempt.FinallyBlock.Kind != ast.KindBlock {
				t.Fatal("official finally block kind changed")
			}
			finallyBody := attempt.FinallyBlock.AsBlock().Statements.Nodes
			if len(finallyBody) != 1 || finallyBody[0].Kind != ast.KindVariableStatement {
				t.Fatal("official finally must contain exactly one original const")
			}
			list := finallyBody[0].AsVariableStatement().DeclarationList
			declarations := list.AsVariableDeclarationList().Declarations.Nodes
			if list.Flags&ast.NodeFlagsConst == 0 || len(declarations) != 1 || declarations[0].Kind != ast.KindVariableDeclaration {
				t.Fatal("official finally const declaration changed")
			}
			localNode := declarations[0]
			local := localNode.AsVariableDeclaration()
			localSymbol := check.GetSymbolAtLocation(local.Name())
			if local.Name() == nil || local.Name().Text() != "copy" || local.Type == nil || local.Initializer == nil || local.Initializer.Kind != ast.KindIdentifier || local.Initializer.Text() != "value" || localSymbol == nil || localSymbol == parameterSymbol || localSymbol == fnSymbol || len(localSymbol.Declarations) != 1 || localSymbol.Declarations[0] != localNode {
				t.Fatal("finally const must retain its distinct original immutable binding")
			}
			if check.GetTypeFromTypeNode(local.Type) != i32 || check.GetTypeOfSymbol(localSymbol) != i32 || check.GetTypeAtLocation(local.Name()) != i32 || check.GetTypeAtLocation(local.Initializer) != i32 || check.GetSymbolAtLocation(local.Initializer) != parameterSymbol {
				t.Fatal("finally const or its initializer lost nominal i32 identity")
			}
			remember("finallyBlock", attempt.FinallyBlock, "{ const copy: i32 = value; }", strings.Index(source, "{ const copy: i32 = value; }"))
			copyStart := strings.Index(source, "copy: i32 = value")
			remember("finallyDeclaration", localNode, "copy: i32 = value", copyStart)
			remember("finallyName", local.Name(), "copy", copyStart)
			remember("finallyInitializer", local.Initializer, "value", copyStart+len("copy: i32 = "))
		}
	}) {
		return
	}
	symbol := func(id, name, kind string, writable bool, where span) map[string]any {
		return map[string]any{"id": id, "name": name, "kind": kind, "type": "i32", "writable": writable, "range": where}
	}
	functionSymbol := symbol("s1", "fail", "function", false, anchors["definition"])
	parameter := symbol("s2", "value", "parameter", true, anchors["parameterName"])
	parameter["presence"] = map[string]any{"undefined": false, "null": false, "optional": false, "default": false,
		"range": anchors["parameter"], "annotation": tokens["parameterAnnotation"]}
	reference := func(label string) map[string]any {
		return map[string]any{"kind": "reference", "type": "i32", "symbol": "s2", "range": anchors[label]}
	}
	exception := map[string]any{
		"tryBlock": map[string]any{"range": anchors["tryBlock"], "body": []any{map[string]any{
			"kind": "throw", "range": anchors["tryThrow"], "expression": reference("tryPayload")}}},
	}
	if hasCatch {
		// Omitting binding/declarationRange/annotationRange asserts their absence.
		exception["catchClause"] = map[string]any{"range": anchors["catchClause"], "block": map[string]any{
			"range": anchors["catchBlock"], "body": []any{map[string]any{
				"kind": "return", "range": anchors["catchReturn"], "expression": reference("catchPayload")}}}}
	}
	symbols := []any{functionSymbol, parameter}
	if hasFinally {
		copied := symbol("s3", "copy", "local", false, anchors["finallyName"])
		symbols = append(symbols, copied)
		exception["finallyBlock"] = map[string]any{"range": anchors["finallyBlock"], "body": []any{map[string]any{
			"kind": "variable", "range": anchors["finallyDeclaration"], "symbol": copied, "initializer": reference("finallyInitializer")}}}
	}
	// Optional regions are added only when present in the original source. Full
	// JSON equality rejects absent catch/finally serialized as null or empty data.
	expected := map[string]any{
		"status": "ok", "inputs": []string{"/src/main.ts", "/src/minimal-integers-v3.d.ts"}, "symbols": symbols,
		"functions": []any{map[string]any{"name": "fail", "symbol": "s1", "parameters": []any{parameter}, "returnType": "i32",
			"parameterInitializers": []any{nil}, "range": anchors["function"],
			"body": []any{map[string]any{"kind": "try", "range": anchors["try"], "exception": exception}}}},
	}
	expectedBytes, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("TSFG_EXCEPTION_PRESENCE_EXPECTED_V1 %s", base64.StdEncoding.EncodeToString(expectedBytes))
	t.Run("ownedProjection", func(t *testing.T) {
		_, raw := throwBoundaryCommandForTest(t, r)
		if !bytes.HasSuffix(raw, []byte("\n")) {
			t.Fatal("public exception response lost its existing final newline")
		}
		var got, want any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("public exception response is not JSON: %v; raw=%s", err, raw)
		}
		if err := json.Unmarshal(expectedBytes, &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("official clause-presence oracle passed; complete public structure differs: got=%s want=%s", raw, expectedBytes)
		}
	})
}
