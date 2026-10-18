// SPDX-License-Identifier: Apache-2.0

// Validates one finite guard body against the original checker; it emits no facts.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

type nullablePresenceBinding struct {
	Symbol  *ast.Symbol
	Payload *checker.Type
	Mask    uint8
}

func nullablePresenceParameterNode(node *ast.Node, index int) *ast.Node {
	if node != nil && node.Kind == ast.KindFunctionDeclaration {
		parameters := node.AsFunctionDeclaration().Parameters.Nodes
		if index >= 0 && index < len(parameters) {
			return parameters[index]
		}
	}
	return node
}

// Join owned binding IDs to original declarations, never to DTO ranges or names.
// This runs only at the first v4 symbol rejection; it is not a second grammar pass.
func nullablePresenceSymbolDeclaration(file *ast.SourceFile, check *checker.Checker, facts response, wanted symbol) *ast.Node {
	if wanted.ID == "" {
		return nil
	}
	for index, fn := range facts.Functions {
		if index >= len(file.Statements.Nodes) {
			return nil
		}
		node := file.Statements.Nodes[index]
		if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			continue
		}
		if fn.Symbol == wanted.ID {
			return nullablePresenceCheckedDeclaration(file, check, node)
		}
		for position, parameter := range fn.Parameters {
			if parameter.ID == wanted.ID {
				return nullablePresenceCheckedDeclaration(file, check, nullablePresenceParameterNode(node, position))
			}
		}
		ordinal, wantedOrdinal := 0, -1
		walkOwnedStatements(fn.Body, func(item *statement) {
			if item.Kind == "variable" && item.Symbol != nil {
				if item.Symbol.ID == wanted.ID {
					wantedOrdinal = ordinal
				}
				ordinal++
			}
		})
		if wantedOrdinal < 0 {
			continue
		}
		ordinal = 0
		var found *ast.Node
		visitStatements(node.Body().AsBlock().Statements.Nodes, func(item *ast.Node) bool {
			for _, declaration := range localDeclarations(item) {
				if ordinal == wantedOrdinal {
					found = nullablePresenceCheckedDeclaration(file, check, declaration)
					return true
				}
				ordinal++
			}
			return false
		})
		return found
	}
	return nil
}

func nullablePresenceCheckedDeclaration(file *ast.SourceFile, check *checker.Checker, node *ast.Node) *ast.Node {
	if node == nil || node.Name() == nil {
		return nil
	}
	actual := check.GetSymbolAtLocation(node.Name())
	if actual != nil {
		for _, declaration := range actual.Declarations {
			if declaration == node && ast.GetSourceFileOfNode(declaration) == file {
				return declaration
			}
		}
	}
	return nil
}

func nullablePresenceGuardCount(body []*ast.Node) int {
	count := 0
	for _, node := range body {
		if node.Kind == ast.KindIfStatement {
			count++
		}
	}
	return count
}

func nullablePresenceType(typ, payload *checker.Type, expected uint8) bool {
	mask, valid := nullableFactsMask(typ)
	return valid && mask == expected && nominalPayload(typ) == payload
}

func nullablePresenceParameter(owned, table symbol, file *ast.SourceFile, node *ast.Node, mask uint8) bool {
	shape := owned.Presence
	return owned.ID != "" && owned.ID == table.ID && owned.Name == node.Name().Text() && owned.Name == table.Name &&
		owned.Kind == "parameter" && table.Kind == owned.Kind && owned.Type == "i32" && table.Type == owned.Type &&
		owned.Writable && table.Writable && owned.Range == offset(file, node.Name()) && table.Range == owned.Range &&
		shape != nil && table.Presence != nil && *shape == *table.Presence && !shape.Optional && !shape.Default &&
		shape.Undefined == (mask&1 != 0) && shape.Null == (mask&2 != 0) &&
		shape.Range == offset(file, node) && shape.Annotation == expressionOffset(file, node.AsParameterDeclaration().Type)
}

func nullablePresenceFactsBody(file *ast.SourceFile, check *checker.Checker, node *ast.Node, fn function,
	info functionInfo, signature *checker.Signature, symbols map[string]symbol, guardCount int,
) error {
	bad := func(detail string, at *ast.Node) error {
		return nullableFactsV4.at("capability", detail, file, at)
	}
	decl, body := node.AsFunctionDeclaration(), node.Body().AsBlock().Statements.Nodes
	if guardCount > 2 {
		return nullableFactsV4.failure("limit", "guards per function")
	}
	if guardCount == 0 || len(body) != guardCount+1 {
		return bad("guard sequence followed by one final return", node.Body())
	}
	if info.ID == "" || info.Declaration != node || fn.Symbol != info.ID || fn.Name != node.Name().Text() ||
		fn.Range != offset(file, node) || fn.ReturnPresence != nil || info.Signature.ReturnPresence != nil ||
		!nullablePresenceType(check.GetReturnTypeOfSignature(signature), nominalPayload(check.GetTypeFromTypeNode(decl.Type)), 0) {
		return bad("guard function requires its scalar declaration and binding", node)
	}
	parameters := decl.Parameters.Nodes
	if len(parameters) != len(fn.Parameters) || len(parameters) != len(signature.Parameters()) ||
		len(parameters) != len(info.Signature.Parameters) || len(parameters) != len(info.Signature.Presence) ||
		len(parameters) != len(fn.ParameterInitializers) {
		return bad("complete required parameter signature", node)
	}
	bindings := map[*ast.Symbol]nullablePresenceBinding{}
	var guarded nullablePresenceBinding
	for index, parameter := range parameters {
		if parameter.Name() == nil || parameter.Name().Kind != ast.KindIdentifier {
			return bad("direct required parameter binding", parameter)
		}
		actual := check.GetSymbolAtLocation(parameter.Name())
		declared := check.GetTypeFromTypeNode(parameter.AsParameterDeclaration().Type)
		mask, valid := nullableFactsMask(declared)
		payload := nominalPayload(declared)
		if actual == nil || !valid || payload == nil || signature.Parameters()[index] != actual ||
			!nullablePresenceType(check.GetTypeOfSymbol(actual), payload, mask) {
			return bad("one declared symbol and signature parameter type", parameter)
		}
		if _, duplicate := bindings[actual]; duplicate {
			return bad("distinct required parameter symbols", parameter)
		}
		owned := fn.Parameters[index]
		if info.Signature.Parameters[index] != "i32" || !nullablePresenceParameter(owned, symbols[owned.ID], file, parameter, mask) ||
			info.Signature.Presence[index] == nil || *info.Signature.Presence[index] != *owned.Presence {
			return bad("owned declaration and signature presence", parameter)
		}
		binding := nullablePresenceBinding{Symbol: actual, Payload: payload, Mask: mask}
		bindings[actual] = binding
		if mask != 0 {
			if guarded.Symbol != nil {
				return bad("one guarded nullable parameter", parameter)
			}
			guarded = binding
		}
	}
	if guarded.Symbol == nil || len(bindings) < 2 {
		return bad("guarded parameter and distinct scalar fallback", node)
	}
	// These masks are expectations only: every use below is read from the checker.
	remaining := guarded.Mask
	for _, statement := range body[:len(body)-1] {
		if statement.Kind != ast.KindIfStatement {
			return bad("only original presence guards before final return", statement)
		}
		branch := statement.AsIfStatement()
		condition := branch.Expression
		if condition == nil || condition.Kind != ast.KindBinaryExpression {
			return bad("direct strict presence condition", condition)
		}
		binary := condition.AsBinaryExpression()
		conditionType := check.GetTypeAtLocation(condition)
		if binary.OperatorToken.Kind != ast.KindEqualsEqualsEqualsToken || conditionType == nil || !conditionType.IsBooleanLike() {
			return bad("strict boolean presence condition", condition)
		}
		left := binary.Left
		if left.Kind != ast.KindIdentifier || check.GetSymbolAtLocation(left) != guarded.Symbol ||
			!nullablePresenceType(check.GetTypeAtLocation(left), guarded.Payload, remaining) {
			return bad("original guarded parameter use-site type", left)
		}
		right := binary.Right
		var bit uint8
		switch {
		case right.Kind == ast.KindNullKeyword && missingState(check, right) == "null":
			bit = 2
		case right.Kind == ast.KindIdentifier && missingState(check, right) == "undefined":
			actual := check.GetSymbolAtLocation(right)
			if actual == nil || actual != check.GetUndefinedSymbol() || len(actual.Declarations) != 0 {
				return bad("official builtin undefined operand", right)
			}
			bit = 1
		default:
			return bad("direct null or builtin undefined operand", right)
		}
		if remaining&bit == 0 {
			return bad("each actual absence guarded once", right)
		}
		if branch.ElseStatement != nil {
			return bad("presence guard has no else", branch.ElseStatement)
		}
		then := branch.ThenStatement
		if then == nil || then.Kind != ast.KindBlock {
			return bad("presence guard then block", then)
		}
		returns := then.AsBlock().Statements.Nodes
		if len(returns) != 1 {
			return bad("then block contains one early return", then)
		}
		returned := returns[0]
		if returned.Kind != ast.KindReturnStatement {
			return bad("then early return statement", returned)
		}
		value := returned.AsReturnStatement().Expression
		if value == nil {
			return bad("direct scalar fallback reference", returned)
		}
		if value.Kind != ast.KindIdentifier {
			return bad("direct scalar fallback reference", value)
		}
		fallback, found := bindings[check.GetSymbolAtLocation(value)]
		if !found || fallback.Symbol == guarded.Symbol || fallback.Mask != 0 ||
			!nullablePresenceType(check.GetTypeAtLocation(value), fallback.Payload, 0) {
			return bad("same-owner required scalar fallback", value)
		}
		remaining &^= bit
	}
	final := body[len(body)-1]
	value := final.AsReturnStatement().Expression
	if value == nil {
		return bad("direct narrowed final reference", final)
	}
	if value.Kind != ast.KindIdentifier {
		return bad("direct narrowed final reference", value)
	}
	if remaining != 0 || check.GetSymbolAtLocation(value) != guarded.Symbol ||
		!nullablePresenceType(check.GetTypeAtLocation(value), guarded.Payload, 0) {
		return bad("complete guards and actual narrowed final use", value)
	}
	return nil
}
