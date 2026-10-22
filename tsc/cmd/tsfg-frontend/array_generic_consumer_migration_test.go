// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestArrayGenericV8ConsumerMigrationOriginalSources25f(t *testing.T) {
	raw, err := os.ReadFile("testdata/array-facts-v8/consumer-migration-inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Name       string                `json:"name"`
			SourceRoot string                `json:"sourceRoot"`
			RootFile   string                `json:"rootFile"`
			Config     arrayFactsConfig      `json:"config"`
			Files      []arrayFactsInputFile `json:"files"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil || len(fixture.Cases) != 10 {
		t.Fatalf("ten complete original migration inputs required: %v", err)
	}
	for _, row := range fixture.Cases {
		t.Run(row.Name, func(t *testing.T) {
			if len(row.Files) != 1 || row.Files[0].Path != row.RootFile || row.Files[0].SHA256 != arrayFactsSHA([]byte(row.Files[0].Content)) {
				t.Fatal("original TypeScript path/content/hash changed")
			}
			source := row.Files[0].Content
			dtsPath := row.SourceRoot + "/minimal-arrays-v8.d.ts"
			request := arrayFactsRequest{Family: "tsfg.array-facts-request-candidate", Version: 8,
				Input: arrayFactsInput{SourceRoot: row.SourceRoot, RootFile: row.RootFile, Config: row.Config,
					Declarations: arrayFactsDeclarations{Version: "minimal-arrays-v8", Paths: []string{dtsPath}},
					Files: append(append([]arrayFactsInputFile{}, row.Files...), arrayFactsInputFile{
						Path: dtsPath, Content: string(arrayGenericDeclarations), SHA256: arrayFactsSHA(arrayGenericDeclarations)})}}
			input, err := arrayFactsCanonical(request)
			if err != nil {
				t.Fatal(err)
			}
			input = append(input, '\n')
			// Retain the exact request/source even when a public projection fails;
			// only the successful existing helper writes an actual package file.
			if root := os.Getenv("TSFG_ARRAY_V8_CAPTURE_ROOT"); root != "" {
				if err := os.MkdirAll(root, 0700); err != nil {
					t.Fatal(err)
				}
				for name, data := range map[string][]byte{"source.ts": []byte(source), "request.json": input} {
					if err := os.WriteFile(filepath.Join(root, row.Files[0].SHA256+"-"+name), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			p := genericArrayPublicRequest25f(t, input, source)
			encoded, err := json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			var payload arrayGenericPayload
			if err := json.Unmarshal(encoded, &payload); err != nil {
				t.Fatal(err)
			}
			if len(payload.Diagnostics) != 0 || payload.SourceRoot != row.SourceRoot || payload.RootFile != row.RootFile || payload.PositionUnit != "utf16" || len(payload.Files) != 2 || payload.Declarations.Version != "minimal-arrays-v8" || len(payload.Declarations.Paths) != 1 || payload.Declarations.Paths[0] != dtsPath {
				t.Fatal("actual v8 must retain the complete original source closure and true v8 declarations")
			}
			sourceUnits := utf16.Encode([]rune(source))
			for _, file := range payload.Files {
				if file.Path == row.RootFile && (file.Content != source || file.SHA256 != row.Files[0].SHA256 || file.ByteLength != len(source) || file.UTF16Length != len(sourceUnits)) {
					t.Fatal("actual source bytes or UTF16 coordinates changed")
				}
			}
			types := map[string]arrayGenericType{}
			for _, typ := range payload.Types {
				types[typ.ID] = typ
			}
			if types["f64"].Kind != "scalar" || types["array<f64>"].ElementType != "f64" || strings.Contains(source, "readSpan") && types["readSpan<f64>"].ElementType != "f64" {
				t.Fatal("original f64 Array/ReadSpan element representation must remain concrete")
			}
			for _, fn := range payload.Functions {
				if fn.SourcePath != row.RootFile || fn.NameRange.Start < 0 || fn.NameRange.End > len(sourceUnits) || string(utf16.Decode(sourceUnits[fn.NameRange.Start:fn.NameRange.End])) != fn.Name {
					t.Fatal("original function identity/name UTF16 anchor changed")
				}
			}
			t.Logf("actual original v8 source=%s sha256=%s bytes=%d functions=%d exports=%d diagnostics=%d", row.RootFile, row.Files[0].SHA256, len(source), len(payload.Functions), len(payload.Exports), len(payload.Diagnostics))
		})
	}
}
