// SPDX-License-Identifier: Apache-2.0
// The M12 official relation owns structural views; nominal keys never decide it.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

func (p *arrayGenericProjection) referenceCompatibility(source, target string, node *ast.Node) (*classCompatibility, error) {
	if p.types[source].Kind != "reference" || p.types[target].Kind != "reference" && p.types[target].Kind != "union" {
		return nil, p.fail(node, "logical expression type does not match its declared slot")
	}
	view := &classProjection{ctx: p.ctx, check: p.check, types: map[*checker.Type]string{}, bindings: map[*ast.Symbol]string{}, scalarKinds: map[string]string{}, resources: p.resources}
	fields, methods := map[string]string{}, map[string]arrayGenericFunction{}
	for symbol, id := range p.classes {
		view.types[p.actual[id]] = id
		view.bindings[symbol] = id
	}
	for symbol, field := range p.fields {
		view.bindings[symbol] = field.ID
		fields[field.ID] = field.Type
	}
	for symbol, method := range p.headers {
		view.bindings[symbol] = method.function.ID
		methods[method.function.ID] = method.function
	}
	relation := arrayGenericViews{p: p, official: view, fields: fields, methods: methods, seen: map[[2]string]bool{}, proofs: map[[2]string]arrayGenericCompatibilityWitness{}}
	witness, err := relation.referenceTarget(source, target, node)
	if err != nil {
		return nil, p.logicalRelationError(node, err)
	}
	if err := relation.publishWitnesses(witness); err != nil {
		return nil, err
	}
	return witness, nil
}
