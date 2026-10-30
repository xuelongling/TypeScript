// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	tsjson "github.com/microsoft/typescript-go/internal/json"
)

func TestCommonProgramVarDeclarationForms19am(t *testing.T) {
	if root := os.Getenv("TSFG_READSPAN_CAPTURE_ROOT"); root != "" {
		t.Setenv("TSFG_READSPAN_CAPTURE_ROOT", root+"/var-forms")
	}
	manifest, err := os.ReadFile("testdata/common-program-v2/var-binding/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ Name, SHA, Failure, Detail string }
	if err := json.Unmarshal(manifest, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 7 {
		t.Fatal("var form and type/lifetime boundary inventory changed")
	}
	for _, sample := range cases {
		t.Run(sample.Name, func(t *testing.T) {
			name := sample.Name
			source, err := os.ReadFile("testdata/common-program-v2/var-binding/" + name + ".ts")
			if err != nil {
				t.Fatal(err)
			}
			if arrayFactsSHA(source) != sample.SHA {
				t.Fatal("frozen complete var-form Source changed")
			}
			readSpanCaptureSave26i(t, name+"-source.ts", source)
			compare := varBindingOfficial19am(t, string(source), "varBinding", "x")
			var request commonProgramRequest
			if err := json.Unmarshal(genericArrayRequest25f(t, string(source)), &request); err != nil {
				t.Fatal(err)
			}
			request.Family, request.Version, request.SchemaHash = "tsfg.program-facts-request-candidate", 2, "sha256:"+arrayFactsSHA(commonProgramSchema)
			request.Input.HostImports = []commonProgramHostImport{}
			request.Input.Target = &commonProgramTarget{Target: "windows-x86_64-msvc", Triple: "x86_64-pc-windows-msvc", PointerBits: "64", LLVMDataLayout: "e-m:w-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128"}
			root := closureRoot{Kind: "application", SourcePath: "/src/main.ts", Name: "varBinding"}
			request.Input.CompilationClosure = &closureConfiguration{Roots: []closureRoot{root, {Kind: "module-initialization", SourcePath: "/src/main.ts", Name: "$moduleInit"}}, RequiredRoots: []closureRoot{root}}
			raw, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			canonical := tsjson.Value(raw)
			if err := canonical.Canonicalize(); err != nil {
				t.Fatal(err)
			}
			raw = append(canonical, '\n')
			readSpanCaptureSave26i(t, name+"-request.json", raw)
			var output bytes.Buffer
			if err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(raw), &output); err != nil {
				if output.Len() != 0 {
					t.Fatal("failed var producer published partial facts")
				}
				if failure, ok := err.(*arrayFactsFailure); ok {
					encoded, encodeErr := json.Marshal(failure)
					if encodeErr != nil {
						t.Fatal(encodeErr)
					}
					readSpanCaptureSave26i(t, name+"-failure.json", append(encoded, '\n'))
					if sample.Failure != "" && failure.Code == sample.Failure && failure.Detail == sample.Detail {
						return
					}
				}
				t.Fatal(err)
			}
			if sample.Failure != "" {
				t.Fatal("conflicting original logical var types were published")
			}
			readSpanCaptureSave26i(t, name+"-package.json", output.Bytes())
			var packet commonProgramPackage
			if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}
			compare(packet.Payload)
		})
	}
}
