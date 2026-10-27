// SPDX-License-Identifier: Apache-2.0
// Original alias declaration/substitution nodes remain separate from actual types.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
)

func (p *arrayGenericProjection) systemAliasTarget(node *ast.Node) systemQueryOrigin {
	if node.Kind == ast.KindTypeReference {
		symbol := p.commonOriginalSymbol(p.check.GetSymbolAtLocation(node.AsTypeReferenceNode().TypeName))
		if symbol != nil && len(symbol.Declarations) == 1 {
			return systemOrigin(symbol.Declarations[0])
		}
	}
	return systemOrigin(node)
}
func (p *arrayGenericProjection) systemResolve(node *ast.Node, env map[*ast.Symbol]*ast.Node, traces *[]systemQueryAlias, depth int) (*ast.Node, map[*ast.Symbol]*ast.Node, error) {
	if node == nil {
		return nil, nil, arrayFactsError("capability", "original alias node required", nil)
	}
	if depth > 64 {
		return nil, nil, p.fail(node, "closed original alias graph required")
	}
	if err := p.annotationWork(); err != nil {
		return nil, nil, err
	}
	if node.Kind == ast.KindParenthesizedType {
		return p.systemResolve(node.AsParenthesizedTypeNode().Type, env, traces, depth+1)
	}
	if node.Kind != ast.KindTypeReference {
		return node, env, nil
	}
	ref := node.AsTypeReferenceNode()
	symbol := p.commonOriginalSymbol(p.check.GetSymbolAtLocation(ref.TypeName))
	if symbol == nil {
		return nil, nil, p.fail(node, "original alias Symbol absent")
	}
	if argument := env[symbol]; argument != nil {
		index := len(*traces)
		*traces = append(*traces, systemQueryAlias{UseSourcePath: ast.GetSourceFileOfNode(node).FileName(), UseRange: *genericSpan(node), Declaration: systemOrigin(symbol.Declarations[0])})
		resolved, next, e := p.systemResolve(argument, env, traces, depth+1)
		if e != nil {
			return nil, nil, e
		}
		(*traces)[index].Target = p.systemAliasTarget(resolved)
		return resolved, next, nil
	}
	if len(symbol.Declarations) != 1 || symbol.Declarations[0].Kind != ast.KindTypeAliasDeclaration {
		return node, env, nil
	}
	decl := symbol.Declarations[0]
	alias := decl.AsTypeAliasDeclaration()
	next := map[*ast.Symbol]*ast.Node{}
	for key, value := range env {
		next[key] = value
	}
	parameters := []*ast.Node{}
	arguments := []*ast.Node{}
	if alias.TypeParameters != nil {
		parameters = alias.TypeParameters.Nodes
	}
	if ref.TypeArguments != nil {
		arguments = ref.TypeArguments.Nodes
	}
	if len(parameters) != len(arguments) {
		return nil, nil, p.fail(node, "alias must retain complete original concrete arguments")
	}
	for index, parameter := range parameters {
		next[p.check.GetSymbolAtLocation(parameter.Name())] = arguments[index]
	}
	index := len(*traces)
	*traces = append(*traces, systemQueryAlias{UseSourcePath: ast.GetSourceFileOfNode(node).FileName(), UseRange: *genericSpan(node), Declaration: systemOrigin(decl)})
	resolved, result, e := p.systemResolve(alias.Type, next, traces, depth+1)
	if e != nil {
		return nil, nil, e
	}
	(*traces)[index].Target = p.systemAliasTarget(resolved)
	return resolved, result, nil
}
func (p *arrayGenericProjection) systemFixed(id string, depth int, active map[string]bool, heights map[string]int) error {
	if e := p.annotationWork(); e != nil {
		return e
	}
	if depth >= 32 {
		return arrayFactsError("capability", "fixed component graph exceeds32 nodes", nil)
	}
	if height, ok := heights[id]; ok {
		if depth+height > 32 {
			return arrayFactsError("capability", "cached fixed component graph exceeds32 nodes", nil)
		}
		return nil
	}
	if active[id] {
		return arrayFactsError("capability", "cyclic fixed component graph", nil)
	}
	typ, ok := p.types[id]
	if !ok {
		return arrayFactsError("identity", "unowned component field type", nil)
	}
	if typ.Kind == "scalar" {
		heights[id] = 1
		return nil
	}
	if typ.Kind != "value" && (typ.NominalKey == nil || typ.NominalKey.Category != "component") {
		return arrayFactsError("capability", "component contains GC/non-fixed content", nil)
	}
	active[id] = true
	height := 1
	seen := map[string]bool{}
	for current := id; current != ""; current = p.types[current].Base {
		if seen[current] || len(seen) >= 32 {
			return arrayFactsError("capability", "unowned/cyclic component base graph", nil)
		}
		seen[current] = true
		row, ok := p.types[current]
		if !ok {
			return arrayFactsError("identity", "component base outside whole type directory", nil)
		}
		if row.Fields != nil {
			for _, field := range *row.Fields {
				if e := p.systemFixed(field.Type, depth+1, active, heights); e != nil {
					return e
				}
				height = max(height, 1+heights[field.Type])
			}
		}
	}
	delete(active, id)
	heights[id] = height
	return nil
}
