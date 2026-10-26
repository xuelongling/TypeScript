// SPDX-License-Identifier: Apache-2.0
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	tsjson "github.com/microsoft/typescript-go/internal/json"
	"github.com/microsoft/typescript-go/internal/scanner"
)

// Independently join the frozen source's original AST/Symbols to the public DTO.
// No inferred target set, closure activation or execution result is an oracle.
func commonIndirectOfficial37s(t *testing.T, source string, payload commonProgramPayload) {
	t.Helper()
	program := commonMethodProgram37q(t, source)
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	facts := map[string]arrayGenericFunction{}
	types := map[string]arrayGenericType{}
	for _, fn := range payload.Functions {
		key := fn.Name
		if fn.LexicalParent != nil && *fn.LexicalParent != nil {
			key = "factory." + key
		}
		facts[key] = fn
	}
	for _, typ := range payload.Types {
		if typ.Name != "" {
			types[typ.Name] = typ
		}
	}
	index := commonMethodIndex37q(t, source, file, check, facts, types)
	if len(index.declarations) != 11 || len(index.classes) != 2 || len(facts) != 11 {
		t.Fatal("independent full declaration inventory differs")
	}
	rangeOf := index.rangeOf
	id := func(domain string, value any) string {
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		c := tsjson.Value(b)
		if err := c.Canonicalize(); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(append([]byte(domain+"\x00"), c...))
		return "sha256:" + hex.EncodeToString(sum[:])
	}
	bindingID := func(node *ast.Node) string {
		return id("tsfg.array-facts.binding.v1", map[string]any{"sourcePath": "/src/main.ts", "nameRange": rangeOf(node.Name())})
	}
	for name, node := range index.declarations {
		fn := facts[name]
		if fn.ID != id("tsfg.array-facts.function.v8", map[string]any{"sourcePath": "/src/main.ts", "declarationRange": rangeOf(node)}) || fn.Captures == nil || fn.DeclarationScope == nil {
			t.Fatal("missing or incorrect original function identity/lexical inventory", name)
		}
		if name == "factory.bump" {
			if fn.LexicalParent == nil || *fn.LexicalParent == nil || **fn.LexicalParent != facts["factory"].ID || *fn.DeclarationScope != rangeOf(index.declarations["factory"].Body()) {
				t.Fatal("nested declaration lost original parent/body scope")
			}
		} else if fn.LexicalParent != nil || *fn.DeclarationScope != (span{0, file.GetPositionMap().UTF8ToUTF16(len(source))}) || len(*fn.Captures) != 0 {
			t.Fatal("top-level function acquired lexical owner/captures", name)
		}
		for _, binding := range append(append([]arrayFactsBinding{}, fn.Parameters...), fn.Bindings...) {
			if binding.LexicalOwner == nil || *binding.LexicalOwner == nil || **binding.LexicalOwner != fn.ID {
				t.Fatal("parameter/local lost owning function", name, binding.Name)
			}
		}
	}
	for _, global := range payload.Globals {
		if global.Binding.LexicalOwner != nil {
			t.Fatal("global acquired function owner")
		}
	}
	alias := file.Statements.Nodes[0]
	if alias.Kind != ast.KindTypeAliasDeclaration || alias.Name().Text() != "Unary" || alias.AsTypeAliasDeclaration().Type.Kind != ast.KindFunctionType {
		t.Fatal("independent original function alias differs")
	}
	functionType := id("tsfg.program.function-type.v2", map[string]any{"parameters": []string{"i32"}, "returnType": "i32"})
	if len(payload.TypeAliases) != 1 {
		t.Fatal("complete alias inventory differs")
	}
	a := payload.TypeAliases[0]
	if a.ID != bindingID(alias) || a.Name != "Unary" || a.Type != functionType || a.DeclarationRange != rangeOf(alias) || a.NameRange != rangeOf(alias.Name()) || a.AnnotationRange != rangeOf(alias.AsTypeAliasDeclaration().Type) {
		t.Fatal("original alias identity/type/UTF16 anchors differ")
	}
	var functionRows int
	for _, typ := range payload.Types {
		if typ.Kind == "function" {
			functionRows++
			if typ.ID != functionType || typ.Parameters == nil || !reflect.DeepEqual(*typ.Parameters, []string{"i32"}) || typ.ReturnType != "i32" || typ.Constructor != nil {
				t.Fatal("function type is not the one original structural signature")
			}
		}
	}
	if functionRows != 1 {
		t.Fatal("structural function aliases were duplicated")
	}
	factory := index.declarations["factory"]
	outer := factory.Body().AsBlock().Statements.Nodes[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0]
	outerSymbol := check.GetSymbolAtLocation(outer.Name())
	bump := index.declarations["factory.bump"]
	mutation := bump.Body().AsBlock().Statements.Nodes[0].AsExpressionStatement().Expression.AsBinaryExpression()
	returned := bump.Body().AsBlock().Statements.Nodes[1].AsReturnStatement().Expression
	if outerSymbol == nil || check.GetSymbolAtLocation(mutation.Left) != outerSymbol || check.GetSymbolAtLocation(returned) != outerSymbol || mutation.OperatorToken.Kind != ast.KindPlusEqualsToken {
		t.Fatal("independent capture must resolve both uses to original outer n Symbol")
	}
	if strings.Contains(source, "{ let n: i32 = 99;") {
		innerBlock := factory.Body().AsBlock().Statements.Nodes[1]
		inner := innerBlock.AsBlock().Statements.Nodes[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0]
		innerUse := innerBlock.AsBlock().Statements.Nodes[1].AsExpressionStatement().Expression.AsBinaryExpression().Left
		if check.GetSymbolAtLocation(inner.Name()) == outerSymbol || check.GetSymbolAtLocation(innerUse) != check.GetSymbolAtLocation(inner.Name()) {
			t.Fatal("independent shadow local Symbol was flattened")
		}
		if facts["factory"].Body[1].Kind != "block" || facts["factory"].Body[1].Body == nil || len(*facts["factory"].Body[1].Body) != 2 {
			t.Fatal("ordinary lexical block was discarded")
		}
	}
	expected := []commonProgramCapture{{Binding: bindingID(outer), Read: true, Write: true, Occurrences: []commonProgramCaptureOccurrence{{Range: rangeOf(mutation.Left), Read: true, Write: true}, {Range: rangeOf(returned), Read: true, Write: false}}}}
	if !reflect.DeepEqual(*facts["factory.bump"].Captures, expected) {
		t.Fatal("direct captures do not match original Symbol/read/write/ordered occurrence ranges", *facts["factory.bump"].Captures)
	}
	var declarationFound bool
	for _, statement := range facts["factory"].Body {
		if statement.Kind == "functionDeclaration" {
			declarationFound = true
			if statement.FunctionID != facts["factory.bump"].ID || statement.Range != rangeOf(bump) {
				t.Fatal("nested declaration body/identity was duplicated or lost")
			}
		}
	}
	if !declarationFound {
		t.Fatal("original nested declaration statement missing")
	}
	last := facts["factory"].Body[len(facts["factory"].Body)-1].Value
	if last == nil || last.Kind != "functionReference" || last.FunctionID != facts["factory.bump"].ID || last.Type != functionType || last.Range != rangeOf(factory.Body().AsBlock().Statements.Nodes[len(factory.Body().AsBlock().Statements.Nodes)-1].AsReturnStatement().Expression) {
		t.Fatal("returned named function value lost original reference")
	}
	// Collect all DTO expressions by original range, then compare original AST
	// callees, selected signatures and every argument in source evaluation order.
	wire, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var tree any
	if err := json.Unmarshal(wire, &tree); err != nil {
		t.Fatal(err)
	}
	expressions := map[span]map[string]any{}
	var walk func(any)
	walk = func(value any) {
		switch v := value.(type) {
		case []any:
			for _, child := range v {
				walk(child)
			}
		case map[string]any:
			if _, ok := v["flowType"]; ok {
				if r, ok := v["range"].(map[string]any); ok {
					expressions[span{int(r["start"].(float64)), int(r["end"].(float64))}] = v
				}
			}
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(tree)
	var indirectCount int
	for _, node := range index.callNodes {
		text := source[scanner.SkipTrivia(source, node.Pos()):node.End()]
		call := node.AsCallExpression()
		e := expressions[rangeOf(node)]
		if e == nil {
			t.Fatal("complete original call missing", text)
		}
		arguments, ok := e["arguments"].([]any)
		if !ok || len(arguments) != len(call.Arguments.Nodes) {
			t.Fatal("original arguments were discarded", text)
		}
		for i, arg := range call.Arguments.Nodes {
			got, _ := json.Marshal(arguments[i].(map[string]any)["range"])
			var actual span
			if err := json.Unmarshal(got, &actual); err != nil {
				t.Fatal(err)
			}
			if actual != rangeOf(arg) {
				t.Fatal("argument order/UTF16 changed", text, i)
			}
		}
		if e["kind"] != "indirectCall" {
			continue
		}
		indirectCount++
		if _, exists := e["callee"]; exists {
			t.Fatal("indirect call acquired string callee")
		}
		var callee arrayGenericExpression
		encoded, _ := json.Marshal(e["expression"])
		if err := json.Unmarshal(encoded, &callee); err != nil {
			t.Fatal(err)
		}
		if callee.Kind != "bindingReference" || callee.Type != functionType || callee.Range != rangeOf(call.Expression) {
			t.Fatal("original indirect callee subtree differs", text)
		}
		selected := check.GetResolvedSignature(node)
		if selected == nil || selected.Declaration() != alias.AsTypeAliasDeclaration().Type {
			t.Fatal("independent selected signature must retain original alias FunctionTypeNode", text)
		}
		var signature arrayGenericSignature
		encoded, _ = json.Marshal(e["signature"])
		if err := json.Unmarshal(encoded, &signature); err != nil {
			t.Fatal(err)
		}
		if signature.DeclarationRange != rangeOf(selected.Declaration()) || signature.ID != id("tsfg.array-facts.signature.v8", map[string]any{"sourcePath": "/src/main.ts", "declarationRange": rangeOf(selected.Declaration())}) {
			t.Fatal("indirect selected signature identity changed", text)
		}
	}
	expectedIndirect := 6
	if strings.Contains(source, "return apply(leafRight, x)") {
		expectedIndirect = 4
	}
	if indirectCount != expectedIndirect {
		t.Fatal("complete indirect call inventory differs", indirectCount, expectedIndirect)
	}
	for _, text := range []string{"factory(1)", "factory(10)", "a(2)", "b(3)", "a(4)", "new Box()"} {
		if text == "new Box()" {
			continue
		}
		if index.calls[text] == nil {
			t.Fatal("original separate closure invocation missing", text)
		}
	}
	if facts["ordinary"].Body[1].Kind != "try" || facts["ordinary"].Body[1].CatchClause == nil || facts["arrayOrdinary"].Body[0].Initializer == nil || len(*types["Delta"].Fields) != 1 || len(*types["Box"].Fields) != 1 {
		t.Fatal("unrelated complete EH/Array/value/reference business was removed")
	}
	t.Log("independent original Program/checker: all function/type/alias inventories, UTF16 anchors, lexical ownership and exact outer capture Symbols, complete indirect callee/argument order and selected signatures; no execution credit")
}
