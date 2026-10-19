// SPDX-License-Identifier: Apache-2.0
// Local and parameter view facts retain real symbols, never borrow decisions.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"strings"
)

func (p *arrayFactsProjection) viewAnnotation(node *ast.Node) *ast.Node {
	if node == nil || node.Kind != ast.KindTypeReference {
		return nil
	}
	annotation := node.AsTypeReferenceNode()
	if annotation.TypeName == nil || annotation.TypeName.Kind != ast.KindIdentifier || p.check.GetSymbolAtLocation(annotation.TypeName) != p.view.symbol || annotation.TypeArguments == nil || len(annotation.TypeArguments.Nodes) != 1 || !p.f64Annotation(annotation.TypeArguments.Nodes[0]) || !p.viewType(p.check.GetTypeFromTypeNode(node), p.check.GetNumberType()) {
		return nil
	}
	return annotation.TypeArguments.Nodes[0]
}

func (p *arrayFactsProjection) viewReference(node *ast.Node, state *arrayFactsFunctionState) (*arrayFactsValue, error) {
	if node == nil || node.Kind != ast.KindIdentifier {
		return nil, arrayFactsCapability(p.file, node, "direct prior readonly view binding required")
	}
	symbol := p.check.GetSymbolAtLocation(node)
	binding, ok := state.bindings[symbol]
	if parameter, found := state.parameters[symbol]; found && parameter.DeclaredType == "readspan.f64" {
		binding.ID, binding.DeclaredType, ok = parameter.ID, "readspan.f64", true
	}
	if !ok || binding.DeclaredType != "readspan.f64" || !p.viewType(p.check.GetTypeAtLocation(node), p.check.GetNumberType()) {
		return nil, arrayFactsCapability(p.file, node, "actual same-function prior ReadSpan symbol/use type required")
	}
	return &arrayFactsValue{Kind: "reference", Binding: binding.ID, FlowType: "readspan.f64", Range: expressionOffset(p.file, node)}, nil
}

func (p *arrayFactsProjection) viewCreate(node *ast.Node, state *arrayFactsFunctionState) (*arrayFactsValue, error) {
	if node == nil || node.Kind != ast.KindCallExpression {
		return nil, arrayFactsCapability(p.file, node, "actual pinned readSpan intrinsic call required")
	}
	call := node.AsCallExpression()
	if call.Expression == nil || call.Expression.Kind != ast.KindIdentifier || p.check.GetSymbolAtLocation(call.Expression) != p.view.intrinsicSymbol || call.QuestionDotToken != nil || call.TypeArguments != nil || call.Arguments == nil || len(call.Arguments.Nodes) != 3 {
		return nil, arrayFactsCapability(p.file, node, "actual unshadowed inferred readSpan intrinsic and three arguments required")
	}
	resolved := p.check.GetResolvedSignature(node)
	if resolved == nil || resolved.Declaration() != p.view.intrinsicNode || len(resolved.Parameters()) != 3 || resolved.MinArgumentCount() != 3 || len(resolved.TypeParameters()) != 0 || resolved.ThisParameter() != nil || resolved.HasRestParameter() || !p.viewType(p.check.GetReturnTypeOfSignature(resolved), p.check.GetNumberType()) || !p.viewType(p.check.GetTypeAtLocation(node), p.check.GetNumberType()) {
		return nil, arrayFactsCapability(p.file, node, "actual instantiated required readSpan signature/return required")
	}
	for i, parameter := range resolved.Parameters() {
		original := p.view.intrinsicNode.AsFunctionDeclaration().Parameters.Nodes[i]
		// Instantiation creates transient Symbols but preserves original declarations.
		if parameter == nil || len(parameter.Declarations) != 1 || parameter.Declarations[0] != original || parameter.Name != original.Name().Text() {
			return nil, arrayFactsCapability(p.file, node, "resolved intrinsic parameter joins original declaration/order")
		}
		typ := p.check.GetTypeOfSymbol(parameter)
		if i == 0 && !p.arrayType(typ, false) || i > 0 && typ != p.check.GetNumberType() || !p.check.IsTypeAssignableTo(p.check.GetTypeAtLocation(call.Arguments.Nodes[i]), typ) {
			return nil, arrayFactsCapability(p.file, call.Arguments.Nodes[i], "actual instantiated intrinsic argument type/assignability required")
		}
	}
	array, err := p.reference(call.Arguments.Nodes[0], state)
	if err != nil {
		return nil, err
	}
	start, err := p.viewEndpoint(call.Arguments.Nodes[1], state)
	if err != nil {
		return nil, err
	}
	length, err := p.viewEndpoint(call.Arguments.Nodes[2], state)
	if err != nil {
		return nil, err
	}
	calleeRange := expressionOffset(p.file, call.Expression)
	// Endpoints remain checked source facts; no evaluation or Array-length invalidation.
	return &arrayFactsValue{Kind: "readSpanCreate", Type: "readspan.f64", Range: expressionOffset(p.file, node), Intrinsic: p.view.intrinsicID, CalleeRange: &calleeRange, Array: array, Start: start, Length: length}, nil
}

func (p *arrayFactsProjection) viewEndpoint(node *ast.Node, state *arrayFactsFunctionState) (*arrayFactsValue, error) {
	value, err := p.unsigned(node, true, state)
	if err == nil && value.Kind == "lengthLiteral" {
		value.Kind = "u32Literal"
	}
	return value, err
}

func (p *arrayFactsProjection) viewLocal(statement, node *ast.Node, symbol *ast.Symbol, state *arrayFactsFunctionState) (arrayFactsBinding, arrayFactsStatement, error) {
	var binding arrayFactsBinding
	var result arrayFactsStatement
	local := node.AsVariableDeclaration()
	id, err := p.bindingID(node.Name())
	if err != nil {
		return binding, result, err
	}
	binding = arrayFactsBinding{ID: id, Name: strings.Clone(node.Name().Text()), Kind: "const", DeclaredType: "readspan.f64", Writable: false, DeclarationRange: expressionOffset(p.file, node), NameRange: expressionOffset(p.file, node.Name())}
	if local.Type != nil {
		elementNode := p.viewAnnotation(local.Type)
		if elementNode == nil {
			return binding, result, arrayFactsCapability(p.file, local.Type, "actual pinned readonly ReadSpan<f64> annotation required")
		}
		full, element := expressionOffset(p.file, local.Type), expressionOffset(p.file, elementNode)
		binding.AnnotationRange, binding.ElementAnnotationRange = &full, &element
	}
	var value *arrayFactsValue
	if local.Initializer.Kind == ast.KindCallExpression {
		call := local.Initializer.AsCallExpression()
		if call.Expression != nil && call.Expression.Kind == ast.KindIdentifier && p.check.GetSymbolAtLocation(call.Expression) == p.view.intrinsicSymbol {
			value, err = p.viewCreate(local.Initializer, state)
		} else {
			value, err = p.call(local.Initializer, state)
			if err == nil && value.Type != "readspan.f64" {
				err = arrayFactsCapability(p.file, local.Initializer, "ordinary initializer call must return a nominal ReadSpan")
			}
		}
	} else {
		value, err = p.viewReference(local.Initializer, state)
	}
	if err != nil {
		return binding, result, err
	}
	if !p.check.IsTypeAssignableTo(p.check.GetTypeAtLocation(local.Initializer), p.check.GetTypeOfSymbol(symbol)) {
		return binding, result, arrayFactsCapability(p.file, local.Initializer, "official readonly view initializer assignment required")
	}
	state.bindings[symbol], state.names[binding.Name] = binding, true
	return binding, arrayFactsStatement{Kind: "variable", Range: expressionOffset(p.file, statement), Binding: id, Initializer: value}, nil
}

func (p *arrayFactsProjection) viewRead(node *ast.Node, state *arrayFactsFunctionState) (*arrayFactsValue, error) {
	if node == nil || node.Kind != ast.KindElementAccessExpression {
		return nil, arrayFactsCapability(p.file, node, "ordinary checked ReadSpan read required")
	}
	access := node.AsElementAccessExpression()
	if access.QuestionDotToken != nil {
		return nil, arrayFactsCapability(p.file, node, "ordinary ReadSpan element access required")
	}
	view, err := p.viewReference(access.Expression, state)
	if err != nil {
		return nil, err
	}
	index, err := p.unsigned(access.ArgumentExpression, false, state)
	if err != nil {
		return nil, err
	}
	if p.check.GetTypeAtLocation(node) != p.check.GetNumberType() {
		return nil, arrayFactsCapability(p.file, node, "official readonly f64 element result required")
	}
	return &arrayFactsValue{Kind: "checkedReadSpanAccess", Type: "f64", Range: expressionOffset(p.file, node), View: view, Index: index}, nil
}

func (p *arrayFactsProjection) viewLength(node *ast.Node, state *arrayFactsFunctionState) (*arrayFactsValue, error) {
	access := node.AsPropertyAccessExpression()
	if access.QuestionDotToken != nil || node.Name() == nil || node.Name().Kind != ast.KindIdentifier || node.Name().Text() != "length" {
		return nil, arrayFactsCapability(p.file, node, "ordinary readonly ReadSpan.length use required")
	}
	view, err := p.viewReference(access.Expression, state)
	if err != nil {
		return nil, err
	}
	property := p.check.GetSymbolAtLocation(node.Name())
	if property == nil || property != p.check.GetPropertyOfType(p.check.GetTypeAtLocation(access.Expression), "length") || len(property.Declarations) != 1 || property.Declarations[0] != p.view.lengthNode || p.check.GetTypeOfSymbolAtLocation(property, node.Name()) != p.check.GetNumberType() || p.check.GetTypeAtLocation(node) != p.check.GetNumberType() {
		return nil, arrayFactsCapability(p.file, node, "actual pinned readonly length property/use origin required")
	}
	name := expressionOffset(p.file, node.Name())
	return &arrayFactsValue{Kind: "readSpanLength", Type: "u32", Range: expressionOffset(p.file, node), View: view, Property: p.view.lengthID, NameRange: &name}, nil
}

func (p *arrayFactsProjection) viewUse(statement *ast.Node, state *arrayFactsFunctionState) (arrayFactsStatement, error) {
	node := statement.AsExpressionStatement().Expression
	var value *arrayFactsValue
	var err error
	if node.Kind == ast.KindElementAccessExpression {
		value, err = p.viewRead(node, state)
	} else {
		value, err = p.viewLength(node, state)
	}
	if err != nil {
		return arrayFactsStatement{}, err
	}
	return arrayFactsStatement{Kind: "readSpanUse", Range: expressionOffset(p.file, statement), Value: value}, nil
}
