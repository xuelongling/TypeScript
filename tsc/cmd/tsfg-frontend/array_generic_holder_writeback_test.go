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

// Separate compiler objects establish original source facts. The public facts
// converter and downstream lifetime decisions are not used as an oracle.
func holderWritebackOriginal25ab(t *testing.T, source string) map[string]span {
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
	at := map[string]span{}
	record := func(key string, node *ast.Node) {
		t.Helper()
		if node == nil {
			t.Fatal("original node missing", key)
		}
		at[key] = span{file.GetPositionMap().UTF8ToUTF16(scanner.SkipTrivia(source, node.Pos())), file.GetPositionMap().UTF8ToUTF16(node.End())}
	}
	named := func(text string, kind ast.Kind) *ast.Node {
		t.Helper()
		node := nodes[text]
		if node == nil || node.Kind != kind {
			t.Fatal("original syntax missing", text)
		}
		return node
	}
	var h, o, set *ast.Node
	for _, node := range file.Statements.Nodes {
		if node.Name() == nil {
			continue
		}
		switch node.Name().Text() {
		case "H":
			h = node
		case "O":
			o = node
		case "set":
			set = node
		}
	}
	if h == nil || o == nil || set == nil || len(set.AsFunctionDeclaration().Parameters.Nodes) != 2 {
		t.Fatal("original classes and two-formal free setter required")
	}
	hCtor, oCtor := h.Members()[0], o.Members()[0]
	hField, oField := hCtor.AsConstructorDeclaration().Parameters.Nodes[0], oCtor.AsConstructorDeclaration().Parameters.Nodes[0]
	oFormal, bFormal := set.AsFunctionDeclaration().Parameters.Nodes[0], set.AsFunctionDeclaration().Parameters.Nodes[1]
	write := named("o.h.span = readSpan(b, 0, 1)", ast.KindBinaryExpression)
	left, rhs := write.AsBinaryExpression().Left, write.AsBinaryExpression().Right
	inner := left.AsPropertyAccessExpression().Expression
	call := named("set(o, b)", ast.KindCallExpression)
	newO := named("new O(new H(readSpan(a, 0, 1)))", ast.KindNewExpression)
	newH := newO.AsNewExpression().Arguments.Nodes[0]
	viewA := newH.AsNewExpression().Arguments.Nodes[0]
	resize := named("a.length = 0", ast.KindBinaryExpression)
	read := named("o.h.span[0]", ast.KindElementAccessExpression)
	finalSpan := read.AsElementAccessExpression().Expression
	finalInner := finalSpan.AsPropertyAccessExpression().Expression
	symbolIs := func(node, declaration *ast.Node) bool {
		symbol := check.GetSymbolAtLocation(node)
		return symbol != nil && len(symbol.Declarations) == 1 && symbol.Declarations[0] == declaration
	}
	actualO, actualB := call.AsCallExpression().Arguments.Nodes[0], call.AsCallExpression().Arguments.Nodes[1]
	actualA := viewA.AsCallExpression().Arguments.Nodes[0]
	if write.AsBinaryExpression().OperatorToken.Kind != ast.KindEqualsToken ||
		!symbolIs(left.Name(), hField) || !symbolIs(inner.Name(), oField) || !symbolIs(inner.AsPropertyAccessExpression().Expression, oFormal) ||
		!symbolIs(rhs.AsCallExpression().Arguments.Nodes[0], bFormal) || !symbolIs(finalSpan.Name(), hField) || !symbolIs(finalInner.Name(), oField) ||
		check.GetSymbolAtLocation(actualO) != check.GetSymbolAtLocation(finalInner.AsPropertyAccessExpression().Expression) ||
		check.GetSymbolAtLocation(actualO) == check.GetSymbolAtLocation(oFormal.Name()) ||
		check.GetSymbolAtLocation(actualB) == check.GetSymbolAtLocation(bFormal.Name()) ||
		check.GetSymbolAtLocation(actualA) == check.GetSymbolAtLocation(actualB) ||
		check.GetSymbolAtLocation(actualA) != check.GetSymbolAtLocation(resize.AsBinaryExpression().Left.AsPropertyAccessExpression().Expression) {
		t.Fatal("original nested fields, callee formals, caller bindings or distinct arrays changed")
	}
	if check.GetResolvedSignature(call).Declaration() != set || check.GetResolvedSignature(newO).Declaration() != oCtor ||
		check.GetResolvedSignature(newH).Declaration() != hCtor || !(call.End() < resize.Pos() && resize.End() < read.Pos()) {
		t.Fatal("original selected calls or caller call/resize/read order changed")
	}
	for key, node := range map[string]*ast.Node{
		"H.field": hField, "O.field": oField, "H.constructor": hCtor, "O.constructor": oCtor, "set": set, "formalO": oFormal, "formalB": bFormal,
		"write": write, "target": left, "targetH": inner, "targetO": inner.AsPropertyAccessExpression().Expression,
		"rhs": rhs, "rhsB": rhs.AsCallExpression().Arguments.Nodes[0], "rhsStart": rhs.AsCallExpression().Arguments.Nodes[1], "rhsCount": rhs.AsCallExpression().Arguments.Nodes[2],
		"call": call, "actualO": actualO, "actualB": actualB, "newO": newO, "newH": newH, "viewA": viewA, "actualA": actualA,
		"resize": resize, "read": read, "readSpan": finalSpan, "readH": finalInner, "readO": finalInner.AsPropertyAccessExpression().Expression,
	} {
		record(key, node)
	}
	t.Log("independent official Program: syntax/semantic/program/global=0; original nested setter, distinct scoped bindings/arrays, selected calls, caller order and UTF16 verified")
	return at
}

func holderWritebackContract25ab(p *arrayGenericPayload, source string, at map[string]span) error {
	fail := func(what string) error { return fmt.Errorf("original holder writeback facts differ: %s", what) }
	if len(p.Diagnostics) != 0 || p.PositionUnit != "utf16" || len(p.Functions) != 2 || len(p.Files) != 2 || len(p.Exports) != 1 {
		return fail("complete source closure")
	}
	sourceFound := false
	for _, file := range p.Files {
		if file.Path == "/src/main.ts" {
			sourceFound = file.Content == source && file.SHA256 == arrayFactsSHA([]byte(source)) && file.ByteLength == len(source) && file.UTF16Length == len(source)
		}
	}
	if !sourceFound {
		return fail("complete original ASCII source bytes and UTF16 lengths")
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
			return fail("original nominal constructor closure")
		}
		field := (*typ.Fields)[0]
		if field.Origin.Source != "/src/main.ts" || field.Origin.Start != at[typ.Name+".field"].Start ||
			field.Origin.End != at[typ.Name+".field"].End || field.ParameterBinding != typ.Constructor.Parameters[0].ID {
			return fail("original parameter-property field identity")
		}
	}
	hField, oField := (*h.Fields)[0], (*o.Fields)[0]
	if h.ID == o.ID || hField.ID == oField.ID || hField.Name != "span" || hField.Type != "readSpan<i32>" ||
		oField.Name != "h" || oField.Type != h.ID || hField.Readonly {
		return fail("two original member/type identities")
	}
	set, run := nestedHolderFunction25z(p, "set"), nestedHolderFunction25z(p, "run")
	if set == nil || run == nil || set.OwnerType != "" || set.ReturnType != "void" || len(set.Parameters) != 2 ||
		set.Parameters[0].DeclaredType != o.ID || set.Parameters[1].DeclaredType != "array<i32>" ||
		set.Parameters[0].DeclarationRange != at["formalO"] || set.Parameters[1].DeclarationRange != at["formalB"] || set.DeclarationRange != at["set"] ||
		len(set.Body) != 1 || len(run.Body) != 6 || len(run.Parameters) != 0 || run.ReturnType != "i32" ||
		p.Exports[0].Name != "run" || p.Exports[0].FunctionID != run.ID {
		return fail("free void setter and complete caller body")
	}
	write := set.Body[0].Value
	if write == nil || write.Kind != "fieldAssignment" || write.Range != at["write"] || write.Target == nil ||
		write.Target.Kind != "fieldTarget" || write.Target.Property != hField.ID || write.Target.Type != hField.Type || write.Target.Range != at["target"] ||
		write.Target.Receiver == nil || write.Target.Receiver.Kind != "fieldAccess" || write.Target.Receiver.Property != oField.ID ||
		write.Target.Receiver.Type != h.ID || write.Target.Receiver.Range != at["targetH"] || write.Target.Receiver.Receiver == nil ||
		write.Target.Receiver.Receiver.Binding != set.Parameters[0].ID || write.Target.Receiver.Receiver.Range != at["targetO"] {
		return fail("actual nested formal receiver and H.span write")
	}
	rhs := write.Value
	if rhs == nil || rhs.Kind != "call" || rhs.Callee != p.ReadSpanIntrinsic.ID || rhs.Type != hField.Type || rhs.Range != at["rhs"] ||
		rhs.Arguments == nil || len(*rhs.Arguments) != 3 || (*rhs.Arguments)[0].Binding != set.Parameters[1].ID || (*rhs.Arguments)[0].Range != at["rhsB"] ||
		(*rhs.Arguments)[1].Decimal != "0" || (*rhs.Arguments)[1].Range != at["rhsStart"] || (*rhs.Arguments)[2].Decimal != "1" || (*rhs.Arguments)[2].Range != at["rhsCount"] {
		return fail("RHS original readSpan source is setter formal b")
	}
	localA, localB, localO := run.Body[0], run.Body[1], run.Body[2]
	for i, local := range []arrayGenericStatement{localA, localB} {
		if local.Binding == "" || local.Initializer == nil || local.Initializer.Kind != "arrayLiteral" || local.Initializer.Type != "array<i32>" ||
			local.Initializer.Elements == nil || len(*local.Initializer.Elements) != 1 || (*local.Initializer.Elements)[0].Decimal != fmt.Sprint(i+1) {
			return fail("original distinct caller array allocations")
		}
	}
	if localA.Binding == localB.Binding || localO.Binding == "" || localO.Binding == set.Parameters[0].ID || localB.Binding == set.Parameters[1].ID {
		return fail("distinct original caller/callee bindings")
	}
	newO := localO.Initializer
	if newO == nil || newO.Kind != "new" || newO.Callee != o.ID || newO.Range != at["newO"] || newO.Signature == nil ||
		newO.Signature.ID != o.Construction.Signature.ID || newO.Arguments == nil || len(*newO.Arguments) != 1 {
		return fail("actual outer constructor")
	}
	newH := &(*newO.Arguments)[0]
	if newH.Kind != "new" || newH.Callee != h.ID || newH.Range != at["newH"] || newH.Signature == nil ||
		newH.Signature.ID != h.Construction.Signature.ID || newH.Arguments == nil || len(*newH.Arguments) != 1 {
		return fail("actual inner constructor")
	}
	viewA := &(*newH.Arguments)[0]
	if viewA.Kind != "call" || viewA.Callee != p.ReadSpanIntrinsic.ID || viewA.Range != at["viewA"] || viewA.Arguments == nil ||
		len(*viewA.Arguments) != 3 || (*viewA.Arguments)[0].Binding != localA.Binding || (*viewA.Arguments)[0].Range != at["actualA"] {
		return fail("constructor view original source is caller a")
	}
	call, resize, read := run.Body[3].Value, run.Body[4].Value, run.Body[5].Value
	if call == nil || call.Kind != "call" || call.Callee != set.ID || call.Type != "void" || call.Range != at["call"] ||
		call.Signature == nil || call.Signature.ReturnType != "void" || call.Signature.DeclarationRange != set.DeclarationRange ||
		len(call.Signature.Parameters) != 2 || call.Signature.Parameters[0] != o.ID || call.Signature.Parameters[1] != "array<i32>" ||
		call.Arguments == nil || len(*call.Arguments) != 2 || (*call.Arguments)[0].Binding != localO.Binding || (*call.Arguments)[0].Range != at["actualO"] ||
		(*call.Arguments)[1].Binding != localB.Binding || (*call.Arguments)[1].Range != at["actualB"] {
		return fail("actual selected void set call and ordered caller operands")
	}
	if resize == nil || resize.Kind != "lengthAssignment" || resize.Range != at["resize"] || resize.Target == nil || resize.Target.Property != p.ArrayLength.ID ||
		resize.Target.Receiver == nil || resize.Target.Receiver.Binding != localA.Binding || resize.Value == nil || resize.Value.Decimal != "0" ||
		read == nil || read.Kind != "checkedElementAccess" || read.Type != "i32" || read.Range != at["read"] ||
		read.Receiver == nil || read.Receiver.Property != hField.ID || read.Receiver.Range != at["readSpan"] ||
		read.Receiver.Receiver == nil || read.Receiver.Receiver.Property != oField.ID || read.Receiver.Receiver.Type != h.ID ||
		read.Receiver.Receiver.Range != at["readH"] || read.Receiver.Receiver.Receiver == nil ||
		read.Receiver.Receiver.Receiver.Binding != localO.Binding || read.Receiver.Receiver.Receiver.Range != at["readO"] ||
		!(call.Range.End < resize.Range.Start && resize.Range.End < read.Range.Start) {
		return fail("caller set, resize only a, then original nested read")
	}
	return nil
}

func TestArrayGenericV8HolderWriteback25ab(t *testing.T) {
	sourceBytes, err := os.ReadFile("testdata/array-facts-v8/holder-writeback-positive.ts")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	original := holderWritebackOriginal25ab(t, source)
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
		t.Fatal("original public v8 package identity changed")
	}
	if err := holderWritebackContract25ab(&packet.Payload, source, original); err != nil {
		t.Fatal(err)
	}
	mutations := []struct {
		name string
		edit func(*arrayGenericPayload)
	}{
		{"written-member", func(p *arrayGenericPayload) {
			s := nestedHolderFunction25z(p, "set")
			s.Body[0].Value.Target.Property = s.Body[0].Value.Target.Receiver.Property
		}},
		{"write-caller-binding", func(p *arrayGenericPayload) {
			nestedHolderFunction25z(p, "set").Body[0].Value.Target.Receiver.Receiver.Binding = nestedHolderFunction25z(p, "run").Body[2].Binding
		}},
		{"RHS-caller-b", func(p *arrayGenericPayload) {
			(*nestedHolderFunction25z(p, "set").Body[0].Value.Value.Arguments)[0].Binding = nestedHolderFunction25z(p, "run").Body[1].Binding
		}},
		{"RHS-other-formal", func(p *arrayGenericPayload) {
			s := nestedHolderFunction25z(p, "set")
			(*s.Body[0].Value.Value.Arguments)[0].Binding = s.Parameters[0].ID
		}},
		{"RHS-occurrence", func(p *arrayGenericPayload) {
			s := nestedHolderFunction25z(p, "set")
			args := *s.Body[0].Value.Value.Arguments
			args[1].Range = args[2].Range
		}},
		{"call-other-array", func(p *arrayGenericPayload) {
			r := nestedHolderFunction25z(p, "run")
			(*r.Body[3].Value.Arguments)[1].Binding = r.Body[0].Binding
		}},
		{"call-argument-order", func(p *arrayGenericPayload) {
			r := nestedHolderFunction25z(p, "run")
			args := *r.Body[3].Value.Arguments
			args[0], args[1] = args[1], args[0]
		}},
		{"call-return-type", func(p *arrayGenericPayload) {
			nestedHolderFunction25z(p, "run").Body[3].Value.Signature.ReturnType = "i32"
		}},
		{"resize-b", func(p *arrayGenericPayload) {
			r := nestedHolderFunction25z(p, "run")
			r.Body[4].Value.Target.Receiver.Binding = r.Body[1].Binding
		}},
		{"resize-before-call", func(p *arrayGenericPayload) {
			r := nestedHolderFunction25z(p, "run")
			r.Body[3], r.Body[4] = r.Body[4], r.Body[3]
		}},
		{"read-formal-o", func(p *arrayGenericPayload) {
			nestedHolderFunction25z(p, "run").Body[5].Value.Receiver.Receiver.Receiver.Binding = nestedHolderFunction25z(p, "set").Parameters[0].ID
		}},
		{"read-other-member", func(p *arrayGenericPayload) {
			r := nestedHolderFunction25z(p, "run")
			r.Body[5].Value.Receiver.Property = r.Body[5].Value.Receiver.Receiver.Property
		}},
		{"constructor-Array-b", func(p *arrayGenericPayload) {
			r := nestedHolderFunction25z(p, "run")
			inner := &(*r.Body[2].Initializer.Arguments)[0]
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
			if holderWritebackContract25ab(&copied.Payload, source, original) == nil {
				t.Fatal("copied DTO corruption accepted")
			}
		})
	}
}
