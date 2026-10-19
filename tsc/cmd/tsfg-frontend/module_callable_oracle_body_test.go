// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"github.com/microsoft/typescript-go/internal/ast"
)

// The recipe fixes the declaration list, parameter names, and complete body
// shape before any official AST is inspected. No producer projection is called.
func (o *mcIndependent23d) headers(recipes []mcFileRecipe23d) {
	for _, recipe := range recipes {
		file := o.program.GetSourceFile(recipe.Path)
		if file == nil {
			o.t.Fatal("independent recipe source missing")
		}
		o.files[recipe.Path] = file
		fi, gi := 0, 0
		for _, node := range file.Statements.Nodes {
			switch node.Kind {
			case ast.KindFunctionDeclaration:
				if fi >= len(recipe.Functions) {
					o.t.Fatal("unlisted original function")
				}
				expected := recipe.Functions[fi]
				fi++
				declaration := node.AsFunctionDeclaration()
				if node.Name() == nil || node.Name().Text() != expected.Name || declaration.Type == nil || declaration.Body == nil || declaration.Body.Kind != ast.KindBlock || declaration.TypeParameters != nil || len(declaration.Parameters.Nodes) != len(expected.Parameters) {
					o.t.Fatal("original function header differs from frozen recipe")
				}
				signature := o.signature(o.check.GetTypeAtLocation(node.Name()), 0)
				if !bytes.Equal(mcCanonical23d(o.t, signature), mcCanonical23d(o.t, expected.Signature)) {
					o.t.Fatalf("independent official signature differs for %s: %s != %s", expected.Name, mcCanonical23d(o.t, signature), mcCanonical23d(o.t, expected.Signature))
				}
				symbol := o.add(file, node, "function", signature["returnType"].(string), false)
				row := mcObject23d{"sourcePath": recipe.Path, "name": expected.Name, "symbol": symbol["id"], "parameters": []any{}, "returnType": signature["returnType"], "body": []any{}, "range": o.anchor(file, node, false)}
				if result, ok := signature["returnSignature"]; ok {
					row["returnSignature"] = result
				}
				o.functions = append(o.functions, &mcFunction23d{file: file, node: node, recipe: expected, row: row})
			case ast.KindVariableStatement:
				declarations := node.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
				if len(declarations) != 1 || gi >= len(recipe.Globals) {
					o.t.Fatal("independent global declaration recipe differs")
				}
				expected := recipe.Globals[gi]
				gi++
				declaration := declarations[0]
				if declaration.Name().Text() != expected.Name || expected.Type != "bool" || !o.check.GetTypeAtLocation(declaration.Name()).IsBooleanLike() {
					o.t.Fatal("official scalar global type/name differs")
				}
				writable := node.AsVariableStatement().DeclarationList.Flags&ast.NodeFlagsLet != 0
				if writable != expected.Writable {
					o.t.Fatal("official global write permission differs")
				}
				symbol := o.add(file, declaration, "global", "bool", writable)
				symbol["bindingKind"] = "source"
			case ast.KindImportDeclaration, ast.KindExportDeclaration:
			default:
				o.t.Fatal("unlisted original top-level statement")
			}
		}
		if fi != len(recipe.Functions) || gi != len(recipe.Globals) {
			o.t.Fatal("missing original header from complete recipe")
		}
	}
}
func (o *mcIndependent23d) functionBody(function *mcFunction23d) {
	declaration := function.node.AsFunctionDeclaration()
	parameters := []any{}
	annotations := []any{}
	for index, node := range declaration.Parameters.Nodes {
		parameter := node.AsParameterDeclaration()
		if node.Name().Text() != function.recipe.Parameters[index] || parameter.Type == nil || parameter.Initializer != nil || parameter.QuestionToken != nil || parameter.DotDotDotToken != nil {
			o.t.Fatal("independent required parameter differs")
		}
		typ, signature := o.scalarOrSignature(o.check.GetTypeAtLocation(node.Name()))
		row := o.add(function.file, node, "parameter", typ, true)
		if signature != nil {
			row["signature"] = signature
		} else {
			row["presence"] = o.requiredPresence(function.file, node)
		}
		parameters = append(parameters, row)
		annotations = append(annotations, mcObject23d{"symbol": row["id"], "declarationRange": o.anchor(function.file, node, false), "nameRange": o.anchor(function.file, node.Name(), false), "annotationRange": o.anchor(function.file, parameter.Type, true)})
	}
	function.row["parameters"] = parameters
	function.row["body"] = o.statements(function.file, declaration.Body.AsBlock().Statements.Nodes, function.recipe.Body)
	o.annotations = append(o.annotations, mcObject23d{"symbol": function.row["symbol"], "sourcePath": function.file.FileName(), "declarationRange": o.anchor(function.file, function.node, false), "nameRange": o.anchor(function.file, function.node.Name(), false), "returnAnnotationRange": o.anchor(function.file, declaration.Type, true), "parameters": annotations})
}
func (o *mcIndependent23d) requiredPresence(file *ast.SourceFile, node *ast.Node) mcObject23d {
	return mcObject23d{"undefined": false, "null": false, "optional": false, "default": false, "range": o.anchor(file, node, false), "annotation": o.anchor(file, node.AsParameterDeclaration().Type, true)}
}
func (o *mcIndependent23d) statements(file *ast.SourceFile, nodes []*ast.Node, recipes []*mcExpressionRecipe23d) []any {
	if len(nodes) != len(recipes) {
		o.t.Fatalf("complete independent body shape differs: %d nodes vs %d recipes", len(nodes), len(recipes))
	}
	result := []any{}
	for index, node := range nodes {
		recipe := recipes[index]
		row := mcObject23d{"range": o.anchor(file, node, false)}
		switch recipe.Kind {
		case "return":
			if node.Kind != ast.KindReturnStatement {
				o.t.Fatal("original return required")
			}
			row["kind"] = "return"
			row["expression"] = o.expression(file, node.AsReturnStatement().Expression, recipe.Value)
		case "local":
			if node.Kind != ast.KindVariableStatement {
				o.t.Fatal("original local required")
			}
			declarations := node.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
			if len(declarations) != 1 || declarations[0].Name().Text() != recipe.Name {
				o.t.Fatal("original single local name differs")
			}
			declaration := declarations[0]
			value := o.expression(file, declaration.AsVariableDeclaration().Initializer, recipe.Value)
			typ, signature := o.scalarOrSignature(o.check.GetTypeAtLocation(declaration.Name()))
			row["symbol"] = o.add(file, declaration, "local", typ, node.AsVariableStatement().DeclarationList.Flags&ast.NodeFlagsLet != 0)
			if signature != nil {
				row["symbol"].(mcObject23d)["signature"] = signature
			}
			row["kind"] = "variable"
			row["initializer"] = value
			row["range"] = o.anchor(file, declaration, false)
		case "assign":
			if node.Kind != ast.KindExpressionStatement {
				o.t.Fatal("original assignment statement required")
			}
			expression := node.AsExpressionStatement().Expression
			if expression.Kind != ast.KindBinaryExpression || expression.AsBinaryExpression().OperatorToken.Kind != ast.KindEqualsToken {
				o.t.Fatal("original plain assignment required")
			}
			binary := expression.AsBinaryExpression()
			row["kind"] = "assignment"
			row["target"] = o.expression(file, binary.Left, &mcExpressionRecipe23d{Kind: "reference", Name: recipe.Name, Target: recipe.Target})
			row["value"] = o.expression(file, binary.Right, recipe.Value)
		case "block":
			if node.Kind != ast.KindBlock {
				o.t.Fatal("original block required")
			}
			row["kind"] = "block"
			row["body"] = o.statements(file, node.AsBlock().Statements.Nodes, recipe.Body)
		case "if":
			if node.Kind != ast.KindIfStatement {
				o.t.Fatal("original branch required")
			}
			branch := node.AsIfStatement()
			row["kind"] = "if"
			row["range"] = o.anchor(file, node, true)
			row["condition"] = o.expression(file, branch.Expression, recipe.Condition)
			row["thenBody"] = o.statements(file, []*ast.Node{branch.ThenStatement}, recipe.Then)
			if branch.ElseStatement != nil {
				row["elseBody"] = o.statements(file, []*ast.Node{branch.ElseStatement}, recipe.Else)
			} else if len(recipe.Else) != 0 {
				o.t.Fatal("original else branch missing")
			}
		default:
			o.t.Fatalf("unknown independent statement recipe %s", recipe.Kind)
		}
		result = append(result, row)
	}
	return result
}
func (o *mcIndependent23d) expression(file *ast.SourceFile, node *ast.Node, recipe *mcExpressionRecipe23d) mcObject23d {
	if node == nil || recipe == nil {
		o.t.Fatal("independent complete expression missing")
	}
	typ, signature := o.scalarOrSignature(o.check.GetTypeAtLocation(node))
	row := mcObject23d{"kind": recipe.Kind, "type": typ, "range": o.anchor(file, node, false)}
	switch recipe.Kind {
	case "boolean":
		if recipe.Boolean == nil || node.Kind != ast.KindTrueKeyword && node.Kind != ast.KindFalseKeyword || *recipe.Boolean != (node.Kind == ast.KindTrueKeyword) {
			o.t.Fatal("original boolean literal differs")
		}
		row["boolean"] = *recipe.Boolean
	case "reference", "functionReference":
		if node.Kind != ast.KindIdentifier || o.spelling(file, node) != recipe.Name {
			o.t.Fatal("original named reference differs")
		}
		binding := o.lookup(node, recipe.Target)
		row["name"] = recipe.Name
		if recipe.Kind == "functionReference" {
			if binding["kind"] != "function" {
				o.t.Fatal("original named function target differs")
			}
			row["target"] = binding["id"]
			row["signature"] = signature
		} else {
			if binding["kind"] == "function" {
				o.t.Fatal("function identity used as lexical/global binding")
			}
			row["symbol"] = binding["id"]
			if signature != nil {
				row["signature"] = signature
			}
		}
	case "call", "indirectCall":
		if node.Kind != ast.KindCallExpression {
			o.t.Fatal("original call required")
		}
		call := node.AsCallExpression()
		if call.QuestionDotToken != nil || call.TypeArguments != nil || o.spelling(file, call.Expression) != recipe.Name || len(call.Arguments.Nodes) != len(recipe.Args) {
			o.t.Fatal("original callee/argument list differs")
		}
		resolved := o.check.GetResolvedSignature(node)
		callSignature := o.signatureValue(resolved, 0)
		row["name"] = recipe.Name
		row["signature"] = callSignature
		if recipe.Kind == "call" {
			binding := o.lookup(call.Expression, recipe.Target)
			if binding["kind"] != "function" {
				o.t.Fatal("direct call does not name original function")
			}
			row["target"] = binding["id"]
			if _, nested := callSignature["parameterSignatures"]; !nested && callSignature["returnType"] != "function" && len(resolved.Parameters()) > 0 {
				declaration := resolved.Declaration().AsFunctionDeclaration()
				presence := []any{}
				for _, parameter := range declaration.Parameters.Nodes {
					presence = append(presence, o.requiredPresence(ast.GetSourceFileOfNode(parameter), parameter))
				}
				callSignature["presence"] = presence
			}
		} else {
			row["callee"] = o.expression(file, call.Expression, recipe.Callee)
		}
		arguments := []any{}
		for index, argument := range call.Arguments.Nodes {
			arguments = append(arguments, o.expression(file, argument, recipe.Args[index]))
		}
		row["arguments"] = arguments
	default:
		o.t.Fatalf("unknown independent expression recipe %s", recipe.Kind)
	}
	return row
}
