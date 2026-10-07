// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"math/big"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

// The target is an admitted producer input, never the Go host's word size.
type integerTarget struct {
	Target         string `json:"target"`
	Triple         string `json:"triple"`
	LLVMDataLayout string `json:"llvmDataLayout"`
	PointerBits    string `json:"pointerBits"`
	floats         bool
}

func validIntegerTarget(t *integerTarget) bool {
	return t != nil && t.Target == "windows-x86_64-msvc" && t.Triple == "x86_64-pc-windows-msvc" &&
		t.LLVMDataLayout == "e-m:w-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128" && t.PointerBits == "64"
}

func integerKind(t string) bool {
	switch t {
	case "i8", "i16", "i32", "i64", "u8", "u16", "u32", "u64", "isize", "usize":
		return true
	}
	return false
}

func integerRange(value *big.Int, kind string, target *integerTarget) bool {
	width := uint(0)
	switch kind {
	case "i8", "u8":
		width = 8
	case "i16", "u16":
		width = 16
	case "i32", "u32":
		width = 32
	case "i64", "u64":
		width = 64
	case "isize", "usize":
		if !validIntegerTarget(target) {
			return false
		}
		// Only the admitted 64-bit target is supported by this producer version.
		bits, ok := new(big.Int).SetString(target.PointerBits, 10)
		if !ok || !bits.IsUint64() {
			return false
		}
		width = uint(bits.Uint64())
	default:
		return false
	}
	limit := new(big.Int).Lsh(big.NewInt(1), width)
	if kind[0] == 'i' {
		limit.Rsh(limit, 1)
		minimum := new(big.Int).Neg(new(big.Int).Set(limit))
		return value.Cmp(minimum) >= 0 && value.Cmp(limit) < 0
	}
	return value.Sign() >= 0 && value.Cmp(limit) < 0
}

// Hints supply literal context only. References keep their own type, so hints
// cannot convert an already typed value or an unannotated default-i32 local.
func integerHint(node *ast.Node, check *checker.Checker, refs map[*ast.Symbol]symbol, functions map[*ast.Symbol]functionInfo) string {
	if node == nil {
		return ""
	}
	switch node.Kind {
	case ast.KindIdentifier:
		return refs[check.GetSymbolAtLocation(node)].Type
	case ast.KindCallExpression:
		call := node.AsCallExpression()
		if call.Expression.Kind == ast.KindIdentifier {
			return functions[check.GetSymbolAtLocation(call.Expression)].Signature.ReturnType
		}
	case ast.KindParenthesizedExpression:
		return integerHint(node.AsParenthesizedExpression().Expression, check, refs, functions)
	case ast.KindPrefixUnaryExpression:
		if node.AsPrefixUnaryExpression().Operator == ast.KindExclamationToken {
			return "bool"
		}
		return integerHint(node.AsPrefixUnaryExpression().Operand, check, refs, functions)
	case ast.KindBinaryExpression:
		binary := node.AsBinaryExpression()
		switch integerOperator(binary.OperatorToken.Kind) {
		case "<", "<=", ">", ">=", "===", "!==", "&&", "||":
			return "bool"
		}
		if hint := integerHint(binary.Left, check, refs, functions); hint != "" {
			return hint
		}
		return integerHint(binary.Right, check, refs, functions)
	case ast.KindTrueKeyword, ast.KindFalseKeyword:
		return "bool"
	}
	return ""
}

func integerOperator(kind ast.Kind) string {
	switch kind {
	case ast.KindPlusToken:
		return "+"
	case ast.KindMinusToken:
		return "-"
	case ast.KindAsteriskToken:
		return "*"
	case ast.KindSlashToken:
		return "/"
	case ast.KindPercentToken:
		return "%"
	case ast.KindAmpersandToken:
		return "&"
	case ast.KindBarToken:
		return "|"
	case ast.KindCaretToken:
		return "^"
	case ast.KindLessThanLessThanToken:
		return "<<"
	case ast.KindGreaterThanGreaterThanToken:
		return ">>"
	case ast.KindGreaterThanGreaterThanGreaterThanToken:
		return ">>>"
	case ast.KindLessThanToken:
		return "<"
	case ast.KindLessThanEqualsToken:
		return "<="
	case ast.KindGreaterThanToken:
		return ">"
	case ast.KindGreaterThanEqualsToken:
		return ">="
	case ast.KindEqualsEqualsEqualsToken:
		return "==="
	case ast.KindExclamationEqualsEqualsToken:
		return "!=="
	case ast.KindAmpersandAmpersandToken:
		return "&&"
	case ast.KindBarBarToken:
		return "||"
	}
	return ""
}

func projectIntegerExpression(file *ast.SourceFile, check *checker.Checker, node *ast.Node, expected string,
	refs map[*ast.Symbol]symbol, functions map[*ast.Symbol]functionInfo, target *integerTarget, operatorContext ...bool) (*expression, *response) {
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
		return projected, nil
	}
	if node == nil {
		return failureAt("r02b-d-unsupported", "unsupported", node)
	}
	valueRange := offset(file, node)
	if len(operatorContext) != 0 && operatorContext[0] {
		valueRange = expressionOffset(file, node)
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
	case ast.KindIdentifier:
		ref, ok := refs[check.GetSymbolAtLocation(node)]
		if !ok || ref.Type == "" {
			return failureAt("r02b-d-type", "error", node)
		}
		// Preserve officially checked scalar reference facts. Unsupported
		// f64 signatures are still rejected by the owned HIR capability seam.
		return finish(&expression{Kind: "reference", Type: ref.Type, Symbol: ref.ID, Range: valueRange})
	case ast.KindTrueKeyword, ast.KindFalseKeyword:
		value := node.Kind == ast.KindTrueKeyword
		return finish(&expression{Kind: "boolean", Type: "bool", Boolean: &value, Range: valueRange})
	case ast.KindParenthesizedExpression:
		inner, failure := projectIntegerExpression(file, check, node.AsParenthesizedExpression().Expression, expected, refs, functions, target, true)
		if failure != nil {
			return nil, failure
		}
		return finish(&expression{Kind: "group", Type: inner.Type, Operand: inner, Range: expressionOffset(file, node)})
	case ast.KindCallExpression:
		call := node.AsCallExpression()
		if call.Expression.Kind != ast.KindIdentifier || call.QuestionDotToken != nil || call.TypeArguments != nil {
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
		if len(call.Arguments.Nodes) != len(fn.Signature.Parameters) {
			failure := failCallAt("r02b-d-type", file, node)
			return nil, &failure
		}
		if fn.Conversion != nil {
			resolved := check.GetResolvedSignature(node)
			if resolved == nil || resolved.Declaration() != fn.Declaration {
				return failureAt("r01-d-conversion-entry", "error", node)
			}
		}
		args := make([]expression, 0, len(call.Arguments.Nodes))
		for index, arg := range call.Arguments.Nodes {
			projected, failure := projectIntegerExpression(file, check, arg, fn.Signature.Parameters[index], refs, functions, target)
			if failure != nil {
				if failure.Diagnostics[0].Code == "r01-d-scalar-type" {
					failure.Diagnostics[0].RuleID = "r01-calls"
					failure.Diagnostics[0].RuleIDs = []string{"r01-calls"}
				}
				return nil, failure
			}
			args = append(args, *projected)
		}
		if fn.Signature.ReturnType == "f64" && !target.floats {
			return failureAt("r02b-d-unsupported", "unsupported", node)
		}
		if fn.Conversion != nil {
			return &expression{Kind: "conversion", Type: fn.Signature.ReturnType, Operand: &args[0], Conversion: fn.Conversion, Range: offset(file, node)}, nil
		}
		sig := fn.Signature
		return &expression{Kind: "call", Type: sig.ReturnType, Target: fn.ID, Signature: &sig, Arguments: &args, Range: offset(file, node)}, nil
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
		if binary.OperatorToken.Kind == ast.KindEqualsEqualsToken || binary.OperatorToken.Kind == ast.KindExclamationEqualsToken {
			return failureAt("r02b-d-prohibited", "prohibited", binary.OperatorToken)
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
				failure = responsePointer(failAt("r01-d-shift-count-type", "error", file, binary.Right))
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

func responsePointer(r response) *response { return &r }

func integerContextBindings(file *ast.SourceFile, check *checker.Checker, decl *ast.FunctionDeclaration,
	functions map[*ast.Symbol]functionInfo, target *integerTarget) map[*ast.Symbol]symbol {
	refs := parameterBindings(check, decl.Parameters.Nodes, nil)
	visitStatements(decl.Body.AsBlock().Statements.Nodes, func(item *ast.Node) bool {
		for _, candidate := range localDeclarations(item) {
			if candidate.Kind != ast.KindVariableDeclaration || candidate.Name() == nil || candidate.Name().Kind != ast.KindIdentifier {
				continue
			}
			local := candidate.AsVariableDeclaration()
			kind := ""
			if local.Type != nil {
				kind = scalar(check.GetTypeFromTypeNode(local.Type))
			}
			if local.Initializer != nil {
				projected, failure := projectIntegerExpression(file, check, local.Initializer, kind, refs, functions, target)
				if failure == nil {
					kind = projected.Type
				} else if local.Type == nil && failure.Diagnostics[0].Code == "r01-d-integer-range" {
					// Invalid values still have a nominal context. Keep that
					// context for related checker diagnostics; the complete
					// initializer projection emits the original range failure.
					kind = integerHint(local.Initializer, check, refs, functions)
					if kind == "" {
						kind = "i32"
					}
				}
			}
			refs[check.GetSymbolAtLocation(candidate.Name())] = symbol{Type: kind}
		}
		return false
	})
	return refs
}

// Adapt only a 2322/2345 inside its actual scalar context, after fully checking
// that expression. Other official errors (name, const write, syntax, arity,
// operator legality) retain their original upstream identity and source span.
func integerContextDiagnostic(file *ast.SourceFile, check *checker.Checker, d *ast.Diagnostic,
	functions map[*ast.Symbol]functionInfo, target *integerTarget) (bool, *response) {
	if d.File() != file || (d.Code() != 2322 && d.Code() != 2345) {
		return false, nil
	}
	for _, fn := range file.Statements.Nodes {
		if fn.Kind != ast.KindFunctionDeclaration || fn.Body() == nil || fn.Body().Kind != ast.KindBlock {
			continue
		}
		decl := fn.AsFunctionDeclaration()
		refs := integerContextBindings(file, check, decl, functions, target)
		var matched bool
		var failure *response
		var visit func(*ast.Node) bool
		visit = func(node *ast.Node) bool {
			if d.Pos() < node.Pos() || d.End() > node.End() {
				return false
			}
			if node.ForEachChild(visit) {
				return true
			}
			var expr *ast.Node
			expected := ""
			if d.Code() == 2345 && node.Kind == ast.KindCallExpression {
				call := node.AsCallExpression()
				if call.Expression.Kind != ast.KindIdentifier || call.QuestionDotToken != nil || call.TypeArguments != nil {
					return false
				}
				callee, ok := functions[check.GetSymbolAtLocation(call.Expression)]
				if !ok || len(call.Arguments.Nodes) != len(callee.Signature.Parameters) {
					return false
				}
				for index, argument := range call.Arguments.Nodes {
					if d.Pos() >= argument.Pos() && d.End() <= argument.End() {
						expr, expected = argument, callee.Signature.Parameters[index]
						break
					}
				}
			}
			if d.Code() == 2322 {
				switch node.Kind {
				case ast.KindVariableDeclaration:
					local := node.AsVariableDeclaration()
					if local.Type != nil {
						expr, expected = local.Initializer, scalar(check.GetTypeFromTypeNode(local.Type))
					}
				case ast.KindReturnStatement:
					if decl.Type != nil {
						expr, expected = node.AsReturnStatement().Expression, scalar(check.GetTypeFromTypeNode(decl.Type))
					}
				case ast.KindExpressionStatement:
					candidate := node.AsExpressionStatement().Expression
					if candidate.Kind == ast.KindBinaryExpression && candidate.AsBinaryExpression().OperatorToken.Kind == ast.KindEqualsToken {
						binary := candidate.AsBinaryExpression()
						expr = binary.Right
						expected = refs[check.GetSymbolAtLocation(binary.Left)].Type
					}
				}
			}
			if expr == nil || (!integerKind(expected) && expected != "bool" && !floatKind(expected)) {
				return false
			}
			_, failure = projectIntegerExpression(file, check, expr, expected, refs, functions, target)
			matched = true
			return true
		}
		fn.Body().ForEachChild(visit)
		if matched {
			return true, failure
		}
	}
	return false, nil
}
