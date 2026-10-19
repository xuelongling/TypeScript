// SPDX-License-Identifier: Apache-2.0
// These builders traverse official original objects, independently of Source projection.
package main

import "github.com/microsoft/typescript-go/internal/ast"

func (s *rs26cOfficialScope) value26c(n *ast.Node, kind, role string) ao23Map {
	ao23Want(s.t, n != nil, "complete original expression")
	if n.Kind == ast.KindCallExpression {
		row := s.call26c(n)
		ao23Want(s.t, row["type"] == kind, "original resolved result matches its actual use")
		return row
	}
	if kind == "readspan.f64" {
		return s.viewReference(n)
	}
	if n.Kind == ast.KindElementAccessExpression {
		e := n.AsElementAccessExpression()
		if s.views[s.check.GetSymbolAtLocation(e.Expression)] != "" {
			ao23Want(s.t, kind == "f64" && (role == "return" || role == "rhs") && e.QuestionDotToken == nil && s.check.GetTypeAtLocation(n) == s.check.GetNumberType(), "actual checked readonly view access in numeric role")
			s.counts["viewReads"]++
			return ao23Map{"kind": "checkedReadSpanAccess", "type": "f64", "range": s.span(s.file, n), "view": s.viewReference(e.Expression), "index": s.scalar(e.ArgumentExpression, "u32", "index")}
		}
	}
	return s.scalar(n, kind, role)
}

func (s *rs26cOfficialScope) viewLocal26c(statement *ast.Node) (ao23Map, ao23Map) {
	list := statement.AsVariableStatement().DeclarationList
	ao23Want(s.t, list != nil && list.Flags&ast.NodeFlagsConst != 0 && len(list.AsVariableDeclarationList().Declarations.Nodes) == 1, "single original const view declarator")
	n := list.AsVariableDeclarationList().Declarations.Nodes[0]
	v := n.AsVariableDeclaration()
	sym := s.original(n)
	ao23Want(s.t, n.Name().Kind == ast.KindIdentifier && v.Initializer != nil && v.ExclamationToken == nil && s.views[sym] == "", "new unique original view local")
	s.viewType(s.check.GetTypeOfSymbol(sym), s.check.GetNumberType())
	row := ao23Map{"id": s.bindingID(s.file, n.Name()), "name": n.Name().Text(), "kind": "const", "declaredType": "readspan.f64", "writable": false, "declarationRange": s.span(s.file, n), "nameRange": s.span(s.file, n.Name()), "annotationRange": nil, "elementAnnotationRange": nil}
	if v.Type != nil {
		element := s.rs26bViewAnnotation(v.Type)
		row["annotationRange"], row["elementAnnotationRange"] = s.span(s.file, v.Type), s.span(s.file, element)
	}
	var init ao23Map
	if v.Initializer.Kind == ast.KindCallExpression && s.check.GetSymbolAtLocation(v.Initializer.AsCallExpression().Expression) == s.intrinsic {
		init = s.create(v.Initializer)
	} else {
		init = s.value26c(v.Initializer, "readspan.f64", "initializer")
		if v.Initializer.Kind == ast.KindIdentifier {
			s.counts["viewAliases"]++
		}
	}
	ao23Want(s.t, s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(v.Initializer), s.check.GetTypeOfSymbol(sym)), "real initializer assignment to original nominal view")
	s.views[sym] = row["id"].(string)
	return row, ao23Map{"kind": "variable", "binding": row["id"], "range": s.span(s.file, statement), "initializer": init}
}

func (s *rs26cOfficialScope) mutation26c(statement *ast.Node) ao23Map {
	expression := statement.AsExpressionStatement().Expression
	ao23Want(s.t, expression != nil && expression.Kind == ast.KindBinaryExpression, "ordinary original assignment expression")
	b := expression.AsBinaryExpression()
	ao23Want(s.t, b.OperatorToken != nil && b.OperatorToken.Kind == ast.KindEqualsToken && b.Type == nil, "plain original assignment")
	if b.Left.Kind == ast.KindPropertyAccessExpression {
		return s.mutation(statement)
	}
	target := s.element(b.Left, true)
	value := s.value26c(b.Right, "f64", "rhs")
	ao23Want(s.t, s.check.GetTypeAtLocation(b.Left) == s.check.GetNumberType() && s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(b.Right), s.check.GetTypeAtLocation(b.Left)), "actual target and RHS assignment types")
	s.counts["mutations"]++
	return ao23Map{"kind": "elementAssignment", "range": s.span(s.file, statement), "expressionRange": s.span(s.file, expression), "operatorRange": s.span(s.file, b.OperatorToken), "target": target, "value": value}
}

func (s *rs26cOfficialScope) functions26c(headers []*ao23Header) []any {
	rows := []any{}
	for _, h := range headers {
		s.current = h.row["id"].(string)
		s.bindings, s.views = map[*ast.Symbol]ao23Binding{}, map[*ast.Symbol]string{}
		for i, sym := range h.signature.Parameters() {
			if h.kinds[i] == "readspan.f64" {
				s.viewType(s.check.GetTypeOfSymbol(sym), s.check.GetNumberType())
				s.views[sym] = h.parameters[i]["id"].(string)
			} else {
				s.bindings[sym] = ao23Binding{h.parameters[i]["id"].(string), h.kinds[i]}
			}
		}
		nodes := h.node.AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
		ao23Want(s.t, len(nodes) > 0 && len(nodes) <= 50, "complete bounded original body")
		bindings, body := []any{}, []any{}
		for i, n := range nodes {
			var row ao23Map
			switch n.Kind {
			case ast.KindVariableStatement:
				list := n.AsVariableStatement().DeclarationList
				ao23Want(s.t, list != nil && len(list.AsVariableDeclarationList().Declarations.Nodes) == 1, "complete single original local")
				decl := list.AsVariableDeclarationList().Declarations.Nodes[0]
				var b ao23Map
				if s.check.IsArrayType(s.check.GetTypeAtLocation(decl.Name())) {
					b, row = s.local(n)
				} else {
					b, row = s.viewLocal26c(n)
				}
				bindings = append(bindings, b)
			case ast.KindExpressionStatement:
				expression := n.AsExpressionStatement().Expression
				ao23Want(s.t, expression != nil, "complete original expression statement")
				switch expression.Kind {
				case ast.KindBinaryExpression:
					row = s.mutation26c(n)
				case ast.KindCallExpression:
					row = ao23Map{"kind": "call", "range": s.span(s.file, n), "value": s.call26c(expression)}
				default:
					row = s.viewRead(n)
				}
			case ast.KindReturnStatement:
				ao23Want(s.t, i == len(nodes)-1, "single final original return")
				value := n.AsReturnStatement().Expression
				ao23Want(s.t, value != nil && s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(value), s.check.GetReturnTypeOfSignature(h.signature)), "actual return assignability to original signature")
				row = ao23Map{"kind": "return", "range": s.span(s.file, n), "value": s.value26c(value, h.row["returnType"].(string), "return")}
			default:
				s.t.Fatalf("uncovered original return-family statement %s", n.Kind)
			}
			body = append(body, row)
			s.counts["statements"]++
		}
		ao23Want(s.t, nodes[len(nodes)-1].Kind == ast.KindReturnStatement && len(bindings) <= 17, "complete final return and bounded local declarations")
		h.row["bindings"], h.row["body"] = bindings, body
		rows = append(rows, h.row)
	}
	s.dag26c()
	return rows
}
