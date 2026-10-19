// SPDX-License-Identifier: Apache-2.0
// Historical official-only v4 payloads are unchanged; only the frozen envelope
// and identity domains are migrated to v5 by pure data, without producer output.
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
func TestReadSpan26aV5PublicWholeFrozenPair(t *testing.T) {
	for _, item := range []struct{ name, requestHash, packageHash string }{
		{"before", "3f2dbdffa9a3c370d159e9b8dce63a511a9c711bb59f93922b94be82c0bdda90", "6ab3f31d925af9f600dc5cf6ff97b87554247c108053ab94fb6afe476b708c45"},
		{"after", "5a4368adaee1897e18f2c27fdd7861d86f5af46a677e1b0e77b7c67866e226d9", "97f7cd88951612271b4e06e7dd94cf203ecf648de856dd7ae4a6df28d818b0ee"},
	} {
		t.Run(item.name, func(t *testing.T) {
			prefix := "testdata/readspan-last-use-v5/" + item.name
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
			err = runFrontendCommand(t.Context(), []string{"--array-facts-io-v5"}, bytes.NewReader(request), &output)
			t.Logf("TSFG_READSPAN_26A_PUBLIC_REQUEST name=%s bytes=%d sha256=%s base64=%s", item.name, len(request), readSpanFirstPublicHash26(request), base64.StdEncoding.EncodeToString(request))
			t.Logf("TSFG_READSPAN_26A_PUBLIC_ACTUAL name=%s bytes=%d sha256=%s writes=%d base64=%s", item.name, output.Len(), readSpanFirstPublicHash26(output.Bytes()), output.writes, base64.StdEncoding.EncodeToString(output.Bytes()))
			if !bytes.Equal(request, inputBefore) {
				t.Fatal("caller input mutated")
			}
			if err != nil {
				if output.Len() != 0 || output.writes != 0 {
					t.Fatalf("failure published partial facts: writes=%d bytes=%d error=%v", output.writes, output.Len(), err)
				}
				t.Fatalf("official-valid complete source must publish v5 facts (protocol migration, not borrow decision): %v", err)
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
