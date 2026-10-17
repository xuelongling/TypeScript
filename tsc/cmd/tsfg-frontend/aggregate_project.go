// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
)

func (p *projection) aggregateExpr(node *ast.Node, expected string, presence ...*parameterPresence) (*expression, response, bool) {
	if node == nil {
		return nil, response{}, false
	}
	finish := func(value *expression, failure response) (*expression, response, bool) {
		if failure.Status != "" {
			return nil, failure, true
		}
		if expected != "" && value.Type != expected {
			code := "r01-d-scalar-type"
			if strings.HasPrefix(expected, "a") || strings.HasPrefix(value.Type, "a") {
				code = "r01-d-aggregate-identity"
			}
			return nil, failAt(code, "error", p.file, node), true
		}
		var shape *parameterPresence
		if len(presence) != 0 {
			shape = presence[0]
		}
		if expected != "" && !permittedPresence(value, shape) {
			return nil, failAt("r01-d-scalar-type", "error", p.file, node), true
		}
		return value, response{}, true
	}
	if node.Kind == ast.KindParenthesizedExpression {
		inner, failure, handled := p.aggregateExpr(node.AsParenthesizedExpression().Expression, expected, presence...)
		if !handled {
			return nil, response{}, false
		}
		if failure.Status != "" {
			return nil, failure, true
		}
		return &expression{Kind: "group", Type: inner.Type, Operand: inner, Range: expressionOffset(p.file, node)}, response{}, true
	}
	if node.Kind == ast.KindThisKeyword {
		if p.self == nil {
			return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node), true
		}
		id := p.aggregates.facts.Declarations[p.self.index].ID
		return finish(&expression{Kind: "constructionReceiver", Type: id, OwnerType: id, Range: expressionOffset(p.file, node)}, response{})
	}
	if node.Kind == ast.KindIdentifier {
		binding, exists := p.bindings[p.check.GetSymbolAtLocation(node)]
		if exists && strings.HasPrefix(binding.Type, "a") {
			return finish(&expression{Kind: "reference", Type: binding.Type, Symbol: binding.ID, Range: expressionOffset(p.file, node)}, response{})
		}
		if p.aggregates.classes[p.check.GetSymbolAtLocation(node)] != nil {
			return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node), true
		}
	}
	if node.Kind == ast.KindPropertyAccessExpression {
		value, failure := p.fieldExpr(node, false)
		return finish(value, failure)
	}
	if node.Kind == ast.KindNewExpression {
		call := node.AsNewExpression()
		if call.Expression.Kind != ast.KindIdentifier || call.TypeArguments != nil {
			return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node), true
		}
		info := p.aggregates.classes[p.check.GetSymbolAtLocation(call.Expression)]
		if info == nil {
			return nil, failAt("r01-d-aggregate-identity", "error", p.file, node), true
		}
		decl := p.aggregates.facts.Declarations[info.index]
		if len(decl.Constructors) != 1 || p.check.GetResolvedSignature(node).Declaration() != info.checked.Declaration() {
			return nil, failAt("r01-d-aggregate-identity", "error", p.file, node), true
		}
		ctor := decl.Constructors[0]
		var nodes []*ast.Node
		if call.Arguments != nil {
			nodes = call.Arguments.Nodes
		}
		if len(nodes) < info.checked.MinArgumentCount() || len(nodes) > len(ctor.Parameters) {
			return nil, failAt("r02b-d-type", "error", p.file, node), true
		}
		args := make([]expression, 0, len(nodes))
		for index, arg := range nodes {
			value, failure := p.expr(arg, ctor.Parameters[index].Type, ctor.Parameters[index].Presence)
			if failure.Status != "" {
				return nil, failure, true
			}
			args = append(args, *value)
		}
		return finish(&expression{Kind: "construct", Type: decl.ID, Target: ctor.ID, Arguments: &args, Signature: &ctor.Signature, Range: expressionOffset(p.file, node)}, response{})
	}
	if node.Kind == ast.KindCallExpression {
		call := node.AsCallExpression()
		if call.Expression.Kind != ast.KindIdentifier {
			return nil, response{}, false
		}
		target, exists := p.functions[p.check.GetSymbolAtLocation(call.Expression)]
		if !exists || !p.involvesAggregate(node) {
			return nil, response{}, false
		}
		if call.QuestionDotToken != nil || call.TypeArguments != nil || len(call.Arguments.Nodes) > len(target.Signature.Parameters) {
			return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node), true
		}
		args := []expression{}
		for index, arg := range call.Arguments.Nodes {
			var shape *parameterPresence
			if target.Signature.Presence != nil {
				shape = target.Signature.Presence[index]
			}
			value, failure := p.expr(arg, target.Signature.Parameters[index], shape)
			if failure.Status != "" {
				return nil, failure, true
			}
			args = append(args, *value)
		}
		value := &expression{Kind: "call", Type: target.Signature.ReturnType, Target: target.ID, Arguments: &args, Signature: &target.Signature, Range: expressionOffset(p.file, node)}
		if target.Conversion != nil {
			if len(args) != 1 {
				return nil, failAt("r01-d-conversion-entry", "error", p.file, node), true
			}
			value.Kind = "conversion"
			value.Conversion = target.Conversion
			value.Target = ""
			value.Operand = &args[0]
			value.Arguments = nil
			value.Signature = nil
		}
		return finish(value, response{})
	}
	if node.Kind == ast.KindBinaryExpression && p.involvesAggregate(node) {
		// Unknown writes take the RHS's own type before aggregate assignment hints.
		if value, failure, handled := projectUnknownWrite(p.file, p.check, node, p.bindings, func(child *ast.Node) (*expression, *response) {
			value, failure := p.aggregateOperand(child, "")
			if failure.Status != "" {
				return nil, &failure
			}
			return value, nil
		}); handled {
			if failure != nil {
				return nil, *failure, true
			}
			// A constructor signature belongs to its construct leaf. Only a
			// function value carries a structural signature on the write itself.
			if value.Type != "function" {
				value.Signature = nil
			}
			return finish(value, response{})
		}
		binary := node.AsBinaryExpression()
		if operator := ordinaryAssignmentOperator(binary.OperatorToken.Kind); operator != "" {
			leaf := ast.SkipParentheses(binary.Left)
			if leaf.Kind == ast.KindIdentifier {
				binding, exists := p.bindings[p.check.GetSymbolAtLocation(leaf)]
				if exists && strings.HasPrefix(binding.Type, "a") {
					if operator != "=" {
						return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node), true
					}
					if !binding.Writable {
						return nil, failAt("r01-d-aggregate-write", "error", p.file, leaf), true
					}
					left, failure := p.expr(leaf, binding.Type)
					if failure.Status != "" {
						return nil, failure, true
					}
					right, failure := p.expr(binary.Right, binding.Type)
					if failure.Status != "" {
						return nil, failure, true
					}
					return finish(&expression{Kind: "write", Type: binding.Type, Operator: "=", Left: left, Right: right, Range: expressionOffset(p.file, node)}, response{})
				}

				if exists {
					if !binding.Writable {
						return nil, failAt("r01-d-scalar-type", "error", p.file, binary.Left), true
					}
					logical := operator == "&&=" || operator == "||="
					allowAbsent := operator == "=" || operator == "??=" || logical
					var shape *parameterPresence
					if allowAbsent {
						shape = binding.Presence
					}
					left, failure := p.aggregateOperand(binary.Left, binding.Type, shape)
					if failure.Status != "" {
						return nil, failure, true
					}
					rightType := binding.Type
					if operator == "<<=" || operator == ">>=" || operator == ">>>=" {
						rightType = "i32"
					}
					right, failure := p.aggregateOperand(binary.Right, rightType, shape)
					if failure.Status != "" {
						return nil, failure, true
					}
					if allowAbsent && !permittedPresence(right, binding.Presence) {
						return nil, failAt("r01-d-scalar-type", "error", p.file, binary.Right), true
					}
					if logical {
						if binding.Type != "bool" {
							return nil, failAt("r01-d-scalar-type", "error", p.file, binary.Left), true
						}
					} else if operator != "=" && operator != "??=" {
						floatOperation := operator == "+=" || operator == "-=" || operator == "*=" || operator == "/=" || operator == "%="
						if !integerKind(binding.Type) && !(p.integerTarget.floats && floatKind(binding.Type) && floatOperation) {
							return nil, failAt("r01-d-scalar-type", "error", p.file, binary.Left), true
						}
					}
					value := &expression{Kind: "write", Type: binding.Type, Operator: operator, Left: left, Right: right, Range: expressionOffset(p.file, node)}
					if logical {
						payload, u, n, valid := finitePresence(p.check.GetTypeAtLocation(node))
						if !valid || payload != "" && payload != binding.Type {
							return nil, failAt("r01-d-scalar-type", "error", p.file, node), true
						}
						value.PresenceFlow = &presenceFlow{Undefined: u, Null: n}
					}
					return finish(value, response{})
				}
			}
			if leaf.Kind != ast.KindPropertyAccessExpression {
				return nil, response{}, false
			}
			left, failure := p.fieldExpr(leaf, true)
			if failure.Status != "" {
				return nil, failure, true
			}
			// Aggregate payload writes currently synchronize only plain assignment.
			if operator == "??=" && strings.HasPrefix(left.Type, "a") {
				return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node), true
			}
			logical := operator == "&&=" || operator == "||="
			floatOperation := operator == "+=" || operator == "-=" || operator == "*=" || operator == "/=" || operator == "%="
			if logical && left.Type != "bool" || operator != "=" && operator != "??=" && !logical && !integerKind(left.Type) && !(p.integerTarget.floats && floatKind(left.Type) && floatOperation) {
				return nil, failAt("r01-d-scalar-type", "error", p.file, binary.Left), true
			}
			rightType := left.Type
			shift := operator == "<<=" || operator == ">>=" || operator == ">>>="
			if shift {
				rightType = "i32"
			}
			right, failure := p.expr(binary.Right, rightType)
			if failure.Status != "" {
				if shift && failure.Diagnostics[0].Code == "r01-d-scalar-type" {
					failure = failAt("r01-d-shift-count-type", "error", p.file, binary.Right)
				}
				return nil, failure, true
			}
			return finish(&expression{Kind: "fieldWrite", Type: left.Type, Operator: operator, Left: left, Right: right, Range: expressionOffset(p.file, node)}, response{})
		}
		op := integerOperator(binary.OperatorToken.Kind)
		if op == "" {
			return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node), true
		}
		comparison := op == "===" || op == "!==" || op == "<" || op == "<=" || op == ">" || op == ">="
		logical := op == "&&" || op == "||"
		shift := op == "<<" || op == ">>" || op == ">>>"
		context := expected
		if comparison || context == "" || context == "bool" {
			context = p.aggregateHint(binary.Left)
			if context == "" && !shift {
				context = p.aggregateHint(binary.Right)
			}
			if context == "" {
				context = "i32"
			}
		}
		if logical {
			if integerKind(p.aggregateHint(binary.Left)) || floatKind(p.aggregateHint(binary.Left)) {
				return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node), true
			}
			context = "bool"
		}
		left, failure := p.aggregateOperand(binary.Left, context)
		if failure.Status != "" {
			return nil, failure, true
		}
		rightType := left.Type
		if shift {
			rightType = "i32"
		}
		right, failure := p.aggregateOperand(binary.Right, rightType)
		if failure.Status != "" {
			if shift && failure.Diagnostics[0].Code == "r01-d-scalar-type" {
				failure = failAt("r01-d-shift-count-type", "error", p.file, binary.Right)
			}
			return nil, failure, true
		}
		t, kind := left.Type, "binary"
		if strings.HasPrefix(t, "a") {
			if op != "===" && op != "!==" {
				return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node), true
			}
			kind = "aggregateEquality"
		}
		floatOperation := p.integerTarget.floats && floatKind(t) && (comparison || op == "+" || op == "-" || op == "*" || op == "/" || op == "%")
		if !strings.HasPrefix(t, "a") && !logical && !integerKind(t) && !floatOperation && !(t == "bool" && (op == "===" || op == "!==")) {
			return nil, failAt("r01-d-scalar-type", "error", p.file, binary.Left), true
		}
		if comparison || logical {
			t = "bool"
		}
		if logical {
			kind = "shortCircuit"
		}
		return finish(&expression{Kind: kind, Type: t, Operator: op, Left: left, Right: right, Range: expressionOffset(p.file, node)}, response{})
	}
	if node.Kind == ast.KindPrefixUnaryExpression && p.involvesAggregate(node) {
		prefix := node.AsPrefixUnaryExpression()
		op := ""
		switch prefix.Operator {
		case ast.KindPlusToken:
			op = "+"
		case ast.KindMinusToken:
			op = "-"
		case ast.KindTildeToken:
			op = "~"
		case ast.KindExclamationToken:
			op = "!"
		}
		if op != "" {
			context := expected
			if context == "" {
				context = p.aggregateHint(prefix.Operand)
			}
			if op == "!" {
				hint := p.aggregateHint(prefix.Operand)
				if integerKind(hint) || floatKind(hint) {
					return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node), true
				}
				context = "bool"
			}
			operand, failure := p.expr(prefix.Operand, context)
			if failure.Status != "" {
				return nil, failure, true
			}
			if op != "!" && !integerKind(operand.Type) && !(p.integerTarget.floats && floatKind(operand.Type) && (op == "+" || op == "-")) {
				return nil, failAt("r01-d-scalar-type", "error", p.file, prefix.Operand), true
			}
			return finish(&expression{Kind: "unary", Type: operand.Type, Operator: op, Operand: operand, Range: expressionOffset(p.file, node)}, response{})
		}
	}
	if aggregateFieldMutation(node) {
		var operand *ast.Node
		var operator ast.Kind
		prefix := node.Kind == ast.KindPrefixUnaryExpression
		if prefix {
			operand = node.AsPrefixUnaryExpression().Operand
			operator = node.AsPrefixUnaryExpression().Operator
		} else {
			operand = node.AsPostfixUnaryExpression().Operand
			operator = node.AsPostfixUnaryExpression().Operator
		}
		left, failure := p.fieldExpr(ast.SkipParentheses(operand), true)
		if failure.Status != "" {
			return nil, failure, true
		}
		if !integerKind(left.Type) && !floatKind(left.Type) {
			return nil, failAt("r01-d-scalar-type", "error", p.file, node), true
		}
		op := "++"
		if operator == ast.KindMinusMinusToken {
			op = "--"
		}
		return finish(&expression{Kind: "fieldUpdate", Type: left.Type, Operator: op, Prefix: &prefix, Operand: left, Range: expressionOffset(p.file, node)}, response{})
	}
	return nil, response{}, false
}

func (p *projection) aggregateHint(node *ast.Node) string {
	node = ast.SkipParentheses(node)
	if integerLiteralNode(p.file, node) {
		return ""
	}
	if node.Kind == ast.KindPrefixUnaryExpression {
		if node.AsPrefixUnaryExpression().Operator == ast.KindExclamationToken {
			return "bool"
		}
		return p.aggregateHint(node.AsPrefixUnaryExpression().Operand)
	}
	if node.Kind == ast.KindBinaryExpression {
		binary := node.AsBinaryExpression()
		switch integerOperator(binary.OperatorToken.Kind) {
		case "===", "!==", "<", "<=", ">", ">=", "&&", "||":
			return "bool"
		}
		if hint := p.aggregateHint(binary.Left); hint != "" {
			return hint
		}
		return p.aggregateHint(binary.Right)
	}
	if node.Kind == ast.KindThisKeyword && p.self != nil {
		return p.aggregates.facts.Declarations[p.self.index].ID
	}
	if node.Kind == ast.KindIdentifier {
		if binding := p.bindings[p.check.GetSymbolAtLocation(node)]; binding.Type != "" {
			return referenceUseType(p.check, node, binding)
		}
	}
	if node.Kind == ast.KindCallExpression && node.AsCallExpression().Expression.Kind == ast.KindIdentifier {
		if target := p.functions[p.check.GetSymbolAtLocation(node.AsCallExpression().Expression)]; target.Signature.ReturnType != "" {
			return target.Signature.ReturnType
		}
	}
	t := p.check.GetTypeAtLocation(node)
	if name := p.aggregates.typeName(t); name != "" {
		return name
	}
	return scalar(t)
}

func (p *projection) involvesAggregate(node *ast.Node) bool {
	var visit func(*ast.Node) bool
	visit = func(n *ast.Node) bool {
		if n.Kind == ast.KindPropertyAccessExpression || n.Kind == ast.KindThisKeyword || n.Kind == ast.KindNewExpression {
			return true
		}
		if n.Kind == ast.KindIdentifier {
			if strings.HasPrefix(p.bindings[p.check.GetSymbolAtLocation(n)].Type, "a") {
				return true
			}
			if strings.HasPrefix(p.functions[p.check.GetSymbolAtLocation(n)].Signature.ReturnType, "a") {
				return true
			}
		}
		return n.ForEachChild(visit)
	}
	return visit(node)
}
