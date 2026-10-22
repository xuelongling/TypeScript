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
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"

	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

func TestArrayGenericV8ConsumerNullishEdges25f(t *testing.T) {
	raw, err := os.ReadFile("testdata/array-facts-v8/consumer-nullish-edges-inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct{ Name, Source string }
	}
	if err := json.Unmarshal(raw, &fixture); err != nil || len(fixture.Cases) != 2 {
		t.Fatalf("two exact nullish-edge sources required: %v", err)
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
			case "view-short-nullish":
				if len(read.Body) != 5 || read.Parameters[3].DeclaredType != "union<bool,undefined>" {
					t.Fatal("original view/body/flag declaration changed")
				}
				first, second := read.Body[0], read.Body[1]
				logic, assign, shrink, result := read.Body[2].Value, get("s=t"), get("a.length=0"), get("s[i]")
				if logic.Kind != "binary" || logic.Operator != "??" || logic.Left.Binding != read.Parameters[3].ID || assign.Kind != "bindingAssignment" || assign.Target.Binding != first.Binding || assign.Value.Binding != second.Binding || assign.Type != "readSpan<i32>" || first.Initializer.Callee != p.ReadSpanIntrinsic.ID || second.Initializer.Callee != p.ReadSpanIntrinsic.ID || (*first.Initializer.Arguments)[0].Binding != read.Parameters[0].ID || (*second.Initializer.Arguments)[0].Binding != read.Parameters[1].ID || shrink.Kind != "lengthAssignment" || shrink.Target.Receiver.Binding != read.Parameters[0].ID || result.Kind != "checkedElementAccess" || result.Receiver.Binding != first.Binding || !(assign.Range.End < shrink.Range.Start && shrink.Range.End < result.Range.Start) {
					t.Fatal("conditional rebinding must retain both distinct origins and the original later view binding")
				}
			case "density-short-nullish":
				if len(read.Body) != 4 {
					t.Fatal("original density body changed")
				}
				logic, shrink, store, result := read.Body[1].Value, get("a.length=0"), get("a[1]=2"), get("a[0]")
				if logic.Kind != "binary" || logic.Operator != "??" || logic.Left.Binding != read.Parameters[0].ID || shrink.Kind != "lengthAssignment" || shrink.Target.Receiver.Binding != read.Body[0].Binding || store.Kind != "elementAssignment" || store.Target.Receiver.Binding != read.Body[0].Binding || store.Target.Index.Decimal != "1" || store.Value.Decimal != "2" || result.Kind != "checkedElementAccess" || result.Receiver.Binding != read.Body[0].Binding || result.Type != "i32" || !(shrink.Range.End < store.Range.Start && store.Range.End < result.Range.Start) {
					t.Fatal("conditional shrink, later store and checked read must remain ordered without a fabricated must length")
				}
			default:
				t.Fatal("unknown exact semantic edge")
			}
			oracleName := "view-short-rebind"
			if row.Name == "density-short-nullish" {
				oracleName = "density-short-shrink"
			}
			semanticEdgeOfficial25f(t, oracleName, row.Source, p, expressions)
		})
	}
}

// false is a non-nullish payload: the original checker narrows the left operand
// to a BooleanLiteral under this guard; its value is not an absence variant.
func TestArrayGenericV8NullishFalseOracle25f(t *testing.T) {
	source := "export function read(flag:boolean|undefined):boolean{if(flag===false){return flag??true;}return false;}"
	files := map[string]string{"/src/main.ts": source, "/src/minimal-arrays-v8.d.ts": string(arrayGenericDeclarations)}
	program := compiler.NewProgram(compiler.ProgramOptions{Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{FileNames: []string{"/src/main.ts", "/src/minimal-arrays-v8.d.ts"}, CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext}}}, Host: compiler.NewCompilerHost("/src", vfstest.FromMap(files, true), "/src", nil, nil), SingleThreaded: core.TSTrue})
	for _, diags := range [][]*ast.Diagnostic{program.GetSyntacticDiagnostics(t.Context(), nil), program.GetSemanticDiagnostics(t.Context(), nil), program.GetProgramDiagnostics(), program.GetGlobalDiagnostics(t.Context())} {
		if len(diags) != 0 {
			t.Fatalf("independent false/nullish diagnostics: %+v", diags)
		}
	}
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	fn := file.Statements.Nodes[0].AsFunctionDeclaration()
	guard := fn.Body.AsBlock().Statements.Nodes[0].AsIfStatement()
	comparison := guard.Expression.AsBinaryExpression()
	result := guard.ThenStatement.AsBlock().Statements.Nodes[0].AsReturnStatement().Expression.AsBinaryExpression()
	actual := check.GetTypeAtLocation(result.Left)
	if comparison.OperatorToken.Kind != ast.KindEqualsEqualsEqualsToken || comparison.Right.Kind != ast.KindFalseKeyword || result.OperatorToken.Kind != ast.KindQuestionQuestionToken || actual.Flags()&checker.TypeFlagsBooleanLiteral == 0 || check.GetBaseTypeOfLiteralType(actual) != check.GetBooleanType() || check.GetSymbolAtLocation(result.Left) != check.GetSymbolAtLocation(fn.Parameters.Nodes[0].Name()) {
		t.Fatal("official guarded false must remain a non-nullish bool payload with original ?? syntax")
	}
	t.Log("independent official 4x0 oracle: false is a boolean payload, not null/undefined; ?? skips RHS for that payload, no runtime observation claimed")
}
