// SPDX-License-Identifier: Apache-2.0
// Official-valid/private-invalid controls retain actual official objects, never a legal facts packet.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

func (s *rs26Scope) rs26bAdjacentNegativeHeaders() []*ao23Header {
	headers, witness := []*ao23Header{}, []any{}
	for _, n := range s.file.Statements.Nodes {
		ao23Want(s.t, n.Kind == ast.KindFunctionDeclaration && n.Name() != nil && (n.ModifierFlags() == 0 || n.ModifierFlags() == ast.ModifierFlagsExport), "complete original negative-control function")
		f, sym := n.AsFunctionDeclaration(), s.original(n)
		ao23Want(s.t, f.TypeParameters == nil && f.AsteriskToken == nil && f.FullSignature == nil && f.Body != nil && f.Body.Kind == ast.KindBlock, "actual ordinary negative-control function signature")
		s.annotationScalar(f.Type, "f64")
		sigs := s.check.GetSignaturesOfType(s.check.GetTypeOfSymbol(sym), checker.SignatureKindCall)
		ao23Want(s.t, len(sigs) == 1 && sigs[0].Declaration() == n && len(sigs[0].TypeParameters()) == 0 && sigs[0].ThisParameter() == nil && !sigs[0].HasRestParameter() && len(sigs[0].Parameters()) == len(f.Parameters.Nodes) && s.check.GetReturnTypeOfSignature(sigs[0]) == s.check.GetNumberType(), "real full signature before selected required-only gate")
		h := &ao23Header{node: n, symbol: sym, signature: sigs[0], row: ao23Map{"id": s.bindingID(s.file, n.Name())}}
		params := []any{}
		for i, pn := range f.Parameters.Nodes {
			p, actual := pn.AsParameterDeclaration(), s.original(pn)
			ao23Want(s.t, actual == h.signature.Parameters()[i] && pn.ModifierFlags() == 0 && p.Initializer == nil && p.DotDotDotToken == nil && p.Type.Kind == ast.KindTypeReference, "real original full parameter slot; optionality observed independently")
			r := p.Type.AsTypeReferenceNode()
			ao23Want(s.t, s.check.GetSymbolAtLocation(r.TypeName) == s.view && r.TypeArguments != nil && len(r.TypeArguments.Nodes) == 1, "negative annotation is actual pinned readonly interface")
			e := r.TypeArguments.Nodes[0]
			ao23Want(s.t, e.Kind == ast.KindTypeReference, "explicit original element annotation")
			eName := e.AsTypeReferenceNode().TypeName.Text()
			s.annotationScalar(e, eName)
			s.viewType(s.check.GetTypeFromTypeNode(p.Type), s.check.GetNumberType())
			actualType := s.check.GetTypeOfSymbol(actual)
			ao23Want(s.t, s.check.GetTypeAtLocation(pn.Name()) == actualType, "real original parameter use/declaration type identity")
			optional := p.QuestionToken != nil
			if optional {
				ao23Want(s.t, actualType.IsUnion() && len(actualType.Types()) == 2, "official optional parameter has complete view/undefined union")
				views, undefined := 0, 0
				for _, member := range actualType.Types() {
					if member.Flags() == checker.TypeFlagsUndefined {
						undefined++
					} else {
						s.viewType(member, s.check.GetNumberType())
						views++
					}
				}
				ao23Want(s.t, views == 1 && undefined == 1, "actual optional type includes exact pinned view and undefined")
			} else {
				s.viewType(actualType, s.check.GetNumberType())
			}
			elementSymbol := s.check.GetSymbolAtLocation(e.AsTypeReferenceNode().TypeName)
			params = append(params, ao23Map{"index": i, "binding": s.bindingID(s.file, pn.Name()), "range": s.span(s.file, pn), "nameRange": s.span(s.file, pn.Name()), "annotationRange": s.span(s.file, p.Type), "elementRange": s.span(s.file, e), "elementName": eName, "elementIsPinnedF64": elementSymbol == s.scalars["f64"], "elementIsPinnedU32": elementSymbol == s.scalars["u32"], "optional": optional, "officialViewNumberRepresentation": true, "originalSignatureSlot": true})
		}
		s.headers[sym], s.names[n.Name().Text()] = h, h
		headers = append(headers, h)
		witness = append(witness, ao23Map{"name": n.Name().Text(), "binding": h.row["id"], "range": s.span(s.file, n), "parameters": params, "declaredArity": len(params), "minimumArity": h.signature.MinArgumentCount(), "originalResolvedDeclaration": true})
	}
	s.proof["negativeHeaders"] = witness
	return headers
}

func (s *rs26Scope) rs26bAdjacentNegativeCall(n *ast.Node) ao23Map {
	ao23Want(s.t, n.Kind == ast.KindCallExpression, "whole original negative call")
	c := n.AsCallExpression()
	ao23Want(s.t, c.Expression.Kind == ast.KindIdentifier && c.TypeArguments == nil && c.QuestionDotToken == nil, "ordinary original negative invocation")
	h := s.headers[s.check.GetSymbolAtLocation(c.Expression)]
	ao23Want(s.t, h != nil && h.symbol == s.original(h.node), "actual complete negative callee Symbol")
	r := s.check.GetResolvedSignature(n)
	ao23Want(s.t, r != nil && r.Declaration() == h.node && len(r.Parameters()) == len(h.signature.Parameters()) && len(c.Arguments.Nodes) == len(r.Parameters()) && r.MinArgumentCount() == len(r.Parameters()) && s.check.GetReturnTypeOfSignature(r) == s.check.GetNumberType(), "actual required resolved negative invocation signature")
	args := []any{}
	for i, arg := range c.Arguments.Nodes {
		ao23Want(s.t, arg.Kind == ast.KindIdentifier && r.Parameters()[i] == h.signature.Parameters()[i] && len(r.Parameters()[i].Declarations) == 1 && r.Parameters()[i].Declarations[0] == h.node.AsFunctionDeclaration().Parameters.Nodes[i], "all original argument and full parameter declaration slots")
		sym := s.check.GetSymbolAtLocation(arg)
		ao23Want(s.t, sym != nil && len(sym.Declarations) == 1, "actual original negative argument Symbol")
		typ := s.check.GetTypeAtLocation(arg)
		kind := "readspan.f64-official-only"
		if s.check.IsArrayType(typ) {
			s.arrayType(typ, false)
			kind = "array.f64"
		} else {
			s.viewType(typ, s.check.GetNumberType())
		}
		s.viewType(s.check.GetTypeOfSymbol(r.Parameters()[i]), s.check.GetNumberType())
		assignable := s.check.IsTypeAssignableTo(typ, s.check.GetTypeOfSymbol(r.Parameters()[i]))
		ao23Want(s.t, assignable, "official structural Array/View argument assignability really succeeds")
		args = append(args, ao23Map{"index": i, "range": s.span(s.file, arg), "binding": s.bindingID(s.file, sym.Declarations[0].Name()), "declarationRange": s.span(s.file, sym.Declarations[0]), "actualTypeOrigin": kind, "nominalViewRegistered": s.views[sym] != "", "assignable": assignable, "parameterBinding": s.bindingID(s.file, r.Parameters()[i].Declarations[0].Name()), "originalParameterSlot": true})
	}
	return ao23Map{"range": s.span(s.file, n), "target": h.row["id"], "targetDeclarationRange": s.span(s.file, h.node), "resolvedOriginalDeclaration": true, "arguments": args}
}

func (s *rs26Scope) rs26bAdjacentNegative(item rs26bAdjacentCase) ao23Map {
	headers := s.rs26bAdjacentNegativeHeaders()
	calls, functions, locals := []any{}, []any{}, []any{}
	for _, h := range headers {
		s.bindings, s.views = map[*ast.Symbol]ao23Binding{}, map[*ast.Symbol]string{}
		for _, sym := range h.signature.Parameters() {
			pn := sym.Declarations[0].AsParameterDeclaration()
			if pn.QuestionToken == nil && pn.Type.AsTypeReferenceNode().TypeArguments.Nodes[0].AsTypeReferenceNode().TypeName.Text() == "f64" {
				s.views[sym] = s.bindingID(s.file, sym.Declarations[0].Name())
			}
		}
		body := h.node.AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
		bodyRows := []any{}
		for i, n := range body {
			bodyRows = append(bodyRows, ao23Map{"kind": n.Kind.String(), "range": s.span(s.file, n)})
			if n.Kind == ast.KindVariableStatement {
				decls := n.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
				ao23Want(s.t, len(decls) == 1, "one original negative const declarator")
				d, sym := decls[0].AsVariableDeclaration(), s.original(decls[0])
				if d.Initializer.Kind == ast.KindArrayLiteralExpression {
					binding, value := s.local(n)
					locals = append(locals, ao23Map{"binding": binding, "statement": value, "nominalArrayRegistered": true})
				} else {
					ao23Want(s.t, item.ID == "nominal-const-annotation-from-array" && d.Initializer.Kind == ast.KindIdentifier, "unique nominal destination control initializer")
					s.rs26bViewAnnotation(d.Type)
					s.viewType(s.check.GetTypeOfSymbol(sym), s.check.GetNumberType())
					init := s.reference(d.Initializer, "array.f64")
					assignable := s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(d.Initializer), s.check.GetTypeOfSymbol(sym))
					ao23Want(s.t, assignable && s.views[sym] == "", "official annotation assignment succeeds without manufacturing nominal view provenance")
					locals = append(locals, ao23Map{"binding": s.bindingID(s.file, decls[0].Name()), "range": s.span(s.file, decls[0]), "annotationRange": s.span(s.file, d.Type), "initializer": init, "destinationPinnedView": true, "assignable": assignable, "nominalViewRegistered": false})
				}
			} else {
				ao23Want(s.t, n.Kind == ast.KindReturnStatement && i == len(body)-1, "complete negative-control original final return")
				v := n.AsReturnStatement().Expression
				ao23Want(s.t, s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(v), s.check.GetReturnTypeOfSignature(h.signature)), "real negative return assignment")
				if v.Kind == ast.KindCallExpression {
					calls = append(calls, s.rs26bAdjacentNegativeCall(v))
				} else {
					s.number(v, "f64", "return")
				}
			}
		}
		functions = append(functions, ao23Map{"name": h.node.Name().Text(), "range": s.span(s.file, h.node), "body": bodyRows})
	}
	ao23Want(s.t, len(headers) == 2, "complete original negative two-function table")
	for _, c := range calls {
		ao23Want(s.t, c.(ao23Map)["target"] == headers[0].row["id"], "all negative direct-call edges target first leaf; complete acyclic graph")
	}
	ao23Want(s.t, len(calls) <= 1, "complete bounded negative direct DAG")
	s.proof["negativeDAG"] = ao23Map{"sourceFunctions": len(headers), "calls": len(calls), "acyclic": true, "maximumDepth": len(calls) + 1}
	s.proof["negativeBodyInventory"], s.proof["negativeCalls"], s.proof["negativeLocals"] = functions, calls, locals
	s.proof["negativeExports"] = s.actualExports(headers)
	var primary *ast.Node
	switch item.ID {
	case "nominal-array-as-view":
		h := headers[1].node.AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
		primary = h[len(h)-1].AsReturnStatement().Expression.AsCallExpression().Arguments.Nodes[0]
		arg := calls[0].(ao23Map)["arguments"].([]any)[0].(ao23Map)
		ao23Want(s.t, arg["actualTypeOrigin"] == "array.f64" && arg["nominalViewRegistered"] == false, "official assignable Array remains nominally distinct")
	case "nominal-const-annotation-from-array":
		n := headers[1].node.AsFunctionDeclaration().Body.AsBlock().Statements.Nodes[1]
		primary = n.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0].AsVariableDeclaration().Initializer
		ao23Want(s.t, calls[0].(ao23Map)["arguments"].([]any)[0].(ao23Map)["nominalViewRegistered"] == false, "bad initializer cannot mint later view argument provenance")
	case "explicit-view-u32-rejected":
		primary = headers[0].node.AsFunctionDeclaration().Parameters.Nodes[0]
		e := primary.AsParameterDeclaration().Type.AsTypeReferenceNode().TypeArguments.Nodes[0]
		ao23Want(s.t, s.check.GetSymbolAtLocation(e.AsTypeReferenceNode().TypeName) == s.scalars["u32"] && s.scalars["u32"] != s.scalars["f64"] && headers[0].signature.MinArgumentCount() == 1, "exact wrong nominal element despite same Number representation")
	case "optional-view-parameter-rejected":
		primary = headers[0].node
		ao23Want(s.t, headers[0].signature.MinArgumentCount() == 0 && len(headers[0].signature.Parameters()) == 1 && primary.AsFunctionDeclaration().Parameters.Nodes[0].AsParameterDeclaration().QuestionToken != nil, "actual optional header fails required arity before parameter conversion")
	default:
		s.t.Fatal("unexpected negative control")
	}
	where := s.span(s.file, primary)
	ao23Want(s.t, where == item.AnchorCandidates[item.SourceExpected.PrimaryAnchor].Range && item.SourceExpected.ExpectedFactsWrites == 0, "exact negative source primary from actual original AST")
	return ao23Map{"kind": "official-valid-private-invalid-witness", "actualSourceRefusalObserved": false, "selectedPacketConstructed": false, "expectedSourceError": ao23Map{"code": item.SourceExpected.Code, "ruleId": item.SourceExpected.RuleID, "phase": item.SourceExpected.Phase, "category": item.SourceExpected.Category, "severity": item.SourceExpected.Severity, "primary": ao23Map{"sourceId": s.file.FileName(), "start": where.Start, "end": where.End}}, "primaryNodeKind": primary.Kind.String(), "primaryRange": where, "expectedFactsWrites": 0}
}
