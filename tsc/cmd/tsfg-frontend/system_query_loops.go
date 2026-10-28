// SPDX-License-Identifier: Apache-2.0
// Original Query iteration is typed by the owned Program. These are data facts,
// not a JavaScript iterator implementation or executable loop lowering.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

func (p *arrayGenericProjection) systemLoops(body *ast.Node, parameters []systemQueryParameter) ([]systemQueryLoop, error) {
	out := []systemQueryLoop{}
	params := map[*ast.Symbol]systemQueryParameter{}
	for index, parameter := range parameters {
		original := body.Parent.AsFunctionDeclaration().Parameters.Nodes[index]
		params[p.check.GetSymbolAtLocation(original.Name())] = parameter
	}
	for _, statement := range body.AsBlock().Statements.Nodes {
		if err := p.annotationWork(); err != nil {
			return nil, err
		}
		if statement.Kind != ast.KindForOfStatement {
			return nil, p.fail(statement, "system body requires original Query for-of statements")
		}
		loop := statement.AsForInOrOfStatement()
		if loop.AwaitModifier != nil || loop.Expression.Kind != ast.KindIdentifier || loop.Initializer.Kind != ast.KindVariableDeclarationList || loop.Statement.Kind != ast.KindBlock {
			return nil, p.fail(statement, "original synchronous Query const tuple loop required")
		}
		parameter, ok := params[p.check.GetSymbolAtLocation(loop.Expression)]
		if !ok || parameter.Query == nil {
			return nil, p.fail(loop.Expression, "loop must resolve original Query parameter Symbol")
		}
		list := loop.Initializer.AsVariableDeclarationList()
		if loop.Initializer.Flags&ast.NodeFlagsConst == 0 || len(list.Declarations.Nodes) != 1 {
			return nil, p.fail(loop.Initializer, "single original const tuple binding required")
		}
		declaration := list.Declarations.Nodes[0]
		if declaration.Name().Kind != ast.KindArrayBindingPattern || declaration.AsVariableDeclaration().Initializer != nil || declaration.AsVariableDeclaration().Type != nil {
			return nil, p.fail(declaration, "complete inferred Query tuple binding required")
		}
		elements := declaration.Name().AsBindingPattern().Elements.Nodes
		if len(elements) != len(parameter.Query.Items) {
			return nil, p.fail(declaration, "Query tuple binding inventory differs")
		}
		row := systemQueryLoop{Range: *genericSpan(statement), QueryBinding: parameter.Binding.ID, QueryRange: *genericSpan(loop.Expression), BindingRange: *genericSpan(declaration.Name()), BodyRange: *genericSpan(loop.Statement), Bindings: []systemQueryLoopBinding{}, LocalBindings: []arrayFactsBinding{}, Statements: []arrayGenericStatement{}}
		bindings := map[*ast.Symbol]systemQueryLoopBinding{}
		for index, element := range elements {
			if element.Kind != ast.KindBindingElement || element.Name() == nil || element.Name().Kind != ast.KindIdentifier {
				return nil, p.fail(element, "one original named binding per Query access required")
			}
			binding := element.AsBindingElement()
			if binding.DotDotDotToken != nil || binding.PropertyName != nil || binding.Initializer != nil {
				return nil, p.fail(element, "plain Query tuple binding required")
			}
			symbol := p.check.GetSymbolAtLocation(element.Name())
			if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != element {
				return nil, p.fail(element, "unique original Query binding Symbol required")
			}
			actual := p.check.GetTypeOfSymbolAtLocation(symbol, element)
			item := parameter.Query.Items[index]
			if actual == nil || actual.Flags()&checker.TypeFlagsObject == 0 || !p.check.IsTypeAssignableTo(actual, p.actual[item.ComponentType]) {
				return nil, p.fail(element, "official Query element type must preserve component fields")
			}
			if item.Mode == "write" && systemTypeSymbol(actual) != p.classNodes[item.ComponentType].Symbol() {
				return nil, p.fail(element, "Write Query element must retain original mutable component type")
			}
			owned := systemQueryLoopBinding{index, systemOrigin(element), item.ComponentType, item.Mode}
			row.Bindings = append(row.Bindings, owned)
			bindings[symbol] = owned
		}
		statements, localBindings, e := p.systemStatements(loop.Statement, bindings, params, "")
		if e != nil {
			return nil, e
		}
		row.Statements, row.LocalBindings = statements, localBindings
		out = append(out, row)
	}
	return out, nil
}

func (p *arrayGenericProjection) systemLoopExpression(node *ast.Node, bindings map[*ast.Symbol]systemQueryLoopBinding, params map[*ast.Symbol]systemQueryParameter, locals map[*ast.Symbol]arrayFactsBinding, depth int, expected ...string) (*arrayGenericExpression, error) {
	if depth >= 32 {
		return nil, p.fail(node, "Query expression exceeds32 nodes")
	}
	if e := p.annotationWork(); e != nil {
		return nil, e
	}
	actual := p.check.GetTypeAtLocation(node)
	if actual == nil || actual.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown) != 0 {
		return nil, p.fail(node, "original checked Query expression type required")
	}
	context := ""
	if len(expected) > 0 {
		context = expected[0]
	}
	switch node.Kind {
	case ast.KindNumericLiteral, ast.KindPrefixUnaryExpression:
		if node.Kind == ast.KindPrefixUnaryExpression && (node.AsPrefixUnaryExpression().Operator != ast.KindMinusToken || node.AsPrefixUnaryExpression().Operand.Kind != ast.KindNumericLiteral) {
			return nil, p.fail(node, "Query floating literal unary syntax differs")
		}
		value, e := p.number(node, context)
		if e != nil {
			return nil, e
		}
		if value.Type != "f32" && value.Type != "f64" {
			return nil, p.fail(node, "Query literal requires original floating-point context")
		}
		value.FlowType = value.Type
		return value, nil
	case ast.KindIdentifier:
		symbol := p.check.GetSymbolAtLocation(node)
		if local, ok := locals[symbol]; ok && actual.Flags()&checker.TypeFlagsNumberLike != 0 {
			return &arrayGenericExpression{Kind: "bindingReference", Type: local.DeclaredType, FlowType: local.DeclaredType, Range: *genericSpan(node), Binding: local.ID}, nil
		}
		if binding, ok := bindings[symbol]; ok {
			return &arrayGenericExpression{Kind: "bindingReference", Type: binding.ComponentType, FlowType: binding.ComponentType, Range: *genericSpan(node), Binding: binding.Declaration.ID}, nil
		}
		if parameter, ok := params[symbol]; ok && parameter.Query == nil && (parameter.LogicalType == "f32" || parameter.LogicalType == "f64") && actual.Flags()&checker.TypeFlagsNumberLike != 0 {
			return &arrayGenericExpression{Kind: "bindingReference", Type: parameter.LogicalType, FlowType: parameter.LogicalType, Range: *genericSpan(node), Binding: parameter.Binding.ID}, nil
		}
		return nil, p.fail(node, "Query expression must join original loop binding or f32/f64 parameter")
	case ast.KindPropertyAccessExpression:
		access := node.AsPropertyAccessExpression()
		if access.QuestionDotToken != nil || access.Expression.Kind != ast.KindIdentifier {
			return nil, p.fail(node, "direct Query component field access required")
		}
		receiver, e := p.systemLoopExpression(access.Expression, bindings, params, locals, depth+1, context)
		if e != nil {
			return nil, e
		}
		if p.types[receiver.Type].NominalKey == nil || p.types[receiver.Type].NominalKey.Category != "component" {
			return nil, p.fail(node, "field receiver must be Query component binding")
		}
		original := p.check.GetPropertyOfType(p.actual[receiver.Type], node.Name().Text())
		selected := p.check.GetSymbolAtLocation(node.Name())
		field, ok := p.fields[original]
		if !ok || original == nil || selected == nil || len(original.Declarations) != 1 || len(selected.Declarations) != 1 || selected.Declarations[0] != original.Declarations[0] {
			return nil, p.fail(node, "Query field must retain original component property declaration")
		}
		if field.Type != "f32" && field.Type != "f64" || actual.Flags()&checker.TypeFlagsNumberLike == 0 {
			return nil, p.fail(node, "Query field arithmetic supports original f32/f64 declaration")
		}
		return &arrayGenericExpression{Kind: "fieldAccess", Type: field.Type, FlowType: field.Type, Range: *genericSpan(node), Receiver: receiver, Property: field.ID, NameRange: genericSpan(node.Name())}, nil
	case ast.KindCallExpression:
		return p.systemScalarCall(node, bindings, params, locals, depth)
	case ast.KindParenthesizedExpression:
		inner, e := p.systemLoopExpression(node.AsParenthesizedExpression().Expression, bindings, params, locals, depth+1, context)
		if e != nil {
			return nil, e
		}
		return &arrayGenericExpression{Kind: "parentheses", Type: inner.Type, FlowType: inner.FlowType, Range: *genericSpan(node), Expression: inner}, nil
	case ast.KindBinaryExpression:
		binary := node.AsBinaryExpression()
		switch binary.OperatorToken.Kind {
		case ast.KindEqualsToken, ast.KindPlusEqualsToken, ast.KindMinusEqualsToken, ast.KindAsteriskEqualsToken, ast.KindSlashEqualsToken:
			return p.systemLoopAssignment(node, bindings, params, locals, depth)
		case ast.KindPlusToken, ast.KindMinusToken, ast.KindAsteriskToken, ast.KindSlashToken:
		default:
			return nil, p.fail(node, "Query expression requires f32 arithmetic operator")
		}
		left, e := p.systemLoopExpression(binary.Left, bindings, params, locals, depth+1, context)
		if e != nil {
			return nil, e
		}
		rightContext := context
		if rightContext == "" {
			rightContext = left.Type
		}
		right, e := p.systemLoopExpression(binary.Right, bindings, params, locals, depth+1, rightContext)
		if e != nil {
			return nil, e
		}
		if left.Type != "f32" && left.Type != "f64" || right.Type != left.Type || actual.Flags()&checker.TypeFlagsNumberLike == 0 {
			return nil, p.fail(node, "Query binary must preserve same-width f32/f64 operands")
		}
		return &arrayGenericExpression{Kind: "binary", Type: left.Type, FlowType: left.Type, Range: *genericSpan(node), Operator: genericText(binary.OperatorToken), OperatorRange: genericSpan(binary.OperatorToken), Left: left, Right: right}, nil
	default:
		return nil, p.fail(node, "Query expression has no owned typed representation")
	}
}

func (p *arrayGenericProjection) systemLoopAssignment(node *ast.Node, bindings map[*ast.Symbol]systemQueryLoopBinding, params map[*ast.Symbol]systemQueryParameter, locals map[*ast.Symbol]arrayFactsBinding, depth int) (*arrayGenericExpression, error) {
	assignment := node.AsBinaryExpression()
	target, e := p.systemLoopExpression(assignment.Left, bindings, params, locals, depth+1)
	if e != nil {
		return nil, e
	}
	if target.Kind == "bindingReference" {
		local, ok := locals[p.check.GetSymbolAtLocation(assignment.Left)]
		if !ok || !local.Writable {
			return nil, p.fail(assignment.Left, "Query local assignment requires original writable local")
		}
		value, e := p.systemLoopExpression(assignment.Right, bindings, params, locals, depth+1, target.Type)
		if e != nil {
			return nil, e
		}
		if value.Type != local.DeclaredType || value.FlowType != local.DeclaredType {
			return nil, p.fail(node, "Query local assignment must preserve floating-point width")
		}
		return &arrayGenericExpression{Kind: "bindingAssignment", Type: local.DeclaredType, FlowType: local.DeclaredType, Range: *genericSpan(node), Operator: genericText(assignment.OperatorToken), OperatorRange: genericSpan(assignment.OperatorToken), Target: &arrayGenericTarget{Kind: "bindingTarget", Type: local.DeclaredType, Range: target.Range, Binding: local.ID}, Value: value}, nil
	}
	if target.Kind != "fieldAccess" || target.Receiver == nil {
		return nil, p.fail(assignment.Left, "Query assignment requires original field target")
	}
	writable := false
	for _, binding := range bindings {
		if binding.Declaration.ID == target.Receiver.Binding {
			writable = binding.Mode == "write"
		}
	}
	if !writable {
		return nil, p.fail(assignment.Left, "Read Query borrow is readonly")
	}
	original := p.check.GetPropertyOfType(p.actual[target.Receiver.Type], assignment.Left.Name().Text())
	if p.fields[original].Readonly {
		return nil, p.fail(assignment.Left, "component field is readonly")
	}
	value, e := p.systemLoopExpression(assignment.Right, bindings, params, locals, depth+1, target.Type)
	if e != nil {
		return nil, e
	}
	if target.Type != "f32" && target.Type != "f64" || value.Type != target.Type || p.check.GetTypeAtLocation(node).Flags()&checker.TypeFlagsNumberLike == 0 {
		return nil, p.fail(node, "Query arithmetic requires original same-width f32/f64 operands")
	}
	return &arrayGenericExpression{Kind: "fieldAssignment", Type: target.Type, FlowType: target.Type, Range: *genericSpan(node), Operator: genericText(assignment.OperatorToken), OperatorRange: genericSpan(assignment.OperatorToken), Target: &arrayGenericTarget{Kind: "fieldTarget", Type: target.Type, Range: target.Range, Receiver: target.Receiver, Property: target.Property, NameRange: target.NameRange}, Value: value}, nil
}
