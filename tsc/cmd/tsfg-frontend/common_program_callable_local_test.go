// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

// The public complete-Program producer is the seam. Original checker Symbols
// and signatures are inspected before it runs; no projected DTO is its oracle.
func TestCommonProgramCallableLocal19o(t *testing.T) {
	for _, sample := range []struct {
		name, sha string
		functions int
		rejected  bool
	}{
		{"local-direct", "6fde305c240639d21f0c0158bd5a7df9b09dd23def1587a4a3b28fd1c5263896", 19, false},
		{"local-const", "ac3f240a2a889c8037fffc693f2a986734791ecb72055ae9c955632799572df3", 18, false},
		{"local-reassignment", "524fbbf34d82f6489bb81c41b8fcca75c4faa2f7a201e5afbbced1e4550f191d", 18, false},
		{"local-sibling-writer", "58836c82109c63cf7df4af8707435da6450e93c204d82dbd0f579940aee374e0", 19, false},
		{"local-ancestor", "8c63f1c7b6526b4f9f01018945f0468c505adfa293abe28555e1b9cf4f6e36ce", 19, false},
		{"local-shadow", "d17e0fc41da28c2f9b70bc17b4bfe3e26edb04d488d782ed2889fb91dac898fe", 19, false},
		{"local-uninitialized", "a0f603f98d2c48c386e1c7b9fd9cf33741688b5f799cfc127381b3d4683db1e8", 18, true},
	} {
		t.Run(sample.name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/common-program-v2/callable-local/" + sample.name + ".ts")
			if err != nil {
				t.Fatal(err)
			}
			if arrayFactsSHA(source) != sample.sha {
				t.Fatal("frozen complete Source changed")
			}
			original := scalarParameterCase19m{functions: sample.functions, reader: "makeOwnedReader.readOwned"}
			program := scalarParameterOriginal19m(t, string(source), original)
			file := program.GetSourceFile("/src/main.ts")
			check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
			declarations := map[*ast.Symbol]*ast.Node{}
			uses := map[*ast.Node][]commonProgramCaptureOccurrence{}
			var walk func(*ast.Node)
			walk = func(n *ast.Node) {
				if n.Kind == ast.KindVariableDeclaration && n.Name().Text() == "selected" && commonLexicalOwner37s(n).Name().Text() != "makeReaderBox" {
					symbol := check.GetSymbolAtLocation(n.Name())
					if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != n {
						t.Fatal("unique local Symbol lost")
					}
					decl := n.AsVariableDeclaration()
					sigs := check.GetSignaturesOfType(check.GetTypeAtLocation(n.Name()), checker.SignatureKindCall)
					if decl.Type == nil || decl.Type.Kind != ast.KindFunctionType || len(sigs) != 1 || len(sigs[0].Parameters()) != 0 || sigs[0].Declaration() != decl.Type || (decl.Initializer == nil) != sample.rejected {
						t.Fatal("original callable local type/initializer/signature differs")
					}
					declarations[symbol] = n
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
							write := n.Parent.Kind == ast.KindBinaryExpression && n.Parent.AsBinaryExpression().Left == n && n.Parent.AsBinaryExpression().OperatorToken.Kind == ast.KindEqualsToken
							uses[owner] = append(uses[owner], commonProgramCaptureOccurrence{Range: *genericSpan(n), Read: !write, Write: write})
						}
						if n.Parent.Kind == ast.KindCallExpression && n.Parent.AsCallExpression().Expression == n {
							sig := check.GetResolvedSignature(n.Parent)
							if sig == nil || sig.Declaration() != decl.AsVariableDeclaration().Type || check.GetReturnTypeOfSignature(sig) != check.GetTypeAtLocation(n.Parent) {
								t.Fatal("original selected callable signature differs")
							}
						}
					}
				}
				n.ForEachChild(func(c *ast.Node) bool { captureWalk(c); return false })
			}
			captureWalk(file.AsNode())
			if len(uses) != 1 && sample.name != "local-sibling-writer" || sample.name == "local-sibling-writer" && len(uses) != 2 {
				t.Fatal("independent local reader/writer inventory differs")
			}
			release()
			t.Log("pre-producer official local declaration/Symbol/initializer/signature and descendant accesses verified")
			request := scalarParameterRequest19m(t, source)
			readSpanCaptureSave26i(t, sample.name+"-request.json", request)
			var output bytes.Buffer
			err = runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(request), &output)
			if err != nil {
				if output.Len() != 0 {
					t.Fatal("failed producer published partial facts")
				}
				if failure, ok := err.(*arrayFactsFailure); ok {
					b, marshalErr := json.Marshal(failure)
					if marshalErr != nil {
						t.Fatal(marshalErr)
					}
					readSpanCaptureSave26i(t, sample.name+"-failure.json", append(b, '\n'))
					t.Log(string(b))
					if sample.rejected && failure.Code == "r03-array-facts-capability" {
						return
					}
				}
				t.Fatal(err)
			}
			if sample.rejected {
				t.Fatal("uninitialized local capture accepted")
			}
			readSpanCaptureSave26i(t, sample.name+"-package.json", output.Bytes())
			var packet commonProgramPackage
			if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}
			scalarParameterOfficial19m(t, string(source), program, packet.Payload, original)
			for owner, occurrences := range uses {
				var fn arrayGenericFunction
				for _, candidate := range packet.Payload.Functions {
					if candidate.DeclarationRange == *genericSpan(owner) {
						fn = candidate
					}
				}
				if fn.ID == "" || fn.Captures == nil || len(*fn.Captures) != 1 || !reflect.DeepEqual((*fn.Captures)[0].Occurrences, occurrences) {
					t.Fatal("original local capture occurrences differ")
				}
				binding := (*fn.Captures)[0].Binding
				read, write := false, false
				for _, occurrence := range occurrences {
					read = read || occurrence.Read
					write = write || occurrence.Write
				}
				if (*fn.Captures)[0].Read != read || (*fn.Captures)[0].Write != write {
					t.Fatal("original capture read/write summary differs")
				}
				var found bool
				for _, decl := range declarations {
					for _, parent := range packet.Payload.Functions {
						for _, b := range parent.Bindings {
							if b.ID == binding && b.DeclarationRange == *genericSpan(decl) && b.NameRange == *genericSpan(decl.Name()) && b.Kind == "local" && b.Name == "selected" && b.Writable == (decl.Parent.Flags&ast.NodeFlagsConst == 0) && b.LexicalOwner != nil && *b.LexicalOwner != nil && **b.LexicalOwner == parent.ID {
								found = true
							}
						}
					}
				}
				if !found {
					t.Fatal("capture lost original local binding/owner/source anchor")
				}
			}
		})
	}
}
