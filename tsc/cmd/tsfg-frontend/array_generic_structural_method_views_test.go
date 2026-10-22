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

func TestArrayGenericV8StructuralMethodViews25f(t *testing.T) {
	raw, err := os.ReadFile("testdata/array-facts-v8/consumer-structural-method-inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct{ Name, Source string }
	}
	if err := json.Unmarshal(raw, &fixture); err != nil || len(fixture.Cases) != 2 {
		t.Fatalf("two exact structural method sources required: %v", err)
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
				t.Fatal("structural method source must remain checked v8")
			}
			var view, actual arrayGenericType
			for _, typ := range p.Types {
				if typ.Name == "View" {
					view = typ
				}
				if typ.Name == "Actual" {
					actual = typ
				}
			}
			var vm, am, read arrayGenericFunction
			for _, fn := range p.Functions {
				if fn.Name == "read" {
					read = fn
				}
				if fn.Name == "m" && fn.OwnerType == view.ID {
					vm = fn
				}
				if fn.Name == "m" && fn.OwnerType == actual.ID {
					am = fn
				}
			}
			found := false
			for _, w := range p.CompatibilityWitnesses {
				if w.Compatibility.SourceType == actual.ID && w.Compatibility.TargetType == view.ID && w.Compatibility.Assignable {
					for _, pair := range w.Compatibility.Members {
						if pair.Source == am.ID && pair.Target == vm.ID {
							found = true
						}
					}
				}
			}
			if !found || len(vm.Body) != 0 || len(am.Body) != 1 || read.Parameters[0].DeclaredType != actual.ID || read.Bindings[0].DeclaredType != view.ID || read.Body[0].Initializer.Binding != read.Parameters[0].ID {
				t.Fatal("actual M12 Actual-to-View selected method projection required")
			}
			shrink := am.Body[0].Value
			if shrink.Kind != "lengthAssignment" || shrink.Target.Receiver.Binding != am.Parameters[0].ID {
				t.Fatal("original Actual.m array truncation missing")
			}
			callIndex := 2
			if row.Name == "structural-method-after-use" {
				callIndex = 3
			}
			call := read.Body[callIndex].Value
			if call.Kind != "call" || call.Callee != vm.ID || call.Receiver.Binding != read.Body[0].Binding || call.Receiver.Type != view.ID || call.Signature == nil || call.Signature.DeclarationRange != vm.DeclarationRange || (*call.Arguments)[0].Binding != read.Parameters[1].ID {
				t.Fatal("call must keep static View.m signature and original argument")
			}
			var checked *arrayGenericExpression
			if callIndex == 2 {
				checked = read.Body[3].Value
			} else {
				checked = read.Body[2].Initializer
				if read.Body[4].Value.Binding != read.Body[2].Binding {
					t.Fatal("original captured read return missing")
				}
			}
			if checked.Kind != "checkedElementAccess" || checked.Receiver.Binding != read.Body[1].Binding || checked.Type != "i32" || row.Source[call.Range.Start:call.Range.End] != "v.m(a)" || row.Source[checked.Range.Start:checked.Range.End] != "s[i]" || (callIndex == 2) != (call.Range.End < checked.Range.Start) {
				t.Fatal("original call/read order changed")
			}
			structuralMethodOfficial25f(t, row.Source, vm, am, call)
		})
	}
}

// This second compiler instance checks the structural assignment and selected
// original member directly, independently of M12 and the Array converter.
func structuralMethodOfficial25f(t *testing.T, source string, vm, am arrayGenericFunction, call *arrayGenericExpression) {
	t.Helper()
	files := map[string]string{"/src/main.ts": source, "/src/minimal-arrays-v8.d.ts": string(arrayGenericDeclarations)}
	program := compiler.NewProgram(compiler.ProgramOptions{Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{FileNames: []string{"/src/main.ts", "/src/minimal-arrays-v8.d.ts"}, CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext}}}, Host: compiler.NewCompilerHost("/src", vfstest.FromMap(files, true), "/src", nil, nil), SingleThreaded: core.TSTrue})
	for _, diags := range [][]*ast.Diagnostic{program.GetSyntacticDiagnostics(t.Context(), nil), program.GetSemanticDiagnostics(t.Context(), nil), program.GetProgramDiagnostics(), program.GetGlobalDiagnostics(t.Context())} {
		if len(diags) != 0 {
			t.Fatalf("independent official structural diagnostics: %+v", diags)
		}
	}
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	var v, a, callNode *ast.Node
	for _, node := range file.Statements.Nodes {
		if node.Kind == ast.KindClassDeclaration {
			if node.Name().Text() == "View" {
				v = node
			}
			if node.Name().Text() == "Actual" {
				a = node
			}
		}
	}
	var walk func(*ast.Node)
	walk = func(node *ast.Node) {
		if node.Kind == ast.KindCallExpression && source[scanner.SkipTrivia(source, node.Pos()):node.End()] == "v.m(a)" {
			callNode = node
		}
		node.ForEachChild(func(child *ast.Node) bool { walk(child); return false })
	}
	walk(file.AsNode())
	viewType := check.GetDeclaredTypeOfSymbol(check.GetSymbolAtLocation(v.Name()))
	actualType := check.GetDeclaredTypeOfSymbol(check.GetSymbolAtLocation(a.Name()))
	if !check.IsTypeAssignableTo(actualType, viewType) || check.GetResolvedSignature(callNode).Declaration() != v.Members()[0] || check.GetSymbolAtLocation(callNode.AsCallExpression().Expression.Name()) != check.GetSymbolAtLocation(v.Members()[0].Name()) {
		t.Fatal("independent actual structural relation and selected View method changed")
	}
	for _, pair := range []struct {
		node *ast.Node
		fact arrayGenericFunction
	}{{v.Members()[0], vm}, {a.Members()[0], am}} {
		start := file.GetPositionMap().UTF8ToUTF16(scanner.SkipTrivia(source, pair.node.Pos()))
		end := file.GetPositionMap().UTF8ToUTF16(pair.node.End())
		if pair.fact.DeclarationRange.Start != start || pair.fact.DeclarationRange.End != end {
			t.Fatal("independent original method anchor differs")
		}
	}
	if call.Range.Start != file.GetPositionMap().UTF8ToUTF16(scanner.SkipTrivia(source, callNode.Pos())) || call.Range.End != file.GetPositionMap().UTF8ToUTF16(callNode.End()) {
		t.Fatal("independent call anchor differs")
	}
	t.Log("independent official structural oracle: 4x0 diagnostics; Actual assignable to View, static View.m and both original member declarations verified")
}
