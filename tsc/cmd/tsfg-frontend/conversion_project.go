// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/sha256"
	"fmt"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/tspath"
)

type conversionOrigin struct {
	DeclarationPath   string            `json:"declarationPath"`
	DeclarationRange  span              `json:"declarationRange"`
	DeclarationSHA256 string            `json:"declarationSha256"`
	EntryID           string            `json:"entryId"`
	Version           string            `json:"version"`
	BrandRefs         []conversionBrand `json:"brandRefs"`
}
type conversionBrand struct {
	Role    string `json:"role"`
	Nominal string `json:"nominal"`
	Range   span   `json:"range"`
}
type conversionFacts struct {
	Functions map[*ast.Symbol]functionInfo
	Types     map[*ast.Symbol]string
	Brands    map[string]span
}

// The request adapter preserves its existing protocol and declaration role.
func admittedConversions(r request, program *compiler.Program, check *checker.Checker) (*conversionFacts, *response) {
	path := tspath.CombinePaths(r.SourceRoot, "minimal-conversions-v5.d.ts")
	content := ""
	for _, input := range r.Files {
		if input.Path == path {
			content = input.Content
		}
	}
	if fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(content))) != conversionDeclarationHash {
		failure := fail("r02b-d-input", "input", path, span{})
		return nil, &failure
	}
	shared, failure := admitScalarDeclarations(program, check, program.GetSourceFile(path), r.RootFile)
	if failure != nil {
		if failure.Path == "" && failure.Node == nil {
			failure.Path = path
		}
		return nil, conversionAdmissionFailure(failure)
	}
	facts := &conversionFacts{Types: shared.Types, Brands: map[string]span{}, Functions: map[*ast.Symbol]functionInfo{}}
	for kind, node := range shared.Brands {
		facts.Brands[kind] = expressionOffset(shared.File, node)
	}
	for symbol, entry := range shared.Entries {
		id := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("tsfg.conversion\x00"+conversionVersion+"\x00"+entry.Mode+"\x00"+entry.From+"\x00"+entry.To+"\x00"+conversionDeclarationHash)))
		refs := []conversionBrand{}
		for index, kind := range []string{entry.From, entry.To} {
			if kind == "f64" {
				continue
			}
			role := "source"
			if index == 1 {
				role = "target"
			}
			refs = append(refs, conversionBrand{Role: role, Nominal: kind, Range: facts.Brands[kind]})
		}
		facts.Functions[symbol] = functionInfo{Signature: signature{Parameters: []string{entry.From}, ReturnType: entry.To},
			Declaration: entry.Declaration, Conversion: &conversionOrigin{
				DeclarationPath: path, DeclarationRange: expressionOffset(shared.File, entry.Declaration),
				DeclarationSHA256: conversionDeclarationHash, EntryID: id, Version: conversionVersion, BrandRefs: refs,
			}}
	}
	return facts, nil
}

func conversionAdmissionFailure(failure *scalarAdmissionFailure) *response {
	var result response
	if failure.Node != nil {
		result = failAt(failure.Code, "error", failure.File, failure.Node)
	} else {
		result = fail(failure.Code, "input", failure.Path, span{})
	}
	return &result
}

func verifyConversionNominals(file *ast.SourceFile, check *checker.Checker, facts *conversionFacts) *response {
	if failure := verifyScalarNominals(file, check, facts.Types); failure != nil {
		return conversionAdmissionFailure(failure)
	}
	return nil
}
