// SPDX-License-Identifier: Apache-2.0
// Added declaration controls reuse only fresh official-object methods, never production.
package main

import (
	"bytes"
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
	"slices"
	"testing"
)

func arrayFirstV4OriginRecords25(t *testing.T, file, dts *ast.SourceFile, check *checker.Checker) ([]any, ao23Map, ao23Map, ao23Map) {
	t.Helper()
	scope := &rs26Scope{ao23Scope: &ao23Scope{t: t, file: file, dts: dts, check: check, counts: map[string]int{}}, proof: ao23Map{}}
	// origins proves all14 declaration Symbols, scoped generic T, readonly numeric
	// indexer/length and the actual original generic signature; PositionMap/rune
	// anchors are recomputed from original nodes, including the unchanged first3.
	return scope.origins()
}

func arrayFirstV4Origins25(t *testing.T, file, dts *ast.SourceFile, check *checker.Checker, packet arrayFirstPackage25) {
	t.Helper()
	types, length, viewLength, intrinsic := arrayFirstV4OriginRecords25(t, file, dts, check)
	actual := make([]any, 0, len(packet.Payload.Types))
	for _, row := range packet.Payload.Types {
		value := map[string]any{"id": row.ID, "kind": row.Kind, "sourcePath": row.SourcePath, "declarationRange": row.DeclarationRange, "nameRange": row.NameRange}
		if row.Kind == "array" || row.Kind == "readSpan" {
			value["elementType"], value["indexSignatureRange"] = row.ElementType, row.IndexSignatureRange
		}
		if row.Kind == "readSpan" {
			value["readonly"] = row.Readonly
		} else if row.Readonly {
			t.Fatal("old scalar/Array origin became readonly")
		}
		actual = append(actual, value)
	}
	for _, pair := range []struct{ actual, official any }{
		{actual, types}, {packet.Payload.ArrayLength, length},
		{packet.Payload.ReadSpanLength, viewLength}, {packet.Payload.ReadSpanIntrinsic, intrinsic},
	} {
		if !bytes.Equal(arrayFirstCanonical25(t, pair.actual), arrayFirstCanonical25(t, pair.official)) {
			t.Fatal("complete owned v4 declaration metadata differs from actual original official objects")
		}
	}
	t.Logf("TSFG_ARRAY_FACTS_V4_ORIGINS_PASS declarations=14 types=4 readonly_indexer_length=true generic_original_signature=true")
}

func arrayDepthV4OfficialOrigins26(t *testing.T, r request, payload map[string]any) {
	t.Helper()
	contents, paths := map[string]string{}, []string{}
	for _, file := range r.Files {
		contents[file.Path] = file.Content
		paths = append(paths, file.Path)
	}
	slices.Sort(paths)
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{FileNames: paths, CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext}}},
		Host:   compiler.NewCompilerHost(r.SourceRoot, vfstest.FromMap(contents, true), r.SourceRoot, nil, nil), SingleThreaded: core.TSTrue,
	})
	for _, group := range []struct {
		name  string
		items []*ast.Diagnostic
	}{
		{"syntax", program.GetSyntacticDiagnostics(t.Context(), nil)}, {"semantic", program.GetSemanticDiagnostics(t.Context(), nil)},
		{"program", program.GetProgramDiagnostics()}, {"global", program.GetGlobalDiagnostics(t.Context())},
	} {
		if len(group.items) != 0 {
			t.Fatalf("fresh depth v4 official %s diagnostics: %+v", group.name, group.items)
		}
	}
	consumed := []string{}
	for _, file := range program.GetSourceFiles() {
		if contents[file.FileName()] != file.Text() {
			t.Fatal("complete depth source content")
		}
		consumed = append(consumed, file.FileName())
	}
	slices.Sort(consumed)
	if !slices.Equal(consumed, paths) {
		t.Fatal("complete original depth two-file closure")
	}
	file, dts := program.GetSourceFile(r.RootFile), program.GetSourceFile(r.Declarations.Paths[0])
	if file == nil || dts == nil || len(dts.Statements.Nodes) != 14 || !ast.IsExternalModule(file) {
		t.Fatal("actual depth module and full14 DTS declarations")
	}
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	types, length, viewLength, intrinsic := arrayFirstV4OriginRecords25(t, file, dts, check)
	for _, pair := range []struct{ actual, official any }{
		{payload["types"], types}, {payload["arrayLength"], length},
		{payload["readSpanLength"], viewLength}, {payload["readSpanIntrinsic"], intrinsic},
	} {
		if !bytes.Equal(arrayFirstCanonical25(t, pair.actual), arrayFirstCanonical25(t, pair.official)) {
			t.Fatal("whole depth4 origins differ from actual official objects")
		}
	}
}
