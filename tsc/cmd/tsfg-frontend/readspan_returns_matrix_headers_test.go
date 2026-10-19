// SPDX-License-Identifier: Apache-2.0
// Legacy scalar-only builders stay fixed; this official witness covers view returns.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

type rs26cOfficialScope struct {
	*rs26Scope
	current string
	edges   map[string][]string
}

func (s *rs26cOfficialScope) result(typ *checker.Type, kind string) {
	if kind == "readspan.f64" {
		s.viewType(typ, s.check.GetNumberType())
		return
	}
	ao23Want(s.t, kind == "f64" && typ == s.check.GetNumberType(), "actual f64 result representation")
}

func (s *rs26cOfficialScope) headers26c() []*ao23Header {
	out, proof := []*ao23Header{}, []any{}
	for _, n := range s.file.Statements.Nodes {
		ao23Want(s.t, n.Kind == ast.KindFunctionDeclaration && n.Name() != nil && n.Name().Kind == ast.KindIdentifier && (n.ModifierFlags() == 0 || n.ModifierFlags() == ast.ModifierFlagsExport), "complete original ordinary function")
		f := n.AsFunctionDeclaration()
		ao23Want(s.t, f.TypeParameters == nil && f.AsteriskToken == nil && f.FullSignature == nil && f.Parameters != nil && f.Body != nil && f.Body.Kind == ast.KindBlock, "original required nongeneric function body")
		sym := s.original(n)
		ao23Want(s.t, s.names[n.Name().Text()] == nil, "unique original function name")
		sigs := s.check.GetSignaturesOfType(s.check.GetTypeOfSymbol(sym), checker.SignatureKindCall)
		ao23Want(s.t, len(sigs) == 1 && sigs[0].Declaration() == n && len(sigs[0].Parameters()) == len(f.Parameters.Nodes) && sigs[0].MinArgumentCount() == len(f.Parameters.Nodes) && len(sigs[0].TypeParameters()) == 0 && sigs[0].ThisParameter() == nil && !sigs[0].HasRestParameter(), "sole complete required original signature")
		kind := "f64"
		var returnElement *ast.Node
		if f.Type != nil && f.Type.Kind == ast.KindTypeReference && s.check.GetSymbolAtLocation(f.Type.AsTypeReferenceNode().TypeName) == s.view {
			kind, returnElement = "readspan.f64", s.rs26bViewAnnotation(f.Type)
			ao23Want(s.t, n.ModifierFlags() == 0, "view-return helper is private")
		} else {
			s.annotationScalar(f.Type, "f64")
		}
		s.result(s.check.GetReturnTypeOfSignature(sigs[0]), kind)
		h := &ao23Header{node: n, symbol: sym, signature: sigs[0], parameters: []ao23Map{}, kinds: []string{}}
		slots := []any{}
		for i, pn := range f.Parameters.Nodes {
			p := pn.AsParameterDeclaration()
			ao23Want(s.t, pn.Name() != nil && pn.Name().Kind == ast.KindIdentifier && pn.ModifierFlags() == 0 && pn.Name().Text() != "this" && p.Initializer == nil && p.QuestionToken == nil && p.DotDotDotToken == nil, "ordinary required original parameter")
			actual := s.original(pn)
			ao23Want(s.t, actual == h.signature.Parameters()[i], "original full parameter slot Symbol")
			pkind := "readspan.f64"
			var element *ast.Node
			if p.Type != nil && p.Type.Kind == ast.KindTypeReference && s.check.GetSymbolAtLocation(p.Type.AsTypeReferenceNode().TypeName) == s.view {
				element = s.rs26bViewAnnotation(p.Type)
				s.viewType(s.check.GetTypeOfSymbol(actual), s.check.GetNumberType())
				s.counts["viewParameters"]++
			} else {
				pkind = s.annotation(p.Type)
				if pkind == "array.f64" {
					element = p.Type.AsArrayTypeNode().ElementType
					s.arrayType(s.check.GetTypeOfSymbol(actual), false)
				} else {
					ao23Want(s.t, s.check.GetTypeOfSymbol(actual) == s.check.GetNumberType(), "actual scalar parameter representation")
				}
			}
			row := ao23Map{"id": s.bindingID(s.file, pn.Name()), "name": pn.Name().Text(), "kind": "parameter", "declaredType": pkind, "writable": pkind != "readspan.f64", "declarationRange": s.span(s.file, pn), "nameRange": s.span(s.file, pn.Name()), "annotationRange": s.span(s.file, p.Type)}
			if element != nil {
				row["elementAnnotationRange"] = s.span(s.file, element)
			}
			h.parameters, h.kinds = append(h.parameters, row), append(h.kinds, pkind)
			slots = append(slots, ao23Map{"index": i, "binding": row["id"], "declaredType": pkind, "signatureSymbolEqualsOriginal": true})
			s.counts["parameters"]++
		}
		h.row = ao23Map{"id": s.bindingID(s.file, n.Name()), "name": n.Name().Text(), "sourcePath": s.file.FileName(), "declarationRange": s.span(s.file, n), "nameRange": s.span(s.file, n.Name()), "returnType": kind, "returnAnnotationRange": s.span(s.file, f.Type), "parameters": h.parameters, "bindings": []any{}, "body": []any{}}
		if returnElement != nil {
			h.row["returnElementAnnotationRange"] = s.span(s.file, returnElement)
		}
		s.headers[sym], s.names[n.Name().Text()] = h, h
		s.edges[h.row["id"].(string)] = []string{}
		out = append(out, h)
		proof = append(proof, ao23Map{"name": n.Name().Text(), "binding": h.row["id"], "returnType": kind, "parameters": slots, "requiredArity": len(slots), "originalDeclaration": true})
	}
	s.counts["functions"], s.proof["headers"] = len(out), proof
	return out
}

func (s *rs26cOfficialScope) call26c(n *ast.Node) ao23Map {
	ao23Want(s.t, n != nil && n.Kind == ast.KindCallExpression, "actual ordinary original call")
	c := n.AsCallExpression()
	ao23Want(s.t, c.Expression != nil && c.Expression.Kind == ast.KindIdentifier && c.QuestionDotToken == nil && c.TypeArguments == nil, "direct original named nongeneric call")
	sym := s.check.GetSymbolAtLocation(c.Expression)
	h := s.headers[sym]
	ao23Want(s.t, h != nil && sym == h.symbol && len(sym.Declarations) == 1 && sym.Declarations[0] == h.node && h.node.Name().Text() == c.Expression.Text(), "actual same-module callee Symbol and declaration")
	resolved := s.check.GetResolvedSignature(n)
	ao23Want(s.t, resolved != nil && resolved.Declaration() == h.node && len(resolved.Parameters()) == len(h.parameters) && len(c.Arguments.Nodes) == len(h.parameters) && resolved.MinArgumentCount() == len(h.parameters) && len(resolved.TypeParameters()) == 0 && resolved.ThisParameter() == nil && !resolved.HasRestParameter(), "complete actual resolved required signature")
	kind := h.row["returnType"].(string)
	s.result(s.check.GetReturnTypeOfSignature(resolved), kind)
	s.result(s.check.GetTypeAtLocation(n), kind)
	args, joins := []any{}, []any{}
	for i, arg := range c.Arguments.Nodes {
		p := resolved.Parameters()[i]
		ao23Want(s.t, p == h.signature.Parameters()[i] && len(p.Declarations) == 1 && p.Declarations[0] == h.node.AsFunctionDeclaration().Parameters.Nodes[i], "resolved original full slot declaration join")
		ao23Want(s.t, s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(arg), s.check.GetTypeOfSymbol(p)), "real official argument assignability")
		var row ao23Map
		switch h.kinds[i] {
		case "readspan.f64":
			s.viewType(s.check.GetTypeOfSymbol(p), s.check.GetNumberType())
			row = s.viewReference(arg)
		case "array.f64":
			row = s.reference(arg, "array.f64")
		default:
			ao23Want(s.t, arg.Kind == ast.KindIdentifier || arg.Kind == ast.KindNumericLiteral || arg.Kind == ast.KindPrefixUnaryExpression, "atomic scalar actual")
			row = s.scalar(arg, h.kinds[i], "argument")
		}
		args = append(args, row)
		joins = append(joins, ao23Map{"index": i, "binding": h.parameters[i]["id"], "argument": row, "assignable": true, "originalParameterDeclaration": true})
	}
	s.edges[s.current] = append(s.edges[s.current], h.row["id"].(string))
	calls, _ := s.proof["calls"].([]any)
	s.proof["calls"] = append(calls, ao23Map{"target": h.row["id"], "range": s.span(s.file, n), "returnType": kind, "arguments": joins, "resolvedOriginalDeclaration": true})
	s.counts["calls"]++
	return ao23Map{"kind": "directCall", "type": kind, "range": s.span(s.file, n), "callee": h.row["id"], "calleeRange": s.span(s.file, c.Expression), "arguments": args}
}

func (s *rs26cOfficialScope) dag26c() {
	active := map[string]bool{}
	var visit func(string, int)
	visit = func(id string, depth int) {
		edges, exists := s.edges[id]
		ao23Want(s.t, exists && !active[id] && depth <= 16, "actual statement/initializer/tail call DAG is complete and bounded")
		active[id] = true
		for _, target := range edges {
			visit(target, depth+1)
		}
		delete(active, id)
	}
	for id := range s.edges {
		visit(id, 1)
	}
	s.proof["completeCallDAG"] = s.edges
}
