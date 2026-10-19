// SPDX-License-Identifier: Apache-2.0
// v4 envelopes are reconstructed from official AST; complete old bodies are immutable controls.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type rsOld26Case struct {
	Name, Group, LegacyRequestText, LegacyPackageText, LegacyRequestSHA256, LegacyPackageSHA256 string
	SourceRangeRoles                                                                            int
}

func rsOld26Packet(t *testing.T, item rsOld26Case, dts string) []byte {
	ao23Want(t, ao23Hash([]byte(item.LegacyRequestText)) == item.LegacyRequestSHA256 && ao23Hash([]byte(item.LegacyPackageText)) == item.LegacyPackageSHA256, "frozen exact whole original request and packet")
	var oldRequest, oldPacket ao23Map
	if err := json.Unmarshal([]byte(item.LegacyRequestText), &oldRequest); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(item.LegacyPackageText), &oldPacket); err != nil {
		t.Fatal(err)
	}
	ao23Want(t, oldRequest["version"] == float64(3) && oldPacket["version"] == float64(3), "only real old private3 inputs")
	old := oldPacket["payload"].(map[string]any)
	oldInput := oldRequest["input"].(map[string]any)
	root, sourcePath := old["sourceRoot"].(string), old["rootFile"].(string)
	oldFiles := oldInput["files"].([]any)
	ao23Want(t, len(oldFiles) == 2, "whole original two-file input")
	sourceFile, oldDTS := oldFiles[0].(map[string]any), oldFiles[1].(map[string]any)
	source, oldDTSPath := sourceFile["content"].(string), oldDTS["path"].(string)
	ao23Want(t, sourceFile["path"] == sourcePath && strings.HasSuffix(oldDTSPath, "/minimal-arrays-v2.d.ts") && oldDTS["content"] == dts[:331], "unchanged Source and exact old DTS prefix")
	ao23Want(t, len(source) < 12000 && ao23Units(source) < 12000, "bounded fixed18 originals; never long/provider input")
	dtsPath := strings.TrimSuffix(oldDTSPath, "minimal-arrays-v2.d.ts") + "minimal-arrays-v3.d.ts"
	s, release := rsOld26Program(t, root, sourcePath, dtsPath, source, dts)
	defer release()
	types, property, viewLength, intrinsic := s.origins()
	headers := s.collectHeaders()
	functions := []any{}
	for _, h := range headers {
		s.buildFunction(h)
		functions = append(functions, h.row)
	}
	s.proveDAG(headers)
	exports := s.actualExports(headers)
	oldProperty := old["arrayLength"].(map[string]any)["id"].(string)
	expectedFunctions := rsOld26Property(old["functions"], oldProperty, property["id"].(string))
	ao23Want(t, bytes.Equal(ao23JCS(t, functions), ao23JCS(t, expectedFunctions)), "every original function/signature/BindingId/local/site/statement/checked obligation and UTF16 role preserved; only DTS property references reidentified")
	ao23Want(t, bytes.Equal(ao23JCS(t, exports), ao23JCS(t, old["exports"])), "unchanged full original exports")
	ao23Want(t, rsOld26RangeRoles(functions) == item.SourceRangeRoles, "full original source range role inventory retained")
	for i, v := range old["types"].([]any) {
		row := v.(map[string]any)
		row["sourcePath"] = dtsPath
		ao23Want(t, bytes.Equal(ao23JCS(t, row), ao23JCS(t, types[i])), "unchanged first331B nominal DTS origins with actual new source path")
	}
	if item.Name == "first-touch-alias" {
		ao23Want(t, len(source) == 273 && ao23Units(source) == 271 && item.SourceRangeRoles == 51 && ao23Hash([]byte(source)) == "9312296861b0ea9fc2e569020a5c9b944c73da022f9b87d26743247d4baeb709", "complete nonBMP/CRLF273B271UTF16 and all51 original roles")
	}
	files, owned := []any{}, []any{}
	for i, text := range []string{source, dts} {
		path := []string{sourcePath, dtsPath}[i]
		sha := ao23Hash([]byte(text))
		files = append(files, ao23Map{"path": path, "content": text, "sha256": sha})
		owned = append(owned, ao23Map{"path": path, "content": text, "sha256": sha, "byteLength": len(text), "utf16Length": ao23Units(text)})
	}
	declarations := ao23Map{"version": "minimal-arrays-v3", "paths": []string{dtsPath}}
	input := ao23Map{"sourceRoot": root, "rootFile": sourcePath, "config": oldInput["config"], "declarations": declarations, "files": files}
	request := ao23Map{"family": "tsfg.array-facts-request-candidate", "version": 4, "input": input}
	requestBytes := append(ao23JCS(t, request), '\n')
	payload := ao23Map{"sourceRoot": root, "rootFile": sourcePath, "positionUnit": "utf16", "config": old["config"], "declarations": declarations, "files": owned, "types": types, "arrayLength": property, "readSpanLength": viewLength, "readSpanIntrinsic": intrinsic, "functions": functions, "exports": exports, "diagnostics": []any{}}
	requestID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.request.v4\x00"), requestBytes...))
	packageID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.package.v4\x00"+rs26Schema+"\x00"), ao23JCS(t, payload)...))
	packet := ao23Map{"family": "tsfg.array-facts-package-candidate", "version": 4, "schemaHash": rs26Schema, "requestId": requestID, "packageId": packageID, "payload": payload}
	packetBytes := append(ao23JCS(t, packet), '\n')
	s.proof["counters"] = s.counts
	s.proof["sourceRangeRoles"] = item.SourceRangeRoles
	s.proof["checks"] = []string{"fresh all4diagnostics0", "complete original source+new484BDTS closure", "all14 original DTS declarations and exact readonly/generic symbols", "full function/header/argument/resolvedsignature/alias/UTF16 roles", "whole original body equal except new DTS Array.length identity", "no view syntax and no liveness plan", "no producer projection invoked"}
	return ao23JCS(t, ao23Map{"name": item.Name, "group": item.Group, "contractSHA256": rs26FrozenPin, "legacy": ao23Map{"requestSHA256": item.LegacyRequestSHA256, "packageSHA256": item.LegacyPackageSHA256}, "requestText": string(requestBytes), "packageText": string(packetBytes), "official": s.proof, "metrics": ao23Map{"request": ao23Metrics(t, request), "package": ao23Metrics(t, packet)}})
}
func TestReadSpan26aOld18IndependentOfficial(t *testing.T) {
	raw, err := os.ReadFile("testdata/array-facts-v4/old18-inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	ao23Want(t, len(raw) == 1109854 && ao23Hash(raw) == "f9ec4507ec6420f2a3eb4167c72a87fde31245f9788d9a63386b2e53bca8e135", "frozen fixed18 complete original inputs")
	var corpus struct {
		Family, ContractSHA256 string
		Version                int
		Cases                  []rsOld26Case
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	ao23Want(t, corpus.Family == "tsfg.readspan-old18-independent-inputs" && corpus.Version == 1 && corpus.ContractSHA256 == rs26FrozenPin && len(corpus.Cases) == 18, "exact fixed18 source corpus")
	dts, err := os.ReadFile("testdata/array-facts-v4/minimal-arrays-v3.d.ts")
	if err != nil {
		t.Fatal(err)
	}
	ao23Want(t, len(dts) == 484 && ao23Hash(dts) == "282b335a6e0e0493fa74a3c1dd56f234f8c679079acdaf0600cefb8306c26415", "exact new pinned484B declarations")
	schema, err := os.ReadFile("testdata/array-facts-v4/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	ao23Want(t, len(schema) == 33428 && "sha256:"+ao23Hash(schema) == rs26Schema, "immutable exact facts4 descriptor")
	total, old8, owner10 := 0, 0, 0
	for _, item := range corpus.Cases {
		if item.Group == "old8" {
			old8++
		} else if item.Group == "owner10" {
			owner10++
		} else {
			t.Fatal("unlisted original group")
		}
		t.Run(item.Group+"/"+item.Name, func(t *testing.T) {
			bundle := rsOld26Packet(t, item, string(dts))
			encoded := base64.StdEncoding.EncodeToString(bundle)
			total += len(encoded)
			ao23Want(t, total < 3*1024*1024, "complete fixed18 raw output remains below observer cap")
			t.Logf("TSFG_READSPAN_26A_OLD18_INDEPENDENT_PACKET name=%s group=%s bytes=%d sha256=%s base64=%s", item.Name, item.Group, len(bundle), ao23Hash(bundle), encoded)
		})
	}
	ao23Want(t, old8 == 8 && owner10 == 10, "all original18 programs, no skipped case")
}
