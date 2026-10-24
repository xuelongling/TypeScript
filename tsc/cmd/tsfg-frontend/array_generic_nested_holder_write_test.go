// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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

// The new write case has its own original-AST oracle; it does not broaden the
// already checked two-case oracle or use consumer lifetime outcomes as facts.
func nestedWriteOriginal25z(t *testing.T, source string) map[string]span {
	t.Helper()
	files := map[string]string{"/src/main.ts": source, "/src/minimal-arrays-v8.d.ts": string(arrayGenericDeclarations)}
	program := compiler.NewProgram(compiler.ProgramOptions{Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
		FileNames:       []string{"/src/main.ts", "/src/minimal-arrays-v8.d.ts"},
		CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext},
	}}, Host: compiler.NewCompilerHost("/src", vfstest.FromMap(files, true), "/src", nil, nil), SingleThreaded: core.TSTrue})
	for _, diagnostics := range [][]*ast.Diagnostic{program.GetSyntacticDiagnostics(t.Context(), nil),
		program.GetSemanticDiagnostics(t.Context(), nil), program.GetProgramDiagnostics(), program.GetGlobalDiagnostics(t.Context())} {
		if len(diagnostics) != 0 {
			t.Fatalf("independent original diagnostics: %+v", diagnostics)
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
	original := map[string]span{}
	record := func(key string, node *ast.Node) {
		t.Helper()
		if node == nil {
			t.Fatal("original node missing", key)
		}
		original[key] = span{file.GetPositionMap().UTF8ToUTF16(scanner.SkipTrivia(source, node.Pos())), file.GetPositionMap().UTF8ToUTF16(node.End())}
	}
	named := func(text string, kind ast.Kind) *ast.Node {
		t.Helper()
		node := nodes[text]
		if node == nil || node.Kind != kind {
			t.Fatal("original syntax missing", text)
		}
		return node
	}
	var h, o, first *ast.Node
	for _, node := range file.Statements.Nodes {
		if node.Name() == nil {
			continue
		}
		switch node.Name().Text() {
		case "H":
			h = node
		case "O":
			o = node
		case "first":
			first = node
		}
	}
	if h == nil || o == nil || first == nil || len(first.AsFunctionDeclaration().Parameters.Nodes) != 2 {
		t.Fatal("original class and two-formal free function closure required")
	}
	hCtor, oCtor := h.Members()[0], o.Members()[0]
	hField, oField := hCtor.AsConstructorDeclaration().Parameters.Nodes[0], oCtor.AsConstructorDeclaration().Parameters.Nodes[0]
	oParam, bParam := first.AsFunctionDeclaration().Parameters.Nodes[0], first.AsFunctionDeclaration().Parameters.Nodes[1]
	write := named("o.h.span = readSpan(b, 0, 1)", ast.KindBinaryExpression)
	left, rhs := write.AsBinaryExpression().Left, write.AsBinaryExpression().Right
	inner := left.AsPropertyAccessExpression().Expression
	resize := named("b.length = 0", ast.KindBinaryExpression)
	read := named("o.h.span[0]", ast.KindElementAccessExpression)
	finalSpan := read.AsElementAccessExpression().Expression
	finalInner := finalSpan.AsPropertyAccessExpression().Expression
	bindingIs := func(node, declaration *ast.Node) bool {
		symbol := check.GetSymbolAtLocation(node)
		return symbol != nil && len(symbol.Declarations) == 1 && symbol.Declarations[0] == declaration
	}
	if write.AsBinaryExpression().OperatorToken.Kind != ast.KindEqualsToken ||
		!bindingIs(left.Name(), hField) || !bindingIs(inner.Name(), oField) || !bindingIs(inner.AsPropertyAccessExpression().Expression, oParam) ||
		!bindingIs(rhs.AsCallExpression().Arguments.Nodes[0], bParam) ||
		!bindingIs(resize.AsBinaryExpression().Left.AsPropertyAccessExpression().Expression, bParam) ||
		!bindingIs(finalSpan.Name(), hField) || !bindingIs(finalInner.Name(), oField) ||
		!bindingIs(finalInner.AsPropertyAccessExpression().Expression, oParam) ||
		!(write.End() < resize.Pos() && resize.End() < read.Pos()) {
		t.Fatal("actual original nested write/RHS parameter/resize/read symbols or source order changed")
	}
	call := named("first(new O(new H(readSpan(a, 0, 1))), b)", ast.KindCallExpression)
	newO := call.AsCallExpression().Arguments.Nodes[0]
	newH := newO.AsNewExpression().Arguments.Nodes[0]
	viewA := newH.AsNewExpression().Arguments.Nodes[0]
	actualA, actualB := viewA.AsCallExpression().Arguments.Nodes[0], call.AsCallExpression().Arguments.Nodes[1]
	if check.GetResolvedSignature(call).Declaration() != first || check.GetResolvedSignature(newO).Declaration() != oCtor ||
		check.GetResolvedSignature(newH).Declaration() != hCtor || check.GetSymbolAtLocation(actualA) == check.GetSymbolAtLocation(actualB) ||
		check.GetTypeAtLocation(inner) != check.GetDeclaredTypeOfSymbol(check.GetSymbolAtLocation(h.Name())) {
		t.Fatal("original selected calls, distinct caller arrays or nested H receiver type changed")
	}
	for key, node := range map[string]*ast.Node{"H.field": hField, "O.field": oField, "first": first, "oParam": oParam, "bParam": bParam,
		"write": write, "writeTarget": left, "writeInner": inner, "writeRoot": inner.AsPropertyAccessExpression().Expression,
		"rhs": rhs, "rhsArray": rhs.AsCallExpression().Arguments.Nodes[0], "rhsStart": rhs.AsCallExpression().Arguments.Nodes[1],
		"rhsCount": rhs.AsCallExpression().Arguments.Nodes[2], "resize": resize, "read": read, "readSpan": finalSpan, "readInner": finalInner,
		"readRoot": finalInner.AsPropertyAccessExpression().Expression, "call": call, "newO": newO, "newH": newH, "viewA": viewA,
		"actualA": actualA, "actualB": actualB, "H.constructor": hCtor, "O.constructor": oCtor} {
		record(key, node)
	}
	t.Log("independent official Program: syntax/semantic/program/global=0; original nested write/read symbols, formal b, distinct caller arrays, selected calls and UTF16 verified")
	return original
}

func nestedWriteContract25z(p *arrayGenericPayload, source string, at map[string]span) error {
	fail := func(what string) error { return fmt.Errorf("original nested write facts differ: %s", what) }
	if len(p.Diagnostics) != 0 || p.PositionUnit != "utf16" || len(p.Functions) != 2 || len(p.Files) != 2 || len(p.Exports) != 1 {
		return fail("complete original source closure")
	}
	sourceFound := false
	for _, file := range p.Files {
		if file.Path == "/src/main.ts" {
			sourceFound = file.Content == source && file.SHA256 == arrayFactsSHA([]byte(source)) && file.ByteLength == len(source) && file.UTF16Length == len(source)
		}
	}
	if !sourceFound {
		return fail("source bytes and original ASCII/UTF16 lengths")
	}
	types := map[string]arrayGenericType{}
	for _, typ := range p.Types {
		if typ.Name == "H" || typ.Name == "O" {
			types[typ.Name] = typ
		}
	}
	h, o := types["H"], types["O"]
	for _, typ := range []arrayGenericType{h, o} {
		if typ.ID == "" || typ.Kind != "reference" || typ.Fields == nil || len(*typ.Fields) != 1 || typ.Constructor == nil ||
			len(typ.Constructor.Parameters) != 1 || typ.Constructor.DeclarationRange != at[typ.Name+".constructor"] || typ.Construction == nil ||
			typ.Construction.Kind != "explicit" || typ.Construction.InitializationPhase != "before-body" {
			return fail("two original classes and constructors")
		}
		field := (*typ.Fields)[0]
		if field.Origin.Source != "/src/main.ts" || field.Origin.Start != at[typ.Name+".field"].Start ||
			field.Origin.End != at[typ.Name+".field"].End || field.ParameterBinding != typ.Constructor.Parameters[0].ID {
			return fail("original parameter-property field declaration")
		}
	}
	hField, oField := (*h.Fields)[0], (*o.Fields)[0]
	if h.ID == o.ID || hField.ID == oField.ID || hField.Name != "span" || hField.Type != "readSpan<i32>" ||
		oField.Name != "h" || oField.Type != h.ID || hField.Readonly {
		return fail("actual writable H.span and O.h member identities")
	}
	first, run := nestedHolderFunction25z(p, "first"), nestedHolderFunction25z(p, "run")
	if first == nil || run == nil || first.OwnerType != "" || len(first.Parameters) != 2 || len(first.Body) != 3 || len(run.Body) != 3 ||
		first.Parameters[0].DeclaredType != o.ID || first.Parameters[1].DeclaredType != "array<i32>" ||
		first.Parameters[0].DeclarationRange != at["oParam"] || first.Parameters[1].DeclarationRange != at["bParam"] ||
		first.DeclarationRange != at["first"] || first.ReturnType != "i32" || p.Exports[0].FunctionID != run.ID {
		return fail("original free function formals and ordered bodies")
	}
	write, resize, read := first.Body[0].Value, first.Body[1].Value, first.Body[2].Value
	if write == nil || write.Kind != "fieldAssignment" || write.Range != at["write"] || write.Target == nil ||
		write.Target.Kind != "fieldTarget" || write.Target.Property != hField.ID || write.Target.Type != hField.Type || write.Target.Range != at["writeTarget"] ||
		write.Target.Receiver == nil || write.Target.Receiver.Kind != "fieldAccess" || write.Target.Receiver.Property != oField.ID ||
		write.Target.Receiver.Type != h.ID || write.Target.Receiver.Range != at["writeInner"] || write.Target.Receiver.Receiver == nil ||
		write.Target.Receiver.Receiver.Binding != first.Parameters[0].ID || write.Target.Receiver.Receiver.Range != at["writeRoot"] {
		return fail("actual H.span write through nested formal receiver")
	}
	rhs := write.Value
	if rhs == nil || rhs.Kind != "call" || rhs.Callee != p.ReadSpanIntrinsic.ID || rhs.Type != "readSpan<i32>" || rhs.Range != at["rhs"] ||
		rhs.Arguments == nil || len(*rhs.Arguments) != 3 || (*rhs.Arguments)[0].Binding != first.Parameters[1].ID || (*rhs.Arguments)[0].Range != at["rhsArray"] ||
		(*rhs.Arguments)[1].Decimal != "0" || (*rhs.Arguments)[1].Range != at["rhsStart"] || (*rhs.Arguments)[2].Decimal != "1" || (*rhs.Arguments)[2].Range != at["rhsCount"] {
		return fail("RHS view from exactly formal b and original operands")
	}
	if resize == nil || resize.Kind != "lengthAssignment" || resize.Range != at["resize"] || resize.Target == nil ||
		resize.Target.Property != p.ArrayLength.ID || resize.Target.Receiver == nil || resize.Target.Receiver.Binding != first.Parameters[1].ID ||
		resize.Value == nil || resize.Value.Decimal != "0" || read == nil || read.Kind != "checkedElementAccess" ||
		read.Range != at["read"] || read.Type != "i32" || read.Receiver == nil || read.Receiver.Property != hField.ID || read.Receiver.Range != at["readSpan"] ||
		read.Receiver.Receiver == nil || read.Receiver.Receiver.Property != oField.ID || read.Receiver.Receiver.Type != h.ID ||
		read.Receiver.Receiver.Range != at["readInner"] || read.Receiver.Receiver.Receiver == nil ||
		read.Receiver.Receiver.Receiver.Binding != first.Parameters[0].ID || read.Receiver.Receiver.Receiver.Range != at["readRoot"] ||
		!(write.Range.End < resize.Range.Start && resize.Range.End < read.Range.Start) {
		return fail("same b resize before final original nested checked read")
	}
	localA, localB, call := run.Body[0], run.Body[1], run.Body[2].Value
	for i, local := range []arrayGenericStatement{localA, localB} {
		if local.Binding == "" || local.Initializer == nil || local.Initializer.Kind != "arrayLiteral" || local.Initializer.Type != "array<i32>" ||
			local.Initializer.Elements == nil || len(*local.Initializer.Elements) != 1 || (*local.Initializer.Elements)[0].Decimal != fmt.Sprint(i+1) {
			return fail("two actual distinct initialized Array locals")
		}
	}
	if localA.Binding == localB.Binding || call == nil || call.Kind != "call" || call.Callee != first.ID || call.Range != at["call"] ||
		call.Signature == nil || call.Signature.DeclarationRange != first.DeclarationRange || call.Arguments == nil || len(*call.Arguments) != 2 ||
		(*call.Arguments)[1].Binding != localB.Binding || (*call.Arguments)[1].Range != at["actualB"] {
		return fail("selected call and ordered actual b")
	}
	newO := &(*call.Arguments)[0]
	if newO.Kind != "new" || newO.Callee != o.ID || newO.Range != at["newO"] || newO.Signature == nil ||
		newO.Signature.ID != o.Construction.Signature.ID || newO.Arguments == nil || len(*newO.Arguments) != 1 {
		return fail("actual outer constructor")
	}
	newH := &(*newO.Arguments)[0]
	if newH.Kind != "new" || newH.Callee != h.ID || newH.Range != at["newH"] || newH.Signature == nil ||
		newH.Signature.ID != h.Construction.Signature.ID || newH.Arguments == nil || len(*newH.Arguments) != 1 {
		return fail("actual inner constructor")
	}
	viewA := &(*newH.Arguments)[0]
	if viewA.Callee != p.ReadSpanIntrinsic.ID || viewA.Range != at["viewA"] || viewA.Arguments == nil || len(*viewA.Arguments) != 3 ||
		(*viewA.Arguments)[0].Binding != localA.Binding || (*viewA.Arguments)[0].Range != at["actualA"] {
		return fail("original constructor view source is distinct local a")
	}
	return nil
}

func TestArrayGenericV8NestedHolderWrite25z(t *testing.T) {
	sourceBytes, err := os.ReadFile("testdata/array-facts-v8/nested-holder-write-conflict.ts")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	original := nestedWriteOriginal25z(t, source)
	input := genericArrayRequest25f(t, source)
	capture := func(name string, data []byte) {
		t.Helper()
		if root := os.Getenv("TSFG_ARRAY_V8_CAPTURE_ROOT"); root != "" {
			if err := os.MkdirAll(root, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, arrayFactsSHA(sourceBytes)+"-"+name), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	capture("source.ts", sourceBytes)
	capture("request.json", input)
	var out bytes.Buffer
	if err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(input), &out); err != nil {
		capture("error.txt", []byte(err.Error()))
		capture("failed-output.bin", out.Bytes())
		t.Fatal(err)
	}
	capture("package.json", out.Bytes())
	var packet arrayGenericPackage
	if err := json.Unmarshal(out.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	if packet.Family != "tsfg.array-facts-package-candidate" || packet.Version != 8 {
		t.Fatal("original public v8 packet identity changed")
	}
	if err := nestedWriteContract25z(&packet.Payload, source, original); err != nil {
		t.Fatal(err)
	}
	mutations := []struct {
		name string
		edit func(*arrayGenericPayload)
	}{
		{"wrong-written-member", func(p *arrayGenericPayload) {
			f := nestedHolderFunction25z(p, "first")
			f.Body[0].Value.Target.Property = f.Body[0].Value.Target.Receiver.Property
		}},
		{"wrong-write-receiver", func(p *arrayGenericPayload) {
			f := nestedHolderFunction25z(p, "first")
			f.Body[0].Value.Target.Receiver.Receiver.Binding = f.Parameters[1].ID
		}},
		{"RHS-other-formal", func(p *arrayGenericPayload) {
			f := nestedHolderFunction25z(p, "first")
			(*f.Body[0].Value.Value.Arguments)[0].Binding = f.Parameters[0].ID
		}},
		{"RHS-caller-binding", func(p *arrayGenericPayload) {
			f := nestedHolderFunction25z(p, "first")
			(*f.Body[0].Value.Value.Arguments)[0].Binding = nestedHolderFunction25z(p, "run").Body[1].Binding
		}},
		{"RHS-selected-call", func(p *arrayGenericPayload) {
			f := nestedHolderFunction25z(p, "first")
			f.Body[0].Value.Value.Callee = f.ID
		}},
		{"RHS-occurrence", func(p *arrayGenericPayload) {
			f := nestedHolderFunction25z(p, "first")
			(*f.Body[0].Value.Value.Arguments)[1].Range = (*f.Body[0].Value.Value.Arguments)[2].Range
		}},
		{"resize-other-binding", func(p *arrayGenericPayload) {
			f := nestedHolderFunction25z(p, "first")
			f.Body[1].Value.Target.Receiver.Binding = f.Parameters[0].ID
		}},
		{"resize-after-read", func(p *arrayGenericPayload) {
			f := nestedHolderFunction25z(p, "first")
			f.Body[1], f.Body[2] = f.Body[2], f.Body[1]
		}},
		{"final-read-member", func(p *arrayGenericPayload) {
			f := nestedHolderFunction25z(p, "first")
			f.Body[2].Value.Receiver.Property = f.Body[2].Value.Receiver.Receiver.Property
		}},
		{"call-second-array", func(p *arrayGenericPayload) {
			r := nestedHolderFunction25z(p, "run")
			(*r.Body[2].Value.Arguments)[1].Binding = r.Body[0].Binding
		}},
		{"call-argument-order", func(p *arrayGenericPayload) {
			r := nestedHolderFunction25z(p, "run")
			args := *r.Body[2].Value.Arguments
			args[0], args[1] = args[1], args[0]
		}},
		{"constructor-Array-b", func(p *arrayGenericPayload) {
			r := nestedHolderFunction25z(p, "run")
			outer := &(*r.Body[2].Value.Arguments)[0]
			inner := &(*outer.Arguments)[0]
			(*(*inner.Arguments)[0].Arguments)[0].Binding = r.Body[1].Binding
		}},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			var copied arrayGenericPackage
			if err := json.Unmarshal(out.Bytes(), &copied); err != nil {
				t.Fatal(err)
			}
			mutation.edit(&copied.Payload)
			if nestedWriteContract25z(&copied.Payload, source, original) == nil {
				t.Fatal("copied DTO corruption accepted")
			}
		})
	}
}
