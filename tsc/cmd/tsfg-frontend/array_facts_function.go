// SPDX-License-Identifier: Apache-2.0
// Complete finite functions retain actual declaration/use identities and source order.
package main

import (
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

// Owners are source proofs, never physical runtime handles. Aliases share a
// pointer so later declarations observe the current state of their original root.
type arrayFactsLengthState struct {
	parameter bool
	known     bool
	length    uint64
}
type arrayFactsFunctionState struct {
	calls      []*ast.Node
	bindings   map[*ast.Symbol]arrayFactsBinding
	parameters map[*ast.Symbol]arrayFactsParameter
	names      map[string]bool
	owners     map[string]*arrayFactsLengthState
}

func (p *arrayFactsProjection) bindingID(node *ast.Node) (string, error) {
	return arrayFactsBindingID(p.file.FileName(), expressionOffset(p.file, node))
}
func arrayFactsBindingID(path string, name span) (string, error) {
	return arrayFactsID("tsfg.array-facts.binding.v1\x00", struct {
		SourcePath string `json:"sourcePath"`
		NameRange  span   `json:"nameRange"`
	}{path, name})
}
func (p *arrayFactsProjection) parameters(nodes []*ast.Node, signature *checker.Signature, state *arrayFactsFunctionState) ([]arrayFactsParameter, error) {
	values := make([]arrayFactsParameter, 0, len(nodes))
	for i, node := range nodes {
		decl := node.AsParameterDeclaration()
		kind := p.scalarAnnotation(decl.Type)
		if kind == "" && decl.Type != nil && decl.Type.Kind == ast.KindArrayType && p.f64Annotation(decl.Type.AsArrayTypeNode().ElementType) && p.arrayType(p.check.GetTypeFromTypeNode(decl.Type), false) {
			kind = "array.f64"
		}
		if node.Name() == nil || node.Name().Kind != ast.KindIdentifier || node.ModifierFlags() != 0 || decl.QuestionToken != nil || decl.Initializer != nil || decl.DotDotDotToken != nil || kind == "" || node.Name().Text() == "this" {
			return nil, arrayFactsCapability(p.file, node, "required direct pinned scalar or Array parameter required")
		}
		symbol := p.check.GetSymbolAtLocation(node.Name())
		if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != node || symbol != signature.Parameters()[i] || state.names[node.Name().Text()] {
			return nil, arrayFactsCapability(p.file, node, "actual unique required signature parameter required")
		}
		typ := p.check.GetTypeOfSymbol(symbol)
		if kind == "array.f64" && !p.arrayType(typ, false) || kind != "array.f64" && typ != p.check.GetNumberType() {
			return nil, arrayFactsCapability(p.file, node, "actual pinned parameter type required")
		}
		id, err := p.bindingID(node.Name())
		if err != nil {
			return nil, err
		}
		value := arrayFactsParameter{ID: id, Name: strings.Clone(node.Name().Text()), Kind: "parameter", DeclaredType: kind, Writable: true, DeclarationRange: expressionOffset(p.file, node), NameRange: expressionOffset(p.file, node.Name()), AnnotationRange: expressionOffset(p.file, decl.Type)}
		if kind == "array.f64" {
			element := expressionOffset(p.file, decl.Type.AsArrayTypeNode().ElementType)
			value.ElementAnnotationRange = &element
			// Slots can alias each other, so none establishes a disjoint exact length.
			state.owners[id] = &arrayFactsLengthState{parameter: true}
		}
		state.parameters[symbol], state.names[value.Name] = value, true
		values = append(values, value)
	}
	return values, nil
}
func (p *arrayFactsProjection) local(statement *ast.Node, state *arrayFactsFunctionState) (arrayFactsBinding, arrayFactsStatement, error) {
	var binding arrayFactsBinding
	var result arrayFactsStatement
	bad := func(at *ast.Node, detail string) (arrayFactsBinding, arrayFactsStatement, error) {
		return binding, result, arrayFactsCapability(p.file, at, detail)
	}
	list := statement.AsVariableStatement().DeclarationList
	locals := list.AsVariableDeclarationList().Declarations.Nodes
	if statement.ModifierFlags() != 0 || list.Flags&ast.NodeFlagsConst == 0 || len(locals) != 1 {
		return bad(statement, "one ordinary const Array declarator required")
	}
	localNode := locals[0]
	local := localNode.AsVariableDeclaration()
	if localNode.Name() == nil || localNode.Name().Kind != ast.KindIdentifier || local.Initializer == nil || local.ExclamationToken != nil {
		return bad(localNode, "initialized direct const Array binding required")
	}
	symbol := p.check.GetSymbolAtLocation(localNode.Name())
	if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != localNode || !p.arrayType(p.check.GetTypeOfSymbol(symbol), false) || state.names[localNode.Name().Text()] {
		return bad(localNode, "actual unique f64 Array local declaration required")
	}
	localID, err := p.bindingID(localNode.Name())
	if err != nil {
		return binding, result, err
	}
	binding = arrayFactsBinding{ID: localID, Name: strings.Clone(localNode.Name().Text()), Kind: "const", DeclaredType: "array.f64", Writable: false, DeclarationRange: expressionOffset(p.file, localNode), NameRange: expressionOffset(p.file, localNode.Name())}
	if local.Type != nil {
		if local.Type.Kind != ast.KindArrayType || !p.f64Annotation(local.Type.AsArrayTypeNode().ElementType) || !p.arrayType(p.check.GetTypeFromTypeNode(local.Type), false) {
			return bad(local.Type, "explicit pinned f64[] annotation required")
		}
		annotation, element := expressionOffset(p.file, local.Type), expressionOffset(p.file, local.Type.AsArrayTypeNode().ElementType)
		binding.AnnotationRange, binding.ElementAnnotationRange = &annotation, &element
	}
	var init *arrayFactsValue
	var owner *arrayFactsLengthState
	if local.Initializer.Kind == ast.KindArrayLiteralExpression {
		if local.Type == nil {
			return bad(local.Initializer, "literal root requires explicit f64[] context")
		}
		init, err = p.literal(local.Initializer, p.check.GetTypeOfSymbol(symbol))
		if err == nil {
			owner = &arrayFactsLengthState{known: true, length: uint64(len(*init.Elements))}
		}
	} else {
		init, err = p.reference(local.Initializer, state)
		if err == nil {
			owner = state.owners[init.Binding]
		}
	}
	if err != nil {
		return binding, result, err
	}
	state.bindings[symbol], state.names[binding.Name], state.owners[binding.ID] = binding, true, owner
	result = arrayFactsStatement{Kind: "variable", Binding: binding.ID, Range: expressionOffset(p.file, statement), Initializer: init}
	return binding, result, nil
}

type arrayFactsHeader struct {
	node      *ast.Node
	signature *checker.Signature
	function  arrayFactsFunction
	state     *arrayFactsFunctionState
}

func (p *arrayFactsProjection) functionHeader(node *ast.Node) (*arrayFactsHeader, *ast.Symbol, error) {
	bad := func(at *ast.Node, detail string) (*arrayFactsHeader, *ast.Symbol, error) {
		return nil, nil, arrayFactsCapability(p.file, at, detail)
	}
	if node.Kind != ast.KindFunctionDeclaration || node.Name() == nil || node.Name().Kind != ast.KindIdentifier || node.ModifierFlags() != 0 && node.ModifierFlags() != ast.ModifierFlagsExport {
		return bad(node, "only named ordinary functions with optional export")
	}
	decl := node.AsFunctionDeclaration()
	if decl.TypeParameters != nil || decl.AsteriskToken != nil || decl.FullSignature != nil || decl.Parameters == nil || decl.Body == nil || decl.Body.Kind != ast.KindBlock || !p.f64Annotation(decl.Type) {
		return bad(node, "explicit f64 ordinary function required")
	}
	actual := p.check.GetSymbolAtLocation(node.Name())
	if actual == nil || len(actual.Declarations) != 1 || actual.Declarations[0] != node {
		return bad(node, "unique actual function binding required")
	}
	sigs := p.check.GetSignaturesOfType(p.check.GetTypeOfSymbol(actual), checker.SignatureKindCall)
	if len(sigs) != 1 || sigs[0].Declaration() != node || len(sigs[0].Parameters()) != len(decl.Parameters.Nodes) || len(sigs[0].TypeParameters()) != 0 || sigs[0].ThisParameter() != nil || sigs[0].HasRestParameter() || sigs[0].MinArgumentCount() != len(decl.Parameters.Nodes) || p.check.GetReturnTypeOfSignature(sigs[0]) != p.check.GetNumberType() {
		return bad(node, "official required parameters and f64 return signature required")
	}
	body := decl.Body.AsBlock().Statements.Nodes
	if len(body) < 1 || body[len(body)-1].Kind != ast.KindReturnStatement {
		return bad(decl.Body, "flat Array/call body with one final f64 return required")
	}
	id, err := p.bindingID(node.Name())
	if err != nil {
		return nil, nil, err
	}
	fn := arrayFactsFunction{ID: id, Name: strings.Clone(node.Name().Text()), SourcePath: p.file.FileName(), DeclarationRange: expressionOffset(p.file, node), NameRange: expressionOffset(p.file, node.Name()), ReturnType: "f64", ReturnAnnotationRange: expressionOffset(p.file, decl.Type), Bindings: []arrayFactsBinding{}, Body: []arrayFactsStatement{}}
	state := &arrayFactsFunctionState{bindings: map[*ast.Symbol]arrayFactsBinding{}, parameters: map[*ast.Symbol]arrayFactsParameter{}, names: map[string]bool{}, owners: map[string]*arrayFactsLengthState{}}
	fn.Parameters, err = p.parameters(decl.Parameters.Nodes, sigs[0], state)
	if err != nil {
		return nil, nil, err
	}
	return &arrayFactsHeader{node: node, signature: sigs[0], function: fn, state: state}, actual, nil
}

func (p *arrayFactsProjection) function(header *arrayFactsHeader) (arrayFactsFunction, error) {
	var empty arrayFactsFunction
	fn, state := header.function, header.state
	body := header.node.AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
	for _, statement := range body[:len(body)-1] {
		if err := p.ctx.Err(); err != nil {
			return empty, arrayFactsError("canceled", "ordered function statements", err)
		}
		switch statement.Kind {
		case ast.KindVariableStatement:
			binding, fact, err := p.local(statement, state)
			if err != nil {
				return empty, err
			}
			fn.Bindings = append(fn.Bindings, binding)
			fn.Body = append(fn.Body, fact)
		case ast.KindExpressionStatement:
			expression := statement.AsExpressionStatement().Expression
			if expression != nil && expression.Kind == ast.KindCallExpression {
				call, err := p.call(expression, state)
				if err != nil {
					return empty, err
				}
				fn.Body = append(fn.Body, arrayFactsStatement{Kind: "call", Range: expressionOffset(p.file, statement), Value: call})
			} else {
				fact, err := p.mutation(statement, state)
				if err != nil {
					return empty, err
				}
				fn.Body = append(fn.Body, fact)
			}
		default:
			return empty, arrayFactsCapability(p.file, statement, "only const Array declarations, mutation and direct call statements before return")
		}
	}
	tail := body[len(body)-1]
	expression := tail.AsReturnStatement().Expression
	var value *arrayFactsValue
	var err error
	if expression != nil && expression.Kind == ast.KindCallExpression {
		value, err = p.call(expression, state)
	} else {
		value, err = p.rhs(expression, state)
	}
	if err != nil {
		return empty, err
	}
	fn.Body = append(fn.Body, arrayFactsStatement{Kind: "return", Range: expressionOffset(p.file, tail), Value: value})
	if err := p.density(fn.Body, body, state); err != nil {
		return empty, err
	}
	return fn, nil
}
