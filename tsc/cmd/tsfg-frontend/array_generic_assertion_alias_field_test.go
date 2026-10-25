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

// This oracle reads the original Program, not the projected DTO or a consumer
// outcome. The constructor parameter-property is a real declaration: its empty
// source body must not acquire a synthetic assignment statement.
func assertionAliasFieldOriginal(t *testing.T, source string) map[string]span {
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
	if len(file.Statements.Nodes) != 2 {
		t.Fatal("original complete class/function closure changed")
	}
	h, fn := file.Statements.Nodes[0], file.Statements.Nodes[1]
	if h.Kind != ast.KindClassDeclaration || h.Name().Text() != "H" || len(h.Members()) != 1 || fn.Kind != ast.KindFunctionDeclaration || fn.Name().Text() != "f" {
		t.Fatal("original H/f declarations changed")
	}
	ctor := h.Members()[0]
	if ctor.Kind != ast.KindConstructor || len(ctor.AsConstructorDeclaration().Parameters.Nodes) != 1 || len(ctor.Body().AsBlock().Statements.Nodes) != 0 {
		t.Fatal("original explicit constructor shape changed")
	}
	field := ctor.AsConstructorDeclaration().Parameters.Nodes[0]
	parameters := fn.AsFunctionDeclaration().Parameters.Nodes
	body := fn.Body().AsBlock().Statements.Nodes
	if field.Name().Text() != "value" || field.ModifierFlags()&ast.ModifierFlagsPublic == 0 || len(parameters) != 2 || len(body) != 4 {
		t.Fatal("actual parameter-property or f body changed")
	}
	decl := func(index int) *ast.Node {
		t.Helper()
		if body[index].Kind != ast.KindVariableStatement {
			t.Fatal("original local declaration missing", index)
		}
		list := body[index].AsVariableStatement().DeclarationList.AsVariableDeclarationList()
		if len(list.Declarations.Nodes) != 1 || body[index].AsVariableStatement().DeclarationList.Flags&ast.NodeFlagsConst == 0 {
			t.Fatal("one immutable original local required", index)
		}
		return list.Declarations.Nodes[0]
	}
	x, y, localH := decl(0), decl(1), decl(2)
	cast := x.AsVariableDeclaration().Initializer
	if cast.Kind != ast.KindAsExpression || body[3].Kind != ast.KindReturnStatement {
		t.Fatal("original as/return expression missing")
	}
	assertion := cast.AsAsExpression()
	access := assertion.Expression
	if access.Kind != ast.KindElementAccessExpression {
		t.Fatal("as must retain its actual array access operand")
	}
	read := access.AsElementAccessExpression()
	created := localH.AsVariableDeclaration().Initializer
	returned := body[3].AsReturnStatement().Expression
	is := func(n, declaration *ast.Node) bool {
		s := check.GetSymbolAtLocation(n)
		return s != nil && len(s.Declarations) == 1 && s.Declarations[0] == declaration
	}
	if x.Name().Text() != "x" || y.Name().Text() != "y" || localH.Name().Text() != "h" ||
		!is(read.Expression, parameters[0]) || !is(read.ArgumentExpression, parameters[1]) ||
		!is(y.AsVariableDeclaration().Initializer, x) || created.Kind != ast.KindNewExpression || len(created.AsNewExpression().Arguments.Nodes) != 1 ||
		!is(created.AsNewExpression().Arguments.Nodes[0], y) || check.GetResolvedSignature(created).Declaration() != ctor ||
		returned.Kind != ast.KindPropertyAccessExpression || !is(returned.AsPropertyAccessExpression().Expression, localH) || !is(returned.Name(), field) {
		t.Fatal("original operand, two aliases, constructor actual or returned field symbol changed")
	}
	union := check.GetTypeAtLocation(access)
	if union.Flags()&checker.TypeFlagsUnion == 0 || len(union.Types()) != 2 {
		t.Fatal("actual access must keep its i32|undefined static type")
	}
	seenNumber, seenUndefined := false, false
	for _, member := range union.Types() {
		seenNumber = seenNumber || member == check.GetNumberType()
		seenUndefined = seenUndefined || member == check.GetUndefinedType()
	}
	if !seenNumber || !seenUndefined || check.GetTypeAtLocation(cast) != check.GetUndefinedType() || check.GetTypeAtLocation(assertion.Type) != check.GetUndefinedType() ||
		check.GetTypeAtLocation(x.Name()) != check.GetUndefinedType() || check.GetTypeAtLocation(y.Name()) != check.GetUndefinedType() ||
		check.GetTypeAtLocation(field.Name()) != check.GetUndefinedType() || check.GetTypeAtLocation(returned) != check.GetUndefinedType() ||
		check.GetTypeAtLocation(created) != check.GetDeclaredTypeOfSymbol(check.GetSymbolAtLocation(h.Name())) {
		t.Fatal("original assertion target, alias, field or constructed static type changed")
	}
	at := map[string]span{}
	for key, n := range map[string]*ast.Node{"H": h, "ctor": ctor, "field": field, "field.name": field.Name(), "field.type": field.AsParameterDeclaration().Type,
		"f": fn, "a": parameters[0], "i": parameters[1], "a.type": parameters[0].AsParameterDeclaration().Type, "i.type": parameters[1].AsParameterDeclaration().Type,
		"x": x, "y": y, "h": localH, "as": cast, "target": assertion.Type, "access": access, "array": read.Expression, "index": read.ArgumentExpression,
		"alias": y.AsVariableDeclaration().Initializer, "new": created, "argument": created.AsNewExpression().Arguments.Nodes[0], "return": body[3], "read": returned,
		"receiver": returned.AsPropertyAccessExpression().Expression, "read.name": returned.Name(), "return.type": fn.AsFunctionDeclaration().Type} {
		at[key] = span{file.GetPositionMap().UTF8ToUTF16(scanner.SkipTrivia(source, n.Pos())), file.GetPositionMap().UTF8ToUTF16(n.End())}
	}
	start := scanner.SkipTrivia(source, access.End())
	if source[start:start+2] != "as" || x.End() >= y.Pos() || y.End() >= localH.Pos() || localH.End() >= returned.Pos() {
		t.Fatal("actual as token or source evaluation order changed")
	}
	at["operator"] = span{file.GetPositionMap().UTF8ToUTF16(start), file.GetPositionMap().UTF8ToUTF16(start + 2)}
	t.Log("independent official Program: four diagnostics0; broad operand/static undefined target, two aliases, actual constructor parameter-property and returned field, UTF16 verified")
	return at
}

func assertionAliasFieldContract(p *arrayGenericPayload, source string, at map[string]span) error {
	fail := func(s string) error { return fmt.Errorf("original assertion/alias/field facts differ: %s", s) }
	if p.SourceRoot != "/src" || p.RootFile != "/src/main.ts" || p.PositionUnit != "utf16" || len(p.Diagnostics) != 0 || len(p.Files) != 2 || len(p.Functions) != 1 || len(p.Exports) != 1 {
		return fail("complete original source closure")
	}
	found := false
	for _, file := range p.Files {
		if file.Path == "/src/main.ts" {
			found = file.Content == source && file.ByteLength == len(source) && file.UTF16Length == len(source) && file.SHA256 == arrayFactsSHA([]byte(source))
		}
	}
	if !found {
		return fail("complete pinned ASCII source and UTF16")
	}
	types := map[string]arrayGenericType{}
	var h arrayGenericType
	for _, typ := range p.Types {
		types[typ.ID] = typ
		if typ.Name == "H" {
			h = typ
		}
	}
	if h.Kind != "reference" || h.DeclarationRange == nil || *h.DeclarationRange != at["H"] || h.Fields == nil || len(*h.Fields) != 1 || h.Constructor == nil || h.Construction == nil {
		return fail("original class, field and explicit constructor")
	}
	field, ctor, construction := (*h.Fields)[0], h.Constructor, h.Construction
	if field.Name != "value" || field.Type != "undefined" || field.ParameterBinding == "" || field.Initializer != nil || field.Origin.Source != "/src/main.ts" || field.Origin.Start != at["field"].Start || field.Origin.End != at["field"].End ||
		ctor.OwnerType != h.ID || ctor.ReturnType != h.ID || ctor.DeclarationRange != at["ctor"] || len(ctor.Parameters) != 1 || len(ctor.Body) != 0 || ctor.Parameters[0].ID != field.ParameterBinding || ctor.Parameters[0].DeclaredType != "undefined" ||
		construction.Kind != "explicit" || construction.InitializationPhase != "before-body" || len(construction.InitializationFields) != 1 || construction.InitializationFields[0] != field.ID || len(construction.Parameters) != 1 || construction.Parameters[0].Symbol != field.ParameterBinding || construction.Parameters[0].Type != "undefined" {
		return fail("real parameter-property initialization joins field and constructor formal without fake body")
	}
	fn := &p.Functions[0]
	if fn.Name != "f" || fn.SourcePath != "/src/main.ts" || fn.DeclarationRange != at["f"] || fn.ReturnType != "undefined" || fn.ReturnAnnotationRange == nil || *fn.ReturnAnnotationRange != at["return.type"] || len(fn.Parameters) != 2 || len(fn.Bindings) != 3 || len(fn.Body) != 4 || p.Exports[0].FunctionID != fn.ID {
		return fail("complete f/export/signature/body")
	}
	a, i := fn.Parameters[0], fn.Parameters[1]
	x, y, localH := fn.Bindings[0], fn.Bindings[1], fn.Bindings[2]
	if a.Name != "a" || i.Name != "i" || a.ID == i.ID || a.DeclarationRange != at["a"] || i.DeclarationRange != at["i"] || a.DeclaredType != "array<union<i32,undefined>>" || i.DeclaredType != "u32" ||
		x.Name != "x" || y.Name != "y" || localH.Name != "h" || x.ID == y.ID || y.ID == localH.ID || x.ID == localH.ID || x.DeclaredType != "undefined" || y.DeclaredType != "undefined" || localH.DeclaredType != h.ID || x.Writable || y.Writable || localH.Writable ||
		x.DeclarationRange != at["x"] || y.DeclarationRange != at["y"] || localH.DeclarationRange != at["h"] {
		return fail("original distinct a/i/x/y/h bindings and declared types")
	}
	for index, binding := range []arrayFactsBinding{x, y, localH} {
		if fn.Body[index].Kind != "constDeclaration" || fn.Body[index].Binding != binding.ID || fn.Body[index].Range != binding.DeclarationRange || fn.Body[index].Initializer == nil {
			return fail("actual ordered const declarations")
		}
	}
	cast, alias, created := fn.Body[0].Initializer, fn.Body[1].Initializer, fn.Body[2].Initializer
	if cast.Kind != "as" || cast.Type != "undefined" || cast.FlowType != "undefined" || cast.Range != at["as"] || cast.AnnotationRange == nil || *cast.AnnotationRange != at["target"] || cast.OperatorRange == nil || *cast.OperatorRange != at["operator"] || cast.Expression == nil {
		return fail("actual assertion operand, static target and token anchors")
	}
	access := cast.Expression
	if access.Kind != "checkedElementAccess" || access.Type != "union<i32,undefined>" || access.FlowType != access.Type || access.Range != at["access"] || access.Receiver == nil || access.Index == nil ||
		access.Receiver.Kind != "bindingReference" || access.Receiver.Binding != a.ID || access.Receiver.Range != at["array"] || access.Index.Kind != "bindingReference" || access.Index.Binding != i.ID || access.Index.Range != at["index"] {
		return fail("broad actual array access and ordered array/index operands")
	}
	union, array := types[access.Type], types[a.DeclaredType]
	if union.Kind != "union" || len(union.Members) != 2 || union.Members[0] != "i32" || union.Members[1] != "undefined" || array.Kind != "array" || array.ElementType != union.ID {
		return fail("original union and array catalogue")
	}
	if alias.Kind != "bindingReference" || alias.Binding != x.ID || alias.Type != "undefined" || alias.FlowType != "undefined" || alias.Range != at["alias"] ||
		created.Kind != "new" || created.Callee != h.ID || created.Type != h.ID || created.FlowType != h.ID || created.Range != at["new"] || created.Signature == nil || created.Signature.ID != construction.Signature.ID || created.Arguments == nil || len(*created.Arguments) != 1 {
		return fail("second alias and selected real constructor")
	}
	argument := (*created.Arguments)[0]
	if argument.Kind != "bindingReference" || argument.Binding != y.ID || argument.Type != "undefined" || argument.FlowType != "undefined" || argument.Range != at["argument"] {
		return fail("constructor consumes actual second alias")
	}
	returned := fn.Body[3]
	read := returned.Value
	if returned.Kind != "return" || returned.Range != at["return"] || read == nil || read.Kind != "fieldAccess" || read.Property != field.ID || read.Type != "undefined" || read.FlowType != "undefined" || read.Range != at["read"] || read.NameRange == nil || *read.NameRange != at["read.name"] ||
		read.Receiver == nil || read.Receiver.Kind != "bindingReference" || read.Receiver.Binding != localH.ID || read.Receiver.Type != h.ID || read.Receiver.Range != at["receiver"] {
		return fail("actual returned H.value and its current receiver binding")
	}
	return nil
}

func TestArrayGenericV8AssertionAliasField(t *testing.T) {
	raw, err := os.ReadFile("testdata/array-facts-v8/assertion-alias-field.ts")
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 212 || arrayFactsSHA(raw) != "d6f038213a47b7ddee00213fda2dabbaa629cfc0ae30a001433d48b5a7b3488b" {
		t.Fatal("complete original frozen source changed")
	}
	source := string(raw)
	at := assertionAliasFieldOriginal(t, source)
	input := genericArrayRequest25f(t, source)
	var out bytes.Buffer
	if err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	genericArrayCapture25f(t, input, source, out.Bytes())
	var packet arrayGenericPackage
	if err := json.Unmarshal(out.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	if packet.Family != "tsfg.array-facts-package-candidate" || packet.Version != 8 || packet.RequestID == "" || packet.PackageID == "" {
		t.Fatal("public v8 package identity changed")
	}
	if err := assertionAliasFieldContract(&packet.Payload, source, at); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{"operand-kind", "operand-type", "assertion-target", "target-anchor", "array-binding", "index-binding", "second-alias", "constructor-argument", "selected-constructor", "parameter-property", "returned-field", "returned-receiver"} {
		t.Run(mutation, func(t *testing.T) {
			var copied arrayGenericPackage
			if err := json.Unmarshal(out.Bytes(), &copied); err != nil {
				t.Fatal(err)
			}
			fn := &copied.Payload.Functions[0]
			cast, created, read := fn.Body[0].Initializer, fn.Body[2].Initializer, fn.Body[3].Value
			switch mutation {
			case "operand-kind":
				cast.Expression.Kind = "undefinedLiteral"
			case "operand-type":
				cast.Expression.Type = "undefined"
			case "assertion-target":
				cast.Type = "i32"
			case "target-anchor":
				cast.AnnotationRange.Start++
			case "array-binding":
				cast.Expression.Receiver.Binding = fn.Parameters[1].ID
			case "index-binding":
				cast.Expression.Index.Binding = fn.Parameters[0].ID
			case "second-alias":
				fn.Body[1].Initializer.Binding = fn.Parameters[0].ID
			case "constructor-argument":
				(*created.Arguments)[0].Binding = fn.Bindings[0].ID
			case "selected-constructor":
				created.Signature.ID = fn.ID
			case "parameter-property":
				for index := range copied.Payload.Types {
					if copied.Payload.Types[index].Name == "H" {
						(*copied.Payload.Types[index].Fields)[0].ParameterBinding = fn.Bindings[0].ID
					}
				}
			case "returned-field":
				read.Property = created.Callee
			case "returned-receiver":
				read.Receiver.Binding = fn.Bindings[0].ID
			}
			if assertionAliasFieldContract(&copied.Payload, source, at) == nil {
				t.Fatal("copied DTO mutation accepted")
			}
		})
	}
}
