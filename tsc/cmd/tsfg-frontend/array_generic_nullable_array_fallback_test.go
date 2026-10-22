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

func TestArrayGenericV8NullableArrayFallback25f(t *testing.T) {
	source := "export function run(a:i32[]|null|undefined,b:i32[],i:u32):i32{const selected:i32[]=a??b;return selected[i];}"
	// The separate original checker oracle is independent of the converter and
	// observes only static nullable/payload types and original binding identities.
	anchors := nullableArrayFallbackOfficial25f(t, source)
	input := genericArrayRequest25f(t, source)
	// This independent batch retains exact inputs and first failures.
	capture := func(name string, data []byte) {
		t.Helper()
		if root := os.Getenv("TSFG_ARRAY_V8_CAPTURE_ROOT"); root != "" {
			if err := os.MkdirAll(root, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, arrayFactsSHA([]byte(source))+"-"+name), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	capture("source.ts", []byte(source))
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
		t.Fatalf("actual original nullable-array fallback Source rejection: %s", record)
	}
	genericArrayCapture25f(t, input, source, output.Bytes())
	var packet arrayGenericPackage
	if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}

	p := packet.Payload
	if packet.Version != 8 || len(p.Diagnostics) != 0 || len(p.Functions) != 1 {
		t.Fatal("one original checked nullable-array export required")
	}
	run := p.Functions[0]
	if run.Name != "run" || len(run.Parameters) != 3 || run.Parameters[0].DeclaredType != "union<array<i32>,null,undefined>" || run.Parameters[1].DeclaredType != "array<i32>" || run.Parameters[2].DeclaredType != "u32" || run.ReturnType != "i32" || len(run.Body) != 2 {
		t.Fatal("full nullable Array domain and original required parameter/return types changed")
	}
	local, result := run.Body[0], run.Body[1].Value
	coalesce := local.Initializer
	if coalesce.Kind != "binary" || coalesce.Operator != "??" || coalesce.Left.Type != "union<array<i32>,null,undefined>" || coalesce.Left.Binding != run.Parameters[0].ID || coalesce.Right.Binding != run.Parameters[1].ID || coalesce.Type != "array<i32>" || coalesce.Range != anchors["coalesce"] || result.Kind != "checkedElementAccess" || result.Receiver.Binding != local.Binding || result.Index.Binding != run.Parameters[2].ID || result.Type != "i32" || result.Range != anchors["read"] || !(coalesce.Range.End < result.Range.Start) {
		t.Fatal("original nullable fallback, selected binding or unknown-index checked read changed")
	}
	found := map[string]bool{}
	for _, typ := range p.Types {
		if typ.ID == "null" || typ.ID == "undefined" || typ.ID == "array<i32>" || typ.ID == "union<array<i32>,null,undefined>" {
			found[typ.ID] = true
		}
		if typ.ID == "union<array<i32>,null,undefined>" {
			if len(typ.Members) != 3 || typ.Members[0] != "array<i32>" || typ.Members[1] != "null" || typ.Members[2] != "undefined" {
				t.Fatal("original absence variants and Array payload changed")
			}
		}
	}
	if len(found) != 4 {
		t.Fatal("missing original null, undefined, Array payload or complete union catalog")
	}
}

func nullableArrayFallbackOfficial25f(t *testing.T, source string) map[string]span {
	t.Helper()
	files := map[string]string{"/src/main.ts": source, "/src/minimal-arrays-v8.d.ts": string(arrayGenericDeclarations)}
	program := compiler.NewProgram(compiler.ProgramOptions{Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{FileNames: []string{"/src/main.ts", "/src/minimal-arrays-v8.d.ts"}, CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext}}}, Host: compiler.NewCompilerHost("/src", vfstest.FromMap(files, true), "/src", nil, nil), SingleThreaded: core.TSTrue})
	for _, diags := range [][]*ast.Diagnostic{program.GetSyntacticDiagnostics(t.Context(), nil), program.GetSemanticDiagnostics(t.Context(), nil), program.GetProgramDiagnostics(), program.GetGlobalDiagnostics(t.Context())} {
		if len(diags) != 0 {
			t.Fatalf("independent nullable Array diagnostics: %+v", diags)
		}
	}
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	fn := file.Statements.Nodes[0].AsFunctionDeclaration()
	body := fn.Body.AsBlock().Statements.Nodes
	selected := body[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0]
	decl := selected.AsVariableDeclaration()
	coalesce := decl.Initializer.AsBinaryExpression()
	result := body[1].AsReturnStatement().Expression
	access := result.AsElementAccessExpression()
	originalUnion := check.GetTypeFromTypeNode(fn.Parameters.Nodes[0].AsParameterDeclaration().Type)
	payload := check.GetTypeFromTypeNode(decl.Type)
	if originalUnion.Flags()&checker.TypeFlagsUnion == 0 || check.GetTypeAtLocation(coalesce.Left) != originalUnion || !check.IsArrayType(payload) || check.GetTypeAtLocation(decl.Initializer) != payload || check.GetTypeAtLocation(coalesce.Right) != payload || coalesce.OperatorToken.Kind != ast.KindQuestionQuestionToken {
		t.Fatal("independent nullable union, concrete Array payload or original ?? syntax changed")
	}
	args := check.GetTypeArguments(payload)
	if len(args) != 1 || args[0] != check.GetNumberType() || check.GetTypeAtLocation(result) != check.GetNumberType() || check.GetTypeAtLocation(access.ArgumentExpression) != check.GetNumberType() || check.GetSymbolAtLocation(coalesce.Left) != check.GetSymbolAtLocation(fn.Parameters.Nodes[0].Name()) || check.GetSymbolAtLocation(coalesce.Right) != check.GetSymbolAtLocation(fn.Parameters.Nodes[1].Name()) || check.GetSymbolAtLocation(access.Expression) != check.GetSymbolAtLocation(selected.Name()) || check.GetSymbolAtLocation(access.ArgumentExpression) != check.GetSymbolAtLocation(fn.Parameters.Nodes[2].Name()) {
		t.Fatal("independent concrete element/index types or original parameter/local binding changed")
	}
	anchors := map[string]span{}
	for key, node := range map[string]*ast.Node{"coalesce": decl.Initializer, "read": result} {
		anchors[key] = span{Start: file.GetPositionMap().UTF8ToUTF16(scanner.SkipTrivia(source, node.Pos())), End: file.GetPositionMap().UTF8ToUTF16(node.End())}
	}
	t.Logf("independent official nullable Array oracle:4x0 diagnostics; original union/payload/selected/index bindings and UTF16 anchors=%+v; no runtime outcome asserted", anchors)
	return anchors
}
