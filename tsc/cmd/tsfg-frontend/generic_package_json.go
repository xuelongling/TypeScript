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
	// internal/json intentionally permits invalid UTF8 when marshaling. Validate
	// DTO scalars first so no replacement character can silently change identity.
	if err := genericPackageBudget(value); err != nil {
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
type genericCanonicalBudget struct{ bytes, tokens int }

func (b *genericCanonicalBudget) add(bytes, tokens int) error {
	b.bytes += bytes
	b.tokens += tokens
	if b.bytes > genericPackageMaxBytes {
		return genericPackageError("limit", "canonical bytes")
	}
	if b.tokens > genericPackageMaxTokens {
		return genericPackageError("limit", "JSON tokens")
	}
	return nil
}

func (b *genericCanonicalBudget) text(text string) error {
	if !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
		return genericPackageError("unicode", "invalid Unicode scalar string or NUL")
	}
	if len(text) > genericPackageMaxString {
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
	switch v.Kind() {
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
		if depth+1 > genericPackageMaxDepth {
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
		if depth+1 > genericPackageMaxDepth {
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
	return (&genericCanonicalBudget{}).walk(reflect.ValueOf(value), 0)
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
func genericPackagePreflight(data []byte) error {
	if len(data) == 0 || len(data) > genericPackageMaxBytes {
		return genericPackageError("limit", "package bytes")
	}
	if !utf8.Valid(data) {
		return genericPackageError("unicode", "invalid UTF8")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	depth, tokens := 0, 0
	for {
		token, err := decoder.ReadToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return genericPackageError("json", "strict JSON token")
		}
		tokens++
		if tokens > genericPackageMaxTokens {
			return genericPackageError("limit", "JSON tokens")
		}
		switch token.Kind() {
		case '{', '[':
			depth++
			if depth > genericPackageMaxDepth {
				return genericPackageError("limit", "JSON depth")
			}
		case '}', ']':
			depth--
		case '"':
			s := token.String()
			if !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
				return genericPackageError("unicode", "invalid Unicode scalar string or NUL")
			}
			if len(s) > genericPackageMaxString {
				return genericPackageError("limit", "string bytes")
			}
		case '0':
			n, err := token.Int()
			if err != nil || n < 0 || n > 9007199254740991 {
				return genericPackageError("json", "nonnegative safe integer required")
			}
		case 'n':
			return genericPackageError("schema", "null is not a candidate DTO value")
		}
	}
	return nil
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
