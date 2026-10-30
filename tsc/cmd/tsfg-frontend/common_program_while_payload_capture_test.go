// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
)

func TestCommonProgramWhileCapturePayloads19ak(t *testing.T) {
	if root := os.Getenv("TSFG_READSPAN_CAPTURE_ROOT"); root != "" {
		t.Setenv("TSFG_READSPAN_CAPTURE_ROOT", root+"/while-payload-capture")
	}
	raw, err := os.ReadFile("testdata/common-program-v2/while-capture/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name, SHA, Failure, Detail string
		Repeated, Scalar           bool
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 31 {
		t.Fatal("complete while payload and boundary inventory changed")
	}
	for _, sample := range cases {
		t.Run(sample.Name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/common-program-v2/while-capture/" + sample.Name + ".ts")
			if err != nil {
				t.Fatal(err)
			}
			if arrayFactsSHA(source) != sample.SHA {
				t.Fatal("frozen complete while capture Source changed")
			}
			readSpanCaptureSave26i(t, sample.Name+"-source.ts", source)
			var compare func(commonProgramPayload)
			if sample.Name == "var-binding" {
				compare = varBindingOfficial19am(t, string(source), "blockEntry", "cell")
				program := commonMethodProgram37q(t, string(source))
				file := program.GetSourceFile("/src/main.ts")
				check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
				t.Cleanup(release)
				found := 0
				var walk func(*ast.Node)
				walk = func(node *ast.Node) {
					if node.Kind == ast.KindVariableDeclaration && node.Name().Text() == "cell" {
						found++
						original := check.GetSymbolAtLocation(node.Name())
						if node.Parent.Flags&ast.NodeFlagsBlockScoped != 0 || original == nil || original.Flags&ast.SymbolFlagsFunctionScopedVariable == 0 || len(original.Declarations) != 1 || original.Declarations[0] != node {
							t.Fatal("case must retain the original function-scoped var Symbol")
						}
					}
					node.ForEachChild(func(child *ast.Node) bool { walk(child); return false })
				}
				walk(file.AsNode())
				if found != 1 {
					t.Fatal("original unique var declaration absent")
				}
			} else {
				compare = singleBlockCaptureOfficial19ae(t, string(source), blockCaptureExpectation{"blockEntry", "cell", 1, 2, sample.Repeated, sample.Scalar})
			}
			request := scalarParameterRequest19m(t, source)
			readSpanCaptureSave26i(t, sample.Name+"-request.json", request)
			var output bytes.Buffer
			err = runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(request), &output)
			if err != nil {
				failure, ok := err.(*arrayFactsFailure)
				if !ok || output.Len() != 0 {
					t.Fatalf("non-atomic original while capture failure: %v/%d", err, output.Len())
				}
				raw, encodeErr := json.Marshal(failure)
				if encodeErr != nil {
					t.Fatal(encodeErr)
				}
				readSpanCaptureSave26i(t, sample.Name+"-failure.json", append(raw, '\n'))
				if sample.Failure == "" || failure.Code != sample.Failure || failure.Detail != sample.Detail {
					t.Fatalf("wanted %q/%q, got %s/%s", sample.Failure, sample.Detail, failure.Code, failure.Detail)
				}
				return
			}
			if sample.Failure != "" {
				t.Fatal("unsupported complete while capture Program published")
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
