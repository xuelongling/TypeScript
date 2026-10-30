// SPDX-License-Identifier: Apache-2.0
// Lexical facts retain original declarations and direct accesses. They do not
// create environments, Cells, target authority or an initialized-value proof.
package main

import "github.com/microsoft/typescript-go/internal/ast"

func commonNullableOwner37s(id string) **string {
	var owner *string
	if id != "" {
		owner = &id
	}
	return &owner
}

func (p *arrayGenericProjection) commonEnterBindings(node *ast.Node, state *arrayGenericState) error {
	var failure error
	p.common.lexical[node].enterBindings(node, func(upstream *ast.Symbol, owned symbol) bool {
		// Inherited bindings can multiply across children. Charge before each
		// map write and stop immediately, before expanding the child body.
		if failure = p.annotationWork(); failure != nil {
			return true
		}
		state.bindings[upstream] = p.common.bindings[owned.ID]
		return false
	})
	return failure
}
func commonLexicalOwner37s(node *ast.Node) *ast.Node {
	for parent := node.Parent; parent != nil; parent = parent.Parent {
		if parent.Kind == ast.KindFunctionDeclaration || parent.Kind == ast.KindMethodDeclaration || parent.Kind == ast.KindConstructor {
			return parent
		}
	}
	return nil
}

// The original block owns lexical visibility. Repeated while entry remains
// explicit in the statement tree; Source facts do not allocate environments.
func (p *arrayGenericProjection) commonDeclarationScope(declaration, owner *ast.Node) (*ast.Node, error) {
	scope := declaration.Parent
	if declaration.Kind == ast.KindVariableDeclaration && scope != nil && scope.Parent != nil {
		if scope.Flags&(ast.NodeFlagsLet|ast.NodeFlagsConst) == 0 {
			return nil, p.fail(declaration, "captured local requires original let/const; var needs function-scope and hoisting facts")
		}
		scope = scope.Parent.Parent
	}
	if scope == nil || scope.Kind != ast.KindBlock {
		return nil, p.fail(declaration, "named closures require direct function-body declaration scope")
	}
	for ancestor := scope; ancestor != owner.Body(); ancestor = ancestor.Parent {
		if err := p.annotationWork(); err != nil {
			return nil, err
		}
		if ancestor == nil {
			return nil, p.fail(declaration, "named closures require direct function-body declaration scope")
		}
		switch ancestor.Kind {
		case ast.KindBlock, ast.KindIfStatement, ast.KindWhileStatement:
		case ast.KindForStatement, ast.KindForInStatement, ast.KindForOfStatement, ast.KindDoStatement:
			return nil, p.fail(declaration, "captured declaration loop has no common Program statement representation")
		default:
			return nil, p.fail(declaration, "named closures require direct function-body declaration scope")
		}
	}
	return scope, nil
}

func (p *arrayGenericProjection) commonBinding(node *ast.Node, b *arrayFactsBinding) error {
	if err := p.annotationWork(); err != nil {
		return err
	}
	id := ""
	if owner := commonLexicalOwner37s(node); owner != nil {
		var err error
		id, err = genericNodeID("tsfg.array-facts.function.v8\x00", owner)
		if err != nil {
			return err
		}
	}
	b.LexicalOwner = commonNullableOwner37s(id)
	p.common.bindings[b.ID] = *b
	return nil
}
func (p *arrayGenericProjection) commonFunctionHeader(node *ast.Node, fn *arrayGenericFunction) error {
	if err := p.annotationWork(); err != nil {
		return err
	}
	file := ast.GetSourceFileOfNode(node)
	scope := span{0, file.GetPositionMap().UTF8ToUTF16(len(file.Text()))}
	parentID := ""
	if parent := commonLexicalOwner37s(node); parent != nil {
		declarationScope, err := p.commonDeclarationScope(node, parent)
		if err != nil {
			return err
		}
		parentID, err = genericNodeID("tsfg.array-facts.function.v8\x00", parent)
		if err != nil {
			return err
		}
		scope = *genericSpan(declarationScope)
	} else if node.Parent != nil && node.Parent.Kind == ast.KindClassDeclaration {
		scope = *genericSpan(node.Parent)
	}
	fn.LexicalParent, fn.DeclarationScope = commonNullableOwner37s(parentID), &scope
	captures := []commonProgramCapture{}
	fn.Captures = &captures
	return nil
}

func (p *arrayGenericProjection) commonFunctionComplete(node *ast.Node, fn *arrayGenericFunction, state *arrayGenericState) error {
	bindings := map[string]symbol{}
	environment := map[*ast.Symbol]symbol{}
	for original, b := range state.bindings {
		if err := p.annotationWork(); err != nil {
			return err
		}
		owner := ""
		if b.LexicalOwner != nil && *b.LexicalOwner != nil {
			owner = **b.LexicalOwner
		}
		owned := symbol{ID: b.ID, LexicalOwner: owner}
		environment[original] = owned
		bindings[b.ID] = owned
	}
	lexical := p.common.lexical[node]
	lexical.completeBindings(node, environment)
	ledger := newClosureCaptureLedger(fn.ID, bindings)
	occurrences := map[string][]commonProgramCaptureOccurrence{}
	var walk func(*ast.Node, bool, bool) error
	walk = func(current *ast.Node, read, write bool) error {
		if err := p.annotationWork(); err != nil {
			return err
		}
		if current.Kind == ast.KindFunctionDeclaration {
			return nil
		}
		if current.Kind == ast.KindIdentifier {
			original := p.check.GetSymbolAtLocation(current)
			b, found := state.bindings[original]
			if !found || !ledger.add(b.ID, read, write) {
				return nil
			}
			if len(original.Declarations) != 1 {
				return p.fail(current, "captured binding must retain its unique original declaration")
			}
			decl := original.Declarations[0]
			owner := commonLexicalOwner37s(decl)
			typeKind := p.types[b.DeclaredType].Kind
			captureKind := typeKind
			if typeKind == "union" {
				// Scalar payloads share the original binding slot, including
				// null/undefined alternatives. Other unions retain one payload.
				captureKind = ""
				payloads, absences := 0, 0
				scalarPayloads := true
				for _, member := range p.types[b.DeclaredType].Members {
					kind := p.types[member].Kind
					switch kind {
					case "null", "undefined":
						absences++
					case "scalar", "string", "function", "array", "reference", "value":
						scalarPayloads = scalarPayloads && kind == "scalar"
						payloads++
						captureKind = kind
					default:
						return p.fail(current, "nullable capture requires one supported payload and null/undefined presence")
					}
				}
				scalarUnion := scalarPayloads && payloads > 1
				if !scalarUnion && (payloads != 1 || absences < 1 || absences > 2) {
					return p.fail(current, "nullable capture requires one supported payload and null/undefined presence")
				}
			}
			// These parameters already have incoming values. Keep their original
			// binding identity; callable types do not prove targets or lifetime.
			if b.Kind == "parameter" && (captureKind == "scalar" || captureKind == "string" || captureKind == "function" || captureKind == "readSpan" || captureKind == "array" || captureKind == "reference" || captureKind == "value") {
				if decl.Kind != ast.KindParameter || owner == nil || decl.Parent != owner {
					return p.fail(current, "captured scalar/String/function/Array/ReadSpan/reference/value parameter must retain its original function declaration")
				}
			} else {
				if b.Kind != "local" || captureKind != "scalar" && captureKind != "string" && captureKind != "function" && captureKind != "readSpan" && captureKind != "array" && captureKind != "reference" && captureKind != "value" {
					return p.fail(current, "named closure capture requires initialized scalar/String/function/Array/ReadSpan/reference/value local or direct scalar/String/function/Array/ReadSpan/reference/value parameter")
				}
				if decl.Kind != ast.KindVariableDeclaration || decl.AsVariableDeclaration().Initializer == nil || owner == nil {
					return p.fail(current, "captured local must be initialized in its original function-body scope")
				}
				declarationScope, err := p.commonDeclarationScope(decl, owner)
				if err != nil {
					return err
				}
				if declarationScope != owner.Body() && captureKind == "readSpan" {
					return p.fail(current, "block ReadSpan capture requires independent borrow lifetime qualification")
				}
			}
			occurrences[b.ID] = append(occurrences[b.ID], commonProgramCaptureOccurrence{Range: *genericSpan(current), Read: read, Write: write})
			return nil
		}
		if current.Kind == ast.KindBinaryExpression && ast.IsAssignmentOperator(current.AsBinaryExpression().OperatorToken.Kind) {
			binary := current.AsBinaryExpression()
			if err := walk(binary.Left, binary.OperatorToken.Kind != ast.KindEqualsToken, true); err != nil {
				return err
			}
			return walk(binary.Right, true, false)
		}
		if current.Kind == ast.KindPrefixUnaryExpression {
			unary := current.AsPrefixUnaryExpression()
			if unary.Operator == ast.KindPlusPlusToken || unary.Operator == ast.KindMinusMinusToken {
				return walk(unary.Operand, true, true)
			}
		}
		if current.Kind == ast.KindPostfixUnaryExpression {
			return walk(current.AsPostfixUnaryExpression().Operand, true, true)
		}
		if current.Kind == ast.KindParenthesizedExpression {
			return walk(current.AsParenthesizedExpression().Expression, read, write)
		}
		var failure error
		current.ForEachChild(func(child *ast.Node) bool { failure = walk(child, true, false); return failure != nil })
		return failure
	}
	if err := walk(node.Body(), true, false); err != nil {
		return err
	}
	captures := []commonProgramCapture{}
	for _, capture := range ledger.captures {
		if err := p.annotationWork(); err != nil {
			return err
		}
		captures = append(captures, commonProgramCapture{Binding: capture.Symbol, Read: capture.Read, Write: capture.Write, Occurrences: occurrences[capture.Symbol]})
	}
	fn.Captures = &captures
	return nil
}
