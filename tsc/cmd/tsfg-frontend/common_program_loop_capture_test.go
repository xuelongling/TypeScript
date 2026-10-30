// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
)

// Retaining a repeated lexical declaration is Source evidence, not a proof of
// fresh runtime environments or permission to let a borrowed value escape.
func TestCommonProgramWhileLetCapture19ak(t *testing.T) {
	if root := os.Getenv("TSFG_READSPAN_CAPTURE_ROOT"); root != "" {
		t.Setenv("TSFG_READSPAN_CAPTURE_ROOT", root+"/while-let-capture")
	}
	source, err := os.ReadFile("testdata/common-program-v2/single-block-capture/while-entry.ts")
	if err != nil {
		t.Fatal(err)
	}
	if len(source) != 2680 || arrayFactsSHA(source) != "94722c6b81b4753dacd5cf15a24f10375a74eb423711d0907554515015bb539d" {
		t.Fatal("original complete while capture Source changed")
	}
	readSpanCaptureSave26i(t, "while-entry-source.ts", source)

	// Establish the exact loop and lexical declaration independently, before the
	// producer. A writable var must not masquerade as this block-scoped let.
	program := commonMethodProgram37q(t, string(source))
	file := program.GetSourceFile("/src/main.ts")
	var entry *ast.Node
	for _, node := range file.Statements.Nodes {
		if node.Kind == ast.KindFunctionDeclaration && node.Name().Text() == "blockEntry" {
			if entry != nil {
				t.Fatal("original while entry duplicated")
			}
			entry = node
		}
	}
	if entry == nil || entry.Body() == nil {
		t.Fatal("original while entry absent")
	}
	statements := entry.Body().AsBlock().Statements.Nodes
	if len(statements) != 4 || statements[2].Kind != ast.KindWhileStatement {
		t.Fatal("original loop moved from its function-body position")
	}
	loop := statements[2].AsWhileStatement()
	if loop.Statement.Kind != ast.KindBlock || loop.Expression.Kind != ast.KindBinaryExpression {
		t.Fatal("original while block/condition absent")
	}
	body := loop.Statement.AsBlock().Statements.Nodes
	if len(body) != 6 || body[0].Kind != ast.KindVariableStatement || body[1].Kind != ast.KindFunctionDeclaration || body[2].Kind != ast.KindFunctionDeclaration || body[1].Name().Text() != "write" || body[2].Name().Text() != "read" {
		t.Fatal("original same-iteration declaration/read/write order differs")
	}
	list := body[0].AsVariableStatement().DeclarationList
	if list.Flags&ast.NodeFlagsBlockScoped != ast.NodeFlagsLet || len(list.AsVariableDeclarationList().Declarations.Nodes) != 1 {
		t.Fatal("original repeated declaration must be lexical let, not var or const")
	}
	cell := list.AsVariableDeclarationList().Declarations.Nodes[0]
	if cell.Name().Text() != "cell" || cell.Type() == nil || cell.AsVariableDeclaration().Initializer == nil || cell.AsVariableDeclaration().Initializer.Kind != ast.KindIdentifier || cell.AsVariableDeclaration().Initializer.Text() != "i" || body[1].Parent != loop.Statement || body[2].Parent != loop.Statement {
		t.Fatal("original lexical cell/initializer/function block differs")
	}
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	t.Cleanup(release)
	symbol := check.GetSymbolAtLocation(cell.Name())
	if symbol == nil || symbol.Flags&ast.SymbolFlagsBlockScopedVariable == 0 || len(symbol.Declarations) != 1 || symbol.Declarations[0] != cell {
		t.Fatal("original cell is not its unique block-scoped Symbol")
	}
	rangeOf := func(node *ast.Node) span {
		start := node.Pos()
		for start < node.End() && strings.ContainsRune(" \r\n\t", rune(source[start])) {
			start++
		}
		return span{file.GetPositionMap().UTF8ToUTF16(start), file.GetPositionMap().UTF8ToUTF16(node.End())}
	}
	compare := singleBlockCaptureOfficial19ae(t, string(source), blockCaptureExpectation{"blockEntry", "cell", 1, 2, true, true})
	t.Log("pre-producer official diagnostics, exact while block and lexical-let Symbol verified")
	request := scalarParameterRequest19m(t, source)
	readSpanCaptureSave26i(t, "while-entry-request.json", request)
	var output bytes.Buffer
	if err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(request), &output); err != nil {
		failure, ok := err.(*arrayFactsFailure)
		if !ok || output.Len() != 0 {
			t.Fatalf("non-atomic original while capture failure: %v/%d", err, output.Len())
		}
		raw, encodeErr := json.Marshal(failure)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		readSpanCaptureSave26i(t, "while-entry-failure.json", append(raw, '\n'))
		t.Fatalf("complete lexical while capture must retain its Source facts: %s", raw)
	}
	readSpanCaptureSave26i(t, "while-entry-package.json", output.Bytes())
	var packet commonProgramPackage
	if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	compare(packet.Payload)
	matches, entryRange := 0, rangeOf(entry)
	for _, fn := range packet.Payload.Functions {
		if fn.DeclarationRange != entryRange {
			continue
		}
		matches++
		if fn.Name != "blockEntry" || len(fn.Body) != len(statements) || fn.Body[2].Kind != "while" || fn.Body[2].Range != rangeOf(statements[2]) || fn.Body[2].Condition == nil || fn.Body[2].Condition.Range != rangeOf(loop.Expression) || fn.Body[2].Condition.Kind != "binary" || fn.Body[2].Condition.Operator != "<" || fn.Body[2].Body == nil || len(*fn.Body[2].Body) != len(body) {
			t.Fatal("original repeated lexical boundary/condition/body was replaced or hoisted")
		}
	}
	if matches != 1 {
		t.Fatal("original while entry must have exactly one matching declaration")
	}
}
