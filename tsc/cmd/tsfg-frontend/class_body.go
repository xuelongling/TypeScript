// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Complete supported class bodies; unsupported statements are never dropped.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

// A terminal branch is a separate method capability. Linear bodies and all
// constructor bodies retain the existing statement projection and rejection.
func (p *classProjection) methodStatements(nodes []*ast.Node, owner int, returnType *checker.Type) ([]classStatement, error) {
	for _, node := range nodes {
		if node.Kind != ast.KindIfStatement {
			continue
		}
		if len(nodes) != 1 {
			return nil, classError("unsupported", "terminal if must be the sole method statement", classNodeAnchor(node))
		}
		return p.terminalIf(node, owner, returnType)
	}
	return p.statements(nodes, owner, returnType)
}

func (p *classProjection) terminalIf(node *ast.Node, owner int, returnType *checker.Type) ([]classStatement, error) {
	if err := p.ctx.Err(); err != nil {
		return nil, err
	}
	branch := node.AsIfStatement()
	if branch.ThenStatement == nil || branch.ElseStatement == nil ||
		branch.ThenStatement.Kind != ast.KindBlock || branch.ElseStatement.Kind != ast.KindBlock {
		return nil, classError("unsupported", "terminal if requires explicit then and else blocks", classNodeAnchor(node))
	}
	if !classReferenceOperand(returnType) {
		return nil, classError("unsupported", "terminal if requires a reference method return", classNodeAnchor(node))
	}
	for _, block := range []*ast.Node{branch.ThenStatement, branch.ElseStatement} {
		nodes := block.AsBlock().Statements.Nodes
		if len(nodes) != 1 || nodes[0].Kind != ast.KindReturnStatement || nodes[0].AsReturnStatement().Expression == nil {
			return nil, classError("unsupported", "terminal if block requires exactly one reference return", classNodeAnchor(block))
		}
		expression := nodes[0].AsReturnStatement().Expression
		if !classReferenceOperand(p.check.GetTypeAtLocation(expression)) {
			return nil, classError("unsupported", "terminal if branch must return a reference", classNodeAnchor(expression))
		}
	}
	conditionType := p.check.GetTypeAtLocation(branch.Expression)
	if conditionType == nil || !conditionType.IsBooleanLike() {
		return nil, classError("unsupported", "terminal if condition requires official BooleanLike type", classNodeAnchor(branch.Expression))
	}
	condition, err := p.expression(branch.Expression, owner)
	if err != nil {
		return nil, err
	}
	witness, err := p.compatibility(conditionType, p.check.GetBooleanType(), branch.Expression)
	if err != nil {
		return nil, err
	}
	thenBody, err := p.statements(branch.ThenStatement.AsBlock().Statements.Nodes, owner, returnType)
	if err != nil {
		return nil, err
	}
	elseBody, err := p.statements(branch.ElseStatement.AsBlock().Statements.Nodes, owner, returnType)
	if err != nil {
		return nil, err
	}
	return []classStatement{{Kind: "if", Origin: classNodeAnchor(node), Branch: &classConditional{
		Condition: condition, ConditionCompatibility: witness,
		Then: classStatementBlock{Body: thenBody, Origin: classNodeAnchor(branch.ThenStatement)},
		Else: classStatementBlock{Body: elseBody, Origin: classNodeAnchor(branch.ElseStatement)},
	}}}, nil
}

func (p *classProjection) statements(nodes []*ast.Node, owner int, returnType *checker.Type) ([]classStatement, error) {
	out := make([]classStatement, 0, len(nodes))
	for _, node := range nodes {
		if err := p.ctx.Err(); err != nil {
			return nil, err
		}
		kind := "expression"
		var expression *ast.Node
		switch node.Kind {
		case ast.KindVariableStatement:
			local, err := p.localStatement(node, owner)
			if err != nil {
				return nil, err
			}
			out = append(out, classStatement{Kind: "local", Local: local, Origin: classNodeAnchor(node)})
			continue
		case ast.KindExpressionStatement:
			expression = node.AsExpressionStatement().Expression
		case ast.KindReturnStatement:
			kind, expression = "return", node.AsReturnStatement().Expression
		default:
			return nil, classError("unsupported", "class body statement kind: "+node.Kind.String(), classNodeAnchor(node))
		}
		var value *classExpression
		if expression != nil {
			var err error
			value, err = p.expression(expression, owner)
			if err != nil {
				return nil, err
			}
		}
		statement := classStatement{Kind: kind, Expression: value, Origin: classNodeAnchor(node)}
		if kind == "return" && expression != nil {
			if err := p.literalContext(value, returnType, expression); err != nil {
				return nil, err
			}
			witness, err := p.compatibility(p.check.GetTypeAtLocation(expression), returnType, expression)
			if err != nil {
				return nil, err
			}
			statement.Compatibility = &witness
		}
		out = append(out, statement)
	}
	return out, nil
}

func (p *classProjection) expression(node *ast.Node, owner int) (*classExpression, error) {
	if err := p.ctx.Err(); err != nil {
		return nil, err
	}
	if node.Kind == ast.KindIdentifier {
		if err := p.localUse(node); err != nil {
			return nil, err
		}
	}
	if node.Kind == ast.KindBinaryExpression {
		binary := node.AsBinaryExpression()
		if binary.OperatorToken.Kind == ast.KindEqualsToken && binary.Left.Kind == ast.KindIdentifier {
			return p.localAssignment(node, owner)
		}
	}
	if node.Kind == ast.KindCallExpression {
		return p.call(node, owner)
	}
	if node.Kind == ast.KindNewExpression {
		return p.construct(node, owner)
	}
	typeRef, err := p.typeRef(p.check.GetTypeAtLocation(node), node)
	if err != nil {
		return nil, err
	}
	out := &classExpression{ID: p.expressionID(), Type: typeRef, Origin: classNodeAnchor(node)}
	switch node.Kind {
	case ast.KindNumericLiteral:
		token, ok := floatingNode(ast.GetSourceFileOfNode(node), node)
		if !ok {
			return nil, classError("identity", "official numeric literal token", out.Origin)
		}
		out.Kind, out.Token = "number", token
		out.NumericLiteral, err = p.literalFact(node, out.Type)
		if err != nil {
			return nil, err
		}
	case ast.KindTrueKeyword, ast.KindFalseKeyword:
		out.Kind, out.Token = "boolean", "false"
		if node.Kind == ast.KindTrueKeyword {
			out.Token = "true"
		}
	case ast.KindThisKeyword:
		out.Kind, out.Symbol = "this", p.graph.Classes[owner].Symbol
		if typeRef != p.graph.Classes[owner].ThisType && typeRef != p.graph.Classes[owner].InstanceType {
			return nil, classError("identity", "official this type differs from containing class", out.Origin)
		}
	case ast.KindSuperKeyword:
		base := p.graph.Classes[owner].Base
		if base == "" {
			return nil, classError("identity", "super has no official base", out.Origin)
		}
		out.Kind = "super"
		for _, declaration := range p.graph.Classes {
			if declaration.ID == base {
				out.Symbol = declaration.Symbol
			}
		}
		if out.Symbol == "" {
			return nil, classError("identity", "super base symbol missing", out.Origin)
		}
	case ast.KindIdentifier:
		resolved := p.valueSymbol(node)
		out.Kind, out.Symbol = "reference", p.bindings[resolved]
		if out.Symbol == "" {
			return nil, classError("unsupported", "unprojected value binding", out.Origin)
		}
	case ast.KindPropertyAccessExpression:
		access := node.AsPropertyAccessExpression()
		if access.QuestionDotToken != nil {
			return nil, classError("unsupported", "optional property access", out.Origin)
		}
		facts, ok := p.check.TSFGClassPropertyAccess(node)
		if !ok {
			return nil, classError("identity", "official property resolution", out.Origin)
		}
		if facts.Property.Flags&ast.SymbolFlagsMethod != 0 {
			return nil, classError("unsupported", "detached instance method value", out.Origin)
		}
		out.Kind, out.Symbol = "field", p.symbolRef(facts.Property)
		if out.Symbol == "" {
			return nil, classError("unsupported", "unprojected instance member", out.Origin)
		}
		out.Permission = &classPermission{Readable: facts.Readable, Writable: facts.Writable, Readonly: facts.Readonly, WriteUse: facts.IsWrite}
		if !facts.Readable || facts.IsWrite && !facts.Writable {
			return nil, classError("access", "official member permission rejected", out.Origin)
		}
		out.Receiver, err = p.expression(access.Expression, owner)
		if err != nil {
			return nil, err
		}
	case ast.KindBinaryExpression:
		binary := node.AsBinaryExpression()
		switch binary.OperatorToken.Kind {
		case ast.KindEqualsToken:
			out.Kind, out.Operator = "assignment", "="
		case ast.KindPlusToken:
			out.Kind, out.Operator = "binary", "+"
		case ast.KindEqualsEqualsEqualsToken:
			out.Kind, out.Operator = "reference-identity", "==="
		case ast.KindExclamationEqualsEqualsToken:
			out.Kind, out.Operator = "reference-identity", "!=="
		default:
			return nil, classError("unsupported", "class body binary operator: "+binary.OperatorToken.Kind.String(), out.Origin)
		}
		if binary.OperatorToken.Kind == ast.KindEqualsToken && binary.Left.Kind != ast.KindPropertyAccessExpression {
			return nil, classError("unsupported", "class body assignment requires an instance field", classNodeAnchor(binary.Left))
		}
		out.Left, err = p.expression(binary.Left, owner)
		if err != nil {
			return nil, err
		}
		out.Right, err = p.expression(binary.Right, owner)
		if err != nil {
			return nil, err
		}
		if binary.OperatorToken.Kind == ast.KindPlusToken {
			// This is a capability check on official types, not overload/type inference.
			if out.Type != "t:number" || out.Left.Type != "t:number" || out.Right.Type != "t:number" {
				return nil, classError("unsupported", "only declared number=f64 operands are currently projected for +", out.Origin)
			}
		} else if out.Kind == "reference-identity" {
			// Official checking has already accepted comparability. Preserve the
			// operation; allocation keys never determine its runtime result.
			if out.Type != "t:boolean" || !classReferenceOperand(p.check.GetTypeAtLocation(binary.Left)) ||
				!classReferenceOperand(p.check.GetTypeAtLocation(binary.Right)) {
				return nil, classError("unsupported", "identity comparison requires officially typed class reference operands", out.Origin)
			}
		} else {
			if err := p.literalContext(out.Right, p.check.GetTypeAtLocation(binary.Left), binary.Right); err != nil {
				return nil, err
			}
			witness, err := p.compatibility(p.check.GetTypeAtLocation(binary.Right), p.check.GetTypeAtLocation(binary.Left), binary.Right)
			if err != nil {
				return nil, err
			}
			out.Compatibility = &witness
		}
	default:
		return nil, classError("unsupported", "class body expression kind: "+node.Kind.String(), out.Origin)
	}
	return out, nil
}

// This is an official type-category capability check, not an assignability rule.
func classReferenceOperand(value *checker.Type) bool {
	return value != nil && (value.IsClass() || value.IsTypeParameter() && value.AsTypeParameter().IsThisType())
}
