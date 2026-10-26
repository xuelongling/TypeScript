// SPDX-License-Identifier: Apache-2.0
package main

import (
	"reflect"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
)

// The original independent Program fixes lexical declarations and occurrences.
// Source binding writes and Array content writes are deliberately distinct.
func arrayCaptureOfficial26k(t *testing.T, source string, program *compiler.Program, p commonProgramPayload, sample arrayCaptureCase26k) {
	t.Helper()
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	facts, allTypes, index := commonCaptureInventory26(t, source, file, check, p, sample.functions)
	rangeOf := index.rangeOf
	ownerNode := index.declarations["makeOwnedReader"]
	owner := facts["makeOwnedReader"]
	readerNode := index.declarations[sample.reader]
	reader := facts[sample.reader]
	declarations := map[string]*ast.Node{}
	if sample.parameter {
		for i, node := range ownerNode.AsFunctionDeclaration().Parameters.Nodes {
			declarations[node.Name().Text()] = node
			if owner.Parameters[i].DeclarationRange != rangeOf(node) || owner.Parameters[i].NameRange != rangeOf(node.Name()) {
				t.Fatal("original parameter order/UTF16 differs", i)
			}
		}
		if len(owner.Parameters) != 2 || owner.Parameters[1].Name != "a" {
			t.Fatal("captured Array parameter must retain original index1")
		}
	} else {
		for _, statement := range ownerNode.Body().AsBlock().Statements.Nodes {
			if statement.Kind == ast.KindVariableStatement {
				for _, decl := range statement.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
					declarations[decl.Name().Text()] = decl
				}
			}
		}
	}
	bindings := map[string]arrayFactsBinding{}
	for _, b := range append(append([]arrayFactsBinding{}, owner.Parameters...), owner.Bindings...) {
		bindings[b.Name] = b
	}
	expected := []commonProgramCapture{}
	for _, use := range sample.uses {
		original := declarations[use.name]
		binding := bindings[use.name]
		if original == nil || binding.ID == "" || binding.DeclarationRange != rangeOf(original) || binding.NameRange != rangeOf(original.Name()) || binding.LexicalOwner == nil || *binding.LexicalOwner == nil || **binding.LexicalOwner != owner.ID {
			t.Fatal("captured binding lost original declaration/owner", use.name)
		}
		expectedKind := "local"
		if sample.parameter {
			expectedKind = "parameter"
		}
		if binding.Kind != expectedKind {
			t.Fatal("original local/parameter role changed")
		}
		typ := allTypes[binding.DeclaredType]
		if use.name == "a" {
			element := allTypes[typ.ElementType]
			wantElement := "scalar"
			if sample.name == "function-element-parameter" {
				wantElement = "function"
			}
			if typ.Kind != "array" || element.Kind != wantElement || binding.Writable != sample.writable {
				t.Fatal("original Array<T>/binding writability changed")
			}
		} else if typ.Kind != "readSpan" || typ.ElementType != "i32" {
			t.Fatal("original ReadSpan type changed")
		}
		symbol := check.GetSymbolAtLocation(original.Name())
		var found *ast.Node
		var walk func(*ast.Node)
		walk = func(node *ast.Node) {
			if node.Kind == ast.KindIdentifier && rangeOf(node) == (span{use.start, use.start + 1}) {
				found = node
			}
			node.ForEachChild(func(child *ast.Node) bool { walk(child); return false })
		}
		walk(readerNode.Body())
		if found == nil || symbol == nil || check.GetSymbolAtLocation(found) != symbol {
			t.Fatal("frozen direct occurrence is not the original captured Symbol", use.name)
		}
		expected = append(expected, commonProgramCapture{Binding: binding.ID, Read: true, Write: false, Occurrences: []commonProgramCaptureOccurrence{{Range: rangeOf(found), Read: true, Write: false}}})
	}
	if !reflect.DeepEqual(*reader.Captures, expected) {
		t.Fatal("original direct capture order/read/write/occurrences differ", *reader.Captures)
	}
	if sample.name == "const-owner-content-write" {
		statement := reader.Body[0]
		if statement.Value == nil || statement.Value.Kind != "elementAssignment" || statement.Value.Target == nil || statement.Value.Target.Receiver == nil || statement.Value.Target.Receiver.Binding != bindings["a"].ID || bindings["a"].Writable || (*reader.Captures)[0].Write {
			t.Fatal("Array content write was confused with const binding reassignment")
		}
	}
	if sample.name == "owner-replaced" {
		changed := false
		for _, statement := range owner.Body {
			if statement.Value != nil && statement.Value.Kind == "bindingAssignment" && statement.Value.Target.Binding == bindings["a"].ID {
				changed = true
			}
		}
		if !changed {
			t.Fatal("original owner-cell replacement was cropped")
		}
	}
	if sample.name == "function-element-parameter" {
		value := reader.Body[0].Value
		if value == nil || value.Kind != "indirectCall" || value.Expression == nil || value.Expression.Kind != "checkedElementAccess" || value.Expression.Receiver.Binding != bindings["a"].ID || value.Arguments == nil || len(*value.Arguments) != 1 {
			t.Fatal("original checked function-element indirect call was cropped")
		}
	}
	if sample.name == "parent-owned-environment" && len(*facts["makeOwnedReader.middle"].Captures) != 0 {
		t.Fatal("indirect descendant use was fabricated as direct parent capture")
	}
	t.Log("independent original AST/checker: complete inventory, lexical parent/owner, original Array parameter ordinal, exact Symbol/UTF16 direct captures and content-vs-binding writes verified; no retention or finite-target permission")
}
