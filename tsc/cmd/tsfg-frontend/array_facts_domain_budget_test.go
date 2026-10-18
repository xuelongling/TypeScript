// SPDX-License-Identifier: Apache-2.0
// Independent domain templates preserve official diagnostics before private admission.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

type arrayDomainPlan25 struct {
	name    string
	lengths [][]int
	accept  bool
}

func arrayDomainRows25(n, size int) [][]int {
	rows := make([][]int, n)
	for i := range rows {
		rows[i] = []int{size}
	}
	return rows
}
func arrayDomainPlans25() []arrayDomainPlan25 {
	consts := func(n int) [][]int {
		row := make([]int, n)
		for i := range row {
			row[i] = 1
		}
		return [][]int{row}
	}
	totalOver := arrayDomainRows25(16, 256)
	totalOver[0] = []int{256, 1}
	return []arrayDomainPlan25{
		{"functions16", arrayDomainRows25(16, 1), true}, {"functions17", arrayDomainRows25(17, 1), false},
		{"consts17", consts(17), true}, {"consts18", consts(18), false},
		{"literal256", [][]int{{256}}, true}, {"literal257", [][]int{{257}}, false},
		{"package4096", arrayDomainRows25(16, 256), true}, {"package4097", totalOver, false},
	}
}
func arrayDomainRange25(start, end int) map[string]any {
	return map[string]any{"start": start, "end": end}
}
func arrayDomainID25(t *testing.T, domain string, value any) string {
	t.Helper()
	return "sha256:" + arrayFirstHash25(append([]byte(domain+"\x00"), arrayFirstCanonical25(t, value)...))
}

// This template is authored directly. It consumes no producer AST/projector or
// float conversion; all elements are token 0 with independently fixed +0 bits.
// Frozen four-case type origins supply the unchanged DTS/config semantic basis.
func arrayDomainPacket25(t *testing.T, lengths [][]int) (string, []byte, []byte) {
	t.Helper()
	var packet map[string]any
	if err := json.Unmarshal([]byte(arrayFirstFixture25(t)[3].PackageText), &packet); err != nil {
		t.Fatal(err)
	}
	payload := packet["payload"].(map[string]any)
	path := payload["rootFile"].(string)
	source := ""
	functions := []any{}
	exports := []any{}
	bindingID := func(where any) string {
		return arrayDomainID25(t, "tsfg.array-facts.binding.v1", map[string]any{"sourcePath": path, "nameRange": where})
	}
	for f, counts := range lengths {
		start := len(source)
		name := fmt.Sprintf("fn%02d", f)
		source += "export function "
		nameRange := arrayDomainRange25(len(source), len(source)+len(name))
		source += name + "(): "
		annotation := arrayDomainRange25(len(source), len(source)+3)
		source += "f64 {\n"
		bindings := []any{}
		body := []any{}
		lastBinding, lastName := "", ""
		for c, count := range counts {
			local := fmt.Sprintf("a%02d", c)
			statementStart := len(source) + 2
			source += "  const "
			localRange := arrayDomainRange25(len(source), len(source)+len(local))
			declarationStart := len(source)
			source += local + ": "
			elementAnnotation := arrayDomainRange25(len(source), len(source)+3)
			arrayAnnotation := arrayDomainRange25(len(source), len(source)+5)
			source += "f64[] = "
			literalStart := len(source)
			source += "["
			elements := []any{}
			for e := 0; e < count; e++ {
				if e > 0 {
					source += ","
				}
				at := len(source)
				source += "0"
				elements = append(elements, map[string]any{"kind": "numericLiteral", "type": "f64", "range": arrayDomainRange25(at, at+1), "token": "0", "bits": "0000000000000000"})
			}
			source += "]"
			literalRange := arrayDomainRange25(literalStart, len(source))
			declarationRange := arrayDomainRange25(declarationStart, len(source))
			binding := bindingID(localRange)
			source += ";"
			body = append(body, map[string]any{"kind": "variable", "range": arrayDomainRange25(statementStart, len(source)), "binding": binding,
				"initializer": map[string]any{"kind": "arrayLiteral", "type": "array.f64", "range": literalRange,
					"allocationSite": arrayDomainID25(t, "tsfg.array-facts.allocation-site.v1", map[string]any{"sourcePath": path, "range": literalRange}), "elements": elements}})
			source += "\n"
			bindings = append(bindings, map[string]any{"id": binding, "name": local, "kind": "const", "declaredType": "array.f64", "writable": false,
				"declarationRange": declarationRange, "nameRange": localRange, "annotationRange": arrayAnnotation, "elementAnnotationRange": elementAnnotation})
			lastBinding, lastName = binding, local
		}
		returnStart := len(source) + 2
		source += "  return "
		readStart := len(source)
		referenceRange := arrayDomainRange25(len(source), len(source)+len(lastName))
		source += lastName + "["
		indexRange := arrayDomainRange25(len(source), len(source)+1)
		source += "0]"
		readEnd := len(source)
		source += ";"
		body = append(body, map[string]any{"kind": "return", "range": arrayDomainRange25(returnStart, len(source)), "value": map[string]any{
			"kind": "checkedElementAccess", "type": "f64", "range": arrayDomainRange25(readStart, readEnd),
			"array": map[string]any{"kind": "reference", "binding": lastBinding, "flowType": "array.f64", "range": referenceRange},
			"index": map[string]any{"kind": "indexLiteral", "type": "u32", "range": indexRange, "token": "0", "decimal": "0"}}})
		source += "\n}"
		id := bindingID(nameRange)
		functions = append(functions, map[string]any{"id": id, "name": name, "sourcePath": path, "declarationRange": arrayDomainRange25(start, len(source)), "nameRange": nameRange,
			"returnType": "f64", "returnAnnotationRange": annotation, "parameters": []any{}, "bindings": bindings, "body": body})
		exports = append(exports, map[string]any{"name": name, "functionId": id})
		source += "\n"
	}
	files := payload["files"].([]any)
	requestFiles := []any{}
	for _, entry := range files {
		file := entry.(map[string]any)
		if file["path"] == path {
			file["content"] = source
			file["sha256"] = arrayFirstHash25([]byte(source))
			file["byteLength"] = len(source)
			file["utf16Length"] = len(source)
		}
		requestFiles = append(requestFiles, map[string]any{"path": file["path"], "content": file["content"], "sha256": file["sha256"]})
	}
	payload["functions"], payload["exports"] = functions, exports
	request := map[string]any{"family": "tsfg.array-facts-request-candidate", "version": 1, "input": map[string]any{
		"sourceRoot": payload["sourceRoot"], "rootFile": path, "files": requestFiles, "declarations": payload["declarations"], "config": payload["config"]}}
	requestText := arrayBoundaryWire25(t, request)
	packet["requestId"] = "sha256:" + arrayFirstHash25(append([]byte("tsfg.array-facts.request.v1\x00"), requestText...))
	packet["packageId"] = "sha256:" + arrayFirstHash25(append([]byte("tsfg.array-facts.package.v1\x00"+packet["schemaHash"].(string)+"\x00"), arrayFirstCanonical25(t, payload)...))
	return source, requestText, arrayBoundaryWire25(t, packet)
}

func arrayDomainOfficialZero25(t *testing.T, input []byte) {
	t.Helper()
	var envelope struct{ Input request }
	if err := json.Unmarshal(input, &envelope); err != nil {
		t.Fatal(err)
	}
	r := envelope.Input
	contents := map[string]string{}
	paths := []string{}
	for _, file := range r.Files {
		contents[file.Path] = file.Content
		paths = append(paths, file.Path)
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
		{"syntax", program.GetSyntacticDiagnostics(t.Context(), nil)}, {"semantic", program.GetSemanticDiagnostics(t.Context(), nil)},
		{"program", program.GetProgramDiagnostics()}, {"global", program.GetGlobalDiagnostics(t.Context())},
	} {
		if len(group.items) != 0 {
			t.Fatalf("complete official %s diagnostics must be zero: %+v", group.name, group.items)
		}
	}
	consumed := []string{}
	for _, file := range program.GetSourceFiles() {
		consumed = append(consumed, file.FileName())
		if contents[file.FileName()] != file.Text() {
			t.Fatal("official source text differs")
		}
	}
	slices.Sort(consumed)
	if !slices.Equal(paths, consumed) {
		t.Fatal("official complete two-file closure")
	}
	t.Log("TSFG_ARRAY_DOMAIN_OFFICIAL diagnostics=4x0 closure=2")
}

func TestArrayFactsDomainBudgets25(t *testing.T) {
	for _, plan := range arrayDomainPlans25() {
		t.Run(plan.name, func(t *testing.T) {
			source, input, expected := arrayDomainPacket25(t, plan.lengths)
			if len(source) > 65536 || len(input) > 2097152 || len(expected) > 2097152 {
				t.Fatal("template crossed unrelated byte budget")
			}
			arrayDomainOfficialZero25(t, input)
			var output arrayBoundaryWriter25
			err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v1"}, bytes.NewReader(input), &output)
			if plan.accept {
				if err != nil || output.calls != 1 {
					t.Fatalf("legal domain boundary: err=%v writes=%d", err, output.calls)
				}
				if !bytes.Equal(output.Bytes(), expected) {
					t.Fatalf("whole independent template mismatch actual=%s expected=%s", output.Bytes(), expected)
				}
				t.Logf("TSFG_ARRAY_DOMAIN_WHOLE_PASS name=%s bytes=%d sha256=%s", plan.name, output.Len(), arrayFirstHash25(output.Bytes()))
			} else {
				owned := arrayBoundaryFailure25(t, err)
				if owned.Code != "r03-array-facts-limit" || owned.RuleID != "r01-input-boundary" || owned.Phase != "input" || output.calls != 0 || output.Len() != 0 {
					t.Fatalf("domain limit/zero publication: %+v writes=%d", owned, output.calls)
				}
			}
		})
	}
}
func TestArrayFactsDomainSyntaxAdmission25(t *testing.T) {
	for _, tc := range []struct{ name, annotation string }{{"bareNumber", "number[]"}, {"implicitFresh", ""}} {
		t.Run(tc.name, func(t *testing.T) {
			declaration := "const values"
			if tc.annotation != "" {
				declaration += ": " + tc.annotation
			}
			source := "export function boundary(): f64 { " + declaration + " = [0]; return values[0]; }"
			input := arrayBoundarySource25(t, source)
			arrayDomainOfficialZero25(t, input)
			_, owned := arrayBoundaryCall25(t, input)
			if owned == nil || owned.Code != "r03-array-facts-capability" || owned.RuleID != "r01-ts-baseline" || owned.Phase != "capability" || len(owned.Diagnostics) == 0 {
				t.Fatalf("official-valid private syntax exclusion: %+v", owned)
			}
		})
	}
}
func TestArrayFactsDomainRawJSONBudgets25(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input []byte
	}{
		{"depth65", []byte(strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65) + "\n")},
		{"tokens200001", []byte("[" + strings.Repeat("0,", 200000) + "0]\n")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, owned := arrayBoundaryCall25(t, tc.input)
			if owned == nil || owned.Code != "r03-array-facts-limit" || owned.RuleID != "r01-input-boundary" || owned.Phase != "input" {
				t.Fatalf("generic budget before schema: %+v", owned)
			}
		})
	}
}
