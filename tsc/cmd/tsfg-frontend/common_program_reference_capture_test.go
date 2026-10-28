// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

// The oracle inspects the original official Program before the public producer.
// Its spans count UTF16 directly from the frozen original text.
func TestCommonProgramReferenceCapture19t(t *testing.T) {
	if root := os.Getenv("TSFG_READSPAN_CAPTURE_ROOT"); root != "" {
		t.Setenv("TSFG_READSPAN_CAPTURE_ROOT", root+"/reference-capture")
	}
	rawCases, err := os.ReadFile("testdata/common-program-v2/reference-capture/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name, SHA string
		Failure   bool
	}
	if err := json.Unmarshal(rawCases, &cases); err != nil {
		t.Fatal(err)
	}
	for _, sample := range cases {
		name := sample.Name
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/common-program-v2/reference-capture/" + name + ".ts")
			if err != nil {
				t.Fatal(err)
			}
			if arrayFactsSHA(source) != sample.SHA {
				t.Fatal("frozen complete Source changed")
			}
			if sample.Failure {
				request := scalarParameterRequest19m(t, source)
				readSpanCaptureSave26i(t, name+"-request.json", request)
				var output bytes.Buffer
				err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(request), &output)
				if err == nil || output.Len() != 0 {
					t.Fatal("unsupported original capture published")
				}
				failure, ok := err.(*arrayFactsFailure)
				if !ok {
					t.Fatal(err)
				}
				expected := "r03-array-facts-capability"
				if name == "const-write" || name == "wrong-class" || name == "mutable-span-field" || name == "constructor-uninitialized" {
					expected = "r03-array-facts-types"
				}
				if failure.Code != expected {
					t.Fatalf("wrong original Source rejection layer: %s", failure.Code)
				}
				raw, e := json.Marshal(failure)
				if e != nil {
					t.Fatal(e)
				}
				readSpanCaptureSave26i(t, name+"-failure.json", append(raw, '\n'))
				t.Log(string(raw))
				return
			}
			var program *compiler.Program
			var imported []byte
			if name == "named-import" {
				imported, err = os.ReadFile("testdata/common-program-v2/reference-capture/held.ts")
				if err != nil {
					t.Fatal(err)
				}
				if arrayFactsSHA(imported) != "4ae14d45d59669b516101408ac6600783ca229958bd7bc0f2a48a57fe3817ce1" {
					t.Fatal("frozen imported nominal declaration changed")
				}
				files := map[string]string{"/src/main.ts": string(source), "/src/held.ts": string(imported), "/src/minimal-arrays-v8.d.ts": string(arrayGenericDeclarations)}
				program = compiler.NewProgram(compiler.ProgramOptions{Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{FileNames: []string{"/src/main.ts", "/src/held.ts", "/src/minimal-arrays-v8.d.ts"}, CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext}}}, Host: compiler.NewCompilerHost("/src", vfstest.FromMap(files, true), "/src", nil, nil), SingleThreaded: core.TSTrue})
				for _, diagnostics := range [][]*ast.Diagnostic{program.GetSyntacticDiagnostics(t.Context(), nil), program.GetSemanticDiagnostics(t.Context(), nil), program.GetProgramDiagnostics(), program.GetGlobalDiagnostics(t.Context())} {
					if len(diagnostics) != 0 {
						t.Fatalf("independent imported official diagnostics: %+v", diagnostics)
					}
				}
			} else {
				program = commonMethodProgram37q(t, string(source))
			}
			file := program.GetSourceFile("/src/main.ts")
			check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
			if name == "named-import" {
				var aliasFound bool
				var aliases func(*ast.Node)
				aliases = func(n *ast.Node) {
					if n.Kind == ast.KindIdentifier && n.Text() == "ImportedHeld" {
						sym := check.GetSymbolAtLocation(n)
						if sym == nil || sym.Flags&ast.SymbolFlagsAlias == 0 {
							t.Fatal("original named import Symbol lost")
						}
						original := check.GetAliasedSymbol(sym)
						if original == nil || len(original.Declarations) != 1 || ast.GetSourceFileOfNode(original.Declarations[0]).FileName() != "/src/held.ts" {
							t.Fatal("original nominal imported declaration lost")
						}
						aliasFound = true
					}
					n.ForEachChild(func(c *ast.Node) bool { aliases(c); return false })
				}
				aliases(file.AsNode())
				if !aliasFound {
					t.Fatal("named import oracle empty")
				}
			}
			anchor := func(n *ast.Node) span {
				source := []byte(ast.GetSourceFileOfNode(n).Text())
				start := n.Pos()
				for start < n.End() && (source[start] == ' ' || source[start] == '\n' || source[start] == '\r' || source[start] == '\t') {
					start++
				}
				return span{len(utf16.Encode([]rune(string(source[:start])))), len(utf16.Encode([]rune(string(source[:n.End()]))))}
			}
			owner := func(n *ast.Node) *ast.Node {
				for p := n.Parent; p != nil; p = p.Parent {
					if p.Kind == ast.KindFunctionDeclaration {
						return p
					}
				}
				return nil
			}
			decls := map[*ast.Symbol]*ast.Node{}
			uses := map[*ast.Node]map[*ast.Node][]commonProgramCaptureOccurrence{}
			constructors := map[span]struct {
				path        string
				declaration span
				parameters  int
			}{}
			var visit func(*ast.Node)
			visit = func(n *ast.Node) {
				if n.Kind == ast.KindNewExpression {
					target := n.AsNewExpression().Expression.Text()
					if target == "Held" || target == "ImportedHeld" {
						sig := check.GetResolvedSignature(n)
						if sig == nil || sig.Declaration() == nil || sig.Declaration().Kind != ast.KindConstructor {
							t.Fatal("original selected nominal constructor signature missing")
						}
						constructors[anchor(n)] = struct {
							path        string
							declaration span
							parameters  int
						}{ast.GetSourceFileOfNode(sig.Declaration()).FileName(), anchor(sig.Declaration()), len(sig.Parameters())}
					}
				}
				if (n.Kind == ast.KindVariableDeclaration || n.Kind == ast.KindParameter) && n.Name().Text() == "held" {
					sym := check.GetSymbolAtLocation(n.Name())
					if sym == nil || len(sym.Declarations) != 1 || sym.Declarations[0] != n || check.GetTypeAtLocation(n.Name()).Flags()&checker.TypeFlagsObject == 0 {
						t.Fatal("original nominal Symbol/type missing")
					}
					nominal := check.GetTypeAtLocation(n.Name()).Symbol()
					if nominal == nil || nominal.Flags&ast.SymbolFlagsClass == 0 || len(nominal.Declarations) != 1 {
						t.Fatal("original nominal class declaration missing")
					}
					decls[sym] = n
				}
				if n.Kind == ast.KindFunctionDeclaration {
					sigs := check.GetSignaturesOfType(check.GetTypeAtLocation(n.Name()), checker.SignatureKindCall)
					if len(sigs) != 1 || sigs[0].Declaration() != n {
						t.Fatal("original callable signature missing")
					}
				}
				n.ForEachChild(func(c *ast.Node) bool { visit(c); return false })
			}
			visit(file.AsNode())
			visit = func(n *ast.Node) {
				if n.Kind == ast.KindIdentifier {
					if d := decls[check.GetSymbolAtLocation(n)]; d != nil && n != d.Name() && owner(n) != owner(d) {
						if uses[owner(n)] == nil {
							uses[owner(n)] = map[*ast.Node][]commonProgramCaptureOccurrence{}
						}
						write := n.Parent.Kind == ast.KindBinaryExpression && n.Parent.AsBinaryExpression().Left == n && n.Parent.AsBinaryExpression().OperatorToken.Kind == ast.KindEqualsToken
						uses[owner(n)][d] = append(uses[owner(n)][d], commonProgramCaptureOccurrence{Range: anchor(n), Read: !write, Write: write})
					}
				}
				n.ForEachChild(func(c *ast.Node) bool { visit(c); return false })
			}
			visit(file.AsNode())
			release()
			if len(uses) == 0 {
				t.Fatal("original capture oracle empty")
			}
			t.Log("pre-producer original nominal Symbol/type/signatures and UTF16 direct accesses verified")
			request := scalarParameterRequest19m(t, source)
			if name == "exported-entry" {
				var input commonProgramRequest
				if err := json.Unmarshal(request, &input); err != nil {
					t.Fatal(err)
				}
				input.Input.CompilationClosure.Roots = append(input.Input.CompilationClosure.Roots, closureRoot{Kind: "application", Name: "referenceEntry", SourcePath: "/src/main.ts"})
				sort.Slice(input.Input.CompilationClosure.Roots, func(i, j int) bool {
					a, b := input.Input.CompilationClosure.Roots[i], input.Input.CompilationClosure.Roots[j]
					return a.Kind+"\x00"+a.SourcePath+"\x00"+a.Name < b.Kind+"\x00"+b.SourcePath+"\x00"+b.Name
				})
				request, err = arrayFactsCanonical(input)
				if err != nil {
					t.Fatal(err)
				}
				request = append(request, '\n')
			}
			if name == "named-import" {
				var input commonProgramRequest
				if err := json.Unmarshal(request, &input); err != nil {
					t.Fatal(err)
				}
				input.Input.Files = append(input.Input.Files, arrayFactsInputFile{Path: "/src/held.ts", Content: string(imported), SHA256: arrayFactsSHA(imported)})
				sort.Slice(input.Input.Files, func(i, j int) bool { return input.Input.Files[i].Path < input.Input.Files[j].Path })
				input.Input.CompilationClosure.Roots = append(input.Input.CompilationClosure.Roots, closureRoot{Kind: "module-initialization", Name: "$moduleInit", SourcePath: "/src/held.ts"})
				sort.Slice(input.Input.CompilationClosure.Roots, func(i, j int) bool {
					a, b := input.Input.CompilationClosure.Roots[i], input.Input.CompilationClosure.Roots[j]
					return a.Kind+"\x00"+a.SourcePath+"\x00"+a.Name < b.Kind+"\x00"+b.SourcePath+"\x00"+b.Name
				})
				request, err = arrayFactsCanonical(input)
				if err != nil {
					t.Fatal(err)
				}
				request = append(request, '\n')
			}
			readSpanCaptureSave26i(t, name+"-request.json", request)
			var output bytes.Buffer
			if err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(request), &output); err != nil {
				if f, ok := err.(*arrayFactsFailure); ok {
					raw, e := json.Marshal(f)
					if e != nil {
						t.Fatal(e)
					}
					readSpanCaptureSave26i(t, name+"-failure.json", append(raw, '\n'))
					t.Log(string(raw))
				}
				if output.Len() != 0 {
					t.Fatal("partial reference facts published")
				}
				t.Fatal(err)
			}
			readSpanCaptureSave26i(t, name+"-package.json", output.Bytes())
			var packet commonProgramPackage
			if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}
			var expression func(arrayGenericExpression)
			expression = func(value arrayGenericExpression) {
				if expected, ok := constructors[value.Range]; ok && value.Kind == "new" {
					if value.Signature == nil || value.Signature.SourcePath != expected.path || value.Signature.DeclarationRange != expected.declaration || len(value.Signature.Parameters) != expected.parameters {
						t.Fatal("original selected nominal constructor signature differs")
					}
					delete(constructors, value.Range)
				}
				for _, child := range []*arrayGenericExpression{value.Receiver, value.Index, value.Expression, value.Value, value.Left, value.Right} {
					if child != nil {
						expression(*child)
					}
				}
				for _, children := range []*[]arrayGenericExpression{value.Arguments, value.Elements} {
					if children != nil {
						for _, child := range *children {
							expression(child)
						}
					}
				}
			}
			var statements func([]arrayGenericStatement)
			statements = func(rows []arrayGenericStatement) {
				for _, row := range rows {
					for _, value := range []*arrayGenericExpression{row.Initializer, row.Value, row.Condition} {
						if value != nil {
							expression(*value)
						}
					}
					for _, body := range []*[]arrayGenericStatement{row.Then, row.Else, row.Body} {
						if body != nil {
							statements(*body)
						}
					}
					if row.CatchClause != nil {
						statements(row.CatchClause.Block.Body)
					}
					if row.FinallyBlock != nil {
						statements(row.FinallyBlock.Body)
					}
				}
			}
			for _, fn := range packet.Payload.Functions {
				statements(fn.Body)
			}
			if len(constructors) != 0 {
				t.Fatal("original nominal constructor occurrence missing")
			}
			for fn, bindings := range uses {
				for decl, occurrences := range bindings {
					found := false
					for _, f := range packet.Payload.Functions {
						if f.DeclarationRange != anchor(fn) {
							continue
						}
						for _, capture := range *f.Captures {
							for _, parent := range packet.Payload.Functions {
								for _, b := range append(parent.Parameters, parent.Bindings...) {
									if b.ID != capture.Binding || b.DeclarationRange != anchor(decl) {
										continue
									}
									var nominal bool
									for _, typ := range packet.Payload.Types {
										if typ.ID == b.DeclaredType {
											nominal = typ.Kind == "reference"
										}
									}
									if !nominal || b.Name != "held" || b.NameRange != anchor(decl.Name()) || b.Writable != (decl.Kind == ast.KindParameter || decl.Parent.Flags&ast.NodeFlagsConst == 0) || (b.Kind == "parameter") != (decl.Kind == ast.KindParameter) || b.LexicalOwner == nil || *b.LexicalOwner == nil || **b.LexicalOwner != parent.ID || !reflect.DeepEqual(capture.Occurrences, occurrences) {
										t.Fatal("original reference binding/owner/type/occurrence differs")
									}
									found = true
								}
							}
						}
					}
					if !found {
						t.Fatal("original reference capture missing")
					}
				}
			}
		})
	}
}
