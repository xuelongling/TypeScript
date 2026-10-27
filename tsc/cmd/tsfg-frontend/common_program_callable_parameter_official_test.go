// SPDX-License-Identifier: Apache-2.0
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"reflect"
	"testing"
)

// This check runs before projection, keeping original incoming argument identity
// separate from the later DTO and every consumer code/environment proof.
func callableParameterOriginal19n(t *testing.T, program *compiler.Program, sample callableParameterCase19n) {
	t.Helper()
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	functions := map[string]*ast.Node{}
	var walk func(*ast.Node, string)
	walk = func(n *ast.Node, parent string) {
		if n.Kind == ast.KindFunctionDeclaration {
			if parent != "" {
				parent += "."
			}
			parent += n.Name().Text()
			functions[parent] = n
		}
		n.ForEachChild(func(c *ast.Node) bool { walk(c, parent); return false })
	}
	walk(file.AsNode(), "")
	owner, child := functions["relayReader"], functions[sample.reader]
	if owner == nil || child == nil || len(functions) != sample.functions {
		t.Fatal("complete relay declaration directory differs")
	}
	params := owner.AsFunctionDeclaration().Parameters.Nodes
	if len(params) != 1 || params[0].Kind != ast.KindParameter || params[0].Parent != owner || params[0].Name().Text() != "reader" {
		t.Fatal("original incoming callable parameter declaration differs")
	}
	param := params[0]
	decl := param.AsParameterDeclaration()
	symbol := check.GetSymbolAtLocation(param.Name())
	if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != param || decl.Initializer != nil || decl.Type == nil || decl.Type.Kind != ast.KindFunctionType {
		t.Fatal("unique direct callable incoming parameter identity lost")
	}
	sigs := check.GetSignaturesOfType(check.GetTypeAtLocation(param.Name()), checker.SignatureKindCall)
	if len(sigs) != 1 || len(sigs[0].Parameters()) != 0 || sigs[0].Declaration() != decl.Type {
		t.Fatal("original incoming callable signature differs")
	}
	statements := child.Body().AsBlock().Statements.Nodes
	returned := statements[len(statements)-1].AsReturnStatement().Expression
	if returned.Kind != ast.KindCallExpression {
		t.Fatal("original callback invocation absent")
	}
	call := returned.AsCallExpression()
	selected := check.GetResolvedSignature(returned)
	if call.Expression.Kind != ast.KindIdentifier || len(call.Arguments.Nodes) != 0 || selected == nil || selected.Declaration() == nil || len(selected.Parameters()) != 0 || check.GetReturnTypeOfSignature(selected) != check.GetTypeAtLocation(returned) {
		t.Fatal("original callback selected signature differs")
	}
	if sample.shadow == (check.GetSymbolAtLocation(call.Expression) == symbol) {
		t.Fatal("same-name local/ancestor callable Symbol relation changed")
	}
	if sample.mutation {
		assignment := statements[0].AsExpressionStatement().Expression.AsBinaryExpression()
		if assignment.OperatorToken.Kind != ast.KindEqualsToken || check.GetSymbolAtLocation(assignment.Left) != symbol || assignment.Right.Kind != ast.KindCallExpression || check.GetResolvedSignature(assignment.Right).Declaration() != functions["makeOwnedReader"] {
			t.Fatal("original shared parameter reassignment differs")
		}
	}
	t.Log("pre-producer original callable Param/unique Symbol/owner/type/signature, invocation and write/shadow relation verified; no closed-target or lifetime claim")
}

func callableParameterOfficial19n(t *testing.T, source string, program *compiler.Program, p commonProgramPayload, sample callableParameterCase19n) {
	t.Helper()
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	facts, byID := map[string]arrayGenericFunction{}, map[string]arrayGenericFunction{}
	types, named := map[string]arrayGenericType{}, map[string]arrayGenericType{}
	for _, f := range p.Functions {
		byID[f.ID] = f
	}
	for _, f := range p.Functions {
		name, parent := f.Name, f.LexicalParent
		for parent != nil && *parent != nil {
			owner := byID[**parent]
			name = owner.Name + "." + name
			parent = owner.LexicalParent
		}
		facts[name] = f
	}
	for _, ty := range p.Types {
		types[ty.ID] = ty
		if ty.Name != "" {
			named[ty.Name] = ty
		}
	}
	index := commonMethodIndex37q(t, source, file, check, facts, named)
	rangeOf := index.rangeOf
	owner, child := facts["relayReader"], facts[sample.reader]
	ownerNode, childNode := index.declarations["relayReader"], index.declarations[sample.reader]
	paramNode := ownerNode.AsFunctionDeclaration().Parameters.Nodes[0]
	param := owner.Parameters[0]
	typ := types[param.DeclaredType]
	if len(owner.Parameters) != 1 || param.Name != "reader" || param.Kind != "parameter" || !param.Writable || param.DeclarationRange != rangeOf(paramNode) || param.NameRange != rangeOf(paramNode.Name()) || param.AnnotationRange == nil || *param.AnnotationRange != rangeOf(paramNode.AsParameterDeclaration().Type) || param.LexicalOwner == nil || *param.LexicalOwner == nil || **param.LexicalOwner != owner.ID || typ.Kind != "function" || typ.Parameters == nil || len(*typ.Parameters) != 0 || typ.ReturnType != "i32" {
		t.Fatal("original incoming callable binding/type/signature/writability lost")
	}
	statements := childNode.Body().AsBlock().Statements.Nodes
	returned := statements[len(statements)-1].AsReturnStatement().Expression
	call := returned.AsCallExpression()
	expected := []commonProgramCapture{}
	if !sample.shadow {
		capture := commonProgramCapture{Binding: param.ID, Read: true, Write: sample.mutation, Occurrences: []commonProgramCaptureOccurrence{}}
		if sample.mutation {
			left := statements[0].AsExpressionStatement().Expression.AsBinaryExpression().Left
			capture.Occurrences = append(capture.Occurrences, commonProgramCaptureOccurrence{Range: rangeOf(left), Read: false, Write: true})
		}
		capture.Occurrences = append(capture.Occurrences, commonProgramCaptureOccurrence{Range: rangeOf(call.Expression), Read: true, Write: false})
		expected = append(expected, capture)
	}
	if child.Captures == nil || !reflect.DeepEqual(*child.Captures, expected) {
		t.Fatal("complete callable captures/access/occurrences differ", child.Captures, expected)
	}
	value := child.Body[len(child.Body)-1].Value
	if value == nil || value.Kind != "indirectCall" || value.Expression == nil || value.Expression.Kind != "bindingReference" || value.Range != rangeOf(returned) || value.CalleeRange == nil || *value.CalleeRange != rangeOf(call.Expression) || value.Signature == nil || value.Signature.DeclarationRange != rangeOf(check.GetResolvedSignature(returned).Declaration()) || value.Arguments == nil || len(*value.Arguments) != 0 {
		t.Fatal("original indirect callable operand/selected signature/UTF16 differs")
	}
	if !sample.shadow && value.Expression.Binding != param.ID {
		t.Fatal("incoming function parameter copied or renamed")
	}
	if sample.shadow {
		var local arrayFactsBinding
		for _, b := range child.Bindings {
			if b.Name == "reader" {
				local = b
			}
		}
		if local.ID == "" || local.Writable || local.ID == param.ID || local.DeclaredType != param.DeclaredType || value.Expression.Binding != local.ID {
			t.Fatal("same-name local falsely captures callable parameter")
		}
	}
	if sample.mutation {
		e := child.Body[0].Value
		assignment := statements[0].AsExpressionStatement().Expression.AsBinaryExpression()
		if e == nil || e.Kind != "bindingAssignment" || e.Operator != "=" || e.Target == nil || e.Target.Binding != param.ID || e.Value == nil || e.Value.Kind != "call" || e.Value.Callee != facts["makeOwnedReader"].ID || e.Value.Signature == nil || e.Value.Signature.DeclarationRange != rangeOf(check.GetResolvedSignature(assignment.Right).Declaration()) || e.Range != rangeOf(statements[0].AsExpressionStatement().Expression) {
			t.Fatal("actual shared function parameter assignment lost")
		}
	}
	chooser := facts["makeReaderBox"]
	last := chooser.Body[len(chooser.Body)-1].Value
	chooserNode := index.declarations["makeReaderBox"]
	body := chooserNode.Body().AsBlock().Statements.Nodes
	constructor := body[len(body)-1].AsReturnStatement().Expression.AsNewExpression()
	actualRelay := constructor.Arguments.Nodes[0]
	if last == nil || last.Kind != "new" || last.Arguments == nil || len(*last.Arguments) != 1 {
		t.Fatal("original aggregate construction lost")
	}
	relayCall := (*last.Arguments)[0]
	if relayCall.Kind != "call" || relayCall.Callee != owner.ID || relayCall.Signature == nil || relayCall.Signature.DeclarationRange != rangeOf(check.GetResolvedSignature(actualRelay).Declaration()) || relayCall.Arguments == nil || len(*relayCall.Arguments) != 1 {
		t.Fatal("selected closure is not passed to original relay signature")
	}
	var selected arrayFactsBinding
	for _, b := range chooser.Bindings {
		if b.Name == "selected" {
			selected = b
		}
	}
	if selected.ID == "" {
		t.Fatal("original selected binding absent")
	}
	if (*relayCall.Arguments)[0].Kind != "bindingReference" || (*relayCall.Arguments)[0].Binding != selected.ID {
		t.Fatal("original selected callable payload lost before relay")
	}
	t.Log("full original inventory/callable parameter binding and exact occurrences, ancestor/shadow, assignment, selected signatures and original callable operand facts retained; no consumer or execution verdict")
}
