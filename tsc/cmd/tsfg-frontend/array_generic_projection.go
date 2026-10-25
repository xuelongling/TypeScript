// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/core"
	"slices"
	"strings"
)

type arrayGenericProjection struct {
	common                             *commonProgramProjection
	ctx                                context.Context
	check                              *checker.Checker
	dts                                *ast.SourceFile
	arrayNode, viewNode, intrinsicNode *ast.Node
	scalars                            map[*ast.Symbol]*ast.Node
	classes                            map[*ast.Symbol]string
	types                              map[string]arrayGenericType
	actual                             map[string]*checker.Type
	payload                            arrayGenericPayload
	headers                            map[*ast.Symbol]*arrayGenericHeader
	classNodes                         map[string]*ast.Node
	constructors                       map[string]*arrayGenericHeader
	fields                             map[*ast.Symbol]arrayGenericField
	stringUnits                        int
	resources                          *classResources
	annotationDepth, annotationCalls   int
	annotationMemo                     map[*ast.Node]string
	annotationAliases                  map[*ast.Symbol]bool
	typeKeyBytes                       int
	witnesses                          map[arrayGenericWitnessIdentity]arrayGenericCompatibilityWitness
}

func projectArrayGeneric(ctx context.Context, r request, input arrayFactsInput, common ...*commonProgramProjection) (arrayGenericPayload, error) {
	var empty arrayGenericPayload
	program, _, consumed, failure := newOwnedProgram(r)
	if failure != nil {
		return empty, arrayFactsDiagnostics("input", "official v8 source boundary", failure.Diagnostics)
	}
	// Construction ordering is bound to the actual fixed owned options, not to
	// a downlevel emitter branch or an unrecorded caller option.
	options := program.Options()
	if options.Target != core.ScriptTargetESNext || !options.GetUseDefineForClassFields() || !options.GetEmitStandardClassFields() {
		return empty, arrayFactsError("identity", "v8 construction requires its pinned native ESNext field options", nil)
	}
	if err := arrayFactsOfficialDiagnostics(ctx, program, r.RootFile); err != nil {
		return empty, err
	}
	if len(consumed) != len(input.Files) {
		return empty, arrayFactsError("input", "complete consumed v8 source closure", nil)
	}
	for i, path := range consumed {
		if path != input.Files[i].Path {
			return empty, arrayFactsError("input", "consumed original path differs", nil)
		}
	}
	file := program.GetSourceFile(r.RootFile)
	check, release := program.GetTypeCheckerForFileExclusive(ctx, file)
	defer release()
	p := &arrayGenericProjection{ctx: ctx, check: check, dts: program.GetSourceFile(input.Declarations.Paths[0]), scalars: map[*ast.Symbol]*ast.Node{}, classes: map[*ast.Symbol]string{}, types: map[string]arrayGenericType{}, actual: map[string]*checker.Type{}, headers: map[*ast.Symbol]*arrayGenericHeader{}, fields: map[*ast.Symbol]arrayGenericField{}}
	if len(common) > 1 {
		return empty, arrayFactsError("input", "one common program projection", nil)
	}
	if len(common) == 1 {
		p.common = common[0]
	}
	p.payload = arrayGenericPayload{SourceRoot: input.SourceRoot, RootFile: input.RootFile, PositionUnit: "utf16", Config: input.Config, Declarations: input.Declarations, Files: []arrayFactsOwnedFile{}, Types: []arrayGenericType{}, Functions: []arrayGenericFunction{}, Exports: []arrayFactsExport{}, Diagnostics: []diagnostic{}}
	p.payload.CompatibilityWitnesses = []arrayGenericCompatibilityWitness{}
	p.payload.EntryCompatibilityRoots = []arrayGenericEntryCompatibilityRoots{}
	files := []*ast.SourceFile{}
	nodes := []*ast.Node{}
	count := 0
	for _, f := range input.Files {
		actual := program.GetSourceFile(f.Path)
		if actual == nil || actual.Text() != f.Content {
			return empty, arrayFactsError("identity", "consumed original source content", nil)
		}
		p.payload.Files = append(p.payload.Files, arrayFactsOwnedFile{Path: f.Path, Content: f.Content, SHA256: f.SHA256, ByteLength: len(f.Content), UTF16Length: actual.GetPositionMap().UTF8ToUTF16(len(f.Content))})
		if actual == p.dts {
			continue
		}
		files = append(files, actual)
		var visit func(*ast.Node, int) error
		visit = func(n *ast.Node, depth int) error {
			count++
			if count > 100000 || depth > 64 {
				return arrayFactsError("limit", "original v8 AST budget", nil)
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			var failure error
			n.ForEachChild(func(child *ast.Node) bool { failure = visit(child, depth+1); return failure != nil })
			return failure
		}
		if err := visit(actual.AsNode(), 0); err != nil {
			return empty, err
		}
		for _, node := range actual.Statements.Nodes {
			switch node.Kind {
			case ast.KindClassDeclaration:
				nodes = append(nodes, node)
			case ast.KindFunctionDeclaration, ast.KindImportDeclaration, ast.KindTypeAliasDeclaration:
			case ast.KindVariableStatement, ast.KindExportDeclaration, ast.KindExportAssignment:
				if p.common == nil {
					return empty, p.fail(node, "complete top-level executable source coverage")
				}
			default:
				return empty, p.fail(node, "complete top-level executable source coverage")
			}
		}
	}
	if err := p.origins(); err != nil {
		return empty, err
	}
	p.resources = &classResources{ctx: ctx, maxNodes: 100000, maxDepth: 64, nodes: count}
	if err := p.classHeaders(nodes); err != nil {
		return empty, err
	}
	if err := p.constructorHeaders(nodes); err != nil {
		return empty, err
	}
	ordered := []*arrayGenericHeader{}
	for _, file := range files {
		for _, node := range file.Statements.Nodes {
			if node.Kind != ast.KindFunctionDeclaration {
				continue
			}
			header, err := p.header(node, "")
			if err != nil {
				return empty, err
			}
			p.headers[check.GetSymbolAtLocation(node.Name())] = header
			ordered = append(ordered, header)
		}
	}
	if p.common != nil {
		if err := p.commonHeaders(files); err != nil {
			return empty, err
		}
	}
	if err := p.classBodies(nodes); err != nil {
		return empty, err
	}
	for _, header := range ordered {
		fn, err := p.function(header)
		if err != nil {
			return empty, err
		}
		p.payload.Functions = append(p.payload.Functions, fn)
	}
	if !ast.IsExternalModule(file) || file.AsNode().Symbol() == nil {
		return empty, p.fail(file.AsNode(), "actual module export table required")
	}
	if p.common != nil {
		if err := p.commonModules(files); err != nil {
			return empty, err
		}
	}
	for _, export := range check.GetExportsOfModule(file.AsNode().Symbol()) {
		symbol := export
		if symbol.Flags&ast.SymbolFlagsAlias != 0 {
			symbol = check.GetAliasedSymbol(symbol)
		}
		header := p.headers[symbol]
		if header == nil {
			if p.common != nil {
				continue // The complete typed module export directory owns non-function exports.
			}
			return empty, p.fail(file.AsNode(), "export must join an actual owned function")
		}
		p.payload.Exports = append(p.payload.Exports, arrayFactsExport{Name: export.Name, FunctionID: header.function.ID})
	}
	if len(p.payload.Exports) == 0 {
		return empty, p.fail(file.AsNode(), "at least one owned function export required")
	}
	slices.SortFunc(p.payload.Exports, func(a, b arrayFactsExport) int { return strings.Compare(a.Name, b.Name) })
	if err := p.entryCompatibilityRoots(); err != nil {
		return empty, err
	}
	for _, typ := range p.types {
		p.payload.Types = append(p.payload.Types, typ)
	}
	slices.SortFunc(p.payload.Types, func(a, b arrayGenericType) int { return strings.Compare(a.ID, b.ID) })
	if err := p.finishWitnesses(); err != nil {
		return empty, err
	}
	return p.payload, nil
}
