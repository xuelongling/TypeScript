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
)

// Independent official-only bounded class/type-namespace diagnostics.
// Expected validity follows the supplied literal declarations; no DTO or Product is consulted.
func TestExceptionScriptClassNamespaceIndependentOfficialOracle(t *testing.T) {
	cases := []struct {
		name, className, source, sha256 string
		external, invalid               bool
		bytes, utf16                    int
	}{
		{name: "scriptPairControl", className: "Pair", source: "// 😀\r\n@tsfgValue class Pair {}\r\nfunction entry(): Pair { return new Pair(); }", sha256: "cc548f7d580c5b3a015f218f9926544bd9664f31024f9b1f4ae66c50c94fb120", external: false, invalid: false, bytes: 80, utf16: 78},
		{name: "moduleI32Control", className: "i32", source: "// 😀\r\n@tsfgValue class i32 {}\r\nexport function entry(): i32 { return new i32(); }", sha256: "89e011df1586c0879e423fd58a1a4cc4a69851ec5fb472558cb743c987ee6c42", external: true, invalid: false, bytes: 84, utf16: 82},
		{name: "scriptI32TypeAliasConflict", className: "i32", source: "// 😀\r\n@tsfgValue class i32 {}\r\nfunction entry(): i32 { return new i32(); }", sha256: "e1c592b14c49d05ee2a65e855d3894f19349464f364f41c8f89b77657d9a29eb", external: false, invalid: true, bytes: 77, utf16: 75},
		{name: "scriptBoolTypeAliasConflict", className: "bool", source: "// 😀\r\n@tsfgValue class bool {}\r\nfunction entry(): bool { return new bool(); }", sha256: "d42889ce8462b2422afa8379659ee6813aadf313d6b33e1af98c86b6b3f8f549", external: false, invalid: true, bytes: 80, utf16: 78},
		{name: "scriptF64TypeAliasConflict", className: "f64", source: "// 😀\r\n@tsfgValue class f64 {}\r\nfunction entry(): f64 { return new f64(); }", sha256: "f9cbcd878be8b87378533fa9bd7004a443704319d45cef21d73b6f88aefc20c0", external: false, invalid: true, bytes: 77, utf16: 75},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sum := sha256.Sum256([]byte(tc.source))
			if len(tc.source) >= 1024 || len(tc.source) != tc.bytes || len(utf16.Encode([]rune(tc.source))) != tc.utf16 || hex.EncodeToString(sum[:]) != tc.sha256 {
				t.Fatal("frozen literal changed")
			}
			request := aggregateRequest(t, tc.source)
			program := throwBoundaryProgramForTest(t, request)
			file := program.GetSourceFile(request.RootFile)
			if file == nil || file.Text() != tc.source || ast.IsExternalModule(file) != tc.external || len(file.Statements.Nodes) != 2 {
				t.Fatal("actual module/script input differs")
			}
			if file.Statements.Nodes[0].Kind != ast.KindClassDeclaration || file.Statements.Nodes[0].Name().Text() != tc.className || file.Statements.Nodes[1].Kind != ast.KindFunctionDeclaration {
				t.Fatal("actual class/function declarations differ")
			}
			consumed := []string{}
			declarations := []map[string]any{}
			for _, input := range program.GetSourceFiles() {
				consumed = append(consumed, input.FileName())
				if input != file {
					hash := sha256.Sum256([]byte(input.Text()))
					declarations = append(declarations, map[string]any{"path": input.FileName(), "bytes": len(input.Text()), "sha256": hex.EncodeToString(hash[:])})
				}
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
			record := map[string]any{"name": tc.name, "source": tc.source, "sha256": tc.sha256, "bytes": tc.bytes, "utf16Length": tc.utf16, "external": tc.external, "invalid": tc.invalid, "consumed": consumed, "declarations": declarations, "diagnostics": diagnostics}
			raw, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			t.Log("OFFICIAL_CLASS_NAMESPACE_JSON " + string(raw))
			if tc.invalid != (len(diagnostics) > 0) {
				t.Fatal("official validity differs from independent literal expectation")
			}
		})
	}
}
