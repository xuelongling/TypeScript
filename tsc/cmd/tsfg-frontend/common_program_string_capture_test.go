// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	tsjson "github.com/microsoft/typescript-go/internal/json"
	"os"
	"reflect"
	"sort"
	"testing"
)

// Observe the original official Program before publishing any facts. This is
// deliberately independent of the producer's lexical ledger and UTF16 decoder.
func TestCommonProgramStringCapture19r(t *testing.T) {
	if root := os.Getenv("TSFG_READSPAN_CAPTURE_ROOT"); root != "" {
		t.Setenv("TSFG_READSPAN_CAPTURE_ROOT", root+"/string-capture")
	}
	for _, sample := range []struct{ name, sha string }{
		{"sibling-early-write", "b1eec004b74f7b84ec01edeb85930247d7bb36d306039b8091d41a52f8125975"},
		{"string-return", "7c5026b327c31a6c600828d32bf889fd07fe34d5088824e625849503abfd695a"},
		{"ancestor", "cc68963921f2549c19bf25ae8feab8c0aff22354eda66aea728775d22bad8d6a"},
		{"descriptor-only", "e8a38d59180b914b76942950bd39eb70c8afa16a13321155d9239685dc7f64e2"},
		{"local-const", "d408f615f918895c0e2b538bdde37b751cbcde224a4061f6c98828c81256f782"},
		{"local-let", "ace61ad05ddafccac4574c58a2bf7d0a18579ba3ad847e9fcec03dec0694fc3b"},
		{"owner-replaced", "38fc5fcbd4084c055c18da7f504a8f4db55692c163a1350ded9f31ca5e1f4e0b"},
		{"owner-write", "4738ee3c3ad848938576e556f8bcdd127f96d67ee4a945a04e3bbc6cc1e0945f"},
		{"parameter", "3e2bd0ba2ed14fa3fcc58772f40f9659bcb835e38ce8301194876a70c7270ebd"},
		{"shadow", "1ba993a4dce7f0ae5d38682e7f7ff39e88cf1403afa1d6152c09adc28a3c7934"},
		{"sibling-write", "d845666de10284deddd280ad2ef6d7b017b8c3613d932270ed7a7437c6c57c7e"},
		{"uninitialized", "dfa54d910b0e780883c97776647862ea05f22f5c13498e5f69307aa35f0f2f1d"},
	} {
		t.Run(sample.name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/common-program-v2/string-capture/" + sample.name + ".ts")
			if err != nil {
				t.Fatal(err)
			}
			if arrayFactsSHA(source) != sample.sha {
				t.Fatal("frozen complete Source changed")
			}
			program := commonMethodProgram37q(t, string(source))
			file := program.GetSourceFile("/src/main.ts")
			check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
			declarations := map[*ast.Symbol]*ast.Node{}
			uses := map[*ast.Node]map[*ast.Node][]commonProgramCaptureOccurrence{}
			literals := map[span][]int32{}
			expected := map[string]struct {
				raw   string
				units []int32
			}{
				`""`: {"", []int32{}}, `"\0"`: {"00", []int32{0}}, `"😀"`: {"f09f9880", []int32{55357, 56832}},
				`"\uD800"`: {"eda080", []int32{55296}}, `"\uDC00"`: {"edb080", []int32{56320}},
				`"outer"`: {"6f75746572", []int32{111, 117, 116, 101, 114}}, `"inner"`: {"696e6e6572", []int32{105, 110, 110, 101, 114}},
				`"descriptor"`: {"64657363726970746f72", []int32{100, 101, 115, 99, 114, 105, 112, 116, 111, 114}},
				`"owner"`:      {"6f776e6572", []int32{111, 119, 110, 101, 114}},
			}
			var walk func(*ast.Node)
			walk = func(n *ast.Node) {
				if (n.Kind == ast.KindVariableDeclaration || n.Kind == ast.KindParameter) && n.Name().Text() == "text" {
					symbol := check.GetSymbolAtLocation(n.Name())
					if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != n || check.GetTypeAtLocation(n.Name()).Flags() != checker.TypeFlagsString {
						t.Fatal("original unique String binding/type lost")
					}
					declarations[symbol] = n
				}
				if n.Kind == ast.KindStringLiteral {
					token := file.Text()[n.Pos():n.End()]
					for len(token) > 0 && (token[0] == ' ' || token[0] == '\n' || token[0] == '\r') {
						token = token[1:]
					}
					want, ok := expected[token]
					if !ok {
						t.Fatalf("missing independent literal oracle %q", token)
					}
					typ := check.GetTypeAtLocation(n)
					if !typ.IsStringLiteral() || hex.EncodeToString([]byte(typ.AsLiteralType().Value().(string))) != want.raw || n.Text() != typ.AsLiteralType().Value() {
						t.Fatal("official raw UTF16 literal oracle differs")
					}
					literals[*genericSpan(n)] = want.units
				}
				if n.Kind == ast.KindFunctionDeclaration && n.AsFunctionDeclaration().Type.Kind == ast.KindStringKeyword {
					sigs := check.GetSignaturesOfType(check.GetTypeAtLocation(n.Name()), checker.SignatureKindCall)
					if len(sigs) != 1 || sigs[0].Declaration() != n || check.GetReturnTypeOfSignature(sigs[0]).Flags() != checker.TypeFlagsString {
						t.Fatal("original String signature differs")
					}
				}
				n.ForEachChild(func(c *ast.Node) bool { walk(c); return false })
			}
			walk(file.AsNode())
			var captureWalk func(*ast.Node)
			captureWalk = func(n *ast.Node) {
				if n.Kind == ast.KindIdentifier {
					if decl := declarations[check.GetSymbolAtLocation(n)]; decl != nil && n != decl.Name() {
						owner := commonLexicalOwner37s(n)
						if owner != commonLexicalOwner37s(decl) {
							if uses[owner] == nil {
								uses[owner] = map[*ast.Node][]commonProgramCaptureOccurrence{}
							}
							write := n.Parent.Kind == ast.KindBinaryExpression && n.Parent.AsBinaryExpression().Left == n && n.Parent.AsBinaryExpression().OperatorToken.Kind == ast.KindEqualsToken
							uses[owner][decl] = append(uses[owner][decl], commonProgramCaptureOccurrence{Range: *genericSpan(n), Read: !write, Write: write})
						}
					}
				}
				n.ForEachChild(func(c *ast.Node) bool { captureWalk(c); return false })
			}
			captureWalk(file.AsNode())
			if len(uses) == 0 || len(literals) == 0 {
				t.Fatal("String family oracle missing")
			}
			release()
			t.Log("pre-producer official unique Symbol, original String signatures, raw literal values and descendant accesses verified")
			request := scalarParameterRequest19m(t, source)
			if sample.name == "string-return" {
				var input commonProgramRequest
				if err := json.Unmarshal(request, &input); err != nil {
					t.Fatal(err)
				}
				input.Input.CompilationClosure.Roots = append(input.Input.CompilationClosure.Roots, closureRoot{Kind: "application", Name: "stringCaptureEntry", SourcePath: "/src/main.ts"})
				sort.Slice(input.Input.CompilationClosure.Roots, func(i, j int) bool {
					a, b := input.Input.CompilationClosure.Roots[i], input.Input.CompilationClosure.Roots[j]
					return a.Kind+"\x00"+a.SourcePath+"\x00"+a.Name < b.Kind+"\x00"+b.SourcePath+"\x00"+b.Name
				})
				var err error
				request, err = json.Marshal(input)
				if err != nil {
					t.Fatal(err)
				}
			}
			canonical := tsjson.Value(request)
			if err := canonical.Canonicalize(); err != nil {
				t.Fatal(err)
			}
			request = append(canonical, '\n')
			readSpanCaptureSave26i(t, sample.name+"-request.json", request)
			var output bytes.Buffer
			err = runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(request), &output)
			if err != nil {
				if output.Len() != 0 {
					t.Fatal("partial String facts published")
				}
				failure, ok := err.(*arrayFactsFailure)
				if ok {
					data, e := json.Marshal(failure)
					if e != nil {
						t.Fatal(e)
					}
					readSpanCaptureSave26i(t, sample.name+"-failure.json", append(data, '\n'))
					t.Log(string(data))
				}
				if sample.name == "uninitialized" && ok && failure.Code == "r03-array-facts-capability" {
					return
				}
				t.Fatal(err)
			}
			if sample.name == "uninitialized" {
				t.Fatal("uninitialized String capture admitted")
			}
			readSpanCaptureSave26i(t, sample.name+"-package.json", output.Bytes())
			var packet commonProgramPackage
			if err = json.Unmarshal(output.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}
			for owner, bindings := range uses {
				for decl, occurrences := range bindings {
					found := false
					for _, fn := range packet.Payload.Functions {
						if fn.DeclarationRange != *genericSpan(owner) {
							continue
						}
						for _, capture := range *fn.Captures {
							for _, parent := range packet.Payload.Functions {
								for _, b := range append(parent.Parameters, parent.Bindings...) {
									if b.ID != capture.Binding || b.DeclarationRange != *genericSpan(decl) {
										continue
									}
									if b.Writable != (decl.Kind == ast.KindParameter || decl.Parent.Flags&ast.NodeFlagsConst == 0) || (b.Kind == "parameter") != (decl.Kind == ast.KindParameter) || b.DeclaredType != "string" || b.Name != "text" || b.NameRange != *genericSpan(decl.Name()) || b.LexicalOwner == nil || *b.LexicalOwner == nil || **b.LexicalOwner != parent.ID || !reflect.DeepEqual(capture.Occurrences, occurrences) {
										t.Fatal("original String binding/owner/UTF16/accesses differ")
									}
									found = true
								}
							}
						}
					}
					if !found {
						t.Fatal("original capture missing")
					}
				}
			}
			var literalWalk func(arrayGenericExpression)
			literalWalk = func(v arrayGenericExpression) {
				if v.Kind == "stringLiteral" {
					if v.CodeUnits == nil || !reflect.DeepEqual(*v.CodeUnits, literals[v.Range]) {
						t.Fatal("String codeUnits differ from official literal oracle")
					}
				}
				for _, child := range []*arrayGenericExpression{v.Receiver, v.Index, v.Expression, v.Value, v.Left, v.Right} {
					if child != nil {
						literalWalk(*child)
					}
				}
				for _, group := range []*[]arrayGenericExpression{v.Arguments, v.Elements} {
					if group != nil {
						for _, child := range *group {
							literalWalk(child)
						}
					}
				}
			}
			var statements func([]arrayGenericStatement)
			statements = func(rows []arrayGenericStatement) {
				for _, row := range rows {
					for _, value := range []*arrayGenericExpression{row.Initializer, row.Value, row.Condition} {
						if value != nil {
							literalWalk(*value)
						}
					}
					for _, body := range []*[]arrayGenericStatement{row.Then, row.Else, row.Body} {
						if body != nil {
							statements(*body)
						}
					}
				}
			}
			for _, fn := range packet.Payload.Functions {
				statements(fn.Body)
			}
		})
	}
}

func TestCommonProgramStringConstWriteBoundary19r(t *testing.T) {
	source, err := os.ReadFile("testdata/common-program-v2/string-capture/local-const.ts")
	if err != nil {
		t.Fatal(err)
	}
	source = bytes.Replace(source, []byte("const current: string = text;"), []byte("text = \"\"; const current: string = text;"), 1)
	request := scalarParameterRequest19m(t, source)
	readSpanCaptureSave26i(t, "const-write-request.json", request)
	var output bytes.Buffer
	err = runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(request), &output)
	if err == nil || output.Len() != 0 {
		t.Fatal("const String rebinding must be rejected atomically")
	}
	failure, ok := err.(*arrayFactsFailure)
	if !ok {
		t.Fatal(err)
	}
	raw, e := json.Marshal(failure)
	if e != nil {
		t.Fatal(e)
	}
	readSpanCaptureSave26i(t, "const-write-failure.json", append(raw, '\n'))
	t.Log(string(raw))
}
