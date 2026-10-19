// SPDX-License-Identifier: Apache-2.0
// Frozen independent programs are witnessed through fresh official objects first.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

type rs26cFrozenFixtureCase struct {
	Name, Source, SourcePath, SourceSHA256, RequestText, ExpectedText, ExpectedSHA256 string
	SourceBytes, ExpectedBytes                                                        int
}

type rs26cFrozenFixture struct {
	FactsSchemaHash string
	Cases           []rs26cFrozenFixtureCase
}

type rs26cFrozenFixturePin struct {
	name, sha string
	bytes     int
	cases     []string
}

var rs26cFrozenFixturePins = []rs26cFrozenFixturePin{
	{"events100101-expected-facts.json", "50eb7c31a3230ce4aea552e85c51aefeed1ab87d24561f1c1fa384dfabe27c60", 283874, []string{"events100", "events101"}},
	{"family-uses5051-expected-facts.json", "a4ce30e1fdcb212bed7a0b6d6daa76e8ce445f7b8405648342aba2253d9d1616", 266063, []string{"familyUses50", "familyUses51"}},
	{"adjacent-part1-expected-facts.json", "9f81d63cde4ce580c592a3fb4fcb030aa97da9ecc73a71e9c2a7edcecad85603", 176615, []string{"const-alias-return", "tail-forward-return", "bound-forward-return", "discarded-return-call", "mixed-original-slots", "local-fresh-return-escape", "array-parameter-return-capability"}},
	{"adjacent-part2-expected-facts.json", "04d86c1a4754b5b8968114bbd065f9bef860090b4daf8485c03b99c7cb2b6572", 129225, []string{"same-owner-distinct-family-before", "same-owner-distinct-family-after", "initializer-array-density-forgets-old-exact"}},
}

func rs26cReadFrozenFixture(t *testing.T, pin rs26cFrozenFixturePin) rs26cFrozenFixture {
	t.Helper()
	path := "testdata/readspan-returns-v6/" + pin.name
	raw, err := os.ReadFile(path)
	ao23Want(t, err == nil && len(raw) == pin.bytes && ao23Hash(raw) == pin.sha, "complete independently authored frozen fixture "+pin.name)
	before := bytes.Clone(raw)
	t.Cleanup(func() {
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(after, before) || !bytes.Equal(raw, before) {
			t.Errorf("frozen return fixture changed: %v", err)
		}
	})
	var fixture rs26cFrozenFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	ao23Want(t, fixture.FactsSchemaHash == "sha256:fc9b976b96070df07f5b9cdae27c58f327f9b67b74945f25ee874087ce55a279" && len(fixture.Cases) == len(pin.cases), "complete frozen private6 case family")
	for i, c := range fixture.Cases {
		ao23Want(t, c.Name == pin.cases[i] && len(c.Source) == c.SourceBytes && ao23Hash([]byte(c.Source)) == c.SourceSHA256, "exact independent original source order/text")
		ao23Want(t, len(c.RequestText) > 0 && len(c.ExpectedText) == c.ExpectedBytes && ao23Hash([]byte(c.ExpectedText)) == c.ExpectedSHA256, "complete fixed request and literal packet")
	}
	return fixture
}

func rs26cOfficialFixtureWitness(t *testing.T, item rs26cFrozenFixtureCase) ao23Map {
	t.Helper()
	var envelope struct {
		Family  string
		Version int
		Input   struct {
			SourceRoot, RootFile string
			Files                []struct{ Path, Content, SHA256 string }
			Config               struct{ Strict, NoEmit, AllowJS bool }
			Declarations         struct {
				Version string
				Paths   []string
			}
		}
	}
	if err := json.Unmarshal([]byte(item.RequestText), &envelope); err != nil {
		t.Fatal(err)
	}
	in := envelope.Input
	ao23Want(t, envelope.Family == "tsfg.array-facts-request-candidate" && envelope.Version == 6 && in.RootFile == item.SourcePath && len(in.Files) == 2 && in.Config.Strict && in.Config.NoEmit && !in.Config.AllowJS && in.Declarations.Version == "minimal-arrays-v3" && len(in.Declarations.Paths) == 1, "complete original private6 two-file request")
	dtsPath := in.Declarations.Paths[0]
	contents := map[string]string{}
	files, owned := []any{}, []any{}
	for _, f := range in.Files {
		_, duplicate := contents[f.Path]
		ao23Want(t, !duplicate && ao23Hash([]byte(f.Content)) == f.SHA256, "unique original requested file and checksum")
		contents[f.Path] = f.Content
		files = append(files, ao23Map{"path": f.Path, "content": f.Content, "sha256": f.SHA256})
		owned = append(owned, ao23Map{"path": f.Path, "content": f.Content, "sha256": f.SHA256, "byteLength": len(f.Content), "utf16Length": ao23Units(f.Content)})
	}
	ao23Want(t, contents[item.SourcePath] == item.Source && contents[dtsPath] == rs26cOfficialDTSCandidate && len(contents[dtsPath]) == 484 && ao23Hash([]byte(contents[dtsPath])) == "282b335a6e0e0493fa74a3c1dd56f234f8c679079acdaf0600cefb8306c26415", "exact full original source and unchanged pinned DTS3")
	adj := rs26bAdjacentCase{ID: item.Name, Text: item.Source, SourceRoot: in.SourceRoot, SourcePath: item.SourcePath, DeclarationPath: dtsPath}
	base, release := rs26bAdjacentProgram(t, adj, contents[dtsPath])
	defer release()
	s := &rs26cOfficialScope{rs26Scope: base, edges: map[string][]string{}}
	types, arrayLength, viewLength, intrinsic := s.origins()
	s.rs26bAST()
	headers := s.headers26c()
	functions := s.functions26c(headers)
	exports := s.actualExports(headers)
	config := ao23Map{"strict": true, "noEmit": true, "allowJs": false}
	declarations := ao23Map{"version": "minimal-arrays-v3", "paths": []string{dtsPath}}
	request := ao23Map{"family": "tsfg.array-facts-request-candidate", "version": 6, "input": ao23Map{"sourceRoot": in.SourceRoot, "rootFile": item.SourcePath, "files": files, "config": config, "declarations": declarations}}
	requestText := append(ao23JCS(t, request), '\n')
	ao23Want(t, bytes.Equal(requestText, []byte(item.RequestText)), "official-only complete request equals independent literal")
	payload := ao23Map{"sourceRoot": in.SourceRoot, "rootFile": item.SourcePath, "positionUnit": "utf16", "config": config, "declarations": declarations, "files": owned, "types": types, "arrayLength": arrayLength, "readSpanLength": viewLength, "readSpanIntrinsic": intrinsic, "functions": functions, "exports": exports, "diagnostics": []any{}}
	schema := "sha256:fc9b976b96070df07f5b9cdae27c58f327f9b67b74945f25ee874087ce55a279"
	requestID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.request.v6\x00"), requestText...))
	packageID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.package.v6\x00"+schema+"\x00"), ao23JCS(t, payload)...))
	packet := ao23Map{"family": "tsfg.array-facts-package-candidate", "version": 6, "schemaHash": schema, "requestId": requestID, "packageId": packageID, "payload": payload}
	packetText := append(ao23JCS(t, packet), '\n')
	if !bytes.Equal(packetText, []byte(item.ExpectedText)) {
		t.Fatalf("official-only whole packet differs from independent literal for %s\nactual=%s\nexpected=%s", item.Name, packetText, item.ExpectedText)
	}
	s.proof["counters"] = s.counts
	return ao23Map{"name": item.Name, "sourcePath": item.SourcePath, "source": item.Source, "sourceBytes": len(item.Source), "sourceSHA256": ao23Hash([]byte(item.Source)), "official": s.proof, "officialPacketText": string(packetText), "SourceProducerCalled": false, "ProductModuleCalled": false, "officialFactsPacketConstructed": true}
}

func TestReadSpan26cOfficialFreshWholeAdjacentAndBudgets(t *testing.T) {
	for _, pin := range rs26cFrozenFixturePins {
		t.Run(pin.name, func(t *testing.T) {
			fixture := rs26cReadFrozenFixture(t, pin)
			for _, item := range fixture.Cases {
				t.Run(item.Name, func(t *testing.T) {
					witness := ao23JCS(t, rs26cOfficialFixtureWitness(t, item))
					t.Logf("TSFG_READSPAN_26C_OFFICIAL_WHOLE_ADJACENT_BUDGET name=%s bytes=%d sha256=%s base64=%s", item.Name, len(witness), ao23Hash(witness), base64.StdEncoding.EncodeToString(witness))
				})
			}
		})
	}
}
