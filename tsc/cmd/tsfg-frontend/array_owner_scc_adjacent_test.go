// SPDX-License-Identifier: Apache-2.0
// Separate frozen adjacent witnesses preserve the first causal test bytes.
// This original-object traversal additionally covers local ReadSpan construction.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
)

func scc23fReadAdjacentWhole(t *testing.T) scc23fWholeFixture {
	t.Helper()
	const file = "testdata/array-owner-scc-v7/adjacent-whole-facts.json"
	raw, err := os.ReadFile(file)
	ao23Want(t, err == nil && len(raw) == 67793 && ao23Hash(raw) == "bb984cb8e5c6b7c3ac085c2b299be431bb2dedb8bd2fd55c22d96560693aef81", "complete independent adjacent SCC fixture")
	before := bytes.Clone(raw)
	t.Cleanup(func() {
		after, err := os.ReadFile(file)
		if err != nil || !bytes.Equal(after, before) || !bytes.Equal(raw, before) {
			t.Errorf("independent adjacent fixture changed: %v", err)
		}
	})
	var fixture scc23fWholeFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	names := []string{"missing-fixed-scalar-self", "missing-fixed-array-self", "mutual-fullslots-duplicate-incoming-fresh", "outgoing-transitive-fresh-capability", "outgoing-transitive-local-view-capability", "direct-fresh-capability", "direct-local-view-capability"}
	ao23Want(t, fixture.FactsSchemaHash == "sha256:d2ee930b76b8fd5c2270ad69579c9c61377831a97034d3a8890d8aa770ebabc2" && len(fixture.Cases) == len(names), "entire seven-source adjacent family")
	for i, item := range fixture.Cases {
		ao23Want(t, item.Name == names[i] && item.SourceShouldPublish && len(item.Source) == item.SourceBytes && ao23Hash([]byte(item.Source)) == item.SourceSHA256, "exact original adjacent sources")
		ao23Want(t, len(item.ExpectedText) == item.ExpectedBytes && ao23Hash([]byte(item.ExpectedText)) == item.ExpectedSHA256, "whole independently authored adjacent packet")
	}
	return fixture
}

func scc23fAdjacentFunctions(s *rs26cOfficialScope, headers []*ao23Header) []any {
	rows, inventory := []any{}, []any{}
	ao23Want(s.t, len(headers) > 0 && len(headers) <= 16, "all original SCC functions")
	for _, h := range headers {
		s.current = h.row["id"].(string)
		s.bindings, s.views = map[*ast.Symbol]ao23Binding{}, map[*ast.Symbol]string{}
		ao23Want(s.t, len(h.parameters) <= 8 && h.row["returnType"] == "f64", "complete bounded scalar-result original signature")
		for i, sym := range h.signature.Parameters() {
			ao23Want(s.t, h.kinds[i] == "array.f64" || h.kinds[i] == "f64", "selected original full slot kind")
			s.bindings[sym] = ao23Binding{h.parameters[i]["id"].(string), h.kinds[i]}
		}
		nodes := h.node.AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
		ao23Want(s.t, len(nodes) > 0 && len(nodes) <= 50, "complete bounded original statement body")
		bindings, body := []any{}, []any{}
		beforeCalls, beforeCreates := s.counts["calls"], s.counts["viewCreates"]
		effects := 0
		for i, node := range nodes {
			var row ao23Map
			switch node.Kind {
			case ast.KindVariableStatement:
				list := node.AsVariableStatement().DeclarationList
				ao23Want(s.t, list != nil && len(list.AsVariableDeclarationList().Declarations.Nodes) == 1, "one whole original Array local")
				decl := list.AsVariableDeclarationList().Declarations.Nodes[0]
				var binding ao23Map
				if s.check.IsArrayType(s.check.GetTypeAtLocation(decl.Name())) {
					binding, row = s.local(node)
				} else {
					binding, row = s.viewLocal26c(node)
				}
				bindings = append(bindings, binding)
			case ast.KindExpressionStatement:
				expression := node.AsExpressionStatement().Expression
				ao23Want(s.t, expression != nil, "complete original expression statement")
				effects++
				if expression.Kind == ast.KindCallExpression {
					row = ao23Map{"kind": "call", "range": s.span(s.file, node), "value": s.call26c(expression)}
				} else {
					ao23Want(s.t, expression.Kind == ast.KindBinaryExpression, "selected plain original Array mutation")
					row = s.mutation26c(node)
				}
			case ast.KindReturnStatement:
				ao23Want(s.t, i == len(nodes)-1, "single final original return")
				value := node.AsReturnStatement().Expression
				ao23Want(s.t, value != nil && s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(value), s.check.GetReturnTypeOfSignature(h.signature)), "real original return assignability")
				row = ao23Map{"kind": "return", "range": s.span(s.file, node), "value": s.value26c(value, "f64", "return")}
			default:
				s.t.Fatalf("uncovered whole original SCC statement %s", node.Kind)
			}
			body = append(body, row)
			s.counts["statements"]++
		}
		calls := s.counts["calls"] - beforeCalls
		ao23Want(s.t, nodes[len(nodes)-1].Kind == ast.KindReturnStatement && len(bindings) <= 17 && calls+s.counts["viewCreates"]-beforeCreates <= 32 && effects <= 32, "all original local/per-function raw limits")
		h.row["bindings"], h.row["body"] = bindings, body
		rows = append(rows, h.row)
		inventory = append(inventory, ao23Map{"name": h.node.Name().Text(), "parameters": len(h.parameters), "statements": len(nodes), "locals": len(bindings), "ordinaryCalls": calls})
	}
	s.proof["completeOrdinaryGraph"], s.proof["functionInventory"] = s.edges, inventory
	return rows
}

func scc23fAdjacentWholeWitness(t *testing.T, item scc23fWholeCase) ao23Map {
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
	ao23Want(t, envelope.Family == "tsfg.array-facts-request-candidate" && envelope.Version == 7 && in.RootFile == item.SourcePath && len(in.Files) == 2 && in.Config.Strict && in.Config.NoEmit && !in.Config.AllowJS && in.Declarations.Version == "minimal-arrays-v3" && len(in.Declarations.Paths) == 1, "complete exact private7 request")
	dtsPath := in.Declarations.Paths[0]
	contents, files, owned := map[string]string{}, []any{}, []any{}
	for _, file := range in.Files {
		_, duplicate := contents[file.Path]
		ao23Want(t, !duplicate && ao23Hash([]byte(file.Content)) == file.SHA256, "unique original full file/checksum")
		contents[file.Path] = file.Content
		files = append(files, ao23Map{"path": file.Path, "content": file.Content, "sha256": file.SHA256})
		owned = append(owned, ao23Map{"path": file.Path, "content": file.Content, "sha256": file.SHA256, "byteLength": len(file.Content), "utf16Length": ao23Units(file.Content)})
	}
	ao23Want(t, contents[item.SourcePath] == item.Source && contents[dtsPath] == rs26cOfficialDTSCandidate && len(contents[dtsPath]) == 484 && ao23Hash([]byte(contents[dtsPath])) == "282b335a6e0e0493fa74a3c1dd56f234f8c679079acdaf0600cefb8306c26415", "complete independent source and pinned DTS3")
	base, release := rs26bAdjacentProgram(t, rs26bAdjacentCase{ID: item.Name, Text: item.Source, SourceRoot: in.SourceRoot, SourcePath: item.SourcePath, DeclarationPath: dtsPath}, contents[dtsPath])
	defer release()
	s := &rs26cOfficialScope{rs26Scope: base, edges: map[string][]string{}}
	types, arrayLength, viewLength, intrinsic := s.origins()
	s.rs26bAST()
	headers := s.headers26c()
	functions := scc23fAdjacentFunctions(s, headers)
	exports := s.actualExports(headers)
	config := ao23Map{"strict": true, "noEmit": true, "allowJs": false}
	declarations := ao23Map{"version": "minimal-arrays-v3", "paths": []string{dtsPath}}
	request := ao23Map{"family": "tsfg.array-facts-request-candidate", "version": 7, "input": ao23Map{"sourceRoot": in.SourceRoot, "rootFile": item.SourcePath, "files": files, "config": config, "declarations": declarations}}
	requestText := append(ao23JCS(t, request), '\n')
	ao23Want(t, bytes.Equal(requestText, []byte(item.RequestText)), "entire official-only request matches literal")
	payload := ao23Map{"sourceRoot": in.SourceRoot, "rootFile": item.SourcePath, "positionUnit": "utf16", "config": config, "declarations": declarations, "files": owned, "types": types, "arrayLength": arrayLength, "readSpanLength": viewLength, "readSpanIntrinsic": intrinsic, "functions": functions, "exports": exports, "diagnostics": []any{}}
	schema := "sha256:d2ee930b76b8fd5c2270ad69579c9c61377831a97034d3a8890d8aa770ebabc2"
	requestID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.request.v7\x00"), requestText...))
	packageID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.package.v7\x00"+schema+"\x00"), ao23JCS(t, payload)...))
	packet := ao23Map{"family": "tsfg.array-facts-package-candidate", "version": 7, "schemaHash": schema, "requestId": requestID, "packageId": packageID, "payload": payload}
	packetText := append(ao23JCS(t, packet), '\n')
	if !bytes.Equal(packetText, []byte(item.ExpectedText)) {
		t.Fatalf("official-only whole packet differs from independent literal %s\nactual=%s\nexpected=%s", item.Name, packetText, item.ExpectedText)
	}
	s.proof["counters"] = s.counts
	return ao23Map{"name": item.Name, "sourcePath": item.SourcePath, "source": item.Source, "sourceSHA256": item.SourceSHA256, "sourceBytes": item.SourceBytes, "official": s.proof, "officialPacketText": string(packetText), "SourceProducerCalled": false, "ProductModuleCalled": false, "officialFactsPacketConstructed": true, "sourceShouldPublish": item.SourceShouldPublish}
}

func TestArrayOwner23fOfficialFreshWholeAdjacent(t *testing.T) {
	for _, item := range scc23fReadAdjacentWhole(t).Cases {
		t.Run(item.Name, func(t *testing.T) {
			witness := ao23JCS(t, scc23fAdjacentWholeWitness(t, item))
			t.Logf("TSFG_ARRAY_OWNER_23F_ADJACENT_OFFICIAL_WHOLE name=%s bytes=%d sha256=%s base64=%s", item.Name, len(witness), ao23Hash(witness), base64.StdEncoding.EncodeToString(witness))
		})
	}
}

func TestArrayOwner23fV7PublicWholeAdjacent(t *testing.T) {
	for _, item := range scc23fReadAdjacentWhole(t).Cases {
		t.Run(item.Name, func(t *testing.T) {
			input := []byte(item.RequestText)
			before := bytes.Clone(input)
			var output readSpanParameterPublicWriter26b
			err := runArrayFactsCandidate(t.Context(), bytes.NewReader(input), &output)
			t.Logf("TSFG_ARRAY_OWNER_23F_ADJACENT_PUBLIC_ACTUAL name=%s bytes=%d sha256=%s writes=%d base64=%s", item.Name, output.Len(), ao23Hash(output.Bytes()), output.writes, base64.StdEncoding.EncodeToString(output.Bytes()))
			ao23Want(t, bytes.Equal(input, before), "original adjacent request unchanged")
			if err != nil {
				ao23Want(t, output.Len() == 0 && output.writes == 0, "all failed publications are atomic")
				t.Fatalf("official-valid adjacent source must publish whole private7 facts before Product domain classification: %s", readSpanParameterPublicError26b(err))
			}
			ao23Want(t, output.writes == 1 && bytes.Equal(output.Bytes(), []byte(item.ExpectedText)), "one complete publication equals independent adjacent packet")
		})
	}
}
