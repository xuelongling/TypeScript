// SPDX-License-Identifier: Apache-2.0
// Only fresh official objects and old independent test helpers construct these packets.
package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

const rs26FrozenPin = "f7dd26e2d3929d718dcf6fd930436b0b0d2c602a67c1e165badd9173ceabcc73"
const rs26Schema = "sha256:1364677f060da070ab1e14a589c1957d9ece10eaf8eb81744cea2a99e410a373"

func rs26Packet(t *testing.T, name, source, dts string) []byte {
	s, release := rs26Program(t, source, dts)
	defer release()
	types, arrayLength, viewLength, intrinsic := s.origins()
	headers := s.collectHeaders()
	functions := s.firstFunctions(headers)
	exports := s.actualExports(headers)
	inputFiles, owned := []any{}, []any{}
	for i, text := range []string{source, dts} {
		path := []string{s.file.FileName(), s.dts.FileName()}[i]
		row := ao23Map{"path": path, "content": text, "sha256": ao23Hash([]byte(text))}
		inputFiles = append(inputFiles, row)
		owned = append(owned, ao23Map{"path": path, "content": text, "sha256": row["sha256"], "byteLength": len(text), "utf16Length": ao23Units(text)})
	}
	config := ao23Map{"strict": true, "noEmit": true, "allowJs": false}
	declarations := ao23Map{"version": "minimal-arrays-v3", "paths": []string{s.dts.FileName()}}
	input := ao23Map{"sourceRoot": "/readspan26a", "rootFile": s.file.FileName(), "files": inputFiles, "config": config, "declarations": declarations}
	request := ao23Map{"family": "tsfg.array-facts-request-candidate", "version": 4, "input": input}
	requestBytes := append(ao23JCS(t, request), '\n')
	payload := ao23Map{"sourceRoot": input["sourceRoot"], "rootFile": input["rootFile"], "positionUnit": "utf16", "config": config, "declarations": declarations, "files": owned, "types": types, "arrayLength": arrayLength, "readSpanLength": viewLength, "readSpanIntrinsic": intrinsic, "functions": functions, "exports": exports, "diagnostics": []any{}}
	requestID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.request.v4\x00"), requestBytes...))
	packageID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.package.v4\x00"+rs26Schema+"\x00"), ao23JCS(t, payload)...))
	packet := ao23Map{"family": "tsfg.array-facts-package-candidate", "version": 4, "schemaHash": rs26Schema, "requestId": requestID, "packageId": packageID, "payload": payload}
	packetBytes := append(ao23JCS(t, packet), '\n')
	entryBody := functions[1].(ao23Map)["body"].([]any)
	callIndex, readIndex := 3, 4
	if name == "after" {
		callIndex, readIndex = 4, 3
	}
	create := entryBody[1].(ao23Map)["initializer"].(ao23Map)
	alias := entryBody[2].(ao23Map)
	call := entryBody[callIndex].(ao23Map)["value"].(ao23Map)
	read := entryBody[readIndex].(ao23Map)["value"].(ao23Map)
	wantCall, wantRead := ao23Range{194, 203}, ao23Range{207, 215}
	if name == "after" {
		wantCall, wantRead = ao23Range{206, 215}, ao23Range{194, 202}
	}
	ao23Want(t, create["range"] == ao23Range{154, 171} && alias["range"] == ao23Range{175, 191} && alias["initializer"].(ao23Map)["range"] == ao23Range{189, 190} && call["range"] == wantCall && read["range"] == wantRead, "fresh official AST/PositionMap agrees with independently frozen first233B semantic anchors")
	s.proof["firstAnchors"] = ao23Map{"create": create["range"], "aliasStatement": alias["range"], "aliasInitializer": alias["initializer"].(ao23Map)["range"], "ordinaryCall": call["range"], "lastViewRead": read["range"]}
	ao23Want(t, s.counts["functions"] == 2 && s.counts["parameters"] == 1 && s.counts["calls"] == 1 && s.counts["allocations"] == 1 && s.counts["viewCreates"] == 1 && s.counts["viewAliases"] == 1 && s.counts["viewReads"] == 1 && s.counts["viewReferences"] == 2 && s.counts["instantiatedParameterDeclarationJoins"] == 3 && s.counts["statements"] == 8, "complete first-pair original AST inventory, no Source node omitted")
	s.proof["counters"] = s.counts
	s.proof["checks"] = []string{"all4diagnostics0", "whole two-file closure", "actual pinned readonly/generic origins", "distinct scoped original T symbols", "instantiated parameter original declaration joins", "full original regular callee signature/argument symbols", "complete Source/UTF16/nominal binding and alias roles"}
	bundle := ao23Map{"name": name, "contractSHA256": rs26FrozenPin, "requestText": string(requestBytes), "packageText": string(packetBytes), "official": s.proof, "metrics": ao23Map{"request": ao23Metrics(t, request), "package": ao23Metrics(t, packet)}}
	return ao23JCS(t, bundle)
}
func TestReadSpan26aIndependentFirstOfficial(t *testing.T) {
	text, err := os.ReadFile("testdata/readspan-last-use-v4/first-source-texts.json")
	if err != nil {
		t.Fatal(err)
	}
	ao23Want(t, len(text) == 914 && ao23Hash(text) == "d7c2179db778c117633abf4a6927a95e4b350698bd5bb4faac3d32eeba06589e", "exact frozen first233B pair input")
	var first struct {
		SourcePath    string
		Before, After struct {
			Content, SHA256 string
			Bytes           int
		}
	}
	if err := json.Unmarshal(text, &first); err != nil {
		t.Fatal(err)
	}
	ao23Want(t, first.SourcePath == "/readspan26a/app.ts", "pinned first source path")
	dts, err := os.ReadFile("testdata/array-facts-v4/minimal-arrays-v3.d.ts")
	if err != nil {
		t.Fatal(err)
	}
	ao23Want(t, len(dts) == 484 && ao23Hash(dts) == "282b335a6e0e0493fa74a3c1dd56f234f8c679079acdaf0600cefb8306c26415", "exact readonly generic DTS4 origin")
	schema, err := os.ReadFile("testdata/array-facts-v4/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	ao23Want(t, len(schema) == 33428 && "sha256:"+ao23Hash(schema) == rs26Schema, "exact facts4 descriptor bytes")
	for _, name := range []string{"before", "after"} {
		t.Run(name, func(t *testing.T) {
			item := first.Before
			if name == "after" {
				item = first.After
			}
			ao23Want(t, item.Bytes == 233 && len(item.Content) == 233 && ao23Units(item.Content) == 233 && ao23Hash([]byte(item.Content)) == item.SHA256, "complete original first233B source")
			raw := rs26Packet(t, name, item.Content, string(dts))
			t.Logf("TSFG_READSPAN_26A_INDEPENDENT_PACKET name=%s bytes=%d sha256=%s base64=%s", name, len(raw), ao23Hash(raw), base64.StdEncoding.EncodeToString(raw))
		})
	}
}
