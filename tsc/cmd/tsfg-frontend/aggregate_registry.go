// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/sha256"
	"fmt"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/tspath"
)

type aggregateInfo struct {
	node     *ast.Node
	symbol   *ast.Symbol
	instance *checker.Type
	static   *checker.Type
	checked  *checker.Signature
	index    int
	fields   map[*ast.Symbol]int
}

type aggregateProjection struct {
	facts      *aggregateFacts
	classes    map[*ast.Symbol]*aggregateInfo
	types      map[*checker.Type]*aggregateInfo
	markers    map[*ast.Symbol]markerOrigin
	categories map[*ast.Symbol]string
}

func markerHash() string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(markerDeclarations)))
}

func validMarkerProfile(r request) bool {
	p := r.Declarations.MarkerProfile
	if p.Version != markerVersion || p.Path != tspath.CombinePaths(r.SourceRoot, "minimal-aggregates-v1.d.ts") || p.SHA256 != markerHash() || p.ContextName != "ClassDecoratorContext" || p.ContextArity != 1 || p.Mode != "standard" || len(p.Entries) != 2 || p.Entries[0] != (markerEntry{"tsfgValue", "value"}) || p.Entries[1] != (markerEntry{"tsfgComponent", "component"}) {
		return false
	}
	count := 0
	for _, path := range r.Declarations.Paths {
		if path == p.Path {
			count++
		}
	}
	return count == 1
}

func admittedAggregates(r request, program *compiler.Program, check *checker.Checker, file *ast.SourceFile, result *response) (*aggregateProjection, *response) {
	if r.Declarations.MarkerProfile == nil {
		return nil, nil
	}
	failureAt := func(at *ast.SourceFile, node *ast.Node) (*aggregateProjection, *response) {
		failure := failAt("r01-d-aggregate-marker", "error", at, node)
		return nil, &failure
	}
	markerFile := program.GetSourceFile(r.Declarations.MarkerProfile.Path)
	if markerFile == nil || fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(markerFile.Text()))) != markerHash() {
		failure := fail("r02b-d-input", "input", r.Declarations.MarkerProfile.Path, span{})
		return nil, &failure
	}
	p := &aggregateProjection{facts: &aggregateFacts{Profile: markerVersion, Declarations: []aggregateDeclaration{}, SourceCoverage: []sourceCoverage{}}, classes: map[*ast.Symbol]*aggregateInfo{}, types: map[*checker.Type]*aggregateInfo{}, markers: map[*ast.Symbol]markerOrigin{}, categories: map[*ast.Symbol]string{}}
	for _, node := range markerFile.Statements.Nodes {
		if node.Kind == ast.KindInterfaceDeclaration {
			resolved := check.GetSymbolAtLocation(node.Name())
			if resolved == nil || len(resolved.Declarations) != 1 || resolved.Declarations[0] != node || len(node.AsInterfaceDeclaration().TypeParameters.Nodes) != 1 {
				return failureAt(markerFile, node)
			}
			continue
		}
		if node.Kind != ast.KindFunctionDeclaration || node.Name() == nil {
			return failureAt(markerFile, node)
		}
		resolved := check.GetSymbolAtLocation(node.Name())
		if resolved == nil || len(resolved.Declarations) != 1 || resolved.Declarations[0] != node {
			return failureAt(markerFile, node)
		}
		signatures := check.GetSignaturesOfType(check.GetTypeOfSymbol(resolved), checker.SignatureKindCall)
		if len(signatures) != 1 || signatures[0].Declaration() != node || len(signatures[0].TypeParameters()) != 1 || len(signatures[0].Parameters()) != 2 || check.GetReturnTypeOfSignature(signatures[0]) != check.GetVoidType() {
			return failureAt(markerFile, node)
		}
		category := "value"
		if node.Name().Text() == "tsfgComponent" {
			category = "component"
		}
		p.categories[resolved] = category
		p.markers[resolved] = markerOrigin{DeclarationPath: markerFile.FileName(), DeclarationRange: expressionOffset(markerFile, node), DeclarationSHA256: markerHash(), EntryID: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("tsfg.marker\x00"+markerVersion+"\x00"+category+"\x00"+markerHash()))), Version: markerVersion, ContextName: "ClassDecoratorContext", ContextArity: 1, Mode: "standard"}
	}
	if len(p.markers) != 2 {
		return failureAt(markerFile, markerFile.AsNode())
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindClassDeclaration {
			continue
		}
		decl := node.AsClassDeclaration()
		if decl.HeritageClauses != nil {
			failure := failAt("r01-d-aggregate-inheritance", "error", file, node)
			return nil, &failure
		}
		if node.Name() == nil || decl.TypeParameters != nil || ast.HasSyntacticModifier(node, ast.ModifierFlagsAbstract|ast.ModifierFlagsAmbient) {
			failure := failAt("r02b-d-unsupported", "unsupported", file, node)
			return nil, &failure
		}
		constructorCount := 0
		for _, member := range node.Members() {
			if member.Kind == ast.KindConstructor {
				constructorCount++
				if constructorCount > 1 {
					failure := failAt("r02b-d-unsupported", "unsupported", file, member)
					return nil, &failure
				}
			}
		}
		var marker *ast.Node
		if node.Modifiers() != nil {
			for _, modifier := range node.Modifiers().Nodes {
				if modifier.Kind == ast.KindDecorator {
					if marker != nil {
						return failureAt(file, modifier)
					}
					marker = modifier
				}
			}
		}
		if marker == nil || marker.AsDecorator().Expression.Kind != ast.KindIdentifier {
			return failureAt(file, node)
		}
		resolvedMarker := check.GetSymbolAtLocation(marker.AsDecorator().Expression)
		origin, exists := p.markers[resolvedMarker]
		if !exists {
			return failureAt(file, marker)
		}
		resolved := check.GetSymbolAtLocation(node.Name())
		if resolved == nil || len(resolved.Declarations) != 1 || resolved.Declarations[0] != node {
			failure := failAt("r01-d-aggregate-identity", "error", file, node)
			return nil, &failure
		}
		instance, static := check.GetDeclaredTypeOfSymbol(resolved), check.GetTypeOfSymbol(resolved)
		constructors := check.GetSignaturesOfType(static, checker.SignatureKindConstruct)
		if instance == nil || !instance.IsClass() || instance == static || len(check.GetSignaturesOfType(instance, checker.SignatureKindConstruct)) != 0 || len(constructors) != 1 || check.GetReturnTypeOfSignature(constructors[0]) != instance {
			failure := failAt("r01-d-aggregate-identity", "error", file, node)
			return nil, &failure
		}
		id := fmt.Sprintf("a%d", len(p.facts.Declarations)+1)
		s := symbol{ID: fmt.Sprintf("s%d", len(result.Symbols)+1), Name: node.Name().Text(), Kind: "class", Type: id, Range: offset(file, node.Name())}
		result.Symbols = append(result.Symbols, s)
		origin.Range = expressionOffset(file, marker)
		bodyStart := decl.Members.Pos() - 1
		// The official member list starts after the class opening brace.
		for bodyStart >= node.Pos() && file.Text()[bodyStart] != '{' {
			bodyStart--
		}
		if bodyStart < node.Pos() || node.End() <= bodyStart || file.Text()[node.End()-1] != '}' {
			return failureAt(file, node)
		}
		bodyRange := span{file.GetPositionMap().UTF8ToUTF16(bodyStart), file.GetPositionMap().UTF8ToUTF16(node.End())}
		p.facts.Declarations = append(p.facts.Declarations, aggregateDeclaration{ID: id, Name: s.Name, Symbol: s.ID, Category: p.categories[resolvedMarker], NominalKey: nominalKey{SourcePath: file.FileName(), DeclarationPath: []string{s.Name}, Category: p.categories[resolvedMarker]}, MarkerOrigin: origin, InstanceType: id, ConstructorType: "constructor:" + id, Fields: []aggregateField{}, Constructors: []aggregateConstructor{}, MemberCoverage: []sourceCoverage{}, Range: expressionOffset(file, node), BodyRange: bodyRange, NoInheritance: true})
		info := &aggregateInfo{node: node, symbol: resolved, instance: instance, static: static, checked: constructors[0], index: len(p.facts.Declarations) - 1, fields: map[*ast.Symbol]int{}}
		p.classes[resolved], p.types[instance] = info, info
	}
	// All class identities exist before resolving fields, so forward nested
	// declarations preserve official type objects without depending on order.
	for _, info := range p.ordered() {
		decl := &p.facts.Declarations[info.index]
		for _, member := range info.node.Members() {
			coverage := sourceCoverage{Kind: member.Kind.String(), Range: expressionOffset(file, member)}
			if member.Name() != nil {
				coverage.Name = member.Name().Text()
			}
			if member.Body() != nil {
				location := expressionOffset(file, member.Body())
				coverage.BodyRange = &location
			}
			decl.MemberCoverage = append(decl.MemberCoverage, coverage)
			if member.Kind == ast.KindConstructor {
				continue
			}
			// Static fields, methods and accessors are explicitly covered, then
			// rejected as unsupported executable members in this capability.
			if member.Kind != ast.KindPropertyDeclaration || ast.IsStatic(member) || ast.HasDecorators(member) || ast.HasSyntacticModifier(member, ast.ModifierFlagsAccessor) || member.Name() == nil || member.Name().Kind != ast.KindIdentifier {
				failure := failAt("r02b-d-unsupported", "unsupported", file, member)
				return nil, &failure
			}
			field := member.AsPropertyDeclaration()
			resolved := check.GetSymbolAtLocation(member.Name())
			if resolved == nil || len(resolved.Declarations) != 1 || resolved.Declarations[0] != member || check.GetPropertyOfType(info.instance, member.Name().Text()) != resolved {
				return failureAt(file, member)
			}
			t := p.typeName(check.GetTypeOfSymbol(resolved))
			if t == "" {
				t = scalar(check.GetTypeOfSymbol(resolved))
			}
			optional := field.PostfixToken != nil && field.PostfixToken.Kind == ast.KindQuestionToken
			if field.Type == nil || t == "" || optional {
				failure := failAt("r02b-d-unsupported", "unsupported", file, member)
				return nil, &failure
			}
			access := "public"
			if ast.HasSyntacticModifier(member, ast.ModifierFlagsPrivate) {
				access = "private"
			} else if ast.HasSyntacticModifier(member, ast.ModifierFlagsProtected) {
				access = "protected"
			}
			s := symbol{ID: fmt.Sprintf("s%d", len(result.Symbols)+1), Name: member.Name().Text(), Kind: "field", Type: t, Writable: !ast.HasSyntacticModifier(member, ast.ModifierFlagsReadonly), Range: offset(file, member.Name())}
			result.Symbols = append(result.Symbols, s)
			ordinal := len(decl.Fields)
			info.fields[resolved] = ordinal
			decl.Fields = append(decl.Fields, aggregateField{ID: fmt.Sprintf("%s.f%d", decl.ID, ordinal), Ordinal: ordinal, Name: s.Name, Symbol: s.ID, Type: t, Readonly: !s.Writable, Access: access, OptionalSyntax: optional, DefiniteAssertionSyntax: field.PostfixToken != nil && field.PostfixToken.Kind == ast.KindExclamationToken, Range: expressionOffset(file, member)})
			decl.MemberCoverage[len(decl.MemberCoverage)-1].Symbol = s.ID
		}
	}
	if cycle := p.cyclic(); cycle != nil {
		failure := failAt("r01-d-aggregate-cycle", "error", file, cycle.node)
		return nil, &failure
	}
	return p, nil
}

func (p *aggregateProjection) ordered() []*aggregateInfo {
	items := make([]*aggregateInfo, len(p.facts.Declarations))
	for _, item := range p.classes {
		items[item.index] = item
	}
	return items
}

func (p *aggregateProjection) typeName(t *checker.Type) string {
	if info := p.types[t]; info != nil {
		return p.facts.Declarations[info.index].ID
	}
	return ""
}

func (p *aggregateProjection) cyclic() *aggregateInfo {
	state := map[string]int{}
	byID := map[string]*aggregateInfo{}
	for _, info := range p.ordered() {
		byID[p.facts.Declarations[info.index].ID] = info
	}
	var visit func(string) *aggregateInfo
	visit = func(id string) *aggregateInfo {
		if state[id] == 1 {
			return byID[id]
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		for _, f := range p.facts.Declarations[byID[id].index].Fields {
			if byID[f.Type] != nil {
				if cycle := visit(f.Type); cycle != nil {
					return cycle
				}
			}
		}
		state[id] = 2
		return nil
	}
	for _, info := range p.ordered() {
		if cycle := visit(p.facts.Declarations[info.index].ID); cycle != nil {
			return cycle
		}
	}
	return nil
}

func (p *projection) aggregateType(t *checker.Type) string {
	if p.aggregates != nil {
		if name := p.aggregates.typeName(t); name != "" {
			return name
		}
	}
	return scalar(t)
}
