// SPDX-License-Identifier: Apache-2.0
// Test-only official object acquisition. No private Source or Product projection is called.
package main

import (
	"slices"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

func rs26bDiagnostics(items []*ast.Diagnostic) []any {
	rows := []any{}
	for _, d := range items {
		row := ao23Map{"code": d.Code(), "category": int(d.Category()), "message": d.String(), "messageKey": d.MessageKey(), "messageArgs": d.MessageArgs(), "reportsUnnecessary": d.ReportsUnnecessary(), "reportsDeprecated": d.ReportsDeprecated(), "skippedOnNoEmit": d.SkippedOnNoEmit(), "messageChain": rs26bDiagnostics(d.MessageChain()), "relatedInformation": rs26bDiagnostics(d.RelatedInformation())}
		if f := d.File(); f != nil {
			p := f.GetPositionMap()
			row["sourcePath"], row["range"] = f.FileName(), ao23Range{p.UTF8ToUTF16(d.Pos()), p.UTF8ToUTF16(d.End())}
		} else {
			row["sourcePath"], row["range"] = nil, nil
		}
		rows = append(rows, row)
	}
	return rows
}

func rs26bProgram(t *testing.T, source, dts string) (*rs26Scope, func()) {
	paths := []string{"/readspan26b/app.ts", "/readspan26b/minimal-arrays-v3.d.ts"}
	contents := map[string]string{paths[0]: source, paths[1]: dts}
	p := compiler.NewProgram(compiler.ProgramOptions{Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{FileNames: paths, CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext}}}, Host: compiler.NewCompilerHost("/readspan26b", vfstest.FromMap(contents, true), "/readspan26b", nil, nil), SingleThreaded: core.TSTrue})
	groups := ao23Map{}
	counts := ao23Map{}
	total := 0
	for _, g := range []struct {
		name  string
		items []*ast.Diagnostic
	}{{"syntax", p.GetSyntacticDiagnostics(t.Context(), nil)}, {"semantic", p.GetSemanticDiagnostics(t.Context(), nil)}, {"program", p.GetProgramDiagnostics()}, {"global", p.GetGlobalDiagnostics(t.Context())}} {
		groups[g.name], counts[g.name] = rs26bDiagnostics(g.items), len(g.items)
		total += len(g.items)
	}
	t.Logf("TSFG_READSPAN_26B_OFFICIAL_DIAGNOSTICS json=%s", ao23JCS(t, groups))
	ao23Want(t, total == 0, "all four complete fresh official diagnostic groups must be empty")
	consumed, files := []string{}, []any{}
	for _, f := range p.GetSourceFiles() {
		text, ok := contents[f.FileName()]
		ao23Want(t, ok && text == f.Text(), "actual Program consumed only exact original source and DTS")
		consumed = append(consumed, f.FileName())
		files = append(files, ao23Map{"path": f.FileName(), "content": f.Text(), "sha256": ao23Hash([]byte(f.Text())), "byteLength": len(f.Text()), "utf16Length": ao23Units(f.Text())})
	}
	slices.Sort(consumed)
	ao23Want(t, slices.Equal(paths, consumed), "unique complete original two-file closure")
	file, decls := p.GetSourceFile(paths[0]), p.GetSourceFile(paths[1])
	ao23Want(t, file != nil && decls != nil && ast.IsExternalModule(file), "original complete external source module")
	check, release := p.GetTypeCheckerForFileExclusive(t.Context(), file)
	base := &ao23Scope{t: t, file: file, dts: decls, check: check, headers: map[*ast.Symbol]*ao23Header{}, names: map[string]*ao23Header{}, counts: map[string]int{}}
	return &rs26Scope{ao23Scope: base, views: map[*ast.Symbol]string{}, proof: ao23Map{"diagnostics": counts, "diagnosticGroups": groups, "consumedPaths": consumed, "consumedFiles": files}}, release
}

func (s *rs26Scope) rs26bAST() {
	rows, declarations := []any{}, []any{}
	for _, f := range []*ast.SourceFile{s.file, s.dts} {
		var visit func(*ast.Node) bool
		visit = func(n *ast.Node) bool {
			ao23Want(s.t, ast.GetSourceFileOfNode(n) == f && n.Pos() >= 0 && n.Pos() <= n.End() && n.End() <= len(f.Text()), "every inventoried node is actual original AST")
			p := f.GetPositionMap()
			r := ao23Range{p.UTF8ToUTF16(n.Pos()), p.UTF8ToUTF16(n.End())}
			ao23Want(s.t, r.Start == ao23Units(f.Text()[:n.Pos()]) && r.End == ao23Units(f.Text()[:n.End()]), "all original AST UTF16 boundaries independently agree")
			row := ao23Map{"sourcePath": f.FileName(), "kind": n.Kind.String(), "fullRange": r}
			if n.Kind == ast.KindIdentifier {
				row["text"] = n.Text()
				sym := s.check.GetSymbolAtLocation(n)
				ao23Want(s.t, sym != nil && len(sym.Declarations) > 0, "every original identifier joins an actual Symbol and declaration")
				origins := []any{}
				for _, d := range sym.Declarations {
					owner := ast.GetSourceFileOfNode(d)
					ao23Want(s.t, owner == s.file || owner == s.dts, "every actual Symbol declaration belongs to original closure")
					origins = append(origins, ao23Map{"sourcePath": owner.FileName(), "kind": d.Kind.String(), "range": s.span(owner, d)})
				}
				row["symbol"] = ao23Map{"name": sym.Name, "declarations": origins}
			}
			rows = append(rows, row)
			n.ForEachChild(visit)
			return false
		}
		visit(f.AsNode())
	}
	names := []string{"Object", "Function", "CallableFunction", "NewableFunction", "IArguments", "String", "Number", "Boolean", "RegExp", "Array", "f64", "u32", "ReadSpan", "readSpan"}
	ao23Want(s.t, len(s.dts.Statements.Nodes) == len(names), "complete fourteen-declaration original DTS")
	for i, n := range s.dts.Statements.Nodes {
		sym := s.original(n)
		ao23Want(s.t, n.Name().Text() == names[i] && sym.Name == names[i], "ordered original DTS declaration and sole Symbol identity")
		declarations = append(declarations, ao23Map{"name": names[i], "range": s.span(s.dts, n), "nameRange": s.span(s.dts, n.Name()), "kind": n.Kind.String()})
	}
	s.proof["originalAST"], s.proof["dtsDeclarations"] = rows, declarations
}

func (s *rs26Scope) rs26bViewAnnotation(n *ast.Node) *ast.Node {
	ao23Want(s.t, n != nil && n.Kind == ast.KindTypeReference, "direct original ReadSpan<f64> annotation")
	r := n.AsTypeReferenceNode()
	ao23Want(s.t, r.TypeName.Kind == ast.KindIdentifier && r.TypeName.Text() == "ReadSpan" && s.check.GetSymbolAtLocation(r.TypeName) == s.view && r.TypeArguments != nil && len(r.TypeArguments.Nodes) == 1, "actual pinned ReadSpan interface origin and explicit arity")
	s.annotationScalar(r.TypeArguments.Nodes[0], "f64")
	s.viewType(s.check.GetTypeFromTypeNode(n), s.check.GetNumberType())
	return r.TypeArguments.Nodes[0]
}

func (s *rs26Scope) rs26bHeaders() []*ao23Header {
	out, proof := []*ao23Header{}, []any{}
	for _, n := range s.file.Statements.Nodes {
		ao23Want(s.t, n.Kind == ast.KindFunctionDeclaration && n.Name() != nil && n.Name().Kind == ast.KindIdentifier && (n.ModifierFlags() == 0 || n.ModifierFlags() == ast.ModifierFlagsExport), "complete original ordinary function declaration")
		f := n.AsFunctionDeclaration()
		ao23Want(s.t, f.TypeParameters == nil && f.AsteriskToken == nil && f.FullSignature == nil && f.Parameters != nil && f.Body != nil && f.Body.Kind == ast.KindBlock, "required ordinary nongeneric body/signature")
		s.annotationScalar(f.Type, "f64")
		sym := s.original(n)
		ao23Want(s.t, s.names[n.Name().Text()] == nil, "unique complete source header name")
		sigs := s.check.GetSignaturesOfType(s.check.GetTypeOfSymbol(sym), checker.SignatureKindCall)
		ao23Want(s.t, len(sigs) == 1 && sigs[0].Declaration() == n && len(sigs[0].Parameters()) == len(f.Parameters.Nodes) && sigs[0].MinArgumentCount() == len(f.Parameters.Nodes) && len(sigs[0].TypeParameters()) == 0 && sigs[0].ThisParameter() == nil && !sigs[0].HasRestParameter() && s.check.GetReturnTypeOfSignature(sigs[0]) == s.check.GetNumberType(), "sole actual signature with complete required original slots")
		h := &ao23Header{node: n, symbol: sym, signature: sigs[0], parameters: []ao23Map{}, kinds: []string{}}
		slots := []any{}
		for i, pn := range f.Parameters.Nodes {
			p := pn.AsParameterDeclaration()
			ao23Want(s.t, pn.Name() != nil && pn.Name().Kind == ast.KindIdentifier && pn.ModifierFlags() == 0 && pn.Name().Text() != "this" && p.Initializer == nil && p.QuestionToken == nil && p.DotDotDotToken == nil, "original required nondefault nonrest nonthis parameter")
			actual := s.original(pn)
			ao23Want(s.t, actual == h.signature.Parameters()[i], "actual original parameter Symbol equals full signature slot")
			kind := "readspan.f64"
			var element *ast.Node
			if p.Type != nil && p.Type.Kind == ast.KindTypeReference && p.Type.AsTypeReferenceNode().TypeName.Text() == "ReadSpan" {
				element = s.rs26bViewAnnotation(p.Type)
				s.counts["viewParameters"]++
				s.viewType(s.check.GetTypeOfSymbol(actual), s.check.GetNumberType())
			} else {
				kind = s.annotation(p.Type)
				if kind == "array.f64" {
					element = p.Type.AsArrayTypeNode().ElementType
					s.arrayType(s.check.GetTypeOfSymbol(actual), false)
				} else {
					ao23Want(s.t, s.check.GetTypeOfSymbol(actual) == s.check.GetNumberType(), "actual scalar parameter representation")
				}
			}
			row := ao23Map{"id": s.bindingID(s.file, pn.Name()), "name": pn.Name().Text(), "kind": "parameter", "declaredType": kind, "writable": kind != "readspan.f64", "declarationRange": s.span(s.file, pn), "nameRange": s.span(s.file, pn.Name()), "annotationRange": s.span(s.file, p.Type)}
			if element != nil {
				row["elementAnnotationRange"] = s.span(s.file, element)
			}
			h.parameters, h.kinds = append(h.parameters, row), append(h.kinds, kind)
			slots = append(slots, ao23Map{"index": i, "name": actual.Name, "binding": row["id"], "declaredType": kind, "declarationRange": row["declarationRange"], "annotationRange": row["annotationRange"], "signatureSymbolEqualsOriginal": true})
			s.counts["parameters"]++
		}
		h.row = ao23Map{"id": s.bindingID(s.file, n.Name()), "name": n.Name().Text(), "sourcePath": s.file.FileName(), "declarationRange": s.span(s.file, n), "nameRange": s.span(s.file, n.Name()), "returnType": "f64", "returnAnnotationRange": s.span(s.file, f.Type), "parameters": h.parameters, "bindings": []any{}, "body": []any{}}
		s.headers[sym], s.names[n.Name().Text()] = h, h
		out = append(out, h)
		proof = append(proof, ao23Map{"name": n.Name().Text(), "binding": h.row["id"], "parameters": slots, "requiredArity": len(slots), "originalDeclaration": true})
	}
	s.counts["functions"] = len(out)
	s.proof["headers"] = proof
	return out
}
