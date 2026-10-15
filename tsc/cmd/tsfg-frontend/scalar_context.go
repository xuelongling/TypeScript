// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Collects local scalar context and detects source forms outside the supported projection.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

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

func visitStatements(nodes []*ast.Node, visit func(*ast.Node) bool) bool {
	for _, node := range nodes {
		if visit(node) {
			return true
		}
		switch node.Kind {
		case ast.KindBlock:
			if visitStatements(node.AsBlock().Statements.Nodes, visit) {
				return true
			}
		case ast.KindIfStatement:
			branch := node.AsIfStatement()
			if visitStatements([]*ast.Node{branch.ThenStatement}, visit) ||
				(branch.ElseStatement != nil && visitStatements([]*ast.Node{branch.ElseStatement}, visit)) {
				return true
			}
		case ast.KindWhileStatement:
			if visitStatements([]*ast.Node{node.AsWhileStatement().Statement}, visit) {
				return true
			}
		}
	}
	return false
}

func operatorBindings(file *ast.SourceFile, check *checker.Checker, decl *ast.FunctionDeclaration) map[*ast.Symbol]symbol {
	bindings := parameterBindings(file, check, decl.Parameters.Nodes, nil)
	visitStatements(decl.Body.AsBlock().Statements.Nodes, func(item *ast.Node) bool {
		for _, local := range localDeclarations(item) {
			if local.Name() == nil || local.Name().Kind != ast.KindIdentifier {
				continue
			}
			t := scalar(check.GetTypeAtLocation(local.Name()))
			if t == "f64" && local.AsVariableDeclaration().Type == nil && local.AsVariableDeclaration().Initializer != nil {
				if _, _, ok := exactInteger(file, local.AsVariableDeclaration().Initializer); ok {
					t = "i32"
				}
			}
			bindings[check.GetSymbolAtLocation(local.Name())] = symbol{Type: t}
		}
		return false
	})
	return bindings
}

// Contextual diagnostic checks use the same owned scalar call signatures as
// expression projection. Only complete, fixed-arity declarations in this source
// participate; unrelated official diagnostics still run before owned projection.
func operatorFunctions(file *ast.SourceFile, check *checker.Checker) map[*ast.Symbol]functionInfo {
	functions := map[*ast.Symbol]functionInfo{}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Name() == nil || node.Name().Kind != ast.KindIdentifier || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			continue
		}
		decl := node.AsFunctionDeclaration()
		if decl.Type == nil || decl.TypeParameters != nil {
			continue
		}
		resolved := check.GetSymbolAtLocation(node.Name())
		if resolved == nil {
			continue
		}
		signatures := check.GetSignaturesOfType(check.GetTypeOfSymbol(resolved), checker.SignatureKindCall)
		if len(signatures) != 1 || signatures[0].Declaration() != node {
			continue
		}
		checked := signatures[0]
		returnType := scalar(check.GetReturnTypeOfSignature(checked))
		if returnType == "" || len(checked.Parameters()) != len(decl.Parameters.Nodes) {
			continue
		}
		parameters := make([]string, 0, len(checked.Parameters()))
		presence := make([]*parameterPresence, 0, len(checked.Parameters()))
		hasPresence := false
		for index, p := range decl.Parameters.Nodes {
			param := p.AsParameterDeclaration()
			if p.Name() == nil || p.Name().Kind != ast.KindIdentifier || param.Type == nil || param.DotDotDotToken != nil {
				break
			}
			t, shape, valid := checkedParameter(file, check, p)
			ownedType, _, _, validType := finitePresence(check.GetTypeOfSymbol(checked.Parameters()[index]))
			if !valid || !validType || ownedType != t {
				break
			}
			parameters = append(parameters, t)
			presence = append(presence, shape)
			hasPresence = hasPresence || shape != nil
		}
		if len(parameters) == len(decl.Parameters.Nodes) {
			if !hasPresence {
				presence = nil
			}
			functions[resolved] = functionInfo{Declaration: node, Signature: signature{Parameters: parameters, ReturnType: returnType, Presence: presence}}
		}
	}
	return functions
}

func supportedOperatorLocal(file *ast.SourceFile, check *checker.Checker, d *ast.Diagnostic, functions map[*ast.Symbol]functionInfo) bool {
	if d.File() != file || d.Code() != 2322 {
		return false
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			continue
		}
		decl := node.AsFunctionDeclaration()
		bindings := operatorBindings(file, check, decl)
		if visitStatements(decl.Body.AsBlock().Statements.Nodes, func(item *ast.Node) bool {
			for _, candidate := range localDeclarations(item) {
				if candidate.Kind != ast.KindVariableDeclaration || candidate.Name() == nil || candidate.Name().Kind != ast.KindIdentifier ||
					d.Pos() < candidate.Pos() || d.End() > candidate.End() {
					continue
				}
				local := candidate.AsVariableDeclaration()
				if local.Type == nil || scalar(check.GetTypeFromTypeNode(local.Type)) != "i32" || local.Initializer == nil {
					continue
				}
				projected, failure := projectOperator(file, check, local.Initializer, bindings, functions)
				return failure == nil && projected.Type == "i32"
			}
			return false
		}) {
			return true
		}
	}
	return false
}

func supportedOperatorAssignment(file *ast.SourceFile, check *checker.Checker, d *ast.Diagnostic, functions map[*ast.Symbol]functionInfo) bool {
	if d.File() != file || d.Code() != 2322 {
		return false
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			continue
		}
		decl := node.AsFunctionDeclaration()
		bindings := operatorBindings(file, check, decl)
		body := decl.Body.AsBlock().Statements.Nodes
		if visitStatements(body, func(item *ast.Node) bool {
			if item.Kind != ast.KindExpressionStatement {
				return false
			}
			expr := item.AsExpressionStatement().Expression
			if expr.Kind != ast.KindBinaryExpression || expr.AsBinaryExpression().OperatorToken.Kind != ast.KindEqualsToken {
				return false
			}
			binary := expr.AsBinaryExpression()
			if d.Pos() < item.Pos() || d.End() > item.End() ||
				scalar(check.GetTypeAtLocation(binary.Left)) != "i32" {
				return false
			}
			projected, failure := projectOperator(file, check, binary.Right, bindings, functions)
			return failure == nil && projected.Type == "i32"
		}) {
			return true
		}
	}
	return false
}

func pendingLocalForm(nodes []*ast.Node) *ast.Node {
	var walk func(*ast.Node) *ast.Node
	walk = func(node *ast.Node) *ast.Node {
		if node.Kind == ast.KindVariableDeclaration && node.AsVariableDeclaration().Initializer == nil && node.AsVariableDeclaration().Type == nil {
			return node
		}
		if node.Kind == ast.KindBinaryExpression && ast.IsCompoundAssignment(node.AsBinaryExpression().OperatorToken.Kind) && ordinaryAssignmentOperator(node.AsBinaryExpression().OperatorToken.Kind) == "" {
			return node
		}
		for child := range node.IterChildren() {
			if found := walk(child); found != nil {
				return found
			}
		}
		return nil
	}
	for _, node := range nodes {
		if found := walk(node); found != nil {
			return found
		}
	}
	return nil
}

func parameterBindings(file *ast.SourceFile, check *checker.Checker, parameters []*ast.Node, projected []symbol) map[*ast.Symbol]symbol {
	bindings := map[*ast.Symbol]symbol{}
	for i, parameter := range parameters {
		if parameter.Name() == nil {
			continue
		}
		resolved := check.GetSymbolAtLocation(parameter.Name())
		if resolved == nil {
			continue
		}
		payload, shape, _ := checkedParameter(file, check, parameter)
		binding := symbol{Type: payload, Writable: true, Presence: shape}
		if i < len(projected) {
			binding = projected[i]
		}
		bindings[resolved] = binding
	}
	return bindings
}

func prohibitedNode(file *ast.SourceFile) *ast.Node {
	var found *ast.Node
	var visit func(*ast.Node) bool
	visit = func(node *ast.Node) bool {
		if node.Kind == ast.KindAnyKeyword {
			found = node
			return true
		}
		if node.Kind == ast.KindBinaryExpression {
			binary := node.AsBinaryExpression()
			operator := binary.OperatorToken
			if operator.Kind == ast.KindEqualsEqualsToken || operator.Kind == ast.KindExclamationEqualsToken {
				found = operator
				return true
			}
			if operator.Kind == ast.KindEqualsToken {
				if binary.Left.Kind == ast.KindElementAccessExpression ||
					(binary.Left.Kind == ast.KindPropertyAccessExpression && binary.Left.AsPropertyAccessExpression().Name().Text() == "__proto__") {
					found = binary.Left
					return true
				}
			}
		}
		if node.Kind == ast.KindDeleteExpression {
			found = node
			return true
		}
		if node.Kind == ast.KindCallExpression {
			callee := node.AsCallExpression().Expression
			if (callee.Kind == ast.KindIdentifier && (callee.Text() == "eval" || callee.Text() == "Function")) ||
				(callee.Kind == ast.KindPropertyAccessExpression && callee.AsPropertyAccessExpression().Expression.Kind == ast.KindIdentifier &&
					callee.AsPropertyAccessExpression().Expression.Text() == "Object" && callee.AsPropertyAccessExpression().Name().Text() == "setPrototypeOf") {
				found = callee
				return true
			}
		}
		if node.Kind == ast.KindNewExpression {
			callee := node.AsNewExpression().Expression
			if callee.Kind == ast.KindIdentifier && (callee.Text() == "Proxy" || callee.Text() == "Function") {
				found = callee
				return true
			}
		}
		return node.ForEachChild(visit)
	}
	file.ForEachChild(visit)
	return found
}
