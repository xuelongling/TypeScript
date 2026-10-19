// SPDX-License-Identifier: Apache-2.0
// Scratch candidate: official Program/AST/checker witnesses only, never private projection.
package main

import (
	"encoding/base64"
	"fmt"
	"slices"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

const rs26cOfficialBeforeCandidate = "function identity(v: ReadSpan<f64>): ReadSpan<f64> { return v; }\nfunction mutate(a: f64[]): f64 { a.length = 0; return 1; }\nexport function entry(): f64 {\n  const a: f64[] = [1, 2];\n  const v = readSpan(a, 0, 2);\n  const w: ReadSpan<f64> = identity(v);\n  mutate(a);\n  w[0];\n  return 1;\n}\n"
const rs26cOfficialAfterCandidate = "function identity(v: ReadSpan<f64>): ReadSpan<f64> { return v; }\nfunction mutate(a: f64[]): f64 { a.length = 0; return 1; }\nexport function entry(): f64 {\n  const a: f64[] = [1, 2];\n  const v = readSpan(a, 0, 2);\n  const w: ReadSpan<f64> = identity(v);\n  w[0];\n  mutate(a);\n  return 1;\n}\n"
const rs26cOfficialDTSCandidate = "// SPDX-License-Identifier: MIT\ninterface Object {}\ninterface Function {}\ninterface CallableFunction {}\ninterface NewableFunction {}\ninterface IArguments {}\ninterface String {}\ninterface Number {}\ninterface Boolean {}\ninterface RegExp {}\ninterface Array<T> { [n: number]: T; length: number; }\ntype f64 = number;\ntype u32 = number;\ninterface ReadSpan<T> { readonly [n: number]: T; readonly length: u32; }\ndeclare function readSpan<T>(owner: T[], start: u32, length: u32): ReadSpan<T>;\n"

func rs26cOfficialProgramCandidate(t *testing.T, source string) (*rs26Scope, func()) {
	t.Helper()
	paths := []string{"/readspan26c/app.ts", "/readspan26c/minimal-arrays-v3.d.ts"}
	contents := map[string]string{paths[0]: source, paths[1]: rs26cOfficialDTSCandidate}
	p := compiler.NewProgram(compiler.ProgramOptions{Config: &tsoptions.ParsedCommandLine{
		ParsedConfig: &core.ParsedOptions{FileNames: paths, CompilerOptions: &core.CompilerOptions{
			NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext,
		}},
	}, Host: compiler.NewCompilerHost("/readspan26c", vfstest.FromMap(contents, true), "/readspan26c", nil, nil), SingleThreaded: core.TSTrue})
	groups, counts := ao23Map{}, ao23Map{}
	total := 0
	for _, group := range []struct {
		name  string
		items []*ast.Diagnostic
	}{
		{"syntax", p.GetSyntacticDiagnostics(t.Context(), nil)},
		{"semantic", p.GetSemanticDiagnostics(t.Context(), nil)},
		{"program", p.GetProgramDiagnostics()},
		{"global", p.GetGlobalDiagnostics(t.Context())},
	} {
		groups[group.name], counts[group.name] = rs26bDiagnostics(group.items), len(group.items)
		total += len(group.items)
	}
	t.Logf("TSFG_READSPAN_26C_OFFICIAL_DIAGNOSTICS json=%s", ao23JCS(t, groups))
	ao23Want(t, total == 0, "all four complete fresh official diagnostic groups are empty")
	consumed, files := []string{}, []any{}
	for _, f := range p.GetSourceFiles() {
		original, ok := contents[f.FileName()]
		ao23Want(t, ok && original == f.Text(), "only complete exact original source and DTS consumed")
		consumed = append(consumed, f.FileName())
		files = append(files, ao23Map{"path": f.FileName(), "content": f.Text(), "bytes": len(f.Text()),
			"utf16Length": ao23Units(f.Text()), "sha256": ao23Hash([]byte(f.Text()))})
	}
	slices.Sort(consumed)
	ao23Want(t, slices.Equal(consumed, paths), "complete original two-file closure")
	file, dts := p.GetSourceFile(paths[0]), p.GetSourceFile(paths[1])
	ao23Want(t, file != nil && dts != nil && ast.IsExternalModule(file), "original external module and pinned declarations")
	check, release := p.GetTypeCheckerForFileExclusive(t.Context(), file)
	base := &ao23Scope{t: t, file: file, dts: dts, check: check,
		headers: map[*ast.Symbol]*ao23Header{}, names: map[string]*ao23Header{}, counts: map[string]int{}}
	return &rs26Scope{ao23Scope: base, views: map[*ast.Symbol]string{}, proof: ao23Map{
		"diagnostics": counts, "diagnosticGroups": groups, "consumedPaths": consumed, "consumedFiles": files,
	}}, release
}

func rs26cOfficialFirstWitnessCandidate(t *testing.T, name, source string) ao23Map {
	t.Helper()
	s, release := rs26cOfficialProgramCandidate(t, source)
	defer release()
	types, arrayLength, viewLength, intrinsic := s.origins()
	s.rs26bAST()
	anchor := func(n *ast.Node, start, end int, token string) ao23Range {
		t.Helper()
		got := s.span(s.file, n)
		ao23Want(t, got == (ao23Range{start, end}) && source[start:end] == token, "literal original UTF16/token anchor: "+token)
		return got
	}
	nodes := s.file.Statements.Nodes
	ao23Want(t, len(nodes) == 3, "complete identity/mutate/entry source inventory")
	sigs := make([]*checker.Signature, 3)
	symbols := make([]*ast.Symbol, 3)
	headerProof := []any{}
	wantNames := []string{"identity", "mutate", "entry"}
	for j, n := range nodes {
		ao23Want(t, n.Kind == ast.KindFunctionDeclaration && n.Name().Kind == ast.KindIdentifier && n.Name().Text() == wantNames[j], "ordered original function declaration")
		f := n.AsFunctionDeclaration()
		if j == 2 {
			ao23Want(t, n.ModifierFlags() == ast.ModifierFlagsExport, "entry export modifier")
		} else {
			ao23Want(t, n.ModifierFlags() == 0, "private helper modifier")
		}
		ao23Want(t, f.TypeParameters == nil && f.AsteriskToken == nil && f.FullSignature == nil && f.Parameters != nil && f.Body != nil && f.Body.Kind == ast.KindBlock, "ordinary named nonoverloaded nongeneric complete function")
		symbols[j] = s.original(n)
		actual := s.check.GetSignaturesOfType(s.check.GetTypeOfSymbol(symbols[j]), checker.SignatureKindCall)
		ao23Want(t, len(actual) == 1, "sole actual call signature")
		sig := actual[0]
		ao23Want(t, sig.Declaration() == n && len(sig.Parameters()) == []int{1, 1, 0}[j] && len(sig.Parameters()) == len(f.Parameters.Nodes) && sig.MinArgumentCount() == len(f.Parameters.Nodes) && len(sig.TypeParameters()) == 0 && sig.ThisParameter() == nil && !sig.HasRestParameter(), "complete required original full signature slots")
		sigs[j] = sig
		returnKind := "f64"
		if j == 0 {
			returnKind = "readspan.f64"
			element := s.rs26bViewAnnotation(f.Type)
			anchor(f.Type, 37, 50, "ReadSpan<f64>")
			anchor(element, 46, 49, "f64")
			s.viewType(s.check.GetReturnTypeOfSignature(sig), s.check.GetNumberType())
		} else {
			s.annotationScalar(f.Type, "f64")
			ao23Want(t, s.check.GetReturnTypeOfSignature(sig) == s.check.GetNumberType(), "actual f64 return representation")
			if j == 1 {
				anchor(f.Type, 92, 95, "f64")
			} else {
				anchor(f.Type, 149, 152, "f64")
			}
		}
		params := []any{}
		for slot, pn := range f.Parameters.Nodes {
			p := pn.AsParameterDeclaration()
			sym := s.original(pn)
			ao23Want(t, pn.Name().Kind == ast.KindIdentifier && pn.ModifierFlags() == 0 && p.QuestionToken == nil && p.Initializer == nil && p.DotDotDotToken == nil && sym == sig.Parameters()[slot], "actual required parameter symbol equals original full slot")
			kind := "array.f64"
			var element *ast.Node
			if j == 0 {
				kind = "readspan.f64"
				element = s.rs26bViewAnnotation(p.Type)
				s.viewType(s.check.GetTypeOfSymbol(sym), s.check.GetNumberType())
				anchor(pn, 18, 34, "v: ReadSpan<f64>")
				anchor(p.Type, 21, 34, "ReadSpan<f64>")
				anchor(element, 30, 33, "f64")
			} else {
				ao23Want(t, p.Type.Kind == ast.KindArrayType, "original explicit Array annotation")
				element = p.Type.AsArrayTypeNode().ElementType
				s.annotationScalar(element, "f64")
				s.arrayType(s.check.GetTypeOfSymbol(sym), false)
				anchor(pn, 81, 89, "a: f64[]")
				anchor(p.Type, 84, 89, "f64[]")
				anchor(element, 84, 87, "f64")
			}
			params = append(params, ao23Map{"index": slot, "name": sym.Name, "declaredType": kind,
				"declarationRange": s.span(s.file, pn), "nameRange": s.span(s.file, pn.Name()),
				"annotationRange": s.span(s.file, p.Type), "elementAnnotationRange": s.span(s.file, element),
				"signatureSymbolEqualsOriginal": true})
		}
		ao23Want(t, len(f.Body.AsBlock().Statements.Nodes) == []int{1, 2, 6}[j], "complete original body count")
		headerProof = append(headerProof, ao23Map{"name": wantNames[j], "range": s.span(s.file, n),
			"nameRange": s.span(s.file, n.Name()), "returnType": returnKind,
			"returnAnnotationRange": s.span(s.file, f.Type), "parameters": params})
	}
	anchor(nodes[0], 0, 64, "function identity(v: ReadSpan<f64>): ReadSpan<f64> { return v; }")
	anchor(nodes[0].Name(), 9, 17, "identity")
	anchor(nodes[1], 65, 123, "function mutate(a: f64[]): f64 { a.length = 0; return 1; }")
	anchor(nodes[1].Name(), 74, 80, "mutate")
	anchor(nodes[2], 124, 287, source[124:287])
	anchor(nodes[2].Name(), 140, 145, "entry")
	identityReturn := nodes[0].AsFunctionDeclaration().Body.AsBlock().Statements.Nodes[0]
	ao23Want(t, identityReturn.Kind == ast.KindReturnStatement, "only final original view return")
	returned := identityReturn.AsReturnStatement().Expression
	anchor(returned, 60, 61, "v")
	ao23Want(t, returned.Kind == ast.KindIdentifier && s.check.GetSymbolAtLocation(returned) == sigs[0].Parameters()[0], "returned reference is actual original required view formal slot0")
	s.viewType(s.check.GetTypeAtLocation(returned), s.check.GetNumberType())
	ao23Want(t, s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(returned), s.check.GetReturnTypeOfSignature(sigs[0])), "real official return assignability")

	body := nodes[2].AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
	locals := make([]*ast.Node, 3)
	localSymbols := make([]*ast.Symbol, 3)
	localProof := []any{}
	for i, stmt := range body[:3] {
		ao23Want(t, stmt.Kind == ast.KindVariableStatement, "complete three original const locals")
		list := stmt.AsVariableStatement().DeclarationList
		decls := list.AsVariableDeclarationList().Declarations.Nodes
		ao23Want(t, list.Flags&ast.NodeFlagsConst != 0 && len(decls) == 1, "one original const declarator")
		locals[i] = decls[0]
		localSymbols[i] = s.original(decls[0])
		decl := decls[0].AsVariableDeclaration()
		ao23Want(t, decl.Initializer != nil && decl.ExclamationToken == nil && decls[0].Name().Text() == []string{"a", "v", "w"}[i], "actual complete ordered const binding")
		if i == 0 {
			s.arrayType(s.check.GetTypeOfSymbol(localSymbols[i]), false)
		} else {
			s.viewType(s.check.GetTypeOfSymbol(localSymbols[i]), s.check.GetNumberType())
		}
		ao23Want(t, s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(decl.Initializer), s.check.GetTypeOfSymbol(localSymbols[i])), "official initializer-to-original-const assignability")
		localProof = append(localProof, ao23Map{"name": decls[0].Name().Text(), "range": s.span(s.file, decls[0]),
			"nameRange": s.span(s.file, decls[0].Name()), "initializerRange": s.span(s.file, decl.Initializer),
			"actualUniqueOriginalSymbol": true})
	}
	ao23Want(t, localSymbols[0] != localSymbols[1] && localSymbols[1] != localSymbols[2], "distinct actual local declaration symbols")
	anchor(locals[0], 163, 180, "a: f64[] = [1, 2]")
	anchor(locals[0].Name(), 163, 164, "a")
	anchor(locals[1], 190, 211, "v = readSpan(a, 0, 2)")
	anchor(locals[1].Name(), 190, 191, "v")
	anchor(locals[2], 221, 251, "w: ReadSpan<f64> = identity(v)")
	anchor(locals[2].Name(), 221, 222, "w")
	wDecl := locals[2].AsVariableDeclaration()
	wElement := s.rs26bViewAnnotation(wDecl.Type)
	anchor(wDecl.Type, 224, 237, "ReadSpan<f64>")
	anchor(wElement, 233, 236, "f64")
	created := locals[1].AsVariableDeclaration().Initializer
	anchor(created, 194, 211, "readSpan(a, 0, 2)")
	ao23Want(t, created.Kind == ast.KindCallExpression, "original checked intrinsic expression")
	intrinsicCall := created.AsCallExpression()
	intrinsicSig := s.check.GetResolvedSignature(created)
	ao23Want(t, intrinsicCall.Expression.Kind == ast.KindIdentifier && s.check.GetSymbolAtLocation(intrinsicCall.Expression) == s.intrinsic && intrinsicCall.TypeArguments == nil && intrinsicCall.QuestionDotToken == nil && len(intrinsicCall.Arguments.Nodes) == 3, "actual original pinned readSpan invocation")
	ao23Want(t, intrinsicSig != nil && intrinsicSig.Declaration() == s.intrinsicNode && intrinsicSig.MinArgumentCount() == 3 && len(intrinsicSig.Parameters()) == 3 && len(intrinsicSig.TypeParameters()) == 0 && intrinsicSig.ThisParameter() == nil && !intrinsicSig.HasRestParameter(), "actual instantiated complete intrinsic signature")
	for i, p := range intrinsicSig.Parameters() {
		original := s.intrinsicNode.AsFunctionDeclaration().Parameters.Nodes[i]
		ao23Want(t, p != nil && len(p.Declarations) == 1 && p.Declarations[0] == original && p.Name == original.Name().Text(), "instantiated parameter retains original declaration and full slot")
		ao23Want(t, s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(intrinsicCall.Arguments.Nodes[i]), s.check.GetTypeOfSymbol(p)), "official intrinsic argument assignability")
		if i == 0 {
			s.arrayElement(s.check.GetTypeOfSymbol(p), s.check.GetNumberType())
		} else {
			ao23Want(t, s.check.GetTypeOfSymbol(p) == s.check.GetNumberType(), "actual scalar intrinsic parameter")
		}
	}
	ao23Want(t, s.check.GetSymbolAtLocation(intrinsicCall.Arguments.Nodes[0]) == localSymbols[0], "actual Array owner binding passed to original intrinsic")
	s.viewType(s.check.GetReturnTypeOfSignature(intrinsicSig), s.check.GetNumberType())
	s.viewType(s.check.GetTypeAtLocation(created), s.check.GetNumberType())

	resultCall := wDecl.Initializer
	anchor(resultCall, 240, 251, "identity(v)")
	ao23Want(t, resultCall.Kind == ast.KindCallExpression, "original ordinary returned-view initializer")
	rc := resultCall.AsCallExpression()
	resolved := s.check.GetResolvedSignature(resultCall)
	ao23Want(t, rc.Expression.Kind == ast.KindIdentifier && s.check.GetSymbolAtLocation(rc.Expression) == symbols[0] && rc.TypeArguments == nil && rc.QuestionDotToken == nil && len(rc.Arguments.Nodes) == 1, "actual same-module identity callee and original argument inventory")
	ao23Want(t, resolved != nil && resolved.Declaration() == nodes[0] && len(resolved.Parameters()) == 1 && resolved.Parameters()[0] == sigs[0].Parameters()[0] && resolved.MinArgumentCount() == 1 && len(resolved.TypeParameters()) == 0 && resolved.ThisParameter() == nil && !resolved.HasRestParameter(), "resolved return call preserves exact original required signature")
	rp := resolved.Parameters()[0]
	ao23Want(t, len(rp.Declarations) == 1 && rp.Declarations[0] == nodes[0].AsFunctionDeclaration().Parameters.Nodes[0], "resolved full slot0 joins original declaration")
	anchor(rc.Expression, 240, 248, "identity")
	anchor(rc.Arguments.Nodes[0], 249, 250, "v")
	ao23Want(t, s.check.GetSymbolAtLocation(rc.Arguments.Nodes[0]) == localSymbols[1] && s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(rc.Arguments.Nodes[0]), s.check.GetTypeOfSymbol(rp)), "actual original nominal view argument and official assignability")
	s.viewType(s.check.GetTypeAtLocation(rc.Arguments.Nodes[0]), s.check.GetNumberType())
	s.viewType(s.check.GetReturnTypeOfSignature(resolved), s.check.GetNumberType())
	s.viewType(s.check.GetTypeAtLocation(resultCall), s.check.GetNumberType())

	mutateIndex, useIndex, mutateStart, useStart := 3, 4, 255, 268
	if name == "after" {
		mutateIndex, useIndex, mutateStart, useStart = 4, 3, 263, 255
	}
	ao23Want(t, body[mutateIndex].Kind == ast.KindExpressionStatement && body[useIndex].Kind == ast.KindExpressionStatement, "original call and returned-view use are statements")
	mutationCall := body[mutateIndex].AsExpressionStatement().Expression
	anchor(mutationCall, mutateStart, mutateStart+9, "mutate(a)")
	mc := mutationCall.AsCallExpression()
	ms := s.check.GetResolvedSignature(mutationCall)
	ao23Want(t, mc.Expression.Kind == ast.KindIdentifier && s.check.GetSymbolAtLocation(mc.Expression) == symbols[1] && len(mc.Arguments.Nodes) == 1 && mc.TypeArguments == nil && mc.QuestionDotToken == nil, "actual ordinary mutate call")
	ao23Want(t, ms != nil && ms.Declaration() == nodes[1] && len(ms.Parameters()) == 1 && ms.Parameters()[0] == sigs[1].Parameters()[0] && ms.MinArgumentCount() == 1 && len(ms.TypeParameters()) == 0 && ms.ThisParameter() == nil && !ms.HasRestParameter() && s.check.GetReturnTypeOfSignature(ms) == s.check.GetNumberType(), "actual resolved original mutate full-slot f64 signature")
	ao23Want(t, s.check.GetSymbolAtLocation(mc.Arguments.Nodes[0]) == localSymbols[0] && s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(mc.Arguments.Nodes[0]), s.check.GetTypeOfSymbol(ms.Parameters()[0])), "actual original Array argument and assignability")
	anchor(mc.Arguments.Nodes[0], mutateStart+7, mutateStart+8, "a")
	used := body[useIndex].AsExpressionStatement().Expression
	anchor(used, useStart, useStart+4, "w[0]")
	ao23Want(t, used.Kind == ast.KindElementAccessExpression, "actual returned-view element read")
	access := used.AsElementAccessExpression()
	anchor(access.Expression, useStart, useStart+1, "w")
	anchor(access.ArgumentExpression, useStart+2, useStart+3, "0")
	ao23Want(t, access.QuestionDotToken == nil && s.check.GetSymbolAtLocation(access.Expression) == localSymbols[2] && s.check.GetTypeAtLocation(used) == s.check.GetNumberType(), "actual read joins original returned-view const, readonly numeric result")
	s.viewType(s.check.GetTypeAtLocation(access.Expression), s.check.GetNumberType())
	ao23Want(t, body[5].Kind == ast.KindReturnStatement, "entry keeps final original f64 return")
	anchor(body[5], 276, 285, "return 1;")
	ao23Want(t, s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(body[5].AsReturnStatement().Expression), s.check.GetReturnTypeOfSignature(sigs[2])), "official entry f64 return assignability")
	exports := s.check.GetExportsOfModule(s.file.AsNode().Symbol())
	ao23Want(t, len(exports) == 1 && exports[0].Name == "entry" && (exports[0] == symbols[2] || symbols[2].ExportSymbol == exports[0]), "exact actual export, helpers remain private")
	inventory := map[string]int{}
	var visit func(*ast.Node) bool
	visit = func(n *ast.Node) bool { inventory[n.Kind.String()]++; n.ForEachChild(visit); return false }
	visit(s.file.AsNode())
	ao23Want(t, inventory[ast.KindFunctionDeclaration.String()] == 3 && inventory[ast.KindCallExpression.String()] == 3 && inventory[ast.KindVariableDeclaration.String()] == 3 && inventory[ast.KindReturnStatement.String()] == 3 && inventory[ast.KindElementAccessExpression.String()] == 1, "complete original function/call/local/return/read AST inventory")
	s.proof["headers"], s.proof["locals"], s.proof["inventory"] = headerProof, localProof, inventory
	s.proof["returnRoles"] = ao23Map{"parameterReference": s.span(s.file, returned),
		"returnAnnotation": s.span(s.file, nodes[0].AsFunctionDeclaration().Type),
		"callResult":       s.span(s.file, resultCall), "callerBinding": s.span(s.file, locals[2]),
		"callArgument": s.span(s.file, rc.Arguments.Nodes[0]), "mutationCall": s.span(s.file, mutationCall),
		"resultUse": s.span(s.file, used), "signatureOriginalFullSlot": 0, "resolvedResultNominalReadonly": true}
	return ao23Map{"name": name, "sourcePath": s.file.FileName(), "source": source,
		"sourceBytes": len(source), "sourceUTF16Length": ao23Units(source), "sourceSHA256": ao23Hash([]byte(source)),
		"DTSBytes": len(rs26cOfficialDTSCandidate), "DTSSHA256": ao23Hash([]byte(rs26cOfficialDTSCandidate)),
		"official": s.proof, "types": types, "arrayLength": arrayLength, "readSpanLength": viewLength, "intrinsic": intrinsic,
		"privateProducerCalled": false, "ProductModuleCalled": false, "factsPacketConstructed": false}
}

func TestReadSpan26cOfficialFreshWholeFirstPair(t *testing.T) {
	ao23Want(t, len(rs26cOfficialDTSCandidate) == 484 && ao23Hash([]byte(rs26cOfficialDTSCandidate)) == "282b335a6e0e0493fa74a3c1dd56f234f8c679079acdaf0600cefb8306c26415", "exact unchanged pinned484B DTS3")
	for _, item := range []struct{ name, source, sha string }{
		{"before", rs26cOfficialBeforeCandidate, "4d6ca7ff0a2961f7e915375f9f241c882e3259e3a780473a152d3d4fa5da52ab"},
		{"after", rs26cOfficialAfterCandidate, "681c010b1954ece146b14d31aaf85413fec84abba257d6df919e6584807def2a"},
	} {
		t.Run(item.name, func(t *testing.T) {
			ao23Want(t, len(item.source) == 288 && ao23Units(item.source) == 288 && ao23Hash([]byte(item.source)) == item.sha, "complete exact independent288B source")
			witness := ao23JCS(t, rs26cOfficialFirstWitnessCandidate(t, item.name, item.source))
			fmt.Printf("TSFG_READSPAN_26C_OFFICIAL_WHOLE_FIRST_PAIR name=%s bytes=%d sha256=%s base64=%s\n", item.name, len(witness), ao23Hash(witness), base64.StdEncoding.EncodeToString(witness))
		})
	}
}
