// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"testing"
)

func TestSystemQueryFactsPrototype38c(t *testing.T) {
	r := systemRequest38c(t)
	r.Target = &integerTarget{Target: "windows-x86_64-msvc", Triple: "x86_64-pc-windows-msvc", LLVMDataLayout: "e-m:w-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128", PointerBits: "64"}
	payload, e := projectSystemQueries(t.Context(), r)
	if e != nil {
		t.Fatal(e)
	}
	if len(payload.Systems) != 1 || len(payload.Systems[0].Parameters) != 2 || len(payload.Systems[0].Parameters[0].Query.Items) != 2 {
		t.Fatal("complete architecture access inventory differs")
	}
	raw, e := arrayFactsCanonical(payload)
	if e != nil {
		t.Fatal(e)
	}
	if path := os.Getenv("TSFG_QUERY38_PROTOTYPE"); path != "" {
		if e = os.WriteFile(path, append(raw, '\n'), 0600); e != nil {
			t.Fatal(e)
		}
	}
	t.Logf("actual original Program query payload bytes=%d types=%d nodes=%d", len(raw)+1, len(payload.Types), len(payload.Nodes))
}
