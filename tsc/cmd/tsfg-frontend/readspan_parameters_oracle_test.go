// SPDX-License-Identifier: Apache-2.0
// Full facts7 expectations are authored from fixed literals before any Source producer output.
package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

const rs26bContractPin = "e9887966e68cd5dcbb042095f344f4a63d41b6070835ba37ccd5057758358e4d"
const rs26bSchema = "sha256:3067c8bb8296ab6687ee6549dd44522cae17f4b87b0a3996616ba7b373952617"
const rs26bInputPin = "5c079481047a2b7437886994a940694e84e86592577fa256fd2b660526de07d7"
const rs26bExpectedPin = "189db5e75e093a7ebf5ea3060349c82b2b52807343ec07419f3b44217d53bee1"

func rs26bPacket(t *testing.T, name, source, dts string) []byte {
	s, release := rs26bProgram(t, source, dts)
	defer release()
	types, arrayLength, viewLength, intrinsic := s.origins()
	s.rs26bAST()
	headers := s.rs26bHeaders()
	functions := s.rs26bFunctions(headers)
	exports := s.actualExports(headers)
	files, owned := []any{}, []any{}
	for i, text := range []string{source, dts} {
		path := []string{s.file.FileName(), s.dts.FileName()}[i]
		row := ao23Map{"path": path, "content": text, "sha256": ao23Hash([]byte(text))}
		files = append(files, row)
		owned = append(owned, ao23Map{"path": path, "content": text, "sha256": row["sha256"], "byteLength": len(text), "utf16Length": ao23Units(text)})
	}
	config := ao23Map{"strict": true, "noEmit": true, "allowJs": false}
	declarations := ao23Map{"version": "minimal-arrays-v3", "paths": []string{s.dts.FileName()}}
	input := ao23Map{"sourceRoot": "/readspan26b", "rootFile": s.file.FileName(), "files": files, "config": config, "declarations": declarations}
	request := ao23Map{"family": "tsfg.array-facts-request-candidate", "version": 7, "input": input}
	requestBytes := append(ao23JCS(t, request), '\n')
	payload := ao23Map{"sourceRoot": input["sourceRoot"], "rootFile": input["rootFile"], "positionUnit": "utf16", "config": config, "declarations": declarations, "files": owned, "types": types, "arrayLength": arrayLength, "readSpanLength": viewLength, "readSpanIntrinsic": intrinsic, "functions": functions, "exports": exports, "diagnostics": []any{}}
	requestID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.request.v7\x00"), requestBytes...))
	packageID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.package.v7\x00"+rs26bSchema+"\x00"), ao23JCS(t, payload)...))
	packet := ao23Map{"family": "tsfg.array-facts-package-candidate", "version": 7, "schemaHash": rs26bSchema, "requestId": requestID, "packageId": packageID, "payload": payload}
	packetBytes := append(ao23JCS(t, packet), '\n')
	entry := functions[2].(ao23Map)["body"].([]any)
	mutateIndex, ignoreIndex := 2, 3
	mutateRange, ignoreRange := ao23Range{232, 241}, ao23Range{245, 254}
	viewArgument := ao23Range{252, 253}
	if name == "after" {
		mutateIndex, ignoreIndex, mutateRange, ignoreRange, viewArgument = 3, 2, ignoreRange, mutateRange, ao23Range{239, 240}
	}
	create := entry[1].(ao23Map)["initializer"].(ao23Map)
	mutate := entry[mutateIndex].(ao23Map)["value"].(ao23Map)
	ignore := entry[ignoreIndex].(ao23Map)["value"].(ao23Map)
	viewParam := functions[0].(ao23Map)["parameters"].([]ao23Map)[0]
	ao23Want(t, viewParam["declaredType"] == "readspan.f64" && viewParam["writable"] == false && viewParam["declarationRange"] == (ao23Range{16, 32}) && viewParam["annotationRange"] == (ao23Range{19, 32}) && viewParam["elementAnnotationRange"] == (ao23Range{28, 31}), "actual original view parameter signature and all literal anchors")
	ao23Want(t, create["range"] == (ao23Range{211, 228}) && mutate["range"] == mutateRange && ignore["range"] == ignoreRange && ignore["arguments"].([]any)[0].(ao23Map)["range"] == viewArgument, "fresh actual AST/PositionMap agrees with independent272B token roles")
	ao23Want(t, s.counts["functions"] == 3 && s.counts["parameters"] == 2 && s.counts["viewParameters"] == 1 && s.counts["calls"] == 2 && s.counts["allocations"] == 1 && s.counts["viewCreates"] == 1 && s.counts["viewAliases"] == 0 && s.counts["viewReads"] == 0 && s.counts["viewReferences"] == 1 && s.counts["instantiatedParameterDeclarationJoins"] == 3 && s.counts["statements"] == 8, "complete all-function/parameter/body/call inventory without liveness projection")
	s.proof["firstAnchors"] = ao23Map{"viewParameter": viewParam["declarationRange"], "viewParameterAnnotation": viewParam["annotationRange"], "viewParameterElement": viewParam["elementAnnotationRange"], "create": create["range"], "mutationCall": mutate["range"], "ignoredViewCall": ignore["range"], "viewArgument": viewArgument}
	s.proof["counters"] = s.counts
	s.proof["checks"] = []string{"fresh complete4diagnostics0", "exact original two-file closure", "whole original AST and Symbol declaration inventory", "all14 original DTS declarations", "actual readonly index/length and separate scoped generic T origins", "actual parameter annotation/type/Symbol/full signature slot", "resolved original direct calls and official assignability", "registered nominal view argument provenance", "all original UTF16 spans", "complete direct-call DAG", "whole literal expected request and facts comparison"}
	requestMetrics, packetMetrics := ao23Metrics(t, request), ao23Metrics(t, packet)
	ao23Want(t, string(ao23JCS(t, requestMetrics)) == string(ao23JCS(t, ao23Map{"bytes": 1332, "tokens": 42, "depth": 5, "stringBytes": 484})) && string(ao23JCS(t, packetMetrics)) == string(ao23JCS(t, ao23Map{"bytes": 9593, "tokens": 941, "depth": 11, "stringBytes": 484})), "complete request/facts JSON metrics agree with independent literal geometry")
	return ao23JCS(t, ao23Map{"name": name, "contractSHA256": rs26bContractPin, "factsSchemaHash": rs26bSchema, "sourceFixtureSHA256": rs26bInputPin, "expectationFixtureSHA256": rs26bExpectedPin, "requestText": string(requestBytes), "packageText": string(packetBytes), "official": s.proof, "metrics": ao23Map{"request": requestMetrics, "package": packetMetrics}})
}

func TestReadSpan26bIndependentFirstOfficial(t *testing.T) {
	read := func(name string, bytes int, pin string) []byte {
		raw, err := os.ReadFile("testdata/readspan-parameters-v7/" + name)
		if err != nil {
			t.Fatal(err)
		}
		ao23Want(t, len(raw) == bytes && ao23Hash(raw) == pin, "exact independently authored fixture "+name)
		return raw
	}
	input := read("first-source-texts.json", 1019, rs26bInputPin)
	dts := read("minimal-arrays-v3.d.ts", 484, "282b335a6e0e0493fa74a3c1dd56f234f8c679079acdaf0600cefb8306c26415")
	read("schema.json", 39596, rs26bSchema[7:])
	expected := read("first-expected-facts.json", 43793, rs26bExpectedPin)
	var first struct {
		SourceRoot, SourcePath, DeclarationPath string
		Before, After                           struct {
			Content, SHA256    string
			Bytes, UTF16Length int
		}
	}
	var want struct {
		Cases []struct{ Name, RequestText, PackageText string }
	}
	if err := json.Unmarshal(input, &first); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(expected, &want); err != nil {
		t.Fatal(err)
	}
	ao23Want(t, first.SourceRoot == "/readspan26b" && first.SourcePath == "/readspan26b/app.ts" && first.DeclarationPath == "/readspan26b/minimal-arrays-v3.d.ts" && len(want.Cases) == 2, "fixed full first-pair input identities")
	for i, name := range []string{"before", "after"} {
		t.Run(name, func(t *testing.T) {
			item := first.Before
			if name == "after" {
				item = first.After
			}
			ao23Want(t, item.Bytes == 272 && item.UTF16Length == 272 && len(item.Content) == 272 && ao23Units(item.Content) == 272 && ao23Hash([]byte(item.Content)) == item.SHA256, "exact full original272B text")
			raw := rs26bPacket(t, name, item.Content, string(dts))
			var bundle struct{ RequestText, PackageText string }
			if err := json.Unmarshal(raw, &bundle); err != nil {
				t.Fatal(err)
			}
			ao23Want(t, want.Cases[i].Name == name && bundle.RequestText == want.Cases[i].RequestText && bundle.PackageText == want.Cases[i].PackageText, "entire official-only request/facts7 equals independently authored literal expectations")
			t.Logf("TSFG_READSPAN_26B_INDEPENDENT_PACKET name=%s bytes=%d sha256=%s base64=%s", name, len(raw), ao23Hash(raw), base64.StdEncoding.EncodeToString(raw))
		})
	}
}
