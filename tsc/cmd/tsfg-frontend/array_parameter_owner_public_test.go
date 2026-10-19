// SPDX-License-Identifier: Apache-2.0
// Public Source output is compared with complete pre-production independent packets.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type arrayOwnerFrozenCase23e struct {
	Name           string
	RequestText    string
	PackageText    string
	ContractSHA256 string
}

func arrayOwnerFrozenBundles23e(t *testing.T) []arrayOwnerFrozenCase23e {
	t.Helper()
	data, err := os.ReadFile("testdata/array-facts-v6/owner10-official-packages.json")
	if err != nil || len(data) != 1073962 || arrayFirstHash25(data) != "771e0363f1f45f6b050711be919b07a459dcbe99006cd75e679809f7eeba776d" {
		t.Fatalf("independent whole Source bundle bytes: %v", err)
	}
	var bundle struct{ Cases []arrayOwnerFrozenCase23e }
	if json.Unmarshal(data, &bundle) != nil || len(bundle.Cases) != 10 {
		t.Fatal("complete ten-case independent Source corpus")
	}
	for _, item := range bundle.Cases {
		if item.ContractSHA256 != "0b0cbffa7efc0d44589ba45bc71f5380d7cc7a089a6303dc2f224e9cc7505081" {
			t.Fatal("frozen independent contract changed")
		}
	}
	return bundle.Cases
}
func TestArrayParameterOwner23ePublicWholeFrozenPackets(t *testing.T) {
	for _, item := range arrayOwnerFrozenBundles23e(t) {
		t.Run(item.Name, func(t *testing.T) {
			input := []byte(item.RequestText)
			before := bytes.Clone(input)
			var output bytes.Buffer
			err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v6"}, bytes.NewReader(input), &output)
			if !bytes.Equal(before, input) {
				t.Fatal("caller input mutated")
			}
			if err != nil {
				if output.Len() != 0 {
					t.Fatal("failed publication produced partial packet")
				}
				t.Fatalf("ordinary complete source must publish v6 facts: %v", err)
			}
			if !bytes.Equal(output.Bytes(), []byte(item.PackageText)) {
				t.Logf("TSFG_ARRAY_OWNER_23E_MISMATCH name=%s actual=%s", item.Name, base64.StdEncoding.EncodeToString(output.Bytes()))
				t.Fatal("complete pre-production independent packet differs")
			}
			t.Logf("TSFG_ARRAY_OWNER_23E_PUBLIC_PACKET name=%s bytes=%d sha256=%s base64=%s", item.Name, output.Len(), arrayFirstHash25(output.Bytes()), base64.StdEncoding.EncodeToString(output.Bytes()))
		})
	}
}
func TestArrayParameterOwner23eRetiredV2Rejected(t *testing.T) {
	input, err := os.ReadFile("testdata/array-parameter-owner-v3/first-v2-request.json")
	if err != nil || len(input) != 1190 || arrayFirstHash25(input) != "188c92bbcb6411a156567d4d49e827813ced26fd27169261d53669eebb30471b" {
		t.Fatalf("unchanged historical FIRST input: %v", err)
	}
	var output bytes.Buffer
	err = runFrontendCommand(t.Context(), []string{"--array-facts-io-v6"}, bytes.NewReader(input), &output)
	failure := arrayBoundaryFailure25(t, err)
	if failure.Code != "r03-array-facts-schema" || failure.Phase != "input" || output.Len() != 0 {
		t.Fatalf("retired v2 envelope must be rejected, not adapted: %+v", failure)
	}
	reader, writer := &arrayBoundaryReader25{input: bytes.NewReader(input)}, &arrayBoundaryWriter25{}
	if err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v2"}, reader, writer); err == nil || reader.calls != 0 || writer.calls != 0 {
		t.Fatal("retired flag touched IO or retained a shim")
	}
}
func TestArrayParameterOwner23eCallLimitsAndUnsignedArguments(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"calls32", "function leaf(a: f64[]): f64 { return a[0]; }\nexport function entry(a: f64[]): f64 {" + strings.Repeat("leaf(a);", 31) + "return leaf(a);}", ""},
		{"calls33", "function leaf(a: f64[]): f64 { return a[0]; }\nexport function entry(a: f64[]): f64 {" + strings.Repeat("leaf(a);", 32) + "return leaf(a);}", "r03-array-facts-limit"},
		{"fullU32CallLiteral", "function leaf(a: f64[], i: u32): f64 { return a[i]; }\nexport function entry(a: f64[]): f64 {return leaf(a,4294967295);}", ""},
		{"overflowU32CallLiteral", "function leaf(a: f64[], i: u32): f64 { return a[i]; }\nexport function entry(a: f64[]): f64 {return leaf(a,4294967296);}", "r03-array-facts-capability"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := arrayBoundarySource25(t, tc.source)
			arrayDomainOfficialZero25(t, input)
			output, failure := arrayBoundaryCall25(t, input)
			if tc.want == "" {
				if failure != nil || len(output) == 0 {
					t.Fatalf("legal exact call boundary rejected: %+v", failure)
				}
				if tc.name == "fullU32CallLiteral" && !bytes.Contains(output, []byte("\"kind\":\"u32Literal\"")) {
					t.Fatal("full-domain ordinary u32 argument role not preserved")
				}
				return
			}
			if failure == nil || failure.Code != tc.want || len(output) != 0 {
				t.Fatalf("exact call boundary failure: %+v", failure)
			}
		})
	}
}
func TestArrayParameterOwner23eUnsupportedCallsStayLocated(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"selfRecursion", "export function entry(a: f64[]): f64 { return entry(a); }"},
		{"mutualRecursion", "function a(x: f64[]): f64 { return b(x); } export function b(x: f64[]): f64 { return a(x); }"},
		{"temporaryArray", "function leaf(a: f64[]): f64 { return a[0]; } export function entry(): f64 { return leaf([1.0]); }"},
		{"nestedRHS", "function leaf(a: f64[]): f64 { return a[0]; } export function entry(a: f64[]): f64 { a[0]=leaf(a);return a[0]; }"},
		{"nestedArgument", "function leaf(a: f64[],v: f64): f64 { return v; } export function entry(a: f64[]): f64 { return leaf(a,leaf(a,1.0)); }"},
		{"checkedReadArgument", "function leaf(a: f64[],v: f64): f64 { return v; } export function entry(a: f64[]): f64 { return leaf(a,a[0]); }"},
		{"sameNumberWrongScalarOrigin", "function leaf(a: f64[],v: f64): f64 { return v; } export function entry(a: f64[],i: u32): f64 { return leaf(a,i); }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := arrayBoundarySource25(t, tc.source)
			arrayDomainOfficialZero25(t, input)
			output, failure := arrayBoundaryCall25(t, input)
			if failure == nil || failure.Code != "r03-array-facts-capability" || failure.Phase != "capability" || len(output) != 0 || len(failure.Diagnostics) != 1 || failure.Diagnostics[0].UpstreamCode != 0 || failure.Diagnostics[0].Range.Start >= failure.Diagnostics[0].Range.End {
				t.Fatalf("official-valid unsupported call requires exact private located capability: %+v", failure)
			}
		})
	}
}

func TestArrayParameterOwner23eRetiredV3Rejected(t *testing.T) {
	input, err := os.ReadFile("testdata/array-parameter-owner-v3/first-v3-request.json")
	if err != nil || len(input) != 1190 || arrayFirstHash25(input) != "d69ea6a2a9fafe0d92a9f0fa34a575ec8260ed0ab6d2c44303104189665195c2" {
		t.Fatalf("unchanged historical v3 request: %v", err)
	}
	var output arrayBoundaryWriter25
	err = runFrontendCommand(t.Context(), []string{"--array-facts-io-v6"}, bytes.NewReader(input), &output)
	failure := arrayBoundaryFailure25(t, err)
	if failure.Code != "r03-array-facts-schema" || failure.Phase != "input" || output.Len() != 0 || output.calls != 0 {
		t.Fatalf("retired v3 envelope must fail before publication: %+v", failure)
	}
	reader, writer := &arrayBoundaryReader25{input: bytes.NewReader(input)}, &arrayBoundaryWriter25{}
	if err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v3"}, reader, writer); err == nil || reader.calls != 0 || writer.calls != 0 {
		t.Fatal("retired v3 flag touched IO or retained a shim")
	}
}
