// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Projects scalar values and operators using official types and exact literal facts.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/scanner"
	"math/big"
	"strings"
)

func scalar(t *checker.Type) string {
	if t != nil && t.IsBooleanLike() {
		return "bool"
	}
	if t != nil && t.Alias() != nil && t.Alias().Symbol() != nil && (integerKind(t.Alias().Symbol().Name) || floatKind(t.Alias().Symbol().Name)) {
		return t.Alias().Symbol().Name
	}
	if t != nil && t.Flags()&checker.TypeFlagsNumberLike != 0 {
		return "f64"
	}
	return ""
}

func exactInteger(file *ast.SourceFile, expr *ast.Node) (*big.Int, *integerLiteral, bool) {
	negative := false
	operand := expr
	if expr.Kind == ast.KindPrefixUnaryExpression {
		prefix := expr.AsPrefixUnaryExpression()
		if prefix.Operator != ast.KindMinusToken || prefix.Operand.Kind != ast.KindNumericLiteral {
			return nil, nil, false
		}
		negative = true
		operand = prefix.Operand
	}
	if operand.Kind != ast.KindNumericLiteral {
		return nil, nil, false
	}
	start, end := scanner.SkipTrivia(file.Text(), operand.Pos()), operand.End()
	if start < 0 || end > len(file.Text()) || start >= end {
		return nil, nil, false
	}
	// Parse the entire source token, not a numeric suffix of a fractional literal.
	token := file.Text()[start:end]
	clean := strings.ReplaceAll(token, "_", "")
	base := 10
	digits := clean
	if len(clean) > 2 && clean[0] == '0' {
		switch clean[1] {
		case 'x', 'X':
			base, digits = 16, clean[2:]
		case 'b', 'B':
			base, digits = 2, clean[2:]
		case 'o', 'O':
			base, digits = 8, clean[2:]
		}
	}
	value, ok := new(big.Int).SetString(digits, base)
	if !ok {
		return nil, nil, false
	}
	if negative {
		value.Neg(value)
		token = "-" + token
	}
	return value, &integerLiteral{Decimal: value.String(), Token: token}, true
}

func i32InRange(value *big.Int) bool {
	return value.Cmp(big.NewInt(-2147483648)) >= 0 && value.Cmp(big.NewInt(2147483647)) <= 0
}

func integerLiteralNode(file *ast.SourceFile, node *ast.Node) bool {
	for node.Kind == ast.KindParenthesizedExpression {
		node = node.AsParenthesizedExpression().Expression
	}
	_, _, ok := exactInteger(file, node)
	return ok
}

func explicitFloatOperand(file *ast.SourceFile, check *checker.Checker, node *ast.Node) bool {
	for node.Kind == ast.KindParenthesizedExpression {
		node = node.AsParenthesizedExpression().Expression
	}
	if node.Kind != ast.KindIdentifier && node.Kind != ast.KindNumericLiteral {
		return false
	}
	return !integerLiteralNode(file, node) && scalar(check.GetTypeAtLocation(node)) == "f64"
}

func projectOperator(file *ast.SourceFile, check *checker.Checker, node *ast.Node, bindings map[*ast.Symbol]symbol, functions ...map[*ast.Symbol]functionInfo) (*expression, *response) {
	if value, failure, handled := projectExceptionTypeTest(file, check, node, func(child *ast.Node, want string) (*expression, *response) {
		var declared map[*ast.Symbol]functionInfo
		if len(functions) != 0 {
			declared = functions[0]
		}
		value, failure := projectExpression(file, check, child, want, bindings, declared)
		return &value, failure
	}); handled {
		return value, failure
	}
	if value, failure, handled := projectUnknownWrite(file, check, node, bindings, func(child *ast.Node) (*expression, *response) {
		return projectOperator(file, check, child, bindings, functions...)
	}); handled {
		return value, failure
	}
	if len(functions) != 0 {
		if value, failure, handled := projectOrdinaryFunctionValue(file, check, node, "", bindings, functions[0]); handled {
			return value, failure
		}
	}
	if node.Kind == ast.KindCallExpression && len(functions) != 0 {
		call := node.AsCallExpression()
		if moduleReference(check, call.Expression) {
			if target, ok := functions[0][check.GetSymbolAtLocation(call.Expression)]; ok {
				projected, failure := projectExpression(file, check, node, target.Signature.ReturnType, bindings, functions[0])
				if failure != nil {
					return nil, failure
				}
				return &projected, nil
			}
		}
	}
	if value, literal, ok := exactInteger(file, node); ok {
		if !i32InRange(value) {
			failure := failAt("r01-d-integer-range", "error", file, node)
			return nil, &failure
		}
		return &expression{Kind: "integer", Type: "i32", Literal: literal, Range: expressionOffset(file, node)}, nil
	}
	if node.Kind == ast.KindNumericLiteral {
		return &expression{Kind: "numeric", Type: "f64", Range: expressionOffset(file, node)}, nil
	}
	if moduleReference(check, node) {
		binding, ok := bindings[check.GetSymbolAtLocation(node)]
		if ok && binding.Type != "" {
			t := referenceUseType(check, node, binding)
			if t == "" {
				failure := failAt("r02b-d-unsupported", "unsupported", file, node)
				return nil, &failure
			}
			return &expression{Kind: "reference", Type: t, Symbol: binding.ID, Range: expressionOffset(file, node)}, nil
		}
	}
	if node.Kind == ast.KindTrueKeyword || node.Kind == ast.KindFalseKeyword {
		value := node.Kind == ast.KindTrueKeyword
		return &expression{Kind: "boolean", Type: "bool", Boolean: &value, Range: expressionOffset(file, node)}, nil
	}
	if node.Kind == ast.KindParenthesizedExpression {
		inner, failure := projectOperator(file, check, node.AsParenthesizedExpression().Expression, bindings, functions...)
		if failure != nil {
			return nil, failure
		}
		return &expression{Kind: "group", Type: inner.Type, Operand: inner, Range: expressionOffset(file, node)}, nil
	}
	if node.Kind == ast.KindPrefixUnaryExpression {
		prefix := node.AsPrefixUnaryExpression()
		operator, expected := "", "i32"
		switch prefix.Operator {
		case ast.KindExclamationToken:
			operator, expected = "!", "bool"
		case ast.KindTildeToken:
			operator = "~"
		case ast.KindPlusToken:
			operator = "+"
		case ast.KindMinusToken:
			operator = "-"
		}
		if operator != "" {
			operand, failure := projectOperator(file, check, prefix.Operand, bindings, functions...)
			if failure != nil {
				return nil, failure
			}
			if (operand.Type == "f64" && (operator == "+" || operator == "-")) ||
				(operator == "!" && (operand.Type == "i32" || operand.Type == "f64")) {
				failure := failAt("r02b-d-unsupported", "unsupported", file, node)
				return nil, &failure
			}
			if operand.Type != expected {
				failure := failAt("r01-d-scalar-type", "error", file, prefix.Operand)
				return nil, &failure
			}
			return &expression{Kind: "unary", Type: expected, Operator: operator, Operand: operand, Range: expressionOffset(file, node)}, nil
		}
	}
	if node.Kind == ast.KindBinaryExpression {
		binary := node.AsBinaryExpression()
		if binary.OperatorToken.Kind == ast.KindEqualsEqualsToken || binary.OperatorToken.Kind == ast.KindExclamationEqualsToken {
			failure := failAt("r02b-d-prohibited", "prohibited", file, binary.OperatorToken)
			return nil, &failure
		}
		operator := ""
		switch binary.OperatorToken.Kind {
		case ast.KindPlusToken:
			operator = "+"
		case ast.KindMinusToken:
			operator = "-"
		case ast.KindAsteriskToken:
			operator = "*"
		case ast.KindSlashToken:
			operator = "/"
		case ast.KindPercentToken:
			operator = "%"
		case ast.KindAmpersandToken:
			operator = "&"
		case ast.KindBarToken:
			operator = "|"
		case ast.KindCaretToken:
			operator = "^"
		case ast.KindLessThanLessThanToken:
			operator = "<<"
		case ast.KindGreaterThanGreaterThanToken:
			operator = ">>"
		case ast.KindGreaterThanGreaterThanGreaterThanToken:
			operator = ">>>"
		case ast.KindLessThanToken:
			operator = "<"
		case ast.KindLessThanEqualsToken:
			operator = "<="
		case ast.KindGreaterThanToken:
			operator = ">"
		case ast.KindGreaterThanEqualsToken:
			operator = ">="
		case ast.KindEqualsEqualsEqualsToken:
			operator = "==="
		case ast.KindExclamationEqualsEqualsToken:
			operator = "!=="
		case ast.KindAmpersandAmpersandToken:
			operator = "&&"
		case ast.KindBarBarToken:
			operator = "||"
		}
		if operator != "" {
			floatOperation := false
			switch operator {
			case "+", "-", "*", "/", "%", "<", "<=", ">", ">=", "===", "!==":
				floatOperation = true
			}
			if floatOperation && ((explicitFloatOperand(file, check, binary.Left) && integerLiteralNode(file, binary.Right)) ||
				(explicitFloatOperand(file, check, binary.Right) && integerLiteralNode(file, binary.Left))) {
				failure := failAt("r02b-d-unsupported", "unsupported", file, node)
				return nil, &failure
			}
			left, failure := projectOperator(file, check, binary.Left, bindings, functions...)
			if failure != nil {
				return nil, failure
			}
			right, failure := projectOperator(file, check, binary.Right, bindings, functions...)
			if failure != nil {
				return nil, failure
			}
			resultType := "i32"
			if operator == "<" || operator == "<=" || operator == ">" || operator == ">=" || operator == "===" || operator == "!==" || operator == "&&" || operator == "||" {
				resultType = "bool"
			}
			leftValid, rightValid := left.Type == "i32", right.Type == "i32"
			if operator == "===" || operator == "!==" {
				leftValid = left.Type == "i32" || left.Type == "bool"
				rightValid = right.Type == left.Type
			}
			if operator == "&&" || operator == "||" {
				leftValid, rightValid = left.Type == "bool", right.Type == "bool"
			}
			if floatOperation && left.Type == "f64" && right.Type == "f64" {
				failure := failAt("r02b-d-unsupported", "unsupported", file, node)
				return nil, &failure
			}
			leftNumeric := left.Type == "i32" || left.Type == "f64"
			rightNumeric := right.Type == "i32" || right.Type == "f64"
			if (operator == "&&" || operator == "||") && leftNumeric && rightNumeric {
				failure := failAt("r02b-d-unsupported", "unsupported", file, node)
				return nil, &failure
			}
			if !leftValid || !rightValid {
				code := "r01-d-scalar-type"
				invalid := binary.Left
				if leftValid {
					invalid = binary.Right
					if operator == "<<" || operator == ">>" || operator == ">>>" {
						code = "r01-d-shift-count-type"
					}
				}
				failure := failAt(code, "error", file, invalid)
				return nil, &failure
			}
			kind := "binary"
			if operator == "&&" || operator == "||" {
				kind = "shortCircuit"
			}
			return &expression{Kind: kind, Type: resultType, Operator: operator, Left: left, Right: right, Range: expressionOffset(file, node)}, nil
		}
	}
	failure := failAt("r02b-d-unsupported", "unsupported", file, node)
	return nil, &failure
}
