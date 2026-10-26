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

func commonIndirectRequest37s(t *testing.T, source []byte) []byte {
	t.Helper()
	var request map[string]any
	if err := json.Unmarshal(genericArrayRequest25f(t, string(source)), &request); err != nil {
		t.Fatal(err)
	}
	request["family"], request["version"], request["schemaHash"] = "tsfg.program-facts-request-candidate", 2, "sha256:"+arrayFactsSHA(commonProgramSchema)
	input := request["input"].(map[string]any)
	input["target"] = map[string]string{"target": "windows-x86_64-msvc", "triple": "x86_64-pc-windows-msvc", "pointerBits": "64", "llvmDataLayout": "e-m:w-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128"}
	root := func(kind, name string) closureRoot {
		return closureRoot{Kind: kind, SourcePath: "/src/main.ts", Name: name}
	}
	input["compilationClosure"] = map[string]any{"roots": []closureRoot{root("application", "arrayOrdinary"), root("application", "closureEntry"), root("application", "ordinary"), root("application", "valueOrdinary"), root("kernel", "left"), root("kernel", "right"), root("module-initialization", "$moduleInit")}, "requiredRoots": []closureRoot{root("kernel", "left"), root("kernel", "right")}}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	canonical := tsjson.Value(encoded)
	if err := canonical.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	return append(canonical, '\n')
}

func TestCommonProgramIndirectPublic37s(t *testing.T) {
	for _, sample := range []struct {
		name  string
		bytes int
		sha   string
	}{{"common-indirect-valid", 1262, "74d1c3c23e2482f3d3fe1e19579e44ae82168057544f0e010f2bbfb51b3fccbb"}, {"common-indirect-broad-valid", 1236, "9810f4c17483a56f8f619a068c2e013b05213cd454ad010a4030828894c97975"}, {"common-indirect-shadow-valid", 1293, "22505ec55ef2a642674ae15a8281b2a2ac412e450e24bb050a3bf657f31fe175"}} {
		t.Run(sample.name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/common-program-v2/" + sample.name + ".ts")
			if err != nil {
				t.Fatal(err)
			}
			if len(source) != sample.bytes || arrayFactsSHA(source) != sample.sha {
				t.Fatal("frozen complete indirect Source changed")
			}
			request := commonIndirectRequest37s(t, source)
			var output bytes.Buffer
			runErr := runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(request), &output)
			if runErr != nil {
				if failure, ok := runErr.(*arrayFactsFailure); ok {
					details, err := json.Marshal(failure)
					if err != nil {
						t.Fatal(err)
					}
					t.Log(string(details))
				}
				t.Fatal(runErr)
			}
			if root := os.Getenv("TSFG_COMMON_PROGRAM_CAPTURE_ROOT"); root != "" {
				if err := os.MkdirAll(root, 0700); err != nil {
					t.Fatal(err)
				}
				for suffix, data := range map[string][]byte{"request.json": request, "package.json": output.Bytes()} {
					path := filepath.Join(root, sample.name+"-"+suffix)
					if old, err := os.ReadFile(path); err == nil {
						if !bytes.Equal(old, data) {
							t.Fatal("original capture differs", path)
						}
					} else if os.IsNotExist(err) {
						if err := os.WriteFile(path, data, 0600); err != nil {
							t.Fatal(err)
						}
					} else {
						t.Fatal(err)
					}
				}
			}
			var packet commonProgramPackage
			if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}
			if len(packet.Payload.Functions) != 11 || len(packet.Payload.Globals) != 2 || len(packet.Payload.Modules) != 1 || len(packet.Payload.Closure.Roots) != 7 || len(packet.Payload.EntryCompatibilityRoots) != 6 {
				t.Fatal("complete indirect Program inventory changed")
			}
			var wire map[string]any
			if err := json.Unmarshal(output.Bytes(), &wire); err != nil {
				t.Fatal(err)
			}
			for _, raw := range wire["payload"].(map[string]any)["functions"].([]any) {
				fn := raw.(map[string]any)
				if _, present := fn["lexicalParent"]; !present {
					t.Fatal("required nullable lexicalParent missing")
				}
				for _, field := range []string{"parameters", "bindings"} {
					for _, rawBinding := range fn[field].([]any) {
						if _, present := rawBinding.(map[string]any)["lexicalOwner"]; !present {
							t.Fatal("required lexicalOwner missing")
						}
					}
				}
			}
			for _, raw := range wire["payload"].(map[string]any)["globals"].([]any) {
				binding := raw.(map[string]any)["binding"].(map[string]any)
				if owner, present := binding["lexicalOwner"]; !present || owner != nil {
					t.Fatal("required global null lexicalOwner missing")
				}
			}
			commonIndirectOfficial37s(t, string(source), packet.Payload)
		})
	}
}
