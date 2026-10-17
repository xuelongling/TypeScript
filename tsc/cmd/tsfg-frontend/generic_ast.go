// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Rebuilds syntax with the official factory. Text is a diagnostic position carrier;
// it is never parsed, and origins are a separate map into the original source.
package main

import (
	"strconv"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/core"
)

type genericTree struct {
	fileName        string
	factory         *ast.NodeFactory
	text            strings.Builder
	identifiers     map[string]string
	identifierCount int
	origins         map[*ast.Node]genericOrigin
	definitions     map[string]*ast.Node
	statements      []*ast.Node
	invocation      *ast.Node
}

func buildGenericTree(g genericGraph, request genericInstanceRequest) (*genericTree, error) {
	fileName, err := genericOwnedFileName(g)
	if err != nil {
		return nil, err
	}
	b := &genericTree{fileName: fileName, factory: ast.NewNodeFactory(ast.NodeFactoryHooks{}),
		identifiers: map[string]string{}, origins: map[*ast.Node]genericOrigin{}, definitions: map[string]*ast.Node{}}
	defs := map[string]genericDefinition{}
	for _, d := range g.Definitions {
		defs[d.Key] = d
	}
	for _, d := range g.Definitions {
		n := b.function(d, defs)
		b.definitions[d.Key] = n
		b.statements = append(b.statements, n)
	}
	b.statements = append(b.statements, b.driver(defs[request.Definition], request))
	return b, nil
}

func (b *genericTree) pos() int          { return b.text.Len() }
func (b *genericTree) write(text string) { b.text.WriteString(text) }

func (b *genericTree) mark(node *ast.Node, start int, origin genericOrigin) *ast.Node {
	node.Loc = core.NewTextRange(start, b.pos())
	if origin.End > origin.Start {
		b.origins[node] = origin
	}
	return node
}

func (b *genericTree) identifier(name string, origin genericOrigin) *ast.Node {
	start := b.pos()
	b.write(name)
	b.identifiers[name] = name
	b.identifierCount++
	return b.mark(b.factory.NewIdentifier(name), start, origin)
}

func (b *genericTree) list(nodes []*ast.Node, start int) *ast.NodeList {
	list := b.factory.NewNodeList(nodes)
	list.Loc = core.NewTextRange(start, b.pos())
	return list
}

func (b *genericTree) typeNode(t genericType, names []string, origin genericOrigin) *ast.Node {
	return b.typeNodeWithArguments(t, names, nil, origin)
}

// Substitute outer parameter syntax leaves only. Compound type and key syntax
// stays intact for the fresh official checker to determine the concrete type.
func (b *genericTree) typeNodeWithArguments(t genericType, names []string, arguments []genericType, origin genericOrigin) *ast.Node {
	start := b.pos()
	if t.Kind == "indexed" {
		object := b.typeNodeWithArguments(t.Indexed.Object, names, arguments, origin)
		b.write("[")
		index := b.keyTypeNode(t.Indexed.Index, origin)
		b.write("]")
		return b.mark(b.factory.NewIndexedAccessTypeNode(object, index), start, origin)
	}
	if t.Kind == "mapped" {
		b.write("{ [")
		parameterStart := b.pos()
		name := b.identifier(t.Mapped.Binder, origin)
		b.write(" in ")
		constraint := b.keyTypeNode(t.Mapped.Keys, origin)
		parameter := b.mark(b.factory.NewTypeParameterDeclaration(nil, name, constraint, nil, nil), parameterStart, origin)
		b.write("]: ")
		value := b.typeNodeWithArguments(t.Mapped.Value, names, arguments, origin)
		// The official grammar checker dereferences Members even when empty.
		members := b.list(nil, b.pos())
		b.write(" }")
		return b.mark(b.factory.NewMappedTypeNode(nil, parameter, nil, nil, value, members), start, origin)
	}
	if t.Kind == "conditional" {
		check := b.typeNodeWithArguments(*t.CheckType, names, arguments, origin)
		b.write(" extends ")
		extends := b.typeNode(*t.ExtendsType, names, origin)
		b.write(" ? ")
		whenTrue := b.typeNode(*t.TrueType, names, origin)
		b.write(" : ")
		whenFalse := b.typeNode(*t.FalseType, names, origin)
		return b.mark(b.factory.NewConditionalTypeNode(check, extends, whenTrue, whenFalse), start, origin)
	}
	if t.Kind == "parameter" && arguments != nil {
		return b.typeNode(arguments[t.Parameter], nil, origin)
	}
	if t.Kind == "parameter" {
		name := b.identifier(names[t.Parameter], origin)
		return b.mark(b.factory.NewTypeReferenceNode(name, nil), start, origin)
	}
	kind := ast.KindBooleanKeyword
	if t.Kind == "number" {
		kind = ast.KindNumberKeyword
	}
	b.write(t.Kind)
	return b.mark(b.factory.NewKeywordTypeNode(kind), start, origin)
}

// Every union occurrence gets its own official literal node. Diagnostic text
// carries positions only; StringLiteral.Text carries the validated key value.
func (b *genericTree) keyTypeNode(t genericKeyType, origin genericOrigin) *ast.Node {
	start := b.pos()
	var nodes []*ast.Node
	for i, text := range t.Literals {
		if i > 0 {
			b.write(" | ")
		}
		literalStart := b.pos()
		b.write(strconv.Quote(text))
		literal := b.mark(b.factory.NewStringLiteral(text, ast.TokenFlagsNone), literalStart, origin)
		nodes = append(nodes, b.mark(b.factory.NewLiteralTypeNode(literal), literalStart, origin))
	}
	if t.Kind == "string-literal" {
		return nodes[0]
	}
	return b.mark(b.factory.NewUnionTypeNode(b.list(nodes, start)), start, origin)
}

func (b *genericTree) expression(e genericExpression, d genericDefinition, defs map[string]genericDefinition) *ast.Node {
	if e.Kind == "number-add" {
		start := b.pos()
		left := b.expression(e.Arguments[0], d, defs)
		b.write(" ")
		operatorStart := b.pos()
		b.write("+")
		operator := b.mark(b.factory.NewToken(ast.KindPlusToken), operatorStart, e.Origin)
		b.write(" ")
		right := b.expression(e.Arguments[1], d, defs)
		return b.mark(b.factory.NewBinaryExpression(nil, left, nil, operator, right), start, e.Origin)
	}
	if e.Kind == "parameter" {
		return b.identifier(d.Parameters[e.Parameter].Name, e.Origin)
	}
	start := b.pos()
	name := b.identifier(defs[e.Definition].Name, e.Origin)
	b.write("<")
	argStart := b.pos()
	var types []*ast.Node
	for i, t := range e.TypeArguments {
		if i > 0 {
			b.write(", ")
		}
		types = append(types, b.typeNode(t, d.TypeParameters, e.Origin))
	}
	typeArgs := b.list(types, argStart)
	b.write(">(")
	argStart = b.pos()
	var args []*ast.Node
	for i, a := range e.Arguments {
		if i > 0 {
			b.write(", ")
		}
		args = append(args, b.expression(a, d, defs))
	}
	arguments := b.list(args, argStart)
	b.write(")")
	return b.mark(b.factory.NewCallExpression(name, nil, typeArgs, arguments, ast.NodeFlagsNone), start, e.Origin)
}

func (b *genericTree) function(d genericDefinition, defs map[string]genericDefinition) *ast.Node {
	start := b.pos()
	var modifiers *ast.ModifierList
	if d.Exported {
		ms := b.pos()
		b.write("export")
		token := b.mark(b.factory.NewToken(ast.KindExportKeyword), ms, d.Origin)
		modifiers = b.factory.NewModifierList([]*ast.Node{token})
		modifiers.Loc = core.NewTextRange(ms, b.pos())
		b.write(" ")
	}
	b.write("function ")
	name := b.identifier(d.Name, d.Origin)
	b.write("<")
	typeStart := b.pos()
	var typeParams []*ast.Node
	for i, tp := range d.TypeParameters {
		if i > 0 {
			b.write(", ")
		}
		ps := b.pos()
		n := b.identifier(tp, d.Origin)
		typeParams = append(typeParams, b.mark(b.factory.NewTypeParameterDeclaration(nil, n, nil, nil, nil), ps, d.Origin))
	}
	types := b.list(typeParams, typeStart)
	b.write(">(")
	paramStart := b.pos()
	var params []*ast.Node
	for i, p := range d.Parameters {
		if i > 0 {
			b.write(", ")
		}
		ps := b.pos()
		n := b.identifier(p.Name, p.Origin)
		b.write(": ")
		t := b.typeNode(p.Type, d.TypeParameters, p.Origin)
		params = append(params, b.mark(b.factory.NewParameterDeclaration(nil, nil, n, nil, t, nil), ps, p.Origin))
	}
	parameters := b.list(params, paramStart)
	b.write("): ")
	ret := b.typeNode(d.Return, d.TypeParameters, d.Origin)
	b.write(" ")
	bs := b.pos()
	b.write("{ ")
	rs := b.pos()
	b.write("return ")
	e := b.expression(d.Body, d, defs)
	b.write(";")
	r := b.mark(b.factory.NewReturnStatement(e), rs, d.Body.Origin)
	statements := b.list([]*ast.Node{r}, rs)
	b.write(" }")
	body := b.mark(b.factory.NewBlock(statements, false), bs, d.Origin)
	node := b.mark(b.factory.NewFunctionDeclaration(modifiers, nil, name, types, parameters, ret, nil, body), start, d.Origin)
	b.write("\n")
	return node
}

func (b *genericTree) driver(d genericDefinition, request genericInstanceRequest) *ast.Node {
	start := b.pos()
	b.write("function ")
	name := b.identifier(b.freshName("__tsfg_request"), d.Origin)
	b.write("(")
	ps := b.pos()
	var params []*ast.Node
	var argumentNames []string
	for i, p := range d.Parameters {
		if i > 0 {
			b.write(", ")
		}
		begin := b.pos()
		argumentName := b.freshName("__tsfg_arg" + strconv.Itoa(i))
		argumentNames = append(argumentNames, argumentName)
		n := b.identifier(argumentName, p.Origin)
		b.write(": ")
		typeNode := b.typeNodeWithArguments(p.Type, nil, request.TypeArguments, p.Origin)
		params = append(params, b.mark(b.factory.NewParameterDeclaration(nil, nil, n, nil, typeNode, nil), begin, p.Origin))
	}
	parameters := b.list(params, ps)
	b.write(") ")
	bs := b.pos()
	b.write("{ ")
	rs := b.pos()
	b.write("return ")
	cs := b.pos()
	callee := b.identifier(d.Name, d.Origin)
	b.write("<")
	ts := b.pos()
	var types []*ast.Node
	for i, a := range request.TypeArguments {
		if i > 0 {
			b.write(", ")
		}
		types = append(types, b.typeNode(a, nil, d.Origin))
	}
	typeArgs := b.list(types, ts)
	b.write(">(")
	as := b.pos()
	var args []*ast.Node
	for i, p := range d.Parameters {
		if i > 0 {
			b.write(", ")
		}
		args = append(args, b.identifier(argumentNames[i], p.Origin))
	}
	arguments := b.list(args, as)
	b.write(")")
	b.invocation = b.mark(b.factory.NewCallExpression(callee, nil, typeArgs, arguments, ast.NodeFlagsNone), cs, d.Origin)
	b.write(";")
	ret := b.mark(b.factory.NewReturnStatement(b.invocation), rs, d.Origin)
	statements := b.list([]*ast.Node{ret}, rs)
	b.write(" }")
	body := b.mark(b.factory.NewBlock(statements, false), bs, d.Origin)
	n := b.mark(b.factory.NewFunctionDeclaration(nil, nil, name, nil, parameters, nil, nil, body), start, d.Origin)
	b.write("\n")
	return n
}

func (b *genericTree) sourceFile(opts ast.SourceFileParseOptions, diagnosticText string) *ast.SourceFile {
	// Equal byte length keeps every Loc bounded even when a test blanks all text.
	if len(diagnosticText) != b.pos() {
		panic("owned AST diagnostic text length mismatch")
	}
	end := b.pos()
	eof := b.factory.NewToken(ast.KindEndOfFile)
	eof.Loc = core.NewTextRange(end, end)
	statements := b.factory.NewNodeList(b.statements)
	statements.Loc = core.NewTextRange(0, end)
	file := b.factory.NewSourceFile(opts, diagnosticText, statements, eof).AsSourceFile()
	file.Loc = core.NewTextRange(0, end)
	file.ScriptKind = core.ScriptKindTS
	file.LanguageVariant = core.LanguageVariantStandard
	file.Identifiers = b.identifiers
	file.IdentifierCount = b.identifierCount
	file.NodeCount = b.factory.NodeCount()
	file.TextCount = b.factory.TextCount()
	// This slice is a single module; no parser-only JS/ambient/async flags apply.
	ast.SetExternalModuleIndicator(file, ast.ExternalModuleIndicatorOptions{Force: true})
	ast.SetParentInChildren(file.AsNode())
	return file
}

// Internal names must not reserve any prefix in user declarations. All source
// identifiers have already been registered before the driver is constructed.
func (b *genericTree) freshName(base string) string {
	for b.identifiers[base] != "" {
		base += "_"
	}
	return base
}

// At most one same-directory candidate conflicts with each explicit input.
func genericOwnedFileName(g genericGraph) (string, error) {
	inputs := []string{g.Source.Path}
	for _, file := range g.Environment.Files {
		inputs = append(inputs, file.Path)
	}
	for suffix := 0; suffix < len(g.Environment.Files)+2; suffix++ {
		name := "/owned/definitions.ts"
		if suffix != 0 {
			name = "/owned/definitions-" + strconv.Itoa(suffix) + ".ts"
		}
		conflict := false
		for _, input := range inputs {
			if genericPackageFilePathConflict(name, input) {
				conflict = true
				break
			}
		}
		if !conflict {
			return name, nil
		}
	}
	return "", genericPackageError("input", "no conflict-free owned source path")
}
