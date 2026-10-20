// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Class projection uses the official access and readonly rules at an actual use.
package checker

import "github.com/microsoft/typescript-go/internal/ast"

// TSFGClassPropertyFacts is compiler-local. Owned exporters copy its results and
// references into their own graph; these objects never belong in a wire DTO.
type TSFGClassPropertyFacts struct {
	Receiver *Type
	Property *ast.Symbol
	Type     *Type
	Readonly bool
	Readable bool
	Writable bool
	IsWrite  bool
}

// TSFGClassPropertyAccess accepts no caller-selected symbol or permission. Every
// object is resolved from a real property expression belonging to this Program.
func (c *Checker) TSFGClassPropertyAccess(expr *ast.Node) (TSFGClassPropertyFacts, bool) {
	if expr == nil || expr.Kind != ast.KindPropertyAccessExpression {
		return TSFGClassPropertyFacts{}, false
	}
	file := ast.GetSourceFileOfNode(expr)
	if file == nil || c.program.GetSourceFile(file.FileName()) != file ||
		expr.Expression() == nil || expr.Name() == nil || expr.Name().Kind != ast.KindIdentifier {
		return TSFGClassPropertyFacts{}, false
	}
	for _, node := range []*ast.Node{expr.Expression(), expr.Name()} {
		if ast.GetSourceFileOfNode(node) != file || node.Parent != expr {
			return TSFGClassPropertyFacts{}, false
		}
	}
	receiver := c.GetTypeAtLocation(expr.Expression())
	property := c.GetSymbolAtLocation(expr.Name())
	if receiver == nil || property == nil ||
		c.GetPropertyOfType(receiver, expr.Name().Text()) != property {
		return TSFGClassPropertyFacts{}, false
	}
	isSuper := expr.Expression().Kind == ast.KindSuperKeyword
	kind := getAssignmentTargetKind(expr)
	return TSFGClassPropertyFacts{
		Receiver: receiver, Property: property, Type: c.GetTypeAtLocation(expr),
		Readonly: c.isReadonlySymbol(property),
		Readable: c.isPropertyAccessible(expr, isSuper, false, receiver, property),
		Writable: c.isPropertyAccessible(expr, isSuper, true, receiver, property) &&
			!c.isAssignmentToReadonlyEntity(expr, property, AssignmentKindDefinite),
		IsWrite: kind != AssignmentKindNone,
	}, true
}
