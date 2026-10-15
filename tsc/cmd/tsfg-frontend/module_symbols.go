// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Owns module model, official namespace/alias identity and scalar global bindings.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"strings"
)

type closureRoot struct {
	Kind       string `json:"kind"`
	SourcePath string `json:"sourcePath"`
	Name       string `json:"name"`
}
type closureConfiguration struct {
	Roots         []closureRoot `json:"roots"`
	RequiredRoots []closureRoot `json:"requiredRoots"`
}
type hostImport struct {
	SourcePath     string   `json:"sourcePath"`
	Name           string   `json:"name"`
	CapabilityID   uint32   `json:"capabilityId"`
	ParameterTypes []string `json:"parameterTypes"`
	ReturnType     string   `json:"returnType"`
}
type hostImportFact struct {
	hostImport
	Symbol string `json:"symbol"`
	Range  span   `json:"range"`
}
type importBinding struct {
	Name   string `json:"name"`
	Target string `json:"target"`
	Range  span   `json:"range"`
}
type importFact struct {
	Kind       string          `json:"kind"`
	Specifier  string          `json:"specifier"`
	SourcePath string          `json:"sourcePath"`
	Range      span            `json:"range"`
	Bindings   []importBinding `json:"bindings"`
}
type moduleFact struct {
	SourcePath     string          `json:"sourcePath"`
	External       bool            `json:"external"`
	Imports        []importFact    `json:"imports"`
	Exports        []importBinding `json:"exports"`
	Globals        []string        `json:"globals"`
	Initialization []statement     `json:"initialization"`
}

func moduleReference(check *checker.Checker, node *ast.Node) bool {
	if node.Kind == ast.KindIdentifier {
		return true
	}
	if node.Kind != ast.KindPropertyAccessExpression {
		return false
	}
	p := node.AsPropertyAccessExpression()
	if p.QuestionDotToken != nil || p.Expression.Kind != ast.KindIdentifier {
		return false
	}
	s := check.GetSymbolAtLocation(p.Expression)
	if s == nil || s.Flags&ast.SymbolFlagsAlias == 0 {
		return false
	}
	for _, d := range s.Declarations {
		if d.Kind == ast.KindNamespaceImport {
			return moduleSymbol(check, node) != nil
		}
	}
	return false
}

func moduleOwnedID(check *checker.Checker, s *ast.Symbol, functions map[*ast.Symbol]functionInfo, globals map[*ast.Symbol]symbol) string {
	if s == nil {
		return ""
	}
	s = moduleAliasTarget(check, s)
	if id := functions[s].ID; id != "" {
		return id
	}
	return globals[s].ID
}

func moduleFunctionSymbol(check *checker.Checker, node *ast.Node) *ast.Symbol {
	if node.Name() != nil {
		return check.GetSymbolAtLocation(node.Name())
	}
	return node.Symbol()
}

func moduleFunctionName(node *ast.Node) string {
	if node.Name() != nil {
		return node.Name().Text()
	}
	return "default"
}

func moduleFunctionAnchor(file *ast.SourceFile, node *ast.Node) span {
	if node.Name() != nil {
		return offset(file, node.Name())
	}
	return expressionOffset(file, node)
}

func moduleSymbol(check *checker.Checker, node *ast.Node) *ast.Symbol {
	return moduleAliasTarget(check, check.GetSymbolAtLocation(node))
}

// The checker may classify `export default identifier` as a type alias to
// identifier. Its runtime value is nevertheless evaluated once, unlike
// `export {identifier as default}`. Preserve this official declaration edge.
func moduleAliasTarget(check *checker.Checker, s *ast.Symbol) *ast.Symbol {
	seen := map[*ast.Symbol]bool{}
	for s != nil && s.Flags&ast.SymbolFlagsAlias != 0 {
		if seen[s] {
			return nil
		}
		seen[s] = true
		for _, d := range s.Declarations {
			if d.Kind == ast.KindExportAssignment {
				return d.Symbol()
			}
		}
		s = check.GetImmediateAliasedSymbol(s)
	}
	return s
}

func moduleHostContext(r request, program *compiler.Program, check *checker.Checker, functions map[*ast.Symbol]functionInfo) {
	for _, host := range r.HostImports {
		file := program.GetSourceFile(host.SourcePath)
		if file == nil {
			continue
		}
		for _, node := range file.Statements.Nodes {
			if node.Kind != ast.KindFunctionDeclaration || node.Name() == nil || node.Name().Text() != host.Name || node.Body() != nil {
				continue
			}
			decl := node.AsFunctionDeclaration()
			if decl.Type == nil || len(decl.Parameters.Nodes) != 1 {
				continue
			}
			parameter := decl.Parameters.Nodes[0]
			if parameter.Name() == nil || scalar(check.GetTypeAtLocation(parameter.Name())) != "i32" || scalar(check.GetTypeFromTypeNode(decl.Type)) != "i32" {
				continue
			}
			functions[check.GetSymbolAtLocation(node.Name())] = functionInfo{Signature: signature{Parameters: []string{"i32"}, ReturnType: "i32"}}
		}
	}
}

// Alias maps come only from the official checker, never guessed import paths.
func moduleAliases(program *compiler.Program, check *checker.Checker, functions map[*ast.Symbol]functionInfo, bindings map[*ast.Symbol]symbol) {
	for _, file := range program.GetSourceFiles() {
		for _, node := range file.Statements.Nodes {
			if node.Kind != ast.KindFunctionDeclaration || node.Name() != nil || node.Body() == nil {
				continue
			}
			decl := node.AsFunctionDeclaration()
			if decl.Type == nil {
				continue
			}
			s := moduleFunctionSymbol(check, node)
			if _, exists := functions[s]; exists {
				continue
			}
			t := scalar(check.GetTypeFromTypeNode(decl.Type))
			if t == "" {
				continue
			}
			parameters := []string{}
			presence := []*parameterPresence{}
			hasPresence := false
			valid := true
			for _, p := range decl.Parameters.Nodes {
				pt, shape, ok := checkedParameter(file, check, p)
				if !ok {
					valid = false
					break
				}
				parameters = append(parameters, pt)
				presence = append(presence, shape)
				hasPresence = hasPresence || shape != nil
			}
			if valid {
				if !hasPresence {
					presence = nil
				}
				functions[s] = functionInfo{Declaration: node, Signature: signature{Parameters: parameters, ReturnType: t, Presence: presence}}
			}
		}
	}
	for s, f := range functions {
		functions[check.GetExportSymbolOfSymbol(s)] = f
	}
	for s, b := range bindings {
		bindings[check.GetExportSymbolOfSymbol(s)] = b
	}
	for _, file := range program.GetSourceFiles() {
		var visit func(*ast.Node) bool
		visit = func(node *ast.Node) bool {
			if moduleReference(check, node) {
				alias := check.GetSymbolAtLocation(node)
				if alias != nil {
					target := moduleSymbol(check, node)
					if f, ok := functions[target]; ok {
						functions[alias] = f
					}
					if b, ok := bindings[target]; ok && bindings != nil {
						bindings[alias] = b
					}
				}
			}
			node.ForEachChild(visit)
			return false
		}
		file.ForEachChild(visit)
	}
}

func moduleGlobals(program *compiler.Program, check *checker.Checker) map[*ast.Symbol]symbol {
	out := map[*ast.Symbol]symbol{}
	for _, file := range program.GetSourceFiles() {
		if strings.HasSuffix(file.FileName(), ".d.ts") {
			continue
		}
		for _, node := range file.Statements.Nodes {
			if node.Kind == ast.KindExportAssignment && !node.AsExportAssignment().IsExportEquals {
				expr := node.AsExportAssignment().Expression
				t := scalar(check.GetTypeAtLocation(expr))
				if n, _, ok := exactInteger(file, expr); ok && i32InRange(n) {
					t = "i32"
				}
				out[node.Symbol()] = symbol{Name: "$default", Type: t, Kind: "global", BindingKind: "default-export-expression", Writable: false, Range: expressionOffset(file, expr), SourcePath: file.FileName()}
			}
			if node.Kind != ast.KindVariableStatement {
				continue
			}
			for _, decl := range localDeclarations(node) {
				if decl.Name() == nil {
					continue
				}
				local := decl.AsVariableDeclaration()
				t := ""
				if local.Type != nil {
					t = scalar(check.GetTypeFromTypeNode(local.Type))
				} else {
					t = scalar(check.GetTypeAtLocation(decl.Name()))
					if _, _, ok := exactInteger(file, local.Initializer); ok {
						t = "i32"
					}
				}
				out[check.GetSymbolAtLocation(decl.Name())] = symbol{Name: decl.Name().Text(), Type: t, Kind: "global", BindingKind: "source", Writable: node.AsVariableStatement().DeclarationList.Flags&ast.NodeFlagsLet != 0, Range: offset(file, decl.Name()), SourcePath: file.FileName()}
			}
		}
	}
	return out
}

// Adapt only an exact scalar expression context. Unrelated checker diagnostics
// (including an unentered function) remain in the normal program diagnostic set.
