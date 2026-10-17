// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

func TestFrontendCommandSharedMutableCaptureOwnedFacts(t *testing.T) {
	const source = "export function entry(seed: bool): bool { let state: bool = seed; function write(value: bool): bool { state = value; return state; } function read(): bool { return state; } write(false); return read(); }"
	r := integerRequest(source)
	if !t.Run("officialDiagnosticsAndBindingOracle", func(t *testing.T) {
		contents := map[string]string{}
		for _, file := range r.Files {
			contents[file.Path] = file.Content
		}
		program := compiler.NewProgram(compiler.ProgramOptions{
			Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
				FileNames: append([]string{r.RootFile}, r.Declarations.Paths...),
				CompilerOptions: &core.CompilerOptions{
					NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext,
				},
			}},
			Host:           compiler.NewCompilerHost(r.SourceRoot, vfstest.FromMap(contents, true), r.SourceRoot, nil, nil),
			SingleThreaded: core.TSTrue,
		})
		file := program.GetSourceFile(r.RootFile)
		if file == nil {
			t.Fatal("official program omitted the original source")
		}
		// This fresh program never receives the adapter's scalar contexts or
		// owned binding maps; official diagnostics must pass before projection.
		if diagnostics := program.GetSyntacticDiagnostics(t.Context(), nil); len(diagnostics) != 0 {
			t.Fatalf("official syntax diagnostics: %+v", diagnostics)
		}
		if diagnostics := program.GetSemanticDiagnostics(t.Context(), nil); len(diagnostics) != 0 {
			t.Fatalf("official semantic diagnostics: %+v", diagnostics)
		}
		if diagnostics := program.GetProgramDiagnostics(); len(diagnostics) != 0 {
			t.Fatalf("official program diagnostics: %+v", diagnostics)
		}
		if diagnostics := program.GetGlobalDiagnostics(t.Context()); len(diagnostics) != 0 {
			t.Fatalf("official global diagnostics: %+v", diagnostics)
		}
		check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
		defer release()
		if len(file.Statements.Nodes) != 1 || file.Statements.Nodes[0].Kind != ast.KindFunctionDeclaration {
			t.Fatal("official source must contain the original entry declaration")
		}
		entry := file.Statements.Nodes[0].AsFunctionDeclaration()
		body := entry.Body.AsBlock().Statements.Nodes
		if len(body) != 5 || body[0].Kind != ast.KindVariableStatement || body[1].Kind != ast.KindFunctionDeclaration || body[2].Kind != ast.KindFunctionDeclaration {
			t.Fatal("official entry lost its local or nested declarations")
		}
		stateDecl := body[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0]
		writer := body[1].AsFunctionDeclaration()
		reader := body[2].AsFunctionDeclaration()
		writerBody := writer.Body.AsBlock().Statements.Nodes
		readerBody := reader.Body.AsBlock().Statements.Nodes
		if len(writerBody) != 2 || len(readerBody) != 1 || writerBody[0].Kind != ast.KindExpressionStatement || writerBody[1].Kind != ast.KindReturnStatement || readerBody[0].Kind != ast.KindReturnStatement {
			t.Fatal("official nested bodies differ from the source oracle")
		}
		assignment := writerBody[0].AsExpressionStatement().Expression.AsBinaryExpression()
		state := check.GetSymbolAtLocation(stateDecl.Name())
		if state == nil {
			t.Fatal("official checker omitted the outer state binding")
		}
		// These source-selected sites independently prove one shared logical
		// binding. No producer-generated IDs or capture analysis are consulted.
		for _, use := range []*ast.Node{assignment.Left, writerBody[1].AsReturnStatement().Expression, readerBody[0].AsReturnStatement().Expression} {
			if check.GetSymbolAtLocation(use) != state {
				t.Fatal("official checker did not resolve each capture to the original state declaration")
			}
		}
		value := check.GetSymbolAtLocation(writer.Parameters.Nodes[0].Name())
		seed := check.GetSymbolAtLocation(entry.Parameters.Nodes[0].Name())
		if value == nil || seed == nil || value == state || seed == state || value == seed || check.GetSymbolAtLocation(assignment.Right) != value || check.GetSymbolAtLocation(stateDecl.AsVariableDeclaration().Initializer) != seed {
			t.Fatal("official checker confused parameters with the captured binding")
		}
		if check.GetSymbolAtLocation(body[3].AsExpressionStatement().Expression.AsCallExpression().Expression) != check.GetSymbolAtLocation(body[1].Name()) || check.GetSymbolAtLocation(body[4].AsReturnStatement().Expression.AsCallExpression().Expression) != check.GetSymbolAtLocation(body[2].Name()) {
			t.Fatal("official direct calls lost their nested declaration targets")
		}
	}) {
		t.Fatal("official diagnostics or binding oracle failed; owned projection was not attempted")
	}

	t.Run("ownedProjection", func(t *testing.T) {
		input, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		original, err := json.Marshal(map[string]any{"rootFile": r.RootFile, "files": r.Files})
		if err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		commandErr := runFrontendCommand(t.Context(), nil, bytes.NewReader(input), &output)
		// Preserve exact public-boundary bytes on RED as well as GREEN.
		t.Logf("TSFG_SHARED_CAPTURE_REQUEST_V1 %s", base64.StdEncoding.EncodeToString(input))
		t.Logf("TSFG_SHARED_CAPTURE_RESPONSE_V1 %s", base64.StdEncoding.EncodeToString(output.Bytes()))
		t.Logf("TSFG_SHARED_CAPTURE_ORIGINAL_V1 %s", base64.StdEncoding.EncodeToString(original))
		if commandErr != nil {
			t.Fatalf("public frontend command failed: %v; rawResponse=%s", commandErr, output.Bytes())
		}
		// Local wire views let the capability RED compile before production DTO
		// fields exist. Snapshot-local IDs below are not stable FunctionIds.
		type ownedSymbol struct {
			ID           string
			Name         string
			Kind         string
			Type         string
			Writable     bool
			Range        span
			LexicalOwner string
		}
		type ownedExpression struct {
			Kind      string
			Type      string
			Symbol    string
			Target    string
			Arguments []ownedExpression
			Boolean   *bool
			Range     span
		}
		type ownedStatement struct {
			Kind        string
			Symbol      *ownedSymbol
			Initializer *ownedExpression
			Expression  *ownedExpression
			Target      *ownedExpression
			Value       *ownedExpression
			Range       span
		}
		type ownedCapture struct {
			Symbol string
			Read   *bool
			Write  *bool
		}
		type ownedFunction struct {
			Name          string
			Symbol        string
			Parameters    []ownedSymbol
			ReturnType    string
			Body          []ownedStatement
			Range         span
			LexicalParent string
			Captures      []ownedCapture
		}
		var result struct {
			Status      string
			Inputs      []string
			Functions   []ownedFunction
			Symbols     []ownedSymbol
			Diagnostics []json.RawMessage
		}
		if err := json.Unmarshal(output.Bytes(), &result); err != nil {
			t.Fatalf("public response is not JSON: %v; rawResponse=%s", err, output.Bytes())
		}
		if result.Status != "ok" {
			t.Fatalf("officially valid shared mutable capture must project owned facts: diagnostics=%+v; rawResponse=%s", result.Diagnostics, output.Bytes())
		}
		if len(result.Diagnostics) != 0 || len(result.Functions) != 3 || len(result.Symbols) != 6 {
			t.Fatalf("incomplete owned closure definitions: %+v", result)
		}
		if len(result.Inputs) != 2 || result.Inputs[0] != "/src/main.ts" || result.Inputs[1] != "/src/minimal-integers-v3.d.ts" {
			t.Fatalf("original input closure changed: %+v", result.Inputs)
		}
		entry, writer, reader := result.Functions[0], result.Functions[1], result.Functions[2]
		if entry.Name != "entry" || writer.Name != "write" || reader.Name != "read" || entry.ReturnType != "bool" || writer.ReturnType != "bool" || reader.ReturnType != "bool" || len(entry.Parameters) != 1 || len(writer.Parameters) != 1 || len(reader.Parameters) != 0 || len(entry.Body) != 5 || len(writer.Body) != 2 || len(reader.Body) != 1 {
			t.Fatalf("ordered functions lost their complete signatures or bodies: %+v", result.Functions)
		}
		definitions := map[string]ownedSymbol{}
		for _, definition := range result.Symbols {
			if definition.ID == "" || definitions[definition.ID].ID != "" {
				t.Fatalf("missing or duplicate owned symbol identity: %+v", definition)
			}
			definitions[definition.ID] = definition
		}
		checkSymbol := func(s ownedSymbol, name, kind, owner string, writable bool) {
			t.Helper()
			if s.ID == "" || s.Name != name || s.Kind != kind || s.Type != "bool" || s.Writable != writable || s.LexicalOwner != owner || sourceText(t, source, s.Range) != name || definitions[s.ID] != s {
				t.Fatalf("owned symbol lost its role, owner, anchor or definition: %+v", s)
			}
		}
		checkSymbol(definitions[entry.Symbol], "entry", "function", "", false)
		checkSymbol(definitions[writer.Symbol], "write", "function", entry.Symbol, false)
		checkSymbol(definitions[reader.Symbol], "read", "function", entry.Symbol, false)
		checkSymbol(entry.Parameters[0], "seed", "parameter", entry.Symbol, true)
		checkSymbol(writer.Parameters[0], "value", "parameter", writer.Symbol, true)
		if entry.LexicalParent != "" || writer.LexicalParent != entry.Symbol || reader.LexicalParent != entry.Symbol || len(entry.Captures) != 0 {
			t.Fatalf("owned lexical ancestry or entry captures are incorrect: %+v", result.Functions)
		}
		local := entry.Body[0]
		if local.Kind != "variable" || local.Symbol == nil || local.Initializer == nil || local.Initializer.Kind != "reference" || local.Initializer.Type != "bool" || local.Initializer.Symbol != entry.Parameters[0].ID || sourceText(t, source, local.Range) != "state: bool = seed" {
			t.Fatalf("outer state initializer lost its original binding: %+v", local)
		}
		state := *local.Symbol
		checkSymbol(state, "state", "local", entry.Symbol, true)
		if state.ID == entry.Parameters[0].ID || state.ID == writer.Parameters[0].ID {
			t.Fatal("captured state identity was reused for a parameter")
		}
		for index, nested := range []ownedFunction{writer, reader} {
			declaration := entry.Body[index+1]
			if declaration.Kind != "functionDeclaration" || declaration.Symbol == nil || *declaration.Symbol != definitions[nested.Symbol] || declaration.Range != nested.Range {
				t.Fatalf("nested declaration lost its ordered owned symbol or full range: %+v", declaration)
			}
			if len(nested.Captures) != 1 || nested.Captures[0].Symbol != state.ID || nested.Captures[0].Read == nil || !*nested.Captures[0].Read || nested.Captures[0].Write == nil || *nested.Captures[0].Write != (index == 0) {
				t.Fatalf("nested function copied or misclassified the shared binding: %+v", nested)
			}
		}
		if sourceText(t, source, entry.Range) != source || sourceText(t, source, writer.Range) != "function write(value: bool): bool { state = value; return state; }" || sourceText(t, source, reader.Range) != "function read(): bool { return state; }" {
			t.Fatal("function declarations lost their original complete source ranges")
		}
		checkReference := func(value *ownedExpression, id, text string) {
			t.Helper()
			if value == nil || value.Kind != "reference" || value.Type != "bool" || value.Symbol != id || sourceText(t, source, value.Range) != text {
				t.Fatalf("reference lost its original checked binding or anchor: %+v", value)
			}
		}
		assignment := writer.Body[0]
		if assignment.Kind != "assignment" || sourceText(t, source, assignment.Range) != "state = value;" {
			t.Fatalf("writer lost the original shared assignment: %+v", assignment)
		}
		checkReference(assignment.Target, state.ID, "state")
		checkReference(assignment.Value, writer.Parameters[0].ID, "value")
		for _, returned := range []ownedStatement{writer.Body[1], reader.Body[0]} {
			if returned.Kind != "return" || sourceText(t, source, returned.Range) != "return state;" {
				t.Fatalf("nested function lost its read of shared state: %+v", returned)
			}
			checkReference(returned.Expression, state.ID, "state")
		}
		writeCall, readCall := entry.Body[3], entry.Body[4]
		if writeCall.Kind != "expression" || writeCall.Expression == nil || writeCall.Expression.Kind != "call" || writeCall.Expression.Type != "bool" || writeCall.Expression.Target != writer.Symbol || len(writeCall.Expression.Arguments) != 1 || sourceText(t, source, writeCall.Range) != "write(false);" {
			t.Fatalf("entry lost the original call to its nested writer: %+v", writeCall)
		}
		argument := writeCall.Expression.Arguments[0]
		if argument.Kind != "boolean" || argument.Type != "bool" || argument.Boolean == nil || *argument.Boolean || sourceText(t, source, argument.Range) != "false" {
			t.Fatalf("writer call lost its false argument: %+v", argument)
		}
		if readCall.Kind != "return" || readCall.Expression == nil || readCall.Expression.Kind != "call" || readCall.Expression.Type != "bool" || readCall.Expression.Target != reader.Symbol || len(readCall.Expression.Arguments) != 0 || sourceText(t, source, readCall.Range) != "return read();" {
			t.Fatalf("entry lost its final nested reader call: %+v", readCall)
		}
		// This public response proves owned Source facts only. Environment
		// instances, GC roots, and shared-state execution still need consumers.
	})
}
