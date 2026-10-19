// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	stdjson "encoding/json"
	"fmt"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/scanner"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

// Retained original default-wire input from the qualified causal capability RED.
// Active FIRST below explicitly migrates the wire/DTS while preserving all three
// original TypeScript sources and comparing the independently frozen whole packet.
const moduleCallableFirstInput23d = `{"compilationClosure":{"roots":[{"kind":"application","sourcePath":"/src/app.ts","name":"entry"}],"requiredRoots":[]},"sourceRoot":"/src","rootFile":"/src/app.ts","files":[{"path":"/src/app.ts","content":"import { chosen } from './barrel';\nexport function identity(fn: () => bool): () => bool { return fn; }\nexport function invoke(fn: () => bool): bool { return fn(); }\nexport function entry(): bool { const selected = identity(chosen); return invoke(selected); }\n"},{"path":"/src/barrel.ts","content":"export { read as chosen } from './state';\n"},{"path":"/src/state.ts","content":"export let shared: bool = false;\nexport function read(): bool { return shared; }\n"},{"path":"/src/minimal-scalars-v2.d.ts","content":"interface Object {}\ninterface Function {}\ninterface CallableFunction {}\ninterface NewableFunction {}\ninterface IArguments {}\ninterface String {}\ninterface Number {}\ninterface Boolean {}\ninterface RegExp {}\ninterface Array<T> {}\ntype bool = boolean;\ndeclare const __tsfg_i32: unique symbol;\ntype i32 = number & { readonly [__tsfg_i32]: true };\ntype f64 = number;\n"}],"declarations":{"version":"minimal-scalars-v2","paths":["/src/minimal-scalars-v2.d.ts"]},"config":{"strict":true,"noEmit":true,"allowJs":false}}
`

func TestModuleCallableFactsFirstPublicPackage(t *testing.T) {
	_, input, expected := mcPublicFixture23d(t, "first-rr")
	before := bytes.Clone(input)
	var envelope struct{ Input request }
	if err := stdjson.Unmarshal(input, &envelope); err != nil {
		t.Fatal(err)
	}
	var legacy request
	if err := stdjson.Unmarshal([]byte(moduleCallableFirstInput23d), &legacy); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/src/app.ts", "/src/barrel.ts", "/src/state.ts"} {
		oldText, newText := "", ""
		for _, file := range legacy.Files {
			if file.Path == path {
				oldText = file.Content
			}
		}
		for _, file := range envelope.Input.Files {
			if file.Path == path {
				newText = file.Content
			}
		}
		if oldText == "" || oldText != newText {
			t.Fatalf("original causal FIRST TypeScript bytes changed at %s", path)
		}
	}
	t.Logf("TSFG_MODULE_CALLABLE_23D_FIRST_MIGRATION sameOriginalTS=3 oldDefaultWireRetained=true declarations=minimal-integers-v3 candidateFlag=--module-callable-facts-io-v1 bytes=%d sha256:%x", len(input), sha256.Sum256(input))
	moduleCallableFirstOfficial23d(t, envelope.Input)
	t.Log("TSFG_MODULE_CALLABLE_23D_FIRST_OFFICIAL_PASS diagnostics=0 modules=3 functions=4 originalAlias=true exactCallable=true")
	var output bytes.Buffer
	if err := runFrontendCommand(t.Context(), []string{"--module-callable-facts-io-v1"}, bytes.NewReader(input), &output); err != nil {
		t.Fatalf("ordinary original TypeScript must succeed after explicit candidate-wire migration: %v", err)
	}
	if !bytes.Equal(input, before) {
		t.Fatal("public command mutated exact frozen input")
	}
	t.Logf("TSFG_MODULE_CALLABLE_23D_FIRST_ACTUAL_BASE64=%s", base64.StdEncoding.EncodeToString(output.Bytes()))
	if !bytes.Equal(output.Bytes(), expected) {
		t.Fatalf("complete independently frozen FIRST package differs: got=%s want=%s", mcHash23d(output.Bytes()), mcHash23d(expected))
	}
}

// Constructs a fresh official Program and selects the original source nodes.
// No production projection, producer IDs, or module alias helpers are used.
func moduleCallableFirstOfficial23d(t *testing.T, r request) {
	t.Helper()
	contents := map[string]string{}
	fileNames := []string{}
	for _, file := range r.Files {
		contents[file.Path] = file.Content
		fileNames = append(fileNames, file.Path)
	}
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
			FileNames: fileNames,
			CompilerOptions: &core.CompilerOptions{
				NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext,
			},
		}},
		Host:           compiler.NewCompilerHost(r.SourceRoot, vfstest.FromMap(contents, true), r.SourceRoot, nil, nil),
		SingleThreaded: core.TSTrue,
	})
	if diagnostics := program.GetSyntacticDiagnostics(t.Context(), nil); len(diagnostics) != 0 {
		t.Fatalf("fresh official syntax diagnostics: %+v", diagnostics)
	}
	if diagnostics := program.GetSemanticDiagnostics(t.Context(), nil); len(diagnostics) != 0 {
		t.Fatalf("fresh official semantic diagnostics: %+v", diagnostics)
	}
	if diagnostics := program.GetProgramDiagnostics(); len(diagnostics) != 0 {
		t.Fatalf("fresh official program diagnostics: %+v", diagnostics)
	}
	if diagnostics := program.GetGlobalDiagnostics(t.Context()); len(diagnostics) != 0 {
		t.Fatalf("fresh official global diagnostics: %+v", diagnostics)
	}
	app := program.GetSourceFile("/src/app.ts")
	barrel := program.GetSourceFile("/src/barrel.ts")
	state := program.GetSourceFile("/src/state.ts")
	if app == nil || barrel == nil || state == nil || len(app.Statements.Nodes) != 4 || len(barrel.Statements.Nodes) != 1 || len(state.Statements.Nodes) != 2 {
		t.Fatal("fresh official Program lost the complete three original modules")
	}
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), app)
	defer release()
	if app.Statements.Nodes[0].Kind != ast.KindImportDeclaration || barrel.Statements.Nodes[0].Kind != ast.KindExportDeclaration || state.Statements.Nodes[0].Kind != ast.KindVariableStatement {
		t.Fatal("original import, reexport, or global declaration changed")
	}
	identityNode, invokeNode, entryNode, readNode := app.Statements.Nodes[1], app.Statements.Nodes[2], app.Statements.Nodes[3], state.Statements.Nodes[1]
	for _, node := range []*ast.Node{identityNode, invokeNode, entryNode, readNode} {
		if node.Kind != ast.KindFunctionDeclaration || node.Name() == nil || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			t.Fatal("original named top-level function declaration changed")
		}
	}
	identity, invoke, entry, read := identityNode.AsFunctionDeclaration(), invokeNode.AsFunctionDeclaration(), entryNode.AsFunctionDeclaration(), readNode.AsFunctionDeclaration()
	if identity.Name().Text() != "identity" || invoke.Name().Text() != "invoke" || entry.Name().Text() != "entry" || read.Name().Text() != "read" || len(identity.Parameters.Nodes) != 1 || len(invoke.Parameters.Nodes) != 1 || len(entry.Parameters.Nodes) != 0 || len(read.Parameters.Nodes) != 0 {
		t.Fatal("original declaration names or parameter counts changed")
	}
	identityBody, invokeBody := identity.Body.AsBlock().Statements.Nodes, invoke.Body.AsBlock().Statements.Nodes
	entryBody, readBody := entry.Body.AsBlock().Statements.Nodes, read.Body.AsBlock().Statements.Nodes
	if len(identityBody) != 1 || len(invokeBody) != 1 || len(entryBody) != 2 || len(readBody) != 1 || identityBody[0].Kind != ast.KindReturnStatement || invokeBody[0].Kind != ast.KindReturnStatement || entryBody[0].Kind != ast.KindVariableStatement || entryBody[1].Kind != ast.KindReturnStatement || readBody[0].Kind != ast.KindReturnStatement {
		t.Fatal("original complete statement structure changed")
	}
	locals := entryBody[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
	globals := state.Statements.Nodes[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
	if len(locals) != 1 || len(globals) != 1 || locals[0].Name().Text() != "selected" || globals[0].Name().Text() != "shared" {
		t.Fatal("original local or scalar global declaration changed")
	}
	selected, shared := locals[0].AsVariableDeclaration(), globals[0].AsVariableDeclaration()
	identityReturn := identityBody[0].AsReturnStatement().Expression
	invokeReturn := invokeBody[0].AsReturnStatement().Expression
	entryReturn := entryBody[1].AsReturnStatement().Expression
	readReturn := readBody[0].AsReturnStatement().Expression
	if selected.Initializer == nil || selected.Initializer.Kind != ast.KindCallExpression || invokeReturn.Kind != ast.KindCallExpression || entryReturn.Kind != ast.KindCallExpression {
		t.Fatal("original ordered direct and indirect calls changed")
	}
	identityCall, invokeCall, callbackCall := selected.Initializer.AsCallExpression(), entryReturn.AsCallExpression(), invokeReturn.AsCallExpression()
	if len(identityCall.Arguments.Nodes) != 1 || len(invokeCall.Arguments.Nodes) != 1 || len(callbackCall.Arguments.Nodes) != 0 {
		t.Fatal("original complete call arguments changed")
	}
	groups := [][]*ast.Node{
		{identity.Name(), identityCall.Expression},
		{invoke.Name(), invokeCall.Expression},
		{identity.Parameters.Nodes[0].Name(), identityReturn},
		{invoke.Parameters.Nodes[0].Name(), callbackCall.Expression},
		{selected.Name(), invokeCall.Arguments.Nodes[0]},
		{shared.Name(), readReturn},
	}
	seen := map[*ast.Symbol]bool{}
	for _, sites := range groups {
		symbol := check.GetExportSymbolOfSymbol(check.GetSymbolAtLocation(sites[0]))
		if symbol == nil || seen[symbol] {
			t.Fatal("official checker merged independent original declarations")
		}
		seen[symbol] = true
		for _, site := range sites[1:] {
			if check.GetExportSymbolOfSymbol(check.GetSymbolAtLocation(site)) != symbol {
				t.Fatal("official use lost its original declaration")
			}
		}
	}
	aliasTarget := func(symbol *ast.Symbol) *ast.Symbol {
		visited := map[*ast.Symbol]bool{}
		for symbol != nil && symbol.Flags&ast.SymbolFlagsAlias != 0 {
			if visited[symbol] {
				t.Fatal("official alias graph unexpectedly cycles")
			}
			visited[symbol] = true
			symbol = check.GetImmediateAliasedSymbol(symbol)
		}
		return symbol
	}
	readSymbol := aliasTarget(check.GetSymbolAtLocation(read.Name()))
	argumentSymbol := aliasTarget(check.GetSymbolAtLocation(identityCall.Arguments.Nodes[0]))
	if readSymbol == nil || argumentSymbol == nil || check.GetExportSymbolOfSymbol(readSymbol) != check.GetExportSymbolOfSymbol(argumentSymbol) {
		t.Fatal("imported function value lost the original state.read declaration")
	}
	containsReadDeclaration := false
	for _, declaration := range readSymbol.Declarations {
		containsReadDeclaration = containsReadDeclaration || declaration == readNode
	}
	if !containsReadDeclaration {
		t.Fatal("official original function symbol lacks its actual declaration")
	}
	exports := check.GetExportsOfModule(barrel.AsNode().Symbol())
	if len(exports) != 1 || exports[0].Name != "chosen" || check.GetExportSymbolOfSymbol(aliasTarget(exports[0])) != check.GetExportSymbolOfSymbol(readSymbol) {
		t.Fatal("official barrel reexport is not the original state.read function")
	}
	callable := func(typ *checker.Type) *checker.Signature {
		signatures := check.GetSignaturesOfType(typ, checker.SignatureKindCall)
		if len(signatures) != 1 || signatures[0].Declaration() == nil || len(signatures[0].TypeParameters()) != 0 || signatures[0].ThisParameter() != nil || signatures[0].HasRestParameter() {
			t.Fatal("official callable must have exactly one ordinary non-generic signature")
		}
		return signatures[0]
	}
	leaf := func(signature *checker.Signature) {
		if len(signature.Parameters()) != 0 || signature.MinArgumentCount() != 0 || !check.GetReturnTypeOfSignature(signature).IsBooleanLike() {
			t.Fatal("official callback must have exact () => bool signature")
		}
	}
	identitySignature := callable(check.GetTypeAtLocation(identity.Name()))
	invokeSignature := callable(check.GetTypeAtLocation(invoke.Name()))
	if identitySignature.Declaration() != identityNode || invokeSignature.Declaration() != invokeNode || len(identitySignature.Parameters()) != 1 || len(invokeSignature.Parameters()) != 1 || identitySignature.MinArgumentCount() != 1 || invokeSignature.MinArgumentCount() != 1 || !check.GetReturnTypeOfSignature(invokeSignature).IsBooleanLike() {
		t.Fatal("official identity/invoke signatures differ from the original source")
	}
	leaf(callable(check.GetTypeOfSymbol(identitySignature.Parameters()[0])))
	leaf(callable(check.GetReturnTypeOfSignature(identitySignature)))
	leaf(callable(check.GetTypeOfSymbol(invokeSignature.Parameters()[0])))
	for _, node := range []*ast.Node{read.Name(), entry.Name(), identityReturn, selected.Name(), identityCall.Arguments.Nodes[0], invokeCall.Arguments.Nodes[0], callbackCall.Expression} {
		leaf(callable(check.GetTypeAtLocation(node)))
	}
	for _, site := range []struct{ call, declaration *ast.Node }{
		{selected.Initializer, identityNode}, {entryReturn, invokeNode},
		{invokeReturn, invoke.Parameters.Nodes[0].AsParameterDeclaration().Type},
	} {
		resolved := check.GetResolvedSignature(site.call)
		if resolved == nil || resolved.Declaration() != site.declaration {
			t.Fatal("official resolved call signature lost its actual declaration")
		}
	}
	if check.GetResolvedSignature(invokeReturn).Declaration() == readNode {
		t.Fatal("callback annotation was mistaken for a concrete runtime target")
	}
	if shared.Initializer == nil || shared.Initializer.Kind != ast.KindFalseKeyword || !check.GetTypeAtLocation(readReturn).IsBooleanLike() {
		t.Fatal("official scalar global initializer or read type changed")
	}
	for _, site := range []struct {
		file     *ast.SourceFile
		node     *ast.Node
		spelling string
	}{
		{app, identity.Name(), "identity"}, {app, invoke.Name(), "invoke"}, {app, entry.Name(), "entry"},
		{app, identityCall.Arguments.Nodes[0], "chosen"}, {app, callbackCall.Expression, "fn"},
		{app, invokeCall.Arguments.Nodes[0], "selected"}, {state, read.Name(), "read"}, {state, readReturn, "shared"},
	} {
		start := scanner.SkipTrivia(site.file.Text(), site.node.Pos())
		if site.file.Text()[start:site.node.End()] != site.spelling {
			t.Fatal("original source-selected token anchor differs")
		}
		positions := site.file.GetPositionMap()
		t.Logf("TSFG_MODULE_CALLABLE_23D_FIRST_ANCHOR %s %s %d %d", site.file.FileName(), fmt.Sprint(site.spelling), positions.UTF8ToUTF16(start), positions.UTF8ToUTF16(site.node.End()))
	}
}
