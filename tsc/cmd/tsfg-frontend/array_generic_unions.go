// SPDX-License-Identifier: Apache-2.0
// Expanded edges and sorting share annotation work, including cached unions.
package main

import (
	"strings"

	"github.com/microsoft/typescript-go/internal/checker"
)

func (p *arrayGenericProjection) annotationWork() error {
	if err := p.ctx.Err(); err != nil {
		return arrayFactsError("canceled", "v8 type projection work", err)
	}
	if p.annotationCalls >= 100000 {
		return arrayFactsError("limit", "v8 type projection work", nil)
	}
	p.annotationCalls++
	return nil
}

func (p *arrayGenericProjection) union(members []string, actual *checker.Type) (string, error) {
	flat, seen, size := []string{}, map[string]bool{}, len("union<>")-1
	add := func(id string) error {
		if err := p.annotationWork(); err != nil {
			return err
		}
		if !seen[id] {
			if size+len(id)+1 > 1048576 {
				return arrayFactsError("limit", "v8 expanded concrete type key bytes", nil)
			}
			size += len(id) + 1
			seen[id] = true
			flat = append(flat, id)
		}
		return nil
	}
	for _, id := range members {
		if err := p.annotationWork(); err != nil {
			return "", err
		}
		if p.types[id].Kind == "union" {
			for _, member := range p.types[id].Members {
				if err := add(member); err != nil {
					return "", err
				}
			}
		} else if err := add(id); err != nil {
			return "", err
		}
	}
	if len(flat) == 1 {
		return flat[0], nil
	}
	if err := p.sortUnion(flat); err != nil {
		return "", err
	}
	id := "union<" + strings.Join(flat, ",") + ">"
	if err := p.typeKeyBudget(id); err != nil {
		return "", err
	}
	return p.addType(arrayGenericType{ID: id, Kind: "union", Members: flat}, actual), nil
}

// A bounded merge checks cancellation/work on every copied sorting edge;
// temporary storage is proportional only to already unique catalog members.
func (p *arrayGenericProjection) sortUnion(ids []string) error {
	return sortArrayGeneric(ids, p.annotationWork, func(a, b string) bool { return a < b })
}

// Union IDs and witness graph records both need bounded, cancelable ordering.
func sortArrayGeneric[T any](values []T, work func() error, less func(T, T) bool) error {
	temporary := make([]T, len(values))
	for width := 1; width < len(values); width *= 2 {
		for start := 0; start < len(values); start += width * 2 {
			middle, end := min(start+width, len(values)), min(start+width*2, len(values))
			left, right := start, middle
			for i := start; i < end; i++ {
				if err := work(); err != nil {
					return err
				}
				if left < middle && (right == end || less(values[left], values[right])) {
					temporary[i], left = values[left], left+1
				} else {
					temporary[i], right = values[right], right+1
				}
			}
		}
		copy(values, temporary)
	}
	return nil
}
