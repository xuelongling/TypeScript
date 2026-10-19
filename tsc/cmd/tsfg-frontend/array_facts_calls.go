// SPDX-License-Identifier: Apache-2.0
// Direct calls join original declarations and atomic arguments before owned publication.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

// Header and call results share the same pinned nominal result check.
func (p *arrayFactsProjection) resultType(typ *checker.Type, kind string) bool {
	switch kind {
	case "f64":
		return typ == p.check.GetNumberType()
	case "readspan.f64":
		return p.viewType(typ, p.check.GetNumberType())
	}
	return false
}

func (p *arrayFactsProjection) call(node *ast.Node, state *arrayFactsFunctionState) (*arrayFactsValue, error) {
	if node == nil || node.Kind != ast.KindCallExpression {
		return nil, arrayFactsCapability(p.file, node, "direct ordinary call required")
	}
	call := node.AsCallExpression()
	if call.Expression == nil || call.Expression.Kind != ast.KindIdentifier || call.QuestionDotToken != nil || call.TypeArguments != nil {
		return nil, arrayFactsCapability(p.file, node, "same-module direct named call required")
	}
	header := p.headers[p.check.GetSymbolAtLocation(call.Expression)]
	resolved := p.check.GetResolvedSignature(node)
	signatures := p.check.GetSignaturesOfType(p.check.GetTypeAtLocation(call.Expression), checker.SignatureKindCall)
	if header == nil || resolved == nil || len(signatures) != 1 || signatures[0].Declaration() != header.node || resolved.Declaration() != header.node || !p.resultType(p.check.GetTypeAtLocation(node), header.function.ReturnType) || !p.resultType(p.check.GetReturnTypeOfSignature(resolved), header.function.ReturnType) || len(resolved.Parameters()) != len(header.function.Parameters) || resolved.MinArgumentCount() != len(header.function.Parameters) || len(resolved.TypeParameters()) != 0 || resolved.ThisParameter() != nil || resolved.HasRestParameter() {
		return nil, arrayFactsCapability(p.file, node, "actual complete same-module required call signature required")
	}
	var arguments []*ast.Node
	if call.Arguments != nil {
		arguments = call.Arguments.Nodes
	}
	if len(arguments) != len(header.function.Parameters) {
		return nil, arrayFactsCapability(p.file, node, "all actual required call arguments in parameter order required")
	}
	values := make([]arrayFactsValue, 0, len(arguments))
	for i, argument := range arguments {
		if err := p.ctx.Err(); err != nil {
			return nil, arrayFactsError("canceled", "ordered call arguments", err)
		}
		parameter := resolved.Parameters()[i]
		original := header.node.AsFunctionDeclaration().Parameters.Nodes[i]
		if parameter == nil || parameter != header.signature.Parameters()[i] || len(parameter.Declarations) != 1 || parameter.Declarations[0] != original || !p.check.IsTypeAssignableTo(p.check.GetTypeAtLocation(argument), p.check.GetTypeOfSymbol(parameter)) {
			return nil, arrayFactsCapability(p.file, argument, "actual argument and original resolved parameter identity/type required")
		}
		var value *arrayFactsValue
		var err error
		switch header.function.Parameters[i].DeclaredType {
		case "array.f64":
			value, err = p.reference(argument, state)
		case "readspan.f64":
			value, err = p.viewReference(argument, state)
		case "f64":
			if argument.Kind == ast.KindIdentifier {
				value, err = p.scalarReference(argument, "f64", state)
			} else {
				value, err = p.floating(argument)
			}
		case "u32":
			value, err = p.unsigned(argument, true, state)
			if err == nil && value.Kind == "lengthLiteral" {
				value.Kind = "u32Literal"
			}
		default:
			return nil, arrayFactsError("identity", "complete projected call parameter type", nil)
		}
		if err != nil {
			return nil, err
		}
		values = append(values, *value)
	}
	state.calls = append(state.calls, node)
	calleeRange := expressionOffset(p.file, call.Expression)
	return &arrayFactsValue{Kind: "directCall", Type: header.function.ReturnType, Range: expressionOffset(p.file, node), Callee: header.function.ID, CalleeRange: &calleeRange, Arguments: &values}, nil
}

func (p *arrayFactsProjection) acyclic(headers []*arrayFactsHeader) error {
	// Sixteen complete functions bound graph depth; source slots prove no physical disjointness.
	state := map[*ast.Node]uint8{}
	var visit func(*arrayFactsHeader) error
	visit = func(header *arrayFactsHeader) error {
		if err := p.ctx.Err(); err != nil {
			return arrayFactsError("canceled", "complete direct call graph", err)
		}
		state[header.node] = 1
		for _, node := range header.state.calls {
			target := p.headers[p.check.GetSymbolAtLocation(node.AsCallExpression().Expression)]
			if target == nil {
				return arrayFactsError("identity", "complete projected call target", nil)
			}
			if state[target.node] == 1 {
				return arrayFactsCapability(p.file, node, "acyclic direct named call graph required")
			}
			if state[target.node] == 0 {
				if err := visit(target); err != nil {
					return err
				}
			}
		}
		state[header.node] = 2
		return nil
	}
	for _, header := range headers {
		if state[header.node] == 0 {
			if err := visit(header); err != nil {
				return err
			}
		}
	}
	return nil
}
