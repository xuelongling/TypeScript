// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

// Each fixture proves its official diagnostic precondition before the public
// command runs. This deliberately does not use owned scalar diagnostic adapters.
func nullableBoundaryOfficialForTest(t *testing.T, r request, semanticCode int32) {
	t.Helper()
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
	if program.GetSourceFile(r.RootFile) == nil {
		t.Fatal("official program omitted original source")
	}
	if diagnostics := program.GetSyntacticDiagnostics(t.Context(), nil); len(diagnostics) != 0 {
		t.Fatalf("official syntax diagnostics: %+v", diagnostics)
	}
	diagnostics := program.GetSemanticDiagnostics(t.Context(), nil)
	if semanticCode == 0 {
		if len(diagnostics) != 0 {
			t.Fatalf("official semantic diagnostics: %+v", diagnostics)
		}
	} else if len(diagnostics) != 1 || diagnostics[0].Code() != semanticCode {
		t.Fatalf("expected official semantic code %d exactly once: %+v", semanticCode, diagnostics)
	}
	if diagnostics := program.GetProgramDiagnostics(); len(diagnostics) != 0 {
		t.Fatalf("official program diagnostics: %+v", diagnostics)
	}
	if diagnostics := program.GetGlobalDiagnostics(t.Context()); len(diagnostics) != 0 {
		t.Fatalf("official global diagnostics: %+v", diagnostics)
	}
}

func nullableBoundaryCommandForTest(t *testing.T, r request) (response, []byte) {
	t.Helper()
	input, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runFrontendCommand(t.Context(), nil, bytes.NewReader(input), &output); err != nil {
		t.Fatalf("public command failed: %v; rawResponse=%s", err, output.Bytes())
	}
	var result response
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("invalid public response: %v; rawResponse=%s", err, output.Bytes())
	}
	if result.Status == "ok" && (len(result.Inputs) != 2 || result.Inputs[0] != r.RootFile || result.Inputs[1] != r.Declarations.Paths[0]) {
		t.Fatalf("public command changed the original source/declaration closure: %+v", result.Inputs)
	}
	return result, output.Bytes()
}

func nullableBoundaryRecordForTest(t *testing.T, raw []byte, collection, key, value string) map[string]json.RawMessage {
	t.Helper()
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	var records []map[string]json.RawMessage
	if err := json.Unmarshal(envelope[collection], &records); err != nil {
		t.Fatal(err)
	}
	var found map[string]json.RawMessage
	for _, record := range records {
		var identity string
		if err := json.Unmarshal(record[key], &identity); err != nil {
			t.Fatal(err)
		}
		if identity == value {
			if found != nil {
				t.Fatalf("duplicate %s %s=%s", collection, key, value)
			}
			found = record
		}
	}
	if found == nil {
		t.Fatalf("missing %s %s=%s; rawResponse=%s", collection, key, value, raw)
	}
	return found
}

func nullableBoundaryFlagsForTest(t *testing.T, fields map[string]json.RawMessage, undefined, null bool) {
	t.Helper()
	for key, expected := range map[string]bool{"undefined": undefined, "null": null} {
		want := "false"
		if expected {
			want = "true"
		}
		if string(fields[key]) != want {
			t.Fatalf("%s must be explicitly serialized as %s: %v", key, want, fields)
		}
	}
}

func nullableBoundaryReturnsForTest(t *testing.T, raw json.RawMessage) []map[string]json.RawMessage {
	t.Helper()
	var statements []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &statements); err != nil {
		t.Fatal(err)
	}
	var returned []map[string]json.RawMessage
	for _, item := range statements {
		var kind string
		if err := json.Unmarshal(item["kind"], &kind); err != nil {
			t.Fatal(err)
		}
		if kind == "return" {
			var expression map[string]json.RawMessage
			if err := json.Unmarshal(item["expression"], &expression); err != nil {
				t.Fatal(err)
			}
			returned = append(returned, expression)
		}
		for _, key := range []string{"body", "thenBody", "elseBody"} {
			if len(item[key]) != 0 {
				returned = append(returned, nullableBoundaryReturnsForTest(t, item[key])...)
			}
		}
	}
	return returned
}

func nullableBoundaryReturnShapeForTest(t *testing.T, raw json.RawMessage, source string, undefined, null bool, declaration, annotation span, annotationText, text string) {
	t.Helper()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || len(fields) != 4 {
		t.Fatalf("return shape must have four exact fields: raw=%s err=%v", raw, err)
	}
	nullableBoundaryFlagsForTest(t, fields, undefined, null)
	var shape struct {
		Range, Annotation span
	}
	if err := json.Unmarshal(raw, &shape); err != nil {
		t.Fatal(err)
	}
	if shape.Range != declaration || shape.Annotation != annotation || sourceText(t, source, shape.Range) != text || sourceText(t, source, shape.Annotation) != annotationText {
		t.Fatalf("return shape lost its original declaration anchors: %s", raw)
	}
}

func TestFrontendCommandNullableReturnSingleAbsenceOwnedFacts(t *testing.T) {
	for _, tc := range []struct {
		name            string
		undefined, null bool
	}{
		{"null", false, true},
		{"undefined", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			typeName := "i32 | " + tc.name
			calleeText := "function echo(value: " + typeName + "): " + typeName + " { return value; }"
			callerText := "export function forward(value: " + typeName + "): " + typeName + " { return echo(value); }"
			source := calleeText + "\n" + callerText
			r := integerRequest(source)
			if !t.Run("officialDiagnostics", func(t *testing.T) { nullableBoundaryOfficialForTest(t, r, 0) }) {
				return
			}
			t.Run("ownedProjection", func(t *testing.T) {
				result, raw := nullableBoundaryCommandForTest(t, r)
				if result.Status != "ok" || len(result.Diagnostics) != 0 || len(result.Functions) != 2 || len(result.Symbols) != 4 {
					t.Fatalf("single-absence return/call rejected: %+v; rawResponse=%s", result, raw)
				}
				functions := map[string]function{}
				symbols := map[string]symbol{}
				for _, fn := range result.Functions {
					functions[fn.Name] = fn
				}
				for _, owned := range result.Symbols {
					symbols[owned.ID] = owned
				}
				echo, forward := functions["echo"], functions["forward"]
				if echo.Symbol == "" || forward.Symbol == "" || echo.Symbol == forward.Symbol || len(echo.Parameters) != 1 || len(forward.Parameters) != 1 || len(echo.Body) != 1 || len(forward.Body) != 1 || echo.Body[0].Expression == nil || forward.Body[0].Expression == nil {
					t.Fatalf("missing distinct callee/caller closure: %+v", result.Functions)
				}
				for _, expected := range []struct {
					fn   function
					text string
					decl span
				}{
					{echo, calleeText, span{0, len(calleeText)}},
					{forward, callerText, span{len(calleeText), len(source)}},
				} {
					fn := expected.fn
					annotation := span{expected.decl.Start + strings.Index(source[expected.decl.Start:expected.decl.End], "): ") + 3, 0}
					annotation.End = annotation.Start + len(typeName)
					record := nullableBoundaryRecordForTest(t, raw, "functions", "name", fn.Name)
					nullableBoundaryReturnShapeForTest(t, record["returnPresence"], source, tc.undefined, tc.null, expected.decl, annotation, typeName, expected.text)
					definition := symbols[fn.Symbol]
					if fn.ReturnType != "i32" || fn.Range != expected.decl || definition.Kind != "function" || definition.Name != fn.Name || definition.Type != "i32" || sourceText(t, source, definition.Range) != fn.Name {
						t.Fatalf("function differs from its declaration or symbol: %+v", fn)
					}
					record = nullableBoundaryRecordForTest(t, raw, "symbols", "id", fn.Symbol)
					nullableBoundaryReturnShapeForTest(t, record["returnPresence"], source, tc.undefined, tc.null, expected.decl, annotation, typeName, expected.text)
					shape := fn.Parameters[0].Presence
					if shape == nil || shape.Undefined != tc.undefined || shape.Null != tc.null || shape.Optional || shape.Default || sourceText(t, source, shape.Annotation) != typeName {
						t.Fatalf("single-absence parameter changed role: %+v", shape)
					}
				}
				call := forward.Body[0].Expression
				if call.Kind != "call" || call.Target != echo.Symbol || call.Signature == nil || call.Arguments == nil || len(*call.Arguments) != 1 || sourceText(t, source, call.Range) != "echo(value)" || echo.Parameters[0].ID == forward.Parameters[0].ID {
					t.Fatalf("call lost its target or distinct bindings: %+v", call)
				}
				argument := (*call.Arguments)[0]
				if argument.Kind != "reference" || argument.Symbol != forward.Parameters[0].ID || echo.Body[0].Expression.Symbol != echo.Parameters[0].ID {
					t.Fatalf("call/reference bound the wrong value: %+v", call)
				}
				for _, value := range []*expression{call, &argument, echo.Body[0].Expression} {
					if value.Type != "i32" || value.PresenceFlow == nil || value.PresenceFlow.Undefined != tc.undefined || value.PresenceFlow.Null != tc.null {
						t.Fatalf("single-absence use-site flow changed: %+v", value)
					}
				}
				record := nullableBoundaryRecordForTest(t, raw, "functions", "name", "forward")
				returns := nullableBoundaryReturnsForTest(t, record["body"])
				if len(returns) != 1 {
					t.Fatal("missing raw forwarding return")
				}
				var signature map[string]json.RawMessage
				if err := json.Unmarshal(returns[0]["signature"], &signature); err != nil {
					t.Fatal(err)
				}
				var flow map[string]json.RawMessage
				if err := json.Unmarshal(returns[0]["presenceFlow"], &flow); err != nil || len(flow) != 2 {
					t.Fatalf("missing explicit call flow flags: %v", err)
				}
				nullableBoundaryFlagsForTest(t, flow, tc.undefined, tc.null)
				annotationStart := strings.Index(calleeText, "): ") + 3
				nullableBoundaryReturnShapeForTest(t, signature["returnPresence"], source, tc.undefined, tc.null, span{0, len(calleeText)}, span{annotationStart, annotationStart + len(typeName)}, typeName, calleeText)
			})
		})
	}
}

func TestFrontendCommandNullableReturnNarrowingOwnedFacts(t *testing.T) {
	const source = "export function narrow(value: i32 | null | undefined): i32 | null | undefined { if (value === null) { return value; } if (value === undefined) { return value; } return value; }"
	r := integerRequest(source)
	if !t.Run("officialDiagnostics", func(t *testing.T) { nullableBoundaryOfficialForTest(t, r, 0) }) {
		return
	}
	t.Run("ownedProjection", func(t *testing.T) {
		result, raw := nullableBoundaryCommandForTest(t, r)
		if result.Status != "ok" || len(result.Diagnostics) != 0 || len(result.Functions) != 1 {
			t.Fatalf("official narrowing rejected: %+v; rawResponse=%s", result, raw)
		}
		fn := result.Functions[0]
		var returned []*expression
		var guards []*expression
		var visit func([]statement)
		visit = func(body []statement) {
			for _, item := range body {
				if item.Kind == "return" {
					returned = append(returned, item.Expression)
				}
				if item.Kind == "if" {
					guards = append(guards, item.Condition)
				}
				visit(item.Body)
				visit(item.ThenBody)
				visit(item.ElseBody)
			}
		}
		visit(fn.Body)
		if len(fn.Parameters) != 1 || len(returned) != 3 || len(guards) != 2 {
			t.Fatalf("missing guard/return structure: %+v", fn.Body)
		}
		for index, expected := range []struct{ undefined, null bool }{{false, true}, {true, false}, {false, false}} {
			value := returned[index]
			if value == nil || value.Kind != "reference" || value.Type != "i32" || value.Symbol != fn.Parameters[0].ID || sourceText(t, source, value.Range) != "value" || value.PresenceFlow == nil || value.PresenceFlow.Undefined != expected.undefined || value.PresenceFlow.Null != expected.null {
				t.Fatalf("return %d lost official narrowing: %+v", index, value)
			}
		}
		for index, text := range []string{"value === null", "value === undefined"} {
			if guards[index] == nil || guards[index].Kind != "presenceTest" || sourceText(t, source, guards[index].Range) != text {
				t.Fatalf("guard %d lost its official source identity: %+v", index, guards[index])
			}
		}
		record := nullableBoundaryRecordForTest(t, raw, "functions", "name", "narrow")
		start := strings.Index(source, "): ") + 3
		nullableBoundaryReturnShapeForTest(t, record["returnPresence"], source, true, true, span{0, len(source)}, span{start, start + len("i32 | null | undefined")}, "i32 | null | undefined", source)
		rawReturns := nullableBoundaryReturnsForTest(t, record["body"])
		if len(rawReturns) != 3 {
			t.Fatal("missing original narrowing return expressions")
		}
		for index, expected := range []struct{ undefined, null bool }{{false, true}, {true, false}, {false, false}} {
			var flow map[string]json.RawMessage
			if err := json.Unmarshal(rawReturns[index]["presenceFlow"], &flow); err != nil || len(flow) != 2 {
				t.Fatalf("narrowed return omitted explicit flow flags: %v", err)
			}
			nullableBoundaryFlagsForTest(t, flow, expected.undefined, expected.null)
		}
	})
}

func TestFrontendCommandScalarReturnOmitsNullableShape(t *testing.T) {
	const source = "function identity(value: i32): i32 { return value; }\nexport function forward(value: i32): i32 { return identity(value); }"
	r := integerRequest(source)
	if !t.Run("officialDiagnostics", func(t *testing.T) { nullableBoundaryOfficialForTest(t, r, 0) }) {
		return
	}
	t.Run("ownedProjection", func(t *testing.T) {
		result, raw := nullableBoundaryCommandForTest(t, r)
		if result.Status != "ok" || len(result.Diagnostics) != 0 || len(result.Functions) != 2 {
			t.Fatalf("pure scalar call changed: %+v; rawResponse=%s", result, raw)
		}
		for _, fn := range result.Functions {
			record := nullableBoundaryRecordForTest(t, raw, "functions", "name", fn.Name)
			if _, exists := record["returnPresence"]; exists {
				t.Fatalf("pure scalar function acquired nullable declaration: %s", raw)
			}
			record = nullableBoundaryRecordForTest(t, raw, "symbols", "id", fn.Symbol)
			if _, exists := record["returnPresence"]; exists {
				t.Fatalf("pure scalar function symbol acquired nullable declaration: %s", raw)
			}
			if fn.ReturnType != "i32" || len(fn.Body) != 1 || fn.Body[0].Expression == nil || fn.Body[0].Expression.PresenceFlow != nil {
				t.Fatalf("pure scalar return acquired nullable flow: %+v", fn)
			}
		}
		var identity, forward function
		for _, fn := range result.Functions {
			if fn.Name == "identity" {
				identity = fn
			} else if fn.Name == "forward" {
				forward = fn
			}
		}
		if identity.Symbol == "" || forward.Symbol == "" || len(identity.Parameters) != 1 || len(forward.Parameters) != 1 {
			t.Fatal("pure scalar fixture omitted its distinct function definitions")
		}
		call := forward.Body[0].Expression
		if call.Kind != "call" || call.Target != identity.Symbol || call.Signature == nil || call.Arguments == nil || len(*call.Arguments) != 1 || (*call.Arguments)[0].Symbol != forward.Parameters[0].ID {
			t.Fatalf("pure scalar call lost its target or argument binding: %+v", call)
		}
		record := nullableBoundaryRecordForTest(t, raw, "functions", "name", "forward")
		returns := nullableBoundaryReturnsForTest(t, record["body"])
		if len(returns) != 1 {
			t.Fatal("missing raw scalar call return")
		}
		var signature map[string]json.RawMessage
		if err := json.Unmarshal(returns[0]["signature"], &signature); err != nil {
			t.Fatal(err)
		}
		if _, exists := signature["returnPresence"]; exists {
			t.Fatalf("pure scalar call acquired nullable signature: %s", returns[0]["signature"])
		}
	})
}

func TestFrontendCommandNullableReturnBoundaryRejects(t *testing.T) {
	for _, tc := range []struct {
		name, source, annotation string
		officialCode             int32
	}{
		{"nonI32", "export function reject(value: bool | null): bool | null { return value; }", "bool | null", 0},
		{"multiplePayloads", "export function reject(value: i32 | i64 | null): i32 | i64 | null { return value; }", "i32 | i64 | null", 0},
		{"unsafeScalarReturn", "function echo(value: i32 | null): i32 | null { return value; }\nexport function reject(value: i32 | null): i32 { return echo(value); }", "", 2322},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := integerRequest(tc.source)
			if !t.Run("officialDiagnostics", func(t *testing.T) { nullableBoundaryOfficialForTest(t, r, tc.officialCode) }) {
				return
			}
			t.Run("ownedRejection", func(t *testing.T) {
				result, raw := nullableBoundaryCommandForTest(t, r)
				if result.Status != "error" || len(result.Diagnostics) != 1 || len(result.Functions) != 0 {
					t.Fatalf("boundary was admitted or lost its diagnostic: %+v; rawResponse=%s", result, raw)
				}
				d := result.Diagnostics[0]
				if d.File != "/src/main.ts" || d.Primary.SourceID != d.File {
					t.Fatalf("rejection lost original source identity: %+v", d)
				}
				if tc.officialCode != 0 {
					if d.UpstreamCode != tc.officialCode || d.Category != "error" || d.Phase != "types" {
						t.Fatalf("unsafe scalar consumption erased the official error: %+v", d)
					}
				} else if d.Code != "r02b-d-unsupported" || d.Category != "unsupported" || d.Phase != "capability" || d.UpstreamCode != 0 || sourceText(t, tc.source, d.Range) != tc.annotation {
					t.Fatalf("finite owned capability rejection changed: %+v", d)
				}
			})
		})
	}
}
