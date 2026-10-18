// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestStringFactsV2ASIAndTrivia(t *testing.T) {
	// All six UTF16 ranges are hand constants; the supplementary prefix and
	// comments are source text, not a parser or operand decoder.
	const original = "\r\n// 🧭\r\nexport default function named(): string {\r\n return \"\" /* left */ + /* right */ \"x\"\r\n}\r\n"
	for _, semicolon := range []bool{false, true} {
		t.Run(map[bool]string{false: "ASI", true: "explicitSemicolon"}[semicolon], func(t *testing.T) {
			source := original
			declEnd, returnEnd := 94, 91
			if semicolon {
				source = strings.Replace(source, "\"x\"\r\n", "\"x\";\r\n", 1)
				declEnd++
				returnEnd++
			}
			one := stringConcatExpectedCase24e{Name: "named", Declaration: stringConcatRange24e(9, declEnd), Statement: stringConcatRange24e(53, returnEnd), Concat: stringConcatRange24e(60, 91), Operator: stringConcatRange24e(74, 75), Left: stringConcatRange24e(60, 62), Right: stringConcatRange24e(88, 91), LeftUnits: []int32{}, RightUnits: []int32{120}}
			input := stringConcatRequest24e(t, source)
			cases := []stringConcatExpectedCase24e{one}
			expected := stringConcatExpected24e(t, input, cases)
			stringConcatOfficial24e(t, input, cases)
			stringConcatSuccess24e(t, input, expected)
		})
	}
}

func TestStringFactsV2OwnedRoundTrip(t *testing.T) {
	input := stringConcatOracleRead24e(t, "request.json", "5815ef980b77978c732239d56b0a1e6966183e0d8710de16d3b9448d19586028")
	expected := stringConcatOracleRead24e(t, "expected.json", "801792594c3f3b81d7d0a726bf99ea33772425f65abe3552236068d848c4498a")
	beforeInput := bytes.Clone(input)
	var requestEnvelope struct{ Input stringFactsInput }
	var expectedEnvelope struct{ Payload json.RawMessage }
	if json.Unmarshal(input, &requestEnvelope) != nil || json.Unmarshal(expected, &expectedEnvelope) != nil {
		t.Fatal("frozen owned input/complete expected decode")
	}
	owned, err := stringFactsInputs(requestEnvelope.Input)
	if err != nil {
		t.Fatal(err)
	}
	// This is the natural real v2 producer DTO, after its exclusive checker release.
	first, err := projectStringFactsConcat(t.Context(), owned, requestEnvelope.Input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := projectStringFactsConcat(t.Context(), owned, requestEnvelope.Input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stringOracleCanonical24(t, first), expectedEnvelope.Payload) || !bytes.Equal(stringOracleCanonical24(t, second), expectedEnvelope.Payload) {
		t.Fatal("complete real owned payload differs from independent frozen payload")
	}
	first.Functions[4].Body[0].Value.Left.CodeUnits[0] = 7
	first.Functions[0].Body[0].Value.Right.CodeUnits[0] = 9
	first.Files[0].Content = "changed first DTO source"
	if !reflect.DeepEqual(first.Functions[4].Body[0].Value.Right.CodeUnits, []int32{55358, 56813}) || !reflect.DeepEqual(first.Functions[1].Body[0].Value.Left.CodeUnits, []int32{0}) {
		t.Fatal("real producer arrays alias sibling operand/function owners")
	}
	requestEnvelope.Input.Files[0].Content = "changed caller source"
	requestEnvelope.Input.Declarations.Paths[0] = "/changed/caller.d.ts"
	for i := range input {
		input[i] = 0xa5
	}
	if !bytes.Equal(stringOracleCanonical24(t, second), expectedEnvelope.Payload) {
		t.Fatal("real producer result aliases caller or an earlier result")
	}
	t.Run("producerPreCanceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		var canceledEnvelope struct{ Input stringFactsInput }
		if json.Unmarshal(beforeInput, &canceledEnvelope) != nil {
			t.Fatal("valid canceled input decode")
		}
		payload, err := projectStringFactsConcat(ctx, owned, canceledEnvelope.Input)
		if !errors.Is(err, context.Canceled) || len(payload.Functions) != 0 {
			t.Fatalf("real producer cancellation exposed partial DTO: %v", err)
		}
	})
	// Cold JSON is just owned data; this test does not admit a v2 reader/HIR consumer.
}
