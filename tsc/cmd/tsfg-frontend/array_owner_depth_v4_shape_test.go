// SPDX-License-Identifier: Apache-2.0
// Selected v6 shape leaves the historical independent v3 depth oracle unchanged.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"unicode/utf16"
)

func arrayOwnerDepth16V6Shape26(t *testing.T, item arrayOwnerDepth16Bundle23e) {
	t.Helper()
	ao23Want(t, item.Name == "forward-chain16" && item.ContractSHA256 == "0b0cbffa7efc0d44589ba45bc71f5380d7cc7a089a6303dc2f224e9cc7505081", "independently frozen depth16 name/contract")
	var requestValue, packetValue ao23Map
	if err := json.Unmarshal([]byte(item.RequestText), &requestValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(item.PackageText), &packetValue); err != nil {
		t.Fatal(err)
	}
	ao23Want(t, bytes.Equal([]byte(item.RequestText), append(ao23JCS(t, requestValue), '\n')) && bytes.Equal([]byte(item.PackageText), append(ao23JCS(t, packetValue), '\n')), "whole independent request/package JCS+LF")
	ao23Want(t, requestValue["family"] == "tsfg.array-facts-request-candidate" && requestValue["version"] == float64(6) && packetValue["family"] == "tsfg.array-facts-package-candidate" && packetValue["version"] == float64(6) && packetValue["schemaHash"] == ("sha256:"+arrayFirstSchemaSHA25), "exact v6 depth16 whole frame")
	payload := packetValue["payload"].(map[string]any)
	arrayOwnerDepth16V6Origins26(t, requestValue, payload)
	requestID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.request.v6\x00"), []byte(item.RequestText)...))
	packageID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.package.v6\x00"+("sha256:"+arrayFirstSchemaSHA25)+"\x00"), ao23JCS(t, payload)...))
	ao23Want(t, packetValue["requestId"] == requestID && packetValue["packageId"] == packageID, "independent complete wire identities")
	inputs := requestValue["input"].(map[string]any)
	files := inputs["files"].([]any)
	ao23Want(t, len(files) == 2 && files[0].(map[string]any)["content"] == arrayOwnerDepth16Source23e(), "whole independent sixteen-function original source")
	functions := payload["functions"].([]any)
	ao23Want(t, len(functions) == 16 && len(payload["diagnostics"].([]any)) == 0, "all16 original functions and zero diagnostics")
	edges := 0
	for i, v := range functions {
		fn := v.(map[string]any)
		body := fn["body"].([]any)
		ao23Want(t, fn["name"] == fmt.Sprintf("f%d", i) && fn["returnType"] == "f64" && len(fn["parameters"].([]any)) == 0 && len(fn["bindings"].([]any)) == 0 && len(body) == 1, "source order f64 functions, no parameters/locals, one final return")
		statement := body[0].(map[string]any)
		value := statement["value"].(map[string]any)
		ao23Want(t, statement["kind"] == "return" && value["type"] == "f64", "all scalar f64 final returns")
		if i == 15 {
			ao23Want(t, value["kind"] == "numericLiteral" && value["token"] == "1.0" && value["bits"] == "3ff0000000000000", "terminal original literal closes finite DAG")
		} else {
			next := functions[i+1].(map[string]any)
			ao23Want(t, value["kind"] == "directCall" && value["callee"] == next["id"] && len(value["arguments"].([]any)) == 0, "full forward original target edge and empty actual argument list")
			edges++
		}
	}
	exports := payload["exports"].([]any)
	ao23Want(t, edges == 15 && len(exports) == 1 && exports[0].(map[string]any)["name"] == "f0" && exports[0].(map[string]any)["functionId"] == functions[0].(map[string]any)["id"], "exactly15 forward edges, path16 and sole original f0 export")
}

func arrayOwnerDepth16V6Origins26(t *testing.T, requestValue, payload map[string]any) {
	t.Helper()
	input := requestValue["input"].(map[string]any)
	declarations := input["declarations"].(map[string]any)
	ao23Want(t, declarations["version"] == "minimal-arrays-v3", "actual declarations profile3 distinct from facts5")
	files := payload["files"].([]any)
	paths := declarations["paths"].([]any)
	ao23Want(t, len(paths) == 1, "one complete pinned DTS path")
	requested := []any{}
	for _, item := range files {
		file := item.(map[string]any)
		content := file["content"].(string)
		ao23Want(t, file["sha256"] == ao23Hash([]byte(content)) && file["byteLength"] == float64(len(content)) && file["utf16Length"] == float64(len(utf16.Encode([]rune(content)))), "complete depth owned source bytes and UTF16 identity")
		if file["path"] == paths[0] {
			ao23Want(t, len(content) == 484 && file["sha256"] == arrayFirstDtsSHA25, "exact complete pinned declarations3 bytes")
		}
		requested = append(requested, map[string]any{"path": file["path"], "content": content, "sha256": file["sha256"]})
	}
	ao23Want(t, bytes.Equal(arrayFirstCanonical25(t, input["files"]), arrayFirstCanonical25(t, requested)), "complete request and owned source text closure")
	var envelope struct{ Input request }
	if err := json.Unmarshal(arrayFirstCanonical25(t, requestValue), &envelope); err != nil {
		t.Fatal(err)
	}
	// Fresh official Program checks all diagnostics and the selected full closure.
	// Declaration origins are then joined through original Symbols/scoped generic
	// signatures. No Source producer helper is used to manufacture metadata.
	arrayDepthV4OfficialOrigins26(t, envelope.Input, payload)
}
