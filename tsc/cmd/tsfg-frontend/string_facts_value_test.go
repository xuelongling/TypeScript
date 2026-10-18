// SPDX-License-Identifier: Apache-2.0
// These hand-specified official-byte vectors are not generated Source or producer output.
package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestStringFactsValuesLosslessBoundaries(t *testing.T) {
	data := stringOracleRead24(t, "boundaries.json", "55445474d16adffd7c665f6644bcde63a7be704ea1b1293fc4e4754975ed5786")
	var vectors struct {
		Accepted []struct {
			Name                     string
			OfficialInternalBytesHex string
			CodeUnits                []int32
		}
		RejectedInternal []struct {
			Name     string
			BytesHex string
		}
	}
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	for _, tc := range vectors.Accepted {
		t.Run(tc.Name, func(t *testing.T) {
			raw, err := hex.DecodeString(strings.ReplaceAll(tc.OfficialInternalBytesHex, " ", ""))
			if err != nil {
				t.Fatal(err)
			}
			actual, err := stringFactsValueUnits(string(raw))
			if err != nil || !reflect.DeepEqual(actual, tc.CodeUnits) {
				t.Fatalf("lossless units: %v %v", actual, err)
			}
			actual[0] = 0
			again, err := stringFactsValueUnits(string(raw))
			if err != nil || !reflect.DeepEqual(again, tc.CodeUnits) {
				t.Fatal("owned units alias a prior result")
			}
		})
	}
	for _, tc := range vectors.RejectedInternal {
		t.Run(tc.Name, func(t *testing.T) {
			raw, err := hex.DecodeString(strings.ReplaceAll(tc.BytesHex, " ", ""))
			if err != nil {
				t.Fatal(err)
			}
			units, err := stringFactsValueUnits(string(raw))
			var failure *stringFactsFailure
			if units != nil || !errors.As(err, &failure) || failure.Kind != "unicode" {
				t.Fatalf("broken internal bytes accepted: %v %v", units, err)
			}
		})
	}
	t.Run("emptyNULAndReversedSurrogates", func(t *testing.T) {
		for _, tc := range []struct {
			raw  string
			want []int32
		}{
			{"", []int32{}}, {"\x00", []int32{0}}, {"\xed\xb8\x80\xed\xa0\xbd", []int32{56832, 55357}},
		} {
			got, err := stringFactsValueUnits(tc.raw)
			if err != nil || got == nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("units=%v err=%v", got, err)
			}
		}
	})
	t.Run("ordinaryJSONLosesSentinelButStrictBoundaryRejects", func(t *testing.T) {
		raw := "\xed\xa0\x80"
		encoded, err := json.Marshal(raw)
		if err != nil {
			t.Fatal(err)
		}
		var ordinary string
		if err = json.Unmarshal(encoded, &ordinary); err != nil || ordinary == raw {
			t.Fatal("ordinary JSON control must expose sentinel replacement")
		}
		if encoded, err := stringFactsCanonical(raw); err == nil || encoded != nil {
			t.Fatal("strict canonical boundary accepted invalid UTF8 scalar bytes")
		}
	})
	// Helper limits only: no long Source, Program, native or SourceEXE threshold credit.
	t.Run("value16384Then16385", func(t *testing.T) {
		units, err := stringFactsValueUnits(strings.Repeat("x", 16384))
		if err != nil || len(units) != 16384 {
			t.Fatal("value limit control")
		}
		if units, err = stringFactsValueUnits(strings.Repeat("x", 16385)); err == nil || units != nil {
			t.Fatal("value overflow accepted")
		}
		wide := strings.Repeat("\xf0\x90\x80\x80", 8192)
		if units, err = stringFactsValueUnits(wide); err != nil || len(units) != 16384 {
			t.Fatal("two-unit scalar boundary control")
		}
		if units, err = stringFactsValueUnits(wide + "x"); err == nil || units != nil {
			t.Fatal("two-unit scalar overflow accepted")
		}
	})
	t.Run("package65536Then65537", func(t *testing.T) {
		values := []string{strings.Repeat("x", 16384), strings.Repeat("y", 16384), strings.Repeat("z", 16384), strings.Repeat("w", 16384)}
		count, err := stringFactsTotalUnits(values)
		if err != nil || count != 65536 {
			t.Fatal("package helper limit control")
		}
		if _, err = stringFactsTotalUnits(append(values, "x")); err == nil {
			t.Fatal("package helper overflow accepted")
		}
	})
}
