// SPDX-License-Identifier: Apache-2.0
// Full original adjacent bodies; view formals and original mixed slots remain nominal.
package main

import "github.com/microsoft/typescript-go/internal/ast"

func (s *rs26Scope) rs26bAdjacentRead(n *ast.Node) ao23Map {
	ao23Want(s.t, n != nil && n.Kind == ast.KindElementAccessExpression, "actual checked readonly formal/local view read")
	e := n.AsElementAccessExpression()
	ao23Want(s.t, e.QuestionDotToken == nil && e.ArgumentExpression != nil && s.check.GetTypeAtLocation(n) == s.check.GetNumberType(), "actual original readonly indexed Number result")
	view, index := s.viewReference(e.Expression), s.scalar(e.ArgumentExpression, "u32", "index")
	row := ao23Map{"kind": "checkedReadSpanAccess", "type": "f64", "range": s.span(s.file, n), "view": view, "index": index}
	roles, _ := s.proof["checkedReadRoles"].([]any)
	s.proof["checkedReadRoles"] = append(roles, ao23Map{"range": row["range"], "view": view, "index": index, "officialReadonlyOrigin": true, "retainedObligations": []string{"u32 index role", "view bounds", "checked load"}})
	s.counts["viewReads"]++
	return row
}

func (s *rs26Scope) rs26bAdjacentValue(n *ast.Node, role string) ao23Map {
	ao23Want(s.t, n != nil, "complete original adjacent value")
	switch n.Kind {
	case ast.KindElementAccessExpression:
		return s.rs26bAdjacentRead(n)
	case ast.KindCallExpression:
		return s.rs26bDirectCall(n)
	default:
		return s.scalar(n, "f64", role)
	}
}

func (s *rs26Scope) rs26bAdjacentFunctions(headers []*ao23Header) []any {
	rows, inventory := []any{}, []any{}
	for _, h := range headers {
		s.bindings, s.views = map[*ast.Symbol]ao23Binding{}, map[*ast.Symbol]string{}
		for i, sym := range h.signature.Parameters() {
			if h.kinds[i] == "readspan.f64" {
				s.viewType(s.check.GetTypeOfSymbol(sym), s.check.GetNumberType())
				s.views[sym] = h.parameters[i]["id"].(string)
			} else {
				s.bindings[sym] = ao23Binding{h.parameters[i]["id"].(string), h.kinds[i]}
			}
		}
		bindings, body := []any{}, []any{}
		nodes := h.node.AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
		beforeCalls, beforeCreates := s.counts["calls"], s.counts["viewCreates"]
		ao23Want(s.t, len(nodes) > 0 && len(nodes) <= 50 && len(h.parameters) <= 8, "complete bounded original signature and body")
		for i, n := range nodes {
			var row ao23Map
			switch n.Kind {
			case ast.KindVariableStatement:
				decls := n.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
				ao23Want(s.t, len(decls) == 1 && decls[0].AsVariableDeclaration().Initializer != nil, "one complete original local initializer")
				init := decls[0].AsVariableDeclaration().Initializer
				isView := s.rsAdj26IsView(init)
				if init.Kind == ast.KindCallExpression {
					callee := init.AsCallExpression().Expression
					isView = callee.Kind == ast.KindIdentifier && s.check.GetSymbolAtLocation(callee) == s.intrinsic
				}
				var binding ao23Map
				if isView {
					binding, row = s.viewLocal(n)
					s.counts["viewLocals"]++
				} else {
					ao23Want(s.t, init.Kind == ast.KindArrayLiteralExpression || init.Kind == ast.KindIdentifier, "selected complete Array local role")
					binding, row = s.local(n)
				}
				bindings = append(bindings, binding)
			case ast.KindExpressionStatement:
				e := n.AsExpressionStatement().Expression
				switch e.Kind {
				case ast.KindBinaryExpression:
					row = s.mutation(n)
				case ast.KindCallExpression:
					row = ao23Map{"kind": "call", "range": s.span(s.file, n), "value": s.rs26bDirectCall(e)}
				case ast.KindElementAccessExpression:
					row = ao23Map{"kind": "readSpanUse", "range": s.span(s.file, n), "value": s.rs26bAdjacentRead(e)}
				case ast.KindPropertyAccessExpression:
					row = ao23Map{"kind": "readSpanUse", "range": s.span(s.file, n), "value": s.rsAdj26Length(e)}
				default:
					s.t.Fatalf("uncovered original adjacent expression %s", e.Kind)
				}
			case ast.KindReturnStatement:
				ao23Want(s.t, i == len(nodes)-1, "single final original adjacent return")
				v := n.AsReturnStatement().Expression
				ao23Want(s.t, v != nil && s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(v), s.check.GetReturnTypeOfSignature(h.signature)), "real original return assignability")
				row = ao23Map{"kind": "return", "range": s.span(s.file, n), "value": s.rs26bAdjacentValue(v, "return")}
			default:
				s.t.Fatalf("uncovered complete original adjacent statement %s", n.Kind)
			}
			body = append(body, row)
			s.counts["statements"]++
		}
		ao23Want(s.t, nodes[len(nodes)-1].Kind == ast.KindReturnStatement && len(bindings) <= 17 && s.counts["calls"]-beforeCalls+s.counts["viewCreates"]-beforeCreates <= 32, "complete final return and bounded original local/raw-call inventory")
		h.row["bindings"], h.row["body"] = bindings, body
		rows = append(rows, h.row)
		inventory = append(inventory, ao23Map{"name": h.node.Name().Text(), "parameters": len(h.parameters), "statements": len(nodes), "locals": len(bindings), "ordinaryCalls": s.counts["calls"] - beforeCalls, "intrinsicCalls": s.counts["viewCreates"] - beforeCreates})
	}
	s.proveDAG(headers)
	s.proof["functionInventory"] = inventory
	return rows
}
