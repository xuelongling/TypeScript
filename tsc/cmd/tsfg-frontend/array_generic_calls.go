// SPDX-License-Identifier: Apache-2.0
// Calls retain one resolved original signature and ordered original arguments.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

func (p *arrayGenericProjection) call(node *ast.Node, state *arrayGenericState, depth int) (*arrayGenericExpression, error) {
	var callee *ast.Node
	var args []*ast.Node
	kind := "call"
	if node.Kind == ast.KindCallExpression {
		call := node.AsCallExpression()
		if call.QuestionDotToken != nil {
			return nil, p.fail(node, "ordinary call required")
		}
		callee, args = call.Expression, call.Arguments.Nodes
	} else {
		call := node.AsNewExpression()
		callee = call.Expression
		if call.Arguments != nil {
			args = call.Arguments.Nodes
		}
		kind = "new"
	}
	var receiver *arrayGenericExpression
	receiverExpected := ""
	if callee.Kind == ast.KindSuperKeyword {
		return p.superConstruct(node, state, depth)
	}
	if callee.Kind != ast.KindIdentifier && callee.Kind != ast.KindPropertyAccessExpression {
		return nil, p.fail(callee, "callee requires actual ordinary function/class binding")
	}
	calleeBinding := callee
	if callee.Kind == ast.KindPropertyAccessExpression {
		calleeBinding = callee.Name()
	}
	symbol := p.check.GetSymbolAtLocation(calleeBinding)
	if symbol != nil && symbol.Flags&ast.SymbolFlagsAlias != 0 {
		symbol = p.check.GetAliasedSymbol(symbol)
	}
	resolved := p.check.GetResolvedSignature(node)
	if symbol == nil || resolved == nil {
		return nil, p.fail(node, "resolved original signature required")
	}
	decl := resolved.Declaration()
	parameters := []string{}
	result, calleeID := "", ""
	var selected *arrayGenericSignature
	var projectedOwner *arrayGenericExpression
	if kind == "new" {
		result = p.classes[symbol]
		if result == "" {
			return nil, p.fail(node, "new must target a closed original nominal class")
		}
		calleeID = result
		signature, err := p.constructorSignature(result, resolved, p.check.GetTypeAtLocation(callee), node)
		if err != nil {
			return nil, err
		}
		selected = &signature
		parameters = signature.Parameters
	} else if decl == p.intrinsicNode {
		if len(args) != 3 {
			return nil, p.fail(node, "readSpan requires three original arguments")
		}
		owner, err := p.expression(args[0], "", state, depth+1)
		if err != nil {
			return nil, err
		}
		projectedOwner = owner
		typ := p.types[owner.FlowType]
		if typ.Kind != "array" {
			return nil, p.fail(args[0], "readSpan requires actual Array<T> owner")
		}
		result, err = p.composite("readSpan", typ.ElementType, p.check.GetTypeAtLocation(node))
		if err != nil {
			return nil, err
		}
		parameters = []string{owner.FlowType, p.scalarType("u32"), p.scalarType("u32")}
		calleeID = p.payload.ReadSpanIntrinsic.ID
	} else {
		header := p.headers[symbol]
		if header == nil {
			// Mirror M12's original-declaration join for instantiated method
			// symbols; the resolved signature must still name that exact node.
			roots := p.check.GetRootSymbols(symbol)
			if len(roots) == 1 {
				header = p.headers[roots[0]]
			}
		}
		if header == nil || decl != header.node {
			return nil, p.fail(node, "call signature must join its owned function declaration")
		}
		calleeID, result = header.function.ID, header.function.ReturnType
		receiverExpected = header.function.OwnerType
		for _, param := range header.function.Parameters {
			parameters = append(parameters, param.DeclaredType)
		}
	}
	if len(parameters) != len(args) || len(resolved.Parameters()) != len(args) {
		return nil, p.fail(node, "all original call parameters required")
	}
	if callee.Kind == ast.KindPropertyAccessExpression {
		var err error
		receiver, err = p.expression(callee.AsPropertyAccessExpression().Expression, receiverExpected, state, depth+1)
		if err != nil {
			return nil, err
		}
	}
	values, err := p.argumentValues(args, resolved, parameters, state, depth, projectedOwner)
	if err != nil {
		return nil, err
	}
	if !p.check.IsTypeAssignableTo(p.check.GetReturnTypeOfSignature(resolved), p.actual[result]) {
		return nil, p.fail(node, "actual resolved call result identity")
	}
	if selected == nil {
		signatureID, err := genericNodeID("tsfg.array-facts.signature.v8\x00", decl)
		if err != nil {
			return nil, err
		}
		selected = &arrayGenericSignature{ID: signatureID, SourcePath: ast.GetSourceFileOfNode(decl).FileName(), DeclarationRange: *genericSpan(decl), Parameters: parameters, ReturnType: result}
	}
	v := &arrayGenericExpression{Kind: kind, Type: result, FlowType: result, Range: *genericSpan(node), Callee: calleeID, CalleeRange: genericSpan(callee), Signature: selected, Arguments: &values}
	v.Receiver = receiver
	if kind == "new" {
		v.AllocationSite, err = genericNodeID("tsfg.array-facts.allocation.v8\x00", node)
		if err != nil {
			return nil, err
		}
	}
	// A call can mutate any aliased owner. Loss of a static length only adds a
	// runtime check; it must never become a proof that a gap or read is safe.
	if decl != p.intrinsicNode {
		state.invalidateLengths()
	}
	return v, nil
}

func (p *arrayGenericProjection) argumentValues(args []*ast.Node, resolved *checker.Signature, parameters []string, state *arrayGenericState, depth int, projectedOwner *arrayGenericExpression) ([]arrayGenericExpression, error) {
	if len(args) != len(parameters) || len(args) != len(resolved.Parameters()) {
		return nil, arrayFactsError("identity", "original argument/parameter arity differs", nil)
	}
	values := []arrayGenericExpression{}
	for i, arg := range args {
		if err := p.annotationWork(); err != nil {
			return nil, err
		}
		if !p.check.IsTypeAssignableTo(p.check.GetTypeAtLocation(arg), p.check.GetTypeOfSymbolAtLocation(resolved.Parameters()[i], arg)) {
			return nil, p.fail(arg, "actual resolved signature argument assignability")
		}
		var value *arrayGenericExpression
		var err error
		if i == 0 && projectedOwner != nil {
			value = projectedOwner
		} else if projectedOwner != nil {
			value, err = p.index(arg, true, state, depth+1)
		} else {
			value, err = p.expression(arg, parameters[i], state, depth+1)
		}
		if err != nil {
			return nil, err
		}
		values = append(values, *value)
	}
	return values, nil
}

func (p *arrayGenericProjection) property(node *ast.Node, state *arrayGenericState, depth int) (*arrayGenericExpression, error) {
	access := node.AsPropertyAccessExpression()
	if access.QuestionDotToken != nil {
		return nil, p.fail(node, "ordinary property access required")
	}
	receiver, err := p.expression(access.Expression, "", state, depth+1)
	if err != nil {
		return nil, err
	}
	owner := p.types[receiver.FlowType]
	symbol := p.check.GetSymbolAtLocation(node.Name())
	if symbol == nil {
		return nil, p.fail(node.Name(), "original property symbol required")
	}
	v := &arrayGenericExpression{Range: *genericSpan(node), Receiver: receiver, NameRange: genericSpan(node.Name())}
	if owner.Kind == "array" || owner.Kind == "readSpan" {
		property := p.payload.ArrayLength.ID
		length := p.arrayNode.AsInterfaceDeclaration().Members.Nodes[1]
		kind := "arrayLength"
		if owner.Kind == "readSpan" {
			property = p.payload.ReadSpanLength.ID
			length = p.viewNode.AsInterfaceDeclaration().Members.Nodes[1]
			kind = "readSpanLength"
		}
		if node.Name().Text() != "length" || len(symbol.Declarations) != 1 || symbol.Declarations[0] != length {
			return nil, p.fail(node, "actual pinned length property required")
		}
		v.Kind, v.Type, v.FlowType, v.Property = kind, p.scalarType("u32"), "u32", property
		return v, nil
	}
	field, ok := p.fields[symbol]
	if !ok {
		return nil, p.fail(node, "property must join actual nominal field")
	}
	v.Kind, v.Type, v.Property = "fieldAccess", field.Type, field.ID
	v.FlowType, err = p.flow(p.check.GetTypeAtLocation(node), field.Type)
	return v, err
}
