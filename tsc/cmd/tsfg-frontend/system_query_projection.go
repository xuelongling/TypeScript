// SPDX-License-Identifier: Apache-2.0
// Supplemental original Program projection; no HIR or loop lowering is implied.
package main

import (
	"context"
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"slices"
	"strings"
)

func systemOrigin(node *ast.Node) systemQueryOrigin {
	path := ast.GetSourceFileOfNode(node).FileName()
	name := node.Name()
	r := *genericSpan(node)
	nr := r
	text := ""
	if name != nil {
		nr = *genericSpan(name)
		text = name.Text()
	}
	id, _ := arrayFactsBindingID(path, nr)
	return systemQueryOrigin{id, path, node.Kind.String(), text, r, nr}
}
func systemTypeSymbol(t *checker.Type) *ast.Symbol {
	if t == nil || t.Flags()&checker.TypeFlagsObject == 0 {
		return nil
	}
	if target := t.Target(); target != nil {
		return target.Symbol()
	}
	return t.Symbol()
}
func systemInventory(node *ast.Node, parent string, out *[]systemQueryNode, count *int) error {
	*count++
	if *count > 100000 {
		return arrayFactsError("limit", "system original AST work", nil)
	}
	path := ast.GetSourceFileOfNode(node).FileName()
	r := *genericSpan(node)
	seed := struct {
		SourcePath string `json:"sourcePath"`
		Kind       string `json:"kind"`
		Range      span   `json:"range"`
		Parent     string `json:"parent"`
	}{path, node.Kind.String(), r, parent}
	id, e := arrayFactsID("tsfg.system-query.node.v1\x00", seed)
	if e != nil {
		return e
	}
	name := ""
	var nr *span
	switch node.Kind {
	case ast.KindClassDeclaration, ast.KindFunctionDeclaration, ast.KindInterfaceDeclaration, ast.KindTypeAliasDeclaration, ast.KindVariableDeclaration, ast.KindParameter, ast.KindPropertyDeclaration, ast.KindMethodDeclaration, ast.KindTypeParameter:
		if node.Name() != nil {
			name = node.Name().Text()
			nr = genericSpan(node.Name())
		}
	}
	*out = append(*out, systemQueryNode{id, parent, path, node.Kind.String(), r, name, nr})
	var failure error
	node.ForEachChild(func(child *ast.Node) bool { failure = systemInventory(child, id, out, count); return failure != nil })
	return failure
}
func projectSystemQueries(ctx context.Context, r request) (systemQueryPayload, error) {
	var empty systemQueryPayload
	if !validIntegerTarget(r.Target) {
		return empty, arrayFactsError("input", "system target required", nil)
	}
	pgr, root, consumed, failure := newOwnedSystemProgram(r)
	if failure != nil {
		return empty, arrayFactsDiagnostics("input", "owned system profile", failure.Diagnostics)
	}
	if err := arrayFactsOfficialDiagnostics(ctx, pgr, r.RootFile); err != nil {
		return empty, err
	}
	if len(consumed) != len(r.Files) {
		return empty, arrayFactsError("identity", "whole original system Program inventory", nil)
	}
	check, release := pgr.GetTypeCheckerForFileExclusive(ctx, root)
	defer release()
	profile := pgr.GetSourceFile(systemQueryProfilePath)
	builtins := map[string]*ast.Node{}
	for _, node := range profile.Statements.Nodes {
		if node.Kind == ast.KindVariableStatement {
			for _, decl := range node.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
				builtins[decl.Name().Text()] = decl
			}
		} else if node.Name() != nil {
			builtins[node.Name().Text()] = node
		}
	}
	for _, name := range []string{"component", "system", "Query", "Read", "Write"} {
		if builtins[name] == nil {
			return empty, arrayFactsError("identity", "complete owned system builtin context", nil)
		}
	}
	p := &arrayGenericProjection{ctx: ctx, check: check, dts: pgr.GetSourceFile("/src/minimal-arrays-v8.d.ts"), scalars: map[*ast.Symbol]*ast.Node{}, classes: map[*ast.Symbol]string{}, types: map[string]arrayGenericType{}, actual: map[string]*checker.Type{}, headers: map[*ast.Symbol]*arrayGenericHeader{}, fields: map[*ast.Symbol]arrayGenericField{}, common: &commonProgramProjection{}, resources: &classResources{ctx: ctx, maxNodes: 100000, maxDepth: 64}}
	p.systemComponent = check.GetSymbolAtLocation(builtins["component"].Name())
	p.payload.Functions = []arrayGenericFunction{}
	p.payload.Files = []arrayFactsOwnedFile{}
	out := systemQueryPayload{Program: systemQueryProgram{SourceRoot: r.SourceRoot, RootFile: r.RootFile, PositionUnit: "utf16", Config: arrayFactsConfig{Strict: true, NoEmit: true, AllowJS: false}, Declarations: arrayFactsDeclarations{Version: r.Declarations.Version, Paths: append([]string{}, r.Declarations.Paths...)}, Target: &commonProgramTarget{Target: r.Target.Target, Triple: r.Target.Triple, LLVMDataLayout: r.Target.LLVMDataLayout, PointerBits: r.Target.PointerBits}, Files: []arrayFactsOwnedFile{}, Diagnostics: []diagnostic{}}, ProfilePath: systemQueryProfilePath, ProfileSHA256: arrayFactsSHA([]byte(systemQueryDeclarations)), Builtins: []systemQueryOrigin{}, Nodes: []systemQueryNode{}, Types: []arrayGenericType{}, Functions: []arrayGenericFunction{}, MarkerAliases: []systemQueryMarkerAlias{}, Systems: []systemQueryFunction{}}
	for _, name := range []string{"component", "system", "Query", "Read", "Write"} {
		out.Builtins = append(out.Builtins, systemOrigin(builtins[name]))
	}
	files := append([]inputFile{}, r.Files...)
	slices.SortFunc(files, func(a, b inputFile) int { return strings.Compare(a.Path, b.Path) })
	classes := []*ast.Node{}
	functions := []*ast.Node{}
	count := 0
	for _, file := range files {
		actual := pgr.GetSourceFile(file.Path)
		if actual == nil || actual.Text() != file.Content {
			return empty, arrayFactsError("identity", "complete original file differs", nil)
		}
		owned := arrayFactsOwnedFile{Path: file.Path, Content: file.Content, SHA256: arrayFactsSHA([]byte(file.Content)), ByteLength: len(file.Content), UTF16Length: actual.GetPositionMap().UTF8ToUTF16(len(file.Content))}
		out.Program.Files = append(out.Program.Files, owned)
		p.payload.Files = append(p.payload.Files, owned)
		if err := systemInventory(actual.AsNode(), "", &out.Nodes, &count); err != nil {
			return empty, err
		}
		if strings.HasSuffix(file.Path, ".d.ts") {
			continue
		}
		for _, node := range actual.Statements.Nodes {
			if node.Kind == ast.KindClassDeclaration {
				classes = append(classes, node)
			}
			if node.Kind == ast.KindFunctionDeclaration {
				functions = append(functions, node)
			}
		}
	}
	if err := p.origins(); err != nil {
		return empty, err
	}
	if err := p.classHeaders(classes); err != nil {
		return empty, err
	}
	if err := p.constructorHeaders(classes); err != nil {
		return empty, err
	}
	if err := p.classBodies(classes); err != nil {
		return empty, err
	}
	for _, node := range classes {
		for _, mod := range node.ModifierNodes() {
			if mod.Kind != ast.KindDecorator {
				continue
			}
			expr := mod.AsDecorator().Expression
			if check.GetSymbolAtLocation(expr) == p.systemComponent {
				calls := check.GetSignaturesOfType(check.GetTypeOfSymbolAtLocation(p.systemComponent, expr), checker.SignatureKindCall)
				out.MarkerAliases = append(out.MarkerAliases, systemQueryMarkerAlias{ast.GetSourceFileOfNode(node).FileName(), *genericSpan(mod), systemOrigin(builtins["component"]), systemOrigin(calls[0].Declaration())})
			}
		}
	}
	for _, node := range functions {
		var marker *ast.Node
		for _, mod := range node.ModifierNodes() {
			if mod.Kind == ast.KindDecorator {
				symbol := check.GetSymbolAtLocation(mod.AsDecorator().Expression)
				if symbol != nil && symbol.Flags&ast.SymbolFlagsAlias != 0 {
					symbol = check.GetAliasedSymbol(symbol)
				}
				if symbol == check.GetSymbolAtLocation(builtins["system"].Name()) {
					if marker != nil {
						return empty, p.fail(mod, "ambiguous duplicate system marker")
					}
					marker = mod
				}
			}
		}
		if marker == nil {
			continue
		}
		system, e := p.systemFunction(node, marker, builtins)
		if e != nil {
			return empty, e
		}
		out.Systems = append(out.Systems, system)
	}
	for _, row := range p.types {
		out.Types = append(out.Types, row)
	}
	slices.SortFunc(out.Types, func(a, b arrayGenericType) int { return strings.Compare(a.ID, b.ID) })
	out.Functions = p.payload.Functions
	return out, nil
}
func (p *arrayGenericProjection) systemFunction(node, marker *ast.Node, builtins map[string]*ast.Node) (systemQueryFunction, error) {
	empty := systemQueryFunction{}
	symbol := p.check.GetSymbolAtLocation(node.Name())
	if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != node {
		return empty, p.fail(node, "unique original system function required")
	}
	calls := p.check.GetSignaturesOfType(p.check.GetTypeOfSymbolAtLocation(symbol, node), checker.SignatureKindCall)
	if len(calls) != 1 || calls[0].Declaration() != node {
		return empty, p.fail(node, "unique original system selected signature required")
	}
	decl := node.AsFunctionDeclaration()
	if decl.Type == nil || decl.Parameters == nil || len(calls[0].Parameters()) != len(decl.Parameters.Nodes) {
		return empty, p.fail(node, "system original parameter inventory")
	}
	returns, e := p.annotation(decl.Type)
	if e != nil {
		return empty, e
	}
	out := systemQueryFunction{Declaration: systemOrigin(node), MarkerRange: *genericSpan(marker), Marker: systemOrigin(builtins["system"]), Signature: systemOrigin(calls[0].Declaration()), Parameters: []systemQueryParameter{}, ReturnType: returns}
	queries := 0
	for index, param := range decl.Parameters.Nodes {
		pd := param.AsParameterDeclaration()
		if param.Name() == nil || param.Name().Kind != ast.KindIdentifier || pd.Type == nil {
			return empty, p.fail(param, "original named system parameter required")
		}
		actualSymbol := p.check.GetSymbolAtLocation(param.Name())
		if actualSymbol != calls[0].Parameters()[index] || len(actualSymbol.Declarations) != 1 || actualSymbol.Declarations[0] != param {
			return empty, p.fail(param, "selected parameter original Symbol differs")
		}
		actual := p.check.GetTypeFromTypeNode(pd.Type)
		row := systemQueryParameter{Index: index, Binding: systemOrigin(param), AnnotationRange: *genericSpan(pd.Type)}
		if systemTypeSymbol(actual) == p.check.GetSymbolAtLocation(builtins["Query"].Name()) {
			query, e := p.systemAccess(pd.Type, actual, builtins)
			if e != nil {
				return empty, e
			}
			row.Query = &query
			row.LogicalType = "Query"
			queries++
		} else {
			logical, e := p.annotation(pd.Type)
			if e != nil {
				return empty, e
			}
			row.LogicalType = logical
		}
		out.Parameters = append(out.Parameters, row)
	}
	if queries == 0 {
		return empty, p.fail(node, "system must retain an original Query parameter")
	}
	return out, nil
}
func (p *arrayGenericProjection) systemAccess(annotation *ast.Node, actual *checker.Type, builtins map[string]*ast.Node) (systemQueryParameterAccess, error) {
	empty := systemQueryParameterAccess{}
	args := p.check.GetTypeArguments(actual)
	if len(args) != 1 || !checker.IsTupleType(args[0]) {
		return empty, p.fail(annotation, "Query requires original fixed tuple")
	}
	elements := p.check.GetTypeArguments(args[0])
	if len(elements) == 0 || len(elements) > 256 || args[0].TargetTupleType().FixedLength() != len(elements) {
		return empty, p.fail(annotation, "closed Query tuple required")
	}

	traces := []systemQueryAlias{}
	syntax, env, e := p.systemResolve(annotation, map[*ast.Symbol]*ast.Node{}, &traces, 0)
	if e != nil {
		return empty, e
	}
	if syntax.Kind != ast.KindTypeReference || syntax.AsTypeReferenceNode().TypeArguments == nil {
		return empty, p.fail(annotation, "original Query annotation required")
	}
	tuple, tupleEnv, e := p.systemResolve(syntax.AsTypeReferenceNode().TypeArguments.Nodes[0], env, &traces, 0)
	if e != nil {
		return empty, e
	}
	if tuple.Kind != ast.KindTupleType {
		return empty, p.fail(tuple, "original Query tuple required")
	}
	nodes := tuple.AsTupleTypeNode().Elements.Nodes
	if len(nodes) != len(elements) {
		return empty, p.fail(tuple, "original tuple inventory differs")
	}
	out := systemQueryParameterAccess{Declaration: systemOrigin(builtins["Query"]), TupleSourcePath: ast.GetSourceFileOfNode(tuple).FileName(), TupleRange: *genericSpan(tuple), Aliases: traces, Items: []systemQueryAccess{}}
	for index, element := range elements {
		owner := systemTypeSymbol(element)
		mode := ""
		var wrapper *ast.Node
		for _, name := range []string{"Read", "Write"} {
			if owner == p.check.GetSymbolAtLocation(builtins[name].Name()) {
				mode = strings.ToLower(name)
				wrapper = builtins[name]
			}
		}
		if mode == "" {
			return empty, p.fail(nodes[index], "access must resolve exact owned Read or Write declaration")
		}
		arguments := p.check.GetTypeArguments(element)
		if len(arguments) != 1 {
			return empty, p.fail(nodes[index], "one actual component type required")
		}
		id := p.classes[arguments[0].Symbol()]
		typ := p.types[id]
		if id == "" || typ.NominalKey == nil || typ.NominalKey.Category != "component" {
			return empty, p.fail(nodes[index], "Query access requires owned component")
		}

		if e := p.systemFixed(id, 0, map[string]bool{}, map[string]int{}); e != nil {
			return empty, e
		}
		accessNode := nodes[index]
		resolved, wrapperEnv, e := p.systemResolve(accessNode, tupleEnv, &out.Aliases, 0)
		if e != nil {
			return empty, e
		}
		if resolved.Kind != ast.KindTypeReference || resolved.AsTypeReferenceNode().TypeArguments == nil {
			return empty, p.fail(accessNode, "original Read/Write arguments required")
		}
		componentNode, _, e := p.systemResolve(resolved.AsTypeReferenceNode().TypeArguments.Nodes[0], wrapperEnv, &out.Aliases, 0)
		if e != nil {
			return empty, e
		}
		if componentNode.Kind != ast.KindTypeReference {
			return empty, p.fail(componentNode, "original concrete component reference required")
		}
		componentSymbol := p.commonOriginalSymbol(p.check.GetSymbolAtLocation(componentNode.AsTypeReferenceNode().TypeName))
		if componentSymbol != arguments[0].Symbol() {
			return empty, p.fail(componentNode, "original concrete component alias target differs")
		}
		out.Items = append(out.Items, systemQueryAccess{Index: index, Mode: mode, Range: *genericSpan(accessNode), Wrapper: systemOrigin(wrapper), ComponentType: id, Component: systemOrigin(p.classNodes[id]), ComponentSourcePath: ast.GetSourceFileOfNode(componentNode).FileName(), ComponentRange: *genericSpan(componentNode)})

	}
	return out, nil
}
