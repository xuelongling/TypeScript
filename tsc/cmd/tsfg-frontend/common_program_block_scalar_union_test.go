// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestCommonProgramBlockScalarUnion19ag(t *testing.T) {
	if root := os.Getenv("TSFG_READSPAN_CAPTURE_ROOT"); root != "" {
		t.Setenv("TSFG_READSPAN_CAPTURE_ROOT", root+"/block-scalar-union")
	}
	raw, err := os.ReadFile("testdata/common-program-v2/block-scalar-union/cases.json")
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
	for _, sample := range cases {
		t.Run(sample.Name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/common-program-v2/block-scalar-union/" + sample.Name + ".ts")
			if err != nil {
				t.Fatal(err)
			}
			if arrayFactsSHA(source) != sample.SHA {
				t.Fatal("frozen complete block scalar union Source changed")
			}
			readSpanCaptureSave26i(t, sample.Name+"-source.ts", source)
			compare := singleBlockCaptureOfficial19ae(t, string(source), blockCaptureExpectation{"blockEntry", "cell", 1, 2, sample.Repeated, sample.Scalar})
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
				t.Fatal("unsupported complete block scalar union Program published")
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
