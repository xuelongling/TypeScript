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

func nullableFactsGrammar(file *ast.SourceFile, check *checker.Checker, functions map[*ast.Symbol]functionInfo, facts response, version nullableFactsVersion) error {
	if len(facts.Functions) == 0 || len(facts.Functions) > 512 || len(facts.Symbols) > 4096 || len(file.Statements.Nodes) != len(facts.Functions) {
		return version.failure("limit", "top-level function/symbol table")
	}
	for fnIndex, fn := range facts.Functions {
		fnNode := file.Statements.Nodes[fnIndex]
		if version == nullableFactsV4 && len(fn.Parameters) > 64 {
			return version.failure("limit", "required parameter count")
		}
		if fn.ReturnType != "i32" || fn.ReturnSignature != nil || fn.LexicalParent != "" || len(fn.Captures) != 0 ||
			fn.ReturnPresence != nil && !fn.ReturnPresence.Undefined && !fn.ReturnPresence.Null || len(fn.Parameters) > 64 {
			return version.at("capability", "only ordinary scalar/nullable i32 functions", file, fnNode)
		}
		for parameterIndex, parameter := range fn.Parameters {
			if parameter.Type != "i32" || parameter.Signature != nil || parameter.Presence == nil || !nullableFactsPresence(parameter.Presence) {
				return version.at("capability", "only required scalar/nullable i32 parameters", file, nullablePresenceParameterNode(fnNode, parameterIndex))
			}
		}
		for parameterIndex, initializer := range fn.ParameterInitializers {
			if initializer != nil {
				return version.at("capability", "default parameters are not represented", file, nullablePresenceParameterNode(fnNode, parameterIndex))
			}
		}
	}
	for _, symbol := range facts.Symbols {
		if symbol.Type != "i32" || symbol.Signature != nil || !nullableFactsPresence(symbol.Presence) ||
			symbol.Kind == "parameter" && symbol.Presence == nil ||
			(symbol.Kind != "function" && symbol.Kind != "parameter" && symbol.Kind != "local") || symbol.Kind == "local" && symbol.Writable {
			if version == nullableFactsV4 {
				return version.at("capability", "only ordinary scalar/nullable bindings and const locals", file,
					nullablePresenceSymbolDeclaration(file, check, facts, symbol))
			}
			return version.failure("capability", "only ordinary scalar/nullable bindings and const locals")
		}
	}
	var symbols map[string]symbol
	if version == nullableFactsV4 {
		symbols = make(map[string]symbol, len(facts.Symbols))
		for _, symbol := range facts.Symbols {
			symbols[symbol.ID] = symbol
		}
	}
	guards := 0
	for fnIndex, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Name() == nil || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			return version.at("capability", "only top-level named function bodies", file, node)
		}
		decl := node.AsFunctionDeclaration()
		if decl.TypeParameters != nil {
			return version.at("capability", "generic functions are not represented", file, node)
		}
		for _, parameter := range decl.Parameters.Nodes {
			p := parameter.AsParameterDeclaration()
			if p.QuestionToken != nil || p.Initializer != nil || p.DotDotDotToken != nil || p.Type == nil {
				return version.at("capability", "required explicitly typed parameters only", file, parameter)
			}
		}
		actual := check.GetSymbolAtLocation(node.Name())
		if actual == nil {
			return version.at("capability", "function requires its official symbol", file, node.Name())
		}
		signatures := check.GetSignaturesOfType(check.GetTypeOfSymbol(actual), checker.SignatureKindCall)
		if len(signatures) != 1 || signatures[0].Declaration() != node {
			return version.at("capability", "one complete ordinary function signature required", file, node)
		}
		returnMask, valid := nullableFactsMask(check.GetReturnTypeOfSignature(signatures[0]))
		if !valid {
			return version.at("capability", "required scalar/nullable i32 return signature", file, node)
		}
		body := node.Body().AsBlock().Statements.Nodes
		if len(body) == 0 || body[len(body)-1].Kind != ast.KindReturnStatement {
			return version.at("capability", "body must end in one return", file, node.Body())
		}
		guardCount := 0
		if version == nullableFactsV4 {
			guardCount = nullablePresenceGuardCount(body)
		}
		if guardCount != 0 {
			guards += guardCount
			if guards > 1024 {
				return version.failure("limit", "presence guard count")
			}
			if err := nullablePresenceFactsBody(file, check, node, facts.Functions[fnIndex], functions[actual], signatures[0], symbols, guardCount); err != nil {
				return err
			}
			continue
		}
		for i, statement := range body {
			switch statement.Kind {
			case ast.KindReturnStatement:
				if i != len(body)-1 {
					return version.at("capability", "return must terminate the body", file, statement)
				}
				mask, failed, err := nullableFactsExpression(check, statement.AsReturnStatement().Expression, 0)
				if err != nil {
					return version.expressionFailure(err, file, failed)
				}
				if !nullableFactsFits(mask, returnMask) {
					return version.at("capability", "return requires the exact declared absence mask", file, statement.AsReturnStatement().Expression)
				}
			case ast.KindVariableStatement:
				list := statement.AsVariableStatement().DeclarationList
				if list.Flags&ast.NodeFlagsConst == 0 {
					return version.at("capability", "only const local initialization", file, statement)
				}
				for _, local := range list.AsVariableDeclarationList().Declarations.Nodes {
					if local.Kind != ast.KindVariableDeclaration || local.Name() == nil || local.Name().Kind != ast.KindIdentifier || local.AsVariableDeclaration().Type == nil {
						return version.at("capability", "explicitly typed local declaration required", file, local)
					}
					expected, valid := nullableFactsMask(check.GetTypeFromTypeNode(local.AsVariableDeclaration().Type))
					mask, failed, err := nullableFactsExpression(check, local.AsVariableDeclaration().Initializer, 0)
					if err != nil {
						return version.expressionFailure(err, file, failed)
					}
					if !valid || !nullableFactsFits(mask, expected) {
						return version.at("capability", "initialization requires the exact declared absence mask", file, local)
					}
				}
			default:
				return version.at("capability", "only const initialization and return statements", file, statement)
			}
		}
	}
	return nil
}

// Failure sites stay callback-local; only their numeric source anchors escape.
func nullableFactsExpression(check *checker.Checker, node *ast.Node, depth int) (uint8, *ast.Node, error) {
	if depth > 128 {
		return 0, node, nullableFactsError("limit", "expression depth")
	}
	if node == nil {
		return 0, node, nullableFactsError("capability", "required value expression")
	}
	// The same fresh checker proves members at each use. Literal hints and
	// projected facts cannot replace type objects or erase a declared absence.
	useMask, valid := nullableFactsMask(check.GetTypeAtLocation(node))
	if !valid {
		return 0, node, nullableFactsError("capability", "only checked scalar/nullable i32 expressions")
	}
	switch node.Kind {
	case ast.KindBinaryExpression:
		binary := node.AsBinaryExpression()
		if binary.OperatorToken.Kind != ast.KindQuestionQuestionToken {
			return 0, node, nullableFactsError("capability", "only ordinary nullish coalesce operators")
		}
		left, failed, err := nullableFactsExpression(check, binary.Left, depth+1)
		if err != nil {
			return 0, failed, err
		}
		right, failed, err := nullableFactsExpression(check, binary.Right, depth+1)
		if err != nil {
			return 0, failed, err
		}
		if left == 0 || right != 0 || useMask != 0 {
			return 0, node, nullableFactsError("capability", "nullish coalesce requires nullable i32 lhs and scalar i32 rhs/result")
		}
		return 0, nil, nil
	case ast.KindIdentifier:
		actual := check.GetSymbolAtLocation(node)
		if actual == nil {
			return 0, node, nullableFactsError("capability", "reference requires its official binding")
		}
		declared, valid := nullableFactsMask(check.GetTypeOfSymbol(actual))
		if !valid || declared != useMask {
			return 0, node, nullableFactsError("capability", "reference must retain its complete declared absence mask")
		}
		return declared, nil, nil
	case ast.KindCallExpression:
		call := node.AsCallExpression()
		if call.Expression.Kind != ast.KindIdentifier || call.QuestionDotToken != nil || call.TypeArguments != nil {
			return 0, node, nullableFactsError("capability", "only direct ordinary calls")
		}
		resolved := check.GetResolvedSignature(node)
		if resolved == nil || resolved.Declaration() == nil || resolved.Declaration().Kind != ast.KindFunctionDeclaration {
			return 0, node, nullableFactsError("capability", "call requires its complete ordinary declaration")
		}
		parameters := resolved.Declaration().AsFunctionDeclaration().Parameters.Nodes
		if len(parameters) != len(call.Arguments.Nodes) {
			return 0, node, nullableFactsError("capability", "call requires all declared parameters")
		}
		for index, argument := range call.Arguments.Nodes {
			parameter := parameters[index].AsParameterDeclaration()
			if parameter.Type == nil {
				return 0, parameters[index], nullableFactsError("capability", "call requires explicitly typed parameters")
			}
			expected, valid := nullableFactsMask(check.GetTypeFromTypeNode(parameter.Type))
			mask, failed, err := nullableFactsExpression(check, argument, depth+1)
			if err != nil {
				return 0, failed, err
			}
			if !valid || !nullableFactsFits(mask, expected) {
				return 0, argument, nullableFactsError("capability", "call argument requires the exact declared absence mask")
			}
		}
		declared, valid := nullableFactsMask(check.GetReturnTypeOfSignature(resolved))
		if !valid || declared != useMask {
			return 0, node, nullableFactsError("capability", "call must retain its complete declared return mask")
		}
		return declared, nil, nil
	}
	return 0, node, nullableFactsError("capability", "only checked references and direct calls")
}

func projectNullableFacts(file *ast.SourceFile, check *checker.Checker, functions map[*ast.Symbol]functionInfo, facts response, version nullableFactsVersion) ([]nullableFactsExport, error) {
	if err := nullableFactsGrammar(file, check, functions, facts, version); err != nil {
		return nil, err
	}
	// A script has no module symbol. Never ask the checker for exports of nil.
	if !ast.IsExternalModule(file) || file.AsNode().Symbol() == nil {
		return nil, version.failure("export", "one direct scalar entry export is required")
	}
	topLevel := map[*ast.Node]bool{}
	for _, node := range file.Statements.Nodes {
		topLevel[node] = true
	}
	owned := make([]nullableFactsExport, 0)
	for _, exported := range check.GetExportsOfModule(file.AsNode().Symbol()) {
		if exported == nil || exported.Flags&ast.SymbolFlagsAlias != 0 || len(exported.Declarations) != 1 {
			return nil, version.failure("export", "aliases or ambiguous export declarations")
		}
		declaration := exported.Declarations[0]
		if !topLevel[declaration] || ast.GetSourceFileOfNode(declaration) != file || declaration.Kind != ast.KindFunctionDeclaration || declaration.Name() == nil || exported.Name != declaration.Name().Text() {
			return nil, version.failure("export", "foreign/nonfunction/renamed export declaration")
		}
		// Export and local symbols may differ; the actual declaration binds the
		// local checker symbol, never a same-spelled lookup or alias fallback.
		actual := check.GetSymbolAtLocation(declaration.Name())
		info, found := functions[actual]
		if actual == nil || !found || info.ID == "" || info.Declaration != declaration {
			return nil, version.failure("export", "foreign/nonfunction/renamed export or missing actual symbol binding")
		}
		owned = append(owned, nullableFactsExport{Name: exported.Name, Target: info.ID, SourcePath: file.FileName(),
			Range: offset(file, declaration), Definition: offset(file, declaration.Name())})
		if info.Signature.ReturnType != "i32" || info.Signature.ReturnPresence != nil || info.Signature.ReturnSignature != nil {
			return nil, version.at("export", "public nullable return ABI is unsupported", file, declaration)
		}
		for i, typ := range info.Signature.Parameters {
			var presence *parameterPresence
			if i < len(info.Signature.Presence) {
				presence = info.Signature.Presence[i]
			}
			if typ != "i32" || presence != nil && (presence.Null || presence.Undefined || presence.Optional || presence.Default) {
				return nil, version.at("export", "public parameters must be required scalar i32", file, nullablePresenceParameterNode(declaration, i))
			}
		}
	}
	if len(owned) != 1 {
		return nil, version.failure("export", "complete official export table must have exactly one entry")
	}
	slices.SortFunc(owned, func(a, b nullableFactsExport) int { return strings.Compare(a.Name, b.Name) })
	return owned, nil
}
