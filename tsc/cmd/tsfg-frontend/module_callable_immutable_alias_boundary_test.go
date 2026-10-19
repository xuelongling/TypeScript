// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/scanner"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

// Each whole original TS case is official-valid before the owned profile is
// asked to reject its single role/initialization/cycle limitation. Anchors are
// preselected from the original grammar and documented rules in literal data;
// no SUT response is used to discover an expected rejection point.
func TestModuleCallableImmutableAliasOfficialBoundaries(t *testing.T) {
	var data immutableAliasPublicData23g
	if err := json.Unmarshal(immutableAliasPublicWhole23g, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.BoundaryCases) != 6 {
		t.Fatal("exact six independently frozen source cases required")
	}
	for _, tc := range data.BoundaryCases {
		t.Run(tc.Name, func(t *testing.T) {
			contents := map[string]string{}
			names := []string{}
			for _, file := range tc.Input.Files {
				contents[file.Path] = file.Content
				names = append(names, file.Path)
			}
			program := compiler.NewProgram(compiler.ProgramOptions{
				Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{FileNames: names,
					CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext}}},
				Host: compiler.NewCompilerHost(tc.Input.SourceRoot, vfstest.FromMap(contents, true), tc.Input.SourceRoot, nil, nil), SingleThreaded: core.TSTrue,
			})
			if ds := program.GetSyntacticDiagnostics(t.Context(), nil); len(ds) != 0 {
				t.Fatalf("official syntax: %+v", ds)
			}
			if ds := program.GetSemanticDiagnostics(t.Context(), nil); len(ds) != 0 {
				t.Fatalf("official semantic: %+v", ds)
			}
			if ds := program.GetProgramDiagnostics(); len(ds) != 0 {
				t.Fatalf("official program: %+v", ds)
			}
			if ds := program.GetGlobalDiagnostics(t.Context()); len(ds) != 0 {
				t.Fatalf("official global: %+v", ds)
			}
			app := program.GetSourceFile(tc.Input.RootFile)
			if app == nil || len(program.GetSourceFiles()) != 2 || app.Text() != tc.Input.Files[0].Content || program.GetSourceFile(tc.Input.Declarations.Paths[0]).Text() != tc.Input.Files[1].Content {
				t.Fatal("official complete input closure differs")
			}
			var anchor *ast.Node
			var visit func(*ast.Node) bool
			visit = func(node *ast.Node) bool {
				positions := app.GetPositionMap()
				start := scanner.SkipTrivia(app.Text(), node.Pos())
				if strings.TrimPrefix(node.Kind.String(), "Kind") == tc.ExpectedAnchor.Kind && positions.UTF8ToUTF16(start) == tc.ExpectedAnchor.Start && positions.UTF8ToUTF16(node.End()) == tc.ExpectedAnchor.End {
					if anchor != nil {
						t.Fatal("independent first anchor is ambiguous")
					}
					if app.Text()[start:node.End()] != tc.ExpectedAnchor.Spelling {
						t.Fatal("original anchor spelling differs")
					}
					anchor = node
				}
				node.ForEachChild(visit)
				return false
			}
			visit(app.AsNode())
			if anchor == nil {
				t.Fatal("independently selected original first anchor missing")
			}
			func() {
				check, release := program.GetTypeCheckerForFileExclusive(t.Context(), app)
				defer release()
				if tc.Name == "transitive-leaf-helper-op-cycle" {
					var leaf, helper, op *ast.Node
					for _, node := range app.Statements.Nodes {
						if node.Kind == ast.KindFunctionDeclaration {
							if node.Name().Text() == "leaf" {
								leaf = node
							}
							if node.Name().Text() == "helper" {
								helper = node
							}
						}
						if node.Kind == ast.KindVariableStatement {
							for _, decl := range node.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
								if decl.Name().Text() == "op" {
									op = decl
								}
							}
						}
					}
					if leaf == nil || helper == nil || op != anchor {
						t.Fatal("complete original cycle declarations differ")
					}
					leafCall := leaf.Body().AsBlock().Statements.Nodes[0].AsReturnStatement().Expression
					helperCall := helper.Body().AsBlock().Statements.Nodes[0].AsReturnStatement().Expression
					if leafCall.Kind != ast.KindCallExpression || helperCall.Kind != ast.KindCallExpression ||
						check.GetSymbolAtLocation(leafCall.AsCallExpression().Expression) != check.GetSymbolAtLocation(helper.Name()) ||
						check.GetSymbolAtLocation(helperCall.AsCallExpression().Expression) != check.GetSymbolAtLocation(op.Name()) ||
						check.GetSymbolAtLocation(op.AsVariableDeclaration().Initializer) != check.GetSymbolAtLocation(leaf.Name()) ||
						check.GetResolvedSignature(leafCall).Declaration() != helper || check.GetResolvedSignature(helperCall).Declaration() != leaf {
						t.Fatal("official original-Symbol leaf→helper→op→leaf path differs")
					}
				}
			}()
			r := tc.Input
			before, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			r.moduleCallable = true
			calls := 0
			got := analyzeWithProjection(t.Context(), r, func(ownedProjectionContext) { calls++ })
			actual, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("TSFG_M23G_SOURCE_BOUNDARY_ACTUAL name=%s fourOfficialDiagnostics0=true actual=%s callback=%d", tc.Name, actual, calls)
			if !reflect.DeepEqual(got, tc.ExpectedResponse) || calls != tc.ExpectedCallbackCount {
				want, _ := json.Marshal(tc.ExpectedResponse)
				t.Fatalf("complete preselected unsupported DTO differs: got=%s want=%s callback=%d", actual, want, calls)
			}
			after, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("original typed request was mutated")
			}
		})
	}
}
