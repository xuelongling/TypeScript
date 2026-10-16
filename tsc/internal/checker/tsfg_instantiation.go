// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Instantiates body facts using this checker's resolved call mapper. The mapper
// remains private and cannot become an owned-package or Product type calculator.
package checker

import "github.com/microsoft/typescript-go/internal/ast"

// TSFGInstantiateBodyType projects one expression or explicit type annotation in
// declaration under invocation's official resolved signature. Both syntax trees
// must belong to this checker, and node must be inside that exact declaration.
// No unchecked Signature or Type supplied by a caller enters the mapper.
func (c *Checker) TSFGInstantiateBodyType(invocation, declaration, node *ast.Node, typeNode bool) (*Type, bool) {
	if invocation == nil || invocation.Kind != ast.KindCallExpression || declaration == nil ||
		declaration.Kind != ast.KindFunctionDeclaration || node == nil {
		return nil, false
	}
	for _, candidate := range []*ast.Node{invocation, declaration, node} {
		file := ast.GetSourceFileOfNode(candidate)
		if file == nil || c.program.GetSourceFile(file.FileName()) != file {
			return nil, false
		}
	}
	inside := false
	for parent := node; parent != nil; parent = parent.Parent {
		if parent == declaration {
			inside = true
			break
		}
	}
	if !inside {
		return nil, false
	}
	call := invocation.AsCallExpression()
	if call.Expression.Kind != ast.KindIdentifier || call.TypeArguments == nil {
		return nil, false
	}
	sig := c.GetResolvedSignature(invocation)
	decl := declaration.AsFunctionDeclaration()
	if sig == nil || sig.declaration != declaration || sig.mapper == nil || decl.TypeParameters == nil ||
		len(call.TypeArguments.Nodes) != len(decl.TypeParameters.Nodes) ||
		c.GetSymbolAtLocation(call.Expression) != c.GetSymbolAtLocation(declaration.Name()) {
		return nil, false
	}
	var t *Type
	if typeNode {
		if !ast.IsTypeNode(node) {
			return nil, false
		}
		t = c.GetTypeFromTypeNode(node)
	} else {
		t = c.GetTypeAtLocation(node)
	}
	if t == nil {
		return nil, false
	}
	return c.instantiateType(t, sig.mapper), true
}
