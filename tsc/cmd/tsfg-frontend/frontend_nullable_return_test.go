// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

func TestFrontendCommandNullableReturnOwnedFacts(t *testing.T) {
	const source = "export function echo(value: i32 | null | undefined): i32 | null | undefined { return value; }"
	r := integerRequest(source)
	if !t.Run("officialDiagnostics", func(t *testing.T) {
		contents := map[string]string{}
		for _, file := range r.Files {
			contents[file.Path] = file.Content
		}
		fileNames := append([]string{r.RootFile}, r.Declarations.Paths...)
		program := compiler.NewProgram(compiler.ProgramOptions{
			Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
				FileNames: fileNames,
				CompilerOptions: &core.CompilerOptions{
					NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext,
				},
			}},
			Host:           compiler.NewCompilerHost(r.SourceRoot, vfstest.FromMap(contents, true), r.SourceRoot, nil, nil),
			SingleThreaded: core.TSTrue,
		})
		if program.GetSourceFile(r.RootFile) == nil {
			t.Fatal("official program omitted the original source")
		}
		// Check the official program directly, before owned projection or any
		// custom scalar assignment context can admit or replace a diagnostic.
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
	}) {
		t.Fatal("official diagnostics failed; owned projection was not attempted")
	}

	t.Run("ownedProjection", func(t *testing.T) {
		input, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		if err := runFrontendCommand(t.Context(), nil, bytes.NewReader(input), &output); err != nil {
			t.Fatalf("public frontend command failed: %v; rawResponse=%s", err, output.Bytes())
		}
		var result response
		if err := json.Unmarshal(output.Bytes(), &result); err != nil {
			t.Fatalf("public response is not JSON: %v; rawResponse=%s", err, output.Bytes())
		}
		if result.Status != "ok" {
			t.Fatalf("officially valid nullable return must project owned facts: response=%+v; diagnostics=%+v; rawResponse=%s", result, result.Diagnostics, output.Bytes())
		}
		if len(result.Diagnostics) != 0 || len(result.Functions) != 1 || len(result.Symbols) != 2 {
			t.Fatalf("incomplete owned nullable function: response=%+v; rawResponse=%s", result, output.Bytes())
		}
		if len(result.Inputs) != 2 || result.Inputs[0] != "/src/main.ts" || result.Inputs[1] != "/src/minimal-integers-v3.d.ts" {
			t.Fatalf("original source/declaration closure changed: %+v", result.Inputs)
		}
		fn := result.Functions[0]
		if fn.Name != "echo" || fn.Symbol == "" || fn.ReturnType != "i32" || fn.Range != (span{Start: 0, End: 93}) || sourceText(t, source, fn.Range) != source {
			t.Fatalf("function lost its payload, identity or original declaration: %+v", fn)
		}
		if len(fn.Parameters) != 1 || len(fn.Body) != 1 || fn.Body[0].Kind != "return" || fn.Body[0].Expression == nil {
			t.Fatalf("function lost its checked parameter or return statement: %+v", fn)
		}
		symbols := map[string]symbol{}
		for _, owned := range result.Symbols {
			if owned.ID == "" || symbols[owned.ID].ID != "" {
				t.Fatalf("missing or duplicate owned symbol: %+v", owned)
			}
			symbols[owned.ID] = owned
		}
		definition := symbols[fn.Symbol]
		// Existing raw spans retain node.Pos, including preceding trivia. The
		// token spans are echo 16..20 and return value 85..90; do not rewrite
		// those existing raw span conventions to add nullable return facts.
		if definition.Name != "echo" || definition.Kind != "function" || definition.Type != "i32" || definition.Writable || definition.Range != (span{Start: 15, End: 20}) || sourceText(t, source, definition.Range) != "echo" || definition.Presence != nil {
			t.Fatalf("function symbol lost its declared role or source anchor: %+v", definition)
		}
		parameter := fn.Parameters[0]
		if parameter.Name != "value" || parameter.Kind != "parameter" || parameter.Type != "i32" || !parameter.Writable || parameter.Range != (span{Start: 21, End: 26}) || sourceText(t, source, parameter.Range) != "value" {
			t.Fatalf("parameter lost its original checked identity: %+v", parameter)
		}
		shape := parameter.Presence
		if shape == nil || !shape.Undefined || !shape.Null || shape.Optional || shape.Default || shape.Range != (span{Start: 21, End: 50}) || shape.Annotation != (span{Start: 28, End: 50}) || sourceText(t, source, shape.Range) != "value: i32 | null | undefined" || sourceText(t, source, shape.Annotation) != "i32 | null | undefined" {
			t.Fatalf("required nullable parameter was confused with optional/default: %+v", shape)
		}
		parameterDefinition := symbols[parameter.ID]
		if parameterDefinition.ID != parameter.ID || parameterDefinition.Name != parameter.Name || parameterDefinition.Kind != parameter.Kind || parameterDefinition.Type != parameter.Type || parameterDefinition.Writable != parameter.Writable || parameterDefinition.Range != parameter.Range || parameterDefinition.Presence == nil || *parameterDefinition.Presence != *shape {
			t.Fatalf("parameter and symbol definition disagree: parameter=%+v definition=%+v", parameter, parameterDefinition)
		}
		returned := fn.Body[0]
		value := returned.Expression
		if returned.Range != (span{Start: 77, End: 91}) || sourceText(t, source, returned.Range) != "return value;" || value.Kind != "reference" || value.Type != "i32" || value.Symbol != parameter.ID || value.Range != (span{Start: 84, End: 90}) || sourceText(t, source, value.Range) != "value" || value.Absence != "" || value.PresenceFlow == nil || !value.PresenceFlow.Undefined || !value.PresenceFlow.Null {
			t.Fatalf("return lost the official use-site null/undefined flow: statement=%+v expression=%+v", returned, value)
		}

		// Local JSON views let the first RED compile against the existing DTO.
		// The new return role must never be read through parameterPresence.
		var wire struct {
			Functions []struct {
				ReturnPresence json.RawMessage
			}
			Symbols []struct {
				ID             string
				ReturnPresence json.RawMessage
			}
		}
		if err := json.Unmarshal(output.Bytes(), &wire); err != nil {
			t.Fatal(err)
		}
		checkReturnShape := func(raw json.RawMessage) {
			t.Helper()
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(raw, &fields); err != nil || len(fields) != 4 {
				t.Fatalf("return presence must have exactly four owned fields: raw=%s err=%v", raw, err)
			}
			for _, key := range []string{"undefined", "null", "range", "annotation"} {
				if _, exists := fields[key]; !exists {
					t.Fatalf("return presence omitted %s: %s", key, raw)
				}
			}
			var declared struct {
				Undefined  bool
				Null       bool
				Range      span
				Annotation span
			}
			if err := json.Unmarshal(raw, &declared); err != nil {
				t.Fatal(err)
			}
			if !declared.Undefined || !declared.Null || declared.Range != (span{Start: 0, End: 93}) || declared.Annotation != (span{Start: 53, End: 75}) || sourceText(t, source, declared.Range) != source || sourceText(t, source, declared.Annotation) != "i32 | null | undefined" {
				t.Fatalf("return declaration lost distinct members or original anchors: %s", raw)
			}
		}
		if len(wire.Functions) != 1 || len(wire.Symbols) != 2 {
			t.Fatalf("JSON definitions differ from the owned response: %s", output.Bytes())
		}
		checkReturnShape(wire.Functions[0].ReturnPresence)
		foundDefinition := false
		for _, owned := range wire.Symbols {
			if owned.ID == fn.Symbol {
				foundDefinition = true
				checkReturnShape(owned.ReturnPresence)
			} else if len(owned.ReturnPresence) != 0 {
				t.Fatalf("return declaration leaked onto a parameter: %s", owned.ReturnPresence)
			}
		}
		if !foundDefinition {
			t.Fatal("JSON response omitted the function's defining symbol")
		}
		original, err := json.Marshal(map[string]any{"rootFile": r.RootFile, "files": r.Files})
		if err != nil {
			t.Fatal(err)
		}
		// Emit exact request/response/original bytes only after every assertion.
		// The source export here checks facts; it is not host nullable ABI support.
		t.Logf("TSFG_NULLABLE_RETURN_REQUEST_V1 %s", base64.StdEncoding.EncodeToString(input))
		t.Logf("TSFG_NULLABLE_RETURN_RESPONSE_V1 %s", base64.StdEncoding.EncodeToString(output.Bytes()))
		t.Logf("TSFG_NULLABLE_RETURN_ORIGINAL_V1 %s", base64.StdEncoding.EncodeToString(original))
	})
}
