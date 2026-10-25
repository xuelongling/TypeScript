// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/scanner"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

// This independent official Program establishes syntax, bindings and types.
// A dynamic checked source read is not a successful bounds or lifetime proof.
func localDynamicSpanOriginal(t *testing.T, source string, dynamicSlice bool) map[string]span {
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
	if len(file.Statements.Nodes) != 1 || file.Statements.Nodes[0].Kind != ast.KindFunctionDeclaration {
		t.Fatal("complete original function table")
	}
	fn := file.Statements.Nodes[0]
	text := func(n *ast.Node) string { return source[scanner.SkipTrivia(source, n.Pos()):n.End()] }
	if fn.Name().Text() != "run" || fn.ModifierFlags()&ast.ModifierFlagsExport == 0 || text(fn.AsFunctionDeclaration().Type) != "i32" {
		t.Fatal("original exported run signature")
	}
	at := map[string]span{}
	record := func(key string, n *ast.Node) {
		t.Helper()
		if n == nil {
			t.Fatal("missing original syntax", key)
		}
		originalFile := ast.GetSourceFileOfNode(n)
		at[key] = span{originalFile.GetPositionMap().UTF8ToUTF16(scanner.SkipTrivia(originalFile.Text(), n.Pos())), originalFile.GetPositionMap().UTF8ToUTF16(n.End())}
	}
	record("function", fn)
	record("name", fn.Name())
	record("return-type", fn.AsFunctionDeclaration().Type)
	parameters := []string{"i"}
	if dynamicSlice {
		parameters = []string{"start", "count", "i"}
	}
	declarations := map[string]*ast.Node{}
	if len(fn.AsFunctionDeclaration().Parameters.Nodes) != len(parameters) {
		t.Fatal("original parameter count")
	}
	for i, p := range fn.AsFunctionDeclaration().Parameters.Nodes {
		if p.Name().Text() != parameters[i] || text(p.AsParameterDeclaration().Type) != "u32" {
			t.Fatal("original dynamic u32 parameter")
		}
		declarations[p.Name().Text()] = p
		record("binding."+p.Name().Text(), p)
	}
	body := fn.Body().AsBlock().Statements.Nodes
	if len(body) != 3 || body[0].Kind != ast.KindVariableStatement || body[1].Kind != ast.KindVariableStatement || body[2].Kind != ast.KindReturnStatement {
		t.Fatal("original local array, span, return order")
	}
	for i, name := range []string{"a", "s"} {
		list := body[i].AsVariableStatement().DeclarationList.AsVariableDeclarationList()
		if len(list.Declarations.Nodes) != 1 || list.Declarations.Nodes[0].Name().Text() != name {
			t.Fatal("original local declaration")
		}
		decl := list.Declarations.Nodes[0]
		want := "i32[]"
		if name == "s" {
			want = "ReadSpan<i32>"
		}
		if text(decl.AsVariableDeclaration().Type) != want {
			t.Fatal("original local annotation")
		}
		declarations[name] = decl
		record("binding."+name, decl)
	}
	array := declarations["a"].AsVariableDeclaration().Initializer
	call := declarations["s"].AsVariableDeclaration().Initializer
	read := body[2].AsReturnStatement().Expression
	if array.Kind != ast.KindArrayLiteralExpression || len(array.AsArrayLiteralExpression().Elements.Nodes) != 2 || call.Kind != ast.KindCallExpression || read.Kind != ast.KindElementAccessExpression {
		t.Fatal("original array, intrinsic and element access AST")
	}
	is := func(n, declaration *ast.Node) bool {
		symbol := check.GetSymbolAtLocation(n)
		return symbol != nil && len(symbol.Declarations) == 1 && symbol.Declarations[0] == declaration
	}
	args := call.AsCallExpression().Arguments.Nodes
	signature := check.GetResolvedSignature(call)
	if text(call.AsCallExpression().Expression) != "readSpan" || len(args) != 3 || signature == nil || signature.Declaration() == nil || signature.Declaration().Name().Text() != "readSpan" || ast.GetSourceFileOfNode(signature.Declaration()).FileName() != "/src/minimal-arrays-v8.d.ts" || !is(args[0], declarations["a"]) {
		t.Fatal("original intrinsic signature and local array symbol")
	}
	for i, element := range array.AsArrayLiteralExpression().Elements.Nodes {
		if element.Kind != ast.KindNumericLiteral || text(element) != fmt.Sprint(i+1) || check.GetTypeAtLocation(element).Flags()&checker.TypeFlagsNumberLike == 0 {
			t.Fatal("original initialized local array cells")
		}
		record(fmt.Sprint("element.", i), element)
	}
	for i, key := range []string{"start", "count"} {
		arg := args[i+1]
		if dynamicSlice {
			if arg.Kind != ast.KindIdentifier || !is(arg, declarations[key]) {
				t.Fatal("original dynamic slice symbol", key)
			}
		} else if arg.Kind != ast.KindNumericLiteral || text(arg) != []string{"0", "2"}[i] {
			t.Fatal("original fixed slice neighbor")
		}
		record("argument."+key, arg)
	}
	access := read.AsElementAccessExpression()
	if !is(access.Expression, declarations["s"]) || !is(access.ArgumentExpression, declarations["i"]) || check.GetTypeAtLocation(read).Flags()&checker.TypeFlagsNumberLike == 0 || check.GetTypeAtLocation(call).Flags()&checker.TypeFlagsObject == 0 {
		t.Fatal("original span receiver, dynamic index and checked types")
	}
	for key, n := range map[string]*ast.Node{"array": array, "call": call, "callee": call.AsCallExpression().Expression, "argument.a": args[0], "read": read, "receiver": access.Expression, "index": access.ArgumentExpression, "return": body[2], "intrinsic": signature.Declaration()} {
		record(key, n)
	}
	t.Log("fresh independent official Program: four diagnostic groups0; local initialized Array cells, intrinsic signature, u32 dynamic symbols and UTF16 anchors checked; no execution/bounds proof")
	return at
}

func localDynamicSpanContract(p *arrayGenericPayload, source string, dynamicSlice bool, at map[string]span) error {
	fail := func(what string) error { return fmt.Errorf("original local dynamic-span facts differ: %s", what) }
	if p.SourceRoot != "/src" || p.RootFile != "/src/main.ts" || p.PositionUnit != "utf16" || len(p.Files) != 2 || len(p.Functions) != 1 || len(p.Exports) != 1 || len(p.Diagnostics) != 0 {
		return fail("complete source closure")
	}
	found := false
	for _, file := range p.Files {
		if file.Path == "/src/main.ts" {
			found = file.Content == source && file.ByteLength == len(source) && file.UTF16Length == len(source) && file.SHA256 == arrayFactsSHA([]byte(source))
		}
	}
	if !found {
		return fail("original source bytes and UTF16")
	}
	f := &p.Functions[0]
	parameters := []string{"i"}
	if dynamicSlice {
		parameters = []string{"start", "count", "i"}
	}
	if f.Name != "run" || f.SourcePath != p.RootFile || f.DeclarationRange != at["function"] || f.NameRange != at["name"] || f.ReturnType != "i32" || f.ReturnAnnotationRange == nil || *f.ReturnAnnotationRange != at["return-type"] || len(f.Parameters) != len(parameters) || len(f.Bindings) != 2 || len(f.Body) != 3 || p.Exports[0].FunctionID != f.ID {
		return fail("original function/binding/statement table")
	}
	bindings := map[string]arrayFactsBinding{}
	for i, b := range append(append([]arrayFactsBinding{}, f.Parameters...), f.Bindings...) {
		name, typ := "", "u32"
		if i < len(parameters) {
			name = parameters[i]
		} else {
			name = []string{"a", "s"}[i-len(parameters)]
			typ = map[string]string{"a": "array<i32>", "s": "readSpan<i32>"}[name]
		}
		if b.ID == "" || b.Name != name || b.DeclaredType != typ || b.DeclarationRange != at["binding."+name] {
			return fail("original binding " + name)
		}
		bindings[name] = b
	}
	ex := func(e *arrayGenericExpression, kind, typ, key string) bool {
		return e != nil && e.Kind == kind && e.Type == typ && e.FlowType == typ && e.Range == at[key]
	}
	ref := func(e *arrayGenericExpression, name, key string) bool {
		return ex(e, "bindingReference", bindings[name].DeclaredType, key) && e.Binding == bindings[name].ID
	}
	for i, name := range []string{"a", "s"} {
		if f.Body[i].Kind != "constDeclaration" || f.Body[i].Binding != bindings[name].ID || f.Body[i].Range != at["binding."+name] || bindings[name].Writable {
			return fail("original local declaration order")
		}
	}
	array, call, read := f.Body[0].Initializer, f.Body[1].Initializer, f.Body[2].Value
	if !ex(array, "arrayLiteral", "array<i32>", "array") || array.Elements == nil || len(*array.Elements) != 2 {
		return fail("complete local array literal")
	}
	for i, element := range *array.Elements {
		if !ex(&element, "numericLiteral", "i32", fmt.Sprint("element.", i)) || element.Token != fmt.Sprint(i+1) || element.Decimal != fmt.Sprint(i+1) {
			return fail("actual initialized local cells")
		}
	}
	if !ex(call, "call", "readSpan<i32>", "call") || call.Callee != p.ReadSpanIntrinsic.ID || call.CalleeRange == nil || *call.CalleeRange != at["callee"] || call.Signature == nil || call.Signature.SourcePath != "/src/minimal-arrays-v8.d.ts" || call.Signature.DeclarationRange != at["intrinsic"] || call.Signature.ReturnType != "readSpan<i32>" || len(call.Signature.Parameters) != 3 || call.Signature.Parameters[0] != "array<i32>" || call.Signature.Parameters[1] != "u32" || call.Signature.Parameters[2] != "u32" || call.Arguments == nil || len(*call.Arguments) != 3 || !ref(&(*call.Arguments)[0], "a", "argument.a") {
		return fail("original intrinsic/array relation")
	}
	for i, name := range []string{"start", "count"} {
		arg := &(*call.Arguments)[i+1]
		if dynamicSlice {
			if !ref(arg, name, "argument."+name) {
				return fail("dynamic slice argument remains original binding")
			}
		} else if !ex(arg, "numericLiteral", "u32", "argument."+name) || arg.Token != []string{"0", "2"}[i] || arg.Decimal != arg.Token {
			return fail("fixed slice neighbor")
		}
	}
	if f.Body[2].Kind != "return" || f.Body[2].Range != at["return"] || !ex(read, "checkedElementAccess", "i32", "read") || !ref(read.Receiver, "s", "receiver") || !ref(read.Index, "i", "index") {
		return fail("original checked span read retains dynamic index")
	}
	return nil
}

func TestArrayGenericV8LocalDynamicSpanFacts(t *testing.T) {
	for _, row := range []struct {
		name, source string
		dynamicSlice bool
	}{
		{"dynamic-slice-and-index", "export function run(start:u32,count:u32,i:u32):i32{const a:i32[]=[1,2];const s:ReadSpan<i32>=readSpan(a,start,count);return s[i];}", true},
		{"fixed-slice-dynamic-index", "export function run(i:u32):i32{const a:i32[]=[1,2];const s:ReadSpan<i32>=readSpan(a,0,2);return s[i];}", false},
	} {
		t.Run(row.name, func(t *testing.T) {
			at := localDynamicSpanOriginal(t, row.source, row.dynamicSlice)
			request := genericArrayRequest25f(t, row.source)
			var out bytes.Buffer
			err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(request), &out)
			genericArrayCapture25f(t, request, row.source, out.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			var packet arrayGenericPackage
			if err := json.Unmarshal(out.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}
			if packet.Family != "tsfg.array-facts-package-candidate" || packet.Version != 8 || packet.RequestID == "" || packet.PackageID == "" {
				t.Fatal("public v8 package identity")
			}
			if err := localDynamicSpanContract(&packet.Payload, row.source, row.dynamicSlice, at); err != nil {
				t.Fatal(err)
			}
			for _, mutation := range []string{"array-cell", "span-owner", "slice-argument", "dynamic-index"} {
				t.Run(mutation, func(t *testing.T) {
					var copied arrayGenericPackage
					if err := json.Unmarshal(out.Bytes(), &copied); err != nil {
						t.Fatal(err)
					}
					f := &copied.Payload.Functions[0]
					switch mutation {
					case "array-cell":
						(*f.Body[0].Initializer.Elements)[1].Decimal = "9"
					case "span-owner":
						(*f.Body[1].Initializer.Arguments)[0].Binding = f.Bindings[1].ID
					case "slice-argument":
						arg := &(*f.Body[1].Initializer.Arguments)[1]
						if row.dynamicSlice {
							arg.Binding = f.Parameters[1].ID
						} else {
							arg.Decimal = "1"
						}
					case "dynamic-index":
						f.Body[2].Value.Index.Kind = "numericLiteral"
					}
					if localDynamicSpanContract(&copied.Payload, row.source, row.dynamicSlice, at) == nil {
						t.Fatal("copied factual corruption accepted")
					}
				})
			}
		})
	}
}
