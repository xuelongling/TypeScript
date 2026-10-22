// SPDX-License-Identifier: Apache-2.0
// The M12 official relation owns structural views; nominal keys never decide it.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

func (p *arrayGenericProjection) referenceCompatibility(source, target string, node *ast.Node) ([]arrayGenericCompatibilityDependency, error) {
	view := &classProjection{ctx: p.ctx, check: p.check, types: map[*checker.Type]string{}, bindings: map[*ast.Symbol]string{}, scalarKinds: map[string]string{}, resources: p.resources}
	fields, methods := map[string]string{}, map[string]arrayGenericFunction{}
	for symbol, id := range p.classes {
		if err := p.annotationWork(); err != nil {
			return nil, err
		}
		view.types[p.actual[id]] = id
		view.bindings[symbol] = id
	}
	for symbol, field := range p.fields {
		if err := p.annotationWork(); err != nil {
			return nil, err
		}
		view.bindings[symbol] = field.ID
		fields[field.ID] = field.Type
	}
	for symbol, method := range p.headers {
		if err := p.annotationWork(); err != nil {
			return nil, err
		}
		view.bindings[symbol] = method.function.ID
		methods[method.function.ID] = method.function
	}
	relation := arrayGenericViews{p: p, official: view, fields: fields, methods: methods, seen: map[[2]string]bool{}, proofs: map[[2]string]arrayGenericCompatibilityWitness{}}
	roots, err := relation.logical(source, target, node)
	if err != nil {
		return nil, p.logicalRelationError(node, err)
	}
	for _, root := range roots {
		proof, ok := relation.proofs[[2]string{root.SourceType, root.TargetType}]
		if !ok {
			return nil, arrayFactsError("identity", "selected expression root has no actual class proof", nil)
		}
		if err := relation.publishWitnesses(&proof.Compatibility); err != nil {
			return nil, err
		}
	}
	return roots, nil
}
