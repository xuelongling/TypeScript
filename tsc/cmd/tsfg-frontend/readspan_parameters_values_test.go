// SPDX-License-Identifier: Apache-2.0
// Complete first-pair bodies and nominal view call arguments from official objects only.
package main

import "github.com/microsoft/typescript-go/internal/ast"

func (s *rs26Scope) rs26bDirectCall(node *ast.Node) ao23Map {
	ao23Want(s.t, node != nil && node.Kind == ast.KindCallExpression, "actual original ordinary call")
	c := node.AsCallExpression()
	ao23Want(s.t, c.Expression != nil && c.Expression.Kind == ast.KindIdentifier && c.QuestionDotToken == nil && c.TypeArguments == nil, "direct original named nongeneric invocation")
	sym := s.check.GetSymbolAtLocation(c.Expression)
	h := s.headers[sym]
	ao23Want(s.t, h != nil && sym == h.symbol && len(sym.Declarations) == 1 && sym.Declarations[0] == h.node && h.node.Name().Text() == c.Expression.Text(), "actual original same-module target Symbol and declaration")
	resolved := s.check.GetResolvedSignature(node)
	ao23Want(s.t, resolved != nil && resolved.Declaration() == h.node && len(resolved.Parameters()) == len(h.parameters) && len(c.Arguments.Nodes) == len(h.parameters) && resolved.MinArgumentCount() == len(h.parameters) && len(resolved.TypeParameters()) == 0 && resolved.ThisParameter() == nil && !resolved.HasRestParameter() && s.check.GetReturnTypeOfSignature(resolved) == s.check.GetNumberType() && s.check.GetTypeAtLocation(node) == s.check.GetNumberType(), "complete actual resolved required signature and f64 result")
	args, proof := []any{}, []any{}
	for i, n := range c.Arguments.Nodes {
		p := resolved.Parameters()[i]
		ao23Want(s.t, p == h.signature.Parameters()[i] && len(p.Declarations) == 1 && p.Declarations[0] == h.node.AsFunctionDeclaration().Parameters.Nodes[i], "actual original full callee slot Symbol and declaration join")
		assignable := s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(n), s.check.GetTypeOfSymbol(p))
		ao23Want(s.t, assignable, "real official argument type assignability before private nominal dispatch")
		var row ao23Map
		switch h.kinds[i] {
		case "readspan.f64":
			s.viewType(s.check.GetTypeOfSymbol(p), s.check.GetNumberType())
			row = s.viewReference(n)
			actual := s.check.GetSymbolAtLocation(n)
			ao23Want(s.t, len(actual.Declarations) == 1 && (actual.Declarations[0].Kind == ast.KindVariableDeclaration || actual.Declarations[0].Kind == ast.KindParameter), "actual nominal view reference retains original local/parameter origin")
		case "array.f64":
			row = s.reference(n, "array.f64")
		default:
			ao23Want(s.t, n.Kind == ast.KindIdentifier || n.Kind == ast.KindNumericLiteral || n.Kind == ast.KindPrefixUnaryExpression, "atomic original scalar argument")
			row = s.scalar(n, h.kinds[i], "argument")
		}
		args = append(args, row)
		proof = append(proof, ao23Map{"index": i, "parameterBinding": h.parameters[i]["id"], "declaredType": h.kinds[i], "argument": row, "assignable": assignable, "originalParameterDeclaration": true})
	}
	calls, _ := s.proof["calls"].([]any)
	s.proof["calls"] = append(calls, ao23Map{"target": h.row["id"], "range": s.span(s.file, node), "resolvedOriginalDeclaration": true, "arguments": proof})
	s.counts["calls"]++
	return ao23Map{"kind": "directCall", "type": "f64", "range": s.span(s.file, node), "callee": h.row["id"], "calleeRange": s.span(s.file, c.Expression), "arguments": args}
}

func (s *rs26Scope) rs26bFunctions(headers []*ao23Header) []any {
	ao23Want(s.t, len(headers) == 3 && headers[0].node.Name().Text() == "ignore" && headers[1].node.Name().Text() == "mutate" && headers[2].node.Name().Text() == "entry", "all three original functions including unused formal helper")
	rows := []any{}
	for j, h := range headers {
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
		ao23Want(s.t, len(nodes) == []int{1, 2, 5}[j], "complete first272B original body inventory")
		bindings, body := []any{}, []any{}
		for i, n := range nodes {
			var row ao23Map
			switch n.Kind {
			case ast.KindVariableStatement:
				var b ao23Map
				if i == 0 {
					b, row = s.local(n)
				} else {
					b, row = s.viewLocal(n)
				}
				bindings = append(bindings, b)
			case ast.KindExpressionStatement:
				e := n.AsExpressionStatement().Expression
				if e.Kind == ast.KindBinaryExpression {
					row = s.mutation(n)
				} else {
					ao23Want(s.t, e.Kind == ast.KindCallExpression, "whole first ordinary-call expression")
					row = ao23Map{"kind": "call", "range": s.span(s.file, n), "value": s.rs26bDirectCall(e)}
				}
			case ast.KindReturnStatement:
				ao23Want(s.t, i == len(nodes)-1, "single final original literal return")
				v := n.AsReturnStatement().Expression
				ao23Want(s.t, v != nil && s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(v), s.check.GetReturnTypeOfSignature(h.signature)), "actual original return assignability")
				row = ao23Map{"kind": "return", "range": s.span(s.file, n), "value": s.number(v, "f64", "return")}
			default:
				s.t.Fatalf("uncovered original first272B statement %s", n.Kind)
			}
			body = append(body, row)
			s.counts["statements"]++
		}
		h.row["bindings"], h.row["body"] = bindings, body
		rows = append(rows, h.row)
	}
	s.proveDAG(headers)
	return rows
}
