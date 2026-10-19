// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

// These two directed whole packets were independently generated from fresh
// official Programs and frozen before their public producer comparisons.
func TestModuleCallableFactsFirstResolvingStarExports(t *testing.T) {
	mcPublicDirectedWhole23d(t, mcStarScenario23d, "first-resolving-star")
}

func TestModuleCallableFactsI32CallbackWhole(t *testing.T) {
	mcPublicDirectedWhole23d(t, mcI32Scenario23d, "i32-callback")
}

func mcPublicDirectedWhole23d(t *testing.T, fixtureBytes []byte, name string) {
	t.Helper()
	var fixture struct{ Case mcCase23d }
	if err := json.Unmarshal(fixtureBytes, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Case.Name != name {
		t.Fatal("independently frozen directed scenario name differs")
	}
	var requestObject mcObject23d
	if err := json.Unmarshal(fixture.Case.Request, &requestObject); err != nil {
		t.Fatal(err)
	}
	input := append(mcCanonical23d(t, requestObject), '\n')
	before := bytes.Clone(input)
	expected, err := mcPublicExpected23d.ReadFile("testdata/module-callable-facts-v1/expected/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	output := &mcPublicWriter23d{}
	if err := runFrontendCommand(t.Context(), []string{"--module-callable-facts-io-v2"}, bytes.NewReader(input), output); err != nil {
		t.Fatalf("ordinary independent directed source must publish complete facts: %v", err)
	}
	if !bytes.Equal(input, before) {
		t.Fatal("public command mutated independently frozen directed request")
	}
	t.Logf("TSFG_MODULE_CALLABLE_23D_PUBLIC_ACTUAL name=%s bytes=%d digest=%s base64=%s", name, output.Len(), mcHash23d(output.Bytes()), base64.StdEncoding.EncodeToString(output.Bytes()))
	if output.writes != 1 || !bytes.Equal(output.Bytes(), expected) {
		t.Fatalf("complete directed independent packet differs for %s: writes=%d got=%s want=%s", name, output.writes, mcHash23d(output.Bytes()), mcHash23d(expected))
	}
	t.Logf("TSFG_MODULE_CALLABLE_23D_PUBLIC_WHOLE_PASS name=%s bytes=%d digest=%s", name, output.Len(), mcHash23d(expected))
}

// R01 defaults an unannotated exact integer literal global to i32. The official
// checker deliberately still reports number; this test keeps those two facts
// separate and does not call the producer's scalar/module-global adapters.
func TestModuleCallableFactsDefaultIntegerGlobal(t *testing.T) {
	requestObject := mcMutableRequest23d(t, "first-rr")
	mcReplaceSource23d(t, requestObject, "/src/app.ts", func(source string) string { return source + "export let counter = 1;\n" })
	input := mcRequestBytes23d(t, requestObject)
	anchors := mcOfficialDefaultIntegerGlobal23d(t, input)
	t.Log("TSFG_MODULE_CALLABLE_23D_DEFAULT_INTEGER_OFFICIAL_PASS diagnostics=0 originalNumericLiteral=1 annotation=false officialNumberLike=true")
	before := bytes.Clone(input)
	output := &mcPublicWriter23d{}
	if err := runFrontendCommand(t.Context(), []string{"--module-callable-facts-io-v2"}, bytes.NewReader(input), output); err != nil {
		t.Fatalf("ordinary R01 default integer global must publish facts as i32: %v", err)
	}
	if output.writes != 1 || !bytes.Equal(input, before) {
		t.Fatal("default integer global requires one complete write and unchanged input")
	}
	t.Logf("TSFG_MODULE_CALLABLE_23D_PUBLIC_ACTUAL name=default-integer-global bytes=%d digest=%s base64=%s", output.Len(), mcHash23d(output.Bytes()), base64.StdEncoding.EncodeToString(output.Bytes()))
	var packet mcObject23d
	if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), append(mcCanonical23d(t, packet), '\n')) || len(packet) != 6 || packet["family"] != "tsfg.module-callable-facts-package-candidate" || packet["version"] != float64(2) || packet["schemaHash"] != mcHash23d(moduleCallableSchema23d) || packet["requestId"] != mcDomain23d(t, "tsfg.module-callable-facts.request.v2", requestObject) {
		t.Fatal("default integer complete canonical envelope/request identity differs")
	}
	packageID := packet["packageId"]
	delete(packet, "packageId")
	if packageID != mcDomain23d(t, "tsfg.module-callable-facts.package.v2", packet) {
		t.Fatal("default integer complete package digest differs")
	}
	payload, ok := packet["payload"].(map[string]any)
	if !ok || len(payload) != 5 || payload["inputIdentity"] != mcDomain23d(t, "tsfg.module-callable-facts.input.v2", requestObject["input"]) {
		t.Fatal("default integer input/payload identity differs")
	}
	manifest, ok := payload["files"].([]any)
	files := mcInputObject23d(requestObject)["files"].([]any)
	if !ok || len(manifest) != len(files) {
		t.Fatal("default integer complete manifest omitted files")
	}
	for index, item := range manifest {
		row := item.(map[string]any)
		file := files[index].(map[string]any)
		content := file["content"].(string)
		if len(row) != 5 || row["path"] != file["path"] || row["digest"] != mcHash23d([]byte(content)) || row["byteLength"] != float64(len([]byte(content))) || row["length"] != float64(len(utf16.Encode([]rune(content)))) || row["positionUnit"] != "utf16" {
			t.Fatal("default integer full manifest content/UTF16 identity differs")
		}
	}
	facts, ok := payload["facts"].(map[string]any)
	if !ok || len(facts) != 5 || facts["status"] != "ok" {
		t.Fatal("default integer complete facts missing")
	}
	symbols, ok := facts["symbols"].([]any)
	if !ok {
		t.Fatal("default integer symbol registry missing")
	}
	var counter map[string]any
	for _, item := range symbols {
		row := item.(map[string]any)
		if row["name"] == "counter" {
			if counter != nil {
				t.Fatal("default integer original global repeated")
			}
			counter = row
		}
	}
	if counter == nil || len(counter) != 8 || counter["sourcePath"] != "/src/app.ts" || counter["kind"] != "global" || counter["bindingKind"] != "source" || counter["type"] != "i32" || counter["writable"] != true || !bytes.Equal(mcCanonical23d(t, counter["range"]), mcCanonical23d(t, anchors["name"])) {
		t.Fatal("unannotated exact integer global must retain original i32 declaration facts")
	}
	modules, ok := facts["modules"].([]any)
	if !ok || len(modules) != 3 {
		t.Fatal("default integer complete modules missing")
	}
	found := false
	for _, item := range modules {
		module := item.(map[string]any)
		if module["sourcePath"] != "/src/app.ts" {
			continue
		}
		initializers := module["initialization"].([]any)
		for _, item := range initializers {
			initialization := item.(map[string]any)
			symbol, ok := initialization["symbol"].(map[string]any)
			if !ok || symbol["id"] != counter["id"] {
				continue
			}
			if found {
				t.Fatal("default integer initializer repeated")
			}
			found = true
			value, ok := initialization["initializer"].(map[string]any)
			if !ok || len(value) != 4 || value["kind"] != "integer" || value["type"] != "i32" || !bytes.Equal(mcCanonical23d(t, value["literal"]), []byte("{\"decimal\":\"1\",\"token\":\"1\"}")) || !bytes.Equal(mcCanonical23d(t, value["range"]), mcCanonical23d(t, anchors["literal"])) || !bytes.Equal(mcCanonical23d(t, initialization["range"]), mcCanonical23d(t, anchors["declaration"])) || !bytes.Equal(mcCanonical23d(t, symbol), mcCanonical23d(t, counter)) {
				t.Fatal("default integer initializer must preserve original literal 1/ranges and i32 global")
			}
		}
	}
	if !found {
		t.Fatal("default integer initializer omitted")
	}
	functions, ok := facts["functions"].([]any)
	if !ok || len(functions) != 4 {
		t.Fatal("default integer complete callable functions missing")
	}
	annotations, ok := payload["annotations"].([]any)
	if !ok || len(annotations) != 4 {
		t.Fatal("default integer complete function annotations missing")
	}
	exports, ok := payload["exports"].([]any)
	if !ok || len(exports) != 3 {
		t.Fatal("default integer complete export groups missing")
	}
}

func mcOfficialDefaultIntegerGlobal23d(t *testing.T, input []byte) mcObject23d {
	t.Helper()
	var envelope struct{ Input mcInput23d }
	if err := json.Unmarshal(input, &envelope); err != nil {
		t.Fatal(err)
	}
	contents := map[string]string{}
	names := []string{}
	for _, file := range envelope.Input.Files {
		contents[file.Path] = file.Content
		names = append(names, file.Path)
	}
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{FileNames: names, CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext}}},
		Host:   compiler.NewCompilerHost(envelope.Input.SourceRoot, vfstest.FromMap(contents, true), envelope.Input.SourceRoot, nil, nil), SingleThreaded: core.TSTrue,
	})
	if ds := program.GetSyntacticDiagnostics(t.Context(), nil); len(ds) != 0 {
		t.Fatalf("independent official syntax: %+v", ds)
	}
	if ds := program.GetSemanticDiagnostics(t.Context(), nil); len(ds) != 0 {
		t.Fatalf("independent official semantic: %+v", ds)
	}
	if ds := program.GetProgramDiagnostics(); len(ds) != 0 {
		t.Fatalf("independent official program: %+v", ds)
	}
	if ds := program.GetGlobalDiagnostics(t.Context()); len(ds) != 0 {
		t.Fatalf("independent official global: %+v", ds)
	}
	consumed := map[string]bool{}
	for _, file := range program.GetSourceFiles() {
		if contents[file.FileName()] != file.Text() || consumed[file.FileName()] {
			t.Fatal("default integer official closure differs")
		}
		consumed[file.FileName()] = true
	}
	if len(consumed) != len(contents) {
		t.Fatal("default integer official closure incomplete")
	}
	file := program.GetSourceFile("/src/app.ts")
	if file == nil || len(file.Statements.Nodes) != 5 {
		t.Fatal("default integer original complete app structure differs")
	}
	statement := file.Statements.Nodes[4]
	if statement.Kind != ast.KindVariableStatement || statement.AsVariableStatement().DeclarationList.Flags&ast.NodeFlagsLet == 0 {
		t.Fatal("default integer original let declaration required")
	}
	declarations := statement.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
	if len(declarations) != 1 {
		t.Fatal("default integer original declaration count differs")
	}
	declaration := declarations[0]
	variable := declaration.AsVariableDeclaration()
	if declaration.Name().Text() != "counter" || variable.Type != nil || variable.Initializer == nil || variable.Initializer.Kind != ast.KindNumericLiteral || variable.Initializer.Text() != "1" {
		t.Fatal("default integer original unannotated NumericLiteral 1 required")
	}
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	symbol := check.GetSymbolAtLocation(declaration.Name())
	typ := check.GetTypeAtLocation(declaration.Name())
	if symbol == nil || typ == nil || typ.Flags()&checker.TypeFlagsNumberLike == 0 || check.GetTypeOfSymbol(symbol) != typ || len(symbol.Declarations) != 1 || symbol.Declarations[0] != declaration {
		t.Fatal("independent original counter must retain official number-like type/symbol")
	}
	positions := file.GetPositionMap()
	anchor := func(node *ast.Node) mcObject23d {
		return mcObject23d{"start": positions.UTF8ToUTF16(node.Pos()), "end": positions.UTF8ToUTF16(node.End())}
	}
	return mcObject23d{"name": anchor(declaration.Name()), "declaration": anchor(declaration), "literal": anchor(variable.Initializer)}
}
