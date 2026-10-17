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
	return shape == nil || !shape.Optional && !shape.Default
}

func nullableFactsMask(typ *checker.Type) (uint8, bool) {
	payload, undefined, null, valid := finitePresence(typ)
	var mask uint8
	if undefined {
		mask |= 1
	}
	if null {
		mask |= 2
	}
	return mask, valid && payload == "i32"
}

func nullableFactsFits(actual, expected uint8) bool {
	// Scalar lifting has an explicit consumer wrap. Nullable masks never
	// widen implicitly, even when the official assignment relation allows it.
	return actual == 0 || actual == expected
}

func nullableFactsGrammar(file *ast.SourceFile, check *checker.Checker, facts response) error {
	if len(facts.Functions) == 0 || len(facts.Functions) > 512 || len(facts.Symbols) > 4096 || len(file.Statements.Nodes) != len(facts.Functions) {
		return nullableFactsError("limit", "top-level function/symbol table")
	}
	for _, fn := range facts.Functions {
		if fn.ReturnType != "i32" || fn.ReturnSignature != nil || fn.LexicalParent != "" || len(fn.Captures) != 0 ||
			fn.ReturnPresence != nil && !fn.ReturnPresence.Undefined && !fn.ReturnPresence.Null || len(fn.Parameters) > 64 {
			return nullableFactsError("capability", "only ordinary scalar/nullable i32 functions")
		}
		for _, parameter := range fn.Parameters {
			if parameter.Type != "i32" || parameter.Signature != nil || parameter.Presence == nil || !nullableFactsPresence(parameter.Presence) {
				return nullableFactsError("capability", "only required scalar/nullable i32 parameters")
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
			symbol.Kind == "parameter" && symbol.Presence == nil ||
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
		actual := check.GetSymbolAtLocation(node.Name())
		if actual == nil {
			return nullableFactsError("capability", "function requires its official symbol")
		}
		signatures := check.GetSignaturesOfType(check.GetTypeOfSymbol(actual), checker.SignatureKindCall)
		if len(signatures) != 1 || signatures[0].Declaration() != node {
			return nullableFactsError("capability", "one complete ordinary function signature required")
		}
		returnMask, valid := nullableFactsMask(check.GetReturnTypeOfSignature(signatures[0]))
		if !valid {
			return nullableFactsError("capability", "required scalar/nullable i32 return signature")
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
				mask, err := nullableFactsExpression(check, statement.AsReturnStatement().Expression, 0)
				if err != nil {
					return err
				}
				if !nullableFactsFits(mask, returnMask) {
					return nullableFactsError("capability", "return requires the exact declared absence mask")
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
					expected, valid := nullableFactsMask(check.GetTypeFromTypeNode(local.AsVariableDeclaration().Type))
					mask, err := nullableFactsExpression(check, local.AsVariableDeclaration().Initializer, 0)
					if err != nil {
						return err
					}
					if !valid || !nullableFactsFits(mask, expected) {
						return nullableFactsError("capability", "initialization requires the exact declared absence mask")
					}
				}
			default:
				return nullableFactsError("capability", "only const initialization and return statements")
			}
		}
	}
	return nil
}

func nullableFactsExpression(check *checker.Checker, node *ast.Node, depth int) (uint8, error) {
	if depth > 128 {
		return 0, nullableFactsError("limit", "expression depth")
	}
	if node == nil {
		return 0, nullableFactsError("capability", "required value expression")
	}
	// The same fresh checker proves members at each use. Literal hints and
	// projected facts cannot replace type objects or erase a declared absence.
	useMask, valid := nullableFactsMask(check.GetTypeAtLocation(node))
	if !valid {
		return 0, nullableFactsError("capability", "only checked scalar/nullable i32 expressions")
	}
	switch node.Kind {
	case ast.KindBinaryExpression:
		binary := node.AsBinaryExpression()
		if binary.OperatorToken.Kind != ast.KindQuestionQuestionToken {
			return 0, nullableFactsError("capability", "only ordinary nullish coalesce operators")
		}
		left, err := nullableFactsExpression(check, binary.Left, depth+1)
		if err != nil {
			return 0, err
		}
		right, err := nullableFactsExpression(check, binary.Right, depth+1)
		if err != nil {
			return 0, err
		}
		if left == 0 || right != 0 || useMask != 0 {
			return 0, nullableFactsError("capability", "nullish coalesce requires nullable i32 lhs and scalar i32 rhs/result")
		}
		return 0, nil
	case ast.KindIdentifier:
		actual := check.GetSymbolAtLocation(node)
		if actual == nil {
			return 0, nullableFactsError("capability", "reference requires its official binding")
		}
		declared, valid := nullableFactsMask(check.GetTypeOfSymbol(actual))
		if !valid || declared != useMask {
			return 0, nullableFactsError("capability", "reference must retain its complete declared absence mask")
		}
		return declared, nil
	case ast.KindCallExpression:
		call := node.AsCallExpression()
		if call.Expression.Kind != ast.KindIdentifier || call.QuestionDotToken != nil || call.TypeArguments != nil {
			return 0, nullableFactsError("capability", "only direct ordinary calls")
		}
		resolved := check.GetResolvedSignature(node)
		if resolved == nil || resolved.Declaration() == nil || resolved.Declaration().Kind != ast.KindFunctionDeclaration {
			return 0, nullableFactsError("capability", "call requires its complete ordinary declaration")
		}
		parameters := resolved.Declaration().AsFunctionDeclaration().Parameters.Nodes
		if len(parameters) != len(call.Arguments.Nodes) {
			return 0, nullableFactsError("capability", "call requires all declared parameters")
		}
		for index, argument := range call.Arguments.Nodes {
			parameter := parameters[index].AsParameterDeclaration()
			if parameter.Type == nil {
				return 0, nullableFactsError("capability", "call requires explicitly typed parameters")
			}
			expected, valid := nullableFactsMask(check.GetTypeFromTypeNode(parameter.Type))
			mask, err := nullableFactsExpression(check, argument, depth+1)
			if err != nil {
				return 0, err
			}
			if !valid || !nullableFactsFits(mask, expected) {
				return 0, nullableFactsError("capability", "call argument requires the exact declared absence mask")
			}
		}
		declared, valid := nullableFactsMask(check.GetReturnTypeOfSignature(resolved))
		if !valid || declared != useMask {
			return 0, nullableFactsError("capability", "call must retain its complete declared return mask")
		}
		return declared, nil
	}
	return 0, nullableFactsError("capability", "only checked references and direct calls")
}

func projectNullableFacts(file *ast.SourceFile, check *checker.Checker, functions map[*ast.Symbol]functionInfo, facts response) ([]nullableFactsExport, error) {
	if err := nullableFactsGrammar(file, check, facts); err != nil {
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
