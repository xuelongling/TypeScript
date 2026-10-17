// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/scanner"
)

// Scratch official-only oracle. Code1263 is pinned from diagnostics_generated.go
// and property grammarchecks.go:1924-1934, never from a Product/Source response.
func TestExceptionInitializedDefinitePropertyIndependentOfficialOracle(t *testing.T) {
	cases := []struct {
		name, source, sha256 string
		invalid              bool
		bytes, utf16         int
	}{
		{name: "operand01InitializedControl", source: "// 😀\r\n@tsfgComponent class Box { flag: bool = true; }\r\nexport function f(box: Box): bool { try { throw true; } catch (caught) { if (typeof caught === \"boolean\") { return box.flag === caught; } throw caught; } }", invalid: false, bytes: 213, utf16: 211, sha256: "894fa65464dd477fe07927b0934539ac87007922c33d11418ff4a084cc905596"},
		{name: "operand01InitializedDefiniteInvalid", source: "// 😀\r\n@tsfgComponent class Box { flag!: bool = true; }\r\nexport function f(box: Box): bool { try { throw true; } catch (caught) { if (typeof caught === \"boolean\") { return box.flag === caught; } throw caught; } }", invalid: true, bytes: 214, utf16: 212, sha256: "901e613d7a7bfd2089c8797c2976d08dda6e06097579eb140fa5c1230ca0bc64"},
		{name: "operand02InitializedControl", source: "// 😀\r\n@tsfgComponent class Box { flag: bool = true; }\r\nexport function f(box: Box): bool { try { throw true; } catch (caught) { if (typeof caught === \"boolean\") { return caught === box.flag; } throw caught; } }", invalid: false, bytes: 213, utf16: 211, sha256: "db2f022e9d8e27e1bd99f50955415b12da7adc5867ae2e8b5a49fcba54279d2e"},
		{name: "operand02InitializedDefiniteInvalid", source: "// 😀\r\n@tsfgComponent class Box { flag!: bool = true; }\r\nexport function f(box: Box): bool { try { throw true; } catch (caught) { if (typeof caught === \"boolean\") { return caught === box.flag; } throw caught; } }", invalid: true, bytes: 214, utf16: 212, sha256: "9376691ebc518d553f8f5b671ed5dbb920754964feea2ddf201527a80e0583fb"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := tc.source
			sum := sha256.Sum256([]byte(source))
			if len(source) >= 1024 || len(source) != tc.bytes || len(utf16.Encode([]rune(source))) != tc.utf16 || hex.EncodeToString(sum[:]) != tc.sha256 {
				t.Fatal("frozen literal changed")
			}
			r := aggregateRequest(t, source)
			program := throwBoundaryProgramForTest(t, r)
			file := program.GetSourceFile(r.RootFile)
			if file == nil || file.Text() != source || !ast.IsExternalModule(file) || len(file.Statements.Nodes) != 2 {
				t.Fatal("actual module/class/function input missing")
			}
			consumed := []string{}
			for _, file := range program.GetSourceFiles() {
				consumed = append(consumed, file.FileName())
			}
			slices.Sort(consumed)
			if !slices.Equal(consumed, []string{"/src/main.ts", "/src/minimal-aggregates-v1.d.ts", "/src/minimal-conversions-v5.d.ts"}) {
				t.Fatal("actual complete input closure differs")
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
			count := 0
			expectedDiagnostic := !tc.invalid
			bang := strings.Index(source, "flag!") + len("flag")
			for _, group := range groups {
				for _, diagnostic := range group.values {
					count++
					item := map[string]any{"group": group.name, "code": diagnostic.Code(), "utf8": [2]int{diagnostic.Pos(), diagnostic.End()}}
					if origin := diagnostic.File(); origin != nil {
						item["file"] = origin.FileName()
						if diagnostic.Pos() >= 0 && diagnostic.End() >= diagnostic.Pos() && diagnostic.End() <= len(origin.Text()) {
							item["utf16"] = [2]int{len(utf16.Encode([]rune(origin.Text()[:diagnostic.Pos()]))), len(utf16.Encode([]rune(origin.Text()[:diagnostic.End()])))}
						}
					}
					diagnostics = append(diagnostics, item)
					expectedDiagnostic = tc.invalid && group.name == "semantic" && diagnostic.Code() == 1263 && diagnostic.File() == file && diagnostic.Pos() == bang && diagnostic.End() == bang+1
				}
			}
			classNode, fnNode := file.Statements.Nodes[0], file.Statements.Nodes[1]
			if classNode.Kind != ast.KindClassDeclaration || classNode.Name().Text() != "Box" || len(classNode.Members()) != 1 || classNode.Modifiers() == nil || len(classNode.Modifiers().Nodes) != 1 || fnNode.Kind != ast.KindFunctionDeclaration || fnNode.Name().Text() != "f" {
				t.Fatal("original class/member/function declarations differ")
			}
			decorator := classNode.Modifiers().Nodes[0]
			if decorator.Kind != ast.KindDecorator || decorator.AsDecorator().Expression.Kind != ast.KindIdentifier || decorator.AsDecorator().Expression.Text() != "tsfgComponent" {
				t.Fatal("actual standard marker missing")
			}
			member := classNode.Members()[0]
			if member.Kind != ast.KindPropertyDeclaration || member.Name().Text() != "flag" || member.Modifiers() != nil {
				t.Fatal("one ordinary public field required")
			}
			property := member.AsPropertyDeclaration()
			if property.Type == nil || property.Initializer == nil || property.Initializer.Kind != ast.KindTrueKeyword || (property.PostfixToken != nil) != tc.invalid {
				t.Fatal("initializer/definite-assertion distinction lost")
			}
			if tc.invalid && (property.PostfixToken.Kind != ast.KindExclamationToken || property.PostfixToken.Pos() != bang || property.PostfixToken.End() != bang+1) {
				t.Fatal("actual ! anchor differs from literal preimage")
			}
			fn := fnNode.AsFunctionDeclaration()
			if len(fn.Parameters.Nodes) != 1 || fn.Parameters.Nodes[0].Name().Text() != "box" || fn.Type == nil || fn.Body == nil {
				t.Fatal("required Box function signature missing")
			}
			parameter := fn.Parameters.Nodes[0]
			check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
			defer release()
			classSymbol, fieldSymbol, fnSymbol, parameterSymbol := check.GetSymbolAtLocation(classNode.Name()), check.GetSymbolAtLocation(member.Name()), check.GetSymbolAtLocation(fn.Name()), check.GetSymbolAtLocation(parameter.Name())
			symbols := []*ast.Symbol{classSymbol, fieldSymbol, fnSymbol, parameterSymbol}
			declarations := []*ast.Node{classNode, member, fnNode, parameter}
			seen := map[*ast.Symbol]bool{}
			for i, symbol := range symbols {
				if symbol == nil || seen[symbol] || len(symbol.Declarations) != 1 || symbol.Declarations[0] != declarations[i] || ast.GetSourceFileOfNode(symbol.Declarations[0]) != file {
					t.Fatal("actual declaration/binding/source identity changed")
				}
				seen[symbol] = true
			}
			instance := check.GetDeclaredTypeOfSymbol(classSymbol)
			boolean := check.GetTypeFromTypeNode(property.Type)
			if instance == nil || !instance.IsClass() || boolean == nil || !boolean.IsBooleanLike() || check.GetTypeOfSymbol(fieldSymbol) != boolean || check.GetPropertyOfType(instance, "flag") != fieldSymbol || check.GetTypeFromTypeNode(parameter.AsParameterDeclaration().Type) != instance || check.GetTypeOfSymbol(parameterSymbol) != instance || check.GetTypeFromTypeNode(fn.Type) != boolean || !check.GetTypeAtLocation(property.Initializer).IsBooleanLike() {
				t.Fatal("actual field/parameter/return type identity changed")
			}
			marker := check.GetSymbolAtLocation(decorator.AsDecorator().Expression)
			if marker == nil || marker.Name != "tsfgComponent" || len(marker.Declarations) != 1 || marker.Declarations[0].Kind != ast.KindFunctionDeclaration || ast.GetSourceFileOfNode(marker.Declarations[0]).FileName() != "/src/minimal-aggregates-v1.d.ts" {
				t.Fatal("decorator did not resolve to the actual supplied ambient marker")
			}
			exports := check.GetExportsOfModule(file.AsNode().Symbol())
			if len(exports) != 1 || exports[0] != fnSymbol || exports[0].Name != "f" || len(exports[0].Declarations) != 1 || exports[0].Declarations[0] != fnNode {
				t.Fatal("complete official export table changed")
			}
			anchors := []map[string]any{}
			fieldUses := 0
			positionsAgree := true
			position := func(start, end int) [2]int {
				if start < 0 || end < start || end > len(source) {
					t.Fatal("invalid official original offset")
				}
				pair := [2]int{len(utf16.Encode([]rune(source[:start]))), len(utf16.Encode([]rune(source[:end])))}
				official := file.GetPositionMap()
				positionsAgree = positionsAgree && pair == ([2]int{official.UTF8ToUTF16(start), official.UTF8ToUTF16(end)})
				return pair
			}
			var visit func(*ast.Node) bool
			visit = func(node *ast.Node) bool {
				if node == nil {
					return false
				}
				trim := scanner.SkipTrivia(source, node.Pos())
				anchors = append(anchors, map[string]any{"kind": int(node.Kind), "rawUTF8": [2]int{node.Pos(), node.End()}, "rawUTF16": position(node.Pos(), node.End()), "trimUTF16": position(trim, node.End()), "raw": source[node.Pos():node.End()]})
				if node.Kind == ast.KindPropertyAccessExpression && node.Name().Text() == "flag" {
					fieldUses++
					access := node.AsPropertyAccessExpression()
					if check.GetSymbolAtLocation(node.Name()) != fieldSymbol || check.GetSymbolAtLocation(access.Expression) != parameterSymbol || check.GetTypeAtLocation(access.Expression) != instance || check.GetTypeAtLocation(node) != boolean {
						t.Fatal("actual field use lost declared identity")
					}
				}
				node.ForEachChild(visit)
				return false
			}
			visit(classNode)
			visit(fnNode)
			expectedCode := 0
			if tc.invalid {
				expectedCode = 1263
			}
			record := map[string]any{"name": tc.name, "source": source, "sourceSha256": tc.sha256, "bytes": tc.bytes, "utf16Length": tc.utf16, "invalid": tc.invalid, "expectedCode": expectedCode, "diagnostics": diagnostics, "consumed": consumed, "anchors": anchors, "actualDeclarationIdentity": true, "fieldAccessCount": fieldUses, "completeExports": []string{"f"}, "positionsAgree": positionsAgree}
			encoded, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("TSFG_EXCEPTION_INITIALIZED_DEFINITE_PROPERTY_OFFICIAL_V1 %s", encoded)
			if !positionsAgree || fieldUses != 1 {
				t.Fatal("full original anchors/field-use occurrence changed")
			}
			if tc.invalid {
				if count != 1 || !expectedDiagnostic {
					t.Fatal("expected exactly the independently pinned semantic1263 at the original ! token")
				}
			} else if count != 0 {
				t.Fatal("original valid field control has official diagnostics")
			}
		})
	}
}
