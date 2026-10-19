// SPDX-License-Identifier: Apache-2.0
// Active success explicitly adopts v4; Root retains original v2 FIRST/RED unchanged.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

func arrayOwnerFirstOfficial23e(t *testing.T, r request) {
	t.Helper()
	contents, paths := map[string]string{}, []string{}
	for _, input := range r.Files {
		contents[input.Path] = input.Content
		paths = append(paths, input.Path)
	}
	slices.Sort(paths)
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{FileNames: paths, CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext}}},
		Host:   compiler.NewCompilerHost(r.SourceRoot, vfstest.FromMap(contents, true), r.SourceRoot, nil, nil), SingleThreaded: core.TSTrue,
	})
	for _, group := range []struct {
		name  string
		items []*ast.Diagnostic
	}{
		{"syntax", program.GetSyntacticDiagnostics(t.Context(), nil)},
		{"semantic", program.GetSemanticDiagnostics(t.Context(), nil)},
		{"program", program.GetProgramDiagnostics()},
		{"global", program.GetGlobalDiagnostics(t.Context())},
	} {
		if len(group.items) != 0 {
			t.Fatalf("complete official %s diagnostics: %+v", group.name, group.items)
		}
	}
	consumed := []string{}
	for _, file := range program.GetSourceFiles() {
		consumed = append(consumed, file.FileName())
		if contents[file.FileName()] != file.Text() {
			t.Fatal("actual complete source text changed")
		}
	}
	slices.Sort(consumed)
	if !slices.Equal(consumed, paths) {
		t.Fatal("actual closed two-file Program differs")
	}
	file, dts := program.GetSourceFile(r.RootFile), program.GetSourceFile(r.Declarations.Paths[0])
	if file == nil || dts == nil || len(file.Statements.Nodes) != 2 || len(dts.Statements.Nodes) != 14 || !ast.IsExternalModule(file) {
		t.Fatal("complete original two-function module and pinned declarations")
	}
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	arrayFirstV4OriginRecords25(t, file, dts, check)
	arrayNode, f64Node, u32Node := dts.Statements.Nodes[9], dts.Statements.Nodes[10], dts.Statements.Nodes[11]
	if arrayNode.Kind != ast.KindInterfaceDeclaration || arrayNode.Name().Text() != "Array" || f64Node.Kind != ast.KindTypeAliasDeclaration || f64Node.Name().Text() != "f64" || u32Node.Kind != ast.KindTypeAliasDeclaration || u32Node.Name().Text() != "u32" {
		t.Fatal("actual pinned Array/f64/u32 declarations")
	}
	arraySymbol, f64Symbol, u32Symbol := check.GetSymbolAtLocation(arrayNode.Name()), check.GetSymbolAtLocation(f64Node.Name()), check.GetSymbolAtLocation(u32Node.Name())
	for _, pair := range []struct {
		symbol *ast.Symbol
		node   *ast.Node
	}{{arraySymbol, arrayNode}, {f64Symbol, f64Node}, {u32Symbol, u32Node}} {
		if pair.symbol == nil || len(pair.symbol.Declarations) != 1 || pair.symbol.Declarations[0] != pair.node {
			t.Fatal("unique original pinned declaration Symbol")
		}
	}
	members := arrayNode.AsInterfaceDeclaration().Members.Nodes
	if len(members) != 2 || members[0].Kind != ast.KindIndexSignature || members[1].Kind != ast.KindPropertySignature || check.GetTypeFromTypeNode(f64Node.AsTypeAliasDeclaration().Type) != check.GetNumberType() || check.GetTypeFromTypeNode(u32Node.AsTypeAliasDeclaration().Type) != check.GetNumberType() {
		t.Fatal("pinned numeric Array indexer and scalar aliases")
	}
	touchNode, entryNode := file.Statements.Nodes[0], file.Statements.Nodes[1]
	if touchNode.Kind != ast.KindFunctionDeclaration || entryNode.Kind != ast.KindFunctionDeclaration {
		t.Fatal("ordinary named functions")
	}
	touch, entry := touchNode.AsFunctionDeclaration(), entryNode.AsFunctionDeclaration()
	if touch.Name().Text() != "touch" || entry.Name().Text() != "entry" || len(touch.Parameters.Nodes) != 3 || len(entry.Parameters.Nodes) != 2 || touch.Body == nil || entry.Body == nil {
		t.Fatal("complete original required parameter functions")
	}
	functionSymbols := []*ast.Symbol{}
	parameterSymbols := [][]*ast.Symbol{}
	for _, node := range []*ast.Node{touchNode, entryNode} {
		fn := node.AsFunctionDeclaration()
		symbol := check.GetSymbolAtLocation(fn.Name())
		if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != node || node.ModifierFlags() != ast.ModifierFlagsExport || fn.TypeParameters != nil || fn.AsteriskToken != nil || fn.FullSignature != nil {
			t.Fatal("unique ordinary actual exported function")
		}
		sigs := check.GetSignaturesOfType(check.GetTypeOfSymbol(symbol), checker.SignatureKindCall)
		if len(sigs) != 1 || sigs[0].Declaration() != node || len(sigs[0].Parameters()) != len(fn.Parameters.Nodes) || sigs[0].MinArgumentCount() != len(fn.Parameters.Nodes) || len(sigs[0].TypeParameters()) != 0 || sigs[0].ThisParameter() != nil || sigs[0].HasRestParameter() || check.GetReturnTypeOfSignature(sigs[0]) != check.GetNumberType() {
			t.Fatal("complete actual required signature and Number result")
		}
		arrayFirstF64Annotation25(t, check, fn.Type, f64Symbol)
		params := []*ast.Symbol{}
		for i, paramNode := range fn.Parameters.Nodes {
			param := paramNode.AsParameterDeclaration()
			actual := check.GetSymbolAtLocation(paramNode.Name())
			if actual == nil || len(actual.Declarations) != 1 || actual.Declarations[0] != paramNode || actual != sigs[0].Parameters()[i] || param.QuestionToken != nil || param.Initializer != nil || param.DotDotDotToken != nil {
				t.Fatal("actual original required parameter and signature-order identity")
			}
			typ := param.Type
			if node == touchNode && i == 0 {
				if typ == nil || typ.Kind != ast.KindArrayType {
					t.Fatal("original explicit array parameter TypeNode")
				}
				arrayFirstF64Annotation25(t, check, typ.AsArrayTypeNode().ElementType, f64Symbol)
				arrayFirstArrayType25(t, check, check.GetTypeFromTypeNode(typ), arraySymbol, members[0], false)
				arrayFirstArrayType25(t, check, check.GetTypeOfSymbol(actual), arraySymbol, members[0], false)
			} else {
				wantName, wantSymbol := "u32", u32Symbol
				if paramNode.Name().Text() == "value" {
					wantName, wantSymbol = "f64", f64Symbol
				}
				if typ == nil || typ.Kind != ast.KindTypeReference || typ.AsTypeReferenceNode().TypeName.Text() != wantName || check.GetSymbolAtLocation(typ.AsTypeReferenceNode().TypeName) != wantSymbol || check.GetTypeFromTypeNode(typ) != check.GetNumberType() || check.GetTypeOfSymbol(actual) != check.GetNumberType() {
					t.Fatal("scalar alias syntax, actual pinned origin and Number parameter")
				}
			}
			params = append(params, actual)
		}
		functionSymbols = append(functionSymbols, symbol)
		parameterSymbols = append(parameterSymbols, params)
	}
	touchBody, entryBody := touch.Body.AsBlock().Statements.Nodes, entry.Body.AsBlock().Statements.Nodes
	if len(touchBody) != 2 || len(entryBody) != 3 || touchBody[0].Kind != ast.KindExpressionStatement || touchBody[1].Kind != ast.KindReturnStatement || entryBody[0].Kind != ast.KindVariableStatement || entryBody[1].Kind != ast.KindVariableStatement || entryBody[2].Kind != ast.KindReturnStatement {
		t.Fatal("complete original body order")
	}
	store := touchBody[0].AsExpressionStatement().Expression.AsBinaryExpression()
	target, read := store.Left.AsElementAccessExpression(), touchBody[1].AsReturnStatement().Expression.AsElementAccessExpression()
	if store.OperatorToken.Kind != ast.KindEqualsToken || check.GetSymbolAtLocation(target.Expression) != parameterSymbols[0][0] || check.GetSymbolAtLocation(read.Expression) != parameterSymbols[0][0] || check.GetSymbolAtLocation(target.ArgumentExpression) != parameterSymbols[0][1] || check.GetSymbolAtLocation(read.ArgumentExpression) != parameterSymbols[0][1] || check.GetSymbolAtLocation(store.Right) != parameterSymbols[0][2] {
		t.Fatal("array receiver/index/RHS and checked read retain original parameter Symbols")
	}
	for _, node := range []*ast.Node{target.Expression, read.Expression} {
		arrayFirstArrayType25(t, check, check.GetTypeAtLocation(node), arraySymbol, members[0], false)
	}
	for _, node := range []*ast.Node{store.Left, store.Right, touchBody[0].AsExpressionStatement().Expression, touchBody[1].AsReturnStatement().Expression} {
		if check.GetTypeAtLocation(node) != check.GetNumberType() {
			t.Fatal("official numeric mutation/read result and RHS")
		}
	}
	actualNode := entryBody[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0]
	aliasNode := entryBody[1].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0]
	actual, alias := actualNode.AsVariableDeclaration(), aliasNode.AsVariableDeclaration()
	actualSymbol, aliasSymbol := check.GetSymbolAtLocation(actualNode.Name()), check.GetSymbolAtLocation(aliasNode.Name())
	if actualSymbol == nil || aliasSymbol == nil || actualSymbol == aliasSymbol || len(actualSymbol.Declarations) != 1 || actualSymbol.Declarations[0] != actualNode || len(aliasSymbol.Declarations) != 1 || aliasSymbol.Declarations[0] != aliasNode || check.GetSymbolAtLocation(alias.Initializer) != actualSymbol || actual.Type.Kind != ast.KindArrayType || actual.Initializer.Kind != ast.KindArrayLiteralExpression || len(actual.Initializer.AsArrayLiteralExpression().Elements.Nodes) != 2 {
		t.Fatal("original local allocation and independent alias Symbol chain")
	}
	arrayFirstF64Annotation25(t, check, actual.Type.AsArrayTypeNode().ElementType, f64Symbol)
	for _, typ := range []*checker.Type{check.GetTypeFromTypeNode(actual.Type), check.GetTypeOfSymbol(actualSymbol), check.GetTypeOfSymbol(aliasSymbol), check.GetTypeAtLocation(actual.Initializer)} {
		arrayFirstArrayType25(t, check, typ, arraySymbol, members[0], false)
	}
	if !check.IsTypeAssignableTo(check.GetTypeAtLocation(actual.Initializer), check.GetTypeOfSymbol(actualSymbol)) {
		t.Fatal("official literal assignment context")
	}
	callNode := entryBody[2].AsReturnStatement().Expression
	if callNode.Kind != ast.KindCallExpression {
		t.Fatal("original direct call")
	}
	call := callNode.AsCallExpression()
	resolved := check.GetResolvedSignature(callNode)
	if call.Expression.Kind != ast.KindIdentifier || call.QuestionDotToken != nil || call.TypeArguments != nil || len(call.Arguments.Nodes) != 3 || check.GetSymbolAtLocation(call.Expression) != functionSymbols[0] || resolved == nil || resolved.Declaration() != touchNode || len(resolved.Parameters()) != 3 || check.GetReturnTypeOfSignature(resolved) != check.GetNumberType() || check.GetTypeAtLocation(callNode) != check.GetNumberType() {
		t.Fatal("actual callee Symbol, original resolved signature and scalar result")
	}
	for i, want := range []*ast.Symbol{aliasSymbol, parameterSymbols[1][0], parameterSymbols[1][1]} {
		if check.GetSymbolAtLocation(call.Arguments.Nodes[i]) != want || resolved.Parameters()[i] != parameterSymbols[0][i] {
			t.Fatal("actual ordered arguments and original signature parameters")
		}
	}
	arrayFirstArrayType25(t, check, check.GetTypeAtLocation(call.Arguments.Nodes[0]), arraySymbol, members[0], false)
	for _, node := range call.Arguments.Nodes[1:] {
		if check.GetTypeAtLocation(node) != check.GetNumberType() {
			t.Fatal("actual scalar call argument types")
		}
	}
	if parameterSymbols[0][1] == parameterSymbols[1][0] || parameterSymbols[0][2] == parameterSymbols[1][1] {
		t.Fatal("same-spelled scalar bindings must stay distinct")
	}
	exports := check.GetExportsOfModule(file.AsNode().Symbol())
	if len(exports) != 2 {
		t.Fatal("complete actual direct export table")
	}
	for i, symbol := range functionSymbols {
		count := 0
		for _, export := range exports {
			if export != nil && export.Name == []string{"touch", "entry"}[i] && (export == symbol || symbol.ExportSymbol == export) {
				count++
			}
		}
		if count != 1 {
			t.Fatal("actual unique export/declaration join")
		}
	}
	anchors := map[string]*ast.Node{
		"touch.function": touchNode, "touch.name": touch.Name(), "touch.returnType": touch.Type,
		"touch.store": touchBody[0], "touch.store.expression": touchBody[0].AsExpressionStatement().Expression,
		"touch.store.target": store.Left, "touch.store.array": target.Expression, "touch.store.index": target.ArgumentExpression,
		"touch.store.operator": store.OperatorToken, "touch.store.value": store.Right,
		"touch.return": touchBody[1], "touch.return.value": touchBody[1].AsReturnStatement().Expression,
		"touch.return.array": read.Expression, "touch.return.index": read.ArgumentExpression,
		"entry.function": entryNode, "entry.name": entry.Name(), "entry.returnType": entry.Type,
		"entry.local0.statement": entryBody[0], "entry.local0.declaration": actualNode, "entry.local0.name": actualNode.Name(),
		"entry.local0.type": actual.Type, "entry.local0.element": actual.Type.AsArrayTypeNode().ElementType,
		"entry.local0.literal":  actual.Initializer,
		"entry.local0.literal0": actual.Initializer.AsArrayLiteralExpression().Elements.Nodes[0],
		"entry.local0.literal1": actual.Initializer.AsArrayLiteralExpression().Elements.Nodes[1],
		"entry.alias.statement": entryBody[1], "entry.alias.declaration": aliasNode, "entry.alias.name": aliasNode.Name(), "entry.alias.initializer": alias.Initializer,
		"entry.return": entryBody[2], "entry.call": callNode, "entry.call.callee": call.Expression,
		"entry.call.arg0": call.Arguments.Nodes[0], "entry.call.arg1": call.Arguments.Nodes[1], "entry.call.arg2": call.Arguments.Nodes[2],
	}
	for _, fn := range []struct {
		name string
		node *ast.Node
	}{{"touch", touchNode}, {"entry", entryNode}} {
		for i, param := range fn.node.AsFunctionDeclaration().Parameters.Nodes {
			suffix := []string{"0", "1", "2"}[i]
			prefix := fn.name + ".parameter" + suffix
			anchors[prefix], anchors[prefix+".name"], anchors[prefix+".type"] = param, param.Name(), param.AsParameterDeclaration().Type
		}
	}
	anchors["touch.parameter0.element"] = touch.Parameters.Nodes[0].AsParameterDeclaration().Type.AsArrayTypeNode().ElementType
	frozen := map[string]arrayFirstRange25{
		"entry.alias.declaration":  {220, 234},
		"entry.alias.initializer":  {228, 234},
		"entry.alias.name":         {220, 225},
		"entry.alias.statement":    {214, 235},
		"entry.call":               {245, 267},
		"entry.call.arg0":          {251, 256},
		"entry.call.arg1":          {258, 259},
		"entry.call.arg2":          {261, 266},
		"entry.call.callee":        {245, 250},
		"entry.function":           {127, 270},
		"entry.local0.declaration": {184, 210},
		"entry.local0.element":     {192, 195},
		"entry.local0.literal":     {200, 210},
		"entry.local0.literal0":    {201, 204},
		"entry.local0.literal1":    {206, 209},
		"entry.local0.name":        {184, 190},
		"entry.local0.statement":   {178, 211},
		"entry.local0.type":        {192, 197},
		"entry.name":               {143, 148},
		"entry.parameter0":         {149, 155},
		"entry.parameter0.name":    {149, 150},
		"entry.parameter0.type":    {152, 155},
		"entry.parameter1":         {157, 167},
		"entry.parameter1.name":    {157, 162},
		"entry.parameter1.type":    {164, 167},
		"entry.return":             {238, 268},
		"entry.returnType":         {170, 173},
		"touch.function":           {35, 126},
		"touch.name":               {51, 56},
		"touch.parameter0":         {57, 65},
		"touch.parameter0.element": {60, 63},
		"touch.parameter0.name":    {57, 58},
		"touch.parameter0.type":    {60, 65},
		"touch.parameter1":         {67, 73},
		"touch.parameter1.name":    {67, 68},
		"touch.parameter1.type":    {70, 73},
		"touch.parameter2":         {75, 85},
		"touch.parameter2.name":    {75, 80},
		"touch.parameter2.type":    {82, 85},
		"touch.return":             {112, 124},
		"touch.return.array":       {119, 120},
		"touch.return.index":       {121, 122},
		"touch.return.value":       {119, 123},
		"touch.returnType":         {88, 91},
		"touch.store":              {96, 109},
		"touch.store.array":        {96, 97},
		"touch.store.expression":   {96, 108},
		"touch.store.index":        {98, 99},
		"touch.store.operator":     {101, 102},
		"touch.store.target":       {96, 100},
		"touch.store.value":        {103, 108},
	}
	if len(anchors) != len(frozen) {
		t.Fatal("complete original UTF16 role collection")
	}
	for role, node := range anchors {
		want, ok := frozen[role]
		if !ok {
			t.Fatalf("missing frozen original UTF16 role %s", role)
		}
		arrayFirstAnchor25(t, file, node, want)
	}
	if frozen["touch.function"].Start != 35 || len(file.Text()) != 273 || file.GetPositionMap().UTF8ToUTF16(len(file.Text())) != 271 {
		t.Fatal("non-BMP/CRLF full UTF16 source proof")
	}
	t.Logf("TSFG_ARRAY_OWNER_23E_OFFICIAL_PASS diagnostics=4x0 functions=2 parameters=5 allocations=1 aliases=1 calls=1 full_utf16_anchors=%d closure=%v", len(frozen), consumed)
}

func TestArrayParameterOwner23eFirstPublicV4Success(t *testing.T) {
	input, err := os.ReadFile("testdata/array-facts-v4/first-owner-request.json")
	if err != nil || len(input) != 1345 || arrayFirstHash25(input) != "391f904346c61e8f5689283823b9d98296f3e057b9b90b737ec8624679d49118" {
		t.Fatalf("frozen explicit migrated v4 FIRST request bytes: %v", err)
	}
	var envelope struct {
		Family  string  `json:"family"`
		Version int     `json:"version"`
		Input   request `json:"input"`
	}
	if json.Unmarshal(input, &envelope) != nil || envelope.Family != "tsfg.array-facts-request-candidate" || envelope.Version != 4 || !bytes.Equal(input, append(arrayFirstCanonical25(t, envelope), '\n')) {
		t.Fatal("explicit migrated v4 JCS+LF envelope")
	}
	r := envelope.Input
	if len(r.Files) != 2 || len(r.Declarations.Paths) != 1 || r.Declarations.Version != "minimal-arrays-v3" || !r.Config.Strict || !r.Config.NoEmit || r.Config.AllowJS {
		t.Fatal("complete declarations3/facts4 input profile")
	}
	for _, file := range r.Files {
		if arrayFirstHash25([]byte(file.Content)) != file.SHA256 {
			t.Fatal("original source bytes and SHA256")
		}
	}
	if r.Files[0].SHA256 != "9312296861b0ea9fc2e569020a5c9b944c73da022f9b87d26743247d4baeb709" || r.Files[1].SHA256 != "282b335a6e0e0493fa74a3c1dd56f234f8c679079acdaf0600cefb8306c26415" {
		t.Fatal("frozen legal source and pinned DTS identity")
	}
	arrayOwnerFirstOfficial23e(t, r)
	before := bytes.Clone(input)
	t.Logf("TSFG_ARRAY_OWNER_23E_FIRST_REQUEST_BASE64=%s", base64.StdEncoding.EncodeToString(input))
	var output bytes.Buffer
	err = runFrontendCommand(t.Context(), []string{"--array-facts-io-v4"}, bytes.NewReader(input), &output)
	if !bytes.Equal(before, input) {
		t.Fatal("caller-owned original request mutated")
	}
	if err != nil {
		if output.Len() != 0 {
			t.Fatal("failed public Source command published a partial packet")
		}
		t.Fatalf("ordinary legal Array parameter/direct-call source must succeed through public v4 command: %v", err)
	}
	expected := arrayOwnerFrozenBundles23e(t)[0]
	if expected.Name != "first-touch-alias" || !bytes.Equal(output.Bytes(), []byte(expected.PackageText)) {
		t.Fatal("complete independently frozen v4 FIRST packet differs")
	}
	t.Logf("TSFG_ARRAY_OWNER_23E_FIRST_ACTUAL_BASE64=%s", base64.StdEncoding.EncodeToString(output.Bytes()))
}
