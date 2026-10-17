// SPDX-License-Identifier: Apache-2.0

package main

import "reflect"

const exceptionFactsMaxParameters = 64
const exceptionFactsMaxSignatureDepth = 64

type exceptionFactsSignatureBudget struct {
	heights map[*signature]int
	active  map[*signature]bool
}

// Cached height is intrinsic to the signature, not to its first occurrence.
// A shared subtree must still fit below every later occurrence's prefix.
func (b *exceptionFactsSignatureBudget) pointer(item *signature, prefix int) (int, bool) {
	if item == nil {
		return 0, true
	}
	if prefix >= exceptionFactsMaxSignatureDepth || b.active[item] {
		return 0, false
	}
	if height, found := b.heights[item]; found {
		return height, prefix+height <= exceptionFactsMaxSignatureDepth
	}
	b.active[item] = true
	height, valid := b.value(*item, prefix)
	delete(b.active, item)
	if valid {
		b.heights[item] = height
	}
	return height, valid
}

func (b *exceptionFactsSignatureBudget) value(item signature, prefix int) (int, bool) {
	if prefix >= exceptionFactsMaxSignatureDepth || len(item.Parameters) > exceptionFactsMaxParameters {
		return 0, false
	}
	height := 1
	for _, parameter := range item.ParameterSignatures {
		childHeight, valid := b.pointer(parameter, prefix+1)
		if !valid {
			return 0, false
		}
		height = max(height, 1+childHeight)
	}
	childHeight, valid := b.pointer(item.ReturnSignature, prefix+1)
	if !valid {
		return 0, false
	}
	height = max(height, 1+childHeight)
	return height, prefix+height <= exceptionFactsMaxSignatureDepth
}

// One typed DTO walk reaches signatures on symbols, expressions, returns and
// constructors. The signature graph has its own path budget and cycle check;
// other DTO arrays retain their own limits. No AST or checker enters this walk.
func exceptionFactsCallableBounds(facts response) error {
	signatures := exceptionFactsSignatureBudget{heights: map[*signature]int{}, active: map[*signature]bool{}}
	var valid func(reflect.Value) bool
	valid = func(value reflect.Value) bool {
		if !value.IsValid() || !value.CanInterface() {
			return true
		}
		if value.Kind() == reflect.Pointer && value.IsNil() {
			return true
		}
		switch item := value.Interface().(type) {
		case *signature:
			_, fits := signatures.pointer(item, 0)
			return fits
		case signature:
			_, fits := signatures.value(item, 0)
			return fits
		case function:
			if len(item.Parameters) > exceptionFactsMaxParameters {
				return false
			}
		case aggregateConstructor:
			if len(item.Parameters) > exceptionFactsMaxParameters {
				return false
			}
		}
		switch value.Kind() {
		case reflect.Pointer, reflect.Interface:
			return valid(value.Elem())
		case reflect.Struct:
			for i := 0; i < value.NumField(); i++ {
				if !valid(value.Field(i)) {
					return false
				}
			}
		case reflect.Array, reflect.Slice:
			for i := 0; i < value.Len(); i++ {
				if !valid(value.Index(i)) {
					return false
				}
			}
		}
		return true
	}
	if !valid(reflect.ValueOf(facts)) {
		return exceptionFactsError("limit", "owned parameter arity or signature path depth exceeds 64")
	}
	return nil
}
