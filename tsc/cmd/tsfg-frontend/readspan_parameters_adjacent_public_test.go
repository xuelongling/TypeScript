// SPDX-License-Identifier: Apache-2.0
// Public IO consumes independently frozen adjacent expectations, never producer goldens.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"unicode/utf16"
)

func TestReadSpan26bV6AdjacentPublicWholeFrozen(t *testing.T) {
	read := func(name string, size int, pin string) []byte {
		t.Helper()
		path := "testdata/readspan-parameters-v6/" + name
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(raw) != size || readSpanParameterPublicHash26b(raw) != pin {
			t.Fatalf("independent adjacent fixture changed: %s", name)
		}
		before := bytes.Clone(raw)
		t.Cleanup(func() {
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(after, before) || !bytes.Equal(raw, before) {
				t.Errorf("independent adjacent fixture mutated: %s err=%v", name, err)
			}
		})
		return raw
	}
	inputRaw := read("adjacent-inputs.json", 92072, "4ce5bddf8bede5e800798733876ce3d38ef9479e1b794b3ec027045dc9a66883")
	expectedRaw := read("adjacent-expected-facts.json", 159931, "54d5a043b0e1ac9f26d3abd852179da969bf382f92dd9adc3d208eab79213c5e")
	var input struct {
		Cases []struct {
			ID             string `json:"id"`
			Text           string `json:"text"`
			SourcePath     string `json:"sourcePath"`
			SHA256         string `json:"sha256"`
			Bytes          int    `json:"bytes"`
			UTF16Length    int    `json:"utf16Length"`
			SourceExpected struct {
				Outcome, Code, Phase string
				PrimaryCandidate     struct {
					SourcePath string
					Range      struct{ Start, End int }
				}
			} `json:"sourceExpected"`
		} `json:"cases"`
	}
	var expected struct {
		Cases []struct {
			Name                      string `json:"name"`
			SourceExpected            string `json:"sourceExpected"`
			RequestText               string `json:"requestText"`
			PackageText               string `json:"packageText"`
			ExpectedFactsWrites       int    `json:"expectedFactsWrites"`
			SelectedPacketConstructed bool   `json:"selectedPacketConstructed"`
			ExpectedSourceError       struct {
				Code, RuleID, Phase, Category, Severity string
				Primary                                 struct {
					SourceID   string `json:"sourceId"`
					Start, End int
				}
			} `json:"expectedSourceError"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(inputRaw, &input); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(expectedRaw, &expected); err != nil {
		t.Fatal(err)
	}
	if len(input.Cases) != 16 || len(expected.Cases) != 16 {
		t.Fatal("all sixteen independent adjacent cases are required")
	}
	accepted, rejected := 0, 0
	for i, item := range expected.Cases {
		source := input.Cases[i]
		if item.Name != source.ID || item.SourceExpected != source.SourceExpected.Outcome ||
			len(source.Text) != source.Bytes || readSpanParameterPublicHash26b([]byte(source.Text)) != source.SHA256 ||
			len(utf16.Encode([]rune(source.Text))) != source.UTF16Length {
			t.Fatalf("whole independent source/request join changed: %s", item.Name)
		}
		switch item.SourceExpected {
		case "accept-complete-facts":
			accepted++
		case "capability":
			rejected++
		default:
			t.Fatalf("unselected independent expectation: %s", item.SourceExpected)
		}
		t.Run(item.Name, func(t *testing.T) {
			request := []byte(item.RequestText)
			requestBefore := bytes.Clone(request)
			if len(request) == 0 {
				t.Fatal("complete frozen request is required")
			}
			var output readSpanParameterPublicWriter26b
			err := runArrayFactsCandidate(t.Context(), bytes.NewReader(request), &output)
			t.Logf("TSFG_READSPAN_26B_ADJACENT_PUBLIC_IO name=%s requestBytes=%d requestSHA256=%s actualBytes=%d actualSHA256=%s writes=%d",
				item.Name, len(request), readSpanParameterPublicHash26b(request), output.Len(), readSpanParameterPublicHash26b(output.Bytes()), output.writes)
			if !bytes.Equal(request, requestBefore) {
				t.Errorf("caller request mutated: %s", readSpanParameterPublicError26b(err))
			}
			if item.SourceExpected == "accept-complete-facts" {
				if err != nil {
					t.Fatalf("independently official-valid adjacent source rejected: %s writes=%d bytes=%d actualBase64=%s",
						readSpanParameterPublicError26b(err), output.writes, output.Len(), base64.StdEncoding.EncodeToString(output.Bytes()))
				}
				packet := []byte(item.PackageText)
				if len(packet) == 0 || output.writes != 1 || !bytes.Equal(output.Bytes(), packet) {
					t.Fatalf("complete independent packet/one-write mismatch: expectedBytes=%d expectedSHA256=%s writes=%d actualBase64=%s",
						len(packet), readSpanParameterPublicHash26b(packet), output.writes, base64.StdEncoding.EncodeToString(output.Bytes()))
				}
				t.Logf("TSFG_READSPAN_26B_ADJACENT_PUBLIC_ACTUAL_PACKET name=%s bytes=%d sha256=%s base64=%s",
					item.Name, output.Len(), readSpanParameterPublicHash26b(output.Bytes()), base64.StdEncoding.EncodeToString(output.Bytes()))
				return
			}
			want := item.ExpectedSourceError
			candidate := source.SourceExpected.PrimaryCandidate
			if item.ExpectedFactsWrites != 0 || item.SelectedPacketConstructed || item.PackageText != "" ||
				want.Code != source.SourceExpected.Code || want.Phase != source.SourceExpected.Phase ||
				want.Primary.SourceID != source.SourcePath || want.Primary.SourceID != candidate.SourcePath ||
				want.Primary.Start != candidate.Range.Start || want.Primary.End != candidate.Range.End {
				t.Fatal("independent capability/error anchor join changed")
			}
			var failure *arrayFactsFailure
			if !errors.As(err, &failure) || failure.Code != want.Code || failure.Phase != want.Phase ||
				failure.RuleID != want.RuleID || len(failure.Diagnostics) != 1 || output.writes != 0 || output.Len() != 0 {
				t.Fatalf("capability refusal/zero publication mismatch: %s writes=%d bytes=%d",
					readSpanParameterPublicError26b(err), output.writes, output.Len())
			}
			d := failure.Diagnostics[0]
			if d.Code != want.Code || d.RuleID != want.RuleID || d.Phase != want.Phase ||
				d.Category != want.Category || d.Severity != want.Severity || d.UpstreamCode != 0 ||
				len(d.RuleIDs) != 1 || d.RuleIDs[0] != want.RuleID || d.File != want.Primary.SourceID ||
				d.Range.Start != want.Primary.Start || d.Range.End != want.Primary.End ||
				d.Primary.SourceID != want.Primary.SourceID || d.Primary.Start != want.Primary.Start || d.Primary.End != want.Primary.End {
				t.Fatalf("original frozen capability diagnostic/primary mismatch: expected=%+v actual=%+v", want, d)
			}
			t.Logf("TSFG_READSPAN_26B_ADJACENT_PUBLIC_CAPABILITY name=%s code=%s phase=%s source=%s start=%d end=%d writes=%d bytes=%d",
				item.Name, failure.Code, failure.Phase, d.Primary.SourceID, d.Primary.Start, d.Primary.End, output.writes, output.Len())
		})
	}
	if accepted != 12 || rejected != 4 {
		t.Fatalf("complete adjacent selection changed: accepted=%d rejected=%d", accepted, rejected)
	}
}
