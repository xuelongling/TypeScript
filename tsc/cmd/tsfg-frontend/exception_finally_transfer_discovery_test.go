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

// Small official-only discovery: no public/private producer or cold reader call.
// Non-control continuation flags are measured, never filled from a transfer model.
func TestExceptionFinallyTransferIndependentOfficialDiscovery(t *testing.T) {
	cases := []struct {
		name, source string
		control      bool
	}{
		{name: "finallyWriteAfterBooleanGuard", source: "// 😀\r\nexport function entry(value: i32): i32 {\r\n  try { throw value; } catch (caught) {\r\n    if (typeof caught !== \"boolean\") { throw value; }\r\n    try { } finally { caught = value; }\r\n    throw caught;\r\n  }\r\n}", control: false},
		{name: "finallyBooleanExitAfterBooleanGuard", source: "// 😀\r\nexport function entry(value: i32): i32 {\r\n  try { throw value; } catch (caught) {\r\n    if (typeof caught !== \"boolean\") { throw value; }\r\n    try { } finally { if (typeof caught === \"boolean\") { throw value; } }\r\n    throw caught;\r\n  }\r\n}", control: false},
		{name: "emptyFinallyAfterBooleanGuard", source: "// 😀\r\nexport function entry(value: i32): i32 {\r\n  try { throw value; } catch (caught) {\r\n    if (typeof caught !== \"boolean\") { throw value; }\r\n    try { } finally { }\r\n    throw caught;\r\n  }\r\n}", control: true},
	}
	total := 0
	for _, tc := range cases {
		total += len(tc.source)
	}
	if len(cases) != 3 || total >= 1024 {
		t.Fatal("bounded discovery input budget changed")
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := integerRequest(tc.source)
			program := throwBoundaryProgramForTest(t, r)
			sum := sha256.Sum256([]byte(tc.source))
			report := map[string]any{"name": tc.name, "source": tc.source, "sourceSha256": hex.EncodeToString(sum[:]), "bytes": len(tc.source), "utf16Length": len(utf16.Encode([]rune(tc.source)))}
			emit := func() {
				encoded, err := json.Marshal(report)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("TSFG_EXCEPTION_FINALLY_TRANSFER_OFFICIAL_V1 %s", encoded)
			}
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
					if file := diagnostic.File(); file != nil {
						item["file"] = file.FileName()
						if diagnostic.Pos() >= 0 && diagnostic.End() >= diagnostic.Pos() && diagnostic.End() <= len(file.Text()) {
							item["utf16"] = [2]int{len(utf16.Encode([]rune(file.Text()[:diagnostic.Pos()]))), len(utf16.Encode([]rune(file.Text()[:diagnostic.End()])))}
						}
					}
					diagnostics = append(diagnostics, item)
				}
			}
			report["diagnostics"] = diagnostics
			consumed := []string{}
			for _, file := range program.GetSourceFiles() {
				consumed = append(consumed, file.FileName())
			}
			slices.Sort(consumed)
			report["consumed"] = consumed
			file := program.GetSourceFile(r.RootFile)
			if len(diagnostics) != 0 || file == nil || file.Text() != tc.source || !ast.IsExternalModule(file) || len(file.Statements.Nodes) != 1 || !slices.Equal(consumed, []string{"/src/main.ts", "/src/minimal-integers-v3.d.ts"}) {
				emit()
				t.Fatal("fresh official literal/diagnostic/input precondition differs")
			}
			node := file.Statements.Nodes[0]
			if node.Kind != ast.KindFunctionDeclaration {
				emit()
				t.Fatal("original function AST missing")
			}
			fn := node.AsFunctionDeclaration()
			if fn.Name() == nil || fn.Name().Text() != "entry" || len(fn.Parameters.Nodes) != 1 || fn.Body == nil {
				emit()
				t.Fatal("original signature/body missing")
			}
			parameter := fn.Parameters.Nodes[0]
			body := fn.Body.AsBlock().Statements.Nodes
			if parameter.Name() == nil || parameter.Name().Text() != "value" || len(body) != 1 || body[0].Kind != ast.KindTryStatement || body[0].AsTryStatement().CatchClause == nil {
				emit()
				t.Fatal("original parameter/catch AST missing")
			}
			catchClause := body[0].AsTryStatement().CatchClause.AsCatchClause()
			caught := catchClause.VariableDeclaration
			if caught == nil || caught.Name() == nil || caught.Name().Text() != "caught" || catchClause.Block == nil {
				emit()
				t.Fatal("original catch binding missing")
			}
			statements := catchClause.Block.AsBlock().Statements.Nodes
			if len(statements) != 3 || statements[2].Kind != ast.KindThrowStatement {
				emit()
				t.Fatal("original terminal continuation missing")
			}
			continuation := statements[2].AsThrowStatement().Expression
			check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
			defer release()
			functionSymbol := check.GetSymbolAtLocation(fn.Name())
			parameterSymbol := check.GetSymbolAtLocation(parameter.Name())
			catchSymbol := check.GetSymbolAtLocation(caught.Name())
			identities := functionSymbol != nil && parameterSymbol != nil && catchSymbol != nil && functionSymbol != parameterSymbol && parameterSymbol != catchSymbol && functionSymbol != catchSymbol
			if identities {
				identities = len(functionSymbol.Declarations) == 1 && functionSymbol.Declarations[0] == node && len(parameterSymbol.Declarations) == 1 && parameterSymbol.Declarations[0] == parameter && len(catchSymbol.Declarations) == 1 && catchSymbol.Declarations[0] == caught
			}
			exported := check.GetExportsOfModule(file.AsNode().Symbol())
			report["completeExports"] = []string{}
			for _, symbol := range exported {
				report["completeExports"] = append(report["completeExports"].([]string), symbol.Name)
			}
			report["declarationIdentitiesValid"] = identities
			positionsAgree, usesBelongToCatch := true, true
			position := func(start, end int) [2]int {
				if start < 0 || end < start || end > len(tc.source) {
					positionsAgree = false
					return [2]int{-1, -1}
				}
				pair := [2]int{len(utf16.Encode([]rune(tc.source[:start]))), len(utf16.Encode([]rune(tc.source[:end])))}
				official := file.GetPositionMap()
				positionsAgree = positionsAgree && pair == ([2]int{official.UTF8ToUTF16(start), official.UTF8ToUTF16(end)})
				return pair
			}
			occurrences := []map[string]any{}
			var continuationType *checker.Type
			var visit func(*ast.Node) bool
			visit = func(item *ast.Node) bool {
				if item == nil {
					return false
				}
				if item.Kind == ast.KindIdentifier && item.Text() == "caught" {
					symbol, typ := check.GetSymbolAtLocation(item), check.GetTypeAtLocation(item)
					usesBelongToCatch = usesBelongToCatch && symbol != nil && symbol == catchSymbol && typ != nil
					trim := scanner.SkipTrivia(tc.source, item.Pos())
					use := map[string]any{"name": item.Text(), "rawUTF8": [2]int{item.Pos(), item.End()}, "rawUTF16": position(item.Pos(), item.End()), "trimUTF16": position(trim, item.End()), "sameCatch": symbol != nil && symbol == catchSymbol, "isDeclaration": item == caught.Name(), "isContinuation": item == continuation}
					if typ != nil {
						use["flags"], use["isUnknown"], use["isBooleanLike"], use["isNever"] = uint64(typ.Flags()), typ.Flags() == checker.TypeFlagsUnknown, typ.Flags()&checker.TypeFlagsBooleanLike != 0, typ.Flags()&checker.TypeFlagsNever != 0
					}
					if item == continuation {
						continuationType = typ
					}
					occurrences = append(occurrences, use)
				}
				item.ForEachChild(visit)
				return false
			}
			visit(node)
			report["occurrences"], report["positionsAgree"] = occurrences, positionsAgree
			if catchSymbol != nil {
				if typ := check.GetTypeOfSymbol(catchSymbol); typ != nil {
					report["catchDeclarationFlags"] = uint64(typ.Flags())
				}
			}
			emit()
			if !identities || len(exported) != 1 || exported[0] != functionSymbol || !positionsAgree || !usesBelongToCatch || continuationType == nil {
				t.Fatal("official ownership/range/continuation precondition differs")
			}
			if tc.control && continuationType.Flags()&checker.TypeFlagsBooleanLike == 0 {
				t.Fatal("empty finally no longer preserves the existing boolean control")
			}
		})
	}
}
