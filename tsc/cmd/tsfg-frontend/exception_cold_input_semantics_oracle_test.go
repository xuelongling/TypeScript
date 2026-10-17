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

// Scratch preparation only. Install only after Root release; this file has not
// been compiled or run. Every case below owns literal text, independent of any
// public/private response. Existing test helpers supply only a fresh official
// Program and the pinned minimal-integers declarations, never projection output.
func TestExceptionColdInputOfficialSemanticsOracle(t *testing.T) {
	cases := []struct {
		name       string
		source     string
		wantClean  bool
		wantSyntax bool
		wantLocal  bool
		inspect    bool
	}{
		{
			name:      "control",
			source:    "// 😀\r\nexport function entry(value: i32): i32 {\r\n  try {\r\n    throw value;\r\n  } catch (caught) {\r\n    throw caught;\r\n  } finally {\r\n    const copied: i32 = value;\r\n  }\r\n}",
			wantClean: true, wantSyntax: true, inspect: true,
		},
		{
			name:   "parameterReservedCatch",
			source: "// 😀\r\nexport function entry(catch: i32): i32 {\r\n  try {\r\n    throw catch;\r\n  } catch (caught) {\r\n    throw caught;\r\n  } finally {\r\n    const copied: i32 = catch;\r\n  }\r\n}",
		},
		{
			name:   "functionReservedCatch",
			source: "// 😀\r\nexport function catch(value: i32): i32 {\r\n  try {\r\n    throw value;\r\n  } catch (caught) {\r\n    throw caught;\r\n  } finally {\r\n    const copied: i32 = value;\r\n  }\r\n}",
		},
		{
			name:   "localReservedReturn",
			source: "// 😀\r\nexport function entry(value: i32): i32 {\r\n  try {\r\n    throw value;\r\n  } catch (caught) {\r\n    throw caught;\r\n  } finally {\r\n    const return: i32 = value;\r\n  }\r\n}",
		},
		{
			name:       "constSelfShadowTDZ",
			source:     "// 😀\r\nexport function entry(value: i32): i32 {\r\n  try {\r\n    throw value;\r\n  } catch (caught) {\r\n    throw caught;\r\n  } finally {\r\n    const value: i32 = value;\r\n  }\r\n}",
			wantSyntax: true, wantLocal: true, inspect: true,
		},
		{
			name:       "nonVoidNormalCompletion",
			source:     "// 😀\r\nexport function entry(value: i32): i32 {\r\n  try {\r\n    throw value;\r\n  } catch (caught) {\r\n    \r\n  } finally {\r\n    const copied: i32 = value;\r\n  }\r\n}",
			wantSyntax: true, inspect: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := integerRequest(tc.source)
			program := throwBoundaryProgramForTest(t, r)
			file := program.GetSourceFile(r.RootFile)
			groups := []struct {
				name string
				list []*ast.Diagnostic
			}{
				{"syntax", program.GetSyntacticDiagnostics(t.Context(), nil)},
				{"semantic", program.GetSemanticDiagnostics(t.Context(), nil)},
				{"program", program.GetProgramDiagnostics()},
				{"global", program.GetGlobalDiagnostics(t.Context())},
			}
			sum := sha256.Sum256([]byte(tc.source))
			record := exceptionColdOfficialRecord{
				Name: tc.name, Source: tc.source, SourceSHA256: hex.EncodeToString(sum[:]),
				Bytes: len(tc.source), UTF16: len(utf16.Encode([]rune(tc.source))),
				Diagnostics: []exceptionColdOfficialDiagnostic{},
			}
			for _, group := range groups {
				for _, diagnostic := range group.list {
					item := exceptionColdOfficialDiagnostic{
						Group: group.name, Code: diagnostic.Code(),
						UTF8: [2]int{diagnostic.Pos(), diagnostic.End()},
					}
					if dfile := diagnostic.File(); dfile != nil {
						item.File = dfile.FileName()
						if diagnostic.Pos() >= 0 && diagnostic.End() >= diagnostic.Pos() && diagnostic.End() <= len(dfile.Text()) {
							positions := [2]int{
								len(utf16.Encode([]rune(dfile.Text()[:diagnostic.Pos()]))),
								len(utf16.Encode([]rune(dfile.Text()[:diagnostic.End()]))),
							}
							item.UTF16 = &positions
						}
					}
					record.Diagnostics = append(record.Diagnostics, item)
				}
			}
			if tc.inspect {
				if len(file.Statements.Nodes) != 1 || file.Statements.Nodes[0].Kind != ast.KindFunctionDeclaration {
					t.Fatal("literal source lost its single function AST")
				}
				fn := file.Statements.Nodes[0].AsFunctionDeclaration()
				if len(fn.Parameters.Nodes) != 1 || fn.Body == nil {
					t.Fatal("official parameter/body missing")
				}
				parameter := fn.Parameters.Nodes[0]
				body := fn.Body.AsBlock().Statements.Nodes
				if len(body) != 1 || body[0].Kind != ast.KindTryStatement {
					t.Fatal("official single try statement missing")
				}
				attempt := body[0].AsTryStatement()
				if attempt.FinallyBlock == nil || attempt.CatchClause == nil {
					t.Fatal("official catch/finally missing")
				}
				finally := attempt.FinallyBlock.AsBlock().Statements.Nodes
				if len(finally) != 1 || finally[0].Kind != ast.KindVariableStatement {
					t.Fatal("official finally-local declaration missing")
				}
				declarations := finally[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
				if len(declarations) != 1 || declarations[0].Kind != ast.KindVariableDeclaration {
					t.Fatal("official single local declaration missing")
				}
				localNode := declarations[0]
				local := localNode.AsVariableDeclaration()
				if local.Name() == nil || local.Initializer == nil || local.Initializer.Kind != ast.KindIdentifier {
					t.Fatal("official local identifier/initializer missing")
				}
				check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
				outer := check.GetSymbolAtLocation(parameter.Name())
				inner := check.GetSymbolAtLocation(local.Name())
				use := check.GetSymbolAtLocation(local.Initializer)
				if outer == nil || inner == nil || use == nil || outer == inner {
					release()
					t.Fatal("official outer/inner/use must have distinct real declarations")
				}
				identity := &exceptionColdOfficialIdentity{
					ParameterName: outer.Name, LocalName: inner.Name,
					InitializerName:    local.Initializer.Text(),
					InitializerIsLocal: use == inner, InitializerIsParameter: use == outer,
					ParameterDeclarationIsNode: len(outer.Declarations) == 1 && outer.Declarations[0] == parameter,
					LocalDeclarationIsNode:     len(inner.Declarations) == 1 && inner.Declarations[0] == localNode,
					ParameterUTF8:              [2]int{parameter.Name().Pos(), parameter.Name().End()},
					LocalUTF8:                  [2]int{local.Name().Pos(), local.Name().End()},
					UseUTF8:                    [2]int{local.Initializer.Pos(), local.Initializer.End()},
				}
				release()
				record.Identity = identity
			}
			encoded, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("EXCEPTION_COLD_OFFICIAL_ORACLE %s", encoded)
			if tc.name == "control" && (record.Bytes != 172 || record.UTF16 != 170 || record.SourceSHA256 != "4c5cfdea7fa418d1b0eaeda99b01cb1e749266ecd8ecb9c0154df4cb7219c8c2") {
				t.Fatal("independent original control identity changed")
			}
			if tc.wantClean != (len(record.Diagnostics) == 0) {
				t.Fatalf("official diagnostics clean=%v; required clean=%v", len(record.Diagnostics) == 0, tc.wantClean)
			}
			if tc.wantSyntax && len(groups[0].list) != 0 {
				t.Fatal("semantic-only candidate unexpectedly has syntax diagnostics")
			}
			if tc.inspect {
				identity := record.Identity
				if !identity.ParameterDeclarationIsNode || !identity.LocalDeclarationIsNode || identity.InitializerIsLocal != tc.wantLocal || identity.InitializerIsParameter == tc.wantLocal {
					t.Fatal("official lexical identity differs from independently specified shadowing rule")
				}
			}
		})
	}
}

type exceptionColdOfficialRecord struct {
	Name         string                            `json:"name"`
	Source       string                            `json:"source"`
	SourceSHA256 string                            `json:"sourceSha256"`
	Bytes        int                               `json:"bytes"`
	UTF16        int                               `json:"utf16Length"`
	Diagnostics  []exceptionColdOfficialDiagnostic `json:"diagnostics"`
	Identity     *exceptionColdOfficialIdentity    `json:"identity,omitempty"`
}

type exceptionColdOfficialDiagnostic struct {
	Group string  `json:"group"`
	Code  int32   `json:"code"`
	File  string  `json:"file,omitempty"`
	UTF8  [2]int  `json:"utf8"`
	UTF16 *[2]int `json:"utf16,omitempty"`
}

type exceptionColdOfficialIdentity struct {
	ParameterName              string `json:"parameterName"`
	LocalName                  string `json:"localName"`
	InitializerName            string `json:"initializerName"`
	InitializerIsLocal         bool   `json:"initializerIsLocal"`
	InitializerIsParameter     bool   `json:"initializerIsParameter"`
	ParameterDeclarationIsNode bool   `json:"parameterDeclarationIsNode"`
	LocalDeclarationIsNode     bool   `json:"localDeclarationIsNode"`
	ParameterUTF8              [2]int `json:"parameterUtf8"`
	LocalUTF8                  [2]int `json:"localUtf8"`
	UseUTF8                    [2]int `json:"useUtf8"`
}
