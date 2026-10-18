// SPDX-License-Identifier: Apache-2.0
// These controls observe original compiler objects, never the Array facts converter.
package main

import (
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/jsnum"
	"github.com/microsoft/typescript-go/internal/scanner"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

func arrayFirstAnchor25(t *testing.T, file *ast.SourceFile, node *ast.Node, want arrayFirstRange25) {
	t.Helper()
	if node == nil || ast.GetSourceFileOfNode(node) != file {
		t.Fatal("foreign or missing original node")
	}
	start := scanner.SkipTrivia(file.Text(), node.Pos())
	positions := file.GetPositionMap()
	actual := arrayFirstRange25{positions.UTF8ToUTF16(start), positions.UTF8ToUTF16(node.End())}
	if start >= node.End() || actual != want {
		t.Fatalf("original UTF16 range differs: actual=%+v frozen=%+v node=%s", actual, want, node.Kind)
	}
}
func arrayFirstBindingID25(t *testing.T, path string, where arrayFirstRange25) string {
	t.Helper()
	return "sha256:" + arrayFirstHash25(append([]byte("tsfg.array-facts.binding.v1\x00"), arrayFirstCanonical25(t, map[string]any{"sourcePath": path, "nameRange": where})...))
}
func arrayFirstF64Annotation25(t *testing.T, check *checker.Checker, node *ast.Node, original *ast.Symbol) {
	t.Helper()
	if node == nil || node.Kind != ast.KindTypeReference {
		t.Fatal("explicit original f64 TypeReference required")
	}
	ref := node.AsTypeReferenceNode()
	if ref.TypeName == nil || ref.TypeName.Kind != ast.KindIdentifier || ref.TypeName.Text() != "f64" || ref.TypeArguments != nil || check.GetSymbolAtLocation(ref.TypeName) != original || check.GetTypeFromTypeNode(node) != check.GetNumberType() {
		t.Fatal("f64 syntax/symbol provenance or official Number changed")
	}
}
func arrayFirstArrayType25(t *testing.T, check *checker.Checker, typ *checker.Type, arraySymbol *ast.Symbol, indexNode *ast.Node, empty bool) {
	t.Helper()
	if typ == nil || !check.IsArrayType(typ) || checker.IsTupleType(typ) || typ.Target().Symbol() != arraySymbol {
		t.Fatal("actual global Array<T> identity required")
	}
	args := check.GetTypeArguments(typ)
	if len(args) != 1 || args[0] != check.GetNumberType() && !(empty && args[0].Flags() == checker.TypeFlagsNever) {
		t.Fatal("official array element argument changed")
	}
	info := check.GetIndexInfoOfType(typ, check.GetNumberType())
	if info == nil || info.KeyType() != check.GetNumberType() || info.ValueType() != args[0] || info.IsReadonly() || info.Declaration() != indexNode {
		t.Fatal("actual pinned numeric indexer/value/permission changed")
	}
}
func arrayFirstLiteral25(t *testing.T, file *ast.SourceFile, check *checker.Checker, node *ast.Node, want arrayFirstValue25) {
	t.Helper()
	arrayFirstAnchor25(t, file, node, want.Range)
	atom, sign := node, ""
	if node.Kind == ast.KindPrefixUnaryExpression {
		p := node.AsPrefixUnaryExpression()
		atom = p.Operand
		if p.Operator == ast.KindMinusToken {
			sign = "-"
		} else if p.Operator == ast.KindPlusToken {
			sign = "+"
		} else {
			t.Fatal("foreign numeric unary operator")
		}
	}
	if atom == nil || atom.Kind != ast.KindNumericLiteral || want.Kind != "numericLiteral" || want.Type != "f64" {
		t.Fatal("original finite numeric element shape")
	}
	start := scanner.SkipTrivia(file.Text(), atom.Pos())
	if sign+file.Text()[start:atom.End()] != want.Token {
		t.Fatal("original numeric spelling/sign differs")
	}
	typ := check.GetTypeAtLocation(atom)
	if typ == nil || !typ.IsNumberLiteral() || check.GetBaseTypeOfLiteralType(typ) != check.GetNumberType() {
		t.Fatal("actual official NumberLiteral element")
	}
	n, ok := typ.AsLiteralType().Value().(jsnum.Number)
	if !ok || n.IsNaN() || n.IsInf() {
		t.Fatal("official finite JS number value")
	}
	// Literal-type caches can merge signed zero. Its original unary AST retains the sign.
	bits := math.Float64bits(math.Abs(float64(n)))
	if sign == "-" {
		bits |= uint64(1) << 63
	}
	if fmt.Sprintf("%016x", bits) != want.Bits {
		t.Fatalf("independent IEEE bits differ: token=%s official=%016x frozen=%s", want.Token, bits, want.Bits)
	}
}
func arrayFirstReference25(t *testing.T, file *ast.SourceFile, check *checker.Checker, node *ast.Node, want *arrayFirstValue25, symbols map[string]*ast.Symbol, arraySymbol *ast.Symbol, indexNode *ast.Node) {
	t.Helper()
	if want == nil || want.Kind != "reference" || want.FlowType != "array.f64" || node == nil || node.Kind != ast.KindIdentifier || symbols[want.Binding] == nil || check.GetSymbolAtLocation(node) != symbols[want.Binding] {
		t.Fatal("original same-binding array reference")
	}
	arrayFirstAnchor25(t, file, node, want.Range)
	arrayFirstArrayType25(t, check, check.GetTypeAtLocation(node), arraySymbol, indexNode, false)
}
func arrayFirstCheckFunction25(t *testing.T, s *arrayOfficialScope25, node *ast.Node, want arrayFirstFunction25) *ast.Symbol {
	t.Helper()
	if node.Kind != ast.KindFunctionDeclaration || node.Name() == nil || node.Name().Kind != ast.KindIdentifier || node.Name().Text() != want.Name || node.ModifierFlags() != ast.ModifierFlagsExport {
		t.Fatal("original named ordinary export declaration")
	}
	fn := node.AsFunctionDeclaration()
	if fn.TypeParameters != nil || fn.AsteriskToken != nil || fn.FullSignature != nil || fn.Parameters == nil || fn.Body == nil || fn.Body.Kind != ast.KindBlock || want.ReturnType != "f64" || want.SourcePath != s.file.FileName() {
		t.Fatal("complete required-scalar explicit f64 function")
	}
	actual := s.check.GetSymbolAtLocation(node.Name())
	if actual == nil || len(actual.Declarations) != 1 || actual.Declarations[0] != node {
		t.Fatal("original function Symbol identity")
	}
	arrayFirstAnchor25(t, s.file, node, want.DeclarationRange)
	arrayFirstAnchor25(t, s.file, node.Name(), want.NameRange)
	arrayFirstAnchor25(t, s.file, fn.Type, want.ReturnAnnotationRange)
	arrayFirstF64Annotation25(t, s.check, fn.Type, s.scalars["f64"])
	if want.ID != arrayFirstBindingID25(t, s.file.FileName(), want.NameRange) {
		t.Fatal("frozen function binding preimage")
	}
	sigs := s.check.GetSignaturesOfType(s.check.GetTypeOfSymbol(actual), checker.SignatureKindCall)
	if len(sigs) != 1 || sigs[0].Declaration() != node || len(sigs[0].TypeParameters()) != 0 || s.check.GetReturnTypeOfSignature(sigs[0]) != s.check.GetNumberType() {
		t.Fatal("complete actual required-scalar f64 return signature")
	}
	s.parameters(t, fn.Parameters.Nodes, sigs[0].Parameters(), want.Parameters)
	body := fn.Body.AsBlock().Statements.Nodes
	if len(body) != len(want.Body) || len(body) < 2 || len(body) > 50 || len(want.Bindings) < 1 || len(want.Bindings) > 17 {
		t.Fatal("complete original finite statement collection")
	}
	locals, mutations := 0, 0
	for i, fact := range want.Body {
		statement := body[i]
		switch fact.Kind {
		case "variable":
			if locals >= len(want.Bindings) {
				t.Fatal("extra original local statement")
			}
			s.local(t, statement, fact, want.Bindings[locals])
			locals++
		case "elementAssignment", "lengthAssignment":
			s.mutation(t, statement, fact)
			mutations++
		case "return":
			if i != len(body)-1 || statement.Kind != ast.KindReturnStatement || fact.Value == nil || fact.Value.Kind != "checkedElementAccess" {
				t.Fatal("one original final checked-read return")
			}
			arrayFirstAnchor25(t, s.file, statement, fact.Range)
			s.value(t, statement.AsReturnStatement().Expression, fact.Value)
		default:
			t.Fatal("unhandled original statement")
		}
	}
	if locals != len(want.Bindings) || mutations > 32 || want.Body[len(want.Body)-1].Kind != "return" {
		t.Fatal("complete local/mutation/final-return accounting")
	}
	return actual
}

func arrayFirstOfficial25(t *testing.T, r request, packet arrayFirstPackage25) {
	t.Helper()
	contents := map[string]string{}
	paths := []string{}
	for _, input := range r.Files {
		contents[input.Path] = input.Content
		paths = append(paths, input.Path)
	}
	slices.Sort(paths)
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{FileNames: paths, CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext}}},
		Host:   compiler.NewCompilerHost(r.SourceRoot, vfstest.FromMap(contents, true), r.SourceRoot, nil, nil), SingleThreaded: core.TSTrue,
	})
	for _, group := range []struct {
		name  string
		items []*ast.Diagnostic
	}{{"syntax", program.GetSyntacticDiagnostics(t.Context(), nil)}, {"semantic", program.GetSemanticDiagnostics(t.Context(), nil)}, {"program", program.GetProgramDiagnostics()}, {"global", program.GetGlobalDiagnostics(t.Context())}} {
		if len(group.items) != 0 {
			t.Fatalf("complete official %s diagnostics: %+v", group.name, group.items)
		}
	}
	consumed := []string{}
	for _, source := range program.GetSourceFiles() {
		consumed = append(consumed, source.FileName())
		if contents[source.FileName()] != source.Text() {
			t.Fatal("complete original source content")
		}
	}
	slices.Sort(consumed)
	if !slices.Equal(consumed, paths) {
		t.Fatal("complete actual consumed two-file closure")
	}
	file, dts := program.GetSourceFile(r.RootFile), program.GetSourceFile(r.Declarations.Paths[0])
	if file == nil || dts == nil || !ast.IsExternalModule(file) || len(dts.Statements.Nodes) != 12 || len(packet.Payload.Types) != 3 {
		t.Fatal("actual source module/private DTS")
	}
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	arrayNode, f64Node, u32Node := dts.Statements.Nodes[9], dts.Statements.Nodes[10], dts.Statements.Nodes[11]
	if arrayNode.Kind != ast.KindInterfaceDeclaration || arrayNode.Name().Text() != "Array" || f64Node.Kind != ast.KindTypeAliasDeclaration || f64Node.Name().Text() != "f64" || u32Node.Kind != ast.KindTypeAliasDeclaration || u32Node.Name().Text() != "u32" {
		t.Fatal("pinned original Array/f64 declarations")
	}
	arraySymbol, f64Symbol, u32Symbol := check.GetSymbolAtLocation(arrayNode.Name()), check.GetSymbolAtLocation(f64Node.Name()), check.GetSymbolAtLocation(u32Node.Name())
	for _, pair := range []struct {
		symbol *ast.Symbol
		node   *ast.Node
	}{{arraySymbol, arrayNode}, {f64Symbol, f64Node}, {u32Symbol, u32Node}} {
		if pair.symbol == nil || len(pair.symbol.Declarations) != 1 || pair.symbol.Declarations[0] != pair.node {
			t.Fatal("original DTS type Symbol identity")
		}
	}
	members := arrayNode.AsInterfaceDeclaration().Members.Nodes
	if len(members) != 2 || members[0].Kind != ast.KindIndexSignature || members[1].Kind != ast.KindPropertySignature || check.GetTypeFromTypeNode(f64Node.AsTypeAliasDeclaration().Type) != check.GetNumberType() || check.GetTypeFromTypeNode(u32Node.AsTypeAliasDeclaration().Type) != check.GetNumberType() {
		t.Fatal("exact pinned indexer and existing f64=number")
	}
	indexNode := members[0]
	for i, node := range []*ast.Node{f64Node, arrayNode, u32Node} {
		fact := packet.Payload.Types[i]
		if fact.SourcePath != dts.FileName() || fact.ID != []string{"f64", "array.f64", "u32"}[i] {
			t.Fatal("whole owned type origin")
		}
		arrayFirstAnchor25(t, dts, node, fact.DeclarationRange)
		arrayFirstAnchor25(t, dts, node.Name(), fact.NameRange)
	}
	arrayFirstAnchor25(t, dts, indexNode, packet.Payload.Types[1].IndexSignatureRange)
	property := packet.Payload.ArrayLength
	lengthNode := members[1]
	if property.Kind != "property" || property.Name != "length" || property.OwnerType != "array.f64" || property.OfficialType != "number" || property.WriteType != "u32" || property.SourcePath != dts.FileName() || lengthNode.Name() == nil || lengthNode.Name().Text() != "length" || property.ID != arrayFirstBindingID25(t, dts.FileName(), property.NameRange) {
		t.Fatal("whole pinned Array length property origin")
	}
	arrayFirstAnchor25(t, dts, lengthNode, property.DeclarationRange)
	arrayFirstAnchor25(t, dts, lengthNode.Name(), property.NameRange)
	arrayFirstAnchor25(t, dts, lengthNode.AsPropertySignatureDeclaration().Type, property.TypeRange)
	lengthSymbol := check.GetSymbolAtLocation(lengthNode.Name())
	if lengthSymbol == nil || len(lengthSymbol.Declarations) != 1 || lengthSymbol.Declarations[0] != lengthNode || check.GetTypeFromTypeNode(lengthNode.AsPropertySignatureDeclaration().Type) != check.GetNumberType() {
		t.Fatal("actual original length property Symbol and Number annotation")
	}

	if len(file.Statements.Nodes) != len(packet.Payload.Functions) {
		t.Fatal("complete original function table")
	}
	owners := map[string]*ast.Symbol{}
	literalSites := map[string]bool{}
	for i, fn := range packet.Payload.Functions {
		scope := &arrayOfficialScope25{file: file, check: check, arraySymbol: arraySymbol, scalars: map[string]*ast.Symbol{"f64": f64Symbol, "u32": u32Symbol}, indexNode: indexNode, lengthNode: lengthNode, property: property, symbols: map[string]*ast.Symbol{}, types: map[string]string{}, sites: literalSites}
		owners[fn.ID] = arrayFirstCheckFunction25(t, scope, file.Statements.Nodes[i], fn)
	}
	if file.AsNode().Symbol() == nil {
		t.Fatal("actual module Symbol")
	}
	exports := check.GetExportsOfModule(file.AsNode().Symbol())
	if len(exports) != len(packet.Payload.Exports) {
		t.Fatal("complete actual direct exports")
	}
	for _, fact := range packet.Payload.Exports {
		count := 0
		for _, exported := range exports {
			actual := owners[fact.FunctionID]
			if exported != nil && actual != nil && exported.Name == fact.Name && exported.Flags&ast.SymbolFlagsAlias == 0 && len(exported.Declarations) == 1 && (exported == actual || actual.ExportSymbol == exported) {
				count++
			}
		}
		if count != 1 {
			t.Fatal("original direct export/definition/binding join")
		}
	}
	t.Logf("TSFG_ARRAY_FACTS_V2_OFFICIAL_PASS functions=%d params_locals_mutations_property_ranges=complete allocationSites=%d diagnostics=4x0 fullClosure=%v", len(owners), len(literalSites), consumed)
}
