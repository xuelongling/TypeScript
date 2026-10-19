// SPDX-License-Identifier: Apache-2.0
// Complete old source bodies are checked using fresh official objects and the new pinned DTS.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
	"slices"
	"testing"
)

func rsOld26Program(t *testing.T, sourceRoot, sourcePath, dtsPath, source, dts string) (*rs26Scope, func()) {
	contents := map[string]string{sourcePath: source, dtsPath: dts}
	paths := []string{sourcePath, dtsPath}
	slices.Sort(paths)
	p := compiler.NewProgram(compiler.ProgramOptions{Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{FileNames: paths, CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext}}}, Host: compiler.NewCompilerHost(sourceRoot, vfstest.FromMap(contents, true), sourceRoot, nil, nil), SingleThreaded: core.TSTrue})
	for _, g := range []struct {
		name  string
		items []*ast.Diagnostic
	}{{"syntax", p.GetSyntacticDiagnostics(t.Context(), nil)}, {"semantic", p.GetSemanticDiagnostics(t.Context(), nil)}, {"program", p.GetProgramDiagnostics()}, {"global", p.GetGlobalDiagnostics(t.Context())}} {
		if len(g.items) != 0 {
			t.Fatalf("fresh official %s diagnostics: %+v", g.name, g.items)
		}
	}
	consumed := []string{}
	for _, f := range p.GetSourceFiles() {
		ao23Want(t, contents[f.FileName()] == f.Text(), "complete unchanged original text")
		consumed = append(consumed, f.FileName())
	}
	slices.Sort(consumed)
	ao23Want(t, slices.Equal(paths, consumed), "exact original root plus new pinned DTS closure")
	file, decls := p.GetSourceFile(sourcePath), p.GetSourceFile(dtsPath)
	ao23Want(t, file != nil && decls != nil && ast.IsExternalModule(file), "complete original external module")
	check, release := p.GetTypeCheckerForFileExclusive(t.Context(), file)
	base := &ao23Scope{t: t, file: file, dts: decls, check: check, headers: map[*ast.Symbol]*ao23Header{}, names: map[string]*ao23Header{}, counts: map[string]int{}}
	return &rs26Scope{ao23Scope: base, views: map[*ast.Symbol]string{}, proof: ao23Map{"diagnostics": ao23Map{"syntax": 0, "semantic": 0, "program": 0, "global": 0}, "consumedPaths": paths}}, release
}

// Only an exact declaration-derived property identity may change inside an old body.
func rsOld26Property(value any, oldID, newID string) any {
	switch v := value.(type) {
	case string:
		if v == oldID {
			return newID
		}
		return v
	case []any:
		out := make([]any, len(v))
		for i, x := range v {
			out[i] = rsOld26Property(x, oldID, newID)
		}
		return out
	case map[string]any:
		out := ao23Map{}
		for k, x := range v {
			out[k] = rsOld26Property(x, oldID, newID)
		}
		return out
	default:
		return value
	}
}
func rsOld26RangeRoles(value any) int {
	switch v := value.(type) {
	case ao23Range:
		return 1
	case []any:
		n := 0
		for _, x := range v {
			n += rsOld26RangeRoles(x)
		}
		return n
	case []ao23Map:
		n := 0
		for _, x := range v {
			n += rsOld26RangeRoles(x)
		}
		return n
	case map[string]any:
		if len(v) == 2 && v["start"] != nil && v["end"] != nil {
			return 1
		}
		n := 0
		for _, x := range v {
			n += rsOld26RangeRoles(x)
		}
		return n
	default:
		return 0
	}
}
