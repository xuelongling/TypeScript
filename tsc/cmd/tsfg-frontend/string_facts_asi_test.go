// SPDX-License-Identifier: Apache-2.0
// A quoted return may end at its literal; no explicit semicolon is required.
package main

import (
	"bytes"
	"encoding/json"
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"testing"
)

func TestStringFactsReturnWithoutSemicolon(t *testing.T) {
	const source = `export function f(): string { return "A" }`
	input := stringBoundaryRequest24(t, source)
	before := bytes.Clone(input)
	var r struct{ Input request }
	if err := json.Unmarshal(input, &r); err != nil {
		t.Fatal(err)
	}
	official := throwBoundaryProgramForTest(t, r.Input)
	throwBoundaryDiagnosticsForTest(t, official)
	file := official.GetSourceFile(r.Input.RootFile)
	if len(file.Statements.Nodes) != 1 || file.Statements.Nodes[0].Kind != ast.KindFunctionDeclaration {
		t.Fatal("official single declaration")
	}
	node := file.Statements.Nodes[0]
	fn := node.AsFunctionDeclaration()
	body := fn.Body.AsBlock().Statements.Nodes
	if len(body) != 1 || body[0].Kind != ast.KindReturnStatement {
		t.Fatal("official single return")
	}
	literal := body[0].AsReturnStatement().Expression
	if literal == nil || literal.Kind != ast.KindStringLiteral || literal.End() != body[0].End() {
		t.Fatal("official equal-end ASI anchor")
	}
	check, release := official.GetTypeCheckerForFileExclusive(t.Context(), file)
	value := check.GetTypeAtLocation(literal)
	sig := check.GetSignaturesOfType(check.GetTypeOfSymbol(check.GetSymbolAtLocation(fn.Name())), checker.SignatureKindCall)
	if value == nil || !value.IsStringLiteral() || value.AsLiteralType().Value() != "A" || len(sig) != 1 || sig[0].Declaration() != node || len(sig[0].Parameters()) != 0 || check.GetReturnTypeOfSignature(sig[0]).Flags() != checker.TypeFlagsString || len(check.GetExportsOfModule(file.AsNode().Symbol())) != 1 {
		release()
		t.Fatal("official literal/signature/export")
	}
	release()
	functions := []any{map[string]any{"name": "f", "sourcePath": "/owned/strings.ts", "declarationRange": stringBoundaryRange24(t, source, source), "parameters": []any{}, "returnType": "string",
		"body": []any{map[string]any{"kind": "return", "range": stringBoundaryRange24(t, source, `return "A"`), "value": map[string]any{"kind": "stringLiteral", "type": "string", "range": stringBoundaryRange24(t, source, `"A"`), "codeUnits": []int32{65}}}}}}
	expected := stringBoundaryPacket24(t, input, functions)
	var output bytes.Buffer
	if err := runFrontendCommand(t.Context(), []string{"--string-facts-io-v1"}, bytes.NewReader(input), &output); err != nil || !bytes.Equal(output.Bytes(), expected) || !bytes.Equal(input, before) {
		t.Fatalf("full manual ASI packet/caller ownership: %v\n%s", err, output.Bytes())
	}
}
