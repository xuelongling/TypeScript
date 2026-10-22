// SPDX-License-Identifier: Apache-2.0
// Expression roots share one occurrence; identity selection needs no class map.
package main

func (p *arrayGenericProjection) assertionContainsNominal(id string) (bool, error) {
	if err := p.annotationWork(); err != nil {
		return false, err
	}
	typ := p.types[id]
	switch typ.Kind {
	case "reference", "value":
		return true, nil
	case "array", "readSpan":
		return p.assertionContainsNominal(typ.ElementType)
	case "union":
		for _, member := range typ.Members {
			found, err := p.assertionContainsNominal(member)
			if err != nil || found {
				return found, err
			}
		}
	}
	return false, nil
}

func (p *arrayGenericProjection) mergeCompatibilityRoots(a, b []arrayGenericCompatibilityDependency) ([]arrayGenericCompatibilityDependency, error) {
	selected := map[arrayGenericCompatibilityDependency]bool{}
	for _, roots := range [][]arrayGenericCompatibilityDependency{a, b} {
		for _, root := range roots {
			if err := p.annotationWork(); err != nil {
				return nil, err
			}
			selected[root] = true
		}
	}
	r := arrayGenericViews{p: p}
	return r.dependencyList(selected)
}

func (p *arrayGenericProjection) assertionIdentityView(source, target string) (bool, error) {
	from, to := p.types[source], p.types[target]
	if (from.Kind == "array" || from.Kind == "readSpan") && (to.Kind == "array" || to.Kind == "readSpan") {
		return p.assertionIdentityView(from.ElementType, to.ElementType)
	}
	members := func(id string) (map[string]bool, error) {
		ids := []string{id}
		if p.types[id].Kind == "union" {
			ids = p.types[id].Members
		}
		out := map[string]bool{}
		for _, member := range ids {
			if err := p.annotationWork(); err != nil {
				return nil, err
			}
			if member != "undefined" && member != "null" {
				out[member] = true
			}
		}
		return out, nil
	}
	a, err := members(source)
	if err != nil {
		return false, err
	}
	b, err := members(target)
	if err != nil {
		return false, err
	}
	if len(a) == 0 || len(b) == 0 {
		return false, nil
	}
	subset := func(x, y map[string]bool) (bool, error) {
		for id := range x {
			if err := p.annotationWork(); err != nil {
				return false, err
			}
			if !y[id] {
				return false, nil
			}
		}
		return true, nil
	}
	match, err := subset(a, b)
	if err != nil || match {
		return match, err
	}
	return subset(b, a)
}
