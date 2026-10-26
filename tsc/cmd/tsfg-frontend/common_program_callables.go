// SPDX-License-Identifier: Apache-2.0
// Callable facts are original signatures and expressions, never target sets.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

func (p *arrayGenericProjection) commonCallableType(parameters []string, result string, actual *checker.Type) (string, error) {
	if err := p.annotationWork(); err != nil {
		return "", err
	}
	id, err := arrayFactsID("tsfg.program.function-type.v2\x00", struct {
		Parameters []string `json:"parameters"`
		ReturnType string   `json:"returnType"`
	}{parameters, result})
	if err != nil {
		return "", err
	}
	if err := p.typeKeyBudget(id); err != nil {
		return "", err
	}
	return p.addType(arrayGenericType{ID: id, Kind: "function", Parameters: &parameters, ReturnType: result}, actual), nil
}

func (p *arrayGenericProjection) commonFunctionType(node *ast.Node, actual *checker.Type) (string, error) {
	decl := node.AsFunctionTypeNode()
	if decl.TypeParameters != nil {
		return "", p.fail(node, "generic callable type requires specialization")
	}
	parameters := []string{}
	for _, parameter := range decl.Parameters.Nodes {
		if err := p.annotationWork(); err != nil {
			return "", err
		}
		v := parameter.AsParameterDeclaration()
		if v.QuestionToken != nil || v.Initializer != nil || v.DotDotDotToken != nil {
			return "", p.fail(parameter, "callable requires explicit required parameter")
		}
		id, err := p.annotation(v.Type)
		if err != nil {
			return "", err
		}
		parameters = append(parameters, id)
	}
	result, err := p.annotation(decl.Type)
	if err != nil {
		return "", err
	}
	signatures := p.check.GetSignaturesOfType(actual, checker.SignatureKindCall)
	if len(signatures) != 1 || signatures[0].Declaration() != node || len(signatures[0].Parameters()) != len(parameters) || !p.check.IsTypeAssignableTo(p.check.GetReturnTypeOfSignature(signatures[0]), p.actual[result]) {
		return "", p.fail(node, "function type must join its unique original signature")
	}
	return p.commonCallableType(parameters, result, actual)
}

func (p *arrayGenericProjection) commonTypeAlias(node *ast.Node) error {
	if err := p.annotationWork(); err != nil {
		return err
	}
	decl := node.AsTypeAliasDeclaration()
	symbol := p.check.GetSymbolAtLocation(node.Name())
	if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != node || decl.TypeParameters != nil {
		return p.fail(node, "original non-generic type alias required")
	}
	typ, err := p.annotation(decl.Type)
	if err != nil {
		return err
	}
	id, err := arrayFactsBindingID(ast.GetSourceFileOfNode(node).FileName(), *genericSpan(node.Name()))
	if err != nil {
		return err
	}
	p.common.typeAliases = append(p.common.typeAliases, commonProgramTypeAlias{ID: id, Name: node.Name().Text(), SourcePath: ast.GetSourceFileOfNode(node).FileName(), DeclarationRange: *genericSpan(node), NameRange: *genericSpan(node.Name()), AnnotationRange: *genericSpan(decl.Type), Type: typ})
	return nil
}

func (p *arrayGenericProjection) commonFunctionReference(node *ast.Node) (*arrayGenericExpression, bool, error) {
	symbol := p.commonOriginalSymbol(p.check.GetSymbolAtLocation(node))
	header := p.headers[symbol]
	if header == nil {
		return nil, false, nil
	}
	if header.node.Kind != ast.KindFunctionDeclaration {
		return nil, true, p.fail(node, "bound method and constructor values are unsupported")
	}
	parameters := []string{}
	for _, parameter := range header.function.Parameters {
		if err := p.annotationWork(); err != nil {
			return nil, true, err
		}
		parameters = append(parameters, parameter.DeclaredType)
	}
	typ, err := p.commonCallableType(parameters, header.function.ReturnType, p.check.GetTypeOfSymbol(symbol))
	if err != nil {
		return nil, true, err
	}
	return &arrayGenericExpression{Kind: "functionReference", Type: typ, FlowType: typ, FunctionID: header.function.ID, Range: *genericSpan(node)}, true, nil
}

func (p *arrayGenericProjection) commonIndirectCall(node, callee *ast.Node, args []*ast.Node, state *arrayGenericState, depth int) (*arrayGenericExpression, error) {
	value, err := p.expression(callee, "", state, depth+1)
	if err != nil {
		return nil, err
	}
	typ := p.types[value.FlowType]
	resolved := p.check.GetResolvedSignature(node)
	if typ.Kind != "function" || typ.Parameters == nil || resolved == nil || resolved.Declaration() == nil {
		return nil, p.fail(callee, "indirect callee must retain an original structural function type")
	}
	parameters := *typ.Parameters
	if len(parameters) != len(args) || len(resolved.Parameters()) != len(args) {
		return nil, p.fail(node, "indirect call must retain every original required argument")
	}
	values, err := p.argumentValues(args, resolved, parameters, state, depth, nil)
	if err != nil {
		return nil, err
	}
	if !p.check.IsTypeAssignableTo(p.check.GetReturnTypeOfSignature(resolved), p.actual[typ.ReturnType]) {
		return nil, p.fail(node, "indirect result must join the original signature")
	}
	decl := resolved.Declaration()
	id, err := genericNodeID("tsfg.array-facts.signature.v8\x00", decl)
	if err != nil {
		return nil, err
	}
	state.invalidateLengths()
	return &arrayGenericExpression{Kind: "indirectCall", Type: typ.ReturnType, FlowType: typ.ReturnType, Expression: value, CalleeRange: genericSpan(callee), Range: *genericSpan(node), Arguments: &values, Signature: &arrayGenericSignature{ID: id, SourcePath: ast.GetSourceFileOfNode(decl).FileName(), DeclarationRange: *genericSpan(decl), Parameters: parameters, ReturnType: typ.ReturnType}}, nil
}
