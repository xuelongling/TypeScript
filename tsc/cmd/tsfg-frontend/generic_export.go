// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Exports checked definitions once; no definition source or upstream object escapes.
package main

import (
	"context"
	"slices"
	"unicode/utf8"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/scanner"
)

func exportGenericGraph(ctx context.Context, program *compiler.Program, file *ast.SourceFile, packageID string, environment genericDeclarationEnvironment) (genericGraph, error) {
	if err := ctx.Err(); err != nil {
		return genericGraph{}, err
	}
	if scope := packageID; scope == "" || len(scope) > 256 || !genericPackageString(scope) {
		return genericGraph{}, genericPackageError("input", "scope")
	}
	environment.Files = slices.Clone(environment.Files)
	if program == nil || file == nil || program.GetSourceFile(file.FileName()) != file {
		return genericGraph{}, genericError("input", "source does not belong to program", genericOrigin{})
	}
	if !utf8.ValidString(file.Text()) || !genericPackagePath(file.FileName()) || file.GetPositionMap().UTF8ToUTF16(len(file.Text())) > 1<<20 {
		return genericGraph{}, genericPackageError("input", "original source metadata")
	}
	options := program.Options()
	if file.IsDeclarationFile || file.ScriptKind != core.ScriptKindTS || file.ExternalModuleIndicator == nil ||
		options.Strict != core.TSTrue || options.StrictNullChecks == core.TSFalse || options.NoImplicitAny == core.TSFalse ||
		options.NoEmit != core.TSTrue || options.NoLib != core.TSTrue || options.NoCheck == core.TSTrue ||
		options.SkipLibCheck == core.TSTrue || options.SkipDefaultLibCheck == core.TSTrue || options.AllowJs == core.TSTrue ||
		options.Target != core.ScriptTargetESNext || options.Module != core.ModuleKindESNext {
		return genericGraph{}, genericError("input", "official module/options differ from generic semantics", genericOrigin{})
	}
	if err := environment.validate(); err != nil {
		return genericGraph{}, err
	}
	inputs := map[string]string{file.FileName(): genericTextDigest(file.Text())}
	for _, declaration := range environment.Files {
		inputs[declaration.Path] = declaration.Digest
	}
	for _, input := range program.GetSourceFiles() {
		if err := ctx.Err(); err != nil {
			return genericGraph{}, err
		}
		if digest, ok := inputs[input.FileName()]; !ok || digest != genericTextDigest(input.Text()) {
			return genericGraph{}, genericError("input", "official program declaration environment differs", genericOrigin{})
		}
		delete(inputs, input.FileName())
	}
	if len(inputs) != 0 {
		return genericGraph{}, genericError("input", "missing official declaration input", genericOrigin{})
	}
	if err := checkGenericDiagnostics(ctx, program, nil); err != nil {
		return genericGraph{}, err
	}
	if err := ctx.Err(); err != nil {
		return genericGraph{}, err
	}
	check, release := program.GetTypeCheckerForFileExclusive(ctx, file)
	defer release()
	if err := ctx.Err(); err != nil {
		return genericGraph{}, err
	}
	if check.WasCanceled() {
		return genericGraph{}, context.Canceled
	}
	g := genericGraph{Package: packageID, Semantics: genericSemantics, Environment: environment,
		Source: genericSourceForFile(file)}
	defs := map[*ast.Symbol]string{}
	nodes := file.Statements.Nodes
	for _, node := range nodes {
		if err := ctx.Err(); err != nil {
			return genericGraph{}, err
		}
		if node.Kind != ast.KindFunctionDeclaration || node.Name() == nil || node.Body() == nil ||
			node.Body().Kind != ast.KindBlock || ast.GetFunctionFlags(node) != ast.FunctionFlagsNormal {
			return genericGraph{}, genericError("unsupported", "only scalar generic function definitions", genericNodeOrigin(node))
		}
		f := node.AsFunctionDeclaration()
		if f.TypeParameters == nil || len(f.TypeParameters.Nodes) == 0 || f.Type == nil || f.FullSignature != nil ||
			len(f.Body.AsBlock().Statements.Nodes) != 1 {
			return genericGraph{}, genericError("unsupported", "explicit generic signature and one return required", genericNodeOrigin(node))
		}
		if m := node.Modifiers(); m != nil && m.ModifierFlags & ^ast.ModifierFlagsExport != 0 {
			return genericGraph{}, genericError("unsupported", "function modifiers", genericNodeOrigin(node))
		}
		d := genericDefinition{Name: node.Name().Text(), Exported: ast.HasSyntacticModifier(node, ast.ModifierFlagsExport), Origin: genericNodeOrigin(node)}
		typeSymbols := map[*ast.Symbol]int{}
		for i, tp := range f.TypeParameters.Nodes {
			p := tp.AsTypeParameterDeclaration()
			if p.Constraint != nil || p.DefaultType != nil || p.Expression != nil || tp.Modifiers() != nil {
				return genericGraph{}, genericError("unsupported", "constrained/default/variant type parameter", genericNodeOrigin(tp))
			}
			sym := check.GetSymbolAtLocation(tp.Name())
			if sym == nil {
				return genericGraph{}, genericError("input", "missing type parameter symbol", genericNodeOrigin(tp))
			}
			typeSymbols[sym] = i
			if !genericPackageString(tp.Name().Text()) {
				return genericGraph{}, genericPackageError("limit", "type parameter string")
			}
			d.TypeParameters = append(d.TypeParameters, tp.Name().Text())
		}
		for _, param := range f.Parameters.Nodes {
			p := param.AsParameterDeclaration()
			if p.Name().Kind != ast.KindIdentifier || p.Type == nil || p.Initializer != nil || p.QuestionToken != nil ||
				p.DotDotDotToken != nil || param.Modifiers() != nil {
				return genericGraph{}, genericError("unsupported", "required typed identifier parameter", genericNodeOrigin(param))
			}
			t, err := exportGenericType(check, p.Type, typeSymbols)
			if err != nil {
				return genericGraph{}, err
			}
			if !genericPackageString(p.Name().Text()) {
				return genericGraph{}, genericPackageError("limit", "parameter string")
			}
			d.Parameters = append(d.Parameters, genericParameter{Name: p.Name().Text(), Type: t, Origin: genericNodeOrigin(param)})
		}
		var err error
		d.Return, err = exportGenericType(check, f.Type, typeSymbols)
		if err != nil {
			return genericGraph{}, err
		}
		if !genericPackageString(d.Name) {
			return genericGraph{}, genericPackageError("limit", "definition string")
		}
		d.Key = genericDefinitionKey(g, d)
		sym := check.GetSymbolAtLocation(node.Name())
		if sym == nil || defs[sym] != "" {
			return genericGraph{}, genericError("unsupported", "overload or missing declaration symbol", d.Origin)
		}
		defs[sym] = d.Key
		g.Definitions = append(g.Definitions, d)
	}
	for i, node := range nodes {
		if err := ctx.Err(); err != nil {
			return genericGraph{}, err
		}
		f := node.AsFunctionDeclaration()
		types := map[*ast.Symbol]int{}
		for j, p := range f.TypeParameters.Nodes {
			types[check.GetSymbolAtLocation(p.Name())] = j
		}
		params := map[*ast.Symbol]int{}
		for j, p := range f.Parameters.Nodes {
			params[check.GetSymbolAtLocation(p.Name())] = j
		}
		ret := f.Body.AsBlock().Statements.Nodes[0]
		if ret.Kind != ast.KindReturnStatement || ret.AsReturnStatement().Expression == nil {
			return genericGraph{}, genericError("unsupported", "single value return required", genericNodeOrigin(ret))
		}
		e, err := exportGenericExpression(ctx, check, ret.AsReturnStatement().Expression, types, params, defs, 0)
		if err != nil {
			return genericGraph{}, err
		}
		g.Definitions[i].Body = e
	}
	if err := genericPackageBudget(g); err != nil {
		return genericGraph{}, err
	}
	g.Digest = genericGraphDigest(g)
	if err := g.validate(); err != nil {
		return genericGraph{}, err
	}
	if err := ctx.Err(); err != nil {
		return genericGraph{}, err
	}
	return g, nil
}

func genericNodeOrigin(node *ast.Node) genericOrigin {
	file := ast.GetSourceFileOfNode(node)
	return genericOrigin{Start: file.GetPositionMap().UTF8ToUTF16(scanner.SkipTrivia(file.Text(), node.Pos())),
		End: file.GetPositionMap().UTF8ToUTF16(node.End())}
}

func exportGenericType(check *checker.Checker, node *ast.Node, params map[*ast.Symbol]int) (genericType, error) {
	switch node.Kind {
	case ast.KindBooleanKeyword:
		return genericType{Kind: "boolean"}, nil
	case ast.KindNumberKeyword:
		return genericType{Kind: "number"}, nil
	case ast.KindIndexedAccessType:
		return exportGenericIndexedType(check, node, params)
	case ast.KindConditionalType:
		c := node.AsConditionalTypeNode()
		if c.CheckType.Kind != ast.KindTypeReference || c.ExtendsType.Kind != ast.KindBooleanKeyword ||
			c.TrueType.Kind != ast.KindBooleanKeyword || c.FalseType.Kind != ast.KindNumberKeyword {
			return genericType{}, genericError("unsupported", "only direct T extends boolean ? boolean : number signatures", genericNodeOrigin(node))
		}
		bound, err := exportGenericType(check, c.CheckType, params)
		if err != nil {
			return genericType{}, err
		}
		return genericType{Kind: "conditional", CheckType: &bound, ExtendsType: &genericType{Kind: "boolean"},
			TrueType: &genericType{Kind: "boolean"}, FalseType: &genericType{Kind: "number"}}, nil
	case ast.KindTypeReference:
		r := node.AsTypeReferenceNode()
		if r.TypeName.Kind == ast.KindIdentifier && r.TypeArguments == nil {
			if i, ok := params[check.GetSymbolAtLocation(r.TypeName)]; ok {
				return genericType{Kind: "parameter", Parameter: i}, nil
			}
		}
	}
	return genericType{}, genericError("unsupported", "type outside scalar/type-parameter slice", genericNodeOrigin(node))
}

func exportGenericKeyType(node *ast.Node) (genericKeyType, error) {
	keys := genericKeyType{Kind: "string-literal"}
	nodes := []*ast.Node{node}
	if node.Kind == ast.KindUnionType {
		keys.Kind = "union"
		nodes = node.AsUnionTypeNode().Types.Nodes
	}
	if len(nodes) > genericPackageMaxKeyMembers {
		return genericKeyType{}, genericError("package-limit", "key union members", genericNodeOrigin(node))
	}
	for _, key := range nodes {
		if key.Kind != ast.KindLiteralType || key.AsLiteralTypeNode().Literal.Kind != ast.KindStringLiteral {
			return genericKeyType{}, genericError("unsupported", "mapped keys require string literals or a finite literal union", genericNodeOrigin(key))
		}
		keys.Literals = append(keys.Literals, key.AsLiteralTypeNode().Literal.Text())
	}
	if err := validateGenericKeyType(keys.Kind, keys.Literals, genericNodeOrigin(node)); err != nil {
		return genericKeyType{}, err
	}
	return keys, nil
}

func exportGenericIndexedType(check *checker.Checker, node *ast.Node, params map[*ast.Symbol]int) (genericType, error) {
	indexed := node.AsIndexedAccessTypeNode()
	if len(params) != 1 || indexed.ObjectType.Kind != ast.KindMappedType {
		return genericType{}, genericError("unsupported", "indexed signature requires one outer type parameter and a direct mapped object", genericNodeOrigin(node))
	}
	mapped := indexed.ObjectType.AsMappedTypeNode()
	if mapped.ReadonlyToken != nil || mapped.QuestionToken != nil || mapped.NameType != nil || mapped.Members != nil && len(mapped.Members.Nodes) != 0 ||
		mapped.TypeParameter == nil || mapped.Type == nil {
		return genericType{}, genericError("unsupported", "mapped modifiers, remapping or members", genericNodeOrigin(indexed.ObjectType))
	}
	parameter := mapped.TypeParameter.AsTypeParameterDeclaration()
	if parameter.Name().Kind != ast.KindIdentifier || parameter.Constraint == nil || parameter.Expression != nil ||
		parameter.DefaultType != nil || mapped.TypeParameter.Modifiers() != nil {
		return genericType{}, genericError("unsupported", "mapped key binder metadata", genericNodeOrigin(mapped.TypeParameter))
	}
	binder := check.GetSymbolAtLocation(parameter.Name())
	if binder == nil {
		return genericType{}, genericError("input", "missing mapped key binder symbol", genericNodeOrigin(mapped.TypeParameter))
	}
	for outer := range params {
		if binder == outer || parameter.Name().Text() == outer.Name {
			return genericType{}, genericError("unsupported", "mapped binder shadows outer type parameter", genericNodeOrigin(mapped.TypeParameter))
		}
	}
	if mapped.Type.Kind != ast.KindBooleanKeyword && mapped.Type.Kind != ast.KindNumberKeyword && mapped.Type.Kind != ast.KindTypeReference {
		return genericType{}, genericError("unsupported", "mapped value requires an outer parameter or scalar leaf", genericNodeOrigin(mapped.Type))
	}
	// exportGenericType accepts a parameter reference only by its real outer
	// symbol. The independent mapped binder is deliberately absent from params.
	value, err := exportGenericType(check, mapped.Type, params)
	if err != nil {
		return genericType{}, err
	}
	keys, err := exportGenericKeyType(parameter.Constraint)
	if err != nil {
		return genericType{}, err
	}
	index, err := exportGenericKeyType(indexed.IndexType)
	if err != nil {
		return genericType{}, err
	}
	result := genericType{Kind: "indexed", Indexed: &genericIndexedType{
		Object: genericType{Kind: "mapped", Mapped: &genericMappedType{Binder: parameter.Name().Text(), Keys: keys, Value: value}}, Index: index}}
	if err := validateGenericIndexedType(result, len(params), genericNodeOrigin(node)); err != nil {
		return genericType{}, err
	}
	return result, nil
}

func exportGenericExpression(ctx context.Context, check *checker.Checker, node *ast.Node, types, params map[*ast.Symbol]int, defs map[*ast.Symbol]string, depth int) (genericExpression, error) {
	if err := ctx.Err(); err != nil {
		return genericExpression{}, err
	}
	if depth > 64 {
		return genericExpression{}, genericError("unsupported", "expression depth", genericNodeOrigin(node))
	}
	e := genericExpression{Origin: genericNodeOrigin(node)}
	if node.Kind == ast.KindIdentifier {
		if i, ok := params[check.GetSymbolAtLocation(node)]; ok {
			e.Kind, e.Parameter = "parameter", i
			return e, nil
		}
	}
	if node.Kind == ast.KindBinaryExpression {
		binary := node.AsBinaryExpression()
		result := check.GetTypeAtLocation(node)
		if depth != 0 || binary.OperatorToken.Kind != ast.KindPlusToken || result == nil ||
			result.Flags()&checker.TypeFlagsNumber == 0 {
			return genericExpression{}, genericError("unsupported", "number-add requires a root number PlusToken expression", e.Origin)
		}
		e.Kind = "number-add"
		for _, operand := range []*ast.Node{binary.Left, binary.Right} {
			symbol := check.GetSymbolAtLocation(operand)
			_, bound := params[symbol]
			value := check.GetTypeAtLocation(operand)
			if operand.Kind != ast.KindIdentifier || !bound || value == nil || value.Flags()&checker.TypeFlagsNumber == 0 {
				return genericExpression{}, genericError("unsupported", "number-add requires direct bound number parameters", genericNodeOrigin(operand))
			}
			argument, err := exportGenericExpression(ctx, check, operand, types, params, defs, depth+1)
			if err != nil {
				return genericExpression{}, err
			}
			e.Arguments = append(e.Arguments, argument)
		}
		return e, nil
	}
	if node.Kind == ast.KindCallExpression {
		call := node.AsCallExpression()
		if call.Expression.Kind == ast.KindIdentifier && call.QuestionDotToken == nil && call.TypeArguments != nil {
			key := defs[check.GetSymbolAtLocation(call.Expression)]
			resolved := check.GetResolvedSignature(node)
			if key != "" && resolved != nil && resolved.Declaration() != nil &&
				defs[check.GetSymbolAtLocation(resolved.Declaration().Name())] == key {
				e.Kind, e.Definition = "call", key
				for _, a := range call.TypeArguments.Nodes {
					if a.Kind == ast.KindConditionalType {
						return genericExpression{}, genericError("unsupported", "conditional type arguments are outside the signature slice", genericNodeOrigin(a))
					}
					if a.Kind == ast.KindMappedType || a.Kind == ast.KindIndexedAccessType {
						return genericExpression{}, genericError("unsupported", "mapped/indexed type arguments are outside the signature slice", genericNodeOrigin(a))
					}
					t, err := exportGenericType(check, a, types)
					if err != nil {
						return genericExpression{}, err
					}
					e.TypeArguments = append(e.TypeArguments, t)
				}
				for _, a := range call.Arguments.Nodes {
					arg, err := exportGenericExpression(ctx, check, a, types, params, defs, depth+1)
					if err != nil {
						return genericExpression{}, err
					}
					e.Arguments = append(e.Arguments, arg)
				}
				return e, nil
			}
		}
	}
	return genericExpression{}, genericError("unsupported", "only parameter values, explicit generic direct calls and root number-add", e.Origin)
}

func genericSourceForFile(file *ast.SourceFile) genericSource {
	source := genericSource{Path: file.FileName(), Digest: genericTextDigest(file.Text()), PositionUnit: "utf16",
		Length: file.GetPositionMap().UTF8ToUTF16(len(file.Text()))}
	position := 0
	for _, ch := range file.Text() {
		if ch > 0xffff {
			source.SurrogateStarts = append(source.SurrogateStarts, position)
			position += 2
		} else {
			position++
		}
	}
	return source
}
