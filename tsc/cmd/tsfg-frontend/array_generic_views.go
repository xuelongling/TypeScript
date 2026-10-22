// SPDX-License-Identifier: Apache-2.0
// Official M12 member IDs join logical representations erased by TS aliases.
package main

import "github.com/microsoft/typescript-go/internal/ast"

type arrayGenericViews struct {
	p           *arrayGenericProjection
	official    *classProjection
	fields      map[string]string
	methods     map[string]arrayGenericFunction
	seen        map[[2]string]bool
	proofs      map[[2]string]arrayGenericCompatibilityWitness
	proofUndo   []arrayGenericProofChange
	proofScopes int
}

func (r *arrayGenericViews) reference(source, target string, node *ast.Node) (*classCompatibility, error) {
	pair := [2]string{source, target}
	if r.seen[pair] {
		return nil, nil
	}
	r.seen[pair] = true
	defer delete(r.seen, pair)
	witness, err := r.official.compatibility(r.p.actual[source], r.p.actual[target], node)
	if err != nil {
		return nil, err
	}
	dependencies := map[arrayGenericCompatibilityDependency]bool{}
	for _, member := range witness.Members {
		if err := r.p.annotationWork(); err != nil {
			return nil, err
		}
		from, to := r.fields[member.Source], r.fields[member.Target]
		if from != "" && to != "" {
			if err := r.requireLogical(dependencies, from, to, node); err != nil {
				return nil, err
			}
			continue
		}
		fromMethod, toMethod := r.methods[member.Source], r.methods[member.Target]
		if fromMethod.ID == "" || toMethod.ID == "" {
			return nil, r.p.fail(node, "official member view has no owned logical field or method pair")
		}
		// A void method view discards its result, as in the official signature
		// relation; no numeric value is reinterpreted at that boundary.
		if toMethod.ReturnType != "void" {
			if err := r.requireLogical(dependencies, fromMethod.ReturnType, toMethod.ReturnType, node); err != nil {
				return nil, err
			}
		}
		for i := 0; i < min(len(fromMethod.Parameters), len(toMethod.Parameters)); i++ {
			if err := r.requireLogical(dependencies, toMethod.Parameters[i].DeclaredType, fromMethod.Parameters[i].DeclaredType, node); err != nil {
				return nil, err
			}
		}
	}
	deps, err := r.dependencyList(dependencies)
	if err != nil {
		return nil, err
	}
	if err := r.storeProof(pair, arrayGenericCompatibilityWitness{Compatibility: witness, Dependencies: deps}); err != nil {
		return nil, err
	}
	return &witness, nil
}

// Identity assertions select existing variants without changing their payload.
// Structural views still require the actual selected M12 member proof roots.
func (p *arrayGenericProjection) assertionClassView(source, target string, node *ast.Node) ([]arrayGenericCompatibilityDependency, error) {
	if source == target {
		return nil, nil
	}
	from, to := p.types[source], p.types[target]
	if to.Kind == "undefined" || to.Kind == "null" || to.Kind == "void" {
		return nil, nil
	}
	identity, err := p.assertionIdentityView(source, target)
	if err != nil || identity {
		return nil, err
	}
	if from.Kind == "union" {
		members := []string{}
		for _, id := range from.Members {
			if id != "undefined" && id != "null" {
				members = append(members, id)
			}
		}
		if len(members) == 1 {
			return p.assertionClassView(members[0], target, node)
		}
		for _, id := range members {
			if p.types[id].Kind == "reference" {
				return nil, p.fail(node, "multiple nominal assertion sources require distinct member view witnesses")
			}
		}
	}
	if to.Kind == "union" {
		for _, id := range to.Members {
			if id == source {
				return nil, nil
			}
		}
		members := []string{}
		for _, id := range to.Members {
			if id != "undefined" && id != "null" {
				members = append(members, id)
			}
		}
		if len(members) == 1 {
			return p.assertionClassView(source, members[0], node)
		}
		if from.Kind == "reference" {
			return nil, p.fail(node, "multiple nominal assertion targets require distinct member view witnesses")
		}
	}
	if (from.Kind == "array" || from.Kind == "readSpan") && (to.Kind == "array" || to.Kind == "readSpan") {
		return p.assertionClassView(from.ElementType, to.ElementType, node)
	}
	if from.Kind == "reference" && to.Kind == "reference" {
		return p.referenceCompatibility(source, target, node)
	}
	if from.Kind == "value" || to.Kind == "value" {
		return nil, p.fail(node, "value assertion requires its original nominal identity")
	}
	return nil, nil
}
