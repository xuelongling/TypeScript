// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Builds deterministic owned module initialization and live bindings after whole-program checking.
package main

import (
	"fmt"
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/scanner"
	"maps"
	"slices"
	"strings"
)

func projectModules(r request, program *compiler.Program, check *checker.Checker, result response, conversions *conversionFacts, callableAliases map[*ast.Symbol]moduleCallableAliasProof, finish ownedProjectionHook) response {
	result.Modules = []moduleFact{}
	functions := map[*ast.Symbol]functionInfo{}
	if conversions != nil {
		maps.Copy(functions, conversions.Functions)
	}
	globals := moduleGlobals(program, check)
	if r.moduleCallable {
		// Keep the original variable symbol separate from its initializer target.
		for original, proof := range callableAliases {
			binding := globals[original]
			binding.Type = "function"
			binding.Signature = proof.ownedSignature
			globals[original] = binding
		}
	}
	files := []*ast.SourceFile{}
	for _, file := range program.GetSourceFiles() {
		if !strings.HasSuffix(file.FileName(), ".d.ts") {
			files = append(files, file)
		}
	}
	slices.SortFunc(files, func(a, b *ast.SourceFile) int { return strings.Compare(a.FileName(), b.FileName()) })
	for _, file := range files {
		if n := prohibitedNode(file); n != nil {
			return failAt("r02b-d-prohibited", "prohibited", file, n)
		}
		for _, node := range file.Statements.Nodes {
			if node.Kind == ast.KindFunctionDeclaration && node.Body() != nil {
				if r.moduleCallable {
					info, failure := moduleCallableHeader(file, check, node, &result)
					if failure.Status != "" {
						return failure
					}
					functions[moduleFunctionSymbol(check, node)] = info
					continue
				}
				decl := node.AsFunctionDeclaration()
				if decl.Type == nil || decl.Body.Kind != ast.KindBlock {
					return failAt("r02b-d-unsupported", "unsupported", file, node)
				}
				t := scalar(check.GetTypeFromTypeNode(decl.Type))
				parameters := []string{}
				presence := []*parameterPresence{}
				hasPresence := false
				if t == "" {
					return failAt("r02b-d-unsupported", "unsupported", file, decl.Type)
				}
				for _, p := range decl.Parameters.Nodes {
					pt, shape, valid := checkedParameter(file, check, p)
					if !valid {
						return failAt("r02b-d-unsupported", "unsupported", file, p)
					}
					parameters = append(parameters, pt)
					presence = append(presence, shape)
					hasPresence = hasPresence || shape != nil
				}
				s := symbol{ID: fmt.Sprintf("s%d", len(result.Symbols)+1), Name: moduleFunctionName(node), Kind: "function", Type: t, Range: moduleFunctionAnchor(file, node), SourcePath: file.FileName()}
				result.Symbols = append(result.Symbols, s)
				if !hasPresence {
					presence = nil
				}
				functions[moduleFunctionSymbol(check, node)] = functionInfo{ID: s.ID, Declaration: node, Signature: signature{Parameters: parameters, ReturnType: t, Presence: presence}}
			}
			if node.Kind == ast.KindExportAssignment {
				if node.AsExportAssignment().IsExportEquals {
					return failAt("r02b-d-unsupported", "unsupported", file, node)
				}
				s := globals[node.Symbol()]
				if s.Type == "" || (floatKind(s.Type) && (r.Target == nil || !r.Target.floats)) {
					return failAt("r02b-d-unsupported", "unsupported", file, node)
				}
				s.ID = fmt.Sprintf("s%d", len(result.Symbols)+1)
				globals[node.Symbol()] = s
				result.Symbols = append(result.Symbols, s)
			}
			if node.Kind == ast.KindVariableStatement {
				if node.AsVariableStatement().DeclarationList.Flags&(ast.NodeFlagsLet|ast.NodeFlagsConst) == 0 {
					return failAt("r02b-d-unsupported", "unsupported", file, node)
				}
				for _, decl := range localDeclarations(node) {
					upstream := check.GetSymbolAtLocation(decl.Name())
					s := globals[upstream]
					if s.Type == "" || (floatKind(s.Type) && (r.Target == nil || !r.Target.floats)) || decl.AsVariableDeclaration().Initializer == nil {
						return failAt("r02b-d-unsupported", "unsupported", file, decl)
					}
					s.ID = fmt.Sprintf("s%d", len(result.Symbols)+1)
					globals[upstream] = s
					result.Symbols = append(result.Symbols, s)
				}
			}
		}
	}
	for _, host := range r.HostImports {
		file := program.GetSourceFile(host.SourcePath)
		if file == nil || !strings.HasSuffix(host.SourcePath, ".d.ts") || host.CapabilityID != 31 || host.ReturnType != "i32" || len(host.ParameterTypes) != 1 || host.ParameterTypes[0] != "i32" {
			return fail("r02b-d-input", "input", host.SourcePath, span{})
		}
		var found *ast.Node
		for _, node := range file.Statements.Nodes {
			if node.Kind == ast.KindFunctionDeclaration && node.Name() != nil && node.Name().Text() == host.Name {
				found = node
				break
			}
		}
		if found == nil || found.Body() != nil {
			return fail("r02b-d-input", "input", host.SourcePath, span{})
		}
		decl := found.AsFunctionDeclaration()
		if decl.Type == nil || scalar(check.GetTypeFromTypeNode(decl.Type)) != "i32" || len(decl.Parameters.Nodes) != 1 || scalar(check.GetTypeAtLocation(decl.Parameters.Nodes[0].Name())) != "i32" {
			return failAt("r02b-d-input", "input", file, found)
		}
		s := symbol{ID: fmt.Sprintf("s%d", len(result.Symbols)+1), Name: host.Name, Kind: "host-function", Type: "i32", Range: offset(file, found.Name()), SourcePath: host.SourcePath}
		result.Symbols = append(result.Symbols, s)
		functions[check.GetSymbolAtLocation(found.Name())] = functionInfo{ID: s.ID, Signature: signature{Parameters: host.ParameterTypes, ReturnType: host.ReturnType}}
		result.HostImports = append(result.HostImports, hostImportFact{hostImport: host, Symbol: s.ID, Range: offset(file, found.Name())})
	}
	moduleAliases(program, check, functions, globals)
	for _, file := range files {
		m := moduleFact{SourcePath: file.FileName(), External: ast.IsExternalModule(file), Imports: []importFact{}, Exports: []importBinding{}, Globals: []string{}, Initialization: []statement{}}
		if m.External {
			for _, exported := range check.GetExportsOfModule(file.AsNode().Symbol()) {
				if check.GetTypeOnlyAliasDeclaration(exported) != nil {
					continue
				}
				id := moduleOwnedID(check, exported, functions, globals)
				if id == "" {
					continue
				} // Type-only exports have no scalar runtime binding.
				anchor := offset(file, file.AsNode())
				for _, d := range exported.Declarations {
					if ast.GetSourceFileOfNode(d) == file {
						anchor = offset(file, d)
						break
					}
				}
				m.Exports = append(m.Exports, importBinding{Name: exported.Name, Target: id, Range: anchor})
			}
			slices.SortFunc(m.Exports, func(a, b importBinding) int { return strings.Compare(a.Name, b.Name) })
		}
		for _, node := range file.Statements.Nodes {
			switch node.Kind {
			case ast.KindImportDeclaration:
				imp := node.AsImportDeclaration()
				if imp.ImportClause != nil && ast.IsTypeOnlyImportDeclaration(imp.ImportClause) {
					continue
				}
				if imp.ImportClause != nil {
					clause := imp.ImportClause.AsImportClause()
					if clause.Name() == nil && clause.NamedBindings != nil && clause.NamedBindings.Kind == ast.KindNamedImports && len(clause.NamedBindings.AsNamedImports().Elements.Nodes) > 0 {
						allType := true
						for _, b := range clause.NamedBindings.AsNamedImports().Elements.Nodes {
							if !b.AsImportSpecifier().IsTypeOnly {
								allType = false
							}
						}
						if allType {
							continue
						}
					}
				}
				resolved := check.ResolveExternalModuleName(imp.ModuleSpecifier)
				if resolved == nil || len(resolved.Declarations) == 0 {
					return failAt("r02b-d-input", "input", file, node)
				}
				targetFile := ast.GetSourceFileOfNode(resolved.Declarations[0])
				if targetFile == nil || strings.HasSuffix(targetFile.FileName(), ".d.ts") {
					return failAt("r02b-d-unsupported", "unsupported", file, node)
				}
				owned := importFact{Kind: "import", Specifier: imp.ModuleSpecifier.Text(), SourcePath: targetFile.FileName(), Range: offset(file, node), Bindings: []importBinding{}}
				if imp.ImportClause != nil {
					clause := imp.ImportClause.AsImportClause()
					bindings := []*ast.Node{}
					if clause.Name() != nil {
						bindings = append(bindings, clause.Name())
					}
					if clause.NamedBindings != nil {
						if clause.NamedBindings.Kind == ast.KindNamedImports {
							for _, b := range clause.NamedBindings.AsNamedImports().Elements.Nodes {
								if b.AsImportSpecifier().IsTypeOnly {
									continue
								}
								bindings = append(bindings, b.Name())
							}
						} else if clause.NamedBindings.Kind == ast.KindNamespaceImport {
							base := check.GetSymbolAtLocation(clause.NamedBindings.Name())
							var members func(*ast.Node) bool
							members = func(n *ast.Node) bool {
								if moduleReference(check, n) && n.Kind == ast.KindPropertyAccessExpression && check.GetSymbolAtLocation(n.AsPropertyAccessExpression().Expression) == base {
									bindings = append(bindings, n)
								}
								n.ForEachChild(members)
								return false
							}
							file.ForEachChild(members)
						} else {
							return failAt("r02b-d-unsupported", "unsupported", file, clause.NamedBindings)
						}
					}
					for _, binding := range bindings {
						id := moduleOwnedID(check, moduleSymbol(check, binding), functions, globals)
						if id == "" {
							return failAt("r02b-d-unsupported", "unsupported", file, binding)
						}
						name := file.Text()[scanner.SkipTrivia(file.Text(), binding.Pos()):binding.End()]
						owned.Bindings = append(owned.Bindings, importBinding{Name: name, Target: id, Range: expressionOffset(file, binding)})
					}
				}
				m.Imports = append(m.Imports, owned)
			case ast.KindFunctionDeclaration:
				decl := node.AsFunctionDeclaration()
				if decl.Body == nil {
					return failAt("r02b-d-unsupported", "unsupported", file, node)
				}
				info := functions[moduleFunctionSymbol(check, node)]
				fn := function{SourcePath: file.FileName(), Name: moduleFunctionName(node), Symbol: info.ID, Parameters: []symbol{}, Body: []statement{}, Range: offset(file, node), ReturnType: info.Signature.ReturnType}
				if r.moduleCallable {
					fn.ReturnSignature = info.Signature.ReturnSignature
				}
				bindings := map[*ast.Symbol]symbol{}
				for s, b := range globals {
					bindings[s] = b
				}
				p := projection{file: file, check: check, result: &result, bindings: bindings, functions: functions, integerTarget: r.Target}
				if failure := p.parameters(&fn, decl.Parameters.Nodes, info.Signature.Presence != nil && !r.moduleCallable); failure.Status != "" {
					return failure
				}
				body, failure := p.statements(decl.Body.AsBlock().Statements.Nodes, fn.ReturnType)
				if failure.Status != "" {
					return failure
				}
				fn.Body = body
				annotations := append([]statement{}, fn.Body...)
				for _, initializer := range fn.ParameterInitializers {
					annotations = append(annotations, statement{Expression: initializer})
				}
				moduleAnnotate(file, annotations)
				if r.moduleCallable {
					moduleCallableNames(file, annotations)
				}
				result.Functions = append(result.Functions, fn)
			case ast.KindVariableStatement:
				p := projection{file: file, check: check, result: &result, bindings: globals, functions: functions, integerTarget: r.Target}
				for _, decl := range localDeclarations(node) {
					s := globals[check.GetSymbolAtLocation(decl.Name())]
					value, failure := p.expr(decl.AsVariableDeclaration().Initializer, s.Type)
					if failure.Status != "" {
						return failure
					}
					m.Globals = append(m.Globals, s.ID)
					m.Initialization = append(m.Initialization, statement{Kind: "initialize", Symbol: &s, Initializer: value, Range: offset(file, decl)})
				}
			case ast.KindExportDeclaration:
				export := node.AsExportDeclaration()
				if export.ExportClause != nil && export.ExportClause.Kind == ast.KindNamedExports && len(export.ExportClause.AsNamedExports().Elements.Nodes) > 0 {
					allType := true
					for _, b := range export.ExportClause.AsNamedExports().Elements.Nodes {
						if !b.AsExportSpecifier().IsTypeOnly {
							allType = false
						}
					}
					if allType {
						continue
					}
				}
				if export.ModuleSpecifier != nil && !export.IsTypeOnly {
					resolved := check.ResolveExternalModuleName(export.ModuleSpecifier)
					if resolved == nil || len(resolved.Declarations) == 0 {
						return failAt("r02b-d-input", "input", file, node)
					}
					target := ast.GetSourceFileOfNode(resolved.Declarations[0])
					if target == nil || strings.HasSuffix(target.FileName(), ".d.ts") {
						return failAt("r02b-d-unsupported", "unsupported", file, node)
					}
					m.Imports = append(m.Imports, importFact{Kind: "export", Specifier: export.ModuleSpecifier.Text(), SourcePath: target.FileName(), Range: offset(file, node), Bindings: []importBinding{}})
				}
			case ast.KindExportAssignment:
				p := projection{file: file, check: check, result: &result, bindings: globals, functions: functions, integerTarget: r.Target}
				s := globals[node.Symbol()]
				value, failure := p.expr(node.AsExportAssignment().Expression, s.Type)
				if failure.Status != "" {
					return failure
				}
				m.Globals = append(m.Globals, s.ID)
				m.Initialization = append(m.Initialization, statement{Kind: "initialize", Symbol: &s, Initializer: value, Range: offset(file, node)})
			default:
				p := projection{file: file, check: check, result: &result, bindings: globals, functions: functions, integerTarget: r.Target}
				body, failure := p.statements([]*ast.Node{node}, "")
				if failure.Status != "" {
					return failure
				}
				m.Initialization = append(m.Initialization, body...)
			}
		}
		moduleAnnotate(file, m.Initialization)
		if r.moduleCallable {
			moduleCallableNames(file, m.Initialization)
		}
		result.Modules = append(result.Modules, m)
	}
	if finish != nil {
		root := program.GetSourceFile(r.RootFile)
		finish(checkedProjectionContext(root, program, check, functions, result))
	}
	return result
}

func moduleAnnotate(file *ast.SourceFile, statements []statement) {
	names := map[span]string{}
	var visit func(*ast.Node) bool
	visit = func(n *ast.Node) bool {
		if n.Kind == ast.KindIdentifier || n.Kind == ast.KindPropertyAccessExpression {
			text := file.Text()[scanner.SkipTrivia(file.Text(), n.Pos()):n.End()]
			names[expressionOffset(file, n)] = text
			names[offset(file, n)] = text
		}
		if n.Kind == ast.KindCallExpression {
			callee := n.AsCallExpression().Expression
			text := file.Text()[scanner.SkipTrivia(file.Text(), callee.Pos()):callee.End()]
			names[expressionOffset(file, n)] = text
			names[offset(file, n)] = text
		}
		n.ForEachChild(visit)
		return false
	}
	file.ForEachChild(visit)
	var expressionNames func(*expression)
	expressionNames = func(e *expression) {
		if e == nil {
			return
		}
		if e.Kind == "reference" || e.Kind == "call" {
			e.Name = names[e.Range]
		}
		expressionNames(e.Left)
		expressionNames(e.Right)
		expressionNames(e.Operand)
		if e.Arguments != nil {
			for i := range *e.Arguments {
				expressionNames(&(*e.Arguments)[i])
			}
		}
	}
	walkOwnedStatements(statements, func(s *statement) {
		expressionNames(s.Expression)
		expressionNames(s.Initializer)
		expressionNames(s.Target)
		expressionNames(s.Value)
		expressionNames(s.Condition)
	})
}
