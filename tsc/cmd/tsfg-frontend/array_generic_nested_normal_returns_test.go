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

// One original-compiler oracle serves all three actual return cases. Earlier
// setter/method oracles assert different ASTs and are deliberately unchanged.
func nestedReturnOriginal(t *testing.T, source string, activation, resizeB bool) map[string]span {
	t.Helper()
	program := compiler.NewProgram(compiler.ProgramOptions{Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
		FileNames:       []string{"/src/main.ts", "/src/minimal-arrays-v8.d.ts"},
		CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext},
	}}, Host: compiler.NewCompilerHost("/src", vfstest.FromMap(map[string]string{"/src/main.ts": source, "/src/minimal-arrays-v8.d.ts": string(arrayGenericDeclarations)}, true), "/src", nil, nil), SingleThreaded: core.TSTrue})
	for _, diagnostics := range [][]*ast.Diagnostic{program.GetSyntacticDiagnostics(t.Context(), nil), program.GetSemanticDiagnostics(t.Context(), nil), program.GetProgramDiagnostics(), program.GetGlobalDiagnostics(t.Context())} {
		if len(diagnostics) != 0 {
			t.Fatalf("independent official diagnostics: %+v", diagnostics)
		}
	}
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	nodes := map[string]*ast.Node{}
	var walk func(*ast.Node)
	walk = func(n *ast.Node) {
		start := scanner.SkipTrivia(source, n.Pos())
		if start < n.End() {
			nodes[source[start:n.End()]] = n
		}
		n.ForEachChild(func(child *ast.Node) bool { walk(child); return false })
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
	functions := map[string]*ast.Node{}
	classes := map[string]*ast.Node{}
	for _, n := range file.Statements.Nodes {
		if n.Name() == nil {
			continue
		}
		if n.Kind == ast.KindClassDeclaration {
			classes[n.Name().Text()] = n
		}
		if n.Kind == ast.KindFunctionDeclaration {
			functions[n.Name().Text()] = n
		}
	}
	inner, wrap := functions["inner"], functions["wrap"]
	h, o := classes["H"], classes["O"]
	if inner == nil || wrap == nil || h == nil || o == nil {
		t.Fatal("original helper/class closure missing")
	}
	ip, wp := inner.AsFunctionDeclaration().Parameters.Nodes[0], wrap.AsFunctionDeclaration().Parameters.Nodes[0]
	hc, oc := h.Members()[0], o.Members()[0]
	hf, of := hc.AsConstructorDeclaration().Parameters.Nodes[0], oc.AsConstructorDeclaration().Parameters.Nodes[0]
	nh := node("new H(readSpan(a,0,1))", ast.KindNewExpression)
	no := node("new O(inner(a))", ast.KindNewExpression)
	ic := no.AsNewExpression().Arguments.Nodes[0]
	view := nh.AsNewExpression().Arguments.Nodes[0]
	is := func(n, decl *ast.Node) bool {
		s := check.GetSymbolAtLocation(n)
		return s != nil && len(s.Declarations) == 1 && s.Declarations[0] == decl
	}
	if inner.Body().AsBlock().Statements.Nodes[0].AsReturnStatement().Expression != nh || wrap.Body().AsBlock().Statements.Nodes[0].AsReturnStatement().Expression != no ||
		check.GetResolvedSignature(nh).Declaration() != hc || check.GetResolvedSignature(no).Declaration() != oc || check.GetResolvedSignature(ic).Declaration() != inner ||
		!is(view.AsCallExpression().Arguments.Nodes[0], ip) || !is(ic.AsCallExpression().Arguments.Nodes[0], wp) {
		t.Fatal("actual helper return/constructor/inner formal relationship changed")
	}
	text := "wrap(a).h.span[0]"
	if activation {
		text = "current.h.span[0]"
	}
	read := node(text, ast.KindElementAccessExpression)
	spanRead := read.AsElementAccessExpression().Expression
	hRead := spanRead.AsPropertyAccessExpression().Expression
	if !is(spanRead.Name(), hf) || !is(hRead.Name(), of) {
		t.Fatal("original nested return field declarations changed")
	}
	ca := node("wrap(a)", ast.KindCallExpression)
	if check.GetResolvedSignature(ca).Declaration() != wrap {
		t.Fatal("actual first selected wrap call changed")
	}
	for key, n := range map[string]*ast.Node{"inner": inner, "wrap": wrap, "inner.param": ip, "wrap.param": wp, "H.field": hf, "O.field": of, "newH": nh, "newO": no, "innerCall": ic, "view": view, "callA": ca, "read": read, "spanRead": spanRead, "hRead": hRead} {
		record(key, n)
	}
	if activation {
		cb := node("wrap(b)", ast.KindCallExpression)
		current := node("current:O=wrap(b)", ast.KindVariableDeclaration)
		old := node("old:O=wrap(a)", ast.KindVariableDeclaration)
		resized := "a.length=0"
		if resizeB {
			resized = "b.length=0"
		}
		resize := node(resized, ast.KindBinaryExpression)
		if check.GetResolvedSignature(cb).Declaration() != wrap || ca == cb || !is(hRead.AsPropertyAccessExpression().Expression, current) ||
			check.GetSymbolAtLocation(ca.AsCallExpression().Arguments.Nodes[0]) == check.GetSymbolAtLocation(cb.AsCallExpression().Arguments.Nodes[0]) ||
			check.GetSymbolAtLocation(resize.AsBinaryExpression().Left.AsPropertyAccessExpression().Expression) != check.GetSymbolAtLocation(map[bool]*ast.Node{false: ca, true: cb}[resizeB].AsCallExpression().Arguments.Nodes[0]) ||
			old.End() >= current.Pos() || current.End() >= resize.Pos() || resize.End() >= read.Pos() {
			t.Fatal("two actual helper calls, original distinct roots/locals or resize/read order changed")
		}
		record("callB", cb)
		record("resize", resize)
	} else if hRead.AsPropertyAccessExpression().Expression != ca {
		t.Fatal("original read must use direct wrap return")
	}
	t.Log("independent official Program: four diagnostic groups0; selected inner/wrap/constructors, original return/field/formal symbols, separate calls and UTF16 verified")
	return at
}

func nestedReturnContract(p *arrayGenericPayload, source string, at map[string]span, activation, resizeB bool) error {
	fail := func(s string) error { return fmt.Errorf("original nested-return facts differ: %s", s) }
	if len(p.Diagnostics) != 0 || p.PositionUnit != "utf16" || len(p.Files) != 2 || len(p.Functions) != 3 || len(p.Exports) != 1 {
		return fail("complete source closure")
	}
	found := false
	for _, f := range p.Files {
		if f.Path == "/src/main.ts" {
			found = f.Content == source && f.SHA256 == arrayFactsSHA([]byte(source)) && f.ByteLength == len(source) && f.UTF16Length == len(source)
		}
	}
	if !found {
		return fail("exact original ASCII source and UTF16")
	}
	types := map[string]arrayGenericType{}
	for _, v := range p.Types {
		if v.Name == "H" || v.Name == "O" {
			types[v.Name] = v
		}
	}
	h, o := types["H"], types["O"]
	if h.Fields == nil || o.Fields == nil || len(*h.Fields) != 1 || len(*o.Fields) != 1 || h.Construction == nil || o.Construction == nil {
		return fail("original constructor/field closure")
	}
	hf, of := (*h.Fields)[0], (*o.Fields)[0]
	if hf.Type != "readSpan<i32>" || hf.Name != "span" || of.Type != h.ID || of.Name != "h" || hf.Origin.Start != at["H.field"].Start || hf.Origin.End != at["H.field"].End || of.Origin.Start != at["O.field"].Start || of.Origin.End != at["O.field"].End {
		return fail("original H.span and O.h identities")
	}
	inner, wrap, run := nestedHolderFunction25z(p, "inner"), nestedHolderFunction25z(p, "wrap"), nestedHolderFunction25z(p, "run")
	if inner == nil || wrap == nil || run == nil || len(inner.Parameters) != 1 || len(wrap.Parameters) != 1 || len(inner.Body) != 1 || len(wrap.Body) != 1 || inner.ReturnType != h.ID || wrap.ReturnType != o.ID || inner.DeclarationRange != at["inner"] || wrap.DeclarationRange != at["wrap"] || inner.Parameters[0].DeclarationRange != at["inner.param"] || wrap.Parameters[0].DeclarationRange != at["wrap.param"] || p.Exports[0].FunctionID != run.ID {
		return fail("original helper declarations and return types")
	}
	nh, no := inner.Body[0].Value, wrap.Body[0].Value
	if inner.Body[0].Kind != "return" || wrap.Body[0].Kind != "return" || nh == nil || no == nil || nh.Kind != "new" || no.Kind != "new" || nh.Callee != h.ID || no.Callee != o.ID || nh.Range != at["newH"] || no.Range != at["newO"] || nh.Signature == nil || no.Signature == nil || nh.Signature.ID != h.Construction.Signature.ID || no.Signature.ID != o.Construction.Signature.ID || nh.Arguments == nil || no.Arguments == nil || len(*nh.Arguments) != 1 || len(*no.Arguments) != 1 {
		return fail("inner creates H then wrap stores actual inner result in O constructor")
	}
	view, ic := (*nh.Arguments)[0], (*no.Arguments)[0]
	if view.Kind != "call" || view.Callee != p.ReadSpanIntrinsic.ID || view.Range != at["view"] || view.Arguments == nil || len(*view.Arguments) != 3 || (*view.Arguments)[0].Binding != inner.Parameters[0].ID || (*view.Arguments)[1].Decimal != "0" || (*view.Arguments)[2].Decimal != "1" || ic.Kind != "call" || ic.Callee != inner.ID || ic.Range != at["innerCall"] || ic.Signature == nil || ic.Signature.DeclarationRange != inner.DeclarationRange || ic.Arguments == nil || len(*ic.Arguments) != 1 || (*ic.Arguments)[0].Binding != wrap.Parameters[0].ID {
		return fail("actual selected inner call and original formals feed complete returns")
	}
	expectedBody := 2
	if activation {
		expectedBody = 6
	}
	if len(run.Body) != expectedBody || run.ReturnType != "i32" || len(run.Parameters) != 0 {
		return fail("complete caller statement sequence")
	}
	read := run.Body[expectedBody-1].Value
	if read == nil || read.Kind != "checkedElementAccess" || read.Type != "i32" || read.Range != at["read"] || read.Receiver == nil || read.Receiver.Kind != "fieldAccess" || read.Receiver.Property != hf.ID || read.Receiver.Range != at["spanRead"] || read.Receiver.Receiver == nil || read.Receiver.Receiver.Property != of.ID || read.Receiver.Receiver.Type != h.ID || read.Receiver.Receiver.Range != at["hRead"] || read.Receiver.Receiver.Receiver == nil {
		return fail("original returned O.h then H.span checked read")
	}
	callA := read.Receiver.Receiver.Receiver
	if activation {
		if run.Body[2].Binding == "" || run.Body[3].Binding == "" || run.Body[2].Binding == run.Body[3].Binding || callA.Binding != run.Body[3].Binding {
			return fail("final field read uses actual second activation current")
		}
		callA = run.Body[2].Initializer
		cb := run.Body[3].Initializer
		if cb == nil || cb.Kind != "call" || cb.Callee != wrap.ID || cb.Range != at["callB"] || cb.Signature == nil || cb.Signature.DeclarationRange != wrap.DeclarationRange || cb.Arguments == nil || len(*cb.Arguments) != 1 || (*cb.Arguments)[0].Binding != run.Body[1].Binding {
			return fail("independent second actual wrap(b)")
		}
		resize := run.Body[4].Value
		binding := run.Body[0].Binding
		if resizeB {
			binding = run.Body[1].Binding
		}
		if resize == nil || resize.Kind != "lengthAssignment" || resize.Range != at["resize"] || resize.Target == nil || resize.Target.Receiver == nil || resize.Target.Receiver.Binding != binding || resize.Value == nil || resize.Value.Decimal != "0" || cb.Range.End >= resize.Range.Start || resize.Range.End >= read.Range.Start {
			return fail("actual A or B resize after both calls before current read")
		}
	}
	if callA == nil || callA.Kind != "call" || callA.Callee != wrap.ID || callA.Range != at["callA"] || callA.Signature == nil || callA.Signature.DeclarationRange != wrap.DeclarationRange || callA.Arguments == nil || len(*callA.Arguments) != 1 || (*callA.Arguments)[0].Binding != run.Body[0].Binding {
		return fail("selected wrap(a) and original caller root")
	}
	for i := 0; i < map[bool]int{false: 1, true: 2}[activation]; i++ {
		lit := run.Body[i].Initializer
		if lit == nil || lit.Kind != "arrayLiteral" || lit.Elements == nil || len(*lit.Elements) != 1 || (*lit.Elements)[0].Decimal != fmt.Sprint(i+1) {
			return fail("complete original distinct caller arrays")
		}
	}
	return nil
}

func TestArrayGenericV8NestedNormalReturns(t *testing.T) {
	for _, row := range []struct {
		name                string
		activation, resizeB bool
	}{{"nested-normal-return-positive", false, false}, {"nested-normal-return-activation-independent", true, false}, {"nested-normal-return-activation-conflict", true, true}} {
		t.Run(row.name, func(t *testing.T) {
			raw, err := os.ReadFile("testdata/array-facts-v8/" + row.name + ".ts")
			if err != nil {
				t.Fatal(err)
			}
			source := string(raw)
			at := nestedReturnOriginal(t, source, row.activation, row.resizeB)
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
			genericArrayCapture25f(t, input, source, out.Bytes())
			var packet arrayGenericPackage
			if err := json.Unmarshal(out.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}
			if packet.Version != 8 || packet.Family != "tsfg.array-facts-package-candidate" {
				t.Fatal("original public packet identity changed")
			}
			if err := nestedReturnContract(&packet.Payload, source, at, row.activation, row.resizeB); err != nil {
				t.Fatal(err)
			}
			// Only the newly introduced return/field edges need extra copied DTO checks.
			for _, mutation := range []string{"wrong-inner-return-callee", "wrong-return-field"} {
				t.Run(mutation, func(t *testing.T) {
					var copied arrayGenericPackage
					if err := json.Unmarshal(out.Bytes(), &copied); err != nil {
						t.Fatal(err)
					}
					if mutation == "wrong-inner-return-callee" {
						w := nestedHolderFunction25z(&copied.Payload, "wrap")
						(*w.Body[0].Value.Arguments)[0].Callee = w.ID
					} else {
						r := nestedHolderFunction25z(&copied.Payload, "run")
						e := r.Body[len(r.Body)-1].Value
						e.Receiver.Property = e.Receiver.Receiver.Property
					}
					if nestedReturnContract(&copied.Payload, source, at, row.activation, row.resizeB) == nil {
						t.Fatal("copied return/field corruption accepted")
					}
				})
			}
		})
	}
}
