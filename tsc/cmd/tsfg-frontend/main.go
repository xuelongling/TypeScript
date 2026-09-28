// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// tsfg-frontend projects a deliberately small, owned analysis result from the
// official TypeScript frontend. It never emits or executes JavaScript.
package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	ID    string `json:"id"`
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Type  string `json:"type"`
	Range span   `json:"range"`
}

type expression struct {
	Kind   string `json:"kind"`
	Type   string `json:"type"`
	Symbol string `json:"symbol,omitempty"`
	Range  span   `json:"range"`
}

type statement struct {
	Kind       string     `json:"kind"`
	Expression expression `json:"expression"`
	Range      span       `json:"range"`
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
	Inputs      []string     `json:"inputs,omitempty"`
	Functions   []function   `json:"functions,omitempty"`
	Symbols     []symbol     `json:"symbols,omitempty"`
	Diagnostics []diagnostic `json:"diagnostics,omitempty"`
}

func fail(code, category, file string, location span) response {
	phase := "syntax"
	if code == "r01-d-scalar-type" {
		phase = "types"
	}
	return response{Status: "error", Diagnostics: []diagnostic{{Code: code, RuleID: "r01-ts-baseline", RuleIDs: []string{"r01-ts-baseline"}, Category: category,
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
		r.Declarations.Version != "minimal-bool-v1" || !r.Config.Strict || !r.Config.NoEmit || r.Config.AllowJS || len(r.Files) == 0 || len(r.Declarations.Paths) == 0 {
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

func scalar(t *checker.Type) string {
	if t != nil && t.IsBooleanLike() {
		return "bool"
	}
	return ""
}

func boolReturnMismatch(file *ast.SourceFile, check *checker.Checker, d *ast.Diagnostic) bool {
	if d.File() != file {
		return false
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil {
			continue
		}
		decl := node.AsFunctionDeclaration()
		if decl.Type == nil || scalar(check.GetTypeFromTypeNode(decl.Type)) != "bool" || decl.Body.Kind != ast.KindBlock {
			continue
		}
		body := decl.Body.AsBlock().Statements.Nodes
		if len(body) != 1 || body[0].Kind != ast.KindReturnStatement {
			continue
		}
		expr := body[0].AsReturnStatement().Expression
		if expr != nil && d.Pos() >= body[0].Pos() && d.End() <= body[0].End() {
			return true
		}
	}
	return false
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
	for _, group := range groups {
		for _, d := range group.items {
			code := group.code
			if d.Code() == 6053 || d.Code() == 2688 || d.Code() == 2307 {
				return fail("r02b-d-input", "input", r.RootFile, span{})
			}
			if d.Code() == 6504 {
				code = "r02b-d-input"
			}
			if code == "r02b-d-type" && d.Code() == 2322 && boolReturnMismatch(file, check, d) {
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
			if code == "r01-d-scalar-type" || code == "r02b-d-type" {
				phase = "types"
			}
			ruleID, category := "r01-ts-baseline", "error"
			if code == "r02b-d-input" {
				ruleID, category = "r01-input-boundary", "input"
			}
			result.Diagnostics = append(result.Diagnostics, diagnostic{Code: code, RuleID: ruleID, RuleIDs: []string{ruleID},
				Category: category, Phase: phase, Severity: "error", File: name, Range: where,
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
		if decl.Type == nil || decl.Body.Kind != ast.KindBlock || len(decl.Body.AsBlock().Statements.Nodes) != 1 {
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
		bodyNode := decl.Body.AsBlock().Statements.Nodes[0]
		if bodyNode.Kind != ast.KindReturnStatement || bodyNode.AsReturnStatement().Expression == nil {
			return failAt("r02b-d-unsupported", "unsupported", file, bodyNode)
		}
		expr := bodyNode.AsReturnStatement().Expression
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
		if ref == "" || scalar(check.GetTypeAtLocation(expr)) != "bool" {
			return failAt("r01-d-scalar-type", "error", file, expr)
		}
		fn.Body = append(fn.Body, statement{Kind: "return", Expression: expression{Kind: "reference", Type: "bool", Symbol: ref, Range: offset(file, expr)}, Range: offset(file, bodyNode)})
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
