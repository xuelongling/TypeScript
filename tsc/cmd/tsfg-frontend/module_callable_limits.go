// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
)

// Bound the complete official AST before recursive owned projection, including
// unentered functions and static dependencies. DTS bytes are independently pinned.
func moduleCallablePreflight(program *compiler.Program) response {
	count, functions := 0, 0
	type entry struct {
		node  *ast.Node
		depth int
	}
	for _, file := range program.GetSourceFiles() {
		if strings.HasSuffix(file.FileName(), ".d.ts") {
			continue
		}
		stack := []entry{{file.AsNode(), 0}}
		for len(stack) > 0 {
			item := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			node := item.node
			if ast.IsStatement(node) || ast.IsExpression(node) {
				count++
			}
			if node.Kind == ast.KindFunctionDeclaration {
				functions++
				if len(node.AsFunctionDeclaration().Parameters.Nodes) > 32 {
					return failAt("r03-module-callable-limit", "input", file, node)
				}
			}
			if count > 16384 || functions > 128 || item.depth > 128 {
				return failAt("r03-module-callable-limit", "input", file, node)
			}
			node.ForEachChild(func(child *ast.Node) bool { stack = append(stack, entry{child, item.depth + 1}); return false })
		}
	}
	return response{}
}

func moduleCallableType(node *ast.Node, depth int) bool {
	if node == nil {
		return false
	}
	switch node.Kind {
	case ast.KindParenthesizedType:
		return moduleCallableType(node.AsParenthesizedTypeNode().Type, depth)
	case ast.KindTypeReference:
		reference := node.AsTypeReferenceNode()
		return reference.TypeArguments == nil && reference.TypeName.Kind == ast.KindIdentifier && (reference.TypeName.Text() == "bool" || reference.TypeName.Text() == "i32")
	case ast.KindFunctionType:
		if depth > 16 {
			return false
		}
		signature := node.AsFunctionTypeNode()
		if signature.TypeParameters != nil || len(signature.Parameters.Nodes) > 32 {
			return false
		}
		for _, parameter := range signature.Parameters.Nodes {
			declaration := parameter.AsParameterDeclaration()
			if parameter.Name() == nil || parameter.Name().Kind != ast.KindIdentifier || declaration.QuestionToken != nil || declaration.Initializer != nil || declaration.DotDotDotToken != nil || !moduleCallableType(declaration.Type, depth+1) {
				return false
			}
		}
		return moduleCallableType(signature.Type, depth+1)
	}
	return false
}

func moduleCallableDeclaredProfile(file *ast.SourceFile) response {
	bad := func(node *ast.Node) response { return failAt("r02b-d-unsupported", "unsupported", file, node) }
	stack := []*ast.Node{file.AsNode()}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch node.Kind {
		case ast.KindFunctionDeclaration:
			declaration := node.AsFunctionDeclaration()
			if node.Parent != file.AsNode() || declaration.Type == nil || !moduleCallableType(declaration.Type, 2) {
				return bad(node)
			}
			for _, parameter := range declaration.Parameters.Nodes {
				value := parameter.AsParameterDeclaration()
				if parameter.Name() == nil || parameter.Name().Kind != ast.KindIdentifier || value.QuestionToken != nil || value.Initializer != nil || value.DotDotDotToken != nil || !moduleCallableType(value.Type, 2) {
					return bad(parameter)
				}
			}
		case ast.KindVariableDeclaration:
			declaration := node.AsVariableDeclaration()
			if node.Name() == nil || node.Name().Kind != ast.KindIdentifier || declaration.Initializer == nil || declaration.Type != nil && !moduleCallableType(declaration.Type, 1) {
				return bad(node)
			}
		}
		node.ForEachChild(func(child *ast.Node) bool { stack = append(stack, child); return false })
	}
	return response{}
}

func moduleCallableFactsBounds(facts response) error {
	if len(facts.Functions) > 128 || len(facts.Modules) > 64 || len(facts.Symbols) > 4096 {
		return moduleCallableError("limit")
	}
	count, calls := 0, 0
	validType := func(name string) bool { return name == "bool" || name == "i32" || name == "function" }
	var signatureFits func(*signature, int) bool
	signatureFits = func(value *signature, depth int) bool {
		if value == nil {
			return true
		}
		if depth > 16 || len(value.Parameters) > 32 {
			return false
		}
		if !validType(value.ReturnType) {
			return false
		}
		for _, name := range value.Parameters {
			if !validType(name) {
				return false
			}
		}
		for _, child := range value.ParameterSignatures {
			if !signatureFits(child, depth+1) {
				return false
			}
		}
		return signatureFits(value.ReturnSignature, depth+1)
	}
	for _, binding := range facts.Symbols {
		if !validType(binding.Type) || !signatureFits(binding.Signature, 1) {
			return moduleCallableError("source")
		}
	}
	var expressionFits func(*expression) bool
	expressionFits = func(value *expression) bool {
		if value == nil {
			return true
		}
		count++
		if !validType(value.Type) || !signatureFits(value.Signature, 1) {
			return false
		}
		switch value.Kind {
		case "integer", "boolean", "reference", "functionReference", "call", "indirectCall", "group", "unary", "binary", "shortCircuit":
		default:
			return false
		}
		if value.Kind == "call" || value.Kind == "indirectCall" {
			calls++
		}
		for _, child := range []*expression{value.Callee, value.Left, value.Right, value.Operand} {
			if !expressionFits(child) {
				return false
			}
		}
		if value.Arguments != nil {
			for i := range *value.Arguments {
				if !expressionFits(&(*value.Arguments)[i]) {
					return false
				}
			}
		}
		return true
	}
	valid := true
	checkBody := func(body []statement) {
		walkOwnedStatements(body, func(value *statement) {
			count++
			switch value.Kind {
			case "variable", "initialize":
				if value.Initializer == nil {
					valid = false
				}
			case "assignment", "return", "expression", "block", "if", "while":
			default:
				valid = false
			}
			for _, expression := range []*expression{value.Expression, value.Initializer, value.Target, value.Value, value.Condition} {
				if !expressionFits(expression) {
					valid = false
				}
			}
		})
	}
	for _, function := range facts.Functions {
		if len(function.Parameters) > 32 || !signatureFits(function.ReturnSignature, 2) {
			return moduleCallableError("limit")
		}
		checkBody(function.Body)
	}
	for _, module := range facts.Modules {
		checkBody(module.Initialization)
	}
	if count > 16384 || calls > 16384 {
		return moduleCallableError("limit")
	}
	if !valid {
		return moduleCallableError("source")
	}
	return nil
}
