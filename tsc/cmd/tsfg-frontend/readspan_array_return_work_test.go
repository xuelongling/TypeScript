// SPDX-License-Identifier: Apache-2.0
// Source publishes both independent boundary packets; Product alone charges view work.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
)

func m26dReadWorkPair(t *testing.T) []readSpan26dPriorCase {
	t.Helper()
	const file = "testdata/readspan-array-returns-v7/global-bound-expected-facts.json"
	raw, err := os.ReadFile(file)
	ao23Want(t, err == nil && len(raw) == 1013550 && ao23Hash(raw) == "24fab08916eb5e212acb9f32593a0c300cd112677bd688818f338ff3b00b0e4f", "complete independently frozen global-bound Source fixture")
	before := bytes.Clone(raw)
	t.Cleanup(func() {
		after, err := os.ReadFile(file)
		if err != nil || !bytes.Equal(after, before) || !bytes.Equal(raw, before) {
			t.Errorf("global-bound fixture changed: %v", err)
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
	ao23Want(t, fixture.Family == "tsfg.m26d-independent-global-bound-source-whole" && fixture.Version == 1 && fixture.SourceRoot == "/readspan26d-budget" && fixture.RootFile == "/readspan26d-budget/app.ts" && fixture.SchemaHash == "sha256:d2ee930b76b8fd5c2270ad69579c9c61377831a97034d3a8890d8aa770ebabc2" && len(fixture.Cases) == 2, "entire current facts7 pair inventory")
	for i, item := range fixture.Cases {
		ao23Want(t, item.Name == []string{"global1024", "global1025"}[i] && item.SourceBytes == []int{6391, 6404}[i] && len(item.Source) == item.SourceBytes && ao23Units(item.Source) == item.SourceBytes && ao23Hash([]byte(item.Source)) == item.SourceSHA256, "complete original source/UTF16/hash")
		ao23Want(t, len(item.RequestText) == []int{7621, 7635}[i] && item.PackageBytes == []int{228551, 228998}[i] && len(item.PackageText) == item.PackageBytes && ao23Hash([]byte(item.PackageText)) == item.PackageSHA256, "full independent request and facts packet")
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

// The ordinary traversal is the established26c original-object traversal.
// The additional property-access branch uses the existing pinned length witness.
// Neither this test nor the reused helpers call the Source producer or Product.
func m26dWorkFunctions(s *rs26cOfficialScope, headers []*ao23Header, lengthUses int) []any {
	rows, inventory := []any{}, []any{}
	ao23Want(s.t, len(headers) == 8, "all eight original functions")
	for j, h := range headers {
		wantName := "borrow"
		if j > 0 && j < 7 {
			wantName = []string{"caller0", "caller1", "caller2", "caller3", "caller4", "caller5"}[j-1]
		}
		if j == 7 {
			wantName = "entry"
		}
		ao23Want(s.t, h.node.Name().Text() == wantName, "complete ordered original function inventory")
		s.current = h.row["id"].(string)
		s.bindings, s.views = map[*ast.Symbol]ao23Binding{}, map[*ast.Symbol]string{}
		for i, sym := range h.signature.Parameters() {
			if h.kinds[i] == "readspan.f64" {
				s.viewType(s.check.GetTypeOfSymbol(sym), s.check.GetNumberType())
				s.views[sym] = h.parameters[i]["id"].(string)
			} else {
				s.bindings[sym] = ao23Binding{h.parameters[i]["id"].(string), h.kinds[i]}
			}
		}
		parameterCount := 8
		if j == 7 {
			parameterCount = 0
		}
		ao23Want(s.t, len(h.parameters) == parameterCount, "complete required original parameter slots")
		nodes := h.node.AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
		wantStatements := 21
		if j == 0 {
			wantStatements = 2
		}
		if j == 6 {
			wantStatements = 22 + lengthUses
		}
		if j == 7 {
			wantStatements = 1
		}
		ao23Want(s.t, len(nodes) == wantStatements && len(nodes) <= 50, "complete bounded original statements")
		bindings, body := []any{}, []any{}
		beforeCalls, beforeCreates := s.counts["calls"], s.counts["viewCreates"]
		effects := 0
		for i, n := range nodes {
			var row ao23Map
			switch n.Kind {
			case ast.KindVariableStatement:
				list := n.AsVariableStatement().DeclarationList
				ao23Want(s.t, list != nil && len(list.AsVariableDeclarationList().Declarations.Nodes) == 1, "complete single original const local")
				decl := list.AsVariableDeclarationList().Declarations.Nodes[0]
				var binding ao23Map
				if s.check.IsArrayType(s.check.GetTypeAtLocation(decl.Name())) {
					binding, row = s.local(n)
				} else {
					binding, row = s.viewLocal26c(n)
				}
				bindings = append(bindings, binding)
			case ast.KindExpressionStatement:
				e := n.AsExpressionStatement().Expression
				ao23Want(s.t, e != nil, "complete original effect expression")
				effects++
				switch e.Kind {
				case ast.KindBinaryExpression:
					row = s.mutation26c(n)
				case ast.KindCallExpression:
					row = ao23Map{"kind": "call", "range": s.span(s.file, n), "value": s.call26c(e)}
				case ast.KindPropertyAccessExpression:
					row = ao23Map{"kind": "readSpanUse", "range": s.span(s.file, n), "value": s.rsAdj26Length(e)}
				default:
					row = s.viewRead(n)
				}
			case ast.KindReturnStatement:
				ao23Want(s.t, i == len(nodes)-1, "single final original return")
				v := n.AsReturnStatement().Expression
				ao23Want(s.t, v != nil && s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(v), s.check.GetReturnTypeOfSignature(h.signature)), "real original return type/assignability")
				row = ao23Map{"kind": "return", "range": s.span(s.file, n), "value": s.value26c(v, h.row["returnType"].(string), "return")}
			default:
				s.t.Fatalf("uncovered original boundary statement %s", n.Kind)
			}
			body = append(body, row)
			s.counts["statements"]++
		}
		calls, creates := s.counts["calls"]-beforeCalls, s.counts["viewCreates"]-beforeCreates
		wantCalls, wantCreates, wantLocals := 20, 0, 0
		if j == 0 {
			wantCalls, wantCreates, wantLocals = 0, 1, 1
		}
		if j == 6 {
			wantCalls = 21
		}
		if j == 7 {
			wantCalls = 0
		}
		ao23Want(s.t, calls == wantCalls && creates == wantCreates && len(bindings) == wantLocals && calls+creates <= 32 && effects <= 32 && len(bindings) <= 17 && nodes[len(nodes)-1].Kind == ast.KindReturnStatement, "complete ordinary/intrinsic/local/raw original inventory")
		h.row["bindings"], h.row["body"] = bindings, body
		rows = append(rows, h.row)
		inventory = append(inventory, ao23Map{"name": wantName, "parameters": len(h.parameters), "statements": len(nodes), "locals": len(bindings), "ordinaryCalls": calls, "intrinsicCalls": creates})
	}
	ao23Want(s.t, s.counts["functions"] == 8 && s.counts["parameters"] == 56 && s.counts["viewParameters"] == 49 && s.counts["calls"] == 121 && s.counts["viewCreates"] == 1 && s.counts["viewLengths"] == lengthUses && s.counts["viewReferences"] == 848+lengthUses, "complete original functions/slots/calls/view-use occurrence counts")
	borrow := headers[0].row["id"].(string)
	for j, h := range headers {
		edges := s.edges[h.row["id"].(string)]
		wantEdges := 20
		if j == 0 || j == 7 {
			wantEdges = 0
		}
		if j == 6 {
			wantEdges = 21
		}
		ao23Want(s.t, len(edges) == wantEdges, "all repeated ordinary edges retained")
		for _, target := range edges {
			ao23Want(s.t, target == borrow, "actual original callee is the sole borrow helper")
		}
	}
	s.dag26c()
	s.proof["functionInventory"] = inventory
	return rows
}

func m26dWorkOfficialWhole(t *testing.T, item readSpan26dPriorCase) ao23Map {
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
	ao23Want(t, envelope.Family == "tsfg.array-facts-request-candidate" && envelope.Version == 7 && in.SourceRoot == "/readspan26d-budget" && in.RootFile == "/readspan26d-budget/app.ts" && len(in.Files) == 2 && in.Config.Strict && in.Config.NoEmit && !in.Config.AllowJS && in.Declarations.Version == "minimal-arrays-v3" && len(in.Declarations.Paths) == 1, "complete private7 two-file original request")
	dtsPath := in.Declarations.Paths[0]
	contents, files, owned := map[string]string{}, []any{}, []any{}
	for _, f := range in.Files {
		_, duplicate := contents[f.Path]
		ao23Want(t, !duplicate && ao23Hash([]byte(f.Content)) == f.SHA256, "unique complete original input/checksum")
		contents[f.Path] = f.Content
		files = append(files, ao23Map{"path": f.Path, "content": f.Content, "sha256": f.SHA256})
		owned = append(owned, ao23Map{"path": f.Path, "content": f.Content, "sha256": f.SHA256, "byteLength": len(f.Content), "utf16Length": ao23Units(f.Content)})
	}
	ao23Want(t, contents[in.RootFile] == item.Source && contents[dtsPath] == rs26cOfficialDTSCandidate && len(contents[dtsPath]) == 484 && ao23Hash([]byte(contents[dtsPath])) == "282b335a6e0e0493fa74a3c1dd56f234f8c679079acdaf0600cefb8306c26415", "complete source and unchanged pinned nominal declarations")
	base, release := rs26bAdjacentProgram(t, rs26bAdjacentCase{ID: item.Name, Text: item.Source, SourceRoot: in.SourceRoot, SourcePath: in.RootFile, DeclarationPath: dtsPath}, contents[dtsPath])
	defer release()
	s := &rs26cOfficialScope{rs26Scope: base, edges: map[string][]string{}}
	types, arrayLength, viewLength, intrinsic := s.origins()
	s.rs26bAST()
	headers := s.headers26c()
	lengthUses := 5
	if item.Name == "global1025" {
		lengthUses = 6
	}
	functions := m26dWorkFunctions(s, headers, lengthUses)
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
	ao23Want(t, bytes.Equal(packetText, []byte(item.PackageText)), "complete original-object facts packet equals independent expected packet")
	s.proof["counters"] = s.counts
	return ao23Map{"name": item.Name, "sourcePath": in.RootFile, "source": item.Source, "sourceBytes": len(item.Source), "sourceSHA256": item.SourceSHA256, "official": s.proof, "officialPacketText": string(packetText), "SourceProducerCalled": false, "ProductModuleCalled": false, "officialFactsPacketConstructed": true}
}

func TestM26dArrayReturnWorkOfficialProgram(t *testing.T) {
	for _, item := range m26dReadWorkPair(t) {
		t.Run(item.Name, func(t *testing.T) {
			data := ao23JCS(t, m26dWorkOfficialWhole(t, item))
			t.Logf("TSFG_M26D_WORK_OFFICIAL_WHOLE name=%s bytes=%d sha256=%s base64=%s", item.Name, len(data), ao23Hash(data), base64.StdEncoding.EncodeToString(data))
		})
	}
}

// Root qualifies official/global1024 and official/global1025 before either public run.
// Both Source packets must publish; upper-view-work refusal belongs solely to Product.
func TestM26dArrayReturnWorkPublicHandler(t *testing.T) {
	for _, item := range m26dReadWorkPair(t) {
		t.Run(item.Name, func(t *testing.T) {
			input := []byte(item.RequestText)
			before := bytes.Clone(input)
			var output readSpanParameterPublicWriter26b
			err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v7"}, bytes.NewReader(input), &output)
			t.Logf("TSFG_M26D_WORK_PUBLIC_ACTUAL name=%s bytes=%d sha256=%s writes=%d base64=%s", item.Name, output.Len(), ao23Hash(output.Bytes()), output.writes, base64.StdEncoding.EncodeToString(output.Bytes()))
			ao23Want(t, bytes.Equal(input, before), "complete public request remains unchanged")
			if err != nil {
				ao23Want(t, output.Len() == 0 && output.writes == 0, "any Source failure publishes no partial packet")
				t.Fatalf("both existing Source boundary programs must publish full facts7: %s", readSpanParameterPublicError26b(err))
			}
			ao23Want(t, output.writes == 1 && bytes.Equal(output.Bytes(), []byte(item.PackageText)), "one complete Source publication equals independent whole boundary literal")
		})
	}
}
