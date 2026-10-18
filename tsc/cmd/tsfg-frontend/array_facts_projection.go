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
	ctx         context.Context
	file        *ast.SourceFile
	check       *checker.Checker
	arraySymbol *ast.Symbol
	f64Symbol   *ast.Symbol
	indexNode   *ast.Node
}

func (p *arrayFactsProjection) f64Annotation(node *ast.Node) bool {
	if node == nil || node.Kind != ast.KindTypeReference {
		return false
	}
	ref := node.AsTypeReferenceNode()
	return ref.TypeName != nil && ref.TypeName.Kind == ast.KindIdentifier && ref.TypeName.Text() == "f64" && ref.TypeArguments == nil && p.check.GetSymbolAtLocation(ref.TypeName) == p.f64Symbol && p.check.GetTypeFromTypeNode(node) == p.check.GetNumberType()
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
		if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			continue
		}
		body := node.Body().AsBlock().Statements.Nodes
		if len(body) > 18 {
			return arrayFactsError("limit", "const declarations per function", nil)
		}
		for _, statement := range body {
			if statement.Kind != ast.KindVariableStatement {
				continue
			}
			for _, decl := range statement.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
				init := decl.AsVariableDeclaration().Initializer
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
	if dts == nil || len(dts.Statements.Nodes) != 11 {
		return empty, arrayFactsError("identity", "actual pinned declarations", nil)
	}
	check, release := program.GetTypeCheckerForFileExclusive(ctx, file)
	defer release()
	arrayNode, f64Node := dts.Statements.Nodes[9], dts.Statements.Nodes[10]
	if arrayNode.Kind != ast.KindInterfaceDeclaration || arrayNode.Name() == nil || arrayNode.Name().Text() != "Array" || f64Node.Kind != ast.KindTypeAliasDeclaration || f64Node.Name() == nil || f64Node.Name().Text() != "f64" {
		return empty, arrayFactsError("identity", "actual Array/f64 declaration kinds", nil)
	}
	arraySymbol, f64Symbol := check.GetSymbolAtLocation(arrayNode.Name()), check.GetSymbolAtLocation(f64Node.Name())
	for _, pair := range []struct {
		symbol *ast.Symbol
		node   *ast.Node
	}{{arraySymbol, arrayNode}, {f64Symbol, f64Node}} {
		if pair.symbol == nil || len(pair.symbol.Declarations) != 1 || pair.symbol.Declarations[0] != pair.node || ast.GetSourceFileOfNode(pair.node) != dts {
			return empty, arrayFactsError("identity", "unique pinned type binding", nil)
		}
	}
	members := arrayNode.AsInterfaceDeclaration().Members.Nodes
	if len(members) != 2 || members[0].Kind != ast.KindIndexSignature || check.GetTypeFromTypeNode(f64Node.AsTypeAliasDeclaration().Type) != check.GetNumberType() {
		return empty, arrayFactsError("identity", "actual numeric indexer/f64 alias", nil)
	}
	p := arrayFactsProjection{ctx: ctx, file: file, check: check, arraySymbol: arraySymbol, f64Symbol: f64Symbol, indexNode: members[0]}
	indexRange := expressionOffset(dts, members[0])
	payload := arrayFactsPayload{SourceRoot: input.SourceRoot, RootFile: input.RootFile, PositionUnit: "utf16", Config: input.Config, Declarations: arrayFactsDeclarations{Version: input.Declarations.Version, Paths: slices.Clone(input.Declarations.Paths)}, Files: []arrayFactsOwnedFile{}, Types: []arrayFactsType{{ID: "f64", Kind: "scalar", SourcePath: dts.FileName(), DeclarationRange: expressionOffset(dts, f64Node), NameRange: expressionOffset(dts, f64Node.Name())}, {ID: "array.f64", Kind: "array", ElementType: "f64", SourcePath: dts.FileName(), DeclarationRange: expressionOffset(dts, arrayNode), NameRange: expressionOffset(dts, arrayNode.Name()), IndexSignatureRange: &indexRange}}, Functions: []arrayFactsFunction{}, Exports: []arrayFactsExport{}, Diagnostics: []diagnostic{}}
	for _, entry := range input.Files {
		actual := program.GetSourceFile(entry.Path)
		if actual == nil || actual.Text() != entry.Content {
			return empty, arrayFactsError("input", "actual consumed source content", nil)
		}
		payload.Files = append(payload.Files, arrayFactsOwnedFile{Path: entry.Path, Content: entry.Content, SHA256: entry.SHA256, ByteLength: len(entry.Content), UTF16Length: actual.GetPositionMap().UTF8ToUTF16(len(entry.Content))})
	}
	owners := map[*ast.Symbol]string{}
	names := map[string]bool{}
	for _, node := range file.Statements.Nodes {
		if err := ctx.Err(); err != nil {
			return empty, arrayFactsError("canceled", "projection", err)
		}
		fn, symbol, err := p.function(node)
		if err != nil {
			return empty, err
		}
		if names[fn.Name] || owners[symbol] != "" {
			return empty, arrayFactsCapability(file, node, "unique function declaration owner")
		}
		names[fn.Name], owners[symbol] = true, fn.ID
		payload.Functions = append(payload.Functions, fn)
	}
	module := file.AsNode().Symbol()
	if module == nil {
		return empty, arrayFactsCapability(file, file.AsNode(), "actual external module symbol required")
	}
	exports := check.GetExportsOfModule(module)
	if len(exports) != len(owners) {
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
		if id == "" || seen[id] || exported != actual && actual.ExportSymbol != exported {
			return empty, arrayFactsCapability(file, node, "export joins its actual owned declaration")
		}
		seen[id] = true
		payload.Exports = append(payload.Exports, arrayFactsExport{Name: strings.Clone(exported.Name), FunctionID: id})
	}
	slices.SortFunc(payload.Exports, func(a, b arrayFactsExport) int { return strings.Compare(a.Name, b.Name) })
	return payload, nil
}
