// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/scanner"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

// This extra return observation is absent from the older nested-return cases.
// A separate original Program owns its syntax/symbol oracle; it does not call
// the facts projector or infer storage, borrow lifetime or Core acceptance.
func foreignRelayOriginal(t *testing.T, source string) map[string]span {
	t.Helper()
	program := compiler.NewProgram(compiler.ProgramOptions{Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
		FileNames:       []string{"/src/main.ts", "/src/minimal-arrays-v8.d.ts"},
		CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext},
	}}, Host: compiler.NewCompilerHost("/src", vfstest.FromMap(map[string]string{"/src/main.ts": source, "/src/minimal-arrays-v8.d.ts": string(arrayGenericDeclarations)}, true), "/src", nil, nil), SingleThreaded: core.TSTrue})
	for _, ds := range [][]*ast.Diagnostic{program.GetSyntacticDiagnostics(t.Context(), nil), program.GetSemanticDiagnostics(t.Context(), nil), program.GetProgramDiagnostics(), program.GetGlobalDiagnostics(t.Context())} {
		if len(ds) != 0 {
			t.Fatalf("independent official diagnostics: %+v", ds)
		}
	}
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	if len(file.Statements.Nodes) != 6 {
		t.Fatal("original two classes and four functions")
	}
	funcs, classes := map[string]*ast.Node{}, map[string]*ast.Node{}
	for _, n := range file.Statements.Nodes {
		if n.Name() == nil {
			t.Fatal("unnamed original declaration")
		}
		if n.Kind == ast.KindClassDeclaration {
			classes[n.Name().Text()] = n
		} else if n.Kind == ast.KindFunctionDeclaration {
			funcs[n.Name().Text()] = n
		} else {
			t.Fatal("unexpected original declaration", n.Kind)
		}
	}
	if len(classes) != 2 || len(funcs) != 4 || classes["H"] == nil || classes["O"] == nil {
		t.Fatal("original H/O declaration closure")
	}
	for _, name := range []string{"inner", "wrap", "relay", "run"} {
		if funcs[name] == nil {
			t.Fatal("missing original function", name)
		}
	}
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
	is := func(n, declaration *ast.Node) bool {
		symbol := check.GetSymbolAtLocation(n)
		return symbol != nil && len(symbol.Declarations) == 1 && symbol.Declarations[0] == declaration
	}
	inner, wrap, relay, run := funcs["inner"], funcs["wrap"], funcs["relay"], funcs["run"]
	hc, oc := classes["H"].Members()[0], classes["O"].Members()[0]
	hf, of := hc.AsConstructorDeclaration().Parameters.Nodes[0], oc.AsConstructorDeclaration().Parameters.Nodes[0]
	nh, no := node("new H(readSpan(a,0,1))", ast.KindNewExpression), node("new O(inner(a))", ast.KindNewExpression)
	ic, wc, rc := node("inner(a)", ast.KindCallExpression), node("wrap(a)", ast.KindCallExpression), node("relay(a)", ast.KindCallExpression)
	view := nh.AsNewExpression().Arguments.Nodes[0]
	read := node("relay(a).h.span[0]", ast.KindElementAccessExpression)
	spanRead := read.AsElementAccessExpression().Expression
	hRead := spanRead.AsPropertyAccessExpression().Expression
	for _, pair := range []struct{ fn, returned *ast.Node }{{inner, nh}, {wrap, no}, {relay, wc}} {
		body := pair.fn.Body().AsBlock().Statements.Nodes
		if len(body) != 1 || body[0].Kind != ast.KindReturnStatement || body[0].AsReturnStatement().Expression != pair.returned {
			t.Fatal("original normal return edge changed")
		}
	}
	if check.GetResolvedSignature(nh).Declaration() != hc || check.GetResolvedSignature(no).Declaration() != oc ||
		check.GetResolvedSignature(ic).Declaration() != inner || check.GetResolvedSignature(wc).Declaration() != wrap || check.GetResolvedSignature(rc).Declaration() != relay ||
		no.AsNewExpression().Arguments.Nodes[0] != ic || hRead.AsPropertyAccessExpression().Expression != rc ||
		!is(spanRead.Name(), hf) || !is(hRead.Name(), of) {
		t.Fatal("independent selected construction/call/field chain")
	}
	for _, pair := range []struct{ call, fn *ast.Node }{{view, inner}, {ic, wrap}, {wc, relay}} {
		if !is(pair.call.AsCallExpression().Arguments.Nodes[0], pair.fn.AsFunctionDeclaration().Parameters.Nodes[0]) {
			t.Fatal("independent actual argument/formal identity")
		}
	}
	runBody := run.Body().AsBlock().Statements.Nodes
	if len(runBody) != 2 || runBody[1].AsReturnStatement().Expression != read {
		t.Fatal("original complete caller statements")
	}
	local := runBody[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0]
	if !is(rc.AsCallExpression().Arguments.Nodes[0], local) {
		t.Fatal("relay actual uses the caller Array declaration")
	}
	at := map[string]span{}
	for key, n := range map[string]*ast.Node{"inner": inner, "wrap": wrap, "relay": relay, "run": run,
		"inner.param": inner.AsFunctionDeclaration().Parameters.Nodes[0], "wrap.param": wrap.AsFunctionDeclaration().Parameters.Nodes[0], "relay.param": relay.AsFunctionDeclaration().Parameters.Nodes[0],
		"H.field": hf, "O.field": of, "newH": nh, "newO": no, "innerCall": ic, "wrapCall": wc, "relayCall": rc, "view": view, "read": read, "spanRead": spanRead, "hRead": hRead, "array": local.AsVariableDeclaration().Initializer} {
		at[key] = span{file.GetPositionMap().UTF8ToUTF16(scanner.SkipTrivia(source, n.Pos())), file.GetPositionMap().UTF8ToUTF16(n.End())}
	}
	t.Log("independent official Program: four diagnostic groups0, two actual constructors, inner/wrap/relay calls, normal returns, field/formal symbols and UTF16 verified; no lifetime inference")
	return at
}

func foreignRelayContract(p *arrayGenericPayload, source string, at map[string]span) error {
	fail := func(reason string) error { return fmt.Errorf("original foreign relay facts differ: %s", reason) }
	if len(p.Diagnostics) != 0 || p.PositionUnit != "utf16" || len(p.Files) != 2 || len(p.Functions) != 4 || len(p.Exports) != 1 {
		return fail("complete source closure")
	}
	fileFound := false
	for _, f := range p.Files {
		if f.Path == "/src/main.ts" {
			fileFound = f.Content == source && f.SHA256 == arrayFactsSHA([]byte(source)) && f.ByteLength == len(source) && f.UTF16Length == len(source)
		}
	}
	if !fileFound {
		return fail("exact original ASCII source and UTF16")
	}
	types := map[string]arrayGenericType{}
	for _, typ := range p.Types {
		if typ.Name == "H" || typ.Name == "O" {
			types[typ.Name] = typ
		}
	}
	h, o := types["H"], types["O"]
	if h.Fields == nil || o.Fields == nil || len(*h.Fields) != 1 || len(*o.Fields) != 1 || h.Construction == nil || o.Construction == nil {
		return fail("original constructor/field closure")
	}
	hf, of := (*h.Fields)[0], (*o.Fields)[0]
	if hf.Name != "span" || hf.Type != "readSpan<i32>" || of.Name != "h" || of.Type != h.ID || hf.Origin.Start != at["H.field"].Start || hf.Origin.End != at["H.field"].End || of.Origin.Start != at["O.field"].Start || of.Origin.End != at["O.field"].End {
		return fail("original H.span and O.h declarations")
	}
	functions := map[string]*arrayGenericFunction{}
	for _, name := range []string{"inner", "wrap", "relay", "run"} {
		fn := nestedHolderFunction25z(p, name)
		if fn == nil || fn.DeclarationRange != at[name] || fn.SourcePath != "/src/main.ts" {
			return fail("original function declaration")
		}
		functions[name] = fn
	}
	inner, wrap, relay, run := functions["inner"], functions["wrap"], functions["relay"], functions["run"]
	for _, fn := range []*arrayGenericFunction{inner, wrap, relay} {
		if len(fn.Parameters) != 1 || fn.Parameters[0].DeclaredType != "array<i32>" || fn.Parameters[0].DeclarationRange != at[fn.Name+".param"] || len(fn.Body) != 1 || fn.Body[0].Kind != "return" || fn.Body[0].Value == nil {
			return fail("complete single-parameter helper and normal return")
		}
	}
	if inner.ReturnType != h.ID || wrap.ReturnType != o.ID || relay.ReturnType != o.ID || run.ReturnType != "i32" || len(run.Parameters) != 0 || len(run.Body) != 2 || p.Exports[0].FunctionID != run.ID {
		return fail("helper return types and complete exported caller")
	}
	nh, no, wc := inner.Body[0].Value, wrap.Body[0].Value, relay.Body[0].Value
	for _, row := range []struct {
		value  *arrayGenericExpression
		typ    arrayGenericType
		anchor string
	}{{nh, h, "newH"}, {no, o, "newO"}} {
		if row.value.Kind != "new" || row.value.Callee != row.typ.ID || row.value.Range != at[row.anchor] || row.value.Signature == nil || row.value.Signature.ID != row.typ.Construction.Signature.ID || row.value.Arguments == nil || len(*row.value.Arguments) != 1 {
			return fail("actual new H/O and construction signatures")
		}
	}
	view, ic := (*nh.Arguments)[0], (*no.Arguments)[0]
	if view.Kind != "call" || view.Callee != p.ReadSpanIntrinsic.ID || view.Range != at["view"] || view.Arguments == nil || len(*view.Arguments) != 3 || (*view.Arguments)[0].Binding != inner.Parameters[0].ID || (*view.Arguments)[1].Decimal != "0" || (*view.Arguments)[2].Decimal != "1" {
		return fail("actual Span construction uses inner Array parameter")
	}
	if run.Body[0].Kind != "constDeclaration" || run.Body[0].Binding == "" || run.Body[0].Initializer == nil || run.Body[1].Kind != "return" || run.Body[1].Value == nil {
		return fail("original caller local/return")
	}
	lit := run.Body[0].Initializer
	if lit.Kind != "arrayLiteral" || lit.Range != at["array"] || lit.Elements == nil || len(*lit.Elements) != 1 || (*lit.Elements)[0].Decimal != "1" {
		return fail("actual caller Array literal")
	}
	read := run.Body[1].Value
	if read.Kind != "checkedElementAccess" || read.Type != "i32" || read.Range != at["read"] || read.Index == nil || read.Index.Decimal != "0" || read.Receiver == nil || read.Receiver.Kind != "fieldAccess" || read.Receiver.Property != hf.ID || read.Receiver.Range != at["spanRead"] || read.Receiver.Receiver == nil || read.Receiver.Receiver.Kind != "fieldAccess" || read.Receiver.Receiver.Property != of.ID || read.Receiver.Receiver.Range != at["hRead"] || read.Receiver.Receiver.Receiver == nil {
		return fail("actual returned O.h.span checked read")
	}
	rc := read.Receiver.Receiver.Receiver
	for _, row := range []struct {
		value           *arrayGenericExpression
		callee          *arrayGenericFunction
		binding, anchor string
	}{{&ic, inner, wrap.Parameters[0].ID, "innerCall"}, {wc, wrap, relay.Parameters[0].ID, "wrapCall"}, {rc, relay, run.Body[0].Binding, "relayCall"}} {
		if row.value.Kind != "call" || row.value.Callee != row.callee.ID || row.value.Range != at[row.anchor] || row.value.Signature == nil || row.value.Signature.DeclarationRange != row.callee.DeclarationRange || row.value.Arguments == nil || len(*row.value.Arguments) != 1 || (*row.value.Arguments)[0].Binding != row.binding {
			return fail("actual selected inner/wrap/relay call and formal identity")
		}
	}
	return nil
}

func TestArrayGenericV8ForeignConstructionRelayReturn(t *testing.T) {
	raw, err := os.ReadFile("testdata/array-facts-v8/foreign-construction-relay-return.ts")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	at := foreignRelayOriginal(t, source)
	input := genericArrayRequest25f(t, source)
	var output bytes.Buffer
	err = runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(input), &output)
	// Preserve the actual stream even if this ordinary producer rejects Source.
	genericArrayCapture25f(t, input, source, output.Bytes())
	if err != nil {
		t.Fatalf("actual private-v8 relay Source failed: %v", err)
	}
	var packet arrayGenericPackage
	if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	if packet.Version != 8 || packet.Family != "tsfg.array-facts-package-candidate" {
		t.Fatal("actual public v8 packet identity")
	}
	if err := foreignRelayContract(&packet.Payload, source, at); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{"relay-callee", "relay-argument", "relay-signature", "relay-return-type", "caller-callee", "inner-callee", "read-field", "missing-relay"} {
		t.Run(mutation, func(t *testing.T) {
			var copy arrayGenericPackage
			if err := json.Unmarshal(output.Bytes(), &copy); err != nil {
				t.Fatal(err)
			}
			p := &copy.Payload
			fn := nestedHolderFunction25z(p, "relay")
			run := nestedHolderFunction25z(p, "run")
			switch mutation {
			case "relay-callee":
				fn.Body[0].Value.Callee = fn.ID
			case "relay-argument":
				(*fn.Body[0].Value.Arguments)[0].Binding = run.Body[0].Binding
			case "relay-signature":
				fn.Body[0].Value.Signature.DeclarationRange = nestedHolderFunction25z(p, "inner").DeclarationRange
			case "relay-return-type":
				fn.ReturnType = nestedHolderFunction25z(p, "inner").ReturnType
			case "caller-callee":
				run.Body[1].Value.Receiver.Receiver.Receiver.Callee = nestedHolderFunction25z(p, "wrap").ID
			case "inner-callee":
				(*nestedHolderFunction25z(p, "wrap").Body[0].Value.Arguments)[0].Callee = fn.ID
			case "read-field":
				run.Body[1].Value.Receiver.Property = run.Body[1].Value.Receiver.Receiver.Property
			case "missing-relay":
				for i := range p.Functions {
					if p.Functions[i].Name == "relay" {
						p.Functions = append(p.Functions[:i], p.Functions[i+1:]...)
						break
					}
				}
			}
			if foreignRelayContract(p, source, at) == nil {
				t.Fatal("copied factual corruption accepted")
			}
		})
	}
}
