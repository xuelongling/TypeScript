// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

// Evidence-only increment at Source 4f643026fb5471910c4709f33af6cd59d8e51119.
// These two hand-owned inputs/expectations do not alter frozen corpus30. Fresh
// official controls precede the existing complete public/private capture helper.
// No invocation, emitted binary, HIR implementation or causal RED is claimed.
func TestExceptionFactsCommandV1ReturnPresenceWidening(t *testing.T) {
	const declarations = `interface Object {}
interface Function {}
interface CallableFunction {}
interface NewableFunction {}
interface IArguments {}
interface String {}
interface Number {}
interface Boolean {}
interface RegExp {}
interface Array<T> {}
type bool = boolean;
declare const __tsfg_i32: unique symbol;
type i32 = number & { readonly [__tsfg_i32]: true };
type f64 = number;
declare const __tsfg_i8: unique symbol;
type i8 = number & { readonly [__tsfg_i8]: true };
declare const __tsfg_i16: unique symbol;
type i16 = number & { readonly [__tsfg_i16]: true };
declare const __tsfg_i64: unique symbol;
type i64 = number & { readonly [__tsfg_i64]: true };
declare const __tsfg_u8: unique symbol;
type u8 = number & { readonly [__tsfg_u8]: true };
declare const __tsfg_u16: unique symbol;
type u16 = number & { readonly [__tsfg_u16]: true };
declare const __tsfg_u32: unique symbol;
type u32 = number & { readonly [__tsfg_u32]: true };
declare const __tsfg_u64: unique symbol;
type u64 = number & { readonly [__tsfg_u64]: true };
declare const __tsfg_isize: unique symbol;
type isize = number & { readonly [__tsfg_isize]: true };
declare const __tsfg_usize: unique symbol;
type usize = number & { readonly [__tsfg_usize]: true };
`
	const declarationPath = "/src/minimal-integers-v3.d.ts"
	const declarationDigest = "sha256:9fc1955899e303b5815590f7150f60662f3cece0a633c94a2a06669cf509a96e"
	for _, tc := range []struct {
		name, source, sourceDigest string
		parameterNull, returnUndefined bool
		end int
		parameter, parameterAnnotation, returnAnnotation span
		returned, read span
	}{
		{
			name: "scalarToNull",
			source: "export function entry(value: i32): i32 | null { return value; }",
			sourceDigest: "sha256:f1ad3b506e908d09770af75de2958a308094e9aac21587e16624213a5916924d",
			end: 63, parameter: span{22, 32}, parameterAnnotation: span{29, 32},
			returnAnnotation: span{35, 45}, returned: span{47, 61}, read: span{54, 60},
		},
		{
			name: "nullToNullUndefined",
			source: "export function entry(value: i32 | null): i32 | null | undefined { return value; }",
			sourceDigest: "sha256:1a2944fb243503410d7945f1b3a035a7c7d531490732ba51d630bd8b753fa6e9",
			parameterNull: true, returnUndefined: true,
			end: 82, parameter: span{22, 39}, parameterAnnotation: span{29, 39},
			returnAnnotation: span{42, 64}, returned: span{66, 80}, read: span{73, 79},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			whole, definition, parameterName := span{0, tc.end}, span{15, 21}, span{22, 27}
			if len(tc.source) != tc.end || len(utf16.Encode([]rune(tc.source))) != tc.end ||
				exceptionPackageExportsDigestForTest([]byte(tc.source)) != tc.sourceDigest ||
				len(declarations) != 1214 || exceptionPackageExportsDigestForTest([]byte(declarations)) != declarationDigest {
				t.Fatal("independent source/DTS bytes or UTF16 envelope changed")
			}
			r := integerRequest(tc.source)
			expectedInput := map[string]any{
				"sourceRoot": "/src", "rootFile": "/src/main.ts",
				"files": []any{map[string]any{"path": "/src/main.ts", "content": tc.source},
					map[string]any{"path": declarationPath, "content": declarations}},
				"declarations": map[string]any{"version": "minimal-integers-v3", "paths": []string{declarationPath}},
				"config": map[string]any{"strict": true, "noEmit": true, "allowJs": false},
				"target": map[string]any{"target": "windows-x86_64-msvc", "triple": "x86_64-pc-windows-msvc",
					"pointerBits": "64", "llvmDataLayout": "e-m:w-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128"},
			}
			if !bytes.Equal(nullableFactsBoundaryCanonical(t, r), nullableFactsBoundaryCanonical(t, expectedInput)) {
				t.Fatal("request helper changed the complete independent input")
			}
			input := exceptionPackageExportsRequestForTest(t, tc.name, r)
			program := throwBoundaryProgramForTest(t, r)
			if !t.Run("officialDiagnostics", func(t *testing.T) { throwBoundaryDiagnosticsForTest(t, program) }) {
				return
			}
			if !t.Run("officialTypesSymbolsExportsSourcesUTF16", func(t *testing.T) {
				expectedFiles := map[string]string{"/src/main.ts": tc.source, declarationPath: declarations}
				for _, file := range program.GetSourceFiles() {
					content, found := expectedFiles[file.FileName()]
					if !found || file.Text() != content {
						t.Fatal("official Program consumed foreign, repeated or changed source")
					}
					delete(expectedFiles, file.FileName())
				}
				if len(expectedFiles) != 0 {
					t.Fatal("official Program omitted an original source or DTS")
				}
				file := program.GetSourceFile(r.RootFile)
				if file == nil || len(file.Statements.Nodes) != 1 || file.Statements.Nodes[0].Kind != ast.KindFunctionDeclaration {
					t.Fatal("one original function declaration required")
				}
				node := file.Statements.Nodes[0]
				fn := node.AsFunctionDeclaration()
				if node.Name() == nil || node.Name().Text() != "entry" || fn.Type == nil || fn.TypeParameters != nil ||
					fn.Parameters == nil || len(fn.Parameters.Nodes) != 1 || fn.Body == nil || fn.Body.Kind != ast.KindBlock ||
					len(fn.Body.AsBlock().Statements.Nodes) != 1 {
					t.Fatal("original function header/body shape changed")
				}
				parameterNode := fn.Parameters.Nodes[0]
				parameter := parameterNode.AsParameterDeclaration()
				terminal := fn.Body.AsBlock().Statements.Nodes[0]
				if parameterNode.Name() == nil || parameterNode.Name().Text() != "value" || parameter.Type == nil ||
					parameter.QuestionToken != nil || parameter.Initializer != nil || parameter.DotDotDotToken != nil ||
					terminal.Kind != ast.KindReturnStatement || terminal.AsReturnStatement().Expression == nil {
					t.Fatal("required original parameter and return required")
				}
				use := terminal.AsReturnStatement().Expression
				if use.Kind != ast.KindIdentifier || use.Text() != "value" {
					t.Fatal("return must be the original direct parameter reference")
				}
				// Independent literal raw spans retain node.Pos trivia. Annotation
				// DTOs use token spans; the fresh AST must confirm both conventions.
				for _, anchor := range []struct { node *ast.Node; want span }{
					{node, whole}, {node.Name(), definition}, {parameterNode, tc.parameter},
					{parameterNode.Name(), parameterName}, {terminal, tc.returned}, {use, tc.read},
				} {
					if throwBoundaryNodeForTest(t, file, anchor.node) != anchor.want {
						t.Fatalf("official raw UTF16 range differs from independent literal: want=%+v", anchor.want)
					}
				}
				for _, anchor := range []struct { node *ast.Node; want span }{
					{parameter.Type, tc.parameterAnnotation}, {fn.Type, tc.returnAnnotation},
				} {
					start := anchor.node.Pos()
					for start < anchor.node.End() && (tc.source[start] == ' ' || tc.source[start] == '\t') { start++ }
					if throwBoundarySpanForTest(tc.source, start, anchor.node.End()) != anchor.want {
						t.Fatal("official annotation token range changed")
					}
				}
				check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
				defer release()
				var nominal *checker.Type
				for _, decl := range program.GetSourceFile(declarationPath).Statements.Nodes {
					if decl.Kind == ast.KindTypeAliasDeclaration && decl.Name().Text() == "i32" {
						if nominal != nil { t.Fatal("duplicate i32 alias") }
						nominal = check.GetDeclaredTypeOfSymbol(check.GetSymbolAtLocation(decl.Name()))
					}
				}
				if nominal == nil { t.Fatal("official nominal i32 declaration missing") }
				members := func(typ *checker.Type, undefined, null bool) {
					t.Helper()
					if typ == nil { t.Fatal("missing official type") }
					parts := []*checker.Type{typ}
					if typ.IsUnion() { parts = typ.Types() }
					wantCount := 1
					if undefined { wantCount++ }; if null { wantCount++ }
					foundPayload, foundUndefined, foundNull := false, false, false
					for _, part := range parts {
						switch {
						case part == nominal && !foundPayload: foundPayload = true
						case part.Flags()&checker.TypeFlagsUndefined != 0 && !foundUndefined: foundUndefined = true
						case part.Flags()&checker.TypeFlagsNull != 0 && !foundNull: foundNull = true
						default: t.Fatal("foreign or repeated official union member")
						}
					}
					if len(parts) != wantCount || !foundPayload || foundUndefined != undefined || foundNull != null {
						t.Fatal("official type does not have the exact independent absence members")
					}
				}
				functionSymbol, parameterSymbol := check.GetSymbolAtLocation(node.Name()), check.GetSymbolAtLocation(parameterNode.Name())
				if functionSymbol == nil || parameterSymbol == nil || functionSymbol == parameterSymbol ||
					len(functionSymbol.Declarations) != 1 || functionSymbol.Declarations[0] != node ||
					len(parameterSymbol.Declarations) != 1 || parameterSymbol.Declarations[0] != parameterNode ||
					check.GetSymbolAtLocation(use) != parameterSymbol {
					t.Fatal("actual original function/parameter/reference symbol identity changed")
				}
				parameterType, resultType := check.GetTypeFromTypeNode(parameter.Type), check.GetTypeFromTypeNode(fn.Type)
				members(parameterType, false, tc.parameterNull)
				members(resultType, tc.returnUndefined, true)
				members(check.GetTypeAtLocation(use), false, tc.parameterNull)
				if parameterType == resultType || check.GetTypeOfSymbol(parameterSymbol) != parameterType ||
					check.GetTypeAtLocation(parameterNode.Name()) != parameterType || check.GetTypeAtLocation(use) != parameterType {
					t.Fatal("current return reference must retain the narrower original parameter type")
				}
				signatures := check.GetSignaturesOfType(check.GetTypeOfSymbol(functionSymbol), checker.SignatureKindCall)
				if len(signatures) != 1 || signatures[0].Declaration() != node || len(signatures[0].Parameters()) != 1 ||
					signatures[0].Parameters()[0] != parameterSymbol || check.GetReturnTypeOfSignature(signatures[0]) != resultType {
					t.Fatal("official callable signature lost the actual declaration or wider return")
				}
				if !ast.IsExternalModule(file) || file.AsNode().Symbol() == nil { t.Fatal("actual external module missing") }
				exported := check.GetExportsOfModule(file.AsNode().Symbol())
				if len(exported) != 1 || exported[0].Name != "entry" || exported[0].Flags&ast.SymbolFlagsAlias != 0 ||
					len(exported[0].Declarations) != 1 || exported[0].Declarations[0] != node ||
					check.GetTypeOnlyAliasDeclaration(exported[0]) != nil {
					t.Fatal("actual same-source direct export identity changed")
				}
			}) { return }

			// These complete facts and exports are hand-owned. Neither public nor
			// private producer output is consulted while constructing expectations.
			parameterPresence := map[string]any{"undefined": false, "null": tc.parameterNull, "optional": false,
				"default": false, "range": tc.parameter, "annotation": tc.parameterAnnotation}
			returnPresence := map[string]any{"undefined": tc.returnUndefined, "null": true,
				"range": whole, "annotation": tc.returnAnnotation}
			parameter := map[string]any{"id": "s2", "name": "value", "kind": "parameter", "type": "i32",
				"writable": true, "range": parameterName, "presence": parameterPresence}
			definitionSymbol := map[string]any{"id": "s1", "name": "entry", "kind": "function", "type": "i32",
				"writable": false, "range": definition, "returnPresence": returnPresence}
			expression := map[string]any{"kind": "reference", "type": "i32", "symbol": "s2", "range": tc.read,
				"presenceFlow": map[string]any{"undefined": false, "null": tc.parameterNull}}
			facts := map[string]any{"status": "ok", "inputs": []string{"/src/main.ts", declarationPath},
				"functions": []any{map[string]any{"name": "entry", "symbol": "s1", "parameters": []any{parameter},
					"returnType": "i32", "returnPresence": returnPresence, "parameterInitializers": []any{nil},
					"range": whole, "body": []any{map[string]any{"kind": "return", "expression": expression, "range": tc.returned}}}},
				"symbols": []any{definitionSymbol, parameter}}
			exports := []any{map[string]any{"sourcePath": "/src/main.ts", "external": true,
				"bindings": []any{map[string]any{"name": "entry", "target": "s1", "sourcePath": "/src/main.ts",
					"range": whole, "definition": definition}}}}
			oracle := map[string]any{"case": tc.name, "sourceSha256": tc.sourceDigest, "bytes": len(tc.source),
				"units": len(utf16.Encode([]rune(tc.source))), "declarationSha256": declarationDigest,
				"raw": map[string]any{"function": whole, "definition": definition, "parameter": tc.parameter,
					"parameterName": parameterName, "return": tc.returned, "read": tc.read},
				"token": map[string]any{"parameterAnnotation": tc.parameterAnnotation, "returnAnnotation": tc.returnAnnotation},
				"parameterMembers": map[string]any{"payload": "i32", "undefined": false, "null": tc.parameterNull},
				"currentReturnMembers": map[string]any{"payload": "i32", "undefined": false, "null": tc.parameterNull},
				"declaredReturnMembers": map[string]any{"payload": "i32", "undefined": tc.returnUndefined, "null": true}, "exports": exports}
			t.Logf("TSFG_EXCEPTION_CORPUS_FRESH_ORACLE_V1 case=%s %s", tc.name,
				base64.StdEncoding.EncodeToString(nullableFactsBoundaryCanonical(t, oracle)))
			exceptionPackageExportsPacketsForTest(t, tc.name, r, input, facts, exports)
		})
	}
}
