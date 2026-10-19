// SPDX-License-Identifier: Apache-2.0
// Root preserved the original v1 FIRST and its actual causal capability RED.
// This active regression adopts v5 and compares the same source's complete facts.
package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

const arrayDenseFirstSource25d = "export function overwrite(): f64 {\n  const a: f64[] = [1.0, 2.0];\n  a[1] = 3.0;\n  return a[0];\n}\n"

func TestArrayDenseMutation25dFirstPublicSourceSuccess(t *testing.T) {
	cases := arrayFirstFixture25(t)
	if len(cases) != 8 || cases[4].Name != "dense-overwrite" {
		t.Fatal("complete v5 corpus and same-source case order")
	}
	for _, tc := range cases[:5] {
		if !t.Run(tc.Name, func(t *testing.T) {
			r, packet := arrayFirstDecode25(t, tc)
			if tc.Name == "dense-overwrite" {
				found := false
				for _, file := range r.Files {
					found = found || file.Path == r.RootFile && file.Content == arrayDenseFirstSource25d
				}
				if !found {
					t.Fatal("same original legal dense overwrite source required")
				}
			}
			arrayFirstOfficial25(t, r, packet)
			input := []byte(tc.RequestText)
			before := bytes.Clone(input)
			var output bytes.Buffer
			err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v5"}, bytes.NewReader(input), &output)
			if !bytes.Equal(input, before) {
				t.Fatal("caller input changed")
			}
			if err != nil {
				if output.Len() != 0 {
					t.Fatal("failed public command published a partial package")
				}
				t.Fatalf("ordinary legal source must succeed through adopted public Source v5 command: %v", err)
			}
			if !bytes.Equal(output.Bytes(), []byte(tc.PackageText)) {
				t.Logf("TSFG_ARRAY_DENSE_25D_V5_ACTUAL_BASE64=%s", base64.StdEncoding.EncodeToString(output.Bytes()))
				t.Fatalf("complete independent v5 package mismatch: %s", tc.Name)
			}
			t.Logf("TSFG_ARRAY_DENSE_25D_V5_WHOLE_PASS name=%s bytes=%d sha256=%s", tc.Name, output.Len(), arrayFirstHash25(output.Bytes()))
		}) {
			return
		}
	}
}
