// SPDX-License-Identifier: Apache-2.0
// Bindings, signatures and statement order remain owned by original symbols.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

type arrayGenericHeader struct {
	node      *ast.Node
	signature *checker.Signature
	function  arrayGenericFunction
}
type arrayGenericState struct {
	ownerType string
	header    *arrayGenericHeader
	bindings  map[*ast.Symbol]arrayFactsBinding
	lengths   map[string]*arrayFactsLengthState
}

func genericNodeID(domain string, node *ast.Node) (string, error) {
	return arrayFactsID(domain, struct {
		SourcePath       string `json:"sourcePath"`
		DeclarationRange span   `json:"declarationRange"`
	}{ast.GetSourceFileOfNode(node).FileName(), *genericSpan(node)})
}
func (p *arrayGenericProjection) binding(node, annotation *ast.Node, kind string, writable bool) (arrayFactsBinding, error) {
	typ, err := p.annotation(annotation)
	if err != nil {
		return arrayFactsBinding{}, err
	}
	return p.typedBinding(node, annotation, kind, writable, typ)
}
func (p *arrayGenericProjection) typedBinding(node, annotation *ast.Node, kind string, writable bool, typ string) (arrayFactsBinding, error) {
	var empty arrayFactsBinding
	if node.Name() == nil || node.Name().Kind != ast.KindIdentifier {
		return empty, p.fail(node, "original named binding required")
	}
	symbol := p.check.GetSymbolAtLocation(node.Name())
	if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != node {
		return empty, p.fail(node, "binding must own its unique original declaration")
	}
	if !p.check.IsTypeAssignableTo(p.check.GetTypeOfSymbol(symbol), p.actual[typ]) {
		return empty, p.fail(node, "binding official type differs from annotation")
	}
	id, err := arrayFactsBindingID(ast.GetSourceFileOfNode(node).FileName(), *genericSpan(node.Name()))
	if err != nil {
		return empty, err
	}
	var element *span
	if annotation != nil && annotation.Kind == ast.KindArrayType {
		element = genericSpan(annotation.AsArrayTypeNode().ElementType)
	}
	if annotation != nil && annotation.Kind == ast.KindTypeReference && annotation.AsTypeReferenceNode().TypeArguments != nil && len(annotation.AsTypeReferenceNode().TypeArguments.Nodes) == 1 {
		element = genericSpan(annotation.AsTypeReferenceNode().TypeArguments.Nodes[0])
	}
	var annotationRange *span
	if annotation != nil {
		annotationRange = genericSpan(annotation)
	}
	return arrayFactsBinding{ID: id, Name: node.Name().Text(), Kind: kind, DeclaredType: typ, Writable: writable, DeclarationRange: *genericSpan(node), NameRange: *genericSpan(node.Name()), AnnotationRange: annotationRange, ElementAnnotationRange: element}, nil
}
func (p *arrayGenericProjection) header(node *ast.Node, constructorType string) (*arrayGenericHeader, error) {
	var signature *checker.Signature
	var returnNode *ast.Node
	var parameters []*ast.Node
	name := "constructor"
	nameRange := *genericSpan(node)
	if constructorType != "" {
		parameters = node.AsConstructorDeclaration().Parameters.Nodes
		symbol := p.check.GetSymbolAtLocation(node.Parent.Name())
		sigs := p.check.GetSignaturesOfType(p.check.GetTypeOfSymbol(symbol), checker.SignatureKindConstruct)
		if len(sigs) != 1 {
			return nil, p.fail(node, "one original constructor signature required")
		}
		signature = sigs[0]
	} else {
		if node.Kind != ast.KindFunctionDeclaration && node.Kind != ast.KindMethodDeclaration || node.Name() == nil || node.Body() == nil || node.Body().Kind != ast.KindBlock {
			return nil, p.fail(node, "ordinary named function body required")
		}
		var typeParameters *ast.NodeList
		if node.Kind == ast.KindMethodDeclaration {
			decl := node.AsMethodDeclaration()
			typeParameters, returnNode, parameters = decl.TypeParameters, decl.Type, decl.Parameters.Nodes
		} else {
			decl := node.AsFunctionDeclaration()
			typeParameters, returnNode, parameters = decl.TypeParameters, decl.Type, decl.Parameters.Nodes
		}
		if typeParameters != nil {
			return nil, p.fail(node, "generic function requires specialization before array projection")
		}
		name = node.Name().Text()
		nameRange = *genericSpan(node.Name())
		symbol := p.check.GetSymbolAtLocation(node.Name())
		if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != node {
			return nil, p.fail(node, "original unique function declaration")
		}
		sigs := p.check.GetSignaturesOfType(p.check.GetTypeOfSymbol(symbol), checker.SignatureKindCall)
		if len(sigs) != 1 || sigs[0].Declaration() != node {
			return nil, p.fail(node, "one original function signature required")
		}
		signature = sigs[0]
	}
	id, err := genericNodeID("tsfg.array-facts.function.v8\x00", node)
	if err != nil {
		return nil, err
	}
	fn := arrayGenericFunction{ID: id, Name: name, SourcePath: ast.GetSourceFileOfNode(node).FileName(), DeclarationRange: *genericSpan(node), NameRange: nameRange, ReturnType: constructorType, Parameters: []arrayFactsBinding{}, Bindings: []arrayFactsBinding{}, Body: []arrayGenericStatement{}}
	if constructorType != "" {
		fn.OwnerType = constructorType
	}
	if constructorType == "" {
		fn.ReturnType, err = p.annotation(returnNode)
		if err != nil {
			return nil, err
		}
		fn.ReturnAnnotationRange = genericSpan(returnNode)
		if !p.check.IsTypeAssignableTo(p.check.GetReturnTypeOfSignature(signature), p.actual[fn.ReturnType]) {
			return nil, p.fail(node, "original return signature must match annotation")
		}
	}
	for i, node := range parameters {
		decl := node.AsParameterDeclaration()
		if decl.QuestionToken != nil || decl.Initializer != nil || decl.DotDotDotToken != nil {
			return nil, p.fail(node, "required original parameter required")
		}
		v, err := p.binding(node, decl.Type, "parameter", true)
		if err != nil {
			return nil, err
		}
		if len(signature.Parameters()) <= i {
			return nil, p.fail(node, "signature parameter binding differs")
		}
		actual := signature.Parameters()[i]
		if actual != p.check.GetSymbolAtLocation(node.Name()) && (constructorType == "" || len(actual.Declarations) != 1 || actual.Declarations[0] != node) {
			return nil, p.fail(node, "signature parameter must join the same original parameter declaration")
		}
		fn.Parameters = append(fn.Parameters, v)
	}
	return &arrayGenericHeader{node: node, signature: signature, function: fn}, nil
}
func (p *arrayGenericProjection) function(header *arrayGenericHeader) (arrayGenericFunction, error) {
	fn := header.function
	state := &arrayGenericState{header: header, ownerType: fn.OwnerType, bindings: map[*ast.Symbol]arrayFactsBinding{}, lengths: map[string]*arrayFactsLengthState{}}
	var nodes []*ast.Node
	if header.node.Kind == ast.KindConstructor {
		nodes = header.node.AsConstructorDeclaration().Parameters.Nodes
	} else if header.node.Kind == ast.KindMethodDeclaration {
		nodes = header.node.AsMethodDeclaration().Parameters.Nodes
	} else {
		nodes = header.node.AsFunctionDeclaration().Parameters.Nodes
	}
	for i, node := range nodes {
		b := fn.Parameters[i]
		state.bindings[p.check.GetSymbolAtLocation(node.Name())] = b
		state.bindings[header.signature.Parameters()[i]] = b
		if p.types[b.DeclaredType].Kind == "array" {
			state.lengths[b.ID] = &arrayFactsLengthState{parameter: true}
		}
	}
	if header.node.Body() == nil {
		return fn, p.fail(header.node, "original implementation body required")
	}
	body, err := p.statements(header.node.Body().AsBlock().Statements.Nodes, state, &fn, 0)
	if err != nil {
		return fn, err
	}
	fn.Body = body
	return fn, nil
}
func (p *arrayGenericProjection) statements(nodes []*ast.Node, state *arrayGenericState, fn *arrayGenericFunction, depth int) ([]arrayGenericStatement, error) {
	if depth > 48 {
		return nil, arrayFactsError("limit", "v8 statement depth", nil)
	}
	result := []arrayGenericStatement{}
	for _, node := range nodes {
		if err := p.ctx.Err(); err != nil {
			return nil, err
		}
		v := arrayGenericStatement{Range: *genericSpan(node)}
		switch node.Kind {
		case ast.KindVariableStatement:
			list := node.AsVariableStatement().DeclarationList
			for _, local := range list.AsVariableDeclarationList().Declarations.Nodes {
				decl := local.AsVariableDeclaration()
				if decl.Initializer == nil {
					return nil, p.fail(local, "dense local requires initialization")
				}
				kind := "const"
				if list.Flags&ast.NodeFlagsConst == 0 {
					kind = "let"
				}
				var b arrayFactsBinding
				var init *arrayGenericExpression
				var err error
				if decl.Type == nil {
					init, err = p.expression(decl.Initializer, "", state, 0)
					if err == nil {
						b, err = p.typedBinding(local, nil, "local", kind == "let", init.FlowType)
					}
				} else {
					b, err = p.binding(local, decl.Type, "local", kind == "let")
				}
				if err != nil {
					return nil, err
				}
				if init == nil {
					init, err = p.expression(decl.Initializer, b.DeclaredType, state, 0)
				}
				if err != nil {
					return nil, err
				}
				state.bindings[p.check.GetSymbolAtLocation(local.Name())] = b
				fn.Bindings = append(fn.Bindings, b)
				if p.types[b.DeclaredType].Kind == "array" {
					length := &arrayFactsLengthState{}
					if init.Kind == "arrayLiteral" {
						length.known = true
						length.length = uint64(len(*init.Elements))
					} else if init.Binding != "" && state.lengths[init.Binding] != nil {
						length = state.lengths[init.Binding]
					}
					state.lengths[b.ID] = length
				}
				result = append(result, arrayGenericStatement{Kind: kind + "Declaration", Range: *genericSpan(local), Binding: b.ID, Initializer: init})
			}
			continue
		case ast.KindReturnStatement:
			v.Kind = "return"
			expr := node.AsReturnStatement().Expression
			if expr != nil {
				value, err := p.expression(expr, fn.ReturnType, state, 0)
				if err != nil {
					return nil, err
				}
				v.Value = value
			} else if fn.ReturnType != "void" && (state.header == nil || state.header.node.Kind != ast.KindConstructor) {
				return nil, p.fail(node, "value return required by original signature")
			}
		case ast.KindExpressionStatement:
			v.Kind = "expression"
			value, err := p.expression(node.AsExpressionStatement().Expression, "", state, 0)
			if err != nil {
				return nil, err
			}
			v.Value = value
		case ast.KindBlock:
			v.Kind = "block"
			body, err := p.statements(node.AsBlock().Statements.Nodes, state, fn, depth+1)
			if err != nil {
				return nil, err
			}
			v.Body = &body
		case ast.KindIfStatement:
			v.Kind = "if"
			decl := node.AsIfStatement()
			condition, err := p.expression(decl.Expression, "bool", state, 0)
			if err != nil {
				return nil, err
			}
			v.Condition = condition
			// Any branch/loop may change a density owner; retain dynamic checks.
			for _, l := range state.lengths {
				l.known = false
			}
			then, err := p.statementBody(decl.ThenStatement, state, fn, depth+1)
			if err != nil {
				return nil, err
			}
			other := []arrayGenericStatement{}
			if decl.ElseStatement != nil {
				other, err = p.statementBody(decl.ElseStatement, state, fn, depth+1)
				if err != nil {
					return nil, err
				}
			}
			v.Then, v.Else = &then, &other
		case ast.KindWhileStatement:
			v.Kind = "while"
			decl := node.AsWhileStatement()
			for _, l := range state.lengths {
				l.known = false
			}
			condition, err := p.expression(decl.Expression, "bool", state, 0)
			if err != nil {
				return nil, err
			}
			v.Condition = condition
			body, err := p.statementBody(decl.Statement, state, fn, depth+1)
			if err != nil {
				return nil, err
			}
			v.Body = &body
		default:
			return nil, p.fail(node, "statement has no owned array v8 representation")
		}
		result = append(result, v)
	}
	return result, nil
}
func (p *arrayGenericProjection) statementBody(node *ast.Node, state *arrayGenericState, fn *arrayGenericFunction, depth int) ([]arrayGenericStatement, error) {
	if node.Kind == ast.KindBlock {
		return p.statements(node.AsBlock().Statements.Nodes, state, fn, depth)
	}
	return p.statements([]*ast.Node{node}, state, fn, depth)
}
