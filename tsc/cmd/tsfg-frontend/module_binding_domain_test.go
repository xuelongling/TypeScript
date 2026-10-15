// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"github.com/microsoft/typescript-go/internal/json"
	"testing"
)

func TestModuleDefaultBindingOwnedOrigin(t *testing.T) {
	for _, spelling := range []string{"$default", `\u0024default`} {
		t.Run(spelling, func(t *testing.T) {
			r := moduleRequest([]inputFile{
				{Path: "/src/state.ts", Content: "export let " + spelling + ": i32 = 7; function make(): i32 { return 35; } export default make();"},
				{Path: "/src/app.ts", Content: "import value, {$default} from './state'; export function entry(): i32 { return value + $default; }"},
			}, "/src/app.ts")
			result := analyze(r)
			if result.Status != "ok" {
				t.Fatalf("normal checked default binding source rejected: %+v", result.Diagnostics)
			}
			var data bytes.Buffer
			if err := json.NewLegacyEncoder(&data).Encode(result); err != nil {
				t.Fatal(err)
			}
			var owned struct {
				Symbols []struct {
					ID, Name, Kind, SourcePath, BindingKind string
					Writable                                bool
				}
			}
			if err := json.NewLegacyDecoder(bytes.NewReader(data.Bytes())).Decode(&owned); err != nil {
				t.Fatal(err)
			}
			origins := map[string]string{}
			for _, symbol := range owned.Symbols {
				if symbol.SourcePath == "/src/state.ts" && symbol.Kind == "global" {
					if symbol.Name != "$default" {
						t.Fatalf("cooked official identifier changed: %+v", symbol)
					}
					if _, exists := origins[symbol.BindingKind]; exists {
						t.Fatalf("duplicate owned origin: %+v", owned.Symbols)
					}
					origins[symbol.BindingKind] = symbol.ID
					if symbol.Writable != (symbol.BindingKind == "source") {
						t.Fatalf("binding writability changed: %+v", symbol)
					}
				}
			}
			if len(origins) != 2 || origins["source"] == "" || origins["default-export-expression"] == "" || origins["source"] == origins["default-export-expression"] {
				t.Fatalf("official symbols must retain two independent binding origins: %+v", origins)
			}
			for _, module := range result.Modules {
				if module.SourcePath != "/src/state.ts" {
					continue
				}
				if len(module.Globals) != 2 || len(module.Initialization) != 2 {
					t.Fatal("both bindings must initialize once")
				}
				for _, export := range module.Exports {
					want := origins["source"]
					if export.Name == "default" {
						want = origins["default-export-expression"]
					}
					if export.Target != want {
						t.Fatalf("official alias target changed: %+v", export)
					}
				}
			}
		})
	}
}

func TestModuleOrdinaryDefaultExportAliasKeepsSourceOrigin(t *testing.T) {
	r := moduleRequest([]inputFile{
		{Path: "/src/state.ts", Content: "export const $default: i32 = 42; export {$default as default};"},
		{Path: "/src/app.ts", Content: "import value from './state'; export function entry(): i32 { return value; }"},
	}, "/src/app.ts")
	result := analyze(r)
	if result.Status != "ok" {
		t.Fatalf("ordinary default alias rejected: %+v", result.Diagnostics)
	}
	var data bytes.Buffer
	if err := json.NewLegacyEncoder(&data).Encode(result); err != nil {
		t.Fatal(err)
	}
	var owned struct {
		Symbols []struct{ Kind, BindingKind string }
	}
	if err := json.NewLegacyDecoder(bytes.NewReader(data.Bytes())).Decode(&owned); err != nil {
		t.Fatal(err)
	}
	for _, symbol := range owned.Symbols {
		if symbol.Kind == "global" && symbol.BindingKind != "source" {
			t.Fatalf("default alias forged a generated binding: %+v", symbol)
		}
	}
}
