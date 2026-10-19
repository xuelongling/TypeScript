// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/scanner"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

//go:embed testdata/module-callable-facts-v1/scenarios.json
var moduleCallableScenarios23d []byte

//go:embed testdata/module-callable-facts-v1/schema.json
var moduleCallableSchema23d []byte

type mcObject23d = map[string]any
type mcExpressionRecipe23d struct {
	Kind, Name, Target       string
	Boolean                  *bool
	Value, Callee, Condition *mcExpressionRecipe23d
	Args                     []*mcExpressionRecipe23d
	Then, Else, Body         []*mcExpressionRecipe23d
}
type mcFunctionRecipe23d struct {
	Name       string
	Signature  mcObject23d
	Parameters []string
	Body       []*mcExpressionRecipe23d
}
type mcFileRecipe23d struct {
	Path    string
	Globals []struct {
		Name, Type string
		Writable   bool
		Value      *mcExpressionRecipe23d
	}
	Functions []mcFunctionRecipe23d
	Imports   []struct {
		Kind, Specifier, SourcePath string
		Star                        bool
		Bindings                    []struct{ Name, Target string }
	}
	Exports []struct{ Name, Target string }
}
type mcCase23d struct {
	Name, Group string
	Request     json.RawMessage
	Recipe      []mcFileRecipe23d
}
type mcInput23d struct {
	SourceRoot, RootFile string
	Files                []struct{ Path, Content, SHA256 string }
}
type mcFunction23d struct {
	file   *ast.SourceFile
	node   *ast.Node
	recipe mcFunctionRecipe23d
	row    mcObject23d
}
type mcIndependent23d struct {
	t            *testing.T
	check        *checker.Checker
	i32          *checker.Type
	program      *compiler.Program
	files        map[string]*ast.SourceFile
	rows         map[*ast.Symbol]mcObject23d
	declarations map[*ast.Symbol]*ast.Node
	keys         map[*ast.Symbol]string
	symbols      []any
	functions    []*mcFunction23d
	annotations  []any
}

func mcCanonical23d(t *testing.T, v any) []byte {
	t.Helper()
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(v); err != nil {
		t.Fatal(err)
	}
	return bytes.TrimSuffix(out.Bytes(), []byte("\n"))
}
func mcHash23d(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func mcDomain23d(t *testing.T, domain string, value any) string {
	return mcHash23d(append([]byte(domain+"\x00"), mcCanonical23d(t, value)...))
}
func (o *mcIndependent23d) anchor(file *ast.SourceFile, node *ast.Node, token bool) mcObject23d {
	if node == nil {
		o.t.Fatal("independent original AST node is missing")
	}
	start := node.Pos()
	if token {
		start = scanner.SkipTrivia(file.Text(), start)
	}
	positions := file.GetPositionMap()
	return mcObject23d{"start": positions.UTF8ToUTF16(start), "end": positions.UTF8ToUTF16(node.End())}
}
func (o *mcIndependent23d) spelling(file *ast.SourceFile, node *ast.Node) string {
	return file.Text()[scanner.SkipTrivia(file.Text(), node.Pos()):node.End()]
}
func (o *mcIndependent23d) original(symbol *ast.Symbol) *ast.Symbol {
	seen := map[*ast.Symbol]bool{}
	for symbol != nil && symbol.Flags&ast.SymbolFlagsAlias != 0 {
		if seen[symbol] {
			o.t.Fatal("independent official alias cycle")
		}
		seen[symbol] = true
		symbol = o.check.GetImmediateAliasedSymbol(symbol)
	}
	if symbol == nil {
		o.t.Fatal("independent official symbol missing")
	}
	return o.check.GetExportSymbolOfSymbol(symbol)
}
func (o *mcIndependent23d) nominalScalar(typ *checker.Type) string {
	if typ == nil {
		o.t.Fatal("independent official scalar type missing")
	}
	if typ.IsBooleanLike() {
		return "bool"
	}
	if o.i32 != nil && typ == o.i32 {
		return "i32"
	}
	return ""
}
func (o *mcIndependent23d) scalarOrSignature(typ *checker.Type) (string, mcObject23d) {
	if typ == nil {
		o.t.Fatal("independent official type missing")
	}
	if name := o.nominalScalar(typ); name != "" {
		return name, nil
	}
	return "function", o.signature(typ, 0)
}
func (o *mcIndependent23d) signature(typ *checker.Type, depth int) mcObject23d {
	if depth > 16 {
		o.t.Fatal("independent fixture signature depth")
	}
	signatures := o.check.GetSignaturesOfType(typ, checker.SignatureKindCall)
	if len(signatures) != 1 {
		o.t.Fatalf("independent exact callable signature count %d", len(signatures))
	}
	return o.signatureValue(signatures[0], depth)
}
func (o *mcIndependent23d) signatureValue(signature *checker.Signature, depth int) mcObject23d {
	if signature == nil || signature.Declaration() == nil || len(signature.TypeParameters()) != 0 || signature.ThisParameter() != nil || signature.HasRestParameter() || signature.MinArgumentCount() != len(signature.Parameters()) {
		o.t.Fatal("independent official signature shape differs")
	}
	parameters := []any{}
	nested := []any{}
	anyNested := false
	for _, parameter := range signature.Parameters() {
		typ := o.check.GetTypeOfSymbol(parameter)
		if name := o.nominalScalar(typ); name != "" {
			parameters = append(parameters, name)
			nested = append(nested, nil)
		} else {
			parameters = append(parameters, "function")
			nested = append(nested, o.signature(typ, depth+1))
			anyNested = true
		}
	}
	result := mcObject23d{"parameters": parameters}
	if anyNested {
		result["parameterSignatures"] = nested
	}
	returns := o.check.GetReturnTypeOfSignature(signature)
	if name := o.nominalScalar(returns); name != "" {
		result["returnType"] = name
	} else {
		result["returnType"] = "function"
		result["returnSignature"] = o.signature(returns, depth+1)
	}
	return result
}
func (o *mcIndependent23d) add(file *ast.SourceFile, node *ast.Node, kind, typ string, writable bool) mcObject23d {
	symbol := o.original(o.check.GetSymbolAtLocation(node.Name()))
	if _, exists := o.rows[symbol]; exists {
		o.t.Fatal("independent fixture repeats original declaration")
	}
	row := mcObject23d{"sourcePath": file.FileName(), "id": fmt.Sprintf("s%d", len(o.symbols)+1), "name": node.Name().Text(), "kind": kind, "type": typ, "writable": writable, "range": o.anchor(file, node.Name(), false)}
	o.rows[symbol] = row
	o.declarations[symbol] = node
	o.keys[symbol] = file.FileName() + "#" + node.Name().Text()
	o.symbols = append(o.symbols, row)
	return row
}
func (o *mcIndependent23d) lookup(node *ast.Node, target string) mcObject23d {
	symbol := o.original(o.check.GetSymbolAtLocation(node))
	row := o.rows[symbol]
	if row == nil || target != "" && o.keys[symbol] != target {
		o.t.Fatalf("independent original declaration differs: got=%s expected=%s", o.keys[symbol], target)
	}
	return row
}

func TestModuleCallable23dIndependentCorpusOracle(t *testing.T) {
	var corpus struct{ Cases []mcCase23d }
	if err := json.Unmarshal(moduleCallableScenarios23d, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 8 {
		t.Fatal("complete frozen six-group/eight-input corpus required")
	}
	if mcHash23d(moduleCallableSchema23d) != "sha256:8d66ba7464ea05c317fc413cf61dbc906c34c1ef38f4c7fcc916038e7dbe388e" {
		t.Fatal("frozen candidate descriptor differs")
	}
	for _, tc := range corpus.Cases {
		t.Run(tc.Name, func(t *testing.T) { mcGenerateIndependent23d(t, tc) })
	}
}
func mcGenerateIndependent23d(t *testing.T, tc mcCase23d) {
	var envelope struct{ Input mcInput23d }
	var requestObject mcObject23d
	if err := json.Unmarshal(tc.Request, &envelope); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(tc.Request, &requestObject); err != nil {
		t.Fatal(err)
	}
	contents := map[string]string{}
	names := []string{}
	for _, file := range envelope.Input.Files {
		if mcHash23d([]byte(file.Content)) != "sha256:"+file.SHA256 {
			t.Fatal("frozen independently owned file digest differs")
		}
		contents[file.Path] = file.Content
		names = append(names, file.Path)
	}
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{FileNames: names, CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext}}},
		Host:   compiler.NewCompilerHost(envelope.Input.SourceRoot, vfstest.FromMap(contents, true), envelope.Input.SourceRoot, nil, nil), SingleThreaded: core.TSTrue,
	})
	if ds := program.GetSyntacticDiagnostics(t.Context(), nil); len(ds) != 0 {
		t.Fatalf("official syntax: %+v", ds)
	}
	if ds := program.GetSemanticDiagnostics(t.Context(), nil); len(ds) != 0 {
		t.Fatalf("official semantic: %+v", ds)
	}
	if ds := program.GetProgramDiagnostics(); len(ds) != 0 {
		t.Fatalf("official program: %+v", ds)
	}
	if ds := program.GetGlobalDiagnostics(t.Context()); len(ds) != 0 {
		t.Fatalf("official global: %+v", ds)
	}
	consumed := map[string]bool{}
	for _, file := range program.GetSourceFiles() {
		if contents[file.FileName()] != file.Text() || consumed[file.FileName()] {
			t.Fatal("official consumed closure differs")
		}
		consumed[file.FileName()] = true
	}
	if len(consumed) != len(contents) {
		t.Fatal("official consumed closure omitted frozen input")
	}
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), program.GetSourceFile(envelope.Input.RootFile))
	defer release()
	o := &mcIndependent23d{t: t, check: check, program: program, files: map[string]*ast.SourceFile{}, rows: map[*ast.Symbol]mcObject23d{}, declarations: map[*ast.Symbol]*ast.Node{}, keys: map[*ast.Symbol]string{}, symbols: []any{}, functions: []*mcFunction23d{}, annotations: []any{}}
	for _, source := range program.GetSourceFiles() {
		if source.FileName() != "/src/minimal-integers-v3.d.ts" {
			continue
		}
		for _, declaration := range source.Statements.Nodes {
			if declaration.Kind == ast.KindTypeAliasDeclaration && declaration.Name().Text() == "i32" {
				if o.i32 != nil {
					t.Fatal("independent original nominal i32 alias repeated")
				}
				o.i32 = check.GetDeclaredTypeOfSymbol(check.GetSymbolAtLocation(declaration.Name()))
			}
		}
	}
	if o.i32 == nil {
		t.Fatal("independent original nominal i32 alias missing")
	}
	o.headers(tc.Recipe)
	for _, function := range o.functions {
		o.functionBody(function)
	}
	modules, exports := o.modules(tc.Recipe)
	functions := []any{}
	for _, function := range o.functions {
		functions = append(functions, function.row)
	}
	sort.Strings(names)
	facts := mcObject23d{"status": "ok", "inputs": names, "modules": modules, "functions": functions, "symbols": o.symbols}
	manifest := []any{}
	for _, path := range names {
		content := contents[path]
		manifest = append(manifest, mcObject23d{"path": path, "digest": mcHash23d([]byte(content)), "byteLength": len([]byte(content)), "length": len(utf16.Encode([]rune(content))), "positionUnit": "utf16"})
	}
	payload := mcObject23d{"inputIdentity": mcDomain23d(t, "tsfg.module-callable-facts.input.v2", requestObject["input"]), "files": manifest, "facts": facts, "annotations": o.annotations, "exports": exports}
	packet := mcObject23d{"family": "tsfg.module-callable-facts-package-candidate", "version": 2, "schemaHash": mcHash23d(moduleCallableSchema23d), "requestId": mcDomain23d(t, "tsfg.module-callable-facts.request.v2", requestObject), "payload": payload}
	packet["packageId"] = mcDomain23d(t, "tsfg.module-callable-facts.package.v2", packet)
	bytes := append(mcCanonical23d(t, packet), '\n')
	t.Logf("TSFG_MODULE_CALLABLE_23D_INDEPENDENT_PACKET name=%s bytes=%d digest=%s base64=%s", tc.Name, len(bytes), mcHash23d(bytes), base64.StdEncoding.EncodeToString(bytes))
	t.Logf("TSFG_MODULE_CALLABLE_23D_INDEPENDENT_OFFICIAL_PASS name=%s diagnostics=0 functions=%d symbols=%d modules=%d", tc.Name, len(functions), len(o.symbols), len(modules))
}
