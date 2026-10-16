// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/json"
)

// The only result escaping this scope is serialized owned data. The original
// parser/checker have already checked the definitions, but no bool/f64 instance
// is requested until callers construct another Program from these bytes.
func exportedGenericBytes(t *testing.T) []byte {
	t.Helper()
	source := "/* original 😀 anchor */\nexport function identity<T>(value: T): T { return value; }\n" +
		"export function route<U>(input: U): U { return identity<U>(input); }\n" +
		"export function unused<V>(flag: boolean): boolean { return flag; }\n"
	graph, _ := exportGenericSourceForTest(t, source)
	start := strings.Index(source, "export function route")
	if graph.Source.PositionUnit != "utf16" || graph.Source.Length != len(utf16.Encode([]rune(source))) ||
		graph.Definitions[1].Origin.Start != len(utf16.Encode([]rune(source[:start]))) || graph.Definitions[1].Origin.Start == start {
		t.Fatal("non-BMP original-source anchor was not converted before export")
	}
	data, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "export function") || strings.Contains(string(data), "__tsfg_request") {
		t.Fatal("definition export retained source text or precomputed concrete instances")
	}
	return data
}

func ownedGenericFixture(t *testing.T) genericGraph {
	t.Helper()
	data := exportedGenericBytes(t)
	var graph genericGraph
	if err := json.Unmarshal(data, &graph); err != nil {
		t.Fatal(err)
	}
	if err := graph.validate(); err != nil {
		t.Fatal(err)
	}
	return graph
}

func genericDefinitionNamed(t *testing.T, g genericGraph, name string) genericDefinition {
	t.Helper()
	for _, d := range g.Definitions {
		if d.Name == name {
			return d
		}
	}
	t.Fatalf("missing owned definition %q", name)
	return genericDefinition{}
}

func TestOwnedGenericHostFreshInstances(t *testing.T) {
	t.Parallel()
	g := ownedGenericFixture(t)
	identity := genericDefinitionNamed(t, g, "identity")
	route := genericDefinitionNamed(t, g, "route")
	unused := genericDefinitionNamed(t, g, "unused")
	boolType, numberType := genericType{Kind: "boolean"}, genericType{Kind: "number"}
	requests := []genericInstanceRequest{
		g.request(route.Key, boolType), g.request(route.Key, numberType),
		g.request(unused.Key, boolType), g.request(unused.Key, numberType),
		g.request(route.Key, boolType),
	}
	result, err := instantiateGenericGraph(t.Context(), g, requests)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Instances) != 6 || len(result.Roots) != 5 || result.Roots[0] != result.Roots[4] {
		t.Fatalf("closure or repeated instance reuse: %+v", result)
	}
	for i, want := range []string{"bool", "f64"} {
		r := result.Instances[result.Roots[i]]
		idKey := genericInstanceKey(g.request(identity.Key, requests[i].TypeArguments...))
		id := result.Instances[idKey]
		if r.Return != want || !reflect.DeepEqual(r.Parameters, []string{want}) || r.Body.Kind != "call" ||
			r.Body.Callee != idKey || r.Body.Type != want || len(r.Body.Arguments) != 1 ||
			r.Body.Arguments[0].Type != want || id.Return != want || id.Body.Type != want || id.Body.Kind != "parameter" {
			t.Fatalf("official mapped transitive body: route=%+v identity=%+v", r, id)
		}
		if r.Origin != route.Origin || r.Body.Origin != route.Body.Origin || !reflect.DeepEqual(r.Source, g.Source) || id.Definition != identity.Key {
			t.Fatal("original declaration/source identity lost")
		}
	}
	a, b := result.Instances[result.Roots[2]], result.Instances[result.Roots[3]]
	if a.Key == b.Key || a.Return != "bool" || b.Return != "bool" || !reflect.DeepEqual(a.Parameters, b.Parameters) ||
		a.TypeArguments[0] == b.TypeArguments[0] {
		t.Fatalf("phantom arguments collapsed: a=%+v b=%+v", a, b)
	}
}

func TestOwnedGenericHostRejectsBadOfficialType(t *testing.T) {
	t.Parallel()
	g := ownedGenericFixture(t)
	bad := genericDefinitionNamed(t, g, "unused")
	for i := range g.Definitions {
		if g.Definitions[i].Key == bad.Key {
			g.Definitions[i].Return = genericType{Kind: "number"}
		}
	}
	g.Digest = genericGraphDigest(g)
	if err := g.validate(); err != nil {
		t.Fatalf("bad type must reach official checking, got owned rejection: %v", err)
	}
	_, err := instantiateGenericGraph(t.Context(), g, []genericInstanceRequest{g.request(bad.Key, genericType{Kind: "boolean"})})
	var failure *genericFailure
	if !errors.As(err, &failure) || failure.Kind != "official-type" || failure.Code != 2322 ||
		!reflect.DeepEqual(failure.Source, g.Source) || failure.Origin.Start < bad.Origin.Start || failure.Origin.End > bad.Origin.End || failure.Origin.End <= failure.Origin.Start {
		t.Fatalf("expected real TS2322 with original source anchor, got %v", err)
	}
}

func TestOwnedGenericHostRejectsMetadataAndRequests(t *testing.T) {
	t.Parallel()
	data := exportedGenericBytes(t)
	for _, test := range []struct {
		name   string
		mutate func(*genericGraph)
	}{
		{"declaration missing", func(g *genericGraph) { g.Environment.Files = nil }},
		{"declaration hash", func(g *genericGraph) { g.Environment.Files[0].Digest = "wrong" }},
		{"declaration profile", func(g *genericGraph) { g.Environment.Profile = "wrong" }},
		{"source unit", func(g *genericGraph) { g.Source.PositionUnit = "utf8" }},
		{"source identity", func(g *genericGraph) { g.Source.Digest = "bad" }},
		{"relative source", func(g *genericGraph) { g.Source.Path = "main.ts" }},
		{"semantic identity", func(g *genericGraph) { g.Semantics = "different-semantics" }},
		{"declaration identity", func(g *genericGraph) { g.Definitions[0].Key = "foreign" }},
		{"origin bounds", func(g *genericGraph) { g.Definitions[0].Parameters[0].Origin.End = g.Source.Length + 1 }},
		{"dangling call", func(g *genericGraph) { g.Definitions = g.Definitions[1:] }},
		{"duplicate definitions", func(g *genericGraph) { g.Definitions = append(g.Definitions, g.Definitions[0]) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var g genericGraph
			if err := json.Unmarshal(data, &g); err != nil {
				t.Fatal(err)
			}
			test.mutate(&g)
			g.Digest = genericGraphDigest(g)
			if _, err := instantiateGenericGraph(t.Context(), g, []genericInstanceRequest{g.request(g.Definitions[0].Key, genericType{Kind: "boolean"})}); err == nil {
				t.Fatal("invalid graph admitted")
			}
		})
	}
	g := ownedGenericFixture(t)
	base := g.request(g.Definitions[0].Key, genericType{Kind: "boolean"})
	for _, test := range []struct {
		name   string
		mutate func(*genericInstanceRequest)
	}{
		{"package", func(r *genericInstanceRequest) { r.Package = "other-package" }},
		{"semantics", func(r *genericInstanceRequest) { r.Semantics = "other-semantics" }},
		{"graph", func(r *genericInstanceRequest) { r.GraphDigest = "other-graph" }},
		{"missing definition", func(r *genericInstanceRequest) { r.Definition = "absent" }},
		{"arity", func(r *genericInstanceRequest) { r.TypeArguments = nil }},
		{"uninstantiated", func(r *genericInstanceRequest) { r.TypeArguments = []genericType{{Kind: "parameter"}} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			r := base
			test.mutate(&r)
			if _, err := instantiateGenericGraph(t.Context(), g, []genericInstanceRequest{r}); err == nil {
				t.Fatal("invalid request admitted")
			}
		})
	}
}

type genericSyntaxFact struct {
	Kind  ast.Kind
	Start int
	End   int
	Name  string
}

func genericSyntaxFacts(tree *genericTree) []genericSyntaxFact {
	var facts []genericSyntaxFact
	var visit func(*ast.Node) bool
	visit = func(n *ast.Node) bool {
		f := genericSyntaxFact{Kind: n.Kind, Start: n.Pos(), End: n.End()}
		if n.Kind == ast.KindIdentifier {
			f.Name = n.Text()
		}
		facts = append(facts, f)
		n.ForEachChild(visit)
		return false
	}
	for _, n := range tree.statements {
		visit(n)
	}
	return facts
}

func TestOwnedGenericHostDiagnosticTextIsNonSemantic(t *testing.T) {
	t.Parallel()
	g := ownedGenericFixture(t)
	d := genericDefinitionNamed(t, g, "route")
	r := g.request(d.Key, genericType{Kind: "number"})
	tree, blankTree := genericTreeForTest(t, g, r), genericTreeForTest(t, g, r)
	if !reflect.DeepEqual(genericSyntaxFacts(tree), genericSyntaxFacts(blankTree)) {
		t.Fatal("metamorphic AST structure differs")
	}
	program := newGenericProgram(g, tree, tree.text.String())
	blank := strings.Repeat(" ", blankTree.text.Len())
	blankProgram := newGenericProgram(g, blankTree, blank)
	if err := checkGenericProgram(t.Context(), g, program, tree); err != nil {
		t.Fatal(err)
	}
	if err := checkGenericProgram(t.Context(), g, blankProgram, blankTree); err != nil {
		t.Fatal(err)
	}
	a, ac, err := projectCheckedGenericInstance(t.Context(), g, r, tree, program)
	if err != nil {
		t.Fatal(err)
	}
	b, bc, err := projectCheckedGenericInstance(t.Context(), g, r, blankTree, blankProgram)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(ac, bc) || b.Return != "f64" || b.Body.Type != "f64" {
		t.Fatalf("diagnostic text changed official concrete facts: normal=%+v blank=%+v", a, b)
	}
	file := blankProgram.GetSourceFile(blankTree.fileName)
	if file.Text() != blank || file.ScriptKind != core.ScriptKindTS || file.LanguageVariant != core.LanguageVariantStandard ||
		file.ExternalModuleIndicator == nil || len(file.Identifiers) == 0 || file.NodeCount == 0 {
		t.Fatal("owned SourceFile metadata missing")
	}
	var bounded func(*ast.Node) bool
	bounded = func(n *ast.Node) bool {
		if n.Pos() < 0 || n.End() < n.Pos() || n.End() > len(file.Text()) ||
			(n != file.AsNode() && n.Parent == nil) {
			t.Fatal("AST text/Loc/parent invariant violated")
		}
		n.ForEachChild(bounded)
		return false
	}
	bounded(file.AsNode())
}

func TestOwnedGenericHostMapperRejectsForeignOwnership(t *testing.T) {
	t.Parallel()
	g := ownedGenericFixture(t)
	d := genericDefinitionNamed(t, g, "identity")
	r := g.request(d.Key, genericType{Kind: "boolean"})
	tree, foreign := genericTreeForTest(t, g, r), genericTreeForTest(t, g, r)
	program, other := newGenericProgram(g, tree, tree.text.String()), newGenericProgram(g, foreign, foreign.text.String())
	if err := checkGenericProgram(t.Context(), g, program, tree); err != nil {
		t.Fatal(err)
	}
	if err := checkGenericProgram(t.Context(), g, other, foreign); err != nil {
		t.Fatal(err)
	}
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), program.GetSourceFile(tree.fileName))
	defer release()
	decl := tree.definitions[d.Key]
	node := decl.Body().AsBlock().Statements.Nodes[0].AsReturnStatement().Expression
	if typ, ok := check.TSFGInstantiateBodyType(tree.invocation, decl, node, false); !ok || genericConcreteType(typ) != "bool" {
		t.Fatal("valid official mapper projection failed")
	}
	if _, ok := check.TSFGInstantiateBodyType(foreign.invocation, decl, node, false); ok {
		t.Fatal("foreign invocation accepted")
	}
	if _, ok := check.TSFGInstantiateBodyType(tree.invocation, foreign.definitions[d.Key], node, false); ok {
		t.Fatal("foreign declaration accepted")
	}
	unrelated := tree.definitions[genericDefinitionNamed(t, g, "unused").Key]
	if _, ok := check.TSFGInstantiateBodyType(tree.invocation, decl, unrelated.Type(), true); ok {
		t.Fatal("unrelated body accepted")
	}
}

func TestOwnedGenericHostRejectsBadDeclarationEnvironment(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"missing global", "invalid declaration"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g := ownedGenericFixture(t)
			content := strings.ReplaceAll(g.Environment.Files[0].Content, "interface Array<T> {}", "")
			if name == "invalid declaration" {
				content = "interface {"
			}
			g.Environment.Files[0].Content = content
			g.Environment.Files[0].Digest = genericTextDigest(content)
			g.Digest = genericGraphDigest(g)
			if err := g.validate(); err != nil {
				t.Fatal(err)
			}
			_, err := instantiateGenericGraph(t.Context(), g, []genericInstanceRequest{g.request(g.Definitions[0].Key, genericType{Kind: "boolean"})})
			var failure *genericFailure
			if !errors.As(err, &failure) || failure.Kind != "official-type" || failure.Code == 0 {
				t.Fatalf("invalid environment must reach official diagnostics: %v", err)
			}
			if name == "missing global" && failure.Code != 2318 {
				t.Fatalf("global diagnostics omitted: %v", err)
			}
			if name == "invalid declaration" && failure.Source.Path != g.Environment.Files[0].Path {
				t.Fatalf("declaration diagnostic identity lost: %v", err)
			}
		})
	}
}
