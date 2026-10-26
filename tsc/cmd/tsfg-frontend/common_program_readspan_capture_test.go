// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	tsjson "github.com/microsoft/typescript-go/internal/json"
)

type readSpanCaptureCase26i struct {
	name, sha                    string
	bytes, functions, occurrence int
	parameter                    bool
	parameterIndex               int
}

// These byte identities and UTF16 positions were frozen from complete source
// before the first producer run. Consumer lifetime outcomes are not Source facts.
var readSpanCaptureCases26i = []readSpanCaptureCase26i{
	{"safe-lastuse", "ec1d0df096efbd583aa9d8adf7475356242f2d707c6a48a406e5adb20e292b7d", 1587, 13, 1471, false, 0},
	{"later-use-conflict", "10ae0105864dcd4d720c40f11fc5fb297c87eb24cfd888c73b48349980b64908", 1598, 13, 1471, false, 0},
	{"never-called", "8df6135e8107241e23f0b007acbd6beba735a4fe99f2f58aae5d3370d73a3f43", 1552, 13, 1471, false, 0},
	{"mutable-safe", "93aa22d31d682a067b9e97031fcce4e8c8add57a87c2728062121f8dede79828", 1645, 13, 1493, false, 0},
	{"mutable-conflict", "c9a4451142a44d4aed0091582c0e221389242fc91ff4cfb491c0404dc4a8b502", 1606, 13, 1493, false, 0},
	{"parameter-safe", "26cd34b26f198ae6bbde81d7bf80532969896e05f9023cdd715dcdae2cd4ac65", 1644, 14, 1409, true, 0},
	{"parameter-conflict", "a03ab29ba21bb19a52b1c5d3cc43c1c4022abd9b7462491afd2ce777721b680d", 1637, 14, 1423, true, 0},
	{"parameter-index1-conflict", "0e09e7de4d4969f5fb5c2dd38c1c624c93a86fc2e80983ff22d79f7bf326ee5e", 1637, 14, 1423, true, 1},
}

func readSpanCaptureRequest26i(t *testing.T, source []byte) []byte {
	t.Helper()
	var request map[string]any
	if err := json.Unmarshal(commonIndirectRequest37s(t, source), &request); err != nil {
		t.Fatal(err)
	}
	closure, err := os.ReadFile("testdata/common-program-v2/readspan-capture/closure.json")
	if err != nil {
		t.Fatal(err)
	}
	var roots any
	if err := json.Unmarshal(closure, &roots); err != nil {
		t.Fatal(err)
	}
	request["input"].(map[string]any)["compilationClosure"] = roots
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

func readSpanCaptureSave26i(t *testing.T, name string, data []byte) {
	t.Helper()
	root := os.Getenv("TSFG_READSPAN_CAPTURE_ROOT")
	if root == "" {
		return
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, name)
	if old, err := os.ReadFile(path); err == nil {
		if !bytes.Equal(old, data) {
			t.Fatal("original capture differs", path)
		}
	} else if os.IsNotExist(err) {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(data); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	} else {
		t.Fatal(err)
	}
}

func TestCommonProgramReadSpanCapture26i(t *testing.T) {
	for _, sample := range readSpanCaptureCases26i {
		t.Run(sample.name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/common-program-v2/readspan-capture/" + sample.name + ".ts")
			if err != nil {
				t.Fatal(err)
			}
			if len(source) != sample.bytes || arrayFactsSHA(source) != sample.sha || arrayFactsSHA(source[:1318]) != "6a86b0b018c8129985fe7f9bdbd50d9399b5622c3ba83a8466ddbb5e8b12ef92" {
				t.Fatal("frozen complete Source/prefix changed")
			}
			// A separate official Program verifies all four diagnostic groups before the
			// public command. Its original AST/checker then supplies the range/Symbol oracle.
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
			readSpanCaptureOfficial26i(t, string(source), program, p, sample)
		})
	}
}
