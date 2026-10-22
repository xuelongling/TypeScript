// SPDX-License-Identifier: Apache-2.0
// Ordinary relations preserve every source variant, unlike explicit assertions.
package main

import (
	"errors"

	"github.com/microsoft/typescript-go/internal/ast"
)

type arrayGenericRelationMismatch struct {
	numeric bool
	detail  string
}

func (e *arrayGenericRelationMismatch) Error() string { return e.detail }

func (p *arrayGenericProjection) logicalRelationError(node *ast.Node, err error) error {
	var mismatch *arrayGenericRelationMismatch
	if !errors.As(err, &mismatch) {
		return err
	}
	if mismatch.numeric {
		return p.numericRepresentationError(node)
	}
	return p.fail(node, mismatch.detail)
}

func (r *arrayGenericViews) logical(source, target string, node *ast.Node) ([]arrayGenericCompatibilityDependency, error) {
	if err := r.p.annotationWork(); err != nil {
		return nil, err
	}
	if source == target {
		return nil, nil
	}
	from, to := r.p.types[source], r.p.types[target]
	if from.Kind == "union" {
		selected := map[arrayGenericCompatibilityDependency]bool{}
		for _, member := range from.Members {
			if err := r.requireLogical(selected, member, target, node); err != nil {
				return nil, err
			}
		}
		return r.dependencyList(selected)
	}
	if to.Kind == "union" {
		// Prefer exact logical identity before structural candidates. A failed
		// candidate never establishes a recursive proof for a later candidate.
		for _, member := range to.Members {
			if err := r.p.annotationWork(); err != nil {
				return nil, err
			}
			if source == member {
				return nil, nil
			}
		}
		var rejected error
		for _, member := range to.Members {
			mark := r.beginProofCandidate()
			deps, err := r.logical(source, member, node)
			r.finishProofCandidate(mark, err == nil)
			var mismatch *arrayGenericRelationMismatch
			if !errors.As(err, &mismatch) {
				return deps, err
			}
			if rejected == nil || mismatch.numeric {
				rejected = err
			}
		}
		if rejected == nil {
			rejected = &arrayGenericRelationMismatch{detail: "source variant has no representation-preserving target member"}
		}
		return nil, rejected
	}
	if from.Kind == "value" || to.Kind == "value" {
		return nil, &arrayGenericRelationMismatch{detail: "logical value member view requires its original nominal identity"}
	}
	if (integerKind(source) || floatKind(source)) && (integerKind(target) || floatKind(target)) {
		return nil, &arrayGenericRelationMismatch{numeric: true, detail: "ordinary numeric member variants require identical representations"}
	}
	if (from.Kind == "array" || from.Kind == "readSpan") && (to.Kind == "array" || to.Kind == "readSpan") {
		return r.logical(from.ElementType, to.ElementType, node)
	}
	if from.Kind == "reference" && to.Kind == "reference" && r.p.check.IsTypeAssignableTo(r.p.actual[source], r.p.actual[target]) {
		_, err := r.reference(source, target, node)
		if err != nil {
			return nil, err
		}
		return []arrayGenericCompatibilityDependency{{SourceType: source, TargetType: target}}, nil
	}
	return nil, &arrayGenericRelationMismatch{detail: "logical member variant does not preserve its declared representation"}
}
