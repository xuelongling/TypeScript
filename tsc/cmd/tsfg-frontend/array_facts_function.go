// SPDX-License-Identifier: Apache-2.0
// Complete finite function projection retains actual declaration/use identities and order.
package main

import (
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

func (p *arrayFactsProjection) bindingID(node *ast.Node) (string, error) {
	return arrayFactsID("tsfg.array-facts.binding.v1\x00", struct {
		SourcePath string `json:"sourcePath"`
		NameRange  span   `json:"nameRange"`
	}{p.file.FileName(), expressionOffset(p.file, node)})
}
func (p *arrayFactsProjection) reference(node *ast.Node, bindings map[*ast.Symbol]arrayFactsBinding) (*arrayFactsValue, error) {
	if node == nil || node.Kind != ast.KindIdentifier {
		return nil, arrayFactsCapability(p.file, node, "direct prior Array binding required")
	}
	binding, ok := bindings[p.check.GetSymbolAtLocation(node)]
	if !ok || !p.arrayType(p.check.GetTypeAtLocation(node), false) {
		return nil, arrayFactsCapability(p.file, node, "original prior f64 Array symbol/use type required")
	}
	return &arrayFactsValue{Kind: "reference", Binding: binding.ID, FlowType: "array.f64", Range: expressionOffset(p.file, node)}, nil
}
func (p *arrayFactsProjection) literal(node *ast.Node, declared *checker.Type) (*arrayFactsValue, error) {
	elements := node.AsArrayLiteralExpression().Elements.Nodes
	actual := p.check.GetTypeAtLocation(node)
	if !p.arrayType(actual, len(elements) == 0) || !p.check.IsTypeAssignableTo(actual, declared) {
		return nil, arrayFactsCapability(p.file, node, "official dense f64 Array literal context required")
	}
	site, err := arrayFactsID("tsfg.array-facts.allocation-site.v1\x00", struct {
		SourcePath string `json:"sourcePath"`
		Range      span   `json:"range"`
	}{p.file.FileName(), expressionOffset(p.file, node)})
	if err != nil {
		return nil, err
	}
	values := make([]arrayFactsValue, 0, len(elements))
	for _, element := range elements {
		if err := p.ctx.Err(); err != nil {
			return nil, arrayFactsError("canceled", "literal elements", err)
		}
		token, valid := floatingNode(p.file, element)
		if !valid {
			return nil, arrayFactsCapability(p.file, element, "dense finite numeric literal element required")
		}
		typ := p.check.GetTypeAtLocation(element)
		if typ == nil || typ.Flags()&checker.TypeFlagsNumberLike == 0 {
			return nil, arrayFactsCapability(p.file, element, "official numeric element type required")
		}
		bits, valid := exactFloatBits(token, "f64")
		if !valid {
			return nil, arrayFactsCapability(p.file, element, "finite f64 literal required")
		}
		values = append(values, arrayFactsValue{Kind: "numericLiteral", Type: "f64", Range: expressionOffset(p.file, element), Token: strings.Clone(token), Bits: bits})
	}
	return &arrayFactsValue{Kind: "arrayLiteral", Type: "array.f64", Range: expressionOffset(p.file, node), AllocationSite: site, Elements: &values}, nil
}
func (p *arrayFactsProjection) function(node *ast.Node) (arrayFactsFunction, *ast.Symbol, error) {
	var empty arrayFactsFunction
	bad := func(at *ast.Node, detail string) (arrayFactsFunction, *ast.Symbol, error) {
		return empty, nil, arrayFactsCapability(p.file, at, detail)
	}
	if node.Kind != ast.KindFunctionDeclaration || node.Name() == nil || node.Name().Kind != ast.KindIdentifier || node.ModifierFlags() != ast.ModifierFlagsExport {
		return bad(node, "only named ordinary exported functions")
	}
	decl := node.AsFunctionDeclaration()
	if decl.TypeParameters != nil || decl.AsteriskToken != nil || decl.FullSignature != nil || decl.Parameters == nil || len(decl.Parameters.Nodes) != 0 || decl.Body == nil || decl.Body.Kind != ast.KindBlock || !p.f64Annotation(decl.Type) {
		return bad(node, "explicit f64 zero-parameter function required")
	}
	actual := p.check.GetSymbolAtLocation(node.Name())
	if actual == nil || len(actual.Declarations) != 1 || actual.Declarations[0] != node {
		return bad(node, "unique actual function binding required")
	}
	sigs := p.check.GetSignaturesOfType(p.check.GetTypeOfSymbol(actual), checker.SignatureKindCall)
	if len(sigs) != 1 || sigs[0].Declaration() != node || len(sigs[0].Parameters()) != 0 || len(sigs[0].TypeParameters()) != 0 || p.check.GetReturnTypeOfSignature(sigs[0]) != p.check.GetNumberType() {
		return bad(node, "official zero-parameter f64 return signature required")
	}
	body := decl.Body.AsBlock().Statements.Nodes
	if len(body) < 2 || body[len(body)-1].Kind != ast.KindReturnStatement {
		return bad(decl.Body, "const Array declarations followed by one final return")
	}
	id, err := p.bindingID(node.Name())
	if err != nil {
		return empty, nil, err
	}
	fn := arrayFactsFunction{ID: id, Name: strings.Clone(node.Name().Text()), SourcePath: p.file.FileName(), DeclarationRange: expressionOffset(p.file, node), NameRange: expressionOffset(p.file, node.Name()), ReturnType: "f64", ReturnAnnotationRange: expressionOffset(p.file, decl.Type), Parameters: []string{}, Bindings: []arrayFactsBinding{}, Body: []arrayFactsStatement{}}
	bindings := map[*ast.Symbol]arrayFactsBinding{}
	for _, statement := range body[:len(body)-1] {
		if err := p.ctx.Err(); err != nil {
			return empty, nil, arrayFactsError("canceled", "const declaration", err)
		}
		if statement.Kind != ast.KindVariableStatement {
			return bad(statement, "only const Array declarations before final return")
		}
		list := statement.AsVariableStatement().DeclarationList
		locals := list.AsVariableDeclarationList().Declarations.Nodes
		if list.Flags&ast.NodeFlagsConst == 0 || len(locals) != 1 {
			return bad(statement, "one const Array declarator required")
		}
		localNode := locals[0]
		local := localNode.AsVariableDeclaration()
		if localNode.Name() == nil || localNode.Name().Kind != ast.KindIdentifier || local.Initializer == nil || local.ExclamationToken != nil {
			return bad(localNode, "initialized direct const Array binding required")
		}
		symbol := p.check.GetSymbolAtLocation(localNode.Name())
		if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != localNode || !p.arrayType(p.check.GetTypeOfSymbol(symbol), false) {
			return bad(localNode, "actual f64 Array local declaration required")
		}
		if _, exists := bindings[symbol]; exists {
			return bad(localNode, "distinct local declaration binding required")
		}
		localID, err := p.bindingID(localNode.Name())
		if err != nil {
			return empty, nil, err
		}
		binding := arrayFactsBinding{ID: localID, Name: strings.Clone(localNode.Name().Text()), Kind: "const", DeclaredType: "array.f64", Writable: false, DeclarationRange: expressionOffset(p.file, localNode), NameRange: expressionOffset(p.file, localNode.Name())}
		if local.Type != nil {
			if local.Type.Kind != ast.KindArrayType || !p.f64Annotation(local.Type.AsArrayTypeNode().ElementType) || !p.arrayType(p.check.GetTypeFromTypeNode(local.Type), false) {
				return bad(local.Type, "explicit pinned f64[] annotation required")
			}
			annotation, element := expressionOffset(p.file, local.Type), expressionOffset(p.file, local.Type.AsArrayTypeNode().ElementType)
			binding.AnnotationRange, binding.ElementAnnotationRange = &annotation, &element
		}
		var init *arrayFactsValue
		if local.Initializer.Kind == ast.KindArrayLiteralExpression {
			if local.Type == nil {
				return bad(local.Initializer, "literal root requires explicit f64[] context")
			}
			init, err = p.literal(local.Initializer, p.check.GetTypeOfSymbol(symbol))
		} else {
			init, err = p.reference(local.Initializer, bindings)
		}
		if err != nil {
			return empty, nil, err
		}
		bindings[symbol] = binding
		fn.Bindings = append(fn.Bindings, binding)
		fn.Body = append(fn.Body, arrayFactsStatement{Kind: "variable", Binding: binding.ID, Range: expressionOffset(p.file, statement), Initializer: init})
	}
	tail := body[len(body)-1]
	value := tail.AsReturnStatement().Expression
	if value == nil || value.Kind != ast.KindElementAccessExpression {
		return bad(tail, "final named Array literal-index read required")
	}
	access := value.AsElementAccessExpression()
	if access.QuestionDotToken != nil || access.ArgumentExpression == nil || access.ArgumentExpression.Kind != ast.KindNumericLiteral {
		return bad(value, "ordinary Array NumericLiteral index required")
	}
	array, err := p.reference(access.Expression, bindings)
	if err != nil {
		return empty, nil, err
	}
	index := access.ArgumentExpression
	token, valid := floatingNode(p.file, index)
	if !valid {
		return bad(index, "original Array numeric index token required")
	}
	decimal, valid := arrayFactsIndex(token)
	if !valid {
		return bad(index, "exact integral Array index domain required")
	}
	if p.check.GetTypeAtLocation(value) != p.check.GetNumberType() {
		return bad(value, "official f64 Array element result required")
	}
	read := &arrayFactsValue{Kind: "checkedElementAccess", Type: "f64", Range: expressionOffset(p.file, value), Array: array, Index: &arrayFactsValue{Kind: "indexLiteral", Type: "u32", Range: expressionOffset(p.file, index), Token: strings.Clone(token), Decimal: decimal}}
	fn.Body = append(fn.Body, arrayFactsStatement{Kind: "return", Range: expressionOffset(p.file, tail), Value: read})
	return fn, actual, nil
}
