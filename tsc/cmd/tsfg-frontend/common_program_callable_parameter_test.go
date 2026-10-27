// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

type callableParameterCase19n struct {
	name, sha, reader string
	bytes, functions  int
	mutation, shadow  bool
}

// These complete programs and independent expectations were frozen before the
// first public producer call. Function types carry no target or lifetime proof.
var callableParameterCases19n = []callableParameterCase19n{
	{name: "callable-readonly", sha: "1f2f84a4f42a741ac9926e8d910fd71a74927da5d5e626f9333046666a89cfc4", bytes: 2327, functions: 18, reader: "relayReader.childForwardReader", mutation: false, shadow: false},
	{name: "callable-reassignment", sha: "37f90cc9d60640864cc8e9ae4ec936575c5647e0d6c193d7f1ce5a98cdb8e040", bytes: 2356, functions: 18, reader: "relayReader.childForwardReader", mutation: true, shadow: false},
	{name: "callable-ancestor-forward", sha: "e89b0c01964dd45ecdb7b863c8f48e8130c5dd73eb7adbf920d8a64e85da8f08", bytes: 2399, functions: 19, reader: "relayReader.forwardLayer.childForwardReader", mutation: false, shadow: false},
	{name: "callable-shadow-local", sha: "605ab1762768a8e5a565d0edd5a6bc1cda2cb8c159e50ac3053ddb6628f3195f", bytes: 2373, functions: 18, reader: "relayReader.childForwardReader", mutation: false, shadow: true},
}

func TestCommonProgramCallableParameter19n(t *testing.T) {
	for _, sample := range callableParameterCases19n {
		t.Run(sample.name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/common-program-v2/callable-parameter/" + sample.name + ".ts")
			if err != nil {
				t.Fatal(err)
			}
			if len(source) != sample.bytes || arrayFactsSHA(source) != sample.sha {
				t.Fatal("frozen complete Source changed")
			}
			original := scalarParameterCase19m{functions: sample.functions, reader: "makeOwnedReader.readOwned"}
			program := scalarParameterOriginal19m(t, string(source), original)
			callableParameterOriginal19n(t, program, sample)
			request := scalarParameterRequest19m(t, source)
			readSpanCaptureSave26i(t, sample.name+"-request.json", request)
			var output bytes.Buffer
			if err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(request), &output); err != nil {
				if output.Len() != 0 {
					t.Fatal("failed producer published partial facts")
				}
				if failure, ok := err.(*arrayFactsFailure); ok {
					b, e := json.Marshal(failure)
					if e != nil {
						t.Fatal(e)
					}
					readSpanCaptureSave26i(t, sample.name+"-failure.json", append(b, '\n'))
					t.Log(string(b))
				}
				t.Fatal(err)
			}
			readSpanCaptureSave26i(t, sample.name+"-package.json", output.Bytes())
			var packet commonProgramPackage
			if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}
			scalarParameterOfficial19m(t, string(source), program, packet.Payload, original)
			callableParameterOfficial19n(t, string(source), program, packet.Payload, sample)
		})
	}
}
