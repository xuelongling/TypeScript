// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArrayGenericV8ConsumerRemainingOrderedSources25f(t *testing.T) {
	raw, err := os.ReadFile("testdata/array-facts-v8/consumer-remaining-ordered-inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Name       string                `json:"name"`
			SourceRoot string                `json:"sourceRoot"`
			RootFile   string                `json:"rootFile"`
			Config     arrayFactsConfig      `json:"config"`
			Files      []arrayFactsInputFile `json:"files"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil || len(fixture.Cases) != 5 {
		t.Fatalf("five complete exact remaining/ordered inputs required: %v", err)
	}
	for _, row := range fixture.Cases {
		t.Run(row.Name, func(t *testing.T) {
			if len(row.Files) != 1 || row.Files[0].Path != row.RootFile || row.Files[0].SHA256 != arrayFactsSHA([]byte(row.Files[0].Content)) {
				t.Fatal("original TypeScript content/path/hash changed")
			}
			source := row.Files[0].Content
			dtsPath := row.SourceRoot + "/minimal-arrays-v8.d.ts"
			request := arrayFactsRequest{Family: "tsfg.array-facts-request-candidate", Version: 8,
				Input: arrayFactsInput{SourceRoot: row.SourceRoot, RootFile: row.RootFile, Config: row.Config,
					Declarations: arrayFactsDeclarations{Version: "minimal-arrays-v8", Paths: []string{dtsPath}},
					Files: append(append([]arrayFactsInputFile{}, row.Files...), arrayFactsInputFile{
						Path: dtsPath, Content: string(arrayGenericDeclarations), SHA256: arrayFactsSHA(arrayGenericDeclarations)})}}
			input, err := arrayFactsCanonical(request)
			if err != nil {
				t.Fatal(err)
			}
			input = append(input, '\n')
			// This batch owns its original failed inputs/errors independently of the
			// frozen eighteen-input files. Never write an error as a package.
			capture := func(name string, data []byte) {
				t.Helper()
				if root := os.Getenv("TSFG_ARRAY_V8_CAPTURE_ROOT"); root != "" {
					if err := os.MkdirAll(root, 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(root, row.Files[0].SHA256+"-"+name), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			capture("source.ts", []byte(source))
			capture("request.json", input)
			var output bytes.Buffer
			if err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(input), &output); err != nil {
				var failure *arrayFactsFailure
				errors.As(err, &failure)
				record, encodeErr := json.MarshalIndent(struct {
					Error   string             `json:"error"`
					Failure *arrayFactsFailure `json:"failure"`
				}{err.Error(), failure}, "", "  ")
				if encodeErr != nil {
					t.Fatal(encodeErr)
				}
				capture("error.json", record)
				capture("failed-output.bin", output.Bytes())
				t.Fatalf("actual original public v8 rejection: %s", record)
			}
			genericArrayCapture25f(t, input, source, output.Bytes())
			var packet arrayGenericPackage
			if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}
			payload := packet.Payload
			if packet.Version != 8 || len(payload.Diagnostics) != 0 || payload.SourceRoot != row.SourceRoot || payload.RootFile != row.RootFile || len(payload.Files) != 2 || payload.Declarations.Version != "minimal-arrays-v8" {
				t.Fatal("actual v8 output must retain complete original source and true declarations")
			}
			functions := map[string]arrayGenericFunction{}
			for _, fn := range payload.Functions {
				functions[fn.Name] = fn
			}
			switch row.Name {
			case "global1024", "global1025":
				borrow := functions["borrow"]
				if len(payload.Functions) != 8 || len(borrow.Parameters) != 8 || borrow.ReturnType != "readSpan<f64>" || borrow.Body[0].Initializer.Callee != payload.ReadSpanIntrinsic.ID || borrow.Body[1].Value.Binding != borrow.Body[0].Binding {
					t.Fatal("original complete Array-origin helper and required view parameter slots changed")
				}
				calls, lengths := 0, 0
				for _, fn := range payload.Functions {
					for _, stmt := range fn.Body {
						if stmt.Value != nil && stmt.Value.Kind == "call" && stmt.Value.Callee == borrow.ID {
							calls++
							if len(*stmt.Value.Arguments) != 8 {
								t.Fatal("original full eight-argument call changed")
							}
						}
						if stmt.Value != nil && stmt.Value.Kind == "readSpanLength" {
							lengths++
						}
					}
				}
				wantLengths := 5
				if row.Name == "global1025" {
					wantLengths = 6
				}
				if calls != strings.Count(source, "borrow(a,") || lengths != wantLengths {
					t.Fatal("original global work boundary call/length occurrences changed")
				}
			case "generic-union-parameter-scc":
				even, odd, read := functions["even"], functions["odd"], functions["read"]
				if len(payload.Functions) != 3 || even.Parameters[0].DeclaredType != "array<union<i32,undefined>>" || odd.Parameters[0].DeclaredType != even.Parameters[0].DeclaredType || even.Body[1].Value.Callee != odd.ID || odd.Body[1].Value.Callee != even.ID || read.Body[0].Value.Callee != even.ID || read.ReturnType != "union<i32,undefined>" {
					t.Fatal("actual union parameter-only SCC and external call edges changed")
				}
			case "ordered-carrier-refinement":
				body := functions["run"].Body
				carrier := body[1].Initializer
				if len(body) != 3 || carrier.Kind != "arrayLiteral" || len(*carrier.Elements) != 1 || (*carrier.Elements)[0].Kind != "checkedElementAccess" || (*carrier.Elements)[0].Receiver.Binding != body[0].Binding || (*carrier.Elements)[0].Index.Decimal != "1" || body[2].Value.Binding != body[1].Binding {
					t.Fatal("carrier must retain its original ordered checked array read and returned binding")
				}
			case "short-circuit-index-binding":
				fn := functions["run"]
				if len(fn.Body) != 4 || fn.Parameters[0].DeclaredType != "bool" || fn.Body[2].Value.Kind != "binary" || fn.Body[2].Value.Operator != "&&" || fn.Body[3].Value.Index.Binding != fn.Body[1].Binding {
					t.Fatal("short-circuit assignment and later original index binding must remain explicit")
				}
			default:
				t.Fatal("unknown exact input")
			}
		})
	}
}
