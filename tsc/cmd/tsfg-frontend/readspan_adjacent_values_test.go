// SPDX-License-Identifier: Apache-2.0
// Independent original readonly-view expressions; no Source producer helper is called.
package main

import "github.com/microsoft/typescript-go/internal/ast"

func (s *rs26Scope) rsAdj26IsView(node *ast.Node) bool {
	if node == nil || node.Kind != ast.KindIdentifier {
		return false
	}
	_, ok := s.views[s.check.GetSymbolAtLocation(node)]
	return ok
}
func (s *rs26Scope) rsAdj26Read(node *ast.Node) ao23Map {
	ao23Want(s.t, node != nil && node.Kind == ast.KindElementAccessExpression, "actual readonly view read")
	e := node.AsElementAccessExpression()
	ao23Want(s.t, e.QuestionDotToken == nil && s.check.GetTypeAtLocation(node) == s.check.GetNumberType(), "ordinary readonly view element Number")
	value := ao23Map{"kind": "checkedReadSpanAccess", "type": "f64", "range": s.span(s.file, node), "view": s.viewReference(e.Expression), "index": s.number(e.ArgumentExpression, "u32", "index")}
	s.counts["viewReads"]++
	return value
}
func (s *rs26Scope) rsAdj26Length(node *ast.Node) ao23Map {
	ao23Want(s.t, node != nil && node.Kind == ast.KindPropertyAccessExpression, "actual standalone saved view length")
	p := node.AsPropertyAccessExpression()
	sym := s.check.GetSymbolAtLocation(p.Name())
	ao23Want(s.t, p.QuestionDotToken == nil && p.Name().Text() == "length" && sym != nil && len(sym.Declarations) == 1 && sym.Declarations[0] == s.viewLength && s.check.GetPropertyOfType(s.check.GetTypeAtLocation(p.Expression), "length") == sym, "actual readonly length property joins pinned original declaration")
	ao23Want(s.t, s.check.GetTypeAtLocation(node) == s.check.GetNumberType() && s.check.GetTypeOfSymbolAtLocation(sym, p.Name()) == s.check.GetNumberType(), "actual readonly length Number representation")
	s.counts["viewLengths"]++
	return ao23Map{"kind": "readSpanLength", "type": "u32", "range": s.span(s.file, node), "view": s.viewReference(p.Expression), "property": s.bindingID(s.dts, s.viewLength.Name()), "nameRange": s.span(s.file, p.Name())}
}
func (s *rs26Scope) rsAdj26Scalar(node *ast.Node, role string) ao23Map {
	if node.Kind == ast.KindElementAccessExpression && s.rsAdj26IsView(node.AsElementAccessExpression().Expression) {
		return s.rsAdj26Read(node)
	}
	return s.scalar(node, "f64", role)
}
func (s *rs26Scope) rsAdj26Mutation(statement *ast.Node) ao23Map {
	node := statement.AsExpressionStatement().Expression
	ao23Want(s.t, node.Kind == ast.KindBinaryExpression, "original ordinary assignment")
	b := node.AsBinaryExpression()
	if b.Right.Kind != ast.KindElementAccessExpression || !s.rsAdj26IsView(b.Right.AsElementAccessExpression().Expression) {
		return s.mutation(statement)
	}
	ao23Want(s.t, b.OperatorToken.Kind == ast.KindEqualsToken && b.Left.Kind == ast.KindElementAccessExpression, "view RHS only original plain Array element assignment")
	target, value := s.element(b.Left, true), s.rsAdj26Read(b.Right)
	ao23Want(s.t, s.check.GetTypeAtLocation(b.Left) == s.check.GetNumberType() && s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(b.Right), s.check.GetTypeAtLocation(b.Left)), "actual original checked view RHS assignment types")
	s.counts["mutations"]++
	return ao23Map{"kind": "elementAssignment", "range": s.span(s.file, statement), "expressionRange": s.span(s.file, node), "operatorRange": s.span(s.file, b.OperatorToken), "target": target, "value": value}
}
func (s *rs26Scope) rsAdj26Functions(headers []*ao23Header) ([]any, []any) {
	rows, inventory := []any{}, []any{}
	for _, h := range headers {
		s.bindings = map[*ast.Symbol]ao23Binding{}
		s.views = map[*ast.Symbol]string{}
		for i, sym := range h.signature.Parameters() {
			s.bindings[sym] = ao23Binding{h.parameters[i]["id"].(string), h.kinds[i]}
		}
		bindings, body := []any{}, []any{}
		nodes := h.node.AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
		beforeCalls, beforeCreates := s.counts["calls"], s.counts["viewCreates"]
		ao23Want(s.t, len(nodes) > 0 && len(nodes) <= 50, "bounded complete original body")
		for i, node := range nodes {
			var row ao23Map
			switch node.Kind {
			case ast.KindVariableStatement:
				declarations := node.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
				ao23Want(s.t, len(declarations) == 1, "one complete original const declaration")
				init := declarations[0].AsVariableDeclaration().Initializer
				ao23Want(s.t, init != nil, "original complete local initializer")
				isView := s.rsAdj26IsView(init)
				if init.Kind == ast.KindCallExpression {
					callee := init.AsCallExpression().Expression
					isView = callee.Kind == ast.KindIdentifier && s.check.GetSymbolAtLocation(callee) == s.intrinsic
				}
				var binding ao23Map
				if isView {
					binding, row = s.viewLocal(node)
					s.counts["viewLocals"]++
				} else {
					binding, row = s.local(node)
				}
				bindings = append(bindings, binding)
			case ast.KindExpressionStatement:
				expression := node.AsExpressionStatement().Expression
				switch expression.Kind {
				case ast.KindBinaryExpression:
					row = s.rsAdj26Mutation(node)
				case ast.KindCallExpression:
					row = ao23Map{"kind": "call", "range": s.span(s.file, node), "value": s.directCall(expression)}
				case ast.KindElementAccessExpression:
					row = ao23Map{"kind": "readSpanUse", "range": s.span(s.file, node), "value": s.rsAdj26Read(expression)}
				case ast.KindPropertyAccessExpression:
					row = ao23Map{"kind": "readSpanUse", "range": s.span(s.file, node), "value": s.rsAdj26Length(expression)}
				default:
					s.t.Fatalf("uncovered original expression %s", expression.Kind)
				}
			case ast.KindReturnStatement:
				ao23Want(s.t, i == len(nodes)-1, "single final original return")
				value := node.AsReturnStatement().Expression
				ao23Want(s.t, value != nil && s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(value), s.check.GetNumberType()), "actual original f64 return assignment")
				row = ao23Map{"kind": "return", "range": s.span(s.file, node), "value": s.rsAdj26Scalar(value, "return")}
			default:
				s.t.Fatalf("uncovered original statement %s", node.Kind)
			}
			body = append(body, row)
			s.counts["statements"]++
		}
		ao23Want(s.t, nodes[len(nodes)-1].Kind == ast.KindReturnStatement && len(bindings) <= 17, "final return and complete bounded Array/view bindings")
		h.row["bindings"], h.row["body"] = bindings, body
		rows = append(rows, h.row)
		inventory = append(inventory, ao23Map{"function": h.node.Name().Text(), "statements": len(nodes), "locals": len(bindings), "ordinaryCalls": s.counts["calls"] - beforeCalls, "intrinsicCalls": s.counts["viewCreates"] - beforeCreates, "rawCalls": s.counts["calls"] - beforeCalls + s.counts["viewCreates"] - beforeCreates})
	}
	s.proveDAG(headers)
	return rows, inventory
}
