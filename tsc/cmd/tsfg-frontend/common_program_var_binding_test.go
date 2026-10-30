// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	tsjson "github.com/microsoft/typescript-go/internal/json"
	"github.com/microsoft/typescript-go/internal/scanner"
)

// The fixed public seam is the complete Program-facts command. A separate
// original Program supplies Symbols, scopes, flow types and source anchors first.
// Hand oracle, not execution credit: before is undefined; block assignment sets
// x to 1; write sets the same x to 2; read observes 2; varBinding returns true.
func TestCommonProgramVarHoistedSharedBinding19am(t *testing.T) {
	if root := os.Getenv("TSFG_READSPAN_CAPTURE_ROOT"); root != "" {
		t.Setenv("TSFG_READSPAN_CAPTURE_ROOT", root+"/var-binding")
	}
	source, err := os.ReadFile("testdata/common-program-v2/var-binding/hoisted-shared.ts")
	if err != nil {
		t.Fatal(err)
	}
	if len(source) != 354 || arrayFactsSHA(source) != "136103c91499efec670d725f26069b0724c8bdf3a74138d01a307da52b368e94" {
		t.Fatal("frozen complete causal var Source changed")
	}
	readSpanCaptureSave26i(t, "hoisted-shared-source.ts", source)
	text := string(source)
	program := commonMethodProgram37q(t, text)
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	t.Cleanup(release)
	rangeOf := func(node *ast.Node) span {
		start := scanner.SkipTrivia(text, node.Pos())
		return span{len(utf16.Encode([]rune(text[:start]))), len(utf16.Encode([]rune(text[:node.End()])))}
	}
	functions := map[string]*ast.Node{}
	var declaration *ast.Node
	var identifiers []*ast.Node
	var visit func(*ast.Node)
	visit = func(node *ast.Node) {
		if node.Kind == ast.KindFunctionDeclaration {
			functions[node.Name().Text()] = node
		}
		if node.Kind == ast.KindVariableDeclaration && node.Name().Text() == "x" {
			if declaration != nil {
				t.Fatal("the first causal case must retain one original var declaration")
			}
			declaration = node
		}
		if node.Kind == ast.KindIdentifier && node.Text() == "x" {
			identifiers = append(identifiers, node)
		}
		node.ForEachChild(func(child *ast.Node) bool { visit(child); return false })
	}
	visit(file.AsNode())
	entry, reader, writer := functions["varBinding"], functions["read"], functions["write"]
	if len(functions) != 3 || entry == nil || reader == nil || writer == nil || declaration == nil || len(identifiers) != 4 {
		t.Fatal("complete original var/function/occurrence inventory differs")
	}
	statements := entry.Body().AsBlock().Statements.Nodes
	if len(statements) != 6 || statements[0].Kind != ast.KindVariableStatement || statements[1].Kind != ast.KindBlock || statements[2] != reader || statements[3] != writer || reader.Parent != entry.Body() || writer.Parent != entry.Body() {
		t.Fatal("original hoisted use, block initializer or sibling function order differs")
	}
	before := statements[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0].Initializer()
	read := reader.Body().AsBlock().Statements.Nodes[0].AsReturnStatement().Expression
	assignment := writer.Body().AsBlock().Statements.Nodes[0].AsExpressionStatement().Expression
	if before.Kind != ast.KindIdentifier || read.Kind != ast.KindIdentifier || assignment.Kind != ast.KindBinaryExpression || assignment.AsBinaryExpression().OperatorToken.Kind != ast.KindEqualsToken {
		t.Fatal("original before/read/write expressions differ")
	}
	write := assignment.AsBinaryExpression().Left
	if declaration.Parent.Kind != ast.KindVariableDeclarationList || declaration.Parent.Flags&(ast.NodeFlagsLet|ast.NodeFlagsConst) != 0 || declaration.Parent.Parent.Parent != statements[1] || declaration.Initializer() == nil {
		t.Fatal("original var flag, block declaration or initializer differs")
	}
	symbol := check.GetSymbolAtLocation(declaration.Name())
	if symbol == nil || symbol.Flags&ast.SymbolFlagsFunctionScopedVariable == 0 || symbol.Flags&ast.SymbolFlagsBlockScopedVariable != 0 || len(symbol.Declarations) != 1 || symbol.Declarations[0] != declaration {
		t.Fatal("official var must retain its one function-scoped Symbol")
	}
	for _, node := range identifiers {
		if check.GetSymbolAtLocation(node) != symbol {
			t.Fatal("all four original x occurrences must resolve to the same Symbol")
		}
	}
	declaredType := check.GetTypeFromTypeNode(declaration.Type())
	beforeFlow, readFlow := check.GetTypeAtLocation(before), check.GetTypeAtLocation(read)
	t.Logf("original official types: declared=%s flags=%d; before=%s flags=%d; read=%s flags=%d", check.TypeToString(declaredType), declaredType.Flags(), check.TypeToString(beforeFlow), beforeFlow.Flags(), check.TypeToString(readFlow), readFlow.Flags())
	if !declaredType.IsUnion() {
		t.Fatal("official declared var type must retain the original i32/undefined union; actual types/flags logged above")
	}
	undefined, number := 0, 0
	for _, member := range declaredType.Types() {
		if member.Flags()&checker.TypeFlagsUndefined != 0 {
			undefined++
		}
		if member.Flags()&checker.TypeFlagsNumberLike != 0 {
			number++
		}
	}
	if len(declaredType.Types()) != 2 || undefined != 1 || number != 1 {
		t.Fatal("official declared var type must be exactly the original i32/undefined union")
	}
	identity := func(domain string, raw []byte) string {
		canonical := tsjson.Value(raw)
		if err := canonical.Canonicalize(); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(append([]byte(domain+"\x00"), canonical...))
		return "sha256:" + hex.EncodeToString(sum[:])
	}
	functionIDs := map[string]string{}
	for name, node := range functions {
		raw, err := json.Marshal(struct {
			SourcePath       string `json:"sourcePath"`
			DeclarationRange span   `json:"declarationRange"`
		}{ast.GetSourceFileOfNode(node).FileName(), rangeOf(node)})
		if err != nil {
			t.Fatal(err)
		}
		functionIDs[name] = identity("tsfg.array-facts.function.v8", raw)
	}
	bindingBytes, err := json.Marshal(struct {
		SourcePath string `json:"sourcePath"`
		NameRange  span   `json:"nameRange"`
	}{ast.GetSourceFileOfNode(declaration).FileName(), rangeOf(declaration.Name())})
	if err != nil {
		t.Fatal(err)
	}
	bindingID := identity("tsfg.array-facts.binding.v1", bindingBytes)
	oracle := struct {
		SourceSHA, DeclarationKind, BindingName, Owner, DeclaredType, BeforeFlow, ReadFlow string
		DeclaredFlags, BeforeFlags, ReadFlags                                              checker.TypeFlags
		Declaration, Name, Block, Before, Read, Write, Initializer                         span
		FunctionIDs                                                                        map[string]string
		BindingID                                                                          string
		Result                                                                             bool
	}{arrayFactsSHA(source), "var", "x", "varBinding", check.TypeToString(declaredType), check.TypeToString(beforeFlow), check.TypeToString(readFlow), declaredType.Flags(), beforeFlow.Flags(), readFlow.Flags(),
		rangeOf(declaration), rangeOf(declaration.Name()), rangeOf(statements[1]), rangeOf(before), rangeOf(read), rangeOf(write), rangeOf(declaration.Initializer()), functionIDs, bindingID, true}
	oracleBytes, err := json.Marshal(oracle)
	if err != nil {
		t.Fatal(err)
	}
	readSpanCaptureSave26i(t, "hoisted-shared-original-oracle.json", append(oracleBytes, '\n'))
	t.Log("pre-producer independent official diagnostics zero; var Symbol/function scope and original declared/before/read flow oracle established")

	var request commonProgramRequest
	if err := json.Unmarshal(genericArrayRequest25f(t, text), &request); err != nil {
		t.Fatal(err)
	}
	request.Family, request.Version, request.SchemaHash = "tsfg.program-facts-request-candidate", 2, "sha256:"+arrayFactsSHA(commonProgramSchema)
	request.Input.HostImports = []commonProgramHostImport{}
	request.Input.Target = &commonProgramTarget{Target: "windows-x86_64-msvc", Triple: "x86_64-pc-windows-msvc", PointerBits: "64", LLVMDataLayout: "e-m:w-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128"}
	root := closureRoot{Kind: "application", SourcePath: "/src/main.ts", Name: "varBinding"}
	request.Input.CompilationClosure = &closureConfiguration{Roots: []closureRoot{root, {Kind: "module-initialization", SourcePath: "/src/main.ts", Name: "$moduleInit"}}, RequiredRoots: []closureRoot{root}}
	requestBytes, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	canonical := tsjson.Value(requestBytes)
	if err := canonical.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	requestBytes = append(canonical, '\n')
	readSpanCaptureSave26i(t, "hoisted-shared-request.json", requestBytes)
	var output bytes.Buffer
	if err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(requestBytes), &output); err != nil {
		if output.Len() != 0 {
			t.Fatal("failed producer published a partial var package")
		}
		if failure, ok := err.(*arrayFactsFailure); ok {
			failureBytes, encodeErr := json.Marshal(failure)
			if encodeErr != nil {
				t.Fatal(encodeErr)
			}
			readSpanCaptureSave26i(t, "hoisted-shared-failure.json", append(failureBytes, '\n'))
			t.Log(string(failureBytes))
		}
		t.Fatal(err)
	}
	readSpanCaptureSave26i(t, "hoisted-shared-package.json", output.Bytes())
	var packet commonProgramPackage
	if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	facts := map[string]arrayGenericFunction{}
	for _, fn := range packet.Payload.Functions {
		facts[fn.Name] = fn
	}
	owned, readFacts, writeFacts := facts["varBinding"], facts["read"], facts["write"]
	if len(facts) != 3 || owned.DeclarationRange != rangeOf(entry) || len(owned.Body) != 6 || len(owned.Bindings) != 2 || readFacts.DeclarationRange != rangeOf(reader) || writeFacts.DeclarationRange != rangeOf(writer) {
		t.Fatal("complete projected original function/binding/body inventory differs")
	}
	if owned.ID != functionIDs["varBinding"] || readFacts.ID != functionIDs["read"] || writeFacts.ID != functionIDs["write"] {
		t.Fatal("function IDs differ from independent original AST/domain identities")
	}
	var binding arrayFactsBinding
	for _, candidate := range owned.Bindings {
		if candidate.Name == "x" {
			binding = candidate
		}
	}
	if binding.ID != bindingID || binding.Kind != "local" || !binding.Writable || binding.LexicalOwner == nil || *binding.LexicalOwner == nil || **binding.LexicalOwner != functionIDs["varBinding"] || binding.DeclarationRange != rangeOf(declaration) || binding.NameRange != rangeOf(declaration.Name()) {
		t.Fatal("original writable var Binding/owner/source identity differs")
	}
	types := map[string]arrayGenericType{}
	for _, typ := range packet.Payload.Types {
		types[typ.ID] = typ
	}
	match := captureTypeMatcherOfficial19ai(t, check, types, rangeOf)
	union := types[binding.DeclaredType]
	if union.Kind != "union" || len(union.Members) != 2 || union.Members[0] == union.Members[1] || !match(binding.DeclaredType, declaredType) {
		t.Fatal("projected var type differs from the exact official i32/undefined constituents")
	}
	beforeFacts := owned.Body[0].Initializer
	if beforeFacts == nil || beforeFacts.Kind != "bindingReference" || beforeFacts.Binding != bindingID || beforeFacts.Type != binding.DeclaredType || !match(beforeFacts.FlowType, beforeFlow) || beforeFacts.Range != rangeOf(before) {
		t.Errorf("hoisted x must remain the original bindingReference and official flow, not a literal: %+v", beforeFacts)
	}
	block := owned.Body[1]
	if block.Kind != "block" || block.Body == nil || len(*block.Body) != 1 {
		t.Fatal("original complete var block differs")
	}
	varFacts := (*block.Body)[0]
	if varFacts.Kind != "varDeclaration" || varFacts.Binding != bindingID || varFacts.Range != rangeOf(declaration) || varFacts.NameRange == nil || *varFacts.NameRange != rangeOf(declaration.Name()) || varFacts.AnnotationRange == nil || *varFacts.AnnotationRange != rangeOf(declaration.Type()) || varFacts.Initializer == nil || varFacts.Initializer.Range != rangeOf(declaration.Initializer()) || varFacts.Initializer.Kind != "numericLiteral" || varFacts.Initializer.Decimal != "1" {
		t.Errorf("original var declaration must distinguish function-scope binding from let and preserve its initializer position: %+v", varFacts)
	}
	if len(readFacts.Body) != 1 || readFacts.Body[0].Value == nil {
		t.Fatal("original reader return expression is absent")
	}
	readValue := readFacts.Body[0].Value
	if readValue.Kind != "bindingReference" || readValue.Binding != bindingID || readValue.Range != rangeOf(read) || !match(readValue.Type, declaredType) || !match(readValue.FlowType, readFlow) {
		t.Error("reader binding/type/flow differs from the independent original checker")
	}
	for _, sample := range []struct {
		fn          arrayGenericFunction
		use         *ast.Node
		read, write bool
	}{{readFacts, read, true, false}, {writeFacts, write, false, true}} {
		if sample.fn.LexicalParent == nil || *sample.fn.LexicalParent == nil || **sample.fn.LexicalParent != functionIDs["varBinding"] || sample.fn.DeclarationScope == nil || *sample.fn.DeclarationScope != rangeOf(entry.Body()) || sample.fn.Captures == nil || len(*sample.fn.Captures) != 1 {
			t.Fatal("sibling closure must retain its original function owner, body scope and one shared capture")
		}
		want := commonProgramCapture{Binding: bindingID, Read: sample.read, Write: sample.write, Occurrences: []commonProgramCaptureOccurrence{{Range: rangeOf(sample.use), Read: sample.read, Write: sample.write}}}
		if !reflect.DeepEqual((*sample.fn.Captures)[0], want) {
			t.Errorf("shared var capture differs from original Symbol/occurrence: got %+v want %+v", (*sample.fn.Captures)[0], want)
		}
	}
}
