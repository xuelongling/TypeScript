// SPDX-License-Identifier: Apache-2.0
// View permission comes from the original owned wrapper, not erased TS aliases.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"strings"
)

func (p *arrayGenericProjection) systemView(annotation *ast.Node, env map[*ast.Symbol]*ast.Node) (systemQueryView, error) {
	empty := systemQueryView{}
	traces := []systemQueryAlias{}
	resolved, wrapperEnv, e := p.systemResolve(annotation, env, &traces, 0)
	if e != nil {
		return empty, e
	}
	if resolved.Kind != ast.KindTypeReference || resolved.AsTypeReferenceNode().TypeArguments == nil || len(resolved.AsTypeReferenceNode().TypeArguments.Nodes) != 1 {
		return empty, p.fail(annotation, "original Read/Write arguments required")
	}
	reference := resolved.AsTypeReferenceNode()
	symbol := p.commonOriginalSymbol(p.check.GetSymbolAtLocation(reference.TypeName))
	var wrapper *ast.Node
	for _, name := range []string{"Read", "Write"} {
		if declaration := p.systemBuiltins[name]; declaration != nil && symbol == p.check.GetSymbolAtLocation(declaration.Name()) {
			wrapper = declaration
		}
	}
	if wrapper == nil {
		return empty, p.fail(annotation, "access must resolve exact owned Read or Write declaration")
	}
	component, _, e := p.systemResolve(reference.TypeArguments.Nodes[0], wrapperEnv, &traces, 0)
	if e != nil {
		return empty, e
	}
	if component.Kind != ast.KindTypeReference {
		return empty, p.fail(component, "original concrete component reference required")
	}
	componentSymbol := p.commonOriginalSymbol(p.check.GetSymbolAtLocation(component.AsTypeReferenceNode().TypeName))
	id := p.classes[componentSymbol]
	typ := p.types[id]
	if id == "" || typ.NominalKey == nil || typ.NominalKey.Category != "component" || p.check.GetTypeFromTypeNode(component) != p.actual[id] {
		return empty, p.fail(component, "Query access requires original owned component")
	}
	if e := p.systemFixed(id, 0, map[string]bool{}, map[string]int{}); e != nil {
		return empty, e
	}
	return systemQueryView{Aliases: traces, Access: systemQueryAccess{Index: 0, Mode: strings.ToLower(wrapper.Name().Text()), Range: *genericSpan(annotation), Wrapper: systemOrigin(wrapper), ComponentType: id, Component: systemOrigin(p.classNodes[id]), ComponentSourcePath: ast.GetSourceFileOfNode(component).FileName(), ComponentRange: *genericSpan(component)}}, nil
}
