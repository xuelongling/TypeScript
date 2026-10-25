// SPDX-License-Identifier: Apache-2.0
// Preserve original exception regions; caught values remain unknown until used.
package main

import "github.com/microsoft/typescript-go/internal/ast"

func (p *arrayGenericProjection) commonException(node *ast.Node, state *arrayGenericState, fn *arrayGenericFunction, depth int) (arrayGenericStatement, error) {
	out := arrayGenericStatement{Range: *genericSpan(node)}
	if node.Kind == ast.KindThrowStatement {
		value, err := p.expression(node.AsThrowStatement().Expression, "", state, 0)
		out.Kind, out.Value = "throw", value
		return out, err
	}
	for _, length := range state.lengths {
		if err := p.annotationWork(); err != nil {
			return out, err
		}
		length.known = false
	}
	block := func(node *ast.Node, selected *arrayGenericState) (*commonProgramBlock, error) {
		body, err := p.statements(node.AsBlock().Statements.Nodes, selected, fn, depth+1)
		return &commonProgramBlock{Range: *genericSpan(node), Body: body}, err
	}
	out.Kind = "try"
	attempt := node.AsTryStatement()
	var err error
	out.TryBlock, err = block(attempt.TryBlock, state)
	if err != nil {
		return out, err
	}
	if attempt.CatchClause != nil {
		clause := attempt.CatchClause.AsCatchClause()
		if err := p.seedCommonGlobals(state); err != nil {
			return out, err
		}
		selected := &arrayGenericState{header: state.header, ownerType: state.ownerType, commonSeeded: true,
			bindings: map[*ast.Symbol]arrayFactsBinding{}, lengths: state.lengths}
		for symbol, binding := range state.bindings {
			if err := p.annotationWork(); err != nil {
				return out, err
			}
			selected.bindings[symbol] = binding
		}
		caught := &commonProgramCatch{Range: *genericSpan(attempt.CatchClause)}
		if declaration := clause.VariableDeclaration; declaration != nil {
			decl := declaration.AsVariableDeclaration()
			if declaration.Name() == nil || declaration.Name().Kind != ast.KindIdentifier || decl.Initializer != nil {
				return out, p.fail(declaration, "catch requires its actual named unknown binding")
			}
			symbol := p.check.GetSymbolAtLocation(declaration.Name())
			unknown := p.check.GetUnknownType()
			if symbol == nil || p.check.GetTypeOfSymbol(symbol) != unknown {
				return out, p.fail(declaration, "catch binding must retain official unknown")
			}
			p.addType(arrayGenericType{ID: "unknown", Kind: "unknown"}, unknown)
			binding, failure := p.typedBinding(declaration, decl.Type, "catch", true, "unknown")
			if failure != nil {
				return out, failure
			}
			caught.Binding, caught.DeclarationRange = &binding, genericSpan(declaration)
			selected.bindings[symbol] = binding
			fn.Bindings = append(fn.Bindings, binding)
		}
		body, failure := block(clause.Block, selected)
		if failure != nil {
			return out, failure
		}
		caught.Block = *body
		out.CatchClause = caught
	}
	if attempt.FinallyBlock != nil {
		out.FinallyBlock, err = block(attempt.FinallyBlock, state)
	}
	return out, err
}
