// SPDX-License-Identifier: Apache-2.0
// The existing callable identities and checker headers own these selected scalar
// definitions. Selection never executes a helper or treats its name as purity.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

type systemScalarCallState struct {
	helpers map[*ast.Node]systemQueryScalarHelper
	heights map[*ast.Node]int
	active  map[*ast.Node]bool
	frames  []int
}

func (p *arrayGenericProjection) systemScalarHelper(node *ast.Node) (systemQueryScalarHelper, int, error) {
	empty := systemQueryScalarHelper{}
	state := p.systemScalarCalls
	if state.active[node] {
		return empty, 0, p.fail(node, "recursive Query scalar helper graph is unsupported")
	}
	if helper, ok := state.helpers[node]; ok {
		height := state.heights[node]
		if len(state.frames)+height > 32 {
			return empty, 0, p.fail(node, "selected scalar helper path exceeds32 functions")
		}
		return helper, height, nil
	}
	if len(state.frames) >= 32 || len(state.helpers)+len(state.active) >= 64 {
		return empty, 0, p.fail(node, "selected scalar helper inventory/depth limit")
	}
	if node.Kind != ast.KindFunctionDeclaration || node.Parent == nil || node.Parent.Kind != ast.KindSourceFile || !ast.IsExternalModule(ast.GetSourceFileOfNode(node)) || ast.GetFunctionFlags(node)&ast.FunctionFlagsAsyncGenerator != 0 {
		return empty, 0, p.fail(node, "scalar helper requires an original ordinary module-level function")
	}
	declaration := node.AsFunctionDeclaration()
	symbol := p.check.GetSymbolAtLocation(node.Name())
	if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != node || declaration.TypeParameters != nil || declaration.Type == nil {
		return empty, 0, p.fail(node, "scalar helper requires original unique concrete function signature")
	}
	signatures := p.check.GetSignaturesOfType(p.check.GetTypeOfSymbolAtLocation(symbol, node), checker.SignatureKindCall)
	if len(signatures) != 1 || signatures[0].Declaration() != node || len(signatures[0].Parameters()) != len(declaration.Parameters.Nodes) {
		return empty, 0, p.fail(node, "scalar helper selected original signature differs")
	}
	returns, e := p.annotation(declaration.Type)
	if e != nil {
		return empty, 0, e
	}
	if returns != "f32" && returns != "f64" || len(declaration.Parameters.Nodes) > 32 || !p.check.IsTypeAssignableTo(p.check.GetReturnTypeOfSignature(signatures[0]), p.actual[returns]) {
		return empty, 0, p.fail(node, "scalar helper requires native f32/f64 return and at most32 scalar parameters")
	}
	body := declaration.Body
	if body == nil || body.Kind != ast.KindBlock {
		return empty, 0, p.fail(node, "original scalar helper body required")
	}
	if len(body.AsBlock().Statements.Nodes) != 1 || body.AsBlock().Statements.Nodes[0].Kind != ast.KindReturnStatement {
		return empty, 0, p.fail(body, "scalar helper requires one original pure scalar return expression")
	}
	statement := body.AsBlock().Statements.Nodes[0]
	expression := statement.AsReturnStatement().Expression
	if expression == nil {
		return empty, 0, p.fail(statement, "scalar helper requires an original f32/f64 return value")
	}
	parameters := []systemQueryParameter{}
	logical := []string{}
	params := map[*ast.Symbol]systemQueryParameter{}
	bindings := map[*ast.Symbol]systemQueryLoopBinding{}
	for index, parameter := range declaration.Parameters.Nodes {
		pd := parameter.AsParameterDeclaration()
		if pd.Type == nil || pd.QuestionToken != nil || pd.Initializer != nil || pd.DotDotDotToken != nil || parameter.Name() == nil || parameter.Name().Kind != ast.KindIdentifier {
			return empty, 0, p.fail(parameter, "required original helper parameter annotation")
		}
		parameterSymbol := p.check.GetSymbolAtLocation(parameter.Name())
		if parameterSymbol != signatures[0].Parameters()[index] {
			return empty, 0, p.fail(parameter, "original helper parameter Symbol differs")
		}
		row := systemQueryParameter{Index: index, Binding: systemOrigin(parameter), AnnotationRange: *genericSpan(pd.Type)}
		traces := []systemQueryAlias{}
		resolved, _, e := p.systemResolve(pd.Type, map[*ast.Symbol]*ast.Node{}, &traces, 0)
		if e != nil {
			return empty, 0, e
		}
		var wrapper *ast.Symbol
		if resolved.Kind == ast.KindTypeReference {
			wrapper = p.commonOriginalSymbol(p.check.GetSymbolAtLocation(resolved.AsTypeReferenceNode().TypeName))
		}
		if wrapper == p.check.GetSymbolAtLocation(p.systemBuiltins["Read"].Name()) || wrapper == p.check.GetSymbolAtLocation(p.systemBuiltins["Write"].Name()) {
			view, e := p.systemView(pd.Type, map[*ast.Symbol]*ast.Node{})
			if e != nil {
				return empty, 0, e
			}
			row.View, row.LogicalType = &view, view.Access.ComponentType
			bindings[parameterSymbol] = systemQueryLoopBinding{index, row.Binding, row.LogicalType, view.Access.Mode}
		} else {
			typ, e := p.annotation(pd.Type)
			if e != nil {
				return empty, 0, e
			}
			if typ != "f32" && typ != "f64" {
				return empty, 0, p.fail(parameter, "scalar helper parameter must retain native f32/f64 annotation or owned Read/Write view")
			}
			row.LogicalType = typ
		}
		binding, e := p.typedBinding(parameter, pd.Type, "parameter", true, row.LogicalType)
		if e != nil {
			return empty, 0, e
		}
		if row.Binding.ID != binding.ID {
			return empty, 0, p.fail(parameter, "scalar helper original parameter identity differs")
		}
		parameters = append(parameters, row)
		logical = append(logical, row.LogicalType)
		params[parameterSymbol] = row
	}
	signatureID, e := genericNodeID("tsfg.array-facts.signature.v8\x00", node)
	if e != nil {
		return empty, 0, e
	}
	state.active[node] = true
	state.frames = append(state.frames, 0)
	defer func() { delete(state.active, node); state.frames = state.frames[:len(state.frames)-1] }()
	value, e := p.systemLoopExpression(expression, bindings, params, 0)
	if e != nil {
		return empty, 0, e
	}
	if value.Type != returns || value.FlowType != returns {
		return empty, 0, p.fail(expression, "scalar helper return must preserve declared f32/f64 type")
	}
	id, e := genericNodeID("tsfg.array-facts.function.v8\x00", node)
	if e != nil {
		return empty, 0, e
	}
	helper := systemQueryScalarHelper{ID: id, Declaration: systemOrigin(node), Signature: arrayGenericSignature{ID: signatureID, SourcePath: ast.GetSourceFileOfNode(node).FileName(), DeclarationRange: *genericSpan(node), Parameters: logical, ReturnType: returns}, Parameters: parameters, ReturnType: returns, ReturnAnnotationRange: *genericSpan(declaration.Type), BodyRange: *genericSpan(body), ReturnRange: *genericSpan(statement), Value: value}
	height := 1 + state.frames[len(state.frames)-1]
	state.helpers[node] = helper
	state.heights[node] = height
	return helper, height, nil
}

func (p *arrayGenericProjection) systemScalarCall(node *ast.Node, bindings map[*ast.Symbol]systemQueryLoopBinding, params map[*ast.Symbol]systemQueryParameter, depth int) (*arrayGenericExpression, error) {
	call := node.AsCallExpression()
	if call.QuestionDotToken != nil || call.TypeArguments != nil || call.Expression.Kind != ast.KindIdentifier {
		return nil, p.fail(node, "Query scalar call requires a direct named function or named import")
	}
	symbol := p.check.GetSymbolAtLocation(call.Expression)
	if symbol != nil && symbol.Flags&ast.SymbolFlagsAlias != 0 {
		if len(symbol.Declarations) != 1 || symbol.Declarations[0].Kind != ast.KindImportSpecifier {
			return nil, p.fail(call.Expression, "scalar call alias requires an original named import declaration")
		}
		symbol = p.check.GetAliasedSymbol(symbol)
	}
	selected := p.check.GetResolvedSignature(node)
	if symbol == nil || len(symbol.Declarations) != 1 || selected == nil || selected.Declaration() != symbol.Declarations[0] || symbol.Declarations[0].Kind != ast.KindFunctionDeclaration {
		return nil, p.fail(node, "scalar call must select one original named helper declaration")
	}
	helper, height, e := p.systemScalarHelper(symbol.Declarations[0])
	if e != nil {
		return nil, e
	}
	if len(p.systemScalarCalls.frames) > 0 {
		index := len(p.systemScalarCalls.frames) - 1
		p.systemScalarCalls.frames[index] = max(p.systemScalarCalls.frames[index], height)
	}
	args := call.Arguments.Nodes
	if len(args) != len(helper.Parameters) || len(selected.Parameters()) != len(args) {
		return nil, p.fail(node, "scalar call requires its complete original ordered arguments")
	}
	values := []arrayGenericExpression{}
	for index, arg := range args {
		value, e := p.systemLoopExpression(arg, bindings, params, depth+1)
		if e != nil {
			return nil, e
		}
		parameter := selected.Parameters()[index]
		formal := helper.Parameters[index]
		if value.Type != formal.LogicalType || value.FlowType != formal.LogicalType || len(parameter.Declarations) != 1 || systemOrigin(parameter.Declarations[0]).ID != formal.Binding.ID || !p.check.IsTypeAssignableTo(p.check.GetTypeAtLocation(arg), p.check.GetTypeOfSymbolAtLocation(parameter, arg)) {
			return nil, p.fail(arg, "scalar call argument must join original parameter and component in Source order")
		}
		if formal.View != nil {
			bound := value
			for bound.Kind == "parentheses" {
				bound = bound.Expression
			}
			permitted := false
			if bound.Kind == "bindingReference" {
				for _, binding := range bindings {
					if binding.Declaration.ID == bound.Binding && binding.ComponentType == formal.View.Access.ComponentType && (formal.View.Access.Mode == "read" || binding.Mode == "write") {
						permitted = true
					}
				}
			}
			if !permitted {
				return nil, p.fail(arg, "view call requires original Query or formal view binding without Read-to-Write upgrade")
			}
		}
		values = append(values, *value)
	}
	if !p.check.IsTypeAssignableTo(p.check.GetReturnTypeOfSignature(selected), p.actual[helper.ReturnType]) {
		return nil, p.fail(node, "scalar call selected return type differs from original f32/f64 annotation")
	}
	return &arrayGenericExpression{Kind: "call", Type: helper.ReturnType, FlowType: helper.ReturnType, Range: *genericSpan(node), Callee: helper.ID, CalleeRange: genericSpan(call.Expression), Signature: &helper.Signature, Arguments: &values}, nil
}
