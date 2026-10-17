// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/base64"
	"testing"
)

// These hand-counted literals cover only the agreed non-trivia, non-EOF lexical
// count. No AST, Source Program, checker, parser host or source execution is used.
// At preparation Root stated that the agreed private API was absent. An initial
// undefined-symbol failure is mechanical API RED only, without semantic credit.
// These controls grant no 50000-token boundary credit,
// no early-stop-at-50001 proof and no whole-Program resource-limit proof.
func TestExceptionFactsSourceTokenCountSmallControls(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
		want   int
	}{
		{"empty", "", 0},
		{"onlyTriviaCRLFAndEmoji", "\r\n \t// 😀\r\n/* 😀 */\r\n", 0},
		{"throwReference", "throw value;", 3},
		{"returnReference", "return value;", 3},
		{"constReference", "const copied = value;", 5},
		{"exportedFunction", "export function entry(value: i32): i32 { throw value; }", 15},
		{"typeofBoolean", "typeof caught === \"boolean\"", 4},
		{"quotedBoolean", "'boolean';", 2},
		{"commentAndEscapedIdentifier", "// 😀\r\nthrow \\u0078;", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.source) >= 200 {
				t.Fatal("small literal must stay below 200 UTF-8 bytes")
			}
			got := exceptionFactsSourceTokenCount(tc.source)
			t.Logf("TSFG_EXCEPTION_SOURCE_TOKEN_SMALL_V1 case=%s sourceBase64=%s want=%d got=%d", tc.name, base64.StdEncoding.EncodeToString([]byte(tc.source)), tc.want, got)
			if got != tc.want {
				t.Fatalf("hand-counted non-trivia/non-EOF tokens differ: got=%d want=%d", got, tc.want)
			}
		})
	}
}
