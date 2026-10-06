// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// tsfg-frontend projects a deliberately small, owned analysis result from the
// official TypeScript frontend. It never emits or executes JavaScript.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/scanner"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

type inputFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	SHA256  string `json:"sha256,omitempty"`
}

type request struct {
	SourceRoot   string      `json:"sourceRoot"`
	RootFile     string      `json:"rootFile"`
	Files        []inputFile `json:"files"`
	Declarations struct {
		Version string   `json:"version"`
		Paths   []string `json:"paths"`
	} `json:"declarations"`
	Config struct {
		Strict  bool `json:"strict"`
		NoEmit  bool `json:"noEmit"`
		AllowJS bool `json:"allowJs"`
	} `json:"config"`
}

type span struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type diagnostic struct {
	Code         string   `json:"code"`
	RuleID       string   `json:"ruleId"`
	RuleIDs      []string `json:"ruleIds"`
	Category     string   `json:"category"`
	Phase        string   `json:"phase"`
	Severity     string   `json:"severity"`
	File         string   `json:"file"`
	Range        span     `json:"range"`
	Primary      primary  `json:"primary"`
	UpstreamCode int32    `json:"upstreamCode,omitempty"`
}

type primary struct {
	SourceID    string `json:"sourceId"`
	Start       int    `json:"start"`
	End         int    `json:"end"`
	StartLine   int    `json:"startLine"`
	StartColumn int    `json:"startColumn"`
	EndLine     int    `json:"endLine"`
	EndColumn   int    `json:"endColumn"`
}

type symbol struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Type     string `json:"type"`
	Writable bool   `json:"writable"`
	Range    span   `json:"range"`
}

type expression struct {
	Kind      string          `json:"kind"`
	Type      string          `json:"type"`
	Operator  string          `json:"operator,omitempty"`
	Left      *expression     `json:"left,omitempty"`
	Right     *expression     `json:"right,omitempty"`
	Operand   *expression     `json:"operand,omitempty"`
	Symbol    string          `json:"symbol,omitempty"`
	Target    string          `json:"target,omitempty"`
	Arguments *[]expression   `json:"arguments,omitempty"`
	Signature *signature      `json:"signature,omitempty"`
	Boolean   *bool           `json:"boolean,omitempty"`
	Literal   *integerLiteral `json:"literal,omitempty"`
	Range     span            `json:"range"`
}

type signature struct {
	Parameters []string `json:"parameters"`
	ReturnType string   `json:"returnType"`
}

type integerLiteral struct {
	Decimal string `json:"decimal"`
	Token   string `json:"token"`
}

type statement struct {
	Kind        string      `json:"kind"`
	Expression  *expression `json:"expression,omitempty"`
	Condition   *expression `json:"condition,omitempty"`
	Symbol      *symbol     `json:"symbol,omitempty"`
	Initializer *expression `json:"initializer,omitempty"`
	Target      *expression `json:"target,omitempty"`
	Value       *expression `json:"value,omitempty"`
	Body        []statement `json:"body,omitempty"`
	ThenBody    []statement `json:"thenBody,omitempty"`
	ElseBody    []statement `json:"elseBody,omitempty"`
	Range       span        `json:"range"`
}

type function struct {
	Name       string      `json:"name"`
	Symbol     string      `json:"symbol"`
	Parameters []symbol    `json:"parameters"`
	ReturnType string      `json:"returnType"`
	Body       []statement `json:"body"`
	Range      span        `json:"range"`
}

type functionInfo struct {
	ID        string
	Signature signature
}

type response struct {
	Status      string       `json:"status"`
	Inputs      []string     `json:"inputs,omitempty"`
	Functions   []function   `json:"functions,omitempty"`
	Symbols     []symbol     `json:"symbols"`
	Diagnostics []diagnostic `json:"diagnostics,omitempty"`
}

func fail(code, category, file string, location span) response {
	phase := "syntax"
	if code == "r01-d-scalar-type" || code == "r01-d-integer-range" || code == "r01-d-shift-count-type" {
		phase = "types"
	} else if category == "unsupported" {
		phase = "capability"
	}
	rule := "r01-ts-baseline"
	if code == "r01-d-integer-range" || code == "r01-d-scalar-type" {
		rule = "r01-literals"
	}
	if code == "r01-d-shift-count-type" {
		rule = "r01-shifts"
	}
	if category == "prohibited" {
		phase, rule = "types", "r01-input-boundary"
	} else if category == "input" {
		rule = "r01-input-boundary"
	}
	return response{Status: "error", Diagnostics: []diagnostic{{Code: code, RuleID: rule, RuleIDs: []string{rule}, Category: category,
		Phase: phase, Severity: "error", File: file, Range: location,
		Primary: primary{SourceID: file, Start: location.Start, End: location.End}}}}
}

func sourcePosition(file *ast.SourceFile, start, end int) primary {
	lines := file.ECMALineMap()
	positions := file.GetPositionMap()
	startLine, _ := core.PositionToLineAndByteOffset(start, lines)
	endLine, _ := core.PositionToLineAndByteOffset(end, lines)
	startOffset := positions.UTF8ToUTF16(start)
	endOffset := positions.UTF8ToUTF16(end)
	return primary{SourceID: file.FileName(), Start: startOffset, End: endOffset,
		StartLine: startLine, StartColumn: startOffset - positions.UTF8ToUTF16(int(lines[startLine])),
		EndLine: endLine, EndColumn: endOffset - positions.UTF8ToUTF16(int(lines[endLine]))}
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

func valid(r request) bool {
	if r.SourceRoot == "" || !strings.HasPrefix(r.SourceRoot, "/") || path.Clean(r.SourceRoot) != r.SourceRoot || r.RootFile == "" ||
		(r.Declarations.Version != "minimal-bool-v1" && r.Declarations.Version != "minimal-scalars-v2") || !r.Config.Strict || !r.Config.NoEmit || r.Config.AllowJS || len(r.Files) == 0 || len(r.Declarations.Paths) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, file := range r.Files {
		if file.Path != path.Clean(file.Path) || !strings.HasPrefix(file.Path, r.SourceRoot+"/") || seen[file.Path] ||
			(file.Path != r.RootFile && !strings.HasSuffix(file.Path, ".d.ts")) {
			return false
		}
		seen[file.Path] = true
	}
	if !seen[r.RootFile] || !strings.HasSuffix(r.RootFile, ".ts") || strings.HasSuffix(r.RootFile, ".d.ts") {
		return false
	}
	declarations := map[string]bool{}
	for _, name := range r.Declarations.Paths {
		if !seen[name] || !strings.HasSuffix(name, ".d.ts") || declarations[name] {
			return false
		}
		declarations[name] = true
	}
	return true
}

func offset(file *ast.SourceFile, node *ast.Node) span {
	positions := file.GetPositionMap()
	return span{positions.UTF8ToUTF16(node.Pos()), positions.UTF8ToUTF16(node.End())}
}

func expressionOffset(file *ast.SourceFile, node *ast.Node) span {
	positions := file.GetPositionMap()
	return span{positions.UTF8ToUTF16(scanner.SkipTrivia(file.Text(), node.Pos())), positions.UTF8ToUTF16(node.End())}
}

func scalar(t *checker.Type) string {
	if t != nil && t.IsBooleanLike() {
		return "bool"
	}
	if t != nil && t.Alias() != nil && t.Alias().Symbol() != nil && t.Alias().Symbol().Name == "i32" {
		return "i32"
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
	if node.Kind == ast.KindCallExpression && len(functions) != 0 {
		call := node.AsCallExpression()
		if call.Expression.Kind == ast.KindIdentifier {
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
	if node.Kind == ast.KindIdentifier {
		binding, ok := bindings[check.GetSymbolAtLocation(node)]
		if ok && binding.Type != "" {
			return &expression{Kind: "reference", Type: binding.Type, Symbol: binding.ID, Range: expressionOffset(file, node)}, nil
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

func localDeclarations(node *ast.Node) []*ast.Node {
	if node.Kind != ast.KindVariableStatement {
		return nil
	}
	return node.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
}

func localDeclaration(node *ast.Node) *ast.Node {
	list := localDeclarations(node)
	if len(list) != 1 || list[0].Kind != ast.KindVariableDeclaration {
		return nil
	}
	return list[0]
}

func visitStatements(nodes []*ast.Node, visit func(*ast.Node) bool) bool {
	for _, node := range nodes {
		if visit(node) {
			return true
		}
		switch node.Kind {
		case ast.KindBlock:
			if visitStatements(node.AsBlock().Statements.Nodes, visit) {
				return true
			}
		case ast.KindIfStatement:
			branch := node.AsIfStatement()
			if visitStatements([]*ast.Node{branch.ThenStatement}, visit) ||
				(branch.ElseStatement != nil && visitStatements([]*ast.Node{branch.ElseStatement}, visit)) {
				return true
			}
		case ast.KindWhileStatement:
			if visitStatements([]*ast.Node{node.AsWhileStatement().Statement}, visit) {
				return true
			}
		}
	}
	return false
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

func operatorBindings(file *ast.SourceFile, check *checker.Checker, decl *ast.FunctionDeclaration) map[*ast.Symbol]symbol {
	bindings := parameterBindings(check, decl.Parameters.Nodes, nil)
	visitStatements(decl.Body.AsBlock().Statements.Nodes, func(item *ast.Node) bool {
		for _, local := range localDeclarations(item) {
			if local.Name() == nil || local.Name().Kind != ast.KindIdentifier {
				continue
			}
			t := scalar(check.GetTypeAtLocation(local.Name()))
			if t == "f64" && local.AsVariableDeclaration().Type == nil && local.AsVariableDeclaration().Initializer != nil {
				if _, _, ok := exactInteger(file, local.AsVariableDeclaration().Initializer); ok {
					t = "i32"
				}
			}
			bindings[check.GetSymbolAtLocation(local.Name())] = symbol{Type: t}
		}
		return false
	})
	return bindings
}

func supportedOperatorLocal(file *ast.SourceFile, check *checker.Checker, d *ast.Diagnostic) bool {
	if d.File() != file || d.Code() != 2322 {
		return false
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			continue
		}
		decl := node.AsFunctionDeclaration()
		bindings := operatorBindings(file, check, decl)
		if visitStatements(decl.Body.AsBlock().Statements.Nodes, func(item *ast.Node) bool {
			for _, candidate := range localDeclarations(item) {
				if candidate.Kind != ast.KindVariableDeclaration || candidate.Name() == nil || candidate.Name().Kind != ast.KindIdentifier ||
					d.Pos() < candidate.Pos() || d.End() > candidate.End() {
					continue
				}
				local := candidate.AsVariableDeclaration()
				if local.Type == nil || scalar(check.GetTypeFromTypeNode(local.Type)) != "i32" || local.Initializer == nil {
					continue
				}
				projected, failure := projectOperator(file, check, local.Initializer, bindings)
				return failure == nil && projected.Type == "i32"
			}
			return false
		}) {
			return true
		}
	}
	return false
}

func supportedOperatorAssignment(file *ast.SourceFile, check *checker.Checker, d *ast.Diagnostic) bool {
	if d.File() != file || d.Code() != 2322 {
		return false
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			continue
		}
		decl := node.AsFunctionDeclaration()
		bindings := operatorBindings(file, check, decl)
		body := decl.Body.AsBlock().Statements.Nodes
		if visitStatements(body, func(item *ast.Node) bool {
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
			projected, failure := projectOperator(file, check, binary.Right, bindings)
			return failure == nil && projected.Type == "i32"
		}) {
			return true
		}
	}
	return false
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

func pendingLocalForm(nodes []*ast.Node) *ast.Node {
	var walk func(*ast.Node) *ast.Node
	walk = func(node *ast.Node) *ast.Node {
		if node.Kind == ast.KindVariableDeclaration && node.AsVariableDeclaration().Initializer == nil {
			return node
		}
		if node.Kind == ast.KindBinaryExpression && ast.IsCompoundAssignment(node.AsBinaryExpression().OperatorToken.Kind) {
			return node
		}
		if node.Kind == ast.KindExpressionStatement {
			expr := node.AsExpressionStatement().Expression
			if expr.Kind == ast.KindBinaryExpression && expr.AsBinaryExpression().OperatorToken.Kind != ast.KindEqualsToken {
				return expr
			}
		}
		if node.Kind == ast.KindPostfixUnaryExpression {
			operator := node.AsPostfixUnaryExpression().Operator
			if operator == ast.KindPlusPlusToken || operator == ast.KindMinusMinusToken {
				return node
			}
		}
		if node.Kind == ast.KindPrefixUnaryExpression {
			operator := node.AsPrefixUnaryExpression().Operator
			if operator == ast.KindPlusPlusToken || operator == ast.KindMinusMinusToken {
				return node
			}
		}
		for child := range node.IterChildren() {
			if found := walk(child); found != nil {
				return found
			}
		}
		return nil
	}
	for _, node := range nodes {
		if found := walk(node); found != nil {
			return found
		}
	}
	return nil
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

type projection struct {
	file      *ast.SourceFile
	check     *checker.Checker
	result    *response
	bindings  map[*ast.Symbol]symbol
	functions map[*ast.Symbol]functionInfo
}

func (p *projection) addSymbol(node *ast.Node, kind, t string, writable bool) (symbol, bool) {
	upstream := p.check.GetSymbolAtLocation(node)
	if upstream == nil {
		return symbol{}, false
	}
	s := symbol{ID: fmt.Sprintf("s%d", len(p.result.Symbols)+1), Name: node.Text(),
		Kind: kind, Type: t, Writable: writable, Range: offset(p.file, node)}
	p.result.Symbols = append(p.result.Symbols, s)
	p.bindings[upstream] = s
	return s, true
}

func (p *projection) expr(node *ast.Node, expected string) (*expression, response) {
	if node == nil {
		return nil, fail("r02b-d-unsupported", "unsupported", p.file.FileName(), span{})
	}
	if node.Kind == ast.KindCallExpression {
		if node.AsCallExpression().Expression.Kind == ast.KindIdentifier {
			if target, ok := p.functions[p.check.GetSymbolAtLocation(node.AsCallExpression().Expression)]; ok {
				if expected == "" {
					expected = target.Signature.ReturnType
				}
			}
		}
		projected, failure := projectExpression(p.file, p.check, node, expected, p.bindings, p.functions)
		if failure != nil {
			return nil, *failure
		}
		return &projected, response{}
	}
	if node.Kind == ast.KindIdentifier {
		s, ok := p.bindings[p.check.GetSymbolAtLocation(node)]
		if !ok {
			return nil, failAt("r02b-d-type", "error", p.file, node)
		}
		if expected != "" && s.Type != expected {
			return nil, failAt("r01-d-scalar-type", "error", p.file, node)
		}
		return &expression{Kind: "reference", Type: s.Type, Symbol: s.ID, Range: offset(p.file, node)}, response{}
	}
	if node.Kind == ast.KindTrueKeyword || node.Kind == ast.KindFalseKeyword {
		if expected != "" && expected != "bool" {
			return nil, failAt("r01-d-scalar-type", "error", p.file, node)
		}
		value := node.Kind == ast.KindTrueKeyword
		return &expression{Kind: "boolean", Type: "bool", Boolean: &value, Range: offset(p.file, node)}, response{}
	}
	if value, literal, ok := exactInteger(p.file, node); ok {
		if expected != "" && expected != "i32" {
			return nil, failAt("r01-d-scalar-type", "error", p.file, node)
		}
		if !i32InRange(value) {
			return nil, failAt("r01-d-integer-range", "error", p.file, node)
		}
		return &expression{Kind: "integer", Type: "i32", Literal: literal, Range: offset(p.file, node)}, response{}
	}
	projected, failure := projectOperator(p.file, p.check, node, p.bindings, p.functions)
	if failure != nil {
		return nil, *failure
	}
	if expected != "" && projected.Type != expected {
		return nil, failAt("r01-d-scalar-type", "error", p.file, node)
	}
	return projected, response{}
}

func (p *projection) statements(nodes []*ast.Node, returnType string) ([]statement, response) {
	out := make([]statement, 0, len(nodes))
	for _, node := range nodes {
		switch node.Kind {
		case ast.KindVariableStatement:
			declarations := localDeclarations(node)
			if len(declarations) == 0 {
				return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node)
			}
			for _, decl := range declarations {
				if decl.Kind != ast.KindVariableDeclaration || decl.Name() == nil || decl.Name().Kind != ast.KindIdentifier {
					return nil, failAt("r02b-d-unsupported", "unsupported", p.file, decl)
				}
				local := decl.AsVariableDeclaration()
				if local.Initializer == nil {
					return nil, failAt("r02b-d-unsupported", "unsupported", p.file, decl)
				}
				t := ""
				if local.Type != nil {
					t = scalar(p.check.GetTypeFromTypeNode(local.Type))
					if t == "" {
						return nil, failAt("r02b-d-unsupported", "unsupported", p.file, local.Type)
					}
				}
				init, failure := p.expr(local.Initializer, t)
				if failure.Status != "" {
					return nil, failure
				}
				if t == "" {
					t = init.Type
				}
				writable := node.AsVariableStatement().DeclarationList.Flags&ast.NodeFlagsLet != 0
				if !writable && node.AsVariableStatement().DeclarationList.Flags&ast.NodeFlagsConst == 0 {
					return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node)
				}
				s, ok := p.addSymbol(decl.Name(), "local", t, writable)
				if !ok {
					return nil, failAt("r02b-d-type", "error", p.file, decl.Name())
				}
				out = append(out, statement{Kind: "variable", Symbol: &s, Initializer: init, Range: offset(p.file, decl)})
			}
		case ast.KindExpressionStatement:
			expr := node.AsExpressionStatement().Expression
			if expr.Kind != ast.KindBinaryExpression || expr.AsBinaryExpression().OperatorToken.Kind != ast.KindEqualsToken {
				return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node)
			}
			binary := expr.AsBinaryExpression()
			if binary.Left.Kind != ast.KindIdentifier {
				return nil, failAt("r02b-d-unsupported", "unsupported", p.file, binary.Left)
			}
			target, failure := p.expr(binary.Left, "")
			if failure.Status != "" {
				return nil, failure
			}
			s := p.bindings[p.check.GetSymbolAtLocation(binary.Left)]
			if !s.Writable {
				return nil, failAt("r01-d-scalar-type", "error", p.file, binary.Left)
			}
			value, failure := p.expr(binary.Right, target.Type)
			if failure.Status != "" {
				return nil, failure
			}
			out = append(out, statement{Kind: "assignment", Target: target, Value: value, Range: offset(p.file, node)})
		case ast.KindReturnStatement:
			expr, failure := p.expr(node.AsReturnStatement().Expression, returnType)
			if failure.Status != "" {
				return nil, failure
			}
			out = append(out, statement{Kind: "return", Expression: expr, Range: offset(p.file, node)})
		case ast.KindBlock:
			body, failure := p.statements(node.AsBlock().Statements.Nodes, returnType)
			if failure.Status != "" {
				return nil, failure
			}
			out = append(out, statement{Kind: "block", Body: body, Range: offset(p.file, node)})
		case ast.KindIfStatement:
			branch := node.AsIfStatement()
			condition, failure := p.expr(branch.Expression, "bool")
			if failure.Status != "" {
				return nil, failure
			}
			thenBody, failure := p.statements([]*ast.Node{branch.ThenStatement}, returnType)
			if failure.Status != "" {
				return nil, failure
			}
			projected := statement{Kind: "if", Condition: condition, ThenBody: thenBody, Range: expressionOffset(p.file, node)}
			if branch.ElseStatement != nil {
				projected.ElseBody, failure = p.statements([]*ast.Node{branch.ElseStatement}, returnType)
				if failure.Status != "" {
					return nil, failure
				}
			}
			out = append(out, projected)
		case ast.KindWhileStatement:
			loop := node.AsWhileStatement()
			condition, failure := p.expr(loop.Expression, "bool")
			if failure.Status != "" {
				return nil, failure
			}
			body, failure := p.statements([]*ast.Node{loop.Statement}, returnType)
			if failure.Status != "" {
				return nil, failure
			}
			out = append(out, statement{Kind: "while", Condition: condition, Body: body, Range: expressionOffset(p.file, node)})
		default:
			return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node)
		}
	}
	return out, response{}
}

func parameterBindings(check *checker.Checker, parameters []*ast.Node, projected []symbol) map[*ast.Symbol]symbol {
	bindings := map[*ast.Symbol]symbol{}
	for i, parameter := range parameters {
		if parameter.Name() == nil {
			continue
		}
		resolved := check.GetSymbolAtLocation(parameter.Name())
		if resolved == nil {
			continue
		}
		binding := symbol{Type: scalar(check.GetTypeAtLocation(parameter.Name()))}
		if i < len(projected) {
			binding = projected[i]
		}
		bindings[resolved] = binding
	}
	return bindings
}

func supportedOperatorReturn(file *ast.SourceFile, check *checker.Checker, d *ast.Diagnostic) bool {
	if d.File() != file || d.Code() != 2322 {
		return false
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			continue
		}
		decl := node.AsFunctionDeclaration()
		if decl.Type == nil || scalar(check.GetTypeFromTypeNode(decl.Type)) != "i32" {
			continue
		}
		bindings := operatorBindings(file, check, decl)
		if visitStatements(decl.Body.AsBlock().Statements.Nodes, func(item *ast.Node) bool {
			if item.Kind != ast.KindReturnStatement || d.Pos() < item.Pos() || d.End() > item.End() {
				return false
			}
			expr := item.AsReturnStatement().Expression
			if expr != nil && (expr.Kind == ast.KindBinaryExpression || expr.Kind == ast.KindPrefixUnaryExpression || expr.Kind == ast.KindParenthesizedExpression) {
				candidate := expr
				for candidate.Kind == ast.KindParenthesizedExpression {
					candidate = candidate.AsParenthesizedExpression().Expression
				}
				if candidate.Kind == ast.KindBinaryExpression && candidate.AsBinaryExpression().OperatorToken.Kind == ast.KindAsteriskAsteriskToken {
					binary := candidate.AsBinaryExpression()
					left, leftFailure := projectOperator(file, check, binary.Left, bindings)
					right, rightFailure := projectOperator(file, check, binary.Right, bindings)
					if leftFailure == nil && rightFailure == nil && left.Type == "i32" && right.Type == "i32" {
						return true
					}
				}
				projected, failure := projectOperator(file, check, expr, bindings)
				return failure == nil && projected.Type == "i32"
			}
			return false
		}) {
			return true
		}
	}
	return false
}

func callDiagnostic(file *ast.SourceFile, d *ast.Diagnostic) (string, bool) {
	if d.File() != file {
		return "", false
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			continue
		}
		for _, item := range node.Body().AsBlock().Statements.Nodes {
			if item.Kind != ast.KindReturnStatement {
				continue
			}
			expr := item.AsReturnStatement().Expression
			if expr == nil || expr.Kind != ast.KindCallExpression || d.Pos() < item.Pos() || d.End() > item.End() {
				continue
			}
			if d.Code() != 2322 && (d.Pos() < expr.Pos() || d.End() > expr.End()) {
				continue
			}
			switch d.Code() {
			case 2304:
				return "r01-d-unresolved-name", true
			case 2345, 2322:
				return "r01-d-scalar-type", true
			case 2554:
				return "r02b-d-type", true
			}
		}
	}
	return "", false
}

func i32LiteralCallArgument(file *ast.SourceFile, check *checker.Checker, d *ast.Diagnostic) (*big.Int, bool) {
	if d.File() != file || d.Code() != 2345 {
		return nil, false
	}
	var inspectCall func(*ast.Node) (*big.Int, bool)
	inspectCall = func(expr *ast.Node) (*big.Int, bool) {
		if expr == nil || expr.Kind != ast.KindCallExpression {
			return nil, false
		}
		call := expr.AsCallExpression()
		for _, argument := range call.Arguments.Nodes {
			if d.Pos() >= argument.Pos() && d.End() <= argument.End() {
				if value, ok := inspectCall(argument); ok {
					return value, true
				}
			}
		}
		if call.Expression.Kind != ast.KindIdentifier {
			return nil, false
		}
		callee := check.GetSymbolAtLocation(call.Expression)
		for _, declaration := range file.Statements.Nodes {
			if declaration.Kind != ast.KindFunctionDeclaration || declaration.Name() == nil ||
				check.GetSymbolAtLocation(declaration.Name()) != callee {
				continue
			}
			parameters := declaration.AsFunctionDeclaration().Parameters.Nodes
			for index, argument := range call.Arguments.Nodes {
				if index >= len(parameters) || d.Pos() < argument.Pos() || d.End() > argument.End() {
					continue
				}
				paramType := parameters[index].AsParameterDeclaration().Type
				if paramType != nil && scalar(check.GetTypeFromTypeNode(paramType)) == "i32" {
					value, _, ok := exactInteger(file, argument)
					return value, ok
				}
			}
		}
		return nil, false
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			continue
		}
		for _, item := range node.Body().AsBlock().Statements.Nodes {
			if item.Kind != ast.KindReturnStatement {
				continue
			}
			if value, ok := inspectCall(item.AsReturnStatement().Expression); ok {
				return value, true
			}
		}
	}
	return nil, false
}

func projectExpression(file *ast.SourceFile, check *checker.Checker, node *ast.Node, expected string,
	refs map[*ast.Symbol]symbol, functions map[*ast.Symbol]functionInfo) (expression, *response) {
	if value, literal, ok := exactInteger(file, node); ok && expected == "i32" {
		if !i32InRange(value) {
			failure := failAt("r01-d-integer-range", "error", file, node)
			return expression{}, &failure
		}
		return expression{Kind: "integer", Type: "i32", Literal: literal, Range: offset(file, node)}, nil
	}
	if node.Kind == ast.KindTrueKeyword || node.Kind == ast.KindFalseKeyword {
		if expected != "bool" {
			failure := failAt("r01-d-scalar-type", "error", file, node)
			return expression{}, &failure
		}
		value := node.Kind == ast.KindTrueKeyword
		return expression{Kind: "boolean", Type: "bool", Boolean: &value, Range: offset(file, node)}, nil
	}
	if node.Kind == ast.KindIdentifier {
		resolved := check.GetSymbolAtLocation(node)
		ref, ok := refs[resolved]
		if !ok {
			failure := failAt("r02b-d-type", "error", file, node)
			return expression{}, &failure
		}
		if ref.Type != expected {
			failure := failAt("r01-d-scalar-type", "error", file, node)
			return expression{}, &failure
		}
		return expression{Kind: "reference", Type: ref.Type, Symbol: ref.ID, Range: offset(file, node)}, nil
	}
	if node.Kind == ast.KindCallExpression {
		call := node.AsCallExpression()
		if call.Expression.Kind != ast.KindIdentifier || call.QuestionDotToken != nil || call.TypeArguments != nil {
			failure := failAt("r02b-d-unsupported", "unsupported", file, node)
			return expression{}, &failure
		}
		target, ok := functions[check.GetSymbolAtLocation(call.Expression)]
		if !ok {
			failure := failCallAt("r01-d-unresolved-name", file, call.Expression)
			return expression{}, &failure
		}
		if target.Signature.ReturnType != expected {
			failure := failCallAt("r01-d-scalar-type", file, node)
			return expression{}, &failure
		}
		if len(call.Arguments.Nodes) != len(target.Signature.Parameters) {
			failure := failCallAt("r02b-d-type", file, node)
			return expression{}, &failure
		}
		arguments := make([]expression, 0, len(call.Arguments.Nodes))
		for index, argument := range call.Arguments.Nodes {
			projected, failure := projectExpression(file, check, argument, target.Signature.Parameters[index], refs, functions)
			if failure != nil {
				if failure.Diagnostics[0].Code == "r01-d-scalar-type" {
					failure.Diagnostics[0].RuleID = "r01-calls"
					failure.Diagnostics[0].RuleIDs = []string{"r01-calls"}
				}
				return expression{}, failure
			}
			arguments = append(arguments, projected)
		}
		sig := target.Signature
		return expression{Kind: "call", Type: sig.ReturnType, Target: target.ID,
			Arguments: &arguments, Signature: &sig, Range: offset(file, node)}, nil
	}
	if node.Kind == ast.KindBinaryExpression || node.Kind == ast.KindPrefixUnaryExpression || node.Kind == ast.KindParenthesizedExpression {
		projected, failure := projectOperator(file, check, node, refs, functions)
		if failure != nil {
			return expression{}, failure
		}
		if projected.Type != expected {
			failure := failAt("r01-d-scalar-type", "error", file, node)
			return expression{}, &failure
		}
		return *projected, nil
	}
	failure := failAt("r02b-d-unsupported", "unsupported", file, node)
	return expression{}, &failure
}

func prohibitedNode(file *ast.SourceFile) *ast.Node {
	var found *ast.Node
	var visit func(*ast.Node) bool
	visit = func(node *ast.Node) bool {
		if node.Kind == ast.KindAnyKeyword {
			found = node
			return true
		}
		if node.Kind == ast.KindBinaryExpression {
			binary := node.AsBinaryExpression()
			operator := binary.OperatorToken
			if operator.Kind == ast.KindEqualsEqualsToken || operator.Kind == ast.KindExclamationEqualsToken {
				found = operator
				return true
			}
			if operator.Kind == ast.KindEqualsToken {
				if binary.Left.Kind == ast.KindElementAccessExpression ||
					(binary.Left.Kind == ast.KindPropertyAccessExpression && binary.Left.AsPropertyAccessExpression().Name().Text() == "__proto__") {
					found = binary.Left
					return true
				}
			}
		}
		if node.Kind == ast.KindDeleteExpression {
			found = node
			return true
		}
		if node.Kind == ast.KindCallExpression {
			callee := node.AsCallExpression().Expression
			if (callee.Kind == ast.KindIdentifier && (callee.Text() == "eval" || callee.Text() == "Function")) ||
				(callee.Kind == ast.KindPropertyAccessExpression && callee.AsPropertyAccessExpression().Expression.Kind == ast.KindIdentifier &&
					callee.AsPropertyAccessExpression().Expression.Text() == "Object" && callee.AsPropertyAccessExpression().Name().Text() == "setPrototypeOf") {
				found = callee
				return true
			}
		}
		if node.Kind == ast.KindNewExpression {
			callee := node.AsNewExpression().Expression
			if callee.Kind == ast.KindIdentifier && (callee.Text() == "Proxy" || callee.Text() == "Function") {
				found = callee
				return true
			}
		}
		return node.ForEachChild(visit)
	}
	file.ForEachChild(visit)
	return found
}

func analyze(r request) response {
	if !valid(r) {
		return fail("r02b-d-input", "input", r.RootFile, span{})
	}
	contents := map[string]string{}
	for _, file := range r.Files {
		contents[file.Path] = file.Content
	}
	fileNames := append([]string{r.RootFile}, r.Declarations.Paths...)
	slices.Sort(fileNames)
	fs := vfstest.FromMap(contents, true)
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
			FileNames:       fileNames,
			CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext},
		}},
		Host:           compiler.NewCompilerHost(r.SourceRoot, fs, r.SourceRoot, nil, nil),
		SingleThreaded: core.TSTrue,
	})
	file := program.GetSourceFile(r.RootFile)
	if file == nil {
		return fail("r02b-d-input", "input", r.RootFile, span{})
	}
	approved := map[string]bool{}
	for name := range contents {
		approved[name] = true
	}
	consumed := []string{}
	for _, source := range program.GetSourceFiles() {
		if !approved[source.FileName()] {
			return fail("r02b-d-input", "input", source.FileName(), span{})
		}
		consumed = append(consumed, source.FileName())
		delete(approved, source.FileName())
	}
	slices.Sort(consumed)
	result := response{Status: "ok", Inputs: consumed, Functions: []function{}, Symbols: []symbol{}}
	groups := []struct {
		items []*ast.Diagnostic
		code  string
	}{
		{program.GetSyntacticDiagnostics(context.Background(), nil), "r02b-d-syntax"},
		{program.GetSemanticDiagnostics(context.Background(), nil), "r02b-d-type"},
		{program.GetProgramDiagnostics(), "r02b-d-type"},
		{program.GetGlobalDiagnostics(context.Background()), "r02b-d-type"},
	}
	check, release := program.GetTypeCheckerForFileExclusive(context.Background(), file)
	defer release()
	if len(groups[0].items) == 0 {
		if node := prohibitedNode(file); node != nil {
			result.Diagnostics = append(result.Diagnostics, failAt("r02b-d-prohibited", "prohibited", file, node).Diagnostics[0])
		}
		for _, node := range file.Statements.Nodes {
			if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil || node.Body().Kind != ast.KindBlock {
				continue
			}
			if pending := pendingLocalForm(node.Body().AsBlock().Statements.Nodes); pending != nil {
				return failAt("r02b-d-unsupported", "unsupported", file, pending)
			}
			decl := node.AsFunctionDeclaration()
			bindings := parameterBindings(check, decl.Parameters.Nodes, nil)
			for _, item := range decl.Body.AsBlock().Statements.Nodes {
				if item.Kind != ast.KindReturnStatement || item.AsReturnStatement().Expression == nil {
					continue
				}
				expr := item.AsReturnStatement().Expression
				if expr.Kind == ast.KindBinaryExpression || expr.Kind == ast.KindPrefixUnaryExpression || expr.Kind == ast.KindParenthesizedExpression {
					_, failure := projectOperator(file, check, expr, bindings)
					if failure != nil && failure.Diagnostics[0].Category != "unsupported" {
						return *failure
					}
				}
			}
		}
	}
	for _, group := range groups {
		for _, d := range group.items {
			code := group.code
			if d.Code() == 6053 || d.Code() == 2688 || d.Code() == 2307 {
				return fail("r02b-d-input", "input", r.RootFile, span{})
			}
			if value, ok := i32LiteralCallArgument(file, check, d); ok {
				if i32InRange(value) {
					continue
				}
				code = "r01-d-integer-range"
			}
			callCode, isCall := callDiagnostic(file, d)
			if isCall && code != "r01-d-integer-range" {
				code = callCode
			}
			if supportedOperatorReturn(file, check, d) || supportedOperatorAssignment(file, check, d) || supportedOperatorLocal(file, check, d) {
				continue
			}
			if value, ok := i32LiteralReturn(file, check, d); ok {
				if i32InRange(value) {
					continue
				}
				code = "r01-d-integer-range"
			}
			if value, ok := defaultI32LocalReturn(file, check, d); ok {
				if i32InRange(value) {
					continue
				}
				code = "r01-d-integer-range"
			}
			if value, ok := contextualI32Local(file, check, d); ok {
				if i32InRange(value) {
					continue
				}
				code = "r01-d-integer-range"
			}
			if value, ok := contextualI32Assignment(file, check, d); ok {
				if i32InRange(value) {
					continue
				}
				code = "r01-d-integer-range"
			}
			if code == "r02b-d-type" && (scalarReturnMismatch(file, check, d) ||
				scalarAssignmentMismatch(file, check, d) || scalarLocalMismatch(file, check, d)) {
				code = "r01-d-scalar-type"
			}
			if code == "r02b-d-type" && (d.Code() == 2304 || d.Code() == 2552) {
				code = "r01-d-unresolved-name"
			}
			if d.Code() == 6504 {
				code = "r02b-d-input"
			}
			name, where := r.RootFile, span{}
			primaryLocation := primary{SourceID: name}
			if d.File() != nil {
				name = d.File().FileName()
				where = span{d.File().GetPositionMap().UTF8ToUTF16(d.Pos()), d.File().GetPositionMap().UTF8ToUTF16(d.End())}
				primaryLocation = sourcePosition(d.File(), d.Pos(), d.End())
			}
			phase := "syntax"
			if code == "r01-d-scalar-type" || code == "r01-d-integer-range" || code == "r01-d-unresolved-name" || code == "r02b-d-type" {
				phase = "types"
			}
			rule := "r01-ts-baseline"
			category := "error"
			if code == "r01-d-integer-range" || code == "r01-d-scalar-type" {
				rule = "r01-literals"
			}
			if isCall && code != "r01-d-integer-range" {
				rule = "r01-calls"
			}
			if code == "r02b-d-input" {
				rule, category = "r01-input-boundary", "input"
			}
			result.Diagnostics = append(result.Diagnostics, diagnostic{Code: code, RuleID: rule, RuleIDs: []string{rule},
				Category: category, Phase: phase, Severity: "error", File: name, Range: where,
				Primary: primaryLocation, UpstreamCode: d.Code()})
		}
	}
	if len(result.Diagnostics) != 0 {
		slices.SortFunc(result.Diagnostics, func(a, b diagnostic) int {
			if order := strings.Compare(a.File, b.File); order != 0 {
				return order
			}
			if a.Range.Start != b.Range.Start {
				return a.Range.Start - b.Range.Start
			}
			if order := strings.Compare(a.Code, b.Code); order != 0 {
				return order
			}
			return a.Range.End - b.Range.End
		})
		result.Status = "error"
		return result
	}
	functions := map[*ast.Symbol]functionInfo{}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Name() == nil || node.Body() == nil {
			return failAt("r02b-d-unsupported", "unsupported", file, node)
		}
		decl := node.AsFunctionDeclaration()
		if decl.Type == nil || decl.Body.Kind != ast.KindBlock {
			return failAt("r02b-d-unsupported", "unsupported", file, node)
		}
		returnType := scalar(check.GetTypeFromTypeNode(decl.Type))
		if returnType == "" {
			return failAt("r02b-d-unsupported", "unsupported", file, decl.Type)
		}
		parameters := make([]string, 0, len(decl.Parameters.Nodes))
		for _, p := range decl.Parameters.Nodes {
			if p.Name() == nil || p.AsParameterDeclaration().Type == nil || p.AsParameterDeclaration().Initializer != nil {
				return failAt("r02b-d-unsupported", "unsupported", file, p)
			}
			t := scalar(check.GetTypeAtLocation(p.Name()))
			if t == "" {
				return failAt("r02b-d-unsupported", "unsupported", file, p)
			}
			parameters = append(parameters, t)
		}
		resolved := check.GetSymbolAtLocation(node.Name())
		if resolved == nil {
			return failAt("r02b-d-type", "error", file, node.Name())
		}
		s := symbol{ID: fmt.Sprintf("s%d", len(result.Symbols)+1), Name: node.Name().Text(),
			Kind: "function", Type: returnType, Range: offset(file, node.Name())}
		result.Symbols = append(result.Symbols, s)
		functions[resolved] = functionInfo{ID: s.ID, Signature: signature{Parameters: parameters, ReturnType: returnType}}
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Name() == nil || node.Body() == nil {
			return failAt("r02b-d-unsupported", "unsupported", file, node)
		}
		decl := node.AsFunctionDeclaration()
		if decl.Type == nil || decl.Body.Kind != ast.KindBlock {
			return failAt("r02b-d-unsupported", "unsupported", file, node)
		}
		body := decl.Body.AsBlock().Statements.Nodes
		info := functions[check.GetSymbolAtLocation(node.Name())]
		fn := function{Name: node.Name().Text(), Symbol: info.ID, Parameters: []symbol{}, Body: []statement{}, Range: offset(file, node)}
		fn.ReturnType = scalar(check.GetTypeFromTypeNode(decl.Type))
		if fn.ReturnType == "" {
			return failAt("r02b-d-unsupported", "unsupported", file, decl.Type)
		}
		project := projection{file: file, check: check, result: &result, bindings: map[*ast.Symbol]symbol{}, functions: functions}
		for _, p := range decl.Parameters.Nodes {
			if p.Name() == nil || p.Name().Kind != ast.KindIdentifier || p.AsParameterDeclaration().Type == nil || p.AsParameterDeclaration().Initializer != nil {
				return failAt("r02b-d-unsupported", "unsupported", file, p)
			}
			t := scalar(check.GetTypeAtLocation(p.Name()))
			if t == "" {
				return failAt("r02b-d-unsupported", "unsupported", file, p)
			}
			s, ok := project.addSymbol(p.Name(), "parameter", t, true)
			if !ok {
				return failAt("r02b-d-type", "error", file, p.Name())
			}
			fn.Parameters = append(fn.Parameters, s)
		}
		projected, failure := project.statements(body, fn.ReturnType)
		if failure.Status != "" {
			return failure
		}
		fn.Body = projected
		result.Functions = append(result.Functions, fn)
	}
	if len(result.Functions) == 0 {
		return fail("r02b-d-unsupported", "unsupported", r.RootFile, span{})
	}
	return result
}

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "tsfg-frontend accepts one JSON request on stdin")
		os.Exit(2)
	}
	decoder := json.NewDecoder(os.Stdin)
	decoder.DisallowUnknownFields()
	var r request
	if err := decoder.Decode(&r); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := json.NewEncoder(os.Stdout).Encode(analyze(r)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
