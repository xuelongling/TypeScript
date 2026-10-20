// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Shared compiler-local admission; adapters own their independent output models.
package main

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
)

const (
	conversionVersion         = "minimal-conversions-v5"
	conversionDeclarationHash = "sha256:60974c1fa82cc549da3a9268f5d82ab10b58f52e2612a377ee4577f05af52c86"
)

type scalarAdmissionFailure struct {
	Code string
	Path string
	File *ast.SourceFile
	Node *ast.Node
}

type scalarAdmissionEntry struct {
	Declaration *ast.Node
	Mode        string
	From        string
	To          string
}

type scalarAdmissionFacts struct {
	File    *ast.SourceFile
	Types   map[*ast.Symbol]string
	Brands  map[string]*ast.Node
	Entries map[*ast.Symbol]scalarAdmissionEntry
}

func scalarAdmissionAt(code string, file *ast.SourceFile, node *ast.Node) *scalarAdmissionFailure {
	return &scalarAdmissionFailure{Code: code, File: file, Node: node}
}

// Exact bytes establish the declaration role; official symbols establish every
// alias, unique brand and intrinsic entry. A name alone never establishes either.
func admitScalarDeclarations(program *compiler.Program, check *checker.Checker, file *ast.SourceFile, rootFile string) (*scalarAdmissionFacts, *scalarAdmissionFailure) {
	if file == nil || program.GetSourceFile(file.FileName()) != file || !file.IsDeclarationFile ||
		fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(file.Text()))) != conversionDeclarationHash {
		path := ""
		if file != nil {
			path = file.FileName()
		}
		return nil, &scalarAdmissionFailure{Code: "r02b-d-input", Path: path}
	}
	facts := &scalarAdmissionFacts{File: file, Types: map[*ast.Symbol]string{}, Brands: map[string]*ast.Node{},
		Entries: map[*ast.Symbol]scalarAdmissionEntry{}}
	for _, node := range file.Statements.Nodes {
		if node.Kind == ast.KindTypeAliasDeclaration && node.Name() != nil {
			name := node.Name().Text()
			if integerKind(name) || floatKind(name) {
				symbol := check.GetSymbolAtLocation(node.Name())
				if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != node {
					return nil, scalarAdmissionAt("r01-d-conversion-entry", file, node)
				}
				facts.Types[symbol] = name
			}
		}
	}
	var failure *scalarAdmissionFailure
	var brands func(*ast.Node) bool
	brands = func(node *ast.Node) bool {
		if node.Kind == ast.KindIdentifier && strings.HasPrefix(node.Text(), "__tsfg_") {
			symbol := check.GetSymbolAtLocation(node)
			if symbol == nil || len(symbol.Declarations) != 1 || ast.GetSourceFileOfNode(symbol.Declarations[0]) != file ||
				check.GetTypeOfSymbol(symbol).Flags()&checker.TypeFlagsUniqueESSymbol == 0 {
				failure = scalarAdmissionAt("r01-d-conversion-entry", file, node)
				return true
			}
			if symbol.Declarations[0].Name() == node {
				facts.Brands[strings.TrimPrefix(node.Text(), "__tsfg_")] = node
			}
		}
		return node.ForEachChild(brands)
	}
	file.AsNode().ForEachChild(brands)
	if failure != nil {
		return nil, failure
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
			return nil, scalarAdmissionAt("r01-d-conversion-entry", file, node)
		}
		entryCount++
		// A real user body keeps ordinary function status, as before. Foreign
		// ambient merging never gains intrinsic conversion ownership.
		if len(symbol.Declarations) == 2 {
			var implementation *ast.Node
			for _, other := range symbol.Declarations {
				if other != node && other.Kind == ast.KindFunctionDeclaration &&
					ast.GetSourceFileOfNode(other).FileName() == rootFile && other.AsFunctionDeclaration().Body != nil {
					implementation = other
				}
			}
			if implementation != nil {
				continue
			}
		}
		signatures := check.GetSignaturesOfType(check.GetTypeOfSymbol(symbol), checker.SignatureKindCall)
		if len(symbol.Declarations) != 1 || symbol.Declarations[0] != node || len(signatures) != 1 ||
			signatures[0].Declaration() != node || decl.Body != nil || decl.TypeParameters != nil ||
			len(decl.Parameters.Nodes) != 1 || len(signatures[0].Parameters()) != 1 ||
			scalar(check.GetTypeOfSymbol(signatures[0].Parameters()[0])) != from ||
			scalar(check.GetReturnTypeOfSignature(signatures[0])) != to {
			return nil, scalarAdmissionAt("r01-d-conversion-entry", file, node)
		}
		for _, nominal := range []string{from, to} {
			if nominal != "f64" && facts.Brands[nominal] == nil {
				return nil, scalarAdmissionAt("r01-d-conversion-entry", file, node)
			}
		}
		facts.Entries[symbol] = scalarAdmissionEntry{Declaration: node, Mode: mode, From: from, To: to}
	}
	if entryCount != 286 || len(facts.Types) != 12 || len(facts.Brands) != 11 {
		return nil, &scalarAdmissionFailure{Code: "r02b-d-input", Path: file.FileName()}
	}
	return facts, nil
}

// Annotation ownership is shared by the old conversion adapter and class facts.
// A compiler-local failure is converted to primitive diagnostics by each caller.
func verifyScalarNominals(file *ast.SourceFile, check *checker.Checker, types map[*ast.Symbol]string) *scalarAdmissionFailure {
	var failure *scalarAdmissionFailure
	var visit func(*ast.Node) bool
	visit = func(node *ast.Node) bool {
		if node.Kind == ast.KindTypeReference {
			reference := node.AsTypeReferenceNode()
			value := check.GetTypeFromTypeNode(node)
			kind := scalar(value)
			if kind == "f64" && value.Flags()&checker.TypeFlagsNumber == 0 {
				failure = scalarAdmissionAt("r01-d-conversion-type", file, node)
				return true
			}
			if integerKind(kind) || kind == "f32" {
				symbol := check.GetSymbolAtLocation(reference.TypeName)
				if types[symbol] != kind || value.Alias() == nil || types[value.Alias().Symbol()] != kind {
					failure = scalarAdmissionAt("r01-d-conversion-type", file, node)
					return true
				}
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
	return failure
}
