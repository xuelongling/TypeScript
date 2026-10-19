// SPDX-License-Identifier: Apache-2.0
// Independent whole fixture controls precede the selected public projection.
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

const arrayFirstOracleSHA25 = "36f1ef530b7a90f6eaa0d078bb47d98e335f30386bbc6a475ceb40af930119a6"
const arrayFirstSchemaSHA25 = "1364677f060da070ab1e14a589c1957d9ece10eaf8eb81744cea2a99e410a373"
const arrayFirstDtsSHA25 = "282b335a6e0e0493fa74a3c1dd56f234f8c679079acdaf0600cefb8306c26415"

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
	Property       string              `json:"property"`
	NameRange      arrayFirstRange25   `json:"nameRange"`
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
type arrayFirstParameter25 struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Kind             string            `json:"kind"`
	DeclaredType     string            `json:"declaredType"`
	Writable         bool              `json:"writable"`
	DeclarationRange arrayFirstRange25 `json:"declarationRange"`
	NameRange        arrayFirstRange25 `json:"nameRange"`
	AnnotationRange  arrayFirstRange25 `json:"annotationRange"`
}
type arrayFirstProperty25 struct {
	ID               string            `json:"id"`
	Kind             string            `json:"kind"`
	Name             string            `json:"name"`
	OwnerType        string            `json:"ownerType"`
	OfficialType     string            `json:"officialType"`
	WriteType        string            `json:"writeType"`
	SourcePath       string            `json:"sourcePath"`
	DeclarationRange arrayFirstRange25 `json:"declarationRange"`
	NameRange        arrayFirstRange25 `json:"nameRange"`
	TypeRange        arrayFirstRange25 `json:"typeRange"`
}
type arrayFirstStatement25 struct {
	ExpressionRange arrayFirstRange25  `json:"expressionRange"`
	OperatorRange   arrayFirstRange25  `json:"operatorRange"`
	Target          *arrayFirstValue25 `json:"target"`
	Kind            string             `json:"kind"`
	Binding         string             `json:"binding"`
	Range           arrayFirstRange25  `json:"range"`
	Initializer     *arrayFirstValue25 `json:"initializer"`
	Value           *arrayFirstValue25 `json:"value"`
}
type arrayFirstFunction25 struct {
	ID                    string                  `json:"id"`
	Name                  string                  `json:"name"`
	SourcePath            string                  `json:"sourcePath"`
	DeclarationRange      arrayFirstRange25       `json:"declarationRange"`
	NameRange             arrayFirstRange25       `json:"nameRange"`
	ReturnType            string                  `json:"returnType"`
	ReturnAnnotationRange arrayFirstRange25       `json:"returnAnnotationRange"`
	Parameters            []arrayFirstParameter25 `json:"parameters"`
	Bindings              []arrayFirstBinding25   `json:"bindings"`
	Body                  []arrayFirstStatement25 `json:"body"`
}
type arrayFirstType25 struct {
	Readonly            bool              `json:"readonly"`
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
		ReadSpanLength    json.RawMessage        `json:"readSpanLength"`
		ReadSpanIntrinsic json.RawMessage        `json:"readSpanIntrinsic"`
		ArrayLength       arrayFirstProperty25   `json:"arrayLength"`
		Functions         []arrayFirstFunction25 `json:"functions"`
		Types             []arrayFirstType25     `json:"types"`
		Exports           []struct {
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
	data, err := os.ReadFile("testdata/array-facts-v4/oracle.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 86843 || arrayFirstHash25(data) != arrayFirstOracleSHA25 {
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
	if oracle.Family != "tsfg.r03.array-facts-whole-oracle" || oracle.Version != 4 || len(oracle.Cases) != 8 {
		t.Fatal("complete independent eight-case oracle")
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
	if envelope.Family != "tsfg.array-facts-request-candidate" || envelope.Version != 4 || packet.Family != "tsfg.array-facts-package-candidate" || packet.Version != 4 || packet.SchemaHash != "sha256:"+arrayFirstSchemaSHA25 {
		t.Fatal("frozen family/version/schema")
	}
	r := envelope.Input
	if len(r.Files) != 2 || len(r.Declarations.Paths) != 1 || r.Declarations.Version != "minimal-arrays-v3" || !r.Config.Strict || !r.Config.NoEmit || r.Config.AllowJS {
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
	if packet.RequestID != "sha256:"+arrayFirstHash25(append([]byte("tsfg.array-facts.request.v4\x00"), []byte(tc.RequestText)...)) || packet.PackageID != "sha256:"+arrayFirstHash25(append([]byte("tsfg.array-facts.package.v4\x00"+packet.SchemaHash+"\x00"), arrayFirstCanonical25(t, whole["payload"])...)) {
		t.Fatal("independent whole identity preimages")
	}
	return r, packet
}

// Root preserved the v1 causal RED before the atomic v4 adoption. The active
// command rejects its original envelope rather than providing a compatibility path.
func TestArrayFactsV4RejectsV1Request(t *testing.T) {
	input, err := os.ReadFile("testdata/array-dense-v2/first-v1-request.json")
	if err != nil || len(input) != 935 || arrayFirstHash25(input) != "e18ca6f7ed8c6b5da40e6e57b5ec137218d25ee2067b2e1c38b2f9a5c0ed78aa" {
		t.Fatalf("frozen baseline request: %v", err)
	}
	var output arrayBoundaryWriter25
	err = runFrontendCommand(t.Context(), []string{"--array-facts-io-v4"}, bytes.NewReader(input), &output)
	owned := arrayBoundaryFailure25(t, err)
	if owned.Code != "r03-array-facts-schema" || owned.Phase != "input" || owned.RuleID != "r01-input-boundary" || output.calls != 0 || output.Len() != 0 {
		t.Fatalf("v1 request must fail at the v4 schema before publication: %+v", owned)
	}
}

func TestArrayFactsV4CommandWholeFrozenPackage(t *testing.T) {
	for _, tc := range arrayFirstFixture25(t) {
		t.Run(tc.Name, func(t *testing.T) {
			r, packet := arrayFirstDecode25(t, tc)
			arrayFirstOfficial25(t, r, packet)
			input := []byte(tc.RequestText)
			before := bytes.Clone(input)
			var output bytes.Buffer
			err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v4"}, bytes.NewReader(input), &output)
			if tc.Name == "full-fresh-aliases" {
				t.Logf("TSFG_ARRAY_FACTS_V4_ACTUAL_PACKAGE_BASE64=%s", base64.StdEncoding.EncodeToString(output.Bytes()))
			} else {
				t.Logf("TSFG_ARRAY_FACTS_V4_CASE_ACTUAL_BASE64 name=%s data=%s", tc.Name, base64.StdEncoding.EncodeToString(output.Bytes()))
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
			t.Logf("TSFG_ARRAY_FACTS_V4_WHOLE_PASS name=%s bytes=%d sha256=%s", tc.Name, output.Len(), arrayFirstHash25(output.Bytes()))
		})
	}
}
