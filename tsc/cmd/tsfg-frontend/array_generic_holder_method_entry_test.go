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

// The original compiler instance is independent of the public converter and
// consumer host-entry/lifetime analysis. No consumer outcome is an oracle.
func holderMethodOriginal(t *testing.T, source string) map[string]span {
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
	node := func(text string, kind ast.Kind) *ast.Node {
		t.Helper()
		n := nodes[text]
		if n == nil || n.Kind != kind {
			t.Fatal("missing original syntax", text)
		}
		return n
	}
	at := map[string]span{}
	record := func(key string, n *ast.Node) {
		t.Helper()
		if n == nil {
			t.Fatal("missing original node", key)
		}
		at[key] = span{file.GetPositionMap().UTF8ToUTF16(scanner.SkipTrivia(source, n.Pos())), file.GetPositionMap().UTF8ToUTF16(n.End())}
	}
	classes := map[string]*ast.Node{}
	for _, n := range file.Statements.Nodes {
		if n.Kind == ast.KindClassDeclaration {
			classes[n.Name().Text()] = n
		}
	}
	h, o, p := classes["H"], classes["O"], classes["P"]
	if h == nil || o == nil || p == nil || len(p.Members()) != 1 || p.Members()[0].Kind != ast.KindMethodDeclaration {
		t.Fatal("P owns one actual instance method, not a constructor")
	}
	method := p.Members()[0]
	params := method.AsMethodDeclaration().Parameters.Nodes
	if len(params) != 5 || ast.IsStatic(method) {
		t.Fatal("five original explicit formals and implicit instance receiver required")
	}
	hCtor, oCtor := h.Members()[0], o.Members()[0]
	hField, oField := hCtor.AsConstructorDeclaration().Parameters.Nodes[0], oCtor.AsConstructorDeclaration().Parameters.Nodes[0]
	x := node("x = o.h", ast.KindVariableDeclaration)
	w1 := node("x.span = readSpan(b, 0, 1)", ast.KindBinaryExpression)
	w2 := node("h.span = readSpan(c, 0, 1)", ast.KindBinaryExpression)
	branch := node("if (shrink) { c.length = 0; }", ast.KindIfStatement)
	resize := node("c.length = 0", ast.KindBinaryExpression)
	read := node("x.span[0]", ast.KindElementAccessExpression)
	is := func(n, decl *ast.Node) bool {
		s := check.GetSymbolAtLocation(n)
		return s != nil && len(s.Declarations) == 1 && s.Declarations[0] == decl
	}
	if !is(x.AsVariableDeclaration().Initializer.Name(), oField) ||
		!is(x.AsVariableDeclaration().Initializer.AsPropertyAccessExpression().Expression, params[0]) ||
		!is(w1.AsBinaryExpression().Left.Name(), hField) || !is(w2.AsBinaryExpression().Left.Name(), hField) ||
		!is(w1.AsBinaryExpression().Left.AsPropertyAccessExpression().Expression, x) ||
		!is(w2.AsBinaryExpression().Left.AsPropertyAccessExpression().Expression, params[1]) ||
		!is(w1.AsBinaryExpression().Right.AsCallExpression().Arguments.Nodes[0], params[2]) ||
		!is(w2.AsBinaryExpression().Right.AsCallExpression().Arguments.Nodes[0], params[3]) ||
		!is(branch.AsIfStatement().Expression, params[4]) ||
		!is(resize.AsBinaryExpression().Left.AsPropertyAccessExpression().Expression, params[3]) ||
		!is(read.AsElementAccessExpression().Expression.Name(), hField) ||
		!is(read.AsElementAccessExpression().Expression.AsPropertyAccessExpression().Expression, x) ||
		!(w1.End() < w2.Pos() && w2.End() < branch.Pos() && resize.End() < read.Pos()) {
		t.Fatal("original method symbols, branch flag and write/resize/read sequence changed")
	}
	call := node("new P().first(new O(h), h, b, c, false)", ast.KindCallExpression)
	receiver := call.AsCallExpression().Expression.AsPropertyAccessExpression().Expression
	args := call.AsCallExpression().Arguments.Nodes
	newH := node("new H(readSpan(a, 0, 1))", ast.KindNewExpression)
	if len(args) != 5 || receiver.Kind != ast.KindNewExpression || args[4].Kind != ast.KindFalseKeyword ||
		check.GetResolvedSignature(call).Declaration() != method || check.GetResolvedSignature(args[0]).Declaration() != oCtor ||
		check.GetResolvedSignature(newH).Declaration() != hCtor ||
		check.GetTypeAtLocation(receiver) != check.GetDeclaredTypeOfSymbol(check.GetSymbolAtLocation(p.Name())) ||
		check.GetSymbolAtLocation(args[0].AsNewExpression().Arguments.Nodes[0]) != check.GetSymbolAtLocation(args[1]) {
		t.Fatal("actual instance receiver, selected method/constructors, false or caller same h changed")
	}
	for key, n := range map[string]*ast.Node{"method": method, "owner": p, "H.field": hField, "O.field": oField,
		"xInit": x.AsVariableDeclaration().Initializer, "write0": w1, "write1": w2, "branch": branch, "condition": branch.AsIfStatement().Expression,
		"resize": resize, "read": read, "call": call, "receiver": receiver, "newH": newH} {
		record(key, n)
	}
	for i, param := range params {
		record(fmt.Sprint("param", i), param)
		record(fmt.Sprint("arg", i), args[i])
	}
	for i, w := range []*ast.Node{w1, w2} {
		record(fmt.Sprint("rhs", i), w.AsBinaryExpression().Right)
		record(fmt.Sprint("target", i), w.AsBinaryExpression().Left)
	}
	t.Log("independent official Program: four diagnostic groups0; actual P.first instance method, five explicit formals plus separate receiver, flag branch, original symbols/signatures/order/UTF16 verified")
	return at
}

func holderMethodContract(p *arrayGenericPayload, source string, at map[string]span) error {
	fail := func(s string) error { return fmt.Errorf("original method-entry facts differ: %s", s) }
	if p.PositionUnit != "utf16" || len(p.Diagnostics) != 0 || len(p.Functions) != 2 || len(p.Exports) != 1 || len(p.Files) != 2 {
		return fail("complete checked source closure")
	}
	found := false
	for _, f := range p.Files {
		if f.Path == "/src/main.ts" {
			found = f.Content == source && f.SHA256 == arrayFactsSHA([]byte(source)) && f.ByteLength == len(source) && f.UTF16Length == len(source)
		}
	}
	if !found {
		return fail("complete original ASCII bytes/UTF16")
	}
	types := map[string]arrayGenericType{}
	for _, typ := range p.Types {
		if typ.Name == "H" || typ.Name == "O" || typ.Name == "P" {
			types[typ.Name] = typ
		}
	}
	h, o, owner := types["H"], types["O"], types["P"]
	if h.Fields == nil || o.Fields == nil || len(*h.Fields) != 1 || len(*o.Fields) != 1 || owner.ID == "" ||
		owner.Constructor != nil || owner.Construction == nil || owner.Construction.Kind != "implicit-base" || len(owner.Construction.Parameters) != 0 {
		return fail("original fields and P implicit constructor are distinct from method")
	}
	hf, of := (*h.Fields)[0], (*o.Fields)[0]
	if hf.Name != "span" || hf.Type != "readSpan<i32>" || of.Name != "h" || of.Type != h.ID || hf.ID == of.ID ||
		hf.Origin.Start != at["H.field"].Start || hf.Origin.End != at["H.field"].End ||
		of.Origin.Start != at["O.field"].Start || of.Origin.End != at["O.field"].End {
		return fail("same original M12 H.span and O.h declarations")
	}
	m, run := nestedHolderFunction25z(p, "first"), nestedHolderFunction25z(p, "run")
	if m == nil || run == nil || m.Name != "first" || m.OwnerType != owner.ID || m.DeclarationRange != at["method"] ||
		m.ReturnType != "i32" || len(m.Parameters) != 5 || len(m.Body) != 5 || len(owner.Methods) != 1 || owner.Methods[0] != m.ID ||
		len(run.Parameters) != 0 || len(run.Body) != 5 || p.Exports[0].FunctionID != run.ID {
		return fail("actual nonconstructor ownerType method; five explicit parameters")
	}
	for i, typ := range []string{o.ID, h.ID, "array<i32>", "array<i32>", "bool"} {
		if m.Parameters[i].Kind != "parameter" || m.Parameters[i].DeclaredType != typ || m.Parameters[i].DeclarationRange != at[fmt.Sprint("param", i)] {
			return fail("ordered original explicit formals; receiver must not be fabricated as parameter")
		}
	}
	x := m.Body[0]
	if x.Binding == "" || x.Initializer == nil || x.Initializer.Kind != "fieldAccess" || x.Initializer.Property != of.ID ||
		x.Initializer.Range != at["xInit"] || x.Initializer.Receiver == nil || x.Initializer.Receiver.Binding != m.Parameters[0].ID {
		return fail("x is original o.h local alias")
	}
	for i, binding := range []string{x.Binding, m.Parameters[1].ID} {
		w := m.Body[i+1].Value
		if w == nil || w.Kind != "fieldAssignment" || w.Range != at[fmt.Sprint("write", i)] || w.Target == nil ||
			w.Target.Property != hf.ID || w.Target.Range != at[fmt.Sprint("target", i)] || w.Target.Receiver == nil ||
			w.Target.Receiver.Binding != binding || w.Target.Receiver.Type != h.ID || w.Value == nil || w.Value.Kind != "call" ||
			w.Value.Callee != p.ReadSpanIntrinsic.ID || w.Value.Range != at[fmt.Sprint("rhs", i)] ||
			w.Value.Arguments == nil || len(*w.Value.Arguments) != 3 || (*w.Value.Arguments)[0].Binding != m.Parameters[i+2].ID ||
			(*w.Value.Arguments)[1].Decimal != "0" || (*w.Value.Arguments)[2].Decimal != "1" {
			return fail("ordered same-field writes from original formal b then c")
		}
	}
	branch, read := m.Body[3], m.Body[4].Value
	if branch.Kind != "if" || branch.Range != at["branch"] || branch.Condition == nil || branch.Condition.Kind != "bindingReference" ||
		branch.Condition.Binding != m.Parameters[4].ID || branch.Condition.Range != at["condition"] ||
		branch.Then == nil || len(*branch.Then) != 1 || branch.Else == nil || len(*branch.Else) != 0 {
		return fail("original unknown shrink condition and single then-only body")
	}
	resize := (*branch.Then)[0].Value
	if resize == nil || resize.Kind != "lengthAssignment" || resize.Range != at["resize"] || resize.Target == nil ||
		resize.Target.Receiver == nil || resize.Target.Receiver.Binding != m.Parameters[3].ID ||
		resize.Value == nil || resize.Value.Decimal != "0" || read == nil || read.Kind != "checkedElementAccess" || read.Type != "i32" ||
		read.Range != at["read"] || read.Receiver == nil || read.Receiver.Property != hf.ID || read.Receiver.Receiver == nil ||
		read.Receiver.Receiver.Binding != x.Binding || !(m.Body[1].Range.End < m.Body[2].Range.Start && branch.Range.End < read.Range.Start) {
		return fail("C resize in method branch precedes original x.span checked read")
	}
	for i := 0; i < 3; i++ {
		lit := run.Body[i].Initializer
		if run.Body[i].Binding == "" || lit == nil || lit.Kind != "arrayLiteral" || lit.Elements == nil || len(*lit.Elements) != 1 ||
			(*lit.Elements)[0].Decimal != fmt.Sprint(i+1) {
			return fail("separate complete caller arrays")
		}
	}
	newH, call := run.Body[3].Initializer, run.Body[4].Value
	if newH == nil || newH.Kind != "new" || newH.Callee != h.ID || newH.Range != at["newH"] ||
		newH.Arguments == nil || len(*newH.Arguments) != 1 || (*newH.Arguments)[0].Arguments == nil ||
		(*(*newH.Arguments)[0].Arguments)[0].Binding != run.Body[0].Binding ||
		call == nil || call.Kind != "call" || call.Callee != m.ID || call.Range != at["call"] || call.Receiver == nil ||
		call.Receiver.Kind != "new" || call.Receiver.Type != owner.ID || call.Receiver.Callee != owner.ID || call.Receiver.Range != at["receiver"] ||
		call.Receiver.Signature == nil || call.Receiver.Signature.ID != owner.Construction.Signature.ID ||
		call.Signature == nil || call.Signature.DeclarationRange != m.DeclarationRange || len(call.Signature.Parameters) != 5 ||
		call.Arguments == nil || len(*call.Arguments) != 5 {
		return fail("separate new P receiver, real H construction and selected instance method")
	}
	args := *call.Arguments
	if args[0].Kind != "new" || args[0].Callee != o.ID || args[0].Arguments == nil || len(*args[0].Arguments) != 1 ||
		(*args[0].Arguments)[0].Binding != run.Body[3].Binding || args[1].Binding != run.Body[3].Binding ||
		args[2].Binding != run.Body[1].Binding || args[3].Binding != run.Body[2].Binding ||
		args[4].Kind != "booleanLiteral" || args[4].Token != "false" {
		return fail("same caller H through O and formal h, ordered arrays and explicit false")
	}
	for i, arg := range args {
		if arg.Range != at[fmt.Sprint("arg", i)] || call.Signature.Parameters[i] != m.Parameters[i].DeclaredType {
			return fail("original selected explicit parameter/argument order and UTF16")
		}
	}
	return nil
}

func TestArrayGenericV8HolderMethodUnknownEntry(t *testing.T) {
	raw, err := os.ReadFile("testdata/array-facts-v8/holder-method-unknown-entry.ts")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	at := holderMethodOriginal(t, source)
	input := genericArrayRequest25f(t, source)
	capture := func(name string, b []byte) {
		t.Helper()
		if root := os.Getenv("TSFG_ARRAY_V8_CAPTURE_ROOT"); root != "" {
			if err := os.MkdirAll(root, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, arrayFactsSHA(raw)+"-"+name), b, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	capture("source.ts", raw)
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
		t.Fatal("original public v8 packet identity changed")
	}
	if err := holderMethodContract(&packet.Payload, source, at); err != nil {
		t.Fatal(err)
	}
	mutations := []struct {
		name string
		edit func(*arrayGenericPayload)
	}{
		{"owner-lost", func(p *arrayGenericPayload) { nestedHolderFunction25z(p, "first").OwnerType = "" }},
		{"constructor-confusion", func(p *arrayGenericPayload) { nestedHolderFunction25z(p, "first").Name = "constructor" }},
		{"fake-this-parameter", func(p *arrayGenericPayload) {
			m := nestedHolderFunction25z(p, "first")
			m.Parameters = append([]arrayFactsBinding{{Name: "this", Kind: "this", DeclaredType: m.OwnerType}}, m.Parameters...)
		}},
		{"flag-binding", func(p *arrayGenericPayload) {
			m := nestedHolderFunction25z(p, "first")
			m.Body[3].Condition.Binding = m.Parameters[2].ID
		}},
		{"resize-B", func(p *arrayGenericPayload) {
			m := nestedHolderFunction25z(p, "first")
			(*m.Body[3].Then)[0].Value.Target.Receiver.Binding = m.Parameters[2].ID
		}},
		{"second-write-x", func(p *arrayGenericPayload) {
			m := nestedHolderFunction25z(p, "first")
			m.Body[2].Value.Target.Receiver.Binding = m.Body[0].Binding
		}},
		{"second-RHS-B", func(p *arrayGenericPayload) {
			m := nestedHolderFunction25z(p, "first")
			(*m.Body[2].Value.Value.Arguments)[0].Binding = m.Parameters[2].ID
		}},
		{"read-formal-h", func(p *arrayGenericPayload) {
			m := nestedHolderFunction25z(p, "first")
			m.Body[4].Value.Receiver.Receiver.Binding = m.Parameters[1].ID
		}},
		{"caller-true", func(p *arrayGenericPayload) {
			(*nestedHolderFunction25z(p, "run").Body[4].Value.Arguments)[4].Token = "true"
		}},
		{"receiver-not-P", func(p *arrayGenericPayload) {
			nestedHolderFunction25z(p, "run").Body[4].Value.Receiver.Type = nestedHolderFunction25z(p, "first").Parameters[0].DeclaredType
		}},
		{"caller-H-mismatch", func(p *arrayGenericPayload) {
			r := nestedHolderFunction25z(p, "run")
			(*r.Body[4].Value.Arguments)[1].Binding = r.Body[0].Binding
		}},
		{"caller-B-C-order", func(p *arrayGenericPayload) {
			r := nestedHolderFunction25z(p, "run")
			args := *r.Body[4].Value.Arguments
			args[2], args[3] = args[3], args[2]
		}},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			var copied arrayGenericPackage
			if err := json.Unmarshal(out.Bytes(), &copied); err != nil {
				t.Fatal(err)
			}
			mutation.edit(&copied.Payload)
			if holderMethodContract(&copied.Payload, source, at) == nil {
				t.Fatal("copied DTO corruption accepted")
			}
		})
	}
}
