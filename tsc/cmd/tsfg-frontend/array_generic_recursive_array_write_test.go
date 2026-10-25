// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/scanner"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

// A fresh original Program keeps this source-specific oracle independent of
// producer/DTO state and the prior numeric oracle. Recursion termination, current
// cell versions, numeric results and runtime effects are deliberately not inferred.
func recursiveWriteOriginal(t *testing.T, source string) map[string]span {
	t.Helper()
	program := compiler.NewProgram(compiler.ProgramOptions{Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
		FileNames:       []string{"/src/main.ts", "/src/minimal-arrays-v8.d.ts"},
		CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext},
	}}, Host: compiler.NewCompilerHost("/src", vfstest.FromMap(map[string]string{"/src/main.ts": source, "/src/minimal-arrays-v8.d.ts": string(arrayGenericDeclarations)}, true), "/src", nil, nil), SingleThreaded: core.TSTrue})
	for _, ds := range [][]*ast.Diagnostic{program.GetSyntacticDiagnostics(t.Context(), nil), program.GetSemanticDiagnostics(t.Context(), nil), program.GetProgramDiagnostics(), program.GetGlobalDiagnostics(t.Context())} {
		if len(ds) != 0 {
			t.Fatalf("original official diagnostics: %+v", ds)
		}
	}
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	at := map[string]span{}
	anchor := func(k string, n *ast.Node) {
		at[k] = span{file.GetPositionMap().UTF8ToUTF16(scanner.SkipTrivia(source, n.Pos())), file.GetPositionMap().UTF8ToUTF16(n.End())}
	}
	text := func(n *ast.Node) string { return source[scanner.SkipTrivia(source, n.Pos()):n.End()] }
	same := func(n, d *ast.Node) {
		s := check.GetSymbolAtLocation(n)
		if n.Kind != ast.KindIdentifier || s == nil || len(s.Declarations) != 1 || s.Declarations[0] != d {
			t.Fatal("original binding symbol", text(n))
		}
	}
	union := func(typ *checker.Type) {
		if typ.Flags()&checker.TypeFlagsUnion == 0 || len(typ.Types()) != 2 {
			t.Fatal("original union shape")
		}
		number, undefined := false, false
		for _, m := range typ.Types() {
			number = number || m == check.GetNumberType()
			undefined = undefined || m == check.GetUndefinedType()
		}
		if !number || !undefined {
			t.Fatal("original i32/undefined union members")
		}
	}
	index := func(k string, n *ast.Node) {
		if n.Kind != ast.KindNumericLiteral || text(n) != "0" || check.GetTypeAtLocation(n).Flags()&checker.TypeFlagsNumberLike == 0 {
			t.Fatal("original zero index")
		}
		anchor(k, n)
	}
	funcs := map[string]*ast.Node{}
	names := []string{"fill", "runTrue", "runFalse", "runUnknown", "cycle", "runCycle", "runCycleBase"}
	if len(file.Statements.Nodes) != len(names) {
		t.Fatal("complete seven original functions")
	}
	for i, n := range file.Statements.Nodes {
		if n.Kind != ast.KindFunctionDeclaration || n.Name().Text() != names[i] {
			t.Fatal("original function name/order")
		}
		name := names[i]
		funcs[name] = n
		anchor(name, n)
		anchor(name+".name", n.Name())
		anchor(name+".return.type", n.AsFunctionDeclaration().Type)
		writer := name == "fill" || name == "cycle"
		want := "i32|undefined"
		if writer {
			want = "void"
		}
		if text(n.AsFunctionDeclaration().Type) != want || (n.ModifierFlags()&ast.ModifierFlagsExport != 0) == writer {
			t.Fatal("original annotation/export", name)
		}
		for _, p := range n.AsFunctionDeclaration().Parameters.Nodes {
			anchor(name+"."+p.Name().Text(), p)
		}
	}
	call := func(name, target string, n, a, binding *ast.Node, boolToken string) {
		if n.Kind != ast.KindCallExpression {
			t.Fatal("original call", name)
		}
		c := n.AsCallExpression()
		sig := check.GetResolvedSignature(n)
		if sig == nil || sig.Declaration() != funcs[target] || len(sig.Parameters()) != 2 || len(c.Arguments.Nodes) != 2 || check.GetTypeAtLocation(n).Flags()&checker.TypeFlagsVoid == 0 {
			t.Fatal("original selected void signature", name)
		}
		same(c.Expression, funcs[target])
		same(c.Arguments.Nodes[0], a)
		arg := c.Arguments.Nodes[1]
		if !check.GetTypeAtLocation(arg).IsBooleanLike() {
			t.Fatal("original bool argument")
		}
		if binding != nil {
			same(arg, binding)
		} else if text(arg) != boolToken || (arg.Kind != ast.KindTrueKeyword && arg.Kind != ast.KindFalseKeyword) {
			t.Fatal("original literal bool argument", name)
		}
		anchor(name+".call", n)
		anchor(name+".callee", c.Expression)
		anchor(name+".call.a", c.Arguments.Nodes[0])
		anchor(name+".call.bool", arg)
	}
	for _, name := range []string{"fill", "cycle"} {
		fn := funcs[name]
		ps := fn.AsFunctionDeclaration().Parameters.Nodes
		body := fn.Body().AsBlock().Statements.Nodes
		if len(ps) != 2 || len(body) != 2 || ps[0].Name().Text() != "a" || ps[1].Name().Text() != "again" || text(ps[0].AsParameterDeclaration().Type) != "(i32|undefined)[]" || text(ps[1].AsParameterDeclaration().Type) != "bool" || body[0].Kind != ast.KindIfStatement || body[1].Kind != ast.KindExpressionStatement {
			t.Fatal("original writer structure", name)
		}
		cond := body[0].AsIfStatement()
		same(cond.Expression, ps[1])
		if !check.GetTypeAtLocation(cond.Expression).IsBooleanLike() || cond.ElseStatement != nil || cond.ThenStatement.Kind != ast.KindBlock {
			t.Fatal("original if condition", name)
		}
		anchor(name+".if", body[0])
		anchor(name+".condition", cond.Expression)
		then := cond.ThenStatement.AsBlock().Statements.Nodes
		if len(then) != 2 || then[0].Kind != ast.KindExpressionStatement || then[1].Kind != ast.KindReturnStatement || then[1].AsReturnStatement().Expression != nil {
			t.Fatal("original recursive then/void return")
		}
		token := "false"
		if name == "cycle" {
			token = "true"
		}
		anchor(name+".call.statement", then[0])
		call(name, name, then[0].AsExpressionStatement().Expression, ps[0], nil, token)
		anchor(name+".early.return", then[1])
		store := body[1].AsExpressionStatement().Expression
		if store.Kind != ast.KindBinaryExpression || store.AsBinaryExpression().OperatorToken.Kind != ast.KindEqualsToken {
			t.Fatal("original array assignment")
		}
		b := store.AsBinaryExpression()
		if b.Left.Kind != ast.KindElementAccessExpression || text(b.Right) != "undefined" || check.GetTypeAtLocation(b.Right) != check.GetUndefinedType() {
			t.Fatal("original store target/value")
		}
		access := b.Left.AsElementAccessExpression()
		same(access.Expression, ps[0])
		union(check.GetTypeAtLocation(b.Left))
		anchor(name+".store.statement", body[1])
		anchor(name+".store", store)
		anchor(name+".store.target", b.Left)
		anchor(name+".store.operator", b.OperatorToken)
		anchor(name+".store.a", access.Expression)
		index(name+".store.index", access.ArgumentExpression)
		anchor(name+".store.value", b.Right)
	}
	for i, name := range []string{"runTrue", "runFalse", "runUnknown", "runCycle", "runCycleBase"} {
		fn := funcs[name]
		ps := fn.AsFunctionDeclaration().Parameters.Nodes
		body := fn.Body().AsBlock().Statements.Nodes
		count := 0
		if name == "runUnknown" {
			count = 1
		}
		if len(ps) != count || len(body) != 3 || body[0].Kind != ast.KindVariableStatement || body[1].Kind != ast.KindExpressionStatement || body[2].Kind != ast.KindReturnStatement {
			t.Fatal("original driver shape", name)
		}
		list := body[0].AsVariableStatement().DeclarationList
		if list.Flags&ast.NodeFlagsConst == 0 || len(list.AsVariableDeclarationList().Declarations.Nodes) != 1 {
			t.Fatal("original fresh const")
		}
		a := list.AsVariableDeclarationList().Declarations.Nodes[0]
		decl := a.AsVariableDeclaration()
		if a.Name().Text() != "a" || text(decl.Type) != "(i32|undefined)[]" || decl.Initializer.Kind != ast.KindArrayLiteralExpression || len(decl.Initializer.AsArrayLiteralExpression().Elements.Nodes) != 1 {
			t.Fatal("original array literal")
		}
		typ := check.GetTypeAtLocation(a.Name())
		if !check.IsArrayType(typ) || len(check.GetTypeArguments(typ)) != 1 {
			t.Fatal("original actual Array")
		}
		union(check.GetTypeArguments(typ)[0])
		element := decl.Initializer.AsArrayLiteralExpression().Elements.Nodes[0]
		if element.Kind != ast.KindNumericLiteral || text(element) != fmt.Sprint(i+1) || check.GetTypeAtLocation(element).Flags()&checker.TypeFlagsNumberLike == 0 {
			t.Fatal("original independent array element")
		}
		anchor(name+".a", a)
		anchor(name+".array", decl.Initializer)
		anchor(name+".element", element)
		anchor(name+".call.statement", body[1])
		target, token := "fill", "true"
		if name == "runFalse" || name == "runCycleBase" {
			token = "false"
		}
		if name == "runCycle" || name == "runCycleBase" {
			target = "cycle"
		}
		var argument *ast.Node
		if name == "runUnknown" {
			argument = ps[0]
			if argument.Name().Text() != "again" || text(argument.AsParameterDeclaration().Type) != "bool" {
				t.Fatal("original unknown bool parameter")
			}
		}
		call(name, target, body[1].AsExpressionStatement().Expression, a, argument, token)
		read := body[2].AsReturnStatement().Expression
		if read.Kind != ast.KindElementAccessExpression {
			t.Fatal("original driver read")
		}
		same(read.AsElementAccessExpression().Expression, a)
		union(check.GetTypeAtLocation(read))
		anchor(name+".return", body[2])
		anchor(name+".read", read)
		anchor(name+".read.a", read.AsElementAccessExpression().Expression)
		index(name+".read.index", read.AsElementAccessExpression().ArgumentExpression)
	}
	t.Log("independent original Program:7functions/5exports/5fresh array literals/2stores/5reads/7calls,4diagnostics0, selected signatures/bool arguments/UTF16; no termination or effect oracle")
	return at
}

func recursiveWriteContract(p *arrayGenericPayload, source string, at map[string]span) error {
	fail := func(s string) error { return fmt.Errorf("original recursive array facts differ: %s", s) }
	if p.SourceRoot != "/src" || p.RootFile != "/src/main.ts" || p.PositionUnit != "utf16" || len(p.Files) != 2 || len(p.Functions) != 7 || len(p.Exports) != 5 || len(p.Diagnostics) != 0 {
		return fail("complete original closure")
	}
	found := false
	for _, f := range p.Files {
		if f.Path == "/src/main.ts" {
			found = f.Content == source && f.ByteLength == 782 && f.UTF16Length == 782 && f.SHA256 == arrayFactsSHA([]byte(source))
		}
	}
	if !found {
		return fail("complete pinned original source")
	}
	fs := map[string]*arrayGenericFunction{}
	for i := range p.Functions {
		f := &p.Functions[i]
		if fs[f.Name] != nil {
			return fail("duplicate function")
		}
		fs[f.Name] = f
	}
	bindings := map[string]arrayFactsBinding{}
	ids := map[string]bool{}
	for _, name := range []string{"fill", "runTrue", "runFalse", "runUnknown", "cycle", "runCycle", "runCycleBase"} {
		f := fs[name]
		typ := "union<i32,undefined>"
		if name == "fill" || name == "cycle" {
			typ = "void"
		}
		if f == nil || f.ID == "" || f.SourcePath != "/src/main.ts" || f.DeclarationRange != at[name] || f.NameRange != at[name+".name"] || f.ReturnType != typ || f.ReturnAnnotationRange == nil || *f.ReturnAnnotationRange != at[name+".return.type"] {
			return fail("original signature " + name)
		}
		for _, b := range append(append([]arrayFactsBinding{}, f.Parameters...), f.Bindings...) {
			typ := "bool"
			if b.Name == "a" {
				typ = "array<union<i32,undefined>>"
			}
			if b.ID == "" || ids[b.ID] || b.DeclaredType != typ || b.DeclarationRange != at[name+"."+b.Name] {
				return fail("original binding " + name)
			}
			ids[b.ID] = true
			bindings[name+"."+b.Name] = b
		}
	}
	exports := map[string]bool{}
	for _, e := range p.Exports {
		if exports[e.FunctionID] || e.FunctionID == fs["fill"].ID || e.FunctionID == fs["cycle"].ID {
			return fail("original export identity")
		}
		exports[e.FunctionID] = true
	}
	for _, name := range []string{"runTrue", "runFalse", "runUnknown", "runCycle", "runCycleBase"} {
		if !exports[fs[name].ID] {
			return fail("missing original export")
		}
	}
	ex := func(e *arrayGenericExpression, kind, typ, key string) bool {
		return e != nil && e.Kind == kind && e.Type == typ && e.FlowType == typ && e.Range == at[key]
	}
	ref := func(e *arrayGenericExpression, name, b, key string) bool {
		v := bindings[name+"."+b]
		return ex(e, "bindingReference", v.DeclaredType, key) && e.Binding == v.ID
	}
	index := func(e *arrayGenericExpression, key string) bool {
		return ex(e, "numericLiteral", "u32", key) && e.Token == "0" && e.Decimal == "0"
	}
	call := func(e *arrayGenericExpression, name, target, token string) bool {
		if !ex(e, "call", "void", name+".call") || e.Callee != fs[target].ID || e.CalleeRange == nil || *e.CalleeRange != at[name+".callee"] || e.Signature == nil || e.Signature.SourcePath != "/src/main.ts" || e.Signature.DeclarationRange != at[target] || e.Signature.ReturnType != "void" || len(e.Signature.Parameters) != 2 || e.Signature.Parameters[0] != "array<union<i32,undefined>>" || e.Signature.Parameters[1] != "bool" || e.Arguments == nil || len(*e.Arguments) != 2 {
			return false
		}
		args := *e.Arguments
		if !ref(&args[0], name, "a", name+".call.a") {
			return false
		}
		if name == "runUnknown" {
			return ref(&args[1], name, "again", name+".call.bool")
		}
		return ex(&args[1], "booleanLiteral", "bool", name+".call.bool") && args[1].Token == token
	}
	for _, name := range []string{"fill", "cycle"} {
		f := fs[name]
		if len(f.Parameters) != 2 || len(f.Bindings) != 0 || len(f.Body) != 2 {
			return fail("original writer body")
		}
		cond := f.Body[0]
		if cond.Kind != "if" || cond.Range != at[name+".if"] || !ref(cond.Condition, name, "again", name+".condition") || cond.Then == nil || len(*cond.Then) != 2 || cond.Else == nil || len(*cond.Else) != 0 {
			return fail("original writer condition")
		}
		then := *cond.Then
		token := "false"
		if name == "cycle" {
			token = "true"
		}
		if then[0].Kind != "expression" || then[0].Range != at[name+".call.statement"] || !call(then[0].Value, name, name, token) || then[1].Kind != "return" || then[1].Range != at[name+".early.return"] || then[1].Value != nil {
			return fail("selected original recursive call/void return")
		}
		s := f.Body[1]
		e := s.Value
		if s.Kind != "expression" || s.Range != at[name+".store.statement"] || e == nil || e.Kind != "elementAssignment" || e.Type != "union<i32,undefined>" || e.FlowType != "undefined" || e.Range != at[name+".store"] || e.Operator != "=" || e.OperatorRange == nil || *e.OperatorRange != at[name+".store.operator"] || e.Target == nil {
			return fail("original post-if assignment")
		}
		target := e.Target
		if target.Kind != "elementTarget" || target.Type != "union<i32,undefined>" || target.Range != at[name+".store.target"] || !ref(target.Receiver, name, "a", name+".store.a") || !index(target.Index, name+".store.index") || !ex(e.Value, "undefinedLiteral", "undefined", name+".store.value") {
			return fail("original array store operands")
		}
	}
	allocations := map[string]bool{}
	for i, name := range []string{"runTrue", "runFalse", "runUnknown", "runCycle", "runCycleBase"} {
		f := fs[name]
		count := 0
		if name == "runUnknown" {
			count = 1
		}
		if len(f.Parameters) != count || len(f.Bindings) != 1 || len(f.Body) != 3 {
			return fail("original driver arity/body")
		}
		b := bindings[name+".a"]
		s := f.Body[0]
		a := s.Initializer
		if b.Writable || s.Kind != "constDeclaration" || s.Binding != b.ID || s.Range != at[name+".a"] || !ex(a, "arrayLiteral", "array<union<i32,undefined>>", name+".array") || a.AllocationSite == "" || allocations[a.AllocationSite] || a.Elements == nil || len(*a.Elements) != 1 {
			return fail("five distinct original array literals")
		}
		allocations[a.AllocationSite] = true
		value := (*a.Elements)[0]
		if !ex(&value, "numericLiteral", "i32", name+".element") || value.Token != fmt.Sprint(i+1) || value.Decimal != fmt.Sprint(i+1) {
			return fail("original driver element token")
		}
		target, token := "fill", "true"
		if name == "runFalse" || name == "runCycleBase" {
			token = "false"
		}
		if name == "runCycle" || name == "runCycleBase" {
			target = "cycle"
		}
		if f.Body[1].Kind != "expression" || f.Body[1].Range != at[name+".call.statement"] || !call(f.Body[1].Value, name, target, token) {
			return fail("original driver call and bool argument")
		}
		ret := f.Body[2]
		if ret.Kind != "return" || ret.Range != at[name+".return"] || !ex(ret.Value, "checkedElementAccess", "union<i32,undefined>", name+".read") || !ref(ret.Value.Receiver, name, "a", name+".read.a") || !index(ret.Value.Index, name+".read.index") {
			return fail("original same-array return access")
		}
	}
	return nil
}

func TestArrayGenericV8RecursiveArrayWriteFacts(t *testing.T) {
	raw, err := os.ReadFile("testdata/array-facts-v8/recursive-array-write-final.ts")
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 782 || arrayFactsSHA(raw) != "88978983a7a1dd667efd74bf9c47e4def5b881f925fd08c006cfccadcf297305" {
		t.Fatal("complete final782B source pin; old448B unused")
	}
	source := string(raw)
	at := recursiveWriteOriginal(t, source)
	request := genericArrayRequest25f(t, source)
	var output bytes.Buffer
	if err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(request), &output); err != nil {
		t.Fatal(err)
	}
	genericArrayCapture25f(t, request, source, output.Bytes())
	var packet arrayGenericPackage
	if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	if packet.Family != "tsfg.array-facts-package-candidate" || packet.Version != 8 || packet.RequestID == "" || packet.PackageID == "" {
		t.Fatal("original publicv8 identity")
	}
	if err := recursiveWriteContract(&packet.Payload, source, at); err != nil {
		t.Fatal(err)
	}
}
