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

// Independent next vertical slice: boolean typeof narrows an official unknown
// catch occurrence. The binding declaration stays unknown; no payload inference,
// assignment/alias, other guard, execution consumer or producer DTO is assumed.
func TestFrontendExceptionBooleanGuardOwnedFacts(t *testing.T) {
	const source = "// 😀\r\nexport function entry(value: i32): i32 {\r\n  try { throw value; } catch (caught) {\r\n    if (typeof caught === \"boolean\") { throw caught; }\r\n    throw caught;\r\n  }\r\n}"
	t.Logf("TSFG_EXCEPTION_BOOLEAN_GUARD_SOURCE_V1 %s", base64.StdEncoding.EncodeToString([]byte(source)))
	r := integerRequest(source)
	requestBytes, err := json.Marshal(r)
	t.Logf("TSFG_EXCEPTION_BOOLEAN_GUARD_REQUEST_V1 %s", base64.StdEncoding.EncodeToString(requestBytes))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(source))
	if len(source) != 173 || len(utf16.Encode([]rune(source))) != 171 || hex.EncodeToString(sum[:]) != "cff0ae6fe9c37b1bcc94dc17dc098b0da47f7227613de203229049c0dfbf0b86" {
		t.Fatal("independent boolean guard source changed")
	}
	defer func() {
		after, marshalErr := json.Marshal(r)
		if marshalErr != nil || !bytes.Equal(requestBytes, after) {
			t.Fatal("official oracle or public command mutated original input")
		}
	}()
	program := throwBoundaryProgramForTest(t, r)
	if !t.Run("officialDiagnostics", func(t *testing.T) {
		throwBoundaryDiagnosticsForTest(t, program)
	}) {
		return
	}
	rawAnchors, tokenAnchors := map[string]span{}, map[string]span{}
	if !t.Run("officialBooleanFlowSymbolsExportsUTF16", func(t *testing.T) {
		file := program.GetSourceFile(r.RootFile)
		if file == nil || file.Text() != source || len(file.Statements.Nodes) != 1 || file.Statements.Nodes[0].Kind != ast.KindFunctionDeclaration {
			t.Fatal("one original root function required")
		}
		fnNode := file.Statements.Nodes[0]
		fn := fnNode.AsFunctionDeclaration()
		if fn.Name().Text() != "entry" || len(fn.Parameters.Nodes) != 1 || fn.Type == nil || fn.Body == nil || fn.Body.Kind != ast.KindBlock {
			t.Fatal("scalar entry declaration changed")
		}
		body := fn.Body.AsBlock().Statements.Nodes
		if len(body) != 1 || body[0].Kind != ast.KindTryStatement {
			t.Fatal("one original try statement required")
		}
		tryNode := body[0]
		attempt := tryNode.AsTryStatement()
		if attempt.TryBlock == nil || attempt.CatchClause == nil || attempt.FinallyBlock != nil {
			t.Fatal("fixture must own try/catch and an absent finally")
		}
		clause := attempt.CatchClause.AsCatchClause()
		if clause.VariableDeclaration == nil || clause.Block == nil {
			t.Fatal("catch declaration/block missing")
		}
		caught := clause.VariableDeclaration.AsVariableDeclaration()
		if caught.Name() == nil || caught.Name().Text() != "caught" || caught.Type != nil || caught.Initializer != nil {
			t.Fatal("catch must remain an unannotated ordinary unknown binding")
		}
		tryBody := attempt.TryBlock.AsBlock().Statements.Nodes
		catchBody := clause.Block.AsBlock().Statements.Nodes
		if len(tryBody) != 1 || tryBody[0].Kind != ast.KindThrowStatement || len(catchBody) != 2 || catchBody[0].Kind != ast.KindIfStatement || catchBody[1].Kind != ast.KindThrowStatement {
			t.Fatal("original try throw, conditional throw and following rethrow structure changed")
		}
		branch := catchBody[0].AsIfStatement()
		if branch.Expression == nil || branch.Expression.Kind != ast.KindBinaryExpression || branch.ThenStatement == nil || branch.ThenStatement.Kind != ast.KindBlock || branch.ElseStatement != nil {
			t.Fatal("one ordinary if without else required")
		}
		condition := branch.Expression
		binary := condition.AsBinaryExpression()
		if binary.OperatorToken.Kind != ast.KindEqualsEqualsEqualsToken || binary.Left.Kind != ast.KindTypeOfExpression || binary.Right.Kind != ast.KindStringLiteral || binary.Right.Text() != "boolean" {
			t.Fatal("guard must preserve actual typeof caught === quoted boolean syntax")
		}
		operand := binary.Left.AsTypeOfExpression().Expression
		thenBody := branch.ThenStatement.AsBlock().Statements.Nodes
		if operand == nil || operand.Kind != ast.KindIdentifier || operand.Text() != "caught" || len(thenBody) != 1 || thenBody[0].Kind != ast.KindThrowStatement {
			t.Fatal("one catch reference and one then-throw required")
		}
		tryValue := tryBody[0].AsThrowStatement().Expression
		narrowed := thenBody[0].AsThrowStatement().Expression
		following := catchBody[1].AsThrowStatement().Expression
		if tryValue == nil || tryValue.Kind != ast.KindIdentifier || tryValue.Text() != "value" || narrowed == nil || narrowed.Kind != ast.KindIdentifier || narrowed.Text() != "caught" || following == nil || following.Kind != ast.KindIdentifier || following.Text() != "caught" {
			t.Fatal("ordinary throw payloads changed")
		}
		check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
		defer release()
		parameter := fn.Parameters.Nodes[0]
		fnSymbol := check.GetSymbolAtLocation(fn.Name())
		parameterSymbol := check.GetSymbolAtLocation(parameter.Name())
		catchSymbol := check.GetSymbolAtLocation(caught.Name())
		if fnSymbol == nil || parameterSymbol == nil || catchSymbol == nil || fnSymbol == parameterSymbol || fnSymbol == catchSymbol || parameterSymbol == catchSymbol || len(catchSymbol.Declarations) != 1 || catchSymbol.Declarations[0] != clause.VariableDeclaration {
			t.Fatal("three distinct official declarations/symbols required")
		}
		if check.GetSymbolAtLocation(tryValue) != parameterSymbol || check.GetSymbolAtLocation(operand) != catchSymbol || check.GetSymbolAtLocation(narrowed) != catchSymbol || check.GetSymbolAtLocation(following) != catchSymbol {
			t.Fatal("flow narrowing must retain original catch binding identity")
		}
		unknown := check.GetTypeOfSymbol(catchSymbol)
		boolean := check.GetBooleanType()
		i32 := check.GetTypeFromTypeNode(parameter.AsParameterDeclaration().Type)
		if unknown == nil || unknown.Flags() != checker.TypeFlagsUnknown || boolean == nil || unknown == boolean || i32 == nil || i32 == boolean || i32 == unknown || check.GetTypeFromTypeNode(fn.Type) != i32 || check.GetTypeAtLocation(tryValue) != i32 {
			t.Fatal("official unknown/bool/i32 identities or scalar signature changed")
		}
		if check.GetTypeAtLocation(caught.Name()) != unknown || check.GetTypeAtLocation(operand) != unknown || check.GetTypeAtLocation(condition) != boolean || check.GetTypeAtLocation(narrowed) != boolean || check.GetTypeAtLocation(following) != unknown {
			t.Fatal("official declaration/guard input must be unknown, then reference bool, subsequent reference unknown")
		}
		exports := check.GetExportsOfModule(file.AsNode().Symbol())
		if len(exports) != 1 || exports[0] != fnSymbol || exports[0].Name != "entry" || len(exports[0].Declarations) != 1 || exports[0].Declarations[0] != fnNode || ast.GetSourceFileOfNode(exports[0].Declarations[0]) != file {
			t.Fatal("complete official exports are not the original root entry")
		}
		// Official raw nodes and independently located token preimages remain distinct.
		remember := func(label string, node *ast.Node, text string, start int) {
			t.Helper()
			if start < 0 || !strings.HasPrefix(source[start:], text) {
				t.Fatalf("original token preimage missing for %s", label)
			}
			raw := throwBoundaryNodeForTest(t, file, node)
			token := throwBoundarySpanForTest(source, start, start+len(text))
			if raw.Start > token.Start || raw.End != token.End || node.Pos() > start || node.End() != start+len(text) {
				t.Fatalf("official raw/token anchor changed: %s raw=%+v token=%+v", label, raw, token)
			}
			rawAnchors[label], tokenAnchors[label] = raw, token
		}
		functionStart := strings.Index(source, "export function")
		remember("function", fnNode, source[functionStart:], functionStart)
		remember("definition", fn.Name(), "entry", strings.Index(source, "entry("))
		parameterStart := strings.Index(source, "value: i32")
		remember("parameter", parameter, "value: i32", parameterStart)
		remember("parameterName", parameter.Name(), "value", parameterStart)
		remember("parameterAnnotation", parameter.AsParameterDeclaration().Type, "i32", parameterStart+len("value: "))
		tryStart, tryEnd := strings.Index(source, "try {"), strings.LastIndex(source, "\r\n}")
		remember("try", tryNode, source[tryStart:tryEnd], tryStart)
		remember("tryBlock", attempt.TryBlock, "{ throw value; }", strings.Index(source, "{ throw value; }"))
		tryThrowStart := strings.Index(source, "throw value;")
		remember("tryThrow", tryBody[0], "throw value;", tryThrowStart)
		remember("tryPayload", tryValue, "value", tryThrowStart+len("throw "))
		catchStart := strings.Index(source, "catch (caught)")
		remember("catchClause", attempt.CatchClause, source[catchStart:tryEnd], catchStart)
		catchNameStart := strings.Index(source, "(caught)") + 1
		remember("catchDeclaration", clause.VariableDeclaration, "caught", catchNameStart)
		remember("catchName", caught.Name(), "caught", catchNameStart)
		catchBlockStart := strings.Index(source, "{\r\n    if")
		remember("catchBlock", clause.Block, source[catchBlockStart:tryEnd], catchBlockStart)
		ifText := "if (typeof caught === \"boolean\") { throw caught; }"
		remember("if", catchBody[0], ifText, strings.Index(source, ifText))
		conditionText := "typeof caught === \"boolean\""
		conditionStart := strings.Index(source, conditionText)
		remember("condition", condition, conditionText, conditionStart)
		remember("typeof", binary.Left, "typeof caught", conditionStart)
		remember("guardOperand", operand, "caught", conditionStart+len("typeof "))
		remember("operator", binary.OperatorToken, "===", strings.Index(source, "==="))
		remember("tag", binary.Right, "\"boolean\"", strings.Index(source, "\"boolean\""))
		remember("thenBlock", branch.ThenStatement, "{ throw caught; }", strings.Index(source, "{ throw caught; }"))
		thenThrowStart := strings.Index(source, "throw caught;")
		remember("thenThrow", thenBody[0], "throw caught;", thenThrowStart)
		remember("thenPayload", narrowed, "caught", thenThrowStart+len("throw "))
		followingStart := strings.LastIndex(source, "throw caught;")
		remember("followingThrow", catchBody[1], "throw caught;", followingStart)
		remember("followingPayload", following, "caught", followingStart+len("throw "))
		if rawAnchors["function"] != (span{0, 171}) || tokenAnchors["function"].Start != 7 || rawAnchors["tag"] == tokenAnchors["tag"] {
			t.Fatal("CRLF/non-BMP or trivia-bearing literal raw/token distinction changed")
		}
		oracle, marshalErr := json.Marshal(map[string]any{
			"kind": "test-only-official-boolean-catch-flow-oracle", "sourcePath": r.RootFile,
			"raw": rawAnchors, "token": tokenAnchors, "declarationType": "unknown", "operandType": "unknown", "conditionType": "same-official-BooleanType", "thenType": "same-official-BooleanType", "afterType": "unknown",
			"symbols": []string{"entry", "value", "caught"}, "exports": []map[string]any{{"name": "entry", "range": rawAnchors["function"], "definition": rawAnchors["definition"]}},
		})
		t.Logf("TSFG_EXCEPTION_BOOLEAN_GUARD_OFFICIAL_ORACLE_V1 %s", base64.StdEncoding.EncodeToString(oracle))
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
		t.Logf("TSFG_EXCEPTION_BOOLEAN_GUARD_RESPONSE_V1 %s", base64.StdEncoding.EncodeToString(raw))
		if commandErr != nil {
			t.Fatalf("public command failed after official boolean oracle: %v; raw=%s", commandErr, raw)
		}
		symbol := func(id, name, kind, typ string, writable bool, where span) map[string]any {
			return map[string]any{"id": id, "name": name, "kind": kind, "type": typ, "writable": writable, "range": where}
		}
		fn := symbol("s1", "entry", "function", "i32", false, rawAnchors["definition"])
		parameter := symbol("s2", "value", "parameter", "i32", true, rawAnchors["parameterName"])
		parameter["presence"] = map[string]any{"undefined": false, "null": false, "optional": false, "default": false,
			"range": rawAnchors["parameter"], "annotation": tokenAnchors["parameterAnnotation"]}
		caught := symbol("s3", "caught", "local", "unknown", true, rawAnchors["catchName"])
		reference := func(typ, id, label string) map[string]any {
			return map[string]any{"kind": "reference", "type": typ, "symbol": id, "range": rawAnchors[label]}
		}
		expected := map[string]any{
			"status": "ok", "inputs": []string{"/src/main.ts", "/src/minimal-integers-v3.d.ts"}, "symbols": []any{fn, parameter, caught},
			"functions": []any{map[string]any{
				"name": "entry", "symbol": "s1", "parameters": []any{parameter}, "returnType": "i32", "parameterInitializers": []any{nil}, "range": rawAnchors["function"],
				"body": []any{map[string]any{"kind": "try", "range": rawAnchors["try"], "exception": map[string]any{
					"tryBlock": map[string]any{"range": rawAnchors["tryBlock"], "body": []any{map[string]any{
						"kind": "throw", "range": rawAnchors["tryThrow"], "expression": reference("i32", "s2", "tryPayload")}}},
					"catchClause": map[string]any{
						"binding": caught, "declarationRange": rawAnchors["catchDeclaration"], "range": rawAnchors["catchClause"],
						"block": map[string]any{"range": rawAnchors["catchBlock"], "body": []any{
							map[string]any{"kind": "if", "range": tokenAnchors["if"], "condition": map[string]any{
								"kind": "type-test", "type": "bool", "operator": "===", "range": tokenAnchors["condition"],
								"operand":  reference("unknown", "s3", "guardOperand"),
								"typeTest": map[string]any{"tag": "boolean", "typeofRange": rawAnchors["typeof"], "tagRange": rawAnchors["tag"]},
							}, "thenBody": []any{map[string]any{"kind": "block", "range": rawAnchors["thenBlock"], "body": []any{map[string]any{
								"kind": "throw", "range": rawAnchors["thenThrow"], "expression": reference("bool", "s3", "thenPayload")}}}}},
							map[string]any{"kind": "throw", "range": rawAnchors["followingThrow"], "expression": reference("unknown", "s3", "followingPayload")},
						}},
					},
				}}},
			}},
		}
		expectedBytes, marshalErr := json.Marshal(expected)
		t.Logf("TSFG_EXCEPTION_BOOLEAN_GUARD_EXPECTED_V1 %s", base64.StdEncoding.EncodeToString(expectedBytes))
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		var gotJSON, wantJSON any
		if decodeErr := json.Unmarshal(raw, &gotJSON); decodeErr != nil {
			t.Fatalf("incomplete public JSON: %v; raw=%s", decodeErr, raw)
		}
		if decodeErr := json.Unmarshal(expectedBytes, &wantJSON); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if !reflect.DeepEqual(gotJSON, wantJSON) {
			t.Fatalf("official boolean-flow oracle passed, but complete owned facts differ: got=%s want=%s", raw, expectedBytes)
		}
	})
}
