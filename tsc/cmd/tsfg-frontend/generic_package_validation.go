// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Structural budgets and Unicode/path checks precede owned graph interpretation.
package main

import (
	"slices"
	"strings"
	"unicode/utf8"
)

func genericPackageString(text string) bool {
	return len(text) <= genericPackageMaxString && utf8.ValidString(text) && !strings.ContainsRune(text, 0)
}

func genericPackagePath(path string) bool {
	return len(path) <= 4096 && genericPackageString(path) && strings.HasPrefix(path, "/") &&
		!strings.Contains(path, "\\") && path == canonicalProjectPath(path) && !strings.HasSuffix(path, "/")
}

func genericPackageSourceBoundaries(source genericSource) bool {
	if len(source.SurrogateStarts) > source.Length/2 {
		return false
	}
	previous := -2
	for _, start := range source.SurrogateStarts {
		if start < previous+2 || start < 0 || start > source.Length-2 {
			return false
		}
		previous = start
	}
	return true
}

func genericPackageOriginBoundary(source genericSource, origin genericOrigin) bool {
	if origin.Start < 0 || origin.End <= origin.Start || origin.End > source.Length {
		return false
	}
	for _, endpoint := range []int{origin.Start, origin.End} {
		if _, split := slices.BinarySearch(source.SurrogateStarts, endpoint-1); split {
			return false
		}
	}
	return true
}

// Validate signature structure before conversion; root/call arguments retain
// their independent leaf-only role even when a compound resolves to a scalar.
func validateGenericPackageType(t genericPackageTypeDTO, count int, signature bool, origin genericOrigin) error {
	leaf := t.CheckType == nil && t.ExtendsType == nil && t.TrueType == nil && t.FalseType == nil && t.Mapped == nil && t.Indexed == nil
	if t.Kind == "indexed" {
		if !signature {
			return genericError("unsupported", "mapped/indexed type arguments are outside the signature slice", origin)
		}
		return validateGenericPackageIndexedType(t, count, origin)
	}
	if t.Kind == "conditional" {
		if !signature || t.Parameter != 0 || t.Mapped != nil || t.Indexed != nil || t.CheckType == nil || t.ExtendsType == nil || t.TrueType == nil || t.FalseType == nil ||
			t.CheckType.Kind != "parameter" || t.ExtendsType.Kind != "boolean" || t.TrueType.Kind != "boolean" || t.FalseType.Kind != "number" {
			return genericError("input", "direct conditional signature metadata", origin)
		}
		for _, child := range []*genericPackageTypeDTO{t.CheckType, t.ExtendsType, t.TrueType, t.FalseType} {
			if err := validateGenericPackageType(*child, count, false, origin); err != nil {
				return err
			}
		}
		return nil
	}
	if !leaf {
		return genericError("input", "scalar/type-parameter leaf has compound fields", origin)
	}
	return validateGenericTypeReference(genericType{Kind: t.Kind, Parameter: t.Parameter}, count, origin)
}

// This validates wire children directly, before allocating owned wrappers.
func validateGenericPackageIndexedType(t genericPackageTypeDTO, count int, origin genericOrigin) error {
	bad := func() error { return genericError("input", "mapped/indexed signature metadata", origin) }
	if count != 1 || t.Parameter != 0 || t.CheckType != nil || t.ExtendsType != nil || t.TrueType != nil || t.FalseType != nil ||
		t.Mapped != nil || t.Indexed == nil {
		return bad()
	}
	object := t.Indexed.Object
	if object.Kind != "mapped" || object.Parameter != 0 || object.CheckType != nil || object.ExtendsType != nil ||
		object.TrueType != nil || object.FalseType != nil || object.Indexed != nil || object.Mapped == nil {
		return bad()
	}
	mapped := object.Mapped
	if !genericIdentifier(mapped.Binder) || !genericPackageString(mapped.Binder) {
		return bad()
	}
	if err := validateGenericPackageType(mapped.Value, count, false, origin); err != nil {
		return err
	}
	if err := validateGenericKeyType(mapped.Keys.Kind, mapped.Keys.Literals, origin); err != nil {
		return err
	}
	if err := validateGenericKeyType(t.Indexed.Index.Kind, t.Indexed.Index.Literals, origin); err != nil {
		return err
	}
	for _, index := range t.Indexed.Index.Literals {
		if !slices.Contains(mapped.Keys.Literals, index) {
			return genericError("package-reference", "index literal outside mapped key domain", origin)
		}
	}
	return nil
}

func validateGenericPackagePayload(p genericPackagePayloadDTO) error {
	if !slices.Equal(p.Capabilities, genericPackageCapabilities()) ||
		p.SemanticProfile != genericSemantics || p.Environment.Profile != genericDeclarationProfile {
		return genericPackageError("schema", "capability or semantic/declaration profile")
	}
	if len(p.Definitions) == 0 || len(p.Definitions) > 256 || len(p.Environment.Files) == 0 || len(p.Environment.Files) > 8 {
		return genericPackageError("limit", "definition/declaration count")
	}
	for i, d := range p.Environment.Files {
		if i > 0 && genericPackageCompareUTF16(p.Environment.Files[i-1].Path, d.Path) >= 0 {
			return genericPackageError("canonical", "declaration ordering")
		}
	}
	total := 0
	for i, d := range p.Definitions {
		if i > 0 && p.Definitions[i-1].Key >= d.Key {
			return genericPackageError("canonical", "definition ordering")
		}
		if len(d.TypeParameters) == 0 || len(d.TypeParameters) > 16 || len(d.Parameters) > 64 {
			return genericPackageError("limit", "binder count")
		}
		if err := validateGenericPackageType(d.ReturnType, len(d.TypeParameters), true, genericPackageOwnedOrigin(d.Origin)); err != nil {
			return err
		}
		for _, parameter := range d.Parameters {
			if err := validateGenericPackageType(parameter.Type, len(d.TypeParameters), true, genericPackageOwnedOrigin(parameter.Origin)); err != nil {
				return err
			}
		}
		if d.ReturnType.Kind == "indexed" && d.ReturnType.Indexed.Object.Mapped.Binder == d.TypeParameters[0] {
			return genericError("unsupported", "mapped binder shadows outer type parameter", genericPackageOwnedOrigin(d.Origin))
		}
		count := 0
		var visit func(genericPackageExpressionDTO, int) error
		visit = func(e genericPackageExpressionDTO, depth int) error {
			count++
			total++
			if depth > 64 || count > 1024 || total > genericPackageMaxExpressions || len(e.Arguments) > 64 || len(e.TypeArguments) > 16 {
				return genericPackageError("limit", "expression depth/count/arity")
			}
			for _, argument := range e.TypeArguments {
				if argument.Kind == "conditional" {
					return genericError("unsupported", "conditional type arguments are outside the signature slice", genericPackageOwnedOrigin(e.Origin))
				}
				if err := validateGenericPackageType(argument, len(d.TypeParameters), false, genericPackageOwnedOrigin(e.Origin)); err != nil {
					return err
				}
			}
			for _, a := range e.Arguments {
				if err := visit(a, depth+1); err != nil {
					return err
				}
			}
			return nil
		}
		if err := visit(d.Body, 0); err != nil {
			return err
		}
	}
	return nil
}

// Explicit files must never also be a directory in the in-memory VFS.
func validateGenericPackageFileSet(source string, files []genericDeclaration) error {
	paths := make([]string, 0, len(files)+1)
	if source != "" {
		paths = append(paths, source)
	}
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	for i, left := range paths {
		for _, right := range paths[i+1:] {
			if genericPackageFilePathConflict(left, right) {
				return genericPackageError("input", "VFS file/directory conflict")
			}
		}
	}
	return nil
}

func genericPackageFilePathConflict(left, right string) bool {
	return left == right || strings.HasPrefix(left, right+"/") || strings.HasPrefix(right, left+"/")
}
