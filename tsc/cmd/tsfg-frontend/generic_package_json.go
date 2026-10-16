// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Canonicalization uses the pinned official jsontext JCS implementation through
// the repository wrapper. No Go declaration ordering enters an identity.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/microsoft/typescript-go/internal/json"
)

func genericPackageError(category, detail string) error {
	return genericError("package-"+category, detail, genericOrigin{})
}

func genericPackageCanonical(value any) ([]byte, error) {
	return genericJSONCanonical(value, genericPackageJSONPolicy())
}

func genericJSONCanonical(value any, policy genericJSONPolicy) ([]byte, error) {
	// internal/json intentionally permits invalid UTF8 when marshaling. Validate
	// DTO scalars first so no replacement character can silently change identity.
	if err := (&genericCanonicalBudget{policy: policy}).walk(reflect.ValueOf(value), 0); err != nil {
		return nil, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, genericPackageError("json", "DTO encoding")
	}
	raw := json.Value(data)
	if err := raw.Canonicalize(); err != nil {
		return nil, genericPackageError("json", "JCS encoding")
	}
	return []byte(raw), nil
}

// The same bounded walk counts canonical wire bytes before Marshal, hashing or
// domain-to-DTO copies. Owned graph fields use explicit wire names and omit the
// internal digest; the injected capabilities are part of that payload view.
type genericJSONPolicy struct {
	maxBytes, maxDepth, maxTokens, maxString int
	allowNUL, allowNull, allowRaw            bool
}

func genericPackageJSONPolicy() genericJSONPolicy {
	return genericJSONPolicy{maxBytes: genericPackageMaxBytes, maxDepth: genericPackageMaxDepth,
		maxTokens: genericPackageMaxTokens, maxString: genericPackageMaxString}
}

type genericCanonicalBudget struct {
	bytes, tokens int
	policy        genericJSONPolicy
}

func (b *genericCanonicalBudget) add(bytes, tokens int) error {
	b.bytes += bytes
	b.tokens += tokens
	if b.bytes > b.policy.maxBytes {
		return genericPackageError("limit", "canonical bytes")
	}
	if b.tokens > b.policy.maxTokens {
		return genericPackageError("limit", "JSON tokens")
	}
	return nil
}

func (b *genericCanonicalBudget) text(text string) error {
	if !utf8.ValidString(text) || !b.policy.allowNUL && strings.ContainsRune(text, 0) {
		return genericPackageError("unicode", "invalid Unicode scalar string or NUL")
	}
	if len(text) > b.policy.maxString {
		return genericPackageError("limit", "string bytes")
	}
	size := 2
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '"', '\\', '\b', '\t', '\n', '\f', '\r':
			size += 2
		default:
			if text[i] < 0x20 {
				size += 6
			} else {
				size++
			}
		}
	}
	return b.add(size, 1)
}

func genericPackageFieldName(typ reflect.Type, field reflect.StructField) (string, error) {
	if tag := field.Tag.Get("json"); tag != "" {
		return tag, nil
	}
	if typ == reflect.TypeFor[genericGraph]() {
		switch field.Name {
		case "Digest":
			return "", nil
		case "Package":
			return "scope", nil
		case "Semantics":
			return "semanticProfile", nil
		}
	}
	switch field.Name {
	case "Source":
		return "source", nil
	case "Environment":
		return "environment", nil
	case "Definitions":
		return "definitions", nil
	case "Name":
		return "name", nil
	case "Path":
		return "path", nil
	case "Digest":
		return "digest", nil
	case "PositionUnit":
		return "positionUnit", nil
	case "Length":
		return "length", nil
	case "SurrogateStarts":
		return "surrogateStarts", nil
	case "Start":
		return "start", nil
	case "End":
		return "end", nil
	case "Profile":
		return "profile", nil
	case "Files":
		return "files", nil
	case "Content":
		return "content", nil
	case "Key":
		return "key", nil
	case "Exported":
		return "exported", nil
	case "TypeParameters":
		return "typeParameters", nil
	case "Parameters":
		return "parameters", nil
	case "Return":
		return "returnType", nil
	case "Body":
		return "body", nil
	case "Origin":
		return "origin", nil
	case "Type":
		return "type", nil
	case "Kind":
		return "kind", nil
	case "Parameter":
		return "parameter", nil
	case "Definition":
		return "definition", nil
	case "TypeArguments":
		return "typeArguments", nil
	case "Arguments":
		return "arguments", nil
	}
	return "", genericPackageError("schema", "unsupported budget field")
}

func (b *genericCanonicalBudget) member(name string, value reflect.Value, depth int, comma bool) error {
	if comma {
		if err := b.add(1, 0); err != nil {
			return err
		}
	}
	if err := b.text(name); err != nil {
		return err
	}
	if err := b.add(1, 0); err != nil {
		return err
	}
	return b.walk(value, depth)
}

func (b *genericCanonicalBudget) walk(v reflect.Value, depth int) error {
	if v.Type() == reflect.TypeFor[json.Value]() && b.policy.allowRaw {
		data := v.Bytes()
		stats, err := inspectGenericJSON(data, b.policy)
		if err != nil {
			return err
		}
		if depth+stats.depth > b.policy.maxDepth {
			return genericPackageError("limit", "JSON depth")
		}
		canonical := json.Value(bytes.Clone(data))
		if err := canonical.Canonicalize(); err != nil || !bytes.Equal(canonical, data) {
			return genericPackageError("canonical", "noncanonical raw value")
		}
		return b.add(len(data), stats.tokens)
	}
	switch v.Kind() {
	case reflect.Pointer:
		if !b.policy.allowNull {
			return genericPackageError("json", "unsupported DTO scalar")
		}
		if v.IsNil() {
			return b.add(4, 1)
		}
		return b.walk(v.Elem(), depth)
	case reflect.String:
		return b.text(v.String())
	case reflect.Int, reflect.Int32, reflect.Int64:
		if v.Int() < 0 || v.Int() > 9007199254740991 {
			return genericPackageError("json", "nonnegative safe integer required")
		}
		return b.add(len(strconv.FormatInt(v.Int(), 10)), 1)
	case reflect.Bool:
		size := 5
		if v.Bool() {
			size = 4
		}
		return b.add(size, 1)
	case reflect.Struct:
		if depth+1 > b.policy.maxDepth {
			return genericPackageError("limit", "JSON depth")
		}
		if err := b.add(2, 2); err != nil {
			return err
		}
		emitted := false
		if v.Type() == reflect.TypeFor[genericGraph]() {
			caps := []string{"explicit-type-arguments", "scalar-generics", "single-module"}
			if err := b.member("capabilities", reflect.ValueOf(caps), depth+1, false); err != nil {
				return err
			}
			emitted = true
		}
		for i := 0; i < v.NumField(); i++ {
			name, err := genericPackageFieldName(v.Type(), v.Type().Field(i))
			if err != nil {
				return err
			}
			if name == "" {
				continue
			}
			if err := b.member(name, v.Field(i), depth+1, emitted); err != nil {
				return err
			}
			emitted = true
		}
		return nil
	case reflect.Slice, reflect.Array:
		if depth+1 > b.policy.maxDepth {
			return genericPackageError("limit", "JSON depth")
		}
		if err := b.add(2, 2); err != nil {
			return err
		}
		if v.Len() > 0 {
			if err := b.add(v.Len()-1, 0); err != nil {
				return err
			}
		}
		for i := 0; i < v.Len(); i++ {
			if err := b.walk(v.Index(i), depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	return genericPackageError("json", "unsupported DTO scalar")
}

func genericPackageBudget(value any) error {
	return (&genericCanonicalBudget{policy: genericPackageJSONPolicy()}).walk(reflect.ValueOf(value), 0)
}

func genericPackageIdentity(domain string, value any) string {
	data, err := genericPackageCanonical(value)
	if err != nil {
		panic(err)
	} // Private identities are only derived from validated owned values.
	hash := sha256.New()
	hash.Write([]byte(domain))
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

// Preflight is streaming and precedes typed allocation. Decoder defaults enforce
// RFC7493 Unicode and escape-equivalent duplicate-key rejection. Tokens include
// object/array opens and closes, member keys, and primitive values.
type genericJSONStats struct{ tokens, depth int }

func genericJSONPreflight(data []byte, policy genericJSONPolicy) error {
	_, err := inspectGenericJSON(data, policy)
	return err
}

func inspectGenericJSON(data []byte, policy genericJSONPolicy) (genericJSONStats, error) {
	stats := genericJSONStats{}
	bad := func(category, detail string) (genericJSONStats, error) {
		return stats, genericPackageError(category, detail)
	}
	if len(data) == 0 || len(data) > policy.maxBytes {
		return bad("limit", "package bytes")
	}
	if !utf8.Valid(data) {
		return bad("unicode", "invalid UTF8")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	depth := 0
	for {
		token, err := decoder.ReadToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return bad("json", "strict JSON token")
		}
		stats.tokens++
		if stats.tokens > policy.maxTokens {
			return bad("limit", "JSON tokens")
		}
		switch token.Kind() {
		case '{', '[':
			depth++
			if depth > stats.depth {
				stats.depth = depth
			}
			if depth > policy.maxDepth {
				return bad("limit", "JSON depth")
			}
		case '}', ']':
			depth--
		case '"':
			s := token.String()
			if !utf8.ValidString(s) || !policy.allowNUL && strings.ContainsRune(s, 0) {
				return bad("unicode", "invalid Unicode scalar string or NUL")
			}
			if len(s) > policy.maxString {
				return bad("limit", "string bytes")
			}
		case '0':
			n, err := token.Int()
			if err != nil || n < 0 || n > 9007199254740991 {
				return bad("json", "nonnegative safe integer required")
			}
		case 'n':
			if !policy.allowNull {
				return bad("schema", "null is not a candidate DTO value")
			}
		}
	}
	return stats, nil
}

func genericPackageCompareUTF16(a, b string) int {
	left, right := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(left) && i < len(right); i++ {
		if left[i] < right[i] {
			return -1
		}
		if left[i] > right[i] {
			return 1
		}
	}
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	return 0
}
