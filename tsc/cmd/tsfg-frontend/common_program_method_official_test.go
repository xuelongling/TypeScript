// SPDX-License-Identifier: Apache-2.0
package main

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/scanner"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

// A separate original Program/checker lease supplies the oracle. No fact DTO,
// M12 projection or eventual Compiler dispatch table selects these expectations.
func commonMethodOfficial37q(t *testing.T, source string, facts map[string]arrayGenericFunction, types map[string]arrayGenericType) {
	t.Helper()
	program := commonMethodProgram37q(t, source)
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	index := commonMethodIndex37q(t, source, file, check, facts, types)
	classes, declarations, calls, rangeOf := index.classes, index.declarations, index.calls, index.rangeOf
	if len(classes) != 7 || len(declarations) != 17 {
		t.Fatal("independent complete declaration inventory changed")
	}
	for _, edge := range [][2]string{{"Middle", "Base"}, {"Derived", "Middle"}, {"Inherited", "Middle"}} {
		bases := check.GetBaseTypes(check.GetDeclaredTypeOfSymbol(check.GetSymbolAtLocation(classes[edge[0]].Name())))
		base := check.GetDeclaredTypeOfSymbol(check.GetSymbolAtLocation(classes[edge[1]].Name()))
		if len(bases) != 1 || bases[0] != base {
			t.Fatal("independent original base edge differs", edge)
		}
	}
	if declarations["Derived.m"].ModifierFlags()&ast.ModifierFlagsOverride == 0 || declarations["Middle.m"].ModifierFlags()&ast.ModifierFlagsOverride == 0 {
		t.Fatal("both original override tokens must remain")
	}
	peer := check.GetDeclaredTypeOfSymbol(check.GetSymbolAtLocation(classes["Peer"].Name()))
	base := check.GetDeclaredTypeOfSymbol(check.GetSymbolAtLocation(classes["Base"].Name()))
	if !check.IsTypeAssignableTo(peer, base) || check.GetPropertyOfType(peer, "m") != check.GetSymbolAtLocation(declarations["Peer.m"].Name()) || check.GetPropertyOfType(base, "m") != check.GetSymbolAtLocation(declarations["Base.m"].Name()) {
		t.Fatal("independent Peer-to-Base structural relation or original method Symbol differs")
	}
	for _, text := range []string{"b.m(a)", "makeReceiver().m(makeArgument())"} {
		node := calls[text]
		if node == nil || check.GetResolvedSignature(node) == nil || check.GetResolvedSignature(node).Declaration() != declarations["Base.m"] || check.GetSymbolAtLocation(node.AsCallExpression().Expression.Name()) != check.GetSymbolAtLocation(declarations["Base.m"].Name()) {
			t.Fatal("independent original static Base.m selected signature differs", text)
		}
	}
	call := calls["b.m(a)"].AsCallExpression()
	parameters := declarations["callBase"].AsFunctionDeclaration().Parameters.Nodes
	if check.GetSymbolAtLocation(call.Expression.AsPropertyAccessExpression().Expression) != check.GetSymbolAtLocation(parameters[0].Name()) || check.GetSymbolAtLocation(call.Arguments.Nodes[0]) != check.GetSymbolAtLocation(parameters[1].Name()) {
		t.Fatal("independent original receiver and argument Symbols differ")
	}
	ordered := calls["makeReceiver().m(makeArgument())"].AsCallExpression()
	receiver, argument := ordered.Expression.AsPropertyAccessExpression().Expression, ordered.Arguments.Nodes[0]
	if receiver.Kind != ast.KindCallExpression || argument.Kind != ast.KindCallExpression || check.GetResolvedSignature(receiver).Declaration() != declarations["makeReceiver"] || check.GetResolvedSignature(argument).Declaration() != declarations["makeArgument"] || receiver.End() >= argument.Pos() {
		t.Fatal("independent original receiver-before-argument call tree differs")
	}
	if facts["ordered"].Body[0].Value.Range != rangeOf(calls["makeReceiver().m(makeArgument())"]) || facts["callBase"].Body[0].Value.Range != rangeOf(calls["b.m(a)"]) {
		t.Fatal("packet method calls do not preserve independent original UTF16 anchors")
	}
	t.Log("independent original Program: 4x0 diagnostics; 17 callable Symbols/ranges, 7 classes, override/base edges, M12 assignability and static receiver/argument order verified; no runtime assertion")
}

type commonMethodOracleIndex37q struct {
	classes, declarations, calls map[string]*ast.Node
	rangeOf                      func(*ast.Node) span
}

// Both real Programs share the read-only AST/range join, while callers retain
// their own checker lease and distinct inheritance/signature/order assertions.
func commonMethodIndex37q(t *testing.T, source string, file *ast.SourceFile, check *checker.Checker, facts map[string]arrayGenericFunction, types map[string]arrayGenericType) commonMethodOracleIndex37q {
	t.Helper()
	index := commonMethodOracleIndex37q{classes: map[string]*ast.Node{}, declarations: map[string]*ast.Node{}, calls: map[string]*ast.Node{}}
	for _, node := range file.Statements.Nodes {
		if node.Kind == ast.KindFunctionDeclaration {
			index.declarations[node.Name().Text()] = node
		}
		if node.Kind == ast.KindClassDeclaration {
			name := node.Name().Text()
			index.classes[name] = node
			for _, member := range node.Members() {
				if member.Kind == ast.KindMethodDeclaration {
					index.declarations[name+"."+member.Name().Text()] = member
				}
			}
		}
	}
	var walk func(*ast.Node)
	walk = func(node *ast.Node) {
		if node.Kind == ast.KindCallExpression {
			index.calls[source[scanner.SkipTrivia(source, node.Pos()):node.End()]] = node
		}
		node.ForEachChild(func(child *ast.Node) bool { walk(child); return false })
	}
	walk(file.AsNode())
	position := file.GetPositionMap()
	index.rangeOf = func(node *ast.Node) span {
		return span{Start: position.UTF8ToUTF16(scanner.SkipTrivia(source, node.Pos())), End: position.UTF8ToUTF16(node.End())}
	}
	for name, node := range index.declarations {
		fact, ok := facts[name]
		if !ok || fact.DeclarationRange != index.rangeOf(node) || fact.NameRange != index.rangeOf(node.Name()) || check.GetSymbolAtLocation(node.Name()) == nil {
			t.Fatal("independent original callable Symbol or UTF16 declaration/name range differs", name)
		}
	}
	for name, node := range index.classes {
		fact, ok := types[name]
		if !ok || fact.DeclarationRange == nil || fact.NameRange == nil || *fact.DeclarationRange != index.rangeOf(node) || *fact.NameRange != index.rangeOf(node.Name()) {
			t.Fatal("independent original class declaration/name range differs", name)
		}
	}
	return index
}

func commonMethodProgram37q(t *testing.T, source string) *compiler.Program {
	t.Helper()
	files := map[string]string{"/src/main.ts": source, "/src/minimal-arrays-v8.d.ts": string(arrayGenericDeclarations)}
	program := compiler.NewProgram(compiler.ProgramOptions{Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{FileNames: []string{"/src/main.ts", "/src/minimal-arrays-v8.d.ts"}, CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext}}}, Host: compiler.NewCompilerHost("/src", vfstest.FromMap(files, true), "/src", nil, nil), SingleThreaded: core.TSTrue})
	for _, diagnostics := range [][]*ast.Diagnostic{program.GetSyntacticDiagnostics(t.Context(), nil), program.GetSemanticDiagnostics(t.Context(), nil), program.GetProgramDiagnostics(), program.GetGlobalDiagnostics(t.Context())} {
		if len(diagnostics) != 0 {
			t.Fatalf("independent official method diagnostics: %+v", diagnostics)
		}
	}
	return program
}
