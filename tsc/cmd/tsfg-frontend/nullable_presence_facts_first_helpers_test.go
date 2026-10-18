// SPDX-License-Identifier: Apache-2.0
// Independent fixtures were frozen before implementation; no product encoder creates expected data.
package main

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"
)

//go:embed testdata/nullable-presence-v4/*.json
var nullablePresenceFirstData embed.FS

var nullablePresenceFirstPins = map[string]struct {
	bytes int
	sha   string
}{
	"mask1-undefined.request.json":      {2110, "b0e187dffc54b9ab7782e6f16209c353d085c66ab75a0e18e1a079e5b8ecf26a"},
	"mask1-undefined.package.json":      {4425, "cb9d5035b28db19e89c23e058e9dd4b40c4f54993768f3141f3e206a5a6481d2"},
	"mask1-undefined.legacy.json":       {3504, "82f97a74b24380f21889053b87821ae9638085d302e4a8efca4fd73b60eb1143"},
	"mask2-null.request.json":           {2100, "c2f52a8754558d246cb4ea8ccb8ccb6c02f5e467bdab9e4b66310338809fc4a1"},
	"mask2-null.package.json":           {4418, "2b10b9d878bca8b1dbb519776860a7bc796676fcc4e5b7d023769d50bccad0b0"},
	"mask2-null.legacy.json":            {3497, "22fc104b52e64d694e871d515373d0d03f1b8e7af10fcbb90df6752f8e52138f"},
	"mask3-null-undefined.request.json": {2161, "f88eabf9b37a5304431af8969225a6818e92bef3066b1c835922fa2632084b90"},
	"mask3-null-undefined.package.json": {5006, "537fb13740ded5481740f41cf79f98629af53e5d8cdcf1bdd4d6bda474aee3e2"},
	"mask3-null-undefined.legacy.json":  {4085, "f4d5f3b773250ed70f789c8f2d3aac9bace06d4deae511e71eb9da37429a1bcd"},
	"mask3-undefined-null.request.json": {2161, "476dbae85716aec0432f9db592f8c51e766978e9333b0139cb5235c68bda580a"},
	"mask3-undefined-null.package.json": {5006, "1f30d1ddaba4f5d79d07020cbfe531d4e527beb603e0e92d0ed4a971e9593122"},
	"mask3-undefined-null.legacy.json":  {4085, "11cfbfec77abdd905f23e17240f7b075f8024b214c1b539130734dbd93b375d2"},
	"ranges.json":                       {20563, "64921c3f2633ca98aedd0ea824e0edf9ee28854c0a973be692261625f4a108f6"},
	"schema.json":                       {12783, "4205bc18e817d983dd0f91d5b1c1d92b99cff5f6e32eca419d22530bfdcf41ac"},
}

type nullablePresenceFirstAnchor struct {
	Range [2]int `json:"range"`
	Text  string `json:"text"`
}

type nullablePresenceFirstGuard struct {
	Absence string                                 `json:"absence"`
	UseMask int                                    `json:"useMask"`
	Anchors map[string]nullablePresenceFirstAnchor `json:"anchors"`
}

type nullablePresenceFirstRanges struct {
	Key         string                                 `json:"key"`
	SourceSHA   string                                 `json:"sourceSha256"`
	UTF8Bytes   int                                    `json:"utf8Bytes"`
	UTF16Length int                                    `json:"utf16Length"`
	LF          int                                    `json:"lf"`
	CR          int                                    `json:"cr"`
	FinalLF     bool                                   `json:"finalLF"`
	Mask        int                                    `json:"declaredMask"`
	GuardMasks  []int                                  `json:"guardUseMasks"`
	TailMask    int                                    `json:"tailUseMask"`
	Anchors     map[string]nullablePresenceFirstAnchor `json:"anchors"`
	Guards      []nullablePresenceFirstGuard           `json:"guards"`
}

type nullablePresenceFirstFixture struct {
	name                                    string
	r                                       request
	ranges                                  nullablePresenceFirstRanges
	requestBytes, packageBytes, legacyBytes []byte
}

func nullablePresenceFirstDigest(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func nullablePresenceFirstRead(t *testing.T, name string) []byte {
	t.Helper()
	pin, ok := nullablePresenceFirstPins[name]
	if !ok {
		t.Fatalf("unlisted independent fixture %s", name)
	}
	data, err := nullablePresenceFirstData.ReadFile("testdata/nullable-presence-v4/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != pin.bytes || nullablePresenceFirstDigest(data) != pin.sha {
		t.Fatalf("independent fixture changed: %s bytes=%d sha=%s", name, len(data), nullablePresenceFirstDigest(data))
	}
	if !json.Valid(data) {
		t.Fatalf("invalid independent JSON %s", name)
	}
	if name != "ranges.json" && (bytes.HasPrefix(data, []byte{239, 187, 191}) || bytes.HasSuffix(data, []byte("\n"))) {
		t.Fatalf("wire fixture must have no BOM or final LF: %s", name)
	}
	return data
}

func nullablePresenceFirstJSON(t *testing.T, raw []byte) any {
	t.Helper()
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("invalid complete JSON: %v raw=%s", err, raw)
	}
	return value
}

func nullablePresenceFirstEqualJSON(t *testing.T, actual, expected []byte) {
	t.Helper()
	if !reflect.DeepEqual(nullablePresenceFirstJSON(t, actual), nullablePresenceFirstJSON(t, expected)) {
		t.Fatalf("complete JSON differs: actual=%s expected=%s", actual, expected)
	}
}

func nullablePresenceFirstFixtures(t *testing.T) []nullablePresenceFirstFixture {
	t.Helper()
	for name := range nullablePresenceFirstPins {
		nullablePresenceFirstRead(t, name)
	}
	var table struct {
		Cases []nullablePresenceFirstRanges `json:"cases"`
	}
	if err := json.Unmarshal(nullablePresenceFirstRead(t, "ranges.json"), &table); err != nil {
		t.Fatal(err)
	}
	if len(table.Cases) != 4 {
		t.Fatal("complete independent table must retain all four cases")
	}
	order := []string{"mask1-undefined", "mask2-null", "mask3-null-undefined", "mask3-undefined-null"}
	fixtures := make([]nullablePresenceFirstFixture, 0, 4)
	for i, spec := range table.Cases {
		if spec.Key != order[i] || len(spec.Anchors) != 43 || len(spec.Guards) != len(spec.GuardMasks) || spec.TailMask != 0 {
			t.Fatal("independent case order, complete anchors or flow constants changed")
		}
		f := nullablePresenceFirstFixture{name: spec.Key, ranges: spec,
			requestBytes: nullablePresenceFirstRead(t, spec.Key+".request.json"),
			packageBytes: nullablePresenceFirstRead(t, spec.Key+".package.json"),
			legacyBytes:  nullablePresenceFirstRead(t, spec.Key+".legacy.json")}
		var envelope struct {
			Family  string          `json:"family"`
			Version int             `json:"version"`
			Schema  string          `json:"schemaHash"`
			Input   json.RawMessage `json:"input"`
		}
		if err := json.Unmarshal(f.requestBytes, &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Family != "tsfg.nullable-facts-request-candidate" || envelope.Version != 4 || envelope.Schema != "sha256:4205bc18e817d983dd0f91d5b1c1d92b99cff5f6e32eca419d22530bfdcf41ac" {
			t.Fatal("independent exact v4 request metadata changed")
		}
		if err := json.Unmarshal(envelope.Input, &f.r); err != nil {
			t.Fatal(err)
		}
		if f.r.SourceRoot != "/src" || f.r.RootFile != "/src/main.ts" || len(f.r.Files) != 2 || f.r.Files[0].Path != f.r.RootFile || f.r.Files[1].Path != "/src/minimal-integers-v3.d.ts" {
			t.Fatal("independent original two-file request closure changed")
		}
		encoded, err := json.Marshal(f.r)
		if err != nil {
			t.Fatal(err)
		}
		nullablePresenceFirstEqualJSON(t, encoded, envelope.Input)
		source := f.r.Files[0].Content
		if !utf8.ValidString(source) || len(source) != spec.UTF8Bytes || len(utf16.Encode([]rune(source))) != spec.UTF16Length || nullablePresenceFirstDigest([]byte(source)) != spec.SourceSHA || strings.Count(source, "\n") != spec.LF || strings.Count(source, "\r") != spec.CR || spec.CR != 1 || strings.HasSuffix(source, "\n") != spec.FinalLF || spec.FinalLF || !strings.HasPrefix(source, "// 🧭 20k\r\n") {
			t.Fatal("original Source bytes, CRLF/emoji UTF16 or no-final-LF changed")
		}
		dts := []byte(f.r.Files[1].Content)
		if len(dts) != 1214 || nullablePresenceFirstDigest(dts) != "9fc1955899e303b5815590f7150f60662f3cece0a633c94a2a06669cf509a96e" {
			t.Fatal("complete existing integer DTS changed")
		}
		fixtures = append(fixtures, f)
	}
	return fixtures
}

func nullablePresenceFirstSourceSlice(t *testing.T, source string, where [2]int) string {
	t.Helper()
	units := utf16.Encode([]rune(source))
	if where[0] < 0 || where[1] <= where[0] || where[1] > len(units) {
		t.Fatalf("bad independent UTF16 range %+v", where)
	}
	for _, at := range where {
		if at > 0 && at < len(units) && units[at] >= 0xdc00 && units[at] <= 0xdfff && units[at-1] >= 0xd800 && units[at-1] <= 0xdbff {
			t.Fatalf("range splits original Source surrogate pair: %+v", where)
		}
	}
	return string(utf16.Decode(units[where[0]:where[1]]))
}

func nullablePresenceFirstLegacy(t *testing.T, f nullablePresenceFirstFixture) {
	t.Helper()
	input, err := json.Marshal(f.r)
	if err != nil {
		t.Fatal(err)
	}
	before := bytes.Clone(input)
	var output bytes.Buffer
	if err := runFrontendCommand(t.Context(), nil, bytes.NewReader(input), &output); err != nil {
		t.Fatalf("legacy public command: %v raw=%s", err, output.Bytes())
	}
	if !bytes.Equal(input, before) {
		t.Fatal("legacy command mutated caller request bytes")
	}
	actual := bytes.Clone(output.Bytes())
	t.Logf("TSFG_20K_FIRST_LEGACY %s %s", f.name, base64.StdEncoding.EncodeToString(actual))
	// Compare the whole graph. Legacy Encoder field order/framing is not private JCS framing.
	nullablePresenceFirstEqualJSON(t, actual, f.legacyBytes)
}
