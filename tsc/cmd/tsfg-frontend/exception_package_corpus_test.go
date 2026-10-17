// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type exceptionCorpusRecordForTest struct {
	ID, Test, Family, Profile string
	Files                     []struct {
		Role, Path, SHA256, SourcePath string
		Bytes                          int
	}
}

// Only old input and independent EXPECTED/ORACLE/source artifacts are fixtures.
// New private packets must be produced by the real command during this test.
func TestExceptionFactsCommandV1ProfilesAndPayloads(t *testing.T) {
	const directory = "testdata/exception-package-corpus-v1/"
	manifestBytes, err := os.ReadFile(directory + "manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct{ Records json.RawMessage }
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	var recordValues any
	var records []exceptionCorpusRecordForTest
	if err := json.Unmarshal(manifest.Records, &recordValues); err != nil {
		t.Fatal(err)
	}
	if exceptionPackageExportsDigestForTest(nullableFactsBoundaryCanonical(t, recordValues)) != "sha256:db7a05d8575154025f75ef4464b5082bb4add44bdd8e736edc28d42d04dd8990" {
		t.Fatal("immutable original corpus record set changed")
	}
	if err := json.Unmarshal(manifest.Records, &records); err != nil || len(records) != 30 {
		t.Fatalf("exact corpus30 required: %v", err)
	}
	for _, record := range records {
		t.Run(record.ID+"_"+record.Test, func(t *testing.T) {
			name := record.ID + "/" + record.Test
			data := map[string][]byte{}
			for _, file := range record.Files {
				value, err := os.ReadFile(directory + file.Path)
				t.Logf("TSFG_EXCEPTION_CORPUS_FIXTURE_V1 case=%s role=%s %s", name, file.Role, base64.StdEncoding.EncodeToString(value))
				if err != nil || len(value) != file.Bytes || exceptionPackageExportsDigestForTest(value) != file.SHA256 {
					t.Fatalf("original %s fixture identity changed: %v", file.Role, err)
				}
				if data[file.Role] != nil {
					t.Fatal("duplicate fixture role")
				}
				data[file.Role] = value
			}
			var r request
			var golden response // Join view only: the full JSON below remains unfiltered.
			var facts, prior any
			if err := json.Unmarshal(data["REQUEST"], &r); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data["EXPECTED"], &golden); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data["EXPECTED"], &facts); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data["OFFICIAL_ORACLE"], &prior); err != nil {
				t.Fatal(err)
			}
			if r.Declarations.Version != record.Profile || golden.Status != "ok" || len(golden.Diagnostics) != 0 {
				t.Fatal("original profile/success expectation changed")
			}
			for _, file := range record.Files {
				if !strings.HasPrefix(file.Role, "SOURCE") {
					continue
				}
				found := false
				for _, input := range r.Files {
					if input.Path == file.SourcePath && bytes.Equal([]byte(input.Content), data[file.Role]) {
						found = true
					}
				}
				if !found {
					t.Fatal("original source artifact differs from request content")
				}
			}
			input := exceptionPackageExportsRequestForTest(t, name, r)
			if !bytes.Equal(input, data["REQUEST"]) {
				t.Fatal("public command must use exact original request bytes")
			}
			var exports []any
			if !t.Run("freshOfficialOracle", func(t *testing.T) { exports = exceptionCorpusOfficialForTest(t, name, r, golden, facts, prior) }) {
				return
			}
			exceptionPackageExportsPacketsForTest(t, name, r, input, facts, exports)
		})
	}
}
