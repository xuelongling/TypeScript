// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/json"
)

func TestCommandWireRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	err := runCommand(strings.NewReader(`{"unexpected":true}`), &output)
	if err == nil || err.Error() != `json: unknown field "unexpected"` || output.Len() != 0 {
		t.Fatalf("unknown input field must fail before analysis/output: err=%v output=%q", err, output.String())
	}
}

func TestCommandWirePreservesLegacyResponseEncoding(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	if err := json.NewLegacyEncoder(&output).Encode(response{Status: "ok", Symbols: []symbol{}}); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "{\"status\":\"ok\",\"symbols\":[]}\n"; got != want {
		t.Fatalf("empty optional collections and required symbols changed: got %q want %q", got, want)
	}
	output.Reset()
	flag := false
	if err := json.NewLegacyEncoder(&output).Encode(expression{Kind: "boolean", Type: "boolean", Boolean: &flag, Symbol: "<>&"}); err != nil {
		t.Fatal(err)
	}
	const want = "{\"kind\":\"boolean\",\"type\":\"boolean\",\"symbol\":\"\\u003c\\u003e\\u0026\",\"boolean\":false,\"range\":{\"start\":0,\"end\":0}}\n"
	if got := output.String(); got != want {
		t.Fatalf("false pointer/HTML escaping/field order/newline changed: got %q want %q", got, want)
	}
}

func TestCommandWirePreservesLegacyRequestDecoding(t *testing.T) {
	t.Parallel()
	var r request
	decoder := json.NewLegacyDecoder(strings.NewReader(`{"SOURCEROOT":"/src","target":null,"files":[{"path":"/src/main.ts","content":"\uD83D\uDE00"}]} {}`))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&r); err != nil {
		t.Fatal(err)
	}
	if r.SourceRoot != "/src" || r.Target != nil || len(r.Files) != 1 || r.Files[0].Content != "😀" {
		t.Fatalf("case-insensitive names/null/Unicode/string values changed: %#v", r)
	}
	var location span
	decoder = json.NewLegacyDecoder(strings.NewReader(`{"start":2147483647,"end":7}`))
	if err := decoder.Decode(&location); err != nil || location.Start != 2147483647 || location.End != 7 {
		t.Fatalf("exact integer decoding changed: %#v err=%v", location, err)
	}
	decoder = json.NewLegacyDecoder(strings.NewReader(`{"start":1.5}`))
	if err := decoder.Decode(&location); err == nil || !strings.Contains(err.Error(), "cannot unmarshal number 1.5") {
		t.Fatalf("fractional integer input must retain a typed JSON error: %v", err)
	}
}
