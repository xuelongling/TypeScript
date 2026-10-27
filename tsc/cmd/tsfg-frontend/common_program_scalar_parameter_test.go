// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	tsjson "github.com/microsoft/typescript-go/internal/json"
	"os"
	"sort"
	"testing"
)

type scalarParameterCase19m struct {
	name, sha, reader string
	bytes, functions  int
	mutation, shadow  bool
}

// Complete sources were frozen independently before public projection. Numeric
// language oracles are external and never passed to this facts producer.
var scalarParameterCases19m = []scalarParameterCase19m{
	{name: "direct-readonly", sha: "cbf8ceeccae0d1c494e888d1c590dc537b52dce22274e7f621adf25fc96bebbc", bytes: 2172, functions: 16, reader: "makeOwnedReader.readOwned", mutation: false, shadow: false},
	{name: "shared-mutation", sha: "b747f01bc7ec5a74230dfcbd835a6e5cab3bca814dec4053cb0d231b00012930", bytes: 2198, functions: 16, reader: "makeOwnedReader.readOwned", mutation: true, shadow: false},
	{name: "ancestor-forward", sha: "89f5c9bd4aec5bccaccbedad48c34bbb87c3291699570b8a70885ed171bb48ee", bytes: 2246, functions: 17, reader: "makeOwnedReader.forwardReader.readOwned", mutation: false, shadow: false},
	{name: "shadow-local", sha: "20013ffca0a2bde9b939e95ff268a53f06bf86750d121e5e5a04c3194c7773ce", bytes: 2193, functions: 16, reader: "makeOwnedReader.readOwned", mutation: false, shadow: true},
}

func TestCommonProgramScalarParameter19m(t *testing.T) {
	for _, sample := range scalarParameterCases19m {
		t.Run(sample.name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/common-program-v2/scalar-parameter/" + sample.name + ".ts")
			if err != nil {
				t.Fatal(err)
			}
			if len(source) != sample.bytes || arrayFactsSHA(source) != sample.sha {
				t.Fatal("frozen complete Source changed")
			}
			program := scalarParameterOriginal19m(t, string(source), sample)
			request := scalarParameterRequest19m(t, source)
			readSpanCaptureSave26i(t, sample.name+"-request.json", request)
			var output bytes.Buffer
			if err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(request), &output); err != nil {
				if output.Len() != 0 {
					t.Fatal("failed producer published partial facts")
				}
				if f, ok := err.(*arrayFactsFailure); ok {
					b, e := json.Marshal(f)
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
			scalarParameterOfficial19m(t, string(source), program, packet.Payload, sample)
		})
	}
}

func scalarParameterRequest19m(t *testing.T, source []byte) []byte {
	t.Helper()
	var request commonProgramRequest
	if err := json.Unmarshal(readSpanCaptureRequest26i(t, source), &request); err != nil {
		t.Fatal(err)
	}
	closure := request.Input.CompilationClosure
	closure.Roots = append(closure.Roots, closureRoot{Kind: "application", Name: "loopRead", SourcePath: "/src/main.ts"})
	roots := closure.Roots
	sort.Slice(roots, func(i, j int) bool {
		a, b := roots[i], roots[j]
		return a.Kind+"\x00"+a.SourcePath+"\x00"+a.Name < b.Kind+"\x00"+b.SourcePath+"\x00"+b.Name
	})
	b, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	canonical := tsjson.Value(b)
	if err := canonical.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	return append(canonical, '\n')
}

// Scalar incoming initialization does not make reference/union parameters legal
// closure captures. These complete Programs remain checker-valid.
func TestCommonProgramScalarParameterBoundary19m(t *testing.T) {
	for _, sample := range []struct {
		name, sha string
		bytes     int
	}{
		{name: "reference-parameter", sha: "9f19e8d23ec70b7db461c5011ae3c1fdf16de473018f5f7ebdf4983733a6ef6e", bytes: 2303},
		{name: "union-parameter", sha: "57825f630aa81016c1af240571904826f5607f07c90b7dfb782ed3167d74ca37", bytes: 2317},
	} {
		t.Run(sample.name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/common-program-v2/scalar-parameter/" + sample.name + ".ts")
			if err != nil {
				t.Fatal(err)
			}
			if len(source) != sample.bytes || arrayFactsSHA(source) != sample.sha {
				t.Fatal("frozen complete boundary source changed")
			}
			commonMethodProgram37q(t, string(source))
			request := scalarParameterRequest19m(t, source)
			readSpanCaptureSave26i(t, sample.name+"-request.json", request)
			var output bytes.Buffer
			err = runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(request), &output)
			failure, ok := err.(*arrayFactsFailure)
			if !ok || failure.Code != "r03-array-facts-capability" || output.Len() != 0 {
				t.Fatalf("non-scalar parameter must remain unsupported atomically: %v/%d", err, output.Len())
			}
			b, e := json.Marshal(failure)
			if e != nil {
				t.Fatal(e)
			}
			readSpanCaptureSave26i(t, sample.name+"-failure.json", append(b, '\n'))
			t.Log(string(b))
		})
	}
}
