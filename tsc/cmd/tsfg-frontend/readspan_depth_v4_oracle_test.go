// SPDX-License-Identifier: Apache-2.0
// Recheck the complete frozen forward DAG against fresh official objects and new declarations.
package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

func TestReadSpan26aIndependentDepth16Official(t *testing.T) {
	raw, err := os.ReadFile("testdata/array-facts-v4/depth16-original-input.json")
	if err != nil {
		t.Fatal(err)
	}
	ao23Want(t, len(raw) == 15462 && ao23Hash(raw) == "f90c857d7c7dbf695104d2b16c9eb8d33c603d3c62d56d5e3b56198afa410a2c", "whole unchanged depth16 original controls")
	var item rsOld26Case
	if err = json.Unmarshal(raw, &item); err != nil {
		t.Fatal(err)
	}
	ao23Want(t, item.Name == "forward-chain16" && item.Group == "depth16", "exact independent forward DAG control")
	dts, err := os.ReadFile("testdata/array-facts-v4/minimal-arrays-v3.d.ts")
	if err != nil {
		t.Fatal(err)
	}
	ao23Want(t, len(dts) == 484 && ao23Hash(dts) == "282b335a6e0e0493fa74a3c1dd56f234f8c679079acdaf0600cefb8306c26415", "pinned new complete declarations")
	bundle := rsOld26Packet(t, item, string(dts))
	var value struct{ PackageText string }
	if err = json.Unmarshal(bundle, &value); err != nil {
		t.Fatal(err)
	}
	var packet ao23Map
	if err = json.Unmarshal([]byte(value.PackageText), &packet); err != nil {
		t.Fatal(err)
	}
	functions := packet["payload"].(map[string]any)["functions"].([]any)
	ao23Want(t, len(functions) == 16, "all16 functions retained")
	t.Logf("TSFG_READSPAN_26A_DEPTH16_INDEPENDENT_PACKET bytes=%d sha256=%s base64=%s", len(bundle), ao23Hash(bundle), base64.StdEncoding.EncodeToString(bundle))
}
