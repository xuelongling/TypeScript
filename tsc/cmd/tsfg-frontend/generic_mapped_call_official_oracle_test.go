// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/json"
	"github.com/microsoft/typescript-go/internal/scanner"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

// This independent oracle never calls the owned exporter, decoder, mapper or
// projector. Its fixture was frozen from literal source and public formulas.
type mappedCallOracle14 struct {
	Source struct {
		LogicalPath string `json:"logicalPath"`
		SourceText  string `json:"sourceText"`
		SHA256      string `json:"sha256"`
		Bytes       int    `json:"bytes"`
		UTF16       int    `json:"utf16"`
		TrailingLF  bool   `json:"trailingLF"`
	} `json:"source"`
	Probe struct {
		LogicalPath string `json:"logicalPath"`
		Text        string `json:"text"`
	} `json:"probe"`
	Declaration struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		SHA256  string `json:"sha256"`
		Bytes   int    `json:"bytes"`
	} `json:"declaration"`
	Package  json.Value `json:"package"`
	Requests json.Value `json:"requests"`
	Result   json.Value `json:"result"`
}

func mappedCallHash14(text string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(text))) }

func mappedCallJSON14(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	result := json.Value(bytes.Clone(data))
	if err := result.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	return result
}

func mappedCallFixture14(t *testing.T) mappedCallOracle14 {
	t.Helper()
	data, err := os.ReadFile("testdata/generic-mapped-call/first-whole-oracle-v7.json")
	if err != nil {
		t.Fatal(err)
	}
	if mappedCallHash14(string(data)) != "5a780b34cadd55421ca418e80a7764e0359b53bedf180fee4d85be6d092d2c2f" {
		t.Fatal("independent mapped-call whole fixture changed")
	}
	var f mappedCallOracle14
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	if f.Source.LogicalPath != "/original/main.ts" || f.Source.Bytes != 243 || f.Source.UTF16 != 241 ||
		f.Source.TrailingLF || len(f.Source.SourceText) != f.Source.Bytes ||
		len(utf16.Encode([]rune(f.Source.SourceText))) != f.Source.UTF16 ||
		mappedCallHash14(f.Source.SourceText) != f.Source.SHA256 || len(f.Declaration.Content) != 362 ||
		f.Declaration.Bytes != 362 || mappedCallHash14(f.Declaration.Content) != f.Declaration.SHA256 {
		t.Fatal("literal source/declaration bytes or UTF16 identity changed")
	}
	return f
}

func mappedCallRange14(node *ast.Node) map[string]int {
	file := ast.GetSourceFileOfNode(node)
	return map[string]int{"start": file.GetPositionMap().UTF8ToUTF16(scanner.SkipTrivia(file.Text(), node.Pos())),
		"end": file.GetPositionMap().UTF8ToUTF16(node.End())}
}

func mappedCallSymbol14(t *testing.T, check *checker.Checker, node, declaration *ast.Node) *ast.Symbol {
	t.Helper()
	sym := check.GetSymbolAtLocation(node)
	if sym == nil || len(sym.Declarations) != 1 || sym.Declarations[0] != declaration {
		t.Fatalf("official symbol does not name the original single declaration: %s", node.Text())
	}
	return sym
}

// All four mapped binders and both independent key/index occurrences are
// checked before constructing an owned-looking manifest for whole comparison.
func mappedCallType14(t *testing.T, check *checker.Checker, node *ast.Node,
	outer *ast.Symbol, binders map[*ast.Symbol]bool) map[string]any {
	t.Helper()
	if node == nil || node.Kind != ast.KindIndexedAccessType {
		t.Fatal("missing complete indexed AST")
	}
	idx := node.AsIndexedAccessTypeNode()
	if idx.ObjectType.Kind != ast.KindMappedType {
		t.Fatal("indexed object is not the actual mapped AST")
	}
	m := idx.ObjectType.AsMappedTypeNode()
	if m.TypeParameter == nil || m.Type == nil || m.ReadonlyToken != nil || m.QuestionToken != nil ||
		m.NameType != nil || m.Members != nil && len(m.Members.Nodes) != 0 {
		t.Fatal("unexpected mapped member/modifier/remap slots")
	}
	p := m.TypeParameter.AsTypeParameterDeclaration()
	if p.Name().Text() != "K" || p.Constraint == nil || p.Expression != nil ||
		p.DefaultType != nil || m.TypeParameter.Modifiers() != nil {
		t.Fatal("mapped binder slots differ")
	}
	binder := mappedCallSymbol14(t, check, p.Name(), m.TypeParameter)
	if binder == outer || binders[binder] {
		t.Fatal("mapped binder aliases an outer or another original binder")
	}
	binders[binder] = true
	key := func(n *ast.Node) map[string]any {
		if n == nil || n.Kind != ast.KindLiteralType || n.AsLiteralTypeNode().Literal.Kind != ast.KindStringLiteral ||
			n.AsLiteralTypeNode().Literal.Text() != "value" {
			t.Fatal("original one-key syntax changed")
		}
		return map[string]any{"kind": "string-literal", "literals": []string{"value"}}
	}
	if p.Constraint == idx.IndexType {
		t.Fatal("key and index must be independent original nodes")
	}
	if m.Type.Kind != ast.KindTypeReference {
		t.Fatal("mapped value lost outer reference syntax")
	}
	ref := m.Type.AsTypeReferenceNode()
	if ref.TypeArguments != nil || ref.TypeName.Kind != ast.KindIdentifier ||
		check.GetSymbolAtLocation(ref.TypeName) != outer {
		t.Fatal("mapped value is not its owner's real outer parameter")
	}
	return map[string]any{"kind": "indexed", "parameter": 0, "indexed": map[string]any{
		"object": map[string]any{"kind": "mapped", "parameter": 0, "mapped": map[string]any{
			"binder": "K", "keys": key(p.Constraint), "value": map[string]any{"kind": "parameter", "parameter": 0}}},
		"index": key(idx.IndexType)}}
}

func TestOwnedGenericMappedCallOfficialPrior(t *testing.T) {
	mappedCallOfficialPrior14(t, mappedCallFixture14(t))
}

func mappedCallOfficialPrior14(t *testing.T, f mappedCallOracle14) {
	t.Helper()
	inputs := map[string]string{f.Source.LogicalPath: f.Source.SourceText,
		f.Declaration.Path: f.Declaration.Content, f.Probe.LogicalPath: f.Probe.Text}
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
			FileNames: []string{f.Source.LogicalPath, f.Declaration.Path, f.Probe.LogicalPath},
			CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue,
				Target: core.ScriptTargetESNext, Module: core.ModuleKindESNext}}},
		Host:           compiler.NewCompilerHost("/original", vfstest.FromMap(inputs, true), "/original", nil, nil),
		SingleThreaded: core.TSTrue})
	if len(program.GetSourceFiles()) != 3 {
		t.Fatal("official prior did not consume exactly main, DTS and probe")
	}
	diagnostics := map[string]any{"syntactic": []any{}, "semantic": []any{}, "program": []any{}, "global": []any{}}
	for _, file := range program.GetSourceFiles() {
		if text, ok := inputs[file.FileName()]; !ok || text != file.Text() {
			t.Fatal("official input substitution")
		}
		if ds := program.GetSyntacticDiagnostics(t.Context(), file); len(ds) != 0 {
			t.Fatalf("official syntax: %v", ds)
		}
		if ds := program.GetSemanticDiagnostics(t.Context(), file); len(ds) != 0 {
			t.Fatalf("official semantics: %v", ds)
		}
	}
	if ds := program.GetProgramDiagnostics(); len(ds) != 0 {
		t.Fatalf("official program: %v", ds)
	}
	if ds := program.GetGlobalDiagnostics(t.Context()); len(ds) != 0 {
		t.Fatalf("official globals: %v", ds)
	}
	main := program.GetSourceFile(f.Source.LogicalPath)
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), main)
	defer release()
	if check.WasCanceled() || len(main.Statements.Nodes) != 2 || main.IsDeclarationFile ||
		main.ExternalModuleIndicator == nil {
		t.Fatal("official original module differs")
	}
	var expected struct {
		Payload struct {
			Definitions []map[string]any `json:"definitions"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(f.Package, &expected); err != nil {
		t.Fatal(err)
	}
	outerSymbols, valueSymbols, names := []*ast.Symbol{}, []*ast.Symbol{}, []*ast.Symbol{}
	binders := map[*ast.Symbol]bool{}
	defs := []map[string]any{}
	for i, node := range main.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || ast.GetFunctionFlags(node) != ast.FunctionFlagsNormal ||
			node.Name() == nil || node.Body() == nil || node.Body().Kind != ast.KindBlock ||
			node.Modifiers() == nil || node.Modifiers().ModifierFlags != ast.ModifierFlagsExport {
			t.Fatal("official function declaration slots changed")
		}
		d := node.AsFunctionDeclaration()
		if d.TypeParameters == nil || len(d.TypeParameters.Nodes) != 1 || d.Type == nil ||
			d.FullSignature != nil || d.Parameters == nil || len(d.Parameters.Nodes) != 1 ||
			len(d.Body.AsBlock().Statements.Nodes) != 1 {
			t.Fatal("official complete signature/body slots changed")
		}
		tp, param := d.TypeParameters.Nodes[0], d.Parameters.Nodes[0]
		p, v := tp.AsTypeParameterDeclaration(), param.AsParameterDeclaration()
		if p.Constraint != nil || p.DefaultType != nil || p.Expression != nil || tp.Modifiers() != nil ||
			v.Name().Kind != ast.KindIdentifier || v.Type == nil || v.Initializer != nil || v.QuestionToken != nil ||
			v.DotDotDotToken != nil || param.Modifiers() != nil {
			t.Fatal("official required parameter slots changed")
		}
		outer := mappedCallSymbol14(t, check, tp.Name(), tp)
		value := mappedCallSymbol14(t, check, param.Name(), param)
		name := mappedCallSymbol14(t, check, node.Name(), node)
		outerSymbols, valueSymbols, names = append(outerSymbols, outer), append(valueSymbols, value), append(names, name)
		sig := check.GetSignatureFromDeclaration(node)
		pt, rt := check.GetTypeFromTypeNode(v.Type), check.GetTypeFromTypeNode(d.Type)
		ot := check.GetDeclaredTypeOfSymbol(outer)
		if sig == nil || sig.Declaration() != node || len(sig.TypeParameters()) != 1 || sig.TypeParameters()[0] != ot ||
			len(sig.Parameters()) != 1 || sig.Parameters()[0] != value || sig.MinArgumentCount() != 1 ||
			sig.ThisParameter() != nil || check.HasEffectiveRestParameter(sig) ||
			check.GetTypeOfSymbol(value) != pt || check.GetReturnTypeOfSignature(sig) != rt ||
			!check.IsTypeAssignableTo(pt, ot) || !check.IsTypeAssignableTo(ot, pt) ||
			!check.IsTypeAssignableTo(rt, ot) || !check.IsTypeAssignableTo(ot, rt) {
			t.Fatal("official signature/type relation differs")
		}
		paramType, retType := mappedCallType14(t, check, v.Type, outer, binders), mappedCallType14(t, check, d.Type, outer, binders)
		ret := d.Body.AsBlock().Statements.Nodes[0]
		if ret.Kind != ast.KindReturnStatement || ret.AsReturnStatement().Expression == nil {
			t.Fatal("original single return changed")
		}
		body := ret.AsReturnStatement().Expression
		expr := map[string]any{"arguments": []any{}, "definition": "", "kind": "parameter",
			"origin": mappedCallRange14(body), "parameter": 0, "typeArguments": []any{}}
		if i == 0 {
			if body.Kind != ast.KindIdentifier || check.GetSymbolAtLocation(body) != value {
				t.Fatal("leaf is not original parameter0")
			}
		} else {
			if body.Kind != ast.KindCallExpression {
				t.Fatal("route is not original direct call")
			}
			call := body.AsCallExpression()
			if call.Expression.Kind != ast.KindIdentifier || check.GetSymbolAtLocation(call.Expression) != names[0] ||
				call.QuestionDotToken != nil || call.TypeArguments == nil || len(call.TypeArguments.Nodes) != 1 ||
				call.Arguments == nil || len(call.Arguments.Nodes) != 1 {
				t.Fatal("original call target/type/value slots changed")
			}
			ta, arg := call.TypeArguments.Nodes[0], call.Arguments.Nodes[0]
			if ta.Kind != ast.KindTypeReference || ta.AsTypeReferenceNode().TypeArguments != nil ||
				check.GetSymbolAtLocation(ta.AsTypeReferenceNode().TypeName) != outer ||
				arg.Kind != ast.KindIdentifier || check.GetSymbolAtLocation(arg) != value {
				t.Fatal("call did not bind caller outer0/value0")
			}
			cs := check.GetResolvedSignature(body)
			if cs == nil || cs.Declaration() != main.Statements.Nodes[0] || len(cs.Parameters()) != 1 ||
				cs.MinArgumentCount() != 1 || cs.ThisParameter() != nil || check.HasEffectiveRestParameter(cs) ||
				!check.IsTypeAssignableTo(check.GetTypeOfSymbol(cs.Parameters()[0]), pt) ||
				!check.IsTypeAssignableTo(pt, check.GetTypeOfSymbol(cs.Parameters()[0])) ||
				!check.IsTypeAssignableTo(check.GetReturnTypeOfSignature(cs), rt) ||
				!check.IsTypeAssignableTo(rt, check.GetReturnTypeOfSignature(cs)) {
				t.Fatal("official one-hop signature differs")
			}
			expr["kind"], expr["definition"] = "call", defs[0]["key"]
			expr["typeArguments"] = []any{map[string]any{"kind": "parameter", "parameter": 0}}
			expr["arguments"] = []any{map[string]any{"kind": "parameter", "parameter": 0, "definition": "",
				"arguments": []any{}, "typeArguments": []any{}, "origin": mappedCallRange14(arg)}}
		}
		source := map[string]any{"path": f.Source.LogicalPath, "digest": f.Source.SHA256, "length": f.Source.UTF16,
			"positionUnit": "utf16", "surrogateStarts": []int{3}}
		key := mappedCallHash14("tsfg:generic-definition:v1\n" + string(mappedCallJSON14(t,
			map[string]any{"name": node.Name().Text(), "origin": mappedCallRange14(node), "scope": "package:mapped-call", "source": source})))
		defs = append(defs, map[string]any{"name": node.Name().Text(), "key": key, "origin": mappedCallRange14(node),
			"exported": true, "typeParameters": []string{tp.Name().Text()}, "returnType": retType,
			"parameters": []any{map[string]any{"name": param.Name().Text(), "type": paramType, "origin": mappedCallRange14(param)}}, "body": expr})
	}
	if outerSymbols[0] == outerSymbols[1] || valueSymbols[0] == valueSymbols[1] || names[0] == names[1] || len(binders) != 4 ||
		!bytes.Equal(mappedCallJSON14(t, defs), mappedCallJSON14(t, expected.Payload.Definitions)) {
		t.Fatalf("independent official complete original definition manifest differs: %s", mappedCallJSON14(t, defs))
	}
	probe := program.GetSourceFile(f.Probe.LogicalPath)
	var calls []*ast.Node
	var collect func(*ast.Node) bool
	collect = func(n *ast.Node) bool {
		if n.Kind == ast.KindCallExpression {
			calls = append(calls, n)
		}
		n.ForEachChild(collect)
		return false
	}
	probe.AsNode().ForEachChild(collect)
	if len(calls) != 4 {
		t.Fatal("official prior requires the exact four real explicit probe calls")
	}
	probes := []any{}
	wantCalls := []string{"leaf<boolean>(true)", "leaf<number>(1)", "route<boolean>(true)", "route<number>(1)"}
	for i, callNode := range calls {
		call := callNode.AsCallExpression()
		scalar, wantType := "bool", check.GetBooleanType()
		if i%2 == 1 {
			scalar, wantType = "f64", check.GetNumberType()
		}
		sig := check.GetResolvedSignature(callNode)
		sym := check.GetSymbolAtLocation(call.Expression)
		if sym == nil || sym.Flags&ast.SymbolFlagsAlias == 0 || check.GetAliasedSymbol(sym) == nil ||
			len(check.GetAliasedSymbol(sym).Declarations) != 1 || check.GetAliasedSymbol(sym).Declarations[0] != main.Statements.Nodes[i/2] ||
			sig == nil || sig.Declaration() != main.Statements.Nodes[i/2] || len(sig.Parameters()) != 1 ||
			sig.MinArgumentCount() != 1 || sig.ThisParameter() != nil || check.HasEffectiveRestParameter(sig) ||
			call.TypeArguments == nil || len(call.TypeArguments.Nodes) != 1 || call.Arguments == nil || len(call.Arguments.Nodes) != 1 ||
			check.GetTypeOfSymbol(sig.Parameters()[0]) != wantType || check.GetReturnTypeOfSignature(sig) != wantType ||
			check.GetTypeAtLocation(callNode) != wantType || check.GetTypeFromTypeNode(call.TypeArguments.Nodes[0]) != wantType ||
			!check.IsTypeAssignableTo(check.GetTypeAtLocation(call.Arguments.Nodes[0]), wantType) ||
			probe.Text()[scanner.SkipTrivia(probe.Text(), callNode.Pos()):callNode.End()] != wantCalls[i] {
			t.Fatalf("official probe %d differs from complete original instantiated signature", i)
		}
		probes = append(probes, map[string]any{"call": wantCalls[i], "parameterType": scalar, "returnType": scalar,
			"origin": mappedCallRange14(callNode), "target": mappedCallRange14(main.Statements.Nodes[i/2])})
	}
	// Preserve every official AST node and every identifier's declarations for
	// all three actual inputs, including DTS, without exporting compiler objects.
	inventory := []any{}
	for _, file := range program.GetSourceFiles() {
		var visit func(*ast.Node) bool
		visit = func(n *ast.Node) bool {
			record := map[string]any{"file": file.FileName(), "kind": n.Kind.String(), "pos": n.Pos(), "end": n.End(),
				"utf16":       map[string]int{"start": file.GetPositionMap().UTF8ToUTF16(n.Pos()), "end": file.GetPositionMap().UTF8ToUTF16(n.End())},
				"tokenOrigin": mappedCallRange14(n)}
			if n.Kind == ast.KindIdentifier {
				record["text"] = n.Text()
				if sym := check.GetSymbolAtLocation(n); sym != nil {
					declarations := []any{}
					for _, decl := range sym.Declarations {
						declarations = append(declarations, map[string]any{"file": ast.GetSourceFileOfNode(decl).FileName(),
							"kind": decl.Kind.String(), "origin": mappedCallRange14(decl)})
					}
					record["symbol"] = map[string]any{"name": sym.Name, "flags": uint64(sym.Flags), "declarations": declarations}
				}
			}
			inventory = append(inventory, record)
			n.ForEachChild(visit)
			return false
		}
		visit(file.AsNode())
	}
	if check.WasCanceled() {
		t.Fatal("official prior checker canceled")
	}
	record := mappedCallJSON14(t, map[string]any{"diagnostics": diagnostics, "definitions": defs, "probes": probes, "ast": inventory})
	t.Logf("TSFG_GENERIC_MAPPED_CALL_OFFICIAL_PRIOR_RECORD %s", base64.StdEncoding.EncodeToString(record))
	t.Log("TSFG_GENERIC_MAPPED_CALL_OFFICIAL_PRIOR_PASS")
}
