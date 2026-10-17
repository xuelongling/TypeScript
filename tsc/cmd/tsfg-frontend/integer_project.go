// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Checks integer target and contextual facts; expression projection lives in integer_expression.go.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"math/big"
)

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
	case ast.KindIdentifier, ast.KindPropertyAccessExpression:
		if moduleReference(check, node) {
			return referenceUseType(check, node, refs[check.GetSymbolAtLocation(node)])
		}
	case ast.KindCallExpression:
		call := node.AsCallExpression()
		if moduleReference(check, call.Expression) {
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

func integerContextBindings(file *ast.SourceFile, check *checker.Checker, decl *ast.FunctionDeclaration,
	functions map[*ast.Symbol]functionInfo, target *integerTarget, inherited map[*ast.Symbol]symbol,
) map[*ast.Symbol]symbol {
	// Copy the checked outer environment before inferring local initializers.
	// Parameters and locals keep their own official symbol identities.
	refs := map[*ast.Symbol]symbol{}
	for resolved, binding := range inherited {
		refs[resolved] = binding
	}
	for resolved, binding := range parameterBindings(file, check, decl.Parameters.Nodes, nil) {
		refs[resolved] = binding
	}
	visitStatements(decl.Body.AsBlock().Statements.Nodes, func(item *ast.Node) bool {
		for _, candidate := range localDeclarations(item) {
			if candidate.Kind != ast.KindVariableDeclaration || candidate.Name() == nil || candidate.Name().Kind != ast.KindIdentifier {
				continue
			}
			local := candidate.AsVariableDeclaration()
			kind := ""
			var shape *parameterPresence
			if local.Type != nil {
				kind, shape, _ = checkedLocalPresence(file, check, candidate)
			}
			if local.Initializer != nil {
				projected, failure := projectIntegerExpression(file, check, local.Initializer, kind, refs, functions, target, false, shape != nil)
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
			refs[check.GetSymbolAtLocation(candidate.Name())] = symbol{Type: kind, Presence: shape, Writable: item.AsVariableStatement().DeclarationList.Flags&ast.NodeFlagsLet != 0}
		}
		return false
	})
	return refs
}

// Adapt only a 2322/2345 inside its actual scalar context, after fully checking
// that expression. Other official errors (name, const write, syntax, arity,
// operator legality) retain their original upstream identity and source span.
func integerContextDiagnostic(file *ast.SourceFile, check *checker.Checker, d *ast.Diagnostic,
	functions map[*ast.Symbol]functionInfo, target *integerTarget,
) (bool, *response) {
	if d.File() != file || (d.Code() != 2322 && d.Code() != 2345) {
		return false, nil
	}
	for _, fn := range file.Statements.Nodes {
		if fn.Kind != ast.KindFunctionDeclaration || fn.Body() == nil || fn.Body().Kind != ast.KindBlock {
			continue
		}
		decl := fn.AsFunctionDeclaration()
		refs := integerContextBindings(file, check, decl, functions, target, nil)
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
			var shape *parameterPresence
			if d.Code() == 2345 && node.Kind == ast.KindCallExpression {
				call := node.AsCallExpression()
				if !moduleReference(check, call.Expression) || call.QuestionDotToken != nil || call.TypeArguments != nil {
					return false
				}
				callee, ok := functions[check.GetSymbolAtLocation(call.Expression)]
				if !ok || len(call.Arguments.Nodes) > len(callee.Signature.Parameters) {
					return false
				}
				for index, argument := range call.Arguments.Nodes {
					if d.Pos() >= argument.Pos() && d.End() <= argument.End() {
						expr, expected = argument, callee.Signature.Parameters[index]
						if len(callee.Signature.Presence) > index {
							shape = callee.Signature.Presence[index]
						}
						break
					}
				}
			}
			if d.Code() == 2322 {
				switch node.Kind {
				case ast.KindVariableDeclaration:
					local := node.AsVariableDeclaration()
					if local.Type != nil {
						expr = local.Initializer
						expected, shape, _ = checkedLocalPresence(file, check, node)
					}
				case ast.KindParameter:
					parameter := node.AsParameterDeclaration()
					if parameter.Type != nil {
						expr = parameter.Initializer
						expected, _, _, _ = finitePresence(check.GetTypeFromTypeNode(parameter.Type))
						shape = checkedInitializerPresence(check, node)
					}
				case ast.KindReturnStatement:
					if decl.Type != nil {
						expr, expected = node.AsReturnStatement().Expression, scalar(check.GetTypeFromTypeNode(decl.Type))
					}
				case ast.KindExpressionStatement:
					candidate := node.AsExpressionStatement().Expression
					expr, expected = candidate, integerHint(candidate, check, refs, functions)
				}
			}
			if expr == nil || (!integerKind(expected) && expected != "bool" && !floatKind(expected)) {
				return false
			}
			projected, projectionFailure := projectIntegerExpression(file, check, expr, expected, refs, functions, target, false, shape != nil && (shape.Undefined || shape.Null))
			failure = projectionFailure
			if failure == nil && !permittedPresence(projected, shape) {
				failure = new(failAt("r01-d-scalar-type", "error", file, expr))
			}
			matched = true
			return true
		}
		for _, parameter := range decl.Parameters.Nodes {
			if visit(parameter) {
				break
			}
		}
		if !matched {
			fn.Body().ForEachChild(visit)
		}
		if matched {
			return true, failure
		}
	}
	return false, nil
}
