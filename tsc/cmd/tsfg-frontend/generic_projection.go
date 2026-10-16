// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// All concrete facts below come from a fresh official checker and its mapper.
package main

import (
	"context"
	"slices"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

type genericProjection struct {
	ctx         context.Context
	graph       genericGraph
	tree        *genericTree
	check       *checker.Checker
	declaration *ast.Node
	calls       []genericInstanceRequest
}

func projectGenericInstance(ctx context.Context, g genericGraph, r genericInstanceRequest) (genericInstance, []genericInstanceRequest, error) {
	if err := ctx.Err(); err != nil {
		return genericInstance{}, nil, err
	}
	tree := buildGenericTree(g, r)
	if err := ctx.Err(); err != nil {
		return genericInstance{}, nil, err
	}
	program := newGenericProgram(g, tree, tree.text.String())
	if err := checkGenericProgram(ctx, g, program, tree); err != nil {
		return genericInstance{}, nil, err
	}
	return projectCheckedGenericInstance(ctx, g, r, tree, program)
}

func genericConcreteType(t *checker.Type) string {
	if t == nil {
		return ""
	}
	if t.IsBooleanLike() {
		return "bool"
	}
	if t.Flags()&checker.TypeFlagsNumber != 0 {
		return "f64"
	}
	return ""
}

func (p *genericProjection) mappedType(node *ast.Node, typeNode bool) (string, error) {
	if err := p.ctx.Err(); err != nil {
		return "", err
	}
	t, ok := p.check.TSFGInstantiateBodyType(p.tree.invocation, p.declaration, node, typeNode)
	if ok {
		if name := genericConcreteType(t); name != "" {
			return name, nil
		}
	}
	return "", genericError("unsupported", "body fact did not instantiate to scalar", p.tree.origins[node])
}

func (p *genericProjection) expression(node *ast.Node) (genericConcreteExpression, error) {
	t, err := p.mappedType(node, false)
	if err != nil {
		return genericConcreteExpression{}, err
	}
	result := genericConcreteExpression{Type: t, Origin: p.tree.origins[node]}
	switch node.Kind {
	case ast.KindIdentifier:
		sym := p.check.GetSymbolAtLocation(node)
		for i, parameter := range p.declaration.AsFunctionDeclaration().Parameters.Nodes {
			if sym == p.check.GetSymbolAtLocation(parameter.Name()) {
				result.Kind, result.Parameter = "parameter", i
				return result, nil
			}
		}
	case ast.KindCallExpression:
		call := node.AsCallExpression()
		sig := p.check.GetResolvedSignature(node)
		key := ""
		for candidate, declaration := range p.tree.definitions {
			if sig != nil && sig.Declaration() == declaration &&
				p.check.GetSymbolAtLocation(call.Expression) == p.check.GetSymbolAtLocation(declaration.Name()) {
				key = candidate
				break
			}
		}
		if key == "" || call.TypeArguments == nil {
			break
		}
		var args []genericType
		for _, argument := range call.TypeArguments.Nodes {
			name, err := p.mappedType(argument, true)
			if err != nil {
				return genericConcreteExpression{}, err
			}
			kind := "boolean"
			if name == "f64" {
				kind = "number"
			}
			args = append(args, genericType{Kind: kind})
		}
		request := p.graph.request(key, args...)
		result.Kind, result.Callee = "call", genericInstanceKey(request)
		p.calls = append(p.calls, request)
		for _, argument := range call.Arguments.Nodes {
			e, err := p.expression(argument)
			if err != nil {
				return genericConcreteExpression{}, err
			}
			result.Arguments = append(result.Arguments, e)
		}
		return result, nil
	}
	return genericConcreteExpression{}, genericError("unsupported", "unowned body symbol or call", result.Origin)
}

func makeGenericInstance(ctx context.Context, g genericGraph, r genericInstanceRequest, tree *genericTree, check *checker.Checker) (genericInstance, []genericInstanceRequest, error) {
	decl := tree.definitions[r.Definition]
	project := genericProjection{ctx: ctx, graph: g, tree: tree, check: check, declaration: decl}
	instance := genericInstance{Key: genericInstanceKey(r), Definition: r.Definition,
		TypeArguments: slices.Clone(r.TypeArguments), Source: cloneGenericSource(g.Source), Origin: tree.origins[decl]}
	for _, parameter := range decl.AsFunctionDeclaration().Parameters.Nodes {
		t, err := project.mappedType(parameter.Type(), true)
		if err != nil {
			return genericInstance{}, nil, err
		}
		instance.Parameters = append(instance.Parameters, t)
	}
	var err error
	instance.Return, err = project.mappedType(decl.Type(), true)
	if err != nil {
		return genericInstance{}, nil, err
	}
	sig := check.GetResolvedSignature(tree.invocation)
	if sig == nil || genericConcreteType(check.GetReturnTypeOfSignature(sig)) != instance.Return {
		return genericInstance{}, nil, genericError("input", "resolved return does not match projected declaration", instance.Origin)
	}
	body := decl.Body().AsBlock().Statements.Nodes[0].AsReturnStatement().Expression
	instance.Body, err = project.expression(body)
	if err != nil {
		return genericInstance{}, nil, err
	}
	if err := ctx.Err(); err != nil {
		return genericInstance{}, nil, err
	}
	return instance, project.calls, nil
}
