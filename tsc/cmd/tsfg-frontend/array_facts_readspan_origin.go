// SPDX-License-Identifier: Apache-2.0
// Readonly view and intrinsic origins are owned only after real declaration joins.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

type arrayFactsViewOrigin struct {
	symbol, intrinsicSymbol              *ast.Symbol
	indexNode, lengthNode, intrinsicNode *ast.Node
	lengthID, intrinsicID                string
	typeFact                             arrayFactsType
	lengthFact                           arrayFactsReadSpanLength
	intrinsicFact                        arrayFactsIntrinsic
}

func (p *arrayFactsProjection) viewType(typ, element *checker.Type) bool {
	if typ == nil || typ.Flags()&checker.TypeFlagsObject == 0 || p.check.IsArrayType(typ) || checker.IsTupleType(typ) || typ.Target() == nil || typ.Target().Symbol() != p.view.symbol {
		return false
	}
	args := p.check.GetTypeArguments(typ)
	if len(args) != 1 || args[0] != element {
		return false
	}
	info := p.check.GetIndexInfoOfType(typ, p.check.GetNumberType())
	property := p.check.GetPropertyOfType(typ, "length")
	return info != nil && info.KeyType() == p.check.GetNumberType() && info.ValueType() == element && info.IsReadonly() && info.Declaration() == p.view.indexNode && property != nil && len(property.Declarations) == 1 && property.Declarations[0] == p.view.lengthNode && p.check.GetTypeOfSymbol(property) == p.check.GetNumberType()
}

func (p *arrayFactsProjection) readSpanOrigin(dts *ast.SourceFile) error {
	bad := func(detail string) error { return arrayFactsError("identity", detail, nil) }
	viewNode, intrinsicNode := dts.Statements.Nodes[12], dts.Statements.Nodes[13]
	if viewNode.Kind != ast.KindInterfaceDeclaration || viewNode.Name() == nil || viewNode.Name().Text() != "ReadSpan" || intrinsicNode.Kind != ast.KindFunctionDeclaration || intrinsicNode.Name() == nil || intrinsicNode.Name().Text() != "readSpan" {
		return bad("actual pinned ReadSpan/intrinsic declarations")
	}
	original := func(node *ast.Node) *ast.Symbol {
		if node == nil || node.Name() == nil || ast.GetSourceFileOfNode(node) != dts {
			return nil
		}
		symbol := p.check.GetSymbolAtLocation(node.Name())
		if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != node {
			return nil
		}
		return symbol
	}
	viewSymbol, intrinsicSymbol := original(viewNode), original(intrinsicNode)
	if viewSymbol == nil || intrinsicSymbol == nil {
		return bad("unique pinned ReadSpan/intrinsic symbols")
	}
	view := viewNode.AsInterfaceDeclaration()
	if view.TypeParameters == nil || len(view.TypeParameters.Nodes) != 1 || len(view.Members.Nodes) != 2 || view.Members.Nodes[0].Kind != ast.KindIndexSignature || view.Members.Nodes[1].Kind != ast.KindPropertySignature {
		return bad("complete generic readonly ReadSpan members")
	}
	indexNode, lengthNode := view.Members.Nodes[0], view.Members.Nodes[1]
	length := lengthNode.AsPropertySignatureDeclaration()
	if indexNode.ModifierFlags() != ast.ModifierFlagsReadonly || lengthNode.ModifierFlags() != ast.ModifierFlagsReadonly || lengthNode.Name() == nil || lengthNode.Name().Kind != ast.KindIdentifier || lengthNode.Name().Text() != "length" || length.PostfixToken != nil || length.Initializer != nil || p.scalarAnnotation(length.Type) != "u32" || original(lengthNode) == nil {
		return bad("actual readonly nonoptional ReadSpan length:u32")
	}
	viewT := original(view.TypeParameters.Nodes[0])
	if viewT == nil || p.check.GetTypeFromTypeNode(indexNode.AsIndexSignatureDeclaration().Type) != p.check.GetDeclaredTypeOfSymbol(viewT) {
		return bad("readonly indexer joins original interface type parameter")
	}
	lengthID, err := arrayFactsBindingID(dts.FileName(), expressionOffset(dts, lengthNode.Name()))
	if err != nil {
		return err
	}
	intrinsicID, err := arrayFactsBindingID(dts.FileName(), expressionOffset(dts, intrinsicNode.Name()))
	if err != nil {
		return err
	}
	p.view = arrayFactsViewOrigin{symbol: viewSymbol, intrinsicSymbol: intrinsicSymbol, indexNode: indexNode, lengthNode: lengthNode, intrinsicNode: intrinsicNode, lengthID: lengthID, intrinsicID: intrinsicID}
	fn := intrinsicNode.AsFunctionDeclaration()
	signatures := p.check.GetSignaturesOfType(p.check.GetTypeOfSymbol(intrinsicSymbol), checker.SignatureKindCall)
	if fn.Body != nil || fn.TypeParameters == nil || len(fn.TypeParameters.Nodes) != 1 || fn.Parameters == nil || len(fn.Parameters.Nodes) != 3 || len(signatures) != 1 {
		return bad("complete original generic readSpan signature")
	}
	signature := signatures[0]
	if signature.Declaration() != intrinsicNode || len(signature.TypeParameters()) != 1 || len(signature.Parameters()) != 3 || signature.MinArgumentCount() != 3 || signature.ThisParameter() != nil || signature.HasRestParameter() {
		return bad("sole required original generic readSpan signature")
	}
	typeSymbol := original(fn.TypeParameters.Nodes[0])
	generic := signature.TypeParameters()[0]
	if typeSymbol == nil || generic.Symbol() != typeSymbol || p.check.GetDeclaredTypeOfSymbol(typeSymbol) != generic || generic == p.check.GetDeclaredTypeOfSymbol(viewT) {
		return bad("intrinsic scoped generic parameter identity")
	}
	parameters := make([]arrayFactsIntrinsicParameter, 0, 3)
	for i, node := range fn.Parameters.Nodes {
		parameter := node.AsParameterDeclaration()
		if original(node) != signature.Parameters()[i] || node.Name().Text() != []string{"owner", "start", "length"}[i] || node.ModifierFlags() != 0 || parameter.Initializer != nil || parameter.QuestionToken != nil || parameter.DotDotDotToken != nil || parameter.Type == nil {
			return bad("required original intrinsic parameter order")
		}
		if i == 0 {
			if parameter.Type.Kind != ast.KindArrayType {
				return bad("intrinsic owner T[] annotation")
			}
			element := parameter.Type.AsArrayTypeNode().ElementType
			if element.Kind != ast.KindTypeReference || p.check.GetSymbolAtLocation(element.AsTypeReferenceNode().TypeName) != typeSymbol || p.check.GetTypeFromTypeNode(element) != generic {
				return bad("intrinsic owner annotation scoped T identity")
			}
			typ := p.check.GetTypeOfSymbol(signature.Parameters()[i])
			if !p.check.IsArrayType(typ) || typ.Target().Symbol() != p.arraySymbol {
				return bad("intrinsic original Array<T> target")
			}
			arguments := p.check.GetTypeArguments(typ)
			info := p.check.GetIndexInfoOfType(typ, p.check.GetNumberType())
			if len(arguments) != 1 || arguments[0] != generic || info == nil || info.ValueType() != generic || info.IsReadonly() || info.Declaration() != p.indexNode {
				return bad("intrinsic original Array<T> element/indexer")
			}
		} else if p.scalarAnnotation(parameter.Type) != "u32" || p.check.GetTypeOfSymbol(signature.Parameters()[i]) != p.check.GetNumberType() {
			return bad("intrinsic pinned u32 parameter origins")
		}
		parameters = append(parameters, arrayFactsIntrinsicParameter{Name: node.Name().Text(), DeclarationRange: expressionOffset(dts, node), NameRange: expressionOffset(dts, node.Name()), TypeRange: expressionOffset(dts, parameter.Type)})
	}
	if fn.Type == nil || fn.Type.Kind != ast.KindTypeReference {
		return bad("intrinsic ReadSpan<T> return annotation")
	}
	result := fn.Type.AsTypeReferenceNode()
	if p.check.GetSymbolAtLocation(result.TypeName) != viewSymbol || result.TypeArguments == nil || len(result.TypeArguments.Nodes) != 1 || p.check.GetTypeFromTypeNode(result.TypeArguments.Nodes[0]) != generic || !p.viewType(p.check.GetReturnTypeOfSignature(signature), generic) {
		return bad("intrinsic actual ReadSpan<T> return origin")
	}
	indexRange := expressionOffset(dts, indexNode)
	p.view.typeFact = arrayFactsType{ID: "readspan.f64", Kind: "readSpan", ElementType: "f64", Readonly: true, SourcePath: dts.FileName(), DeclarationRange: expressionOffset(dts, viewNode), NameRange: expressionOffset(dts, viewNode.Name()), IndexSignatureRange: &indexRange}
	p.view.lengthFact = arrayFactsReadSpanLength{ID: lengthID, Kind: "property", Name: "length", OwnerType: "readspan.f64", OfficialType: "number", ReadType: "u32", Readonly: true, SourcePath: dts.FileName(), DeclarationRange: expressionOffset(dts, lengthNode), NameRange: expressionOffset(dts, lengthNode.Name()), TypeRange: expressionOffset(dts, length.Type)}
	p.view.intrinsicFact = arrayFactsIntrinsic{ID: intrinsicID, Kind: "intrinsic", Name: "readSpan", SourcePath: dts.FileName(), DeclarationRange: expressionOffset(dts, intrinsicNode), NameRange: expressionOffset(dts, intrinsicNode.Name()), TypeParameterRange: expressionOffset(dts, fn.TypeParameters.Nodes[0]), Parameters: parameters, ReturnTypeRange: expressionOffset(dts, fn.Type)}
	return nil
}
