// SPDX-License-Identifier: Apache-2.0
// Independent exact Source sites for two Spec findings; no production oracle.
package main

import (
	"bytes"
	"errors"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/scanner"
)

const nullablePresenceErrorSitesGroupSource = "function id(x: i32): i32 { return x; }\nexport function entry(x: i32): i32 { return id((x)); }"
const nullablePresenceErrorSitesLocalSource = "export function entry(x: i32): i32 { let y: i32 = x; return y; }"
const nullablePresenceErrorSitesGroupLegacy = "{\"functions\":[{\"body\":[{\"expression\":{\"kind\":\"reference\",\"range\":{\"end\":35,\"start\":33},\"symbol\":\"s3\",\"type\":\"i32\"},\"kind\":\"return\",\"range\":{\"end\":36,\"start\":26}}],\"name\":\"id\",\"parameterInitializers\":[null],\"parameters\":[{\"id\":\"s3\",\"kind\":\"parameter\",\"name\":\"x\",\"presence\":{\"annotation\":{\"end\":18,\"start\":15},\"default\":false,\"null\":false,\"optional\":false,\"range\":{\"end\":18,\"start\":12},\"undefined\":false},\"range\":{\"end\":13,\"start\":12},\"type\":\"i32\",\"writable\":true}],\"range\":{\"end\":38,\"start\":0},\"returnType\":\"i32\",\"symbol\":\"s1\"},{\"body\":[{\"expression\":{\"arguments\":[{\"kind\":\"group\",\"operand\":{\"kind\":\"reference\",\"range\":{\"end\":88,\"start\":87},\"symbol\":\"s4\",\"type\":\"i32\"},\"range\":{\"end\":89,\"start\":86},\"type\":\"i32\"}],\"kind\":\"call\",\"range\":{\"end\":90,\"start\":82},\"signature\":{\"parameters\":[\"i32\"],\"presence\":[{\"annotation\":{\"end\":18,\"start\":15},\"default\":false,\"null\":false,\"optional\":false,\"range\":{\"end\":18,\"start\":12},\"undefined\":false}],\"returnType\":\"i32\"},\"target\":\"s1\",\"type\":\"i32\"},\"kind\":\"return\",\"range\":{\"end\":91,\"start\":75}}],\"name\":\"entry\",\"parameterInitializers\":[null],\"parameters\":[{\"id\":\"s4\",\"kind\":\"parameter\",\"name\":\"x\",\"presence\":{\"annotation\":{\"end\":67,\"start\":64},\"default\":false,\"null\":false,\"optional\":false,\"range\":{\"end\":67,\"start\":61},\"undefined\":false},\"range\":{\"end\":62,\"start\":61},\"type\":\"i32\",\"writable\":true}],\"range\":{\"end\":93,\"start\":38},\"returnType\":\"i32\",\"symbol\":\"s2\"}],\"inputs\":[\"/src/main.ts\",\"/src/minimal-integers-v3.d.ts\"],\"status\":\"ok\",\"symbols\":[{\"id\":\"s1\",\"kind\":\"function\",\"name\":\"id\",\"range\":{\"end\":11,\"start\":8},\"type\":\"i32\",\"writable\":false},{\"id\":\"s2\",\"kind\":\"function\",\"name\":\"entry\",\"range\":{\"end\":60,\"start\":54},\"type\":\"i32\",\"writable\":false},{\"id\":\"s3\",\"kind\":\"parameter\",\"name\":\"x\",\"presence\":{\"annotation\":{\"end\":18,\"start\":15},\"default\":false,\"null\":false,\"optional\":false,\"range\":{\"end\":18,\"start\":12},\"undefined\":false},\"range\":{\"end\":13,\"start\":12},\"type\":\"i32\",\"writable\":true},{\"id\":\"s4\",\"kind\":\"parameter\",\"name\":\"x\",\"presence\":{\"annotation\":{\"end\":67,\"start\":64},\"default\":false,\"null\":false,\"optional\":false,\"range\":{\"end\":67,\"start\":61},\"undefined\":false},\"range\":{\"end\":62,\"start\":61},\"type\":\"i32\",\"writable\":true}]}"
const nullablePresenceErrorSitesLocalLegacy = "{\"functions\":[{\"body\":[{\"initializer\":{\"kind\":\"reference\",\"range\":{\"end\":51,\"start\":49},\"symbol\":\"s2\",\"type\":\"i32\"},\"kind\":\"variable\",\"range\":{\"end\":51,\"start\":40},\"symbol\":{\"id\":\"s3\",\"kind\":\"local\",\"name\":\"y\",\"range\":{\"end\":42,\"start\":40},\"type\":\"i32\",\"writable\":true}},{\"expression\":{\"kind\":\"reference\",\"range\":{\"end\":61,\"start\":59},\"symbol\":\"s3\",\"type\":\"i32\"},\"kind\":\"return\",\"range\":{\"end\":62,\"start\":52}}],\"name\":\"entry\",\"parameterInitializers\":[null],\"parameters\":[{\"id\":\"s2\",\"kind\":\"parameter\",\"name\":\"x\",\"presence\":{\"annotation\":{\"end\":28,\"start\":25},\"default\":false,\"null\":false,\"optional\":false,\"range\":{\"end\":28,\"start\":22},\"undefined\":false},\"range\":{\"end\":23,\"start\":22},\"type\":\"i32\",\"writable\":true}],\"range\":{\"end\":64,\"start\":0},\"returnType\":\"i32\",\"symbol\":\"s1\"}],\"inputs\":[\"/src/main.ts\",\"/src/minimal-integers-v3.d.ts\"],\"status\":\"ok\",\"symbols\":[{\"id\":\"s1\",\"kind\":\"function\",\"name\":\"entry\",\"range\":{\"end\":21,\"start\":15},\"type\":\"i32\",\"writable\":false},{\"id\":\"s2\",\"kind\":\"parameter\",\"name\":\"x\",\"presence\":{\"annotation\":{\"end\":28,\"start\":25},\"default\":false,\"null\":false,\"optional\":false,\"range\":{\"end\":28,\"start\":22},\"undefined\":false},\"range\":{\"end\":23,\"start\":22},\"type\":\"i32\",\"writable\":true},{\"id\":\"s3\",\"kind\":\"local\",\"name\":\"y\",\"range\":{\"end\":42,\"start\":40},\"type\":\"i32\",\"writable\":true}]}"

var nullablePresenceErrorSitesGroupRanges = map[string]nullablePresenceFirstAnchor{
	"id":                 {Range: [2]int{0, 38}, Text: "function id(x: i32): i32 { return x; }"},
	"idName":             {Range: [2]int{8, 11}, Text: " id"},
	"idParameter":        {Range: [2]int{12, 18}, Text: "x: i32"},
	"idParameterName":    {Range: [2]int{12, 13}, Text: "x"},
	"idAnnotation":       {Range: [2]int{15, 18}, Text: "i32"},
	"idBlock":            {Range: [2]int{24, 38}, Text: " { return x; }"},
	"idReturn":           {Range: [2]int{26, 36}, Text: " return x;"},
	"idReference":        {Range: [2]int{33, 35}, Text: " x"},
	"entry":              {Range: [2]int{38, 93}, Text: "\nexport function entry(x: i32): i32 { return id((x)); }"},
	"entryName":          {Range: [2]int{54, 60}, Text: " entry"},
	"entryParameter":     {Range: [2]int{61, 67}, Text: "x: i32"},
	"entryParameterName": {Range: [2]int{61, 62}, Text: "x"},
	"entryAnnotation":    {Range: [2]int{64, 67}, Text: "i32"},
	"entryBlock":         {Range: [2]int{73, 93}, Text: " { return id((x)); }"},
	"entryReturn":        {Range: [2]int{75, 91}, Text: " return id((x));"},
	"call":               {Range: [2]int{82, 90}, Text: " id((x))"},
	"callee":             {Range: [2]int{82, 85}, Text: " id"},
	"group":              {Range: [2]int{86, 89}, Text: "(x)"},
	"inner":              {Range: [2]int{87, 88}, Text: "x"},
	"idAnnotationRaw":    {Range: [2]int{14, 18}, Text: " i32"},
	"entryAnnotationRaw": {Range: [2]int{63, 67}, Text: " i32"},
}
var nullablePresenceErrorSitesLocalRanges = map[string]nullablePresenceFirstAnchor{
	"entry":              {Range: [2]int{0, 64}, Text: "export function entry(x: i32): i32 { let y: i32 = x; return y; }"},
	"entryName":          {Range: [2]int{15, 21}, Text: " entry"},
	"parameter":          {Range: [2]int{22, 28}, Text: "x: i32"},
	"parameterName":      {Range: [2]int{22, 23}, Text: "x"},
	"annotation":         {Range: [2]int{25, 28}, Text: "i32"},
	"block":              {Range: [2]int{34, 64}, Text: " { let y: i32 = x; return y; }"},
	"variableStatement":  {Range: [2]int{36, 52}, Text: " let y: i32 = x;"},
	"declaration":        {Range: [2]int{40, 51}, Text: " y: i32 = x"},
	"localName":          {Range: [2]int{40, 42}, Text: " y"},
	"localAnnotation":    {Range: [2]int{44, 47}, Text: "i32"},
	"initializer":        {Range: [2]int{49, 51}, Text: " x"},
	"return":             {Range: [2]int{52, 62}, Text: " return y;"},
	"reference":          {Range: [2]int{59, 61}, Text: " y"},
	"annotationRaw":      {Range: [2]int{24, 28}, Text: " i32"},
	"localAnnotationRaw": {Range: [2]int{43, 47}, Text: " i32"},
}

func nullablePresenceErrorSitesOfficial(t *testing.T, source string, local bool) {
	t.Helper()
	raw := nullablePresenceBoundaryRequest(t, source, 4)
	r := nullablePresenceBoundaryInput(t, raw)
	program := nullablePresenceFirstProgram(t, r)
	nullablePresenceFirstDiagnostics(t, program)
	file := program.GetSourceFile(r.RootFile)
	if file == nil {
		t.Fatal("official Source file missing")
	}
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	table := nullablePresenceErrorSitesGroupRanges
	if local {
		table = nullablePresenceErrorSitesLocalRanges
	}
	seen := map[string]bool{}
	observe := func(label string, node *ast.Node, token bool) {
		t.Helper()
		expected, exists := table[label]
		if !exists || seen[label] || node == nil || ast.GetSourceFileOfNode(node) != file {
			t.Fatalf("absent/foreign/repeated official node %s", label)
		}
		start := node.Pos()
		if token {
			start = scanner.SkipTrivia(file.Text(), start)
		}
		actual := [2]int{file.GetPositionMap().UTF8ToUTF16(start), file.GetPositionMap().UTF8ToUTF16(node.End())}
		if actual != expected.Range || nullablePresenceFirstSourceSlice(t, source, actual) != expected.Text {
			t.Fatalf("independent raw/token UTF16 site %s differs: actual=%v expected=%v", label, actual, expected.Range)
		}
		seen[label] = true
	}
	var exported *ast.Node
	if !local {
		if len(file.Statements.Nodes) != 2 {
			t.Fatal("two exact ordinary function declarations required")
		}
		id, entry := file.Statements.Nodes[0], file.Statements.Nodes[1]
		if id.Kind != ast.KindFunctionDeclaration || entry.Kind != ast.KindFunctionDeclaration {
			t.Fatal("official ordinary functions changed")
		}
		idParam := id.AsFunctionDeclaration().Parameters.Nodes[0]
		entryParam := entry.AsFunctionDeclaration().Parameters.Nodes[0]
		idReturn := id.Body().AsBlock().Statements.Nodes[0]
		idRef := idReturn.AsReturnStatement().Expression
		entryReturn := entry.Body().AsBlock().Statements.Nodes[0]
		call := entryReturn.AsReturnStatement().Expression
		if call.Kind != ast.KindCallExpression || len(call.AsCallExpression().Arguments.Nodes) != 1 {
			t.Fatal("official direct call shape changed")
		}
		group := call.AsCallExpression().Arguments.Nodes[0]
		if group.Kind != ast.KindParenthesizedExpression {
			t.Fatal("actual inner group missing")
		}
		inner := group.AsParenthesizedExpression().Expression
		if inner.Kind != ast.KindIdentifier {
			t.Fatal("group did not contain direct x")
		}
		observe("id", id, false)
		observe("idName", id.Name(), false)
		observe("idParameter", idParam, false)
		observe("idParameterName", idParam.Name(), false)
		observe("idAnnotationRaw", idParam.AsParameterDeclaration().Type, false)
		observe("idAnnotation", idParam.AsParameterDeclaration().Type, true)
		observe("idBlock", id.Body(), false)
		observe("idReturn", idReturn, false)
		observe("idReference", idRef, false)
		observe("entry", entry, false)
		observe("entryName", entry.Name(), false)
		observe("entryParameter", entryParam, false)
		observe("entryParameterName", entryParam.Name(), false)
		observe("entryAnnotationRaw", entryParam.AsParameterDeclaration().Type, false)
		observe("entryAnnotation", entryParam.AsParameterDeclaration().Type, true)
		observe("entryBlock", entry.Body(), false)
		observe("entryReturn", entryReturn, false)
		observe("call", call, false)
		observe("callee", call.AsCallExpression().Expression, false)
		observe("group", group, false)
		observe("inner", inner, false)
		idSymbol := check.GetSymbolAtLocation(id.Name())
		entryBinding := check.GetSymbolAtLocation(entryParam.Name())
		if idSymbol == nil || entryBinding == nil || check.GetSymbolAtLocation(call.AsCallExpression().Expression) != idSymbol || check.GetSymbolAtLocation(inner) != entryBinding || check.GetSymbolAtLocation(idRef) != check.GetSymbolAtLocation(idParam.Name()) {
			t.Fatal("official symbols do not own actual call/inner references")
		}
		signature := check.GetResolvedSignature(call)
		if signature == nil || signature.Declaration() != id || len(signature.Parameters()) != 1 || signature.Parameters()[0] != check.GetSymbolAtLocation(idParam.Name()) {
			t.Fatal("actual complete call signature changed")
		}
		payload := check.GetTypeFromTypeNode(entryParam.AsParameterDeclaration().Type)
		if payload == nil || check.GetTypeAtLocation(group) != payload || check.GetTypeAtLocation(inner) != payload || check.GetReturnTypeOfSignature(signature) != payload || check.GetTypeOfSymbol(entryBinding) != payload {
			t.Fatal("official group/scalar payload changed")
		}
		exported = entry
	} else {
		if len(file.Statements.Nodes) != 1 {
			t.Fatal("one exact ordinary function required")
		}
		entry := file.Statements.Nodes[0]
		if entry.Kind != ast.KindFunctionDeclaration {
			t.Fatal("official entry is not a function")
		}
		parameter := entry.AsFunctionDeclaration().Parameters.Nodes[0]
		body := entry.Body().AsBlock().Statements.Nodes
		if len(body) != 2 || body[0].Kind != ast.KindVariableStatement || body[1].Kind != ast.KindReturnStatement {
			t.Fatal("official let/return body changed")
		}
		variable, returned := body[0], body[1]
		list := variable.AsVariableStatement().DeclarationList
		if list.Flags&ast.NodeFlagsLet == 0 || len(list.AsVariableDeclarationList().Declarations.Nodes) != 1 {
			t.Fatal("actual writable let declaration missing")
		}
		declaration := list.AsVariableDeclarationList().Declarations.Nodes[0]
		initializer := declaration.AsVariableDeclaration().Initializer
		reference := returned.AsReturnStatement().Expression
		observe("entry", entry, false)
		observe("entryName", entry.Name(), false)
		observe("parameter", parameter, false)
		observe("parameterName", parameter.Name(), false)
		observe("annotationRaw", parameter.AsParameterDeclaration().Type, false)
		observe("annotation", parameter.AsParameterDeclaration().Type, true)
		observe("block", entry.Body(), false)
		observe("variableStatement", variable, false)
		observe("declaration", declaration, false)
		observe("localName", declaration.Name(), false)
		observe("localAnnotationRaw", declaration.AsVariableDeclaration().Type, false)
		observe("localAnnotation", declaration.AsVariableDeclaration().Type, true)
		observe("initializer", initializer, false)
		observe("return", returned, false)
		observe("reference", reference, false)
		localSymbol := check.GetSymbolAtLocation(declaration.Name())
		parameterSymbol := check.GetSymbolAtLocation(parameter.Name())
		if localSymbol == nil || len(localSymbol.Declarations) != 1 || localSymbol.Declarations[0] != declaration || check.GetSymbolAtLocation(reference) != localSymbol || parameterSymbol == nil || check.GetSymbolAtLocation(initializer) != parameterSymbol {
			t.Fatal("actual writable symbol declaration/uses changed")
		}
		payload := check.GetTypeFromTypeNode(parameter.AsParameterDeclaration().Type)
		if payload == nil || check.GetTypeOfSymbol(localSymbol) != payload || check.GetTypeAtLocation(reference) != payload || check.GetTypeAtLocation(initializer) != payload {
			t.Fatal("official local scalar payload changed")
		}
		exported = entry
	}
	if len(seen) != len(table) {
		t.Fatal("complete manual AST site table was not observed")
	}
	exports := check.GetExportsOfModule(file.AsNode().Symbol())
	if len(exports) != 1 || exports[0].Name != "entry" || len(exports[0].Declarations) != 1 || exports[0].Declarations[0] != exported {
		t.Fatal("complete actual entry export changed")
	}
}

func TestNullableFactsPresenceErrorSites(t *testing.T) {
	rows := []struct {
		name, source, legacy, v3Error string
		local                         bool
		where                         span
		expectedCause                 int
	}{
		{"group-argument", nullablePresenceErrorSitesGroupSource, nullablePresenceErrorSitesGroupLegacy, "[r03-nullable-facts-capability] only checked references and direct calls", false, span{Start: 86, End: 89}, 1},
		{"writable-local", nullablePresenceErrorSitesLocalSource, nullablePresenceErrorSitesLocalLegacy, "[r03-nullable-facts-capability] only ordinary scalar/nullable bindings and const locals", true, span{Start: 40, End: 51}, 0},
	}
	ready := true
	// Both complete official/AST/symbol/type/export and complete legacy controls
	// must finish before the first private public-command rejection is observed.
	for _, row := range rows {
		ok := t.Run("controls/"+row.name, func(t *testing.T) {
			nullablePresenceErrorSitesOfficial(t, row.source, row.local)
			raw := nullablePresenceBoundaryRequest(t, row.source, 4)
			facts := nullablePresenceBoundaryLegacy(t, raw, []byte(row.legacy))
			if facts.Status != "ok" || len(facts.Diagnostics) != 0 {
				t.Fatal("complete original legacy admission rejected new Source")
			}
		})
		ready = ready && ok
	}
	if !ready {
		t.Fatal("independent prerequisites failed before private command calls")
	}
	for _, row := range rows {
		t.Run("v3-plain/"+row.name, func(t *testing.T) {
			raw := nullablePresenceBoundaryRequest(t, row.source, 3)
			r, w, err := nullablePresenceBoundaryCommand(t.Context(), []string{"--nullable-facts-io-v3"}, raw)
			var structured *nullablePresenceFactsError
			if err == nil || err.Error() != row.v3Error || errors.As(err, &structured) || r.delivered != len(raw) || w.writes != 0 {
				t.Fatalf("v3 original plain error/order changed: %v", err)
			}
		})
	}
	for _, row := range rows {
		t.Run("v4-exact-site/"+row.name, func(t *testing.T) {
			raw := nullablePresenceBoundaryRequest(t, row.source, 4)
			before := bytes.Clone(raw)
			r, w, err := nullablePresenceBoundaryCommand(t.Context(), []string{"--nullable-facts-io-v4"}, raw)
			if r.delivered != len(raw) || w.writes != 0 || !bytes.Equal(raw, before) {
				t.Fatal("error-site rejection did not preserve complete input/zero publish")
			}
			got := nullablePresenceBoundaryError(t, err, "capability", &row.where, row.expectedCause)
			if errors.Unwrap(got) != got.Cause {
				t.Fatal("structured private error lost its actual cause chain")
			}
			if row.expectedCause == 1 && got.Cause.Error() != row.v3Error {
				t.Fatal("group rejection did not retain the original internal expression error")
			}
		})
	}
}
