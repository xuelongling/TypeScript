// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	tsjson "github.com/microsoft/typescript-go/internal/json"
)

func TestCommonProgramVarNarrowedInitializer19am(t *testing.T) {
	if root := os.Getenv("TSFG_READSPAN_CAPTURE_ROOT"); root != "" {
		t.Setenv("TSFG_READSPAN_CAPTURE_ROOT", root+"/var-flow")
	}
	source, err := os.ReadFile("testdata/common-program-v2/var-binding/inferred-narrowed.ts")
	if err != nil {
		t.Fatal(err)
	}
	if arrayFactsSHA(source) != "ec9e1172614554edbcffd3eaab940ee23c8140fc533ce1795f34688b80aa30fe" {
		t.Fatal("frozen complete narrowed-var Source changed")
	}
	readSpanCaptureSave26i(t, "inferred-narrowed-source.ts", source)
	program := commonMethodProgram37q(t, string(source))
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	var entry, declaration *ast.Node
	var walk func(*ast.Node)
	walk = func(node *ast.Node) {
		if node.Kind == ast.KindFunctionDeclaration && node.Name().Text() == "varBinding" {
			entry = node
		}
		if node.Kind == ast.KindVariableDeclaration && node.Name().Text() == "x" {
			if declaration != nil {
				t.Fatal("narrowing fixture must have one original x declaration")
			}
			declaration = node
		}
		node.ForEachChild(func(child *ast.Node) bool { walk(child); return false })
	}
	walk(file.AsNode())
	if entry == nil || declaration == nil || declaration.Initializer() == nil || declaration.Initializer().Kind != ast.KindIdentifier || declaration.Type() != nil {
		t.Fatal("original inferred var initializer inventory differs")
	}
	parameters := entry.AsFunctionDeclaration().Parameters.Nodes
	if len(parameters) != 1 || parameters[0].Name().Text() != "seed" {
		t.Fatal("original seed parameter differs")
	}
	seed := check.GetSymbolAtLocation(parameters[0].Name())
	if seed == nil || check.GetSymbolAtLocation(declaration.Initializer()) != seed {
		t.Fatal("initializer lost original seed Symbol")
	}
	declared, flow := check.GetTypeOfSymbol(seed), check.GetTypeAtLocation(declaration.Initializer())
	inferred := check.GetTypeOfSymbol(check.GetSymbolAtLocation(declaration.Name()))
	oracle := struct {
		SeedDeclared, InitializerFlow, InferredDeclared string
		SeedFlags, FlowFlags, InferredFlags             checker.TypeFlags
		Initializer                                     span
	}{check.TypeToString(declared), check.TypeToString(flow), check.TypeToString(inferred), declared.Flags(), flow.Flags(), inferred.Flags(), *genericSpan(declaration.Initializer())}
	original, err := json.Marshal(oracle)
	if err != nil {
		t.Fatal(err)
	}
	readSpanCaptureSave26i(t, "inferred-narrowed-original-oracle.json", append(original, '\n'))
	t.Logf("pre-producer official narrowed initializer: seed=%s/%d flow=%s/%d inferred-x=%s/%d", oracle.SeedDeclared, oracle.SeedFlags, oracle.InitializerFlow, oracle.FlowFlags, oracle.InferredDeclared, oracle.InferredFlags)
	if !declared.IsUnion() || len(declared.Types()) != 2 {
		t.Fatal("seed original declaration must retain number | undefined")
	}
	number, undefined := 0, 0
	for _, member := range declared.Types() {
		if member.Flags() == checker.TypeFlagsNumber {
			number++
		}
		if member.Flags() == checker.TypeFlagsUndefined {
			undefined++
		}
	}
	if number != 1 || undefined != 1 || flow.Flags() != checker.TypeFlagsNumber || inferred.Flags() != checker.TypeFlagsNumber {
		t.Fatal("official original flow/inferred declaration did not narrow exactly to number; actual oracle retained")
	}
	compare := varBindingOfficial19am(t, string(source), "varBinding", "x")
	var request commonProgramRequest
	if err := json.Unmarshal(genericArrayRequest25f(t, string(source)), &request); err != nil {
		t.Fatal(err)
	}
	request.Family, request.Version, request.SchemaHash = "tsfg.program-facts-request-candidate", 2, "sha256:"+arrayFactsSHA(commonProgramSchema)
	request.Input.HostImports = []commonProgramHostImport{}
	request.Input.Target = &commonProgramTarget{Target: "windows-x86_64-msvc", Triple: "x86_64-pc-windows-msvc", PointerBits: "64", LLVMDataLayout: "e-m:w-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128"}
	root := closureRoot{Kind: "application", SourcePath: "/src/main.ts", Name: "varBinding"}
	request.Input.CompilationClosure = &closureConfiguration{Roots: []closureRoot{root, {Kind: "module-initialization", SourcePath: "/src/main.ts", Name: "$moduleInit"}}, RequiredRoots: []closureRoot{root}}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	canonical := tsjson.Value(raw)
	if err := canonical.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	raw = append(canonical, '\n')
	readSpanCaptureSave26i(t, "inferred-narrowed-request.json", raw)
	var output bytes.Buffer
	if err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(raw), &output); err != nil {
		if output.Len() != 0 {
			t.Fatal("failed narrowed-var producer published partial facts")
		}
		failure, encodeErr := json.Marshal(err)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		readSpanCaptureSave26i(t, "inferred-narrowed-failure.json", append(failure, '\n'))
		t.Fatal(err)
	}
	readSpanCaptureSave26i(t, "inferred-narrowed-package.json", output.Bytes())
	var packet commonProgramPackage
	if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	compare(packet.Payload)
}
