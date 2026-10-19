// SPDX-License-Identifier: Apache-2.0
// Distinct original Array symbols remain facts; Product decides owner disjointness.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

func m26dReadOwnerAlias(t *testing.T) []readSpan26dPriorCase {
	t.Helper()
	const file = "testdata/readspan-array-returns-v7/owner-alias-expected-facts.json"
	raw, err := os.ReadFile(file)
	ao23Want(t, err == nil && len(raw) == 85634 && ao23Hash(raw) == "2db2ba71f314daab0910d1ff49b9b08a0b3961d5630d3e666bc430e0bd4fe335", "complete independent owner-alias Source whole fixture")
	before := bytes.Clone(raw)
	t.Cleanup(func() {
		after, err := os.ReadFile(file)
		if err != nil || !bytes.Equal(after, before) || !bytes.Equal(raw, before) {
			t.Errorf("independent owner-alias fixture changed: %v", err)
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
	ao23Want(t, fixture.Family == "tsfg.m26d-independent-owner-alias-source-whole-oracle" && fixture.Version == 1 && fixture.SourceRoot == "/readspan26d-semantic" && fixture.RootFile == "/readspan26d-semantic/app.ts" && fixture.SchemaHash == "sha256:d2ee930b76b8fd5c2270ad69579c9c61377831a97034d3a8890d8aa770ebabc2" && len(fixture.Cases) == 2, "whole current facts7 owner-alias inventory")
	for i, item := range fixture.Cases {
		ao23Want(t, item.Name == []string{"fresh-disjoint-structural-comparison", "formal-array-slots-may-alias"}[i] && item.SourceBytes == []int{318, 324}[i] && len(item.Source) == item.SourceBytes && ao23Units(item.Source) == item.SourceBytes && ao23Hash([]byte(item.Source)) == item.SourceSHA256, "complete original source/UTF16/hash")
		ao23Want(t, len(item.RequestText) == []int{1425, 1432}[i] && item.PackageBytes == []int{11366, 10992}[i] && len(item.PackageText) == item.PackageBytes && ao23Hash([]byte(item.PackageText)) == item.PackageSHA256, "entire independent request and expected packet")
		for _, frame := range []struct {
			data json.RawMessage
			text string
		}{{item.Request, item.RequestText}, {item.Package, item.PackageText}} {
			var value any
			if err := json.Unmarshal(frame.data, &value); err != nil {
				t.Fatal(err)
			}
			ao23Want(t, bytes.Equal(append(ao23JCS(t, value), '\n'), []byte(frame.text)), "literal objects equal their whole canonical frames")
		}
	}
	return fixture.Cases
}

func TestM26dArrayReturnOwnerAliasOfficialProgram(t *testing.T) {
	for i, item := range m26dReadOwnerAlias(t) {
		t.Run(item.Name, func(t *testing.T) {
			witness := rs26cOfficialFixtureWitness(t, rs26cFrozenFixtureCase{Name: item.Name, Source: item.Source, SourcePath: "/readspan26d-semantic/app.ts", SourceSHA256: item.SourceSHA256, SourceBytes: item.SourceBytes, RequestText: item.RequestText, ExpectedText: item.PackageText, ExpectedBytes: item.PackageBytes, ExpectedSHA256: item.PackageSHA256})
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
			names, parameters, locals, statements := []string{"borrow", "mutate", "entry"}, []int{1, 1, 0}, []int{1, 0, 3}, []int{2, 2, 6}
			callerIndex := 2
			if i == 1 {
				names, parameters, locals, statements = []string{"borrow", "mutate", "run", "entry"}, []int{1, 1, 2, 0}, []int{1, 0, 1, 0}, []int{2, 2, 4, 1}
			}
			ao23Want(t, len(headers) == len(names) && len(packet.Payload.Functions) == len(names) && len(calls) == 2, "every original function and both resolved calls retained")
			for j, fn := range packet.Payload.Functions {
				header := headers[j].(ao23Map)
				slots := header["parameters"].([]any)
				ao23Want(t, fn.Name == names[j] && header["name"] == names[j] && len(fn.Parameters) == parameters[j] && len(slots) == parameters[j] && len(fn.Bindings) == locals[j] && len(fn.Body) == statements[j], "complete ordered original function, required slots, locals and statements")
				for k, slotValue := range slots {
					slot := slotValue.(ao23Map)
					ao23Want(t, slot["index"] == k && slot["declaredType"] == "array.f64" && slot["signatureSymbolEqualsOriginal"] == true && fn.Parameters[k]["declaredType"] == "array.f64", "each original Array parameter occupies its complete signature slot")
				}
			}
			caller := packet.Payload.Functions[callerIndex]
			var a, b any
			if i == 0 {
				a, b = caller.Bindings[0]["id"], caller.Bindings[1]["id"]
				ao23Want(t, caller.Bindings[0]["name"] == "a" && caller.Bindings[1]["name"] == "b" && caller.Bindings[0]["declaredType"] == "array.f64" && caller.Bindings[1]["declaredType"] == "array.f64" && caller.Body[0]["initializer"].(map[string]any)["kind"] == "arrayLiteral" && caller.Body[1]["initializer"].(map[string]any)["kind"] == "arrayLiteral", "two complete fresh Array literal declarations retain distinct original symbols")
			} else {
				a, b = caller.Parameters[0]["id"], caller.Parameters[1]["id"]
				ao23Want(t, caller.Name == "run" && caller.Parameters[0]["name"] == "a" && caller.Parameters[1]["name"] == "b" && len(caller.Bindings) == 1, "uninvoked run retains both distinct formal Array symbols without inferring disjoint owners")
			}
			ao23Want(t, a != nil && b != nil && a != b, "distinct original binding identities are preserved")
			for j, actualBinding := range []any{a, b} {
				call := calls[j].(ao23Map)
				args := call["arguments"].([]any)
				ao23Want(t, call["target"] == headers[j].(ao23Map)["binding"] && call["resolvedOriginalDeclaration"] == true && len(args) == 1, "borrow/mutate calls resolve to their actual full original signatures")
				arg := args[0].(ao23Map)
				value := arg["argument"].(ao23Map)
				ao23Want(t, arg["index"] == 0 && arg["originalParameterDeclaration"] == true && arg["assignable"] == true && value["binding"] == actualBinding && value["flowType"] == "array.f64", "actual caller Array symbol joins original required slot0")
			}
			graph := proof["completeCallDAG"].(map[string][]string)
			ao23Want(t, len(graph) == len(names) && len(graph[caller.ID]) == 2 && graph[caller.ID][0] == packet.Payload.Functions[0].ID && graph[caller.ID][1] == packet.Payload.Functions[1].ID, "complete actual caller-to-borrow/mutate DAG")
			if i == 1 {
				entry := packet.Payload.Functions[3]
				ao23Want(t, entry.Name == "entry" && len(graph[entry.ID]) == 0 && caller.ID != entry.ID && len(caller.Body) == 4, "uncalled run is still fully analyzed alongside the exported entry")
			}
			data := ao23JCS(t, witness)
			t.Logf("TSFG_M26D_OWNER_ALIAS_OFFICIAL_WHOLE name=%s bytes=%d sha256=%s base64=%s", item.Name, len(data), ao23Hash(data), base64.StdEncoding.EncodeToString(data))
		})
	}
}

// Root qualifies official2 before this separate public2 run. No Product is called.
func TestM26dArrayReturnOwnerAliasPublicFacts(t *testing.T) {
	for _, item := range m26dReadOwnerAlias(t) {
		t.Run(item.Name, func(t *testing.T) {
			input := []byte(item.RequestText)
			before := bytes.Clone(input)
			var output readSpanParameterPublicWriter26b
			err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v7"}, bytes.NewReader(input), &output)
			t.Logf("TSFG_M26D_OWNER_ALIAS_PUBLIC_ACTUAL name=%s bytes=%d sha256=%s writes=%d base64=%s", item.Name, output.Len(), ao23Hash(output.Bytes()), output.writes, base64.StdEncoding.EncodeToString(output.Bytes()))
			ao23Want(t, bytes.Equal(input, before), "whole public request remains immutable")
			if err != nil {
				ao23Want(t, output.Len() == 0 && output.writes == 0, "any Source failure publishes no partial packet")
				t.Fatalf("existing Source owner-alias form must publish full facts7: %s", readSpanParameterPublicError26b(err))
			}
			ao23Want(t, output.writes == 1 && bytes.Equal(output.Bytes(), []byte(item.PackageText)), "one complete Source publication equals independent whole literal")
		})
	}
}
