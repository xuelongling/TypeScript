// SPDX-License-Identifier: Apache-2.0
// Whole packets come from fresh official Program objects, never producer output.
package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

const ao23ContractPin = "f58dcc0f7e75c5ae739f7d2455815799f18e2c7a582d8aa7138276d862a246de"
const ao23FactsHash = "sha256:69233ab57ae3d991c5806ab282b73c526632d75bb0052ff78049092a45da40e8"

func ao23Input(t *testing.T, source, dts string) ao23Map {
	files := []any{}
	for i, text := range []string{source, dts} {
		files = append(files, ao23Map{"path": []string{"/array-owner23e/app.ts", "/array-owner23e/minimal-arrays-v2.d.ts"}[i], "content": text, "sha256": ao23Hash([]byte(text))})
	}
	return ao23Map{"sourceRoot": "/array-owner23e", "rootFile": "/array-owner23e/app.ts", "config": ao23Map{"strict": true, "noEmit": true, "allowJs": false}, "declarations": ao23Map{"version": "minimal-arrays-v2", "paths": []string{"/array-owner23e/minimal-arrays-v2.d.ts"}}, "files": files}
}
func ao23Metrics(t *testing.T, value any) ao23Map {
	raw := ao23JCS(t, value)
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	tokens, depth, strings := 0, 0, 0
	var walk func(any, int)
	walk = func(v any, d int) {
		tokens++
		if d > depth {
			depth = d
		}
		switch x := v.(type) {
		case map[string]any:
			for k, item := range x {
				tokens++
				if len(k) > strings {
					strings = len(k)
				}
				walk(item, d+1)
			}
		case []any:
			for _, item := range x {
				walk(item, d+1)
			}
		case string:
			if len(x) > strings {
				strings = len(x)
			}
		}
	}
	walk(decoded, 1)
	ao23Want(t, len(raw)+1 <= 2097152 && tokens <= 200000 && depth <= 64 && strings <= 1048576, "independent complete JSON budget, no unrelated front-door shadow")
	return ao23Map{"bytes": len(raw) + 1, "tokens": tokens, "depth": depth, "stringBytes": strings}
}
func ao23Packet(t *testing.T, item ao23Map, dts string) []byte {
	source := item["source"].(string)
	input := ao23Input(t, source, dts)
	request := ao23Map{"family": "tsfg.array-facts-request-candidate", "version": 3, "input": input}
	requestBytes := append(ao23JCS(t, request), '\n')
	requestID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.request.v3\x00"), requestBytes...))
	s, release := ao23Program(t, source, dts)
	defer release()
	types, property := s.pinnedTypes()
	headers := s.collectHeaders()
	functions := []any{}
	for _, h := range headers {
		s.buildFunction(h)
		functions = append(functions, h.row)
	}
	s.proveDAG(headers)
	exports := s.actualExports(headers)
	ao23Want(t, len(headers) <= 16, "bounded complete original functions")
	owned := []any{}
	for _, file := range input["files"].([]any) {
		v := file.(ao23Map)
		text := v["content"].(string)
		ao23Want(t, len(text) <= 65536 && ao23Units(text) <= 65536, "bounded original source")
		owned = append(owned, ao23Map{"path": v["path"], "content": text, "sha256": v["sha256"], "byteLength": len(text), "utf16Length": ao23Units(text)})
	}
	payload := ao23Map{"sourceRoot": input["sourceRoot"], "rootFile": input["rootFile"], "positionUnit": "utf16", "config": input["config"], "declarations": input["declarations"], "files": owned, "types": types, "arrayLength": property, "functions": functions, "exports": exports, "diagnostics": []any{}}
	packageID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.package.v3\x00"+ao23FactsHash+"\x00"), ao23JCS(t, payload)...))
	packet := ao23Map{"family": "tsfg.array-facts-package-candidate", "version": 3, "schemaHash": ao23FactsHash, "requestId": requestID, "packageId": packageID, "payload": payload}
	packetBytes := append(ao23JCS(t, packet), '\n')
	metrics := ao23Map{"request": ao23Metrics(t, request), "package": ao23Metrics(t, packet)}
	official := ao23Map{"diagnostics": ao23Map{"syntax": 0, "semantic": 0, "program": 0, "global": 0}, "consumedPaths": []string{s.file.FileName(), s.dts.FileName()}, "counters": s.counts, "checks": []string{"complete original AST", "original pinned type symbols/indexer/property", "original signature/argument assignability", "same-owner alias references", "all exports and private subset", "UTF16 original PositionMap plus independent rune count", "complete acyclic call graph"}}
	bundle := ao23Map{"name": item["name"], "requestText": string(requestBytes), "packageText": string(packetBytes), "contractSHA256": ao23ContractPin, "official": official, "metrics": metrics}
	if item["variantOf"] != nil {
		bundle["variantOf"] = item["variantOf"]
	}
	return ao23JCS(t, bundle)
}
func ao23Run(t *testing.T, group string) {
	raw, err := os.ReadFile("testdata/array-owner-oracle/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus ao23Map
	if err = json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	ao23Want(t, corpus["family"] == "tsfg.array-owner-independent-inputs" && corpus["contractSHA256"] == ao23ContractPin && corpus["factsSchemaHash"] == ao23FactsHash, "frozen independent input contract")
	dts, err := os.ReadFile("testdata/array-owner-oracle/minimal-arrays-v2.d.ts")
	if err != nil {
		t.Fatal(err)
	}
	ao23Want(t, len(dts) == 331 && ao23Hash(dts) == "6420493372913f019f64b094e181d1346afcca79e720ea85dc5e5026e03ba01e", "exact independently pinned DTS bytes")
	count := 0
	total := 0
	for _, v := range corpus["cases"].([]any) {
		item := v.(map[string]any)
		if item["group"] != group {
			continue
		}
		count++
		t.Run(item["name"].(string), func(t *testing.T) {
			bundle := ao23Packet(t, item, string(dts))
			encoded := base64.StdEncoding.EncodeToString(bundle)
			total += len(encoded)
			ao23Want(t, total < 7*1024*1024, "complete group base64 remains below observer stdout cap")
			t.Logf("TSFG_ARRAY_OWNER_23E_INDEPENDENT_PACKET name=%s bytes=%d sha256=%s base64=%s", item["name"], len(bundle), ao23Hash(bundle), encoded)
		})
	}
	want := 8
	if group == "work" {
		want = 2
	}
	ao23Want(t, count == want, "complete requested independent corpus group")
}
func TestArrayOwner23eIndependentFunctionalOracle(t *testing.T) { ao23Run(t, "functional") }
func TestArrayOwner23eIndependentWorkOracle(t *testing.T)       { ao23Run(t, "work") }
