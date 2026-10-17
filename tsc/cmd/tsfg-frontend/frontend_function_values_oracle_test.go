// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

// This oracle checks callable signatures and source-selected call sites in a
// fresh official Program. The older scalar binding helper has no call-signature
// responsibility, and producer-owned IDs or projection helpers are not used here.
func functionValuesOfficialOracle(t *testing.T, r request) map[string]span {
	t.Helper()
	contents := map[string]string{}
	for _, file := range r.Files {
		contents[file.Path] = file.Content
	}
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
			FileNames: append([]string{r.RootFile}, r.Declarations.Paths...),
			CompilerOptions: &core.CompilerOptions{
				NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext,
			},
		}},
		Host:           compiler.NewCompilerHost(r.SourceRoot, vfstest.FromMap(contents, true), r.SourceRoot, nil, nil),
		SingleThreaded: core.TSTrue,
	})
	file := program.GetSourceFile(r.RootFile)
	if file == nil {
		t.Fatal("fresh official program omitted the original source")
	}
	if diagnostics := program.GetSyntacticDiagnostics(t.Context(), nil); len(diagnostics) != 0 {
		t.Fatalf("official syntax diagnostics: %+v", diagnostics)
	}
	if diagnostics := program.GetSemanticDiagnostics(t.Context(), nil); len(diagnostics) != 0 {
		t.Fatalf("official semantic diagnostics: %+v", diagnostics)
	}
	if diagnostics := program.GetProgramDiagnostics(); len(diagnostics) != 0 {
		t.Fatalf("official program diagnostics: %+v", diagnostics)
	}
	if diagnostics := program.GetGlobalDiagnostics(t.Context()); len(diagnostics) != 0 {
		t.Fatalf("official global diagnostics: %+v", diagnostics)
	}
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	if len(file.Statements.Nodes) != 1 || file.Statements.Nodes[0].Kind != ast.KindFunctionDeclaration {
		t.Fatal("official source lost the original entry declaration")
	}
	entryNode := file.Statements.Nodes[0]
	entry := entryNode.AsFunctionDeclaration()
	entryBody := entry.Body.AsBlock().Statements.Nodes
	if len(entry.Parameters.Nodes) != 1 || len(entryBody) != 5 || entryBody[0].Kind != ast.KindFunctionDeclaration || entryBody[1].Kind != ast.KindFunctionDeclaration || entryBody[2].Kind != ast.KindVariableStatement || entryBody[3].Kind != ast.KindVariableStatement || entryBody[4].Kind != ast.KindReturnStatement {
		t.Fatal("official entry differs from the source-selected declaration oracle")
	}
	makeNode, invokeNode := entryBody[0], entryBody[1]
	makeDecl, invokeDecl := makeNode.AsFunctionDeclaration(), invokeNode.AsFunctionDeclaration()
	makeBody, invokeBody := makeDecl.Body.AsBlock().Statements.Nodes, invokeDecl.Body.AsBlock().Statements.Nodes
	if len(makeDecl.Parameters.Nodes) != 1 || len(invokeDecl.Parameters.Nodes) != 1 || len(makeBody) != 2 || makeBody[0].Kind != ast.KindFunctionDeclaration || makeBody[1].Kind != ast.KindReturnStatement || len(invokeBody) != 1 || invokeBody[0].Kind != ast.KindReturnStatement {
		t.Fatal("official factory or callback consumer lost its original body")
	}
	readNode := makeBody[0]
	readDecl := readNode.AsFunctionDeclaration()
	readBody := readDecl.Body.AsBlock().Statements.Nodes
	if len(readDecl.Parameters.Nodes) != 0 || len(readBody) != 1 || readBody[0].Kind != ast.KindReturnStatement {
		t.Fatal("official reader lost its source-selected capture")
	}
	local := func(node *ast.Node) *ast.Node {
		t.Helper()
		declarations := node.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
		if len(declarations) != 1 || declarations[0].Kind != ast.KindVariableDeclaration {
			t.Fatal("official source lost an independent local declaration")
		}
		return declarations[0]
	}
	selected, alias := local(entryBody[2]), local(entryBody[3])
	selectedInit := selected.AsVariableDeclaration().Initializer
	aliasInit := alias.AsVariableDeclaration().Initializer
	entryReturn := entryBody[4].AsReturnStatement().Expression
	invokeReturn := invokeBody[0].AsReturnStatement().Expression
	makeReturn := makeBody[1].AsReturnStatement().Expression
	readReturn := readBody[0].AsReturnStatement().Expression
	if selectedInit.Kind != ast.KindCallExpression || entryReturn.Kind != ast.KindCallExpression || invokeReturn.Kind != ast.KindCallExpression {
		t.Fatal("official source lost its three distinct calls")
	}
	makeCall, invokeCall, fnCall := selectedInit.AsCallExpression(), entryReturn.AsCallExpression(), invokeReturn.AsCallExpression()
	if len(makeCall.Arguments.Nodes) != 1 || len(invokeCall.Arguments.Nodes) != 1 || len(fnCall.Arguments.Nodes) != 0 {
		t.Fatal("official source lost ordered call arguments")
	}
	groups := [][]*ast.Node{
		{entry.Name()},
		{makeDecl.Name(), makeCall.Expression},
		{readDecl.Name(), makeReturn},
		{invokeDecl.Name(), invokeCall.Expression},
		{entry.Parameters.Nodes[0].Name(), makeCall.Arguments.Nodes[0]},
		{makeDecl.Parameters.Nodes[0].Name(), readReturn},
		{invokeDecl.Parameters.Nodes[0].Name(), fnCall.Expression},
		{selected.Name(), aliasInit},
		{alias.Name(), invokeCall.Arguments.Nodes[0]},
	}
	seen := map[*ast.Symbol]bool{}
	for _, sites := range groups {
		binding := check.GetSymbolAtLocation(sites[0])
		if binding == nil || seen[binding] {
			t.Fatal("official checker merged or omitted independently declared bindings")
		}
		seen[binding] = true
		for _, site := range sites[1:] {
			if check.GetSymbolAtLocation(site) != binding {
				t.Fatalf("official source use lost its original binding: %s", sites[0].Text())
			}
		}
	}
	callable := func(typ *checker.Type) *checker.Signature {
		t.Helper()
		signatures := check.GetSignaturesOfType(typ, checker.SignatureKindCall)
		if len(signatures) != 1 || signatures[0].Declaration() == nil || len(signatures[0].TypeParameters()) != 0 || signatures[0].ThisParameter() != nil || signatures[0].HasRestParameter() {
			t.Fatal("official callable oracle requires one non-generic ordinary signature")
		}
		return signatures[0]
	}
	leaf := func(sig *checker.Signature) {
		t.Helper()
		if len(sig.Parameters()) != 0 || sig.MinArgumentCount() != 0 || !check.GetReturnTypeOfSignature(sig).IsBooleanLike() {
			t.Fatal("official reader signature must have no parameters and return bool")
		}
	}
	makeSig := callable(check.GetTypeAtLocation(makeDecl.Name()))
	invokeSig := callable(check.GetTypeAtLocation(invokeDecl.Name()))
	if makeSig.Declaration() != makeNode || invokeSig.Declaration() != invokeNode || len(makeSig.Parameters()) != 1 || len(invokeSig.Parameters()) != 1 || makeSig.MinArgumentCount() != 1 || invokeSig.MinArgumentCount() != 1 || !check.GetTypeOfSymbol(makeSig.Parameters()[0]).IsBooleanLike() || !check.GetReturnTypeOfSignature(invokeSig).IsBooleanLike() {
		t.Fatal("official factory or consumer signature differs from the declared contract")
	}
	leaf(callable(check.GetReturnTypeOfSignature(makeSig)))
	leaf(callable(check.GetTypeOfSymbol(invokeSig.Parameters()[0])))
	leaf(callable(check.GetTypeAtLocation(makeReturn)))
	for _, node := range []*ast.Node{selected.Name(), aliasInit, alias.Name(), invokeCall.Arguments.Nodes[0]} {
		leaf(callable(check.GetTypeAtLocation(node)))
	}
	for _, site := range []struct {
		call *ast.Node
		decl *ast.Node
	}{
		{selectedInit, makeNode}, {entryReturn, invokeNode},
		{invokeReturn, invokeDecl.Parameters.Nodes[0].AsParameterDeclaration().Type},
	} {
		resolved := check.GetResolvedSignature(site.call)
		if resolved == nil || resolved.Declaration() != site.decl {
			t.Fatal("official resolved signature lost its actual call-site declaration")
		}
		if site.call == invokeReturn {
			leaf(resolved)
		}
	}
	// A callback parameter's signature declaration is its type annotation,
	// not the reader's declaration or proof of the callback's runtime target.
	if check.GetResolvedSignature(invokeReturn).Declaration() == readNode {
		t.Fatal("official callback signature was confused with a concrete target")
	}
	nodes := map[string]*ast.Node{
		"entry": entryNode, "make": makeNode, "read": readNode, "invoke": invokeNode,
		"entryDefinition": entry.Name(), "makeDefinition": makeDecl.Name(), "readDefinition": readDecl.Name(), "invokeDefinition": invokeDecl.Name(),
		"seed": entry.Parameters.Nodes[0].Name(), "value": makeDecl.Parameters.Nodes[0].Name(), "fn": invokeDecl.Parameters.Nodes[0].Name(),
		"selected": selected.Name(), "alias": alias.Name(), "selectedStatement": selected, "aliasStatement": alias,
		"entryReturn": entryBody[4], "makeReturn": makeBody[1], "readReturn": readBody[0], "invokeReturn": invokeBody[0],
		"makeCall": selectedInit, "invokeCall": entryReturn, "fnCall": invokeReturn,
		"makeArgument": makeCall.Arguments.Nodes[0], "invokeArgument": invokeCall.Arguments.Nodes[0],
		"aliasInitializer": aliasInit, "returnedFunction": makeReturn, "captureReference": readReturn, "indirectCallee": fnCall.Expression,
	}
	ranges := map[string]span{}
	for label, node := range nodes {
		positions := file.GetPositionMap()
		ranges[label] = span{Start: positions.UTF8ToUTF16(node.Pos()), End: positions.UTF8ToUTF16(node.End())}
	}
	return ranges
}
