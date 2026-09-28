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
	Kind    string          `json:"kind"`
	Type    string          `json:"type"`
	Symbol  string          `json:"symbol,omitempty"`
	Literal *integerLiteral `json:"literal,omitempty"`
	Boolean *bool           `json:"boolean,omitempty"`
	Range   span            `json:"range"`
}

type integerLiteral struct {
	Decimal string `json:"decimal"`
	Token   string `json:"token"`
}

type statement struct {
	Kind        string      `json:"kind"`
	Expression  *expression `json:"expression,omitempty"`
	Symbol      *symbol     `json:"symbol,omitempty"`
	Initializer *expression `json:"initializer,omitempty"`
	Target      *expression `json:"target,omitempty"`
	Value       *expression `json:"value,omitempty"`
	Body        []statement `json:"body,omitempty"`
	Range       span        `json:"range"`
}

type function struct {
	Name       string      `json:"name"`
	Parameters []symbol    `json:"parameters"`
	ReturnType string      `json:"returnType"`
	Body       []statement `json:"body"`
	Range      span        `json:"range"`
}

type response struct {
	Status      string       `json:"status"`
	Functions   []function   `json:"functions,omitempty"`
	Symbols     []symbol     `json:"symbols"`
	Diagnostics []diagnostic `json:"diagnostics,omitempty"`
}

func fail(code, category, file string, location span) response {
	phase := "syntax"
	if code == "r01-d-scalar-type" || code == "r01-d-integer-range" {
		phase = "types"
	}
	rule := "r01-ts-baseline"
	if code == "r01-d-integer-range" || code == "r01-d-scalar-type" {
		rule = "r01-literals"
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
	result := fail(code, category, file.FileName(), offset(file, node))
	result.Diagnostics[0].Primary = sourcePosition(file, node.Pos(), node.End())
	return result
}

func valid(r request) bool {
	if r.SourceRoot == "" || !strings.HasPrefix(r.SourceRoot, "/") || path.Clean(r.SourceRoot) != r.SourceRoot || r.RootFile == "" ||
		(r.Declarations.Version != "minimal-bool-v1" && r.Declarations.Version != "minimal-scalars-v2") || !r.Config.Strict || !r.Config.NoEmit || r.Config.AllowJS || len(r.Files) == 0 || len(r.Declarations.Paths) == 0 {
		return false
	}
	if len(r.Files) != len(r.Declarations.Paths)+1 {
		return false
	}
	seen := map[string]bool{}
	for _, file := range r.Files {
		if file.Path != path.Clean(file.Path) || !strings.HasPrefix(file.Path, r.SourceRoot+"/") || seen[file.Path] ||
			(!strings.HasSuffix(file.Path, ".ts") && !strings.HasSuffix(file.Path, ".d.ts")) {
			return false
		}
		seen[file.Path] = true
	}
	if !seen[r.RootFile] || !strings.HasSuffix(r.RootFile, ".ts") || strings.HasSuffix(r.RootFile, ".d.ts") {
		return false
	}
	for _, name := range r.Declarations.Paths {
		if !seen[name] || !strings.HasSuffix(name, ".d.ts") {
			return false
		}
	}
	return true
}

func offset(file *ast.SourceFile, node *ast.Node) span {
	positions := file.GetPositionMap()
	return span{positions.UTF8ToUTF16(node.Pos()), positions.UTF8ToUTF16(node.End())}
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
		for _, item := range decl.Body.AsBlock().Statements.Nodes {
			if item.Kind != ast.KindReturnStatement || d.Pos() < item.Pos() || d.End() > item.End() {
				continue
			}
			expr := item.AsReturnStatement().Expression
			if expr == nil {
				continue
			}
			value, _, ok := exactInteger(file, expr)
			if ok {
				return value, true
			}
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
		for _, item := range fn.Body.AsBlock().Statements.Nodes {
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
						return value, true
					}
				}
			}
		}
	}
	return nil, false
}

func contextualI32Local(file *ast.SourceFile, check *checker.Checker, d *ast.Diagnostic) (*big.Int, bool) {
	if d.File() != file || d.Code() != 2322 {
		return nil, false
	}
	var walk func([]*ast.Node) (*big.Int, bool)
	walk = func(items []*ast.Node) (*big.Int, bool) {
		for _, item := range items {
			if item.Kind == ast.KindBlock {
				if value, ok := walk(item.AsBlock().Statements.Nodes); ok {
					return value, true
				}
				continue
			}
			for _, decl := range localDeclarations(item) {
				if decl.Kind != ast.KindVariableDeclaration || d.Pos() < decl.Pos() || d.End() > decl.End() {
					continue
				}
				local := decl.AsVariableDeclaration()
				if local.Type == nil || scalar(check.GetTypeFromTypeNode(local.Type)) != "i32" || local.Initializer == nil {
					continue
				}
				value, _, ok := exactInteger(file, local.Initializer)
				if ok {
					return value, true
				}
			}
		}
		return nil, false
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			continue
		}
		if value, ok := walk(node.Body().AsBlock().Statements.Nodes); ok {
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
		for _, item := range node.Body().AsBlock().Statements.Nodes {
			if item.Kind != ast.KindExpressionStatement {
				continue
			}
			expr := item.AsExpressionStatement().Expression
			if expr.Kind != ast.KindBinaryExpression || expr.AsBinaryExpression().OperatorToken.Kind != ast.KindEqualsToken {
				continue
			}
			binary := expr.AsBinaryExpression()
			if d.Pos() < item.Pos() || d.End() > item.End() ||
				scalar(check.GetTypeAtLocation(binary.Left)) != "i32" {
				continue
			}
			value, _, ok := exactInteger(file, binary.Right)
			if ok {
				return value, true
			}
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

func pendingLocalForm(nodes []*ast.Node) *ast.Node {
	for _, node := range nodes {
		if node.Kind == ast.KindBlock {
			if found := pendingLocalForm(node.AsBlock().Statements.Nodes); found != nil {
				return found
			}
		}
		for _, decl := range localDeclarations(node) {
			if decl.Kind == ast.KindVariableDeclaration && decl.AsVariableDeclaration().Initializer == nil {
				return decl
			}
		}
		if node.Kind == ast.KindExpressionStatement {
			expr := node.AsExpressionStatement().Expression
			if expr.Kind == ast.KindBinaryExpression && expr.AsBinaryExpression().OperatorToken.Kind != ast.KindEqualsToken {
				return expr
			}
			if expr.Kind == ast.KindPostfixUnaryExpression {
				operator := expr.AsPostfixUnaryExpression().Operator
				if operator == ast.KindPlusPlusToken || operator == ast.KindMinusMinusToken {
					return expr
				}
			}
			if expr.Kind == ast.KindPrefixUnaryExpression {
				operator := expr.AsPrefixUnaryExpression().Operator
				if operator == ast.KindPlusPlusToken || operator == ast.KindMinusMinusToken {
					return expr
				}
			}
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
		body := decl.Body.AsBlock().Statements.Nodes
		for _, item := range body {
			if item.Kind != ast.KindReturnStatement || d.Pos() < item.Pos() || d.End() > item.End() {
				continue
			}
			expr := item.AsReturnStatement().Expression
			if expr != nil && scalar(check.GetTypeAtLocation(expr)) != "" && scalar(check.GetTypeAtLocation(expr)) != scalar(check.GetTypeFromTypeNode(decl.Type)) {
				return true
			}
		}
	}
	return false
}

type projection struct {
	file     *ast.SourceFile
	check    *checker.Checker
	result   *response
	bindings map[*ast.Symbol]symbol
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
	return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node)
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
		default:
			return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node)
		}
	}
	return out, response{}
}

func analyze(r request) response {
	if !valid(r) {
		return fail("r02b-d-input", "input", r.RootFile, span{})
	}
	contents := map[string]string{}
	fileNames := []string{}
	for _, file := range r.Files {
		contents[file.Path] = file.Content
		fileNames = append(fileNames, file.Path)
	}
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
	for _, name := range fileNames {
		approved[name] = true
	}
	for _, source := range program.GetSourceFiles() {
		if !approved[source.FileName()] {
			return fail("r02b-d-input", "input", source.FileName(), span{})
		}
		delete(approved, source.FileName())
	}
	if len(approved) != 0 {
		return fail("r02b-d-input", "input", r.RootFile, span{})
	}
	result := response{Status: "ok", Functions: []function{}, Symbols: []symbol{}}
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
		for _, node := range file.Statements.Nodes {
			if node.Kind == ast.KindFunctionDeclaration && node.Body() != nil && node.Body().Kind == ast.KindBlock {
				if pending := pendingLocalForm(node.Body().AsBlock().Statements.Nodes); pending != nil {
					return failAt("r02b-d-unsupported", "unsupported", file, pending)
				}
			}
		}
	}
	for _, group := range groups {
		for _, d := range group.items {
			code := group.code
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
			if code == "r02b-d-type" && d.Code() == 2304 {
				code = "r01-d-unresolved-name"
			}
			name, where := r.RootFile, span{}
			primaryLocation := primary{SourceID: name}
			if d.File() != nil {
				name = d.File().FileName()
				where = span{d.File().GetPositionMap().UTF8ToUTF16(d.Pos()), d.File().GetPositionMap().UTF8ToUTF16(d.End())}
				primaryLocation = sourcePosition(d.File(), d.Pos(), d.End())
			}
			phase := "syntax"
			if code == "r01-d-scalar-type" || code == "r01-d-integer-range" || code == "r02b-d-type" {
				phase = "types"
			}
			rule := "r01-ts-baseline"
			if code == "r01-d-integer-range" || code == "r01-d-scalar-type" {
				rule = "r01-literals"
			}
			result.Diagnostics = append(result.Diagnostics, diagnostic{Code: code, RuleID: rule, RuleIDs: []string{rule},
				Category: "error", Phase: phase, Severity: "error", File: name, Range: where,
				Primary: primaryLocation, UpstreamCode: d.Code()})
		}
	}
	if len(result.Diagnostics) != 0 {
		result.Status = "error"
		return result
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
		fn := function{Name: node.Name().Text(), Parameters: []symbol{}, Body: []statement{}, Range: offset(file, node)}
		fn.ReturnType = scalar(check.GetTypeFromTypeNode(decl.Type))
		if fn.ReturnType == "" {
			return failAt("r02b-d-unsupported", "unsupported", file, decl.Type)
		}
		project := projection{file: file, check: check, result: &result, bindings: map[*ast.Symbol]symbol{}}
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
