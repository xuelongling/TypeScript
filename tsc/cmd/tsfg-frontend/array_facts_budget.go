// SPDX-License-Identifier: Apache-2.0
// One trusted DTO walk bounds its actual JSONv2 shape before encoding or hashing.
package main

import (
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

type arrayFactsJSONField struct {
	index               int
	name                string
	omitEmpty, omitZero bool
}
type arrayFactsJSONBudget struct {
	policy        genericJSONPolicy
	bytes, tokens int
	types         map[reflect.Type]bool
	fields        map[reflect.Type][]arrayFactsJSONField
}

func arrayFactsOutputBudget(packet arrayFactsPackage) error {
	// Framing affects bytes, but a single LF is not a JSON token or container.
	return arrayFactsDTOBudget(packet, 1)
}
func arrayFactsDTOBudget(value any, framing int) error {
	if framing != 0 && framing != 1 {
		return arrayFactsError("json", "invalid DTO framing", nil)
	}
	budget := arrayFactsJSONBudget{policy: arrayFactsPolicy(true), bytes: framing,
		types: map[reflect.Type]bool{}, fields: map[reflect.Type][]arrayFactsJSONField{}}
	actual := reflect.ValueOf(value)
	if !actual.IsValid() {
		return arrayFactsError("json", "unsupported DTO scalar", nil)
	}
	if err := budget.trusted(actual.Type()); err != nil {
		return err
	}
	return budget.walk(actual, 0)
}
func (b *arrayFactsJSONBudget) add(bytes, tokens int) error {
	if bytes > b.policy.maxBytes-b.bytes {
		return arrayFactsError("limit", "canonical bytes", nil)
	}
	if tokens > b.policy.maxTokens-b.tokens {
		return arrayFactsError("limit", "JSON tokens", nil)
	}
	b.bytes += bytes
	b.tokens += tokens
	return nil
}
func (b *arrayFactsJSONBudget) text(text string) error {
	if !utf8.ValidString(text) || !b.policy.allowNUL && strings.ContainsRune(text, 0) {
		return arrayFactsError("unicode", "invalid Unicode scalar string or NUL", nil)
	}
	if len(text) > b.policy.maxString {
		return arrayFactsError("limit", "string bytes", nil)
	}
	size := 2
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '"', '\\', '\b', '\t', '\n', '\f', '\r':
			size += 2
		default:
			if text[i] < 32 {
				size += 6
			} else {
				size++
			}
		}
	}
	return b.add(size, 1)
}

// The producer supplies only fixed, tagged structs and scalar/slice children.
// Reject method-bearing or otherwise unsupported types even when a field would
// be omitted. In particular, never invoke IsZero/MarshalJSON or a guest callback.
func (b *arrayFactsJSONBudget) trusted(typ reflect.Type) error {
	if b.types[typ] {
		return nil
	}
	if typ.NumMethod() != 0 || reflect.PointerTo(typ).NumMethod() != 0 {
		return arrayFactsError("json", "custom DTO methods are unsupported", nil)
	}
	b.types[typ] = true
	switch typ.Kind() {
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int32, reflect.Int64:
		return nil
	case reflect.Pointer:
		// The selected DTO has single pointers to scalar/struct/slice values.
		// Reject a recursive pointer-only type that has no JSON container depth.
		if typ.Elem().Kind() == reflect.Pointer {
			return arrayFactsError("json", "unsupported DTO pointer chain", nil)
		}
		return b.trusted(typ.Elem())
	case reflect.Slice, reflect.Array:
		return b.trusted(typ.Elem())
	case reflect.Struct:
		names := map[string]bool{}
		fields := []arrayFactsJSONField{}
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.PkgPath != "" || field.Anonymous {
				return arrayFactsError("json", "explicit exported DTO fields required", nil)
			}
			if err := b.trusted(field.Type); err != nil {
				return err
			}
			tag, tagged := field.Tag.Lookup("json")
			if !tagged {
				return arrayFactsError("json", "explicit DTO JSON name required", nil)
			}
			parts := strings.Split(tag, ",")
			if parts[0] == "-" && len(parts) == 1 {
				continue
			}
			if parts[0] == "" || names[parts[0]] || !utf8.ValidString(parts[0]) {
				return arrayFactsError("json", "unique explicit DTO JSON names required", nil)
			}
			item := arrayFactsJSONField{index: i, name: parts[0]}
			for _, option := range parts[1:] {
				switch option {
				case "omitempty":
					item.omitEmpty = true
				case "omitzero":
					item.omitZero = true
				default:
					return arrayFactsError("json", "unsupported DTO JSON option", nil)
				}
			}
			names[item.name] = true
			fields = append(fields, item)
		}
		b.fields[typ] = fields
		return nil
	default:
		return arrayFactsError("json", "unsupported DTO scalar", nil)
	}
}

func (b *arrayFactsJSONBudget) omitted(field arrayFactsJSONField, value reflect.Value, depth int) (bool, error) {
	// Test the original pointer/slice value, so nonnil pointers to empty argument
	// or element slices remain present under omitzero and encode as required [].
	if field.omitZero && value.IsZero() {
		return true, nil
	}
	if !field.omitEmpty {
		return false, nil
	}
	return b.empty(value, depth)
}
func (b *arrayFactsJSONBudget) empty(value reflect.Value, depth int) (bool, error) {
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return true, nil
		}
		return b.empty(value.Elem(), depth)
	}
	switch value.Kind() {
	case reflect.String, reflect.Slice, reflect.Array:
		return value.Len() == 0, nil
	case reflect.Struct:
		fields := b.fields[value.Type()]
		// Required members make the JSON object nonempty, even for false or zero.
		for _, field := range fields {
			if !field.omitEmpty && !field.omitZero {
				return false, nil
			}
		}
		if len(fields) == 0 {
			return true, nil
		}
		if depth+1 > b.policy.maxDepth {
			return false, arrayFactsError("limit", "JSON depth", nil)
		}
		for _, field := range fields {
			omit, err := b.omitted(field, value.Field(field.index), depth+1)
			if err != nil || !omit {
				return false, err
			}
		}
		return true, nil
	default:
		// JSONv2 omitempty keeps boolean false and numeric zero.
		return false, nil
	}
}
func (b *arrayFactsJSONBudget) walk(value reflect.Value, depth int) error {
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return b.add(4, 1)
		}
		return b.walk(value.Elem(), depth)
	}
	switch value.Kind() {
	case reflect.String:
		return b.text(value.String())
	case reflect.Bool:
		if value.Bool() {
			return b.add(4, 1)
		}
		return b.add(5, 1)
	case reflect.Int, reflect.Int32, reflect.Int64:
		integer := value.Int()
		if integer < 0 || integer > 9007199254740991 {
			return arrayFactsError("json", "nonnegative safe integer required", nil)
		}
		return b.add(len(strconv.FormatInt(integer, 10)), 1)
	case reflect.Slice, reflect.Array:
		if depth+1 > b.policy.maxDepth {
			return arrayFactsError("limit", "JSON depth", nil)
		}
		// JSONv2's default nil slice encoding is [], never null.
		if err := b.add(2, 2); err != nil {
			return err
		}
		for i := 0; i < value.Len(); i++ {
			if i > 0 {
				if err := b.add(1, 0); err != nil {
					return err
				}
			}
			if err := b.walk(value.Index(i), depth+1); err != nil {
				return err
			}
		}
		return nil
	case reflect.Struct:
		if depth+1 > b.policy.maxDepth {
			return arrayFactsError("limit", "JSON depth", nil)
		}
		if err := b.add(2, 2); err != nil {
			return err
		}
		emitted := false
		for _, field := range b.fields[value.Type()] {
			item := value.Field(field.index)
			omit, err := b.omitted(field, item, depth+1)
			if err != nil {
				return err
			}
			if omit {
				continue
			}
			if emitted {
				if err := b.add(1, 0); err != nil {
					return err
				}
			}
			emitted = true
			if err := b.text(field.name); err != nil {
				return err
			}
			if err := b.add(1, 0); err != nil {
				return err
			}
			if err := b.walk(item, depth+1); err != nil {
				return err
			}
		}
		return nil
	default:
		return arrayFactsError("json", "unsupported DTO scalar", nil)
	}
}
