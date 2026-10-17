// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strconv"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

// These local JSON views observe the public seam. They neither construct Source
// success facts nor assign identities; same-named declarations stay unrenamed.
type closureIdentitySignature struct {
	Parameters      []string
	ReturnType      string
	ReturnSignature *closureIdentitySignature
}

type closureIdentitySymbol struct {
	ID, Name, Kind, Type, LexicalOwner string
	Writable                           bool
	Range                              span
	Signature                          *closureIdentitySignature
}

type closureIdentityExpression struct {
	Kind, Type, Operator, Symbol string
	Target                       *string
	Callee, Left, Right, Operand *closureIdentityExpression
	Arguments                    []closureIdentityExpression
	Signature                    *closureIdentitySignature
	Range                        span
}

type closureIdentityStatement struct {
	Kind                               string
	Symbol                             *closureIdentitySymbol
	Initializer, Expression, Condition *closureIdentityExpression
	Body, ThenBody, ElseBody           []closureIdentityStatement
	Range                              span
}

type closureIdentityFunction struct {
	Name, Symbol, ReturnType, LexicalParent string
	Parameters                              []closureIdentitySymbol
	ReturnSignature                         *closureIdentitySignature
	Body                                    []closureIdentityStatement
	Captures                                []struct {
		Symbol      string
		Read, Write *bool
	}
	Range span
}

type closureIdentityResponse struct {
	Status      string
	Inputs      []string
	Functions   []closureIdentityFunction
	Symbols     []closureIdentitySymbol
	Diagnostics []json.RawMessage
}

func TestFrontendCommandSameNamedClosureIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		blocks       bool
	}{
		{"differentLexicalParents", "export function entry(seed: bool): bool { function left(input: bool): () => bool { function read(): bool { return input; } return read; } function right(input: bool): () => bool { function read(): bool { return !input; } return read; } const leftAlias = left(seed); const rightAlias = right(seed); return leftAlias() && rightAlias(); }", false},
		{"sameParentDifferentBlocks", "export function entry(seed: bool): bool { if (seed) { function read(): bool { return seed; } return read(); } else { function read(): bool { return !seed; } return read(); } }", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := integerRequest(tc.source)
			digest := sha256.Sum256([]byte(r.Files[1].Content))
			if r.Declarations.Version != "minimal-integers-v3" || r.Files[1].Path != "/src/minimal-integers-v3.d.ts" || hex.EncodeToString(digest[:]) != "9fc1955899e303b5815590f7150f60662f3cece0a633c94a2a06669cf509a96e" {
				t.Fatal("closure identity requires the existing minimal-integers-v3 declaration framework")
			}
			var anchors map[string]span
			if !t.Run("officialDiagnosticsSymbolsAndResolvedSignatures", func(t *testing.T) {
				anchors = closureIdentityOfficial(t, r, tc.blocks)
			}) {
				t.Fatal("independent official oracle failed; owned command was not attempted")
			}
			t.Run("ownedProjection", func(t *testing.T) {
				result := closureIdentityCommand(t, r)
				closureIdentityOwned(t, tc.source, result, anchors, tc.blocks)
			})
		})
	}
}

// The official Program is fresh and receives only original request files. AST
// structure and official Symbols/signatures select every expected declaration.
// No producer classifier, binding map, closure analysis or ID algorithm is used.
func closureIdentityOfficial(t *testing.T, r request, blocks bool) map[string]span {
	t.Helper()
	contents := map[string]string{}
	for _, file := range r.Files {
		contents[file.Path] = file.Content
	}
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
			FileNames:       append([]string{r.RootFile}, r.Declarations.Paths...),
			CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext},
		}},
		Host:           compiler.NewCompilerHost(r.SourceRoot, vfstest.FromMap(contents, true), r.SourceRoot, nil, nil),
		SingleThreaded: core.TSTrue,
	})
	file := program.GetSourceFile(r.RootFile)
	closureIdentityRequire(t, !(file == nil || file.Text() != r.Files[0].Content), "official Program lost the unmodified source")
	if d := program.GetSyntacticDiagnostics(t.Context(), nil); len(d) != 0 {
		t.Fatalf("official syntax diagnostics: %+v", d)
	}
	if d := program.GetSemanticDiagnostics(t.Context(), nil); len(d) != 0 {
		t.Fatalf("official semantic diagnostics: %+v", d)
	}
	if d := program.GetProgramDiagnostics(); len(d) != 0 {
		t.Fatalf("official program diagnostics: %+v", d)
	}
	if d := program.GetGlobalDiagnostics(t.Context()); len(d) != 0 {
		t.Fatalf("official global diagnostics: %+v", d)
	}
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	anchors := map[string]span{}
	anchor := func(label string, node *ast.Node) {
		positions := file.GetPositionMap()
		anchors[label] = span{Start: positions.UTF8ToUTF16(node.Pos()), End: positions.UTF8ToUTF16(node.End())}
	}
	seen := map[*ast.Symbol]bool{}
	bind := func(label string, definition *ast.Node, uses ...*ast.Node) {
		t.Helper()
		binding := check.GetSymbolAtLocation(definition)
		if binding == nil || seen[binding] {
			t.Fatalf("official merged or omitted distinct binding %s", label)
		}
		seen[binding] = true
		anchor(label, definition)
		for _, use := range uses {
			if check.GetSymbolAtLocation(use) != binding {
				t.Fatalf("official use lost original binding %s", label)
			}
		}
	}
	body := func(node *ast.Node) []*ast.Node {
		t.Helper()
		closureIdentityRequire(t, !(node.Kind != ast.KindFunctionDeclaration || node.Body() == nil || node.Body().Kind != ast.KindBlock), "official function body missing")
		return node.Body().AsBlock().Statements.Nodes
	}
	returned := func(node *ast.Node) *ast.Node {
		t.Helper()
		closureIdentityRequire(t, !(node.Kind != ast.KindReturnStatement || node.AsReturnStatement().Expression == nil), "official return missing")
		return node.AsReturnStatement().Expression
	}
	call := func(node, declaration *ast.Node, parameters int, functionResult bool) {
		t.Helper()
		closureIdentityRequire(t, !(node.Kind != ast.KindCallExpression), "official call missing")
		resolved := check.GetResolvedSignature(node)
		signatures := check.GetSignaturesOfType(check.GetTypeAtLocation(node.AsCallExpression().Expression), checker.SignatureKindCall)
		if resolved == nil || resolved.Declaration() != declaration || len(signatures) != 1 || signatures[0].Declaration() != declaration || len(resolved.Parameters()) != parameters || resolved.MinArgumentCount() != parameters || len(resolved.TypeParameters()) != 0 || resolved.ThisParameter() != nil || resolved.HasRestParameter() {
			t.Fatal("official call lost its actual resolved signature declaration")
		}
		result := check.GetReturnTypeOfSignature(resolved)
		if functionResult {
			leaves := check.GetSignaturesOfType(result, checker.SignatureKindCall)
			closureIdentityRequire(t, !(len(leaves) != 1 || len(leaves[0].Parameters()) != 0 || !check.GetReturnTypeOfSignature(leaves[0]).IsBooleanLike()), "official factory must return a zero-parameter bool callable")
		} else if !result.IsBooleanLike() {
			t.Fatal("official read call must return bool")
		}
		closureIdentityRequire(t, !(parameters == 1 && !check.GetTypeOfSymbol(resolved.Parameters()[0]).IsBooleanLike()), "official factory parameter must be bool")
	}
	closureIdentityRequire(t, !(len(file.Statements.Nodes) != 1), "official entry declaration count changed")
	entry := file.Statements.Nodes[0]
	entryBody := body(entry)
	anchor("entry", entry)
	bind("entryDefinition", entry.Name())
	closureIdentityRequire(t, !(len(entry.AsFunctionDeclaration().Parameters.Nodes) != 1), "official seed parameter missing")
	seed := entry.AsFunctionDeclaration().Parameters.Nodes[0].Name()
	var readers [2]*ast.Node
	var owners [2]*ast.Node
	var uses [2]*ast.Node
	var seedUses []*ast.Node
	if blocks {
		closureIdentityRequire(t, !(len(entryBody) != 1 || entryBody[0].Kind != ast.KindIfStatement), "official branch structure changed")
		branch := entryBody[0].AsIfStatement()
		seedUses = append(seedUses, branch.Expression)
		for index, scope := range []*ast.Node{branch.ThenStatement, branch.ElseStatement} {
			closureIdentityRequire(t, !(scope == nil || scope.Kind != ast.KindBlock), "official declaration scope missing")
			statements := scope.AsBlock().Statements.Nodes
			closureIdentityRequire(t, !(len(statements) != 2), "official branch declaration/return order changed")
			readers[index], owners[index] = statements[0], entry
			uses[index] = returned(statements[1]).AsCallExpression().Expression
			call(returned(statements[1]), readers[index], 0, false)
			anchor("scope"+strconv.Itoa(index), scope)
		}
	} else {
		closureIdentityRequire(t, !(len(entryBody) != 5), "official factory/alias/return order changed")
		binary := returned(entryBody[4])
		closureIdentityRequire(t, !(binary.Kind != ast.KindBinaryExpression || binary.AsBinaryExpression().OperatorToken.Kind != ast.KindAmpersandAmpersandToken), "official ordered conjunction missing")
		for index, aliasCall := range []*ast.Node{binary.AsBinaryExpression().Left, binary.AsBinaryExpression().Right} {
			key := strconv.Itoa(index)
			factory := entryBody[index]
			factoryBody := body(factory)
			closureIdentityRequire(t, !(len(factoryBody) != 2 || len(factory.AsFunctionDeclaration().Parameters.Nodes) != 1 || factory.Name().Text() != []string{"left", "right"}[index]), "official factory declaration changed")
			readers[index], owners[index], uses[index] = factoryBody[0], factory, returned(factoryBody[1])
			aliasStatement := entryBody[index+2]
			closureIdentityRequire(t, !(aliasStatement.Kind != ast.KindVariableStatement), "official alias declaration missing")
			locals := aliasStatement.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
			closureIdentityRequire(t, !(len(locals) != 1), "official alias must retain one declaration")
			alias := locals[0]
			initializer := alias.AsVariableDeclaration().Initializer
			call(initializer, factory, 1, true)
			call(aliasCall, factory.AsFunctionDeclaration().Type, 0, false)
			closureIdentityRequire(t, !(len(initializer.AsCallExpression().Arguments.Nodes) != 1 || len(aliasCall.AsCallExpression().Arguments.Nodes) != 0), "official ordered call arguments changed")
			bind("factory"+key+"Definition", factory.Name(), initializer.AsCallExpression().Expression)
			bind("alias"+key, alias.Name(), aliasCall.AsCallExpression().Expression)
			seedUses = append(seedUses, initializer.AsCallExpression().Arguments.Nodes[0])
			anchor("factory"+key, factory)
			anchor("aliasDeclaration"+key, alias)
		}
	}
	nearestFunction := func(node *ast.Node) *ast.Node {
		for node = node.Parent; node != nil; node = node.Parent {
			if node.Kind == ast.KindFunctionDeclaration {
				return node
			}
		}
		return nil
	}
	closureIdentityRequire(t, !(readers[0] == readers[1] || readers[0].Parent == readers[1].Parent), "official read declarations must occupy distinct declaration scopes")
	for index, reader := range readers {
		key := strconv.Itoa(index)
		readerBody := body(reader)
		closureIdentityRequire(t, !(reader.Name().Text() != "read" || len(reader.AsFunctionDeclaration().Parameters.Nodes) != 0 || len(readerBody) != 1 || nearestFunction(reader) != owners[index]), "official read identity or lexical parent changed")
		binding := check.GetSymbolAtLocation(reader.Name())
		closureIdentityRequire(t, !(binding == nil || binding.ValueDeclaration != reader || len(binding.Declarations) != 1 || binding.Declarations[0] != reader), "official read Symbol lost its unique original declaration")
		bind("read"+key+"Definition", reader.Name(), uses[index])
		capture := returned(readerBody[0])
		if index == 1 {
			closureIdentityRequire(t, !(capture.Kind != ast.KindPrefixUnaryExpression || capture.AsPrefixUnaryExpression().Operator != ast.KindExclamationToken), "official second reader lost negation")
			capture = capture.AsPrefixUnaryExpression().Operand
		}
		if blocks {
			seedUses = append(seedUses, capture)
		} else {
			bind("input"+key, owners[index].AsFunctionDeclaration().Parameters.Nodes[0].Name(), capture)
		}
		anchor("read"+key, reader)
	}
	bind("seed", seed, seedUses...)
	return anchors
}

func closureIdentityCommand(t *testing.T, r request) closureIdentityResponse {
	t.Helper()
	input, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	original, err := json.Marshal(map[string]any{"rootFile": r.RootFile, "files": r.Files})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	commandErr := runFrontendCommand(t.Context(), nil, bytes.NewReader(input), &output)
	t.Logf("TSFG_CLOSURE_IDENTITY_REQUEST_V1 %s", base64.StdEncoding.EncodeToString(input))
	t.Logf("TSFG_CLOSURE_IDENTITY_RESPONSE_V1 %s", base64.StdEncoding.EncodeToString(output.Bytes()))
	t.Logf("TSFG_CLOSURE_IDENTITY_ORIGINAL_V1 %s", base64.StdEncoding.EncodeToString(original))
	if commandErr != nil {
		t.Fatalf("public frontend command failed: %v; raw=%s", commandErr, output.Bytes())
	}
	var result closureIdentityResponse
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("invalid public JSON: %v; raw=%s", err, output.Bytes())
	}
	return result
}

func closureIdentityOwned(t *testing.T, source string, result closureIdentityResponse, anchors map[string]span, blocks bool) {
	t.Helper()
	if result.Status != "ok" || len(result.Diagnostics) != 0 {
		t.Fatalf("officially valid same-named closures must project owned facts: %+v", result)
	}
	count, symbols := 5, 10
	if blocks {
		count, symbols = 3, 4
	}
	if len(result.Functions) != count || len(result.Symbols) != symbols || !reflect.DeepEqual(result.Inputs, []string{"/src/main.ts", "/src/minimal-integers-v3.d.ts"}) {
		t.Fatalf("incomplete original inputs or closure identities: %+v", result)
	}
	definitions := map[string]closureIdentitySymbol{}
	for _, s := range result.Symbols {
		if s.ID == "" || definitions[s.ID].ID != "" {
			t.Fatalf("missing or duplicate owned binding: %+v", s)
		}
		definitions[s.ID] = s
	}
	at := func(label string) closureIdentityFunction {
		t.Helper()
		for _, fn := range result.Functions {
			if fn.Range == anchors[label] {
				return fn
			}
		}
		t.Fatalf("missing function at independent official declaration %s", label)
		return closureIdentityFunction{}
	}
	checkSymbol := func(s closureIdentitySymbol, label, name, kind, typ, owner string, writable bool) {
		t.Helper()
		if s.ID == "" || s.Range != anchors[label] || s.Name != name || s.Kind != kind || s.Type != typ || s.LexicalOwner != owner || s.Writable != writable || !reflect.DeepEqual(definitions[s.ID], s) {
			t.Fatalf("owned original binding changed at %s: %+v", label, s)
		}
	}
	checkFunction := func(fn closureIdentityFunction, label, name, typ, parent string, parameters, statements int) {
		t.Helper()
		if fn.Name != name || fn.ReturnType != typ || fn.LexicalParent != parent || len(fn.Parameters) != parameters || len(fn.Body) != statements {
			t.Fatalf("owned function lost original name, lexical parent or body: %+v", fn)
		}
		checkSymbol(definitions[fn.Symbol], label+"Definition", name, "function", typ, parent, false)
	}
	declaration := func(s closureIdentityStatement, fn closureIdentityFunction) {
		t.Helper()
		if s.Kind != "functionDeclaration" || s.Symbol == nil || s.Range != fn.Range || !reflect.DeepEqual(*s.Symbol, definitions[fn.Symbol]) {
			t.Fatalf("function declaration lost its original scope position: %+v", s)
		}
	}
	leaf := &closureIdentitySignature{Parameters: []string{}, ReturnType: "bool"}
	factorySignature := &closureIdentitySignature{Parameters: []string{"bool"}, ReturnType: "function", ReturnSignature: leaf}
	signature := func(got, want *closureIdentitySignature) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("owned callable shape changed: got=%+v want=%+v", got, want)
		}
	}
	reference := func(e *closureIdentityExpression, binding closureIdentitySymbol) {
		t.Helper()
		if e == nil || e.Kind != "reference" || e.Type != binding.Type || e.Symbol != binding.ID || e.Target != nil || e.Callee != nil || sourceText(t, source, e.Range) != binding.Name {
			t.Fatalf("reference lost actual binding: %+v", e)
		}
		signature(e.Signature, binding.Signature)
	}
	returned := func(s closureIdentityStatement) *closureIdentityExpression {
		t.Helper()
		closureIdentityRequire(t, !(s.Kind != "return" || s.Expression == nil), "owned return missing")
		return s.Expression
	}
	direct := func(e *closureIdentityExpression, target, typ string, args int) {
		t.Helper()
		if e == nil || e.Kind != "call" || e.Type != typ || e.Target == nil || *e.Target != target || e.Callee != nil || len(e.Arguments) != args {
			t.Fatalf("direct call lost original declaration: %+v", e)
		}
	}
	entry := at("entry")
	entryStatements := 5
	if blocks {
		entryStatements = 1
	}
	checkFunction(entry, "entry", "entry", "bool", "", 1, entryStatements)
	seed := entry.Parameters[0]
	checkSymbol(seed, "seed", "seed", "parameter", "bool", entry.Symbol, true)
	closureIdentityRequire(t, !(len(entry.Captures) != 0), "entry gained transitive captures")
	reader0, reader1 := at("read0"), at("read1")
	closureIdentityRequire(t, !(reader0.Symbol == reader1.Symbol), "same-named read declarations were merged")
	var scopes [2][]closureIdentityStatement
	if blocks {
		branch := entry.Body[0]
		closureIdentityRequire(t, !(branch.Kind != "if" || len(branch.ThenBody) != 1 || len(branch.ElseBody) != 1 || branch.ThenBody[0].Kind != "block" || branch.ElseBody[0].Kind != "block"), "owned if lost separate declaration scopes")
		reference(branch.Condition, seed)
		scopes = [2][]closureIdentityStatement{branch.ThenBody[0].Body, branch.ElseBody[0].Body}
		closureIdentityRequire(t, !(branch.ThenBody[0].Range != anchors["scope0"] || branch.ElseBody[0].Range != anchors["scope1"]), "owned declaration scopes lost original ranges")
	} else {
		conjunction := returned(entry.Body[4])
		closureIdentityRequire(t, !(conjunction.Kind != "shortCircuit" || conjunction.Type != "bool" || conjunction.Operator != "&&"), "owned ordered conjunction missing")
		for index, indirect := range []*closureIdentityExpression{conjunction.Left, conjunction.Right} {
			key := strconv.Itoa(index)
			factory := at("factory" + key)
			checkFunction(factory, "factory"+key, []string{"left", "right"}[index], "function", entry.Symbol, 1, 2)
			signature(factory.ReturnSignature, leaf)
			closureIdentityRequire(t, !(len(factory.Captures) != 0), "factory gained transitive captures")
			declaration(entry.Body[index], factory)
			aliasStatement := entry.Body[index+2]
			closureIdentityRequire(t, !(aliasStatement.Kind != "variable" || aliasStatement.Symbol == nil || aliasStatement.Range != anchors["aliasDeclaration"+key]), "owned alias declaration missing")
			alias := *aliasStatement.Symbol
			checkSymbol(alias, "alias"+key, []string{"leftAlias", "rightAlias"}[index], "local", "function", entry.Symbol, false)
			signature(alias.Signature, leaf)
			direct(aliasStatement.Initializer, factory.Symbol, "function", 1)
			signature(aliasStatement.Initializer.Signature, factorySignature)
			reference(&aliasStatement.Initializer.Arguments[0], seed)
			if indirect == nil || indirect.Kind != "indirectCall" || indirect.Type != "bool" || indirect.Target != nil || indirect.Symbol != "" || len(indirect.Arguments) != 0 {
				t.Fatalf("alias call gained a guessed declaration target: %+v", indirect)
			}
			reference(indirect.Callee, alias)
			signature(indirect.Signature, leaf)
			scopes[index] = factory.Body
		}
	}
	for index, reader := range []closureIdentityFunction{reader0, reader1} {
		key := strconv.Itoa(index)
		parent, captured := entry.Symbol, seed
		if !blocks {
			factory := at("factory" + key)
			parent, captured = factory.Symbol, factory.Parameters[0]
			checkSymbol(captured, "input"+key, "input", "parameter", "bool", parent, true)
		}
		checkFunction(reader, "read"+key, "read", "bool", parent, 0, 1)
		if len(reader.Captures) != 1 || reader.Captures[0].Symbol != captured.ID || reader.Captures[0].Read == nil || !*reader.Captures[0].Read || reader.Captures[0].Write == nil || *reader.Captures[0].Write {
			t.Fatalf("same-named reader lost original capture: %+v", reader)
		}
		value := returned(reader.Body[0])
		if index == 1 {
			closureIdentityRequire(t, !(value.Kind != "unary" || value.Operator != "!" || value.Type != "bool"), "second reader lost original negation")
			value = value.Operand
		}
		reference(value, captured)
		closureIdentityRequire(t, !(len(scopes[index]) != 2), "owned declaration scope lost statement order")
		declaration(scopes[index][0], reader)
		value = returned(scopes[index][1])
		if blocks {
			direct(value, reader.Symbol, "bool", 0)
			signature(value.Signature, leaf)
		} else {
			if value.Kind != "functionReference" || value.Type != "function" || value.Target == nil || *value.Target != reader.Symbol || value.Symbol != "" || value.Callee != nil {
				t.Fatalf("factory returned the wrong same-named reader: %+v", value)
			}
			signature(value.Signature, leaf)
		}
	}
	// These are original Source identities, not runtime environment instances,
	// target sets, escape/lifetime conclusions, GC evidence or stable IDs.
}

func closureIdentityRequire(t *testing.T, valid bool, reason string) {
	t.Helper()
	if !valid {
		t.Fatal(reason)
	}
}
