// SPDX-License-Identifier: Apache-2.0
// Pinned declaration identity is checked once before owned Array origins are built.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

type arrayFactsOrigin struct {
	arraySymbol *ast.Symbol
	f64Symbol   *ast.Symbol
	u32Symbol   *ast.Symbol
	indexNode   *ast.Node
	lengthNode  *ast.Node
	lengthID    string
	types       []arrayFactsType
	length      arrayFactsLengthProperty
}

// The caller checks the complete pinned DTS closure/count before acquiring its
// exclusive checker. This function owns the selected declarations and origins;
// no compiler object escapes into the returned wire type/property values.
func readArrayFactsOrigin(dts *ast.SourceFile, check *checker.Checker) (arrayFactsOrigin, error) {
	var empty arrayFactsOrigin
	arrayNode, f64Node, u32Node := dts.Statements.Nodes[9], dts.Statements.Nodes[10], dts.Statements.Nodes[11]
	if arrayNode.Kind != ast.KindInterfaceDeclaration || arrayNode.Name() == nil || arrayNode.Name().Text() != "Array" || f64Node.Kind != ast.KindTypeAliasDeclaration || f64Node.Name() == nil || f64Node.Name().Text() != "f64" || u32Node.Kind != ast.KindTypeAliasDeclaration || u32Node.Name() == nil || u32Node.Name().Text() != "u32" {
		return empty, arrayFactsError("identity", "actual Array/f64 declaration kinds", nil)
	}
	arraySymbol, f64Symbol, u32Symbol := check.GetSymbolAtLocation(arrayNode.Name()), check.GetSymbolAtLocation(f64Node.Name()), check.GetSymbolAtLocation(u32Node.Name())
	for _, pair := range []struct {
		symbol *ast.Symbol
		node   *ast.Node
	}{{arraySymbol, arrayNode}, {f64Symbol, f64Node}, {u32Symbol, u32Node}} {
		if pair.symbol == nil || len(pair.symbol.Declarations) != 1 || pair.symbol.Declarations[0] != pair.node || ast.GetSourceFileOfNode(pair.node) != dts {
			return empty, arrayFactsError("identity", "unique pinned type binding", nil)
		}
	}
	members := arrayNode.AsInterfaceDeclaration().Members.Nodes
	if len(members) != 2 || members[0].Kind != ast.KindIndexSignature || members[1].Kind != ast.KindPropertySignature || check.GetTypeFromTypeNode(f64Node.AsTypeAliasDeclaration().Type) != check.GetNumberType() || check.GetTypeFromTypeNode(u32Node.AsTypeAliasDeclaration().Type) != check.GetNumberType() {
		return empty, arrayFactsError("identity", "actual numeric indexer/f64 alias", nil)
	}
	lengthNode := members[1]
	lengthDecl := lengthNode.AsPropertySignatureDeclaration()
	if lengthNode.Name() == nil {
		return empty, arrayFactsError("identity", "actual named length property", nil)
	}
	lengthSymbol := check.GetSymbolAtLocation(lengthNode.Name())
	if lengthNode.Name() == nil || lengthNode.Name().Kind != ast.KindIdentifier || lengthNode.Name().Text() != "length" || lengthNode.ModifierFlags() != 0 || lengthDecl.PostfixToken != nil || lengthDecl.Type == nil || lengthDecl.Type.Kind != ast.KindNumberKeyword || lengthDecl.Initializer != nil || lengthSymbol == nil || len(lengthSymbol.Declarations) != 1 || lengthSymbol.Declarations[0] != lengthNode || check.GetTypeFromTypeNode(lengthDecl.Type) != check.GetNumberType() {
		return empty, arrayFactsError("identity", "actual writable nonoptional length:number property", nil)
	}
	lengthID, err := arrayFactsBindingID(dts.FileName(), expressionOffset(dts, lengthNode.Name()))
	if err != nil {
		return empty, err
	}
	indexRange := expressionOffset(dts, members[0])
	return arrayFactsOrigin{
		arraySymbol: arraySymbol, f64Symbol: f64Symbol, u32Symbol: u32Symbol,
		indexNode: members[0], lengthNode: lengthNode, lengthID: lengthID,
		types:  []arrayFactsType{{ID: "f64", Kind: "scalar", SourcePath: dts.FileName(), DeclarationRange: expressionOffset(dts, f64Node), NameRange: expressionOffset(dts, f64Node.Name())}, {ID: "array.f64", Kind: "array", ElementType: "f64", SourcePath: dts.FileName(), DeclarationRange: expressionOffset(dts, arrayNode), NameRange: expressionOffset(dts, arrayNode.Name()), IndexSignatureRange: &indexRange}, {ID: "u32", Kind: "scalar", SourcePath: dts.FileName(), DeclarationRange: expressionOffset(dts, u32Node), NameRange: expressionOffset(dts, u32Node.Name())}},
		length: arrayFactsLengthProperty{ID: lengthID, Kind: "property", Name: "length", OwnerType: "array.f64", OfficialType: "number", WriteType: "u32", SourcePath: dts.FileName(), DeclarationRange: expressionOffset(dts, lengthNode), NameRange: expressionOffset(dts, lengthNode.Name()), TypeRange: expressionOffset(dts, lengthDecl.Type)},
	}, nil
}
