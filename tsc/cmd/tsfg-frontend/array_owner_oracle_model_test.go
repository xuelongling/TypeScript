// SPDX-License-Identifier: Apache-2.0
// Independent official-object packet construction; no producer projection is called.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	tsjson "github.com/microsoft/typescript-go/internal/json"
	"github.com/microsoft/typescript-go/internal/scanner"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

type ao23Map = map[string]any
type ao23Range struct{ Start, End int }

func (r ao23Range) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]int{"start": r.Start, "end": r.End})
}

type ao23Binding struct{ id, kind string }
type ao23Header struct {
	node       *ast.Node
	symbol     *ast.Symbol
	signature  *checker.Signature
	row        ao23Map
	parameters []ao23Map
	kinds      []string
}
type ao23Scope struct {
	t             *testing.T
	file, dts     *ast.SourceFile
	check         *checker.Checker
	array         *ast.Symbol
	scalars       map[string]*ast.Symbol
	index, length *ast.Node
	headers       map[*ast.Symbol]*ao23Header
	names         map[string]*ao23Header
	bindings      map[*ast.Symbol]ao23Binding
	counts        map[string]int
}

func ao23Want(t *testing.T, yes bool, detail string) {
	t.Helper()
	if !yes {
		t.Fatal(detail)
	}
}
func ao23Hash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func ao23JCS(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	out := tsjson.Value(raw)
	if err := out.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	return out
}
func ao23ID(t *testing.T, domain string, value any) string {
	return "sha256:" + ao23Hash(append([]byte(domain+"\x00"), ao23JCS(t, value)...))
}
func ao23Units(text string) int {
	n := 0
	for _, r := range text {
		if r > 65535 {
			n += 2
		} else {
			n++
		}
	}
	return n
}
func (s *ao23Scope) span(file *ast.SourceFile, node *ast.Node) ao23Range {
	s.t.Helper()
	ao23Want(s.t, node != nil && ast.GetSourceFileOfNode(node) == file, "original owned AST range")
	start, end := scanner.SkipTrivia(file.Text(), node.Pos()), node.End()
	ao23Want(s.t, start >= 0 && start < end && end <= len(file.Text()), "nonempty original AST token range")
	positions := file.GetPositionMap()
	r := ao23Range{positions.UTF8ToUTF16(start), positions.UTF8ToUTF16(end)}
	ao23Want(s.t, r.Start == ao23Units(file.Text()[:start]) && r.End == ao23Units(file.Text()[:end]), "official UTF16 map differs from independent rune count")
	s.counts["anchors"]++
	return r
}
func (s *ao23Scope) bindingID(file *ast.SourceFile, node *ast.Node) string {
	return ao23ID(s.t, "tsfg.array-facts.binding.v1", ao23Map{"sourcePath": file.FileName(), "nameRange": s.span(file, node)})
}
func (s *ao23Scope) arrayType(typ *checker.Type, empty bool) {
	ao23Want(s.t, typ != nil && s.check.IsArrayType(typ) && !checker.IsTupleType(typ) && typ.Target().Symbol() == s.array, "actual original pinned Array target")
	args := s.check.GetTypeArguments(typ)
	ao23Want(s.t, len(args) == 1 && (args[0] == s.check.GetNumberType() || empty && args[0].Flags() == checker.TypeFlagsNever), "actual Array Number element")
	info := s.check.GetIndexInfoOfType(typ, s.check.GetNumberType())
	ao23Want(s.t, info != nil && info.KeyType() == s.check.GetNumberType() && info.ValueType() == args[0] && !info.IsReadonly() && info.Declaration() == s.index, "actual original Array indexer")
}
func (s *ao23Scope) annotation(node *ast.Node) string {
	ao23Want(s.t, node != nil, "explicit original annotation")
	if node.Kind == ast.KindArrayType {
		s.annotationScalar(node.AsArrayTypeNode().ElementType, "f64")
		s.arrayType(s.check.GetTypeFromTypeNode(node), false)
		return "array.f64"
	}
	ao23Want(s.t, node.Kind == ast.KindTypeReference, "original scalar alias annotation")
	name := node.AsTypeReferenceNode().TypeName.Text()
	s.annotationScalar(node, name)
	return name
}
func (s *ao23Scope) annotationScalar(node *ast.Node, name string) {
	ao23Want(s.t, node != nil && node.Kind == ast.KindTypeReference && s.scalars[name] != nil, "pinned scalar TypeReference")
	ref := node.AsTypeReferenceNode()
	ao23Want(s.t, ref.TypeName.Kind == ast.KindIdentifier && ref.TypeName.Text() == name && ref.TypeArguments == nil && s.check.GetSymbolAtLocation(ref.TypeName) == s.scalars[name] && s.check.GetTypeFromTypeNode(node) == s.check.GetNumberType(), "actual pinned scalar declaration origin")
}
func (s *ao23Scope) pinnedTypes() ([]any, ao23Map) {
	nodes := s.dts.Statements.Nodes
	ao23Want(s.t, len(nodes) == 12, "complete pinned DTS")
	an, fn, un := nodes[9], nodes[10], nodes[11]
	ao23Want(s.t, an.Kind == ast.KindInterfaceDeclaration && an.Name().Text() == "Array" && fn.Kind == ast.KindTypeAliasDeclaration && fn.Name().Text() == "f64" && un.Kind == ast.KindTypeAliasDeclaration && un.Name().Text() == "u32", "pinned declarations")
	s.array = s.check.GetSymbolAtLocation(an.Name())
	s.scalars = map[string]*ast.Symbol{"f64": s.check.GetSymbolAtLocation(fn.Name()), "u32": s.check.GetSymbolAtLocation(un.Name())}
	for i, node := range []*ast.Node{an, fn, un} {
		sym := []*ast.Symbol{s.array, s.scalars["f64"], s.scalars["u32"]}[i]
		ao23Want(s.t, sym != nil && len(sym.Declarations) == 1 && sym.Declarations[0] == node, "unique original DTS Symbol")
	}
	for _, node := range []*ast.Node{fn, un} {
		ao23Want(s.t, s.check.GetTypeFromTypeNode(node.AsTypeAliasDeclaration().Type) == s.check.GetNumberType(), "pinned scalar Number representation")
	}
	members := an.AsInterfaceDeclaration().Members.Nodes
	ao23Want(s.t, len(members) == 2 && members[0].Kind == ast.KindIndexSignature && members[1].Kind == ast.KindPropertySignature, "complete Array members")
	s.index, s.length = members[0], members[1]
	types := []any{}
	for i, node := range []*ast.Node{fn, an, un} {
		row := ao23Map{"id": []string{"f64", "array.f64", "u32"}[i], "kind": "scalar", "sourcePath": s.dts.FileName(), "declarationRange": s.span(s.dts, node), "nameRange": s.span(s.dts, node.Name())}
		if i == 1 {
			row["kind"] = "array"
			row["elementType"] = "f64"
			row["indexSignatureRange"] = s.span(s.dts, s.index)
		}
		types = append(types, row)
	}
	lengthType := s.length.AsPropertySignatureDeclaration().Type
	ls := s.check.GetSymbolAtLocation(s.length.Name())
	ao23Want(s.t, ls != nil && len(ls.Declarations) == 1 && ls.Declarations[0] == s.length && s.check.GetTypeFromTypeNode(lengthType) == s.check.GetNumberType(), "original length Symbol/type")
	property := ao23Map{"id": s.bindingID(s.dts, s.length.Name()), "kind": "property", "name": "length", "ownerType": "array.f64", "officialType": "number", "writeType": "u32", "sourcePath": s.dts.FileName(), "declarationRange": s.span(s.dts, s.length), "nameRange": s.span(s.dts, s.length.Name()), "typeRange": s.span(s.dts, lengthType)}
	return types, property
}
func ao23Program(t *testing.T, source, dtsText string) (*ao23Scope, func()) {
	contents := map[string]string{"/array-owner23e/app.ts": source, "/array-owner23e/minimal-arrays-v2.d.ts": dtsText}
	paths := []string{"/array-owner23e/app.ts", "/array-owner23e/minimal-arrays-v2.d.ts"}
	program := compiler.NewProgram(compiler.ProgramOptions{Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{FileNames: paths, CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext}}}, Host: compiler.NewCompilerHost("/array-owner23e", vfstest.FromMap(contents, true), "/array-owner23e", nil, nil), SingleThreaded: core.TSTrue})
	for _, group := range []struct {
		name        string
		diagnostics []*ast.Diagnostic
	}{{"syntax", program.GetSyntacticDiagnostics(t.Context(), nil)}, {"semantic", program.GetSemanticDiagnostics(t.Context(), nil)}, {"program", program.GetProgramDiagnostics()}, {"global", program.GetGlobalDiagnostics(t.Context())}} {
		if len(group.diagnostics) != 0 {
			t.Fatalf("independent official %s diagnostics: %+v", group.name, group.diagnostics)
		}
	}
	consumed := []string{}
	for _, file := range program.GetSourceFiles() {
		ao23Want(t, contents[file.FileName()] == file.Text(), "complete original source text")
		consumed = append(consumed, file.FileName())
	}
	slices.Sort(consumed)
	ao23Want(t, slices.Equal(paths, consumed), "complete original Program closure")
	file, dts := program.GetSourceFile(paths[0]), program.GetSourceFile(paths[1])
	ao23Want(t, file != nil && dts != nil && ast.IsExternalModule(file), "whole source module")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	return &ao23Scope{t: t, file: file, dts: dts, check: check, headers: map[*ast.Symbol]*ao23Header{}, names: map[string]*ao23Header{}, counts: map[string]int{}}, release
}
func (s *ao23Scope) collectHeaders() []*ao23Header {
	out := []*ao23Header{}
	for _, node := range s.file.Statements.Nodes {
		ao23Want(s.t, node.Kind == ast.KindFunctionDeclaration && node.Name() != nil && node.Name().Kind == ast.KindIdentifier && (node.ModifierFlags() == 0 || node.ModifierFlags() == ast.ModifierFlagsExport), "original ordinary function")
		f := node.AsFunctionDeclaration()
		ao23Want(s.t, f.TypeParameters == nil && f.AsteriskToken == nil && f.FullSignature == nil && f.Parameters != nil && f.Body != nil && f.Body.Kind == ast.KindBlock, "required ordinary function body/signature")
		s.annotationScalar(f.Type, "f64")
		sym := s.check.GetSymbolAtLocation(node.Name())
		ao23Want(s.t, sym != nil && len(sym.Declarations) == 1 && sym.Declarations[0] == node && s.names[node.Name().Text()] == nil, "unique original function Symbol")
		sigs := s.check.GetSignaturesOfType(s.check.GetTypeOfSymbol(sym), checker.SignatureKindCall)
		ao23Want(s.t, len(sigs) == 1 && sigs[0].Declaration() == node && len(sigs[0].Parameters()) == len(f.Parameters.Nodes) && sigs[0].MinArgumentCount() == len(f.Parameters.Nodes) && len(sigs[0].TypeParameters()) == 0 && sigs[0].ThisParameter() == nil && !sigs[0].HasRestParameter() && s.check.GetReturnTypeOfSignature(sigs[0]) == s.check.GetNumberType(), "complete original resolved function signature")
		h := &ao23Header{node: node, symbol: sym, signature: sigs[0], parameters: []ao23Map{}, kinds: []string{}}
		for i, pn := range f.Parameters.Nodes {
			p := pn.AsParameterDeclaration()
			ao23Want(s.t, pn.Name() != nil && pn.Name().Kind == ast.KindIdentifier && pn.ModifierFlags() == 0 && p.Initializer == nil && p.QuestionToken == nil && p.DotDotDotToken == nil, "original required parameter")
			actual := s.check.GetSymbolAtLocation(pn.Name())
			ao23Want(s.t, actual != nil && actual == sigs[0].Parameters()[i] && len(actual.Declarations) == 1 && actual.Declarations[0] == pn, "actual signature parameter order")
			kind := s.annotation(p.Type)
			if kind == "array.f64" {
				s.arrayType(s.check.GetTypeOfSymbol(actual), false)
			} else {
				ao23Want(s.t, s.check.GetTypeOfSymbol(actual) == s.check.GetNumberType(), "official scalar parameter Number")
			}
			row := ao23Map{"id": s.bindingID(s.file, pn.Name()), "name": pn.Name().Text(), "kind": "parameter", "declaredType": kind, "writable": true, "declarationRange": s.span(s.file, pn), "nameRange": s.span(s.file, pn.Name()), "annotationRange": s.span(s.file, p.Type)}
			if kind == "array.f64" {
				row["elementAnnotationRange"] = s.span(s.file, p.Type.AsArrayTypeNode().ElementType)
			}
			h.parameters = append(h.parameters, row)
			h.kinds = append(h.kinds, kind)
			s.counts["parameters"]++
		}
		h.row = ao23Map{"id": s.bindingID(s.file, node.Name()), "name": node.Name().Text(), "sourcePath": s.file.FileName(), "declarationRange": s.span(s.file, node), "nameRange": s.span(s.file, node.Name()), "returnType": "f64", "returnAnnotationRange": s.span(s.file, f.Type), "parameters": h.parameters, "bindings": []any{}, "body": []any{}}
		s.headers[sym] = h
		s.names[node.Name().Text()] = h
		out = append(out, h)
	}
	s.counts["functions"] = len(out)
	return out
}
func (s *ao23Scope) actualExports(headers []*ao23Header) []any {
	actual := s.check.GetExportsOfModule(s.file.AsNode().Symbol())
	out := []any{}
	for _, h := range headers {
		count := 0
		for _, e := range actual {
			if e != nil && e.Name == h.node.Name().Text() && (e == h.symbol || h.symbol.ExportSymbol == e) {
				count++
			}
		}
		if h.node.ModifierFlags() == ast.ModifierFlagsExport {
			ao23Want(s.t, count == 1, "actual exported declaration")
			out = append(out, ao23Map{"name": h.node.Name().Text(), "functionId": h.row["id"]})
		} else {
			ao23Want(s.t, count == 0, "private helper is not exported")
		}
	}
	ao23Want(s.t, len(out) == len(actual), "complete actual exports, no omitted binding")
	slices.SortFunc(out, func(a, b any) int {
		return stringsCompare23e(a.(ao23Map)["name"].(string), b.(ao23Map)["name"].(string))
	})
	return out
}
func stringsCompare23e(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
