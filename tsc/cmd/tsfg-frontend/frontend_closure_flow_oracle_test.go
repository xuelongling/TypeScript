// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

// Oracle identities are actual official Symbols, never adapter IDs or a target-set
// guess. These records remain test-local and are never submitted as success facts.
type closureFlowDecl struct {
	symbol, owner   *ast.Symbol
	name, kind, typ string
	writable        bool
	anchor          span
	signature       *functionValueWireSignature
}
type closureFlowExpr struct {
	kind, typ, operator          string
	anchor                       span
	symbol                       *ast.Symbol
	signature                    *functionValueWireSignature
	callee, left, right, operand *closureFlowExpr
	arguments                    []*closureFlowExpr
}
type closureFlowStmt struct {
	kind                                              string
	anchor                                            span
	symbol                                            *ast.Symbol
	initializer, expression, target, value, condition *closureFlowExpr
	body, thenBody, elseBody                          []*closureFlowStmt
}
type closureFlowFunction struct {
	symbol, parent *ast.Symbol
	name           string
	anchor         span
	parameters     []*ast.Symbol
	signature      *functionValueWireSignature
	body           []*closureFlowStmt
	captures       map[*ast.Symbol][2]bool
}
type closureFlowOracle struct {
	functions    []*closureFlowFunction
	declarations []*closureFlowDecl
	booleans     map[span]bool
}

// A fresh official Program receives only the original files. All four diagnostic
// families pass before Symbols, AST structure and resolved signatures are read.
func closureFlowOfficial(t *testing.T, r request) closureFlowOracle {
	t.Helper()
	contents := map[string]string{}
	for _, file := range r.Files {
		contents[file.Path] = file.Content
	}
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
			FileNames:       append([]string{r.RootFile}, r.Declarations.Paths...),
			CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext},
		}},
		Host: compiler.NewCompilerHost(r.SourceRoot, vfstest.FromMap(contents, true), r.SourceRoot, nil, nil), SingleThreaded: core.TSTrue,
	})
	file := program.GetSourceFile(r.RootFile)
	if file == nil || file.Text() != r.Files[0].Content {
		t.Fatal("official Program lost original source")
	}
	if d := program.GetSyntacticDiagnostics(t.Context(), nil); len(d) != 0 {
		t.Fatalf("official syntax diagnostics: %+v", d)
	}
	if d := program.GetSemanticDiagnostics(t.Context(), nil); len(d) != 0 {
		t.Fatalf("official semantic diagnostics: %+v", d)
	}
	if d := program.GetProgramDiagnostics(); len(d) != 0 {
		t.Fatalf("official program diagnostics: %+v", d)
	}
	if d := program.GetGlobalDiagnostics(t.Context()); len(d) != 0 {
		t.Fatalf("official global diagnostics: %+v", d)
	}
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	positions := file.GetPositionMap()
	anchor := func(n *ast.Node) span {
		return span{Start: positions.UTF8ToUTF16(n.Pos()), End: positions.UTF8ToUTF16(n.End())}
	}
	var typ func(*checker.Type) (string, *functionValueWireSignature)
	var signature func(*checker.Signature) *functionValueWireSignature
	signature = func(sig *checker.Signature) *functionValueWireSignature {
		if sig == nil || sig.Declaration() == nil || len(sig.TypeParameters()) != 0 || sig.ThisParameter() != nil || sig.HasRestParameter() || sig.MinArgumentCount() != len(sig.Parameters()) {
			t.Fatal("official signature is not the required non-generic callable")
		}
		want := &functionValueWireSignature{Parameters: []string{}}
		nested := make([]*functionValueWireSignature, len(sig.Parameters()))
		hasCallable := false
		for i, p := range sig.Parameters() {
			kind, child := typ(check.GetTypeOfSymbol(p))
			want.Parameters = append(want.Parameters, kind)
			nested[i], hasCallable = child, hasCallable || child != nil
		}
		if hasCallable {
			want.ParameterSignatures = nested
		}
		want.ReturnType, want.ReturnSignature = typ(check.GetReturnTypeOfSignature(sig))
		return want
	}
	typ = func(value *checker.Type) (string, *functionValueWireSignature) {
		if value != nil && value.IsBooleanLike() {
			return "bool", nil
		}
		sigs := check.GetSignaturesOfType(value, checker.SignatureKindCall)
		if len(sigs) != 1 {
			t.Fatal("official source lost its unique bool/callable type")
		}
		return "function", signature(sigs[0])
	}
	oracle := closureFlowOracle{booleans: map[span]bool{}}
	decls := map[*ast.Symbol]*closureFlowDecl{}
	functions := map[*ast.Symbol]*closureFlowFunction{}
	functionNodes := map[*closureFlowFunction]*ast.Node{}
	add := func(node *ast.Node, owner *ast.Symbol, kind string, writable bool) *ast.Symbol {
		symbol := check.GetSymbolAtLocation(node.Name())
		if symbol == nil || decls[symbol] != nil {
			t.Fatal("official omitted or merged distinct declarations")
		}
		payload, sig := typ(check.GetTypeOfSymbol(symbol))
		if kind == "function" {
			payload, sig = sig.ReturnType, nil
		}
		decl := &closureFlowDecl{symbol: symbol, owner: owner, name: node.Name().Text(), kind: kind, typ: payload, writable: writable, anchor: anchor(node.Name()), signature: sig}
		decls[symbol] = decl
		oracle.declarations = append(oracle.declarations, decl)
		return symbol
	}
	var collect func(*ast.Node, *ast.Symbol)
	collect = func(node *ast.Node, owner *ast.Symbol) {
		if node.Kind == ast.KindFunctionDeclaration {
			symbol := add(node, owner, "function", false)
			_, sig := typ(check.GetTypeAtLocation(node.Name()))
			fn := &closureFlowFunction{symbol: symbol, parent: owner, name: node.Name().Text(), anchor: anchor(node), signature: sig, captures: map[*ast.Symbol][2]bool{}}
			for _, p := range node.AsFunctionDeclaration().Parameters.Nodes {
				fn.parameters = append(fn.parameters, add(p, symbol, "parameter", true))
			}
			oracle.functions = append(oracle.functions, fn)
			functions[symbol], functionNodes[fn], owner = fn, node, symbol
		} else if node.Kind == ast.KindVariableStatement {
			list := node.AsVariableStatement().DeclarationList
			for _, declaration := range list.AsVariableDeclarationList().Declarations.Nodes {
				add(declaration, owner, "local", list.Flags&ast.NodeFlagsConst == 0)
			}
		}
		node.ForEachChild(func(child *ast.Node) bool { collect(child, owner); return false })
	}
	file.ForEachChild(func(node *ast.Node) bool { collect(node, nil); return false })
	for _, fn := range oracle.functions {
		access := func(symbol *ast.Symbol, write bool) {
			decl := decls[symbol]
			if decl == nil {
				t.Fatal("official expression resolved outside the original declarations")
			}
			if decl.owner == nil || decl.owner == fn.symbol {
				return
			}
			flags := fn.captures[symbol]
			flags[0], flags[1] = flags[0] || !write, flags[1] || write
			fn.captures[symbol] = flags
		}
		var expression func(*ast.Node, bool) *closureFlowExpr
		expression = func(node *ast.Node, write bool) *closureFlowExpr {
			if node == nil {
				return nil
			}
			kind, sig := typ(check.GetTypeAtLocation(node))
			want := &closureFlowExpr{typ: kind, signature: sig, anchor: anchor(node)}
			switch node.Kind {
			case ast.KindIdentifier:
				want.kind, want.symbol = "reference", check.GetSymbolAtLocation(node)
				if functions[want.symbol] != nil {
					want.kind = "functionReference"
				}
				access(want.symbol, write)
			case ast.KindTrueKeyword, ast.KindFalseKeyword:
				want.kind = "boolean"
				oracle.booleans[anchor(node)] = node.Kind == ast.KindTrueKeyword
			case ast.KindParenthesizedExpression:
				want.kind, want.operand = "group", expression(node.AsParenthesizedExpression().Expression, write)
			case ast.KindPrefixUnaryExpression:
				prefix := node.AsPrefixUnaryExpression()
				if prefix.Operator != ast.KindExclamationToken {
					t.Fatal("unexpected official unary fixture")
				}
				want.kind, want.operator, want.operand = "unary", "!", expression(prefix.Operand, false)
			case ast.KindBinaryExpression:
				binary := node.AsBinaryExpression()
				if binary.OperatorToken.Kind == ast.KindEqualsToken {
					want.kind, want.operator = "write", "="
				} else if binary.OperatorToken.Kind == ast.KindAmpersandAmpersandToken {
					want.kind, want.operator = "shortCircuit", "&&"
				} else {
					t.Fatal("unexpected official binary fixture")
				}
				want.left, want.right = expression(binary.Left, want.kind == "write"), expression(binary.Right, false)
			case ast.KindCallExpression:
				call := node.AsCallExpression()
				resolved := check.GetResolvedSignature(node)
				calleeTypes := check.GetSignaturesOfType(check.GetTypeAtLocation(call.Expression), checker.SignatureKindCall)
				if resolved == nil || len(calleeTypes) != 1 || resolved.Declaration() != calleeTypes[0].Declaration() {
					t.Fatal("official callee type and resolved signature disagree")
				}
				want.kind, want.signature = "indirectCall", signature(resolved)
				if symbol := check.GetSymbolAtLocation(call.Expression); call.Expression.Kind == ast.KindIdentifier && functions[symbol] != nil {
					if resolved.Declaration() != functionNodes[functions[symbol]] {
						t.Fatal("official direct signature lost its actual declaration")
					}
					want.kind, want.symbol = "call", symbol
				} else {
					want.callee = expression(call.Expression, false)
				}
				for _, argument := range call.Arguments.Nodes {
					want.arguments = append(want.arguments, expression(argument, false))
				}
			default:
				t.Fatalf("unexpected official expression kind %v", node.Kind)
			}
			return want
		}
		var statements func([]*ast.Node) []*closureFlowStmt
		statements = func(nodes []*ast.Node) []*closureFlowStmt {
			var body []*closureFlowStmt
			for _, node := range nodes {
				want := &closureFlowStmt{anchor: anchor(node)}
				switch node.Kind {
				case ast.KindFunctionDeclaration:
					want.kind, want.symbol = "functionDeclaration", check.GetSymbolAtLocation(node.Name())
				case ast.KindVariableStatement:
					declarations := node.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
					if len(declarations) != 1 {
						t.Fatal("fixture requires one original declaration per statement")
					}
					decl := declarations[0]
					want.kind, want.symbol, want.anchor = "variable", check.GetSymbolAtLocation(decl.Name()), anchor(decl)
					want.initializer = expression(decl.AsVariableDeclaration().Initializer, false)
				case ast.KindReturnStatement:
					want.kind, want.expression = "return", expression(node.AsReturnStatement().Expression, false)
				case ast.KindExpressionStatement:
					value := node.AsExpressionStatement().Expression
					if value.Kind == ast.KindBinaryExpression && value.AsBinaryExpression().OperatorToken.Kind == ast.KindEqualsToken {
						want.kind, want.target, want.value = "assignment", expression(value.AsBinaryExpression().Left, true), expression(value.AsBinaryExpression().Right, false)
					} else {
						want.kind, want.expression = "expression", expression(value, false)
					}
				case ast.KindBlock:
					want.kind, want.body = "block", statements(node.AsBlock().Statements.Nodes)
				case ast.KindIfStatement:
					branch := node.AsIfStatement()
					want.kind, want.condition = "if", expression(branch.Expression, false)
					want.thenBody = statements([]*ast.Node{branch.ThenStatement})
					if branch.ElseStatement != nil {
						want.elseBody = statements([]*ast.Node{branch.ElseStatement})
					}
				case ast.KindWhileStatement:
					loop := node.AsWhileStatement()
					want.kind, want.condition, want.body = "while", expression(loop.Expression, false), statements([]*ast.Node{loop.Statement})
				default:
					t.Fatalf("unexpected official statement kind %v", node.Kind)
				}
				body = append(body, want)
			}
			return body
		}
		fn.body = statements(functionNodes[fn].Body().AsBlock().Statements.Nodes)
	}
	return oracle
}
