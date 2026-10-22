// SPDX-License-Identifier: Apache-2.0
// Logical aliases retain declaration identity even when TS erases numeric aliases.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

func genericSpan(node *ast.Node) *span {
	v := expressionOffset(ast.GetSourceFileOfNode(node), node)
	return &v
}
func (p *arrayGenericProjection) fail(node *ast.Node, detail string) error {
	return arrayFactsCapability(ast.GetSourceFileOfNode(node), node, detail)
}
func (p *arrayGenericProjection) addType(v arrayGenericType, actual *checker.Type) string {
	if _, ok := p.types[v.ID]; !ok {
		p.types[v.ID] = v
		p.actual[v.ID] = actual
		p.typeKeyBytes += len(v.ID)
	}
	return v.ID
}
func (p *arrayGenericProjection) typeKeyBudget(id string) error {
	if _, exists := p.types[id]; !exists && p.typeKeyBytes+len(id) > 1048576 {
		return arrayFactsError("limit", "v8 total concrete type key bytes", nil)
	}
	return nil
}
func (p *arrayGenericProjection) composite(kind, element string, actual *checker.Type) (string, error) {
	owner := p.arrayNode
	if kind == "readSpan" {
		owner = p.viewNode
	}
	if actual == nil || actual.Target().Symbol() != p.check.GetSymbolAtLocation(owner.Name()) || checker.IsTupleType(actual) {
		return "", p.fail(owner, "original generic owner identity")
	}
	args := p.check.GetTypeArguments(actual)
	info := p.check.GetIndexInfoOfType(actual, p.check.GetNumberType())
	index := owner.AsInterfaceDeclaration().Members.Nodes[0]
	if len(args) != 1 || info == nil || info.Declaration() != index || info.ValueType() != args[0] || info.IsReadonly() != (kind == "readSpan") || !p.check.IsTypeAssignableTo(args[0], p.actual[element]) {
		return "", p.fail(owner, "original index signature and element ownership")
	}
	if len(kind)+len(element)+2 > 1048576 {
		return "", arrayFactsError("limit", "v8 expanded concrete type key bytes", nil)
	}
	id := kind + "<" + element + ">"
	if err := p.typeKeyBudget(id); err != nil {
		return "", err
	}
	return p.addType(arrayGenericType{ID: id, Kind: kind, ElementType: element, SourcePath: p.dts.FileName(), DeclarationRange: genericSpan(owner), NameRange: genericSpan(owner.Name()), IndexSignatureRange: genericSpan(index)}, actual), nil
}
func (p *arrayGenericProjection) annotation(node *ast.Node) (string, error) {
	if err := p.annotationWork(); err != nil {
		return "", err
	}
	if node == nil {
		return "", arrayFactsError("capability", "explicit type annotation required", nil)
	}
	if p.annotationDepth >= 64 {
		return "", arrayFactsError("limit", "v8 type annotation expansion work/depth", nil)
	}
	if id := p.annotationMemo[node]; id != "" {
		return id, nil
	}
	if p.annotationMemo == nil {
		p.annotationMemo = map[*ast.Node]string{}
		p.annotationAliases = map[*ast.Symbol]bool{}
	}
	p.annotationDepth++
	defer func() { p.annotationDepth-- }()
	id, err := p.annotationValue(node)
	if err == nil {
		p.annotationMemo[node] = id
	}
	return id, err
}

func (p *arrayGenericProjection) annotationValue(node *ast.Node) (string, error) {
	actual := p.check.GetTypeFromTypeNode(node)
	if node.Kind == ast.KindNumberKeyword && actual == p.check.GetNumberType() {
		return p.scalarType("f64"), nil
	}
	if node.Kind == ast.KindBooleanKeyword && actual == p.check.GetBooleanType() {
		return p.scalarType("bool"), nil
	}
	if node.Kind == ast.KindStringKeyword && actual == p.check.GetStringType() {
		return p.stringType(), nil
	}
	if node.Kind == ast.KindParenthesizedType {
		return p.annotation(node.AsParenthesizedTypeNode().Type)
	}
	if node.Kind == ast.KindUnionType {
		ids := []string{}
		for _, member := range node.AsUnionTypeNode().Types.Nodes {
			id, err := p.annotation(member)
			if err != nil {
				return "", err
			}
			ids = append(ids, id)
		}
		return p.union(ids, actual)
	}
	if actual != nil && actual.Flags()&(checker.TypeFlagsUndefined|checker.TypeFlagsNull|checker.TypeFlagsVoid) != 0 {
		id := "undefined"
		if actual.Flags()&checker.TypeFlagsNull != 0 {
			id = "null"
		}
		if actual.Flags()&checker.TypeFlagsVoid != 0 {
			id = "void"
		}
		return p.addType(arrayGenericType{ID: id, Kind: id}, actual), nil
	}
	if node.Kind == ast.KindArrayType {
		element, err := p.annotation(node.AsArrayTypeNode().ElementType)
		if err != nil {
			return "", err
		}
		return p.composite("array", element, actual)
	}
	if node.Kind == ast.KindTypeReference {
		ref := node.AsTypeReferenceNode()
		symbol := p.check.GetSymbolAtLocation(ref.TypeName)
		if symbol != nil && symbol.Flags&ast.SymbolFlagsAlias != 0 {
			symbol = p.check.GetAliasedSymbol(symbol)
		}
		if ref.TypeName.Kind == ast.KindIdentifier && (integerKind(ref.TypeName.Text()) || floatKind(ref.TypeName.Text()) || ref.TypeName.Text() == "bool") && p.scalars[symbol] == nil {
			return "", p.fail(node, "reserved scalar alias must join its pinned declaration")
		}
		if scalar := p.scalars[symbol]; scalar != nil && ref.TypeArguments == nil {
			id := scalar.Name().Text()
			return p.addType(arrayGenericType{ID: id, Name: id, Kind: "scalar", SourcePath: p.dts.FileName(), DeclarationRange: genericSpan(scalar), NameRange: genericSpan(scalar.Name())}, actual), nil
		}
		if ref.TypeArguments != nil && len(ref.TypeArguments.Nodes) == 1 && (symbol == p.check.GetSymbolAtLocation(p.arrayNode.Name()) || symbol == p.check.GetSymbolAtLocation(p.viewNode.Name())) {
			element, err := p.annotation(ref.TypeArguments.Nodes[0])
			if err != nil {
				return "", err
			}
			kind := "array"
			if symbol == p.check.GetSymbolAtLocation(p.viewNode.Name()) {
				kind = "readSpan"
			}
			return p.composite(kind, element, actual)
		}
		if id := p.classes[symbol]; id != "" && ref.TypeArguments == nil {
			return id, nil
		}
		if symbol != nil && len(symbol.Declarations) == 1 && symbol.Declarations[0].Kind == ast.KindTypeAliasDeclaration && ref.TypeArguments == nil {
			// v8's concrete type-key grammar has no back-reference alias identity.
			// Recursive classes already have nominal IDs and do not enter this path.
			if p.annotationAliases[symbol] {
				return "", p.fail(node, "recursive alias requires a finite back-reference type identity outside the v8 concrete type-key grammar")
			}
			p.annotationAliases[symbol] = true
			defer delete(p.annotationAliases, symbol)
			return p.annotation(symbol.Declarations[0].AsTypeAliasDeclaration().Type)
		}
	}
	return "", p.fail(node, "type must join its original pinned scalar, absence, union, generic owner or nominal declaration")
}
func (p *arrayGenericProjection) stringType() string {
	for _, node := range p.dts.Statements.Nodes {
		if node.Name() != nil && node.Name().Text() == "String" {
			return p.addType(arrayGenericType{ID: "string", Kind: "string", Name: "string", SourcePath: p.dts.FileName(), DeclarationRange: genericSpan(node), NameRange: genericSpan(node.Name())}, p.check.GetStringType())
		}
	}
	return ""
}

// Flow intersects the declared logical constituents with actual checker objects.
// Numeric references never acquire the expected slot's different alias name.
func (p *arrayGenericProjection) flow(actual *checker.Type, declared string) (string, error) {
	if actual == nil {
		return "", arrayFactsError("identity", "missing original flow type", nil)
	}
	if p.types[declared].Kind != "union" {
		return declared, nil
	}
	ids := []string{}
	actualMembers := []*checker.Type{actual}
	if actual.IsUnion() {
		actualMembers = actual.Types()
	}
	for _, id := range p.types[declared].Members {
		t := p.actual[id]
		for _, member := range actualMembers {
			if t != nil && p.check.IsTypeAssignableTo(member, t) {
				ids = append(ids, id)
				break
			}
		}
	}
	if len(ids) == 0 {
		return "", arrayFactsError("identity", "flow type has no declared constituent", nil)
	}
	return p.union(ids, actual)
}

func (p *arrayGenericProjection) origins() error {
	for _, node := range p.dts.Statements.Nodes {
		if node.Name() == nil {
			continue
		}
		name := node.Name().Text()
		symbol := p.check.GetSymbolAtLocation(node.Name())
		if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != node {
			return p.fail(node, "unique pinned declaration symbol required")
		}
		if node.Kind == ast.KindTypeAliasDeclaration {
			actual := p.check.GetTypeFromTypeNode(node.AsTypeAliasDeclaration().Type)
			if actual != p.check.GetNumberType() && !(name == "bool" && actual == p.check.GetBooleanType()) {
				return p.fail(node, "pinned scalar primitive proof")
			}
			p.scalars[symbol] = node
		}
		switch name {
		case "Array":
			p.arrayNode = node
		case "ReadSpan":
			p.viewNode = node
		case "readSpan":
			p.intrinsicNode = node
		}
	}
	if len(p.scalars) != 13 || p.arrayNode == nil || p.viewNode == nil || p.intrinsicNode == nil {
		return arrayFactsError("identity", "complete pinned generic declarations", nil)
	}
	for _, entry := range []struct {
		node *ast.Node
		kind string
	}{{p.arrayNode, "array"}, {p.viewNode, "readSpan"}} {
		members := entry.node.AsInterfaceDeclaration().Members.Nodes
		if len(members) != 2 || members[0].Kind != ast.KindIndexSignature || members[1].Kind != ast.KindPropertySignature {
			return p.fail(entry.node, "original generic member inventory")
		}
		length := members[1]
		id, err := arrayFactsBindingID(p.dts.FileName(), *genericSpan(length.Name()))
		if err != nil {
			return err
		}
		typ := length.AsPropertySignatureDeclaration().Type
		if entry.kind == "array" {
			p.payload.ArrayLength = arrayFactsLengthProperty{ID: id, Kind: "property", Name: "length", OwnerType: "Array<T>", OfficialType: "number", WriteType: "u32", SourcePath: p.dts.FileName(), DeclarationRange: *genericSpan(length), NameRange: *genericSpan(length.Name()), TypeRange: *genericSpan(typ)}
		} else {
			p.payload.ReadSpanLength = arrayFactsReadSpanLength{ID: id, Kind: "property", Name: "length", OwnerType: "ReadSpan<T>", OfficialType: "number", ReadType: "u32", Readonly: true, SourcePath: p.dts.FileName(), DeclarationRange: *genericSpan(length), NameRange: *genericSpan(length.Name()), TypeRange: *genericSpan(typ)}
		}
	}
	node := p.intrinsicNode
	decl := node.AsFunctionDeclaration()
	id, err := arrayFactsBindingID(p.dts.FileName(), *genericSpan(node.Name()))
	if err != nil {
		return err
	}
	params := []arrayFactsIntrinsicParameter{}
	for _, node := range decl.Parameters.Nodes {
		params = append(params, arrayFactsIntrinsicParameter{Name: node.Name().Text(), DeclarationRange: *genericSpan(node), NameRange: *genericSpan(node.Name()), TypeRange: *genericSpan(node.AsParameterDeclaration().Type)})
	}
	p.payload.ReadSpanIntrinsic = arrayFactsIntrinsic{ID: id, Kind: "intrinsic", Name: "readSpan", SourcePath: p.dts.FileName(), DeclarationRange: *genericSpan(node), NameRange: *genericSpan(node.Name()), TypeParameterRange: *genericSpan(decl.TypeParameters.Nodes[0]), Parameters: params, ReturnTypeRange: *genericSpan(decl.Type)}
	p.scalarType("u32")
	return nil
}
