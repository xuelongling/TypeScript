// SPDX-License-Identifier: Apache-2.0
package main

import "github.com/microsoft/typescript-go/internal/ast"

// Locals are original checker Symbols scoped to this straight-line body. An
// initializer is projected before publishing its binding, retaining TDZ/order.
func (p *arrayGenericProjection) systemStatements(body *ast.Node, views map[*ast.Symbol]systemQueryLoopBinding, params map[*ast.Symbol]systemQueryParameter, returns string) ([]arrayGenericStatement, []arrayFactsBinding, error) {
	statements := []arrayGenericStatement{}
	directory := []arrayFactsBinding{}
	locals := map[*ast.Symbol]arrayFactsBinding{}
	nodes := body.AsBlock().Statements.Nodes
	for index, node := range nodes {
		if e := p.annotationWork(); e != nil {
			return nil, nil, e
		}
		switch node.Kind {
		case ast.KindVariableStatement:
			list := node.AsVariableStatement().DeclarationList
			if list.Flags&(ast.NodeFlagsLet|ast.NodeFlagsConst) == 0 {
				return nil, nil, p.fail(node, "Query local must be const or let")
			}
			for _, local := range list.AsVariableDeclarationList().Declarations.Nodes {
				decl := local.AsVariableDeclaration()
				if decl.Initializer == nil || local.Name().Kind != ast.KindIdentifier {
					return nil, nil, p.fail(local, "Query local requires original named initializer")
				}
				context := ""
				if decl.Type != nil {
					var e error
					context, e = p.annotation(decl.Type)
					if e != nil {
						return nil, nil, e
					}
				}
				value, e := p.systemLoopExpression(decl.Initializer, views, params, locals, 0, context)
				if e != nil {
					return nil, nil, e
				}
				typ := value.Type
				if context != "" {
					typ = context
				}
				if (typ != "f32" && typ != "f64") || value.Type != typ || value.FlowType != typ {
					return nil, nil, p.fail(local, "Query local initializer must preserve f32/f64 width")
				}
				writable := list.Flags&ast.NodeFlagsLet != 0
				binding, e := p.typedBinding(local, decl.Type, "local", writable, typ)
				if e != nil {
					return nil, nil, e
				}
				owner := body.Parent
				for owner != nil && owner.Kind != ast.KindFunctionDeclaration {
					owner = owner.Parent
				}
				if owner == nil {
					return nil, nil, p.fail(local, "Query local original function owner missing")
				}
				ownerID, e := genericNodeID("tsfg.array-facts.function.v8\x00", owner)
				if e != nil {
					return nil, nil, e
				}
				ownerPointer := &ownerID
				binding.LexicalOwner = &ownerPointer
				symbol := p.check.GetSymbolAtLocation(local.Name())
				if _, exists := locals[symbol]; exists {
					return nil, nil, p.fail(local, "duplicate Query local Symbol")
				}
				locals[symbol] = binding
				directory = append(directory, binding)
				kind := "constDeclaration"
				if writable {
					kind = "letDeclaration"
				}
				statements = append(statements, arrayGenericStatement{Kind: kind, Range: *genericSpan(local), Binding: binding.ID, Initializer: value})
			}
		case ast.KindExpressionStatement:
			value, e := p.systemLoopExpression(node.AsExpressionStatement().Expression, views, params, locals, 0)
			if e != nil {
				return nil, nil, e
			}
			if value.Kind != "fieldAssignment" && value.Kind != "bindingAssignment" && value.Kind != "call" {
				return nil, nil, p.fail(node, "Query statement must be original assignment or helper call")
			}
			statements = append(statements, arrayGenericStatement{Kind: "expression", Range: *genericSpan(node), Value: value})
		case ast.KindReturnStatement:
			if returns == "" || index != len(nodes)-1 || node.AsReturnStatement().Expression == nil {
				return nil, nil, p.fail(node, "Query helper requires one final value return")
			}
			value, e := p.systemLoopExpression(node.AsReturnStatement().Expression, views, params, locals, 0, returns)
			if e != nil {
				return nil, nil, e
			}
			if value.Type != returns || value.FlowType != returns {
				return nil, nil, p.fail(node, "Query helper return width differs")
			}
			statements = append(statements, arrayGenericStatement{Kind: "return", Range: *genericSpan(node), Value: value})
		default:
			return nil, nil, p.fail(node, "Query body requires straight-line floating-point local statements")
		}
	}
	if returns != "" && (len(statements) == 0 || statements[len(statements)-1].Kind != "return") {
		return nil, nil, p.fail(body, "Query helper requires final return")
	}
	return statements, directory, nil
}
