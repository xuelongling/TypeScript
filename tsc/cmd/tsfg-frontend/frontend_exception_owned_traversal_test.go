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
	"github.com/microsoft/typescript-go/internal/checker"
)

// Two existing consumers must walk exception-owned children: lexical capture
// analysis and module source spelling. These are Source facts, not Runtime cell,
// GC, unwinding or execution evidence. Expected JSON never comes from projection.
func TestFrontendExceptionOwnedTraversal(t *testing.T) {
	t.Run("catchClosureOwnedRead", exceptionTraversalCatchClosureForTest)
	t.Run("moduleFunctionAndInitializationNames", exceptionTraversalModulesForTest)
}

type exceptionTraversalOracleForTest struct {
	t          *testing.T
	file       *ast.SourceFile
	raw, token map[string]span
}

func newExceptionTraversalOracleForTest(t *testing.T, file *ast.SourceFile) *exceptionTraversalOracleForTest {
	t.Helper()
	if file == nil {
		t.Fatal("official original source missing")
	}
	return &exceptionTraversalOracleForTest{t: t, file: file, raw: map[string]span{}, token: map[string]span{}}
}

func (o *exceptionTraversalOracleForTest) remember(label string, node *ast.Node, text string) {
	o.t.Helper()
	if node == nil || node.End() < len(text) {
		o.t.Fatalf("official node missing: %s", label)
	}
	start := node.End() - len(text)
	if node.Pos() > start || o.file.Text()[start:node.End()] != text {
		o.t.Fatalf("original preimage changed: %s", label)
	}
	o.raw[label] = throwBoundaryNodeForTest(o.t, o.file, node)
	o.token[label] = throwBoundarySpanForTest(o.file.Text(), start, node.End())
}

// Selection is by fixed literal source, official kind and a bounded AST subtree.
// It does not use producer walkers, normalized facts, IDs or offset helpers.
func (o *exceptionTraversalOracleForTest) site(root *ast.Node, label string, kind ast.Kind, text string) *ast.Node {
	o.t.Helper()
	var matches []*ast.Node
	var visit func(*ast.Node) bool
	visit = func(node *ast.Node) bool {
		if node.Kind == kind && strings.TrimSpace(o.file.Text()[node.Pos():node.End()]) == text {
			matches = append(matches, node)
		}
		node.ForEachChild(visit)
		return false
	}
	visit(root)
	if len(matches) != 1 {
		o.t.Fatalf("expected one official %s node, got %d", label, len(matches))
	}
	o.remember(label, matches[0], text)
	return matches[0]
}

func exceptionTraversalIdentityForTest(t *testing.T, check *checker.Checker, declaration *ast.Node, uses ...*ast.Node) *ast.Symbol {
	t.Helper()
	if declaration == nil || declaration.Name() == nil {
		t.Fatal("official named declaration missing")
	}
	s := check.GetSymbolAtLocation(declaration.Name())
	if s == nil || len(s.Declarations) != 1 || s.Declarations[0] != declaration {
		t.Fatal("actual official defining symbol identity changed")
	}
	for _, use := range uses {
		if use == nil || check.GetSymbolAtLocation(use) != s {
			t.Fatal("official use was substituted or rebound")
		}
	}
	return s
}

func exceptionTraversalExportsForTest(t *testing.T, check *checker.Checker, file *ast.SourceFile, declaration *ast.Node, name string) {
	t.Helper()
	exports := check.GetExportsOfModule(file.AsNode().Symbol())
	s := check.GetSymbolAtLocation(declaration.Name())
	if len(exports) != 1 || exports[0] != s || exports[0].Name != name || len(s.Declarations) != 1 || s.Declarations[0] != declaration || ast.GetSourceFileOfNode(s.Declarations[0]) != file {
		t.Fatal("complete official export set or declaration provenance changed")
	}
}

func exceptionTraversalRequestForTest(t *testing.T, r request) []byte {
	t.Helper()
	input, err := json.Marshal(r)
	t.Logf("TSFG_EXCEPTION_TRAVERSAL_REQUEST_V1 %s", base64.StdEncoding.EncodeToString(input))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		after, err := json.Marshal(r)
		if err != nil || !bytes.Equal(input, after) {
			t.Fatal("oracle/public command changed original request")
		}
	})
	return input
}

func exceptionTraversalCompareForTest(t *testing.T, input []byte, expected any) {
	t.Helper()
	var output bytes.Buffer
	err := runFrontendCommand(t.Context(), nil, bytes.NewReader(input), &output)
	raw := output.Bytes()
	t.Logf("TSFG_EXCEPTION_TRAVERSAL_RESPONSE_V1 %s", base64.StdEncoding.EncodeToString(raw))
	if err != nil {
		t.Fatalf("public command failed after official oracle: %v; raw=%s", err, raw)
	}
	if !bytes.HasSuffix(raw, []byte("\n")) {
		t.Fatal("public response lost final newline")
	}
	want, err := json.Marshal(expected)
	t.Logf("TSFG_EXCEPTION_TRAVERSAL_EXPECTED_V1 %s", base64.StdEncoding.EncodeToString(want))
	if err != nil {
		t.Fatal(err)
	}
	var gotJSON, wantJSON any
	if err := json.Unmarshal(raw, &gotJSON); err != nil {
		t.Fatalf("invalid public JSON: %v; raw=%s", err, raw)
	}
	if err := json.Unmarshal(want, &wantJSON); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotJSON, wantJSON) {
		t.Fatalf("official oracle passed, owned traversal differs: got=%s want=%s", raw, want)
	}
}

func exceptionTraversalSymbolForTest(id, name, kind, typ, owner, path string, writable bool, where span) map[string]any {
	s := map[string]any{"id": id, "name": name, "kind": kind, "type": typ, "writable": writable, "range": where}
	if owner != "" {
		s["lexicalOwner"] = owner
	}
	if path != "" {
		s["sourcePath"] = path
	}
	return s
}

func exceptionTraversalReferenceForTest(typ, id, name string, where span) map[string]any {
	e := map[string]any{"kind": "reference", "type": typ, "symbol": id, "range": where}
	if name != "" {
		e["name"] = name
	}
	return e
}

func exceptionTraversalGuardForTest(operand any, where, typeofRange, tagRange span) map[string]any {
	return map[string]any{"kind": "type-test", "type": "bool", "operator": "===", "operand": operand, "range": where,
		"typeTest": map[string]any{"tag": "boolean", "typeofRange": typeofRange, "tagRange": tagRange}}
}

func exceptionTraversalPresenceForTest(o *exceptionTraversalOracleForTest, label string) map[string]any {
	return map[string]any{"undefined": false, "null": false, "optional": false, "default": false, "range": o.raw[label], "annotation": o.token[label+"Type"]}
}

func exceptionTraversalCatchClosureForTest(t *testing.T) {
	const source = "/* 😀 */ export function entry(value: i32): bool { try { throw value; } catch (caught) { function inner(): bool { try { throw caught; } catch (nested) { return typeof nested === \"boolean\"; } } return inner(); } }"
	t.Logf("TSFG_EXCEPTION_TRAVERSAL_SOURCE_V1 %s", base64.StdEncoding.EncodeToString([]byte(source)))
	r := integerRequest(source)
	input := exceptionTraversalRequestForTest(t, r)
	program := throwBoundaryProgramForTest(t, r)
	var o *exceptionTraversalOracleForTest
	if !t.Run("officialDiagnosticsSymbolsExportsUTF16", func(t *testing.T) {
		throwBoundaryDiagnosticsForTest(t, program)
		file := program.GetSourceFile(r.RootFile)
		o = newExceptionTraversalOracleForTest(t, file)
		if file.Text() != source || len(file.Statements.Nodes) != 1 || file.Statements.Nodes[0].Kind != ast.KindFunctionDeclaration {
			t.Fatal("one original entry declaration required")
		}
		entry := file.Statements.Nodes[0]
		o.remember("entry", entry, strings.TrimPrefix(source, "/* 😀 */ "))
		o.remember("entryName", entry.Name(), "entry")
		parameter := o.site(entry, "parameter", ast.KindParameter, "value: i32")
		o.remember("parameterName", parameter.Name(), "value")
		o.remember("parameterType", parameter.AsParameterDeclaration().Type, "i32")
		outer := o.site(entry, "outer", ast.KindTryStatement, "try { throw value; } catch (caught) { function inner(): bool { try { throw caught; } catch (nested) { return typeof nested === \"boolean\"; } } return inner(); }")
		attempt := outer.AsTryStatement()
		if attempt.CatchClause == nil || attempt.FinallyBlock != nil {
			t.Fatal("outer catch-only presence changed")
		}
		o.remember("outerTryBlock", attempt.TryBlock, "{ throw value; }")
		firstThrow := o.site(attempt.TryBlock, "firstThrow", ast.KindThrowStatement, "throw value;")
		firstValue := firstThrow.AsThrowStatement().Expression
		o.remember("firstValue", firstValue, "value")
		clause := attempt.CatchClause.AsCatchClause()
		o.remember("outerCatch", attempt.CatchClause, "catch (caught) { function inner(): bool { try { throw caught; } catch (nested) { return typeof nested === \"boolean\"; } } return inner(); }")
		o.remember("outerCatchBlock", clause.Block, "{ function inner(): bool { try { throw caught; } catch (nested) { return typeof nested === \"boolean\"; } } return inner(); }")
		caught := clause.VariableDeclaration
		if caught == nil || caught.AsVariableDeclaration().Type != nil {
			t.Fatal("outer catch must remain unannotated local")
		}
		o.remember("caughtDeclaration", caught, "caught")
		o.remember("caughtName", caught.Name(), "caught")
		inner := o.site(clause.Block, "inner", ast.KindFunctionDeclaration, "function inner(): bool { try { throw caught; } catch (nested) { return typeof nested === \"boolean\"; } }")
		o.remember("innerName", inner.Name(), "inner")
		if len(inner.AsFunctionDeclaration().Parameters.Nodes) != 0 {
			t.Fatal("capture cannot be replaced by a parameter")
		}
		inside := o.site(inner, "inside", ast.KindTryStatement, "try { throw caught; } catch (nested) { return typeof nested === \"boolean\"; }")
		nestedTry := inside.AsTryStatement()
		if nestedTry.CatchClause == nil || nestedTry.FinallyBlock != nil {
			t.Fatal("inner catch-only presence changed")
		}
		o.remember("innerTryBlock", nestedTry.TryBlock, "{ throw caught; }")
		capturedThrow := o.site(nestedTry.TryBlock, "capturedThrow", ast.KindThrowStatement, "throw caught;")
		captured := capturedThrow.AsThrowStatement().Expression
		o.remember("capturedValue", captured, "caught")
		nestedClause := nestedTry.CatchClause.AsCatchClause()
		o.remember("nestedCatch", nestedTry.CatchClause, "catch (nested) { return typeof nested === \"boolean\"; }")
		o.remember("nestedBlock", nestedClause.Block, "{ return typeof nested === \"boolean\"; }")
		nested := nestedClause.VariableDeclaration
		if nested == nil || nested.AsVariableDeclaration().Type != nil {
			t.Fatal("nested catch must remain unannotated local")
		}
		o.remember("nestedDeclaration", nested, "nested")
		o.remember("nestedName", nested.Name(), "nested")
		returned := o.site(nestedClause.Block, "guardReturn", ast.KindReturnStatement, "return typeof nested === \"boolean\";")
		guard := returned.AsReturnStatement().Expression
		o.remember("guard", guard, "typeof nested === \"boolean\"")
		if guard.Kind != ast.KindBinaryExpression {
			t.Fatal("original typeof equality lost")
		}
		binary := guard.AsBinaryExpression()
		if binary.OperatorToken.Kind != ast.KindEqualsEqualsEqualsToken || binary.Left.Kind != ast.KindTypeOfExpression || binary.Right.Kind != ast.KindStringLiteral || binary.Right.Text() != "boolean" {
			t.Fatal("guard syntax changed")
		}
		operand := binary.Left.AsTypeOfExpression().Expression
		o.remember("typeof", binary.Left, "typeof nested")
		o.remember("tag", binary.Right, "\"boolean\"")
		o.remember("operand", operand, "nested")
		callReturn := o.site(clause.Block, "callReturn", ast.KindReturnStatement, "return inner();")
		call := callReturn.AsReturnStatement().Expression
		if call.Kind != ast.KindCallExpression || len(call.AsCallExpression().Arguments.Nodes) != 0 {
			t.Fatal("original zero-argument inner call lost")
		}
		o.remember("call", call, "inner()")
		check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
		defer release()
		entrySymbol := exceptionTraversalIdentityForTest(t, check, entry)
		innerSymbol := exceptionTraversalIdentityForTest(t, check, inner, call.AsCallExpression().Expression)
		parameterSymbol := exceptionTraversalIdentityForTest(t, check, parameter, firstValue)
		caughtSymbol := exceptionTraversalIdentityForTest(t, check, caught, captured)
		nestedSymbol := exceptionTraversalIdentityForTest(t, check, nested, operand)
		seen := map[*ast.Symbol]bool{}
		for _, s := range []*ast.Symbol{entrySymbol, innerSymbol, parameterSymbol, caughtSymbol, nestedSymbol} {
			if seen[s] {
				t.Fatal("official lexical identities collapsed")
			}
			seen[s] = true
		}
		i32 := check.GetTypeFromTypeNode(parameter.AsParameterDeclaration().Type)
		if i32 == nil || check.GetTypeAtLocation(parameter.Name()) != i32 || check.GetTypeAtLocation(firstValue) != i32 || check.GetTypeOfSymbol(caughtSymbol) != check.GetUnknownType() || check.GetTypeAtLocation(caught.Name()) != check.GetUnknownType() || check.GetTypeAtLocation(captured) != check.GetUnknownType() || check.GetTypeOfSymbol(nestedSymbol) != check.GetUnknownType() || check.GetTypeAtLocation(nested.Name()) != check.GetUnknownType() || check.GetTypeAtLocation(operand) != check.GetUnknownType() || !check.GetTypeAtLocation(guard).IsBooleanLike() || !check.GetTypeFromTypeNode(entry.AsFunctionDeclaration().Type).IsBooleanLike() || !check.GetTypeFromTypeNode(inner.AsFunctionDeclaration().Type).IsBooleanLike() || !check.GetTypeAtLocation(call).IsBooleanLike() {
			t.Fatal("official nominal/unknown/boolean roles changed")
		}
		exceptionTraversalExportsForTest(t, check, file, entry, "entry")
		oracle, err := json.Marshal(map[string]any{"kind": "test-only-official-traversal-oracle", "sourcePath": file.FileName(), "raw": o.raw, "token": o.token, "exports": []string{"entry"}, "capture": "caught-declaration == inner.tryBlock.throw-use; not parameter; read-only occurrence"})
		t.Logf("TSFG_EXCEPTION_TRAVERSAL_OFFICIAL_ORACLE_V1 %s", base64.StdEncoding.EncodeToString(oracle))
		if err != nil {
			t.Fatal(err)
		}
	}) {
		return
	}
	t.Run("ownedProjection", func(t *testing.T) {
		entry := exceptionTraversalSymbolForTest("s1", "entry", "function", "bool", "", "", false, o.raw["entryName"])
		inner := exceptionTraversalSymbolForTest("s2", "inner", "function", "bool", "s1", "", false, o.raw["innerName"])
		parameter := exceptionTraversalSymbolForTest("s3", "value", "parameter", "i32", "s1", "", true, o.raw["parameterName"])
		parameter["presence"] = exceptionTraversalPresenceForTest(o, "parameter")
		caught := exceptionTraversalSymbolForTest("s4", "caught", "local", "unknown", "s1", "", true, o.raw["caughtName"])
		nested := exceptionTraversalSymbolForTest("s5", "nested", "local", "unknown", "s2", "", true, o.raw["nestedName"])
		guard := exceptionTraversalGuardForTest(exceptionTraversalReferenceForTest("unknown", "s5", "", o.raw["operand"]), o.token["guard"], o.raw["typeof"], o.raw["tag"])
		inside := map[string]any{"kind": "try", "range": o.raw["inside"], "exception": map[string]any{
			"tryBlock":    map[string]any{"range": o.raw["innerTryBlock"], "body": []any{map[string]any{"kind": "throw", "range": o.raw["capturedThrow"], "expression": exceptionTraversalReferenceForTest("unknown", "s4", "", o.raw["capturedValue"])}}},
			"catchClause": map[string]any{"binding": nested, "range": o.raw["nestedCatch"], "declarationRange": o.raw["nestedDeclaration"], "block": map[string]any{"range": o.raw["nestedBlock"], "body": []any{map[string]any{"kind": "return", "range": o.raw["guardReturn"], "expression": guard}}}}}}
		outer := map[string]any{"kind": "try", "range": o.raw["outer"], "exception": map[string]any{
			"tryBlock": map[string]any{"range": o.raw["outerTryBlock"], "body": []any{map[string]any{"kind": "throw", "range": o.raw["firstThrow"], "expression": exceptionTraversalReferenceForTest("i32", "s3", "", o.raw["firstValue"])}}},
			"catchClause": map[string]any{"binding": caught, "range": o.raw["outerCatch"], "declarationRange": o.raw["caughtDeclaration"], "block": map[string]any{"range": o.raw["outerCatchBlock"], "body": []any{
				map[string]any{"kind": "functionDeclaration", "symbol": inner, "range": o.raw["inner"]},
				map[string]any{"kind": "return", "range": o.raw["callReturn"], "expression": map[string]any{"kind": "call", "type": "bool", "target": "s2", "arguments": []any{}, "signature": map[string]any{"parameters": []any{}, "returnType": "bool"}, "range": o.raw["call"]}},
			}}}}}
		expected := map[string]any{"status": "ok", "inputs": []string{"/src/main.ts", "/src/minimal-integers-v3.d.ts"}, "symbols": []any{entry, inner, parameter, caught, nested}, "functions": []any{
			map[string]any{"name": "entry", "symbol": "s1", "parameters": []any{parameter}, "returnType": "bool", "parameterInitializers": []any{nil}, "range": o.raw["entry"], "body": []any{outer}},
			map[string]any{"name": "inner", "symbol": "s2", "parameters": []any{}, "returnType": "bool", "lexicalParent": "s1", "captures": []any{map[string]any{"symbol": "s4", "read": true, "write": false}}, "range": o.raw["inner"], "body": []any{inside}},
		}}
		exceptionTraversalCompareForTest(t, input, expected)
	})
}

func exceptionTraversalModulesForTest(t *testing.T) {
	const app = "import { probe } from \"./worker\"; export function entry(value: i32): bool { return probe(value); }"
	const worker = "/* 😀 */ export function probe(value: i32): bool { try { throw value; } catch (caught: unknown) { if (typeof caught === \"boolean\") { return caught; } throw caught; } finally { const copied: i32 = value; } } try { throw true; } catch (initCaught: unknown) { if (typeof initCaught === \"boolean\") { throw initCaught; } throw initCaught; } finally {}"
	t.Logf("TSFG_EXCEPTION_TRAVERSAL_SOURCE_V1 app=%s worker=%s", base64.StdEncoding.EncodeToString([]byte(app)), base64.StdEncoding.EncodeToString([]byte(worker)))
	r := moduleRequest([]inputFile{{Path: "/src/app.ts", Content: app}, {Path: "/src/worker.ts", Content: worker}}, "/src/app.ts")
	input := exceptionTraversalRequestForTest(t, r)
	program := throwBoundaryProgramForTest(t, r)
	var a, w *exceptionTraversalOracleForTest
	if !t.Run("officialDiagnosticsSymbolsExportsUTF16", func(t *testing.T) {
		throwBoundaryDiagnosticsForTest(t, program)
		appFile, workerFile := program.GetSourceFile("/src/app.ts"), program.GetSourceFile("/src/worker.ts")
		a, w = newExceptionTraversalOracleForTest(t, appFile), newExceptionTraversalOracleForTest(t, workerFile)
		if appFile.Text() != app || workerFile.Text() != worker || len(appFile.Statements.Nodes) != 2 || len(workerFile.Statements.Nodes) != 2 || appFile.Statements.Nodes[0].Kind != ast.KindImportDeclaration || appFile.Statements.Nodes[1].Kind != ast.KindFunctionDeclaration || workerFile.Statements.Nodes[0].Kind != ast.KindFunctionDeclaration || workerFile.Statements.Nodes[1].Kind != ast.KindTryStatement {
			t.Fatal("original two-module statement tables changed")
		}
		entry, probe := appFile.Statements.Nodes[1], workerFile.Statements.Nodes[0]
		a.remember("entry", entry, "export function entry(value: i32): bool { return probe(value); }")
		a.remember("entryName", entry.Name(), "entry")
		ap := a.site(entry, "parameter", ast.KindParameter, "value: i32")
		a.remember("parameterName", ap.Name(), "value")
		a.remember("parameterType", ap.AsParameterDeclaration().Type, "i32")
		returned := a.site(entry, "return", ast.KindReturnStatement, "return probe(value);")
		call := returned.AsReturnStatement().Expression
		if call.Kind != ast.KindCallExpression || len(call.AsCallExpression().Arguments.Nodes) != 1 {
			t.Fatal("original imported call shape changed")
		}
		a.remember("call", call, "probe(value)")
		a.remember("argument", call.AsCallExpression().Arguments.Nodes[0], "value")
		imp := appFile.Statements.Nodes[0]
		a.remember("import", imp, "import { probe } from \"./worker\";")
		importName := a.site(imp, "importName", ast.KindIdentifier, "probe")
		w.remember("probe", probe, "export function probe(value: i32): bool { try { throw value; } catch (caught: unknown) { if (typeof caught === \"boolean\") { return caught; } throw caught; } finally { const copied: i32 = value; } }")
		w.remember("probeName", probe.Name(), "probe")
		wp := w.site(probe, "parameter", ast.KindParameter, "value: i32")
		w.remember("parameterName", wp.Name(), "value")
		w.remember("parameterType", wp.AsParameterDeclaration().Type, "i32")
		check, release := program.GetTypeCheckerForFileExclusive(t.Context(), appFile)
		defer release()
		entrySymbol := exceptionTraversalIdentityForTest(t, check, entry)
		probeSymbol := exceptionTraversalIdentityForTest(t, check, probe)
		alias := check.GetSymbolAtLocation(importName)
		if alias == nil || alias.Flags&ast.SymbolFlagsAlias == 0 || check.GetImmediateAliasedSymbol(alias) != probeSymbol || check.GetSymbolAtLocation(call.AsCallExpression().Expression) != alias || entrySymbol == probeSymbol {
			t.Fatal("official imported binding lost actual worker declaration identity")
		}
		exceptionTraversalIdentityForTest(t, check, ap, call.AsCallExpression().Arguments.Nodes[0])
		wpSymbol := exceptionTraversalIdentityForTest(t, check, wp)
		i32 := check.GetTypeFromTypeNode(wp.AsParameterDeclaration().Type)
		if i32 == nil || check.GetTypeFromTypeNode(ap.AsParameterDeclaration().Type) != i32 || !check.GetTypeFromTypeNode(entry.AsFunctionDeclaration().Type).IsBooleanLike() || !check.GetTypeFromTypeNode(probe.AsFunctionDeclaration().Type).IsBooleanLike() || !check.GetTypeAtLocation(call).IsBooleanLike() {
			t.Fatal("official cross-module scalar/boolean signature identity changed")
		}
		for index, root := range []*ast.Node{probe.Body(), workerFile.AsNode()} {
			prefix, catchName, tryText, blockText, catchText, finallyText := "fn", "caught", "try { throw value; } catch (caught: unknown) { if (typeof caught === \"boolean\") { return caught; } throw caught; } finally { const copied: i32 = value; }", "{ throw value; }", "catch (caught: unknown) { if (typeof caught === \"boolean\") { return caught; } throw caught; }", "{ const copied: i32 = value; }"
			if index == 1 {
				prefix, catchName, tryText, blockText, catchText, finallyText = "init", "initCaught", "try { throw true; } catch (initCaught: unknown) { if (typeof initCaught === \"boolean\") { throw initCaught; } throw initCaught; } finally {}", "{ throw true; }", "catch (initCaught: unknown) { if (typeof initCaught === \"boolean\") { throw initCaught; } throw initCaught; }", "{}"
			}
			node := w.site(root, prefix+"Try", ast.KindTryStatement, tryText)
			attempt := node.AsTryStatement()
			if attempt.CatchClause == nil || attempt.FinallyBlock == nil {
				t.Fatal("both exception consumers require all three present regions")
			}
			w.remember(prefix+"TryBlock", attempt.TryBlock, blockText)
			w.remember(prefix+"Catch", attempt.CatchClause, catchText)
			w.remember(prefix+"Finally", attempt.FinallyBlock, finallyText)
			clause := attempt.CatchClause.AsCatchClause()
			declaration := clause.VariableDeclaration
			if declaration == nil || declaration.AsVariableDeclaration().Type == nil {
				t.Fatal("explicit official unknown annotation missing")
			}
			w.remember(prefix+"Declaration", declaration, catchName+": unknown")
			w.remember(prefix+"Name", declaration.Name(), catchName)
			w.remember(prefix+"Annotation", declaration.AsVariableDeclaration().Type, "unknown")
			catchBody := strings.TrimPrefix(catchText, "catch ("+catchName+": unknown) ")
			w.remember(prefix+"CatchBlock", clause.Block, catchBody)
			statements := clause.Block.AsBlock().Statements.Nodes
			if len(statements) != 2 || statements[0].Kind != ast.KindIfStatement || statements[1].Kind != ast.KindThrowStatement {
				t.Fatal("guard and fallback rethrow order changed")
			}
			guarded := statements[0].AsIfStatement()
			if guarded.ElseStatement != nil || guarded.ThenStatement.Kind != ast.KindBlock || len(guarded.ThenStatement.AsBlock().Statements.Nodes) != 1 {
				t.Fatal("single guarded region required")
			}
			action := "return "
			actionKind := ast.KindReturnStatement
			if index == 1 {
				action, actionKind = "throw ", ast.KindThrowStatement
			}
			w.remember(prefix+"If", statements[0], "if (typeof "+catchName+" === \"boolean\") { "+action+catchName+"; }")
			guard := guarded.Expression
			w.remember(prefix+"Guard", guard, "typeof "+catchName+" === \"boolean\"")
			if guard.Kind != ast.KindBinaryExpression {
				t.Fatal("official typeof equality required")
			}
			binary := guard.AsBinaryExpression()
			if binary.OperatorToken.Kind != ast.KindEqualsEqualsEqualsToken || binary.Left.Kind != ast.KindTypeOfExpression || binary.Right.Kind != ast.KindStringLiteral || binary.Right.Text() != "boolean" {
				t.Fatal("official boolean tag syntax changed")
			}
			operand := binary.Left.AsTypeOfExpression().Expression
			w.remember(prefix+"Typeof", binary.Left, "typeof "+catchName)
			w.remember(prefix+"Tag", binary.Right, "\"boolean\"")
			w.remember(prefix+"Operand", operand, catchName)
			w.remember(prefix+"ActionBlock", guarded.ThenStatement, "{ "+action+catchName+"; }")
			branch := w.site(guarded.ThenStatement, prefix+"Action", actionKind, action+catchName+";")
			var use *ast.Node
			if index == 0 {
				use = branch.AsReturnStatement().Expression
			} else {
				use = branch.AsThrowStatement().Expression
			}
			w.remember(prefix+"Narrowed", use, catchName)
			w.remember(prefix+"Rethrow", statements[1], "throw "+catchName+";")
			fallback := statements[1].AsThrowStatement().Expression
			w.remember(prefix+"Fallback", fallback, catchName)
			catchSymbol := exceptionTraversalIdentityForTest(t, check, declaration, operand, use, fallback)
			if catchSymbol == wpSymbol || check.GetTypeOfSymbol(catchSymbol) != check.GetUnknownType() || check.GetTypeFromTypeNode(declaration.AsVariableDeclaration().Type) != check.GetUnknownType() || check.GetTypeAtLocation(operand) != check.GetUnknownType() || check.GetTypeAtLocation(fallback) != check.GetUnknownType() || !check.GetTypeAtLocation(use).IsBooleanLike() || !check.GetTypeAtLocation(guard).IsBooleanLike() {
				t.Fatal("module catch declaration/occurrence identities or flow changed")
			}
			throwText, payloadText, payloadKind := "throw value;", "value", ast.KindIdentifier
			if index == 1 {
				throwText, payloadText, payloadKind = "throw true;", "true", ast.KindTrueKeyword
			}
			thrown := w.site(attempt.TryBlock, prefix+"Throw", ast.KindThrowStatement, throwText)
			payload := w.site(thrown, prefix+"Payload", payloadKind, payloadText)
			if index == 0 {
				exceptionTraversalIdentityForTest(t, check, wp, payload)
				if check.GetTypeAtLocation(payload) != i32 {
					t.Fatal("original module parameter payload changed")
				}
				local := w.site(attempt.FinallyBlock, "copyDeclaration", ast.KindVariableDeclaration, "copied: i32 = value")
				w.remember("copyName", local.Name(), "copied")
				initializer := local.AsVariableDeclaration().Initializer
				w.remember("copyInitializer", initializer, "value")
				exceptionTraversalIdentityForTest(t, check, local)
				exceptionTraversalIdentityForTest(t, check, wp, initializer)
				if check.GetTypeFromTypeNode(local.AsVariableDeclaration().Type) != i32 || check.GetTypeAtLocation(initializer) != i32 || check.GetTypeAtLocation(local.Name()) != i32 || attempt.FinallyBlock.AsBlock().Statements.Nodes[0].AsVariableStatement().DeclarationList.Flags&ast.NodeFlagsConst == 0 {
					t.Fatal("finally immutable same-i32 local changed")
				}
			} else if !check.GetTypeAtLocation(payload).IsBooleanLike() || len(attempt.FinallyBlock.AsBlock().Statements.Nodes) != 0 {
				t.Fatal("minimal top-level boolean/empty-finally control changed")
			}
		}
		exceptionTraversalExportsForTest(t, check, appFile, entry, "entry")
		exceptionTraversalExportsForTest(t, check, workerFile, probe, "probe")
		oracle, err := json.Marshal(map[string]any{"kind": "test-only-official-module-traversal-oracle", "app": map[string]any{"path": appFile.FileName(), "raw": a.raw, "token": a.token, "exports": []string{"entry"}}, "worker": map[string]any{"path": workerFile.FileName(), "raw": w.raw, "token": w.token, "exports": []string{"probe"}}, "importTarget": "same-official-worker-probe"})
		t.Logf("TSFG_EXCEPTION_TRAVERSAL_OFFICIAL_ORACLE_V1 %s", base64.StdEncoding.EncodeToString(oracle))
		if err != nil {
			t.Fatal(err)
		}
	}) {
		return
	}
	t.Run("ownedProjection", func(t *testing.T) {
		entry := exceptionTraversalSymbolForTest("s1", "entry", "function", "bool", "", "/src/app.ts", false, a.raw["entryName"])
		probe := exceptionTraversalSymbolForTest("s2", "probe", "function", "bool", "", "/src/worker.ts", false, w.raw["probeName"])
		ap := exceptionTraversalSymbolForTest("s3", "value", "parameter", "i32", "", "/src/app.ts", true, a.raw["parameterName"])
		wp := exceptionTraversalSymbolForTest("s4", "value", "parameter", "i32", "", "/src/worker.ts", true, w.raw["parameterName"])
		ap["presence"], wp["presence"] = exceptionTraversalPresenceForTest(a, "parameter"), exceptionTraversalPresenceForTest(w, "parameter")
		caught := exceptionTraversalSymbolForTest("s5", "caught", "local", "unknown", "", "/src/worker.ts", true, w.raw["fnName"])
		copied := exceptionTraversalSymbolForTest("s6", "copied", "local", "i32", "", "/src/worker.ts", false, w.raw["copyName"])
		initCaught := exceptionTraversalSymbolForTest("s7", "initCaught", "local", "unknown", "", "/src/worker.ts", true, w.raw["initName"])
		regions := map[string]any{}
		for _, row := range []struct {
			prefix, name, id string
			binding          any
		}{{"fn", "caught", "s5", caught}, {"init", "initCaught", "s7", initCaught}} {
			p := row.prefix
			guard := exceptionTraversalGuardForTest(exceptionTraversalReferenceForTest("unknown", row.id, row.name, w.raw[p+"Operand"]), w.token[p+"Guard"], w.raw[p+"Typeof"], w.raw[p+"Tag"])
			payload := any(exceptionTraversalReferenceForTest("i32", "s4", "value", w.raw[p+"Payload"]))
			action := "return"
			finalBody := []any{map[string]any{"kind": "variable", "symbol": copied, "initializer": exceptionTraversalReferenceForTest("i32", "s4", "value", w.raw["copyInitializer"]), "range": w.raw["copyDeclaration"]}}
			if p == "init" {
				payload = map[string]any{"kind": "boolean", "type": "bool", "boolean": true, "range": w.raw[p+"Payload"]}
				action = "throw"
				finalBody = []any{}
			}
			regions[p] = map[string]any{"kind": "try", "range": w.raw[p+"Try"], "exception": map[string]any{
				"tryBlock": map[string]any{"range": w.raw[p+"TryBlock"], "body": []any{map[string]any{"kind": "throw", "range": w.raw[p+"Throw"], "expression": payload}}},
				"catchClause": map[string]any{"binding": row.binding, "declarationRange": w.raw[p+"Declaration"], "annotationRange": w.raw[p+"Annotation"], "range": w.raw[p+"Catch"], "block": map[string]any{"range": w.raw[p+"CatchBlock"], "body": []any{
					map[string]any{"kind": "if", "condition": guard, "range": w.token[p+"If"], "thenBody": []any{map[string]any{"kind": "block", "range": w.raw[p+"ActionBlock"], "body": []any{map[string]any{"kind": action, "expression": exceptionTraversalReferenceForTest("bool", row.id, row.name, w.raw[p+"Narrowed"]), "range": w.raw[p+"Action"]}}}}},
					map[string]any{"kind": "throw", "expression": exceptionTraversalReferenceForTest("unknown", row.id, row.name, w.raw[p+"Fallback"]), "range": w.raw[p+"Rethrow"]},
				}}}, "finallyBlock": map[string]any{"range": w.raw[p+"Finally"], "body": finalBody},
			}}
		}
		// sourcePath belongs to the existing function/module/symbol owner. Each
		// child span is checked against that worker's original text, not app text.
		expected := map[string]any{"status": "ok", "inputs": []string{"/src/app.ts", "/src/minimal-scalars-v2.d.ts", "/src/worker.ts"}, "symbols": []any{entry, probe, ap, wp, caught, copied, initCaught}, "functions": []any{
			map[string]any{"sourcePath": "/src/app.ts", "name": "entry", "symbol": "s1", "parameters": []any{ap}, "returnType": "bool", "parameterInitializers": []any{nil}, "range": a.raw["entry"], "body": []any{map[string]any{"kind": "return", "range": a.raw["return"], "expression": map[string]any{"kind": "call", "name": "probe", "type": "bool", "target": "s2", "arguments": []any{exceptionTraversalReferenceForTest("i32", "s3", "value", a.raw["argument"])}, "signature": map[string]any{"parameters": []string{"i32"}, "returnType": "bool", "presence": []any{exceptionTraversalPresenceForTest(w, "parameter")}}, "range": a.raw["call"]}}}},
			map[string]any{"sourcePath": "/src/worker.ts", "name": "probe", "symbol": "s2", "parameters": []any{wp}, "returnType": "bool", "parameterInitializers": []any{nil}, "range": w.raw["probe"], "body": []any{regions["fn"]}},
		}, "modules": []any{
			map[string]any{"sourcePath": "/src/app.ts", "external": true, "globals": []any{}, "initialization": []any{}, "exports": []any{map[string]any{"name": "entry", "target": "s1", "range": a.raw["entry"]}}, "imports": []any{map[string]any{"kind": "import", "specifier": "./worker", "sourcePath": "/src/worker.ts", "range": a.raw["import"], "bindings": []any{map[string]any{"name": "probe", "target": "s2", "range": a.token["importName"]}}}}},
			map[string]any{"sourcePath": "/src/worker.ts", "external": true, "globals": []any{}, "initialization": []any{regions["init"]}, "exports": []any{map[string]any{"name": "probe", "target": "s2", "range": w.raw["probe"]}}, "imports": []any{}},
		}}
		exceptionTraversalCompareForTest(t, input, expected)
	})
}
