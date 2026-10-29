// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestCommonProgramScalarUnionCapture19z(t *testing.T) {
	raw, err := os.ReadFile("testdata/common-program-v2/scalar-union-capture/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ Name, SHA, Failure, Detail string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, sample := range cases {
		t.Run(sample.Name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/common-program-v2/scalar-union-capture/" + sample.Name + ".ts")
			if err != nil {
				t.Fatal(err)
			}
			if arrayFactsSHA(source) != sample.SHA {
				t.Fatal("frozen complete scalar union Source changed")
			}
			readSpanCaptureSave26i(t, sample.Name+"-source.ts", source)
			compare := unionCaptureOfficial19x(t, string(source), sample.Failure == "")
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
				if sample.Failure == "" || failure.Code != sample.Failure || failure.Detail != sample.Detail {
					t.Fatal(string(raw))
				}
				return
			}
			if sample.Failure != "" {
				t.Fatal("unsupported complete scalar union Program published")
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
