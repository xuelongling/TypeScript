// SPDX-License-Identifier: Apache-2.0
// Official Program/type ownership is consumed before constructing the private snapshot.
package main

import (
	"context"
	"slices"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

type arrayFactsProjection struct {
	view        arrayFactsViewOrigin
	headers     map[*ast.Symbol]*arrayFactsHeader
	ctx         context.Context
	file        *ast.SourceFile
	check       *checker.Checker
	arraySymbol *ast.Symbol
	f64Symbol   *ast.Symbol
	u32Symbol   *ast.Symbol
	lengthNode  *ast.Node
	lengthID    string
	indexNode   *ast.Node
}

func (p *arrayFactsProjection) scalarAnnotation(node *ast.Node) string {
	if node == nil || node.Kind != ast.KindTypeReference {
		return ""
	}
	ref := node.AsTypeReferenceNode()
	if ref.TypeName == nil || ref.TypeName.Kind != ast.KindIdentifier || ref.TypeArguments != nil || p.check.GetTypeFromTypeNode(node) != p.check.GetNumberType() {
		return ""
	}
	symbol := p.check.GetSymbolAtLocation(ref.TypeName)
	if ref.TypeName.Text() == "f64" && symbol == p.f64Symbol {
		return "f64"
	}
	if ref.TypeName.Text() == "u32" && symbol == p.u32Symbol {
		return "u32"
	}
	return ""
}
func (p *arrayFactsProjection) f64Annotation(node *ast.Node) bool {
	return p.scalarAnnotation(node) == "f64"
}
func (p *arrayFactsProjection) arrayType(typ *checker.Type, empty bool) bool {
	if typ == nil || !p.check.IsArrayType(typ) || checker.IsTupleType(typ) || typ.Target().Symbol() != p.arraySymbol {
		return false
	}
	args := p.check.GetTypeArguments(typ)
	if len(args) != 1 || args[0] != p.check.GetNumberType() && !(empty && args[0].Flags() == checker.TypeFlagsNever) {
		return false
	}
	info := p.check.GetIndexInfoOfType(typ, p.check.GetNumberType())
	return info != nil && info.KeyType() == p.check.GetNumberType() && info.ValueType() == args[0] && !info.IsReadonly() && info.Declaration() == p.indexNode
}
func arrayFactsSourceCounts(file *ast.SourceFile) error {
	if len(file.Statements.Nodes) > 16 {
		return arrayFactsError("limit", "module functions", nil)
	}
	total := 0
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration {
			continue
		}
		decl := node.AsFunctionDeclaration()
		if decl.Parameters != nil && len(decl.Parameters.Nodes) > 8 {
			return arrayFactsError("limit", "required parameter nodes per function", nil)
		}
		if decl.Body == nil || decl.Body.Kind != ast.KindBlock {
			continue
		}
		calls := 0
		var countCalls func(*ast.Node) bool
		countCalls = func(node *ast.Node) bool {
			if node.Kind == ast.KindCallExpression {
				calls++
			}
			if calls > 32 {
				return true
			}
			return node.ForEachChild(countCalls)
		}
		decl.Body.ForEachChild(countCalls)
		if calls > 32 {
			return arrayFactsError("limit", "raw call nodes per function", nil)
		}
		body := decl.Body.AsBlock().Statements.Nodes
		if len(body) > 50 {
			return arrayFactsError("limit", "raw body statements per function", nil)
		}
		locals, mutations := 0, 0
		for _, statement := range body {
			if statement.Kind == ast.KindExpressionStatement {
				mutations++
				if mutations > 32 {
					return arrayFactsError("limit", "raw effect expression statements per function", nil)
				}
			}
			if statement.Kind != ast.KindVariableStatement {
				continue
			}
			locals++
			if locals > 17 {
				return arrayFactsError("limit", "raw local statements per function", nil)
			}
			for _, local := range statement.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
				init := local.AsVariableDeclaration().Initializer
				if init == nil || init.Kind != ast.KindArrayLiteralExpression {
					continue
				}
				count := len(init.AsArrayLiteralExpression().Elements.Nodes)
				if count > 256 || total > 4096-count {
					return arrayFactsError("limit", "dense literal elements", nil)
				}
				total += count
			}
		}
	}
	return nil
}
func projectArrayFacts(ctx context.Context, r request, input arrayFactsInput) (arrayFactsPayload, error) {
	var empty arrayFactsPayload
	if err := ctx.Err(); err != nil {
		return empty, arrayFactsError("canceled", "before Program creation", err)
	}
	program, file, consumed, failure := newOwnedProgram(r)
	if failure != nil {
		return empty, arrayFactsDiagnostics("input", "official Program source boundary", failure.Diagnostics)
	}
	if err := arrayFactsOfficialDiagnostics(ctx, program, r.RootFile); err != nil {
		return empty, err
	}
	if len(consumed) != 2 {
		return empty, arrayFactsError("input", "complete consumed source closure", nil)
	}
	for i, name := range consumed {
		if name != input.Files[i].Path {
			return empty, arrayFactsError("input", "actual consumed source identity", nil)
		}
	}
	if err := arrayFactsSourceCounts(file); err != nil {
		return empty, err
	}
	if !ast.IsExternalModule(file) || len(file.Statements.Nodes) == 0 {
		return empty, arrayFactsCapability(file, file.AsNode(), "explicit nonempty function module required")
	}
	dts := program.GetSourceFile(input.Declarations.Paths[0])
	if dts == nil || len(dts.Statements.Nodes) != 14 {
		return empty, arrayFactsError("identity", "actual pinned declarations", nil)
	}
	check, release := program.GetTypeCheckerForFileExclusive(ctx, file)
	defer release()
	origin, err := readArrayFactsOrigin(dts, check)
	if err != nil {
		return empty, err
	}
	p := arrayFactsProjection{ctx: ctx, file: file, check: check, arraySymbol: origin.arraySymbol, f64Symbol: origin.f64Symbol, u32Symbol: origin.u32Symbol, indexNode: origin.indexNode, lengthNode: origin.lengthNode, lengthID: origin.lengthID}
	if err := p.readSpanOrigin(dts); err != nil {
		return empty, err
	}
	origin.types = append(origin.types, p.view.typeFact)
	payload := arrayFactsPayload{ReadSpanLength: p.view.lengthFact, ReadSpanIntrinsic: p.view.intrinsicFact, SourceRoot: input.SourceRoot, RootFile: input.RootFile, PositionUnit: "utf16", Config: input.Config, Declarations: arrayFactsDeclarations{Version: input.Declarations.Version, Paths: slices.Clone(input.Declarations.Paths)}, Files: []arrayFactsOwnedFile{}, Types: origin.types, ArrayLength: origin.length, Functions: []arrayFactsFunction{}, Exports: []arrayFactsExport{}, Diagnostics: []diagnostic{}}
	for _, entry := range input.Files {
		actual := program.GetSourceFile(entry.Path)
		if actual == nil || actual.Text() != entry.Content {
			return empty, arrayFactsError("input", "actual consumed source content", nil)
		}
		payload.Files = append(payload.Files, arrayFactsOwnedFile{Path: entry.Path, Content: entry.Content, SHA256: entry.SHA256, ByteLength: len(entry.Content), UTF16Length: actual.GetPositionMap().UTF8ToUTF16(len(entry.Content))})
	}
	owners := map[*ast.Symbol]string{}
	names := map[string]bool{}
	p.headers = map[*ast.Symbol]*arrayFactsHeader{}
	headers := []*arrayFactsHeader{}
	expectedExports := 0
	for _, node := range file.Statements.Nodes {
		if err := ctx.Err(); err != nil {
			return empty, arrayFactsError("canceled", "function header inventory", err)
		}
		header, symbol, err := p.functionHeader(node)
		if err != nil {
			return empty, err
		}
		fn := header.function
		if names[fn.Name] || owners[symbol] != "" {
			return empty, arrayFactsCapability(file, node, "unique function declaration owner")
		}
		names[fn.Name], owners[symbol], p.headers[symbol] = true, fn.ID, header
		headers = append(headers, header)
		if node.ModifierFlags() == ast.ModifierFlagsExport {
			expectedExports++
		}
	}
	for _, header := range headers {
		fn, err := p.function(header)
		if err != nil {
			return empty, err
		}
		payload.Functions = append(payload.Functions, fn)
	}
	if err := p.acyclic(headers); err != nil {
		return empty, err
	}
	module := file.AsNode().Symbol()
	if module == nil {
		return empty, arrayFactsCapability(file, file.AsNode(), "actual external module symbol required")
	}
	exports := check.GetExportsOfModule(module)
	if expectedExports == 0 || len(exports) != expectedExports {
		return empty, arrayFactsCapability(file, file.AsNode(), "complete direct function exports required")
	}
	seen := map[string]bool{}
	for _, exported := range exports {
		if exported == nil || exported.Flags&ast.SymbolFlagsAlias != 0 || len(exported.Declarations) != 1 || ast.GetSourceFileOfNode(exported.Declarations[0]) != file {
			return empty, arrayFactsCapability(file, file.AsNode(), "direct unambiguous exports required")
		}
		node := exported.Declarations[0]
		if node.Name() == nil {
			return empty, arrayFactsCapability(file, node, "exported original function name required")
		}
		actual := check.GetSymbolAtLocation(node.Name())
		id := owners[actual]
		if id == "" || seen[id] || node.ModifierFlags() != ast.ModifierFlagsExport || exported != actual && actual.ExportSymbol != exported {
			return empty, arrayFactsCapability(file, node, "export joins its actual owned declaration")
		}
		seen[id] = true
		payload.Exports = append(payload.Exports, arrayFactsExport{Name: strings.Clone(exported.Name), FunctionID: id})
	}
	slices.SortFunc(payload.Exports, func(a, b arrayFactsExport) int { return strings.Compare(a.Name, b.Name) })
	return payload, nil
}
