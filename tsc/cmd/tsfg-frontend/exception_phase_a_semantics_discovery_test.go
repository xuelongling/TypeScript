// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/scanner"
)

// Independent small-input discovery. No public projection, private command, executable or
// response-derived expectation. Flow occurrence types are measured, not assumed.
func TestExceptionPhaseAIndependentSemanticsOracleRev2(t *testing.T) {
	cases := []struct {
		name, source, parameterName         string
		syntaxInvalid, diagnosticsDiscovery bool
	}{
		{name: "unicodeActualAstralControl", source: "// 😀\r\nexport function entry(𝐀: i32): i32 {\r\n  try {\r\n    throw 𝐀;\r\n  } catch (caught) {\r\n    throw caught;\r\n  } finally {\r\n    const copied: i32 = 𝐀;\r\n  }\r\n}", parameterName: "𝐀", syntaxInvalid: false, diagnosticsDiscovery: false},
		{name: "unicodeBracedEscapeControl", source: "// 😀\r\nexport function entry(\\u{1D400}: i32): i32 {\r\n  try {\r\n    throw \\u{1D400};\r\n  } catch (caught) {\r\n    throw caught;\r\n  } finally {\r\n    const copied: i32 = \\u{1D400};\r\n  }\r\n}", parameterName: "𝐀", syntaxInvalid: false, diagnosticsDiscovery: false},
		{name: "unicodeSurrogateEscapeCandidate", source: "// 😀\r\nexport function entry(\\uD835\\uDC00: i32): i32 {\r\n  try {\r\n    throw \\uD835\\uDC00;\r\n  } catch (caught) {\r\n    throw caught;\r\n  } finally {\r\n    const copied: i32 = \\uD835\\uDC00;\r\n  }\r\n}", parameterName: "𝐀", syntaxInvalid: true, diagnosticsDiscovery: false},
		{name: "guardOriginalActualControl", source: "// 😀\r\nexport function entry(value: i32): i32 {\r\n  try { throw value; } catch (caught) {\r\n    if (typeof caught === \"boolean\") { throw caught; }\r\n    throw caught;\r\n  }\r\n}", parameterName: "value", syntaxInvalid: false, diagnosticsDiscovery: false},
		{name: "guardContradictionCandidate", source: "// 😀\r\nexport function entry(value: i32): i32 {\r\n  try { throw value; } catch (caught) {\r\n    if (typeof caught === \"boolean\") {\r\n      if (typeof caught !== \"boolean\") { throw caught; }\r\n      throw caught;\r\n    }\r\n    throw caught;\r\n  }\r\n}", parameterName: "value", syntaxInvalid: false, diagnosticsDiscovery: false},
		{name: "finallyGuardBeforeControl", source: "// 😀\r\nexport function entry(value: i32): i32 {\r\n  try { throw value; } catch (caught) {\r\n    if (typeof caught !== \"boolean\") { throw value; }\r\n    try { } finally { }\r\n    throw caught;\r\n  }\r\n}", parameterName: "value", syntaxInvalid: false, diagnosticsDiscovery: false},
		{name: "finallyGuardInsideCandidate", source: "// 😀\r\nexport function entry(value: i32): i32 {\r\n  try { throw value; } catch (caught) {\r\n    try {\r\n      if (typeof caught !== \"boolean\") { throw value; }\r\n    } finally { }\r\n    throw caught;\r\n  }\r\n}", parameterName: "value", syntaxInvalid: false, diagnosticsDiscovery: false},
		{name: "intrinsicUndefinedScriptDiscovery", source: "// 😀\r\nfunction undefined(value: i32): i32 {\r\n  try {\r\n    throw value;\r\n  } catch (caught) {\r\n    throw caught;\r\n  } finally {\r\n    const copied: i32 = value;\r\n  }\r\n}", parameterName: "value", syntaxInvalid: false, diagnosticsDiscovery: true},
		{name: "intrinsicUndefinedModuleControl", source: "// 😀\r\nexport function undefined(value: i32): i32 {\r\n  try {\r\n    throw value;\r\n  } catch (caught) {\r\n    throw caught;\r\n  } finally {\r\n    const copied: i32 = value;\r\n  }\r\n}", parameterName: "value", syntaxInvalid: false, diagnosticsDiscovery: true},
		{name: "intrinsicGlobalThisScriptDiscovery", source: "// 😀\r\nfunction globalThis(value: i32): i32 {\r\n  try {\r\n    throw value;\r\n  } catch (caught) {\r\n    throw caught;\r\n  } finally {\r\n    const copied: i32 = value;\r\n  }\r\n}", parameterName: "value", syntaxInvalid: false, diagnosticsDiscovery: true},
		{name: "intrinsicGlobalThisModuleControl", source: "// 😀\r\nexport function globalThis(value: i32): i32 {\r\n  try {\r\n    throw value;\r\n  } catch (caught) {\r\n    throw caught;\r\n  } finally {\r\n    const copied: i32 = value;\r\n  }\r\n}", parameterName: "value", syntaxInvalid: false, diagnosticsDiscovery: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.source) >= 1024 {
				t.Fatal("bounded literal exceeds 1KiB")
			}
			r := integerRequest(tc.source)
			program := throwBoundaryProgramForTest(t, r)
			file := program.GetSourceFile(r.RootFile)
			groups := []struct {
				name   string
				values []*ast.Diagnostic
			}{
				{"syntax", program.GetSyntacticDiagnostics(t.Context(), nil)},
				{"semantic", program.GetSemanticDiagnostics(t.Context(), nil)},
				{"program", program.GetProgramDiagnostics()},
				{"global", program.GetGlobalDiagnostics(t.Context())},
			}
			diagnostics := []map[string]any{}
			for _, group := range groups {
				for _, diagnostic := range group.values {
					item := map[string]any{"group": group.name, "code": diagnostic.Code(), "utf8": [2]int{diagnostic.Pos(), diagnostic.End()}}
					if origin := diagnostic.File(); origin != nil {
						item["file"] = origin.FileName()
						if diagnostic.Pos() >= 0 && diagnostic.End() >= diagnostic.Pos() && diagnostic.End() <= len(origin.Text()) {
							item["utf16"] = [2]int{len(utf16.Encode([]rune(origin.Text()[:diagnostic.Pos()]))), len(utf16.Encode([]rune(origin.Text()[:diagnostic.End()])))}
						}
					}
					diagnostics = append(diagnostics, item)
				}
			}
			sum := sha256.Sum256([]byte(tc.source))
			record := map[string]any{"name": tc.name, "source": tc.source, "sourceSha256": hex.EncodeToString(sum[:]), "bytes": len(tc.source),
				"utf16Length": len(utf16.Encode([]rune(tc.source))), "diagnostics": diagnostics}
			if !tc.syntaxInvalid && len(diagnostics) == 0 {
				if file == nil || file.Text() != tc.source || len(file.Statements.Nodes) != 1 || file.Statements.Nodes[0].Kind != ast.KindFunctionDeclaration {
					t.Fatal("independent literal function AST missing")
				}
				consumed := []string{}
				for _, source := range program.GetSourceFiles() {
					consumed = append(consumed, source.FileName())
				}
				slices.Sort(consumed)
				if !slices.Equal(consumed, []string{"/src/main.ts", "/src/minimal-integers-v3.d.ts"}) {
					t.Fatal("fresh complete official input closure differs")
				}
				node := file.Statements.Nodes[0]
				fn := node.AsFunctionDeclaration()
				if len(fn.Parameters.Nodes) != 1 || fn.Parameters.Nodes[0].Name().Text() != tc.parameterName || fn.Body == nil {
					t.Fatal("literal parameter/body missing")
				}
				parameter := fn.Parameters.Nodes[0]
				body := fn.Body.AsBlock().Statements.Nodes
				if len(body) != 1 || body[0].Kind != ast.KindTryStatement || body[0].AsTryStatement().CatchClause == nil {
					t.Fatal("outer catch AST missing")
				}
				caughtNode := body[0].AsTryStatement().CatchClause.AsCatchClause().VariableDeclaration
				if caughtNode == nil || caughtNode.Name() == nil || caughtNode.Name().Text() != "caught" {
					t.Fatal("literal catch declaration missing")
				}
				check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
				defer release()
				parameterSymbol := check.GetSymbolAtLocation(parameter.Name())
				caughtSymbol := check.GetSymbolAtLocation(caughtNode.Name())
				if parameterSymbol == nil || caughtSymbol == nil || parameterSymbol == caughtSymbol ||
					len(parameterSymbol.Declarations) != 1 || parameterSymbol.Declarations[0] != parameter ||
					len(caughtSymbol.Declarations) != 1 || caughtSymbol.Declarations[0] != caughtNode {
					t.Fatal("official original binding identity differs")
				}
				anchors, occurrences := []map[string]any{}, []map[string]any{}
				positions := func(start, end int) [2]int {
					got := [2]int{len(utf16.Encode([]rune(tc.source[:start]))), len(utf16.Encode([]rune(tc.source[:end])))}
					official := file.GetPositionMap()
					if got != ([2]int{official.UTF8ToUTF16(start), official.UTF8ToUTF16(end)}) {
						t.Fatal("independent UTF16 prefix differs")
					}
					return got
				}
				var visit func(*ast.Node) bool
				visit = func(item *ast.Node) bool {
					if item == nil {
						return false
					}
					if item.Pos() >= 0 && item.End() >= item.Pos() && item.End() <= len(tc.source) {
						trim := scanner.SkipTrivia(tc.source, item.Pos())
						anchors = append(anchors, map[string]any{"kind": int(item.Kind), "rawUTF8": [2]int{item.Pos(), item.End()},
							"rawUTF16": positions(item.Pos(), item.End()), "trimUTF16": positions(trim, item.End()), "raw": tc.source[item.Pos():item.End()]})
						if item.Kind == ast.KindIdentifier && (item.Text() == tc.parameterName || item.Text() == "caught") {
							actual, typ := check.GetSymbolAtLocation(item), check.GetTypeAtLocation(item)
							if actual == nil || typ == nil {
								t.Fatal("actual occurrence symbol/type missing")
							}
							occurrences = append(occurrences, map[string]any{"name": item.Text(), "rawUTF16": positions(item.Pos(), item.End()), "trimUTF16": positions(trim, item.End()),
								"flags": uint64(typ.Flags()), "isUnknown": typ.Flags() == checker.TypeFlagsUnknown,
								"isBooleanLike": typ.Flags()&checker.TypeFlagsBooleanLike != 0, "isNever": typ.Flags()&checker.TypeFlagsNever != 0,
								"sameParameter": actual == parameterSymbol, "sameCatch": actual == caughtSymbol,
								"isParameterDeclaration": item == parameter.Name(), "isCatchDeclaration": item == caughtNode.Name()})
						}
					}
					item.ForEachChild(visit)
					return false
				}
				visit(node)
				record["anchors"], record["occurrences"], record["consumed"] = anchors, occurrences, consumed
				record["catchDeclarationFlags"] = uint64(check.GetTypeOfSymbol(caughtSymbol).Flags())
			}
			encoded, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("TSFG_EXCEPTION_PHASE_A_DISCOVERY %s", encoded)
			if tc.syntaxInvalid {
				if len(groups[0].values) == 0 {
					t.Fatal("surrogate-escape syntax hypothesis disproved; do not freeze Product negative expectation")
				}
			} else if !tc.diagnosticsDiscovery && len(diagnostics) != 0 {
				t.Fatal("literal official control/flow discovery has diagnostics; do not freeze inferred occurrence types")
			}
		})
	}
}
