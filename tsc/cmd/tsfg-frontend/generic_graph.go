// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Owned scalar definitions contain no compiler objects or executable closures.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/core"
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
	Path            string
	Digest          string
	PositionUnit    string // Original-source anchors are UTF-16 code units.
	Length          int
	SurrogateStarts []int
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

func genericTextDigest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func genericGraphDigest(g genericGraph) string {
	return genericPackageIdentity(genericPackageDomain, genericPackagePayload(g))
}

func genericDefinitionKey(g genericGraph, d genericDefinition) string {
	return genericPackageIdentity(genericDefinitionDomain, genericPackageDefinitionIdentityDTO{
		Name: d.Name, Origin: genericPackageOrigin(d.Origin), Scope: g.Package, Source: genericPackageSource(g.Source)})
}

func cloneGenericSource(source genericSource) genericSource {
	source.SurrogateStarts = slices.Clone(source.SurrogateStarts)
	return source
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
	if err := genericPackageBudget(g); err != nil {
		return err
	}
	bad := func(detail string) error { return genericError("input", detail, genericOrigin{}) }
	if g.Package == "" || len(g.Package) > 256 || !genericPackageString(g.Package) || g.Semantics != genericSemantics ||
		!genericPackagePath(g.Source.Path) || !strings.HasSuffix(g.Source.Path, ".ts") ||
		!strings.HasPrefix(g.Source.Path, "/") || strings.HasSuffix(g.Source.Path, ".d.ts") ||
		g.Source.PositionUnit != "utf16" || g.Source.Length <= 0 || g.Source.Length > 1<<20 ||
		len(g.Definitions) == 0 || len(g.Definitions) > 256 || !genericPackageSourceBoundaries(g.Source) {
		return bad("graph metadata")
	}
	if !genericDigest(g.Source.Digest) {
		return genericPackageError("identity", "source digest")
	}
	if err := g.Environment.validate(); err != nil {
		return err
	}
	origins := func(o genericOrigin) bool { return genericPackageOriginBoundary(g.Source, o) }
	defs := map[string]genericDefinition{}
	names := map[string]bool{}
	for _, d := range g.Definitions {
		if !genericIdentifier(d.Name) || !genericPackageString(d.Name) || names[d.Name] || !origins(d.Origin) ||
			len(d.TypeParameters) == 0 || len(d.TypeParameters) > 16 || len(d.Parameters) > 64 {
			return bad("definition metadata")
		}
		if d.Key != genericDefinitionKey(g, d) {
			return genericError("package-identity", "definition key", d.Origin)
		}
		if err := validateGenericTypeReference(d.Return, len(d.TypeParameters), d.Origin); err != nil {
			return err
		}
		names[d.Name] = true
		defs[d.Key] = d
		typeNames := map[string]bool{}
		for _, name := range d.TypeParameters {
			if !genericIdentifier(name) || !genericPackageString(name) || typeNames[name] {
				return bad("type parameter identity")
			}
			typeNames[name] = true
		}
		valueNames := map[string]bool{}
		for _, p := range d.Parameters {
			if !genericIdentifier(p.Name) || !genericPackageString(p.Name) || valueNames[p.Name] || !origins(p.Origin) ||
				p.Origin.Start < d.Origin.Start || p.Origin.End > d.Origin.End {
				return bad("parameter metadata")
			}
			if err := validateGenericTypeReference(p.Type, len(d.TypeParameters), p.Origin); err != nil {
				return err
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
				if e.Definition != "" || len(e.TypeArguments) != 0 || len(e.Arguments) != 0 {
					return bad("parameter expression metadata")
				}
				if e.Parameter < 0 || e.Parameter >= len(d.Parameters) {
					return genericError("package-reference", "parameter reference", e.Origin)
				}
			case "call":
				if e.Parameter != 0 {
					return bad("call expression metadata")
				}
				target, ok := defs[e.Definition]
				if !ok || len(e.TypeArguments) != len(target.TypeParameters) || len(e.Arguments) != len(target.Parameters) {
					return genericError("package-reference", "dangling definition or call arity", e.Origin)
				}
				for _, p := range d.Parameters {
					if p.Name == target.Name {
						return genericError("package-reference", "call target shadowed by parameter", e.Origin)
					}
				}
				for _, a := range e.TypeArguments {
					if err := validateGenericTypeReference(a, len(d.TypeParameters), e.Origin); err != nil {
						return err
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
		return genericPackageError("identity", "graph digest")
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
		if !genericPackagePath(f.Path) || !strings.HasSuffix(f.Path, ".d.ts") ||
			seen[f.Path] || len(f.Content) == 0 || !genericPackageString(f.Content) {
			return genericError("input", "declaration environment path/content metadata", genericOrigin{})
		}
		if f.Digest != genericTextDigest(f.Content) {
			return genericPackageError("identity", "declaration content digest")
		}
		seen[f.Path] = true
	}
	return nil
}

// Binder resolution is distinct from the representation of a type node.
func validateGenericTypeReference(t genericType, count int, origin genericOrigin) error {
	if genericValidType(t, count) {
		return nil
	}
	if t.Kind == "parameter" {
		return genericError("package-reference", "type parameter reference", origin)
	}
	return genericError("input", "type metadata", origin)
}
