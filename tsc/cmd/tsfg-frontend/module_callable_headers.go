// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
)

// Only this private request profile reaches callable module headers. Its source
// identities remain snapshot-local; neither structural types nor names select
// indirect runtime targets here.
func moduleCallableHeader(file *ast.SourceFile, check *checker.Checker, node *ast.Node, result *response) (functionInfo, response) {
	bad := func(at *ast.Node) (functionInfo, response) {
		return functionInfo{}, failAt("r02b-d-unsupported", "unsupported", file, at)
	}
	declaration := node.AsFunctionDeclaration()
	actual := check.GetSymbolAtLocation(node.Name())
	if actual == nil {
		return bad(node)
	}
	signatures := check.GetSignaturesOfType(check.GetTypeOfSymbol(actual), checker.SignatureKindCall)
	if len(signatures) != 1 || signatures[0].Declaration() != node {
		return bad(node)
	}
	owned, ok := functionValueSignature(check, signatures[0], map[*checker.Type]bool{})
	if !ok {
		return bad(node)
	}
	presence := make([]*parameterPresence, len(declaration.Parameters.Nodes))
	hasPresence := false
	for i, parameter := range declaration.Parameters.Nodes {
		if check.GetSymbolAtLocation(parameter.Name()) != signatures[0].Parameters()[i] {
			return bad(parameter)
		}
		if owned.Parameters[i] != "function" {
			scalarType, shape, valid := checkedParameter(file, check, parameter)
			if !valid || scalarType != owned.Parameters[i] {
				return bad(parameter)
			}
			presence[i] = shape
			hasPresence = true
		}
	}
	if hasPresence {
		owned.Presence = presence
	}
	binding := symbol{SourcePath: file.FileName(), ID: fmt.Sprintf("s%d", len(result.Symbols)+1), Name: node.Name().Text(), Kind: "function", Type: owned.ReturnType, Range: offset(file, node.Name())}
	result.Symbols = append(result.Symbols, binding)
	return functionInfo{ID: binding.ID, Declaration: node, Signature: *owned, ModuleCallable: true}, response{}
}

// This proof borrows original checker objects only until projectModules returns.
// Its owned signature is structural data; no alias target enters the DTO early.
type moduleCallableAliasProof struct {
	declaration       *ast.Node
	target            *ast.Symbol
	targetDeclaration *ast.Node
	ownedSignature    *signature
}

func moduleCallableAliasSignature(check *checker.Checker, resolved *checker.Signature, target *ast.Node) (*signature, bool) {
	if target == nil || target.Kind != ast.KindFunctionDeclaration {
		return nil, false
	}
	declaration := target.AsFunctionDeclaration()
	if declaration.TypeParameters != nil || declaration.Type == nil || len(declaration.Parameters.Nodes) != 1 {
		return nil, false
	}
	parameter := declaration.Parameters.Nodes[0]
	value := parameter.AsParameterDeclaration()
	if parameter.Name() == nil || parameter.Name().Kind != ast.KindIdentifier || value.Type == nil || value.Initializer != nil || value.QuestionToken != nil || value.DotDotDotToken != nil {
		return nil, false
	}
	kind, presence, valid := checkedParameter(ast.GetSourceFileOfNode(target), check, parameter)
	if !valid || (kind != "bool" && kind != "i32") || scalar(check.GetTypeFromTypeNode(declaration.Type)) != kind {
		return nil, false
	}
	if presence != nil && (presence.Undefined || presence.Null || presence.Optional || presence.Default) {
		return nil, false
	}
	if resolved == nil || resolved.Declaration() != target || len(resolved.Parameters()) != 1 || resolved.Parameters()[0] != check.GetSymbolAtLocation(parameter.Name()) {
		return nil, false
	}
	owned, ok := functionValueSignature(check, resolved, map[*checker.Type]bool{})
	if !ok || len(owned.Parameters) != 1 || owned.Parameters[0] != kind || owned.ReturnType != kind || signatureHasCallable(*owned) {
		return nil, false
	}
	return owned, true
}

func moduleCallableAlias(check *checker.Checker, file *ast.SourceFile, statement, node *ast.Node) (moduleCallableAliasProof, bool) {
	bad := moduleCallableAliasProof{}
	declaration := node.AsVariableDeclaration()
	if statement.AsVariableStatement().DeclarationList.Flags&ast.NodeFlagsConst == 0 || len(localDeclarations(statement)) != 1 || ast.HasSyntacticModifier(statement, ast.ModifierFlagsExport) || declaration.Type != nil || declaration.Initializer == nil || declaration.Initializer.Kind != ast.KindIdentifier {
		return bad, false
	}
	original := check.GetSymbolAtLocation(node.Name())
	target := check.GetSymbolAtLocation(declaration.Initializer)
	if original == nil || target == nil || original == target || original.Flags&ast.SymbolFlagsAlias != 0 || target.Flags&ast.SymbolFlagsAlias != 0 || len(original.Declarations) != 1 || original.Declarations[0] != node || len(target.Declarations) != 1 {
		return bad, false
	}
	function := target.Declarations[0]
	if function.Kind != ast.KindFunctionDeclaration || function.Parent != file.AsNode() || function.Name() == nil || function.Name().Kind != ast.KindIdentifier || check.GetSymbolAtLocation(function.Name()) != target || function.Body() == nil || function.Body().Kind != ast.KindBlock {
		return bad, false
	}
	var owned *signature
	for _, site := range []*ast.Node{node.Name(), declaration.Initializer, function.Name()} {
		signatures := check.GetSignaturesOfType(check.GetTypeAtLocation(site), checker.SignatureKindCall)
		if len(signatures) != 1 {
			return bad, false
		}
		var valid bool
		owned, valid = moduleCallableAliasSignature(check, signatures[0], function)
		if !valid {
			return bad, false
		}
	}
	return moduleCallableAliasProof{declaration: node, target: target, targetDeclaration: function, ownedSignature: owned}, true
}

func moduleCallableAliasUses(program *compiler.Program, check *checker.Checker, original *ast.Symbol, proof moduleCallableAliasProof, globals map[*ast.Symbol]symbol) response {
	file := ast.GetSourceFileOfNode(proof.declaration)
	bad := func(node *ast.Node) response { return failAt("r02b-d-unsupported", "unsupported", file, node) }
	if file == nil || !ast.IsExternalModule(file) {
		return bad(proof.declaration)
	}
	for _, source := range program.GetSourceFiles() {
		if !strings.HasSuffix(source.FileName(), ".d.ts") && source != file {
			return bad(proof.declaration)
		}
	}
	functions := map[*ast.Symbol]*ast.Node{}
	for _, node := range file.Statements.Nodes {
		switch node.Kind {
		case ast.KindFunctionDeclaration:
			actual := check.GetSymbolAtLocation(node.Name())
			if actual == nil || actual.Flags&ast.SymbolFlagsAlias != 0 {
				return bad(node)
			}
			signatures := check.GetSignaturesOfType(check.GetTypeOfSymbol(actual), checker.SignatureKindCall)
			if len(signatures) != 1 || signatures[0].Declaration() != node {
				return bad(node)
			}
			owned, valid := functionValueSignature(check, signatures[0], map[*checker.Type]bool{})
			if !valid || signatureHasCallable(*owned) || (owned.ReturnType != "bool" && owned.ReturnType != "i32") {
				return bad(node)
			}
			for _, kind := range owned.Parameters {
				if kind != "bool" && kind != "i32" {
					return bad(node)
				}
			}
			functions[actual] = node
		case ast.KindVariableStatement:
			for _, declaration := range localDeclarations(node) {
				if declaration == proof.declaration {
					continue
				}
				initializer := declaration.AsVariableDeclaration().Initializer
				kind := globals[check.GetSymbolAtLocation(declaration.Name())].Type
				if kind == "bool" && initializer != nil && (initializer.Kind == ast.KindTrueKeyword || initializer.Kind == ast.KindFalseKeyword) {
					continue
				}
				if kind == "i32" {
					if value, _, valid := exactInteger(file, initializer); valid && i32InRange(value) {
						continue
					}
				}
				return bad(declaration)
			}
		default:
			// No initializer can observe this binding before module initialization.
			return bad(node)
		}
	}
	type entry struct {
		node  *ast.Node
		owner *ast.Symbol
	}
	stack := []entry{{node: file.AsNode()}}
	edges := map[*ast.Symbol][]*ast.Symbol{}
	aliasCalls := map[*ast.Symbol]bool{}
	count, calls := 0, 0
	for len(stack) > 0 {
		item := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		node := item.node
		if ast.IsStatement(node) || ast.IsExpression(node) {
			count++
		}
		if count > 16384 {
			return failAt("r03-module-callable-limit", "input", file, node)
		}
		switch node.Kind {
		case ast.KindFunctionDeclaration:
			if node.Parent != file.AsNode() {
				return bad(node)
			}
			item.owner = check.GetSymbolAtLocation(node.Name())
		case ast.KindFunctionExpression, ast.KindArrowFunction, ast.KindFunctionType, ast.KindShorthandPropertyAssignment:
			return bad(node)
		case ast.KindVariableDeclaration:
			if node != proof.declaration {
				if signatures := check.GetSignaturesOfType(check.GetTypeAtLocation(node.Name()), checker.SignatureKindCall); len(signatures) != 0 {
					return bad(node)
				}
			}
		case ast.KindCallExpression:
			calls++
			if calls > 16384 {
				return failAt("r03-module-callable-limit", "input", file, node)
			}
			call := node.AsCallExpression()
			if item.owner == nil || call.Expression.Kind != ast.KindIdentifier || call.QuestionDotToken != nil || call.TypeArguments != nil {
				return bad(node)
			}
			target := check.GetSymbolAtLocation(call.Expression)
			if target == original {
				aliasCalls[item.owner] = true
			} else {
				function := functions[target]
				resolved := check.GetResolvedSignature(node)
				if function == nil || resolved == nil || resolved.Declaration() != function {
					return bad(node)
				}
				edges[item.owner] = append(edges[item.owner], target)
			}
		case ast.KindIdentifier:
			actual := check.GetSymbolAtLocation(node)
			if actual == original && node != proof.declaration.Name() {
				parent := node.Parent
				if parent == nil || parent.Kind != ast.KindCallExpression || parent.AsCallExpression().Expression != node || len(parent.AsCallExpression().Arguments.Nodes) != 1 {
					return bad(node)
				}
				if _, valid := moduleCallableAliasSignature(check, check.GetResolvedSignature(parent), proof.targetDeclaration); !valid {
					return bad(node)
				}
				signatures := check.GetSignaturesOfType(check.GetTypeAtLocation(node), checker.SignatureKindCall)
				if len(signatures) != 1 {
					return bad(node)
				}
				if _, valid := moduleCallableAliasSignature(check, signatures[0], proof.targetDeclaration); !valid {
					return bad(node)
				}
			} else if function := functions[actual]; function != nil && node != function.Name() && node != proof.declaration.AsVariableDeclaration().Initializer {
				if node.Parent == nil || node.Parent.Kind != ast.KindCallExpression || node.Parent.AsCallExpression().Expression != node {
					return bad(node)
				}
			}
		}
		// Preserve source order when choosing the first invalid original use.
		children := []*ast.Node{}
		node.ForEachChild(func(child *ast.Node) bool { children = append(children, child); return false })
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, entry{node: children[i], owner: item.owner})
		}
	}
	// Follow original direct-call edges, not names. Ordinary SCCs remain valid;
	// any reachable use of this alias would close a self or mutual alias cycle.
	pending := []*ast.Symbol{proof.target}
	seen := map[*ast.Symbol]bool{}
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[current] {
			continue
		}
		seen[current] = true
		if len(seen) > 128 {
			return failAt("r03-module-callable-limit", "input", file, proof.declaration)
		}
		if aliasCalls[current] {
			return bad(proof.declaration)
		}
		pending = append(pending, edges[current]...)
	}
	return response{}
}

func moduleCallableProfile(program *compiler.Program, check *checker.Checker) (map[*ast.Symbol]moduleCallableAliasProof, response) {
	// Reuse the module boundary's default integer classification. The official
	// number-like type alone does not encode R01's unannotated i32 literal rule.
	globals := moduleGlobals(program, check)
	aliases := map[*ast.Symbol]moduleCallableAliasProof{}
	for _, file := range program.GetSourceFiles() {
		if strings.HasSuffix(file.FileName(), ".d.ts") {
			continue
		}
		bad := func(node *ast.Node) response { return failAt("r02b-d-unsupported", "unsupported", file, node) }
		for _, node := range file.Statements.Nodes {
			if ast.HasSyntacticModifier(node, ast.ModifierFlagsDefault|ast.ModifierFlagsAmbient|ast.ModifierFlagsAsync|ast.ModifierFlagsDecorator) {
				return nil, bad(node)
			}
			switch node.Kind {
			case ast.KindFunctionDeclaration:
				declaration := node.AsFunctionDeclaration()
				if node.Name() == nil || node.Name().Kind != ast.KindIdentifier || declaration.Body == nil || declaration.Body.Kind != ast.KindBlock || declaration.Type == nil || declaration.TypeParameters != nil {
					return nil, bad(node)
				}
			case ast.KindVariableStatement:
				if node.AsVariableStatement().DeclarationList.Flags&(ast.NodeFlagsLet|ast.NodeFlagsConst) == 0 {
					return nil, bad(node)
				}
				for _, declaration := range localDeclarations(node) {
					if declaration.Name() == nil || declaration.Name().Kind != ast.KindIdentifier || declaration.AsVariableDeclaration().Initializer == nil {
						return nil, bad(declaration)
					}
					original := check.GetSymbolAtLocation(declaration.Name())
					typ := globals[original].Type
					if typ != "bool" && typ != "i32" {
						proof, valid := moduleCallableAlias(check, file, node, declaration)
						if !valid || len(aliases) != 0 {
							return nil, bad(declaration)
						}
						aliases[original] = proof
					}
				}
			case ast.KindImportDeclaration:
				clause := node.AsImportDeclaration().ImportClause
				if clause != nil && ast.IsTypeOnlyImportDeclaration(clause) {
					return nil, bad(node)
				}
				if clause != nil && clause.AsImportClause().NamedBindings != nil && clause.AsImportClause().NamedBindings.Kind == ast.KindNamedImports {
					for _, element := range clause.AsImportClause().NamedBindings.AsNamedImports().Elements.Nodes {
						if element.AsImportSpecifier().IsTypeOnly {
							return nil, bad(element)
						}
					}
				}
			case ast.KindExportDeclaration:
				declaration := node.AsExportDeclaration()
				if declaration.IsTypeOnly || declaration.ExportClause != nil && declaration.ExportClause.Kind != ast.KindNamedExports {
					return nil, bad(node)
				}
				if declaration.ExportClause != nil {
					for _, element := range declaration.ExportClause.AsNamedExports().Elements.Nodes {
						if element.AsExportSpecifier().IsTypeOnly {
							return nil, bad(element)
						}
					}
				}
			case ast.KindExpressionStatement, ast.KindBlock, ast.KindIfStatement, ast.KindWhileStatement:
				// The existing module initializer owns these ordinary statements.
			default:
				return nil, bad(node)
			}
		}
	}
	for original, proof := range aliases {
		if failure := moduleCallableAliasUses(program, check, original, proof, globals); failure.Status != "" {
			return nil, failure
		}
	}
	return aliases, response{}
}
