// SPDX-License-Identifier: Apache-2.0
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"os"
	"strings"
	"testing"
)

func systemRequest38c(t *testing.T) request {
	t.Helper()
	path := os.Getenv("TSFG_QUERY38_SOURCE")
	if path == "" {
		path = "testdata/system-query-v2/declarations-only.ts"
	}
	source, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	r := request{SourceRoot: "/src", RootFile: "/src/main.ts"}
	r.Config.Strict, r.Config.NoEmit = true, true
	r.Declarations.Version = "minimal-arrays-v8"
	r.Declarations.Paths = []string{"/src/minimal-arrays-v8.d.ts", systemQueryProfilePath}
	r.Files = []inputFile{{Path: r.RootFile, Content: string(source)}, {Path: "/src/minimal-arrays-v8.d.ts", Content: string(arrayGenericDeclarations)}, {Path: systemQueryProfilePath, Content: systemQueryDeclarations}}
	return r
}
func systemDiagnosticCodes38c(t *testing.T, p *compiler.Program) []int {
	t.Helper()
	groups := [][]*ast.Diagnostic{p.GetSyntacticDiagnostics(t.Context(), nil), p.GetSemanticDiagnostics(t.Context(), nil), p.GetProgramDiagnostics(), p.GetGlobalDiagnostics(t.Context())}
	codes := []int{}
	for i, group := range groups {
		t.Logf("official group %d count=%d", i, len(group))
		for _, d := range group {
			t.Logf("official code=%d pos=%d end=%d %s", d.Code(), d.Pos(), d.End(), d.String())
			codes = append(codes, int(d.Code()))
		}
	}
	return codes
}
func TestSystemQueryArchitecture38c(t *testing.T) {
	r := systemRequest38c(t)
	p, _, consumed, failure := newOwnedSystemProgram(r)
	if failure != nil {
		t.Fatal(failure)
	}
	if len(consumed) != 3 {
		t.Fatal("complete Program inventory differs", consumed)
	}
	if codes := systemDiagnosticCodes38c(t, p); len(codes) != 0 {
		t.Fatal("architecture requires all four official diagnostics0", codes)
	}
}
func TestSystemQueryOriginalContext38c(t *testing.T) {
	for _, item := range []struct {
		name, source string
		owned        bool
		code         int
	}{{"ordinary Program", "", false, 1206}, {"shadowed marker", "function system(target:unknown):unknown{return target;}\n", true, 1206}, {"foreign marker", "declare function other(target:unknown):unknown;\n", true, 1206}, {"body remains checked", "badbody", true, 2322}} {
		t.Run(item.name, func(t *testing.T) {
			r := systemRequest38c(t)
			if item.source == "badbody" {
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "void {}", "void {dt='bad';}", 1)
			} else if item.source != "" {
				r.Files[0].Content = strings.ReplaceAll(item.source, "\\n", "\n") + r.Files[0].Content
				if item.name == "foreign marker" {
					r.Files[0].Content = strings.Replace(r.Files[0].Content, "@system", "@other", 1)
				}
			}
			var p *compiler.Program
			var failure *response
			if item.owned {
				p, _, _, failure = newOwnedSystemProgram(r)
			} else {
				p, _, _, failure = newOwnedProgram(r)
			}
			if failure != nil {
				t.Fatal(failure)
			}
			found := false
			for _, code := range systemDiagnosticCodes38c(t, p) {
				if code == item.code {
					found = true
				}
			}
			if !found {
				t.Fatal("required original diagnostic absent", item.code)
			}
		})
	}
	r := systemRequest38c(t)
	r.Files[2].Content += "declare const foreign: unknown;"
	if _, _, _, failure := newOwnedSystemProgram(r); failure == nil {
		t.Fatal("foreign context admitted")
	}
}
