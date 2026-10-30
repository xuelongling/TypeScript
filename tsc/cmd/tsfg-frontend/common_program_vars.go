// SPDX-License-Identifier: Apache-2.0
// Function-scoped declarations retain one original Symbol and every source
// declaration site. Entry undefined is a binding fact, not an invented AST node.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

func commonVarDeclaration(node *ast.Node) bool {
	return node != nil && node.Kind == ast.KindVariableDeclaration && node.Parent != nil && node.Parent.Kind == ast.KindVariableDeclarationList && node.Parent.Flags&ast.NodeFlagsBlockScoped == 0
}

// The official value declaration supplies the existing BindingID anchor. A var
// may share that Symbol with other vars or its incoming parameter, not a foreign
// lexical binding or a function declaration with separate callable semantics.
func (p *arrayGenericProjection) commonVarOriginal(original *ast.Symbol, owner *ast.Node) (*ast.Node, error) {
	if original == nil || original.ValueDeclaration == nil || owner == nil {
		return nil, arrayFactsError("capability", "original function-scoped var Symbol required", nil)
	}
	root := original.ValueDeclaration
	if p.check.GetTypeOfSymbol(original).Flags()&checker.TypeFlagsAny != 0 {
		return nil, p.fail(root, "var requires a supported original declared type, not any")
	}
	for _, declaration := range original.Declarations {
		if err := p.annotationWork(); err != nil {
			return nil, err
		}
		if commonLexicalOwner37s(declaration) != owner || !commonVarDeclaration(declaration) && declaration.Kind != ast.KindParameter {
			return nil, p.fail(declaration, "var must retain only its original same-function variable or parameter declarations")
		}
		if declaration.Kind == ast.KindParameter && declaration != root {
			return nil, p.fail(declaration, "var parameter redeclaration must retain its original incoming parameter")
		}
	}
	if !commonVarDeclaration(root) && root.Kind != ast.KindParameter {
		return nil, p.fail(root, "var original value declaration has no common binding representation")
	}
	return root, nil
}

func (p *arrayGenericProjection) commonEnterVars(state *arrayGenericState) error {
	owner := state.header.node
	state.varDeclarations = map[*ast.Symbol]bool{}
	var declarations []*ast.Node
	var visit func(*ast.Node) error
	visit = func(node *ast.Node) error {
		if err := p.annotationWork(); err != nil {
			return err
		}
		if node != owner.Body() && ast.IsFunctionLike(node) {
			return nil
		}
		if commonVarDeclaration(node) {
			declarations = append(declarations, node)
		}
		var failure error
		node.ForEachChild(func(child *ast.Node) bool { failure = visit(child); return failure != nil })
		return failure
	}
	if err := visit(owner.Body()); err != nil {
		return err
	}
	for _, declaration := range declarations {
		original := p.check.GetSymbolAtLocation(declaration.Name())
		if _, exists := state.bindings[original]; exists {
			continue
		}
		root, err := p.commonVarOriginal(original, owner)
		if err != nil {
			return err
		}
		annotation := root.Type()
		if annotation == nil && root.Initializer() == nil {
			for _, sibling := range original.Declarations {
				if err := p.annotationWork(); err != nil {
					return err
				}
				if sibling.Type() != nil {
					annotation = sibling.Type()
					break
				}
			}
		}
		var typ string
		if annotation != nil {
			typ, err = p.annotation(annotation)
		} else {
			typ, err = p.commonVarType(root, state, map[*ast.Node]bool{})
		}
		if err != nil {
			return err
		}
		binding, err := p.typedBinding(root, root.Type(), "local", true, typ)
		if err != nil {
			return err
		}
		state.bindings[original] = binding
		if p.types[binding.DeclaredType].Kind == "array" {
			state.lengths[binding.ID] = &arrayFactsLengthState{}
		}
	}
	return nil
}

// Select logical type provenance without projecting/evaluating an initializer
// early. Official Symbols and selected return annotations retain numeric aliases
// that a bare checker number Type cannot distinguish.
func (p *arrayGenericProjection) commonVarType(node *ast.Node, state *arrayGenericState, active map[*ast.Node]bool) (string, error) {
	if err := p.annotationWork(); err != nil {
		return "", err
	}
	if node == nil {
		return "", arrayFactsError("capability", "var requires original logical type provenance", nil)
	}
	if active[node] {
		return "", p.fail(node, "var logical type provenance is cyclic")
	}
	if len(active) >= 64 {
		return "", arrayFactsError("limit", "var logical type provenance depth", nil)
	}
	active[node] = true
	defer delete(active, node)
	switch node.Kind {
	case ast.KindVariableDeclaration, ast.KindParameter:
		if node.Type() != nil {
			return p.annotation(node.Type())
		}
		return p.commonVarType(node.Initializer(), state, active)
	case ast.KindIdentifier:
		original := p.commonOriginalSymbol(p.check.GetSymbolAtLocation(node))
		if binding, found := state.bindings[original]; found {
			return p.flow(p.check.GetTypeAtLocation(node), binding.DeclaredType)
		}
		if value, found, err := p.commonFunctionReference(node); found || err != nil {
			if err != nil {
				return "", err
			}
			return value.Type, nil
		}
		if original != nil && original.ValueDeclaration != nil {
			typ, err := p.commonVarType(original.ValueDeclaration, state, active)
			if err != nil {
				return "", err
			}
			return p.flow(p.check.GetTypeAtLocation(node), typ)
		}
	case ast.KindParenthesizedExpression:
		return p.commonVarType(node.AsParenthesizedExpression().Expression, state, active)
	case ast.KindAsExpression:
		return p.annotation(node.AsAsExpression().Type)
	case ast.KindStringLiteral, ast.KindNoSubstitutionTemplateLiteral:
		return p.stringType(), nil
	case ast.KindTrueKeyword, ast.KindFalseKeyword:
		return p.scalarType("bool"), nil
	case ast.KindNumericLiteral:
		value, err := p.number(node, "")
		if err != nil {
			return "", err
		}
		return value.Type, nil
	case ast.KindPrefixUnaryExpression:
		if node.AsPrefixUnaryExpression().Operator == ast.KindMinusToken && node.AsPrefixUnaryExpression().Operand.Kind == ast.KindNumericLiteral {
			value, err := p.number(node, "")
			if err != nil {
				return "", err
			}
			return value.Type, nil
		}
	case ast.KindNewExpression:
		if id := p.classes[p.check.GetTypeAtLocation(node).Symbol()]; id != "" {
			return id, nil
		}
	case ast.KindCallExpression:
		if signature := p.check.GetResolvedSignature(node); signature != nil && signature.Declaration() != nil && signature.Declaration().Type() != nil {
			return p.annotation(signature.Declaration().Type())
		}
	}
	return "", p.fail(node, "var requires supported original logical type provenance")
}

func (p *arrayGenericProjection) commonVarStatement(node *ast.Node, state *arrayGenericState, fn *arrayGenericFunction) (arrayGenericStatement, error) {
	original := p.check.GetSymbolAtLocation(node.Name())
	binding, found := state.bindings[original]
	if !found {
		return arrayGenericStatement{}, p.fail(node, "var declaration must join its preindexed original function binding")
	}
	decl := node.AsVariableDeclaration()
	row := arrayGenericStatement{Kind: "varDeclaration", Binding: binding.ID, Range: *genericSpan(node), NameRange: genericSpan(node.Name())}
	if decl.Type != nil {
		typ, err := p.annotation(decl.Type)
		if err != nil {
			return row, err
		}
		if typ != binding.DeclaredType {
			return row, p.fail(node, "var redeclaration must retain the original logical declared type")
		}
		row.AnnotationRange = genericSpan(decl.Type)
	}
	if decl.Initializer != nil {
		value, err := p.expression(decl.Initializer, binding.DeclaredType, state, 0)
		if err != nil {
			return row, err
		}
		row.Initializer = value
		if p.types[binding.DeclaredType].Kind == "array" {
			length := &arrayFactsLengthState{}
			if value.Kind == "arrayLiteral" {
				length.known, length.length = true, uint64(len(*value.Elements))
			} else if value.Binding != "" && state.lengths[value.Binding] != nil {
				length = state.lengths[value.Binding]
			}
			state.lengths[binding.ID] = length
		}
	}
	if binding.Kind == "local" && !state.varDeclarations[original] {
		fn.Bindings = append(fn.Bindings, binding)
		state.varDeclarations[original] = true
	}
	return row, nil
}
