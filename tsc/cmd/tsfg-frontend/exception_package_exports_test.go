// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

func exceptionPackageExportsDigestForTest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// These two real consumers share packet identity mechanics only. Each test owns
// its original source, independent full facts expectation and fresh export oracle.
func exceptionPackageExportsRequestForTest(t *testing.T, name string, r request) []byte {
	t.Helper()
	input, err := json.Marshal(r)
	t.Logf("TSFG_EXCEPTION_EXPORTS_PUBLIC_REQUEST_V1 case=%s %s", name, base64.StdEncoding.EncodeToString(input))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		after, err := json.Marshal(r)
		if err != nil || !bytes.Equal(input, after) {
			t.Fatal("original request changed")
		}
	})
	return input
}

func exceptionPackageExportsPacketsForTest(t *testing.T, name string, r request, input []byte, facts, exports any) {
	t.Helper()
	canonical := func(value any) []byte { return nullableFactsBoundaryCanonical(t, value) }
	wantFacts := canonical(facts)
	t.Logf("TSFG_EXCEPTION_EXPORTS_PUBLIC_EXPECTED_V1 case=%s %s", name, base64.StdEncoding.EncodeToString(wantFacts))
	var publicResponse []byte
	if !t.Run("publicControl", func(t *testing.T) {
		var output bytes.Buffer
		err := runFrontendCommand(t.Context(), nil, bytes.NewReader(input), &output)
		publicResponse = bytes.Clone(output.Bytes())
		t.Logf("TSFG_EXCEPTION_EXPORTS_PUBLIC_RESPONSE_V1 case=%s %s", name, base64.StdEncoding.EncodeToString(publicResponse))
		if err != nil {
			t.Fatalf("public command failed: %v; raw=%s", err, publicResponse)
		}
		if !bytes.HasSuffix(publicResponse, []byte("\n")) {
			t.Fatal("public response final newline changed")
		}
		var value any
		if err := json.Unmarshal(publicResponse, &value); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(nullableFactsBoundaryCanonical(t, value), wantFacts) {
			t.Fatalf("complete independent public expected differs: got=%s want=%s", publicResponse, wantFacts)
		}
	}) {
		return
	}
	const schema = "sha256:5221b50822965c633418462b28a9b58bfbc41b01e0fa4b6d31c50ab83805424d"
	files := slices.Clone(r.Files)
	slices.SortFunc(files, func(a, b inputFile) int { return strings.Compare(a.Path, b.Path) })
	identityFiles, manifest := []map[string]any{}, []map[string]any{}
	for _, file := range files {
		identityFiles = append(identityFiles, map[string]any{"path": file.Path, "content": file.Content})
		manifest = append(manifest, map[string]any{"path": file.Path, "digest": exceptionPackageExportsDigestForTest([]byte(file.Content)), "byteLength": len(file.Content), "length": len(utf16.Encode([]rune(file.Content))), "positionUnit": "utf16"})
	}
	var normalized map[string]any
	if err := json.Unmarshal(input, &normalized); err != nil {
		t.Fatal(err)
	}
	normalized["files"] = identityFiles
	privateRequest := canonical(map[string]any{"family": "tsfg.exception-facts-request-candidate", "version": 1, "schemaHash": schema, "input": r})
	before := bytes.Clone(privateRequest)
	requestID := exceptionPackageExportsDigestForTest(append([]byte("tsfg.exception-facts.request.v1\x00"), privateRequest...))
	// facts is the test's independent expected value, never an actual response.
	payload := map[string]any{"inputIdentity": exceptionPackageExportsDigestForTest(canonical(normalized)), "files": manifest, "facts": facts, "exports": exports}
	packageID := exceptionPackageExportsDigestForTest(append([]byte("tsfg.exception-facts.package.v1\x00"+schema+"\x00"), canonical(payload)...))
	expected := canonical(map[string]any{"family": "tsfg.exception-facts-package-candidate", "version": 1, "schemaHash": schema, "requestId": requestID, "packageId": packageID, "payload": payload})
	t.Logf("TSFG_EXCEPTION_EXPORTS_PRIVATE_CANONICAL_REQUEST_V1 case=%s %s", name, base64.StdEncoding.EncodeToString(privateRequest))
	t.Logf("TSFG_EXCEPTION_EXPORTS_PRIVATE_EXPECTED_V1 case=%s %s", name, base64.StdEncoding.EncodeToString(expected))
	t.Run("privateCommand", func(t *testing.T) {
		var output bytes.Buffer
		err := runFrontendCommand(t.Context(), []string{"--exception-facts-io-v1"}, bytes.NewReader(privateRequest), &output)
		raw := output.Bytes()
		t.Logf("TSFG_EXCEPTION_EXPORTS_PRIVATE_RESPONSE_V1 case=%s %s", name, base64.StdEncoding.EncodeToString(raw))
		if err != nil {
			t.Fatalf("private command rejected after official/public controls: %v; raw=%s", err, raw)
		}
		if !bytes.Equal(before, privateRequest) {
			t.Fatal("private command mutated original canonical bytes")
		}
		if !bytes.Equal(raw, expected) {
			t.Fatalf("complete independent canonical packet differs: got=%s want=%s", raw, expected)
		}
		var publicValue any
		var packet struct {
			Payload struct{ Facts json.RawMessage }
		}
		if err := json.Unmarshal(publicResponse, &publicValue); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &packet); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(packet.Payload.Facts, nullableFactsBoundaryCanonical(t, publicValue)) {
			t.Fatal("private facts lost complete verified same-input public facts")
		}
	})
}

func TestExceptionFactsCommandV1CompleteExports(t *testing.T) {
	const source = "// 😀\r\nfunction hidden(value: i32): i32 { return value; }\r\nexport function zebra(value: i32): i32 { throw value; }\r\nexport function alpha(value: i32): i32 { return hidden(value); }"
	t.Logf("TSFG_EXCEPTION_EXPORTS_SOURCE_V1 case=directExports %s", base64.StdEncoding.EncodeToString([]byte(source)))
	r := integerRequest(source)
	input := exceptionPackageExportsRequestForTest(t, "directExports", r)
	program := throwBoundaryProgramForTest(t, r)
	raw, token := map[string]span{}, map[string]span{}
	var exportGroups []any
	if !t.Run("officialOracle", func(t *testing.T) {
		throwBoundaryDiagnosticsForTest(t, program)
		file := program.GetSourceFile(r.RootFile)
		if file == nil || file.Text() != source || !ast.IsExternalModule(file) || len(file.Statements.Nodes) != 3 {
			t.Fatal("original three-function module changed")
		}
		consumed := []string{}
		for _, sourceFile := range program.GetSourceFiles() {
			consumed = append(consumed, sourceFile.FileName())
		}
		slices.Sort(consumed)
		if !reflect.DeepEqual(consumed, []string{"/src/main.ts", "/src/minimal-integers-v3.d.ts"}) {
			t.Fatal("complete actual consumed closure changed")
		}
		check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
		defer release()
		remember := func(label string, node *ast.Node, text string) {
			t.Helper()
			if node == nil || node.End() < len(text) {
				t.Fatal("official original node missing")
			}
			start := node.End() - len(text)
			if node.Pos() > start || source[start:node.End()] != text {
				t.Fatalf("original %s preimage changed", label)
			}
			raw[label] = throwBoundaryNodeForTest(t, file, node)
			token[label] = throwBoundarySpanForTest(source, start, node.End())
		}
		functionSymbols, parameterSymbols := []*ast.Symbol{}, []*ast.Symbol{}
		ids := map[*ast.Symbol]string{}
		var i32 *checker.Type
		for i, name := range []string{"hidden", "zebra", "alpha"} {
			node := file.Statements.Nodes[i]
			if node.Kind != ast.KindFunctionDeclaration || node.Name().Text() != name {
				t.Fatal("source preorder declaration changed")
			}
			fn := node.AsFunctionDeclaration()
			if fn.Type == nil || fn.TypeParameters != nil || len(fn.Parameters.Nodes) != 1 || fn.Body == nil || fn.Body.Kind != ast.KindBlock {
				t.Fatal("complete function signature changed")
			}
			parameter := fn.Parameters.Nodes[0]
			p := parameter.AsParameterDeclaration()
			if parameter.Name().Text() != "value" || p.Type == nil || p.Initializer != nil || p.QuestionToken != nil || p.DotDotDotToken != nil {
				t.Fatal("original required scalar parameter changed")
			}
			fnSymbol := exceptionTraversalIdentityForTest(t, check, node)
			parameterSymbol := exceptionTraversalIdentityForTest(t, check, parameter)
			functionSymbols, parameterSymbols = append(functionSymbols, fnSymbol), append(parameterSymbols, parameterSymbol)
			ids[fnSymbol] = []string{"s1", "s2", "s3"}[i]
			declared := check.GetTypeFromTypeNode(p.Type)
			if i == 0 {
				i32 = declared
			}
			if i32 == nil || i32.Alias() == nil || i32.Alias().Symbol() == nil || i32.Alias().Symbol().Name != "i32" || declared != i32 || check.GetTypeOfSymbol(parameterSymbol) != i32 || check.GetTypeFromTypeNode(fn.Type) != i32 {
				t.Fatal("shared actual nominal i32 identity changed")
			}
			signatures := check.GetSignaturesOfType(check.GetTypeOfSymbol(fnSymbol), checker.SignatureKindCall)
			if len(signatures) != 1 || signatures[0].Declaration() != node || len(signatures[0].Parameters()) != 1 || signatures[0].Parameters()[0] != parameterSymbol || check.GetReturnTypeOfSignature(signatures[0]) != i32 {
				t.Fatal("official complete signature identity changed")
			}
			body := fn.Body.AsBlock().Statements.Nodes
			if len(body) != 1 {
				t.Fatal("one original statement per function required")
			}
			text, kind := "return value;", ast.KindReturnStatement
			if i == 1 {
				text, kind = "throw value;", ast.KindThrowStatement
			}
			if i == 2 {
				text = "return hidden(value);"
			}
			if body[0].Kind != kind {
				t.Fatal("original return/throw direction changed")
			}
			var value *ast.Node
			if i == 1 {
				value = body[0].AsThrowStatement().Expression
			} else {
				value = body[0].AsReturnStatement().Expression
			}
			prefix := name + "."
			functionText := []string{"function hidden(value: i32): i32 { return value; }", "export function zebra(value: i32): i32 { throw value; }", "export function alpha(value: i32): i32 { return hidden(value); }"}[i]
			remember(prefix+"function", node, functionText)
			remember(prefix+"name", node.Name(), name)
			remember(prefix+"parameter", parameter, "value: i32")
			remember(prefix+"parameterName", parameter.Name(), "value")
			remember(prefix+"annotation", p.Type, "i32")
			remember(prefix+"statement", body[0], text)
			if i < 2 {
				if value.Kind != ast.KindIdentifier || check.GetSymbolAtLocation(value) != parameterSymbol || check.GetTypeAtLocation(value) != i32 {
					t.Fatal("original return/throw reference identity changed")
				}
				remember(prefix+"value", value, "value")
			} else {
				if value.Kind != ast.KindCallExpression || len(value.AsCallExpression().Arguments.Nodes) != 1 {
					t.Fatal("original nonexported call changed")
				}
				call := value.AsCallExpression()
				argument := call.Arguments.Nodes[0]
				resolved := check.GetResolvedSignature(value)
				if resolved == nil || check.GetSymbolAtLocation(call.Expression) != functionSymbols[0] || resolved.Declaration() != file.Statements.Nodes[0] || check.GetSymbolAtLocation(argument) != parameterSymbol || check.GetTypeAtLocation(argument) != i32 || check.GetTypeAtLocation(value) != i32 {
					t.Fatal("actual nonexported declaration/argument/resolved call identity changed")
				}
				remember(prefix+"value", value, "hidden(value)")
				remember(prefix+"argument", argument, "value")
			}
		}
		if raw["hidden.function"].Start != 0 || token["hidden.function"].Start != 7 {
			t.Fatal("original CRLF/non-BMP prefix changed")
		}
		seen := map[*ast.Symbol]bool{}
		for _, table := range [][]*ast.Symbol{functionSymbols, parameterSymbols} {
			for _, symbol := range table {
				if seen[symbol] {
					t.Fatal("same-spelled declarations merged actual symbols")
				}
				seen[symbol] = true
			}
		}
		bindings := []map[string]any{}
		exports := check.GetExportsOfModule(file.AsNode().Symbol())
		if len(exports) != 2 {
			t.Fatal("complete official export set must contain both direct functions")
		}
		for _, exported := range exports {
			if exported.Flags&ast.SymbolFlagsAlias != 0 || len(exported.Declarations) != 1 {
				t.Fatal("direct actual exported declaration required")
			}
			declaration := exported.Declarations[0]
			actual := check.GetSymbolAtLocation(declaration.Name())
			if ast.GetSourceFileOfNode(declaration) != file || declaration.Kind != ast.KindFunctionDeclaration || declaration != file.Statements.Nodes[1] && declaration != file.Statements.Nodes[2] || actual != exported || ids[actual] == "" {
				t.Fatal("official export did not map its actual defining node/symbol")
			}
			bindings = append(bindings, map[string]any{"name": exported.Name, "target": ids[actual], "sourcePath": file.FileName(), "range": throwBoundaryNodeForTest(t, file, declaration), "definition": throwBoundaryNodeForTest(t, file, declaration.Name())})
		}
		slices.SortFunc(bindings, func(a, b map[string]any) int { return strings.Compare(a["name"].(string), b["name"].(string)) })
		if bindings[0]["name"] != "alpha" || bindings[0]["target"] != "s3" || bindings[1]["name"] != "zebra" || bindings[1]["target"] != "s2" {
			t.Fatal("hidden leaked or complete official export ordering/identity changed")
		}
		exportGroups = []any{map[string]any{"sourcePath": file.FileName(), "external": true, "bindings": bindings}}
		oracle, err := json.Marshal(map[string]any{"case": "directExports", "kind": "test-only-official-complete-exports-oracle", "raw": raw, "token": token, "exports": exportGroups, "actualFunctionPreorder": []string{"hidden", "zebra", "alpha"}})
		t.Logf("TSFG_EXCEPTION_EXPORTS_OFFICIAL_ORACLE_V1 case=directExports %s", base64.StdEncoding.EncodeToString(oracle))
		if err != nil {
			t.Fatal(err)
		}
	}) {
		return
	}
	symbols, functions := []any{}, []any{}
	parameters := []map[string]any{}
	for i, name := range []string{"hidden", "zebra", "alpha"} {
		id := []string{"s1", "s2", "s3"}[i]
		symbols = append(symbols, exceptionTraversalSymbolForTest(id, name, "function", "i32", "", "", false, raw[name+".name"]))
		parameter := exceptionTraversalSymbolForTest([]string{"s4", "s5", "s6"}[i], "value", "parameter", "i32", "", "", true, raw[name+".parameterName"])
		parameter["presence"] = map[string]any{"undefined": false, "null": false, "optional": false, "default": false, "range": raw[name+".parameter"], "annotation": token[name+".annotation"]}
		parameters = append(parameters, parameter)
	}
	for i, name := range []string{"hidden", "zebra", "alpha"} {
		parameterID := []string{"s4", "s5", "s6"}[i]
		value := any(exceptionTraversalReferenceForTest("i32", parameterID, "", raw[name+".value"]))
		if i == 2 {
			value = map[string]any{"kind": "call", "type": "i32", "target": "s1", "range": raw[name+".value"], "arguments": []any{exceptionTraversalReferenceForTest("i32", parameterID, "", raw[name+".argument"])}, "signature": map[string]any{"parameters": []string{"i32"}, "returnType": "i32", "presence": []any{parameters[0]["presence"]}}}
		}
		kind := "return"
		if i == 1 {
			kind = "throw"
		}
		functions = append(functions, map[string]any{"name": name, "symbol": []string{"s1", "s2", "s3"}[i], "parameters": []any{parameters[i]}, "returnType": "i32", "parameterInitializers": []any{nil}, "range": raw[name+".function"], "body": []any{map[string]any{"kind": kind, "expression": value, "range": raw[name+".statement"]}}})
	}
	for _, parameter := range parameters {
		symbols = append(symbols, parameter)
	}
	facts := map[string]any{"status": "ok", "inputs": []string{"/src/main.ts", "/src/minimal-integers-v3.d.ts"}, "symbols": symbols, "functions": functions}
	exceptionPackageExportsPacketsForTest(t, "directExports", r, input, facts, exportGroups)
}
