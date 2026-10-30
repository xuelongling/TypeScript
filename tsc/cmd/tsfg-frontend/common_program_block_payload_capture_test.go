// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

func TestCommonProgramBlockPayloadCapture19ai(t *testing.T) {
	if root := os.Getenv("TSFG_READSPAN_CAPTURE_ROOT"); root != "" {
		t.Setenv("TSFG_READSPAN_CAPTURE_ROOT", root+"/block-payload-capture")
	}
	raw, err := os.ReadFile("testdata/common-program-v2/block-payload-capture/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name, SHA, Failure, Detail string
		Repeated                   bool
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, sample := range cases {
		t.Run(sample.Name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/common-program-v2/block-payload-capture/" + sample.Name + ".ts")
			if err != nil {
				t.Fatal(err)
			}
			if arrayFactsSHA(source) != sample.SHA {
				t.Fatal("frozen complete block payload Source changed")
			}
			readSpanCaptureSave26i(t, sample.Name+"-source.ts", source)
			compare := singleBlockCaptureOfficial19ae(t, string(source), blockCaptureExpectation{"blockEntry", "cell", 1, 2, sample.Repeated, false})
			request := scalarParameterRequest19m(t, source)
			readSpanCaptureSave26i(t, sample.Name+"-request.json", request)
			var output bytes.Buffer
			err = runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(request), &output)
			if err != nil {
				failure, ok := err.(*arrayFactsFailure)
				if !ok || output.Len() != 0 {
					t.Fatalf("non-atomic original failure: %v/%d", err, output.Len())
				}
				raw, encodeErr := json.Marshal(failure)
				if encodeErr != nil {
					t.Fatal(encodeErr)
				}
				readSpanCaptureSave26i(t, sample.Name+"-failure.json", append(raw, '\n'))
				t.Log(string(raw))
				if sample.Failure == "" || failure.Code != sample.Failure || failure.Detail != sample.Detail {
					t.Fatalf("wanted %q/%q, got %s/%s", sample.Failure, sample.Detail, failure.Code, failure.Detail)
				}
				return
			}
			if sample.Failure != "" {
				t.Fatal("unsupported complete block payload Program published")
			}
			readSpanCaptureSave26i(t, sample.Name+"-package.json", output.Bytes())
			var packet commonProgramPackage
			if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}
			compare(packet.Payload)
		})
	}
}

func TestCommonProgramBlockPayloadScopeRejection19ai(t *testing.T) {
	if root := os.Getenv("TSFG_READSPAN_CAPTURE_ROOT"); root != "" {
		t.Setenv("TSFG_READSPAN_CAPTURE_ROOT", root+"/block-payload-scope-rejection")
	}
	for _, sample := range []struct{ name, sha string }{
		{"invisible-sibling", "f6fb5f8955e4497aa406904406f7e7b8e741a8c0f8e93efcdec997f92c09623b"},
		{"wrong-owner", "08f45dcceb9e80d0dace17024a576f861c347fd96c3ed63272d5bddfee3cd9a3"},
	} {
		t.Run(sample.name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/common-program-v2/block-payload-capture/" + sample.name + ".ts")
			if err != nil {
				t.Fatal(err)
			}
			if arrayFactsSHA(source) != sample.sha {
				t.Fatal("frozen complete scope-negative Source changed")
			}
			files := map[string]string{"/src/main.ts": string(source), "/src/minimal-arrays-v8.d.ts": string(arrayGenericDeclarations)}
			program := compiler.NewProgram(compiler.ProgramOptions{Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{FileNames: []string{"/src/main.ts", "/src/minimal-arrays-v8.d.ts"}, CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext}}}, Host: compiler.NewCompilerHost("/src", vfstest.FromMap(files, true), "/src", nil, nil), SingleThreaded: core.TSTrue})
			if len(program.GetSyntacticDiagnostics(t.Context(), nil))+len(program.GetProgramDiagnostics())+len(program.GetGlobalDiagnostics(t.Context())) != 0 {
				t.Fatal("scope-negative original Program has unrelated diagnostics")
			}
			diagnostics := program.GetSemanticDiagnostics(t.Context(), nil)
			if len(diagnostics) != 1 || diagnostics[0].Code() != 2304 || string(source[diagnostics[0].Pos():diagnostics[0].End()]) != "cell" {
				t.Fatalf("official checker did not reject the original invisible cell: %+v", diagnostics)
			}
			t.Log("pre-producer official original scope error 2304 at cell; no foreign-owner binding exists")
			request := scalarParameterRequest19m(t, source)
			readSpanCaptureSave26i(t, sample.name+"-source.ts", source)
			readSpanCaptureSave26i(t, sample.name+"-request.json", request)
			var output bytes.Buffer
			err = runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(request), &output)
			failure, ok := err.(*arrayFactsFailure)
			if !ok || output.Len() != 0 || failure.Code != "r03-array-facts-types" {
				t.Fatalf("original invisible binding did not fail atomically at types: %v/%d", err, output.Len())
			}
			raw, err := json.Marshal(failure)
			if err != nil {
				t.Fatal(err)
			}
			readSpanCaptureSave26i(t, sample.name+"-failure.json", append(raw, '\n'))
		})
	}
}
