// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// The existing independent official oracle checks the complete original Program
// before the public facts producer. Presence does not replace scalar payloads.
func TestCommonProgramScalarPresenceCapture19ac(t *testing.T) {
	if root := os.Getenv("TSFG_READSPAN_CAPTURE_ROOT"); root != "" {
		t.Setenv("TSFG_READSPAN_CAPTURE_ROOT", root+"/scalar-presence-capture")
	}
	raw, err := os.ReadFile("testdata/common-program-v2/scalar-presence-capture/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name, Source, SHA, Failure, Detail string
		Members                            []string
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, sample := range cases {
		t.Run(sample.Name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/common-program-v2/" + sample.Source)
			if err != nil {
				t.Fatal(err)
			}
			if arrayFactsSHA(source) != sample.SHA {
				t.Fatal("frozen complete scalar presence Source changed")
			}
			readSpanCaptureSave26i(t, sample.Name+"-source.ts", source)
			var compare func(commonProgramPayload)
			if sample.Name == "block-local" {
				compare = singleBlockCaptureOfficial19ae(t, string(source), blockCaptureExpectation{"makeOwnedReader", "held", 1, 1, false, true})
			} else {
				compare = unionCaptureOfficial19x(t, string(source), sample.Failure == "")
			}
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
				t.Fatal("unsupported complete scalar presence Program published")
			}
			readSpanCaptureSave26i(t, sample.Name+"-package.json", output.Bytes())
			var packet commonProgramPackage
			if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}
			compare(packet.Payload)
			found := false
			for _, typ := range packet.Payload.Types {
				found = found || typ.Kind == "union" && reflect.DeepEqual(typ.Members, sample.Members)
			}
			if !found {
				t.Fatal("complete original scalar and presence member identities missing")
			}
		})
	}
}
