// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/microsoft/typescript-go/internal/json"
)

// The independent fresh official prior must pass before this separate SUT
// Program is created. Only owned bytes escape; original VFS inputs are cleared.
func mappedCallColdBytes14(t *testing.T, f mappedCallOracle14) []byte {
	t.Helper()
	program, environment := genericProgramForTest(f.Source.SourceText)
	if len(environment.Files) != 1 || environment.Files[0].Path != f.Declaration.Path ||
		environment.Files[0].Content != f.Declaration.Content || environment.Files[0].Digest != f.Declaration.SHA256 {
		t.Fatal("SUT declaration input differs from independent official prior")
	}
	data, err := exportGenericPackage(t.Context(), program, program.GetSourceFile(f.Source.LogicalPath),
		"package:mapped-call", environment)
	if err != nil {
		var failure *genericFailure
		if errors.As(err, &failure) && failure.Kind == "unsupported" && failure.Code == 0 &&
			failure.Origin == (genericOrigin{Start: 119, End: 241}) &&
			failure.Detail == "mapped/indexed signature requires one type parameter and one directly returned parameter with the identical indexed type" {
			t.Fatalf("TSFG_GENERIC_MAPPED_CALL_CAPABILITY_RED actual indexed one-hop body rejected at original UTF16 119..241: %v", err)
		}
		t.Fatalf("mapped-call export failed outside the real capability gate: %v", err)
	}
	for _, file := range program.GetSourceFiles() {
		if err := program.Host().FS().WriteFile(file.FileName(), ""); err != nil {
			t.Fatal(err)
		}
		if text, ok := program.Host().FS().ReadFile(file.FileName()); !ok || text != "" {
			t.Fatal("original mapped-call Program VFS input was not cleared")
		}
	}
	return data
}

func TestOwnedGenericMappedCallColdInstances(t *testing.T) {
	f := mappedCallFixture14(t)
	mappedCallOfficialPrior14(t, f)
	data := mappedCallColdBytes14(t, f)
	original := bytes.Clone(data)
	// Header/version checks deliberately follow successful real export.
	var wire genericPackageDTO
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.Version != 7 || wire.Family != "tsfg-owned-generic-package" ||
		wire.SchemaHash != "2e9d1e07016b6f8ce5f48bf8a3508491eaf1ce7e6fd43bf497c133f65c8dff94" ||
		wire.Payload.SemanticProfile != "tsfg-owned-scalar-generics-host-v7" {
		t.Fatal("successful mapped-call export did not use the real private-v6 contract")
	}
	if !bytes.Equal(data, mappedCallJSON14(t, f.Package)) {
		t.Fatalf("complete mapped-call package differs from frozen independent literal: %s", data)
	}
	g, err := decodeGenericPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	again, err := encodeGenericPackage(g)
	if err != nil || !bytes.Equal(again, data) {
		t.Fatalf("cold package roundtrip differs: %v", err)
	}
	var requests []genericPackageRoot
	if err := json.Unmarshal(f.Requests, &requests); err != nil {
		t.Fatal(err)
	}
	requestBefore := mappedCallJSON14(t, requests)
	facts, err := instantiateGenericPackageFacts(t.Context(), data, requests)
	if err != nil {
		t.Fatal(err)
	}
	actual := mappedCallJSON14(t, facts)
	if !bytes.Equal(actual, mappedCallJSON14(t, f.Result)) {
		t.Fatalf("complete actual four-instance result differs from independent literal: %s", actual)
	}
	var persisted genericPackageFacts
	if err := json.Unmarshal(actual, &persisted); err != nil ||
		!bytes.Equal(mappedCallJSON14(t, persisted), actual) {
		t.Fatalf("cold result persistence differs: %v", err)
	}
	if !bytes.Equal(original, data) || !bytes.Equal(requestBefore, mappedCallJSON14(t, requests)) {
		t.Fatal("cold instantiation mutated actual package or original root requests")
	}
	record := mappedCallJSON14(t, map[string]any{"requests": requests, "result": facts})
	t.Logf("TSFG_GENERIC_MAPPED_CALL_PACKAGE_V7 %s", base64.StdEncoding.EncodeToString(data))
	t.Logf("TSFG_GENERIC_MAPPED_CALL_INSTANCE_V7 %s", base64.StdEncoding.EncodeToString(record))
	for _, source := range []map[string]string{
		{"path": f.Source.LogicalPath, "content": f.Source.SourceText, "digest": f.Source.SHA256},
		{"path": f.Declaration.Path, "content": f.Declaration.Content, "digest": f.Declaration.SHA256},
	} {
		t.Logf("TSFG_GENERIC_MAPPED_CALL_ORIGINAL_V7 %s", base64.StdEncoding.EncodeToString(mappedCallJSON14(t, source)))
	}
}
