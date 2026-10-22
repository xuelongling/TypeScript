// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	tsjson "github.com/microsoft/typescript-go/internal/json"
	"io"
	"strings"
	"testing"
)

func TestArrayGenericV8ScalarCatalogAndWhilePublic25f(t *testing.T) {
	for _, typ := range []string{"i8", "i16", "i32", "i64", "isize", "u8", "u16", "u32", "u64", "usize", "f32", "f64", "bool", "string", "undefined", "null"} {
		p := genericArrayPublic25f(t, "export function read(a:"+typ+"[],i:u32):"+typ+" { return a[i]; }")
		v := genericReturn25f(t, p, "read")
		if v["type"] != typ {
			t.Fatalf("concrete %s changed to %v", typ, v)
		}
	}
	p := genericArrayPublic25f(t, "export function fill(a:i32[],n:u32):i32 { let i:u32=0; while(i<n){ a[i]=2; i++; } return a[0]; }")
	loop := genericFindFunction25f(t, p, "fill")["body"].([]any)[1].(map[string]any)
	if loop["kind"] != "while" || loop["body"].([]any)[1].(map[string]any)["value"].(map[string]any)["kind"] != "bindingUpdate" {
		t.Fatalf("scalar loop facts lost: %v", loop)
	}
}
func TestArrayGenericV8StructuralReferenceWitnessPublic25f(t *testing.T) {
	p := genericArrayPublic25f(t, "class Base { n:i32=1; } class Other { n:i32=2; extra:i32=3; } function pass(b:Base):Base { return b; } export function read(o:Other,i:u32):Base { const a:Base[]=[o]; a[i]=o; return pass(a[i]); }")
	body := genericFindFunction25f(t, p, "read")["body"].([]any)
	init := body[0].(map[string]any)["initializer"].(map[string]any)["elements"].([]any)[0].(map[string]any)
	witness := genericRootProof25f(t, p, init)
	if witness["assignable"] != true || witness["sourceType"] == witness["targetType"] || len(witness["members"].([]any)) != 1 || witness["origin"] == nil {
		t.Fatalf("actual M12 structural witness missing: %v", witness)
	}
	store := body[1].(map[string]any)["value"].(map[string]any)["value"].(map[string]any)
	if store["compatibilityRoots"] == nil {
		t.Fatal("element store lost the target contextual reference view")
	}
}
func TestArrayGenericV8StoreResultAndInferredLocalPublic25f(t *testing.T) {
	p := genericArrayPublic25f(t, "export function store(a:(i32|undefined)[],i:u32):undefined { let x=(a[i]=undefined); return x; }")
	body := genericFindFunction25f(t, p, "store")["body"].([]any)
	wrapper := body[0].(map[string]any)["initializer"].(map[string]any)
	store := wrapper["expression"].(map[string]any)
	if store["type"] != "union<i32,undefined>" || store["flowType"] != "undefined" || genericReturn25f(t, p, "store")["flowType"] != "undefined" {
		t.Fatalf("store narrowed result lost: %v", store)
	}
	genericArrayPublic25f(t, "export function read(i:u32):string { const a=[\"a\",\"b\"]; return a[i]; }")
	for _, source := range []string{"export function read(i:u32):i32 { const a=[1,2]; return a[i]; }", "class Box { constructor(public n:i32){} } export function read(a:Box[],i:u32):Box { return a[i]; }"} {
		p = genericArrayPublic25f(t, source)
		for _, raw := range p["types"].([]any) {
			typ := raw.(map[string]any)
			if typ["name"] == "Box" {
				field := typ["fields"].([]any)[0].(map[string]any)
				param := typ["constructor"].(map[string]any)["parameters"].([]any)[0].(map[string]any)
				if field["parameterBinding"] != param["id"] || field["id"] == param["id"] {
					t.Fatal("parameter-property dual original identities conflated")
				}
			}
		}
	}
}
func TestArrayGenericV8ExactIndexAndInheritancePublic25f(t *testing.T) {
	for _, token := range []string{"1.0", "1e0", "0x1", "(1.0)"} {
		genericArrayPublic25f(t, "export function read():i32 { const a:i32[]=[1,2]; a.length=2.0; return a["+token+"]; }")
	}
	p := genericArrayPublic25f(t, "class Base { n:i32=1; } class Derived extends Base { extra:i32=2; } export function read(v:Derived,i:u32):Base { const a:Base[]=[v]; return a[i]; }")
	init := genericFindFunction25f(t, p, "read")["body"].([]any)[0].(map[string]any)["initializer"].(map[string]any)["elements"].([]any)[0].(map[string]any)
	if init["compatibilityRoots"] == nil {
		t.Fatal("inheritance erased original reference view witness")
	}
}
func TestArrayGenericV8MultiSourceClosurePublic25f(t *testing.T) {
	source := "import { Box } from './box'; import { index } from './index'; export function read(a:Box[],i:u32):Box { return a[index(i)]; }"
	var request arrayFactsRequest
	if err := json.Unmarshal(genericArrayRequest25f(t, source), &request); err != nil {
		t.Fatal(err)
	}
	box := "export class Box { n:i32=1; }"
	index := "export function index(i:u32):u32 { return i; }"
	request.Input.Files = append([]arrayFactsInputFile{{Path: "/src/box.ts", Content: box, SHA256: arrayFactsSHA([]byte(box))}, {Path: "/src/index.ts", Content: index, SHA256: arrayFactsSHA([]byte(index))}}, request.Input.Files...)
	b, _ := json.Marshal(request)
	raw := tsjson.Value(b)
	if err := raw.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	p := genericArrayPublicRequest25f(t, append(raw, '\n'), source)
	if len(p["files"].([]any)) != 4 {
		t.Fatal("original source closure truncated")
	}
	value := genericReturn25f(t, p, "read")
	if value["index"].(map[string]any)["signature"].(map[string]any)["sourcePath"] != "/src/index.ts" {
		t.Fatal("cross-source original signature join lost")
	}
}

type genericArrayWriter25f struct {
	n   int
	err error
}

func (w *genericArrayWriter25f) Write(p []byte) (int, error) {
	w.n++
	if w.err != nil {
		return 0, w.err
	}
	return len(p) - 1, nil
}
func TestArrayGenericV8IOAndDiagnosticBoundariesPublic25f(t *testing.T) {
	input := genericArrayRequest25f(t, "export function read(a:i32[],i:u32):i32 { return a[i]; }")
	t.Run("canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		var out bytes.Buffer
		err := runFrontendCommand(ctx, []string{"--array-facts-io-v8"}, bytes.NewReader(input), &out)
		if !errors.Is(err, context.Canceled) || out.Len() != 0 {
			t.Fatalf("cancel lost: %v", err)
		}
	})
	t.Run("short write", func(t *testing.T) {
		w := &genericArrayWriter25f{}
		err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(input), w)
		if !errors.Is(err, io.ErrShortWrite) || w.n != 1 {
			t.Fatalf("one-shot short write: %v", err)
		}
	})
	t.Run("oversize", func(t *testing.T) {
		var out bytes.Buffer
		if err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, strings.NewReader(strings.Repeat(" ", arrayFactsMaxBytes+1)), &out); err == nil || out.Len() != 0 {
			t.Fatal("oversize accepted")
		}
	})
	t.Run("official diagnostics", func(t *testing.T) {
		var out bytes.Buffer
		err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(genericArrayRequest25f(t, "export function bad(a:string[],i:u32):i32 { return a[i]; }")), &out)
		var failure *arrayFactsFailure
		if !errors.As(err, &failure) || len(failure.Diagnostics) == 0 || failure.Diagnostics[0].UpstreamCode == 0 || out.Len() != 0 {
			t.Fatalf("original official diagnostic swallowed: %v", err)
		}
	})
	t.Run("unconsumed", func(t *testing.T) {
		var request arrayFactsRequest
		json.Unmarshal(input, &request)
		text := "export function extra():i32{return 0;}"
		request.Input.Files = append([]arrayFactsInputFile{{Path: "/src/extra.ts", Content: text, SHA256: arrayFactsSHA([]byte(text))}}, request.Input.Files...)
		b, _ := json.Marshal(request)
		raw := tsjson.Value(b)
		raw.Canonicalize()
		var out bytes.Buffer
		if err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(append(raw, '\n')), &out); err == nil || out.Len() != 0 {
			t.Fatal("unconsumed closure member accepted")
		}
	})
}
