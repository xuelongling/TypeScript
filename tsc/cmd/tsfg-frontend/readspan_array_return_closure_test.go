// SPDX-License-Identifier: Apache-2.0
// Source publishes complete tail/maximum-call facts; Product alone decides escape and root work.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

func m26dReadClosure(t *testing.T) []readSpan26dPriorCase {
	t.Helper()
	const file = "testdata/readspan-array-returns-v7/closure-expected-facts.json"
	raw, err := os.ReadFile(file)
	ao23Want(t, err == nil && len(raw) == 151303 && ao23Hash(raw) == "92991ccfb66c73ebd81051dcb0ed2326a5da6e137d00415c2a173432515e6a8d", "complete independent closure Source whole fixture")
	before := bytes.Clone(raw)
	t.Cleanup(func() {
		after, err := os.ReadFile(file)
		if err != nil || !bytes.Equal(after, before) || !bytes.Equal(raw, before) {
			t.Errorf("independent closure fixture changed: %v", err)
		}
	})
	var fixture struct {
		Family, SourceRoot, RootFile, SchemaHash string
		Version                                  int
		Cases                                    []readSpan26dPriorCase
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	ao23Want(t, fixture.Family == "tsfg.m26d-independent-closure-source-whole-oracle" && fixture.Version == 1 && fixture.SourceRoot == "/readspan26d-closure" && fixture.RootFile == "/readspan26d-closure/app.ts" && fixture.SchemaHash == "sha256:d2ee930b76b8fd5c2270ad69579c9c61377831a97034d3a8890d8aa770ebabc2" && len(fixture.Cases) == 2, "whole current facts7 closure inventory")
	for i, item := range fixture.Cases {
		ao23Want(t, item.Name == []string{"tail-array-call-return-local-escape", "reachable-forty-root-call-results"}[i] && item.SourceBytes == []int{229, 1160}[i] && len(item.Source) == item.SourceBytes && ao23Units(item.Source) == item.SourceBytes && ao23Hash([]byte(item.Source)) == item.SourceSHA256, "entire original source/UTF16/hash")
		ao23Want(t, len(item.RequestText) == []int{1326, 2286}[i] && item.PackageBytes == []int{8211, 31456}[i] && len(item.PackageText) == item.PackageBytes && ao23Hash([]byte(item.PackageText)) == item.PackageSHA256, "complete independent request and expected packet")
		for _, frame := range []struct {
			data json.RawMessage
			text string
		}{{item.Request, item.RequestText}, {item.Package, item.PackageText}} {
			var value any
			if err := json.Unmarshal(frame.data, &value); err != nil {
				t.Fatal(err)
			}
			ao23Want(t, bytes.Equal(append(ao23JCS(t, value), '\n'), []byte(frame.text)), "literal objects equal their entire canonical frames")
		}
	}
	return fixture.Cases
}

func TestM26dArrayReturnClosureOfficialProgram(t *testing.T) {
	for i, item := range m26dReadClosure(t) {
		t.Run(item.Name, func(t *testing.T) {
			witness := rs26cOfficialFixtureWitness(t, rs26cFrozenFixtureCase{Name: item.Name, Source: item.Source, SourcePath: "/readspan26d-closure/app.ts", SourceSHA256: item.SourceSHA256, SourceBytes: item.SourceBytes, RequestText: item.RequestText, ExpectedText: item.PackageText, ExpectedBytes: item.PackageBytes, ExpectedSHA256: item.PackageSHA256})
			proof := witness["official"].(ao23Map)
			headers, calls := proof["headers"].([]any), proof["calls"].([]any)
			var packet struct {
				Payload struct {
					Functions []struct {
						ID, Name             string
						Parameters, Bindings []map[string]any
						Body                 []map[string]any
					}
				}
			}
			if err := json.Unmarshal([]byte(witness["officialPacketText"].(string)), &packet); err != nil {
				t.Fatal(err)
			}
			names, parameters, locals, statements, ordinary := []string{"borrow", "escape", "entry"}, []int{1, 0, 0}, []int{1, 1, 0}, []int{2, 2, 1}, 1
			if i == 1 {
				names, parameters, locals, statements, ordinary = []string{"borrow", "entry"}, []int{1, 8}, []int{1, 17}, []int{2, 34}, 32
			}
			ao23Want(t, len(headers) == len(names) && len(packet.Payload.Functions) == len(names) && len(calls) == ordinary, "all original functions and ordinary calls retained")
			for j, fn := range packet.Payload.Functions {
				header := headers[j].(ao23Map)
				slots := header["parameters"].([]any)
				ao23Want(t, fn.Name == names[j] && header["name"] == names[j] && len(fn.Parameters) == parameters[j] && len(slots) == parameters[j] && len(fn.Bindings) == locals[j] && len(fn.Body) == statements[j], "complete ordered original function, required slots, locals and statements")
				for k, slotValue := range slots {
					slot := slotValue.(ao23Map)
					kind := "array.f64"
					if i == 1 && j == 1 {
						kind = "readspan.f64"
					}
					ao23Want(t, slot["index"] == k && slot["declaredType"] == kind && slot["signatureSymbolEqualsOriginal"] == true && fn.Parameters[k]["declaredType"] == kind, "complete required original slot and nominal Symbol identity")
				}
			}
			borrow, caller := packet.Payload.Functions[0], packet.Payload.Functions[1]
			array := caller.Bindings[0]
			ao23Want(t, array["name"] == "a" && array["declaredType"] == "array.f64" && caller.Body[0]["initializer"].(map[string]any)["kind"] == "arrayLiteral", "complete fresh local Array declaration and initializer")
			for _, value := range calls {
				call := value.(ao23Map)
				args := call["arguments"].([]any)
				ao23Want(t, call["target"] == borrow.ID && call["resolvedOriginalDeclaration"] == true && call["returnType"] == "readspan.f64" && len(args) == 1, "every ordinary call resolves the real original borrow signature")
				arg := args[0].(ao23Map)
				actual := arg["argument"].(ao23Map)
				ao23Want(t, arg["index"] == 0 && arg["binding"] == borrow.Parameters[0]["id"] && arg["originalParameterDeclaration"] == true && arg["assignable"] == true && actual["binding"] == array["id"] && actual["flowType"] == "array.f64", "every complete original actual Array joins required slot0")
			}
			graph := proof["completeCallDAG"].(map[string][]string)
			ao23Want(t, len(graph) == len(names) && len(graph[borrow.ID]) == 0 && len(graph[caller.ID]) == ordinary, "complete original graph retains every repeated call edge")
			for _, target := range graph[caller.ID] {
				ao23Want(t, target == borrow.ID, "all original caller edges reach the actual borrow declaration")
			}
			if i == 0 {
				tail := caller.Body[1]["value"].(map[string]any)
				span := tail["range"].(map[string]any)
				ao23Want(t, caller.Name == "escape" && caller.Body[1]["kind"] == "return" && tail["kind"] == "directCall" && span["start"] == float64(171) && span["end"] == float64(180) && len(graph[packet.Payload.Functions[2].ID]) == 0, "actual local-Array tail call and uninvoked exported entry retain full original geometry")
			} else {
				for j := 1; j <= 16; j++ {
					binding, statement := caller.Bindings[j], caller.Body[j]
					ao23Want(t, binding["declaredType"] == "readspan.f64" && statement["kind"] == "variable" && statement["binding"] == binding["id"] && statement["initializer"].(map[string]any)["kind"] == "directCall", "all sixteen original bound call results retain their real const bindings")
				}
				for j := 17; j <= 32; j++ {
					ao23Want(t, caller.Body[j]["kind"] == "call" && caller.Body[j]["value"].(map[string]any)["kind"] == "directCall", "all sixteen original discarded calls remain complete")
				}
			}
			data := ao23JCS(t, witness)
			t.Logf("TSFG_M26D_CLOSURE_OFFICIAL_WHOLE name=%s bytes=%d sha256=%s base64=%s", item.Name, len(data), ao23Hash(data), base64.StdEncoding.EncodeToString(data))
		})
	}
}

// Root qualifies official2 before this separate public2 run. Both Source programs publish facts.
func TestM26dArrayReturnClosurePublicFacts(t *testing.T) {
	for _, item := range m26dReadClosure(t) {
		t.Run(item.Name, func(t *testing.T) {
			input := []byte(item.RequestText)
			before := bytes.Clone(input)
			var output readSpanParameterPublicWriter26b
			err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v7"}, bytes.NewReader(input), &output)
			t.Logf("TSFG_M26D_CLOSURE_PUBLIC_ACTUAL name=%s bytes=%d sha256=%s writes=%d base64=%s", item.Name, output.Len(), ao23Hash(output.Bytes()), output.writes, base64.StdEncoding.EncodeToString(output.Bytes()))
			ao23Want(t, bytes.Equal(input, before), "complete public request remains unchanged")
			if err != nil {
				ao23Want(t, output.Len() == 0 && output.writes == 0, "any Source failure publishes no partial packet")
				t.Fatalf("existing Source closure form must publish full facts7: %s", readSpanParameterPublicError26b(err))
			}
			ao23Want(t, output.writes == 1 && bytes.Equal(output.Bytes(), []byte(item.PackageText)), "one complete Source publication equals independent whole literal")
		})
	}
}
