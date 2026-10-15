// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Maps precise scalar source diagnostics without changing unrelated official errors.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/scanner"
	"math/big"
	"strings"
)

func fail(code, category, file string, location span) response {
	phase := "syntax"
	if code == "r01-d-scalar-type" || code == "r01-d-integer-range" || code == "r01-d-float-range" || code == "r01-d-shift-count-type" {
		phase = "types"
	} else if category == "unsupported" {
		phase = "capability"
	}
	rule := "r01-ts-baseline"
	if code == "r01-d-integer-range" || code == "r01-d-float-range" || code == "r01-d-scalar-type" {
		rule = "r01-literals"
	}
	if code == "r01-d-shift-count-type" {
		rule = "r01-shifts"
	}
	if code == "r01-d-conversion-entry" || code == "r01-d-conversion-type" {
		phase, rule = "types", "r01-conversions"
	}
	if strings.HasPrefix(code, "r01-d-aggregate-") {
		phase, rule = "types", "r01-values"
	}
	if category == "prohibited" {
		phase, rule = "types", "r01-input-boundary"
	} else if category == "input" {
		rule = "r01-input-boundary"
	}
	return response{Status: "error", Diagnostics: []diagnostic{{
		Code: code, RuleID: rule, RuleIDs: []string{rule}, Category: category,
		Phase: phase, Severity: "error", File: file, Range: location,
		Primary: primary{SourceID: file, Start: location.Start, End: location.End},
	}}}
}

func sourcePosition(file *ast.SourceFile, start, end int) primary {
	lines := file.ECMALineMap()
	positions := file.GetPositionMap()
	startLine, _ := core.PositionToLineAndByteOffset(start, lines)
	endLine, _ := core.PositionToLineAndByteOffset(end, lines)
	startOffset := positions.UTF8ToUTF16(start)
	endOffset := positions.UTF8ToUTF16(end)
	return primary{
		SourceID: file.FileName(), Start: startOffset, End: endOffset,
		StartLine: startLine, StartColumn: startOffset - positions.UTF8ToUTF16(int(lines[startLine])),
		EndLine: endLine, EndColumn: endOffset - positions.UTF8ToUTF16(int(lines[endLine])),
	}
}

func failAt(code, category string, file *ast.SourceFile, node *ast.Node) response {
	start := scanner.SkipTrivia(file.Text(), node.Pos())
	result := fail(code, category, file.FileName(), expressionOffset(file, node))
	result.Diagnostics[0].Primary = sourcePosition(file, start, node.End())
	return result
}

func failCallAt(code string, file *ast.SourceFile, node *ast.Node) response {
	result := failAt(code, "error", file, node)
	result.Diagnostics[0].RuleID = "r01-calls"
	result.Diagnostics[0].RuleIDs = []string{"r01-calls"}
	result.Diagnostics[0].Phase = "types"
	return result
}

func offset(file *ast.SourceFile, node *ast.Node) span {
	positions := file.GetPositionMap()
	return span{positions.UTF8ToUTF16(node.Pos()), positions.UTF8ToUTF16(node.End())}
}

func expressionOffset(file *ast.SourceFile, node *ast.Node) span {
	positions := file.GetPositionMap()
	return span{positions.UTF8ToUTF16(scanner.SkipTrivia(file.Text(), node.Pos())), positions.UTF8ToUTF16(node.End())}
}

func i32LiteralReturn(file *ast.SourceFile, check *checker.Checker, d *ast.Diagnostic) (*big.Int, bool) {
	if d.File() != file || d.Code() != 2322 {
		return nil, false
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil {
			continue
		}
		decl := node.AsFunctionDeclaration()
		if decl.Type == nil || scalar(check.GetTypeFromTypeNode(decl.Type)) != "i32" || decl.Body.Kind != ast.KindBlock {
			continue
		}
		var value *big.Int
		if visitStatements(decl.Body.AsBlock().Statements.Nodes, func(item *ast.Node) bool {
			if item.Kind != ast.KindReturnStatement || d.Pos() < item.Pos() || d.End() > item.End() ||
				item.AsReturnStatement().Expression == nil {
				return false
			}
			value, _, _ = exactInteger(file, item.AsReturnStatement().Expression)
			return value != nil
		}) {
			return value, true
		}
	}
	return nil, false
}

func defaultI32LocalReturn(file *ast.SourceFile, check *checker.Checker, d *ast.Diagnostic) (*big.Int, bool) {
	if d.File() != file || d.Code() != 2322 {
		return nil, false
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil {
			continue
		}
		fn := node.AsFunctionDeclaration()
		if fn.Type == nil || scalar(check.GetTypeFromTypeNode(fn.Type)) != "i32" || fn.Body.Kind != ast.KindBlock {
			continue
		}
		defaults := map[*ast.Symbol]*big.Int{}
		var result *big.Int
		if visitStatements(fn.Body.AsBlock().Statements.Nodes, func(item *ast.Node) bool {
			for _, decl := range localDeclarations(item) {
				if decl.Kind != ast.KindVariableDeclaration || decl.Name() == nil {
					continue
				}
				local := decl.AsVariableDeclaration()
				if local.Type != nil || local.Initializer == nil {
					continue
				}
				if value, _, ok := exactInteger(file, local.Initializer); ok {
					defaults[check.GetSymbolAtLocation(decl.Name())] = value
				} else if local.Initializer.Kind == ast.KindIdentifier {
					if value := defaults[check.GetSymbolAtLocation(local.Initializer)]; value != nil {
						defaults[check.GetSymbolAtLocation(decl.Name())] = value
					}
				}
			}
			if item.Kind == ast.KindReturnStatement && d.Pos() >= item.Pos() && d.End() <= item.End() {
				ret := item.AsReturnStatement().Expression
				if ret != nil && ret.Kind == ast.KindIdentifier {
					if value := defaults[check.GetSymbolAtLocation(ret)]; value != nil {
						result = value
						return true
					}
				}
			}
			return false
		}) {
			return result, true
		}
	}
	return nil, false
}

func contextualI32Local(file *ast.SourceFile, check *checker.Checker, d *ast.Diagnostic) (*big.Int, bool) {
	if d.File() != file || d.Code() != 2322 {
		return nil, false
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			continue
		}
		var value *big.Int
		if visitStatements(node.Body().AsBlock().Statements.Nodes, func(item *ast.Node) bool {
			for _, decl := range localDeclarations(item) {
				if decl.Kind != ast.KindVariableDeclaration || d.Pos() < decl.Pos() || d.End() > decl.End() {
					continue
				}
				local := decl.AsVariableDeclaration()
				if local.Type == nil || scalar(check.GetTypeFromTypeNode(local.Type)) != "i32" || local.Initializer == nil {
					continue
				}
				value, _, _ = exactInteger(file, local.Initializer)
				if value != nil {
					return true
				}
			}
			return false
		}) {
			return value, true
		}
	}
	return nil, false
}

func contextualI32Assignment(file *ast.SourceFile, check *checker.Checker, d *ast.Diagnostic) (*big.Int, bool) {
	if d.File() != file || d.Code() != 2322 {
		return nil, false
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			continue
		}
		var value *big.Int
		if visitStatements(node.Body().AsBlock().Statements.Nodes, func(item *ast.Node) bool {
			if item.Kind != ast.KindExpressionStatement {
				return false
			}
			expr := item.AsExpressionStatement().Expression
			if expr.Kind != ast.KindBinaryExpression || expr.AsBinaryExpression().OperatorToken.Kind != ast.KindEqualsToken {
				return false
			}
			binary := expr.AsBinaryExpression()
			if d.Pos() < item.Pos() || d.End() > item.End() ||
				scalar(check.GetTypeAtLocation(binary.Left)) != "i32" {
				return false
			}
			value, _, _ = exactInteger(file, binary.Right)
			return value != nil
		}) {
			return value, true
		}
	}
	return nil, false
}

func scalarAssignmentMismatch(file *ast.SourceFile, check *checker.Checker, d *ast.Diagnostic) bool {
	if d.File() != file || d.Code() != 2322 {
		return false
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			continue
		}
		for _, item := range node.Body().AsBlock().Statements.Nodes {
			if item.Kind != ast.KindExpressionStatement || d.Pos() < item.Pos() || d.End() > item.End() {
				continue
			}
			expr := item.AsExpressionStatement().Expression
			if expr.Kind != ast.KindBinaryExpression || expr.AsBinaryExpression().OperatorToken.Kind != ast.KindEqualsToken {
				continue
			}
			binary := expr.AsBinaryExpression()
			left, right := scalar(check.GetTypeAtLocation(binary.Left)), scalar(check.GetTypeAtLocation(binary.Right))
			if left != "" && right != "" && left != right {
				return true
			}
		}
	}
	return false
}

func scalarLocalMismatch(file *ast.SourceFile, check *checker.Checker, d *ast.Diagnostic) bool {
	if d.File() != file || d.Code() != 2322 {
		return false
	}
	var walk func([]*ast.Node) bool
	walk = func(items []*ast.Node) bool {
		for _, item := range items {
			if item.Kind == ast.KindBlock && walk(item.AsBlock().Statements.Nodes) {
				return true
			}
			for _, decl := range localDeclarations(item) {
				if decl.Kind != ast.KindVariableDeclaration || d.Pos() < decl.Pos() || d.End() > decl.End() {
					continue
				}
				local := decl.AsVariableDeclaration()
				if local.Type == nil || local.Initializer == nil {
					continue
				}
				expected := scalar(check.GetTypeFromTypeNode(local.Type))
				actual := scalar(check.GetTypeAtLocation(local.Initializer))
				if expected != "" && actual != "" && expected != actual {
					return true
				}
			}
		}
		return false
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind == ast.KindFunctionDeclaration && node.Body() != nil && node.Body().Kind == ast.KindBlock &&
			walk(node.Body().AsBlock().Statements.Nodes) {
			return true
		}
	}
	return false
}

func scalarReturnMismatch(file *ast.SourceFile, check *checker.Checker, d *ast.Diagnostic) bool {
	if d.File() != file || d.Code() != 2322 {
		return false
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil {
			continue
		}
		decl := node.AsFunctionDeclaration()
		if decl.Type == nil || scalar(check.GetTypeFromTypeNode(decl.Type)) == "" || decl.Body.Kind != ast.KindBlock {
			continue
		}
		if visitStatements(decl.Body.AsBlock().Statements.Nodes, func(item *ast.Node) bool {
			if item.Kind != ast.KindReturnStatement || d.Pos() < item.Pos() || d.End() > item.End() {
				return false
			}
			expr := item.AsReturnStatement().Expression
			return expr != nil && scalar(check.GetTypeAtLocation(expr)) != "" &&
				scalar(check.GetTypeAtLocation(expr)) != scalar(check.GetTypeFromTypeNode(decl.Type))
		}) {
			return true
		}
	}
	return false
}
