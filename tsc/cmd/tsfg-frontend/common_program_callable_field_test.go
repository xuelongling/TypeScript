// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
)

func callableFieldSource26n(t *testing.T) []byte {
	t.Helper()
	source, err := os.ReadFile("testdata/common-program-v2/callable-field.ts")
	if err != nil {
		t.Fatal(err)
	}
	if len(source) != 1916 || arrayFactsSHA(source) != "12f53d905cf09cd56a6eba7e04f519772e99152a50c37a6008987468363c21d9" || arrayFactsSHA(source[:1318]) != "6a86b0b018c8129985fe7f9bdbd50d9399b5622c3ba83a8466ddbb5e8b12ef92" {
		t.Fatal("original complete field-call source/prefix changed")
	}
	return source
}

func TestCommonProgramCallableField26n(t *testing.T) {
	source := callableFieldSource26n(t)
	original := commonMethodProgram37q(t, string(source))
	var output bytes.Buffer
	if err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(readSpanCaptureRequest26i(t, source)), &output); err != nil {
		t.Fatal(err)
	}
	var packet commonProgramPackage
	if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	p := packet.Payload
	if len(p.Functions) != 17 || len(p.Globals) != 2 || len(p.Modules) != 1 || len(p.Modules[0].Exports) != 7 || len(p.Closure.Roots) != 8 || len(p.Closure.RequiredRoots) != 2 {
		t.Fatal("complete program inventory changed")
	}
	types, named := map[string]arrayGenericType{}, map[string]arrayGenericType{}
	byID, facts := map[string]arrayGenericFunction{}, map[string]arrayGenericFunction{}
	for _, typ := range p.Types {
		types[typ.ID] = typ
		if typ.Name != "" {
			named[typ.Name] = typ
		}
	}
	for _, fn := range p.Functions {
		byID[fn.ID] = fn
	}
	for _, fn := range p.Functions {
		name := fn.Name
		if fn.OwnerType != "" {
			name = types[fn.OwnerType].Name + "." + name
		}
		for parent := fn.LexicalParent; parent != nil && *parent != nil; parent = byID[**parent].LexicalParent {
			name = byID[**parent].Name + "." + name
		}
		facts[name] = fn
	}
	file := original.GetSourceFile("/src/main.ts")
	check, release := original.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	index := commonMethodIndex37q(t, string(source), file, check, facts, named)
	if len(index.declarations) != 17 || len(index.classes) != 4 {
		t.Fatal("complete original AST inventory lost")
	}
	node := index.calls["fieldReceiver().apply(fieldArgument())"]
	if node == nil {
		t.Fatal("original field invocation missing")
	}
	originalCall := node.AsCallExpression()
	access := originalCall.Expression.AsPropertyAccessExpression()
	fieldBox := named["FieldBox"]
	if fieldBox.Fields == nil || len(*fieldBox.Fields) != 1 || fieldBox.Constructor == nil {
		t.Fatal("original field/constructor missing")
	}
	var ctor *ast.Node
	for _, member := range index.classes["FieldBox"].Members() {
		if member.Kind == ast.KindConstructor {
			ctor = member
		}
	}
	if ctor == nil || len(ctor.AsConstructorDeclaration().Parameters.Nodes) != 1 {
		t.Fatal("original constructor parameter-property missing")
	}
	param := ctor.AsConstructorDeclaration().Parameters.Nodes[0]
	symbol := check.GetSymbolAtLocation(access.Name())
	if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != param || check.GetResolvedSignature(node).Declaration() != param.AsParameterDeclaration().Type {
		t.Fatal("field call is not original parameter-property function signature")
	}
	if access.Expression.Kind != ast.KindCallExpression || len(originalCall.Arguments.Nodes) != 1 || originalCall.Arguments.Nodes[0].Kind != ast.KindCallExpression || check.GetResolvedSignature(access.Expression).Declaration() != index.declarations["fieldReceiver"] || check.GetResolvedSignature(originalCall.Arguments.Nodes[0]).Declaration() != index.declarations["fieldArgument"] || access.Expression.End() >= originalCall.Arguments.Nodes[0].Pos() {
		t.Fatal("original receiver-before-argument evaluation tree changed")
	}
	fn := facts["captureSpanOrdinary"]
	value := fn.Body[0].Initializer
	field := (*fieldBox.Fields)[0]
	if value == nil || value.Kind != "indirectCall" || value.Range != index.rangeOf(node) || value.CalleeRange == nil || *value.CalleeRange != index.rangeOf(originalCall.Expression) || value.Expression == nil || value.Expression.Kind != "fieldAccess" || value.Expression.Property != field.ID || value.Expression.NameRange == nil || *value.Expression.NameRange != index.rangeOf(access.Name()) || value.Signature == nil || value.Signature.DeclarationRange != index.rangeOf(param.AsParameterDeclaration().Type) {
		t.Fatal("stored callable field/signature/UTF16 ranges not preserved")
	}
	receiver := value.Expression.Receiver
	if receiver == nil || receiver.Kind != "call" || receiver.Callee != facts["fieldReceiver"].ID || receiver.Range != index.rangeOf(access.Expression) || value.Arguments == nil || len(*value.Arguments) != 1 || (*value.Arguments)[0].Kind != "call" || (*value.Arguments)[0].Callee != facts["fieldArgument"].ID || (*value.Arguments)[0].Range != index.rangeOf(originalCall.Arguments.Nodes[0]) {
		t.Fatal("receiver/argument original calls lost or duplicated")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var tree any
	if err = json.Unmarshal(raw, &tree); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	var walk func(any)
	walk = func(node any) {
		switch v := node.(type) {
		case []any:
			for _, child := range v {
				walk(child)
			}
		case map[string]any:
			if v["kind"] == "call" {
				if id, ok := v["callee"].(string); ok {
					counts[id]++
				}
			}
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(tree)
	if counts[facts["fieldReceiver"].ID] != 1 || counts[facts["fieldArgument"].ID] != 1 {
		t.Fatal("field invocation projects receiver/argument more than once")
	}
	construction := facts["fieldReceiver"].Body[1].Value
	if construction == nil || construction.Kind != "new" || construction.Callee != fieldBox.ID || construction.Arguments == nil || len(*construction.Arguments) != 1 || (*construction.Arguments)[0].Kind != "functionReference" || (*construction.Arguments)[0].FunctionID != facts["fieldTarget"].ID || fieldBox.Constructor.Parameters[0].ID != field.ParameterBinding {
		t.Fatal("constructor must preserve original function payload and parameter-property join")
	}
	if fn.Body[1].Initializer.Kind != "call" || fn.Body[1].Initializer.Callee != facts["directCall"].ID || fn.Body[2].Initializer.Kind != "new" || fn.Body[3].Value.Kind != "call" || fn.Body[3].Value.Callee != facts["MethodBox.m"].ID || fn.Body[3].Value.Receiver == nil || facts["directCall"].Body[0].Value.Callee != facts["fieldTarget"].ID {
		t.Fatal("direct function, method or constructor route changed")
	}
	t.Log("original Program four diagnostics0; owned function field with original Symbol/signature/ranges, receiver then argument once, real method/direct/constructor routes preserved; no execution or target authority")
}
