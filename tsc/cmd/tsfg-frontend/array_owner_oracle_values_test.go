// SPDX-License-Identifier: Apache-2.0
// Values are reconstructed only from original checker objects and original tokens.
package main

import (
	"fmt"
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/jsnum"
	"github.com/microsoft/typescript-go/internal/scanner"
	"math"
	"strconv"
	"strings"
)

func (s *ao23Scope) reference(node *ast.Node, kind string) ao23Map {
	ao23Want(s.t, node != nil && node.Kind == ast.KindIdentifier, "atomic original binding reference")
	sym := s.check.GetSymbolAtLocation(node)
	b, ok := s.bindings[sym]
	ao23Want(s.t, sym != nil && ok && b.kind == kind, "original same-function nominal binding, not name/type guessing")
	if kind == "array.f64" {
		s.arrayType(s.check.GetTypeAtLocation(node), false)
	} else {
		ao23Want(s.t, s.check.GetTypeAtLocation(node) == s.check.GetNumberType(), "original scalar reference Number")
	}
	return ao23Map{"kind": "reference", "binding": b.id, "flowType": kind, "range": s.span(s.file, node)}
}
func (s *ao23Scope) number(node *ast.Node, kind, role string) ao23Map {
	atom, sign := node, ""
	if node.Kind == ast.KindPrefixUnaryExpression {
		p := node.AsPrefixUnaryExpression()
		atom = p.Operand
		if p.Operator == ast.KindMinusToken {
			sign = "-"
		} else if p.Operator == ast.KindPlusToken {
			sign = "+"
		} else {
			s.t.Fatal("finite signed numeric literal")
		}
	}
	ao23Want(s.t, atom != nil && atom.Kind == ast.KindNumericLiteral, "original numeric literal")
	text := s.file.Text()
	token := sign + text[scanner.SkipTrivia(text, atom.Pos()):atom.End()]
	typ := s.check.GetTypeAtLocation(atom)
	ao23Want(s.t, typ != nil && typ.IsNumberLiteral() && s.check.GetBaseTypeOfLiteralType(typ) == s.check.GetNumberType(), "official NumberLiteral and Number base")
	number, ok := typ.AsLiteralType().Value().(jsnum.Number)
	ao23Want(s.t, ok && !number.IsInf() && !number.IsNaN(), "official finite numeric value")
	if kind == "f64" {
		bits := math.Float64bits(math.Abs(float64(number)))
		if sign == "-" {
			bits |= uint64(1) << 63
		}
		parsed, err := strconv.ParseFloat(strings.ReplaceAll(token, "_", ""), 64)
		ao23Want(s.t, err == nil && math.Float64bits(parsed) == bits, "independent standard IEEE conversion agrees with original literal")
		return ao23Map{"kind": "numericLiteral", "type": "f64", "range": s.span(s.file, node), "token": token, "bits": fmt.Sprintf("%016x", bits)}
	}
	ao23Want(s.t, sign == "" && node == atom, "unsigned contextual u32 literal")
	n := float64(number)
	maximum := uint64(4294967295)
	name := "u32Literal"
	if role == "index" {
		maximum = 4294967294
		name = "indexLiteral"
	} else if role == "length" {
		name = "lengthLiteral"
	}
	ao23Want(s.t, n >= 0 && n <= float64(maximum) && math.Trunc(n) == n, "context-specific exact integer domain")
	return ao23Map{"kind": name, "type": "u32", "range": s.span(s.file, node), "token": token, "decimal": strconv.FormatUint(uint64(n), 10)}
}
func (s *ao23Scope) scalar(node *ast.Node, kind, role string) ao23Map {
	ao23Want(s.t, node != nil, "complete original scalar expression")
	if node.Kind == ast.KindIdentifier {
		return s.reference(node, kind)
	}
	if node.Kind == ast.KindNumericLiteral || node.Kind == ast.KindPrefixUnaryExpression {
		return s.number(node, kind, role)
	}
	if node.Kind == ast.KindElementAccessExpression {
		ao23Want(s.t, kind == "f64" && (role == "return" || role == "rhs"), "checked read only in allowed f64 role")
		return s.element(node, false)
	}
	if node.Kind == ast.KindCallExpression {
		ao23Want(s.t, kind == "f64" && role == "return", "only final return direct call")
		return s.directCall(node)
	}
	s.t.Fatalf("unhandled original scalar expression %s", node.Kind)
	return nil
}
func (s *ao23Scope) element(node *ast.Node, target bool) ao23Map {
	ao23Want(s.t, node.Kind == ast.KindElementAccessExpression, "original element access")
	e := node.AsElementAccessExpression()
	ao23Want(s.t, e.QuestionDotToken == nil && e.ArgumentExpression != nil, "ordinary complete element access")
	ao23Want(s.t, s.check.GetTypeAtLocation(node) == s.check.GetNumberType(), "actual element Number type")
	name := "checkedElementAccess"
	if target {
		name = "elementTarget"
	}
	return ao23Map{"kind": name, "type": "f64", "range": s.span(s.file, node), "array": s.reference(e.Expression, "array.f64"), "index": s.scalar(e.ArgumentExpression, "u32", "index")}
}
func (s *ao23Scope) directCall(node *ast.Node) ao23Map {
	c := node.AsCallExpression()
	ao23Want(s.t, c.Expression != nil && c.Expression.Kind == ast.KindIdentifier && c.QuestionDotToken == nil && c.TypeArguments == nil, "ordinary direct named call")
	sym := s.check.GetSymbolAtLocation(c.Expression)
	h := s.headers[sym]
	ao23Want(s.t, h != nil && h.node.Name().Text() == c.Expression.Text(), "actual complete same-source declaration target")
	resolved := s.check.GetResolvedSignature(node)
	ao23Want(s.t, resolved != nil && resolved.Declaration() == h.node && len(resolved.Parameters()) == len(h.parameters) && len(c.Arguments.Nodes) == len(h.parameters) && s.check.GetReturnTypeOfSignature(resolved) == s.check.GetNumberType() && s.check.GetTypeAtLocation(node) == s.check.GetNumberType(), "original exact resolved call signature")
	args := []any{}
	for i, arg := range c.Arguments.Nodes {
		ao23Want(s.t, resolved.Parameters()[i] == h.signature.Parameters()[i], "resolved original parameter order")
		if h.kinds[i] == "array.f64" {
			args = append(args, s.reference(arg, "array.f64"))
		} else {
			ao23Want(s.t, arg.Kind == ast.KindIdentifier || arg.Kind == ast.KindNumericLiteral || arg.Kind == ast.KindPrefixUnaryExpression, "atomic scalar call argument")
			args = append(args, s.scalar(arg, h.kinds[i], "argument"))
		}
		ao23Want(s.t, s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(arg), s.check.GetTypeOfSymbol(resolved.Parameters()[i])), "official actual argument assignability")
	}
	s.counts["calls"]++
	return ao23Map{"kind": "directCall", "type": "f64", "range": s.span(s.file, node), "callee": h.row["id"], "calleeRange": s.span(s.file, c.Expression), "arguments": args}
}
func (s *ao23Scope) arrayLiteral(node *ast.Node) ao23Map {
	ao23Want(s.t, node.Kind == ast.KindArrayLiteralExpression, "original array literal")
	items := node.AsArrayLiteralExpression().Elements.Nodes
	s.arrayType(s.check.GetTypeAtLocation(node), len(items) == 0)
	values := []any{}
	for _, item := range items {
		values = append(values, s.number(item, "f64", "element"))
	}
	r := s.span(s.file, node)
	s.counts["allocations"]++
	return ao23Map{"kind": "arrayLiteral", "type": "array.f64", "range": r, "allocationSite": ao23ID(s.t, "tsfg.array-facts.allocation-site.v1", ao23Map{"sourcePath": s.file.FileName(), "range": r}), "elements": values}
}
