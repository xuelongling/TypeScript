// SPDX-License-Identifier: Apache-2.0

package main

import (
	"reflect"
	"strconv"
	"strings"
)

// Account for the actual encoded field set before allocating canonical bytes.
// The generic package budget has package-specific field aliases; this candidate
// uses only explicit JSON tags and the legacy DTO's normal omitempty behavior.
func moduleCallableWireBudget(value any) error {
	budget := genericCanonicalBudget{policy: moduleCallablePolicy()}
	var walk func(reflect.Value, int) error
	walk = func(v reflect.Value, depth int) error {
		if !v.IsValid() {
			return budget.add(4, 1)
		}
		if depth > 128 {
			return moduleCallableError("limit")
		}
		switch v.Kind() {
		case reflect.Pointer, reflect.Interface:
			if v.IsNil() {
				return budget.add(4, 1)
			}
			return walk(v.Elem(), depth)
		case reflect.Struct:
			if err := budget.add(2, 2); err != nil {
				return err
			}
			emitted := false
			for i := 0; i < v.NumField(); i++ {
				field := v.Type().Field(i)
				if field.PkgPath != "" {
					continue
				}
				name, options, _ := strings.Cut(field.Tag.Get("json"), ",")
				if name == "-" {
					continue
				}
				if name == "" {
					return moduleCallableError("input")
				}
				child := v.Field(i)
				empty := child.IsZero()
				if child.Kind() == reflect.Slice || child.Kind() == reflect.Map || child.Kind() == reflect.String {
					empty = child.Len() == 0
				}
				if options == "omitempty" && empty {
					continue
				}
				if emitted {
					if err := budget.add(1, 0); err != nil {
						return err
					}
				}
				if err := budget.text(name); err != nil {
					return err
				}
				if err := budget.add(1, 0); err != nil {
					return err
				}
				if err := walk(child, depth+1); err != nil {
					return err
				}
				emitted = true
			}
		case reflect.Slice, reflect.Array:
			if v.Kind() == reflect.Slice && v.IsNil() {
				return budget.add(4, 1)
			}
			if err := budget.add(2, 2); err != nil {
				return err
			}
			for i := 0; i < v.Len(); i++ {
				if i > 0 {
					if err := budget.add(1, 0); err != nil {
						return err
					}
				}
				if err := walk(v.Index(i), depth+1); err != nil {
					return err
				}
			}
		case reflect.String:
			return budget.text(v.String())
		case reflect.Bool:
			if v.Bool() {
				return budget.add(4, 1)
			}
			return budget.add(5, 1)
		case reflect.Int, reflect.Int32, reflect.Int64, reflect.Uint32:
			n := int64(0)
			if v.Kind() == reflect.Uint32 {
				n = int64(v.Uint())
			} else {
				n = v.Int()
			}
			if n < 0 || n > 9007199254740991 {
				return moduleCallableError("input")
			}
			return budget.add(len(strconv.FormatInt(n, 10)), 1)
		default:
			return moduleCallableError("input")
		}
		return nil
	}
	if err := walk(reflect.ValueOf(value), 0); err != nil {
		return moduleCallableJSONError(err)
	}
	return nil
}
