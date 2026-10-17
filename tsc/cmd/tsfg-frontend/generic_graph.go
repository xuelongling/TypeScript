// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Owned scalar definitions contain no compiler objects or executable closures.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/scanner"
)

const genericSemantics = "tsfg-owned-scalar-generics-host-v5"
const genericDeclarationProfile = "minimal-scalars-v2"

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
	Kind        string // scalar leaf, direct conditional, or mapped/indexed signature
	Parameter   int
	CheckType   *genericType        `json:"checkType,omitempty"`
	ExtendsType *genericType        `json:"extendsType,omitempty"`
	TrueType    *genericType        `json:"trueType,omitempty"`
	FalseType   *genericType        `json:"falseType,omitempty"`
	Mapped      *genericMappedType  `json:"mapped,omitempty"`
	Indexed     *genericIndexedType `json:"indexed,omitempty"`
}

type genericKeyType struct {
	Kind     string   `json:"kind"`
	Literals []string `json:"literals"`
}

type genericMappedType struct {
	Binder string         `json:"binder"`
	Keys   genericKeyType `json:"keys"`
	Value  genericType    `json:"value"`
}

type genericIndexedType struct {
	Object genericType    `json:"object"`
	Index  genericKeyType `json:"index"`
}

type genericParameter struct {
	Name   string
	Type   genericType
	Origin genericOrigin
}

type genericExpression struct {
	Kind          string // parameter, call, or root number-add
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
	if err := genericPackageBudget(g); err != nil {
		panic(err)
	} // Bound wrappers before recursive domain-to-DTO copies.
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

func genericTypeLeaf(t genericType) bool {
	return genericConditionalFieldsEmpty(t) && t.Mapped == nil && t.Indexed == nil
}

func genericConditionalFieldsEmpty(t genericType) bool {
	return t.CheckType == nil && t.ExtendsType == nil && t.TrueType == nil && t.FalseType == nil
}

func genericValidType(t genericType, count int) bool {
	if t.Kind == "indexed" {
		return validateGenericIndexedType(t, count, genericOrigin{}) == nil
	}
	if t.Kind == "conditional" {
		return t.Parameter == 0 && t.Mapped == nil && t.Indexed == nil && t.CheckType != nil && t.ExtendsType != nil && t.TrueType != nil && t.FalseType != nil &&
			t.CheckType.Kind == "parameter" && genericValidType(*t.CheckType, count) && genericTypeLeaf(*t.CheckType) &&
			*t.ExtendsType == (genericType{Kind: "boolean"}) && *t.TrueType == (genericType{Kind: "boolean"}) &&
			*t.FalseType == (genericType{Kind: "number"})
	}
	if !genericTypeLeaf(t) {
		return false
	}
	switch t.Kind {
	case "boolean", "number":
		return t.Parameter == 0
	case "parameter":
		return t.Parameter >= 0 && t.Parameter < count
	}
	return false
}

// Key membership is a wire role check only; result typing remains official.
// Count raw union occurrences, preserving duplicates and source order.
func validateGenericKeyType(kind string, literals []string, origin genericOrigin) error {
	if len(literals) > genericPackageMaxKeyMembers {
		return genericError("package-limit", "key union members", origin)
	}
	if kind != "string-literal" && kind != "union" || kind == "string-literal" && len(literals) != 1 ||
		kind == "union" && len(literals) < 2 {
		return genericError("input", "literal/union key metadata", origin)
	}
	for _, literal := range literals {
		if len(literal) > genericPackageMaxKeyBytes {
			return genericError("package-limit", "key bytes", origin)
		}
		if !genericPackageString(literal) {
			return genericError("input", "key Unicode or NUL", origin)
		}
	}
	return nil
}

func validateGenericIndexedType(t genericType, count int, origin genericOrigin) error {
	bad := func() error { return genericError("input", "mapped/indexed signature metadata", origin) }
	if count != 1 || t.Parameter != 0 || !genericConditionalFieldsEmpty(t) || t.Mapped != nil || t.Indexed == nil {
		return bad()
	}
	object := t.Indexed.Object
	if object.Kind != "mapped" || object.Parameter != 0 || !genericConditionalFieldsEmpty(object) || object.Indexed != nil || object.Mapped == nil {
		return bad()
	}
	mapped := object.Mapped
	if !genericIdentifier(mapped.Binder) || !genericPackageString(mapped.Binder) ||
		!genericTypeLeaf(mapped.Value) || !genericValidType(mapped.Value, count) {
		return bad()
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

// The conditional extension is one hop only. Matching complete owned types
// preserves the official syntax; scalar instantiation remains the checker's job.
func genericOneHopConditionalCall(caller, callee genericDefinition) bool {
	call := caller.Body
	if call.Kind != "call" || call.Parameter != 0 || len(call.TypeArguments) != 1 ||
		call.TypeArguments[0] != (genericType{Kind: "parameter"}) || len(call.Arguments) != 1 ||
		len(callee.TypeParameters) != 1 || len(callee.Parameters) != 1 ||
		!reflect.DeepEqual(callee.Return, caller.Return) || !reflect.DeepEqual(callee.Parameters[0].Type, caller.Return) {
		return false
	}
	argument, body := call.Arguments[0], callee.Body
	return argument.Kind == "parameter" && argument.Parameter == 0 && argument.Definition == "" &&
		len(argument.TypeArguments) == 0 && len(argument.Arguments) == 0 &&
		body.Kind == "parameter" && body.Parameter == 0 && body.Definition == "" &&
		len(body.TypeArguments) == 0 && len(body.Arguments) == 0
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
	if err := validateGenericPackageFileSet(g.Source.Path, g.Environment.Files); err != nil {
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
		conditional, indexed := d.Return.Kind == "conditional", d.Return.Kind == "indexed"
		for _, p := range d.Parameters {
			conditional = conditional || p.Type.Kind == "conditional"
			indexed = indexed || p.Type.Kind == "indexed"
		}
		if conditional || indexed {
			kind, detail := "indexed", "mapped/indexed signature requires one type parameter and one directly returned parameter with the identical indexed type"
			if conditional {
				kind, detail = "conditional", "conditional signature requires one type parameter, one value parameter and identical conditional types with a direct parameter return or one-hop generic call"
			}
			if len(d.TypeParameters) != 1 || len(d.Parameters) != 1 || d.Return.Kind != kind ||
				!reflect.DeepEqual(d.Parameters[0].Type, d.Return) ||
				!(d.Body.Kind == "parameter" && d.Body.Parameter == 0 || conditional && d.Body.Kind == "call") {
				return genericError("unsupported", detail, d.Origin)
			}
			if indexed && d.Return.Indexed.Object.Mapped.Binder == d.TypeParameters[0] {
				return genericError("unsupported", "mapped binder shadows outer type parameter", d.Origin)
			}
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
			case "number-add":
				if depth != 0 || e.Parameter != 0 || e.Definition != "" || len(e.TypeArguments) != 0 ||
					len(e.Arguments) != 2 || d.Return != (genericType{Kind: "number"}) {
					return bad("number-add root metadata or return type")
				}
				previous := e.Origin.Start
				for _, a := range e.Arguments {
					if a.Kind != "parameter" || a.Parameter < 0 || a.Parameter >= len(d.Parameters) ||
						d.Parameters[a.Parameter].Type != (genericType{Kind: "number"}) {
						return genericError("package-reference", "number-add requires declared number parameters", a.Origin)
					}
					if a.Origin.Start < previous || a.Origin.End > e.Origin.End {
						return genericError("input", "number-add operand order or containment", a.Origin)
					}
					if err := visit(a, depth+1); err != nil {
						return err
					}
					previous = a.Origin.End
				}
			case "call":
				if e.Parameter != 0 {
					return bad("call expression metadata")
				}
				target, ok := defs[e.Definition]
				if !ok || len(e.TypeArguments) != len(target.TypeParameters) || len(e.Arguments) != len(target.Parameters) {
					return genericError("package-reference", "dangling definition or call arity", e.Origin)
				}
				if d.Return.Kind == "conditional" && (depth != 0 || !genericOneHopConditionalCall(d, target)) {
					return genericError("unsupported", "conditional generic call requires bound parameter0 type/value arguments and a matching direct-return conditional callee", d.Origin)
				}
				for _, p := range d.Parameters {
					if p.Name == target.Name {
						return genericError("package-reference", "call target shadowed by parameter", e.Origin)
					}
				}
				for _, a := range e.TypeArguments {
					if !genericTypeLeaf(a) {
						if a.Kind == "conditional" {
							return genericError("unsupported", "conditional type arguments are outside the signature slice", e.Origin)
						}
						return genericError("unsupported", "mapped/indexed type arguments are outside the signature slice", e.Origin)
					}
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
	return validateGenericPackageFileSet("", e.Files)
}

// Binder resolution is distinct from the representation of a type node.
func validateGenericTypeReference(t genericType, count int, origin genericOrigin) error {
	if t.Kind == "indexed" {
		return validateGenericIndexedType(t, count, origin)
	}
	if genericValidType(t, count) {
		return nil
	}
	if t.Kind == "parameter" || t.Kind == "conditional" && t.CheckType != nil &&
		t.CheckType.Kind == "parameter" && (t.CheckType.Parameter < 0 || t.CheckType.Parameter >= count) {
		return genericError("package-reference", "type parameter reference", origin)
	}
	return genericError("input", "type metadata", origin)
}
