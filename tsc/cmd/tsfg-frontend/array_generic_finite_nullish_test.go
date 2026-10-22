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
	"github.com/microsoft/typescript-go/internal/scanner"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

func TestArrayGenericV8FiniteNullishFacts25f(t *testing.T) {
	raw, err := os.ReadFile("testdata/array-facts-v8/finite-nullish-inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct{ Name, Source string }
	}
	if err := json.Unmarshal(raw, &fixture); err != nil || len(fixture.Cases) != 4 {
		t.Fatalf("four exact finite nullish sources required: %v", err)
	}
	for _, row := range fixture.Cases {
		t.Run(row.Name, func(t *testing.T) {
			// Check separate compiler objects before any converter failure can hide the
			// official source legality, selected bindings or non-nullish left identity.
			anchors := finiteNullishOfficial25f(t, row.Name, row.Source)
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
				t.Fatalf("actual original finite-nullish Source rejection: %s", record)
			}
			genericArrayCapture25f(t, input, row.Source, output.Bytes())
			var packet arrayGenericPackage
			if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}

			p := packet.Payload
			if packet.Version != 8 || len(p.Diagnostics) != 0 || len(p.Functions) != 1 {
				t.Fatal("checked complete actual finite source required")
			}
			run := p.Functions[0]
			if run.Name != "run" || len(run.Body) != 3 {
				t.Fatal("original initializer, conditional RHS and final read order changed")
			}
			logic, result := run.Body[1].Value, run.Body[2].Value
			if logic.Kind != "binary" || logic.Operator != "??" || logic.Left.Binding != run.Parameters[0].ID || logic.Right.Kind != "parentheses" {
				t.Fatal("original nullish operator, left binding and conditional RHS wrapper changed")
			}
			assignment := logic.Right.Expression
			if logic.Range != anchors["logic"] || assignment.Range != anchors["assignment"] || result.Range != anchors["result"] || !(logic.Range.End < result.Range.Start) {
				t.Fatal("independent original UTF16 anchors or evaluation order changed")
			}
			if row.Name == "nullish-field-i32" {
				initial := run.Body[0].Value
				if initial.Kind != "fieldAssignment" || initial.Value.Decimal != "1" || assignment.Kind != "fieldAssignment" || assignment.Value.Decimal != "2" || initial.Target.Property != assignment.Target.Property || result.Kind != "fieldAccess" || result.Property != initial.Target.Property || initial.Target.Receiver.Binding != run.Parameters[1].ID || assignment.Target.Receiver.Binding != run.Parameters[1].ID || result.Receiver.Binding != run.Parameters[1].ID {
					t.Fatal("field initial1, conditional2 and final same-field read must remain explicit")
				}
			} else {
				literal := run.Body[0].Initializer
				if literal.Kind != "arrayLiteral" || len(*literal.Elements) != 1 || (*literal.Elements)[0].Decimal != "1" || assignment.Kind != "elementAssignment" || assignment.Target.Receiver.Binding != run.Body[0].Binding || assignment.Target.Index.Decimal != "0" || result.Kind != "checkedElementAccess" || result.Receiver.Binding != run.Body[0].Binding || result.Index.Decimal != "0" {
					t.Fatal("array initial1 and conditional same-slot assignment/read changed")
				}
				if row.Name == "nullish-array-string" {
					if assignment.Value.Binding != run.Parameters[1].ID || assignment.Value.Type != "string" || result.Type != "union<i32,string>" || run.ReturnType != result.Type {
						t.Fatal("full i32/String domain and String RHS binding must remain explicit")
					}
				} else if assignment.Value.Decimal != "2" || result.Type != "i32" {
					t.Fatal("conditional numeric2 and checked i32 result changed")
				}
			}
			expectedLeft := "union<i32,undefined>"
			if row.Name == "nullish-array-nonnullable" {
				expectedLeft = "i32"
			}
			if row.Name == "nullish-array-string" {
				expectedLeft = "union<string,undefined>"
			}
			if run.Parameters[0].DeclaredType != expectedLeft || logic.Left.Type != expectedLeft {
				t.Fatal("original declared absence/payload domain changed")
			}
		})
	}
}

// This oracle uses only independently constructed original compiler objects,
// never projected facts, runtime evaluation or a consumer's reaching analysis.
func finiteNullishOfficial25f(t *testing.T, name, source string) map[string]span {
	t.Helper()
	files := map[string]string{"/src/main.ts": source, "/src/minimal-arrays-v8.d.ts": string(arrayGenericDeclarations)}
	program := compiler.NewProgram(compiler.ProgramOptions{Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{FileNames: []string{"/src/main.ts", "/src/minimal-arrays-v8.d.ts"}, CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext}}}, Host: compiler.NewCompilerHost("/src", vfstest.FromMap(files, true), "/src", nil, nil), SingleThreaded: core.TSTrue})
	for _, diags := range [][]*ast.Diagnostic{program.GetSyntacticDiagnostics(t.Context(), nil), program.GetSemanticDiagnostics(t.Context(), nil), program.GetProgramDiagnostics(), program.GetGlobalDiagnostics(t.Context())} {
		if len(diags) != 0 {
			t.Fatalf("independent official finite nullish diagnostics: %+v", diags)
		}
	}
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	var fn *ast.Node
	for _, node := range file.Statements.Nodes {
		if node.Kind == ast.KindFunctionDeclaration {
			fn = node
		}
	}
	run := fn.AsFunctionDeclaration()
	body := run.Body.AsBlock().Statements.Nodes
	logic := body[1].AsExpressionStatement().Expression
	binary := logic.AsBinaryExpression()
	assignment := binary.Right.AsParenthesizedExpression().Expression
	target := assignment.AsBinaryExpression().Left
	result := body[2].AsReturnStatement().Expression
	if binary.OperatorToken.Kind != ast.KindQuestionQuestionToken || assignment.Kind != ast.KindBinaryExpression || assignment.AsBinaryExpression().OperatorToken.Kind != ast.KindEqualsToken || check.GetSymbolAtLocation(binary.Left) != check.GetSymbolAtLocation(run.Parameters.Nodes[0].Name()) {
		t.Fatal("independent original nullish/binding/assignment syntax changed")
	}
	leftType := check.GetTypeAtLocation(binary.Left)
	if name == "nullish-array-nonnullable" {
		if leftType != check.GetNumberType() {
			t.Fatal("independent non-nullish i32 left must stay Number")
		}
	} else if leftType.Flags()&checker.TypeFlagsUnion == 0 {
		t.Fatal("independent optional left must retain its union")
	}
	if name == "nullish-field-i32" {
		first := body[0].AsExpressionStatement().Expression.AsBinaryExpression()
		if first.Right.Kind != ast.KindNumericLiteral || source[first.Right.Pos():first.Right.End()] != "1" || check.GetSymbolAtLocation(first.Left.Name()) != check.GetSymbolAtLocation(target.Name()) || check.GetSymbolAtLocation(target.Name()) != check.GetSymbolAtLocation(result.Name()) {
			t.Fatal("independent original initial/conditional/final field identity changed")
		}
	} else {
		decl := body[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0]
		literal := decl.AsVariableDeclaration().Initializer.AsArrayLiteralExpression()
		if len(literal.Elements.Nodes) != 1 || literal.Elements.Nodes[0].Kind != ast.KindNumericLiteral || check.GetSymbolAtLocation(target.AsElementAccessExpression().Expression) != check.GetSymbolAtLocation(decl.Name()) || check.GetSymbolAtLocation(result.AsElementAccessExpression().Expression) != check.GetSymbolAtLocation(decl.Name()) {
			t.Fatal("independent original local-array initializer/target/read binding changed")
		}
	}
	if name == "nullish-array-string" {
		if check.GetTypeAtLocation(assignment.AsBinaryExpression().Right) != check.GetStringType() || check.GetTypeAtLocation(result).Flags()&checker.TypeFlagsUnion == 0 {
			t.Fatal("independent String write and union element domain changed")
		}
	} else if check.GetTypeAtLocation(result) != check.GetNumberType() {
		t.Fatal("independent i32 read must remain Number")
	}
	anchors := map[string]span{}
	for key, node := range map[string]*ast.Node{"logic": logic, "assignment": assignment, "result": result} {
		anchors[key] = span{Start: file.GetPositionMap().UTF8ToUTF16(scanner.SkipTrivia(source, node.Pos())), End: file.GetPositionMap().UTF8ToUTF16(node.End())}
	}
	t.Logf("independent official finite nullish oracle:4x0 diagnostics; left=%s; original binding/assignment/read anchors=%+v; no runtime or reaching payload asserted", name, anchors)
	return anchors
}
