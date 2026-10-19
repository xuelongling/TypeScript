// SPDX-License-Identifier: Apache-2.0
// Prior Source syntax is rechecked under the adopted descriptor before Product work.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

type readSpan26dPriorCase struct {
	Name, Source, SourceSHA256, RequestText, PackageText, PackageSHA256 string
	SourceBytes, PackageBytes                                           int
	Request, Package                                                    json.RawMessage
}

func TestReadSpan26dV7SourcePriorWholeFirstPair(t *testing.T) {
	const fixturePath = "testdata/readspan-array-returns-v7/first-prior-expected-facts.json"
	raw, err := os.ReadFile(fixturePath)
	ao23Want(t, err == nil && len(raw) == 79978 && ao23Hash(raw) == "e915076c409718b46050a63d2832b6956cdeb2085971362cc8460f4236b3bcda", "frozen independent adopted-descriptor whole fixture")
	before := bytes.Clone(raw)
	t.Cleanup(func() {
		after, err := os.ReadFile(fixturePath)
		if err != nil || !bytes.Equal(after, before) || !bytes.Equal(raw, before) {
			t.Errorf("independent prior fixture changed: %v", err)
		}
	})
	var fixture struct {
		Family  string
		Version int
		Schema  struct{ SchemaHash string }
		Cases   []readSpan26dPriorCase
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	ao23Want(t, fixture.Family == "tsfg.m26d-independent-new-source-whole" && fixture.Version == 1 && len(fixture.Cases) == 2 && fixture.Schema.SchemaHash == "sha256:d2ee930b76b8fd5c2270ad69579c9c61377831a97034d3a8890d8aa770ebabc2", "complete frozen current-descriptor facts7 family")
	pins := []struct{ name, sourceHash, requestHash, packageHash string }{
		{"before", "86caca97f916afef6a21e1c27676bf1dd494b56713bb68aad80fd1a83f04c356", "e4169d046bc72f39e378996cd951daae5d91b3946651a87e9cffc64be59b57ff", "9617a774699ebcb9b8c756eb692a2c5c2072d815a34f0db10457bfe5e81aa814"},
		{"after", "ec9b1c181370e473f23435dcb98810ff0a918df337aacbed4b64ebb59f160d99", "95e6dc02d17d80bafa34fbebc929e5df1aa5792f2be5414b63b65291d72e9968", "bd6742a7e2b0e2a8464ec63238eab5ab88ad5acc43d9d9b01d063f84fff1a838"},
	}
	for i, item := range fixture.Cases {
		pin := pins[i]
		ao23Want(t, item.Name == pin.name && item.SourceBytes == 294 && len(item.Source) == item.SourceBytes && ao23Hash([]byte(item.Source)) == pin.sourceHash && item.SourceSHA256 == pin.sourceHash, "complete original independent source")
		ao23Want(t, len(item.RequestText) == 1355 && ao23Hash([]byte(item.RequestText)) == pin.requestHash && item.PackageBytes == 10447 && len(item.PackageText) == item.PackageBytes && ao23Hash([]byte(item.PackageText)) == pin.packageHash && item.PackageSHA256 == pin.packageHash, "exact independent request and entire current-descriptor facts7 packet")
		for _, frame := range []struct {
			data json.RawMessage
			text string
		}{{item.Request, item.RequestText}, {item.Package, item.PackageText}} {
			var value any
			if err := json.Unmarshal(frame.data, &value); err != nil {
				t.Fatal(err)
			}
			ao23Want(t, bytes.Equal(append(ao23JCS(t, value), '\n'), []byte(frame.text)), "literal object and full canonical frame agree")
		}
	}
	// All fresh official witnesses must pass before any Source producer is called.
	for _, item := range fixture.Cases {
		if !t.Run("official/"+item.Name, func(t *testing.T) {
			witness := rs26cOfficialFixtureWitness(t, rs26cFrozenFixtureCase{
				Name: item.Name, Source: item.Source, SourcePath: "/readspan26d/app.ts",
				SourceSHA256: item.SourceSHA256, SourceBytes: item.SourceBytes,
				RequestText: item.RequestText, ExpectedText: item.PackageText,
				ExpectedBytes: item.PackageBytes, ExpectedSHA256: item.PackageSHA256,
			})
			// The reused witness checks all four fresh diagnostic groups, original
			// symbols, nominal declarations, complete signatures/full slots, DAG,
			// body/reference/return anchors and whole independent packet equality.
			data := ao23JCS(t, witness)
			t.Logf("TSFG_READSPAN_26D_PRIOR_OFFICIAL_WHOLE name=%s bytes=%d sha256=%s base64=%s", item.Name, len(data), ao23Hash(data), base64.StdEncoding.EncodeToString(data))
		}) {
			t.Fatal("official witness failed; Source publication not attempted")
		}
	}
	for _, item := range fixture.Cases {
		t.Run("public/"+item.Name, func(t *testing.T) {
			input := []byte(item.RequestText)
			inputBefore := bytes.Clone(input)
			var output readSpanParameterPublicWriter26b
			err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v7"}, bytes.NewReader(input), &output)
			t.Logf("TSFG_READSPAN_26D_PRIOR_PUBLIC_ACTUAL name=%s bytes=%d sha256=%s writes=%d base64=%s", item.Name, output.Len(), ao23Hash(output.Bytes()), output.writes, base64.StdEncoding.EncodeToString(output.Bytes()))
			ao23Want(t, bytes.Equal(input, inputBefore), "original public command request remains immutable")
			if err != nil {
				ao23Want(t, output.Len() == 0 && output.writes == 0, "failed command publishes no partial packet")
				t.Fatalf("existing Source capability must publish frozen current-descriptor facts7: %s", readSpanParameterPublicError26b(err))
			}
			ao23Want(t, output.writes == 1 && bytes.Equal(output.Bytes(), []byte(item.PackageText)), "one atomic complete command publication equals independent literal")
		})
	}
}
