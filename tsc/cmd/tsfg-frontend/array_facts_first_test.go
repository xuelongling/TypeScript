// SPDX-License-Identifier: Apache-2.0
// Independent whole fixture controls run before either owned projection route.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	tsjson "github.com/microsoft/typescript-go/internal/json"
)

const arrayFirstOracleSHA25 = "877483469d61070970cd48a9053fb2efd2028f45d934fda326c43e4b272c1b9d"
const arrayFirstSchemaSHA25 = "fbde82e304c30461ab69a2bb0c4d488fdeea0500c8d7d3f5dcda3764b03ec47e"
const arrayFirstDtsSHA25 = "04a97584458d9212250e94d4fc517264c277a9ce635fbb14de11452194fd76b2"

type arrayFirstRange25 struct {
	Start int `json:"start"`
	End   int `json:"end"`
}
type arrayFirstValue25 struct {
	Kind           string              `json:"kind"`
	Type           string              `json:"type"`
	Range          arrayFirstRange25   `json:"range"`
	Token          string              `json:"token"`
	Bits           string              `json:"bits"`
	Binding        string              `json:"binding"`
	FlowType       string              `json:"flowType"`
	Decimal        string              `json:"decimal"`
	AllocationSite string              `json:"allocationSite"`
	Elements       []arrayFirstValue25 `json:"elements"`
	Array          *arrayFirstValue25  `json:"array"`
	Index          *arrayFirstValue25  `json:"index"`
}
type arrayFirstBinding25 struct {
	ID                     string             `json:"id"`
	Name                   string             `json:"name"`
	Kind                   string             `json:"kind"`
	DeclaredType           string             `json:"declaredType"`
	Writable               bool               `json:"writable"`
	DeclarationRange       arrayFirstRange25  `json:"declarationRange"`
	NameRange              arrayFirstRange25  `json:"nameRange"`
	AnnotationRange        *arrayFirstRange25 `json:"annotationRange"`
	ElementAnnotationRange *arrayFirstRange25 `json:"elementAnnotationRange"`
}
type arrayFirstStatement25 struct {
	Kind        string             `json:"kind"`
	Binding     string             `json:"binding"`
	Range       arrayFirstRange25  `json:"range"`
	Initializer *arrayFirstValue25 `json:"initializer"`
	Value       *arrayFirstValue25 `json:"value"`
}
type arrayFirstFunction25 struct {
	ID                    string                  `json:"id"`
	Name                  string                  `json:"name"`
	SourcePath            string                  `json:"sourcePath"`
	DeclarationRange      arrayFirstRange25       `json:"declarationRange"`
	NameRange             arrayFirstRange25       `json:"nameRange"`
	ReturnType            string                  `json:"returnType"`
	ReturnAnnotationRange arrayFirstRange25       `json:"returnAnnotationRange"`
	Parameters            []json.RawMessage       `json:"parameters"`
	Bindings              []arrayFirstBinding25   `json:"bindings"`
	Body                  []arrayFirstStatement25 `json:"body"`
}
type arrayFirstType25 struct {
	ID                  string            `json:"id"`
	Kind                string            `json:"kind"`
	ElementType         string            `json:"elementType"`
	SourcePath          string            `json:"sourcePath"`
	DeclarationRange    arrayFirstRange25 `json:"declarationRange"`
	NameRange           arrayFirstRange25 `json:"nameRange"`
	IndexSignatureRange arrayFirstRange25 `json:"indexSignatureRange"`
}
type arrayFirstCase25 struct {
	Name          string `json:"name"`
	RequestText   string `json:"requestText"`
	RequestBytes  int    `json:"requestBytes"`
	RequestSHA256 string `json:"requestSHA256"`
	PackageText   string `json:"packageText"`
	PackageBytes  int    `json:"packageBytes"`
	PackageSHA256 string `json:"packageSHA256"`
}
type arrayFirstPackage25 struct {
	Family     string `json:"family"`
	Version    int    `json:"version"`
	SchemaHash string `json:"schemaHash"`
	RequestID  string `json:"requestId"`
	PackageID  string `json:"packageId"`
	Payload    struct {
		Functions []arrayFirstFunction25 `json:"functions"`
		Types     []arrayFirstType25     `json:"types"`
		Exports   []struct {
			Name       string `json:"name"`
			FunctionID string `json:"functionId"`
		} `json:"exports"`
	} `json:"payload"`
}

func arrayFirstHash25(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func arrayFirstCanonical25(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	out := tsjson.Value(data)
	if err := out.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	return out
}
func arrayFirstFixture25(t *testing.T) []arrayFirstCase25 {
	t.Helper()
	data, err := os.ReadFile("testdata/array-facts-v1/oracle.json")
	if err != nil {
		t.Fatal(err)
	}
	if arrayFirstHash25(data) != arrayFirstOracleSHA25 {
		t.Fatal("independent whole oracle changed")
	}
	var oracle struct {
		Family  string
		Version int
		Cases   []arrayFirstCase25
	}
	if err := json.Unmarshal(data, &oracle); err != nil {
		t.Fatal(err)
	}
	if oracle.Family != "tsfg.r03.array-facts-whole-oracle" || oracle.Version != 1 || len(oracle.Cases) != 4 {
		t.Fatal("complete independent four-case oracle")
	}
	for _, tc := range oracle.Cases {
		for _, item := range []struct {
			text string
			size int
			hash string
		}{{tc.RequestText, tc.RequestBytes, tc.RequestSHA256}, {tc.PackageText, tc.PackageBytes, tc.PackageSHA256}} {
			if len(item.text) != item.size || arrayFirstHash25([]byte(item.text)) != item.hash {
				t.Fatal("whole fixture bytes/hash changed")
			}
			var parsed any
			if json.Unmarshal([]byte(item.text), &parsed) != nil || !bytes.Equal([]byte(item.text), append(arrayFirstCanonical25(t, parsed), '\n')) {
				t.Fatal("whole fixture is not JCS plus one LF")
			}
		}
	}
	return oracle.Cases
}
func arrayFirstDecode25(t *testing.T, tc arrayFirstCase25) (request, arrayFirstPackage25) {
	t.Helper()
	var envelope struct {
		Family  string
		Version int
		Input   request
	}
	var packet arrayFirstPackage25
	if json.Unmarshal([]byte(tc.RequestText), &envelope) != nil || json.Unmarshal([]byte(tc.PackageText), &packet) != nil {
		t.Fatal("complete request/package decode")
	}
	if envelope.Family != "tsfg.array-facts-request-candidate" || envelope.Version != 1 || packet.Family != "tsfg.array-facts-package-candidate" || packet.Version != 1 || packet.SchemaHash != "sha256:"+arrayFirstSchemaSHA25 {
		t.Fatal("frozen family/version/schema")
	}
	r := envelope.Input
	if len(r.Files) != 2 || len(r.Declarations.Paths) != 1 || r.Declarations.Version != "minimal-arrays-v1" || !r.Config.Strict || !r.Config.NoEmit || r.Config.AllowJS {
		t.Fatal("closed request profile")
	}
	for _, file := range r.Files {
		if arrayFirstHash25([]byte(file.Content)) != file.SHA256 {
			t.Fatal("frozen source checksum")
		}
		if file.Path == r.Declarations.Paths[0] && file.SHA256 != arrayFirstDtsSHA25 {
			t.Fatal("private DTS changed")
		}
	}
	var whole map[string]any
	if json.Unmarshal([]byte(tc.PackageText), &whole) != nil {
		t.Fatal("payload decode")
	}
	if packet.RequestID != "sha256:"+arrayFirstHash25(append([]byte("tsfg.array-facts.request.v1\x00"), []byte(tc.RequestText)...)) || packet.PackageID != "sha256:"+arrayFirstHash25(append([]byte("tsfg.array-facts.package.v1\x00"+packet.SchemaHash+"\x00"), arrayFirstCanonical25(t, whole["payload"])...)) {
		t.Fatal("independent whole identity preimages")
	}
	return r, packet
}

// Root retained the original natural RED before production. This final regression
// preserves that precise old-path capability boundary; the new command is separate.
func TestArrayFactsV1LegacyProjectionBoundary(t *testing.T) {
	cases := arrayFirstFixture25(t)
	for _, tc := range cases {
		if !t.Run(tc.Name+"/officialCompleteControls", func(t *testing.T) {
			r, packet := arrayFirstDecode25(t, tc)
			arrayFirstOfficial25(t, r, packet)
		}) {
			return
		}
	}
	r, _ := arrayFirstDecode25(t, cases[0])
	if cases[0].Name != "full-fresh-aliases" {
		t.Fatal("legacy causal input order changed")
	}
	// Legacy admission accepts this DTS content, but its ordinary projection lacks Array.
	// Keep the same exact source/DTS bytes; this control uses the old admitted profile name.
	r.Declarations.Version = "minimal-scalars-v2"
	if !valid(r) {
		t.Fatal("legacy control request must pass existing admission before causal projection")
	}
	got := analyzeWithProjection(t.Context(), r, nil)
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("TSFG_ARRAY_FACTS_V1_LEGACY_ACTUAL_BASE64=%s", base64.StdEncoding.EncodeToString(encoded))
	if got.Status != "error" || len(got.Diagnostics) != 1 {
		t.Fatalf("legacy capability boundary changed: %+v", got)
	}
	d := got.Diagnostics[0]
	if d.Code != "r02b-d-unsupported" || d.RuleID != "r01-ts-baseline" || len(d.RuleIDs) != 1 || d.RuleIDs[0] != "r01-ts-baseline" || d.Phase != "capability" || d.Category != "unsupported" || d.Severity != "error" || d.File != r.RootFile || d.Range != (span{Start: 68, End: 73}) || d.Primary.Start != 68 || d.Primary.End != 73 || d.UpstreamCode != 0 {
		t.Fatalf("exact old-path f64[] diagnostic changed: %+v", d)
	}
}

func TestArrayFactsV1CommandWholeFrozenPackage(t *testing.T) {
	for _, tc := range arrayFirstFixture25(t) {
		t.Run(tc.Name, func(t *testing.T) {
			r, packet := arrayFirstDecode25(t, tc)
			arrayFirstOfficial25(t, r, packet)
			input := []byte(tc.RequestText)
			before := bytes.Clone(input)
			var output bytes.Buffer
			err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v1"}, bytes.NewReader(input), &output)
			if tc.Name == "full-fresh-aliases" {
				t.Logf("TSFG_ARRAY_FACTS_V1_ACTUAL_PACKAGE_BASE64=%s", base64.StdEncoding.EncodeToString(output.Bytes()))
			} else {
				t.Logf("TSFG_ARRAY_FACTS_V1_CASE_ACTUAL_BASE64 name=%s data=%s", tc.Name, base64.StdEncoding.EncodeToString(output.Bytes()))
			}
			if !bytes.Equal(input, before) {
				t.Fatal("caller input changed")
			}
			if err != nil {
				if output.Len() != 0 {
					t.Fatal("failed command published partial package")
				}
				t.Fatalf("real private Array command failed: %v", err)
			}
			if !bytes.Equal(output.Bytes(), []byte(tc.PackageText)) {
				t.Fatalf("whole independent package mismatch for %s\nactual=%s\nexpected=%s", tc.Name, output.Bytes(), tc.PackageText)
			}
			t.Logf("TSFG_ARRAY_FACTS_V1_WHOLE_PASS name=%s bytes=%d sha256=%s", tc.Name, output.Len(), arrayFirstHash25(output.Bytes()))
		})
	}
}
