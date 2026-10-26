// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"slices"
	"testing"
	"unicode/utf16"
)

// The independent frozen input retains two original arguments even though
// the structural source method has no parameters. Source does not adapt calls.
func TestCommonProgramMethodAdaptation37q(t *testing.T) {
	raw, err := os.ReadFile("testdata/common-program-v1/common-method-adaptation.ts")
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 984 || arrayFactsSHA(raw) != "483691a454754cdb80b774fbb3066d637458013b84f2f4ef10219fec3755e270" {
		t.Fatal("frozen adaptation Source changed")
	}
	packet := commonMethodPacket37q(t, "common-method-adaptation", []string{"adapted", "ordinary"})
	p := packet.Payload
	if packet.Version != 1 || len(p.Diagnostics) != 0 || len(p.Functions) != 10 || len(p.Globals) != 2 || len(p.Modules) != 1 || len(p.Closure.Roots) != 5 || len(p.EntryCompatibilityRoots) != 3 || p.Files[0].SHA256 != arrayFactsSHA(raw) || p.Files[0].UTF16Length != 982 {
		t.Fatal("complete adaptation inventory or original Source identity changed")
	}
	types, typeNames, members := map[string]arrayGenericType{}, map[string]string{}, map[string]string{}
	fields := 0
	for _, typ := range p.Types {
		if typ.Kind == "value" || typ.Kind == "reference" {
			types[typ.Name], typeNames[typ.ID] = typ, typ.Name
			for _, field := range *typ.Fields {
				fields++
				members[field.ID] = typ.Name + "." + field.Name
			}
		}
	}
	functions, free := map[string]arrayGenericFunction{}, []string{}
	for _, fn := range p.Functions {
		name := fn.Name
		if fn.OwnerType != "" {
			name = typeNames[fn.OwnerType] + "." + fn.Name
		} else {
			free = append(free, name)
		}
		functions[name], members[fn.ID] = fn, name
	}
	if len(types) != 4 || fields != 4 || !slices.Equal(free, []string{"add", "left", "right", "ordinary", "mapFewer", "sideArray", "sideScalar", "adapted"}) {
		t.Fatal("four classes/own fields and eight free declarations must remain")
	}
	target, fewer := functions["Target.m"], functions["Fewer.m"]
	if !slices.Equal(types["Target"].Methods, []string{target.ID}) || !slices.Equal(types["Fewer"].Methods, []string{fewer.ID}) || len(target.Parameters) != 2 || target.ReturnType != "void" || len(target.Body) != 0 || len(fewer.Parameters) != 0 || fewer.ReturnType != "i32" {
		t.Fatal("original parameter-prefix and void/nonvoid signatures changed")
	}
	if target.Parameters[1].DeclaredType != "i32" {
		t.Fatal("original second method parameter is i32")
	}
	arrayFound := false
	for _, typ := range p.Types {
		if typ.ID == target.Parameters[0].DeclaredType {
			arrayFound = typ.Kind == "array" && typ.ElementType == "i32"
		}
	}
	if !arrayFound {
		t.Fatal("original first method parameter is array<i32>")
	}
	converted := functions["mapFewer"].Body[0].Value
	if converted == nil || converted.Kind != "bindingReference" || converted.Binding != functions["mapFewer"].Parameters[0].ID || converted.FlowType != types["Fewer"].ID || len(converted.CompatibilityRoots) != 1 {
		t.Fatal("structural return must retain original Fewer parameter")
	}
	found := false
	for _, row := range p.CompatibilityWitnesses {
		proof := row.Compatibility
		if proof.SourceType == types["Fewer"].ID && proof.TargetType == types["Target"].ID && proof.Origin.Start == converted.Range.Start && proof.Origin.End == converted.Range.End {
			pairs := []string{}
			for _, pair := range proof.Members {
				pairs = append(pairs, members[pair.Source]+">"+members[pair.Target])
			}
			slices.Sort(pairs)
			found = proof.Assignable && slices.Equal(pairs, []string{"Fewer.m>Target.m", "Fewer.n>Target.n"}) && len(row.Dependencies) == 0
		}
	}
	if !found {
		t.Fatal("original Fewer-to-Target n/m correspondence missing")
	}
	body := functions["adapted"].Body
	if len(body) != 4 || body[0].Initializer == nil || body[0].Initializer.Callee != types["Fewer"].ID || body[1].Initializer == nil || body[1].Initializer.Callee != functions["mapFewer"].ID || (*body[1].Initializer.Arguments)[0].Binding != body[0].Binding {
		t.Fatal("original actual construction and view binding changed")
	}
	call := body[2].Value
	if call == nil || call.Kind != "call" || call.Callee != target.ID || call.Type != "void" || call.Receiver == nil || call.Receiver.Binding != body[1].Binding || call.Receiver.Type != types["Target"].ID || call.Signature == nil || call.Signature.DeclarationRange != target.DeclarationRange || call.Signature.ReturnType != "void" || len(*call.Arguments) != 2 {
		t.Fatal("static Target.m call must retain void signature and both operands")
	}
	arguments := *call.Arguments
	if arguments[0].Kind != "call" || arguments[0].Callee != functions["sideArray"].ID || arguments[1].Kind != "call" || arguments[1].Callee != functions["sideScalar"].ID || call.Receiver.Range.End >= arguments[0].Range.Start || arguments[0].Range.End >= arguments[1].Range.Start {
		t.Fatal("receiver, sideArray, sideScalar original order changed")
	}
	returned := body[3].Value
	if returned == nil || returned.Kind != "fieldAccess" || members[returned.Property] != "Fewer.n" || returned.Receiver.Binding != body[0].Binding || returned.Type != "i32" || fewer.Body[0].Value.Property != returned.Property {
		t.Fatal("original Fewer field identity changed through static void use")
	}
	if functions["sideArray"].Body[0].Value.Target.Binding != p.Globals[1].Binding.ID || functions["sideArray"].Body[1].Value.Kind != "arrayLiteral" || functions["sideScalar"].Body[0].Value.Target.Binding != p.Globals[0].Binding.ID || functions["ordinary"].Body[1].CatchClause == nil {
		t.Fatal("complete side-effect operands and ordinary EH must remain")
	}
	units := utf16.Encode([]rune(string(raw)))
	for _, occurrence := range []struct {
		value *arrayGenericExpression
		text  string
	}{{call, "view.m(sideArray(), sideScalar())"}, {&arguments[0], "sideArray()"}, {&arguments[1], "sideScalar()"}, {returned, "actual.n"}} {
		if string(utf16.Decode(units[occurrence.value.Range.Start:occurrence.value.Range.End])) != occurrence.text {
			t.Fatal("original adaptation UTF16 anchor changed", occurrence.text)
		}
	}
	commonMethodAdaptationOfficial37q(t, string(raw), functions, types)
}

func commonMethodAdaptationOfficial37q(t *testing.T, source string, facts map[string]arrayGenericFunction, types map[string]arrayGenericType) {
	t.Helper()
	program := commonMethodProgram37q(t, source)
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	index := commonMethodIndex37q(t, source, file, check, facts, types)
	classes, declarations, calls, rangeOf := index.classes, index.declarations, index.calls, index.rangeOf
	if len(classes) != 4 || len(declarations) != 10 {
		t.Fatal("independent complete adaptation declaration inventory changed")
	}
	fewer, target := check.GetDeclaredTypeOfSymbol(check.GetSymbolAtLocation(classes["Fewer"].Name())), check.GetDeclaredTypeOfSymbol(check.GetSymbolAtLocation(classes["Target"].Name()))
	if !check.IsTypeAssignableTo(fewer, target) || check.GetPropertyOfType(fewer, "m") != check.GetSymbolAtLocation(declarations["Fewer.m"].Name()) || check.GetPropertyOfType(target, "m") != check.GetSymbolAtLocation(declarations["Target.m"].Name()) {
		t.Fatal("independent original structural method relation differs")
	}
	if len(declarations["Fewer.m"].AsMethodDeclaration().Parameters.Nodes) != 0 || len(declarations["Target.m"].AsMethodDeclaration().Parameters.Nodes) != 2 {
		t.Fatal("independent method parameter-prefix signatures differ")
	}
	node := calls["view.m(sideArray(), sideScalar())"]
	if node == nil || check.GetResolvedSignature(node) == nil || check.GetResolvedSignature(node).Declaration() != declarations["Target.m"] || check.GetSymbolAtLocation(node.AsCallExpression().Expression.Name()) != check.GetSymbolAtLocation(declarations["Target.m"].Name()) {
		t.Fatal("independent selected static Target.m signature differs")
	}
	arguments := node.AsCallExpression().Arguments.Nodes
	if len(arguments) != 2 || check.GetResolvedSignature(arguments[0]).Declaration() != declarations["sideArray"] || check.GetResolvedSignature(arguments[1]).Declaration() != declarations["sideScalar"] || arguments[0].End() >= arguments[1].Pos() {
		t.Fatal("independent original two argument evaluations differ")
	}
	actual := facts["adapted"].Body[2].Value
	if actual.Range != rangeOf(node) || (*actual.Arguments)[0].Range != rangeOf(arguments[0]) || (*actual.Arguments)[1].Range != rangeOf(arguments[1]) {
		t.Fatal("packet method/arguments differ from independent official UTF16 anchors")
	}
	t.Log("independent original Program: 4x0 diagnostics, 10 callable Symbols/ranges, 4 classes, Fewer-to-Target assignability and static Target.m with two ordered original arguments verified; no dispatch/execution result")
}
