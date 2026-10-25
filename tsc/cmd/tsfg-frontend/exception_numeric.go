// SPDX-License-Identifier: Apache-2.0
package main

import (
	_ "embed"
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/scanner"
)

//go:embed testdata/exception-facts-v2/descriptor.json
var exceptionNumericDescriptor []byte

const exceptionNumericFactsSchema = "sha256:c01b980763f42bd48bb297f947c82446152f7e333e1b609b165a720eb6b0b8f6"

func exceptionFactsSchemaForVersion(version int) (string, bool) {
	if version == 1 {
		return exceptionFactsSchema, true
	}
	if version == 2 && exceptionFactsDigest(exceptionNumericDescriptor) == exceptionNumericFactsSchema {
		return exceptionNumericFactsSchema, true
	}
	return "", false
}

// The annotation is a static view only. Operand facts retain their official
// declaration/use-site type and original Binding, including unknown/undefined.
// No dynamic domain, conversion, numeric check or request trap is inferred here.
type assertionFact struct {
	Annotation span `json:"annotation"`
	Operator   span `json:"operator"`
}

func projectExceptionNumeric(file *ast.SourceFile, check *checker.Checker, node *ast.Node,
	refs map[*ast.Symbol]symbol, functions map[*ast.Symbol]functionInfo, target *integerTarget,
) (*expression, *response, bool) {
	fail := func() (*expression, *response, bool) {
		failure := failAt("r02b-d-unsupported", "unsupported", file, node)
		return nil, &failure, true
	}
	if node.Kind == ast.KindIdentifier && missingState(check, node) == "undefined" {
		return &expression{Kind: "undefined", Type: "undefined", Range: expressionOffset(file, node)}, nil, true
	}
	if moduleReference(check, node) {
		binding, exists := refs[check.GetSymbolAtLocation(node)]
		if exists && binding.Type == "unknown" && check.GetTypeOfSymbol(check.GetSymbolAtLocation(node)) == check.GetUnknownType() {
			actual := check.GetTypeAtLocation(node)
			t := scalar(actual)
			if actual != nil && actual.Flags()&checker.TypeFlagsUndefined != 0 {
				t = "undefined"
			}
			if t != "" {
				return &expression{Kind: "reference", Type: t, Symbol: binding.ID, Range: offset(file, node)}, nil, true
			}
		}
	}
	var operand, annotation *ast.Node
	operator := "as"
	switch node.Kind {
	case ast.KindAsExpression:
		operand, annotation = node.AsAsExpression().Expression, node.AsAsExpression().Type
	case ast.KindTypeAssertionExpression:
		operand, annotation = node.AsTypeAssertion().Expression, node.AsTypeAssertion().Type
		operator = "type-assertion"
	default:
		return nil, nil, false
	}
	view := check.GetTypeFromTypeNode(annotation)
	t := scalar(view)
	if view == check.GetUnknownType() {
		t = "unknown"
	}
	if t == "" || t == "f64" && !target.floats || check.GetTypeAtLocation(node) != view {
		return fail()
	}
	inner, failure := projectIntegerExpression(file, check, operand, "", refs, functions, target)
	if failure != nil {
		return nil, failure, true
	}
	start, end := scanner.SkipTrivia(file.Text(), operand.End()), 0
	if operator == "as" {
		end = start + 2
		if end > len(file.Text()) || file.Text()[start:end] != "as" {
			return fail()
		}
	} else {
		start = scanner.SkipTrivia(file.Text(), node.Pos())
		end = scanner.SkipTrivia(file.Text(), annotation.End()) + 1
		if start >= len(file.Text()) || end > len(file.Text()) || file.Text()[start] != '<' || file.Text()[end-1] != '>' {
			return fail()
		}
	}
	positions := file.GetPositionMap()
	return &expression{Kind: "assertion", Type: t, Operator: operator, Operand: inner,
		Assertion: &assertionFact{Annotation: expressionOffset(file, annotation), Operator: span{positions.UTF8ToUTF16(start), positions.UTF8ToUTF16(end)}},
		Range:     expressionOffset(file, node)}, nil, true
}
