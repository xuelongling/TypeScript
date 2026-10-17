// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

// Explicitly authorized small contract case: each original source stays <2KiB.
// It is not a scalability or stress fixture; no other large limit is generated.
func TestExceptionFactsCommandV1ParameterLimit64(t *testing.T) {
	for _, count := range []int{64, 65} {
		t.Run(fmt.Sprintf("parameters%d", count), func(t *testing.T) {
			names := make([]string, count)
			for i := range names {
				names[i] = fmt.Sprintf("p%02d: i32", i)
			}
			source := "export function entry(" + strings.Join(names, ", ") + "): i32 { throw p00; }"
			if len(source) >= 2048 {
				t.Fatal("small-source authorization exceeded")
			}
			r := integerRequest(source)
			program := throwBoundaryProgramForTest(t, r)
			var facts, exports any
			if !t.Run("freshOfficialDiagnosticsAndCompleteParameterOracle", func(t *testing.T) {
				throwBoundaryDiagnosticsForTest(t, program)
				file := program.GetSourceFile(r.RootFile)
				if file == nil || file.Text() != source || len(file.Statements.Nodes) != 1 || file.Statements.Nodes[0].Kind != ast.KindFunctionDeclaration {
					t.Fatal("original parameter function changed")
				}
				node := file.Statements.Nodes[0]
				fn := node.AsFunctionDeclaration()
				if len(fn.Parameters.Nodes) != count || fn.Body == nil || len(fn.Body.AsBlock().Statements.Nodes) != 1 {
					t.Fatal("complete original parameter/body count differs")
				}
				thrown := fn.Body.AsBlock().Statements.Nodes[0]
				if thrown.Kind != ast.KindThrowStatement || thrown.AsThrowStatement().Expression == nil {
					t.Fatal("one original throw required")
				}
				value := thrown.AsThrowStatement().Expression
				check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
				defer release()
				functionSymbol := exceptionTraversalIdentityForTest(t, check, node)
				functionRange := throwBoundaryNodeForTest(t, file, node)
				nameRange := throwBoundaryNodeForTest(t, file, node.Name())
				i32 := check.GetTypeFromTypeNode(fn.Type)
				if i32 == nil || i32.Alias() == nil || i32.Alias().Symbol() == nil || i32.Alias().Symbol().Name != "i32" {
					t.Fatal("actual nominal i32 result missing")
				}
				parameters, symbols, initializers := []any{}, []any{map[string]any{"id": "s1", "name": "entry", "kind": "function", "type": "i32", "writable": false, "range": nameRange}}, []any{}
				for i, parameter := range fn.Parameters.Nodes {
					p := parameter.AsParameterDeclaration()
					if parameter.Name() == nil || parameter.Name().Text() != fmt.Sprintf("p%02d", i) || p.Type == nil || p.QuestionToken != nil || p.Initializer != nil || p.DotDotDotToken != nil {
						t.Fatal("required i32 parameter syntax changed")
					}
					actual := exceptionTraversalIdentityForTest(t, check, parameter)
					if check.GetTypeOfSymbol(actual) != i32 || check.GetTypeFromTypeNode(p.Type) != i32 {
						t.Fatal("actual i32 parameter type changed")
					}
					if i == 0 && (check.GetSymbolAtLocation(value) != actual || check.GetTypeAtLocation(value) != i32) {
						t.Fatal("single payload must be the actual first parameter")
					}
					start := p.Type.End() - len("i32")
					if source[start:p.Type.End()] != "i32" {
						t.Fatal("original annotation preimage changed")
					}
					binding := map[string]any{"id": fmt.Sprintf("s%d", i+2), "name": parameter.Name().Text(), "kind": "parameter", "type": "i32", "writable": true, "range": throwBoundaryNodeForTest(t, file, parameter.Name()), "presence": map[string]any{"undefined": false, "null": false, "optional": false, "default": false, "range": throwBoundaryNodeForTest(t, file, parameter), "annotation": throwBoundarySpanForTest(source, start, p.Type.End())}}
					parameters = append(parameters, binding)
					symbols = append(symbols, binding)
					initializers = append(initializers, nil)
				}
				signatures := check.GetSignaturesOfType(check.GetTypeOfSymbol(functionSymbol), checker.SignatureKindCall)
				if len(signatures) != 1 || len(signatures[0].Parameters()) != count || signatures[0].Declaration() != node || check.GetReturnTypeOfSignature(signatures[0]) != i32 {
					t.Fatal("fresh complete callable signature changed")
				}
				actualExports := check.GetExportsOfModule(file.AsNode().Symbol())
				if len(actualExports) != 1 || actualExports[0] != functionSymbol || len(actualExports[0].Declarations) != 1 || actualExports[0].Declarations[0] != node {
					t.Fatal("fresh complete direct export identity changed")
				}
				facts = map[string]any{"status": "ok", "inputs": []string{"/src/main.ts", "/src/minimal-integers-v3.d.ts"}, "symbols": symbols, "functions": []any{map[string]any{"name": "entry", "symbol": "s1", "parameters": parameters, "returnType": "i32", "parameterInitializers": initializers, "range": functionRange, "body": []any{map[string]any{"kind": "throw", "range": throwBoundaryNodeForTest(t, file, thrown), "expression": map[string]any{"kind": "reference", "type": "i32", "symbol": "s2", "range": throwBoundaryNodeForTest(t, file, value)}}}}}}
				exports = []any{map[string]any{"sourcePath": r.RootFile, "external": true, "bindings": []any{map[string]any{"name": "entry", "target": "s1", "sourcePath": r.RootFile, "range": functionRange, "definition": nameRange}}}}
			}) {
				return
			}
			// Both directions first prove complete public projection independently.
			input, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			var public bytes.Buffer
			if err := runFrontendCommand(t.Context(), nil, bytes.NewReader(input), &public); err != nil {
				t.Fatal(err)
			}
			var publicValue any
			if err := json.Unmarshal(public.Bytes(), &publicValue); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(nullableFactsBoundaryCanonical(t, publicValue), nullableFactsBoundaryCanonical(t, facts)) {
				t.Fatalf("earlier public projection differs; not private-limit evidence: %s", public.Bytes())
			}
			_, requestBytes, expected := exceptionBoundaryPacketForTest(t, r, facts, exports)
			if count == 65 {
				exceptionBoundaryRejectForTest(t, requestBytes, "limit", "65 required parameters exceed descriptor maximum64 after fresh official/public controls")
				return
			}
			output := &exceptionBoundaryWriterForTest{limit: -1}
			err = runFrontendCommand(t.Context(), []string{"--exception-facts-io-v1"}, bytes.NewReader(requestBytes), output)
			if err != nil || output.calls != 1 || !bytes.Equal(output.accepted, expected) {
				t.Fatalf("64-parameter independent complete positive control failed: err=%v writes=%d", err, output.calls)
			}
		})
	}
}
