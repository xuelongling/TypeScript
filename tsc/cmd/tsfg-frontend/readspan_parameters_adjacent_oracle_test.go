// SPDX-License-Identifier: Apache-2.0
// Complete16 official-only adjacent bundles are frozen before any new producer output.
package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

const rs26bAdjacentInputPin = "4ce5bddf8bede5e800798733876ce3d38ef9479e1b794b3ec027045dc9a66883"
const rs26bAdjacentLiteralPin = "3931979872d7a69742632d4b61265bb872857fb581b35ba93efc5189aa8a291e"

func rs26bAdjacentBundle(t *testing.T, item rs26bAdjacentCase, dts string) []byte {
	ao23Want(t, len(item.Text) == item.Bytes && ao23Units(item.Text) == item.UTF16Length && ao23Hash([]byte(item.Text)) == item.SHA256 && item.Bytes <= 261 && item.SourcePath == item.SourceRoot+"/app.ts" && item.DeclarationPath == item.SourceRoot+"/minimal-arrays-v3.d.ts", "exact complete original small adjacent source identity")
	s, release := rs26bAdjacentProgram(t, item, dts)
	defer release()
	types, arrayLength, viewLength, intrinsic := s.origins()
	s.rs26bAST()
	s.rs26bAdjacentAnchors(item)
	files, owned := []any{}, []any{}
	for i, text := range []string{item.Text, dts} {
		path := []string{item.SourcePath, item.DeclarationPath}[i]
		row := ao23Map{"path": path, "content": text, "sha256": ao23Hash([]byte(text))}
		files = append(files, row)
		owned = append(owned, ao23Map{"path": path, "content": text, "sha256": row["sha256"], "byteLength": len(text), "utf16Length": ao23Units(text)})
	}
	config := ao23Map{"strict": true, "noEmit": true, "allowJs": false}
	declarations := ao23Map{"version": "minimal-arrays-v3", "paths": []string{item.DeclarationPath}}
	request := ao23Map{"family": "tsfg.array-facts-request-candidate", "version": 5, "input": ao23Map{"sourceRoot": item.SourceRoot, "rootFile": item.SourcePath, "files": files, "config": config, "declarations": declarations}}
	requestBytes := append(ao23JCS(t, request), '\n')
	bundle := ao23Map{"name": item.ID, "contractSHA256": rs26bContractPin, "factsSchemaHash": rs26bSchema, "inputSHA256": rs26bAdjacentInputPin, "literalSHA256": rs26bAdjacentLiteralPin, "requestText": string(requestBytes), "sourceExpectation": item.SourceExpected.Outcome, "official": s.proof, "metrics": ao23Map{"request": ao23Metrics(t, request)}}
	if item.SourceExpected.Outcome == "capability" {
		bundle["rawRefusalWitness"] = s.rs26bAdjacentNegative(item)
		s.proof["checks"] = []string{"fresh complete4diagnostics0", "complete original closure/AST/Symbol declarations/14DTS/readonly/generic origins", "actual full signature slots and optional minimum arity", "real Array/View assignability and exact annotation origin", "no legal selected facts packet constructed", "no Source/Product producer invoked"}
		s.proof["counters"] = s.counts
		return ao23JCS(t, bundle)
	}
	ao23Want(t, item.SourceExpected.Outcome == "accept-complete-facts", "only independently designed twelve positive inputs")
	headers := s.rs26bHeaders()
	functions := s.rs26bAdjacentFunctions(headers)
	ao23Want(t, len(functions) > 0 && len(functions) <= 16, "whole bounded original function table")
	payload := ao23Map{"sourceRoot": item.SourceRoot, "rootFile": item.SourcePath, "positionUnit": "utf16", "config": config, "declarations": declarations, "files": owned, "types": types, "arrayLength": arrayLength, "readSpanLength": viewLength, "readSpanIntrinsic": intrinsic, "functions": functions, "exports": s.actualExports(headers), "diagnostics": []any{}}
	requestID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.request.v5\x00"), requestBytes...))
	packageID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.package.v5\x00"+rs26bSchema+"\x00"), ao23JCS(t, payload)...))
	packet := ao23Map{"family": "tsfg.array-facts-package-candidate", "version": 5, "schemaHash": rs26bSchema, "requestId": requestID, "packageId": packageID, "payload": payload}
	bundle["packageText"] = string(append(ao23JCS(t, packet), '\n'))
	bundle["metrics"].(ao23Map)["package"] = ao23Metrics(t, packet)
	s.proof["counters"] = s.counts
	s.proof["checks"] = []string{"fresh complete4diagnostics0", "complete original closure/AST/Symbol declarations/14DTS/readonly/generic origins", "actual nominal view parameter full slots and every reference", "real resolved direct calls/required arity/ordered arguments/assignability", "whole body and complete direct DAG", "alias/read/length/checked roles and independent UTF16 anchors", "whole literal expected request/facts5/JSON metrics/counters comparison", "no Source/Product producer invoked"}
	return ao23JCS(t, bundle)
}

func TestReadSpan26bAdjacentIndependentOfficial(t *testing.T) {
	read := func(name string, bytes int, pin string) []byte {
		raw, err := os.ReadFile("testdata/readspan-parameters-v5/" + name)
		if err != nil {
			t.Fatal(err)
		}
		ao23Want(t, len(raw) == bytes && ao23Hash(raw) == pin, "exact frozen adjacent input "+name)
		return raw
	}
	raw := read("adjacent-inputs.json", 92072, rs26bAdjacentInputPin)
	literal := read("adjacent-expected-facts.json", 153273, rs26bAdjacentLiteralPin)
	dts := read("minimal-arrays-v3.d.ts", 484, "282b335a6e0e0493fa74a3c1dd56f234f8c679079acdaf0600cefb8306c26415")
	read("schema.json", 35522, rs26bSchema[7:])
	var corpus struct{ Cases []rs26bAdjacentCase }
	var expected struct {
		ContractSHA256, FactsSchemaHash string
		Cases                           []struct {
			Name, RequestText, PackageText string
			Counters                       map[string]int
			Metrics, ExpectedSourceError   ao23Map
		}
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(literal, &expected); err != nil {
		t.Fatal(err)
	}
	ao23Want(t, len(corpus.Cases) == 16 && len(expected.Cases) == 16 && expected.ContractSHA256 == rs26bContractPin && expected.FactsSchemaHash == rs26bSchema, "complete16 frozen R2 adjacent cases and exact inner facts5 pin")
	positive, negative, total := 0, 0, 0
	for i, item := range corpus.Cases {
		want := expected.Cases[i]
		ao23Want(t, want.Name == item.ID, "complete original case order")
		if item.SourceExpected.Outcome == "capability" {
			negative++
		} else {
			positive++
		}
		t.Run(item.ID, func(t *testing.T) {
			bundle := rs26bAdjacentBundle(t, item, string(dts))
			var got struct {
				RequestText, PackageText   string
				Official                   struct{ Counters map[string]int }
				Metrics, RawRefusalWitness ao23Map
			}
			if err := json.Unmarshal(bundle, &got); err != nil {
				t.Fatal(err)
			}
			ao23Want(t, got.RequestText == want.RequestText && got.PackageText == want.PackageText && string(ao23JCS(t, got.Metrics)) == string(ao23JCS(t, want.Metrics)), "entire original adjacent request/package/JSON geometry equals preproduction literal expectations")
			for key, value := range want.Counters {
				ao23Want(t, got.Official.Counters[key] == value, "complete original adjacent inventory "+key)
			}
			prefix := "TSFG_READSPAN_26B_ADJACENT_INDEPENDENT_PACKET"
			if item.SourceExpected.Outcome == "capability" {
				ao23Want(t, got.PackageText == "" && got.RawRefusalWitness["selectedPacketConstructed"] == false && got.RawRefusalWitness["actualSourceRefusalObserved"] == false && string(ao23JCS(t, got.RawRefusalWitness["expectedSourceError"])) == string(ao23JCS(t, want.ExpectedSourceError)), "raw independent official refusal witness; no fabricated legal packet or observed Source refusal")
				prefix = "TSFG_READSPAN_26B_ADJACENT_INDEPENDENT_REFUSAL"
			}
			encoded := base64.StdEncoding.EncodeToString(bundle)
			total += len(encoded)
			ao23Want(t, total < 3*1024*1024, "all complete adjacent official bundles below observer output bound")
			t.Logf("%s name=%s bytes=%d sha256=%s base64=%s", prefix, item.ID, len(bundle), ao23Hash(bundle), encoded)
		})
	}
	ao23Want(t, positive == 12 && negative == 4, "complete12 facts-positive and4 official-valid private-invalid controls")
}
