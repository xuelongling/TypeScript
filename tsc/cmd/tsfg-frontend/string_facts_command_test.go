// SPDX-License-Identifier: Apache-2.0
// Complete oracle bytes predate this producer. Official values never call its converter.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"slices"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	tsjson "github.com/microsoft/typescript-go/internal/json"
	"github.com/microsoft/typescript-go/internal/scanner"
)

type stringOracleRange24 struct {
	Start int `json:"start"`
	End   int `json:"end"`
}
type stringOracleCase24 struct {
	Name              string              `json:"name"`
	DeclarationRange  stringOracleRange24 `json:"declarationRange"`
	ReturnRange       stringOracleRange24 `json:"returnRange"`
	LiteralRange      stringOracleRange24 `json:"literalRange"`
	DeclarationSource string              `json:"declarationSource"`
	ReturnSource      string              `json:"returnSource"`
	LiteralSource     string              `json:"literalSource"`
	CodeUnits         []int32             `json:"codeUnits"`
}

func stringOracleRead24(t *testing.T, name, hash string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/string-facts-v1/" + name)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != hash {
		t.Fatal("independent fixture changed: " + name)
	}
	return data
}
func stringOracleCanonical24(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	raw := tsjson.Value(data)
	if err := raw.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	return raw
}
func stringOracleHash24(domain string, data []byte) string {
	sum := sha256.Sum256(append([]byte(domain), data...))
	return "sha256:" + hex.EncodeToString(sum[:])
}
func stringOracleAnchor24(t *testing.T, file *ast.SourceFile, node *ast.Node, want stringOracleRange24, spelling string) {
	t.Helper()
	if node == nil {
		t.Fatal("official node missing")
	}
	text := file.Text()
	start, end := scanner.SkipTrivia(text, node.Pos()), node.End()
	if start < node.Pos() || start > end || end > len(text) || text[start:end] != spelling {
		t.Fatalf("official raw/trim spelling mismatch: %q", spelling)
	}
	actual := stringOracleRange24{len(utf16.Encode([]rune(text[:start]))), len(utf16.Encode([]rune(text[:end])))}
	positions := file.GetPositionMap()
	if actual != want || positions.UTF8ToUTF16(start) != want.Start || positions.UTF8ToUTF16(end) != want.End {
		t.Fatalf("official UTF16 anchor: got %+v want %+v", actual, want)
	}
}

func TestStringFactsCommandV1LosslessUTF16(t *testing.T) {
	input := stringOracleRead24(t, "request.json", "153b9e72ac0dec01815f05218cad4bc7377a46456b5030e0659e4223474fdec5")
	expected := stringOracleRead24(t, "expected.json", "64e164fc00016eda6052453993c0eab1de9cac0f4879de3b79fe42df4b38a468")
	schema := stringOracleRead24(t, "schema.json", "6dfed46326c15a81aeb6a1ea9b5ed2fc10db7bb1bf27733ac3df530d78d5cc62")
	manual := stringOracleRead24(t, "manual.json", "4cb416fc86a071c8bdca9a4467f6f924270b039a601f84096dd09738383b931e")
	var envelope struct {
		Input request `json:"input"`
	}
	var oracle struct {
		Cases               []stringOracleCase24 `json:"cases"`
		TotalValueCodeUnits int                  `json:"totalValueCodeUnits"`
	}
	var want map[string]any
	if json.Unmarshal(input, &envelope) != nil || json.Unmarshal(manual, &oracle) != nil || json.Unmarshal(expected, &want) != nil {
		t.Fatal("independent fixture decode")
	}
	r := envelope.Input
	if len(oracle.Cases) != 14 || oracle.TotalValueCodeUnits != 29 || len(r.Files) != 2 || r.Files[0].Path != "/src/main.ts" || r.Files[1].Path != "/src/minimal-scalars-v2.d.ts" {
		t.Fatal("complete independent input/case set changed")
	}
	source := r.Files[0].Content
	if len(source) != 787 || len(utf16.Encode([]rune(source))) != 783 {
		t.Fatal("CRLF and astral original source changed")
	}
	for _, file := range r.Files {
		sum := sha256.Sum256([]byte(file.Content))
		if hex.EncodeToString(sum[:]) != file.SHA256 {
			t.Fatal("independent file checksum mismatch")
		}
	}
	schemaSum := sha256.Sum256(schema)
	schemaID := "sha256:" + hex.EncodeToString(schemaSum[:])
	payload := want["payload"]
	if want["schemaHash"] != schemaID || want["requestId"] != stringOracleHash24("tsfg.string-facts.request.v1\x00", input) || want["packageId"] != stringOracleHash24("tsfg.string-facts.package.v1\x00"+schemaID+"\x00", stringOracleCanonical24(t, payload)) {
		t.Fatal("independent complete package identity preimages changed")
	}
	if !bytes.Equal(expected, append(stringOracleCanonical24(t, want), '\n')) {
		t.Fatal("complete expected package is not JCS plus one LF")
	}
	t.Logf("TSFG_STRING_FACTS_REQUEST_V1 %s", base64.StdEncoding.EncodeToString(input))
	t.Logf("TSFG_STRING_FACTS_EXPECTED_V1 %s", base64.StdEncoding.EncodeToString(expected))
	program := throwBoundaryProgramForTest(t, r)
	if !t.Run("officialDiagnostics", func(t *testing.T) { throwBoundaryDiagnosticsForTest(t, program) }) {
		return
	}
	if !t.Run("officialLiteralValuesSignaturesExportsUTF16", func(t *testing.T) {
		file := program.GetSourceFile(r.RootFile)
		if file == nil || file.Text() != source || !ast.IsExternalModule(file) || len(file.Statements.Nodes) != 14 {
			t.Fatal("complete official module declarations")
		}
		consumed := []string{}
		for _, f := range program.GetSourceFiles() {
			consumed = append(consumed, f.FileName())
		}
		slices.Sort(consumed)
		if !reflect.DeepEqual(consumed, []string{"/src/main.ts", "/src/minimal-scalars-v2.d.ts"}) {
			t.Fatal("official source closure")
		}
		check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
		defer release()
		exports := check.GetExportsOfModule(file.AsNode().Symbol())
		if len(exports) != 14 {
			t.Fatal("official complete exports")
		}
		// These literal raw bytes are independently pinned to scanner's documented representation.
		rawHex := []string{"", "00", "f09f9880", "f09f9880", "f09f9880", "eda080", "edb080", "edb880eda0bd", "41eda08000edb0805a", "5c220a0d094142", "f0908080", "ee8080", "65cc81", "c3a9"}
		anchors, total := 0, 0
		for i, tc := range oracle.Cases {
			node := file.Statements.Nodes[i]
			if node.Kind != ast.KindFunctionDeclaration {
				t.Fatal("official function kind")
			}
			fn := node.AsFunctionDeclaration()
			if fn.Name() == nil || fn.Name().Text() != tc.Name || len(fn.Parameters.Nodes) != 0 || fn.Type == nil || fn.Body == nil || fn.Body.Kind != ast.KindBlock {
				t.Fatalf("official zero-parameter string function %s", tc.Name)
			}
			body := fn.Body.AsBlock().Statements.Nodes
			if len(body) != 1 || body[0].Kind != ast.KindReturnStatement {
				t.Fatal("complete single return body")
			}
			literal := body[0].AsReturnStatement().Expression
			if literal == nil || literal.Kind != ast.KindStringLiteral {
				t.Fatal("ordinary StringLiteral required")
			}
			actualType := check.GetTypeAtLocation(literal)
			if actualType == nil || !actualType.IsStringLiteral() || actualType.Flags() != checker.TypeFlagsStringLiteral {
				t.Fatal("official literal flags")
			}
			value, ok := actualType.AsLiteralType().Value().(string)
			if !ok || hex.EncodeToString([]byte(value)) != rawHex[i] || literal.Text() != value {
				t.Fatalf("official raw Value/AST bytes %s: %x", tc.Name, []byte(value))
			}
			symbol := check.GetSymbolAtLocation(fn.Name())
			if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != node {
				t.Fatal("official unique function binding")
			}
			signatures := check.GetSignaturesOfType(check.GetTypeOfSymbol(symbol), checker.SignatureKindCall)
			if len(signatures) != 1 || signatures[0].Declaration() != node || len(signatures[0].Parameters()) != 0 {
				t.Fatal("official unique zero-param signature")
			}
			returned := check.GetReturnTypeOfSignature(signatures[0])
			declared := check.GetTypeFromTypeNode(fn.Type)
			if returned == nil || returned != declared || returned.Flags() != checker.TypeFlagsString {
				t.Fatal("official return must be string")
			}
			matches := 0
			for _, exported := range exports {
				if exported == symbol && exported.Name == tc.Name && len(exported.Declarations) == 1 && exported.Declarations[0] == node && ast.GetSourceFileOfNode(node) == file {
					matches++
				}
			}
			if matches != 1 {
				t.Fatal("same official symbol/export/declaration identity")
			}
			stringOracleAnchor24(t, file, node, tc.DeclarationRange, tc.DeclarationSource)
			stringOracleAnchor24(t, file, body[0], tc.ReturnRange, tc.ReturnSource)
			stringOracleAnchor24(t, file, literal, tc.LiteralRange, tc.LiteralSource)
			anchors += 3
			total += len(tc.CodeUnits)
			t.Logf("OFFICIAL_STRING_FACT_V1 name=%s flags=%d returnFlags=%d rawHex=%s codeUnits=%v declaration=%+v return=%+v literal=%+v", tc.Name, actualType.Flags(), returned.Flags(), rawHex[i], tc.CodeUnits, tc.DeclarationRange, tc.ReturnRange, tc.LiteralRange)
		}
		if anchors != 42 || total != 29 {
			t.Fatal("incomplete independent anchor/code-unit control")
		}
	}) {
		return
	}
	t.Run("privateCommandCompleteFrozenPackage", func(t *testing.T) {
		before := bytes.Clone(input)
		var output bytes.Buffer
		err := runFrontendCommand(t.Context(), []string{"--string-facts-io-v1"}, bytes.NewReader(input), &output)
		if !bytes.Equal(before, input) {
			t.Fatal("command changed caller input bytes")
		}
		t.Logf("TSFG_STRING_FACTS_ACTUAL_V1 %s", base64.StdEncoding.EncodeToString(output.Bytes()))
		if err != nil {
			var failure *stringFactsFailure
			if errors.As(err, &failure) {
				encoded, _ := json.Marshal(failure)
				t.Logf("TSFG_STRING_FACTS_FAILURE_V1 %s", base64.StdEncoding.EncodeToString(encoded))
			}
			if output.Len() != 0 {
				t.Fatal("failed command published partial package")
			}
			t.Fatalf("real public string-facts command failed: %v", err)
		}
		if !bytes.Equal(output.Bytes(), expected) {
			t.Fatalf("complete frozen package mismatch\nactual=%s\nexpected=%s", output.Bytes(), expected)
		}
	})
}
