// SPDX-License-Identifier: Apache-2.0
// Full original slots and forwarding facts precede Product owner/family decisions.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

func m26dReadSlotsForward(t *testing.T) []readSpan26dPriorCase {
	t.Helper()
	const file = "testdata/readspan-array-returns-v7/slots-forward-expected-facts.json"
	raw, err := os.ReadFile(file)
	ao23Want(t, err == nil && len(raw) == 95351 && ao23Hash(raw) == "3a9c208ab559c2086cff2932ae64346e808b94642891abed457d619bb51a6b3d", "complete independent mixed-slots/two-forwarders fixture")
	before := bytes.Clone(raw)
	t.Cleanup(func() {
		after, err := os.ReadFile(file)
		if err != nil || !bytes.Equal(after, before) || !bytes.Equal(raw, before) {
			t.Errorf("independent slots-forward fixture changed: %v", err)
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
	ao23Want(t, fixture.Family == "tsfg.m26d-independent-slots-forward-source-whole" && fixture.Version == 1 && fixture.SourceRoot == "/readspan26d-semantic" && fixture.RootFile == "/readspan26d-semantic/app.ts" && fixture.SchemaHash == "sha256:d2ee930b76b8fd5c2270ad69579c9c61377831a97034d3a8890d8aa770ebabc2" && len(fixture.Cases) == 2, "whole current facts7 source-form inventory")
	for i, item := range fixture.Cases {
		ao23Want(t, item.Name == []string{"mixed-slots-repeated-owner-then-view-helper", "two-forwarders-bound-then-tail"}[i] && item.SourceBytes == []int{427, 386}[i] && len(item.Source) == item.SourceBytes && ao23Units(item.Source) == item.SourceBytes && ao23Hash([]byte(item.Source)) == item.SourceSHA256, "entire original source/UTF16/hash")
		ao23Want(t, len(item.RequestText) == []int{1533, 1494}[i] && item.PackageBytes == []int{13410, 11753}[i] && len(item.PackageText) == item.PackageBytes && ao23Hash([]byte(item.PackageText)) == item.PackageSHA256, "complete independent request and expected packet")
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

func TestM26dArrayReturnSlotsForwardOfficialProgram(t *testing.T) {
	for i, item := range m26dReadSlotsForward(t) {
		t.Run(item.Name, func(t *testing.T) {
			witness := rs26cOfficialFixtureWitness(t, rs26cFrozenFixtureCase{Name: item.Name, Source: item.Source, SourcePath: "/readspan26d-semantic/app.ts", SourceSHA256: item.SourceSHA256, SourceBytes: item.SourceBytes, RequestText: item.RequestText, ExpectedText: item.PackageText, ExpectedBytes: item.PackageBytes, ExpectedSHA256: item.PackageSHA256})
			proof := witness["official"].(ao23Map)
			headers, calls := proof["headers"].([]any), proof["calls"].([]any)
			if i == 0 {
				ao23Want(t, len(headers) == 3 && len(calls) == 2, "all original select/identity/entry headers and calls")
				selectHeader := headers[0].(ao23Map)
				slots := selectHeader["parameters"].([]any)
				ao23Want(t, len(slots) == 5, "all five original required slots retained")
				for index, kind := range []string{"f64", "array.f64", "u32", "array.f64", "readspan.f64"} {
					slot := slots[index].(ao23Map)
					ao23Want(t, slot["index"] == index && slot["declaredType"] == kind && slot["signatureSymbolEqualsOriginal"] == true, "actual original full slot and nominal symbol identity")
				}
				var selectCall ao23Map
				for _, value := range calls {
					if value.(ao23Map)["target"] == selectHeader["binding"] {
						selectCall = value.(ao23Map)
					}
				}
				ao23Want(t, selectCall != nil && selectCall["resolvedOriginalDeclaration"] == true, "real resolved select declaration")
				args := selectCall["arguments"].([]any)
				ao23Want(t, len(args) == 5, "all original actual slots retained")
				a1, a3 := args[1].(ao23Map)["argument"].(ao23Map), args[3].(ao23Map)["argument"].(ao23Map)
				ao23Want(t, a1["binding"] == a3["binding"] && a1["flowType"] == "array.f64" && a3["flowType"] == "array.f64", "repeated actual a uses the same original Array symbol in slots1and3")
				var packet struct {
					Payload struct {
						Functions []struct{ Parameters []map[string]any }
					}
				}
				if err := json.Unmarshal([]byte(witness["officialPacketText"].(string)), &packet); err != nil {
					t.Fatal(err)
				}
				params := packet.Payload.Functions[0].Parameters
				for _, index := range []int{0, 2} {
					_, hasElement := params[index]["elementAnnotationRange"]
					ao23Want(t, !hasElement, "scalar parameter metadata omits inapplicable element annotation")
				}
			} else {
				ao23Want(t, len(headers) == 4 && len(calls) == 3, "all original borrow/bound-forward/tail-forward/entry headers and complete three-edge DAG")
			}
			data := ao23JCS(t, witness)
			t.Logf("TSFG_M26D_SLOTS_FORWARD_OFFICIAL_WHOLE name=%s bytes=%d sha256=%s base64=%s", item.Name, len(data), ao23Hash(data), base64.StdEncoding.EncodeToString(data))
		})
	}
}

// Root qualifies official2 before this separate public2 run. No Product is called.
func TestM26dArrayReturnSlotsForwardPublicHandler(t *testing.T) {
	for _, item := range m26dReadSlotsForward(t) {
		t.Run(item.Name, func(t *testing.T) {
			input := []byte(item.RequestText)
			before := bytes.Clone(input)
			var output readSpanParameterPublicWriter26b
			err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v7"}, bytes.NewReader(input), &output)
			t.Logf("TSFG_M26D_SLOTS_FORWARD_PUBLIC_ACTUAL name=%s bytes=%d sha256=%s writes=%d base64=%s", item.Name, output.Len(), ao23Hash(output.Bytes()), output.writes, base64.StdEncoding.EncodeToString(output.Bytes()))
			ao23Want(t, bytes.Equal(input, before), "complete caller request remains immutable")
			if err != nil {
				ao23Want(t, output.Len() == 0 && output.writes == 0, "any Source failure publishes no partial packet")
				t.Fatalf("existing Source slot/forwarding form must publish complete current facts7: %s", readSpanParameterPublicError26b(err))
			}
			ao23Want(t, output.writes == 1 && bytes.Equal(output.Bytes(), []byte(item.PackageText)), "one whole publication equals independent complete packet")
		})
	}
}
