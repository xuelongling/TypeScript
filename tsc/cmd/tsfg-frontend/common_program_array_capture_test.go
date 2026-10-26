// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

type arrayCaptureUse26k struct {
	name  string
	start int
}
type arrayCaptureCase26k struct {
	name, sha, reader   string
	bytes, functions    int
	parameter, writable bool
	uses                []arrayCaptureUse26k
}

// Complete sources and Symbol/UTF16 expectations were independently frozen
// before public projection. Consumer lifetime outcomes never become Source flags.
var arrayCaptureCases26k = []arrayCaptureCase26k{
	{name: "explicit-owner", sha: "74157282e114185c970bb9566014640644ae74a0f8bf3a34e6f98a91b4696235", bytes: 1616, functions: 14, reader: "makeOwnedReader.readOwned", parameter: false, writable: false, uses: []arrayCaptureUse26k{{name: "a", start: 1464}, {name: "r", start: 1471}}},
	{name: "descriptor-only", sha: "f5cfb8423f973a39823b114c722edd70a6a69e36f8e7099ffc4383be6170a4a7", bytes: 1609, functions: 14, reader: "makeOwnedReader.readOwned", parameter: false, writable: false, uses: []arrayCaptureUse26k{{name: "r", start: 1464}}},
	{name: "owner-replaced", sha: "b151db8adbee5878e1b23826986d19d2c343e592ed03bdeacc5dcdc4e6209d9e", bytes: 1625, functions: 14, reader: "makeOwnedReader.readOwned", parameter: false, writable: true, uses: []arrayCaptureUse26k{{name: "a", start: 1462}, {name: "r", start: 1469}}},
	{name: "parameter-owner-index1", sha: "86f8d135f2b57636a267b13073e1d2bcf3b9940b3d1ef088ee6e9c567ab1e073", bytes: 1646, functions: 14, reader: "makeOwnedReader.readOwned", parameter: true, writable: true, uses: []arrayCaptureUse26k{{name: "a", start: 1420}, {name: "r", start: 1427}}},
	{name: "const-owner-content-write", sha: "f731203283bbc6dd557f7464373ed915035b5ac2813a75ea6211cf002d5b6a80", bytes: 1619, functions: 14, reader: "makeOwnedReader.readOwned", parameter: false, writable: false, uses: []arrayCaptureUse26k{{name: "a", start: 1457}, {name: "r", start: 1474}}},
	{name: "parent-owned-environment", sha: "711bf357c72fb59fcce2a9f4928ab0ac6af213105a88aac814bd76d115bdf321", bytes: 1676, functions: 15, reader: "makeOwnedReader.middle.readOwned", parameter: false, writable: false, uses: []arrayCaptureUse26k{{name: "a", start: 1499}, {name: "r", start: 1506}}},
	{name: "function-element-parameter", sha: "f2854e069b9127f5ea47db05e017feb8483a641635dc09e01c1229d50c5526d8", bytes: 1603, functions: 14, reader: "makeOwnedReader.readOwned", parameter: true, writable: true, uses: []arrayCaptureUse26k{{name: "a", start: 1418}}},
}

func TestCommonProgramArrayCapture26k(t *testing.T) {
	for _, sample := range arrayCaptureCases26k {
		t.Run(sample.name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/common-program-v2/array-capture/" + sample.name + ".ts")
			if err != nil {
				t.Fatal(err)
			}
			if len(source) != sample.bytes || arrayFactsSHA(source) != sample.sha || arrayFactsSHA(source[:1318]) != "6a86b0b018c8129985fe7f9bdbd50d9399b5622c3ba83a8466ddbb5e8b12ef92" {
				t.Fatal("frozen complete Source/prefix changed")
			}
			program := commonMethodProgram37q(t, string(source))
			t.Log("independent original Program: syntactic/semantic/program/global diagnostics all zero")
			request := readSpanCaptureRequest26i(t, source)
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
					t.Log(string(b))
					readSpanCaptureSave26i(t, sample.name+"-failure.json", append(b, '\n'))
				}
				t.Fatal(err)
			}
			readSpanCaptureSave26i(t, sample.name+"-package.json", output.Bytes())
			var packet commonProgramPackage
			if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}
			p := packet.Payload
			if len(p.Functions) != sample.functions || len(p.Globals) != 2 || len(p.Modules) != 1 || len(p.Modules[0].Exports) != 7 || len(p.Closure.Roots) != 8 || len(p.Closure.RequiredRoots) != 2 || len(p.EntryCompatibilityRoots) != 6 || len(p.TypeAliases) != 1 {
				t.Fatal("complete original Program inventory changed")
			}
			arrayCaptureOfficial26k(t, string(source), program, p, sample)
		})
	}
}
