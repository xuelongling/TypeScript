// SPDX-License-Identifier: Apache-2.0
// Independent fixture construction supplies units; it never evaluates a Source operand.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

type stringConcatExpectedCase24e struct {
	Name                                                  string
	Declaration, Statement, Concat, Operator, Left, Right stringOracleRange24
	LeftUnits, RightUnits                                 []int32
}

func stringConcatRequest24e(t *testing.T, source string) []byte {
	t.Helper()
	return stringConcatMutate24e(t, stringConcatOracleRead24e(t, "request.json", "5815ef980b77978c732239d56b0a1e6966183e0d8710de16d3b9448d19586028"), func(r map[string]any) {
		f := r["input"].(map[string]any)["files"].([]any)[0].(map[string]any)
		sum := sha256.Sum256([]byte(source))
		f["content"], f["sha256"] = source, hex.EncodeToString(sum[:])
	})
}

func stringConcatMutate24e(t *testing.T, input []byte, edit func(map[string]any)) []byte {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(input, &value); err != nil {
		t.Fatal(err)
	}
	edit(value)
	return append(stringOracleCanonical24(t, value), '\n')
}

func stringConcatInput24e(t *testing.T, input []byte) request {
	t.Helper()
	var envelope struct{ Input request }
	if err := json.Unmarshal(input, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Input
}

func stringConcatFailure24e(t *testing.T, input []byte, kind string) *stringFactsFailure {
	t.Helper()
	before := bytes.Clone(input)
	writer := &stringWriterProbe24{}
	err := runFrontendCommand(t.Context(), []string{"--string-facts-io-v2"}, bytes.NewReader(input), writer)
	var failure *stringFactsFailure
	// Official diagnostic envelopes expose the first sorted diagnostic phase as Code.
	if !errors.As(err, &failure) || failure.Kind != kind || kind != "diagnostic" && failure.Code != "r03-string-facts-"+kind || writer.calls != 0 || writer.Len() != 0 || !bytes.Equal(input, before) {
		t.Fatalf("public v2 wanted %s / zero writes / owned input, got %v calls=%d bytes=%d", kind, err, writer.calls, writer.Len())
	}
	encoded, _ := json.Marshal(failure)
	t.Logf("TSFG_STRING_FACTS_V2_PUBLIC_REJECTION error=%v payload=%s", err, encoded)
	return failure
}

func stringConcatRange24e(start, end int) stringOracleRange24 {
	return stringOracleRange24{Start: start, End: end}
}
func stringConcatSourceUnits24e(source string) int { return len(utf16.Encode([]rune(source))) }

// This constructs one known lexical template. Units are explicit independent inputs,
// not decoded from tokens, official values or any producer output.
func stringConcatSimple24e(name, left, right string, lu, ru []int32) (string, stringConcatExpectedCase24e) {
	prefix := "export function " + name + "(): string { "
	ls := stringConcatSourceUnits24e(prefix + "return ")
	le := ls + stringConcatSourceUnits24e(left)
	ps, rs := le+1, le+3
	re := rs + stringConcatSourceUnits24e(right)
	source := prefix + "return " + left + " + " + right + "; }\n"
	return source, stringConcatExpectedCase24e{Name: name, Declaration: stringConcatRange24e(0, re+3), Statement: stringConcatRange24e(stringConcatSourceUnits24e(prefix), re+1), Concat: stringConcatRange24e(ls, re), Operator: stringConcatRange24e(ps, ps+1), Left: stringConcatRange24e(ls, le), Right: stringConcatRange24e(rs, re), LeftUnits: lu, RightUnits: ru}
}

func stringConcatShift24e(tc stringConcatExpectedCase24e, by int) stringConcatExpectedCase24e {
	for _, where := range []*stringOracleRange24{&tc.Declaration, &tc.Statement, &tc.Concat, &tc.Operator, &tc.Left, &tc.Right} {
		where.Start += by
		where.End += by
	}
	return tc
}

func stringConcatExpected24e(t *testing.T, input []byte, cases []stringConcatExpectedCase24e) []byte {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(input, &envelope); err != nil {
		t.Fatal(err)
	}
	i := envelope["input"].(map[string]any)
	functions := []any{}
	for _, tc := range cases {
		literal := func(r stringOracleRange24, units []int32) any {
			return map[string]any{"kind": "stringLiteral", "type": "string", "range": r, "codeUnits": append([]int32{}, units...)}
		}
		functions = append(functions, map[string]any{"name": tc.Name, "sourcePath": i["rootFile"], "declarationRange": tc.Declaration, "parameters": []string{}, "returnType": "string", "body": []any{map[string]any{"kind": "return", "range": tc.Statement, "value": map[string]any{"kind": "stringConcat", "type": "string", "range": tc.Concat, "operatorRange": tc.Operator, "left": literal(tc.Left, tc.LeftUnits), "right": literal(tc.Right, tc.RightUnits)}}}})
	}
	files := []any{}
	for _, raw := range i["files"].([]any) {
		f := raw.(map[string]any)
		text := f["content"].(string)
		files = append(files, map[string]any{"path": f["path"], "content": text, "sha256": f["sha256"], "byteLength": len(text), "utf16Length": stringConcatSourceUnits24e(text)})
	}
	payload := map[string]any{"sourceRoot": i["sourceRoot"], "rootFile": i["rootFile"], "positionUnit": "utf16", "config": i["config"], "declarations": i["declarations"], "files": files, "functions": functions}
	const schema = "sha256:79d8f807552e301b606aacdee7b81fcee83d8996ff2043e58c435e5f2b7b519c"
	packet := map[string]any{"family": "tsfg.string-facts-package-candidate", "version": 2, "schemaHash": schema, "requestId": stringOracleHash24("tsfg.string-facts.request.v2\x00", input), "packageId": stringOracleHash24("tsfg.string-facts.package.v2\x00"+schema+"\x00", stringOracleCanonical24(t, payload)), "payload": payload}
	return append(stringOracleCanonical24(t, packet), '\n')
}

func stringConcatSuccess24e(t *testing.T, input, expected []byte) {
	t.Helper()
	before := bytes.Clone(input)
	writer := &stringWriterProbe24{}
	if err := runFrontendCommand(t.Context(), []string{"--string-facts-io-v2"}, bytes.NewReader(input), writer); err != nil || writer.calls != 1 || !bytes.Equal(writer.Bytes(), expected) || !bytes.Equal(input, before) {
		t.Fatalf("public v2 complete independently specified packet: %v writes=%d\nactual=%s\nexpected=%s", err, writer.calls, writer.Bytes(), expected)
	}
}

func stringConcatOfficial24e(t *testing.T, input []byte, cases []stringConcatExpectedCase24e) {
	t.Helper()
	r := stringConcatInput24e(t, input)
	program := throwBoundaryProgramForTest(t, r)
	throwBoundaryDiagnosticsForTest(t, program)
	file := program.GetSourceFile(r.RootFile)
	if file == nil || !ast.IsExternalModule(file) || file.Text() != r.Files[0].Content || len(file.Statements.Nodes) != len(cases) {
		t.Fatal("official complete concat module")
	}
	consumed := []string{}
	for _, f := range program.GetSourceFiles() {
		consumed = append(consumed, f.FileName())
	}
	slices.Sort(consumed)
	paths := []string{r.Files[0].Path, r.Files[1].Path}
	slices.Sort(paths)
	if !reflect.DeepEqual(consumed, paths) {
		t.Fatal("official complete source closure")
	}
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	exports := check.GetExportsOfModule(file.AsNode().Symbol())
	if len(exports) != len(cases) {
		t.Fatal("official complete exports")
	}
	for index, tc := range cases {
		node := file.Statements.Nodes[index]
		if node.Kind != ast.KindFunctionDeclaration {
			t.Fatal("official function")
		}
		fn := node.AsFunctionDeclaration()
		if fn.Name() == nil || fn.Name().Text() != tc.Name || fn.Type == nil || fn.Type.Kind != ast.KindStringKeyword || fn.Parameters == nil || len(fn.Parameters.Nodes) != 0 || fn.Body == nil || fn.Body.Kind != ast.KindBlock {
			t.Fatal("official closed signature")
		}
		body := fn.Body.AsBlock().Statements.Nodes
		if len(body) != 1 || body[0].Kind != ast.KindReturnStatement {
			t.Fatal("official single return")
		}
		expr := body[0].AsReturnStatement().Expression
		if expr == nil || expr.Kind != ast.KindBinaryExpression {
			t.Fatal("official binary")
		}
		binary := expr.AsBinaryExpression()
		if binary.OperatorToken == nil || binary.OperatorToken.Kind != ast.KindPlusToken || binary.Type != nil || binary.Left == nil || binary.Left.Kind != ast.KindStringLiteral || binary.Right == nil || binary.Right.Kind != ast.KindStringLiteral {
			t.Fatal("official direct ordinary literal plus")
		}
		for _, operand := range []*ast.Node{binary.Left, binary.Right} {
			typ := check.GetTypeAtLocation(operand)
			if typ == nil || !typ.IsStringLiteral() {
				t.Fatal("official literal type")
			}
			value, ok := typ.AsLiteralType().Value().(string)
			if !ok || value != operand.Text() {
				t.Fatal("official literal AST/raw value identity")
			}
		}
		symbol := check.GetSymbolAtLocation(fn.Name())
		if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != node {
			t.Fatal("official binding owner")
		}
		sigs := check.GetSignaturesOfType(check.GetTypeOfSymbol(symbol), checker.SignatureKindCall)
		declared := check.GetTypeFromTypeNode(fn.Type)
		if len(sigs) != 1 || sigs[0].Declaration() != node || len(sigs[0].Parameters()) != 0 || len(sigs[0].TypeParameters()) != 0 || declared == nil || declared.Flags() != checker.TypeFlagsString || check.GetReturnTypeOfSignature(sigs[0]) != declared || check.GetTypeAtLocation(expr) != declared {
			t.Fatal("official primitive string result/signature")
		}
		matches := 0
		for _, exported := range exports {
			if exported != nil && exported.Flags&ast.SymbolFlagsAlias == 0 && (exported == symbol || symbol.ExportSymbol == exported) && len(exported.Declarations) == 1 && exported.Declarations[0] == node {
				matches++
			}
		}
		if matches != 1 {
			t.Fatal("official export/declaration join")
		}
		for _, anchor := range []struct {
			node  *ast.Node
			where stringOracleRange24
		}{{node, tc.Declaration}, {body[0], tc.Statement}, {expr, tc.Concat}, {binary.OperatorToken, tc.Operator}, {binary.Left, tc.Left}, {binary.Right, tc.Right}} {
			text := utf16.Encode([]rune(file.Text()))
			if anchor.where.Start < 0 || anchor.where.End > len(text) {
				t.Fatal("manual source range")
			}
			spelling := string(utf16.Decode(text[anchor.where.Start:anchor.where.End]))
			stringOracleAnchor24(t, file, anchor.node, anchor.where, spelling)
		}
	}
}
