// SPDX-License-Identifier: Apache-2.0
// Original AST statements are independently reconstructed without producer helpers.
package main

import "github.com/microsoft/typescript-go/internal/ast"

func (s *ao23Scope) local(statement *ast.Node) (ao23Map, ao23Map) {
	list := statement.AsVariableStatement().DeclarationList
	ao23Want(s.t, list != nil && list.Flags&ast.NodeFlagsConst != 0 && len(list.AsVariableDeclarationList().Declarations.Nodes) == 1, "one original const declarator")
	decl := list.AsVariableDeclarationList().Declarations.Nodes[0]
	v := decl.AsVariableDeclaration()
	ao23Want(s.t, decl.Name() != nil && decl.Name().Kind == ast.KindIdentifier && v.Initializer != nil, "complete original Array local")
	sym := s.check.GetSymbolAtLocation(decl.Name())
	_, exists := s.bindings[sym]
	ao23Want(s.t, sym != nil && !exists && len(sym.Declarations) == 1 && sym.Declarations[0] == decl, "unique same-function local Symbol")
	s.arrayType(s.check.GetTypeOfSymbol(sym), false)
	row := ao23Map{"id": s.bindingID(s.file, decl.Name()), "name": decl.Name().Text(), "kind": "const", "declaredType": "array.f64", "writable": false, "declarationRange": s.span(s.file, decl), "nameRange": s.span(s.file, decl.Name()), "annotationRange": nil, "elementAnnotationRange": nil}
	if v.Type != nil {
		ao23Want(s.t, s.annotation(v.Type) == "array.f64", "complete Array annotation")
		row["annotationRange"] = s.span(s.file, v.Type)
		row["elementAnnotationRange"] = s.span(s.file, v.Type.AsArrayTypeNode().ElementType)
	}
	var init ao23Map
	if v.Initializer.Kind == ast.KindArrayLiteralExpression {
		ao23Want(s.t, v.Type != nil, "literal requires original explicit Array annotation")
		init = s.arrayLiteral(v.Initializer)
	} else {
		init = s.reference(v.Initializer, "array.f64")
		s.counts["aliases"]++
	}
	ao23Want(s.t, s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(v.Initializer), s.check.GetTypeOfSymbol(sym)), "official initializer assignment")
	s.bindings[sym] = ao23Binding{row["id"].(string), "array.f64"}
	s.counts["locals"]++
	return row, ao23Map{"kind": "variable", "binding": row["id"], "range": s.span(s.file, statement), "initializer": init}
}
func (s *ao23Scope) mutation(statement *ast.Node) ao23Map {
	expr := statement.AsExpressionStatement().Expression
	ao23Want(s.t, expr != nil && expr.Kind == ast.KindBinaryExpression, "original assignment expression")
	b := expr.AsBinaryExpression()
	ao23Want(s.t, b.OperatorToken.Kind == ast.KindEqualsToken, "ordinary original assignment")
	kind := "elementAssignment"
	var target, value ao23Map
	if b.Left.Kind == ast.KindElementAccessExpression {
		target = s.element(b.Left, true)
		value = s.scalar(b.Right, "f64", "rhs")
	} else {
		ao23Want(s.t, b.Left.Kind == ast.KindPropertyAccessExpression, "only original Array length property assignment")
		p := b.Left.AsPropertyAccessExpression()
		ao23Want(s.t, p.QuestionDotToken == nil && p.Name() != nil && p.Name().Text() == "length", "ordinary length property")
		sym := s.check.GetSymbolAtLocation(p.Name())
		ao23Want(s.t, sym != nil && len(sym.Declarations) == 1 && sym.Declarations[0] == s.length && s.check.GetPropertyOfType(s.check.GetTypeAtLocation(p.Expression), "length") == sym && s.check.GetTypeOfSymbolAtLocation(sym, p.Name()) == s.check.GetNumberType(), "actual original pinned length Symbol")
		kind = "lengthAssignment"
		target = ao23Map{"kind": "lengthTarget", "type": "u32", "range": s.span(s.file, b.Left), "array": s.reference(p.Expression, "array.f64"), "property": s.bindingID(s.dts, s.length.Name()), "nameRange": s.span(s.file, p.Name())}
		value = s.scalar(b.Right, "u32", "length")
	}
	ao23Want(s.t, s.check.GetTypeAtLocation(b.Left) == s.check.GetNumberType() && s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(b.Right), s.check.GetTypeAtLocation(b.Left)), "official assignment target and RHS types")
	s.counts["mutations"]++
	return ao23Map{"kind": kind, "range": s.span(s.file, statement), "expressionRange": s.span(s.file, expr), "operatorRange": s.span(s.file, b.OperatorToken), "target": target, "value": value}
}
func (s *ao23Scope) buildFunction(h *ao23Header) {
	s.bindings = map[*ast.Symbol]ao23Binding{}
	for i, sym := range h.signature.Parameters() {
		s.bindings[sym] = ao23Binding{h.parameters[i]["id"].(string), h.kinds[i]}
	}
	bindings, body := []any{}, []any{}
	nodes := h.node.AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
	ao23Want(s.t, len(nodes) > 0 && len(nodes) <= 50, "complete bounded original body")
	for i, node := range nodes {
		var row ao23Map
		switch node.Kind {
		case ast.KindVariableStatement:
			binding, statement := s.local(node)
			bindings = append(bindings, binding)
			row = statement
		case ast.KindExpressionStatement:
			expr := node.AsExpressionStatement().Expression
			if expr.Kind == ast.KindCallExpression {
				row = ao23Map{"kind": "call", "range": s.span(s.file, node), "value": s.directCall(expr)}
			} else {
				row = s.mutation(node)
			}
		case ast.KindReturnStatement:
			ao23Want(s.t, i == len(nodes)-1, "single final original return")
			value := node.AsReturnStatement().Expression
			ao23Want(s.t, value != nil && s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(value), s.check.GetNumberType()), "original f64 return assignability")
			row = ao23Map{"kind": "return", "range": s.span(s.file, node), "value": s.scalar(value, "f64", "return")}
		default:
			s.t.Fatalf("unsupported independent original statement %s", node.Kind)
		}
		body = append(body, row)
		s.counts["statements"]++
	}
	ao23Want(s.t, nodes[len(nodes)-1].Kind == ast.KindReturnStatement && len(bindings) <= 17, "final return and bounded local declarations")
	h.row["bindings"], h.row["body"] = bindings, body
}
func (s *ao23Scope) proveDAG(headers []*ao23Header) {
	byID := map[string]*ao23Header{}
	for _, h := range headers {
		byID[h.row["id"].(string)] = h
	}
	active := map[string]bool{}
	var visit func(string, int)
	visit = func(id string, depth int) {
		ao23Want(s.t, !active[id] && depth <= 16, "complete direct call graph is acyclic and bounded")
		active[id] = true
		h := byID[id]
		ao23Want(s.t, h != nil, "call target is actual complete source function")
		for _, item := range h.row["body"].([]any) {
			row := item.(ao23Map)
			v, ok := row["value"].(ao23Map)
			if ok && v["kind"] == "directCall" {
				visit(v["callee"].(string), depth+1)
			}
		}
		delete(active, id)
	}
	for id := range byID {
		visit(id, 1)
	}
}
