// SPDX-License-Identifier: Apache-2.0

// Restricts the private grammar and owns the complete official export table.
// Symbol/AST objects stay inside the same analyzer callback; only owned facts leave.
package main

import (
	"slices"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

func nullableFactsPresence(shape *parameterPresence) bool {
	return shape == nil || !shape.Undefined && !shape.Optional && !shape.Default
}

func nullableFactsGrammar(file *ast.SourceFile, facts response) error {
	if len(facts.Functions) == 0 || len(facts.Functions) > 512 || len(facts.Symbols) > 4096 || len(file.Statements.Nodes) != len(facts.Functions) {
		return nullableFactsError("limit", "top-level function/symbol table")
	}
	for _, fn := range facts.Functions {
		if fn.ReturnType != "i32" || fn.ReturnSignature != nil || fn.LexicalParent != "" || len(fn.Captures) != 0 ||
			fn.ReturnPresence != nil && (fn.ReturnPresence.Undefined || !fn.ReturnPresence.Null) || len(fn.Parameters) > 64 {
			return nullableFactsError("capability", "only ordinary i32/i32|null functions")
		}
		for _, parameter := range fn.Parameters {
			if parameter.Type != "i32" || parameter.Signature != nil || !nullableFactsPresence(parameter.Presence) {
				return nullableFactsError("capability", "only required i32/i32|null parameters")
			}
		}
		for _, initializer := range fn.ParameterInitializers {
			if initializer != nil {
				return nullableFactsError("capability", "default parameters are not represented")
			}
		}
	}
	for _, symbol := range facts.Symbols {
		if symbol.Type != "i32" || symbol.Signature != nil || !nullableFactsPresence(symbol.Presence) ||
			(symbol.Kind != "function" && symbol.Kind != "parameter" && symbol.Kind != "local") || symbol.Kind == "local" && symbol.Writable {
			return nullableFactsError("capability", "only ordinary scalar/nullable bindings and const locals")
		}
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Name() == nil || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			return nullableFactsError("capability", "only top-level named function bodies")
		}
		decl := node.AsFunctionDeclaration()
		if decl.TypeParameters != nil {
			return nullableFactsError("capability", "generic functions are not represented")
		}
		for _, parameter := range decl.Parameters.Nodes {
			p := parameter.AsParameterDeclaration()
			if p.QuestionToken != nil || p.Initializer != nil || p.DotDotDotToken != nil || p.Type == nil {
				return nullableFactsError("capability", "required explicitly typed parameters only")
			}
		}
		body := node.Body().AsBlock().Statements.Nodes
		if len(body) == 0 || body[len(body)-1].Kind != ast.KindReturnStatement {
			return nullableFactsError("capability", "body must end in one return")
		}
		for i, statement := range body {
			switch statement.Kind {
			case ast.KindReturnStatement:
				if i != len(body)-1 {
					return nullableFactsError("capability", "return must terminate the body")
				}
				if err := nullableFactsExpression(statement.AsReturnStatement().Expression, 0); err != nil {
					return err
				}
			case ast.KindVariableStatement:
				list := statement.AsVariableStatement().DeclarationList
				if list.Flags&ast.NodeFlagsConst == 0 {
					return nullableFactsError("capability", "only const local initialization")
				}
				for _, local := range list.AsVariableDeclarationList().Declarations.Nodes {
					if local.Kind != ast.KindVariableDeclaration || local.Name() == nil || local.Name().Kind != ast.KindIdentifier || local.AsVariableDeclaration().Type == nil {
						return nullableFactsError("capability", "explicitly typed local declaration required")
					}
					if err := nullableFactsExpression(local.AsVariableDeclaration().Initializer, 0); err != nil {
						return err
					}
				}
			default:
				return nullableFactsError("capability", "only const initialization and return statements")
			}
		}
	}
	return nil
}

func nullableFactsExpression(node *ast.Node, depth int) error {
	if depth > 128 {
		return nullableFactsError("limit", "expression depth")
	}
	if node == nil {
		return nullableFactsError("capability", "required value expression")
	}
	switch node.Kind {
	case ast.KindIdentifier:
		return nil
	case ast.KindCallExpression:
		call := node.AsCallExpression()
		if call.Expression.Kind != ast.KindIdentifier || call.QuestionDotToken != nil || call.TypeArguments != nil {
			return nullableFactsError("capability", "only direct ordinary calls")
		}
		for _, argument := range call.Arguments.Nodes {
			if err := nullableFactsExpression(argument, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	return nullableFactsError("capability", "only checked references and direct calls")
}

func projectNullableFacts(file *ast.SourceFile, check *checker.Checker, functions map[*ast.Symbol]functionInfo, facts response) ([]nullableFactsExport, error) {
	if err := nullableFactsGrammar(file, facts); err != nil {
		return nil, err
	}
	// A script has no module symbol. Never ask the checker for exports of nil.
	if !ast.IsExternalModule(file) || file.AsNode().Symbol() == nil {
		return nil, nullableFactsError("export", "one direct scalar entry export is required")
	}
	topLevel := map[*ast.Node]bool{}
	for _, node := range file.Statements.Nodes {
		topLevel[node] = true
	}
	owned := make([]nullableFactsExport, 0)
	for _, exported := range check.GetExportsOfModule(file.AsNode().Symbol()) {
		if exported == nil || exported.Flags&ast.SymbolFlagsAlias != 0 || len(exported.Declarations) != 1 {
			return nil, nullableFactsError("export", "aliases or ambiguous export declarations")
		}
		declaration := exported.Declarations[0]
		if !topLevel[declaration] || ast.GetSourceFileOfNode(declaration) != file || declaration.Kind != ast.KindFunctionDeclaration || declaration.Name() == nil || exported.Name != declaration.Name().Text() {
			return nil, nullableFactsError("export", "foreign/nonfunction/renamed export declaration")
		}
		// Export and local symbols may differ; the actual declaration binds the
		// local checker symbol, never a same-spelled lookup or alias fallback.
		actual := check.GetSymbolAtLocation(declaration.Name())
		info, found := functions[actual]
		if actual == nil || !found || info.ID == "" || info.Declaration != declaration {
			return nil, nullableFactsError("export", "foreign/nonfunction/renamed export or missing actual symbol binding")
		}
		owned = append(owned, nullableFactsExport{Name: exported.Name, Target: info.ID, SourcePath: file.FileName(),
			Range: offset(file, declaration), Definition: offset(file, declaration.Name())})
		if info.Signature.ReturnType != "i32" || info.Signature.ReturnPresence != nil || info.Signature.ReturnSignature != nil {
			return nil, nullableFactsError("export", "public nullable return ABI is unsupported")
		}
		for i, typ := range info.Signature.Parameters {
			var presence *parameterPresence
			if i < len(info.Signature.Presence) {
				presence = info.Signature.Presence[i]
			}
			if typ != "i32" || presence != nil && (presence.Null || presence.Undefined || presence.Optional || presence.Default) {
				return nil, nullableFactsError("export", "public parameters must be required scalar i32")
			}
		}
	}
	if len(owned) != 1 {
		return nil, nullableFactsError("export", "complete official export table must have exactly one entry")
	}
	slices.SortFunc(owned, func(a, b nullableFactsExport) int { return strings.Compare(a.Name, b.Name) })
	return owned, nil
}
