// SPDX-License-Identifier: Apache-2.0
// Complete first-case view facts from actual original symbols and resolved signatures.
package main

import "github.com/microsoft/typescript-go/internal/ast"

func (s *rs26Scope) viewReference(node *ast.Node) ao23Map {
	ao23Want(s.t, node != nil && node.Kind == ast.KindIdentifier, "original atomic view reference")
	sym := s.check.GetSymbolAtLocation(node)
	id, ok := s.views[sym]
	ao23Want(s.t, sym != nil && ok && len(sym.Declarations) == 1, "original registered same-function view binding")
	s.viewType(s.check.GetTypeAtLocation(node), s.check.GetNumberType())
	s.counts["viewReferences"]++
	return ao23Map{"kind": "reference", "binding": id, "flowType": "readspan.f64", "range": s.span(s.file, node)}
}
func (s *rs26Scope) create(node *ast.Node) ao23Map {
	ao23Want(s.t, node.Kind == ast.KindCallExpression, "original intrinsic call expression")
	c := node.AsCallExpression()
	ao23Want(s.t, c.Expression.Kind == ast.KindIdentifier && c.Expression.Text() == "readSpan" && s.check.GetSymbolAtLocation(c.Expression) == s.intrinsic && c.TypeArguments == nil && c.QuestionDotToken == nil && len(c.Arguments.Nodes) == 3, "actual canonical global intrinsic, no spelling-only dispatch")
	resolved := s.check.GetResolvedSignature(node)
	ao23Want(s.t, resolved != nil && resolved.Declaration() == s.intrinsicNode && len(resolved.Parameters()) == 3 && resolved.MinArgumentCount() == 3 && len(resolved.TypeParameters()) == 0 && !resolved.HasRestParameter() && resolved.ThisParameter() == nil, "actual instantiated intrinsic required signature")
	for i, p := range resolved.Parameters() {
		original := s.intrinsicNode.AsFunctionDeclaration().Parameters.Nodes[i]
		ao23Want(s.t, p != nil && len(p.Declarations) == 1 && p.Declarations[0] == original && p.Name == original.Name().Text(), "transient instantiated parameter joins original declaration/order, not guessed pointer equality")
		if i == 0 {
			s.arrayElement(s.check.GetTypeOfSymbol(p), s.check.GetNumberType())
		} else {
			ao23Want(s.t, s.check.GetTypeOfSymbol(p) == s.check.GetNumberType(), "actual instantiated u32 Number parameter")
		}
		ao23Want(s.t, s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(c.Arguments.Nodes[i]), s.check.GetTypeOfSymbol(p)), "official actual intrinsic argument assignability")
	}
	s.viewType(s.check.GetReturnTypeOfSignature(resolved), s.check.GetNumberType())
	s.viewType(s.check.GetTypeAtLocation(node), s.check.GetNumberType())
	s.counts["instantiatedParameterDeclarationJoins"] += 3
	s.counts["viewCreates"]++
	return ao23Map{"kind": "readSpanCreate", "type": "readspan.f64", "range": s.span(s.file, node), "intrinsic": s.bindingID(s.dts, s.intrinsicNode.Name()), "calleeRange": s.span(s.file, c.Expression), "array": s.reference(c.Arguments.Nodes[0], "array.f64"), "start": s.number(c.Arguments.Nodes[1], "u32", "argument"), "length": s.number(c.Arguments.Nodes[2], "u32", "argument")}
}
func (s *rs26Scope) viewLocal(statement *ast.Node) (ao23Map, ao23Map) {
	list := statement.AsVariableStatement().DeclarationList
	ao23Want(s.t, list.Flags&ast.NodeFlagsConst != 0 && len(list.AsVariableDeclarationList().Declarations.Nodes) == 1, "one ordinary original readonly-view const declarator")
	node := list.AsVariableDeclarationList().Declarations.Nodes[0]
	v := node.AsVariableDeclaration()
	sym := s.original(node)
	ao23Want(s.t, node.Name().Kind == ast.KindIdentifier && v.Initializer != nil && s.views[sym] == "", "new original unique local view Symbol")
	s.viewType(s.check.GetTypeOfSymbol(sym), s.check.GetNumberType())
	row := ao23Map{"id": s.bindingID(s.file, node.Name()), "name": node.Name().Text(), "kind": "const", "declaredType": "readspan.f64", "writable": false, "declarationRange": s.span(s.file, node), "nameRange": s.span(s.file, node.Name()), "annotationRange": nil, "elementAnnotationRange": nil}
	if v.Type != nil {
		ao23Want(s.t, v.Type.Kind == ast.KindTypeReference, "original ReadSpan<f64> annotation")
		typ := v.Type.AsTypeReferenceNode()
		ao23Want(s.t, s.check.GetSymbolAtLocation(typ.TypeName) == s.view && typ.TypeArguments != nil && len(typ.TypeArguments.Nodes) == 1, "actual pinned readonly view annotation symbol/arity")
		s.annotationScalar(typ.TypeArguments.Nodes[0], "f64")
		s.viewType(s.check.GetTypeFromTypeNode(v.Type), s.check.GetNumberType())
		row["annotationRange"] = s.span(s.file, v.Type)
		row["elementAnnotationRange"] = s.span(s.file, typ.TypeArguments.Nodes[0])
	}
	var init ao23Map
	if v.Initializer.Kind == ast.KindCallExpression {
		init = s.create(v.Initializer)
	} else {
		init = s.viewReference(v.Initializer)
		s.counts["viewAliases"]++
	}
	ao23Want(s.t, s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(v.Initializer), s.check.GetTypeOfSymbol(sym)), "actual original view initializer assignment")
	s.views[sym] = row["id"].(string)
	return row, ao23Map{"kind": "variable", "binding": row["id"], "range": s.span(s.file, statement), "initializer": init}
}
func (s *rs26Scope) viewRead(statement *ast.Node) ao23Map {
	node := statement.AsExpressionStatement().Expression
	ao23Want(s.t, node.Kind == ast.KindElementAccessExpression, "actual standalone checked view-read expression")
	e := node.AsElementAccessExpression()
	ao23Want(s.t, e.QuestionDotToken == nil && s.check.GetTypeAtLocation(node) == s.check.GetNumberType(), "actual readonly view element Number and ordinary access")
	value := ao23Map{"kind": "checkedReadSpanAccess", "type": "f64", "range": s.span(s.file, node), "view": s.viewReference(e.Expression), "index": s.number(e.ArgumentExpression, "u32", "index")}
	s.counts["viewReads"]++
	return ao23Map{"kind": "readSpanUse", "range": s.span(s.file, statement), "value": value}
}
func (s *rs26Scope) firstFunctions(headers []*ao23Header) []any {
	ao23Want(s.t, len(headers) == 2 && headers[0].node.Name().Text() == "mutate" && headers[1].node.Name().Text() == "entry", "complete two-function original first tracer")
	rows := []any{}
	for j, h := range headers {
		s.bindings = map[*ast.Symbol]ao23Binding{}
		s.views = map[*ast.Symbol]string{}
		for i, sym := range h.signature.Parameters() {
			s.bindings[sym] = ao23Binding{h.parameters[i]["id"].(string), h.kinds[i]}
		}
		nodes := h.node.AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
		want := 2
		if j == 1 {
			want = 6
		}
		ao23Want(s.t, len(nodes) == want, "complete first tracer original body")
		bindings, body := []any{}, []any{}
		for i, node := range nodes {
			var row ao23Map
			switch node.Kind {
			case ast.KindVariableStatement:
				var binding ao23Map
				if i == 0 {
					binding, row = s.local(node)
				} else {
					binding, row = s.viewLocal(node)
				}
				bindings = append(bindings, binding)
			case ast.KindExpressionStatement:
				expression := node.AsExpressionStatement().Expression
				if expression.Kind == ast.KindBinaryExpression {
					row = s.mutation(node)
				} else if expression.Kind == ast.KindCallExpression {
					row = ao23Map{"kind": "call", "range": s.span(s.file, node), "value": s.directCall(expression)}
				} else {
					row = s.viewRead(node)
				}
			case ast.KindReturnStatement:
				ao23Want(s.t, i == len(nodes)-1, "single final original f64 literal return")
				value := node.AsReturnStatement().Expression
				ao23Want(s.t, s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(value), s.check.GetNumberType()), "actual f64 return assignment")
				row = ao23Map{"kind": "return", "range": s.span(s.file, node), "value": s.number(value, "f64", "return")}
			default:
				s.t.Fatalf("uncovered original first statement %s", node.Kind)
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
