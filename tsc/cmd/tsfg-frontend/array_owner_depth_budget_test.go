// SPDX-License-Identifier: Apache-2.0
// Independent depth16 packet is frozen before the public command comparison.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func arrayOwnerDepth16Source23e() string {
	var source strings.Builder
	for i := 0; i < 16; i++ {
		if i == 0 {
			source.WriteString("export ")
		}
		fmt.Fprintf(&source, "function f%d(): f64 { return ", i)
		if i == 15 {
			source.WriteString("1.0")
		} else {
			fmt.Fprintf(&source, "f%d()", i+1)
		}
		source.WriteString("; }\n")
	}
	return source.String()
}

type arrayOwnerDepth16Bundle23e struct {
	Name, RequestText, PackageText, ContractSHA256 string
}

func arrayOwnerDepth16Shape23e(t *testing.T, item arrayOwnerDepth16Bundle23e) {
	t.Helper()
	ao23Want(t, item.Name == "forward-chain16" && item.ContractSHA256 == ao23ContractPin, "independently frozen depth16 name/contract")
	var requestValue, packetValue ao23Map
	if err := json.Unmarshal([]byte(item.RequestText), &requestValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(item.PackageText), &packetValue); err != nil {
		t.Fatal(err)
	}
	ao23Want(t, bytes.Equal([]byte(item.RequestText), append(ao23JCS(t, requestValue), '\n')) && bytes.Equal([]byte(item.PackageText), append(ao23JCS(t, packetValue), '\n')), "whole independent request/package JCS+LF")
	ao23Want(t, requestValue["family"] == "tsfg.array-facts-request-candidate" && requestValue["version"] == float64(3) && packetValue["family"] == "tsfg.array-facts-package-candidate" && packetValue["version"] == float64(3) && packetValue["schemaHash"] == ao23FactsHash, "exact v3 depth16 whole frame")
	payload := packetValue["payload"].(map[string]any)
	requestID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.request.v3\x00"), []byte(item.RequestText)...))
	packageID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.package.v3\x00"+ao23FactsHash+"\x00"), ao23JCS(t, payload)...))
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

func TestArrayOwner23eIndependentDepth16Oracle(t *testing.T) {
	dts, err := os.ReadFile("testdata/array-owner-oracle/minimal-arrays-v2.d.ts")
	if err != nil {
		t.Fatal(err)
	}
	ao23Want(t, len(dts) == 331 && ao23Hash(dts) == "6420493372913f019f64b094e181d1346afcca79e720ea85dc5e5026e03ba01e", "unchanged pinned independent DTS")
	// ao23Packet visits fresh official AST/types/symbols/resolved signatures and proves the entire DAG.
	// This selector never invokes any producer projection or public command.
	bundle := ao23Packet(t, ao23Map{"name": "forward-chain16", "source": arrayOwnerDepth16Source23e()}, string(dts))
	var item arrayOwnerDepth16Bundle23e
	if err := json.Unmarshal(bundle, &item); err != nil {
		t.Fatal(err)
	}
	arrayOwnerDepth16Shape23e(t, item)
	t.Logf("TSFG_ARRAY_OWNER_23E_INDEPENDENT_PACKET name=forward-chain16 bytes=%d sha256=%s base64=%s", len(bundle), ao23Hash(bundle), base64.StdEncoding.EncodeToString(bundle))
}

type arrayOwnerDepth16Writer23e struct {
	output bytes.Buffer
	writes int
}

func (w *arrayOwnerDepth16Writer23e) Write(data []byte) (int, error) {
	w.writes++
	return w.output.Write(data)
}

func TestArrayParameterOwner23ePublicDepth16Whole(t *testing.T) {
	frozen, err := os.ReadFile("testdata/array-facts-v6/depth16-expected.json")
	if err != nil || len(frozen) != 18710 || arrayFirstHash25(frozen) != "8eec21f436c4618c90ba1b47f5eb500adb6691a42795959ec1f47087ca72ab5c" {
		t.Fatalf("independently frozen v6 depth16 bytes: %v", err)
	}
	var item arrayOwnerDepth16Bundle23e
	if err := json.Unmarshal(frozen, &item); err != nil {
		t.Fatal(err)
	}
	arrayOwnerDepth16V6Shape26(t, item)
	input := []byte(item.RequestText)
	before := bytes.Clone(input)
	writer := &arrayOwnerDepth16Writer23e{}
	err = runFrontendCommand(t.Context(), []string{"--array-facts-io-v6"}, bytes.NewReader(input), writer)
	ao23Want(t, bytes.Equal(input, before), "public command preserves caller request")
	if err != nil {
		ao23Want(t, writer.writes == 0 && writer.output.Len() == 0, "rejection must not publish any partial packet")
		t.Fatalf("officially valid full forward chain16 must publish complete v6 package: %v", err)
	}
	actual := writer.output.Bytes()
	t.Logf("TSFG_ARRAY_OWNER_23E_PUBLIC_PACKET name=forward-chain16 bytes=%d sha256=%s base64=%s", len(actual), ao23Hash(actual), base64.StdEncoding.EncodeToString(actual))
	ao23Want(t, writer.writes == 1 && bytes.Equal(actual, []byte(item.PackageText)), "one complete public write equals independent frozen whole packet")
	actualItem := item
	actualItem.PackageText = string(actual)
	arrayOwnerDepth16V6Shape26(t, actualItem)
}
