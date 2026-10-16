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

func TestFrontendCommandNullableReturnCallOwnedFacts(t *testing.T) {
	const source = "function echo(value: i32 | null | undefined): i32 | null | undefined { return value; }\nexport function forward(value: i32 | null | undefined): i32 | null | undefined { return echo(value); }"
	r := integerRequest(source)
	if !t.Run("officialDiagnostics", func(t *testing.T) {
		// Keep this fixture's official precondition independent of owned
		// projection and of the first nullable-return test's result.
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
			t.Fatalf("officially valid nullable call must preserve owned facts: response=%+v; diagnostics=%+v; rawResponse=%s", result, result.Diagnostics, output.Bytes())
		}
		if len(result.Diagnostics) != 0 || len(result.Functions) != 2 || len(result.Symbols) != 4 {
			t.Fatalf("incomplete nullable call closure: response=%+v; rawResponse=%s", result, output.Bytes())
		}
		if len(result.Inputs) != 2 || result.Inputs[0] != "/src/main.ts" || result.Inputs[1] != "/src/minimal-integers-v3.d.ts" {
			t.Fatalf("original source/declaration closure changed: %+v", result.Inputs)
		}
		echo, forward := result.Functions[0], result.Functions[1]
		if echo.Name != "echo" || forward.Name != "forward" || echo.Symbol == "" || forward.Symbol == "" || echo.Symbol == forward.Symbol {
			t.Fatalf("internal callee and caller lost distinct identities: %+v", result.Functions)
		}
		symbols := map[string]symbol{}
		for _, owned := range result.Symbols {
			if owned.ID == "" || symbols[owned.ID].ID != "" {
				t.Fatalf("missing or duplicate owned symbol: %+v", owned)
			}
			symbols[owned.ID] = owned
		}
		for index, expected := range []struct {
			name                                      string
			declaration, nameRange, parameterRange    span
			parameterDeclaration, parameterAnnotation span
			text                                      string
		}{
			{"echo", span{0, 86}, span{8, 13}, span{14, 19}, span{14, 43}, span{21, 43}, source[:86]},
			{"forward", span{86, 189}, span{102, 110}, span{111, 116}, span{111, 140}, span{118, 140}, source[87:]},
		} {
			fn := result.Functions[index]
			if fn.ReturnType != "i32" || fn.Range != expected.declaration || sourceText(t, source, fn.Range) != expected.text || len(fn.Parameters) != 1 || len(fn.Body) != 1 || fn.Body[0].Kind != "return" || fn.Body[0].Expression == nil {
				t.Fatalf("function lost its original declaration or body: %+v", fn)
			}
			definition := symbols[fn.Symbol]
			if definition.Name != expected.name || definition.Kind != "function" || definition.Type != "i32" || definition.Writable || definition.Range != expected.nameRange || sourceText(t, source, definition.Range) != expected.name || definition.Presence != nil {
				t.Fatalf("function symbol lost its original role: %+v", definition)
			}
			parameter := fn.Parameters[0]
			if parameter.ID == "" || parameter.Name != "value" || parameter.Kind != "parameter" || parameter.Type != "i32" || !parameter.Writable || parameter.Range != expected.parameterRange || sourceText(t, source, parameter.Range) != "value" {
				t.Fatalf("parameter lost its distinct checked binding: %+v", parameter)
			}
			shape := parameter.Presence
			if shape == nil || !shape.Undefined || !shape.Null || shape.Optional || shape.Default || shape.Range != expected.parameterDeclaration || shape.Annotation != expected.parameterAnnotation || sourceText(t, source, shape.Range) != "value: i32 | null | undefined" || sourceText(t, source, shape.Annotation) != "i32 | null | undefined" {
				t.Fatalf("nullable parameter was confused with omission or default: %+v", shape)
			}
			definition = symbols[parameter.ID]
			if definition.ID != parameter.ID || definition.Name != parameter.Name || definition.Kind != parameter.Kind || definition.Type != parameter.Type || definition.Writable != parameter.Writable || definition.Range != parameter.Range || definition.Presence == nil || *definition.Presence != *shape {
				t.Fatalf("parameter and owned symbol disagree: parameter=%+v definition=%+v", parameter, definition)
			}
		}
		if echo.Parameters[0].ID == forward.Parameters[0].ID {
			t.Fatal("same-spelled parameters in different functions share an owned identity")
		}
		checkFlow := func(value *expression) {
			t.Helper()
			if value == nil || value.Type != "i32" || value.PresenceFlow == nil || !value.PresenceFlow.Undefined || !value.PresenceFlow.Null {
				t.Fatalf("official use-site null/undefined flow was erased: %+v", value)
			}
		}
		echoValue := echo.Body[0].Expression
		if echo.Body[0].Range != (span{70, 84}) || sourceText(t, source, echo.Body[0].Range) != "return value;" || echoValue.Kind != "reference" || echoValue.Symbol != echo.Parameters[0].ID || echoValue.Range != (span{77, 83}) || sourceText(t, source, echoValue.Range) != "value" {
			t.Fatalf("callee return lost its parameter binding or original span: %+v", echoValue)
		}
		checkFlow(echoValue)
		call := forward.Body[0].Expression
		// Preserve raw node.Pos trivia; the call token starts at 175.
		if forward.Body[0].Range != (span{167, 187}) || sourceText(t, source, forward.Body[0].Range) != "return echo(value);" || call.Kind != "call" || call.Target != echo.Symbol || call.Range != (span{174, 186}) || sourceText(t, source, call.Range) != "echo(value)" || call.Arguments == nil || len(*call.Arguments) != 1 || call.Signature == nil {
			t.Fatalf("forwarding call lost its callee, arguments or source span: %+v", call)
		}
		checkFlow(call)
		argument := (*call.Arguments)[0]
		if argument.Kind != "reference" || argument.Symbol != forward.Parameters[0].ID || argument.Symbol == echo.Parameters[0].ID || argument.Range != (span{180, 185}) || sourceText(t, source, argument.Range) != "value" {
			t.Fatalf("call argument resolved to the wrong function's value binding: %+v", argument)
		}
		checkFlow(&argument)
		signature := call.Signature
		if signature.ReturnType != "i32" || len(signature.Parameters) != 1 || signature.Parameters[0] != "i32" || len(signature.Presence) != 1 || signature.Presence[0] == nil || *signature.Presence[0] != *echo.Parameters[0].Presence {
			t.Fatalf("resolved call signature differs from the callee's checked declaration: %+v", signature)
		}

		// Read new declaration fields through local JSON views, without
		// coupling this RED to a new production DTO or checker interface.
		var wire struct {
			Functions []struct {
				ReturnPresence json.RawMessage
				Body           []struct {
					Expression struct {
						Signature struct {
							ReturnPresence json.RawMessage
						}
					}
				}
			}
			Symbols []struct {
				ID             string
				ReturnPresence json.RawMessage
			}
		}
		if err := json.Unmarshal(output.Bytes(), &wire); err != nil {
			t.Fatal(err)
		}
		checkReturnShape := func(raw json.RawMessage, declaration, annotation span, text string) {
			t.Helper()
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(raw, &fields); err != nil || len(fields) != 4 {
				t.Fatalf("return presence must have exactly four fields: raw=%s err=%v", raw, err)
			}
			for _, key := range []string{"undefined", "null", "range", "annotation"} {
				if _, exists := fields[key]; !exists {
					t.Fatalf("return presence omitted %s: %s", key, raw)
				}
			}
			var shape struct {
				Undefined, Null   bool
				Range, Annotation span
			}
			if err := json.Unmarshal(raw, &shape); err != nil {
				t.Fatal(err)
			}
			if !shape.Undefined || !shape.Null || shape.Range != declaration || shape.Annotation != annotation || sourceText(t, source, shape.Range) != text || sourceText(t, source, shape.Annotation) != "i32 | null | undefined" {
				t.Fatalf("return presence points to the wrong declaration or members: %s", raw)
			}
		}
		if len(wire.Functions) != 2 || len(wire.Symbols) != 4 || len(wire.Functions[1].Body) != 1 {
			t.Fatalf("JSON definitions differ from the owned closure: %s", output.Bytes())
		}
		checkReturnShape(wire.Functions[0].ReturnPresence, span{0, 86}, span{46, 68}, source[:86])
		checkReturnShape(wire.Functions[1].ReturnPresence, span{86, 189}, span{143, 165}, source[87:])
		// A resolved-call signature belongs to echo, never to forward's
		// similarly spelled return annotation or to the call's own span.
		checkReturnShape(wire.Functions[1].Body[0].Expression.Signature.ReturnPresence, span{0, 86}, span{46, 68}, source[:86])
		foundEcho, foundForward := false, false
		for _, owned := range wire.Symbols {
			switch owned.ID {
			case echo.Symbol:
				foundEcho = true
				checkReturnShape(owned.ReturnPresence, span{0, 86}, span{46, 68}, source[:86])
			case forward.Symbol:
				foundForward = true
				checkReturnShape(owned.ReturnPresence, span{86, 189}, span{143, 165}, source[87:])
			default:
				if len(owned.ReturnPresence) != 0 {
					t.Fatalf("return declaration leaked onto a parameter: %s", owned.ReturnPresence)
				}
			}
		}
		if !foundEcho || !foundForward {
			t.Fatal("JSON response omitted a function's defining symbol")
		}
		original, err := json.Marshal(map[string]any{"rootFile": r.RootFile, "files": r.Files})
		if err != nil {
			t.Fatal(err)
		}
		// Both functions here check facts, not host nullable ABI support.
		t.Logf("TSFG_NULLABLE_CALL_REQUEST_V1 %s", base64.StdEncoding.EncodeToString(input))
		t.Logf("TSFG_NULLABLE_CALL_RESPONSE_V1 %s", base64.StdEncoding.EncodeToString(output.Bytes()))
		t.Logf("TSFG_NULLABLE_CALL_ORIGINAL_V1 %s", base64.StdEncoding.EncodeToString(original))
	})
}
