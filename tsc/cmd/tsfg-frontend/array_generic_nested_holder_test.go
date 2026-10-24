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

func nestedHolderFunction25z(p *arrayGenericPayload, name string) *arrayGenericFunction {
	for i := range p.Functions {
		if p.Functions[i].Name == name {
			return &p.Functions[i]
		}
	}
	return nil
}

// This oracle owns separate compiler objects and derives ranges and symbol
// identities from the original AST, without calling the facts projector.
func nestedHolderOriginal25z(t *testing.T, source string, conflict bool) map[string]span {
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
	original := map[string]span{}
	record := func(key string, node *ast.Node) {
		t.Helper()
		if node == nil {
			t.Fatal("original node missing", key)
		}
		original[key] = span{file.GetPositionMap().UTF8ToUTF16(scanner.SkipTrivia(source, node.Pos())),
			file.GetPositionMap().UTF8ToUTF16(node.End())}
	}
	named := func(text string, kind ast.Kind) *ast.Node {
		t.Helper()
		node := nodes[text]
		if node == nil || node.Kind != kind {
			t.Fatal("original syntax missing", text)
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
	if h == nil || o == nil || first == nil || len(h.Members()) != 1 || len(o.Members()) != 1 {
		t.Fatal("two original classes and free callee required")
	}
	hCtor, oCtor := h.Members()[0], o.Members()[0]
	hParam, oParam := hCtor.AsConstructorDeclaration().Parameters.Nodes[0], oCtor.AsConstructorDeclaration().Parameters.Nodes[0]
	record("H", h)
	record("O", o)
	record("H.constructor", hCtor)
	record("O.constructor", oCtor)
	record("H.field", hParam)
	record("O.field", oParam)
	record("first", first)
	record("first.o", first.AsFunctionDeclaration().Parameters.Nodes[0])
	read := named("o.h.span[0]", ast.KindElementAccessExpression)
	spanRead := read.AsElementAccessExpression().Expression
	hRead := spanRead.AsPropertyAccessExpression().Expression
	oRead := hRead.AsPropertyAccessExpression().Expression
	if check.GetSymbolAtLocation(spanRead.Name()).Declarations[0] != hParam ||
		check.GetSymbolAtLocation(hRead.Name()).Declarations[0] != oParam ||
		check.GetSymbolAtLocation(oRead).Declarations[0] != first.AsFunctionDeclaration().Parameters.Nodes[0] ||
		check.GetTypeAtLocation(hRead) != check.GetDeclaredTypeOfSymbol(check.GetSymbolAtLocation(h.Name())) {
		t.Fatal("original nested property symbols, formal receiver or H type changed")
	}
	for key, node := range map[string]*ast.Node{"read": read, "span": spanRead, "h": hRead, "o": oRead,
		"index": read.AsElementAccessExpression().ArgumentExpression} {
		record(key, node)
	}
	callText := "first(new O(new H(readSpan(a, 0, 1))))"
	if conflict {
		callText = "first(new O(new H(readSpan(a, 0, 1))), a)"
	}
	call := named(callText, ast.KindCallExpression)
	newO := call.AsCallExpression().Arguments.Nodes[0]
	newH := newO.AsNewExpression().Arguments.Nodes[0]
	view := newH.AsNewExpression().Arguments.Nodes[0]
	if check.GetResolvedSignature(call).Declaration() != first ||
		check.GetResolvedSignature(newO).Declaration() != oCtor ||
		check.GetResolvedSignature(newH).Declaration() != hCtor {
		t.Fatal("actual selected free or constructor signature changed")
	}
	for key, node := range map[string]*ast.Node{"call": call, "newO": newO, "newH": newH, "view": view,
		"arrayActual": view.AsCallExpression().Arguments.Nodes[0], "start": view.AsCallExpression().Arguments.Nodes[1],
		"count": view.AsCallExpression().Arguments.Nodes[2]} {
		record(key, node)
	}
	if conflict {
		shrink := named("a.length = 0", ast.KindBinaryExpression)
		arrayParam := first.AsFunctionDeclaration().Parameters.Nodes[1]
		actual := call.AsCallExpression().Arguments.Nodes[1]
		if check.GetSymbolAtLocation(shrink.AsBinaryExpression().Left.AsPropertyAccessExpression().Expression).Declarations[0] != arrayParam ||
			check.GetSymbolAtLocation(actual) != check.GetSymbolAtLocation(view.AsCallExpression().Arguments.Nodes[0]) ||
			shrink.End() >= read.Pos() {
			t.Fatal("original callee resize, actual Array alias or source order changed")
		}
		record("resize", shrink)
		record("arrayCallActual", actual)
	}
	t.Log("independent official Program: syntax/semantic/program/global=0; original nested symbols, selected signatures and UTF16 anchors verified")
	return original
}

func nestedHolderContract25z(p *arrayGenericPayload, source string, original map[string]span, conflict bool) error {
	fail := func(what string) error { return fmt.Errorf("original nested Holder facts differ: %s", what) }
	if p.PositionUnit != "utf16" || len(p.Diagnostics) != 0 || len(p.Functions) != 2 || len(p.Exports) != 1 || len(p.Files) != 2 {
		return fail("complete source closure")
	}
	found := false
	for _, file := range p.Files {
		if file.Path == "/src/main.ts" {
			found = file.Content == source && file.SHA256 == arrayFactsSHA([]byte(source)) && file.ByteLength == len(source) && file.UTF16Length == len(source)
		}
	}
	if !found {
		return fail("exact original ASCII source bytes and UTF16 length")
	}
	types := map[string]arrayGenericType{}
	for _, typ := range p.Types {
		if typ.Name == "H" || typ.Name == "O" {
			types[typ.Name] = typ
		}
	}
	h, o := types["H"], types["O"]
	fields := map[string]arrayGenericField{}
	for _, typ := range []arrayGenericType{h, o} {
		if typ.ID == "" || typ.Kind != "reference" || typ.Base != "" || typ.Fields == nil || len(*typ.Fields) != 1 ||
			typ.DeclarationRange == nil || *typ.DeclarationRange != original[typ.Name] || typ.Constructor == nil ||
			typ.Constructor.DeclarationRange != original[typ.Name+".constructor"] || len(typ.Constructor.Parameters) != 1 ||
			typ.Construction == nil || typ.Construction.Kind != "explicit" || typ.Construction.InitializationPhase != "before-body" {
			return fail("nominal class and original constructor")
		}
		field := (*typ.Fields)[0]
		at := original[typ.Name+".field"]
		if field.Origin.Source != "/src/main.ts" || field.Origin.Start != at.Start || field.Origin.End != at.End ||
			field.ParameterBinding != typ.Constructor.Parameters[0].ID || field.ParameterBinding == "" || field.Readonly ||
			typ.Constructor.Parameters[0].Kind != "parameter" || typ.Constructor.Parameters[0].DeclarationRange != at {
			return fail("original parameter-property identity")
		}
		fields[typ.Name] = field
	}
	hField, oField := fields["H"], fields["O"]
	if h.ID == o.ID || hField.ID == oField.ID || hField.Name != "span" || hField.Type != "readSpan<i32>" ||
		oField.Name != "h" || oField.Type != h.ID {
		return fail("two distinct original field/type identities")
	}
	first, run := nestedHolderFunction25z(p, "first"), nestedHolderFunction25z(p, "run")
	count := 1
	if conflict {
		count = 2
	}
	if first == nil || run == nil || first.OwnerType != "" || len(first.Parameters) != count ||
		first.Parameters[0].Kind != "parameter" || first.Parameters[0].DeclaredType != o.ID ||
		first.Parameters[0].DeclarationRange != original["first.o"] || first.DeclarationRange != original["first"] ||
		first.ReturnType != "i32" || len(first.Body) != count || len(run.Body) != 2 || len(run.Parameters) != 0 ||
		p.Exports[0].Name != "run" || p.Exports[0].FunctionID != run.ID {
		return fail("original free callee, parameters and export")
	}
	read := first.Body[count-1].Value
	if first.Body[count-1].Kind != "return" || read == nil || read.Kind != "checkedElementAccess" || read.Type != "i32" ||
		read.Range != original["read"] || read.Index == nil || read.Index.Decimal != "0" || read.Index.Range != original["index"] ||
		read.Receiver == nil || read.Receiver.Kind != "fieldAccess" || read.Receiver.Property != hField.ID ||
		read.Receiver.Type != hField.Type || read.Receiver.Range != original["span"] ||
		read.Receiver.Receiver == nil || read.Receiver.Receiver.Kind != "fieldAccess" ||
		read.Receiver.Receiver.Property != oField.ID || read.Receiver.Receiver.Type != h.ID || read.Receiver.Receiver.Range != original["h"] ||
		read.Receiver.Receiver.Receiver == nil || read.Receiver.Receiver.Receiver.Binding != first.Parameters[0].ID ||
		read.Receiver.Receiver.Receiver.Type != o.ID || read.Receiver.Receiver.Receiver.Range != original["o"] {
		return fail("original two-level receiver chain")
	}
	local, call := run.Body[0], run.Body[1].Value
	if local.Binding == "" || local.Initializer == nil || local.Initializer.Kind != "arrayLiteral" || local.Initializer.Type != "array<i32>" ||
		local.Initializer.Elements == nil || len(*local.Initializer.Elements) != 1 || (*local.Initializer.Elements)[0].Decimal != "1" ||
		call == nil || call.Kind != "call" || call.Callee != first.ID || call.Range != original["call"] ||
		call.Signature == nil || call.Signature.DeclarationRange != first.DeclarationRange || call.Arguments == nil || len(*call.Arguments) != count {
		return fail("original Array creation and selected call")
	}
	newO := &(*call.Arguments)[0]
	if newO.Kind != "new" || newO.Callee != o.ID || newO.Type != o.ID || newO.Range != original["newO"] ||
		newO.Signature == nil || newO.Signature.ID != o.Construction.Signature.ID || newO.Arguments == nil || len(*newO.Arguments) != 1 {
		return fail("actual outer constructor and operand")
	}
	newH := &(*newO.Arguments)[0]
	if newH.Kind != "new" || newH.Callee != h.ID || newH.Type != h.ID || newH.Range != original["newH"] ||
		newH.Signature == nil || newH.Signature.ID != h.Construction.Signature.ID || newH.Arguments == nil || len(*newH.Arguments) != 1 {
		return fail("actual inner constructor and operand")
	}
	view := &(*newH.Arguments)[0]
	if view.Kind != "call" || view.Callee != p.ReadSpanIntrinsic.ID || view.Type != "readSpan<i32>" ||
		view.Range != original["view"] || view.Arguments == nil || len(*view.Arguments) != 3 ||
		(*view.Arguments)[0].Binding != local.Binding || (*view.Arguments)[0].Range != original["arrayActual"] ||
		(*view.Arguments)[1].Decimal != "0" || (*view.Arguments)[1].Range != original["start"] ||
		(*view.Arguments)[2].Decimal != "1" || (*view.Arguments)[2].Range != original["count"] {
		return fail("actual Array/view operands and occurrence ranges")
	}
	if conflict {
		resize := first.Body[0].Value
		if first.Parameters[1].DeclaredType != "array<i32>" || resize == nil || resize.Kind != "lengthAssignment" ||
			resize.Range != original["resize"] || resize.Range.End >= read.Range.Start || resize.Target == nil ||
			resize.Target.Kind != "lengthTarget" || resize.Target.Property != p.ArrayLength.ID || resize.Target.Receiver == nil ||
			resize.Target.Receiver.Binding != first.Parameters[1].ID || resize.Value == nil || resize.Value.Decimal != "0" ||
			(*call.Arguments)[1].Binding != local.Binding || (*call.Arguments)[1].Range != original["arrayCallActual"] {
			return fail("callee resize before read and same actual Array")
		}
	}
	return nil
}

func TestArrayGenericV8NestedHolder25z(t *testing.T) {
	for _, row := range []struct {
		name     string
		conflict bool
	}{{"nested-holder-positive", false}, {"nested-holder-conflict", true}} {
		t.Run(row.name, func(t *testing.T) {
			sourceBytes, err := os.ReadFile("testdata/array-facts-v8/" + row.name + ".ts")
			if err != nil {
				t.Fatal(err)
			}
			source := string(sourceBytes)
			original := nestedHolderOriginal25z(t, source, row.conflict)
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
				t.Fatal("original public v8 package identity changed")
			}
			if err := nestedHolderContract25z(&packet.Payload, source, original, row.conflict); err != nil {
				t.Fatal(err)
			}
			mutations := []struct {
				name string
				edit func(*arrayGenericPayload)
			}{
				{"outer-binding", func(p *arrayGenericPayload) {
					nestedHolderFunction25z(p, "first").Body[len(nestedHolderFunction25z(p, "first").Body)-1].Value.Receiver.Receiver.Receiver.Binding = nestedHolderFunction25z(p, "run").Body[0].Binding
				}},
				{"outer-field", func(p *arrayGenericPayload) {
					e := nestedHolderFunction25z(p, "first").Body[len(nestedHolderFunction25z(p, "first").Body)-1].Value
					e.Receiver.Receiver.Property = e.Receiver.Property
				}},
				{"inner-field", func(p *arrayGenericPayload) {
					e := nestedHolderFunction25z(p, "first").Body[len(nestedHolderFunction25z(p, "first").Body)-1].Value
					e.Receiver.Property = e.Receiver.Receiver.Property
				}},
				{"receiver-type", func(p *arrayGenericPayload) {
					f := nestedHolderFunction25z(p, "first")
					f.Body[len(f.Body)-1].Value.Receiver.Receiver.Type = f.Parameters[0].DeclaredType
				}},
				{"read-range", func(p *arrayGenericPayload) {
					f := nestedHolderFunction25z(p, "first")
					f.Body[len(f.Body)-1].Value.Range = f.Body[len(f.Body)-1].Value.Index.Range
				}},
				{"field-origin", func(p *arrayGenericPayload) {
					for i := range p.Types {
						if p.Types[i].Name == "H" {
							(*p.Types[i].Fields)[0].Origin.End--
						}
					}
				}},
				{"parameter-field-binding", func(p *arrayGenericPayload) {
					for i := range p.Types {
						if p.Types[i].Name == "H" {
							(*p.Types[i].Fields)[0].ParameterBinding = nestedHolderFunction25z(p, "run").Body[0].Binding
						}
					}
				}},
				{"constructor-operand", func(p *arrayGenericPayload) {
					outer := &(*nestedHolderFunction25z(p, "run").Body[1].Value.Arguments)[0]
					inner := (*outer.Arguments)[0]
					(*outer.Arguments)[0] = (*inner.Arguments)[0]
				}},
				{"constructor-signature", func(p *arrayGenericPayload) {
					outer := &(*nestedHolderFunction25z(p, "run").Body[1].Value.Arguments)[0]
					(*outer.Arguments)[0].Signature = outer.Signature
				}},
				{"view-array-binding", func(p *arrayGenericPayload) {
					outer := &(*nestedHolderFunction25z(p, "run").Body[1].Value.Arguments)[0]
					inner := &(*outer.Arguments)[0]
					(*(*inner.Arguments)[0].Arguments)[0].Binding = nestedHolderFunction25z(p, "first").Parameters[0].ID
				}},
			}
			if row.conflict {
				mutations = append(mutations,
					struct {
						name string
						edit func(*arrayGenericPayload)
					}{"resize-order", func(p *arrayGenericPayload) {
						f := nestedHolderFunction25z(p, "first")
						f.Body[0], f.Body[1] = f.Body[1], f.Body[0]
					}},
					struct {
						name string
						edit func(*arrayGenericPayload)
					}{"resize-binding", func(p *arrayGenericPayload) {
						nestedHolderFunction25z(p, "first").Body[0].Value.Target.Receiver.Binding = nestedHolderFunction25z(p, "run").Body[0].Binding
					}},
					struct {
						name string
						edit func(*arrayGenericPayload)
					}{"call-array-binding", func(p *arrayGenericPayload) {
						(*nestedHolderFunction25z(p, "run").Body[1].Value.Arguments)[1].Binding = nestedHolderFunction25z(p, "first").Parameters[1].ID
					}})
			}
			for _, mutation := range mutations {
				t.Run(mutation.name, func(t *testing.T) {
					var copied arrayGenericPackage
					if err := json.Unmarshal(out.Bytes(), &copied); err != nil {
						t.Fatal(err)
					}
					mutation.edit(&copied.Payload)
					if nestedHolderContract25z(&copied.Payload, source, original, row.conflict) == nil {
						t.Fatal("copied DTO corruption accepted")
					}
				})
			}
		})
	}
}
