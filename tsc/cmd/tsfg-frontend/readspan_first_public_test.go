// SPDX-License-Identifier: Apache-2.0
// Whole packets were frozen from fresh official objects before any v4 producer output.
// A missing v4 mode is a protocol-migration RED, never a borrow-consumer RED.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"testing"
)

type readSpanFirstPublicWriter26 struct {
	bytes.Buffer
	writes int
}

func (w *readSpanFirstPublicWriter26) Write(data []byte) (int, error) {
	w.writes++
	return w.Buffer.Write(data)
}
func readSpanFirstPublicHash26(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
func TestReadSpan26aV4PublicWholeFrozenPair(t *testing.T) {
	for _, item := range []struct{ name, requestHash, packageHash string }{
		{"before", "d2020f72eb5ab4944d4a7a96e511a4e30fc282b9cdcc7f835d05dfaaa87d3d26", "5832c826f5907114dfa921a5ef5595d4dbe3eedf53ba364980b015fc0e231c07"},
		{"after", "731ca4cf803970886ca1c7c7b6c1ee60bfe62a1588c4b9434478ac798d9c0e0f", "1b03a5ad4263f055c1fa0545f9a17ca53fe63742c4f79ed07d62ad6d0f3d1357"},
	} {
		t.Run(item.name, func(t *testing.T) {
			prefix := "testdata/readspan-last-use-v4/" + item.name
			request, err := os.ReadFile(prefix + "-request.json")
			if err != nil {
				t.Fatal(err)
			}
			expected, err := os.ReadFile(prefix + "-package.json")
			if err != nil {
				t.Fatal(err)
			}
			if len(request) != 1291 || readSpanFirstPublicHash26(request) != item.requestHash || len(expected) != 9334 || readSpanFirstPublicHash26(expected) != item.packageHash {
				t.Fatal("complete pre-production independent request/package fixture changed")
			}
			inputBefore := bytes.Clone(request)
			var output readSpanFirstPublicWriter26
			err = runFrontendCommand(t.Context(), []string{"--array-facts-io-v4"}, bytes.NewReader(request), &output)
			t.Logf("TSFG_READSPAN_26A_PUBLIC_REQUEST name=%s bytes=%d sha256=%s base64=%s", item.name, len(request), readSpanFirstPublicHash26(request), base64.StdEncoding.EncodeToString(request))
			t.Logf("TSFG_READSPAN_26A_PUBLIC_ACTUAL name=%s bytes=%d sha256=%s writes=%d base64=%s", item.name, output.Len(), readSpanFirstPublicHash26(output.Bytes()), output.writes, base64.StdEncoding.EncodeToString(output.Bytes()))
			if !bytes.Equal(request, inputBefore) {
				t.Fatal("caller input mutated")
			}
			if err != nil {
				if output.Len() != 0 || output.writes != 0 {
					t.Fatalf("failure published partial facts: writes=%d bytes=%d error=%v", output.writes, output.Len(), err)
				}
				t.Fatalf("official-valid complete source must publish v4 facts (protocol migration, not borrow decision): %v", err)
			}
			if output.writes != 1 {
				t.Fatalf("whole facts publication requires one complete write, got %d", output.writes)
			}
			if !bytes.Equal(output.Bytes(), expected) {
				t.Fatal("complete output differs from pre-production independent packet")
			}
		})
	}
}
