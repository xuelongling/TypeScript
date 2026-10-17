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

// Two real projection entrances: statement assignment and a value-producing
// assignment expression, each through integer and ordinary scalar profiles.
// Official unknown reads are never guessed to narrow. Parameter presence and
// three null default slots are the same existing contract in both profiles.
func TestFrontendExceptionUnknownAssignmentAliasOwnedFacts(t *testing.T) {
	const prefix = "// 😀\r\nexport function entry(value: i32, flag: bool, stop: bool): i32 {\r\n  try { throw value; } catch (caught) {\r\n    const before = caught;\r\n    if (stop) { throw before; }\r\n    "
	const suffix = "\r\n    throw after;\r\n  }\r\n}"
	for _, tc := range []struct {
		name, tail, digest string
		bytes, units       int
		nested             bool
	}{
		{"statement", "caught = flag;\r\n    const after = caught;", "5c60e72e5620b30b49858af09897cd4e7dceb1b1189ca563333943fa5beeb9bc", 248, 246, false},
		{"nested", "const after = (caught = flag);", "100829408729f16ccb7e42a1c3d21857ecd008ee2a8befef39e7c115a8247261", 237, 235, true},
	} {
		for _, profile := range []string{"integer", "ordinary"} {
			t.Run(tc.name+"/"+profile, func(t *testing.T) {
				source := prefix + tc.tail + suffix
				t.Logf("TSFG_EXCEPTION_ASSIGNMENT_SOURCE_V1 %s", base64.StdEncoding.EncodeToString([]byte(source)))
				r := integerRequest(source)
				declarationPath := "/src/minimal-integers-v3.d.ts"
				if profile == "ordinary" {
					r = scalarRequest(source)
					declarationPath = "/src/minimal-scalars-v2.d.ts"
				}
				requestBytes, err := json.Marshal(r)
				t.Logf("TSFG_EXCEPTION_ASSIGNMENT_REQUEST_V1 %s", base64.StdEncoding.EncodeToString(requestBytes))
				if err != nil {
					t.Fatal(err)
				}
				sum := sha256.Sum256([]byte(source))
				if len(source) != tc.bytes || len(utf16.Encode([]rune(source))) != tc.units || hex.EncodeToString(sum[:]) != tc.digest {
					t.Fatal("independent assignment source preimage changed")
				}
				defer func() {
					after, marshalErr := json.Marshal(r)
					if marshalErr != nil || !bytes.Equal(after, requestBytes) {
						t.Fatal("original request was mutated")
					}
				}()
				program := throwBoundaryProgramForTest(t, r)
				if !t.Run("officialDiagnostics", func(t *testing.T) { throwBoundaryDiagnosticsForTest(t, program) }) {
					return
				}
				rawAnchors, tokenAnchors := map[string]span{}, map[string]span{}
				if !t.Run("officialAssignmentAliasTypesSymbolsExportsUTF16", func(t *testing.T) {
					file := program.GetSourceFile(r.RootFile)
					if file == nil || file.Text() != source || len(file.Statements.Nodes) != 1 || file.Statements.Nodes[0].Kind != ast.KindFunctionDeclaration {
						t.Fatal("one original entry function required")
					}
					fnNode := file.Statements.Nodes[0]
					fn := fnNode.AsFunctionDeclaration()
					if fn.Name().Text() != "entry" || len(fn.Parameters.Nodes) != 3 || fn.Type == nil || fn.Body == nil || fn.Body.Kind != ast.KindBlock {
						t.Fatal("explicit scalar parameter/return shape changed")
					}
					body := fn.Body.AsBlock().Statements.Nodes
					if len(body) != 1 || body[0].Kind != ast.KindTryStatement {
						t.Fatal("one original try required")
					}
					tryNode := body[0]
					attempt := tryNode.AsTryStatement()
					if attempt.TryBlock == nil || attempt.CatchClause == nil || attempt.FinallyBlock != nil {
						t.Fatal("original try/catch presence changed")
					}
					clause := attempt.CatchClause.AsCatchClause()
					if clause.VariableDeclaration == nil || clause.Block == nil {
						t.Fatal("ordinary catch declaration missing")
					}
					caught := clause.VariableDeclaration.AsVariableDeclaration()
					if caught.Name() == nil || caught.Name().Text() != "caught" || caught.Type != nil || caught.Initializer != nil {
						t.Fatal("unannotated unknown catch required")
					}
					tryBody := attempt.TryBlock.AsBlock().Statements.Nodes
					catchBody := clause.Block.AsBlock().Statements.Nodes
					expectedBody, afterIndex := 5, 3
					if tc.nested {
						expectedBody, afterIndex = 4, 2
					}
					if len(tryBody) != 1 || tryBody[0].Kind != ast.KindThrowStatement || len(catchBody) != expectedBody || catchBody[0].Kind != ast.KindVariableStatement || catchBody[1].Kind != ast.KindIfStatement || catchBody[afterIndex].Kind != ast.KindVariableStatement || catchBody[expectedBody-1].Kind != ast.KindThrowStatement {
						t.Fatal("original assignment/alias body order changed")
					}
					local := func(node *ast.Node, name string) *ast.Node {
						t.Helper()
						list := node.AsVariableStatement().DeclarationList
						declarations := list.AsVariableDeclarationList().Declarations.Nodes
						if list.Flags&ast.NodeFlagsConst == 0 || len(declarations) != 1 || declarations[0].Name() == nil || declarations[0].Name().Text() != name || declarations[0].AsVariableDeclaration().Type != nil || declarations[0].Initializer() == nil {
							t.Fatalf("one ordinary inferred const %s required", name)
						}
						return declarations[0]
					}
					beforeNode := local(catchBody[0], "before")
					afterNode := local(catchBody[afterIndex], "after")
					beforeInitializer, afterInitializer := beforeNode.Initializer(), afterNode.Initializer()
					branch := catchBody[1].AsIfStatement()
					if branch.Expression == nil || branch.Expression.Kind != ast.KindIdentifier || branch.Expression.Text() != "stop" || branch.ThenStatement == nil || branch.ThenStatement.Kind != ast.KindBlock || branch.ElseStatement != nil {
						t.Fatal("before path must use independent stop parameter")
					}
					thenBody := branch.ThenStatement.AsBlock().Statements.Nodes
					if len(thenBody) != 1 || thenBody[0].Kind != ast.KindThrowStatement {
						t.Fatal("one before-alias throw required")
					}
					beforeUse := thenBody[0].AsThrowStatement().Expression
					afterUse := catchBody[expectedBody-1].AsThrowStatement().Expression
					tryValue := tryBody[0].AsThrowStatement().Expression
					var assignmentNode *ast.Node
					if tc.nested {
						if afterInitializer.Kind != ast.KindParenthesizedExpression {
							t.Fatal("nested assignment group missing")
						}
						assignmentNode = afterInitializer.AsParenthesizedExpression().Expression
					} else {
						if catchBody[2].Kind != ast.KindExpressionStatement {
							t.Fatal("ordinary assignment statement missing")
						}
						assignmentNode = catchBody[2].AsExpressionStatement().Expression
					}
					if assignmentNode == nil || assignmentNode.Kind != ast.KindBinaryExpression {
						t.Fatal("one actual binary assignment required")
					}
					assignment := assignmentNode.AsBinaryExpression()
					if assignment.OperatorToken.Kind != ast.KindEqualsToken || assignment.Left.Kind != ast.KindIdentifier || assignment.Left.Text() != "caught" || assignment.Right.Kind != ast.KindIdentifier || assignment.Right.Text() != "flag" {
						t.Fatal("assignment target/RHS syntax changed")
					}
					if beforeInitializer.Kind != ast.KindIdentifier || beforeInitializer.Text() != "caught" || beforeUse == nil || beforeUse.Kind != ast.KindIdentifier || beforeUse.Text() != "before" || afterUse == nil || afterUse.Kind != ast.KindIdentifier || afterUse.Text() != "after" || tryValue == nil || tryValue.Kind != ast.KindIdentifier {
						t.Fatal("ordinary alias/payload syntax changed")
					}
					if !tc.nested && (afterInitializer.Kind != ast.KindIdentifier || afterInitializer.Text() != "caught") {
						t.Fatal("statement alias must independently read caught")
					}
					check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
					defer release()
					fnSymbol := check.GetSymbolAtLocation(fn.Name())
					valueSymbol := check.GetSymbolAtLocation(fn.Parameters.Nodes[0].Name())
					flagSymbol := check.GetSymbolAtLocation(fn.Parameters.Nodes[1].Name())
					stopSymbol := check.GetSymbolAtLocation(fn.Parameters.Nodes[2].Name())
					caughtSymbol := check.GetSymbolAtLocation(caught.Name())
					beforeSymbol := check.GetSymbolAtLocation(beforeNode.Name())
					afterSymbol := check.GetSymbolAtLocation(afterNode.Name())
					seen := map[*ast.Symbol]bool{}
					for _, symbol := range []*ast.Symbol{fnSymbol, valueSymbol, flagSymbol, stopSymbol, caughtSymbol, beforeSymbol, afterSymbol} {
						if symbol == nil || seen[symbol] {
							t.Fatal("seven independent original symbols required")
						}
						seen[symbol] = true
					}
					if len(caughtSymbol.Declarations) != 1 || caughtSymbol.Declarations[0] != clause.VariableDeclaration || len(beforeSymbol.Declarations) != 1 || beforeSymbol.Declarations[0] != beforeNode || len(afterSymbol.Declarations) != 1 || afterSymbol.Declarations[0] != afterNode {
						t.Fatal("catch/before/after declaration identity changed")
					}
					if check.GetSymbolAtLocation(beforeInitializer) != caughtSymbol || check.GetSymbolAtLocation(beforeUse) != beforeSymbol || check.GetSymbolAtLocation(assignment.Left) != caughtSymbol || check.GetSymbolAtLocation(assignment.Right) != flagSymbol || check.GetSymbolAtLocation(branch.Expression) != stopSymbol || check.GetSymbolAtLocation(afterUse) != afterSymbol || check.GetSymbolAtLocation(tryValue) != valueSymbol {
						t.Fatal("assignment/alias/throw rebound an official symbol")
					}
					if !tc.nested && check.GetSymbolAtLocation(afterInitializer) != caughtSymbol {
						t.Fatal("independent post-write read must bind caught")
					}
					unknown, boolean := check.GetTypeOfSymbol(caughtSymbol), check.GetBooleanType()
					i32 := check.GetTypeFromTypeNode(fn.Parameters.Nodes[0].AsParameterDeclaration().Type)
					if unknown == nil || unknown.Flags() != checker.TypeFlagsUnknown || boolean == nil || unknown == boolean || i32 == nil || i32 == unknown || i32 == boolean || check.GetTypeFromTypeNode(fn.Type) != i32 || check.GetTypeAtLocation(tryValue) != i32 {
						t.Fatal("nominal i32/official unknown/bool identities changed")
					}
					for _, parameter := range fn.Parameters.Nodes[1:] {
						if check.GetTypeFromTypeNode(parameter.AsParameterDeclaration().Type) != boolean || check.GetTypeAtLocation(parameter.Name()) != boolean {
							t.Fatal("flag/stop declarations must remain the same official bool")
						}
					}
					if check.GetTypeAtLocation(caught.Name()) != unknown || check.GetTypeOfSymbol(beforeSymbol) != unknown || check.GetTypeAtLocation(beforeInitializer) != unknown || check.GetTypeAtLocation(beforeUse) != unknown || check.GetTypeAtLocation(assignment.Left) != unknown {
						t.Fatal("catch declaration/definite target and before alias must remain actual official unknown")
					}
					if check.GetTypeAtLocation(branch.Expression) != boolean || check.GetTypeAtLocation(assignment.Right) != boolean || check.GetTypeAtLocation(assignmentNode) != boolean {
						t.Fatal("independent stop must not narrow flag; assignment result must retain official RHS bool")
					}
					afterType := unknown
					if tc.nested {
						afterType = boolean
					}
					if check.GetTypeOfSymbol(afterSymbol) != afterType || check.GetTypeAtLocation(afterNode.Name()) != afterType || check.GetTypeAtLocation(afterInitializer) != afterType || check.GetTypeAtLocation(afterUse) != afterType {
						t.Fatal("official statement read/assignment-result alias distinction changed")
					}
					exports := check.GetExportsOfModule(file.AsNode().Symbol())
					if len(exports) != 1 || exports[0] != fnSymbol || exports[0].Name != "entry" || len(exports[0].Declarations) != 1 || exports[0].Declarations[0] != fnNode || ast.GetSourceFileOfNode(exports[0].Declarations[0]) != file {
						t.Fatal("complete official exports changed")
					}
					remember := func(label string, node *ast.Node, text string, start int) {
						t.Helper()
						if start < 0 || !strings.HasPrefix(source[start:], text) {
							t.Fatalf("original token preimage missing for %s", label)
						}
						raw := throwBoundaryNodeForTest(t, file, node)
						token := throwBoundarySpanForTest(source, start, start+len(text))
						if raw.Start > token.Start || raw.End != token.End || node.Pos() > start || node.End() != start+len(text) {
							t.Fatalf("official raw/token mismatch for %s", label)
						}
						rawAnchors[label], tokenAnchors[label] = raw, token
					}
					functionStart := strings.Index(source, "export function")
					remember("function", fnNode, source[functionStart:], functionStart)
					remember("definition", fn.Name(), "entry", strings.Index(source, "entry("))
					for i, text := range []string{"value: i32", "flag: bool", "stop: bool"} {
						parameter := fn.Parameters.Nodes[i]
						start := strings.Index(source, text)
						label := parameter.Name().Text()
						name := parameter.Name().Text()
						remember(label+"Parameter", parameter, text, start)
						remember(label+"Name", parameter.Name(), name, start)
						annotation := "bool"
						if i == 0 {
							annotation = "i32"
						}
						remember(label+"Annotation", parameter.AsParameterDeclaration().Type, annotation, start+len(name)+len(": "))
					}
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
					catchBlockStart := strings.Index(source, "{\r\n    const before")
					remember("catchBlock", clause.Block, source[catchBlockStart:tryEnd], catchBlockStart)
					beforeStart := strings.Index(source, "before = caught")
					remember("beforeDeclaration", beforeNode, "before = caught", beforeStart)
					remember("beforeName", beforeNode.Name(), "before", beforeStart)
					remember("beforeInitializer", beforeInitializer, "caught", beforeStart+len("before = "))
					ifText := "if (stop) { throw before; }"
					remember("if", catchBody[1], ifText, strings.Index(source, ifText))
					remember("condition", branch.Expression, "stop", strings.Index(source, "(stop)")+1)
					remember("thenBlock", branch.ThenStatement, "{ throw before; }", strings.Index(source, "{ throw before; }"))
					beforeThrowStart := strings.Index(source, "throw before;")
					remember("beforeThrow", thenBody[0], "throw before;", beforeThrowStart)
					remember("beforePayload", beforeUse, "before", beforeThrowStart+len("throw "))
					assignmentStart := strings.Index(source, "caught = flag")
					remember("assignment", assignmentNode, "caught = flag", assignmentStart)
					remember("assignmentTarget", assignment.Left, "caught", assignmentStart)
					remember("assignmentValue", assignment.Right, "flag", assignmentStart+len("caught = "))
					afterStart := strings.Index(source, "after = ")
					afterText := "after = caught"
					initializerText := "caught"
					if tc.nested {
						afterText, initializerText = "after = (caught = flag)", "(caught = flag)"
					}
					remember("afterDeclaration", afterNode, afterText, afterStart)
					remember("afterName", afterNode.Name(), "after", afterStart)
					remember("afterInitializer", afterInitializer, initializerText, afterStart+len("after = "))
					if !tc.nested {
						remember("assignmentStatement", catchBody[2], "caught = flag;", assignmentStart)
					}
					afterThrowStart := strings.Index(source, "throw after;")
					remember("afterThrow", catchBody[expectedBody-1], "throw after;", afterThrowStart)
					remember("afterPayload", afterUse, "after", afterThrowStart+len("throw "))
					if rawAnchors["function"] != (span{0, tc.units}) || tokenAnchors["function"].Start != 7 {
						t.Fatal("CRLF/non-BMP original source anchors changed")
					}
					oracle, marshalErr := json.Marshal(map[string]any{"kind": "test-only-official-unknown-assignment-alias-oracle", "case": tc.name, "profile": profile, "raw": rawAnchors, "token": tokenAnchors, "caughtDeclaration": "unknown", "assignmentTarget": "unknown", "assignmentRHS": "same-official-bool", "assignmentResult": "same-official-bool", "before": "unknown-independent-binding", "after": map[bool]string{false: "unknown-read", true: "bool-assignment-result"}[tc.nested], "symbols": []string{"entry", "value", "flag", "stop", "caught", "before", "after"}, "exports": []map[string]any{{"name": "entry", "range": rawAnchors["function"], "definition": rawAnchors["definition"]}}})
					t.Logf("TSFG_EXCEPTION_ASSIGNMENT_OFFICIAL_ORACLE_V1 %s", base64.StdEncoding.EncodeToString(oracle))
					if marshalErr != nil {
						t.Fatal(marshalErr)
					}
				}) {
					return
				}
				t.Run("ownedProjection", func(t *testing.T) {
					symbol := func(id, name, kind, typ string, writable bool, where span) map[string]any {
						return map[string]any{"id": id, "name": name, "kind": kind, "type": typ, "writable": writable, "range": where}
					}
					fn := symbol("s1", "entry", "function", "i32", false, rawAnchors["definition"])
					params := []any{}
					symbols := []any{fn}
					for i, name := range []string{"value", "flag", "stop"} {
						typ := "bool"
						if i == 0 {
							typ = "i32"
						}
						id := []string{"s2", "s3", "s4"}[i]
						parameter := symbol(id, name, "parameter", typ, true, rawAnchors[name+"Name"])
						parameter["presence"] = map[string]any{"undefined": false, "null": false, "optional": false, "default": false, "range": rawAnchors[name+"Parameter"], "annotation": tokenAnchors[name+"Annotation"]}
						params, symbols = append(params, parameter), append(symbols, parameter)
					}
					caught := symbol("s5", "caught", "local", "unknown", true, rawAnchors["catchName"])
					before := symbol("s6", "before", "local", "unknown", false, rawAnchors["beforeName"])
					afterType := "unknown"
					if tc.nested {
						afterType = "bool"
					}
					after := symbol("s7", "after", "local", afterType, false, rawAnchors["afterName"])
					symbols = append(symbols, caught, before, after)
					reference := func(typ, id, label string, token bool) map[string]any {
						where := rawAnchors[label]
						if token {
							where = tokenAnchors[label]
						}
						return map[string]any{"kind": "reference", "type": typ, "symbol": id, "range": where}
					}
					catchBody := []any{
						map[string]any{"kind": "variable", "symbol": before, "initializer": reference("unknown", "s5", "beforeInitializer", false), "range": rawAnchors["beforeDeclaration"]},
						map[string]any{"kind": "if", "condition": reference("bool", "s4", "condition", false), "range": tokenAnchors["if"], "thenBody": []any{map[string]any{"kind": "block", "range": rawAnchors["thenBlock"], "body": []any{map[string]any{"kind": "throw", "expression": reference("unknown", "s6", "beforePayload", false), "range": rawAnchors["beforeThrow"]}}}}},
					}
					var initializer any = reference("unknown", "s5", "afterInitializer", false)
					if tc.nested {
						initializer = map[string]any{"kind": "group", "type": "bool", "range": tokenAnchors["afterInitializer"], "operand": map[string]any{"kind": "write", "type": "bool", "operator": "=", "range": tokenAnchors["assignment"], "left": reference("unknown", "s5", "assignmentTarget", true), "right": reference("bool", "s3", "assignmentValue", true)}}
					} else {
						catchBody = append(catchBody, map[string]any{"kind": "assignment", "target": reference("unknown", "s5", "assignmentTarget", false), "value": reference("bool", "s3", "assignmentValue", false), "range": rawAnchors["assignmentStatement"]})
					}
					catchBody = append(catchBody, map[string]any{"kind": "variable", "symbol": after, "initializer": initializer, "range": rawAnchors["afterDeclaration"]}, map[string]any{"kind": "throw", "expression": reference(afterType, "s7", "afterPayload", false), "range": rawAnchors["afterThrow"]})
					expected := map[string]any{"status": "ok", "inputs": []string{"/src/main.ts", declarationPath}, "symbols": symbols, "functions": []any{map[string]any{"name": "entry", "symbol": "s1", "parameters": params, "returnType": "i32", "parameterInitializers": []any{nil, nil, nil}, "range": rawAnchors["function"], "body": []any{map[string]any{"kind": "try", "range": rawAnchors["try"], "exception": map[string]any{
						"tryBlock":    map[string]any{"range": rawAnchors["tryBlock"], "body": []any{map[string]any{"kind": "throw", "expression": reference("i32", "s2", "tryPayload", false), "range": rawAnchors["tryThrow"]}}},
						"catchClause": map[string]any{"binding": caught, "declarationRange": rawAnchors["catchDeclaration"], "range": rawAnchors["catchClause"], "block": map[string]any{"range": rawAnchors["catchBlock"], "body": catchBody}},
					}}}}}}
					expectedBytes, marshalErr := json.Marshal(expected)
					t.Logf("TSFG_EXCEPTION_ASSIGNMENT_EXPECTED_V1 %s", base64.StdEncoding.EncodeToString(expectedBytes))
					if marshalErr != nil {
						t.Fatal(marshalErr)
					}
					var output bytes.Buffer
					commandErr := runFrontendCommand(t.Context(), nil, bytes.NewReader(requestBytes), &output)
					raw := output.Bytes()
					t.Logf("TSFG_EXCEPTION_ASSIGNMENT_RESPONSE_V1 %s", base64.StdEncoding.EncodeToString(raw))
					if commandErr != nil {
						t.Fatalf("public command failed after official assignment oracle: %v; raw=%s", commandErr, raw)
					}
					var gotJSON, wantJSON any
					if decodeErr := json.Unmarshal(raw, &gotJSON); decodeErr != nil {
						t.Fatalf("incomplete public response: %v; raw=%s", decodeErr, raw)
					}
					if decodeErr := json.Unmarshal(expectedBytes, &wantJSON); decodeErr != nil {
						t.Fatal(decodeErr)
					}
					if !reflect.DeepEqual(gotJSON, wantJSON) {
						t.Fatalf("official assignment/alias oracle passed, but complete owned facts differ: got=%s want=%s", raw, expectedBytes)
					}
				})
			})
		}
	}
}
