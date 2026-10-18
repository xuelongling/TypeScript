// SPDX-License-Identifier: Apache-2.0
// Frozen complete bytes predate the v2 producer; official values never use its converter.
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
)

type stringConcatOracleCase24e struct {
	Name, SourcePath                              string
	DeclarationRange, ReturnRange, ConcatRange    stringOracleRange24
	OperatorRange, LeftRange, RightRange          stringOracleRange24
	DeclarationSource, ReturnSource, ConcatSource string
	OperatorSource, LeftSource, RightSource       string
	LeftCodeUnits, RightCodeUnits                 []int32
	LeftRawHex, RightRawHex, StatementTerminator  string
}

type stringConcatLiteralOracle24e struct {
	Kind, Type string
	Range      stringOracleRange24
	CodeUnits  []int32
}

type stringConcatValueOracle24e struct {
	Kind, Type           string
	Range, OperatorRange stringOracleRange24
	Left, Right          stringConcatLiteralOracle24e
}

type stringConcatFunctionOracle24e struct {
	Name, SourcePath, ReturnType string
	DeclarationRange             stringOracleRange24
	Parameters                   []string
	Body                         []struct {
		Kind  string
		Range stringOracleRange24
		Value stringConcatValueOracle24e
	}
}

func stringConcatOracleRead24e(t *testing.T, name, hash string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/string-facts-v2/" + name)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != hash {
		t.Fatal("frozen independent v2 fixture changed: " + name)
	}
	return data
}

func TestStringFactsCommandV2TypedConcat(t *testing.T) {
	input := stringConcatOracleRead24e(t, "request.json", "5815ef980b77978c732239d56b0a1e6966183e0d8710de16d3b9448d19586028")
	expected := stringConcatOracleRead24e(t, "expected.json", "801792594c3f3b81d7d0a726bf99ea33772425f65abe3552236068d848c4498a")
	schema := stringConcatOracleRead24e(t, "schema.json", "79d8f807552e301b606aacdee7b81fcee83d8996ff2043e58c435e5f2b7b519c")
	manual := stringConcatOracleRead24e(t, "manual.json", "3b342e9a8012c01047c3278402ba92d39f7b07a9dc322ddd2196db20cf7cbc6f")
	var envelope struct {
		Family  string
		Version int
		Input   request
	}
	var oracle struct {
		SourceRoot, RootFile, FrozenOracleSHA256            string
		SourceUTF8Bytes, SourceUTF16Length                  int
		Functions, Anchors, Literals, TotalOperandCodeUnits int
		Cases                                               []stringConcatOracleCase24e
	}
	var want map[string]any
	if json.Unmarshal(input, &envelope) != nil || json.Unmarshal(manual, &oracle) != nil || json.Unmarshal(expected, &want) != nil {
		t.Fatal("complete independent v2 fixture decode")
	}
	r := envelope.Input
	if envelope.Family != "tsfg.string-facts-request-candidate" || envelope.Version != 2 ||
		oracle.FrozenOracleSHA256 != "5ff82efe41d01e50bd9657dd57b80925af64240a64a939ebb07af409a24c1c11" ||
		oracle.SourceRoot != "/oracle24e" || oracle.RootFile != "/oracle24e/corpus.ts" ||
		oracle.Functions != 6 || oracle.Anchors != 36 || oracle.Literals != 12 || oracle.TotalOperandCodeUnits != 13 || len(oracle.Cases) != 6 ||
		r.SourceRoot != oracle.SourceRoot || r.RootFile != oracle.RootFile || len(r.Files) != 2 ||
		r.Files[0].Path != oracle.RootFile || r.Files[1].Path != "/oracle24e/minimal-scalars-v2.d.ts" {
		t.Fatal("complete frozen v2 corpus/owners changed")
	}
	source := r.Files[0].Content
	if oracle.SourceUTF8Bytes != 427 || oracle.SourceUTF16Length != 422 || len(source) != 427 || len(utf16.Encode([]rune(source))) != 422 || len(input) != 1393 || len(expected) != 4855 || len(schema) != 8087 {
		t.Fatal("exact source or complete wire byte lengths changed")
	}
	for _, file := range r.Files {
		sum := sha256.Sum256([]byte(file.Content))
		if hex.EncodeToString(sum[:]) != file.SHA256 {
			t.Fatal("independent source checksum mismatch")
		}
	}
	schemaSum := sha256.Sum256(schema)
	schemaID := "sha256:" + hex.EncodeToString(schemaSum[:])
	if want["family"] != "tsfg.string-facts-package-candidate" || want["version"] != float64(2) ||
		want["schemaHash"] != schemaID || want["requestId"] != stringOracleHash24("tsfg.string-facts.request.v2\x00", input) ||
		want["packageId"] != stringOracleHash24("tsfg.string-facts.package.v2\x00"+schemaID+"\x00", stringOracleCanonical24(t, want["payload"])) {
		t.Fatal("complete independent v2 identity preimages changed")
	}
	var requestJSON, schemaJSON any
	if json.Unmarshal(input, &requestJSON) != nil || json.Unmarshal(schema, &schemaJSON) != nil ||
		!bytes.Equal(input, append(stringOracleCanonical24(t, requestJSON), '\n')) ||
		!bytes.Equal(schema, append(stringOracleCanonical24(t, schemaJSON), '\n')) ||
		!bytes.Equal(expected, append(stringOracleCanonical24(t, want), '\n')) {
		t.Fatal("frozen schema/request/packet must be complete JCS plus exactly one LF")
	}
	t.Logf("TSFG_STRING_FACTS_REQUEST_V2 %s", base64.StdEncoding.EncodeToString(input))
	t.Logf("TSFG_STRING_FACTS_EXPECTED_V2 %s", base64.StdEncoding.EncodeToString(expected))
	program := throwBoundaryProgramForTest(t, r)
	if !t.Run("officialDiagnostics", func(t *testing.T) {
		throwBoundaryDiagnosticsForTest(t, program)
		t.Log("OFFICIAL_STRING_FACTS_V2_DIAGNOSTICS syntactic=0 semantic=0 program=0 global=0")
	}) {
		return
	}
	if !t.Run("officialConcatTypesOwnersExportsUTF16", func(t *testing.T) {
		file := program.GetSourceFile(r.RootFile)
		if file == nil || file.Text() != source || !ast.IsExternalModule(file) || len(file.Statements.Nodes) != 6 {
			t.Fatal("complete official module declarations")
		}
		consumed := []string{}
		for _, f := range program.GetSourceFiles() {
			consumed = append(consumed, f.FileName())
		}
		slices.Sort(consumed)
		if !reflect.DeepEqual(consumed, []string{"/oracle24e/corpus.ts", "/oracle24e/minimal-scalars-v2.d.ts"}) {
			t.Fatal("complete actual official source closure")
		}
		for _, owned := range r.Files {
			actual := program.GetSourceFile(owned.Path)
			if actual == nil || actual.Text() != owned.Content {
				t.Fatal("official source closure content ownership")
			}
		}
		check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
		defer release()
		if file.AsNode().Symbol() == nil {
			t.Fatal("official module symbol missing")
		}
		exports := check.GetExportsOfModule(file.AsNode().Symbol())
		if len(exports) != 6 {
			t.Fatal("complete direct official exports")
		}
		anchors, literals, total := 0, 0, 0
		for i, tc := range oracle.Cases {
			node := file.Statements.Nodes[i]
			if node.Kind != ast.KindFunctionDeclaration || node.ModifierFlags() != ast.ModifierFlagsExport {
				t.Fatal("official directly exported function kind")
			}
			fn := node.AsFunctionDeclaration()
			if fn.Name() == nil || fn.Name().Kind != ast.KindIdentifier || fn.Name().Text() != tc.Name || tc.SourcePath != file.FileName() ||
				fn.AsteriskToken != nil || fn.TypeParameters != nil && len(fn.TypeParameters.Nodes) != 0 ||
				fn.Parameters == nil || len(fn.Parameters.Nodes) != 0 || fn.FullSignature != nil ||
				fn.Type == nil || fn.Type.Kind != ast.KindStringKeyword || fn.Body == nil || fn.Body.Kind != ast.KindBlock {
				t.Fatalf("official closed zero-parameter string function %s", tc.Name)
			}
			body := fn.Body.AsBlock().Statements.Nodes
			if len(body) != 1 || body[0].Kind != ast.KindReturnStatement {
				t.Fatal("complete official single-return body")
			}
			expression := body[0].AsReturnStatement().Expression
			if expression == nil || expression.Kind != ast.KindBinaryExpression {
				t.Fatal("official direct binary return")
			}
			binary := expression.AsBinaryExpression()
			if binary.Type != nil || expression.ModifierFlags() != 0 || binary.OperatorToken == nil || binary.OperatorToken.Kind != ast.KindPlusToken ||
				binary.Left == nil || binary.Left.Kind != ast.KindStringLiteral || binary.Right == nil || binary.Right.Kind != ast.KindStringLiteral || binary.Left == binary.Right {
				t.Fatal("official plus with two distinct direct ordinary StringLiteral operands")
			}
			binaryType := check.GetTypeAtLocation(expression)
			if binaryType == nil || binaryType.Flags() != checker.TypeFlagsString {
				t.Fatal("official binary result must be primitive string")
			}
			for _, operand := range []struct {
				node   *ast.Node
				rawHex string
			}{{binary.Left, tc.LeftRawHex}, {binary.Right, tc.RightRawHex}} {
				typ := check.GetTypeAtLocation(operand.node)
				if typ == nil || !typ.IsStringLiteral() || typ.Flags() != checker.TypeFlagsStringLiteral || ast.GetSourceFileOfNode(operand.node) != file {
					t.Fatal("official owned operand literal type")
				}
				value, ok := typ.AsLiteralType().Value().(string)
				if !ok || value != operand.node.Text() || hex.EncodeToString([]byte(value)) != operand.rawHex {
					t.Fatalf("official operand raw Value/AST bytes %s: %x", tc.Name, []byte(value))
				}
				literals++
			}
			symbol := check.GetSymbolAtLocation(fn.Name())
			if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != node || ast.GetSourceFileOfNode(node) != file {
				t.Fatal("official unique function binding owner")
			}
			signatures := check.GetSignaturesOfType(check.GetTypeOfSymbol(symbol), checker.SignatureKindCall)
			declared := check.GetTypeFromTypeNode(fn.Type)
			if len(signatures) != 1 || signatures[0].Declaration() != node || len(signatures[0].Parameters()) != 0 || len(signatures[0].TypeParameters()) != 0 ||
				declared == nil || declared.Flags() != checker.TypeFlagsString || check.GetReturnTypeOfSignature(signatures[0]) != declared || binaryType != declared {
				t.Fatal("official unique primitive-string signature and binary result")
			}
			matches := 0
			for _, exported := range exports {
				if exported != nil && exported.Flags&ast.SymbolFlagsAlias == 0 && (exported == symbol || symbol.ExportSymbol == exported) && exported.Name == tc.Name &&
					len(exported.Declarations) == 1 && exported.Declarations[0] == node {
					matches++
				}
			}
			if matches != 1 {
				t.Fatal("same official symbol/export/declaration identity")
			}
			stringOracleAnchor24(t, file, node, tc.DeclarationRange, tc.DeclarationSource)
			stringOracleAnchor24(t, file, body[0], tc.ReturnRange, tc.ReturnSource)
			stringOracleAnchor24(t, file, expression, tc.ConcatRange, tc.ConcatSource)
			stringOracleAnchor24(t, file, binary.OperatorToken, tc.OperatorRange, tc.OperatorSource)
			stringOracleAnchor24(t, file, binary.Left, tc.LeftRange, tc.LeftSource)
			stringOracleAnchor24(t, file, binary.Right, tc.RightRange, tc.RightSource)
			if tc.StatementTerminator == "ASI" && body[0].End() != expression.End() {
				t.Fatal("official equal-end ASI anchor")
			}
			anchors += 6
			total += len(tc.LeftCodeUnits) + len(tc.RightCodeUnits)
			t.Logf("OFFICIAL_STRING_FACT_V2 name=%s resultFlags=%d leftRawHex=%s rightRawHex=%s leftUnits=%v rightUnits=%v declaration=%+v return=%+v concat=%+v plus=%+v left=%+v right=%+v", tc.Name, binaryType.Flags(), tc.LeftRawHex, tc.RightRawHex, tc.LeftCodeUnits, tc.RightCodeUnits, tc.DeclarationRange, tc.ReturnRange, tc.ConcatRange, tc.OperatorRange, tc.LeftRange, tc.RightRange)
		}
		if anchors != 36 || literals != 12 || total != 13 {
			t.Fatal("incomplete independent official anchors/literals/manual units")
		}
	}) {
		return
	}
	// The exclusive checker above has been released before entering the public producer.
	t.Run("publicCommandCompleteFrozenPackage", func(t *testing.T) {
		before := bytes.Clone(input)
		var output bytes.Buffer
		err := runFrontendCommand(t.Context(), []string{"--string-facts-io-v2"}, bytes.NewReader(input), &output)
		if !bytes.Equal(before, input) {
			t.Fatal("public command changed caller input bytes")
		}
		t.Logf("TSFG_STRING_FACTS_ACTUAL_V2 %s", base64.StdEncoding.EncodeToString(output.Bytes()))
		if err != nil {
			var failure *stringFactsFailure
			if errors.As(err, &failure) {
				encoded, _ := json.Marshal(failure)
				t.Logf("TSFG_STRING_FACTS_FAILURE_V2 %s", base64.StdEncoding.EncodeToString(encoded))
			}
			if err.Error() == "tsfg-frontend accepts one JSON request on stdin" {
				t.Log("TSFG_STRING_FACTS_V2_FIRST_FAILURE_CLASS MECHANICAL_OR_API_NOT_CAPABILITY")
			}
			if output.Len() != 0 {
				t.Fatal("failed public command published partial package")
			}
			t.Fatalf("real public v2 command failed after all official prerequisites: %v", err)
		}
		if !bytes.Equal(output.Bytes(), expected) {
			t.Fatalf("complete frozen v2 package mismatch\nactual=%s\nexpected=%s", output.Bytes(), expected)
		}
		var cold struct {
			Payload struct {
				Files     []struct{ Content string }
				Functions []stringConcatFunctionOracle24e
			}
		}
		if json.Unmarshal(output.Bytes(), &cold) != nil || len(cold.Payload.Functions) != 6 || len(cold.Payload.Files) != 2 {
			t.Fatal("cold complete v2 JSON ownership decode")
		}
		for i, fn := range cold.Payload.Functions {
			tc := oracle.Cases[i]
			if fn.Name != tc.Name || fn.SourcePath != tc.SourcePath || fn.DeclarationRange != tc.DeclarationRange || fn.Parameters == nil || len(fn.Parameters) != 0 || fn.ReturnType != "string" || len(fn.Body) != 1 {
				t.Fatal("cold JSON function owner/order/signature changed")
			}
			statement := fn.Body[0]
			value := statement.Value
			if statement.Kind != "return" || statement.Range != tc.ReturnRange || value.Kind != "stringConcat" || value.Type != "string" || value.Range != tc.ConcatRange || value.OperatorRange != tc.OperatorRange ||
				value.Left.Kind != "stringLiteral" || value.Left.Type != "string" || value.Left.Range != tc.LeftRange || value.Right.Kind != "stringLiteral" || value.Right.Type != "string" || value.Right.Range != tc.RightRange ||
				!reflect.DeepEqual(value.Left.CodeUnits, tc.LeftCodeUnits) || !reflect.DeepEqual(value.Right.CodeUnits, tc.RightCodeUnits) {
				t.Fatal("cold JSON operation/operand owners, order, ranges or original units changed")
			}
		}
		// Only JSON data ownership is claimed here; there is no v2 reader or HIR consumer.
		cold.Payload.Functions[4].Body[0].Value.Left.CodeUnits[0] = 7
		if !reflect.DeepEqual(cold.Payload.Functions[4].Body[0].Value.Right.CodeUnits, []int32{55358, 56813}) {
			t.Fatal("equal left/right values share mutable JSON arrays")
		}
		cold.Payload.Functions[0].Body[0].Value.Right.CodeUnits[0] = 9
		if !reflect.DeepEqual(cold.Payload.Functions[1].Body[0].Value.Left.CodeUnits, []int32{0}) {
			t.Fatal("equal values under distinct function owners share mutable JSON arrays")
		}
		for i := range input {
			input[i] = 0xa5
		}
		for i := range output.Bytes() {
			output.Bytes()[i] = 0xa5
		}
		if cold.Payload.Files[0].Content != source || cold.Payload.Files[1].Content != r.Files[1].Content || !reflect.DeepEqual(cold.Payload.Functions[2].Body[0].Value.Left.CodeUnits, []int32{55357}) || !reflect.DeepEqual(cold.Payload.Functions[2].Body[0].Value.Right.CodeUnits, []int32{56960}) {
			t.Fatal("cold JSON owned source/units changed with caller or output bytes")
		}
		t.Log("TSFG_STRING_FACTS_V2_COMPLETE_BYTES_AND_JSON_OWNERS 4855B functions=6 operands=12 units=13")
	})
}
