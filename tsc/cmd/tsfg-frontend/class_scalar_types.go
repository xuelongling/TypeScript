// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Owned scalar meaning depends on official identity, not alias spelling.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

type classScalarNominal struct {
	Source  string      `json:"source"`
	SHA256  string      `json:"sha256"`
	Version string      `json:"version"`
	Alias   classAnchor `json:"alias"`
	Brand   classAnchor `json:"brand"`
}

func (p *classProjection) admitClassScalars(files []*ast.SourceFile) error {
	for _, source := range p.graph.Sources {
		if !source.Declaration || source.SHA256 != conversionDeclarationHash {
			continue
		}
		if p.scalars != nil {
			return classError("input", "duplicate canonical scalar declaration role", classAnchor{Source: source.Path})
		}
		facts, failure := admitScalarDeclarations(p.program, p.check, p.program.GetSourceFile(source.Path), files[0].FileName())
		if failure != nil {
			return classScalarFailure(failure)
		}
		p.scalars = facts
	}
	types := map[*ast.Symbol]string{}
	if p.scalars != nil {
		types = p.scalars.Types
	}
	for _, file := range files {
		if err := p.ctx.Err(); err != nil {
			return err
		}
		if failure := verifyScalarNominals(file, p.check, types); failure != nil {
			return classScalarFailure(failure)
		}
	}
	return nil
}

func classScalarFailure(failure *scalarAdmissionFailure) error {
	origin := classAnchor{Source: failure.Path}
	if failure.Node != nil {
		origin = classNodeAnchor(failure.Node)
	}
	return classError("input", failure.Code, origin)
}

func (p *classProjection) admittedScalarRef(value *checker.Type, node *ast.Node) (string, bool, error) {
	if p.scalars == nil || value == nil || value.Alias() == nil {
		return "", false, nil
	}
	symbol := value.Alias().Symbol()
	kind := p.scalars.Types[symbol]
	if kind == "" {
		return "", false, nil
	}
	if kind != "i32" && kind != "f32" {
		return "", true, classError("unsupported", "admitted class scalar kind not yet projected: "+kind, classNodeAnchor(node))
	}
	id := "t:" + kind
	if p.scalarKinds[id] == "" {
		brand := p.scalars.Brands[kind]
		if len(symbol.Declarations) != 1 || brand == nil {
			return "", true, classError("identity", "admitted scalar alias/brand provenance missing", classNodeAnchor(node))
		}
		p.graph.Types = append(p.graph.Types, classType{ID: id, Kind: "scalar", Scalar: kind,
			Nominal: &classScalarNominal{Source: p.scalars.File.FileName(), SHA256: conversionDeclarationHash,
				Version: conversionVersion, Alias: classNodeAnchor(symbol.Declarations[0]), Brand: classNodeAnchor(brand)}})
		p.scalarKinds[id] = kind
	}
	p.types[value] = id
	return id, true, nil
}
