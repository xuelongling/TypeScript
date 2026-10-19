// SPDX-License-Identifier: Apache-2.0
// Complete adjacent packets precede any observation of their v4 producer output.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

type rsAdj26Case struct {
	Name, SourceRoot, SourcePath, Source, SourceSHA256 string
	SourceBytes, SourceUTF16                           int
	Recipe                                             ao23Map
}

func rsAdj26Anchors(t *testing.T, item rsAdj26Case, functions []any) ao23Map {
	want, ok := item.Recipe["diagnostic"].(map[string]any)
	if !ok {
		return nil
	}
	var entry ao23Map
	for _, f := range functions {
		row := f.(ao23Map)
		if row["name"] == "entry" {
			entry = row
		}
	}
	ao23Want(t, entry != nil, "original declared entry owns planned diagnostic")
	body := entry["body"].([]any)
	operation := body[int(item.Recipe["operationBody"].(float64))].(ao23Map)
	last := body[int(item.Recipe["lastUseBody"].(float64))].(ao23Map)
	primary := operation["expressionRange"]
	if operation["kind"] == "call" {
		primary = operation["value"].(ao23Map)["range"]
	}
	lastRange := last["value"]
	if last["kind"] == "variable" {
		lastRange = last["initializer"]
	}
	lastRange = lastRange.(ao23Map)["range"]
	aliases := []any{}
	var origin any
	for _, v := range body {
		row := v.(ao23Map)
		if row["kind"] != "variable" {
			continue
		}
		init := row["initializer"].(ao23Map)
		if init["kind"] == "readSpanCreate" && origin == nil {
			origin = init["range"]
		}
		if init["kind"] == "reference" && init["flowType"] == "readspan.f64" {
			aliases = append(aliases, row["range"])
		}
	}
	got := ao23Map{"primary": primary, "origin": origin, "aliasStatements": aliases, "lastUse": lastRange}
	for k, v := range got {
		ao23Want(t, bytes.Equal(ao23JCS(t, v), ao23JCS(t, want[k])), "complete independent diagnostic anchor equals original AST/PositionMap: "+k)
	}
	return got
}
func rsAdj26Packet(t *testing.T, item rsAdj26Case, dts string) []byte {
	ao23Want(t, item.SourceRoot == "/readspan26a" && item.SourcePath == "/readspan26a/app.ts" && len(item.Source) == item.SourceBytes && ao23Units(item.Source) == item.SourceUTF16 && ao23Hash([]byte(item.Source)) == item.SourceSHA256, "complete independently frozen Source")
	ao23Want(t, item.SourceBytes <= 13151, "small complete adjacent corpus, no provider/long input")
	dtsPath := item.SourceRoot + "/minimal-arrays-v3.d.ts"
	s, release := rsOld26Program(t, item.SourceRoot, item.SourcePath, dtsPath, item.Source, dts)
	defer release()
	types, property, viewLength, intrinsic := s.origins()
	headers := s.collectHeaders()
	functions, inventory := s.rsAdj26Functions(headers)
	exports := s.actualExports(headers)
	owned, files := []any{}, []any{}
	for i, text := range []string{item.Source, dts} {
		path := []string{item.SourcePath, dtsPath}[i]
		sha := ao23Hash([]byte(text))
		files = append(files, ao23Map{"path": path, "content": text, "sha256": sha})
		owned = append(owned, ao23Map{"path": path, "content": text, "sha256": sha, "byteLength": len(text), "utf16Length": ao23Units(text)})
	}
	config := ao23Map{"strict": true, "noEmit": true, "allowJs": false}
	declarations := ao23Map{"version": "minimal-arrays-v3", "paths": []string{dtsPath}}
	request := ao23Map{"family": "tsfg.array-facts-request-candidate", "version": 4, "input": ao23Map{"sourceRoot": item.SourceRoot, "rootFile": item.SourcePath, "config": config, "declarations": declarations, "files": files}}
	requestBytes := append(ao23JCS(t, request), '\n')
	s.proof["counters"] = s.counts
	s.proof["functions"] = inventory
	if anchors := rsAdj26Anchors(t, item, functions); anchors != nil {
		s.proof["diagnosticAnchors"] = anchors
	}
	s.proof["checks"] = []string{"fresh all4diagnostics0", "exact complete original Source/newDTS two-file closure", "14originalDTS declarations/4types/readonly generic symbols", "actual generic instantiation transient parameters join original declarations", "all source syntax/signatures/required params/aliases/readonly reads/length/RHS/fullcall/DAG/UTF16 roles", "pre-frozen ASCII diagnostic anchors equal official spans", "no production converter or Product module invoked"}
	bundle := ao23Map{"name": item.Name, "contractSHA256": rs26FrozenPin, "requestText": string(requestBytes), "official": s.proof, "recipe": item.Recipe, "metrics": ao23Map{"request": ao23Metrics(t, request)}}
	unsupported := item.Name == "raw-calls-33"
	for _, row := range inventory {
		rawCalls := row.(ao23Map)["rawCalls"].(int)
		ao23Want(t, rawCalls <= 32 || unsupported && rawCalls == 33, "all original per-function raw-call inventories agree with bounded corpus")
	}
	if unsupported {
		ao23Want(t, s.counts["viewCreates"] == 17 && s.counts["calls"] == 16 && s.counts["viewLocals"] == 17, "official-positive exact raw33-only private boundary")
		bundle["expectedSourceError"] = ao23Map{"code": "r03-array-facts-limit", "ruleId": "r01-input-boundary", "phase": "input"}
		return ao23JCS(t, bundle)
	}
	payload := ao23Map{"sourceRoot": item.SourceRoot, "rootFile": item.SourcePath, "positionUnit": "utf16", "config": config, "declarations": declarations, "files": owned, "types": types, "arrayLength": property, "readSpanLength": viewLength, "readSpanIntrinsic": intrinsic, "functions": functions, "exports": exports, "diagnostics": []any{}}
	requestID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.request.v4\x00"), requestBytes...))
	packageID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.package.v4\x00"+rs26Schema+"\x00"), ao23JCS(t, payload)...))
	packet := ao23Map{"family": "tsfg.array-facts-package-candidate", "version": 4, "schemaHash": rs26Schema, "requestId": requestID, "packageId": packageID, "payload": payload}
	bundle["packageText"] = string(append(ao23JCS(t, packet), '\n'))
	bundle["metrics"].(ao23Map)["package"] = ao23Metrics(t, packet)
	return ao23JCS(t, bundle)
}
func TestReadSpan26aAdjacentIndependentOfficial(t *testing.T) {
	raw, err := os.ReadFile("testdata/readspan-last-use-v4/adjacent-inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	ao23Want(t, len(raw) == 38491 && ao23Hash(raw) == "257444d0ab02c10a6db9569a88cc0c323b6172e305cc219d5fc46f7311098c1d", "pre-production complete18 adjacent inputs")
	var corpus struct {
		Family, ContractSHA256 string
		Version                int
		Cases                  []rsAdj26Case
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	ao23Want(t, corpus.Family == "tsfg.readspan-adjacent-independent-inputs" && corpus.Version == 1 && corpus.ContractSHA256 == rs26FrozenPin && len(corpus.Cases) == 18, "complete approved adjacent corpus")
	dts, err := os.ReadFile("testdata/array-facts-v4/minimal-arrays-v3.d.ts")
	if err != nil {
		t.Fatal(err)
	}
	ao23Want(t, len(dts) == 484 && ao23Hash(dts) == "282b335a6e0e0493fa74a3c1dd56f234f8c679079acdaf0600cefb8306c26415", "exact pinned readonly generic declarations")
	schema, err := os.ReadFile("testdata/array-facts-v4/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	ao23Want(t, len(schema) == 33428 && "sha256:"+ao23Hash(schema) == rs26Schema, "immutable facts4 descriptor")
	total := 0
	for _, item := range corpus.Cases {
		t.Run(item.Name, func(t *testing.T) {
			bundle := rsAdj26Packet(t, item, string(dts))
			encoded := base64.StdEncoding.EncodeToString(bundle)
			total += len(encoded)
			ao23Want(t, total < 3*1024*1024, "complete adjacent output below raw observer cap")
			t.Logf("TSFG_READSPAN_26A_ADJACENT_INDEPENDENT_PACKET name=%s bytes=%d sha256=%s base64=%s", item.Name, len(bundle), ao23Hash(bundle), encoded)
		})
	}
}
