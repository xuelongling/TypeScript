// SPDX-License-Identifier: Apache-2.0

package main

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
)

//go:embed testdata/module-callable-facts-v1/star-scenario.json
var mcStarScenario23d []byte

// Source-order export-star custody is separate from the original eight cases.
// Both paths resolve the same actual source declaration. A same-target second
// site must never replace the first original resolving star's provenance.
func TestModuleCallable23dIndependentStarOracle(t *testing.T) {
	var fixture struct{ Case mcCase23d }
	if err := json.Unmarshal(mcStarScenario23d, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Case.Name != "first-resolving-star" || len(fixture.Case.Recipe) != 5 {
		t.Fatal("complete independent five-source star case required")
	}
	var original struct{ Cases []mcCase23d }
	if err := json.Unmarshal(moduleCallableScenarios23d, &original); err != nil {
		t.Fatal(err)
	}
	var first, star struct{ Input mcInput23d }
	if err := json.Unmarshal(original.Cases[0].Request, &first); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(fixture.Case.Request, &star); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/src/app.ts", "/src/state.ts"} {
		oldText, newText := "", ""
		for _, file := range first.Input.Files {
			if file.Path == path {
				oldText = file.Content
			}
		}
		for _, file := range star.Input.Files {
			if file.Path == path {
				newText = file.Content
			}
		}
		if oldText == "" || oldText != newText {
			t.Fatalf("original first-case source changed at %s", path)
		}
	}
	mcGenerateIndependent23d(t, fixture.Case)
}

func (o *mcIndependent23d) firstResolvingStar(file *ast.SourceFile, name string, original *ast.Symbol, wantCount int) *ast.Node {
	var first *ast.Node
	count := 0
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindExportDeclaration {
			continue
		}
		declaration := node.AsExportDeclaration()
		if declaration.ExportClause != nil || declaration.ModuleSpecifier == nil || declaration.IsTypeOnly {
			continue
		}
		module := o.check.ResolveExternalModuleName(declaration.ModuleSpecifier)
		if module == nil {
			o.t.Fatal("independent original star module missing")
		}
		for _, export := range o.check.GetExportsOfModule(module) {
			if export.Name == name && o.original(export) == original {
				if first == nil {
					first = node
				}
				count++
				break
			}
		}
	}
	if first == nil || count != wantCount {
		o.t.Fatalf("independent resolving star coverage differs: name=%s actual=%d expected=%d", name, count, wantCount)
	}
	o.t.Logf("TSFG_MODULE_CALLABLE_23D_INDEPENDENT_FIRST_STAR name=%s source=%s resolvingSites=%d range=%v", name, file.FileName(), count, o.anchor(file, first, true))
	return first
}
