// SPDX-License-Identifier: Apache-2.0

package main

import "github.com/microsoft/typescript-go/internal/ast"

// Compiler symbols are adapter-local keys. Only owned IDs and lexical facts
// cross the response boundary; unrelated top-level scalar functions retain their
// existing wire shape, including omitted lexical metadata.
type closureProjection struct {
	parents      map[*ast.Node]*ast.Node
	members      map[*ast.Node]bool
	definitions  map[*ast.Node]symbol
	environments map[*ast.Node]map[*ast.Symbol]symbol
}

// Discover declarations before projection, without a speculative projection or
// fallback. Source preorder registers every parent header before its children.
func closureDeclarations(file *ast.SourceFile) ([]*ast.Node, *closureProjection) {
	state := &closureProjection{
		parents: map[*ast.Node]*ast.Node{}, members: map[*ast.Node]bool{},
		definitions: map[*ast.Node]symbol{}, environments: map[*ast.Node]map[*ast.Symbol]symbol{},
	}
	var declarations []*ast.Node
	var collect func(*ast.Node, *ast.Node)
	collect = func(node, parent *ast.Node) {
		declarations = append(declarations, node)
		state.parents[node] = parent
		if parent != nil {
			for ancestor := node; ancestor != nil; ancestor = state.parents[ancestor] {
				state.members[ancestor] = true
			}
		}
		if node.Kind != ast.KindFunctionDeclaration || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			return
		}
		visitStatements(node.Body().AsBlock().Statements.Nodes, func(item *ast.Node) bool {
			if item.Kind == ast.KindFunctionDeclaration {
				collect(item, node)
			}
			return false
		})
	}
	for _, node := range file.Statements.Nodes {
		collect(node, nil)
	}
	if len(state.members) == 0 {
		return file.Statements.Nodes, nil
	}
	return declarations, state
}

func (c *closureProjection) register(node *ast.Node, owned *symbol) {
	if c == nil || !c.members[node] {
		return
	}
	owned.LexicalOwner = c.definitions[c.parents[node]].ID
	c.definitions[node] = *owned
}

func (c *closureProjection) enter(node *ast.Node, p *projection) {
	if c == nil || !c.members[node] {
		return
	}
	p.closures = c
	p.closureOwner = c.definitions[node].ID
	// Each child receives a separate map after its parent's complete body has
	// registered locals. Official symbol identity handles forward references
	// and shadowing; siblings cannot install bindings into one another's map.
	for upstream, owned := range c.environments[c.parents[node]] {
		p.bindings[upstream] = owned
	}
}

func (c *closureProjection) complete(node *ast.Node, fn *function, p *projection) {
	if c == nil || !c.members[node] {
		return
	}
	c.environments[node] = p.bindings
	fn.LexicalParent = c.definitions[c.parents[node]].ID
	bindings := make(map[string]symbol, len(p.bindings))
	for _, owned := range p.bindings {
		bindings[owned.ID] = owned
	}
	fn.Captures = directClosureCaptures(fn, bindings)
}

func (p *projection) closureDeclaration(node *ast.Node) (statement, response) {
	if p.closures == nil {
		return statement{}, failAt("r02b-d-unsupported", "unsupported", p.file, node)
	}
	owned, ok := p.closures.definitions[node]
	if !ok || owned.LexicalOwner != p.closureOwner {
		return statement{}, failAt("r02b-d-unsupported", "unsupported", p.file, node)
	}
	return statement{Kind: "functionDeclaration", Symbol: &owned, Range: offset(p.file, node)}, response{}
}
