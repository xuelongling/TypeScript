// SPDX-License-Identifier: Apache-2.0
// Fresh official-object proof only; no Source producer or Product module is called.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
	"slices"
	"testing"
)

type rs26Scope struct {
	*ao23Scope
	view, intrinsic                                *ast.Symbol
	viewNode, viewIndex, viewLength, intrinsicNode *ast.Node
	generic                                        *checker.Signature
	genericT                                       *checker.Type
	views                                          map[*ast.Symbol]string
	proof                                          ao23Map
}

func rs26Program(t *testing.T, source, dts string) (*rs26Scope, func()) {
	paths := []string{"/readspan26a/app.ts", "/readspan26a/minimal-arrays-v3.d.ts"}
	contents := map[string]string{paths[0]: source, paths[1]: dts}
	p := compiler.NewProgram(compiler.ProgramOptions{Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{FileNames: paths, CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext}}}, Host: compiler.NewCompilerHost("/readspan26a", vfstest.FromMap(contents, true), "/readspan26a", nil, nil), SingleThreaded: core.TSTrue})
	for _, group := range []struct {
		name  string
		items []*ast.Diagnostic
	}{{"syntax", p.GetSyntacticDiagnostics(t.Context(), nil)}, {"semantic", p.GetSemanticDiagnostics(t.Context(), nil)}, {"program", p.GetProgramDiagnostics()}, {"global", p.GetGlobalDiagnostics(t.Context())}} {
		if len(group.items) != 0 {
			t.Fatalf("fresh official %s diagnostics: %+v", group.name, group.items)
		}
	}
	consumed := []string{}
	for _, f := range p.GetSourceFiles() {
		ao23Want(t, contents[f.FileName()] == f.Text(), "complete original Source/DTS text")
		consumed = append(consumed, f.FileName())
	}
	slices.Sort(consumed)
	ao23Want(t, slices.Equal(paths, consumed), "complete two-file original closure")
	file, decls := p.GetSourceFile(paths[0]), p.GetSourceFile(paths[1])
	ao23Want(t, file != nil && decls != nil && ast.IsExternalModule(file), "complete original external module and pinned DTS")
	check, release := p.GetTypeCheckerForFileExclusive(t.Context(), file)
	base := &ao23Scope{t: t, file: file, dts: decls, check: check, headers: map[*ast.Symbol]*ao23Header{}, names: map[string]*ao23Header{}, counts: map[string]int{}}
	return &rs26Scope{ao23Scope: base, views: map[*ast.Symbol]string{}, proof: ao23Map{"diagnostics": ao23Map{"syntax": 0, "semantic": 0, "program": 0, "global": 0}, "consumedPaths": paths}}, release
}
func (s *rs26Scope) original(node *ast.Node) *ast.Symbol {
	sym := s.check.GetSymbolAtLocation(node.Name())
	ao23Want(s.t, sym != nil && len(sym.Declarations) == 1 && sym.Declarations[0] == node, "actual unique original declaration Symbol")
	return sym
}
func (s *rs26Scope) viewType(typ, element *checker.Type) {
	ao23Want(s.t, typ != nil && !s.check.IsArrayType(typ) && !checker.IsTupleType(typ) && typ.Target().Symbol() == s.view, "canonical original ReadSpan target distinct from Array")
	args := s.check.GetTypeArguments(typ)
	ao23Want(s.t, len(args) == 1 && args[0] == element, "actual exact instantiated ReadSpan element")
	info := s.check.GetIndexInfoOfType(typ, s.check.GetNumberType())
	ao23Want(s.t, info != nil && info.KeyType() == s.check.GetNumberType() && info.ValueType() == element && info.IsReadonly() && info.Declaration() == s.viewIndex, "actual readonly numeric indexer joins pinned original declaration")
	prop := s.check.GetPropertyOfType(typ, "length")
	ao23Want(s.t, prop != nil && len(prop.Declarations) == 1 && prop.Declarations[0] == s.viewLength && s.check.GetTypeOfSymbol(prop) == s.check.GetNumberType(), "actual readonly length property's original declaration and Number representation")
}
func (s *rs26Scope) arrayElement(typ, element *checker.Type) {
	ao23Want(s.t, typ != nil && s.check.IsArrayType(typ) && typ.Target().Symbol() == s.array, "canonical original generic Array target")
	args := s.check.GetTypeArguments(typ)
	ao23Want(s.t, len(args) == 1 && args[0] == element, "exact original generic Array element")
	info := s.check.GetIndexInfoOfType(typ, s.check.GetNumberType())
	ao23Want(s.t, info != nil && !info.IsReadonly() && info.ValueType() == element && info.Declaration() == s.index, "original generic Array indexer")
}
func (s *rs26Scope) origins() ([]any, ao23Map, ao23Map, ao23Map) {
	nodes := s.dts.Statements.Nodes
	ao23Want(s.t, len(nodes) == 14, "all fourteen original pinned declarations")
	an, fn, un, vn, in := nodes[9], nodes[10], nodes[11], nodes[12], nodes[13]
	ao23Want(s.t, an.Name().Text() == "Array" && fn.Name().Text() == "f64" && un.Name().Text() == "u32" && vn.Name().Text() == "ReadSpan" && in.Name().Text() == "readSpan", "complete ordered original DTS nominal names")
	s.array = s.original(an)
	s.scalars = map[string]*ast.Symbol{"f64": s.original(fn), "u32": s.original(un)}
	s.view = s.original(vn)
	s.intrinsic = s.original(in)
	s.viewNode, s.intrinsicNode = vn, in
	for _, n := range []*ast.Node{fn, un} {
		ao23Want(s.t, s.check.GetTypeFromTypeNode(n.AsTypeAliasDeclaration().Type) == s.check.GetNumberType(), "actual pinned number aliases")
	}
	am, vm := an.AsInterfaceDeclaration().Members.Nodes, vn.AsInterfaceDeclaration().Members.Nodes
	ao23Want(s.t, len(am) == 2 && len(vm) == 2 && am[0].Kind == ast.KindIndexSignature && vm[0].Kind == ast.KindIndexSignature && am[1].Kind == ast.KindPropertySignature && vm[1].Kind == ast.KindPropertySignature, "all original Array/ReadSpan index and length declarations")
	s.index, s.length, s.viewIndex, s.viewLength = am[0], am[1], vm[0], vm[1]
	ao23Want(s.t, s.viewIndex.ModifierFlags()&ast.ModifierFlagsReadonly != 0 && s.viewLength.ModifierFlags()&ast.ModifierFlagsReadonly != 0 && s.length.ModifierFlags()&ast.ModifierFlagsReadonly == 0, "actual readonly syntax differs from mutable Array")
	s.annotationScalar(s.viewLength.AsPropertySignatureDeclaration().Type, "u32")
	viewParams := vn.AsInterfaceDeclaration().TypeParameters.Nodes
	ao23Want(s.t, len(viewParams) == 1, "single original ReadSpan type parameter")
	vt := s.check.GetDeclaredTypeOfSymbol(s.original(viewParams[0]))
	ao23Want(s.t, s.check.GetTypeFromTypeNode(s.viewIndex.AsIndexSignatureDeclaration().Type) == vt, "readonly generic indexer uses original interface T")
	types := []any{}
	for i, n := range []*ast.Node{fn, an, un, vn} {
		row := ao23Map{"id": []string{"f64", "array.f64", "u32", "readspan.f64"}[i], "kind": "scalar", "sourcePath": s.dts.FileName(), "declarationRange": s.span(s.dts, n), "nameRange": s.span(s.dts, n.Name())}
		if i == 1 {
			row["kind"] = "array"
			row["elementType"] = "f64"
			row["indexSignatureRange"] = s.span(s.dts, s.index)
		}
		if i == 3 {
			row["kind"] = "readSpan"
			row["elementType"] = "f64"
			row["readonly"] = true
			row["indexSignatureRange"] = s.span(s.dts, s.viewIndex)
		}
		types = append(types, row)
	}
	property := func(n *ast.Node, view bool) ao23Map {
		p := n.AsPropertySignatureDeclaration()
		sym := s.original(n)
		ao23Want(s.t, n.Name().Text() == "length" && p.PostfixToken == nil && s.check.GetTypeFromTypeNode(p.Type) == s.check.GetNumberType() && s.check.GetTypeOfSymbol(sym) == s.check.GetNumberType(), "original complete length property's type")
		row := ao23Map{"id": s.bindingID(s.dts, n.Name()), "kind": "property", "name": "length", "ownerType": "array.f64", "officialType": "number", "writeType": "u32", "sourcePath": s.dts.FileName(), "declarationRange": s.span(s.dts, n), "nameRange": s.span(s.dts, n.Name()), "typeRange": s.span(s.dts, p.Type)}
		if view {
			row["ownerType"] = "readspan.f64"
			delete(row, "writeType")
			row["readType"] = "u32"
			row["readonly"] = true
		}
		return row
	}
	arrayLength, viewLength := property(s.length, false), property(s.viewLength, true)
	sigs := s.check.GetSignaturesOfType(s.check.GetTypeOfSymbol(s.intrinsic), checker.SignatureKindCall)
	f := in.AsFunctionDeclaration()
	ao23Want(s.t, len(sigs) == 1 && sigs[0].Declaration() == in && len(sigs[0].TypeParameters()) == 1 && sigs[0].MinArgumentCount() == 3 && len(sigs[0].Parameters()) == 3 && !sigs[0].HasRestParameter() && sigs[0].ThisParameter() == nil && f.Body == nil && len(f.TypeParameters.Nodes) == 1, "actual sole original generic intrinsic signature")
	s.generic = sigs[0]
	s.genericT = s.generic.TypeParameters()[0]
	tSym := s.original(f.TypeParameters.Nodes[0])
	ao23Want(s.t, s.genericT.Symbol() == tSym && vt != s.genericT && s.check.GetDeclaredTypeOfSymbol(tSym) == s.genericT, "two scoped T identities, actual signature type parameter")
	ownerType := f.Parameters.Nodes[0].AsParameterDeclaration().Type
	ownerT := ownerType.AsArrayTypeNode().ElementType
	ao23Want(s.t, s.check.GetSymbolAtLocation(ownerT.AsTypeReferenceNode().TypeName) == tSym && s.check.GetTypeFromTypeNode(ownerT) == s.genericT, "intrinsic owner annotation uses original scoped T")
	s.arrayElement(s.check.GetTypeOfSymbol(s.generic.Parameters()[0]), s.genericT)
	s.viewType(s.check.GetReturnTypeOfSignature(s.generic), s.genericT)
	ret := f.Type.AsTypeReferenceNode()
	ao23Want(s.t, s.check.GetSymbolAtLocation(ret.TypeName) == s.view && len(ret.TypeArguments.Nodes) == 1 && s.check.GetTypeFromTypeNode(ret.TypeArguments.Nodes[0]) == s.genericT, "actual generic ReadSpan<T> return origin")
	params := []any{}
	for i, n := range f.Parameters.Nodes {
		p := n.AsParameterDeclaration()
		ao23Want(s.t, p.Initializer == nil && p.QuestionToken == nil && p.DotDotDotToken == nil && s.original(n) == s.generic.Parameters()[i], "complete required generic parameter declaration/order")
		if i > 0 {
			s.annotationScalar(p.Type, "u32")
		}
		params = append(params, ao23Map{"name": n.Name().Text(), "declarationRange": s.span(s.dts, n), "nameRange": s.span(s.dts, n.Name()), "typeRange": s.span(s.dts, p.Type)})
	}
	intrinsic := ao23Map{"id": s.bindingID(s.dts, in.Name()), "kind": "intrinsic", "name": "readSpan", "sourcePath": s.dts.FileName(), "declarationRange": s.span(s.dts, in), "nameRange": s.span(s.dts, in.Name()), "typeParameterRange": s.span(s.dts, f.TypeParameters.Nodes[0]), "parameters": params, "returnTypeRange": s.span(s.dts, f.Type)}
	s.proof["viewInterfaceTypeParameter"] = s.span(s.dts, viewParams[0])
	s.proof["genericTypeParameter"] = s.span(s.dts, f.TypeParameters.Nodes[0])
	s.proof["readonlyIndexer"] = true
	s.proof["readonlyLength"] = true
	return types, arrayLength, viewLength, intrinsic
}
