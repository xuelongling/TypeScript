// SPDX-License-Identifier: Apache-2.0
// Publish only selected root-reachable proofs; origins keep occurrences distinct.
package main

import (
	"slices"

	"github.com/microsoft/typescript-go/internal/ast"
)

type arrayGenericWitnessIdentity struct {
	Origin classAnchor
	Pair   [2]string
}

func (r *arrayGenericViews) requireLogical(selected map[arrayGenericCompatibilityDependency]bool, source, target string, node *ast.Node) error {
	deps, err := r.logical(source, target, node)
	if err != nil {
		return err
	}
	for _, dep := range deps {
		if err := r.p.annotationWork(); err != nil {
			return err
		}
		selected[dep] = true
	}
	return nil
}

func dependencyLess(a, b arrayGenericCompatibilityDependency) bool {
	return a.SourceType < b.SourceType || a.SourceType == b.SourceType && a.TargetType < b.TargetType
}

func (r *arrayGenericViews) dependencyList(selected map[arrayGenericCompatibilityDependency]bool) ([]arrayGenericCompatibilityDependency, error) {
	deps := make([]arrayGenericCompatibilityDependency, 0, len(selected))
	for dep := range selected {
		if err := r.p.annotationWork(); err != nil {
			return nil, err
		}
		deps = append(deps, dep)
	}
	if err := sortArrayGeneric(deps, r.p.annotationWork, dependencyLess); err != nil {
		return nil, err
	}
	return deps, nil
}

func (r *arrayGenericViews) publishWitnesses(root *classCompatibility) error {
	if root == nil {
		return arrayFactsError("identity", "missing selected witness root", nil)
	}
	initial := [2]string{root.SourceType, root.TargetType}
	pending, queued := [][2]string{initial}, map[[2]string]bool{initial: true}
	selected := []arrayGenericCompatibilityWitness{}
	for len(pending) > 0 {
		if err := r.p.annotationWork(); err != nil {
			return err
		}
		pair := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		record, ok := r.proofs[pair]
		if !ok || record.Compatibility.Origin != root.Origin {
			return arrayFactsError("identity", "selected witness dependency is not closed in its original source occurrence", nil)
		}
		selected = append(selected, record)
		for _, dep := range record.Dependencies {
			if err := r.p.annotationWork(); err != nil {
				return err
			}
			next := [2]string{dep.SourceType, dep.TargetType}
			if !queued[next] {
				queued[next] = true
				pending = append(pending, next)
			}
		}
	}
	if r.p.witnesses == nil {
		r.p.witnesses = map[arrayGenericWitnessIdentity]arrayGenericCompatibilityWitness{}
	}
	for _, record := range selected {
		if err := r.p.annotationWork(); err != nil {
			return err
		}
		proof := record.Compatibility
		key := arrayGenericWitnessIdentity{Origin: proof.Origin, Pair: [2]string{proof.SourceType, proof.TargetType}}
		if prior, exists := r.p.witnesses[key]; exists {
			if prior.Compatibility.Assignable != proof.Assignable || !slices.Equal(prior.Compatibility.Members, proof.Members) || !slices.Equal(prior.Dependencies, record.Dependencies) {
				return arrayFactsError("identity", "same original witness identity changed its selected member proof", nil)
			}
			continue
		}
		if len(r.p.witnesses) >= 100000 {
			return arrayFactsError("limit", "owned compatibility witness records", nil)
		}
		r.p.witnesses[key] = record
	}
	return nil
}

func (p *arrayGenericProjection) finishWitnesses() error {
	for _, record := range p.witnesses {
		if err := p.annotationWork(); err != nil {
			return err
		}
		p.payload.CompatibilityWitnesses = append(p.payload.CompatibilityWitnesses, record)
	}
	return sortArrayGeneric(p.payload.CompatibilityWitnesses, p.annotationWork, func(a, b arrayGenericCompatibilityWitness) bool {
		x, y := a.Compatibility, b.Compatibility
		if x.Origin.Source != y.Origin.Source {
			return x.Origin.Source < y.Origin.Source
		}
		if x.Origin.Start != y.Origin.Start {
			return x.Origin.Start < y.Origin.Start
		}
		if x.Origin.End != y.Origin.End {
			return x.Origin.End < y.Origin.End
		}
		return dependencyLess(arrayGenericCompatibilityDependency{SourceType: x.SourceType, TargetType: x.TargetType}, arrayGenericCompatibilityDependency{SourceType: y.SourceType, TargetType: y.TargetType})
	})
}
