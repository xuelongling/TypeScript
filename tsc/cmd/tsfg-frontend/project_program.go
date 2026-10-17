// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Runs official checking before selecting the ordinary or multi-source owned projection.
package main

import (
	"context"
	"fmt"
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
	"maps"
	"slices"
	"strings"
)

func analyze(r request) response {
	if !valid(r) {
		return fail("r02b-d-input", "input", r.RootFile, span{})
	}
	if r.Target != nil {
		r.Target.floats = r.Declarations.Version == "minimal-floats-v4" || r.Declarations.Version == conversionVersion
	}
	contents := map[string]string{}
	for _, file := range r.Files {
		contents[file.Path] = file.Content
	}
	fileNames := append([]string{r.RootFile}, r.Declarations.Paths...)
	if r.CompilationClosure != nil {
		fileNames = nil
		for _, input := range r.Files {
			fileNames = append(fileNames, input.Path)
		}
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
	syntacticDiagnostics := program.GetSyntacticDiagnostics(context.Background(), nil)
	check, release := program.GetTypeCheckerForFileExclusive(context.Background(), file)
	contextFunctions := operatorFunctions(file, check)
	if r.CompilationClosure != nil {
		for _, source := range program.GetSourceFiles() {
			maps.Copy(contextFunctions, operatorFunctions(source, check))
		}
		moduleHostContext(r, program, check, contextFunctions)
		moduleAliases(program, check, contextFunctions, nil)
	}
	var conversions *conversionFacts
	var aggregates *aggregateProjection
	// Custom admission assumes complete syntax and must not replace parser diagnostics.
	if len(syntacticDiagnostics) == 0 {
		if r.Declarations.Version == conversionVersion {
			var failure *response
			conversions, failure = admittedConversions(r, program, check)
			if failure != nil {
				release()
				return *failure
			}
			if failure = verifyProgramConversionNominals(r, program, check, file, conversions); failure != nil {
				release()
				return *failure
			}
			maps.Copy(contextFunctions, conversions.Functions)
		}
		var aggregateFailure *response
		aggregates, aggregateFailure = admittedAggregates(r, program, check, file, &result)
		if aggregateFailure != nil {
			release()
			return *aggregateFailure
		}
		if aggregates != nil {
			if failure := aggregates.constructorHeaders(file, check, &result); failure != nil {
				release()
				return *failure
			}
			maps.Copy(contextFunctions, aggregates.functionContexts(file, check))
		}
		if r.Target != nil {
			check.SetTSFGScalarAssignments(programScalarAssignmentContexts(r, program, check, file, contextFunctions))
		}
	}
	release()
	groups := []struct {
		items []*ast.Diagnostic
		code  string
	}{
		{syntacticDiagnostics, "r02b-d-syntax"},
		{program.GetSemanticDiagnostics(context.Background(), nil), "r02b-d-type"},
		{program.GetProgramDiagnostics(), "r02b-d-type"},
		{program.GetGlobalDiagnostics(context.Background()), "r02b-d-type"},
	}
	configured := check
	check, release = program.GetTypeCheckerForFileExclusive(context.Background(), file)
	defer release()
	if check != configured {
		return fail("r02b-d-input", "input", r.RootFile, span{})
	}
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
			bindings := operatorBindings(file, check, decl)
			if r.Declarations.Version == "minimal-integers-v3" || r.Declarations.Version == "minimal-floats-v4" || r.Declarations.Version == conversionVersion {
				continue
			}
			for _, item := range decl.Body.AsBlock().Statements.Nodes {
				if item.Kind != ast.KindReturnStatement || item.AsReturnStatement().Expression == nil {
					continue
				}
				expr := item.AsReturnStatement().Expression
				if expr.Kind == ast.KindBinaryExpression || expr.Kind == ast.KindPrefixUnaryExpression || expr.Kind == ast.KindParenthesizedExpression {
					_, failure := projectOperator(file, check, expr, bindings, contextFunctions)
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
			diagnosticFile := file
			if r.CompilationClosure != nil && d.File() != nil {
				diagnosticFile = d.File()
			}
			if r.CompilationClosure != nil {
				if handled, failure := moduleContextDiagnostic(program, check, d, contextFunctions, r.Target); handled {
					if failure != nil {
						result.Diagnostics = append(result.Diagnostics, failure.Diagnostics...)
					}
					continue
				}
			}
			if aggregates != nil {
				if handled, failure := aggregates.contextDiagnostic(file, check, d, &result, contextFunctions, r.Target); handled {
					if failure == nil {
						continue
					}
					owned := failure.Diagnostics[0]
					owned.UpstreamCode = d.Code()
					result.Diagnostics = append(result.Diagnostics, owned)
					continue
				}
			}
			if r.Declarations.Version == "minimal-integers-v3" || r.Declarations.Version == "minimal-floats-v4" || r.Declarations.Version == conversionVersion {
				if handled, failure := integerContextDiagnostic(diagnosticFile, check, d, contextFunctions, r.Target); handled {
					if failure == nil {
						continue
					}
					owned := failure.Diagnostics[0]
					// Float overflow is a token fact from exact rounding. Keep its
					// literal anchor rather than the checker's wider assignment span.
					if owned.Code != "r01-d-float-range" {
						owned.Range = span{d.File().GetPositionMap().UTF8ToUTF16(d.Pos()), d.File().GetPositionMap().UTF8ToUTF16(d.End())}
						owned.Primary = sourcePosition(d.File(), d.Pos(), d.End())
					}
					owned.UpstreamCode = d.Code()
					result.Diagnostics = append(result.Diagnostics, owned)
					continue
				}
			} else {
				if value, ok := i32LiteralCallArgument(diagnosticFile, check, d); ok {
					if i32InRange(value) {
						continue
					}
					code = "r01-d-integer-range"
				}
				callCode, isCall := callDiagnostic(diagnosticFile, d)
				if isCall && code != "r01-d-integer-range" {
					code = callCode
				}
				if supportedOperatorReturn(diagnosticFile, check, d, contextFunctions) || supportedOperatorAssignment(diagnosticFile, check, d, contextFunctions) || supportedOperatorLocal(diagnosticFile, check, d, contextFunctions) || supportedOperatorCallArgument(diagnosticFile, check, d, contextFunctions) {
					continue
				}
				if value, ok := i32LiteralReturn(diagnosticFile, check, d); ok {
					if i32InRange(value) {
						continue
					}
					code = "r01-d-integer-range"
				}
				if value, ok := defaultI32LocalReturn(diagnosticFile, check, d); ok {
					if i32InRange(value) {
						continue
					}
					code = "r01-d-integer-range"
				}
				if value, ok := contextualI32Local(diagnosticFile, check, d); ok {
					if i32InRange(value) {
						continue
					}
					code = "r01-d-integer-range"
				}
				if value, ok := contextualI32Assignment(diagnosticFile, check, d); ok {
					if i32InRange(value) {
						continue
					}
					code = "r01-d-integer-range"
				}
				if code == "r02b-d-type" && (scalarReturnMismatch(diagnosticFile, check, d) ||
					scalarAssignmentMismatch(diagnosticFile, check, d) || scalarLocalMismatch(diagnosticFile, check, d)) {
					code = "r01-d-scalar-type"
				}
			}
			_, isCall := callDiagnostic(diagnosticFile, d)
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
			result.Diagnostics = append(result.Diagnostics, diagnostic{
				Code: code, RuleID: rule, RuleIDs: []string{rule},
				Category: category, Phase: phase, Severity: "error", File: name, Range: where,
				Primary: primaryLocation, UpstreamCode: d.Code(),
			})
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
	if r.CompilationClosure != nil {
		return projectModules(r, program, check, result, conversions)
	}
	nodes, closures := closureDeclarations(file)
	functions := map[*ast.Symbol]functionInfo{}
	if conversions != nil {
		maps.Copy(functions, conversions.Functions)
	}
	for _, node := range nodes {
		if aggregates != nil && node.Kind == ast.KindClassDeclaration {
			continue
		}
		if node.Kind != ast.KindFunctionDeclaration || node.Name() == nil || node.Body() == nil {
			return failAt("r02b-d-unsupported", "unsupported", file, node)
		}
		decl := node.AsFunctionDeclaration()
		if decl.Type == nil || decl.Body.Kind != ast.KindBlock {
			return failAt("r02b-d-unsupported", "unsupported", file, node)
		}
		returnType := scalar(check.GetTypeFromTypeNode(decl.Type))
		var returnShape *returnPresence
		if returnType == "" && aggregates == nil && r.Target != nil {
			returnType, returnShape, _ = checkedNullableReturn(file, check, node)
		}
		if aggregates != nil && returnType == "" {
			returnType = aggregates.typeName(check.GetTypeFromTypeNode(decl.Type))
		}
		if returnType == "" {
			return failAt("r02b-d-unsupported", "unsupported", file, decl.Type)
		}
		parameters := make([]string, 0, len(decl.Parameters.Nodes))
		presence := make([]*parameterPresence, 0, len(decl.Parameters.Nodes))
		hasPresence := false
		for _, p := range decl.Parameters.Nodes {
			t, shape, valid := checkedParameter(file, check, p)
			if aggregates != nil && !valid {
				t, shape, valid = aggregates.parameter(file, check, p)
			}
			if !valid {
				return failAt("r02b-d-unsupported", "unsupported", file, p)
			}
			parameters = append(parameters, t)
			presence = append(presence, shape)
			hasPresence = hasPresence || shape != nil
		}
		resolved := check.GetSymbolAtLocation(node.Name())
		if resolved == nil {
			return failAt("r02b-d-type", "error", file, node.Name())
		}
		s := symbol{
			ID: fmt.Sprintf("s%d", len(result.Symbols)+1), Name: node.Name().Text(),
			Kind: "function", Type: returnType, Range: offset(file, node.Name()), ReturnPresence: returnShape,
		}
		closures.register(node, &s)
		result.Symbols = append(result.Symbols, s)
		if !hasPresence {
			presence = nil
		}
		functions[resolved] = functionInfo{ID: s.ID, Declaration: node, Signature: signature{Parameters: parameters, ReturnType: returnType, Presence: presence, ReturnPresence: returnShape}}
	}
	if aggregates != nil {
		if failure := aggregates.constructorBodies(file, check, &result, functions, r.Target); failure != nil {
			return *failure
		}
		if failure := aggregates.coverage(file, check, functions); failure != nil {
			return *failure
		}
		result.AggregateFacts = aggregates.facts
	}
	for _, node := range nodes {
		if aggregates != nil && node.Kind == ast.KindClassDeclaration {
			continue
		}
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
		fn.ReturnPresence = info.Signature.ReturnPresence
		fn.ReturnType = scalar(check.GetTypeFromTypeNode(decl.Type))
		if fn.ReturnPresence != nil {
			fn.ReturnType = info.Signature.ReturnType
		}
		if aggregates != nil && fn.ReturnType == "" {
			fn.ReturnType = aggregates.typeName(check.GetTypeFromTypeNode(decl.Type))
		}
		if fn.ReturnType == "" {
			return failAt("r02b-d-unsupported", "unsupported", file, decl.Type)
		}
		project := projection{file: file, check: check, result: &result, bindings: map[*ast.Symbol]symbol{}, functions: functions, aggregates: aggregates}
		closures.enter(node, &project)
		if r.Declarations.Version == "minimal-integers-v3" || r.Declarations.Version == "minimal-floats-v4" || r.Declarations.Version == conversionVersion {
			project.integerTarget = r.Target
		}
		if failure := project.parameters(&fn, decl.Parameters.Nodes, info.Signature.Presence != nil); failure.Status != "" {
			return failure
		}
		projected, failure := project.statements(body, fn.ReturnType, fn.ReturnPresence)
		if failure.Status != "" {
			return failure
		}
		fn.Body = projected
		closures.complete(node, &fn, &project)
		result.Functions = append(result.Functions, fn)
	}
	if len(result.Functions) == 0 {
		return fail("r02b-d-unsupported", "unsupported", r.RootFile, span{})
	}
	return result
}
