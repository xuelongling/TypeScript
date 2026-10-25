// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

const exceptionNumericSource = "// 😀\r\nexport function keep():i32{try{throw undefined;}catch(caught){const viewed:i32=caught as i32;const alias:i32=viewed;return alias;}finally{const done:bool=true;}}\r\nexport function use(value:i32):i32{try{throw undefined;}catch(caught){const viewed:i32=<i32>caught;const alias:i32=viewed;alias+value;return alias;}finally{const done:bool=true;}}\r\nexport function caller(value:i32):i32{return (keep()+use(value)) as i32;}"

// Freeze the original official prerequisite before the producer is called.
// A type assertion supplies a static view, not evidence of an actual i32 value.
func exceptionNumericOriginal(t *testing.T, r request) {
	t.Helper()
	p := throwBoundaryProgramForTest(t, r)
	throwBoundaryDiagnosticsForTest(t, p)
	f := p.GetSourceFile(r.RootFile)
	c, release := p.GetTypeCheckerForFileExclusive(t.Context(), f)
	defer release()
	if f.Text() != exceptionNumericSource || len(f.Statements.Nodes) != 3 || len(p.GetSourceFiles()) != 2 || len(c.GetExportsOfModule(f.AsNode().Symbol())) != 3 {
		t.Fatal("complete original Program and exports")
	}
	for i, name := range []string{"keep", "use", "caller"} {
		n := f.Statements.Nodes[i]
		if n.Kind != ast.KindFunctionDeclaration || n.Name().Text() != name {
			t.Fatal("ordered original function declarations")
		}
		if i == 2 {
			continue
		}
		attempt := n.AsFunctionDeclaration().Body.AsBlock().Statements.Nodes[0].AsTryStatement()
		u := attempt.TryBlock.AsBlock().Statements.Nodes[0].AsThrowStatement().Expression
		binding := attempt.CatchClause.AsCatchClause().VariableDeclaration
		assertion := attempt.CatchClause.AsCatchClause().Block.AsBlock().Statements.Nodes[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0].AsVariableDeclaration().Initializer
		var operand, annotation *ast.Node
		if i == 0 {
			if assertion.Kind != ast.KindAsExpression {
				t.Fatal("original as syntax")
			}
			operand, annotation = assertion.AsAsExpression().Expression, assertion.AsAsExpression().Type
		} else {
			if assertion.Kind != ast.KindTypeAssertionExpression {
				t.Fatal("original angle assertion syntax")
			}
			operand, annotation = assertion.AsTypeAssertion().Expression, assertion.AsTypeAssertion().Type
		}
		actualUndefined := c.GetSymbolAtLocation(u)
		actualCatch := c.GetSymbolAtLocation(binding.Name())
		view := c.GetTypeFromTypeNode(annotation)
		if u.Kind != ast.KindIdentifier || u.Text() != "undefined" || actualUndefined == nil || len(actualUndefined.Declarations) != 0 || c.GetTypeAtLocation(u).Flags()&checker.TypeFlagsUndefined == 0 || actualCatch == nil || len(actualCatch.Declarations) != 1 || actualCatch.Declarations[0] != binding || c.GetSymbolAtLocation(operand) != actualCatch || c.GetTypeAtLocation(operand) != c.GetUnknownType() || view == nil || view.Alias() == nil || view.Alias().Symbol().Name != "i32" || c.GetTypeAtLocation(assertion) != view {
			t.Fatal("official undefined identity, current catch and independent static view")
		}
		for _, node := range []*ast.Node{u, binding, assertion, operand, annotation} {
			throwBoundaryNodeForTest(t, f, node)
		}
	}
}

func exceptionNumericPacketForTest(t *testing.T, label string, r request) (exceptionFactsPackage, response) {
	t.Helper()
	input := nullableFactsBoundaryCanonical(t, map[string]any{"family": "tsfg.exception-facts-request-candidate", "version": 2, "schemaHash": exceptionNumericFactsSchema, "input": r})
	var out bytes.Buffer
	err := runFrontendCommand(t.Context(), []string{"--exception-facts-io-v2"}, bytes.NewReader(input), &out)
	if root := os.Getenv("TSFG_EXCEPTION_NUMERIC_CAPTURE_ROOT"); root != "" {
		root = filepath.Join(root, label)
		if err := os.MkdirAll(root, 0755); err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string][]byte{"exception-request.json": input, "package.json": out.Bytes(), "tested-input-set.json": nullableFactsBoundaryCanonical(t, map[string]any{"rootFile": r.RootFile, "files": r.Files})} {
			if err := os.WriteFile(filepath.Join(root, name), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
		for _, file := range r.Files {
			path := filepath.Join(root, "sources", filepath.FromSlash(strings.TrimPrefix(file.Path, "/")))
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(file.Content), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err != nil {
		t.Fatalf("complete official numeric exception Program: %v", err)
	}
	var packet exceptionFactsPackage
	if err := json.Unmarshal(out.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	if packet.Family != "tsfg.exception-facts-package-candidate" || packet.Version != 2 || packet.SchemaHash != exceptionNumericFactsSchema || packet.RequestID != exceptionPackageExportsDigestForTest(append([]byte("tsfg.exception-facts.request.v2\x00"), input...)) {
		t.Fatal("explicit v2 identity")
	}
	if packet.PackageID != exceptionPackageExportsDigestForTest(append([]byte("tsfg.exception-facts.package.v2\x00"+exceptionNumericFactsSchema+"\x00"), nullableFactsBoundaryCanonical(t, packet.Payload)...)) {
		t.Fatal("complete canonical package identity")
	}
	if packet.Payload.InputIdentity != exceptionPackageExportsDigestForTest(nullableFactsBoundaryCanonical(t, r)) {
		t.Fatal("complete normalized input identity")
	}
	var facts response
	if err := json.Unmarshal(packet.Payload.Facts, &facts); err != nil {
		t.Fatal(err)
	}
	if err := exceptionNumericFactsOracle(t, r, facts); err != nil {
		t.Fatal(err)
	}
	files := slices.Clone(r.Files)
	slices.SortFunc(files, func(a, b inputFile) int { return strings.Compare(a.Path, b.Path) })
	if len(packet.Payload.Files) != len(files) {
		t.Fatal("complete file count")
	}
	for i, f := range files {
		want := exceptionFactsFile{Path: f.Path, Digest: exceptionPackageExportsDigestForTest([]byte(f.Content)), ByteLength: len(f.Content), Length: len(utf16.Encode([]rune(f.Content))), PositionUnit: "utf16"}
		if packet.Payload.Files[i] != want {
			t.Fatal("exact original file bytes and UTF16")
		}
	}
	program := throwBoundaryProgramForTest(t, r)
	file := program.GetSourceFile(r.RootFile)
	want := []exceptionFactsExport{}
	for i, n := range file.Statements.Nodes {
		want = append(want, exceptionFactsExport{Name: n.Name().Text(), Target: facts.Functions[i].Symbol, SourcePath: r.RootFile, Range: throwBoundaryNodeForTest(t, file, n), Definition: throwBoundaryNodeForTest(t, file, n.Name())})
	}
	slices.SortFunc(want, func(a, b exceptionFactsExport) int { return strings.Compare(a.Name, b.Name) })
	if !reflect.DeepEqual(packet.Payload.Exports, []exceptionFactsExportGroup{{SourcePath: r.RootFile, External: true, Bindings: want}}) {
		t.Fatal("complete actual exports")
	}
	return packet, facts
}

func TestExceptionNumericFactsCompleteProgram(t *testing.T) {
	r := integerRequest(exceptionNumericSource)
	exceptionNumericOriginal(t, r)
	packet, _ := exceptionNumericPacketForTest(t, "complete-program", r)
	for _, mutation := range []string{"erased-operand-type", "wrong-catch-binding", "assertion-range", "operator", "alias-source", "return-source", "catch-readonly"} {
		t.Run(mutation, func(t *testing.T) {
			var changed response
			if err := json.Unmarshal(packet.Payload.Facts, &changed); err != nil {
				t.Fatal(err)
			}
			c := changed.Functions[0].Body[0].Exception.CatchClause
			switch mutation {
			case "erased-operand-type":
				c.Block.Body[0].Initializer.Operand.Type = "i32"
			case "wrong-catch-binding":
				c.Block.Body[0].Initializer.Operand.Symbol = changed.Functions[1].Parameters[0].ID
			case "assertion-range":
				c.Block.Body[0].Initializer.Assertion.Annotation.Start++
			case "operator":
				c.Block.Body[0].Initializer.Operator = "type-assertion"
			case "alias-source":
				c.Block.Body[1].Initializer.Symbol = c.Binding.ID
			case "return-source":
				c.Block.Body[2].Expression.Symbol = c.Block.Body[0].Symbol.ID
			case "catch-readonly":
				c.Binding.Writable = false
			}
			if exceptionNumericFactsOracle(t, r, changed) == nil {
				t.Fatal("copied fact mutation accepted by independent official oracle")
			}
		})
	}
}
