// SPDX-License-Identifier: Apache-2.0
// Entry proofs describe possible logical views, never allocations or noalias.
package main

import (
	"errors"

	"github.com/microsoft/typescript-go/internal/ast"
)

// Walk only types carried by this parameter. Inheritance contributes members,
// not a separately admitted base-class payload. Seen sets close recursive data.
func (p *arrayGenericProjection) entryReferenceTargets(id string) ([]string, error) {
	pending, seen, members := []string{id}, map[string]bool{}, map[string]bool{}
	targets := []string{}
	methods := map[string]arrayGenericFunction{}
	for _, header := range p.headers {
		if err := p.annotationWork(); err != nil {
			return nil, err
		}
		methods[header.function.ID] = header.function
	}
	push := func(id string) error {
		if err := p.annotationWork(); err != nil {
			return err
		}
		if !seen[id] {
			seen[id] = true
			pending = append(pending, id)
		}
		return nil
	}
	seen[id] = true
	for len(pending) > 0 {
		if err := p.annotationWork(); err != nil {
			return nil, err
		}
		id := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		typ, ok := p.types[id]
		if !ok {
			return nil, arrayFactsError("identity", "entry type closure has no catalog record", nil)
		}
		switch typ.Kind {
		case "array", "readSpan":
			if err := push(typ.ElementType); err != nil {
				return nil, err
			}
		case "union":
			for _, child := range typ.Members {
				if err := push(child); err != nil {
					return nil, err
				}
			}
		case "reference", "value":
			if typ.Kind == "reference" {
				targets = append(targets, id)
			}
			for owner := id; owner != "" && !members[owner]; owner = p.types[owner].Base {
				if err := p.annotationWork(); err != nil {
					return nil, err
				}
				members[owner] = true
				if p.types[owner].Fields != nil {
					for _, field := range *p.types[owner].Fields {
						if err := push(field.Type); err != nil {
							return nil, err
						}
					}
				}
				for _, method := range p.types[owner].Methods {
					if err := p.annotationWork(); err != nil {
						return nil, err
					}
					fn, ok := methods[method]
					if !ok || fn.OwnerType != owner {
						return nil, arrayFactsError("identity", "entry method must join its actual owner", nil)
					}
					if err := push(fn.ReturnType); err != nil {
						return nil, err
					}
					for _, param := range fn.Parameters {
						if err := push(param.DeclaredType); err != nil {
							return nil, err
						}
					}
				}
			}
		}
	}
	if err := sortArrayGeneric(targets, p.annotationWork, func(a, b string) bool { return a < b }); err != nil {
		return nil, err
	}
	return targets, nil
}

func (p *arrayGenericProjection) entryParameterRoots(view string, node *ast.Node, candidates []string) ([]arrayGenericCompatibilityDependency, error) {
	targets, err := p.entryReferenceTargets(view)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return []arrayGenericCompatibilityDependency{}, nil
	}
	r, err := p.classViews()
	if err != nil {
		return nil, err
	}
	selected := map[arrayGenericCompatibilityDependency]bool{}
	for _, target := range targets {
		for _, source := range candidates {
			if err := p.annotationWork(); err != nil {
				return nil, err
			}
			if source == target || !p.check.IsTypeAssignableTo(p.actual[source], p.actual[target]) {
				continue
			}
			mark := r.beginProofCandidate()
			roots, err := r.logical(source, target, node)
			r.finishProofCandidate(mark, err == nil)
			var mismatch *arrayGenericRelationMismatch
			if errors.As(err, &mismatch) {
				continue
			}
			if err != nil {
				return nil, err
			}
			for _, root := range roots {
				if err := p.annotationWork(); err != nil {
					return nil, err
				}
				proof := r.proofs[[2]string{root.SourceType, root.TargetType}]
				if err := r.publishWitnesses(&proof.Compatibility); err != nil {
					return nil, err
				}
				selected[root] = true
			}
		}
	}
	return r.dependencyList(selected)
}

func (p *arrayGenericProjection) entryCompatibilityRoots() error {
	candidates := []string{}
	for id, typ := range p.types {
		if err := p.annotationWork(); err != nil {
			return err
		}
		if typ.Kind == "reference" {
			candidates = append(candidates, id)
		}
	}
	if err := sortArrayGeneric(candidates, p.annotationWork, func(a, b string) bool { return a < b }); err != nil {
		return err
	}
	headers := map[string]*arrayGenericHeader{}
	for _, header := range p.headers {
		if err := p.annotationWork(); err != nil {
			return err
		}
		headers[header.function.ID] = header
	}
	entries, seen := []arrayGenericEntryCompatibilityRoots{}, map[string]bool{}
	for _, export := range p.payload.Exports {
		if err := p.annotationWork(); err != nil {
			return err
		}
		if seen[export.FunctionID] {
			continue
		}
		seen[export.FunctionID] = true
		header := headers[export.FunctionID]
		if header == nil || header.node.Kind != ast.KindFunctionDeclaration {
			return arrayFactsError("identity", "entry must join its original exported function", nil)
		}
		for i, node := range header.node.AsFunctionDeclaration().Parameters.Nodes {
			if err := p.annotationWork(); err != nil {
				return err
			}
			param := header.function.Parameters[i]
			roots, err := p.entryParameterRoots(param.DeclaredType, node, candidates)
			if err != nil {
				return err
			}
			entries = append(entries, arrayGenericEntryCompatibilityRoots{FunctionID: export.FunctionID, ParameterBinding: param.ID, ViewType: param.DeclaredType, Origin: classNodeAnchor(node), Roots: roots})
		}
	}
	if err := sortArrayGeneric(entries, p.annotationWork, func(a, b arrayGenericEntryCompatibilityRoots) bool {
		return a.FunctionID < b.FunctionID || a.FunctionID == b.FunctionID && a.ParameterBinding < b.ParameterBinding
	}); err != nil {
		return err
	}
	p.payload.EntryCompatibilityRoots = entries
	return nil
}
