// SPDX-License-Identifier: Apache-2.0
package main

import (
	"reflect"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
)

// The independent original Program fixes inventory, Symbols and UTF16 anchors.
// Neither the projected DTO nor the future consumer selects capture expectations.
func readSpanCaptureOfficial26i(t *testing.T, source string, program *compiler.Program, p commonProgramPayload, sample readSpanCaptureCase26i) {
	t.Helper()
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	byID := map[string]arrayGenericFunction{}
	facts := map[string]arrayGenericFunction{}
	types := map[string]arrayGenericType{}
	allTypes := map[string]arrayGenericType{}
	for _, fn := range p.Functions {
		byID[fn.ID] = fn
	}
	for _, fn := range p.Functions {
		name := fn.Name
		if fn.LexicalParent != nil && *fn.LexicalParent != nil {
			parent, ok := byID[**fn.LexicalParent]
			if !ok {
				t.Fatal("missing original lexical parent")
			}
			name = parent.Name + "." + name
		}
		if _, exists := facts[name]; exists {
			t.Fatal("duplicate original callable", name)
		}
		facts[name] = fn
	}
	functionTypes := 0
	for _, typ := range p.Types {
		allTypes[typ.ID] = typ
		if typ.Name != "" {
			types[typ.Name] = typ
		}
		if typ.Kind == "function" {
			functionTypes++
		}
	}
	index := commonMethodIndex37q(t, source, file, check, facts, types)
	if len(index.declarations) != sample.functions || len(index.classes) != 2 || functionTypes != 2 {
		t.Fatal("independent full declaration/type inventory differs")
	}
	rangeOf := index.rangeOf
	for name, node := range index.declarations {
		fn := facts[name]
		if fn.Captures == nil || fn.DeclarationScope == nil {
			t.Fatal("missing exact lexical inventory", name)
		}
		parent := commonLexicalOwnerForOracle26i(node)
		if parent == nil {
			if fn.LexicalParent != nil || *fn.DeclarationScope != (span{0, file.GetPositionMap().UTF8ToUTF16(len(source))}) || len(*fn.Captures) != 0 {
				t.Fatal("top-level lexical facts changed", name)
			}
		} else {
			if fn.LexicalParent == nil || *fn.LexicalParent == nil || **fn.LexicalParent != facts[parent.Name().Text()].ID || *fn.DeclarationScope != rangeOf(parent.Body()) {
				t.Fatal("original nested owner/scope differs", name)
			}
		}
		for _, b := range append(append([]arrayFactsBinding{}, fn.Parameters...), fn.Bindings...) {
			if b.LexicalOwner == nil || *b.LexicalOwner == nil || **b.LexicalOwner != fn.ID {
				t.Fatal("original storage owner differs", name, b.Name)
			}
		}
	}
	ownerName := "captureSpanOrdinary"
	if sample.parameter {
		ownerName = "invokeCapturedSpan"
	}
	ownerNode := index.declarations[ownerName]
	readerNode := index.declarations[ownerName+".readCurrent"]
	var original *ast.Node
	if sample.parameter {
		parameters := ownerNode.AsFunctionDeclaration().Parameters.Nodes
		if sample.parameterIndex < 0 || sample.parameterIndex >= len(parameters) {
			t.Fatal("frozen captured parameter ordinal outside original declaration")
		}
		original = parameters[sample.parameterIndex]
		if original.Name().Text() != "r" {
			t.Fatal("frozen parameter ordinal does not name original r")
		}
	} else {
		for _, statement := range ownerNode.Body().AsBlock().Statements.Nodes {
			if statement.Kind == ast.KindVariableStatement {
				for _, decl := range statement.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
					if decl.Name().Text() == "r" {
						original = decl
					}
				}
			}
		}
	}
	if original == nil {
		t.Fatal("original captured declaration missing")
	}
	symbol := check.GetSymbolAtLocation(original.Name())
	returned := readerNode.Body().AsBlock().Statements.Nodes[0].AsReturnStatement().Expression
	if returned.Kind != ast.KindElementAccessExpression || symbol == nil || check.GetSymbolAtLocation(returned.AsElementAccessExpression().Expression) != symbol {
		t.Fatal("reader must capture the original r Symbol")
	}
	use := returned.AsElementAccessExpression().Expression
	if rangeOf(use) != (span{sample.occurrence, sample.occurrence + 1}) {
		t.Fatal("frozen original UTF16 capture anchor differs", rangeOf(use))
	}
	owner := facts[ownerName]
	if sample.parameter {
		parameters := ownerNode.AsFunctionDeclaration().Parameters.Nodes
		if len(owner.Parameters) != len(parameters) {
			t.Fatal("original parameter directory is incomplete")
		}
		for i, node := range parameters {
			binding := owner.Parameters[i]
			if binding.Name != node.Name().Text() || binding.DeclarationRange != rangeOf(node) || binding.NameRange != rangeOf(node.Name()) {
				t.Fatal("original parameter order or UTF16 identity changed", i)
			}
		}
	}
	reader := facts[ownerName+".readCurrent"]
	var captured arrayFactsBinding
	for _, b := range append(append([]arrayFactsBinding{}, owner.Parameters...), owner.Bindings...) {
		if b.Name == "r" {
			captured = b
		}
	}
	expectedKind := "local"
	if sample.parameter {
		expectedKind = "parameter"
	}
	if sample.parameter && owner.Parameters[sample.parameterIndex].ID != captured.ID {
		t.Fatal("capture did not retain its original parameter ordinal")
	}
	typ := allTypes[captured.DeclaredType]
	if captured.ID == "" || captured.Kind != expectedKind || captured.DeclarationRange != rangeOf(original) || captured.NameRange != rangeOf(original.Name()) || typ.Kind != "readSpan" || typ.ElementType != "i32" {
		t.Fatal("captured original binding/type identity differs")
	}
	expected := []commonProgramCapture{{Binding: captured.ID, Read: true, Write: false, Occurrences: []commonProgramCaptureOccurrence{{Range: rangeOf(use), Read: true, Write: false}}}}
	if !reflect.DeepEqual(*reader.Captures, expected) {
		t.Fatal("exact original capture Symbol/access/occurrence differs", *reader.Captures)
	}
	value := reader.Body[0].Value
	if len(reader.Body) != 1 || value == nil || value.Kind != "checkedElementAccess" || value.Range != rangeOf(returned) || value.Receiver == nil || value.Receiver.Kind != "bindingReference" || value.Receiver.Binding != captured.ID || value.Receiver.Range != rangeOf(use) {
		t.Fatal("original nested Span read tree missing")
	}
	found := false
	for _, statement := range owner.Body {
		if statement.Kind == "functionDeclaration" {
			if statement.FunctionID != reader.ID || statement.Range != rangeOf(readerNode) {
				t.Fatal("nested declaration changed")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("nested declaration was cropped")
	}
	// Keep the unrelated complete ordinary EH, Array/View, scalar closure and both
	// nominal classes. Source emits no selected-call liveness or borrow verdict.
	if facts["ordinary"].Body[1].Kind != "try" || facts["ordinary"].Body[1].CatchClause == nil || facts["arrayOrdinary"].Body[0].Initializer == nil || len(*facts["factory.bump"].Captures) != 1 || !(*facts["factory.bump"].Captures)[0].Write || len(*types["Delta"].Fields) != 1 || len(*types["Box"].Fields) != 1 {
		t.Fatal("unrelated complete business was cropped")
	}
	for _, global := range p.Globals {
		if global.Binding.LexicalOwner != nil {
			t.Fatal("global acquired function owner")
		}
	}
	t.Log("independent original AST/checker: complete callable/class inventory, original lexical scopes, r Symbol/type/UTF16 and exact read-only capture verified; no lifetime or execution authority")
}

func commonLexicalOwnerForOracle26i(node *ast.Node) *ast.Node {
	for p := node.Parent; p != nil; p = p.Parent {
		if p.Kind == ast.KindFunctionDeclaration {
			return p
		}
	}
	return nil
}
