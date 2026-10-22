// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/scanner"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

func TestArrayGenericV8ConsumerSemanticEdges25f(t *testing.T) {
	raw, err := os.ReadFile("testdata/array-facts-v8/consumer-semantic-edges-inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct{ Name, Source string }
	}
	if err := json.Unmarshal(raw, &fixture); err != nil || len(fixture.Cases) != 3 {
		t.Fatalf("three exact semantic-edge sources required: %v", err)
	}
	for _, row := range fixture.Cases {
		t.Run(row.Name, func(t *testing.T) {
			input := genericArrayRequest25f(t, row.Source)
			// This independent batch retains exact inputs and first failures.
			capture := func(name string, data []byte) {
				t.Helper()
				if root := os.Getenv("TSFG_ARRAY_V8_CAPTURE_ROOT"); root != "" {
					if err := os.MkdirAll(root, 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(root, arrayFactsSHA([]byte(row.Source))+"-"+name), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			capture("source.ts", []byte(row.Source))
			capture("request.json", input)
			var output bytes.Buffer
			if err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(input), &output); err != nil {
				var failure *arrayFactsFailure
				errors.As(err, &failure)
				record, encodeErr := json.MarshalIndent(struct {
					Error   string             `json:"error"`
					Failure *arrayFactsFailure `json:"failure"`
				}{err.Error(), failure}, "", "  ")
				if encodeErr != nil {
					t.Fatal(encodeErr)
				}
				capture("error.json", record)
				capture("failed-output.bin", output.Bytes())
				t.Fatalf("actual original semantic-edge Source rejection: %s", record)
			}
			genericArrayCapture25f(t, input, row.Source, output.Bytes())
			var packet arrayGenericPackage
			if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}

			p := packet.Payload
			if packet.Version != 8 || len(p.Diagnostics) != 0 {
				t.Fatal("actual v8 source must be checked without diagnostics")
			}
			var read arrayGenericFunction
			for _, fn := range p.Functions {
				if fn.Name == "read" {
					read = fn
				}
			}
			if read.Name != "read" || read.ReturnType != "i32" {
				t.Fatal("original checked return signature changed")
			}
			expressions := map[string]*arrayGenericExpression{}
			var visit func(*arrayGenericExpression)
			visit = func(v *arrayGenericExpression) {
				if v == nil {
					return
				}
				expressions[row.Source[v.Range.Start:v.Range.End]] = v
				for _, x := range []*arrayGenericExpression{v.Receiver, v.Index, v.Expression, v.Value, v.Left, v.Right} {
					visit(x)
				}
				if v.Arguments != nil {
					for i := range *v.Arguments {
						visit(&(*v.Arguments)[i])
					}
				}
				if v.Elements != nil {
					for i := range *v.Elements {
						visit(&(*v.Elements)[i])
					}
				}
			}
			for _, fn := range p.Functions {
				for _, s := range fn.Body {
					visit(s.Initializer)
					visit(s.Value)
				}
			}
			get := func(text string) *arrayGenericExpression {
				t.Helper()
				v := expressions[text]
				if v == nil {
					t.Fatal("missing original expression", text)
				}
				return v
			}
			switch row.Name {
			case "view-short-rebind":
				if len(read.Body) != 5 || read.Parameters[3].DeclaredType != "bool" {
					t.Fatal("original view/body/flag declaration changed")
				}
				first, second := read.Body[0], read.Body[1]
				logic, assign, shrink, result := read.Body[2].Value, get("s=t"), get("a.length=0"), get("s[i]")
				if logic.Kind != "binary" || logic.Operator != "&&" || logic.Left.Binding != read.Parameters[3].ID || assign.Kind != "bindingAssignment" || assign.Target.Binding != first.Binding || assign.Value.Binding != second.Binding || assign.Type != "readSpan<i32>" || first.Initializer.Callee != p.ReadSpanIntrinsic.ID || second.Initializer.Callee != p.ReadSpanIntrinsic.ID || (*first.Initializer.Arguments)[0].Binding != read.Parameters[0].ID || (*second.Initializer.Arguments)[0].Binding != read.Parameters[1].ID || shrink.Kind != "lengthAssignment" || shrink.Target.Receiver.Binding != read.Parameters[0].ID || result.Kind != "checkedElementAccess" || result.Receiver.Binding != first.Binding || !(assign.Range.End < shrink.Range.Start && shrink.Range.End < result.Range.Start) {
					t.Fatal("conditional rebinding must retain both distinct origins and the original later view binding")
				}
			case "density-short-shrink":
				if len(read.Body) != 4 {
					t.Fatal("original density body changed")
				}
				logic, shrink, store, result := read.Body[1].Value, get("a.length=0"), get("a[1]=2"), get("a[0]")
				if logic.Kind != "binary" || logic.Operator != "&&" || logic.Left.Binding != read.Parameters[0].ID || shrink.Kind != "lengthAssignment" || shrink.Target.Receiver.Binding != read.Body[0].Binding || store.Kind != "elementAssignment" || store.Target.Receiver.Binding != read.Body[0].Binding || store.Target.Index.Decimal != "1" || store.Value.Decimal != "2" || result.Kind != "checkedElementAccess" || result.Receiver.Binding != read.Body[0].Binding || result.Type != "i32" || !(shrink.Range.End < store.Range.Start && store.Range.End < result.Range.Start) {
					t.Fatal("conditional shrink, later store and checked read must remain ordered without a fabricated must length")
				}
			case "override-active-view":
				var base, derived arrayGenericType
				for _, typ := range p.Types {
					if typ.Name == "Base" {
						base = typ
					}
					if typ.Name == "Derived" {
						derived = typ
					}
				}
				var bm, dm arrayGenericFunction
				for _, fn := range p.Functions {
					if fn.OwnerType == base.ID && fn.Name == "m" {
						bm = fn
					}
					if fn.OwnerType == derived.ID && fn.Name == "m" {
						dm = fn
					}
				}
				call, shrink, result := get("b.m(a)"), get("a.length=0"), get("s[i]")
				if base.ID == "" || derived.Base != base.ID || bm.ID == "" || len(bm.Body) != 0 || dm.OverrideTarget != bm.ID || dm.ReturnType != "void" || len(dm.Body) != 1 || shrink.Kind != "lengthAssignment" || shrink.Target.Receiver.Binding != dm.Parameters[0].ID || len(read.Body) != 3 || call.Kind != "call" || call.Callee != bm.ID || call.Signature == nil || call.Signature.DeclarationRange != bm.DeclarationRange || call.Receiver.Binding != read.Parameters[0].ID || call.Receiver.Type != base.ID || (*call.Arguments)[0].Binding != read.Parameters[1].ID || result.Kind != "checkedElementAccess" || result.Receiver.Binding != read.Body[0].Binding {
					t.Fatal("static Base member signature and actual Derived override effect must both remain explicit")
				}
			default:
				t.Fatal("unknown exact semantic edge")
			}
			semanticEdgeOfficial25f(t, row.Name, row.Source, p, expressions)
		})
	}
}

// Build separate original compiler objects; the converter is not used as an oracle.
func semanticEdgeOfficial25f(t *testing.T, name, source string, p arrayGenericPayload, expressions map[string]*arrayGenericExpression) {
	t.Helper()
	files := map[string]string{"/src/main.ts": source, "/src/minimal-arrays-v8.d.ts": string(arrayGenericDeclarations)}
	program := compiler.NewProgram(compiler.ProgramOptions{Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{FileNames: []string{"/src/main.ts", "/src/minimal-arrays-v8.d.ts"}, CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext}}}, Host: compiler.NewCompilerHost("/src", vfstest.FromMap(files, true), "/src", nil, nil), SingleThreaded: core.TSTrue})
	for _, diags := range [][]*ast.Diagnostic{program.GetSyntacticDiagnostics(t.Context(), nil), program.GetSemanticDiagnostics(t.Context(), nil), program.GetProgramDiagnostics(), program.GetGlobalDiagnostics(t.Context())} {
		if len(diags) != 0 {
			t.Fatalf("independent official diagnostics: %+v", diags)
		}
	}
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	nodes := map[string]*ast.Node{}
	var walk func(*ast.Node)
	walk = func(node *ast.Node) {
		start := scanner.SkipTrivia(source, node.Pos())
		if start < node.End() {
			nodes[source[start:node.End()]] = node
		}
		node.ForEachChild(func(child *ast.Node) bool { walk(child); return false })
	}
	walk(file.AsNode())
	key := func(text string, kind ast.Kind) *ast.Node {
		t.Helper()
		node := nodes[text]
		fact := expressions[text]
		if node == nil || node.Kind != kind || fact == nil {
			t.Fatal("independent original syntax missing", text)
		}
		start := file.GetPositionMap().UTF8ToUTF16(scanner.SkipTrivia(source, node.Pos()))
		end := file.GetPositionMap().UTF8ToUTF16(node.End())
		if fact.Range.Start != start || fact.Range.End != end {
			t.Fatal("independent UTF16 range differs", text)
		}
		return node
	}
	shrink := key("a.length=0", ast.KindBinaryExpression).AsBinaryExpression()
	if shrink.OperatorToken.Kind != ast.KindEqualsToken || shrink.Left.Kind != ast.KindPropertyAccessExpression || check.GetTypeAtLocation(shrink.Left) != check.GetNumberType() {
		t.Fatal("independent actual length assignment changed")
	}
	var read arrayGenericFunction
	for _, fn := range p.Functions {
		if fn.Name == "read" {
			read = fn
		}
	}
	if name == "view-short-rebind" {
		assignment := key("s=t", ast.KindBinaryExpression).AsBinaryExpression()
		if assignment.OperatorToken.Kind != ast.KindEqualsToken || check.GetSymbolAtLocation(assignment.Left) == check.GetSymbolAtLocation(assignment.Right) || check.GetTypeAtLocation(assignment.Left) != check.GetTypeAtLocation(assignment.Right) {
			t.Fatal("independent original distinct view bindings changed")
		}
		result := key("s[i]", ast.KindElementAccessExpression).AsElementAccessExpression()
		if check.GetSymbolAtLocation(result.Expression) != check.GetSymbolAtLocation(assignment.Left) {
			t.Fatal("independent final read no longer references original mutable s")
		}
	} else if name == "density-short-shrink" {
		key("a[1]=2", ast.KindBinaryExpression)
		result := key("a[0]", ast.KindElementAccessExpression)
		if check.GetTypeAtLocation(result) != check.GetNumberType() {
			t.Fatal("independent checked i32 element result changed")
		}
	} else {
		call := key("b.m(a)", ast.KindCallExpression)
		signature := check.GetResolvedSignature(call)
		if signature == nil || signature.Declaration().Kind != ast.KindMethodDeclaration {
			t.Fatal("independent selected method signature missing")
		}
		var baseNode, derivedNode *ast.Node
		for _, node := range file.Statements.Nodes {
			if node.Kind == ast.KindClassDeclaration {
				if node.Name().Text() == "Base" {
					baseNode = node
				}
				if node.Name().Text() == "Derived" {
					derivedNode = node
				}
			}
		}
		bm, dm := baseNode.Members()[0], derivedNode.Members()[0]
		if signature.Declaration() != bm || dm.ModifierFlags()&ast.ModifierFlagsOverride == 0 || len(check.GetBaseTypes(check.GetDeclaredTypeOfSymbol(check.GetSymbolAtLocation(derivedNode.Name())))) != 1 {
			t.Fatal("independent static Base signature/explicit derived override changed")
		}
		fact := expressions["b.m(a)"]
		if fact.Signature.DeclarationRange.Start != file.GetPositionMap().UTF8ToUTF16(scanner.SkipTrivia(source, bm.Pos())) || fact.Signature.DeclarationRange.End != file.GetPositionMap().UTF8ToUTF16(bm.End()) {
			t.Fatal("packet selected method does not match independent original declaration")
		}
	}
	if read.ReturnType != "i32" {
		t.Fatal("original source return annotation changed")
	}
	t.Log("independent official compiler oracle: 4x0 diagnostics; original AST/symbol/type/selected-signature anchors verified; no runtime outcome asserted")
}
