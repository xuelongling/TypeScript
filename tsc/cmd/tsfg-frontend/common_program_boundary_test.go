// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	tsjson "github.com/microsoft/typescript-go/internal/json"
)

func TestCommonProgramInputBoundary37n(t *testing.T) {
	cases := []struct {
		name string
		edit func(map[string]any)
	}{
		{"wrong-schema", func(r map[string]any) { r["schemaHash"] = "sha256:" + strings.Repeat("0", 64) }},
		{"wrong-version", func(r map[string]any) { r["version"] = 2 }},
		{"unknown-field", func(r map[string]any) { r["permission"] = true }},
		{"wrong-target", func(r map[string]any) { r["input"].(map[string]any)["target"].(map[string]any)["pointerBits"] = "32" }},
		{"unresolved-root", func(r map[string]any) {
			c := r["input"].(map[string]any)["compilationClosure"].(map[string]any)
			c["roots"].([]any)[0].(map[string]any)["name"] = "notDeclared"
		}},
		{"unsorted-roots", func(r map[string]any) {
			c := r["input"].(map[string]any)["compilationClosure"].(map[string]any)
			roots := c["roots"].([]any)
			roots[0], roots[1] = roots[1], roots[0]
		}},
		{"missing-required-root", func(r map[string]any) {
			c := r["input"].(map[string]any)["compilationClosure"].(map[string]any)
			c["requiredRoots"].([]any)[0].(map[string]any)["name"] = "missing"
		}},
		{"stale-source-hash", func(r map[string]any) {
			files := r["input"].(map[string]any)["files"].([]any)
			files[0].(map[string]any)["content"] = files[0].(map[string]any)["content"].(string) + "\n"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var input map[string]any
			if err := json.Unmarshal(commonProgramRequest37n(t, "positive"), &input); err != nil {
				t.Fatal(err)
			}
			tc.edit(input)
			data, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			canonical := tsjson.Value(data)
			if err := canonical.Canonicalize(); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v1"}, bytes.NewReader(append(canonical, '\n')), &output); err == nil || output.Len() != 0 {
				t.Fatal("invalid original input must not publish any partial Program")
			}
		})
	}
}

func TestCommonProgramCanceled37n(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var output bytes.Buffer
	if err := runFrontendCommand(ctx, []string{"--program-facts-io-v1"}, bytes.NewReader(commonProgramRequest37n(t, "positive")), &output); err == nil || output.Len() != 0 {
		t.Fatal("canceled original projection must not publish a partial Program")
	}
}

func TestCommonProgramAccess37n(t *testing.T) {
	var request map[string]any
	if err := json.Unmarshal(commonProgramRequest37n(t, "positive"), &request); err != nil {
		t.Fatal(err)
	}
	file := request["input"].(map[string]any)["files"].([]any)[0].(map[string]any)
	source := strings.Replace(file["content"].(string), "class Box { n: i32 = 0; }", "class Box { private secret: i32 = 1; protected inherited: i32 = 2; n: i32 = 0; }", 1)
	file["content"], file["sha256"] = source, arrayFactsSHA([]byte(source))
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	canonical := tsjson.Value(encoded)
	if err := canonical.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v1"}, bytes.NewReader(append(canonical, '\n')), &output); err != nil {
		t.Fatal(err)
	}
	var packet commonProgramPackage
	if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	for _, typ := range packet.Payload.Types {
		if typ.Name != "Box" {
			continue
		}
		if typ.Fields == nil || len(*typ.Fields) != 3 {
			t.Fatal("complete original member inventory")
		}
		for i, access := range []string{"private", "protected", "public"} {
			if (*typ.Fields)[i].Access != access {
				t.Fatal("original permissions were replaced by implicit public access")
			}
		}
		return
	}
	t.Fatal("original Box type absent")
}
