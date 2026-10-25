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

// This case needs different original AST relations from the earlier one-write
// or caller-writeback cases. Their fixture-specific oracles remain unchanged.
func holderAliasOriginal25ac(t *testing.T, source string) map[string]span {
	t.Helper()
	program := compiler.NewProgram(compiler.ProgramOptions{Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
		FileNames:       []string{"/src/main.ts", "/src/minimal-arrays-v8.d.ts"},
		CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext},
	}}, Host: compiler.NewCompilerHost("/src", vfstest.FromMap(map[string]string{"/src/main.ts": source,
		"/src/minimal-arrays-v8.d.ts": string(arrayGenericDeclarations)}, true), "/src", nil, nil), SingleThreaded: core.TSTrue})
	for _, diagnostics := range [][]*ast.Diagnostic{program.GetSyntacticDiagnostics(t.Context(), nil),
		program.GetSemanticDiagnostics(t.Context(), nil), program.GetProgramDiagnostics(), program.GetGlobalDiagnostics(t.Context())} {
		if len(diagnostics) != 0 {
			t.Fatalf("independent official diagnostics: %+v", diagnostics)
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
			t.Fatal("missing original node", key)
		}
		at[key] = span{file.GetPositionMap().UTF8ToUTF16(scanner.SkipTrivia(source, node.Pos())), file.GetPositionMap().UTF8ToUTF16(node.End())}
	}
	named := func(text string, kind ast.Kind) *ast.Node {
		t.Helper()
		node := nodes[text]
		if node == nil || node.Kind != kind {
			t.Fatal("missing original syntax", text)
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
	if h == nil || o == nil || first == nil || len(first.AsFunctionDeclaration().Parameters.Nodes) != 4 {
		t.Fatal("original classes and four formals required")
	}
	hCtor, oCtor := h.Members()[0], o.Members()[0]
	hField, oField := hCtor.AsConstructorDeclaration().Parameters.Nodes[0], oCtor.AsConstructorDeclaration().Parameters.Nodes[0]
	params := first.AsFunctionDeclaration().Parameters.Nodes
	x := named("x = o.h", ast.KindVariableDeclaration)
	xInit := x.AsVariableDeclaration().Initializer
	w1 := named("x.span = readSpan(b, 0, 1)", ast.KindBinaryExpression)
	w2 := named("h.span = readSpan(c, 0, 1)", ast.KindBinaryExpression)
	resize := named("c.length = 0", ast.KindBinaryExpression)
	read := named("x.span[0]", ast.KindElementAccessExpression)
	symbolIs := func(node, decl *ast.Node) bool {
		s := check.GetSymbolAtLocation(node)
		return s != nil && len(s.Declarations) == 1 && s.Declarations[0] == decl
	}
	if !symbolIs(xInit.Name(), oField) || !symbolIs(xInit.AsPropertyAccessExpression().Expression, params[0]) ||
		!symbolIs(w1.AsBinaryExpression().Left.Name(), hField) || !symbolIs(w2.AsBinaryExpression().Left.Name(), hField) ||
		!symbolIs(w1.AsBinaryExpression().Left.AsPropertyAccessExpression().Expression, x) ||
		!symbolIs(w2.AsBinaryExpression().Left.AsPropertyAccessExpression().Expression, params[1]) ||
		!symbolIs(w1.AsBinaryExpression().Right.AsCallExpression().Arguments.Nodes[0], params[2]) ||
		!symbolIs(w2.AsBinaryExpression().Right.AsCallExpression().Arguments.Nodes[0], params[3]) ||
		!symbolIs(resize.AsBinaryExpression().Left.AsPropertyAccessExpression().Expression, params[3]) ||
		!symbolIs(read.AsElementAccessExpression().Expression.Name(), hField) ||
		!symbolIs(read.AsElementAccessExpression().Expression.AsPropertyAccessExpression().Expression, x) ||
		!(x.End() < w1.Pos() && w1.End() < w2.Pos() && w2.End() < resize.Pos() && resize.End() < read.Pos()) {
		t.Fatal("original local/formal bindings, same H.span or ordered writes/resize/read changed")
	}
	call := named("first(new O(h), h, b, c)", ast.KindCallExpression)
	newO := call.AsCallExpression().Arguments.Nodes[0]
	newH := named("new H(readSpan(a, 0, 1))", ast.KindNewExpression)
	if check.GetResolvedSignature(call).Declaration() != first || check.GetResolvedSignature(newO).Declaration() != oCtor ||
		check.GetResolvedSignature(newH).Declaration() != hCtor ||
		check.GetSymbolAtLocation(newO.AsNewExpression().Arguments.Nodes[0]) != check.GetSymbolAtLocation(call.AsCallExpression().Arguments.Nodes[1]) {
		t.Fatal("original selected call/constructors or same actual h binding changed")
	}
	for key, node := range map[string]*ast.Node{"H.field": hField, "O.field": oField, "H.ctor": hCtor, "O.ctor": oCtor,
		"first": first, "xInit": xInit, "call": call, "newO": newO, "newH": newH,
		"resize": resize, "read": read, "readReceiver": read.AsElementAccessExpression().Expression} {
		record(key, node)
	}
	for i, param := range params {
		record(fmt.Sprint("param", i), param)
		record(fmt.Sprint("actual", i), call.AsCallExpression().Arguments.Nodes[i])
	}
	for i, write := range []*ast.Node{w1, w2} {
		record(fmt.Sprint("write", i), write)
		record(fmt.Sprint("target", i), write.AsBinaryExpression().Left)
		record(fmt.Sprint("rhs", i), write.AsBinaryExpression().Right)
		for j, arg := range write.AsBinaryExpression().Right.AsCallExpression().Arguments.Nodes {
			record(fmt.Sprintf("rhs%d.arg%d", i, j), arg)
		}
	}
	t.Log("independent official Program: syntax/semantic/program/global=0; four formals, x alias, same H.span, two ordered writes, C resize/read, same caller h constructor/argument and UTF16 verified")
	return at
}

func holderAliasContract25ac(p *arrayGenericPayload, source string, at map[string]span) error {
	fail := func(s string) error { return fmt.Errorf("original holder alias facts differ: %s", s) }
	if p.PositionUnit != "utf16" || len(p.Diagnostics) != 0 || len(p.Files) != 2 || len(p.Functions) != 2 || len(p.Exports) != 1 {
		return fail("complete source closure")
	}
	found := false
	for _, file := range p.Files {
		if file.Path == "/src/main.ts" {
			found = file.Content == source && file.SHA256 == arrayFactsSHA([]byte(source)) && file.ByteLength == len(source) && file.UTF16Length == len(source)
		}
	}
	if !found {
		return fail("original ASCII source bytes and UTF16 length")
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
			typ.Constructor.DeclarationRange != at[typ.Name+".ctor"] || len(typ.Constructor.Parameters) != 1 || typ.Construction == nil {
			return fail("original classes/constructors")
		}
		field := (*typ.Fields)[0]
		if field.Origin.Source != "/src/main.ts" || field.Origin.Start != at[typ.Name+".field"].Start ||
			field.Origin.End != at[typ.Name+".field"].End || field.ParameterBinding != typ.Constructor.Parameters[0].ID {
			return fail("original M12 parameter property")
		}
	}
	hf, of := (*h.Fields)[0], (*o.Fields)[0]
	if hf.Name != "span" || hf.Type != "readSpan<i32>" || hf.Readonly || of.Name != "h" || of.Type != h.ID || hf.ID == of.ID {
		return fail("same original writable H.span and O.h identities")
	}
	f, run := nestedHolderFunction25z(p, "first"), nestedHolderFunction25z(p, "run")
	if f == nil || run == nil || len(f.Parameters) != 4 || f.OwnerType != "" || f.ReturnType != "i32" || f.DeclarationRange != at["first"] ||
		len(f.Body) != 5 || len(run.Body) != 5 || len(run.Parameters) != 0 || p.Exports[0].FunctionID != run.ID {
		return fail("four-formal free function and complete caller")
	}
	for i, typ := range []string{o.ID, h.ID, "array<i32>", "array<i32>"} {
		if f.Parameters[i].DeclaredType != typ || f.Parameters[i].Kind != "parameter" || f.Parameters[i].DeclarationRange != at[fmt.Sprint("param", i)] {
			return fail("ordered original formal declarations")
		}
	}
	x := f.Body[0]
	if x.Binding == "" || x.Initializer == nil || x.Initializer.Kind != "fieldAccess" || x.Initializer.Property != of.ID ||
		x.Initializer.Type != h.ID || x.Initializer.Range != at["xInit"] || x.Initializer.Receiver == nil || x.Initializer.Receiver.Binding != f.Parameters[0].ID {
		return fail("original local x initialized from formal o.h")
	}
	for i, receiver := range []string{x.Binding, f.Parameters[1].ID} {
		w := f.Body[i+1].Value
		if w == nil || w.Kind != "fieldAssignment" || w.Range != at[fmt.Sprint("write", i)] || w.Target == nil ||
			w.Target.Kind != "fieldTarget" || w.Target.Property != hf.ID || w.Target.Type != hf.Type || w.Target.Range != at[fmt.Sprint("target", i)] ||
			w.Target.Receiver == nil || w.Target.Receiver.Kind != "bindingReference" || w.Target.Receiver.Binding != receiver || w.Target.Receiver.Type != h.ID {
			return fail("ordered writes through original distinct receiver bindings")
		}
		rhs := w.Value
		if rhs == nil || rhs.Kind != "call" || rhs.Callee != p.ReadSpanIntrinsic.ID || rhs.Type != hf.Type || rhs.Range != at[fmt.Sprint("rhs", i)] ||
			rhs.Arguments == nil || len(*rhs.Arguments) != 3 || (*rhs.Arguments)[0].Binding != f.Parameters[i+2].ID ||
			(*rhs.Arguments)[1].Decimal != "0" || (*rhs.Arguments)[2].Decimal != "1" {
			return fail("RHS original formal b then c")
		}
		for j, arg := range *rhs.Arguments {
			if arg.Range != at[fmt.Sprintf("rhs%d.arg%d", i, j)] {
				return fail("RHS original operand UTF16 occurrence")
			}
		}
	}
	resize, read := f.Body[3].Value, f.Body[4].Value
	if resize == nil || resize.Kind != "lengthAssignment" || resize.Range != at["resize"] || resize.Target == nil ||
		resize.Target.Property != p.ArrayLength.ID || resize.Target.Receiver == nil || resize.Target.Receiver.Binding != f.Parameters[3].ID ||
		resize.Value == nil || resize.Value.Decimal != "0" || read == nil || read.Kind != "checkedElementAccess" || read.Type != "i32" ||
		read.Range != at["read"] || read.Receiver == nil || read.Receiver.Kind != "fieldAccess" || read.Receiver.Property != hf.ID ||
		read.Receiver.Range != at["readReceiver"] || read.Receiver.Receiver == nil || read.Receiver.Receiver.Binding != x.Binding ||
		!(f.Body[1].Value.Range.End < f.Body[2].Value.Range.Start && f.Body[2].Value.Range.End < resize.Range.Start && resize.Range.End < read.Range.Start) {
		return fail("second write, original formal C resize, final x.span read")
	}
	for i := 0; i < 3; i++ {
		local := run.Body[i]
		if local.Binding == "" || local.Initializer == nil || local.Initializer.Kind != "arrayLiteral" || local.Initializer.Type != "array<i32>" ||
			local.Initializer.Elements == nil || len(*local.Initializer.Elements) != 1 || (*local.Initializer.Elements)[0].Decimal != fmt.Sprint(i+1) {
			return fail("three complete separately declared Array locals")
		}
	}
	localH, call := run.Body[3], run.Body[4].Value
	newH := localH.Initializer
	if localH.Binding == "" || newH == nil || newH.Kind != "new" || newH.Callee != h.ID || newH.Range != at["newH"] ||
		newH.Signature == nil || newH.Signature.ID != h.Construction.Signature.ID || newH.Arguments == nil || len(*newH.Arguments) != 1 {
		return fail("actual caller H constructor")
	}
	view := &(*newH.Arguments)[0]
	if view.Callee != p.ReadSpanIntrinsic.ID || view.Arguments == nil || len(*view.Arguments) != 3 || (*view.Arguments)[0].Binding != run.Body[0].Binding ||
		call == nil || call.Kind != "call" || call.Callee != f.ID || call.Range != at["call"] || call.Signature == nil ||
		call.Signature.DeclarationRange != f.DeclarationRange || len(call.Signature.Parameters) != 4 || call.Arguments == nil || len(*call.Arguments) != 4 {
		return fail("original A view and selected four-operand call")
	}
	args := *call.Arguments
	newO := args[0]
	if newO.Kind != "new" || newO.Callee != o.ID || newO.Range != at["newO"] || newO.Signature == nil ||
		newO.Signature.ID != o.Construction.Signature.ID || newO.Arguments == nil || len(*newO.Arguments) != 1 ||
		(*newO.Arguments)[0].Binding != localH.Binding || args[1].Binding != localH.Binding ||
		args[2].Binding != run.Body[1].Binding || args[3].Binding != run.Body[2].Binding {
		return fail("same caller h feeds O.h construction and independent formal h; ordered b/c")
	}
	for i, arg := range args {
		if arg.Range != at[fmt.Sprint("actual", i)] || call.Signature.Parameters[i] != f.Parameters[i].DeclaredType {
			return fail("original ordered selected signature and operand UTF16")
		}
	}
	return nil
}

func TestArrayGenericV8HolderAliasWriteback25ac(t *testing.T) {
	sourceBytes, err := os.ReadFile("testdata/array-facts-v8/holder-alias-writeback-conflict.ts")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	original := holderAliasOriginal25ac(t, source)
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
	if packet.Version != 8 || packet.Family != "tsfg.array-facts-package-candidate" {
		t.Fatal("original public packet identity changed")
	}
	if err := holderAliasContract25ac(&packet.Payload, source, original); err != nil {
		t.Fatal(err)
	}
	mutations := []struct {
		name string
		edit func(*arrayGenericPayload)
	}{
		{"local-origin", func(p *arrayGenericPayload) {
			f := nestedHolderFunction25z(p, "first")
			f.Body[0].Initializer.Receiver.Binding = f.Parameters[1].ID
		}},
		{"second-write-stale-x", func(p *arrayGenericPayload) {
			f := nestedHolderFunction25z(p, "first")
			f.Body[2].Value.Target.Receiver.Binding = f.Body[0].Binding
		}},
		{"second-written-member", func(p *arrayGenericPayload) {
			f := nestedHolderFunction25z(p, "first")
			f.Body[2].Value.Target.Property = f.Body[0].Initializer.Property
		}},
		{"second-RHS-B", func(p *arrayGenericPayload) {
			f := nestedHolderFunction25z(p, "first")
			(*f.Body[2].Value.Value.Arguments)[0].Binding = f.Parameters[2].ID
		}},
		{"write-order", func(p *arrayGenericPayload) {
			f := nestedHolderFunction25z(p, "first")
			f.Body[1], f.Body[2] = f.Body[2], f.Body[1]
		}},
		{"resize-B", func(p *arrayGenericPayload) {
			f := nestedHolderFunction25z(p, "first")
			f.Body[3].Value.Target.Receiver.Binding = f.Parameters[2].ID
		}},
		{"read-formal-h", func(p *arrayGenericPayload) {
			f := nestedHolderFunction25z(p, "first")
			f.Body[4].Value.Receiver.Receiver.Binding = f.Parameters[1].ID
		}},
		{"read-range", func(p *arrayGenericPayload) {
			f := nestedHolderFunction25z(p, "first")
			f.Body[4].Value.Range = f.Body[3].Value.Range
		}},
		{"caller-H-mismatch", func(p *arrayGenericPayload) {
			r := nestedHolderFunction25z(p, "run")
			(*r.Body[4].Value.Arguments)[1].Binding = r.Body[0].Binding
		}},
		{"constructor-H-mismatch", func(p *arrayGenericPayload) {
			r := nestedHolderFunction25z(p, "run")
			(*(*r.Body[4].Value.Arguments)[0].Arguments)[0].Binding = r.Body[1].Binding
		}},
		{"caller-B-C-order", func(p *arrayGenericPayload) {
			r := nestedHolderFunction25z(p, "run")
			args := *r.Body[4].Value.Arguments
			args[2], args[3] = args[3], args[2]
		}},
		{"formal-declaration", func(p *arrayGenericPayload) {
			f := nestedHolderFunction25z(p, "first")
			f.Parameters[1].DeclarationRange = f.Parameters[0].DeclarationRange
		}},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			var copied arrayGenericPackage
			if err := json.Unmarshal(out.Bytes(), &copied); err != nil {
				t.Fatal(err)
			}
			mutation.edit(&copied.Payload)
			if holderAliasContract25ac(&copied.Payload, source, original) == nil {
				t.Fatal("copied DTO corruption accepted")
			}
		})
	}
}
