// SPDX-License-Identifier: Apache-2.0
// Fresh official objects for exact adjacent inputs; no Source/Product producer is called.
package main

import (
	"slices"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/scanner"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

type rs26bAdjacentCase struct {
	ID, Text, SourceRoot, SourcePath, DeclarationPath, SHA256 string
	Bytes, UTF16Length                                        int
	AnchorCandidates                                          map[string]struct {
		SourcePath, SourceText string
		Range                  ao23Range
	}
	SourceExpected struct {
		Outcome, Code, RuleID, Phase, Category, Severity, PrimaryAnchor string
		ExpectedFactsWrites                                             int
	}
}

func rs26bAdjacentProgram(t *testing.T, item rs26bAdjacentCase, dts string) (*rs26Scope, func()) {
	contents := map[string]string{item.SourcePath: item.Text, item.DeclarationPath: dts}
	paths := []string{item.SourcePath, item.DeclarationPath}
	slices.Sort(paths)
	p := compiler.NewProgram(compiler.ProgramOptions{Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{FileNames: paths, CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext}}}, Host: compiler.NewCompilerHost(item.SourceRoot, vfstest.FromMap(contents, true), item.SourceRoot, nil, nil), SingleThreaded: core.TSTrue})
	groups, counts, total := ao23Map{}, ao23Map{}, 0
	for _, g := range []struct {
		name  string
		items []*ast.Diagnostic
	}{{"syntax", p.GetSyntacticDiagnostics(t.Context(), nil)}, {"semantic", p.GetSemanticDiagnostics(t.Context(), nil)}, {"program", p.GetProgramDiagnostics()}, {"global", p.GetGlobalDiagnostics(t.Context())}} {
		groups[g.name], counts[g.name] = rs26bDiagnostics(g.items), len(g.items)
		total += len(g.items)
	}
	t.Logf("TSFG_READSPAN_26B_ADJACENT_OFFICIAL_DIAGNOSTICS name=%s json=%s", item.ID, ao23JCS(t, groups))
	ao23Want(t, total == 0, "complete fresh official four diagnostic groups are empty")
	consumed, files := []string{}, []any{}
	for _, f := range p.GetSourceFiles() {
		text, ok := contents[f.FileName()]
		ao23Want(t, ok && text == f.Text(), "exact original adjacent text consumed")
		consumed = append(consumed, f.FileName())
		files = append(files, ao23Map{"path": f.FileName(), "content": f.Text(), "sha256": ao23Hash([]byte(f.Text())), "byteLength": len(f.Text()), "utf16Length": ao23Units(f.Text())})
	}
	slices.Sort(consumed)
	ao23Want(t, slices.Equal(paths, consumed), "complete unique original adjacent root and pinned DTS closure")
	file, decls := p.GetSourceFile(item.SourcePath), p.GetSourceFile(item.DeclarationPath)
	ao23Want(t, file != nil && decls != nil && ast.IsExternalModule(file), "original complete adjacent external module")
	check, release := p.GetTypeCheckerForFileExclusive(t.Context(), file)
	base := &ao23Scope{t: t, file: file, dts: decls, check: check, headers: map[*ast.Symbol]*ao23Header{}, names: map[string]*ao23Header{}, counts: map[string]int{}}
	return &rs26Scope{ao23Scope: base, views: map[*ast.Symbol]string{}, proof: ao23Map{"diagnostics": counts, "diagnosticGroups": groups, "consumedPaths": consumed, "consumedFiles": files}}, release
}

func (s *rs26Scope) rs26bAdjacentAnchors(item rs26bAdjacentCase) {
	anchors := ao23Map{}
	for label, want := range item.AnchorCandidates {
		ao23Want(s.t, want.SourcePath == s.file.FileName() && want.Range.Start >= 0 && want.Range.Start < want.Range.End && want.Range.End <= len(item.Text) && item.Text[want.Range.Start:want.Range.End] == want.SourceText, "complete preauthored ASCII token candidate")
		kinds := []string{}
		var visit func(*ast.Node) bool
		visit = func(n *ast.Node) bool {
			start := scanner.SkipTrivia(item.Text, n.Pos())
			if start < n.End() && s.span(s.file, n) == want.Range {
				kinds = append(kinds, n.Kind.String())
			}
			n.ForEachChild(visit)
			return false
		}
		visit(s.file.AsNode())
		ao23Want(s.t, len(kinds) > 0, "every preauthored token range equals at least one actual original AST node")
		anchors[label] = ao23Map{"sourcePath": want.SourcePath, "range": want.Range, "sourceText": want.SourceText, "originalNodeKinds": kinds}
	}
	s.proof["literalAnchors"] = anchors
}
