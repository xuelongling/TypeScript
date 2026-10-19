// SPDX-License-Identifier: Apache-2.0
// Historical official-only v4 payloads are unchanged; only the frozen envelope
// and identity domains are migrated to v7 by pure data, without producer output.
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
func TestReadSpan26aV7PublicWholeFrozenPair(t *testing.T) {
	for _, item := range []struct{ name, requestHash, packageHash string }{
		{"before", "056ff43750d102a9608900a2be4a0c7739311ca95bb7754fe2005b1fc92b407f", "f85099631271442ffcf5c5d60a229754874e938cb9dcdec23dac36186bbc29e4"},
		{"after", "1e70bf783699f9e8ca09e9a55da35eec4e6f73f806a8b0f3b82c09b5ce423b43", "cc478742a1c530fa2a561dc69348810455caae33ead096def231459baa296208"},
	} {
		t.Run(item.name, func(t *testing.T) {
			prefix := "testdata/readspan-last-use-v7/" + item.name
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
			err = runFrontendCommand(t.Context(), []string{"--array-facts-io-v7"}, bytes.NewReader(request), &output)
			t.Logf("TSFG_READSPAN_26A_PUBLIC_REQUEST name=%s bytes=%d sha256=%s base64=%s", item.name, len(request), readSpanFirstPublicHash26(request), base64.StdEncoding.EncodeToString(request))
			t.Logf("TSFG_READSPAN_26A_PUBLIC_ACTUAL name=%s bytes=%d sha256=%s writes=%d base64=%s", item.name, output.Len(), readSpanFirstPublicHash26(output.Bytes()), output.writes, base64.StdEncoding.EncodeToString(output.Bytes()))
			if !bytes.Equal(request, inputBefore) {
				t.Fatal("caller input mutated")
			}
			if err != nil {
				if output.Len() != 0 || output.writes != 0 {
					t.Fatalf("failure published partial facts: writes=%d bytes=%d error=%v", output.writes, output.Len(), err)
				}
				t.Fatalf("official-valid complete source must publish v7 facts (protocol migration, not borrow decision): %v", err)
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
