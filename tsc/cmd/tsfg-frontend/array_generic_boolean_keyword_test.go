// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestArrayGenericV8BooleanKeywordIdentity25f(t *testing.T) {
	for _, source := range []string{
		"export function echo(flag:boolean):boolean{return flag;}",
		"export function echo(flag:bool):bool{return flag;}",
		"type Truth=boolean; export function echo(flag:Truth):Truth{return flag;}",
		"type bool=number; export function echo(flag:boolean):boolean{return flag;}",
	} {
		p := genericArrayPublic25f(t, source)
		fn := genericFindFunction25f(t, p, "echo")
		parameter := fn["parameters"].([]any)[0].(map[string]any)
		result := genericReturn25f(t, p, "echo")
		if fn["returnType"] != "bool" || parameter["declaredType"] != "bool" || result["type"] != "bool" || result["flowType"] != "bool" || result["binding"] != parameter["id"] {
			t.Fatal("boolean keyword and valid aliases must share the canonical bool type and original binding")
		}
		found := false
		for _, raw := range p["types"].([]any) {
			typ := raw.(map[string]any)
			if typ["id"] != "bool" {
				continue
			}
			found = true
			name := typ["nameRange"].(map[string]any)
			declaration := typ["declarationRange"].(map[string]any)
			dts := string(arrayGenericDeclarations)
			if typ["kind"] != "scalar" || typ["name"] != "bool" || typ["sourcePath"] != "/src/minimal-arrays-v8.d.ts" || dts[int(name["start"].(float64)):int(name["end"].(float64))] != "bool" || dts[int(declaration["start"].(float64)):int(declaration["end"].(float64))] != "type bool = boolean;" {
				t.Fatal("keyword identity must retain the actual pinned bool declaration, even with a shadowing local alias")
			}
		}
		if !found {
			t.Fatal("missing original pinned bool catalog entry")
		}
	}
	for _, source := range []string{
		"type bool=boolean; export function echo(flag:bool):boolean{return flag;}",
		"type bool=number; export function echo(flag:bool):boolean{return true;}",
	} {
		var output bytes.Buffer
		err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(genericArrayRequest25f(t, source)), &output)
		var failure *arrayFactsFailure
		if !errors.As(err, &failure) || failure.Kind != "capability" || failure.Code != "r03-array-facts-capability" || output.Len() != 0 {
			t.Fatalf("unowned reserved bool aliases must remain rejected: %v", err)
		}
	}
	t.Log("four canonical keyword/pinned/alias provenance positives and two unowned scalar negatives passed")
}

func TestArrayGenericV8NumberKeywordIdentity25f(t *testing.T) {
	for _, row := range []struct{ name, source string }{
		{"foreign-number-alias", "type Foreign=number; export function echo(flag:Foreign):Foreign{return flag;}"},
		{"direct-number", "export function echo(flag:number):number{return flag;}"},
		{"pinned-f64", "export function echo(flag:f64):f64{return flag;}"},
		{"keyword-with-shadow", "type f64=i32; export function echo(flag:number):number{return flag;}"},
	} {
		t.Run(row.name, func(t *testing.T) {
			input := genericArrayRequest25f(t, row.source)
			// Keep the exact newly contract-corrected number RED/GREEN separate
			// from the historical20760 mistaken Foreign rejection expectation.
			capture := func(name string, data []byte) {
				t.Helper()
				if root := os.Getenv("TSFG_ARRAY_V8_CAPTURE_ROOT"); root != "" {
					if err := os.MkdirAll(root, 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(root, arrayFactsSHA([]byte(row.source))+"-"+name), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			capture("source.ts", []byte(row.source))
			capture("request.json", input)
			var output bytes.Buffer
			if err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(input), &output); err != nil {
				var failure *arrayFactsFailure
				errors.As(err, &failure)
				record, encodeErr := json.MarshalIndent(struct {
					Error   string             `json:"error"`
					Failure *arrayFactsFailure `json:"failure"`
				}{err.Error(), failure}, "", "  ")
				if encodeErr != nil {
					t.Fatal(encodeErr)
				}
				capture("error.json", record)
				capture("failed-output.bin", output.Bytes())
				t.Fatalf("R01 canonical f64 source must be accepted: %s", record)
			}
			genericArrayCapture25f(t, input, row.source, output.Bytes())
			var packet arrayGenericPackage
			if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}
			fn := packet.Payload.Functions[0]
			if len(packet.Payload.Diagnostics) != 0 || fn.ReturnType != "f64" || fn.Parameters[0].DeclaredType != "f64" || fn.Body[0].Value.Type != "f64" || fn.Body[0].Value.FlowType != "f64" || fn.Body[0].Value.Binding != fn.Parameters[0].ID {
				t.Fatal("number keyword and legal aliases must share canonical f64 and original binding")
			}
			found := false
			for _, typ := range packet.Payload.Types {
				if typ.ID != "f64" {
					continue
				}
				found = true
				dts := string(arrayGenericDeclarations)
				if typ.Kind != "scalar" || typ.Name != "f64" || typ.SourcePath != "/src/minimal-arrays-v8.d.ts" || dts[typ.NameRange.Start:typ.NameRange.End] != "f64" || dts[typ.DeclarationRange.Start:typ.DeclarationRange.End] != "type f64 = number;" {
					t.Fatal("canonical f64 must retain the original pinned declaration despite local shadowing")
				}
			}
			if !found {
				t.Fatal("missing original pinned f64 catalog entry")
			}
		})
	}
}

func TestArrayGenericV8NumberKeywordDoesNotRetag25f(t *testing.T) {
	for _, source := range []string{
		"export function invalid(a:i32):number{return a;}",
		"export function invalid(a:number):i32{return a;}",
	} {
		var output bytes.Buffer
		err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(genericArrayRequest25f(t, source)), &output)
		var failure *arrayFactsFailure
		if !errors.As(err, &failure) || output.Len() != 0 {
			t.Fatalf("canonical number=f64 must not erase a typed numeric representation boundary: %v", err)
		}
	}
}
