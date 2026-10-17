// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
)

// These sources exercise owned facts only. Abrupt finally bodies are preserved
// as source structure; this test does not execute exception or cleanup semantics.
func TestFrontendExceptionStructureBoundaries(t *testing.T) {
	cases := []struct {
		name, body, shape, code, phase, category, token string
		upstream                                        int32
	}{
		{name: "presentEmptyRegions", body: "try {} catch {} finally {} return value;", shape: "try([],catch:none:[],finally:[]);return:s2"},
		{name: "explicitUnknownAnnotation", body: "try { throw value; } catch (caught: unknown) { throw caught; }", shape: "try([throw:s2],catch:s3:[throw:s3],_)"},
		{name: "nestedSameNameCatches", body: "try { throw value; } catch (caught) { try { throw caught; } catch (caught) { throw caught; } }", shape: "try([throw:s2],catch:s3:[try([throw:s3],catch:s4:[throw:s4],_)],_)"},
		{name: "nestedFinallyAbruptStructure", body: "try { try { throw value; } finally { return value; } } finally { throw value; }", shape: "try([try([throw:s2],_,finally:[return:s2])],_,finally:[throw:s2])"},
		{name: "outsideCatchBinding", body: "try { throw value; } catch (caught) {} return caught;", code: "r01-d-unresolved-name", phase: "types", category: "error", token: "caught", upstream: 2304},
		{name: "illegalCatchAnnotation", body: "try { throw value; } catch (caught: i32) { throw caught; }", code: "r02b-d-type", phase: "types", category: "error", token: "i32", upstream: 1196},
		{name: "ordinaryBareLocalInCatch", body: "try { throw value; } catch (caught) { let bare; throw caught; }", code: "r02b-d-unsupported", phase: "capability", category: "unsupported", token: "bare"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := "/* 😀 */ export function entry(value: i32): i32 { " + tc.body + " }"
			r := integerRequest(source)
			requestBytes, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			// Retain complete original bytes even when an official prerequisite fails.
			t.Logf("TSFG_EXCEPTION_BOUNDARY_SOURCE_V1 %s", base64.StdEncoding.EncodeToString([]byte(source)))
			t.Logf("TSFG_EXCEPTION_BOUNDARY_REQUEST_V1 %s", base64.StdEncoding.EncodeToString(requestBytes))
			defer func() {
				after, marshalErr := json.Marshal(r)
				if marshalErr != nil || !bytes.Equal(requestBytes, after) {
					t.Fatal("exception boundary checks mutated the original request")
				}
			}()
			program := throwBoundaryProgramForTest(t, r)
			file := program.GetSourceFile(r.RootFile)
			var officialDiagnostic *ast.Diagnostic
			if !t.Run("officialFourDiagnosticGroups", func(t *testing.T) {
				groups := [][]*ast.Diagnostic{program.GetSyntacticDiagnostics(t.Context(), nil), program.GetSemanticDiagnostics(t.Context(), nil), program.GetProgramDiagnostics(), program.GetGlobalDiagnostics(t.Context())}
				for i, diagnostics := range groups {
					if i == 1 && tc.upstream != 0 {
						if len(diagnostics) != 1 || diagnostics[0].Code() != tc.upstream || diagnostics[0].File() != file {
							t.Fatalf("official semantic prerequisite differs: %+v", diagnostics)
						}
						officialDiagnostic = diagnostics[0]
					} else if len(diagnostics) != 0 {
						t.Fatalf("official diagnostic group %d must be empty: %+v", i, diagnostics)
					}
				}
			}) {
				return
			}
			var expected map[string]any
			var diagnosticRange span
			if !t.Run("officialASTSymbolsTypesUTF16", func(t *testing.T) {
				if file == nil || file.Text() != source || len(file.Statements.Nodes) != 1 || file.Statements.Nodes[0].Kind != ast.KindFunctionDeclaration {
					t.Fatal("official original function table changed")
				}
				fnNode := file.Statements.Nodes[0]
				fn := fnNode.AsFunctionDeclaration()
				if fn.Name() == nil || fn.Name().Text() != "entry" || fn.Type == nil || fn.TypeParameters != nil || len(fn.Parameters.Nodes) != 1 || fn.Body == nil || fn.Body.Kind != ast.KindBlock {
					t.Fatal("official typed entry shape changed")
				}
				parameter := fn.Parameters.Nodes[0]
				decl := parameter.AsParameterDeclaration()
				if parameter.Name() == nil || parameter.Name().Text() != "value" || decl.Type == nil || decl.QuestionToken != nil || decl.Initializer != nil || decl.DotDotDotToken != nil {
					t.Fatal("official required i32 parameter changed")
				}
				check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
				defer release()
				fnSymbol := check.GetSymbolAtLocation(fn.Name())
				parameterSymbol := check.GetSymbolAtLocation(parameter.Name())
				i32, unknown := check.GetTypeFromTypeNode(decl.Type), check.GetUnknownType()
				if fnSymbol == nil || parameterSymbol == nil || fnSymbol == parameterSymbol || len(fnSymbol.Declarations) != 1 || fnSymbol.Declarations[0] != fnNode || len(parameterSymbol.Declarations) != 1 || parameterSymbol.Declarations[0] != parameter || i32 == nil || i32.Alias() == nil || i32.Alias().Symbol() == nil || i32.Alias().Symbol().Name != "i32" || check.GetTypeFromTypeNode(fn.Type) != i32 || check.GetTypeOfSymbol(parameterSymbol) != i32 {
					t.Fatal("official declaration/symbol/nominal type identity changed")
				}
				exports := check.GetExportsOfModule(file.AsNode().Symbol())
				if len(exports) != 1 || exports[0] != fnSymbol || exports[0].Name != "entry" || len(exports[0].Declarations) != 1 || exports[0].Declarations[0] != fnNode {
					t.Fatal("complete official export set changed")
				}
				anchor := func(node *ast.Node) span {
					t.Helper()
					return throwBoundaryNodeForTest(t, file, node)
				}
				body := fn.Body.AsBlock().Statements.Nodes
				if tc.code != "" {
					if len(body) == 0 || body[0].Kind != ast.KindTryStatement {
						t.Fatal("negative source lost its original try")
					}
					attempt := body[0].AsTryStatement()
					if attempt.CatchClause == nil || attempt.CatchClause.AsCatchClause().VariableDeclaration == nil {
						t.Fatal("negative source lost its actual catch declaration")
					}
					catchNode := attempt.CatchClause.AsCatchClause().VariableDeclaration
					caught := catchNode.AsVariableDeclaration()
					if caught.Name() == nil || caught.Name().Text() != "caught" || check.GetSymbolAtLocation(caught.Name()) == nil {
						t.Fatal("negative source catch symbol missing")
					}
					anchor(body[0])
					anchor(attempt.TryBlock)
					anchor(attempt.CatchClause)
					anchor(catchNode)
					if officialDiagnostic != nil {
						diagnosticRange = throwBoundarySpanForTest(source, officialDiagnostic.Pos(), officialDiagnostic.End())
						if sourceText(t, source, diagnosticRange) != tc.token {
							t.Fatalf("official diagnostic lost the original %s token: %+v", tc.token, diagnosticRange)
						}
						if tc.upstream == 2304 {
							last := body[len(body)-1]
							if last.Kind != ast.KindReturnStatement || last.AsReturnStatement().Expression == nil || check.GetSymbolAtLocation(last.AsReturnStatement().Expression) != nil {
								t.Fatal("outside-catch use must remain unresolved by the official checker")
							}
							anchor(last.AsReturnStatement().Expression)
						} else if caught.Type == nil || check.GetTypeFromTypeNode(caught.Type) != i32 {
							t.Fatal("illegal annotation fixture lost its explicit nominal i32 type")
						}
					} else {
						catchBody := attempt.CatchClause.AsCatchClause().Block.AsBlock().Statements.Nodes
						if len(catchBody) != 2 || catchBody[0].Kind != ast.KindVariableStatement || catchBody[1].Kind != ast.KindThrowStatement {
							t.Fatal("bare local fixture lost its original catch body")
						}
						declarations := catchBody[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
						if len(declarations) != 1 || declarations[0].Kind != ast.KindVariableDeclaration {
							t.Fatal("one bare local declaration required")
						}
						bare := declarations[0].AsVariableDeclaration()
						if bare.Name() == nil || bare.Name().Text() != "bare" || bare.Type != nil || bare.Initializer != nil || check.GetSymbolAtLocation(bare.Name()) == nil || check.GetTypeOfSymbol(check.GetSymbolAtLocation(caught.Name())) != unknown {
							t.Fatal("ordinary bare local must remain unannotated inside an unknown catch")
						}
						raw := anchor(declarations[0])
						at := strings.Index(source, "bare;")
						if at < 0 {
							t.Fatal("original bare declaration preimage missing")
						}
						diagnosticRange = throwBoundarySpanForTest(source, at, at+len("bare"))
						if at < 0 || raw.End != diagnosticRange.End || raw.Start > diagnosticRange.Start {
							t.Fatal("independent bare declaration diagnostic preimage changed")
						}
					}
					return
				}
				// Only the four literal success grammars above are translated by this
				// independent oracle; the fixed shape string checks their complete AST.
				definition := map[string]any{"id": "s1", "name": "entry", "kind": "function", "type": "i32", "writable": false, "range": anchor(fn.Name())}
				parameterStart := strings.Index(source, "value: i32")
				annotation := throwBoundarySpanForTest(source, parameterStart+len("value: "), parameterStart+len("value: i32"))
				if sourceText(t, source, anchor(decl.Type)) != "i32" {
					t.Fatal("original parameter annotation spelling changed")
				}
				ownedParameter := map[string]any{"id": "s2", "name": "value", "kind": "parameter", "type": "i32", "writable": true, "range": anchor(parameter.Name()),
					"presence": map[string]any{"undefined": false, "null": false, "optional": false, "default": false, "range": anchor(parameter), "annotation": annotation}}
				symbols := []any{definition, ownedParameter}
				bindings := map[*ast.Symbol]string{parameterSymbol: "s2"}
				var statements func([]*ast.Node) ([]any, string)
				statements = func(nodes []*ast.Node) ([]any, string) {
					out, shapes := []any{}, []string{}
					for _, node := range nodes {
						item := map[string]any{"range": anchor(node)}
						switch node.Kind {
						case ast.KindThrowStatement, ast.KindReturnStatement:
							kind := "throw"
							var value *ast.Node
							if node.Kind == ast.KindThrowStatement {
								value = node.AsThrowStatement().Expression
							} else {
								kind, value = "return", node.AsReturnStatement().Expression
							}
							if value == nil || value.Kind != ast.KindIdentifier {
								t.Fatal("literal boundary expects one identifier expression")
							}
							actual := check.GetSymbolAtLocation(value)
							id, typ := bindings[actual], "i32"
							if id == "" || len(actual.Declarations) != 1 {
								t.Fatal("official reference escaped its actual defining binding")
							}
							if actual != parameterSymbol {
								typ = "unknown"
								if check.GetTypeAtLocation(value) != unknown || check.GetTypeOfSymbol(actual) != unknown {
									t.Fatal("catch occurrence must retain the actual official unknown type")
								}
							} else if check.GetTypeAtLocation(value) != i32 {
								t.Fatal("parameter occurrence lost nominal i32 identity")
							}
							item["kind"], item["expression"] = kind, map[string]any{"kind": "reference", "type": typ, "symbol": id, "range": anchor(value)}
							shapes = append(shapes, kind+":"+id)
						case ast.KindTryStatement:
							attempt := node.AsTryStatement()
							if attempt.TryBlock == nil || attempt.TryBlock.Kind != ast.KindBlock {
								t.Fatal("official try block missing")
							}
							tryBody, tryShape := statements(attempt.TryBlock.AsBlock().Statements.Nodes)
							exception := map[string]any{"tryBlock": map[string]any{"range": anchor(attempt.TryBlock), "body": tryBody}}
							catchShape, finallyShape := "_", "_"
							if attempt.CatchClause != nil {
								clause := attempt.CatchClause.AsCatchClause()
								if clause.Block == nil || clause.Block.Kind != ast.KindBlock {
									t.Fatal("official catch block missing")
								}
								caught := map[string]any{"range": anchor(attempt.CatchClause)}
								id := "none"
								if clause.VariableDeclaration != nil {
									declared := clause.VariableDeclaration.AsVariableDeclaration()
									actual := check.GetSymbolAtLocation(declared.Name())
									if declared.Name() == nil || declared.Name().Text() != "caught" || declared.Initializer != nil || actual == nil || actual == fnSymbol || bindings[actual] != "" || len(actual.Declarations) != 1 || actual.Declarations[0] != clause.VariableDeclaration || check.GetTypeOfSymbol(actual) != unknown || check.GetTypeAtLocation(declared.Name()) != unknown {
										t.Fatal("each catch must have a distinct actual unknown declaration symbol")
									}
									id = fmt.Sprintf("s%d", len(symbols)+1)
									binding := map[string]any{"id": id, "name": "caught", "kind": "local", "type": "unknown", "writable": true, "range": anchor(declared.Name())}
									bindings[actual] = id
									symbols = append(symbols, binding)
									caught["binding"], caught["declarationRange"] = binding, anchor(clause.VariableDeclaration)
									if declared.Type != nil {
										if check.GetTypeFromTypeNode(declared.Type) != unknown || sourceText(t, source, anchor(declared.Type)) != "unknown" {
											t.Fatal("explicit unknown annotation lost its official type/range")
										}
										caught["annotationRange"] = anchor(declared.Type)
									}
								}
								catchBody, innerShape := statements(clause.Block.AsBlock().Statements.Nodes)
								caught["block"] = map[string]any{"range": anchor(clause.Block), "body": catchBody}
								exception["catchClause"], catchShape = caught, "catch:"+id+":["+innerShape+"]"
							}
							if attempt.FinallyBlock != nil {
								if attempt.FinallyBlock.Kind != ast.KindBlock {
									t.Fatal("official finally block kind changed")
								}
								finallyBody, innerShape := statements(attempt.FinallyBlock.AsBlock().Statements.Nodes)
								exception["finallyBlock"], finallyShape = map[string]any{"range": anchor(attempt.FinallyBlock), "body": finallyBody}, "finally:["+innerShape+"]"
							}
							item["kind"], item["exception"] = "try", exception
							shapes = append(shapes, "try(["+tryShape+"],"+catchShape+","+finallyShape+")")
						default:
							t.Fatalf("literal success boundary has unexpected official statement kind %v", node.Kind)
						}
						out = append(out, item)
					}
					return out, strings.Join(shapes, ";")
				}
				ownedBody, shape := statements(body)
				if shape != tc.shape {
					t.Fatalf("official AST differs from independently fixed source shape: got=%s want=%s", shape, tc.shape)
				}
				expected = map[string]any{"status": "ok", "inputs": []string{"/src/main.ts", "/src/minimal-integers-v3.d.ts"}, "symbols": symbols,
					"functions": []any{map[string]any{"name": "entry", "symbol": "s1", "parameters": []any{ownedParameter}, "returnType": "i32", "parameterInitializers": []any{nil}, "range": anchor(fnNode), "body": ownedBody}}}
			}) {
				return
			}
			expectedBytes, err := json.Marshal(map[string]any{"response": expected, "diagnosticRange": diagnosticRange, "upstream": tc.upstream})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("TSFG_EXCEPTION_BOUNDARY_OFFICIAL_ORACLE_V1 %s", base64.StdEncoding.EncodeToString(expectedBytes))
			t.Run("publicProjection", func(t *testing.T) {
				var output bytes.Buffer
				commandErr := runFrontendCommand(t.Context(), nil, bytes.NewReader(requestBytes), &output)
				raw := bytes.Clone(output.Bytes())
				// Complete actual bytes precede command, JSON and semantic assertions.
				t.Logf("TSFG_EXCEPTION_BOUNDARY_RESPONSE_V1 %s", base64.StdEncoding.EncodeToString(raw))
				if commandErr != nil {
					t.Fatalf("public boundary command failed: %v; raw=%s", commandErr, raw)
				}
				if tc.code == "" {
					wantBytes, err := json.Marshal(expected)
					if err != nil {
						t.Fatal(err)
					}
					var got, want any
					if err := json.Unmarshal(raw, &got); err != nil {
						t.Fatalf("public success is not JSON: %v; raw=%s", err, raw)
					}
					if err := json.Unmarshal(wantBytes, &want); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("complete owned structure differs from official/source oracle: got=%s want=%s", raw, wantBytes)
					}
					return
				}
				var result response
				if err := json.Unmarshal(raw, &result); err != nil {
					t.Fatalf("public diagnostic is not JSON: %v; raw=%s", err, raw)
				}
				if result.Status != "error" || len(result.Functions) != 0 || len(result.Diagnostics) != 1 {
					t.Fatalf("expected one exact source rejection: %+v; raw=%s", result, raw)
				}
				d := result.Diagnostics[0]
				if d.Code != tc.code || d.UpstreamCode != tc.upstream || d.Phase != tc.phase || d.Category != tc.category || d.Severity != "error" || d.RuleID != "r01-ts-baseline" || !reflect.DeepEqual(d.RuleIDs, []string{"r01-ts-baseline"}) || d.File != r.RootFile || d.Range != diagnosticRange || d.Primary.SourceID != r.RootFile || d.Primary.Start != diagnosticRange.Start || d.Primary.End != diagnosticRange.End {
					t.Fatalf("source diagnostic lost exact code/layer/original UTF16 anchor: %+v; raw=%s", d, raw)
				}
				if sourceText(t, source, diagnosticRange) != tc.token {
					t.Fatal("public diagnostic range lost its original source token")
				}
			})
		})
	}
}
