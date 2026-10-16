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

func validateGenericPackagePayload(p genericPackagePayloadDTO) error {
	if !slices.Equal(p.Capabilities, []string{"explicit-type-arguments", "scalar-generics", "single-module"}) ||
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
		count := 0
		var visit func(genericPackageExpressionDTO, int) error
		visit = func(e genericPackageExpressionDTO, depth int) error {
			count++
			total++
			if depth > 64 || count > 1024 || total > genericPackageMaxExpressions || len(e.Arguments) > 64 || len(e.TypeArguments) > 16 {
				return genericPackageError("limit", "expression depth/count/arity")
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
