// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Conversions allocate independent wire/domain arrays and preserve ordered binders.
package main

import (
	"slices"
	"strings"
)

func genericPackageOrigin(o genericOrigin) genericPackageOriginDTO {
	return genericPackageOriginDTO{Start: o.Start, End: o.End}
}

func genericPackageOwnedOrigin(o genericPackageOriginDTO) genericOrigin {
	return genericOrigin{Start: o.Start, End: o.End}
}

func genericPackageSource(s genericSource) genericPackageSourceDTO {
	return genericPackageSourceDTO{Digest: s.Digest, Length: s.Length, Path: s.Path,
		PositionUnit: s.PositionUnit, SurrogateStarts: append([]int{}, s.SurrogateStarts...)}
}

// Both conversions allocate every child so protocol and domain ownership stay
// independent. Callers have already applied the shared structural budget.
func genericPackageType(t genericType) genericPackageTypeDTO {
	out := genericPackageTypeDTO{Kind: t.Kind, Parameter: t.Parameter}
	for _, child := range []struct {
		in  *genericType
		out **genericPackageTypeDTO
	}{{t.CheckType, &out.CheckType}, {t.ExtendsType, &out.ExtendsType}, {t.TrueType, &out.TrueType}, {t.FalseType, &out.FalseType}} {
		if child.in != nil {
			value := genericPackageType(*child.in)
			*child.out = &value
		}
	}
	if t.Mapped != nil {
		out.Mapped = &genericPackageMappedTypeDTO{Binder: t.Mapped.Binder,
			Keys:  genericPackageKeyTypeDTO{Kind: t.Mapped.Keys.Kind, Literals: append([]string{}, t.Mapped.Keys.Literals...)},
			Value: genericPackageType(t.Mapped.Value)}
	}
	if t.Indexed != nil {
		out.Indexed = &genericPackageIndexedTypeDTO{Object: genericPackageType(t.Indexed.Object),
			Index: genericPackageKeyTypeDTO{Kind: t.Indexed.Index.Kind, Literals: append([]string{}, t.Indexed.Index.Literals...)}}
	}
	return out
}

func genericPackageOwnedType(t genericPackageTypeDTO) genericType {
	out := genericType{Kind: t.Kind, Parameter: t.Parameter}
	for _, child := range []struct {
		in  *genericPackageTypeDTO
		out **genericType
	}{{t.CheckType, &out.CheckType}, {t.ExtendsType, &out.ExtendsType}, {t.TrueType, &out.TrueType}, {t.FalseType, &out.FalseType}} {
		if child.in != nil {
			value := genericPackageOwnedType(*child.in)
			*child.out = &value
		}
	}
	if t.Mapped != nil {
		out.Mapped = &genericMappedType{Binder: t.Mapped.Binder,
			Keys:  genericKeyType{Kind: t.Mapped.Keys.Kind, Literals: slices.Clone(t.Mapped.Keys.Literals)},
			Value: genericPackageOwnedType(t.Mapped.Value)}
	}
	if t.Indexed != nil {
		out.Indexed = &genericIndexedType{Object: genericPackageOwnedType(t.Indexed.Object),
			Index: genericKeyType{Kind: t.Indexed.Index.Kind, Literals: slices.Clone(t.Indexed.Index.Literals)}}
	}
	return out
}

func genericPackageTypes(types []genericType) []genericPackageTypeDTO {
	result := make([]genericPackageTypeDTO, len(types))
	for i, t := range types {
		result[i] = genericPackageType(t)
	}
	return result
}

func genericPackageExpression(e genericExpression) genericPackageExpressionDTO {
	result := genericPackageExpressionDTO{Kind: e.Kind, Parameter: e.Parameter, Definition: e.Definition,
		Origin: genericPackageOrigin(e.Origin), TypeArguments: genericPackageTypes(e.TypeArguments),
		Arguments: make([]genericPackageExpressionDTO, len(e.Arguments))}
	for i, a := range e.Arguments {
		result.Arguments[i] = genericPackageExpression(a)
	}
	return result
}

func genericPackagePayload(g genericGraph) genericPackagePayloadDTO {
	p := genericPackagePayloadDTO{Scope: g.Package, SemanticProfile: g.Semantics,
		Capabilities: genericPackageCapabilities(),
		Source:       genericPackageSource(g.Source), Definitions: make([]genericPackageDefinitionDTO, len(g.Definitions)),
		Environment: genericPackageEnvironmentDTO{Profile: g.Environment.Profile,
			Files: make([]genericPackageDeclarationDTO, len(g.Environment.Files))}}
	for i, f := range g.Environment.Files {
		p.Environment.Files[i] = genericPackageDeclarationDTO{Path: f.Path, Digest: f.Digest, Content: f.Content}
	}
	slices.SortFunc(p.Environment.Files, func(a, b genericPackageDeclarationDTO) int { return genericPackageCompareUTF16(a.Path, b.Path) })
	for i, d := range g.Definitions {
		p.Definitions[i] = genericPackageDefinitionDTO{Key: d.Key, Name: d.Name, Exported: d.Exported,
			Origin: genericPackageOrigin(d.Origin), TypeParameters: append([]string{}, d.TypeParameters...),
			ReturnType: genericPackageType(d.Return), Body: genericPackageExpression(d.Body),
			Parameters: make([]genericPackageParameterDTO, len(d.Parameters))}
		for j, parameter := range d.Parameters {
			p.Definitions[i].Parameters[j] = genericPackageParameterDTO{Name: parameter.Name,
				Origin: genericPackageOrigin(parameter.Origin), Type: genericPackageType(parameter.Type)}
		}
	}
	slices.SortFunc(p.Definitions, func(a, b genericPackageDefinitionDTO) int { return strings.Compare(a.Key, b.Key) })
	return p
}

func genericPackageOwnedExpression(e genericPackageExpressionDTO) genericExpression {
	result := genericExpression{Kind: e.Kind, Parameter: e.Parameter, Definition: e.Definition,
		Origin: genericPackageOwnedOrigin(e.Origin), TypeArguments: make([]genericType, len(e.TypeArguments)),
		Arguments: make([]genericExpression, len(e.Arguments))}
	for i, t := range e.TypeArguments {
		result.TypeArguments[i] = genericPackageOwnedType(t)
	}
	for i, a := range e.Arguments {
		result.Arguments[i] = genericPackageOwnedExpression(a)
	}
	return result
}

func genericPackageOwnedGraph(p genericPackagePayloadDTO) genericGraph {
	g := genericGraph{Package: p.Scope, Semantics: p.SemanticProfile,
		Source: genericSource{Path: p.Source.Path, Digest: p.Source.Digest, Length: p.Source.Length,
			PositionUnit: p.Source.PositionUnit, SurrogateStarts: slices.Clone(p.Source.SurrogateStarts)},
		Environment: genericDeclarationEnvironment{Profile: p.Environment.Profile, Files: make([]genericDeclaration, len(p.Environment.Files))},
		Definitions: make([]genericDefinition, len(p.Definitions))}
	for i, f := range p.Environment.Files {
		g.Environment.Files[i] = genericDeclaration{Path: f.Path, Digest: f.Digest, Content: f.Content}
	}
	for i, d := range p.Definitions {
		g.Definitions[i] = genericDefinition{Key: d.Key, Name: d.Name, Exported: d.Exported,
			Origin: genericPackageOwnedOrigin(d.Origin), TypeParameters: slices.Clone(d.TypeParameters),
			Return: genericPackageOwnedType(d.ReturnType), Body: genericPackageOwnedExpression(d.Body),
			Parameters: make([]genericParameter, len(d.Parameters))}
		for j, parameter := range d.Parameters {
			g.Definitions[i].Parameters[j] = genericParameter{Name: parameter.Name, Origin: genericPackageOwnedOrigin(parameter.Origin),
				Type: genericPackageOwnedType(parameter.Type)}
		}
	}
	g.Digest = genericPackageIdentity(genericPackageDomain, p)
	return g
}
