// SPDX-License-Identifier: Apache-2.0
// R01 assertions select existing variants; conversion entries change scalars.
package main

import "github.com/microsoft/typescript-go/internal/ast"

func (p *arrayGenericProjection) assertionLiteralContext(node *ast.Node, target string) string {
	for node.Kind == ast.KindParenthesizedExpression {
		node = node.AsParenthesizedExpression().Expression
	}
	if node.Kind == ast.KindNumericLiteral || node.Kind == ast.KindArrayLiteralExpression && p.types[target].Kind == "array" {
		return target
	}
	if node.Kind == ast.KindPrefixUnaryExpression {
		prefix := node.AsPrefixUnaryExpression()
		if prefix.Operator == ast.KindMinusToken && prefix.Operand.Kind == ast.KindNumericLiteral {
			return target
		}
	}
	return ""
}

// Union/presence assertions may select or retain an existing numeric variant.
// Inspect homogeneous elements too, so asserting a whole Array/ReadSpan cannot
// bypass the scalar rule. Nominal object views retain their separate M12 proof.
func (p *arrayGenericProjection) assertionRepresentation(node *ast.Node, source, target string) error {
	from, to := map[int]map[string]bool{}, map[int]map[string]bool{}
	p.assertionNumerics(source, 0, from)
	p.assertionNumerics(target, 0, to)
	for depth, kinds := range from {
		if len(to[depth]) == 0 {
			continue
		}
		shared := false
		for kind := range kinds {
			shared = shared || to[depth][kind]
		}
		if !shared {
			return p.numericRepresentationError(node)
		}
	}
	return nil
}

func (p *arrayGenericProjection) numericRepresentationError(node *ast.Node) error {
	result := failAt("r01-d-scalar-type", "error", ast.GetSourceFileOfNode(node), node)
	return arrayFactsDiagnostics("types", "typed numeric representations require an owned conversion entry", result.Diagnostics)
}

func (p *arrayGenericProjection) assertionNumerics(id string, depth int, kinds map[int]map[string]bool) {
	typ := p.types[id]
	switch typ.Kind {
	case "scalar":
		if integerKind(id) || floatKind(id) {
			if kinds[depth] == nil {
				kinds[depth] = map[string]bool{}
			}
			kinds[depth][id] = true
		}
	case "union":
		for _, member := range typ.Members {
			p.assertionNumerics(member, depth, kinds)
		}
	case "array", "readSpan":
		p.assertionNumerics(typ.ElementType, depth+1, kinds)
	}
}
