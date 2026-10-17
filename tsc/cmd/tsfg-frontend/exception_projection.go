// SPDX-License-Identifier: Apache-2.0

package main

import "github.com/microsoft/typescript-go/internal/ast"

func (p *projection) exceptionStatement(node *ast.Node, returnType string, returns ...*returnPresence) (statement, response) {
	if node.Kind == ast.KindThrowStatement {
		return p.throwStatement(node)
	}
	attempt := node.AsTryStatement()
	body, failure := p.exceptionBlock(attempt.TryBlock, returnType, returns...)
	if failure.Status != "" {
		return statement{}, failure
	}
	fact := &exceptionFact{TryBlock: body}
	if attempt.CatchClause != nil {
		fact.CatchClause, failure = p.exceptionCatch(attempt.CatchClause, returnType, returns...)
		if failure.Status != "" {
			return statement{}, failure
		}
	}
	if attempt.FinallyBlock != nil {
		body, failure := p.exceptionBlock(attempt.FinallyBlock, returnType, returns...)
		if failure.Status != "" {
			return statement{}, failure
		}
		fact.FinallyBlock = &body
	}
	return statement{Kind: "try", Exception: fact, Range: offset(p.file, node)}, response{}
}

func (p *projection) exceptionBlock(node *ast.Node, returnType string, returns ...*returnPresence) (exceptionBlockFact, response) {
	body, failure := p.statements(node.AsBlock().Statements.Nodes, returnType, returns...)
	if failure.Status != "" {
		return exceptionBlockFact{}, failure
	}
	return exceptionBlockFact{Body: body, Range: offset(p.file, node)}, response{}
}

func (p *projection) exceptionCatch(node *ast.Node, returnType string, returns ...*returnPresence) (*exceptionCatchFact, response) {
	clause := node.AsCatchClause()
	fact := &exceptionCatchFact{Range: offset(p.file, node)}
	if declaration := clause.VariableDeclaration; declaration != nil {
		local := declaration.AsVariableDeclaration()
		if local.Name() == nil || local.Name().Kind != ast.KindIdentifier || local.Initializer != nil {
			return nil, failAt("r02b-d-unsupported", "unsupported", p.file, declaration)
		}
		upstream := p.check.GetSymbolAtLocation(local.Name())
		unknown := p.check.GetUnknownType()
		if upstream == nil || len(upstream.Declarations) != 1 || upstream.Declarations[0] != declaration ||
			p.check.GetTypeOfSymbol(upstream) != unknown || p.check.GetTypeAtLocation(local.Name()) != unknown {
			return nil, failAt("r02b-d-unsupported", "unsupported", p.file, declaration)
		}
		binding, ok := p.addSymbol(local.Name(), "local", "unknown", true)
		if !ok {
			return nil, failAt("r02b-d-type", "error", p.file, local.Name())
		}
		fact.Binding = &binding
		where := offset(p.file, declaration)
		fact.DeclarationRange = &where
		if local.Type != nil {
			annotation := offset(p.file, local.Type)
			fact.AnnotationRange = &annotation
		}
	}
	body, failure := p.exceptionBlock(clause.Block, returnType, returns...)
	if failure.Status != "" {
		return nil, failure
	}
	fact.Block = body
	return fact, response{}
}

func (p *projection) throwStatement(node *ast.Node) (statement, response) {
	payload := node.AsThrowStatement().Expression
	actual := p.check.GetTypeAtLocation(payload)
	payloadType, undefined, null, finite := finitePresence(actual)
	var shape *parameterPresence
	if finite && (undefined || null) {
		// A bare absence has no owned nominal payload type. A finite union's
		// original checker members permit absence without supplying a type hint.
		if payloadType == "" {
			return statement{}, failAt("r02b-d-unsupported", "unsupported", p.file, node)
		}
		shape = &parameterPresence{Undefined: undefined, Null: null}
	}
	value, failure := p.expr(payload, "", shape)
	if failure.Status != "" {
		return statement{}, failure
	}
	// Successful projection retains this occurrence's scalar, callable or
	// aggregate facts. Unknown still requires the actual official catch type.
	if value.Type == "unknown" && actual != p.check.GetUnknownType() {
		return statement{}, failAt("r02b-d-unsupported", "unsupported", p.file, node)
	}
	return statement{Kind: "throw", Expression: value, Range: offset(p.file, node)}, response{}
}
