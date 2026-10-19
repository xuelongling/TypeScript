// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/scanner"
)

// Keep the legacy annotator byte-for-byte behavior. This private profile adds
// complete callee traversal and the two callable discriminants it introduces.
func moduleCallableNames(file *ast.SourceFile, body []statement) {
	names := map[span]string{}
	stack := []*ast.Node{file.AsNode()}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		spelling := node
		if node.Kind == ast.KindCallExpression {
			spelling = node.AsCallExpression().Expression
		}
		if node.Kind == ast.KindIdentifier || node.Kind == ast.KindPropertyAccessExpression || node.Kind == ast.KindCallExpression {
			name := file.Text()[scanner.SkipTrivia(file.Text(), spelling.Pos()):spelling.End()]
			names[offset(file, node)] = name
			names[expressionOffset(file, node)] = name
		}
		node.ForEachChild(func(child *ast.Node) bool { stack = append(stack, child); return false })
	}
	var visit func(*expression)
	visit = func(value *expression) {
		if value == nil {
			return
		}
		switch value.Kind {
		case "reference", "functionReference", "call", "indirectCall":
			value.Name = names[value.Range]
		}
		for _, child := range []*expression{value.Callee, value.Left, value.Right, value.Operand} {
			visit(child)
		}
		if value.Arguments != nil {
			for i := range *value.Arguments {
				visit(&(*value.Arguments)[i])
			}
		}
	}
	walkOwnedStatements(body, func(value *statement) {
		for _, expression := range []*expression{value.Expression, value.Initializer, value.Target, value.Value, value.Condition} {
			visit(expression)
		}
	})
}
