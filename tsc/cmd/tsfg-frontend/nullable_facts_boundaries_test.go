// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/json"
)

const nullableFactsBoundarySchema = "sha256:badc9425050a8f538fcb7b9397e5ef31fbde96391989dfeebb8cab6a122783bd"
const nullableFactsBoundarySource = "function echo(value: i32 | null): i32 | null { return value; }\nexport function entry(value: i32): i32 { const result: i32 | null = echo(value); return value; }"

// Official canonicalization admits these fixed test DTOs without borrowing the
// private producer or the generic-package budget walker, which rejects maps.
func nullableFactsBoundaryCanonical(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	canonical := json.Value(data)
	if err := canonical.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	return canonical
}

func nullableFactsBoundaryRequest(t *testing.T, r request) []byte {
	t.Helper()
	return nullableFactsBoundaryCanonical(t, map[string]any{
		"family": "tsfg.nullable-facts-request-candidate", "version": 2,
		"schemaHash": nullableFactsBoundarySchema, "input": r,
	})
}

func nullableFactsBoundaryRun(ctx context.Context, args []string, input []byte) ([]byte, error) {
	var output bytes.Buffer
	err := runFrontendCommand(ctx, args, bytes.NewReader(input), &output)
	return bytes.Clone(output.Bytes()), err
}

func nullableFactsBoundaryLegacy(t *testing.T, r request) response {
	t.Helper()
	input, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := nullableFactsBoundaryRun(t.Context(), nil, input)
	if err != nil {
		t.Fatalf("legacy command failed before private boundary: %v", err)
	}
	var result response
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("legacy response is not JSON: %v; raw=%s", err, raw)
	}
	return result
}

func nullableFactsBoundaryReject(t *testing.T, input []byte) {
	t.Helper()
	before := bytes.Clone(input)
	output, err := nullableFactsBoundaryRun(t.Context(), []string{"--nullable-facts-io-v2"}, input)
	if err == nil || len(output) != 0 {
		t.Fatalf("private rejection must return error and no partial package: err=%v output=%s", err, output)
	}
	if !bytes.Equal(input, before) {
		t.Fatal("rejection modified caller input bytes")
	}
}

func TestNullableFactsCommandStrictBoundary(t *testing.T) {
	baseline := nullableFactsBoundaryRequest(t, integerRequest(nullableFactsBoundarySource))
	if output, err := nullableFactsBoundaryRun(t.Context(), []string{"--nullable-facts-io-v2"}, baseline); err != nil || len(output) == 0 {
		t.Fatalf("canonical positive control failed: %v; raw=%s", err, output)
	}
	mutated := func(change func(map[string]json.Value)) []byte {
		var value map[string]json.Value
		if err := json.Unmarshal(baseline, &value); err != nil {
			t.Fatal(err)
		}
		change(value)
		return nullableFactsBoundaryCanonical(t, value)
	}
	inputMutated := func(change func(map[string]json.Value)) []byte {
		return mutated(func(envelope map[string]json.Value) {
			var input map[string]json.Value
			if err := json.Unmarshal(envelope["input"], &input); err != nil {
				t.Fatal(err)
			}
			change(input)
			envelope["input"] = nullableFactsBoundaryCanonical(t, input)
		})
	}
	cases := []struct {
		name  string
		input []byte
	}{
		{"unknown", mutated(func(v map[string]json.Value) { v["unknown"] = json.Value("true") })},
		{"missing-family", mutated(func(v map[string]json.Value) { delete(v, "family") })},
		{"missing-input", mutated(func(v map[string]json.Value) { delete(v, "input") })},
		{"missing-version", mutated(func(v map[string]json.Value) { delete(v, "version") })},
		{"missing-schema", mutated(func(v map[string]json.Value) { delete(v, "schemaHash") })},
		{"case-variant", mutated(func(v map[string]json.Value) { v["Family"] = v["family"]; delete(v, "family") })},
		{"mixed-key-case", mutated(func(v map[string]json.Value) { v["Family"] = v["family"] })},
		{"mixed-family", mutated(func(v map[string]json.Value) { v["family"] = json.Value("\"tsfg-owned-generic-package\"") })},
		{"mixed-version", mutated(func(v map[string]json.Value) { v["version"] = json.Value("1") })},
		{"mixed-schema", mutated(func(v map[string]json.Value) {
			v["schemaHash"] = json.Value("\"sha256:" + strings.Repeat("0", 64) + "\"")
		})},
		{"unknown-input-key", inputMutated(func(v map[string]json.Value) { v["unknown"] = json.Value("true") })},
		{"input-key-case", inputMutated(func(v map[string]json.Value) { v["RootFile"] = v["rootFile"]; delete(v, "rootFile") })},
		{"duplicate-input-key", []byte(strings.Replace(string(baseline), "\"rootFile\":", "\"rootFile\":\"/src/duplicate.ts\",\"rootFile\":", 1))},
		{"null-input", mutated(func(v map[string]json.Value) { v["input"] = json.Value("null") })},
		{"empty-input", mutated(func(v map[string]json.Value) { v["input"] = json.Value("{}") })},
		{"duplicate-family", append([]byte("{\"family\":\"tsfg.nullable-facts-request-candidate\","), baseline[1:]...)},
		{"leading-space", append([]byte(" "), baseline...)},
		{"trailing-LF", append(bytes.Clone(baseline), '\n')},
		{"concatenated-request", append(bytes.Clone(baseline), baseline...)},
		{"BOM", append([]byte{0xef, 0xbb, 0xbf}, baseline...)},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if bytes.Equal(test.input, baseline) {
				t.Fatal("negative framing mutation was a no-op")
			}
			nullableFactsBoundaryReject(t, test.input)
		})
	}
}

func TestNullableFactsCommandExportBoundary(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{"nullable-parameter", "export function entry(value: i32 | null, result: i32): i32 { return result; }"},
		{"nullable-return-and-parameter", "export function entry(value: i32 | null): i32 | null { return value; }"},
		{"two-direct-exports", nullableFactsBoundarySource + "\nexport function extra(value: i32): i32 { return value; }"},
		{"no-export", "function entry(value: i32): i32 { return value; }"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			r := integerRequest(test.source)
			legacy := nullableFactsBoundaryLegacy(t, r)
			if legacy.Status != "ok" || len(legacy.Diagnostics) != 0 {
				t.Fatalf("export negative did not reach the private gate: %+v", legacy)
			}
			nullableFactsBoundaryReject(t, nullableFactsBoundaryRequest(t, r))
		})
	}
	// Some inputs fail existing top-level/input projection first. Record the
	// actual distinction without skip or mislabeling it as export-gate coverage.
	preflight := []struct {
		name string
		r    request
	}{
		{"alias", integerRequest("function entry(value: i32): i32 { return value; }\nexport { entry as renamed };")},
		{"nonfunction", integerRequest("export const entry: i32 = 0 as i32;")},
		{"foreign", integerRequest("export { foreign as entry } from \"./other\";")},
	}
	preflight[2].r.Files = append(preflight[2].r.Files, inputFile{
		Path: "/src/other.ts", Content: "export function foreign(value: i32): i32 { return value; }",
	})
	for _, test := range preflight {
		t.Run(test.name, func(t *testing.T) {
			legacy := nullableFactsBoundaryLegacy(t, test.r)
			if legacy.Status == "ok" && len(legacy.Diagnostics) == 0 {
				t.Log("legacy projection passed; private rejection is export-boundary coverage")
			} else {
				t.Logf("preprojection rejection only; no export-gate credit: status=%s diagnostics=%v", legacy.Status, legacy.Diagnostics)
			}
			nullableFactsBoundaryReject(t, nullableFactsBoundaryRequest(t, test.r))
		})
	}
}

func TestNullableFactsCommandCanceledBoundary(t *testing.T) {
	input := nullableFactsBoundaryRequest(t, integerRequest(nullableFactsBoundarySource))
	before := bytes.Clone(input)
	if output, err := nullableFactsBoundaryRun(t.Context(), []string{"--nullable-facts-io-v2"}, input); err != nil || len(output) == 0 {
		t.Fatalf("noncanceled public control failed: %v; raw=%s", err, output)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	output, err := nullableFactsBoundaryRun(ctx, []string{"--nullable-facts-io-v2"}, input)
	if !errors.Is(err, context.Canceled) || len(output) != 0 || !bytes.Equal(input, before) {
		t.Fatalf("pre-canceled call must fail without output or input mutation: err=%v output=%s", err, output)
	}
}

func TestNullableFactsCommandFreshOwnershipBoundary(t *testing.T) {
	r := integerRequest(nullableFactsBoundarySource)
	input := nullableFactsBoundaryRequest(t, r)
	before := bytes.Clone(input)
	type packet struct {
		RequestID string `json:"requestId"`
		PackageID string `json:"packageId"`
		Payload   struct {
			InputIdentity string `json:"inputIdentity"`
			Files         []struct {
				Path string `json:"path"`
			} `json:"files"`
			Facts   json.Value `json:"facts"`
			Exports []struct {
				Name string `json:"name"`
			} `json:"exports"`
		} `json:"payload"`
	}
	acquire := func(r request, input []byte, entry string) ([]byte, packet) {
		t.Helper()
		before := bytes.Clone(input)
		output, err := nullableFactsBoundaryRun(t.Context(), []string{"--nullable-facts-io-v2"}, input)
		if err != nil || !bytes.Equal(input, before) {
			t.Fatalf("fresh private acquisition failed or changed input: %v; raw=%s", err, output)
		}
		var got packet
		if err := json.Unmarshal(output, &got); err != nil || len(got.Payload.Exports) != 1 || got.Payload.Exports[0].Name != entry {
			t.Fatalf("fresh export does not match this request: err=%v raw=%s", err, output)
		}
		sum := sha256.Sum256(append([]byte("tsfg.nullable-facts.request.v2\x00"), input...))
		if got.RequestID != "sha256:"+hex.EncodeToString(sum[:]) {
			t.Fatal("request identity came from another acquisition")
		}
		legacy := nullableFactsBoundaryLegacy(t, r)
		if legacy.Status != "ok" || !bytes.Equal(got.Payload.Facts, nullableFactsBoundaryCanonical(t, legacy)) {
			t.Fatal("fresh package altered or reused stale legacy facts")
		}
		wantPaths := make([]string, 0, len(r.Files))
		for _, file := range r.Files {
			wantPaths = append(wantPaths, file.Path)
		}
		slices.Sort(wantPaths)
		gotPaths := make([]string, 0, len(got.Payload.Files))
		for _, file := range got.Payload.Files {
			gotPaths = append(gotPaths, file.Path)
		}
		if !slices.Equal(wantPaths, gotPaths) || !slices.Equal(wantPaths, legacy.Inputs) {
			t.Fatal("package omitted or invented a consumed input")
		}
		return output, got
	}
	first, firstPacket := acquire(r, input, "entry")
	firstBefore := bytes.Clone(first)
	// Mutate the caller-owned request DTO after completion, retaining the same
	// logical source path. The next invocation must use a new official Program.
	r.Files[0].Content = strings.Replace(nullableFactsBoundarySource, "function entry(", "function freshEntry(", 1)
	changed := nullableFactsBoundaryRequest(t, r)
	_, secondPacket := acquire(r, changed, "freshEntry")
	if bytes.Equal(changed, before) || firstPacket.PackageID == secondPacket.PackageID || firstPacket.Payload.InputIdentity == secondPacket.Payload.InputIdentity || !bytes.Equal(first, firstBefore) {
		t.Fatal("changed source reused identities or changed the earlier owned result")
	}
	again, _ := acquire(integerRequest(nullableFactsBoundarySource), before, "entry")
	if !bytes.Equal(first, again) || !bytes.Equal(input, before) {
		t.Fatal("fresh round trip retained mutable state from the intervening request")
	}
	unused := integerRequest(nullableFactsBoundarySource)
	unused.Files = append(unused.Files, inputFile{Path: "/src/unconsumed.d.ts", Content: "interface Unconsumed {}"})
	t.Run("unconsumed-file", func(t *testing.T) {
		nullableFactsBoundaryReject(t, nullableFactsBoundaryRequest(t, unused))
	})
}
