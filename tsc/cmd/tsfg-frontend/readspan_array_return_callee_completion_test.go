// SPDX-License-Identifier: Apache-2.0
// Source records callee mutations and standalone view lengths; Product owns their effects.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
)

func m26dReadCalleeCompletion(t *testing.T) []readSpan26dPriorCase {
	t.Helper()
	const file = "testdata/readspan-array-returns-v7/callee-completion-expected-facts.json"
	raw, err := os.ReadFile(file)
	ao23Want(t, err == nil && len(raw) == 111930 && ao23Hash(raw) == "4d32d4657243c536b6bf7bbd50938981e9551616c3b2c02bb5f1e741e36deca4", "complete independent callee-completion whole fixture")
	before := bytes.Clone(raw)
	t.Cleanup(func() {
		after, err := os.ReadFile(file)
		if err != nil || !bytes.Equal(after, before) || !bytes.Equal(raw, before) {
			t.Errorf("independent callee-completion fixture changed: %v", err)
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
	ao23Want(t, fixture.Family == "tsfg.m26d-independent-callee-completion-source-whole-oracle" && fixture.Version == 1 && fixture.SourceRoot == "/readspan26d-semantic" && fixture.RootFile == "/readspan26d-semantic/app.ts" && fixture.SchemaHash == "sha256:d2ee930b76b8fd5c2270ad69579c9c61377831a97034d3a8890d8aa770ebabc2" && len(fixture.Cases) == 3, "complete current facts7 callee-completion inventory")
	for i, item := range fixture.Cases {
		ao23Want(t, item.Name == []string{"callee-modify-before-create", "callee-create-before-modify", "caller-old-live-view-consumes-fixed-S"}[i] && item.SourceBytes == []int{238, 238, 300}[i] && len(item.Source) == item.SourceBytes && ao23Units(item.Source) == item.SourceBytes && ao23Hash([]byte(item.Source)) == item.SourceSHA256, "complete original source/UTF16/hash")
		ao23Want(t, len(item.RequestText) == []int{1340, 1340, 1404}[i] && item.PackageBytes == []int{9293, 9294, 10833}[i] && len(item.PackageText) == item.PackageBytes && ao23Hash([]byte(item.PackageText)) == item.PackageSHA256, "entire independent request and facts packet")
		for _, frame := range []struct {
			data json.RawMessage
			text string
		}{{item.Request, item.RequestText}, {item.Package, item.PackageText}} {
			var value any
			if err := json.Unmarshal(frame.data, &value); err != nil {
				t.Fatal(err)
			}
			ao23Want(t, bytes.Equal(append(ao23JCS(t, value), '\n'), []byte(frame.text)), "complete literal object equals its canonical whole frame")
		}
	}
	return fixture.Cases
}

// Keep the established26c traversal intact. This local body walker additionally
// visits standalone lengths through the existing original-symbol length witness.
func m26dCalleeCompletionFunctions(s *rs26cOfficialScope, headers []*ao23Header, lengthUses int) []any {
	ao23Want(s.t, len(headers) == 2 && headers[0].node.Name().Text() == "borrow" && headers[1].node.Name().Text() == "entry", "complete original borrow/entry function inventory")
	rows, inventory := []any{}, []any{}
	for j, h := range headers {
		s.current = h.row["id"].(string)
		s.bindings, s.views = map[*ast.Symbol]ao23Binding{}, map[*ast.Symbol]string{}
		for i, sym := range h.signature.Parameters() {
			ao23Want(s.t, j == 0 && i == 0 && h.kinds[i] == "array.f64", "original required Array parameter remains in full slot0")
			s.bindings[sym] = ao23Binding{h.parameters[i]["id"].(string), h.kinds[i]}
		}
		wantParameters, wantStatements, wantLocals, wantCalls, wantCreates, wantEffects := 1, 3, 1, 0, 1, 1
		if j == 1 {
			wantParameters, wantStatements, wantLocals, wantCalls, wantCreates, wantEffects = 0, 2+2*lengthUses, 1+lengthUses, 1, lengthUses-1, lengthUses
		}
		ao23Want(s.t, len(h.parameters) == wantParameters && h.row["returnType"] == []string{"readspan.f64", "f64"}[j], "complete original arity and nominal return type")
		nodes := h.node.AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
		ao23Want(s.t, len(nodes) == wantStatements && len(nodes) <= 50, "all original body statements retained in source order")
		bindings, body := []any{}, []any{}
		beforeCalls, beforeCreates, effects := s.counts["calls"], s.counts["viewCreates"], 0
		for i, n := range nodes {
			var row ao23Map
			switch n.Kind {
			case ast.KindVariableStatement:
				list := n.AsVariableStatement().DeclarationList
				ao23Want(s.t, list != nil && len(list.AsVariableDeclarationList().Declarations.Nodes) == 1, "complete single original local")
				decl := list.AsVariableDeclarationList().Declarations.Nodes[0]
				var binding ao23Map
				if s.check.IsArrayType(s.check.GetTypeAtLocation(decl.Name())) {
					binding, row = s.local(n)
				} else {
					binding, row = s.viewLocal26c(n)
				}
				bindings = append(bindings, binding)
			case ast.KindExpressionStatement:
				expression := n.AsExpressionStatement().Expression
				ao23Want(s.t, expression != nil, "complete original effect expression")
				effects++
				switch expression.Kind {
				case ast.KindBinaryExpression:
					row = s.mutation26c(n)
					ao23Want(s.t, j == 0 && row["kind"] == "lengthAssignment", "callee mutation joins the actual pinned writable Array length")
				case ast.KindPropertyAccessExpression:
					row = ao23Map{"kind": "readSpanUse", "range": s.span(s.file, n), "value": s.rsAdj26Length(expression)}
					ao23Want(s.t, j == 1, "standalone view length belongs to the caller")
				default:
					s.t.Fatalf("uncovered original callee-completion effect %s", expression.Kind)
				}
			case ast.KindReturnStatement:
				ao23Want(s.t, i == len(nodes)-1, "single final original return")
				value := n.AsReturnStatement().Expression
				ao23Want(s.t, value != nil && s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(value), s.check.GetReturnTypeOfSignature(h.signature)), "real original return assignment")
				row = ao23Map{"kind": "return", "range": s.span(s.file, n), "value": s.value26c(value, h.row["returnType"].(string), "return")}
			default:
				s.t.Fatalf("uncovered original callee-completion statement %s", n.Kind)
			}
			body = append(body, row)
			s.counts["statements"]++
		}
		calls, creates := s.counts["calls"]-beforeCalls, s.counts["viewCreates"]-beforeCreates
		ao23Want(s.t, len(bindings) == wantLocals && calls == wantCalls && creates == wantCreates && effects == wantEffects && len(bindings) <= 17 && calls+creates <= 32 && effects <= 32 && nodes[len(nodes)-1].Kind == ast.KindReturnStatement, "complete bounded original locals, calls, effects and final return")
		h.row["bindings"], h.row["body"] = bindings, body
		rows = append(rows, h.row)
		inventory = append(inventory, ao23Map{"name": h.node.Name().Text(), "parameters": len(h.parameters), "statements": len(nodes), "locals": len(bindings), "ordinaryCalls": calls, "intrinsicCalls": creates, "effects": effects})
	}
	ao23Want(s.t, s.counts["functions"] == 2 && s.counts["parameters"] == 1 && s.counts["viewParameters"] == 0 && s.counts["calls"] == 1 && s.counts["viewCreates"] == lengthUses && s.counts["mutations"] == 1 && s.counts["viewLengths"] == lengthUses && s.counts["viewReferences"] == lengthUses+1 && s.counts["statements"] == 5+2*lengthUses, "all actual original signature, mutation, call and view-use occurrences")
	borrow, entry := headers[0].row["id"].(string), headers[1].row["id"].(string)
	ao23Want(s.t, len(s.edges[borrow]) == 0 && len(s.edges[entry]) == 1 && s.edges[entry][0] == borrow, "complete real one-edge caller-to-borrow graph")
	s.dag26c()
	s.proof["functionInventory"] = inventory
	return rows
}

func m26dCalleeCompletionOfficialWhole(t *testing.T, item readSpan26dPriorCase) ao23Map {
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
	ao23Want(t, envelope.Family == "tsfg.array-facts-request-candidate" && envelope.Version == 7 && in.SourceRoot == "/readspan26d-semantic" && in.RootFile == "/readspan26d-semantic/app.ts" && len(in.Files) == 2 && in.Config.Strict && in.Config.NoEmit && !in.Config.AllowJS && in.Declarations.Version == "minimal-arrays-v3" && len(in.Declarations.Paths) == 1, "complete original private7 two-file request")
	dtsPath := in.Declarations.Paths[0]
	contents, files, owned := map[string]string{}, []any{}, []any{}
	for _, f := range in.Files {
		_, duplicate := contents[f.Path]
		ao23Want(t, !duplicate && ao23Hash([]byte(f.Content)) == f.SHA256, "unique complete original requested file and checksum")
		contents[f.Path] = f.Content
		files = append(files, ao23Map{"path": f.Path, "content": f.Content, "sha256": f.SHA256})
		owned = append(owned, ao23Map{"path": f.Path, "content": f.Content, "sha256": f.SHA256, "byteLength": len(f.Content), "utf16Length": ao23Units(f.Content)})
	}
	ao23Want(t, contents[in.RootFile] == item.Source && contents[dtsPath] == rs26cOfficialDTSCandidate && len(contents[dtsPath]) == 484 && ao23Hash([]byte(contents[dtsPath])) == "282b335a6e0e0493fa74a3c1dd56f234f8c679079acdaf0600cefb8306c26415", "entire original source and unchanged nominal declarations")
	base, release := rs26bAdjacentProgram(t, rs26bAdjacentCase{ID: item.Name, Text: item.Source, SourceRoot: in.SourceRoot, SourcePath: in.RootFile, DeclarationPath: dtsPath}, contents[dtsPath])
	defer release()
	s := &rs26cOfficialScope{rs26Scope: base, edges: map[string][]string{}}
	types, arrayLength, viewLength, intrinsic := s.origins()
	s.rs26bAST()
	headers := s.headers26c()
	lengthUses := 1
	if item.Name == "caller-old-live-view-consumes-fixed-S" {
		lengthUses = 2
	}
	functions := m26dCalleeCompletionFunctions(s, headers, lengthUses)
	exports := s.actualExports(headers)
	config := ao23Map{"strict": true, "noEmit": true, "allowJs": false}
	declarations := ao23Map{"version": "minimal-arrays-v3", "paths": []string{dtsPath}}
	request := ao23Map{"family": "tsfg.array-facts-request-candidate", "version": 7, "input": ao23Map{"sourceRoot": in.SourceRoot, "rootFile": in.RootFile, "files": files, "config": config, "declarations": declarations}}
	requestText := append(ao23JCS(t, request), '\n')
	ao23Want(t, bytes.Equal(requestText, []byte(item.RequestText)), "official-only whole request equals independent literal")
	payload := ao23Map{"sourceRoot": in.SourceRoot, "rootFile": in.RootFile, "positionUnit": "utf16", "config": config, "declarations": declarations, "files": owned, "types": types, "arrayLength": arrayLength, "readSpanLength": viewLength, "readSpanIntrinsic": intrinsic, "functions": functions, "exports": exports, "diagnostics": []any{}}
	schema := "sha256:d2ee930b76b8fd5c2270ad69579c9c61377831a97034d3a8890d8aa770ebabc2"
	requestID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.request.v7\x00"), requestText...))
	packageID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.package.v7\x00"+schema+"\x00"), ao23JCS(t, payload)...))
	packet := ao23Map{"family": "tsfg.array-facts-package-candidate", "version": 7, "schemaHash": schema, "requestId": requestID, "packageId": packageID, "payload": payload}
	packetText := append(ao23JCS(t, packet), '\n')
	ao23Want(t, bytes.Equal(packetText, []byte(item.PackageText)), "complete original-object facts packet equals independent whole literal")
	s.proof["counters"] = s.counts
	return ao23Map{"name": item.Name, "sourcePath": in.RootFile, "source": item.Source, "sourceBytes": len(item.Source), "sourceSHA256": item.SourceSHA256, "official": s.proof, "officialPacketText": string(packetText), "SourceProducerCalled": false, "ProductModuleCalled": false, "officialFactsPacketConstructed": true}
}

func TestM26dArrayReturnCalleeCompletionOfficialProgram(t *testing.T) {
	for _, item := range m26dReadCalleeCompletion(t) {
		t.Run(item.Name, func(t *testing.T) {
			data := ao23JCS(t, m26dCalleeCompletionOfficialWhole(t, item))
			t.Logf("TSFG_M26D_CALLEE_COMPLETION_OFFICIAL_WHOLE name=%s bytes=%d sha256=%s base64=%s", item.Name, len(data), ao23Hash(data), base64.StdEncoding.EncodeToString(data))
		})
	}
}

// Root qualifies official3 before this separate public3 run, each with fresh programs.
func TestM26dArrayReturnCalleeCompletionPublicFacts(t *testing.T) {
	for _, item := range m26dReadCalleeCompletion(t) {
		t.Run(item.Name, func(t *testing.T) {
			input := []byte(item.RequestText)
			before := bytes.Clone(input)
			var output readSpanParameterPublicWriter26b
			err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v7"}, bytes.NewReader(input), &output)
			t.Logf("TSFG_M26D_CALLEE_COMPLETION_PUBLIC_ACTUAL name=%s bytes=%d sha256=%s writes=%d base64=%s", item.Name, output.Len(), ao23Hash(output.Bytes()), output.writes, base64.StdEncoding.EncodeToString(output.Bytes()))
			ao23Want(t, bytes.Equal(input, before), "complete public request remains unchanged")
			if err != nil {
				ao23Want(t, output.Len() == 0 && output.writes == 0, "any Source failure publishes no partial packet")
				t.Fatalf("existing Source callee-completion form must publish full facts7: %s", readSpanParameterPublicError26b(err))
			}
			ao23Want(t, output.writes == 1 && bytes.Equal(output.Bytes(), []byte(item.PackageText)), "one complete Source publication equals independent whole literal")
		})
	}
}
