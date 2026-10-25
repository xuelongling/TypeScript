// SPDX-License-Identifier: Apache-2.0
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/scanner"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
	"testing"
)

// This independent Program checks source facts only. Numeric consumption of an
// absence payload, runtime traps and consumer representation are not its oracle.
func validNumericOriginal(t *testing.T, source string) map[string]span {
	t.Helper()
	program := compiler.NewProgram(compiler.ProgramOptions{Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
		FileNames:       []string{"/src/main.ts", "/src/minimal-arrays-v8.d.ts"},
		CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext},
	}}, Host: compiler.NewCompilerHost("/src", vfstest.FromMap(map[string]string{"/src/main.ts": source, "/src/minimal-arrays-v8.d.ts": string(arrayGenericDeclarations)}, true), "/src", nil, nil), SingleThreaded: core.TSTrue})
	for _, diagnostics := range [][]*ast.Diagnostic{program.GetSyntacticDiagnostics(t.Context(), nil), program.GetSemanticDiagnostics(t.Context(), nil), program.GetProgramDiagnostics(), program.GetGlobalDiagnostics(t.Context())} {
		if len(diagnostics) != 0 {
			t.Fatalf("independent original diagnostics: %+v", diagnostics)
		}
	}
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	at := map[string]span{}
	anchor := func(key string, n *ast.Node) {
		at[key] = span{file.GetPositionMap().UTF8ToUTF16(scanner.SkipTrivia(source, n.Pos())), file.GetPositionMap().UTF8ToUTF16(n.End())}
	}
	text := func(n *ast.Node) string { return source[scanner.SkipTrivia(source, n.Pos()):n.End()] }
	number := func(n *ast.Node) {
		if check.GetTypeAtLocation(n).Flags()&checker.TypeFlagsNumberLike == 0 {
			t.Fatal("original numeric static type", text(n))
		}
	}
	boolean := func(n *ast.Node) {
		if !check.GetTypeAtLocation(n).IsBooleanLike() {
			t.Fatal("original boolean static type", text(n))
		}
	}
	same := func(n, decl *ast.Node) {
		s := check.GetSymbolAtLocation(n)
		if n.Kind != ast.KindIdentifier || s == nil || len(s.Declarations) != 1 || s.Declarations[0] != decl {
			t.Fatal("original identifier declaration", text(n))
		}
	}
	names := []string{"keep", "binary", "unary", "short", "numeric"}
	if len(file.Statements.Nodes) != len(names) {
		t.Fatal("complete original five-function closure")
	}
	functions := map[string]*ast.Node{}
	for j, n := range file.Statements.Nodes {
		if n.Kind != ast.KindFunctionDeclaration || n.Name().Text() != names[j] {
			t.Fatal("original function order/name")
		}
		name := names[j]
		functions[name] = n
		anchor(name, n)
		anchor(name+".name", n.Name())
		anchor(name+".return.type", n.AsFunctionDeclaration().Type)
		want := "i32"
		if name == "short" {
			want = "bool"
		}
		if text(n.AsFunctionDeclaration().Type) != want || (n.ModifierFlags()&ast.ModifierFlagsExport != 0) != (name != "keep") {
			t.Fatal("original return annotation/export", name)
		}
		for _, p := range n.AsFunctionDeclaration().Parameters.Nodes {
			anchor(name+"."+p.Name().Text(), p)
		}
	}
	local := func(name string, index int) *ast.Node {
		body := functions[name].Body().AsBlock().Statements.Nodes
		if index >= len(body) || body[index].Kind != ast.KindVariableStatement {
			t.Fatal("original local", name, index)
		}
		list := body[index].AsVariableStatement().DeclarationList
		if list.Flags&ast.NodeFlagsConst == 0 || len(list.AsVariableDeclarationList().Declarations.Nodes) != 1 {
			t.Fatal("one original const", name)
		}
		n := list.AsVariableDeclarationList().Declarations.Nodes[0]
		anchor(name+"."+n.Name().Text(), n)
		return n
	}
	literal := func(key string, n *ast.Node, token string) {
		anchor(key, n)
		if text(n) != token {
			t.Fatal("original literal token", key)
		}
		if token == "undefined" {
			if check.GetTypeAtLocation(n) != check.GetUndefinedType() {
				t.Fatal("original undefined type")
			}
		} else {
			if n.Kind != ast.KindNumericLiteral {
				t.Fatal("original numeric literal")
			}
			number(n)
		}
	}
	array := func(name string) *ast.Node {
		a := local(name, 0)
		decl := a.AsVariableDeclaration()
		typ := check.GetTypeAtLocation(a.Name())
		if a.Name().Text() != "a" || text(decl.Type) != "(i32|undefined)[]" || !check.IsArrayType(typ) {
			t.Fatal("original declared union Array", name)
		}
		args := check.GetTypeArguments(typ)
		if len(args) != 1 || args[0].Flags()&checker.TypeFlagsUnion == 0 || len(args[0].Types()) != 2 {
			t.Fatal("original Array element union")
		}
		seenNumber, seenUndefined := false, false
		for _, m := range args[0].Types() {
			seenNumber = seenNumber || m == check.GetNumberType()
			seenUndefined = seenUndefined || m == check.GetUndefinedType()
		}
		if !seenNumber || !seenUndefined || decl.Initializer.Kind != ast.KindArrayLiteralExpression || len(decl.Initializer.AsArrayLiteralExpression().Elements.Nodes) != 1 {
			t.Fatal("original complete union/array initializer")
		}
		anchor(name+".array", decl.Initializer)
		token := "undefined"
		if name == "numeric" {
			token = "1"
		}
		literal(name+".element", decl.Initializer.AsArrayLiteralExpression().Elements.Nodes[0], token)
		return a
	}
	keep := functions["keep"]
	params := keep.AsFunctionDeclaration().Parameters.Nodes
	if len(params) != 2 || params[0].Name().Text() != "a" || params[1].Name().Text() != "i" || text(params[0].AsParameterDeclaration().Type) != "(i32|undefined)[]" || text(params[1].AsParameterDeclaration().Type) != "u32" {
		t.Fatal("original keep parameters")
	}
	kbody := keep.Body().AsBlock().Statements.Nodes
	if len(kbody) != 3 {
		t.Fatal("original keep body")
	}
	x, alias := local("keep", 0), local("keep", 1)
	cast := x.AsVariableDeclaration().Initializer
	if x.Name().Text() != "x" || alias.Name().Text() != "alias" || text(x.AsVariableDeclaration().Type) != "i32" || text(alias.AsVariableDeclaration().Type) != "i32" || cast.Kind != ast.KindAsExpression {
		t.Fatal("original assertion/alias declarations")
	}
	anchor("keep.as", cast)
	target := cast.AsAsExpression().Type
	anchor("keep.target", target)
	number(cast)
	number(target)
	if text(target) != "i32" || cast.AsAsExpression().Expression.Kind != ast.KindElementAccessExpression {
		t.Fatal("original as target/operand")
	}
	access := cast.AsAsExpression().Expression
	anchor("keep.access", access)
	read := access.AsElementAccessExpression()
	same(read.Expression, params[0])
	same(read.ArgumentExpression, params[1])
	anchor("keep.array", read.Expression)
	anchor("keep.index", read.ArgumentExpression)
	operandType := check.GetTypeAtLocation(access)
	if operandType.Flags()&checker.TypeFlagsUnion == 0 || len(operandType.Types()) != 2 {
		t.Fatal("original checked access keeps union")
	}
	seenNumber, seenUndefined := false, false
	for _, m := range operandType.Types() {
		seenNumber = seenNumber || m == check.GetNumberType()
		seenUndefined = seenUndefined || m == check.GetUndefinedType()
	}
	if !seenNumber || !seenUndefined {
		t.Fatal("original checked access members")
	}
	start := scanner.SkipTrivia(source, access.End())
	if source[start:start+2] != "as" {
		t.Fatal("original as token")
	}
	at["keep.operator"] = span{file.GetPositionMap().UTF8ToUTF16(start), file.GetPositionMap().UTF8ToUTF16(start + 2)}
	same(alias.AsVariableDeclaration().Initializer, x)
	anchor("keep.alias.value", alias.AsVariableDeclaration().Initializer)
	if kbody[2].Kind != ast.KindReturnStatement {
		t.Fatal("original keep return")
	}
	anchor("keep.return", kbody[2])
	same(kbody[2].AsReturnStatement().Expression, alias)
	anchor("keep.return.value", kbody[2].AsReturnStatement().Expression)
	number(kbody[2].AsReturnStatement().Expression)
	call := func(name string, n, a *ast.Node) {
		if n.Kind != ast.KindCallExpression {
			t.Fatal("original keep call", name)
		}
		c := n.AsCallExpression()
		if len(c.Arguments.Nodes) != 2 || check.GetResolvedSignature(n).Declaration() != keep {
			t.Fatal("original resolved keep signature", name)
		}
		same(c.Expression, keep)
		same(c.Arguments.Nodes[0], a)
		number(n)
		anchor(name+".call", n)
		anchor(name+".callee", c.Expression)
		anchor(name+".call.a", c.Arguments.Nodes[0])
		literal(name+".call.i", c.Arguments.Nodes[1], "0")
	}
	binary := func(key string, n *ast.Node, op string) {
		if n.Kind != ast.KindBinaryExpression || scanner.TokenToString(n.AsBinaryExpression().OperatorToken.Kind) != op {
			t.Fatal("original binary operator", key)
		}
		anchor(key, n)
		anchor(key+".operator", n.AsBinaryExpression().OperatorToken)
	}
	for _, name := range []string{"binary", "unary"} {
		fn := functions[name]
		body := fn.Body().AsBlock().Statements.Nodes
		ps := fn.AsFunctionDeclaration().Parameters.Nodes
		if len(body) != 4 || len(ps) != 1 || ps[0].Name().Text() != "use" || text(ps[0].AsParameterDeclaration().Type) != "bool" {
			t.Fatal("original conditional function shape", name)
		}
		a := array(name)
		value := local(name, 1)
		if value.Name().Text() != "x" || text(value.AsVariableDeclaration().Type) != "i32" {
			t.Fatal("original call result binding")
		}
		call(name, value.AsVariableDeclaration().Initializer, a)
		if body[2].Kind != ast.KindIfStatement || body[3].Kind != ast.KindReturnStatement {
			t.Fatal("original if/fallback", name)
		}
		cond := body[2].AsIfStatement()
		same(cond.Expression, ps[0])
		boolean(cond.Expression)
		anchor(name+".if", body[2])
		anchor(name+".condition", cond.Expression)
		if cond.ThenStatement.Kind != ast.KindBlock || cond.ElseStatement != nil || len(cond.ThenStatement.AsBlock().Statements.Nodes) != 1 {
			t.Fatal("original then branch")
		}
		ret := cond.ThenStatement.AsBlock().Statements.Nodes[0]
		if ret.Kind != ast.KindReturnStatement {
			t.Fatal("original conditional return")
		}
		anchor(name+".then.return", ret)
		v := ret.AsReturnStatement().Expression
		number(v)
		if name == "binary" {
			binary(name+".operation", v, "+")
			same(v.AsBinaryExpression().Left, value)
			anchor(name+".operand", v.AsBinaryExpression().Left)
			literal(name+".one", v.AsBinaryExpression().Right, "1")
		} else {
			if v.Kind != ast.KindPrefixUnaryExpression || v.AsPrefixUnaryExpression().Operator != ast.KindMinusToken {
				t.Fatal("original unary minus")
			}
			same(v.AsPrefixUnaryExpression().Operand, value)
			anchor(name+".operation", v)
			anchor(name+".operand", v.AsPrefixUnaryExpression().Operand)
			s := scanner.SkipTrivia(source, v.Pos())
			at[name+".operation.operator"] = span{file.GetPositionMap().UTF8ToUTF16(s), file.GetPositionMap().UTF8ToUTF16(s + 1)}
		}
		anchor(name+".return", body[3])
		literal(name+".fallback", body[3].AsReturnStatement().Expression, "7")
	}
	for _, name := range []string{"short", "numeric"} {
		fn := functions[name]
		body := fn.Body().AsBlock().Statements.Nodes
		ps := fn.AsFunctionDeclaration().Parameters.Nodes
		if len(body) != 2 || body[1].Kind != ast.KindReturnStatement {
			t.Fatal("original final return", name)
		}
		a := array(name)
		anchor(name+".return", body[1])
		v := body[1].AsReturnStatement().Expression
		if name == "short" {
			if len(ps) != 1 || ps[0].Name().Text() != "use" || text(ps[0].AsParameterDeclaration().Type) != "bool" {
				t.Fatal("original short parameter")
			}
			binary("short.logical", v, "&&")
			boolean(v)
			same(v.AsBinaryExpression().Left, ps[0])
			anchor("short.condition", v.AsBinaryExpression().Left)
			paren := v.AsBinaryExpression().Right
			if paren.Kind != ast.KindParenthesizedExpression {
				t.Fatal("original short RHS wrapper")
			}
			anchor("short.parentheses", paren)
			compare := paren.AsParenthesizedExpression().Expression
			binary("short.compare", compare, ">")
			boolean(compare)
			call(name, compare.AsBinaryExpression().Left, a)
			literal("short.zero", compare.AsBinaryExpression().Right, "0")
		} else {
			if len(ps) != 0 {
				t.Fatal("original numeric parameters")
			}
			binary("numeric.operation", v, "+")
			number(v)
			call(name, v.AsBinaryExpression().Left, a)
			literal("numeric.one", v.AsBinaryExpression().Right, "1")
		}
	}
	t.Log("independent Program: five functions, four diagnostics0, original signatures/symbols/static types/UTF16; no runtime consumption oracle")
	return at
}
