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

// The independent official Program proves syntax, symbols and static types.
// Operand effects and request traps are later consumer expectations, not facts
// inferred by this oracle or observed by this test.
func numericOrderOriginal(t *testing.T, source string) map[string]span {
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
	names := []string{"left", "right", "ordered", "rhsFails", "plus", "plusValue", "direct", "caller"}
	if len(file.Statements.Nodes) != len(names) {
		t.Fatal("complete eight-function source")
	}
	functions := map[string]*ast.Node{}
	at := map[string]span{}
	text := func(n *ast.Node) string { return source[scanner.SkipTrivia(source, n.Pos()):n.End()] }
	for index, n := range file.Statements.Nodes {
		if n.Kind != ast.KindFunctionDeclaration || n.Name().Text() != names[index] || text(n.AsFunctionDeclaration().Type) != "i32" || (n.ModifierFlags()&ast.ModifierFlagsExport != 0) != (index >= 2) {
			t.Fatal("original function signature/order")
		}
		functions[n.Name().Text()] = n
	}
	for _, name := range names {
		fn := functions[name]
		for key, n := range map[string]*ast.Node{"$function": fn, "$name": fn.Name(), "$return-type": fn.AsFunctionDeclaration().Type} {
			at[name+"|"+key] = span{file.GetPositionMap().UTF8ToUTF16(scanner.SkipTrivia(source, n.Pos())), file.GetPositionMap().UTF8ToUTF16(n.End())}
		}
		var walk func(*ast.Node)
		walk = func(n *ast.Node) {
			start := scanner.SkipTrivia(source, n.Pos())
			if start < n.End() {
				at[name+"|"+source[start:n.End()]] = span{file.GetPositionMap().UTF8ToUTF16(start), file.GetPositionMap().UTF8ToUTF16(n.End())}
			}
			if n.Kind == ast.KindCallExpression {
				call := n.AsCallExpression()
				callee := functions[text(call.Expression)]
				if callee == nil || check.GetResolvedSignature(n).Declaration() != callee {
					t.Fatal("original selected call signature")
				}
				symbol := check.GetSymbolAtLocation(call.Expression)
				if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != callee || len(call.Arguments.Nodes) != len(callee.AsFunctionDeclaration().Parameters.Nodes) {
					t.Fatal("original call symbol/arity")
				}
			}
			if n.Kind == ast.KindPrefixUnaryExpression {
				if n.AsPrefixUnaryExpression().Operator != ast.KindPlusToken || check.GetTypeAtLocation(n).Flags()&checker.TypeFlagsNumberLike == 0 {
					t.Fatal("original numeric unary plus")
				}
			}
			if n.Kind == ast.KindIdentifier && text(n) == "a" {
				symbol := check.GetSymbolAtLocation(n)
				if symbol == nil || len(symbol.Declarations) != 1 || ast.GetSourceFileOfNode(symbol.Declarations[0]) != file {
					t.Fatal("original array binding symbol")
				}
			}
			n.ForEachChild(func(child *ast.Node) bool { walk(child); return false })
		}
		walk(fn)
	}
	for _, name := range []string{"plus", "plusValue"} {
		if _, ok := at[name+"|+(a[0] as i32)"]; !ok {
			t.Fatal("complete original unary source")
		}
	}
	for _, key := range []string{"ordered|left(a)+right(a)", "rhsFails|left(a)+(1/0)", "rhsFails|1/0", "left|a[1]=11", "right|a[1]=22", "direct|x+1", "caller|direct()+1"} {
		if _, ok := at[key]; !ok {
			t.Fatal("complete original ordered source", key)
		}
	}
	return at
}

func numericOrderContract(p *arrayGenericPayload, source string, at map[string]span) error {
	fail := func(s string) error { return fmt.Errorf("original numeric ordering facts differ: %s", s) }
	if p.SourceRoot != "/src" || p.RootFile != "/src/main.ts" || p.PositionUnit != "utf16" || len(p.Files) != 2 || len(p.Functions) != 8 || len(p.Exports) != 6 || len(p.Diagnostics) != 0 {
		return fail("complete closure")
	}
	found := false
	for _, f := range p.Files {
		if f.Path == "/src/main.ts" {
			found = f.Content == source && f.ByteLength == len(source) && f.UTF16Length == len(source) && f.SHA256 == arrayFactsSHA([]byte(source))
		}
	}
	if !found {
		return fail("complete original source bytes")
	}
	functions := map[string]*arrayGenericFunction{}
	for i := range p.Functions {
		f := &p.Functions[i]
		if functions[f.Name] != nil {
			return fail("duplicate function")
		}
		functions[f.Name] = f
	}
	for _, name := range []string{"left", "right", "ordered", "rhsFails", "plus", "plusValue", "direct", "caller"} {
		f := functions[name]
		bodyCount := 2
		if name == "direct" {
			bodyCount = 4
		} else if name == "caller" {
			bodyCount = 1
		}
		if f == nil || f.ReturnType != "i32" || f.SourcePath != "/src/main.ts" || len(f.Body) != bodyCount || f.DeclarationRange != at[name+"|$function"] || f.NameRange != at[name+"|$name"] || f.ReturnAnnotationRange == nil || *f.ReturnAnnotationRange != at[name+"|$return-type"] {
			return fail("function body/signature")
		}
		if name == "left" || name == "right" {
			if len(f.Parameters) != 1 || f.Parameters[0].DeclaredType != "array<union<i32,undefined>>" {
				return fail("writer parameter")
			}
		} else if name == "caller" {
			if len(f.Parameters) != 0 || len(f.Bindings) != 0 {
				return fail("caller arity")
			}
		} else {
			bindings := 1
			if name == "direct" {
				bindings = 3
			}
			if len(f.Parameters) != 0 || len(f.Bindings) != bindings || f.Bindings[0].Name != "a" || f.Bindings[0].DeclaredType != "array<union<i32,undefined>>" || f.Body[0].Kind != "constDeclaration" || f.Body[0].Binding != f.Bindings[0].ID {
				return fail("driver local")
			}
		}
	}
	exported := map[string]bool{}
	for _, e := range p.Exports {
		if exported[e.FunctionID] || e.FunctionID == functions["left"].ID || e.FunctionID == functions["right"].ID {
			return fail("unique driver exports")
		}
		exported[e.FunctionID] = true
	}
	for _, name := range []string{"ordered", "rhsFails", "plus", "plusValue", "direct", "caller"} {
		if !exported[functions[name].ID] {
			return fail("complete driver exports")
		}
	}
	for _, name := range []string{"ordered", "rhsFails"} {
		array := functions[name].Body[0].Initializer
		if array == nil || array.Kind != "arrayLiteral" || array.Elements == nil || len(*array.Elements) != 2 || (*array.Elements)[0].Kind != "undefinedLiteral" || (*array.Elements)[1].Decimal != "0" {
			return fail("original initialized ordered Array")
		}
	}
	ex := func(e *arrayGenericExpression, kind, name, snippet string) bool {
		return e != nil && e.Kind == kind && e.Range == at[name+"|"+snippet]
	}
	call := func(e *arrayGenericExpression, owner, callee string) bool {
		f, target := functions[owner], functions[callee]
		return ex(e, "call", owner, callee+"(a)") && e.Type == "i32" && e.Callee == target.ID && e.Signature != nil && e.Signature.DeclarationRange == target.DeclarationRange && e.Arguments != nil && len(*e.Arguments) == 1 && (*e.Arguments)[0].Binding == f.Bindings[0].ID
	}
	for _, row := range []struct{ name, value string }{{"left", "11"}, {"right", "22"}} {
		f := functions[row.name]
		e := f.Body[0].Value
		if f.Body[0].Kind != "expression" || !ex(e, "elementAssignment", row.name, "a[1]="+row.value) || e.Operator != "=" || e.Target == nil || e.Target.Receiver == nil || e.Target.Receiver.Binding != f.Parameters[0].ID || e.Target.Index == nil || e.Target.Index.Decimal != "1" || e.Value == nil || e.Value.Decimal != row.value {
			return fail("ordered writer")
		}
	}
	leftResult := functions["left"].Body[1].Value
	if !ex(leftResult, "as", "left", "a[0] as i32") || leftResult.Type != "i32" || leftResult.Expression == nil || leftResult.Expression.Kind != "checkedElementAccess" || leftResult.Expression.Type != "union<i32,undefined>" {
		return fail("left actual-value transport")
	}
	if functions["right"].Body[1].Kind != "return" || functions["right"].Body[1].Value == nil || functions["right"].Body[1].Value.Decimal != "1" {
		return fail("right numeric return")
	}
	ordered := functions["ordered"].Body[1].Value
	if !ex(ordered, "binary", "ordered", "left(a)+right(a)") || ordered.Operator != "+" || !call(ordered.Left, "ordered", "left") || !call(ordered.Right, "ordered", "right") {
		return fail("ordered selected operands")
	}
	rhs := functions["rhsFails"].Body[1].Value
	if !ex(rhs, "binary", "rhsFails", "left(a)+(1/0)") || rhs.Operator != "+" || !call(rhs.Left, "rhsFails", "left") || !ex(rhs.Right, "parentheses", "rhsFails", "(1/0)") {
		return fail("right failure expression")
	}
	divide := rhs.Right.Expression
	if !ex(divide, "binary", "rhsFails", "1/0") || divide.Operator != "/" || divide.Left == nil || divide.Right == nil || divide.Left.Decimal != "1" || divide.Right.Decimal != "0" {
		return fail("original RHS division")
	}
	for _, name := range []string{"plus", "plusValue"} {
		f := functions[name]
		value := f.Body[1].Value
		if f.Body[1].Kind != "return" || !ex(value, "unary", name, "+(a[0] as i32)") || value.Type != "i32" || value.Operator != "+" || value.OperatorRange == nil || value.OperatorRange.Start != value.Range.Start || value.OperatorRange.End != value.Range.Start+1 || !ex(value.Expression, "parentheses", name, "(a[0] as i32)") {
			return fail("original unary plus")
		}
		cast := value.Expression.Expression
		if !ex(cast, "as", name, "a[0] as i32") || cast.Type != "i32" || cast.Expression == nil || cast.Expression.Kind != "checkedElementAccess" || cast.Expression.Type != "union<i32,undefined>" || cast.Expression.Receiver == nil || cast.Expression.Receiver.Binding != f.Bindings[0].ID {
			return fail("unary actual-value transport")
		}
		array := f.Body[0].Initializer
		if array == nil || array.Kind != "arrayLiteral" || array.Elements == nil || len(*array.Elements) != 1 {
			return fail("unary initialized Array")
		}
		if name == "plus" && (*array.Elements)[0].Kind != "undefinedLiteral" || name == "plusValue" && (*array.Elements)[0].Decimal != "7" {
			return fail("unary original element")
		}
	}
	direct := functions["direct"]
	array, cast, operation := direct.Body[0].Initializer, direct.Body[1].Initializer, direct.Body[2].Initializer
	if array == nil || array.Elements == nil || len(*array.Elements) != 1 || (*array.Elements)[0].Kind != "undefinedLiteral" || !ex(cast, "as", "direct", "a[0] as i32") || cast.Expression == nil || cast.Expression.Kind != "checkedElementAccess" || cast.Expression.Type != "union<i32,undefined>" || direct.Body[1].Binding != direct.Bindings[1].ID {
		return fail("initialized direct U transport")
	}
	if direct.Body[2].Kind != "constDeclaration" || direct.Body[2].Binding != direct.Bindings[2].ID || !ex(operation, "binary", "direct", "x+1") || operation.Operator != "+" || operation.Left == nil || operation.Left.Binding != direct.Bindings[1].ID || operation.Right == nil || operation.Right.Decimal != "1" || direct.Body[3].Kind != "return" || direct.Body[3].Value == nil || direct.Body[3].Value.Decimal != "7" {
		return fail("actual operation before later return")
	}
	caller := functions["caller"].Body[0].Value
	if !ex(caller, "binary", "caller", "direct()+1") || caller.Operator != "+" || !ex(caller.Left, "call", "caller", "direct()") || caller.Left.Callee != direct.ID || caller.Left.Arguments == nil || len(*caller.Left.Arguments) != 0 || caller.Left.Signature == nil || caller.Left.Signature.DeclarationRange != direct.DeclarationRange || caller.Right == nil || caller.Right.Decimal != "1" {
		return fail("real direct-call continuation")
	}
	return nil
}

func TestArrayGenericV8NumericUseOperandOrder(t *testing.T) {
	raw, err := os.ReadFile("testdata/array-facts-v8/numeric-use-order.ts")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	at := numericOrderOriginal(t, source)
	input := genericArrayRequest25f(t, source)
	var out bytes.Buffer
	err = runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(input), &out)
	genericArrayCapture25f(t, input, source, out.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	var packet arrayGenericPackage
	if err := json.Unmarshal(out.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	if packet.Family != "tsfg.array-facts-package-candidate" || packet.Version != 8 {
		t.Fatal("private v8 package identity")
	}
	if err := numericOrderContract(&packet.Payload, source, at); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{"unary-operator", "left-callee", "right-callee", "operand-order", "rhs-operator", "writer-value", "direct-operand", "direct-call"} {
		t.Run(mutation, func(t *testing.T) {
			var copy arrayGenericPackage
			if err := json.Unmarshal(out.Bytes(), &copy); err != nil {
				t.Fatal(err)
			}
			f := map[string]*arrayGenericFunction{}
			for i := range copy.Payload.Functions {
				v := &copy.Payload.Functions[i]
				f[v.Name] = v
			}
			switch mutation {
			case "unary-operator":
				f["plus"].Body[1].Value.Operator = "-"
			case "left-callee":
				f["ordered"].Body[1].Value.Left.Callee = f["right"].ID
			case "right-callee":
				f["ordered"].Body[1].Value.Right.Callee = f["left"].ID
			case "operand-order":
				v := f["ordered"].Body[1].Value
				v.Left, v.Right = v.Right, v.Left
			case "rhs-operator":
				f["rhsFails"].Body[1].Value.Right.Expression.Operator = "%"
			case "writer-value":
				f["right"].Body[0].Value.Value.Decimal = "11"
			case "direct-operand":
				f["direct"].Body[2].Initializer.Left.Binding = f["direct"].Bindings[2].ID
			case "direct-call":
				f["caller"].Body[0].Value.Left.Callee = f["plusValue"].ID
			}
			if numericOrderContract(&copy.Payload, source, at) == nil {
				t.Fatal("copied factual corruption accepted")
			}
		})
	}
}
