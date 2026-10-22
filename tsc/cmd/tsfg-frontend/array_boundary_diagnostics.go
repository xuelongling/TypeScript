// SPDX-License-Identifier: Apache-2.0
// Original official codes/coordinates precede finite private capability decisions.
package main

import (
	"context"
	"slices"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/scanner"
)

func arrayFactsDiagnostics(kind, detail string, items []diagnostic) *arrayFactsFailure {
	failure := arrayFactsError(kind, detail, nil)
	failure.Diagnostics = slices.Clone(items)
	for i := range failure.Diagnostics {
		failure.Diagnostics[i].RuleIDs = slices.Clone(failure.Diagnostics[i].RuleIDs)
	}
	if len(items) > 0 {
		failure.Code, failure.RuleID, failure.Phase = items[0].Code, items[0].RuleID, items[0].Phase
	}
	return failure
}
func arrayFactsCapability(file *ast.SourceFile, node *ast.Node, detail string) error {
	if node == nil {
		node = file.AsNode()
	}
	where := expressionOffset(file, node)
	item := diagnostic{Code: "r03-array-facts-capability", RuleID: "r01-ts-baseline", RuleIDs: []string{"r01-ts-baseline"}, Category: "unsupported", Phase: "capability", Severity: "error", File: file.FileName(), Range: where, Primary: sourcePosition(file, scanner.SkipTrivia(file.Text(), node.Pos()), node.End())}
	return arrayFactsDiagnostics("capability", detail, []diagnostic{item})
}
func arrayFactsDensity(file *ast.SourceFile, node *ast.Node, detail string) error {
	where := expressionOffset(file, node)
	item := diagnostic{Code: "r03-array-facts-density", RuleID: "r01-ts-baseline", RuleIDs: []string{"r01-ts-baseline"}, Category: "error", Phase: "semantic", Severity: "error", File: file.FileName(), Range: where, Primary: sourcePosition(file, scanner.SkipTrivia(file.Text(), node.Pos()), node.End())}
	return arrayFactsDiagnostics("density", detail, []diagnostic{item})
}
func arrayFactsOfficialDiagnostics(ctx context.Context, program *compiler.Program, root string) error {
	groups := []struct {
		items []*ast.Diagnostic
		phase string
	}{{program.GetSyntacticDiagnostics(ctx, nil), "syntax"}, {program.GetSemanticDiagnostics(ctx, nil), "types"}, {program.GetProgramDiagnostics(), "types"}, {program.GetGlobalDiagnostics(ctx), "types"}}
	if err := ctx.Err(); err != nil {
		return arrayFactsError("canceled", "official diagnostics", err)
	}
	items := []diagnostic{}
	for _, group := range groups {
		for _, upstream := range group.items {
			name, where, location := root, span{}, primary{SourceID: root}
			if file := upstream.File(); file != nil {
				name = file.FileName()
				positions := file.GetPositionMap()
				where = span{positions.UTF8ToUTF16(upstream.Pos()), positions.UTF8ToUTF16(upstream.End())}
				location = sourcePosition(file, upstream.Pos(), upstream.End())
			}
			items = append(items, diagnostic{Code: "r03-array-facts-" + group.phase, RuleID: "r01-ts-baseline", RuleIDs: []string{"r01-ts-baseline"}, Category: "error", Phase: group.phase, Severity: "error", File: name, Range: where, Primary: location, UpstreamCode: upstream.Code()})
		}
	}
	if len(items) == 0 {
		return nil
	}
	slices.SortStableFunc(items, func(a, b diagnostic) int {
		if n := strings.Compare(a.File, b.File); n != 0 {
			return n
		}
		if a.Range.Start != b.Range.Start {
			return a.Range.Start - b.Range.Start
		}
		if n := strings.Compare(a.Phase, b.Phase); n != 0 {
			return n
		}
		if a.Range.End != b.Range.End {
			return a.Range.End - b.Range.End
		}
		return int(a.UpstreamCode - b.UpstreamCode)
	})
	return arrayFactsDiagnostics("diagnostic", "complete official diagnostics", items)
}
