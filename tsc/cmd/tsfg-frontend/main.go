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
	ID    string `json:"id"`
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Type  string `json:"type"`
	Range span   `json:"range"`
}

type expression struct {
	Kind    string          `json:"kind"`
	Type    string          `json:"type"`
	Symbol  string          `json:"symbol,omitempty"`
	Literal *integerLiteral `json:"literal,omitempty"`
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
	if code == "r01-d-integer-range" {
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
	start, end := operand.Pos(), operand.End()
	if start < 0 || end > len(file.Text()) || start >= end {
		return nil, nil, false
	}
	fragment := strings.TrimSpace(file.Text()[start:end])
	// The scanner owns token boundaries; the original source retains separators and radix spelling.
	index := len(fragment)
	for index > 0 {
		ch := fragment[index-1]
		if ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch == '_' {
			index--
			continue
		}
		break
	}
	token := fragment[index:]
	if token == "" {
		return nil, nil, false
	}
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

func localDeclaration(node *ast.Node) *ast.Node {
	if node.Kind != ast.KindVariableStatement {
		return nil
	}
	list := node.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
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
		body := fn.Body.AsBlock().Statements.Nodes
		if len(body) != 2 || body[1].Kind != ast.KindReturnStatement || d.Pos() < body[1].Pos() || d.End() > body[1].End() {
			continue
		}
		decl := localDeclaration(body[0])
		if decl == nil || decl.Name() == nil || decl.AsVariableDeclaration().Type != nil {
			continue
		}
		ret := body[1].AsReturnStatement().Expression
		if ret == nil || ret.Kind != ast.KindIdentifier || check.GetSymbolAtLocation(ret) != check.GetSymbolAtLocation(decl.Name()) {
			continue
		}
		init := decl.AsVariableDeclaration().Initializer
		if init == nil {
			continue
		}
		value, _, ok := exactInteger(file, init)
		if ok {
			return value, true
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
		for _, item := range node.Body().AsBlock().Statements.Nodes {
			decl := localDeclaration(item)
			if decl == nil || d.Pos() < decl.Pos() || d.End() > decl.End() {
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
			if code == "r02b-d-type" && scalarReturnMismatch(file, check, d) {
				code = "r01-d-scalar-type"
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
			if code == "r01-d-integer-range" {
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
		if len(body) < 1 || len(body) > 2 {
			return failAt("r02b-d-unsupported", "unsupported", file, node)
		}
		fn := function{Name: node.Name().Text(), Parameters: []symbol{}, Body: []statement{}, Range: offset(file, node)}
		fn.ReturnType = scalar(check.GetTypeFromTypeNode(decl.Type))
		if fn.ReturnType == "" {
			return failAt("r02b-d-unsupported", "unsupported", file, decl.Type)
		}
		for _, p := range decl.Parameters.Nodes {
			if p.Name() == nil || p.AsParameterDeclaration().Type == nil || p.AsParameterDeclaration().Initializer != nil {
				return failAt("r02b-d-unsupported", "unsupported", file, p)
			}
			t := scalar(check.GetTypeAtLocation(p.Name()))
			if t == "" {
				return failAt("r02b-d-unsupported", "unsupported", file, p)
			}
			s := symbol{ID: fmt.Sprintf("s%d", len(result.Symbols)+1), Name: p.Name().Text(), Kind: "parameter", Type: t, Range: offset(file, p.Name())}
			result.Symbols = append(result.Symbols, s)
			fn.Parameters = append(fn.Parameters, s)
		}
		var localNode *ast.Node
		var localSymbol *ast.Symbol
		if len(body) == 2 {
			localNode = localDeclaration(body[0])
			if localNode == nil || localNode.Name() == nil {
				return failAt("r02b-d-unsupported", "unsupported", file, body[0])
			}
			if localType := localNode.AsVariableDeclaration().Type; localType != nil && scalar(check.GetTypeFromTypeNode(localType)) != "i32" {
				return failAt("r02b-d-unsupported", "unsupported", file, localType)
			}
			init := localNode.AsVariableDeclaration().Initializer
			if init == nil {
				return failAt("r02b-d-unsupported", "unsupported", file, body[0])
			}
			value, literal, ok := exactInteger(file, init)
			if !ok {
				return failAt("r02b-d-unsupported", "unsupported", file, init)
			}
			if !i32InRange(value) {
				return failAt("r01-d-integer-range", "error", file, init)
			}
			localSymbol = check.GetSymbolAtLocation(localNode.Name())
			if localSymbol == nil {
				return failAt("r02b-d-type", "error", file, localNode.Name())
			}
			s := symbol{ID: fmt.Sprintf("s%d", len(result.Symbols)+1), Name: localNode.Name().Text(), Kind: "local", Type: "i32", Range: offset(file, localNode.Name())}
			result.Symbols = append(result.Symbols, s)
			fn.Body = append(fn.Body, statement{Kind: "variable", Symbol: &s, Initializer: &expression{Kind: "integer", Type: "i32", Literal: literal, Range: offset(file, init)}, Range: offset(file, body[0])})
		}
		bodyNode := body[len(body)-1]
		if bodyNode.Kind != ast.KindReturnStatement || bodyNode.AsReturnStatement().Expression == nil {
			return failAt("r02b-d-unsupported", "unsupported", file, bodyNode)
		}
		expr := bodyNode.AsReturnStatement().Expression
		if value, literal, ok := exactInteger(file, expr); ok && fn.ReturnType == "i32" {
			if !i32InRange(value) {
				return failAt("r01-d-integer-range", "error", file, expr)
			}
			fn.Body = append(fn.Body, statement{Kind: "return", Expression: &expression{Kind: "integer", Type: "i32", Literal: literal, Range: offset(file, expr)}, Range: offset(file, bodyNode)})
			result.Functions = append(result.Functions, fn)
			continue
		}
		if expr.Kind != ast.KindIdentifier {
			return failAt("r02b-d-unsupported", "unsupported", file, expr)
		}
		resolved := check.GetSymbolAtLocation(expr)
		ref := ""
		for i, p := range decl.Parameters.Nodes {
			if resolved != nil && resolved == check.GetSymbolAtLocation(p.Name()) {
				ref = fn.Parameters[i].ID
				break
			}
		}
		if localSymbol != nil && resolved == localSymbol {
			ref = fn.Body[0].Symbol.ID
		}
		if ref == "" || scalar(check.GetTypeAtLocation(expr)) != fn.ReturnType {
			if !(localSymbol != nil && resolved == localSymbol && fn.ReturnType == "i32") {
				return failAt("r01-d-scalar-type", "error", file, expr)
			}
		}
		fn.Body = append(fn.Body, statement{Kind: "return", Expression: &expression{Kind: "reference", Type: fn.ReturnType, Symbol: ref, Range: offset(file, expr)}, Range: offset(file, bodyNode)})
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
