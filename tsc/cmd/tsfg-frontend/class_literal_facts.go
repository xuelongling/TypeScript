// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Literal definitions preserve the official narrow identity and unresolved R01
// typing. Consumers must reject unresolved numeric execution, never use the
// declared primitive number's f64 label as a fallback.
package main

import (
	"fmt"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

type classNumericLiteral struct {
	State         string `json:"state"`
	OfficialType  string `json:"officialType"`
	DefaultScalar string `json:"defaultScalar"`
	ContextType   string `json:"contextType"`
}

func (p *classProjection) numberLiteralRef(value *checker.Type, node *ast.Node) (string, error) {
	regular := value.AsLiteralType().RegularType()
	if regular == nil || regular.Flags()&checker.TypeFlagsNumberLiteral == 0 {
		return "", classError("identity", "official number literal regular type missing", classNodeAnchor(node))
	}
	if id := p.types[regular]; id != "" {
		p.types[value] = id
		return id, nil
	}
	p.literalTypes++
	id := fmt.Sprintf("nl%d", p.literalTypes)
	p.types[regular], p.types[value] = id, id
	// This text identifies the official TS narrow value only. Exact R01
	// interpretation always uses each occurrence's original Token, not this text.
	p.graph.Types = append(p.graph.Types, classType{ID: id, Kind: "number-literal", Literal: regular.AsLiteralType().String()})
	return id, nil
}

func (p *classProjection) literalFact(node *ast.Node, typeRef string) (*classNumericLiteral, error) {
	file := ast.GetSourceFileOfNode(node)
	hint := floatDefaultHint(file, node)
	if hint == "" {
		if _, _, ok := exactInteger(file, node); !ok {
			return nil, classError("unsupported", "numeric literal default is outside the exact scalar domain", classNodeAnchor(node))
		}
		hint = "i32"
	}
	return &classNumericLiteral{State: "unresolved", OfficialType: typeRef, DefaultScalar: hint}, nil
}

// Only a literal receives slot context. Already typed references keep their own
// meaning; this function neither converts them nor marks a literal settled.
func (p *classProjection) literalContext(value *classExpression, target *checker.Type, node *ast.Node) error {
	if value == nil || value.NumericLiteral == nil {
		return nil
	}
	context, err := p.typeRef(target, node)
	if err != nil {
		return err
	}
	value.NumericLiteral.ContextType = context
	return nil
}
