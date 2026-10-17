// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
)

// Golden is the independent EXPECTED from the 21b owned traversal test, not an
// actual public response: same-current-GREEN-public-captures/07/expected.json.
// Its SHA256 is 055c3add3d592f06b25c3a01e7dfd4146e7c2c176cf5c79f13a03da4fae5ec0b.
func TestExceptionFactsCommandV1ModuleTerminalExports(t *testing.T) {
	const app = "import { probe } from \"./worker\"; export function entry(value: i32): bool { return probe(value); }"
	const worker = "/* 😀 */ export function probe(value: i32): bool { try { throw value; } catch (caught: unknown) { if (typeof caught === \"boolean\") { return caught; } throw caught; } finally { const copied: i32 = value; } } try { throw true; } catch (initCaught: unknown) { if (typeof initCaught === \"boolean\") { throw initCaught; } throw initCaught; } finally {}"
	t.Logf("TSFG_EXCEPTION_EXPORTS_SOURCE_V1 case=moduleTerminal app=%s worker=%s", base64.StdEncoding.EncodeToString([]byte(app)), base64.StdEncoding.EncodeToString([]byte(worker)))
	r := moduleRequest([]inputFile{{Path: "/src/app.ts", Content: app}, {Path: "/src/worker.ts", Content: worker}}, "/src/app.ts")
	input := exceptionPackageExportsRequestForTest(t, "moduleTerminal", r)
	if exceptionPackageExportsDigestForTest(input) != "sha256:4df98ce9bd11d42cfca10acdc54b0bd7cc3528333a7445a2e23f96bb04f8a9ce" {
		t.Fatal("original 21b module request bytes changed")
	}
	program := throwBoundaryProgramForTest(t, r)
	var a, w *exceptionTraversalOracleForTest
	var exportGroups []any
	if !t.Run("officialDiagnosticsSymbolsExportsUTF16", func(t *testing.T) {
		throwBoundaryDiagnosticsForTest(t, program)
		appFile, workerFile := program.GetSourceFile("/src/app.ts"), program.GetSourceFile("/src/worker.ts")
		a, w = newExceptionTraversalOracleForTest(t, appFile), newExceptionTraversalOracleForTest(t, workerFile)
		if appFile.Text() != app || workerFile.Text() != worker || len(appFile.Statements.Nodes) != 2 || len(workerFile.Statements.Nodes) != 2 || appFile.Statements.Nodes[0].Kind != ast.KindImportDeclaration || appFile.Statements.Nodes[1].Kind != ast.KindFunctionDeclaration || workerFile.Statements.Nodes[0].Kind != ast.KindFunctionDeclaration || workerFile.Statements.Nodes[1].Kind != ast.KindTryStatement {
			t.Fatal("original two-module statement tables changed")
		}
		consumed := []string{}
		for _, sourceFile := range program.GetSourceFiles() {
			consumed = append(consumed, sourceFile.FileName())
		}
		slices.Sort(consumed)
		if !reflect.DeepEqual(consumed, []string{"/src/app.ts", "/src/minimal-scalars-v2.d.ts", "/src/worker.ts"}) {
			t.Fatal("complete original module consumption changed")
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
		ids := map[*ast.Symbol]string{entrySymbol: "s1", probeSymbol: "s2"}
		definingNodes := []*ast.Node{entry, probe}
		for i, file := range []*ast.SourceFile{appFile, workerFile} {
			exported := check.GetExportsOfModule(file.AsNode().Symbol())
			if !ast.IsExternalModule(file) || len(exported) != 1 || exported[0].Flags&ast.SymbolFlagsAlias != 0 || len(exported[0].Declarations) != 1 {
				t.Fatal("complete actual direct module export group changed")
			}
			declaration := exported[0].Declarations[0]
			if declaration != definingNodes[i] || ast.GetSourceFileOfNode(declaration) != file {
				t.Fatal("actual exported declaration/source identity changed")
			}
			actual := check.GetSymbolAtLocation(declaration.Name())
			if actual != exported[0] || ids[actual] == "" {
				t.Fatal("official export did not map actual checker declaration symbol")
			}
			exportGroups = append(exportGroups, map[string]any{"sourcePath": file.FileName(), "external": true, "bindings": []any{map[string]any{
				"name": exported[0].Name, "target": ids[actual], "sourcePath": file.FileName(), "range": throwBoundaryNodeForTest(t, file, declaration), "definition": throwBoundaryNodeForTest(t, file, declaration.Name()),
			}}})
		}
		oracle, err := json.Marshal(map[string]any{"case": "moduleTerminal", "completeExports": exportGroups, "kind": "test-only-official-module-traversal-oracle", "app": map[string]any{"path": appFile.FileName(), "raw": a.raw, "token": a.token, "exports": []string{"entry"}}, "worker": map[string]any{"path": workerFile.FileName(), "raw": w.raw, "token": w.token, "exports": []string{"probe"}}, "importTarget": "same-official-worker-probe"})
		t.Logf("TSFG_EXCEPTION_EXPORTS_OFFICIAL_ORACLE_V1 case=moduleTerminal %s", base64.StdEncoding.EncodeToString(oracle))
		if err != nil {
			t.Fatal(err)
		}
	}) {
		return
	}
	const golden = "{\"functions\":[{\"body\":[{\"expression\":{\"arguments\":[{\"kind\":\"reference\",\"name\":\"value\",\"range\":{\"start\":89,\"end\":94},\"symbol\":\"s3\",\"type\":\"i32\"}],\"kind\":\"call\",\"name\":\"probe\",\"range\":{\"start\":82,\"end\":95},\"signature\":{\"parameters\":[\"i32\"],\"presence\":[{\"annotation\":{\"start\":38,\"end\":41},\"default\":false,\"null\":false,\"optional\":false,\"range\":{\"start\":31,\"end\":41},\"undefined\":false}],\"returnType\":\"bool\"},\"target\":\"s2\",\"type\":\"bool\"},\"kind\":\"return\",\"range\":{\"start\":75,\"end\":96}}],\"name\":\"entry\",\"parameterInitializers\":[null],\"parameters\":[{\"id\":\"s3\",\"kind\":\"parameter\",\"name\":\"value\",\"presence\":{\"annotation\":{\"start\":63,\"end\":66},\"default\":false,\"null\":false,\"optional\":false,\"range\":{\"start\":56,\"end\":66},\"undefined\":false},\"range\":{\"start\":56,\"end\":61},\"sourcePath\":\"/src/app.ts\",\"type\":\"i32\",\"writable\":true}],\"range\":{\"start\":33,\"end\":98},\"returnType\":\"bool\",\"sourcePath\":\"/src/app.ts\",\"symbol\":\"s1\"},{\"body\":[{\"exception\":{\"catchClause\":{\"annotationRange\":{\"start\":86,\"end\":94},\"binding\":{\"id\":\"s5\",\"kind\":\"local\",\"name\":\"caught\",\"range\":{\"start\":79,\"end\":85},\"sourcePath\":\"/src/worker.ts\",\"type\":\"unknown\",\"writable\":true},\"block\":{\"body\":[{\"condition\":{\"kind\":\"type-test\",\"operand\":{\"kind\":\"reference\",\"name\":\"caught\",\"range\":{\"start\":108,\"end\":115},\"symbol\":\"s5\",\"type\":\"unknown\"},\"operator\":\"===\",\"range\":{\"start\":102,\"end\":129},\"type\":\"bool\",\"typeTest\":{\"tag\":\"boolean\",\"tagRange\":{\"start\":119,\"end\":129},\"typeofRange\":{\"start\":102,\"end\":115}}},\"kind\":\"if\",\"range\":{\"start\":98,\"end\":149},\"thenBody\":[{\"body\":[{\"expression\":{\"kind\":\"reference\",\"name\":\"caught\",\"range\":{\"start\":139,\"end\":146},\"symbol\":\"s5\",\"type\":\"bool\"},\"kind\":\"return\",\"range\":{\"start\":132,\"end\":147}}],\"kind\":\"block\",\"range\":{\"start\":130,\"end\":149}}]},{\"expression\":{\"kind\":\"reference\",\"name\":\"caught\",\"range\":{\"start\":155,\"end\":162},\"symbol\":\"s5\",\"type\":\"unknown\"},\"kind\":\"throw\",\"range\":{\"start\":149,\"end\":163}}],\"range\":{\"start\":95,\"end\":165}},\"declarationRange\":{\"start\":79,\"end\":94},\"range\":{\"start\":71,\"end\":165}},\"finallyBlock\":{\"body\":[{\"initializer\":{\"kind\":\"reference\",\"name\":\"value\",\"range\":{\"start\":195,\"end\":201},\"symbol\":\"s4\",\"type\":\"i32\"},\"kind\":\"variable\",\"range\":{\"start\":181,\"end\":201},\"symbol\":{\"id\":\"s6\",\"kind\":\"local\",\"name\":\"copied\",\"range\":{\"start\":181,\"end\":188},\"sourcePath\":\"/src/worker.ts\",\"type\":\"i32\",\"writable\":false}}],\"range\":{\"start\":173,\"end\":204}},\"tryBlock\":{\"body\":[{\"expression\":{\"kind\":\"reference\",\"name\":\"value\",\"range\":{\"start\":62,\"end\":68},\"symbol\":\"s4\",\"type\":\"i32\"},\"kind\":\"throw\",\"range\":{\"start\":56,\"end\":69}}],\"range\":{\"start\":54,\"end\":71}}},\"kind\":\"try\",\"range\":{\"start\":50,\"end\":204}}],\"name\":\"probe\",\"parameterInitializers\":[null],\"parameters\":[{\"id\":\"s4\",\"kind\":\"parameter\",\"name\":\"value\",\"presence\":{\"annotation\":{\"start\":38,\"end\":41},\"default\":false,\"null\":false,\"optional\":false,\"range\":{\"start\":31,\"end\":41},\"undefined\":false},\"range\":{\"start\":31,\"end\":36},\"sourcePath\":\"/src/worker.ts\",\"type\":\"i32\",\"writable\":true}],\"range\":{\"start\":0,\"end\":206},\"returnType\":\"bool\",\"sourcePath\":\"/src/worker.ts\",\"symbol\":\"s2\"}],\"inputs\":[\"/src/app.ts\",\"/src/minimal-scalars-v2.d.ts\",\"/src/worker.ts\"],\"modules\":[{\"exports\":[{\"name\":\"entry\",\"range\":{\"start\":33,\"end\":98},\"target\":\"s1\"}],\"external\":true,\"globals\":[],\"imports\":[{\"bindings\":[{\"name\":\"probe\",\"range\":{\"start\":9,\"end\":14},\"target\":\"s2\"}],\"kind\":\"import\",\"range\":{\"start\":0,\"end\":33},\"sourcePath\":\"/src/worker.ts\",\"specifier\":\"./worker\"}],\"initialization\":[],\"sourcePath\":\"/src/app.ts\"},{\"exports\":[{\"name\":\"probe\",\"range\":{\"start\":0,\"end\":206},\"target\":\"s2\"}],\"external\":true,\"globals\":[],\"imports\":[],\"initialization\":[{\"exception\":{\"catchClause\":{\"annotationRange\":{\"start\":245,\"end\":253},\"binding\":{\"id\":\"s7\",\"kind\":\"local\",\"name\":\"initCaught\",\"range\":{\"start\":234,\"end\":244},\"sourcePath\":\"/src/worker.ts\",\"type\":\"unknown\",\"writable\":true},\"block\":{\"body\":[{\"condition\":{\"kind\":\"type-test\",\"operand\":{\"kind\":\"reference\",\"name\":\"initCaught\",\"range\":{\"start\":267,\"end\":278},\"symbol\":\"s7\",\"type\":\"unknown\"},\"operator\":\"===\",\"range\":{\"start\":261,\"end\":292},\"type\":\"bool\",\"typeTest\":{\"tag\":\"boolean\",\"tagRange\":{\"start\":282,\"end\":292},\"typeofRange\":{\"start\":261,\"end\":278}}},\"kind\":\"if\",\"range\":{\"start\":257,\"end\":315},\"thenBody\":[{\"body\":[{\"expression\":{\"kind\":\"reference\",\"name\":\"initCaught\",\"range\":{\"start\":301,\"end\":312},\"symbol\":\"s7\",\"type\":\"bool\"},\"kind\":\"throw\",\"range\":{\"start\":295,\"end\":313}}],\"kind\":\"block\",\"range\":{\"start\":293,\"end\":315}}]},{\"expression\":{\"kind\":\"reference\",\"name\":\"initCaught\",\"range\":{\"start\":321,\"end\":332},\"symbol\":\"s7\",\"type\":\"unknown\"},\"kind\":\"throw\",\"range\":{\"start\":315,\"end\":333}}],\"range\":{\"start\":254,\"end\":335}},\"declarationRange\":{\"start\":234,\"end\":253},\"range\":{\"start\":226,\"end\":335}},\"finallyBlock\":{\"body\":[],\"range\":{\"start\":343,\"end\":346}},\"tryBlock\":{\"body\":[{\"expression\":{\"boolean\":true,\"kind\":\"boolean\",\"range\":{\"start\":218,\"end\":223},\"type\":\"bool\"},\"kind\":\"throw\",\"range\":{\"start\":212,\"end\":224}}],\"range\":{\"start\":210,\"end\":226}}},\"kind\":\"try\",\"range\":{\"start\":206,\"end\":346}}],\"sourcePath\":\"/src/worker.ts\"}],\"status\":\"ok\",\"symbols\":[{\"id\":\"s1\",\"kind\":\"function\",\"name\":\"entry\",\"range\":{\"start\":49,\"end\":55},\"sourcePath\":\"/src/app.ts\",\"type\":\"bool\",\"writable\":false},{\"id\":\"s2\",\"kind\":\"function\",\"name\":\"probe\",\"range\":{\"start\":24,\"end\":30},\"sourcePath\":\"/src/worker.ts\",\"type\":\"bool\",\"writable\":false},{\"id\":\"s3\",\"kind\":\"parameter\",\"name\":\"value\",\"presence\":{\"annotation\":{\"start\":63,\"end\":66},\"default\":false,\"null\":false,\"optional\":false,\"range\":{\"start\":56,\"end\":66},\"undefined\":false},\"range\":{\"start\":56,\"end\":61},\"sourcePath\":\"/src/app.ts\",\"type\":\"i32\",\"writable\":true},{\"id\":\"s4\",\"kind\":\"parameter\",\"name\":\"value\",\"presence\":{\"annotation\":{\"start\":38,\"end\":41},\"default\":false,\"null\":false,\"optional\":false,\"range\":{\"start\":31,\"end\":41},\"undefined\":false},\"range\":{\"start\":31,\"end\":36},\"sourcePath\":\"/src/worker.ts\",\"type\":\"i32\",\"writable\":true},{\"id\":\"s5\",\"kind\":\"local\",\"name\":\"caught\",\"range\":{\"start\":79,\"end\":85},\"sourcePath\":\"/src/worker.ts\",\"type\":\"unknown\",\"writable\":true},{\"id\":\"s6\",\"kind\":\"local\",\"name\":\"copied\",\"range\":{\"start\":181,\"end\":188},\"sourcePath\":\"/src/worker.ts\",\"type\":\"i32\",\"writable\":false},{\"id\":\"s7\",\"kind\":\"local\",\"name\":\"initCaught\",\"range\":{\"start\":234,\"end\":244},\"sourcePath\":\"/src/worker.ts\",\"type\":\"unknown\",\"writable\":true}]}"
	if exceptionPackageExportsDigestForTest([]byte(golden)) != "sha256:055c3add3d592f06b25c3a01e7dfd4146e7c2c176cf5c79f13a03da4fae5ec0b" {
		t.Fatal("independent original module expected golden changed")
	}
	var facts any
	if err := json.Unmarshal([]byte(golden), &facts); err != nil {
		t.Fatal(err)
	}
	exceptionPackageExportsPacketsForTest(t, "moduleTerminal", r, input, facts, exportGroups)
}
