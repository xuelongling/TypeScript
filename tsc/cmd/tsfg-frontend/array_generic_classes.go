// SPDX-License-Identifier: Apache-2.0
// Reuses M11 nominal/marker and M12 source-anchor facts without runtime layout.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"strings"
)

func (p *arrayGenericProjection) classHeaders(nodes []*ast.Node) error {
	for _, node := range nodes {
		if node.Name() == nil || node.AsClassDeclaration().TypeParameters != nil {
			return p.fail(node, "closed original nominal class required")
		}
		symbol := p.check.GetSymbolAtLocation(node.Name())
		if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != node {
			return p.fail(node, "unique original class symbol required")
		}
		actual := p.check.GetDeclaredTypeOfSymbol(symbol)
		if actual == nil || !actual.IsClass() {
			return p.fail(node, "original class instance proof required")
		}
		kind, category := "reference", "reference"
		var origin *markerOrigin
		if node.Modifiers() != nil {
			for _, modifier := range node.Modifiers().Nodes {
				if modifier.Kind != ast.KindDecorator {
					continue
				}
				expr := modifier.AsDecorator().Expression
				resolved := p.check.GetSymbolAtLocation(expr)
				if origin != nil || expr.Kind != ast.KindIdentifier || resolved == nil || len(resolved.Declarations) != 1 || ast.GetSourceFileOfNode(resolved.Declarations[0]) != p.dts {
					return p.fail(modifier, "pinned standard class marker symbol required")
				}
				name := resolved.Name
				if name == "tsfgValue" {
					kind, category = "value", "value"
				} else if name == "tsfgComponent" {
					category = "component"
				} else {
					return p.fail(modifier, "unknown class marker")
				}
				decl := resolved.Declarations[0]
				id, err := genericNodeID("tsfg.array-facts.marker.v8\x00", decl)
				if err != nil {
					return err
				}
				origin = &markerOrigin{DeclarationPath: p.dts.FileName(), DeclarationRange: *genericSpan(decl), DeclarationSHA256: "sha256:" + arrayFactsSHA(arrayGenericDeclarations), EntryID: id, Version: "minimal-arrays-v8", ContextName: "ClassDecoratorContext", ContextArity: 1, Mode: "standard", Range: *genericSpan(modifier)}
			}
		}
		key := nominalKey{SourcePath: ast.GetSourceFileOfNode(node).FileName(), DeclarationPath: []string{node.Name().Text()}, Category: category}
		digest, err := arrayFactsID("tsfg.array-facts.nominal.v8\x00", key)
		if err != nil {
			return err
		}
		id := "nominal:" + strings.TrimPrefix(digest, "sha256:")
		files := []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		}{}
		for _, file := range p.payload.Files {
			files = append(files, struct {
				Path   string `json:"path"`
				SHA256 string `json:"sha256"`
			}{file.Path, file.SHA256})
		}
		runtimeKey, err := arrayFactsID("tsfg.array-facts.runtime-class.v8\x00", struct {
			Files []struct {
				Path   string `json:"path"`
				SHA256 string `json:"sha256"`
			} `json:"files"`
			NominalKey nominalKey `json:"nominalKey"`
		}{files, key})
		if err != nil {
			return err
		}
		fields := []arrayGenericField{}
		p.classes[symbol] = p.addType(arrayGenericType{ID: id, Kind: kind, Name: node.Name().Text(), SourcePath: key.SourcePath, DeclarationRange: genericSpan(node), NameRange: genericSpan(node.Name()), NominalKey: &key, MarkerOrigin: origin, RuntimeKey: runtimeKey, Fields: &fields}, actual)
	}
	for _, node := range nodes {
		id := p.classes[p.check.GetSymbolAtLocation(node.Name())]
		typ := p.types[id]
		bases := p.check.GetBaseTypes(p.actual[id])
		if len(bases) > 1 {
			return p.fail(node, "single actual class base required")
		}
		if len(bases) == 1 {
			base := p.classes[bases[0].Symbol()]
			if base == "" {
				return p.fail(node, "base must belong to the same source closure")
			}
			typ.Base = base
		}
		fields := []arrayGenericField{}
		add := func(field, annotation *ast.Node) error {
			symbol := p.check.GetPropertyOfType(p.actual[id], field.Name().Text())
			if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != field {
				return p.fail(field, "original instance field ownership required")
			}
			fieldType, err := p.annotation(annotation)
			if err != nil {
				return err
			}
			fieldID, err := arrayFactsID("tsfg.array-facts.field.v8\x00", struct {
				SourcePath string `json:"sourcePath"`
				NameRange  span   `json:"nameRange"`
			}{ast.GetSourceFileOfNode(field).FileName(), *genericSpan(field.Name())})
			if err != nil {
				return err
			}
			v := arrayGenericField{ID: fieldID, Name: field.Name().Text(), Type: fieldType, Readonly: field.ModifierFlags()&ast.ModifierFlagsReadonly != 0, Origin: classNodeAnchor(field)}
			if field.Kind == ast.KindParameter {
				v.ParameterBinding, err = arrayFactsBindingID(ast.GetSourceFileOfNode(field).FileName(), *genericSpan(field.Name()))
				if err != nil {
					return err
				}
			}
			fields = append(fields, v)
			p.fields[symbol] = v
			return nil
		}
		for _, member := range node.Members() {
			switch member.Kind {
			case ast.KindPropertyDeclaration:
				if ast.IsStatic(member) || member.Name() == nil || member.AsPropertyDeclaration().PostfixToken != nil && member.AsPropertyDeclaration().PostfixToken.Kind == ast.KindQuestionToken {
					return p.fail(member, "ordinary required instance field required")
				}
				if err := add(member, member.AsPropertyDeclaration().Type); err != nil {
					return err
				}
			case ast.KindConstructor:
				for _, param := range member.AsConstructorDeclaration().Parameters.Nodes {
					if param.ModifierFlags()&(ast.ModifierFlagsPublic|ast.ModifierFlagsPrivate|ast.ModifierFlagsProtected|ast.ModifierFlagsReadonly) != 0 {
						if err := add(param, param.AsParameterDeclaration().Type); err != nil {
							return err
						}
					}
				}
			case ast.KindSemicolonClassElement:
			case ast.KindMethodDeclaration:
			default:
				return p.fail(member, "class member requires the shared class-method source capability")
			}
		}
		typ.Fields = &fields
		p.types[id] = typ
	}
	for _, node := range nodes {
		id := p.classes[p.check.GetSymbolAtLocation(node.Name())]
		typ := p.types[id]
		for _, member := range node.Members() {
			if member.Kind != ast.KindMethodDeclaration {
				continue
			}
			if ast.IsStatic(member) {
				return p.fail(member, "instance method required")
			}
			header, err := p.header(member, "")
			if err != nil {
				return err
			}
			header.function.OwnerType = id
			symbol := p.check.GetSymbolAtLocation(member.Name())
			actual := p.check.GetPropertyOfType(p.actual[id], member.Name().Text())
			if actual != symbol {
				// Official instantiation can wrap a method (including its this
				// receiver); M12 joins the original declaration through root symbols.
				if actual == nil {
					return p.fail(member, "method must join actual instance member")
				}
				roots := p.check.GetRootSymbols(actual)
				if len(roots) != 1 || roots[0] != symbol {
					return p.fail(member, "method must join actual instance member")
				}
			}
			p.headers[symbol] = header
			typ.Methods = append(typ.Methods, header.function.ID)
		}
		p.types[id] = typ
	}
	for _, header := range p.headers {
		if header.function.OwnerType == "" {
			continue
		}
		base := p.types[header.function.OwnerType].Base
		if base != "" {
			symbol := p.check.GetPropertyOfType(p.actual[base], header.function.Name)
			if original := p.headers[symbol]; original != nil {
				header.function.OverrideTarget = original.function.ID
			}
		}
	}
	return nil
}
func (p *arrayGenericProjection) classBodies(nodes []*ast.Node) error {
	for _, node := range nodes {
		id := p.classes[p.check.GetSymbolAtLocation(node.Name())]
		typ := p.types[id]
		for _, member := range node.Members() {
			if member.Kind == ast.KindMethodDeclaration {
				header := p.headers[p.check.GetSymbolAtLocation(member.Name())]
				fn, err := p.function(header)
				if err != nil {
					return err
				}
				p.payload.Functions = append(p.payload.Functions, fn)
				continue
			}
			if member.Kind != ast.KindConstructor {
				continue
			}
			header := p.constructors[id]
			if header == nil || header.node != member {
				return p.fail(member, "original constructor header differs")
			}
			fn, err := p.function(header)
			if err != nil {
				return err
			}
			typ.Constructor = &fn
		}
		for i, field := range *typ.Fields {
			var actual *ast.Node
			for _, member := range node.Members() {
				if member.Kind == ast.KindPropertyDeclaration && member.Name().Text() == field.Name {
					actual = member
				}
			}
			if actual == nil || actual.AsPropertyDeclaration().Initializer == nil {
				continue
			}
			state := &arrayGenericState{ownerType: id, bindings: map[*ast.Symbol]arrayFactsBinding{}, lengths: map[string]*arrayFactsLengthState{}}
			expr, err := p.expression(actual.AsPropertyDeclaration().Initializer, field.Type, state, 0)
			if err != nil {
				return err
			}
			field.Initializer = expr
			(*typ.Fields)[i] = field
		}
		p.types[id] = typ
	}
	return nil
}
