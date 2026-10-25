// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Projects integer, presence and conversion expressions with explicit official call identity.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

func projectIntegerExpression(file *ast.SourceFile, check *checker.Checker, node *ast.Node, expected string,
	refs map[*ast.Symbol]symbol, functions map[*ast.Symbol]functionInfo, target *integerTarget, operatorContext ...bool,
) (*expression, *response) {
	// Direct values/calls retain node.Pos. Legacy operator operands use token
	// starts; literal spelling and diagnostic anchors remain independent.
	failureAt := func(code, category string, at *ast.Node) (*expression, *response) {
		if at == nil {
			failure := fail(code, category, file.FileName(), span{})
			return nil, &failure
		}
		failure := failAt(code, category, file, at)
		return nil, &failure
	}
	finish := func(projected *expression) (*expression, *response) {
		if expected != "" && projected.Type != expected {
			return failureAt("r01-d-scalar-type", "error", node)
		}
		if expected != "" && !permittedPresence(projected, nil) && !(len(operatorContext) > 1 && operatorContext[1]) {
			return failureAt("r01-d-scalar-type", "error", node)
		}
		return projected, nil
	}
	if node == nil {
		return failureAt("r02b-d-unsupported", "unsupported", node)
	}
	if value, failure, handled := projectExceptionTypeTest(file, check, node, func(child *ast.Node, want string) (*expression, *response) {
		return projectIntegerExpression(file, check, child, want, refs, functions, target)
	}); handled {
		if failure != nil {
			return nil, failure
		}
		return finish(value)
	}
	if value, failure, handled := projectUnknownWrite(file, check, node, refs, func(child *ast.Node) (*expression, *response) {
		return projectIntegerExpression(file, check, child, "", refs, functions, target, true)
	}); handled {
		if failure != nil {
			return nil, failure
		}
		return finish(value)
	}
	var updateOperand *ast.Node
	var updateOperator ast.Kind
	prefixUpdate := node.Kind == ast.KindPrefixUnaryExpression
	if prefixUpdate {
		updateOperand, updateOperator = node.AsPrefixUnaryExpression().Operand, node.AsPrefixUnaryExpression().Operator
	} else if node.Kind == ast.KindPostfixUnaryExpression {
		updateOperand, updateOperator = node.AsPostfixUnaryExpression().Operand, node.AsPostfixUnaryExpression().Operator
	}
	if updateOperator == ast.KindPlusPlusToken || updateOperator == ast.KindMinusMinusToken {
		leaf := ast.SkipParentheses(updateOperand)
		if !moduleReference(check, leaf) {
			return failureAt("r02b-d-unsupported", "unsupported", updateOperand)
		}
		binding, ok := refs[check.GetSymbolAtLocation(leaf)]
		if !ok || !binding.Writable || !integerKind(binding.Type) && !(target.floats && floatKind(binding.Type)) {
			return failureAt("r01-d-scalar-type", "error", updateOperand)
		}
		operand, failure := projectIntegerExpression(file, check, updateOperand, binding.Type, refs, functions, target, true)
		if failure != nil {
			return nil, failure
		}
		operator := "++"
		if updateOperator == ast.KindMinusMinusToken {
			operator = "--"
		}
		return finish(&expression{Kind: "update", Type: binding.Type, Operator: operator, Prefix: &prefixUpdate, Operand: operand, Range: expressionOffset(file, node)})
	}
	valueRange := offset(file, node)
	if len(operatorContext) != 0 && operatorContext[0] {
		valueRange = expressionOffset(file, node)
	}
	if len(operatorContext) > 1 && operatorContext[1] && expected != "" {
		if state := missingState(check, node); state != "" && node.Kind != ast.KindParenthesizedExpression {
			return &expression{Kind: "absence", Type: expected, Absence: state, Range: valueRange}, nil
		}
	}
	// An existing declared nullable context owns its typed absence above. Only
	// an unconsumed standalone value uses v2's independent undefined variant.
	if target.exceptionNumericFacts {
		if value, failure, handled := projectExceptionNumeric(file, check, node, refs, functions, target); handled {
			if failure != nil {
				return nil, failure
			}
			return finish(value)
		}
	}
	if value, failure, handled := projectFunctionValueExpression(file, check, node, expected, refs, functions, valueRange, func(child *ast.Node, want string) (*expression, *response) {
		return projectIntegerExpression(file, check, child, want, refs, functions, target)
	}); handled {
		if failure != nil {
			return nil, failure
		}
		if value.Type == "f64" && !target.floats {
			return failureAt("r02b-d-unsupported", "unsupported", node)
		}
		return finish(value)
	}
	if target != nil && target.floats {
		if token, ok := floatingNode(file, node); ok && (floatKind(expected) || expected == "" && floatDefaultHint(file, node) != "") {
			kind := expected
			if kind == "" {
				kind = "f64"
			}
			bits, finite := exactFloatBits(token, kind)
			if !finite {
				return failureAt("r01-d-float-range", "error", node)
			}
			return &expression{Kind: "floating", Type: kind, Float: &floatLiteral{Token: token, Bits: bits}, Range: valueRange}, nil
		}
	}
	if value, literal, ok := exactInteger(file, node); ok {
		kind := expected
		if kind == "" {
			kind = "i32"
		}
		if kind == "f64" {
			return failureAt("r02b-d-unsupported", "unsupported", node)
		}
		if !integerKind(kind) {
			return failureAt("r01-d-scalar-type", "error", node)
		}
		if !integerRange(value, kind, target) {
			return failureAt("r01-d-integer-range", "error", node)
		}
		return &expression{Kind: "integer", Type: kind, Literal: literal, Range: valueRange}, nil
	}
	switch node.Kind {
	case ast.KindNumericLiteral:
		if integerKind(expected) || expected == "bool" {
			return failureAt("r01-d-scalar-type", "error", node)
		}
		return failureAt("r02b-d-unsupported", "unsupported", node)
	case ast.KindIdentifier, ast.KindPropertyAccessExpression:
		ref, ok := refs[check.GetSymbolAtLocation(node)]
		if !ok || ref.Type == "" {
			return failureAt("r02b-d-type", "error", node)
		}
		// Preserve officially checked scalar reference facts. Unsupported
		// f64 signatures are still rejected by the owned HIR capability seam.
		t := referenceUseType(check, node, ref)
		if t == "" {
			return failureAt("r02b-d-unsupported", "unsupported", node)
		}
		projected := &expression{Kind: "reference", Type: t, Symbol: ref.ID, Range: valueRange}
		if ref.Presence != nil && (ref.Presence.Undefined || ref.Presence.Null) {
			payload, u, n, valid := finitePresence(check.GetTypeAtLocation(node))
			if !valid || payload != "" && payload != ref.Type || u && !ref.Presence.Undefined || n && !ref.Presence.Null {
				return failureAt("r01-d-scalar-type", "error", node)
			}
			projected.PresenceFlow = &presenceFlow{Undefined: u, Null: n}
		}
		return finish(projected)
	case ast.KindTrueKeyword, ast.KindFalseKeyword:
		value := node.Kind == ast.KindTrueKeyword
		return finish(&expression{Kind: "boolean", Type: "bool", Boolean: &value, Range: valueRange})
	case ast.KindParenthesizedExpression:
		inner, failure := projectIntegerExpression(file, check, node.AsParenthesizedExpression().Expression, expected, refs, functions, target, true, len(operatorContext) > 1 && operatorContext[1])
		if failure != nil {
			return nil, failure
		}
		return finish(&expression{Kind: "group", Type: inner.Type, Operand: inner, Range: expressionOffset(file, node)})
	case ast.KindCallExpression:
		call := node.AsCallExpression()
		if !moduleReference(check, call.Expression) || call.QuestionDotToken != nil || call.TypeArguments != nil {
			return failureAt("r02b-d-unsupported", "unsupported", node)
		}
		fn, ok := functions[check.GetSymbolAtLocation(call.Expression)]
		if !ok {
			failure := failCallAt("r01-d-unresolved-name", file, call.Expression)
			return nil, &failure
		}
		if expected != "" && expected != fn.Signature.ReturnType {
			failure := failCallAt("r01-d-scalar-type", file, node)
			return nil, &failure
		}
		arityOK := len(call.Arguments.Nodes) <= len(fn.Signature.Parameters)
		for index := len(call.Arguments.Nodes); index < len(fn.Signature.Parameters); index++ {
			arityOK = arityOK && len(fn.Signature.Presence) > index && fn.Signature.Presence[index] != nil && fn.Signature.Presence[index].Optional
		}
		if !arityOK {
			failure := failCallAt("r02b-d-type", file, node)
			return nil, &failure
		}
		if fn.Declaration != nil {
			resolved := check.GetResolvedSignature(node)
			if !resolvedCallMatches(file, check, resolved, fn) {
				return failureAt("r01-d-conversion-entry", "error", node)
			}
		}
		args := make([]expression, 0, len(call.Arguments.Nodes))
		for index, arg := range call.Arguments.Nodes {
			if state := missingState(check, arg); state != "" {
				var shape *parameterPresence
				if len(fn.Signature.Presence) > index {
					shape = fn.Signature.Presence[index]
				}
				if shape == nil || state == "undefined" && !shape.Undefined || state == "null" && !shape.Null {
					return failureAt("r01-d-scalar-type", "error", arg)
				}
				// Project the original AST below so each parenthesized layer and
				// the checked builtin leaf retain their own source anchors.
			}
			var shape *parameterPresence
			if len(fn.Signature.Presence) > index {
				shape = fn.Signature.Presence[index]
			}
			projected, failure := projectIntegerExpression(file, check, arg, fn.Signature.Parameters[index], refs, functions, target, false, shape != nil && (shape.Undefined || shape.Null))
			if failure != nil {
				if failure.Diagnostics[0].Code == "r01-d-scalar-type" {
					failure.Diagnostics[0].RuleID = "r01-calls"
					failure.Diagnostics[0].RuleIDs = []string{"r01-calls"}
				}
				return nil, failure
			}
			if !permittedPresence(projected, shape) {
				return failureAt("r01-d-scalar-type", "error", arg)
			}
			args = append(args, *projected)
		}
		if fn.Signature.ReturnType == "f64" && !target.floats {
			return failureAt("r02b-d-unsupported", "unsupported", node)
		}
		if fn.Conversion != nil {
			return &expression{Kind: "conversion", Type: fn.Signature.ReturnType, Operand: &args[0], Conversion: fn.Conversion, Range: expressionOffset(file, node)}, nil
		}
		sig := fn.Signature
		value := &expression{Kind: "call", Type: sig.ReturnType, Target: fn.ID, Signature: &sig, Arguments: &args, Range: offset(file, node)}
		if declared := sig.ReturnPresence; declared != nil {
			// The signature retains the callee's declaration anchors, while
			// the call expression owns independently checked use-site flow.
			payload, undefined, null, valid := finitePresence(check.GetTypeAtLocation(node))
			if !valid || payload != "" && payload != sig.ReturnType || undefined && !declared.Undefined || null && !declared.Null {
				return failureAt("r01-d-scalar-type", "error", node)
			}
			value.PresenceFlow = &presenceFlow{Undefined: undefined, Null: null}
			return finish(value)
		}
		return value, nil
	case ast.KindPrefixUnaryExpression:
		prefix := node.AsPrefixUnaryExpression()
		operator, context := "", expected
		switch prefix.Operator {
		case ast.KindExclamationToken:
			operator, context = "!", "bool"
		case ast.KindTildeToken:
			operator = "~"
		case ast.KindPlusToken:
			operator = "+"
		case ast.KindMinusToken:
			operator = "-"
		}
		if operator == "" {
			return failureAt("r02b-d-unsupported", "unsupported", node)
		}
		if context == "" || (context == "bool" && operator != "!") {
			context = integerHint(prefix.Operand, check, refs, functions)
			if context == "" {
				if target.floats {
					context = floatDefaultHint(file, prefix.Operand)
				}
			}
			if context == "" {
				context = "i32"
			}
		}
		if operator == "!" {
			operandHint := integerHint(prefix.Operand, check, refs, functions)
			if integerKind(operandHint) || operandHint == "f64" || integerLiteralNode(file, prefix.Operand) || prefix.Operand.Kind == ast.KindNumericLiteral {
				return failureAt("r02b-d-unsupported", "unsupported", node)
			}
		}
		operand, failure := projectIntegerExpression(file, check, prefix.Operand, context, refs, functions, target, true)
		if failure != nil {
			return nil, failure
		}
		if operand.Type == "f64" && (operator == "+" || operator == "-") && !target.floats {
			return failureAt("r02b-d-unsupported", "unsupported", node)
		}
		if operator != "!" && !integerKind(operand.Type) && !(target.floats && floatKind(operand.Type) && (operator == "+" || operator == "-")) {
			return failureAt("r01-d-scalar-type", "error", prefix.Operand)
		}
		return finish(&expression{Kind: "unary", Type: operand.Type, Operator: operator, Operand: operand, Range: expressionOffset(file, node)})
	case ast.KindBinaryExpression:
		binary := node.AsBinaryExpression()
		if binary.OperatorToken.Kind == ast.KindQuestionQuestionToken {
			value, failure := projectNullishCoalesce(file, check, node, refs, functions, target)
			if failure != nil {
				return nil, failure
			}
			return finish(value)
		}
		if binary.OperatorToken.Kind == ast.KindEqualsEqualsToken || binary.OperatorToken.Kind == ast.KindExclamationEqualsToken {
			return failureAt("r02b-d-prohibited", "prohibited", binary.OperatorToken)
		}
		if binary.OperatorToken.Kind == ast.KindEqualsEqualsEqualsToken || binary.OperatorToken.Kind == ast.KindExclamationEqualsEqualsToken {
			reference, sentinel := binary.Left, binary.Right
			reversed := false
			if missingState(check, reference) != "" {
				reference, sentinel = binary.Right, binary.Left
				reversed = true
			}
			leaf := ast.SkipParentheses(reference)
			if state := missingState(check, sentinel); state != "" && leaf.Kind == ast.KindIdentifier {
				binding, ok := refs[check.GetSymbolAtLocation(leaf)]
				if !ok || binding.Presence == nil || !binding.Presence.Undefined && !binding.Presence.Null {
					return failureAt("r02b-d-unsupported", "unsupported", node)
				}
				owned, failure := projectIntegerExpression(file, check, reference, binding.Type, refs, functions, target, true, true)
				if failure != nil {
					return nil, failure
				}
				absent, failure := projectIntegerExpression(file, check, sentinel, binding.Type, refs, functions, target, true, true)
				if failure != nil {
					return nil, failure
				}
				left, right := owned, absent
				if reversed {
					left, right = absent, owned
				}
				return finish(&expression{Kind: "presenceTest", Type: "bool", Operator: integerOperator(binary.OperatorToken.Kind), Left: left, Right: right, Range: expressionOffset(file, node)})
			}
		}
		if operator := ordinaryAssignmentOperator(binary.OperatorToken.Kind); operator != "" {
			logical := operator == "&&=" || operator == "||="
			leaf := ast.SkipParentheses(binary.Left)
			if !moduleReference(check, leaf) {
				return failureAt("r02b-d-unsupported", "unsupported", binary.Left)
			}
			binding, ok := refs[check.GetSymbolAtLocation(leaf)]
			if !ok || !binding.Writable {
				return failureAt("r01-d-scalar-type", "error", binary.Left)
			}
			left, failure := projectIntegerExpression(file, check, binary.Left, binding.Type, refs, functions, target, true, operator == "=" || operator == "??=" || logical)
			if failure != nil {
				return nil, failure
			}
			rightType := binding.Type
			if operator == "<<=" || operator == ">>=" || operator == ">>>=" {
				rightType = "i32"
			}
			right, failure := projectIntegerExpression(file, check, binary.Right, rightType, refs, functions, target, true, (operator == "=" || operator == "??=" || logical) && binding.Presence != nil && (binding.Presence.Undefined || binding.Presence.Null))
			if failure != nil {
				return nil, failure
			}
			if (operator == "=" || operator == "??=" || logical) && !permittedPresence(right, binding.Presence) {
				return failureAt("r01-d-scalar-type", "error", binary.Right)
			}
			if operator == "&&=" || operator == "||=" {
				if binding.Type != "bool" {
					return failureAt("r01-d-scalar-type", "error", binary.Left)
				}
			} else if operator != "=" && operator != "??=" {
				floatOperation := operator == "+=" || operator == "-=" || operator == "*=" || operator == "/=" || operator == "%="
				if !integerKind(binding.Type) && !(target.floats && floatKind(binding.Type) && floatOperation) {
					return failureAt("r01-d-scalar-type", "error", binary.Left)
				}
			}
			projected := &expression{Kind: "write", Type: binding.Type, Operator: operator, Left: left, Right: right, Range: expressionOffset(file, node)}
			if binding.Signature != nil {
				var valid bool
				projected.Signature, valid = functionValueType(check, check.GetTypeAtLocation(node))
				if !valid {
					return failureAt("r02b-d-unsupported", "unsupported", node)
				}
			}
			if logical {
				payload, u, n, valid := finitePresence(check.GetTypeAtLocation(node))
				if !valid || payload != "" && payload != binding.Type {
					return failureAt("r01-d-scalar-type", "error", node)
				}
				projected.PresenceFlow = &presenceFlow{Undefined: u, Null: n}
			}
			return finish(projected)
		}
		operator := integerOperator(binary.OperatorToken.Kind)
		if operator == "" {
			return failureAt("r02b-d-unsupported", "unsupported", node)
		}
		comparison := operator == "<" || operator == "<=" || operator == ">" || operator == ">=" || operator == "===" || operator == "!=="
		logical := operator == "&&" || operator == "||"
		shift := operator == "<<" || operator == ">>" || operator == ">>>"
		context := expected
		if comparison || context == "" || context == "bool" {
			context = integerHint(binary.Left, check, refs, functions)
			if context == "" && !shift {
				context = integerHint(binary.Right, check, refs, functions)
			}
			if context == "" {
				if target.floats {
					context = floatDefaultHint(file, binary.Left)
					if context == "" {
						context = floatDefaultHint(file, binary.Right)
					}
				}
			}
			if context == "" {
				context = "i32"
			}
		}
		if logical {
			leftHint := integerHint(binary.Left, check, refs, functions)
			rightHint := integerHint(binary.Right, check, refs, functions)
			leftNumeric := integerKind(leftHint) || leftHint == "f64" || integerLiteralNode(file, binary.Left) || binary.Left.Kind == ast.KindNumericLiteral
			rightNumeric := integerKind(rightHint) || rightHint == "f64" || integerLiteralNode(file, binary.Right) || binary.Right.Kind == ast.KindNumericLiteral
			if leftNumeric && rightNumeric {
				return failureAt("r02b-d-unsupported", "unsupported", node)
			}
			context = "bool"
		}
		if context == "f64" && !target.floats {
			return failureAt("r02b-d-unsupported", "unsupported", node)
		}
		left, failure := projectIntegerExpression(file, check, binary.Left, context, refs, functions, target, true)
		if failure != nil {
			return nil, failure
		}
		rightContext := left.Type
		if shift {
			rightContext = "i32"
		}
		right, failure := projectIntegerExpression(file, check, binary.Right, rightContext, refs, functions, target, true)
		if failure != nil {
			if shift && failure.Diagnostics[0].Code == "r01-d-scalar-type" {
				failure = new(failAt("r01-d-shift-count-type", "error", file, binary.Right))
			}
			return nil, failure
		}
		floatOperation := target.floats && floatKind(left.Type) && (comparison || operator == "+" || operator == "-" || operator == "*" || operator == "/" || operator == "%")
		if !logical && !integerKind(left.Type) && !floatOperation && !(left.Type == "bool" && (operator == "===" || operator == "!==")) {
			return failureAt("r01-d-scalar-type", "error", binary.Left)
		}
		resultType, kind := left.Type, "binary"
		if comparison || logical {
			resultType = "bool"
		}
		if logical {
			kind = "shortCircuit"
		}
		return finish(&expression{Kind: kind, Type: resultType, Operator: operator, Left: left, Right: right, Range: expressionOffset(file, node)})
	}
	return failureAt("r02b-d-unsupported", "unsupported", node)
}
