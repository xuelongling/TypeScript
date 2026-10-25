// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/scanner"
)

// This oracle visits the actual official Program and resolves every occurrence
// to its own defining Symbol. It never invokes the owned projection, scalar
// hinting, presence analysis or offset helpers to manufacture expected facts.
type exceptionNumericOracle struct {
	t     *testing.T
	file  *ast.SourceFile
	check *checker.Checker
	rows  map[string]symbol
	ids   map[*ast.Symbol]string
}

func (o *exceptionNumericOracle) at(n *ast.Node, token bool) span {
	start := n.Pos()
	if token {
		start = scanner.SkipTrivia(o.file.Text(), start)
	}
	s := o.file.Text()
	want := span{len(utf16.Encode([]rune(s[:start]))), len(utf16.Encode([]rune(s[:n.End()])))}
	if want != (span{o.file.GetPositionMap().UTF8ToUTF16(start), o.file.GetPositionMap().UTF8ToUTF16(n.End())}) {
		o.t.Fatal("independent UTF16 offset")
	}
	return want
}

func (o *exceptionNumericOracle) static(t *checker.Type) string {
	if t == o.check.GetUnknownType() {
		return "unknown"
	}
	if t.Flags()&checker.TypeFlagsUndefined != 0 {
		return "undefined"
	}
	if t.IsBooleanLike() {
		return "bool"
	}
	if t.Alias() != nil && t.Alias().Symbol() != nil {
		return t.Alias().Symbol().Name
	}
	return ""
}

func (o *exceptionNumericOracle) bind(n *ast.Node, actual symbol, kind, typ string, writable bool) error {
	upstream := o.check.GetSymbolAtLocation(n.Name())
	if upstream == nil || len(upstream.Declarations) != 1 || upstream.Declarations[0] != n || o.ids[upstream] != "" || actual.ID == "" || actual.Kind != kind || actual.Type != typ || actual.Writable != writable || actual.Name != n.Name().Text() || actual.Range != o.at(n.Name(), false) || !reflect.DeepEqual(o.rows[actual.ID], actual) {
		return fmt.Errorf("original %s defining symbol/owned row", kind)
	}
	o.ids[upstream] = actual.ID
	return nil
}

func (o *exceptionNumericOracle) expr(n *ast.Node, e *expression, token bool) error {
	if n == nil || e == nil {
		return fmt.Errorf("missing original expression")
	}
	kind, typ := "", ""
	wantRange := o.at(n, token)
	switch n.Kind {
	case ast.KindAsExpression, ast.KindTypeAssertionExpression:
		var value, annotation *ast.Node
		op := "as"
		if n.Kind == ast.KindAsExpression {
			value, annotation = n.AsAsExpression().Expression, n.AsAsExpression().Type
		} else {
			value, annotation = n.AsTypeAssertion().Expression, n.AsTypeAssertion().Type
			op = "type-assertion"
		}
		view := o.check.GetTypeFromTypeNode(annotation)
		kind, typ, wantRange = "assertion", o.static(view), o.at(n, true)
		if o.check.GetTypeAtLocation(n) != view || e.Operator != op || e.Assertion == nil || e.Assertion.Annotation != o.at(annotation, true) {
			return fmt.Errorf("original assertion view and annotation")
		}
		start, end := scanner.SkipTrivia(o.file.Text(), value.End()), 0
		if op == "as" {
			end = start + 2
		} else {
			start = scanner.SkipTrivia(o.file.Text(), n.Pos())
			end = scanner.SkipTrivia(o.file.Text(), annotation.End()) + 1
		}
		want := span{len(utf16.Encode([]rune(o.file.Text()[:start]))), len(utf16.Encode([]rune(o.file.Text()[:end])))}
		if e.Assertion.Operator != want {
			return fmt.Errorf("original assertion operator range")
		}
		if err := o.expr(value, e.Operand, false); err != nil {
			return err
		}
	case ast.KindIdentifier:
		upstream := o.check.GetSymbolAtLocation(n)
		if n.Text() == "undefined" && upstream != nil && len(upstream.Declarations) == 0 && o.check.GetTypeAtLocation(n).Flags()&checker.TypeFlagsUndefined != 0 {
			kind, typ, wantRange = "undefined", "undefined", o.at(n, true)
			if e.Symbol != "" || e.Absence != "" {
				return fmt.Errorf("builtin undefined must not borrow a declared Binding or typed absence")
			}
		} else {
			kind = "reference"
			typ = o.static(o.check.GetTypeAtLocation(n))
			if e.Symbol == "" || e.Symbol != o.ids[upstream] {
				return fmt.Errorf("original reference resolves its own actual declaration")
			}
		}
	case ast.KindTrueKeyword, ast.KindFalseKeyword:
		kind, typ = "boolean", "bool"
		if e.Boolean == nil || *e.Boolean != (n.Kind == ast.KindTrueKeyword) {
			return fmt.Errorf("original bool token")
		}
	case ast.KindParenthesizedExpression:
		kind, wantRange = "group", o.at(n, true)
		if err := o.expr(n.AsParenthesizedExpression().Expression, e.Operand, true); err != nil {
			return err
		}
		typ = e.Operand.Type
	case ast.KindPrefixUnaryExpression:
		kind, typ, wantRange = "unary", "i32", o.at(n, true)
		prefix := n.AsPrefixUnaryExpression()
		start := scanner.SkipTrivia(o.file.Text(), n.Pos())
		if e.Operator != o.file.Text()[start:start+1] {
			return fmt.Errorf("actual original unary operator")
		}
		if prefix.Operator == ast.KindExclamationToken {
			typ = "bool"
		}
		if err := o.expr(prefix.Operand, e.Operand, true); err != nil {
			return err
		}
	case ast.KindBinaryExpression:
		binary := n.AsBinaryExpression()
		kind, typ, wantRange = "binary", "i32", o.at(n, true)
		operator := o.file.Text()[scanner.SkipTrivia(o.file.Text(), binary.OperatorToken.Pos()):binary.OperatorToken.End()]
		if e.Operator != operator {
			return fmt.Errorf("actual original binary operator")
		}
		if strings.Contains(" < <= > >= === !== ", " "+operator+" ") {
			typ = "bool"
		}
		if operator == "&&" || operator == "||" {
			kind, typ = "shortCircuit", "bool"
		}
		if operator == "=" {
			kind = "write"
			typ = o.static(o.check.GetTypeAtLocation(binary.Right))
		}
		if err := o.expr(binary.Left, e.Left, true); err != nil {
			return err
		}
		if err := o.expr(binary.Right, e.Right, true); err != nil {
			return err
		}
	case ast.KindCallExpression:
		call := n.AsCallExpression()
		kind, typ = "call", "i32"
		target := o.check.GetSymbolAtLocation(call.Expression)
		resolved := o.check.GetResolvedSignature(n)
		if target == nil || len(target.Declarations) != 1 || resolved == nil || resolved.Declaration() != target.Declarations[0] || e.Target != o.ids[target] || e.Target == "" || e.Signature == nil || e.Signature.ReturnType != "i32" || e.Arguments == nil || len(*e.Arguments) != len(call.Arguments.Nodes) {
			return fmt.Errorf("actual selected call and complete ordered arguments")
		}
		for i, arg := range call.Arguments.Nodes {
			if err := o.expr(arg, &(*e.Arguments)[i], false); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected original expression %v", n.Kind)
	}
	if e.Kind != kind || e.Type != typ || e.Range != wantRange {
		return fmt.Errorf("original %s kind/static type/UTF16", kind)
	}
	if kind != "assertion" && e.Assertion != nil {
		return fmt.Errorf("assertion fields attached to another variant")
	}
	return nil
}

func (o *exceptionNumericOracle) block(nodes []*ast.Node, facts []statement) error {
	if len(nodes) != len(facts) {
		return fmt.Errorf("complete ordered statements")
	}
	for i, n := range nodes {
		s := facts[i]
		if n.Kind != ast.KindVariableStatement && s.Range != o.at(n, false) {
			return fmt.Errorf("original statement UTF16")
		}
		switch n.Kind {
		case ast.KindTryStatement:
			a := n.AsTryStatement()
			if s.Kind != "try" || s.Exception == nil || s.Exception.TryBlock.Range != o.at(a.TryBlock, false) {
				return fmt.Errorf("original try region")
			}
			if err := o.block(a.TryBlock.AsBlock().Statements.Nodes, s.Exception.TryBlock.Body); err != nil {
				return err
			}
			if (a.CatchClause == nil) != (s.Exception.CatchClause == nil) || (a.FinallyBlock == nil) != (s.Exception.FinallyBlock == nil) {
				return fmt.Errorf("present original catch/finally")
			}
			if a.CatchClause != nil {
				c, owned := a.CatchClause.AsCatchClause(), s.Exception.CatchClause
				if owned.Binding == nil || owned.Range != o.at(a.CatchClause, false) || owned.Block.Range != o.at(c.Block, false) || owned.DeclarationRange == nil || *owned.DeclarationRange != o.at(c.VariableDeclaration, false) {
					return fmt.Errorf("original current catch Binding/ranges")
				}
				if err := o.bind(c.VariableDeclaration, *owned.Binding, "local", "unknown", true); err != nil {
					return err
				}
				if err := o.block(c.Block.AsBlock().Statements.Nodes, owned.Block.Body); err != nil {
					return err
				}
			}
			if a.FinallyBlock != nil {
				if s.Exception.FinallyBlock.Range != o.at(a.FinallyBlock, false) {
					return fmt.Errorf("original finally range")
				}
				if err := o.block(a.FinallyBlock.AsBlock().Statements.Nodes, s.Exception.FinallyBlock.Body); err != nil {
					return err
				}
			}
		case ast.KindVariableStatement:
			list := n.AsVariableStatement().DeclarationList.AsVariableDeclarationList()
			if len(list.Declarations.Nodes) != 1 || s.Kind != "variable" || s.Symbol == nil {
				return fmt.Errorf("original scalar declaration")
			}
			decl := list.Declarations.Nodes[0]
			v := decl.AsVariableDeclaration()
			// Public variable rows anchor the declaration, not its const/let statement.
			if s.Range != o.at(decl, false) {
				return fmt.Errorf("original declaration UTF16")
			}
			if err := o.expr(v.Initializer, s.Initializer, false); err != nil {
				return err
			}
			typ := o.static(o.check.GetTypeOfSymbol(o.check.GetSymbolAtLocation(decl.Name())))
			if err := o.bind(decl, *s.Symbol, "local", typ, n.AsVariableStatement().DeclarationList.Flags&ast.NodeFlagsLet != 0); err != nil {
				return err
			}
		case ast.KindReturnStatement, ast.KindThrowStatement:
			kind := "return"
			var value *ast.Node
			if n.Kind == ast.KindThrowStatement {
				kind, value = "throw", n.AsThrowStatement().Expression
			} else {
				value = n.AsReturnStatement().Expression
			}
			if s.Kind != kind {
				return fmt.Errorf("original return/throw direction")
			}
			if err := o.expr(value, s.Expression, false); err != nil {
				return err
			}
		case ast.KindExpressionStatement:
			if s.Kind == "assignment" {
				b := n.AsExpressionStatement().Expression.AsBinaryExpression()
				if err := o.expr(b.Left, s.Target, false); err != nil {
					return err
				}
				if err := o.expr(b.Right, s.Value, false); err != nil {
					return err
				}
			} else {
				if s.Kind != "expression" {
					return fmt.Errorf("original discarded expression")
				}
				if err := o.expr(n.AsExpressionStatement().Expression, s.Expression, false); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unexpected original statement %v", n.Kind)
		}
	}
	return nil
}

func exceptionNumericFactsOracle(t *testing.T, r request, facts response) error {
	t.Helper()
	p := throwBoundaryProgramForTest(t, r)
	throwBoundaryDiagnosticsForTest(t, p)
	file := p.GetSourceFile(r.RootFile)
	check, release := p.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	o := exceptionNumericOracle{t: t, file: file, check: check, rows: map[string]symbol{}, ids: map[*ast.Symbol]string{}}
	if facts.Status != "ok" || len(facts.Diagnostics) != 0 || len(file.Statements.Nodes) != len(facts.Functions) || len(p.GetSourceFiles()) != len(r.Files) {
		return fmt.Errorf("complete official Program closure")
	}
	for _, s := range facts.Symbols {
		if s.ID == "" || o.rows[s.ID].ID != "" {
			return fmt.Errorf("distinct symbol rows")
		}
		o.rows[s.ID] = s
	}
	for i, n := range file.Statements.Nodes {
		f := facts.Functions[i]
		if n.Kind != ast.KindFunctionDeclaration || f.Name != n.Name().Text() || f.ReturnType != "i32" || f.Range != o.at(n, false) {
			return fmt.Errorf("complete ordered function table")
		}
		if err := o.bind(n, o.rows[f.Symbol], "function", "i32", false); err != nil {
			return err
		}
	}
	for i, n := range file.Statements.Nodes {
		f, decl := facts.Functions[i], n.AsFunctionDeclaration()
		if len(decl.Parameters.Nodes) != len(f.Parameters) || len(f.ParameterInitializers) != len(f.Parameters) {
			return fmt.Errorf("complete own parameters/defaults")
		}
		for j, param := range decl.Parameters.Nodes {
			if err := o.bind(param, f.Parameters[j], "parameter", "i32", true); err != nil {
				return err
			}
			shape := f.Parameters[j].Presence
			if f.ParameterInitializers[j] != nil || shape == nil || shape.Undefined || shape.Null || shape.Optional || shape.Default || shape.Range != o.at(param, false) || shape.Annotation != o.at(param.AsParameterDeclaration().Type, true) {
				return fmt.Errorf("original required parameter annotation")
			}
		}
		if err := o.block(decl.Body.AsBlock().Statements.Nodes, f.Body); err != nil {
			return err
		}
	}
	if len(o.ids) != len(facts.Symbols) {
		return fmt.Errorf("no missing or extra declaration rows")
	}
	return nil
}
