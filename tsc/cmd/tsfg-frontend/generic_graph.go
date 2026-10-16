// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Owned scalar definitions contain no compiler objects or executable closures.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/json"
	"github.com/microsoft/typescript-go/internal/scanner"
)

const genericSemantics = "tsfg-owned-scalar-generics-host-v1"
const genericDeclarationProfile = "minimal-scalars-v2"
const genericFileName = "/owned/definitions.ts"

type genericOrigin struct {
	Start int
	End   int
}

type genericSource struct {
	Path         string
	Digest       string
	PositionUnit string // Original-source anchors are UTF-16 code units.
	Length       int
}

type genericType struct {
	Kind      string // boolean, number, or parameter
	Parameter int
}

type genericParameter struct {
	Name   string
	Type   genericType
	Origin genericOrigin
}

type genericExpression struct {
	Kind          string // parameter or call
	Parameter     int
	Definition    string
	TypeArguments []genericType
	Arguments     []genericExpression
	Origin        genericOrigin
}

type genericDefinition struct {
	Key            string
	Name           string
	Exported       bool
	TypeParameters []string
	Parameters     []genericParameter
	Return         genericType
	Body           genericExpression
	Origin         genericOrigin
}

type genericDeclaration struct {
	Path    string
	Digest  string
	Content string
}

type genericDeclarationEnvironment struct {
	Profile string
	Files   []genericDeclaration
}

type genericGraph struct {
	Package     string
	Semantics   string
	Source      genericSource
	Environment genericDeclarationEnvironment
	Definitions []genericDefinition
	Digest      string
}

type genericFailure struct {
	Kind   string
	Source genericSource
	Code   int32 // Official diagnostic code; zero for the owned boundary.
	Origin genericOrigin
	Detail string
}

func (e *genericFailure) Error() string {
	return fmt.Sprintf("owned generic %s (%d): %s", e.Kind, e.Code, e.Detail)
}

func genericError(kind, detail string, origin genericOrigin) error {
	return &genericFailure{Kind: kind, Detail: detail, Origin: origin}
}

func genericHash(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err) // All callers supply bounded, acyclic owned value structs.
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func genericTextDigest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func genericGraphDigest(g genericGraph) string {
	g.Digest = ""
	return genericHash(g)
}

func genericDefinitionKey(g genericGraph, d genericDefinition) string {
	return genericHash(struct {
		Package string
		Source  genericSource
		Name    string
		Origin  genericOrigin
	}{g.Package, g.Source, d.Name, d.Origin})
}

func genericIdentifier(name string) bool {
	return name != "" && scanner.IsIdentifierText(name, core.LanguageVariantStandard) &&
		!ast.IsNonContextualKeyword(scanner.StringToToken(name))
}

func genericValidType(t genericType, count int) bool {
	switch t.Kind {
	case "boolean", "number":
		return t.Parameter == 0
	case "parameter":
		return t.Parameter >= 0 && t.Parameter < count
	}
	return false
}

func (g genericGraph) validate() error {
	bad := func(detail string) error { return genericError("input", detail, genericOrigin{}) }
	if g.Package == "" || len(g.Package) > 256 || g.Semantics != genericSemantics ||
		g.Source.Path != canonicalProjectPath(g.Source.Path) || !strings.HasSuffix(g.Source.Path, ".ts") ||
		!strings.HasPrefix(g.Source.Path, "/") || strings.HasSuffix(g.Source.Path, ".d.ts") ||
		g.Source.PositionUnit != "utf16" || g.Source.Length <= 0 || g.Source.Length > 1<<20 || !genericDigest(g.Source.Digest) ||
		len(g.Definitions) == 0 || len(g.Definitions) > 256 {
		return bad("graph identity or metadata")
	}
	if err := g.Environment.validate(); err != nil {
		return err
	}
	origins := func(o genericOrigin) bool { return o.Start >= 0 && o.End > o.Start && o.End <= g.Source.Length }
	defs := map[string]genericDefinition{}
	names := map[string]bool{}
	for _, d := range g.Definitions {
		if !genericIdentifier(d.Name) || names[d.Name] || !origins(d.Origin) ||
			d.Key != genericDefinitionKey(g, d) || len(d.TypeParameters) == 0 ||
			len(d.TypeParameters) > 16 || len(d.Parameters) > 64 || !genericValidType(d.Return, len(d.TypeParameters)) {
			return bad("definition metadata")
		}
		names[d.Name] = true
		defs[d.Key] = d
		typeNames := map[string]bool{}
		for _, name := range d.TypeParameters {
			if !genericIdentifier(name) || typeNames[name] {
				return bad("type parameter identity")
			}
			typeNames[name] = true
		}
		valueNames := map[string]bool{}
		for _, p := range d.Parameters {
			if !genericIdentifier(p.Name) || valueNames[p.Name] || !origins(p.Origin) ||
				p.Origin.Start < d.Origin.Start || p.Origin.End > d.Origin.End ||
				!genericValidType(p.Type, len(d.TypeParameters)) {
				return bad("parameter metadata")
			}
			valueNames[p.Name] = true
		}
	}
	for _, d := range g.Definitions {
		nodes := 0
		var visit func(genericExpression, int) error
		visit = func(e genericExpression, depth int) error {
			nodes++
			if depth > 64 || nodes > 1024 || !origins(e.Origin) ||
				e.Origin.Start < d.Origin.Start || e.Origin.End > d.Origin.End {
				return bad("expression origin or size")
			}
			switch e.Kind {
			case "parameter":
				if e.Parameter < 0 || e.Parameter >= len(d.Parameters) || e.Definition != "" ||
					len(e.TypeArguments) != 0 || len(e.Arguments) != 0 {
					return bad("parameter reference")
				}
			case "call":
				target, ok := defs[e.Definition]
				if !ok || e.Parameter != 0 || len(e.TypeArguments) != len(target.TypeParameters) || len(e.Arguments) != len(target.Parameters) {
					return bad("dangling definition or call arity")
				}
				for _, p := range d.Parameters {
					if p.Name == target.Name {
						return bad("call target shadowed by parameter")
					}
				}
				for _, a := range e.TypeArguments {
					if !genericValidType(a, len(d.TypeParameters)) {
						return bad("call type argument")
					}
				}
				for _, a := range e.Arguments {
					if err := visit(a, depth+1); err != nil {
						return err
					}
				}
			default:
				return genericError("unsupported", "expression kind", e.Origin)
			}
			return nil
		}
		if err := visit(d.Body, 0); err != nil {
			return err
		}
	}
	if g.Digest != genericGraphDigest(g) {
		return bad("graph digest")
	}
	return nil
}

func genericDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}

func (e genericDeclarationEnvironment) validate() error {
	if e.Profile != genericDeclarationProfile || len(e.Files) == 0 || len(e.Files) > 8 {
		return genericError("input", "declaration environment profile/count", genericOrigin{})
	}
	seen := map[string]bool{}
	for _, f := range e.Files {
		if !strings.HasPrefix(f.Path, "/") || f.Path != canonicalProjectPath(f.Path) || !strings.HasSuffix(f.Path, ".d.ts") ||
			seen[f.Path] || len(f.Content) == 0 || len(f.Content) > 1<<20 || f.Digest != genericTextDigest(f.Content) {
			return genericError("input", "declaration environment path/content identity", genericOrigin{})
		}
		seen[f.Path] = true
	}
	return nil
}
