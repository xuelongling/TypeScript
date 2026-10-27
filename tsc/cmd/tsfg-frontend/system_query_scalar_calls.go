// SPDX-License-Identifier: Apache-2.0
// The existing callable identities and checker headers own these selected scalar
// definitions. Selection never executes a helper or treats its name as purity.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
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
	header, e := p.header(node, "")
	if e != nil {
		return empty, 0, e
	}
	if header.function.ReturnType != "f32" || len(header.function.Parameters) > 32 {
		return empty, 0, p.fail(node, "scalar helper requires native f32 return and at most32 scalar parameters")
	}
	declaration := node.AsFunctionDeclaration()
	body := declaration.Body
	if len(body.AsBlock().Statements.Nodes) != 1 || body.AsBlock().Statements.Nodes[0].Kind != ast.KindReturnStatement {
		return empty, 0, p.fail(body, "scalar helper requires one original pure scalar return expression")
	}
	statement := body.AsBlock().Statements.Nodes[0]
	expression := statement.AsReturnStatement().Expression
	if expression == nil {
		return empty, 0, p.fail(statement, "scalar helper requires an original f32 return value")
	}
	parameters := []systemQueryParameter{}
	logical := []string{}
	params := map[*ast.Symbol]systemQueryParameter{}
	for index, binding := range header.function.Parameters {
		parameter := declaration.Parameters.Nodes[index]
		if binding.DeclaredType != "f32" {
			return empty, 0, p.fail(parameter, "scalar helper parameter must retain native f32 annotation")
		}
		row := systemQueryParameter{Index: index, Binding: systemOrigin(parameter), AnnotationRange: *binding.AnnotationRange, LogicalType: "f32"}
		if row.Binding.ID != binding.ID {
			return empty, 0, p.fail(parameter, "scalar helper original parameter identity differs")
		}
		parameters = append(parameters, row)
		logical = append(logical, "f32")
		params[p.check.GetSymbolAtLocation(parameter.Name())] = row
	}
	signatureID, e := genericNodeID("tsfg.array-facts.signature.v8\x00", node)
	if e != nil {
		return empty, 0, e
	}
	state.active[node] = true
	state.frames = append(state.frames, 0)
	defer func() { delete(state.active, node); state.frames = state.frames[:len(state.frames)-1] }()
	value, e := p.systemLoopExpression(expression, map[*ast.Symbol]systemQueryLoopBinding{}, params, 0)
	if e != nil {
		return empty, 0, e
	}
	if value.Type != "f32" || value.FlowType != "f32" {
		return empty, 0, p.fail(expression, "scalar helper return must preserve f32")
	}
	helper := systemQueryScalarHelper{ID: header.function.ID, Declaration: systemOrigin(node), Signature: arrayGenericSignature{ID: signatureID, SourcePath: header.function.SourcePath, DeclarationRange: *genericSpan(node), Parameters: logical, ReturnType: "f32"}, Parameters: parameters, ReturnType: "f32", ReturnAnnotationRange: *header.function.ReturnAnnotationRange, BodyRange: *genericSpan(body), ReturnRange: *genericSpan(statement), Value: value}
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
		if value.Type != "f32" || value.FlowType != "f32" || len(parameter.Declarations) != 1 || systemOrigin(parameter.Declarations[0]).ID != helper.Parameters[index].Binding.ID || !p.check.IsTypeAssignableTo(p.check.GetTypeAtLocation(arg), p.check.GetTypeOfSymbolAtLocation(parameter, arg)) {
			return nil, p.fail(arg, "scalar call argument must join original f32 parameter in Source order")
		}
		values = append(values, *value)
	}
	if !p.check.IsTypeAssignableTo(p.check.GetReturnTypeOfSignature(selected), p.actual["f32"]) {
		return nil, p.fail(node, "scalar call selected return type differs from f32")
	}
	return &arrayGenericExpression{Kind: "call", Type: "f32", FlowType: "f32", Range: *genericSpan(node), Callee: helper.ID, CalleeRange: genericSpan(call.Expression), Signature: &helper.Signature, Arguments: &values}, nil
}
