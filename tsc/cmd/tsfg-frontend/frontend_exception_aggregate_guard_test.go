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

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

// The real component profile uses an official standard decorator, not a JSDoc
// marker. Each operand direction owns a fresh oracle and complete expectation;
// the passing direction's public response is never an expectation factory.
func TestFrontendExceptionAggregateGuardOperandBoundaries(t *testing.T) {
	run := func(t *testing.T, name, comparisonText string, caughtLeft bool) {
		reject := func(t *testing.T, invalid bool, reason string) {
			t.Helper()
			if invalid {
				t.Fatal(reason)
			}
		}
		const header = "// 😀\r\n@tsfgComponent class Box { flag: bool = true; }\r\n"
		bodyText := "{ try { throw true; } catch (caught) { if (typeof caught === \"boolean\") { return " + comparisonText + "; } throw caught; } }"
		source := header + "export function f(box: Box): bool " + bodyText
		r := aggregateRequest(t, source)
		input, err := json.Marshal(r)
		t.Logf("TSFG_EXCEPTION_AGGREGATE_GUARD_SOURCE_V1 case=%s %s", name, base64.StdEncoding.EncodeToString([]byte(source)))
		t.Logf("TSFG_EXCEPTION_AGGREGATE_GUARD_REQUEST_V1 case=%s %s", name, base64.StdEncoding.EncodeToString(input))
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			after, err := json.Marshal(r)
			reject(t, err != nil || !bytes.Equal(input, after), "original request changed")
		}()
		program := throwBoundaryProgramForTest(t, r)
		raw, token := map[string]span{}, map[string]span{}
		var markerDigest, markerEntryID string
		var markerRange span
		if !t.Run("officialOracle", func(t *testing.T) {
			throwBoundaryDiagnosticsForTest(t, program)
			file := program.GetSourceFile(r.RootFile)
			reject(t, file == nil || file.Text() != source || len(file.Statements.Nodes) != 2, "original component/function closure changed")
			classNode, fnNode := file.Statements.Nodes[0], file.Statements.Nodes[1]
			reject(t, classNode.Kind != ast.KindClassDeclaration || classNode.Name().Text() != "Box" || len(classNode.Members()) != 1 || classNode.Modifiers() == nil || len(classNode.Modifiers().Nodes) != 1, "one marked class/field required")
			decorator := classNode.Modifiers().Nodes[0]
			reject(t, decorator.Kind != ast.KindDecorator || decorator.AsDecorator().Expression.Kind != ast.KindIdentifier || decorator.AsDecorator().Expression.Text() != "tsfgComponent", "actual standard component marker changed")
			fieldNode := classNode.Members()[0]
			reject(t, fieldNode.Kind != ast.KindPropertyDeclaration || fieldNode.Name().Text() != "flag" || fieldNode.Modifiers() != nil, "one ordinary public writable field required")
			field := fieldNode.AsPropertyDeclaration()
			reject(t, field.Type == nil || field.PostfixToken != nil || field.Initializer == nil || field.Initializer.Kind != ast.KindTrueKeyword, "field annotation/true initializer changed")
			reject(t, fnNode.Kind != ast.KindFunctionDeclaration || fnNode.Name().Text() != "f", "one exported f required")
			fn := fnNode.AsFunctionDeclaration()
			reject(t, fn.Type == nil || len(fn.Parameters.Nodes) != 1 || fn.Body == nil || fn.Body.Kind != ast.KindBlock, "original signature/body changed")
			parameter := fn.Parameters.Nodes[0]
			parameterDecl := parameter.AsParameterDeclaration()
			reject(t, parameter.Name().Text() != "box" || parameterDecl.Type == nil || parameterDecl.QuestionToken != nil || parameterDecl.Initializer != nil || parameterDecl.DotDotDotToken != nil, "required Box parameter changed")
			statements := fn.Body.AsBlock().Statements.Nodes
			reject(t, len(statements) != 1 || statements[0].Kind != ast.KindTryStatement, "single original try required")
			attempt := statements[0].AsTryStatement()
			reject(t, attempt.CatchClause == nil || attempt.FinallyBlock != nil, "catch-only presence changed")
			tryBody := attempt.TryBlock.AsBlock().Statements.Nodes
			clause := attempt.CatchClause.AsCatchClause()
			caught := clause.VariableDeclaration
			catchBody := clause.Block.AsBlock().Statements.Nodes
			reject(t, caught == nil || caught.Name().Text() != "caught" || caught.AsVariableDeclaration().Type != nil || caught.AsVariableDeclaration().Initializer != nil, "unannotated ordinary catch required")
			reject(t, len(tryBody) != 1 || tryBody[0].Kind != ast.KindThrowStatement || tryBody[0].AsThrowStatement().Expression.Kind != ast.KindTrueKeyword || len(catchBody) != 2 || catchBody[0].Kind != ast.KindIfStatement || catchBody[1].Kind != ast.KindThrowStatement, "try/guard/fallback statement order changed")
			branch := catchBody[0].AsIfStatement()
			reject(t, branch.Expression.Kind != ast.KindBinaryExpression || branch.ThenStatement.Kind != ast.KindBlock || branch.ElseStatement != nil, "original boolean guard/block changed")
			guard := branch.Expression.AsBinaryExpression()
			reject(t, guard.OperatorToken.Kind != ast.KindEqualsEqualsEqualsToken || guard.Left.Kind != ast.KindTypeOfExpression || guard.Right.Kind != ast.KindStringLiteral || guard.Right.Text() != "boolean", "original typeof boolean test changed")
			guardOperand := guard.Left.AsTypeOfExpression().Expression
			thenBody := branch.ThenStatement.AsBlock().Statements.Nodes
			reject(t, len(thenBody) != 1 || thenBody[0].Kind != ast.KindReturnStatement || thenBody[0].AsReturnStatement().Expression.Kind != ast.KindBinaryExpression, "single guarded comparison return required")
			comparison := thenBody[0].AsReturnStatement().Expression
			binary := comparison.AsBinaryExpression()
			reject(t, binary.OperatorToken.Kind != ast.KindEqualsEqualsEqualsToken, "original equality operator changed")
			narrowed, access := binary.Right, binary.Left
			if caughtLeft {
				narrowed, access = binary.Left, binary.Right
			}
			reject(t, narrowed.Kind != ast.KindIdentifier || narrowed.Text() != "caught" || access.Kind != ast.KindPropertyAccessExpression || access.Name().Text() != "flag", "original operand direction/field changed")
			receiver := access.AsPropertyAccessExpression().Expression
			fallback := catchBody[1].AsThrowStatement().Expression
			reject(t, receiver.Kind != ast.KindIdentifier || receiver.Text() != "box" || fallback.Kind != ast.KindIdentifier || fallback.Text() != "caught", "original receiver/fallback changed")
			check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
			defer release()
			classSymbol := check.GetSymbolAtLocation(classNode.Name())
			fieldSymbol := check.GetSymbolAtLocation(fieldNode.Name())
			fnSymbol := check.GetSymbolAtLocation(fnNode.Name())
			parameterSymbol := check.GetSymbolAtLocation(parameter.Name())
			catchSymbol := check.GetSymbolAtLocation(caught.Name())
			seen := map[*ast.Symbol]bool{}
			for i, symbol := range []*ast.Symbol{classSymbol, fieldSymbol, fnSymbol, parameterSymbol, catchSymbol} {
				declarations := []*ast.Node{classNode, fieldNode, fnNode, parameter, caught}
				reject(t, symbol == nil || seen[symbol] || len(symbol.Declarations) != 1 || symbol.Declarations[0] != declarations[i], "distinct actual official defining symbols changed")
				seen[symbol] = true
			}
			instance, static := check.GetDeclaredTypeOfSymbol(classSymbol), check.GetTypeOfSymbol(classSymbol)
			constructors := check.GetSignaturesOfType(static, checker.SignatureKindConstruct)
			reject(t, instance == nil || !instance.IsClass() || instance == static || len(check.GetSignaturesOfType(instance, checker.SignatureKindConstruct)) != 0 || len(constructors) != 1 || constructors[0].Declaration() != nil || len(constructors[0].Parameters()) != 0 || check.GetReturnTypeOfSignature(constructors[0]) != instance, "official nominal/static/implicit constructor identity changed")
			boolean := check.GetTypeFromTypeNode(field.Type)
			reject(t, boolean == nil || !boolean.IsBooleanLike() || check.GetTypeOfSymbol(fieldSymbol) != boolean || check.GetTypeAtLocation(access) != boolean || check.GetTypeAtLocation(narrowed) != boolean || check.GetTypeFromTypeNode(fn.Type) != boolean || !check.GetTypeAtLocation(field.Initializer).IsBooleanLike(), "official narrowed occurrence/field/return boolean identities changed")
			reject(t, check.GetTypeFromTypeNode(parameterDecl.Type) != instance || check.GetTypeOfSymbol(parameterSymbol) != instance || check.GetTypeAtLocation(receiver) != instance || check.GetSymbolAtLocation(receiver) != parameterSymbol || check.GetPropertyOfType(instance, "flag") != fieldSymbol || check.GetSymbolAtLocation(access.Name()) != fieldSymbol, "actual Box parameter/field identity changed")
			reject(t, !check.IsPropertyAccessible(access, false, false, instance, fieldSymbol) || !check.IsPropertyAccessible(access, false, true, instance, fieldSymbol), "ordinary component field read/write permissions changed")
			unknown := check.GetUnknownType()
			reject(t, check.GetTypeOfSymbol(catchSymbol) != unknown || check.GetTypeAtLocation(caught.Name()) != unknown || check.GetTypeAtLocation(guardOperand) != unknown || check.GetTypeAtLocation(fallback) != unknown || check.GetSymbolAtLocation(guardOperand) != catchSymbol || check.GetSymbolAtLocation(narrowed) != catchSymbol || check.GetSymbolAtLocation(fallback) != catchSymbol || !check.GetTypeAtLocation(branch.Expression).IsBooleanLike() || !check.GetTypeAtLocation(comparison).IsBooleanLike(), "official unknown declaration versus narrowed occurrence identity changed")
			signatures := check.GetSignaturesOfType(check.GetTypeOfSymbol(fnSymbol), checker.SignatureKindCall)
			reject(t, len(signatures) != 1 || signatures[0].Declaration() != fnNode || len(signatures[0].Parameters()) != 1 || signatures[0].Parameters()[0] != parameterSymbol || check.GetReturnTypeOfSignature(signatures[0]) != boolean, "complete official function signature changed")
			exports := check.GetExportsOfModule(file.AsNode().Symbol())
			reject(t, len(exports) != 1 || exports[0] != fnSymbol || exports[0].Name != "f" || len(exports[0].Declarations) != 1 || exports[0].Declarations[0] != fnNode || ast.GetSourceFileOfNode(fnNode) != file, "complete official exports/provenance changed")
			anchor := func(label string, node *ast.Node, text string) {
				t.Helper()
				reject(t, node == nil || node.End() < len(text), "missing official "+label+" node")
				start := node.End() - len(text)
				reject(t, node.Pos() > start || source[start:node.End()] != text, "original "+label+" source preimage changed")
				raw[label] = throwBoundaryNodeForTest(t, file, node)
				token[label] = throwBoundarySpanForTest(source, start, node.End())
				reject(t, raw[label].Start > token[label].Start || raw[label].End != token[label].End, "independent raw/token UTF16 mismatch")
			}
			anchor("class", classNode, "@tsfgComponent class Box { flag: bool = true; }")
			anchor("className", classNode.Name(), "Box")
			anchor("marker", decorator, "@tsfgComponent")
			classOpen := strings.Index(source, "{ flag: bool")
			reject(t, classOpen < 0 || classNode.Members()[0].Pos() <= classOpen, "official class body preimage changed")
			token["classBody"] = throwBoundarySpanForTest(source, classOpen, classNode.End())
			anchor("field", fieldNode, "flag: bool = true;")
			anchor("fieldName", fieldNode.Name(), "flag")
			anchor("fieldInitializer", field.Initializer, "true")
			anchor("function", fnNode, "export function f(box: Box): bool "+bodyText)
			anchor("functionName", fnNode.Name(), "f")
			anchor("functionBody", fn.Body, bodyText)
			anchor("parameter", parameter, "box: Box")
			anchor("parameterName", parameter.Name(), "box")
			anchor("parameterAnnotation", parameterDecl.Type, "Box")
			anchor("try", statements[0], strings.TrimSuffix(strings.TrimPrefix(bodyText, "{ "), " }"))
			anchor("tryBlock", attempt.TryBlock, "{ throw true; }")
			anchor("tryThrow", tryBody[0], "throw true;")
			anchor("tryValue", tryBody[0].AsThrowStatement().Expression, "true")
			catchText := "catch (caught) { if (typeof caught === \"boolean\") { return " + comparisonText + "; } throw caught; }"
			anchor("catchClause", attempt.CatchClause, catchText)
			anchor("catchDeclaration", caught, "caught")
			anchor("catchName", caught.Name(), "caught")
			anchor("catchBlock", clause.Block, strings.TrimPrefix(catchText, "catch (caught) "))
			anchor("if", catchBody[0], "if (typeof caught === \"boolean\") { return "+comparisonText+"; }")
			anchor("guard", branch.Expression, "typeof caught === \"boolean\"")
			anchor("typeof", guard.Left, "typeof caught")
			anchor("tag", guard.Right, "\"boolean\"")
			anchor("guardOperand", guardOperand, "caught")
			anchor("thenBlock", branch.ThenStatement, "{ return "+comparisonText+"; }")
			anchor("return", thenBody[0], "return "+comparisonText+";")
			anchor("comparison", comparison, comparisonText)
			anchor("narrowed", narrowed, "caught")
			anchor("fieldAccess", access, "box.flag")
			anchor("receiver", receiver, "box")
			anchor("fallbackThrow", catchBody[1], "throw caught;")
			anchor("fallback", fallback, "caught")
			reject(t, raw["class"].Start != 0 || token["class"].Start != 7, "CRLF/non-BMP prefix identity changed")
			markerFile := program.GetSourceFile("/src/minimal-aggregates-v1.d.ts")
			reject(t, markerFile == nil, "actual marker declaration source missing")
			var markerNode *ast.Node
			for _, node := range markerFile.Statements.Nodes {
				if node.Kind == ast.KindFunctionDeclaration && node.Name().Text() == "tsfgComponent" {
					markerNode = node
				}
			}
			reject(t, markerNode == nil || check.GetSymbolAtLocation(decorator.AsDecorator().Expression) != check.GetSymbolAtLocation(markerNode.Name()), "actual decorator declaration binding changed")
			markerSymbol := check.GetSymbolAtLocation(markerNode.Name())
			markerCalls := check.GetSignaturesOfType(check.GetTypeOfSymbol(markerSymbol), checker.SignatureKindCall)
			reject(t, len(markerSymbol.Declarations) != 1 || markerSymbol.Declarations[0] != markerNode || len(markerCalls) != 1 || markerCalls[0].Declaration() != markerNode || len(markerCalls[0].TypeParameters()) != 1 || len(markerCalls[0].Parameters()) != 2 || check.GetReturnTypeOfSignature(markerCalls[0]) != check.GetVoidType(), "official standard marker signature/provenance changed")
			start := markerNode.Pos()
			for start < markerNode.End() && strings.ContainsRune(" \t\r\n", rune(markerFile.Text()[start])) {
				start++
			}
			markerRange = throwBoundarySpanForTest(markerFile.Text(), start, markerNode.End())
			markerRaw := throwBoundaryNodeForTest(t, markerFile, markerNode)
			reject(t, markerRaw.Start > markerRange.Start || markerRaw.End != markerRange.End, "marker independent UTF16 identity changed")
			sum := sha256.Sum256([]byte(markerFile.Text()))
			markerDigest = "sha256:" + hex.EncodeToString(sum[:])
			entrySum := sha256.Sum256([]byte("tsfg.marker\x00minimal-aggregates-v1\x00component\x00" + markerDigest))
			markerEntryID = "sha256:" + hex.EncodeToString(entrySum[:])
			oracle, err := json.Marshal(map[string]any{"case": name, "kind": "test-only-official-aggregate-guard-oracle", "sourcePath": file.FileName(), "raw": raw, "token": token, "exports": []string{"f"}, "catchDeclaration": "official-unknown", "guardAndFallback": "same-official-unknown", "comparisonOccurrence": "same-official-boolean-as-field", "markerDeclarationPath": markerFile.FileName(), "markerDeclarationRange": markerRange, "markerSha256": markerDigest})
			t.Logf("TSFG_EXCEPTION_AGGREGATE_GUARD_OFFICIAL_ORACLE_V1 case=%s %s", name, base64.StdEncoding.EncodeToString(oracle))
			if err != nil {
				t.Fatal(err)
			}
		}) {
			return
		}
		sym := func(id, name, kind, typ string, writable bool, where span) map[string]any {
			return map[string]any{"id": id, "name": name, "kind": kind, "type": typ, "writable": writable, "range": where}
		}
		class := sym("s1", "Box", "class", "a1", false, raw["className"])
		field := sym("s2", "flag", "field", "bool", true, raw["fieldName"])
		definition := sym("s3", "f", "function", "bool", false, raw["functionName"])
		parameter := sym("s4", "box", "parameter", "a1", true, raw["parameterName"])
		parameter["presence"] = map[string]any{"undefined": false, "null": false, "optional": false, "default": false, "range": raw["parameter"], "annotation": token["parameterAnnotation"]}
		caught := sym("s5", "caught", "local", "unknown", true, raw["catchName"])
		ref := func(typ, id string, where span) map[string]any {
			return map[string]any{"kind": "reference", "type": typ, "symbol": id, "range": where}
		}
		fieldValue := map[string]any{"kind": "field", "type": "bool", "receiver": ref("a1", "s4", token["receiver"]), "fieldPath": []string{"a1.f0"}, "symbol": "s2", "permission": map[string]any{"readable": true, "writable": true, "constructorSelf": false, "classRef": "a1"}, "range": token["fieldAccess"]}
		narrowed := ref("bool", "s5", token["narrowed"])
		left, right := any(fieldValue), any(narrowed)
		if caughtLeft {
			left, right = narrowed, fieldValue
		}
		comparison := map[string]any{"kind": "binary", "type": "bool", "operator": "===", "left": left, "right": right, "range": token["comparison"]}
		guard := map[string]any{"kind": "type-test", "type": "bool", "operator": "===", "operand": ref("unknown", "s5", raw["guardOperand"]), "typeTest": map[string]any{"tag": "boolean", "typeofRange": raw["typeof"], "tagRange": raw["tag"]}, "range": token["guard"]}
		attempt := map[string]any{"kind": "try", "range": raw["try"], "exception": map[string]any{
			"tryBlock": map[string]any{"range": raw["tryBlock"], "body": []any{map[string]any{"kind": "throw", "range": raw["tryThrow"], "expression": map[string]any{"kind": "boolean", "type": "bool", "boolean": true, "range": raw["tryValue"]}}}},
			"catchClause": map[string]any{"binding": caught, "declarationRange": raw["catchDeclaration"], "range": raw["catchClause"], "block": map[string]any{"range": raw["catchBlock"], "body": []any{
				map[string]any{"kind": "if", "range": token["if"], "condition": guard, "thenBody": []any{map[string]any{"kind": "block", "range": raw["thenBlock"], "body": []any{map[string]any{"kind": "return", "range": raw["return"], "expression": comparison}}}}},
				map[string]any{"kind": "throw", "range": raw["fallbackThrow"], "expression": ref("unknown", "s5", raw["fallback"])},
			}}},
		}}
		ctor := map[string]any{"id": "c1", "kind": "implicitDefault", "classRef": "a1", "signatureDeclaration": nil, "implementation": nil, "parameters": []any{}, "signature": map[string]any{"parameters": []any{}, "returnType": "a1"}, "resultType": "a1", "body": []any{}, "parameterInitializers": []any{}, "range": token["class"], "bodyRange": nil}
		fieldFact := map[string]any{"id": "a1.f0", "ordinal": 0, "name": "flag", "symbol": "s2", "type": "bool", "readonly": false, "access": "public", "optionalSyntax": false, "definiteAssertionSyntax": false, "initializer": map[string]any{"kind": "boolean", "type": "bool", "boolean": true, "range": raw["fieldInitializer"]}, "range": token["field"]}
		decl := map[string]any{"id": "a1", "name": "Box", "symbol": "s1", "category": "component", "nominalKey": map[string]any{"sourcePath": r.RootFile, "declarationPath": []string{"Box"}, "category": "component"}, "markerOrigin": map[string]any{"declarationPath": "/src/minimal-aggregates-v1.d.ts", "declarationRange": markerRange, "declarationSha256": markerDigest, "entryId": markerEntryID, "version": "minimal-aggregates-v1", "contextName": "ClassDecoratorContext", "contextArity": 1, "mode": "standard", "range": token["marker"]}, "instanceType": "a1", "constructorType": "constructor:a1", "fields": []any{fieldFact}, "constructors": []any{ctor}, "memberCoverage": []any{map[string]any{"kind": "KindPropertyDeclaration", "name": "flag", "symbol": "s2", "range": token["field"], "bodyRange": nil}}, "range": token["class"], "bodyRange": token["classBody"], "noInheritance": true}
		expected := map[string]any{"status": "ok", "inputs": []string{r.RootFile, "/src/minimal-aggregates-v1.d.ts", "/src/minimal-conversions-v5.d.ts"}, "symbols": []any{class, field, definition, parameter, caught}, "functions": []any{map[string]any{"name": "f", "symbol": "s3", "parameters": []any{parameter}, "returnType": "bool", "parameterInitializers": []any{nil}, "range": raw["function"], "body": []any{attempt}}}, "aggregateFacts": map[string]any{"profile": "minimal-aggregates-v1", "declarations": []any{decl}, "sourceCoverage": []any{map[string]any{"kind": "class", "name": "Box", "symbol": "s1", "range": token["class"], "bodyRange": token["classBody"]}, map[string]any{"kind": "function", "name": "f", "symbol": "s3", "range": raw["function"], "bodyRange": token["functionBody"]}}}}
		want, err := json.Marshal(expected)
		t.Logf("TSFG_EXCEPTION_AGGREGATE_GUARD_EXPECTED_V1 case=%s %s", name, base64.StdEncoding.EncodeToString(want))
		if err != nil {
			t.Fatal(err)
		}
		t.Run("publicProjection", func(t *testing.T) {
			var output bytes.Buffer
			commandErr := runFrontendCommand(t.Context(), nil, bytes.NewReader(input), &output)
			actual := output.Bytes()
			t.Logf("TSFG_EXCEPTION_AGGREGATE_GUARD_RESPONSE_V1 case=%s %s", name, base64.StdEncoding.EncodeToString(actual))
			if commandErr != nil {
				t.Fatalf("public command failed after official oracle: %v; raw=%s", commandErr, actual)
			}
			reject(t, !bytes.HasSuffix(actual, []byte("\n")), "public JSON final newline changed")
			var gotJSON, wantJSON any
			if err := json.Unmarshal(actual, &gotJSON); err != nil {
				t.Fatalf("invalid public JSON: %v; raw=%s", err, actual)
			}
			if err := json.Unmarshal(want, &wantJSON); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(gotJSON, wantJSON) {
				t.Fatalf("official occurrence oracle passed; complete owned facts differ: got=%s want=%s", actual, want)
			}
		})
	}
	if !t.Run("aggregateLeftControl", func(t *testing.T) { run(t, "aggregateLeftControl", "box.flag === caught", false) }) {
		return
	}
	t.Run("narrowedLeftCandidate", func(t *testing.T) { run(t, "narrowedLeftCandidate", "caught === box.flag", true) })
}
