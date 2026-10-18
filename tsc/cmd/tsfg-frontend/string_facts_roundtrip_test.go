// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"testing"
)

func TestStringFactsOwnedRoundTrip(t *testing.T) {
	const source = "\r\n/* 🐈 */ export function other(): string { return 'same'; }\r\nexport function again(): string { return \"same\"; }\r\n"
	input := stringBoundaryRequest24(t, source)
	before := bytes.Clone(input)
	expected := stringBoundaryExpected24(t, input, source)
	var request struct{ Input request }
	if err := json.Unmarshal(input, &request); err != nil {
		t.Fatal(err)
	}
	program := throwBoundaryProgramForTest(t, request.Input)
	throwBoundaryDiagnosticsForTest(t, program)
	file := program.GetSourceFile(request.Input.RootFile)
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	if len(file.Statements.Nodes) != 2 || len(check.GetExportsOfModule(file.AsNode().Symbol())) != 2 {
		release()
		t.Fatal("official complete two-owner module")
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration {
			release()
			t.Fatal("official function")
		}
		fn := node.AsFunctionDeclaration()
		body := fn.Body.AsBlock().Statements.Nodes
		if len(fn.Parameters.Nodes) != 0 || len(body) != 1 || body[0].Kind != ast.KindReturnStatement {
			release()
			t.Fatal("official zero-param complete body")
		}
		literal := body[0].AsReturnStatement().Expression
		typ := check.GetTypeAtLocation(literal)
		if literal.Kind != ast.KindStringLiteral || typ == nil || !typ.IsStringLiteral() || typ.AsLiteralType().Value() != "same" {
			release()
			t.Fatal("independent official literal value")
		}
		sig := check.GetSignaturesOfType(check.GetTypeOfSymbol(check.GetSymbolAtLocation(fn.Name())), checker.SignatureKindCall)
		if len(sig) != 1 || sig[0].Declaration() != node || len(sig[0].Parameters()) != 0 || check.GetReturnTypeOfSignature(sig[0]).Flags() != checker.TypeFlagsString {
			release()
			t.Fatal("official signature")
		}
	}
	release()
	for repeat := 0; repeat < 2; repeat++ {
		var output bytes.Buffer
		if err := runFrontendCommand(t.Context(), []string{"--string-facts-io-v1"}, bytes.NewReader(input), &output); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(output.Bytes(), expected) || !bytes.Equal(input, before) {
			t.Fatalf("complete independent packet/caller ownership mismatch: %s", output.Bytes())
		}
		var cold map[string]any
		if err := json.Unmarshal(output.Bytes(), &cold); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(append(stringOracleCanonical24(t, cold), '\n'), expected) {
			t.Fatal("complete JSON cold roundtrip")
		}
		functions := cold["payload"].(map[string]any)["functions"].([]any)
		first := functions[0].(map[string]any)["body"].([]any)[0].(map[string]any)["value"].(map[string]any)["codeUnits"].([]any)
		second := functions[1].(map[string]any)["body"].([]any)[0].(map[string]any)["value"].(map[string]any)["codeUnits"].([]any)
		first[0] = float64(0)
		if second[0] != float64(115) {
			t.Fatal("equal values lost independent owners")
		}
	}
}
