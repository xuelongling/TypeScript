// SPDX-License-Identifier: Apache-2.0
// Count this private owned DTO's complete JCS size before serialization or hashing.
package main

import (
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

func arrayFactsOutputBudget(packet arrayFactsPackage) error {
	// The single framing LF participates in the complete output byte budget.
	count := 1
	add := func(n int) bool {
		if n > arrayFactsMaxBytes-count {
			return false
		}
		count += n
		return true
	}
	var walk func(reflect.Value) bool
	quoted := func(text string) bool {
		if !add(2) {
			return false
		}
		for _, r := range text {
			width := utf8.RuneLen(r)
			switch {
			case r == '"' || r == '\\' || r == '\b' || r == '\t' || r == '\n' || r == '\f' || r == '\r':
				width = 2
			case r < 32:
				width = 6
			}
			if !add(width) {
				return false
			}
		}
		return true
	}
	walk = func(value reflect.Value) bool {
		if value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
			if value.IsNil() {
				return add(4)
			}
			return walk(value.Elem())
		}
		switch value.Kind() {
		case reflect.String:
			return quoted(value.String())
		case reflect.Bool:
			if value.Bool() {
				return add(4)
			}
			return add(5)
		case reflect.Int, reflect.Int32, reflect.Int64:
			return add(len(strconv.FormatInt(value.Int(), 10)))
		case reflect.Slice, reflect.Array:
			if value.Kind() == reflect.Slice && value.IsNil() {
				return add(4)
			}
			if !add(2) {
				return false
			}
			for i := 0; i < value.Len(); i++ {
				if i > 0 && !add(1) {
					return false
				}
				if !walk(value.Index(i)) {
					return false
				}
			}
			return true
		case reflect.Struct:
			if !add(2) {
				return false
			}
			fields := 0
			for i := 0; i < value.NumField(); i++ {
				field := value.Type().Field(i)
				tags := strings.Split(field.Tag.Get("json"), ",")
				if tags[0] == "-" {
					continue
				}
				item := value.Field(i)
				// JSON v2 omitzero tests the Go value; omitempty tests the encoded
				// JSON null/string/array/object emptiness, including through pointers.
				omit := false
				for _, option := range tags[1:] {
					if option == "omitzero" && item.IsZero() {
						omit = true
					}
					if option == "omitempty" {
						encoded := item
						for (encoded.Kind() == reflect.Interface || encoded.Kind() == reflect.Pointer) && !encoded.IsNil() {
							encoded = encoded.Elem()
						}
						switch encoded.Kind() {
						case reflect.Interface, reflect.Pointer:
							omit = true
						case reflect.String, reflect.Slice, reflect.Array, reflect.Map:
							if encoded.Len() == 0 {
								omit = true
							}
						}
					}
				}
				if omit {
					continue
				}
				if fields > 0 && !add(1) {
					return false
				}
				fields++
				if !quoted(tags[0]) || !add(1) || !walk(item) {
					return false
				}
			}
			return true
		}
		return false
	}
	if !walk(reflect.ValueOf(packet)) {
		return arrayFactsError("limit", "complete output bytes before serialization", nil)
	}
	return nil
}
