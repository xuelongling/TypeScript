// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
)

// Independently prepared literals; this installed official oracle isolates
// script-vs-module global binding using the same complete official DTS input.
func TestExceptionColdGlobalDTSBindingOfficialOracle(t *testing.T) {
	cases := []struct {
		name      string
		source    string
		wantClean bool
	}{
		{
			name:      "scriptDistinctNameControl",
			source:    "// 😀\r\nfunction entry(value: i32): i32 {\r\n  try {\r\n    throw value;\r\n  } catch (caught) {\r\n    throw caught;\r\n  } finally {\r\n    const copied: i32 = value;\r\n  }\r\n}",
			wantClean: true,
		},
		{
			name:      "exportedSameSpellingControl",
			source:    "// 😀\r\nexport function __tsfg_i32(value: i32): i32 {\r\n  try {\r\n    throw value;\r\n  } catch (caught) {\r\n    throw caught;\r\n  } finally {\r\n    const copied: i32 = value;\r\n  }\r\n}",
			wantClean: true,
		},
		{
			name:   "scriptConflictsWithDTSValue",
			source: "// 😀\r\nfunction __tsfg_i32(value: i32): i32 {\r\n  try {\r\n    throw value;\r\n  } catch (caught) {\r\n    throw caught;\r\n  } finally {\r\n    const copied: i32 = value;\r\n  }\r\n}",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := integerRequest(tc.source)
			program := throwBoundaryProgramForTest(t, r)
			file, dts := program.GetSourceFile(r.RootFile), program.GetSourceFile(r.Declarations.Paths[0])
			if dts == nil || file == dts || dts.FileName() != "/src/minimal-integers-v3.d.ts" {
				t.Fatal("actual distinct consumed declaration source missing")
			}
			groups := []struct {
				name string
				list []*ast.Diagnostic
			}{
				{"syntax", program.GetSyntacticDiagnostics(t.Context(), nil)},
				{"semantic", program.GetSemanticDiagnostics(t.Context(), nil)},
				{"program", program.GetProgramDiagnostics()},
				{"global", program.GetGlobalDiagnostics(t.Context())},
			}
			var diagnostics []map[string]any
			for _, group := range groups {
				for _, diagnostic := range group.list {
					item := map[string]any{
						"group": group.name, "code": diagnostic.Code(),
						"utf8": [2]int{diagnostic.Pos(), diagnostic.End()},
					}
					if origin := diagnostic.File(); origin != nil {
						item["file"] = origin.FileName()
						if diagnostic.Pos() >= 0 && diagnostic.End() >= diagnostic.Pos() && diagnostic.End() <= len(origin.Text()) {
							item["utf16"] = [2]int{
								len(utf16.Encode([]rune(origin.Text()[:diagnostic.Pos()]))),
								len(utf16.Encode([]rune(origin.Text()[:diagnostic.End()]))),
							}
						}
					}
					diagnostics = append(diagnostics, item)
				}
			}
			if len(file.Statements.Nodes) != 1 || file.Statements.Nodes[0].Kind != ast.KindFunctionDeclaration {
				t.Fatal("literal root function AST missing")
			}
			fnNode := file.Statements.Nodes[0]
			fn := fnNode.AsFunctionDeclaration()
			var brandNode *ast.Node
			for _, item := range dts.Statements.Nodes {
				if item.Kind != ast.KindVariableStatement {
					continue
				}
				list := item.AsVariableStatement().DeclarationList
				if list.Flags&ast.NodeFlagsConst == 0 {
					continue
				}
				for _, declaration := range list.AsVariableDeclarationList().Declarations.Nodes {
					if declaration.Name() != nil && declaration.Name().Text() == "__tsfg_i32" {
						if brandNode != nil {
							t.Fatal("duplicate official brand declaration")
						}
						brandNode = declaration
					}
				}
			}
			if brandNode == nil || brandNode.AsVariableDeclaration().Type == nil {
				t.Fatal("actual typed ambient const declaration missing")
			}
			check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
			rootSymbol := check.GetSymbolAtLocation(fn.Name())
			brandSymbol := check.GetSymbolAtLocation(brandNode.Name())
			if rootSymbol == nil || brandSymbol == nil || brandSymbol.Flags&ast.SymbolFlagsBlockScopedVariable == 0 {
				release()
				t.Fatal("real root/ambient value symbols missing")
			}
			describe := func(symbol *ast.Symbol) map[string]any {
				declarations := []map[string]any{}
				for _, declaration := range symbol.Declarations {
					declarations = append(declarations, map[string]any{
						"file":               ast.GetSourceFileOfNode(declaration).FileName(),
						"kind":               int(declaration.Kind),
						"utf8":               [2]int{declaration.Pos(), declaration.End()},
						"isRootFunctionNode": declaration == fnNode,
						"isDTSConstNode":     declaration == brandNode,
					})
				}
				return map[string]any{"name": symbol.Name, "flags": uint64(symbol.Flags), "declarations": declarations}
			}
			rootIdentity, brandIdentity := describe(rootSymbol), describe(brandSymbol)
			ownsBrand := false
			for _, declaration := range brandSymbol.Declarations {
				ownsBrand = ownsBrand || declaration == brandNode
			}
			sameSymbol := rootSymbol == brandSymbol
			release()
			if !ownsBrand {
				t.Fatal("ambient value symbol must own the actual pinned-DTS const node")
			}
			sourceSum, dtsSum := sha256.Sum256([]byte(tc.source)), sha256.Sum256([]byte(dts.Text()))
			record := map[string]any{
				"name": tc.name, "source": tc.source, "sourceSha256": hex.EncodeToString(sourceSum[:]),
				"bytes": len(tc.source), "utf16Length": len(utf16.Encode([]rune(tc.source))),
				"dtsPath": dts.FileName(), "dtsSha256": hex.EncodeToString(dtsSum[:]),
				"diagnostics": diagnostics, "rootSymbol": rootIdentity, "dtsSymbol": brandIdentity,
				"sameSymbol": sameSymbol, "dtsConstOwnsActualNode": ownsBrand,
				"rootFunctionName": fn.Name().Text(), "actualDTSConstName": brandNode.Name().Text(),
			}
			encoded, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("EXCEPTION_COLD_GLOBAL_DTS_ORACLE %s", encoded)
			if len(groups[0].list) != 0 {
				t.Fatal("all literal variants must remain syntax-clean")
			}
			if tc.wantClean != (len(diagnostics) == 0) {
				t.Fatalf("official clean=%v; required clean=%v", len(diagnostics) == 0, tc.wantClean)
			}
			if tc.wantClean && sameSymbol {
				t.Fatal("control declaration must not be the global ambient value symbol")
			}
		})
	}
}
