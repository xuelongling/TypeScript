// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestArrayBoundaryRetiredV7Dispatch25f(t *testing.T) {
	for _, args := range [][]string{{"--array-facts-io-v7"}, {"--array-facts-io-v7", "extra"}} {
		streams := &frontendCommandUnusedIO{}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		err := runFrontendCommand(ctx, args, streams, streams)
		if err == nil || err.Error() != "tsfg-frontend accepts one JSON request on stdin" || streams.reads != 0 || streams.writes != 0 {
			t.Fatalf("retired argv must fail before IO: %v %+v", err, streams)
		}
	}
}

func TestArrayBoundaryV8Public25f(t *testing.T) {
	source := "export function run(a:i32[],i:u32):i32{const s:ReadSpan<i32>=readSpan(a,0,1);return s[i];}"
	input := genericArrayRequest25f(t, source)
	var output bytes.Buffer
	if err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	var packet arrayGenericPackage
	if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	p := packet.Payload
	if packet.Version != 8 || packet.SchemaHash != "sha256:72dbf45fe1bc910892fef065af517787aed1819f43901f123f42ebaf6aef52ba" || len(p.Diagnostics) != 0 || len(p.Functions) != 1 || p.Functions[0].Body[0].Initializer.Callee != p.ReadSpanIntrinsic.ID || p.Functions[0].Body[1].Value.Kind != "checkedElementAccess" {
		t.Fatal("shared boundary migration changed actual v8 input/binding/intrinsic/checked-read facts")
	}
	oldRequest := strings.Replace(string(input), "\"version\":8", "\"version\":7", 1)
	output.Reset()
	if err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, strings.NewReader(oldRequest), &output); err == nil || output.Len() != 0 {
		t.Fatal("v8 must reject retired request version without a shim")
	}
}
