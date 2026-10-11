// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/tspath"
)

const markerVersion = "minimal-aggregates-v1"

// These declarations are compile-time markers. Their ordinary standard
// decorator signatures are checked by the official frontend; no decorator is
// executed and no replacement constructor is accepted.
const markerDeclarations = `// SPDX-License-Identifier: MIT
interface ClassDecoratorContext<C> {
  readonly kind: "class";
  readonly name: string | undefined;
  addInitializer(initializer: (this: C) => void): void;
}
declare function tsfgValue<C>(target: C, context: ClassDecoratorContext<C>): void;
declare function tsfgComponent<C>(target: C, context: ClassDecoratorContext<C>): void;
`

type markerEntry struct {
	Name     string `json:"name"`
	Category string `json:"category"`
}

type markerProfile struct {
	Version      string        `json:"version"`
	Path         string        `json:"path"`
	SHA256       string        `json:"sha256"`
	ContextName  string        `json:"contextName"`
	ContextArity int           `json:"contextArity"`
	Mode         string        `json:"mode"`
	Entries      []markerEntry `json:"entries"`
}

type markerOrigin struct {
	DeclarationPath   string `json:"declarationPath"`
	DeclarationRange  span   `json:"declarationRange"`
	DeclarationSHA256 string `json:"declarationSha256"`
	EntryID           string `json:"entryId"`
	Version           string `json:"version"`
	ContextName       string `json:"contextName"`
	ContextArity      int    `json:"contextArity"`
	Mode              string `json:"mode"`
	Range             span   `json:"range"`
}

type nominalKey struct {
	SourcePath      string   `json:"sourcePath"`
	DeclarationPath []string `json:"declarationPath"`
	Category        string   `json:"category"`
}

type sourceCoverage struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Symbol    string `json:"symbol,omitempty"`
	Range     span   `json:"range"`
	BodyRange *span  `json:"bodyRange"`
}

type fieldPermission struct {
	Readable        bool   `json:"readable"`
	Writable        bool   `json:"writable"`
	ConstructorSelf bool   `json:"constructorSelf"`
	ClassRef        string `json:"classRef"`
}

type aggregateField struct {
	ID                      string      `json:"id"`
	Ordinal                 int         `json:"ordinal"`
	Name                    string      `json:"name"`
	Symbol                  string      `json:"symbol"`
	Type                    string      `json:"type"`
	Readonly                bool        `json:"readonly"`
	Access                  string      `json:"access"`
	OptionalSyntax          bool        `json:"optionalSyntax"`
	DefiniteAssertionSyntax bool        `json:"definiteAssertionSyntax"`
	Initializer             *expression `json:"initializer"`
	Range                   span        `json:"range"`
}

type aggregateConstructor struct {
	ID                    string        `json:"id"`
	Kind                  string        `json:"kind"`
	ClassRef              string        `json:"classRef"`
	SignatureDeclaration  *span         `json:"signatureDeclaration"`
	Implementation        *span         `json:"implementation"`
	Parameters            []symbol      `json:"parameters"`
	Signature             signature     `json:"signature"`
	ResultType            string        `json:"resultType"`
	Body                  []statement   `json:"body"`
	ParameterInitializers []*expression `json:"parameterInitializers"`
	Range                 span          `json:"range"`
	BodyRange             *span         `json:"bodyRange"`
}

type aggregateDeclaration struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Symbol          string                 `json:"symbol"`
	Category        string                 `json:"category"`
	NominalKey      nominalKey             `json:"nominalKey"`
	MarkerOrigin    markerOrigin           `json:"markerOrigin"`
	InstanceType    string                 `json:"instanceType"`
	ConstructorType string                 `json:"constructorType"`
	Fields          []aggregateField       `json:"fields"`
	Constructors    []aggregateConstructor `json:"constructors"`
	MemberCoverage  []sourceCoverage       `json:"memberCoverage"`
	Range           span                   `json:"range"`
	BodyRange       span                   `json:"bodyRange"`
	NoInheritance   bool                   `json:"noInheritance"`
}

type aggregateFacts struct {
	Profile        string                 `json:"profile"`
	Declarations   []aggregateDeclaration `json:"declarations"`
	SourceCoverage []sourceCoverage       `json:"sourceCoverage"`
}

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

func aggregateFieldMutation(node *ast.Node) bool {
	node = ast.SkipParentheses(node)
	if node.Kind == ast.KindBinaryExpression {
		b := node.AsBinaryExpression()
		return ordinaryAssignmentOperator(b.OperatorToken.Kind) != "" && ast.SkipParentheses(b.Left).Kind == ast.KindPropertyAccessExpression
	}
	if node.Kind == ast.KindPrefixUnaryExpression {
		return ast.SkipParentheses(node.AsPrefixUnaryExpression().Operand).Kind == ast.KindPropertyAccessExpression && (node.AsPrefixUnaryExpression().Operator == ast.KindPlusPlusToken || node.AsPrefixUnaryExpression().Operator == ast.KindMinusMinusToken)
	}
	if node.Kind == ast.KindPostfixUnaryExpression {
		return ast.SkipParentheses(node.AsPostfixUnaryExpression().Operand).Kind == ast.KindPropertyAccessExpression
	}
	return false
}

// fieldExpr owns the actual resolved field symbols along an ordered path.
// A this read denotes a location; it does not assert initialized storage.
func (p *projection) fieldExpr(node *ast.Node, write bool) (*expression, response) {
	access := node.AsPropertyAccessExpression()
	if access.QuestionDotToken != nil {
		return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node)
	}
	var receiver *expression
	var failure response
	if access.Expression.Kind == ast.KindPropertyAccessExpression {
		receiver, failure = p.fieldExpr(access.Expression, false)
	} else {
		receiver, failure = p.expr(access.Expression, "")
	}
	if failure.Status != "" {
		return nil, failure
	}
	info := p.aggregates.types[p.check.GetTypeAtLocation(access.Expression)]
	if access.Expression.Kind == ast.KindThisKeyword {
		info = p.self
	}
	if info == nil {
		return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node)
	}
	resolved := p.check.GetSymbolAtLocation(access.Name())
	ordinal, exists := info.fields[resolved]
	if !exists {
		return nil, failAt("r01-d-aggregate-identity", "error", p.file, node)
	}
	decl := p.aggregates.facts.Declarations[info.index]
	field := decl.Fields[ordinal]
	self := p.self == info && access.Expression.Kind == ast.KindThisKeyword
	readable := p.check.IsPropertyAccessible(node, false, false, info.instance, resolved)
	writable := p.check.IsPropertyAccessible(node, false, true, info.instance, resolved) && (!field.Readonly || self)
	if !readable || write && !writable {
		return nil, failAt("r01-d-aggregate-write", "error", p.file, node)
	}
	path := []string{field.ID}
	if receiver.Kind == "field" {
		path = append(append([]string{}, receiver.FieldPath...), field.ID)
		receiver = receiver.Receiver
	}
	return &expression{Kind: "field", Type: field.Type, Receiver: receiver, FieldPath: path, Symbol: field.Symbol, Permission: &fieldPermission{Readable: readable, Writable: writable, ConstructorSelf: self, ClassRef: decl.ID}, Range: expressionOffset(p.file, node)}, response{}
}

func (p *projection) aggregateExpr(node *ast.Node, expected string) (*expression, response, bool) {
	if node == nil {
		return nil, response{}, false
	}
	finish := func(value *expression, failure response) (*expression, response, bool) {
		if failure.Status != "" {
			return nil, failure, true
		}
		if expected != "" && value.Type != expected {
			code := "r01-d-scalar-type"
			if strings.HasPrefix(expected, "a") || strings.HasPrefix(value.Type, "a") {
				code = "r01-d-aggregate-identity"
			}
			return nil, failAt(code, "error", p.file, node), true
		}
		return value, response{}, true
	}
	if node.Kind == ast.KindParenthesizedExpression {
		inner, failure, handled := p.aggregateExpr(node.AsParenthesizedExpression().Expression, expected)
		if !handled {
			return nil, response{}, false
		}
		if failure.Status != "" {
			return nil, failure, true
		}
		return &expression{Kind: "group", Type: inner.Type, Operand: inner, Range: expressionOffset(p.file, node)}, response{}, true
	}
	if node.Kind == ast.KindThisKeyword {
		if p.self == nil {
			return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node), true
		}
		id := p.aggregates.facts.Declarations[p.self.index].ID
		return finish(&expression{Kind: "constructionReceiver", Type: id, OwnerType: id, Range: expressionOffset(p.file, node)}, response{})
	}
	if node.Kind == ast.KindIdentifier {
		binding, exists := p.bindings[p.check.GetSymbolAtLocation(node)]
		if exists && strings.HasPrefix(binding.Type, "a") {
			return finish(&expression{Kind: "reference", Type: binding.Type, Symbol: binding.ID, Range: expressionOffset(p.file, node)}, response{})
		}
		if p.aggregates.classes[p.check.GetSymbolAtLocation(node)] != nil {
			return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node), true
		}
	}
	if node.Kind == ast.KindPropertyAccessExpression {
		value, failure := p.fieldExpr(node, false)
		return finish(value, failure)
	}
	if node.Kind == ast.KindNewExpression {
		call := node.AsNewExpression()
		if call.Expression.Kind != ast.KindIdentifier || call.TypeArguments != nil {
			return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node), true
		}
		info := p.aggregates.classes[p.check.GetSymbolAtLocation(call.Expression)]
		if info == nil {
			return nil, failAt("r01-d-aggregate-identity", "error", p.file, node), true
		}
		decl := p.aggregates.facts.Declarations[info.index]
		if len(decl.Constructors) != 1 || p.check.GetResolvedSignature(node).Declaration() != info.checked.Declaration() {
			return nil, failAt("r01-d-aggregate-identity", "error", p.file, node), true
		}
		ctor := decl.Constructors[0]
		var nodes []*ast.Node
		if call.Arguments != nil {
			nodes = call.Arguments.Nodes
		}
		if len(nodes) < info.checked.MinArgumentCount() || len(nodes) > len(ctor.Parameters) {
			return nil, failAt("r02b-d-type", "error", p.file, node), true
		}
		args := make([]expression, 0, len(nodes))
		for index, arg := range nodes {
			value, failure := p.expr(arg, ctor.Parameters[index].Type, ctor.Parameters[index].Presence)
			if failure.Status != "" {
				return nil, failure, true
			}
			args = append(args, *value)
		}
		return finish(&expression{Kind: "construct", Type: decl.ID, Target: ctor.ID, Arguments: &args, Signature: &ctor.Signature, Range: expressionOffset(p.file, node)}, response{})
	}
	if node.Kind == ast.KindCallExpression {
		call := node.AsCallExpression()
		if call.Expression.Kind != ast.KindIdentifier {
			return nil, response{}, false
		}
		target, exists := p.functions[p.check.GetSymbolAtLocation(call.Expression)]
		if !exists || !p.involvesAggregate(node) {
			return nil, response{}, false
		}
		if call.QuestionDotToken != nil || call.TypeArguments != nil || len(call.Arguments.Nodes) > len(target.Signature.Parameters) {
			return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node), true
		}
		args := []expression{}
		for index, arg := range call.Arguments.Nodes {
			var shape *parameterPresence
			if target.Signature.Presence != nil {
				shape = target.Signature.Presence[index]
			}
			value, failure := p.expr(arg, target.Signature.Parameters[index], shape)
			if failure.Status != "" {
				return nil, failure, true
			}
			args = append(args, *value)
		}
		value := &expression{Kind: "call", Type: target.Signature.ReturnType, Target: target.ID, Arguments: &args, Signature: &target.Signature, Range: expressionOffset(p.file, node)}
		if target.Conversion != nil {
			if len(args) != 1 {
				return nil, failAt("r01-d-conversion-entry", "error", p.file, node), true
			}
			value.Kind = "conversion"
			value.Conversion = target.Conversion
			value.Target = ""
			value.Operand = &args[0]
			value.Arguments = nil
			value.Signature = nil
		}
		return finish(value, response{})
	}
	if node.Kind == ast.KindBinaryExpression && p.involvesAggregate(node) {
		binary := node.AsBinaryExpression()
		if operator := ordinaryAssignmentOperator(binary.OperatorToken.Kind); operator != "" {
			leaf := ast.SkipParentheses(binary.Left)
			if leaf.Kind != ast.KindPropertyAccessExpression {
				return nil, response{}, false
			}
			left, failure := p.fieldExpr(leaf, true)
			if failure.Status != "" {
				return nil, failure, true
			}
			logical := operator == "&&=" || operator == "||="
			floatOperation := operator == "+=" || operator == "-=" || operator == "*=" || operator == "/=" || operator == "%="
			if logical && left.Type != "bool" || operator != "=" && operator != "??=" && !logical && !integerKind(left.Type) && !(p.integerTarget.floats && floatKind(left.Type) && floatOperation) {
				return nil, failAt("r01-d-scalar-type", "error", p.file, binary.Left), true
			}
			rightType := left.Type
			shift := operator == "<<=" || operator == ">>=" || operator == ">>>="
			if shift {
				rightType = "i32"
			}
			right, failure := p.expr(binary.Right, rightType)
			if failure.Status != "" {
				if shift && failure.Diagnostics[0].Code == "r01-d-scalar-type" {
					failure = failAt("r01-d-shift-count-type", "error", p.file, binary.Right)
				}
				return nil, failure, true
			}
			return finish(&expression{Kind: "fieldWrite", Type: left.Type, Operator: operator, Left: left, Right: right, Range: expressionOffset(p.file, node)}, response{})
		}
		op := integerOperator(binary.OperatorToken.Kind)
		if op == "" {
			return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node), true
		}
		comparison := op == "===" || op == "!==" || op == "<" || op == "<=" || op == ">" || op == ">="
		logical := op == "&&" || op == "||"
		shift := op == "<<" || op == ">>" || op == ">>>"
		context := expected
		if comparison || context == "" || context == "bool" {
			context = p.aggregateHint(binary.Left)
			if context == "" && !shift {
				context = p.aggregateHint(binary.Right)
			}
			if context == "" {
				context = "i32"
			}
		}
		if logical {
			if integerKind(p.aggregateHint(binary.Left)) || floatKind(p.aggregateHint(binary.Left)) {
				return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node), true
			}
			context = "bool"
		}
		left, failure := p.expr(binary.Left, context)
		if failure.Status != "" {
			return nil, failure, true
		}
		rightType := left.Type
		if shift {
			rightType = "i32"
		}
		right, failure := p.expr(binary.Right, rightType)
		if failure.Status != "" {
			if shift && failure.Diagnostics[0].Code == "r01-d-scalar-type" {
				failure = failAt("r01-d-shift-count-type", "error", p.file, binary.Right)
			}
			return nil, failure, true
		}
		t, kind := left.Type, "binary"
		if strings.HasPrefix(t, "a") {
			if op != "===" && op != "!==" {
				return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node), true
			}
			kind = "aggregateEquality"
		}
		floatOperation := p.integerTarget.floats && floatKind(t) && (comparison || op == "+" || op == "-" || op == "*" || op == "/" || op == "%")
		if !strings.HasPrefix(t, "a") && !logical && !integerKind(t) && !floatOperation && !(t == "bool" && (op == "===" || op == "!==")) {
			return nil, failAt("r01-d-scalar-type", "error", p.file, binary.Left), true
		}
		if comparison || logical {
			t = "bool"
		}
		if logical {
			kind = "shortCircuit"
		}
		return finish(&expression{Kind: kind, Type: t, Operator: op, Left: left, Right: right, Range: expressionOffset(p.file, node)}, response{})
	}
	if node.Kind == ast.KindPrefixUnaryExpression && p.involvesAggregate(node) {
		prefix := node.AsPrefixUnaryExpression()
		op := ""
		switch prefix.Operator {
		case ast.KindPlusToken:
			op = "+"
		case ast.KindMinusToken:
			op = "-"
		case ast.KindTildeToken:
			op = "~"
		case ast.KindExclamationToken:
			op = "!"
		}
		if op != "" {
			context := expected
			if context == "" {
				context = p.aggregateHint(prefix.Operand)
			}
			if op == "!" {
				hint := p.aggregateHint(prefix.Operand)
				if integerKind(hint) || floatKind(hint) {
					return nil, failAt("r02b-d-unsupported", "unsupported", p.file, node), true
				}
				context = "bool"
			}
			operand, failure := p.expr(prefix.Operand, context)
			if failure.Status != "" {
				return nil, failure, true
			}
			if op != "!" && !integerKind(operand.Type) && !(p.integerTarget.floats && floatKind(operand.Type) && (op == "+" || op == "-")) {
				return nil, failAt("r01-d-scalar-type", "error", p.file, prefix.Operand), true
			}
			return finish(&expression{Kind: "unary", Type: operand.Type, Operator: op, Operand: operand, Range: expressionOffset(p.file, node)}, response{})
		}
	}
	if aggregateFieldMutation(node) {
		var operand *ast.Node
		var operator ast.Kind
		prefix := node.Kind == ast.KindPrefixUnaryExpression
		if prefix {
			operand = node.AsPrefixUnaryExpression().Operand
			operator = node.AsPrefixUnaryExpression().Operator
		} else {
			operand = node.AsPostfixUnaryExpression().Operand
			operator = node.AsPostfixUnaryExpression().Operator
		}
		left, failure := p.fieldExpr(ast.SkipParentheses(operand), true)
		if failure.Status != "" {
			return nil, failure, true
		}
		if !integerKind(left.Type) && !floatKind(left.Type) {
			return nil, failAt("r01-d-scalar-type", "error", p.file, node), true
		}
		op := "++"
		if operator == ast.KindMinusMinusToken {
			op = "--"
		}
		return finish(&expression{Kind: "fieldUpdate", Type: left.Type, Operator: op, Prefix: &prefix, Operand: left, Range: expressionOffset(p.file, node)}, response{})
	}
	return nil, response{}, false
}

func (p *projection) aggregateHint(node *ast.Node) string {
	node = ast.SkipParentheses(node)
	if integerLiteralNode(p.file, node) {
		return ""
	}
	if node.Kind == ast.KindPrefixUnaryExpression {
		if node.AsPrefixUnaryExpression().Operator == ast.KindExclamationToken {
			return "bool"
		}
		return p.aggregateHint(node.AsPrefixUnaryExpression().Operand)
	}
	if node.Kind == ast.KindBinaryExpression {
		binary := node.AsBinaryExpression()
		switch integerOperator(binary.OperatorToken.Kind) {
		case "===", "!==", "<", "<=", ">", ">=", "&&", "||":
			return "bool"
		}
		if hint := p.aggregateHint(binary.Left); hint != "" {
			return hint
		}
		return p.aggregateHint(binary.Right)
	}
	if node.Kind == ast.KindThisKeyword && p.self != nil {
		return p.aggregates.facts.Declarations[p.self.index].ID
	}
	if node.Kind == ast.KindIdentifier {
		if binding := p.bindings[p.check.GetSymbolAtLocation(node)]; binding.Type != "" {
			return binding.Type
		}
	}
	if node.Kind == ast.KindCallExpression && node.AsCallExpression().Expression.Kind == ast.KindIdentifier {
		if target := p.functions[p.check.GetSymbolAtLocation(node.AsCallExpression().Expression)]; target.Signature.ReturnType != "" {
			return target.Signature.ReturnType
		}
	}
	t := p.check.GetTypeAtLocation(node)
	if name := p.aggregates.typeName(t); name != "" {
		return name
	}
	return scalar(t)
}

func (p *projection) involvesAggregate(node *ast.Node) bool {
	var visit func(*ast.Node) bool
	visit = func(n *ast.Node) bool {
		if n.Kind == ast.KindPropertyAccessExpression || n.Kind == ast.KindThisKeyword || n.Kind == ast.KindNewExpression {
			return true
		}
		if n.Kind == ast.KindIdentifier {
			if strings.HasPrefix(p.bindings[p.check.GetSymbolAtLocation(n)].Type, "a") {
				return true
			}
			if strings.HasPrefix(p.functions[p.check.GetSymbolAtLocation(n)].Signature.ReturnType, "a") {
				return true
			}
		}
		return n.ForEachChild(visit)
	}
	return visit(node)
}

func (a *aggregateProjection) parameter(file *ast.SourceFile, check *checker.Checker, node *ast.Node) (string, *parameterPresence, bool) {
	parameter := node.AsParameterDeclaration()
	if parameter.Type != nil && parameter.DotDotDotToken == nil && node.Name() != nil && node.Name().Kind == ast.KindIdentifier && !ast.IsParameterPropertyDeclaration(node, node.Parent) {
		if t := a.typeName(check.GetTypeFromTypeNode(parameter.Type)); t != "" {
			if parameter.QuestionToken != nil {
				return "", nil, false
			}
			var presence *parameterPresence
			if parameter.Initializer != nil {
				presence = &parameterPresence{Default: true, Optional: true, Undefined: true, Range: offset(file, node), Annotation: expressionOffset(file, parameter.Type)}
			}
			return t, presence, true
		}
	}
	return checkedParameter(file, check, node)
}

func (a *aggregateProjection) constructorHeaders(file *ast.SourceFile, check *checker.Checker, result *response) *response {
	for _, info := range a.ordered() {
		decl := &a.facts.Declarations[info.index]
		ctor := aggregateConstructor{ID: fmt.Sprintf("c%d", info.index+1), Kind: "implicitDefault", ClassRef: decl.ID, Parameters: []symbol{}, Signature: signature{Parameters: []string{}, ReturnType: decl.ID}, ResultType: decl.ID, Body: []statement{}, ParameterInitializers: []*expression{}, Range: decl.Range}
		implementation := ast.GetFirstConstructorWithBody(info.node)
		if implementation == nil {
			if info.checked.Declaration() != nil || len(info.checked.Parameters()) != 0 {
				failure := failAt("r01-d-aggregate-identity", "error", file, info.node)
				return &failure
			}
		} else {
			if info.checked.Declaration() != implementation || implementation.Body().Kind != ast.KindBlock {
				failure := failAt("r02b-d-unsupported", "unsupported", file, implementation)
				return &failure
			}
			ctor.Kind = "explicit"
			location := expressionOffset(file, implementation)
			body := expressionOffset(file, implementation.Body())
			ctor.SignatureDeclaration = &location
			ctor.Implementation = &location
			ctor.BodyRange = &body
			parameters := implementation.AsConstructorDeclaration().Parameters.Nodes
			if len(parameters) != len(info.checked.Parameters()) {
				failure := failAt("r01-d-aggregate-identity", "error", file, implementation)
				return &failure
			}
			hasPresence := false
			for index, node := range parameters {
				t, presence, valid := a.parameter(file, check, node)
				if !valid || check.GetSymbolAtLocation(node.Name()) != info.checked.Parameters()[index] {
					failure := failAt("r02b-d-unsupported", "unsupported", file, node)
					return &failure
				}
				s := symbol{ID: fmt.Sprintf("s%d", len(result.Symbols)+1), Name: node.Name().Text(), Kind: "parameter", Type: t, Writable: true, Presence: presence, Range: offset(file, node.Name())}
				result.Symbols = append(result.Symbols, s)
				ctor.Parameters = append(ctor.Parameters, s)
				ctor.Signature.Parameters = append(ctor.Signature.Parameters, t)
				ctor.Signature.Presence = append(ctor.Signature.Presence, presence)
				ctor.ParameterInitializers = append(ctor.ParameterInitializers, nil)
				hasPresence = hasPresence || presence != nil
			}
			if !hasPresence {
				ctor.Signature.Presence = nil
			}
		}
		decl.Constructors = append(decl.Constructors, ctor)
	}
	return nil
}

func (a *aggregateProjection) constructorBodies(file *ast.SourceFile, check *checker.Checker, result *response, functions map[*ast.Symbol]functionInfo, target *integerTarget) *response {
	for _, info := range a.ordered() {
		decl := &a.facts.Declarations[info.index]
		ctor := &decl.Constructors[0]
		project := projection{file: file, check: check, result: result, bindings: map[*ast.Symbol]symbol{}, functions: functions, integerTarget: target, aggregates: a, self: info}
		implementation := ast.GetFirstConstructorWithBody(info.node)
		if implementation != nil {
			for index, node := range implementation.AsConstructorDeclaration().Parameters.Nodes {
				project.bindings[check.GetSymbolAtLocation(node.Name())] = ctor.Parameters[index]
			}
			for index, node := range implementation.AsConstructorDeclaration().Parameters.Nodes {
				if initializer := node.AsParameterDeclaration().Initializer; initializer != nil {
					value, failure := project.expr(initializer, ctor.Parameters[index].Type)
					if failure.Status != "" {
						return &failure
					}
					ctor.ParameterInitializers[index] = value
				}
			}
		}
		for _, member := range info.node.Members() {
			if member.Kind != ast.KindPropertyDeclaration || member.AsPropertyDeclaration().Initializer == nil {
				continue
			}
			ordinal := info.fields[check.GetSymbolAtLocation(member.Name())]
			value, failure := project.expr(member.AsPropertyDeclaration().Initializer, decl.Fields[ordinal].Type)
			if failure.Status != "" {
				return &failure
			}
			decl.Fields[ordinal].Initializer = value
		}
		if implementation != nil {
			body, failure := project.statements(implementation.Body().AsBlock().Statements.Nodes, decl.ID)
			if failure.Status != "" {
				return &failure
			}
			ctor.Body = body
		}
	}
	return nil
}

// Only a fully projected, present scalar expression qualifies an upstream
// branded-numeric assignment diagnostic. Other diagnostics, particularly
// readonly/visibility/initialization errors, remain official errors.
func (a *aggregateProjection) contextDiagnostic(file *ast.SourceFile, check *checker.Checker, d *ast.Diagnostic, result *response, functions map[*ast.Symbol]functionInfo, target *integerTarget) (bool, *response) {
	if d.File() != file || d.Code() != 2322 && d.Code() != 2345 {
		return false, nil
	}
	bindings := map[*ast.Symbol]symbol{}
	var bind func(*ast.Node) bool
	bind = func(node *ast.Node) bool {
		var annotation *ast.Node
		if node.Kind == ast.KindParameter {
			annotation = node.AsParameterDeclaration().Type
		}
		if node.Kind == ast.KindVariableDeclaration {
			annotation = node.AsVariableDeclaration().Type
		}
		if node.Name() != nil && (annotation != nil || node.Kind == ast.KindVariableDeclaration) {
			var checked *checker.Type
			if annotation != nil {
				checked = check.GetTypeFromTypeNode(annotation)
			} else {
				checked = check.GetTypeAtLocation(node.Name())
			}
			t := a.typeName(checked)
			if t == "" {
				t = scalar(checked)
			}
			if annotation == nil && node.Kind == ast.KindVariableDeclaration && node.AsVariableDeclaration().Initializer != nil {
				// The checker sees numeric initializers as number. Diagnostic
				// qualification must retain the owned local's default i32 (or
				// other inferred owned type), exactly as body projection does.
				project := projection{file: file, check: check, result: result, bindings: bindings, functions: functions, integerTarget: target, aggregates: a}
				if value, failure := project.expr(node.AsVariableDeclaration().Initializer, ""); failure.Status == "" {
					t = value.Type
				}
			}
			if t != "" {
				bindings[check.GetSymbolAtLocation(node.Name())] = symbol{Type: t, Writable: true}
			}
		}
		return node.ForEachChild(bind)
	}
	file.AsNode().ForEachChild(bind)
	var handled bool
	var failure *response
	var visit func(*ast.Node) bool
	visit = func(node *ast.Node) bool {
		if d.Pos() < node.Pos() || d.End() > node.End() {
			return false
		}
		var candidate *ast.Node
		expected := ""
		if d.Code() == 2322 && node.Kind == ast.KindPropertyDeclaration {
			candidate = node.AsPropertyDeclaration().Initializer
			expected = scalar(check.GetTypeOfSymbol(check.GetSymbolAtLocation(node.Name())))
		}
		if d.Code() == 2322 && node.Kind == ast.KindVariableDeclaration && node.AsVariableDeclaration().Type != nil {
			candidate = node.AsVariableDeclaration().Initializer
			expected = scalar(check.GetTypeFromTypeNode(node.AsVariableDeclaration().Type))
		}
		if d.Code() == 2322 && node.Kind == ast.KindReturnStatement {
			if function := ast.FindAncestor(node, ast.IsFunctionDeclaration); function != nil && function.AsFunctionDeclaration().Type != nil {
				candidate = node.AsReturnStatement().Expression
				expected = scalar(check.GetTypeFromTypeNode(function.AsFunctionDeclaration().Type))
			}
		}
		if d.Code() == 2322 && node.Kind == ast.KindBinaryExpression && node.AsBinaryExpression().OperatorToken.Kind == ast.KindEqualsToken && ast.SkipParentheses(node.AsBinaryExpression().Left).Kind == ast.KindPropertyAccessExpression {
			candidate = node.AsBinaryExpression().Right
			expected = scalar(check.GetTypeAtLocation(node.AsBinaryExpression().Left))
		}
		if d.Code() == 2322 && node.Kind == ast.KindParameter && ast.FindAncestor(node, ast.IsConstructorDeclaration) != nil {
			candidate = node.AsParameterDeclaration().Initializer
			expected = scalar(check.GetTypeOfSymbol(check.GetSymbolAtLocation(node.Name())))
		}
		if d.Code() == 2345 && node.Kind == ast.KindNewExpression {
			call := node.AsNewExpression()
			info := a.classes[check.GetSymbolAtLocation(call.Expression)]
			if info != nil && call.Arguments != nil {
				for index, arg := range call.Arguments.Nodes {
					if index < len(info.checked.Parameters()) && d.Pos() >= arg.Pos() && d.End() <= arg.End() {
						candidate = arg
						expected = scalar(check.GetTypeOfSymbol(info.checked.Parameters()[index]))
						break
					}
				}
			}
		}
		if d.Code() == 2345 && node.Kind == ast.KindCallExpression {
			call := node.AsCallExpression()
			if call.Expression.Kind == ast.KindIdentifier {
				if target, exists := functions[check.GetSymbolAtLocation(call.Expression)]; exists && call.Arguments != nil {
					for index, arg := range call.Arguments.Nodes {
						if index < len(target.Signature.Parameters) && !strings.HasPrefix(target.Signature.Parameters[index], "a") && d.Pos() >= arg.Pos() && d.End() <= arg.End() {
							candidate = arg
							expected = target.Signature.Parameters[index]
							break
						}
					}
				}
			}
		}
		if candidate != nil && expected != "" {
			var self *aggregateInfo
			if class := ast.FindAncestor(node, ast.IsClassLike); class != nil {
				self = a.classes[check.GetSymbolAtLocation(class.Name())]
			}
			p := projection{file: file, check: check, result: result, bindings: bindings, functions: functions, integerTarget: target, aggregates: a, self: self}
			value, owned := p.expr(candidate, expected)
			if owned.Status != "" {
				handled = true
				failure = &owned
				return true
			}
			if permittedPresence(value, nil) {
				handled = true
				return true
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
	return handled, failure
}

func (a *aggregateProjection) coverage(file *ast.SourceFile, check *checker.Checker, functions map[*ast.Symbol]functionInfo) *response {
	for _, node := range file.Statements.Nodes {
		if node.Kind == ast.KindClassDeclaration {
			info := a.classes[check.GetSymbolAtLocation(node.Name())]
			if info == nil {
				failure := failAt("r01-d-aggregate-identity", "error", file, node)
				return &failure
			}
			decl := a.facts.Declarations[info.index]
			body := decl.BodyRange
			a.facts.SourceCoverage = append(a.facts.SourceCoverage, sourceCoverage{Kind: "class", Name: decl.Name, Symbol: decl.Symbol, Range: decl.Range, BodyRange: &body})
			continue
		}
		if node.Kind != ast.KindFunctionDeclaration || node.Name() == nil || node.Body() == nil {
			failure := failAt("r02b-d-unsupported", "unsupported", file, node)
			return &failure
		}
		info, exists := functions[check.GetSymbolAtLocation(node.Name())]
		if !exists || info.Declaration != node || info.Conversion != nil || info.ID == "" {
			failure := failAt("r01-d-aggregate-identity", "error", file, node)
			return &failure
		}
		body := expressionOffset(file, node.Body())
		a.facts.SourceCoverage = append(a.facts.SourceCoverage, sourceCoverage{Kind: "function", Name: node.Name().Text(), Symbol: info.ID, Range: offset(file, node), BodyRange: &body})
	}
	return nil
}
