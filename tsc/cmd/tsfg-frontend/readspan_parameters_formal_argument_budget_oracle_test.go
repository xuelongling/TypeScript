// SPDX-License-Identifier: Apache-2.0
// Official-only proof of original formal/repeated-argument geometry; no Product meter is called.
package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

const rs26bBudgetInputPin = "fc3746c9b29bb79dee77254673dbc79ba3fbfc24d6ccb87613e508211229f988"
const rs26bBudgetLiteralPin = "09c8cd7e3db0c7e7a9fb67be3323056092b895521057dd22f3c23b9a996d80c3"

type rs26bBudgetCase struct {
	ID, Text, SourceRoot, SourcePath, DeclarationPath, SHA256 string
	Bytes, UTF16Length                                        int
}

func rs26bBudgetOfficialBundle(t *testing.T, item rs26bBudgetCase, dts string) []byte {
	ao23Want(t, len(item.Text) == item.Bytes && ao23Units(item.Text) == item.UTF16Length && ao23Hash([]byte(item.Text)) == item.SHA256 && item.SourceRoot == "/readspan26b-budget" && item.SourcePath == "/readspan26b-budget/app.ts" && item.DeclarationPath == "/readspan26b-budget/minimal-arrays-v3.d.ts", "exact complete original budget source and shared paths")
	programItem := rs26bAdjacentCase{ID: item.ID, Text: item.Text, SourceRoot: item.SourceRoot, SourcePath: item.SourcePath, DeclarationPath: item.DeclarationPath, SHA256: item.SHA256, Bytes: item.Bytes, UTF16Length: item.UTF16Length}
	s, release := rs26bAdjacentProgram(t, programItem, dts)
	defer release()
	types, arrayLength, viewLength, intrinsic := s.origins()
	s.rs26bAST()
	headers := s.rs26bHeaders()
	ao23Want(t, len(headers) == 7, "all seven original budget functions")
	for i, name := range []string{"sink8", "sink1", "c0", "c1", "c2", "c3", "c4"} {
		ao23Want(t, headers[i].node.Name().Text() == name, "complete source order including unused c4 formal")
	}
	functions := s.rs26bAdjacentFunctions(headers)
	files, owned := []any{}, []any{}
	for i, text := range []string{item.Text, dts} {
		path := []string{item.SourcePath, item.DeclarationPath}[i]
		row := ao23Map{"path": path, "content": text, "sha256": ao23Hash([]byte(text))}
		files = append(files, row)
		owned = append(owned, ao23Map{"path": path, "content": text, "sha256": row["sha256"], "byteLength": len(text), "utf16Length": ao23Units(text)})
	}
	config := ao23Map{"strict": true, "noEmit": true, "allowJs": false}
	declarations := ao23Map{"version": "minimal-arrays-v3", "paths": []string{item.DeclarationPath}}
	request := ao23Map{"family": "tsfg.array-facts-request-candidate", "version": 6, "input": ao23Map{"sourceRoot": item.SourceRoot, "rootFile": item.SourcePath, "files": files, "config": config, "declarations": declarations}}
	requestBytes := append(ao23JCS(t, request), '\n')
	payload := ao23Map{"sourceRoot": item.SourceRoot, "rootFile": item.SourcePath, "positionUnit": "utf16", "config": config, "declarations": declarations, "files": owned, "types": types, "arrayLength": arrayLength, "readSpanLength": viewLength, "readSpanIntrinsic": intrinsic, "functions": functions, "exports": s.actualExports(headers), "diagnostics": []any{}}
	requestID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.request.v6\x00"), requestBytes...))
	packageID := "sha256:" + ao23Hash(append([]byte("tsfg.array-facts.package.v6\x00"+rs26bSchema+"\x00"), ao23JCS(t, payload)...))
	packet := ao23Map{"family": "tsfg.array-facts-package-candidate", "version": 6, "schemaHash": rs26bSchema, "requestId": requestID, "packageId": packageID, "payload": payload}
	s.proof["counters"] = s.counts
	s.proof["checks"] = []string{"fresh complete4diagnostics0 and exact original two-file closure", "complete original AST/Symbol declarations/14DTS/readonly/scoped generic origins", "all14 actual required formal Symbols at original full slots", "all128 actual resolved calls with complete original signatures and assignability", "every1010 or1011 nominal descriptor reference retained, repeated full slots never compacted", "whole source/body/direct DAG/UTF16 roles", "original function/body/local/expression/raw-call limits", "whole independently frozen request/facts6/JSON metrics and counters comparison", "no Source producer, Product module or Product fee consumer invoked"}
	return ao23JCS(t, ao23Map{"name": item.ID, "contractSHA256": rs26bContractPin, "factsSchemaHash": rs26bSchema, "inputSHA256": rs26bBudgetInputPin, "literalSHA256": rs26bBudgetLiteralPin, "requestText": string(requestBytes), "packageText": string(append(ao23JCS(t, packet), '\n')), "official": s.proof, "metrics": ao23Map{"request": ao23Metrics(t, request), "package": ao23Metrics(t, packet)}})
}

func TestReadSpan26bFormalArgumentBudgetIndependentOfficial(t *testing.T) {
	read := func(name string, bytes int, pin string) []byte {
		raw, err := os.ReadFile("testdata/readspan-parameters-v6/" + name)
		if err != nil {
			t.Fatal(err)
		}
		ao23Want(t, len(raw) == bytes && ao23Hash(raw) == pin, "exact preproduction formal/argument budget fixture "+name)
		return raw
	}
	input := read("formal-argument-budget-inputs.json", 24466, rs26bBudgetInputPin)
	literal := read("formal-argument-budget-expected-facts.json", 503470, rs26bBudgetLiteralPin)
	dts := read("minimal-arrays-v3.d.ts", 484, "282b335a6e0e0493fa74a3c1dd56f234f8c679079acdaf0600cefb8306c26415")
	read("schema.json", 38816, rs26bSchema[7:])
	var corpus struct {
		ContractSHA256, FactsSchemaHash string
		Cases                           []rs26bBudgetCase
	}
	var expected struct {
		ContractSHA256, FactsSchemaHash string
		Cases                           []struct {
			Name, RequestText, PackageText string
			Counters                       map[string]int
			FunctionInventory              []any
			Metrics                        ao23Map
		}
	}
	if err := json.Unmarshal(input, &corpus); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(literal, &expected); err != nil {
		t.Fatal(err)
	}
	ao23Want(t, len(corpus.Cases) == 2 && len(expected.Cases) == 2 && corpus.ContractSHA256 == rs26bContractPin && expected.ContractSHA256 == rs26bContractPin && corpus.FactsSchemaHash == rs26bSchema && expected.FactsSchemaHash == rs26bSchema && rs26bContractPin == "e9887966e68cd5dcbb042095f344f4a63d41b6070835ba37ccd5057758358e4d", "exact two new budget cases with full R2 inner identities")
	for i, item := range corpus.Cases {
		want := expected.Cases[i]
		ao23Want(t, want.Name == item.ID && item.ID == []string{"work1024", "work1025"}[i] && item.Bytes == []int{4700, 4712}[i], "frozen same-root witness order and source extent")
		t.Run(item.ID, func(t *testing.T) {
			raw := rs26bBudgetOfficialBundle(t, item, string(dts))
			var got struct {
				RequestText, PackageText string
				Official                 struct {
					Counters          map[string]int
					FunctionInventory []any
				}
				Metrics ao23Map
			}
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			ao23Want(t, got.RequestText == want.RequestText && got.PackageText == want.PackageText && string(ao23JCS(t, got.Metrics)) == string(ao23JCS(t, want.Metrics)) && string(ao23JCS(t, got.Official.FunctionInventory)) == string(ao23JCS(t, want.FunctionInventory)), "entire official-only request/facts/metrics/full function inventory equals independent literal expectations")
			for key, value := range want.Counters {
				ao23Want(t, got.Official.Counters[key] == value, "complete formal/argument witness counter "+key)
			}
			t.Logf("TSFG_READSPAN_26B_FORMAL_ARGUMENT_BUDGET_INDEPENDENT_PACKET name=%s bytes=%d sha256=%s base64=%s", item.ID, len(raw), ao23Hash(raw), base64.StdEncoding.EncodeToString(raw))
		})
	}
}
