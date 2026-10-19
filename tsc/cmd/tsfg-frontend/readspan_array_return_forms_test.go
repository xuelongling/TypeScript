// SPDX-License-Identifier: Apache-2.0
// Independent whole facts witness existing Source forms; Product owns SCC refusal.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
)

func m26dReadForms(t *testing.T) []readSpan26dPriorCase {
	t.Helper()
	const file = "testdata/readspan-array-returns-v7/representative-expected-facts.json"
	raw, err := os.ReadFile(file)
	ao23Want(t, err == nil && len(raw) == 163373 && ao23Hash(raw) == "17948c29954bad8cb18e7d755bb2157390c1fd2c9778e4c14e317c10c5776610", "complete frozen independent five-form fixture")
	before := bytes.Clone(raw)
	t.Cleanup(func() {
		after, err := os.ReadFile(file)
		if err != nil || !bytes.Equal(after, before) || !bytes.Equal(raw, before) {
			t.Errorf("independent five-form fixture changed: %v", err)
		}
	})
	var fixture struct {
		Family, SourceRoot, RootFile, SchemaHash string
		Version                                  int
		Cases                                    []readSpan26dPriorCase
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	ao23Want(t, fixture.Family == "tsfg.m26d-independent-representative-source-whole" && fixture.Version == 1 && fixture.SourceRoot == "/readspan26d-forms" && fixture.RootFile == "/readspan26d-forms/app.ts" && fixture.SchemaHash == "sha256:d2ee930b76b8fd5c2270ad69579c9c61377831a97034d3a8890d8aa770ebabc2" && len(fixture.Cases) == 5, "exact current facts7 source-form inventory")
	names := []string{"bound-const-alias", "tail-discard-and-unused-bound", "view-parameter-same-family", "scc-outgoing-array-origin-view-rejected", "scc-fresh-owner-rejected"}
	for i, item := range fixture.Cases {
		ao23Want(t, item.Name == names[i] && item.SourceBytes == []int{248, 291, 260, 212, 125}[i] && len(item.Source) == item.SourceBytes && ao23Units(item.Source) == item.SourceBytes && ao23Hash([]byte(item.Source)) == item.SourceSHA256, "complete original ASCII source literal and UTF16 extent")
		ao23Want(t, item.PackageBytes == []int{9280, 9841, 9315, 8081, 6504}[i] && len(item.PackageText) == item.PackageBytes && ao23Hash([]byte(item.PackageText)) == item.PackageSHA256, "entire independent expected packet")
		for _, frame := range []struct {
			data json.RawMessage
			text string
		}{{item.Request, item.RequestText}, {item.Package, item.PackageText}} {
			var value any
			if err := json.Unmarshal(frame.data, &value); err != nil {
				t.Fatal(err)
			}
			ao23Want(t, bytes.Equal(append(ao23JCS(t, value), '\n'), []byte(frame.text)), "complete literal object equals canonical wire frame")
		}
	}
	return fixture.Cases
}

// Only this original program combines a view-return header with an actual cycle.
// Reuse the two-statement view traversal and the existing scalar SCC traversal;
// no DAG assertion is applied to a partial or cyclic function inventory.
func m26dCyclicViewWitness(t *testing.T, item readSpan26dPriorCase) ao23Map {
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
	ao23Want(t, envelope.Family == "tsfg.array-facts-request-candidate" && envelope.Version == 7 && in.SourceRoot == "/readspan26d-forms" && in.RootFile == "/readspan26d-forms/app.ts" && len(in.Files) == 2 && in.Config.Strict && in.Config.NoEmit && !in.Config.AllowJS && in.Declarations.Version == "minimal-arrays-v3" && len(in.Declarations.Paths) == 1, "complete exact source request")
	dtsPath := in.Declarations.Paths[0]
	contents, files, owned := map[string]string{}, []any{}, []any{}
	for _, f := range in.Files {
		_, duplicate := contents[f.Path]
		ao23Want(t, !duplicate && ao23Hash([]byte(f.Content)) == f.SHA256, "unique complete input and checksum")
		contents[f.Path] = f.Content
		files = append(files, ao23Map{"path": f.Path, "content": f.Content, "sha256": f.SHA256})
		owned = append(owned, ao23Map{"path": f.Path, "content": f.Content, "sha256": f.SHA256, "byteLength": len(f.Content), "utf16Length": ao23Units(f.Content)})
	}
	ao23Want(t, contents[in.RootFile] == item.Source && contents[dtsPath] == rs26cOfficialDTSCandidate && len(contents[dtsPath]) == 484 && ao23Hash([]byte(contents[dtsPath])) == "282b335a6e0e0493fa74a3c1dd56f234f8c679079acdaf0600cefb8306c26415", "exact original source and pinned declarations")
	base, release := rs26bAdjacentProgram(t, rs26bAdjacentCase{ID: item.Name, Text: item.Source, SourceRoot: in.SourceRoot, SourcePath: in.RootFile, DeclarationPath: dtsPath}, contents[dtsPath])
	defer release()
	s := &rs26cOfficialScope{rs26Scope: base, edges: map[string][]string{}}
	types, arrayLength, viewLength, intrinsic := s.origins()
	s.rs26bAST()
	headers := s.headers26c()
	ao23Want(t, len(headers) == 3 && headers[0].node.Name().Text() == "borrow" && headers[1].node.Name().Text() == "spin" && headers[2].node.Name().Text() == "entry", "complete ordered original function declarations")
	h := headers[0]
	ao23Want(t, h.row["returnType"] == "readspan.f64" && len(h.parameters) == 1 && h.kinds[0] == "array.f64", "view-return helper with complete original Array slot")
	s.current = h.row["id"].(string)
	s.bindings, s.views = map[*ast.Symbol]ao23Binding{}, map[*ast.Symbol]string{}
	s.bindings[h.signature.Parameters()[0]] = ao23Binding{h.parameters[0]["id"].(string), "array.f64"}
	nodes := h.node.AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
	ao23Want(t, len(nodes) == 2 && nodes[0].Kind == ast.KindVariableStatement && nodes[1].Kind == ast.KindReturnStatement, "entire two-statement original view helper body")
	binding, local := s.viewLocal26c(nodes[0])
	returned := nodes[1].AsReturnStatement().Expression
	ao23Want(t, returned != nil && returned.Kind == ast.KindIdentifier && s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(returned), s.check.GetReturnTypeOfSignature(h.signature)), "real original view return symbol/type/assignability")
	returnValue := s.value26c(returned, "readspan.f64", "return")
	ao23Want(t, returnValue["binding"] == binding["id"], "returned original view Symbol joins its actual preceding local")
	h.row["bindings"] = []any{binding}
	h.row["body"] = []any{local, ao23Map{"kind": "return", "range": s.span(s.file, nodes[1]), "value": returnValue}}
	s.counts["statements"] += 2
	functions := append([]any{h.row}, scc23fAdjacentFunctions(s, headers[1:])...)
	borrowID, spinID, entryID := h.row["id"].(string), headers[1].row["id"].(string), headers[2].row["id"].(string)
	wantGraph := map[string][]string{borrowID: {}, spinID: {borrowID, spinID}, entryID: {}}
	ao23Want(t, bytes.Equal(ao23JCS(t, s.edges), ao23JCS(t, wantGraph)), "complete ordered actual outgoing call graph including original self edge")
	inventory, ok := s.proof["functionInventory"].([]any)
	ao23Want(t, ok, "complete scalar original-object inventory")
	s.proof["functionInventory"] = append([]any{ao23Map{"name": "borrow", "parameters": 1, "statements": 2, "locals": 1, "ordinaryCalls": 0, "intrinsicCalls": 1}}, inventory...)
	exports := s.actualExports(headers)
	config := ao23Map{"strict": true, "noEmit": true, "allowJs": false}
	declarations := ao23Map{"version": "minimal-arrays-v3", "paths": []string{dtsPath}}
	request := ao23Map{"family": "tsfg.array-facts-request-candidate", "version": 7, "input": ao23Map{"sourceRoot": in.SourceRoot, "rootFile": in.RootFile, "files": files, "config": config, "declarations": declarations}}
	requestText := append(ao23JCS(t, request), '\n')
	ao23Want(t, bytes.Equal(requestText, []byte(item.RequestText)), "whole official-only request equals independent literal")
	payload := ao23Map{"sourceRoot": in.SourceRoot, "rootFile": in.RootFile, "positionUnit": "utf16", "config": config, "declarations": declarations, "files": owned, "types": types, "arrayLength": arrayLength, "readSpanLength": viewLength, "readSpanIntrinsic": intrinsic, "functions": functions, "exports": exports, "diagnostics": []any{}}
	schema := "sha256:d2ee930b76b8fd5c2270ad69579c9c61377831a97034d3a8890d8aa770ebabc2"
	requestID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.request.v7\x00"), requestText...))
	packageID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.package.v7\x00"+schema+"\x00"), ao23JCS(t, payload)...))
	packet := ao23Map{"family": "tsfg.array-facts-package-candidate", "version": 7, "schemaHash": schema, "requestId": requestID, "packageId": packageID, "payload": payload}
	packetText := append(ao23JCS(t, packet), '\n')
	ao23Want(t, bytes.Equal(packetText, []byte(item.PackageText)), "entire actual official-object packet equals independent expected packet")
	s.proof["counters"] = s.counts
	return ao23Map{"name": item.Name, "sourcePath": in.RootFile, "source": item.Source, "sourceBytes": len(item.Source), "sourceSHA256": item.SourceSHA256, "official": s.proof, "officialPacketText": string(packetText), "SourceProducerCalled": false, "ProductModuleCalled": false, "officialFactsPacketConstructed": true}
}

func TestM26dArrayReturnFormsOfficialProgram(t *testing.T) {
	for i, item := range m26dReadForms(t) {
		t.Run(item.Name, func(t *testing.T) {
			var witness ao23Map
			switch i {
			case 3:
				witness = m26dCyclicViewWitness(t, item)
			case 4:
				witness = scc23fAdjacentWholeWitness(t, scc23fWholeCase{Name: item.Name, Source: item.Source, SourcePath: "/readspan26d-forms/app.ts", SourceSHA256: item.SourceSHA256, SourceBytes: item.SourceBytes, RequestText: item.RequestText, ExpectedText: item.PackageText, ExpectedBytes: item.PackageBytes, ExpectedSHA256: item.PackageSHA256, SourceShouldPublish: true})
			default:
				witness = rs26cOfficialFixtureWitness(t, rs26cFrozenFixtureCase{Name: item.Name, Source: item.Source, SourcePath: "/readspan26d-forms/app.ts", SourceSHA256: item.SourceSHA256, SourceBytes: item.SourceBytes, RequestText: item.RequestText, ExpectedText: item.PackageText, ExpectedBytes: item.PackageBytes, ExpectedSHA256: item.PackageSHA256})
			}
			data := ao23JCS(t, witness)
			t.Logf("TSFG_M26D_FORMS_OFFICIAL_WHOLE name=%s bytes=%d sha256=%s base64=%s", item.Name, len(data), ao23Hash(data), base64.StdEncoding.EncodeToString(data))
		})
	}
}

// Root qualifies the separate official5 run before executing this public5 selector.
// All five are Source-positive; the SCC names describe only Product decisions.
func TestM26dArrayReturnFormsPublicHandler(t *testing.T) {
	for _, item := range m26dReadForms(t) {
		t.Run(item.Name, func(t *testing.T) {
			input := []byte(item.RequestText)
			before := bytes.Clone(input)
			var output readSpanParameterPublicWriter26b
			err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v7"}, bytes.NewReader(input), &output)
			t.Logf("TSFG_M26D_FORMS_PUBLIC_ACTUAL name=%s bytes=%d sha256=%s writes=%d base64=%s", item.Name, output.Len(), ao23Hash(output.Bytes()), output.writes, base64.StdEncoding.EncodeToString(output.Bytes()))
			ao23Want(t, bytes.Equal(input, before), "whole public request remains immutable")
			if err != nil {
				ao23Want(t, output.Len() == 0 && output.writes == 0, "failed publication remains atomic")
				t.Fatalf("existing Source form must publish complete current facts7 before Product classification: %s", readSpanParameterPublicError26b(err))
			}
			ao23Want(t, output.writes == 1 && bytes.Equal(output.Bytes(), []byte(item.PackageText)), "one complete publication equals independent whole literal")
		})
	}
}
