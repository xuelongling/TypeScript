// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/sha256"
	"fmt"
	"path"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
)

const conversionVersion = "minimal-conversions-v5"
const conversionDeclarationHash = "sha256:60974c1fa82cc549da3a9268f5d82ab10b58f52e2612a377ee4577f05af52c86"

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

// Admission is checked against the exact source bytes and official declaration
// and brand symbols. Text is a usability label, never proof of ownership.
func admittedConversions(r request, program *compiler.Program, check *checker.Checker) (*conversionFacts, *response) {
	declarationPath := path.Join(r.SourceRoot, "minimal-conversions-v5.d.ts")
	content := ""
	for _, input := range r.Files {
		if input.Path == declarationPath {
			content = input.Content
		}
	}
	file := program.GetSourceFile(declarationPath)
	if file == nil || fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(content))) != conversionDeclarationHash {
		failure := fail("r02b-d-input", "input", declarationPath, span{})
		return nil, &failure
	}
	facts := &conversionFacts{Functions: map[*ast.Symbol]functionInfo{}, Types: map[*ast.Symbol]string{}, Brands: map[string]span{}}
	for _, node := range file.Statements.Nodes {
		if node.Kind == ast.KindTypeAliasDeclaration && node.Name() != nil {
			name := node.Name().Text()
			if integerKind(name) || floatKind(name) {
				symbol := check.GetSymbolAtLocation(node.Name())
				if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != node {
					failure := failAt("r01-d-conversion-entry", "error", file, node)
					return nil, &failure
				}
				facts.Types[symbol] = name
			}
		}
	}
	var brandFailure *response
	var brands func(*ast.Node) bool
	brands = func(node *ast.Node) bool {
		if node.Kind == ast.KindIdentifier && strings.HasPrefix(node.Text(), "__tsfg_") {
			symbol := check.GetSymbolAtLocation(node)
			if symbol == nil || len(symbol.Declarations) != 1 || ast.GetSourceFileOfNode(symbol.Declarations[0]) != file || check.GetTypeOfSymbol(symbol).Flags()&checker.TypeFlagsUniqueESSymbol == 0 {
				failure := failAt("r01-d-conversion-entry", "error", file, node)
				brandFailure = &failure
				return true
			}
			if symbol.Declarations[0].Name() == node {
				facts.Brands[strings.TrimPrefix(node.Text(), "__tsfg_")] = expressionOffset(file, node)
			}
		}
		return node.ForEachChild(brands)
	}
	file.AsNode().ForEachChild(brands)
	if brandFailure != nil {
		return nil, brandFailure
	}
	entryCount := 0
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Name() == nil {
			continue
		}
		decl := node.AsFunctionDeclaration()
		parts := strings.Split(node.Name().Text(), "_")
		if len(parts) != 4 || parts[2] != "from" {
			continue
		}
		mode, to, from := parts[0], parts[1], parts[3]
		symbol := check.GetSymbolAtLocation(node.Name())
		if symbol == nil {
			failure := failAt("r01-d-conversion-entry", "error", file, node)
			return nil, &failure
		}
		entryCount++
		// A valid global user implementation can share the ambient symbol. It
		// owns an ordinary function body and never gains intrinsic conversion
		// status. Unrelated ambient/foreign declaration merging remains invalid.
		if len(symbol.Declarations) == 2 {
			var implementation *ast.Node
			for _, other := range symbol.Declarations {
				if other != node && other.Kind == ast.KindFunctionDeclaration &&
					ast.GetSourceFileOfNode(other).FileName() == r.RootFile && other.AsFunctionDeclaration().Body != nil {
					implementation = other
				}
			}
			if implementation != nil {
				continue
			}
		}
		signatures := check.GetSignaturesOfType(check.GetTypeOfSymbol(symbol), checker.SignatureKindCall)
		if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != node || len(signatures) != 1 || signatures[0].Declaration() != node || decl.Body != nil || decl.TypeParameters != nil || len(decl.Parameters.Nodes) != 1 || len(signatures[0].Parameters()) != 1 || scalar(check.GetTypeOfSymbol(signatures[0].Parameters()[0])) != from || scalar(check.GetReturnTypeOfSignature(signatures[0])) != to {
			failure := failAt("r01-d-conversion-entry", "error", file, node)
			return nil, &failure
		}
		id := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("tsfg.conversion\x00"+conversionVersion+"\x00"+mode+"\x00"+from+"\x00"+to+"\x00"+conversionDeclarationHash)))
		// Exclude surrounding trivia from the owned admitted declaration range.
		location := expressionOffset(file, node)
		refs := []conversionBrand{}
		for index, nominal := range []string{from, to} {
			if nominal == "f64" {
				continue
			}
			brandRange, exists := facts.Brands[nominal]
			if !exists {
				failure := failAt("r01-d-conversion-entry", "error", file, node)
				return nil, &failure
			}
			role := "source"
			if index == 1 {
				role = "target"
			}
			refs = append(refs, conversionBrand{Role: role, Nominal: nominal, Range: brandRange})
		}
		facts.Functions[symbol] = functionInfo{Signature: signature{Parameters: []string{from}, ReturnType: to}, Declaration: node, Conversion: &conversionOrigin{DeclarationPath: declarationPath, DeclarationRange: location, DeclarationSHA256: conversionDeclarationHash, EntryID: id, Version: conversionVersion, BrandRefs: refs}}
	}
	if entryCount != 286 || len(facts.Types) != 12 || len(facts.Brands) != 11 {
		failure := fail("r02b-d-input", "input", declarationPath, span{})
		return nil, &failure
	}
	return facts, nil
}

// Numeric annotations from foreign same-spelling aliases must not be projected
// as admitted nominal types. Primitive number remains the specified f64 alias.
func verifyConversionNominals(file *ast.SourceFile, check *checker.Checker, facts *conversionFacts) *response {
	var failure *response
	var visit func(*ast.Node) bool
	visit = func(node *ast.Node) bool {
		if node.Kind == ast.KindTypeReference {
			reference := node.AsTypeReferenceNode()
			t := check.GetTypeFromTypeNode(node)
			kind := scalar(t)
			if integerKind(kind) || kind == "f32" {
				symbol := check.GetSymbolAtLocation(reference.TypeName)
				if facts.Types[symbol] != kind || t.Alias() == nil || facts.Types[t.Alias().Symbol()] != kind {
					result := failAt("r01-d-conversion-type", "error", file, node)
					failure = &result
					return true
				}
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
	return failure
}
