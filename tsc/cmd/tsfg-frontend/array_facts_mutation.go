// SPDX-License-Identifier: Apache-2.0
// Ordinary assignments preserve operand order, property origin, and checked obligations.
package main

import (
	"strconv"

	"github.com/microsoft/typescript-go/internal/ast"
)

func (p *arrayFactsProjection) mutation(statement *ast.Node, state *arrayFactsFunctionState) (arrayFactsStatement, error) {
	var result arrayFactsStatement
	expression := statement.AsExpressionStatement().Expression
	if expression == nil || expression.Kind != ast.KindBinaryExpression {
		return result, arrayFactsCapability(p.file, statement, "ordinary Array assignment statement required")
	}
	binary := expression.AsBinaryExpression()
	if binary.OperatorToken == nil || binary.OperatorToken.Kind != ast.KindEqualsToken || binary.Type != nil || expression.ModifierFlags() != 0 {
		return result, arrayFactsCapability(p.file, expression, "only plain Array assignment required")
	}
	left := binary.Left
	if left == nil {
		return result, arrayFactsCapability(p.file, expression, "direct Array target required")
	}
	var target *arrayFactsTarget
	var value *arrayFactsValue
	var err error
	kind := ""
	switch left.Kind {
	case ast.KindElementAccessExpression:
		access := left.AsElementAccessExpression()
		if access.QuestionDotToken != nil {
			return result, arrayFactsCapability(p.file, left, "ordinary Array element target required")
		}
		array, failure := p.reference(access.Expression, state.bindings)
		if failure != nil {
			return result, failure
		}
		index, failure := p.unsigned(access.ArgumentExpression, false, state)
		if failure != nil {
			return result, failure
		}
		target = &arrayFactsTarget{Kind: "elementTarget", Type: "f64", Range: expressionOffset(p.file, left), Array: array, Index: index}
		kind = "elementAssignment"
		// Receiver and index are retained before the RHS checked read; the final
		// operation remains a checked mutation, never an evaluated source fact.
		value, err = p.rhs(binary.Right, state)
	case ast.KindPropertyAccessExpression:
		access := left.AsPropertyAccessExpression()
		if access.QuestionDotToken != nil || left.Name() == nil || left.Name().Kind != ast.KindIdentifier || left.Name().Text() != "length" {
			return result, arrayFactsCapability(p.file, left, "ordinary original Array.length target required")
		}
		array, failure := p.reference(access.Expression, state.bindings)
		if failure != nil {
			return result, failure
		}
		property := p.check.GetSymbolAtLocation(left.Name())
		receiverType := p.check.GetTypeAtLocation(access.Expression)
		if property == nil || property != p.check.GetPropertyOfType(receiverType, "length") || len(property.Declarations) != 1 || property.Declarations[0] != p.lengthNode || p.check.GetTypeOfSymbolAtLocation(property, left.Name()) != p.check.GetNumberType() {
			return result, arrayFactsCapability(p.file, left, "actual pinned Array length property binding/type required")
		}
		name := expressionOffset(p.file, left.Name())
		target = &arrayFactsTarget{Kind: "lengthTarget", Type: "u32", Range: expressionOffset(p.file, left), Array: array, Property: p.lengthID, NameRange: &name}
		kind = "lengthAssignment"
		value, err = p.unsigned(binary.Right, true, state)
	default:
		return result, arrayFactsCapability(p.file, left, "only direct Array element or length target required")
	}
	if err != nil {
		return result, err
	}
	if p.check.GetTypeAtLocation(left) != p.check.GetNumberType() || p.check.GetTypeAtLocation(expression) == nil || p.check.GetTypeAtLocation(binary.Right) == nil || p.check.GetBaseTypeOfLiteralType(p.check.GetTypeAtLocation(expression)) != p.check.GetNumberType() || p.check.GetBaseTypeOfLiteralType(p.check.GetTypeAtLocation(binary.Right)) != p.check.GetNumberType() {
		return result, arrayFactsCapability(p.file, expression, "official numeric assignment target/result/RHS required")
	}
	full, operator := expressionOffset(p.file, expression), expressionOffset(p.file, binary.OperatorToken)
	return arrayFactsStatement{Kind: kind, Range: expressionOffset(p.file, statement), ExpressionRange: &full, OperatorRange: &operator, Target: target, Value: value}, nil
}

// This pass runs only after complete finite grammar projection. It proves the
// selected dense source invariant without evaluating reads, RHS, or parameters.
// Even when exact length is known, checked operations stay in the owned body.
func (p *arrayFactsProjection) density(facts []arrayFactsStatement, nodes []*ast.Node, state *arrayFactsFunctionState) error {
	for i, fact := range facts {
		if err := p.ctx.Err(); err != nil {
			return arrayFactsError("canceled", "dense source proof", err)
		}
		if fact.Kind != "elementAssignment" && fact.Kind != "lengthAssignment" {
			continue
		}
		owner := state.owners[fact.Target.Array.Binding]
		binary := nodes[i].AsExpressionStatement().Expression.AsBinaryExpression()
		if fact.Kind == "elementAssignment" {
			if fact.Target.Index.Kind == "reference" {
				owner.known = false
				continue
			}
			index, err := strconv.ParseUint(fact.Target.Index.Decimal, 10, 32)
			if err != nil {
				return arrayFactsError("identity", "projected exact index invariant", err)
			}
			if owner.known {
				if index > owner.length {
					return arrayFactsDensity(p.file, binary.Left.AsElementAccessExpression().ArgumentExpression, "literal assignment creates a proven dense gap")
				}
				if index == owner.length {
					owner.length++
				}
			}
			continue
		}
		if fact.Value.Kind == "reference" {
			owner.known = false
			continue
		}
		length, err := strconv.ParseUint(fact.Value.Decimal, 10, 32)
		if err != nil {
			return arrayFactsError("identity", "projected exact length invariant", err)
		}
		if owner.known && length > owner.length {
			return arrayFactsDensity(p.file, binary.Right, "literal length assignment creates proven uninitialized growth")
		}
		owner.known, owner.length = true, length
	}
	return nil
}
