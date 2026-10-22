// SPDX-License-Identifier: Apache-2.0
// Actual constructor values own signatures; implicit forwarding has no fake AST.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"strconv"
)

func (p *arrayGenericProjection) constructorHeaders(nodes []*ast.Node) error {
	p.classNodes, p.constructors = map[string]*ast.Node{}, map[string]*arrayGenericHeader{}
	for _, node := range nodes {
		id := p.classes[p.check.GetSymbolAtLocation(node.Name())]
		p.classNodes[id] = node
		for _, member := range node.Members() {
			if err := p.annotationWork(); err != nil {
				return err
			}
			if member.Kind != ast.KindConstructor {
				continue
			}
			if p.constructors[id] != nil || member.Body() == nil {
				return p.fail(member, "one actual constructor implementation required")
			}
			header, err := p.header(member, id)
			if err != nil {
				return err
			}
			p.constructors[id] = header
		}
	}
	active := map[string]bool{}
	for _, node := range nodes {
		if err := p.constructorMetadata(p.classes[p.check.GetSymbolAtLocation(node.Name())], active, 0); err != nil {
			return err
		}
	}
	return nil
}

func (p *arrayGenericProjection) constructorMetadata(id string, active map[string]bool, depth int) error {
	if err := p.annotationWork(); err != nil {
		return err
	}
	if depth > 64 || active[id] {
		return arrayFactsError("limit", "v8 constructor base-chain projection depth/cycle", nil)
	}
	typ := p.types[id]
	if typ.Construction != nil {
		return nil
	}
	node := p.classNodes[id]
	if node == nil {
		return arrayFactsError("identity", "constructor owner outside original class closure", nil)
	}
	active[id] = true
	defer delete(active, id)
	if typ.Base != "" {
		if typ.Kind != "reference" {
			return p.fail(node, "value type retains its noninherited nominal identity")
		}
		if err := p.constructorMetadata(typ.Base, active, depth+1); err != nil {
			return err
		}
	}
	static := p.check.GetTypeOfSymbol(p.check.GetSymbolAtLocation(node.Name()))
	sigs := p.check.GetSignaturesOfType(static, checker.SignatureKindConstruct)
	if len(sigs) != 1 {
		return p.fail(node, "one actual closed construct signature required")
	}
	sig, err := p.constructorSignature(id, sigs[0], static, node)
	if err != nil {
		return err
	}
	meta := &arrayGenericConstruction{ID: id + ".ctor", Kind: "implicit-base", Access: "public", Signature: sig, Parameters: []classParameter{}, InitializationPhase: "before-body", InitializationFields: []string{}, Origin: classNodeAnchor(node)}
	if typ.Base != "" {
		meta.Kind = "implicit-derived"
		meta.Access = p.types[typ.Base].Construction.Access
		meta.InitializationPhase = "after-successful-super"
	}
	if header := p.constructors[id]; header != nil {
		meta.Kind = "explicit"
		meta.Origin = classNodeAnchor(header.node)
		if header.node.ModifierFlags()&ast.ModifierFlagsPrivate != 0 {
			meta.Access = "private"
		} else if header.node.ModifierFlags()&ast.ModifierFlagsProtected != 0 {
			meta.Access = "protected"
		}
		for i, param := range header.node.AsConstructorDeclaration().Parameters.Nodes {
			if err := p.annotationWork(); err != nil {
				return err
			}
			meta.Parameters = append(meta.Parameters, classParameter{Symbol: header.function.Parameters[i].ID, Type: header.function.Parameters[i].DeclaredType, Origin: classNodeAnchor(param)})
		}
	} else if typ.Base != "" {
		base := p.types[typ.Base].Construction
		forward := &classImplicitForward{Target: base.ID, BaseSignature: classSignature{Parameters: base.Parameters, Return: typ.Base}, Parameters: []classImplicitParameter{}, Origin: classNodeAnchor(node)}
		baseSigs := p.check.GetSignaturesOfType(p.check.GetTypeOfSymbol(p.check.GetSymbolAtLocation(p.classNodes[typ.Base].Name())), checker.SignatureKindConstruct)
		if len(baseSigs) != 1 || len(base.Parameters) != len(sig.Parameters) {
			return p.fail(node, "actual inherited constructor arity differs")
		}
		for i, param := range sigs[0].Parameters() {
			if err := p.annotationWork(); err != nil {
				return err
			}
			inherited := base.Parameters[i]
			if param != baseSigs[0].Parameters()[i] || sig.Parameters[i] != inherited.Type {
				return p.fail(node, "actual inherited constructor parameter identity differs")
			}
			symbol := meta.ID + ".p" + strconv.Itoa(i)
			meta.Parameters = append(meta.Parameters, classParameter{Symbol: symbol, Type: inherited.Type, Origin: classNodeAnchor(param.Declarations[0]), InheritedFrom: inherited.Symbol})
			proof, err := p.constructorForwardCompatibility(param, baseSigs[0].Parameters()[i], inherited.Type, node)
			if err != nil {
				return err
			}
			forward.Parameters = append(forward.Parameters, classImplicitParameter{Source: symbol, Target: inherited.Symbol, Compatibility: proof})
		}
		meta.ImplicitForward = forward
	} else if len(sig.Parameters) != 0 {
		return p.fail(node, "actual default base constructor has unexpected parameters")
	}
	// The owned Program pins native ESNext fields. Their initializers run
	// before constructor-body parameter-property assignments, unlike downlevel
	// classfields transforms. Catalogue/layout order remains independent.
	for _, parameters := range []bool{false, true} {
		for _, field := range *typ.Fields {
			if err := p.annotationWork(); err != nil {
				return err
			}
			if (field.ParameterBinding != "") == parameters {
				meta.InitializationFields = append(meta.InitializationFields, field.ID)
			}
		}
	}
	typ.Construction = meta
	p.types[id] = typ
	return nil
}

func (p *arrayGenericProjection) constructorSignature(id string, signature *checker.Signature, static *checker.Type, at *ast.Node) (arrayGenericSignature, error) {
	var empty arrayGenericSignature
	node := p.classNodes[id]
	if node == nil {
		return empty, p.fail(at, "constructor owner must join its original class")
	}
	expected := p.check.GetTypeOfSymbol(p.check.GetSymbolAtLocation(node.Name()))
	sigs := p.check.GetSignaturesOfType(static, checker.SignatureKindConstruct)
	if static != expected || len(sigs) != 1 || signature != sigs[0] || p.check.GetReturnTypeOfSignature(signature) != p.actual[id] {
		return empty, p.fail(at, "actual selected constructor value/signature differs from owner")
	}
	if signature.HasRestParameter() || signature.MinArgumentCount() != len(signature.Parameters()) {
		return empty, p.fail(at, "constructor requires the existing required-parameter source capability")
	}
	decl := signature.Declaration()
	if decl == nil {
		decl = node
	}
	originOwner := decl
	if decl.Kind == ast.KindConstructor {
		originOwner = decl.Parent
	}
	found := false
	for current, depth := id, 0; current != ""; current, depth = p.types[current].Base, depth+1 {
		if err := p.annotationWork(); err != nil {
			return empty, err
		}
		if depth > 64 {
			return empty, arrayFactsError("limit", "v8 construct signature ancestry depth", nil)
		}
		if p.classNodes[current] == originOwner {
			found = true
			break
		}
	}
	if !found || decl.Kind != ast.KindConstructor && decl.Kind != ast.KindClassDeclaration {
		return empty, p.fail(at, "constructor signature declaration is not its actual owner/ancestor")
	}
	idHash, err := genericNodeID("tsfg.array-facts.signature.v8\x00", decl)
	if err != nil {
		return empty, err
	}
	result := arrayGenericSignature{ID: idHash, SourcePath: ast.GetSourceFileOfNode(decl).FileName(), DeclarationRange: *genericSpan(decl), Parameters: []string{}, ReturnType: id}
	for _, param := range signature.Parameters() {
		if err := p.annotationWork(); err != nil {
			return empty, err
		}
		if len(param.Declarations) != 1 || param.Declarations[0].Kind != ast.KindParameter {
			return empty, p.fail(at, "actual constructor parameter declaration required")
		}
		original := param.Declarations[0].AsParameterDeclaration()
		if original.QuestionToken != nil || original.Initializer != nil || original.DotDotDotToken != nil {
			return empty, p.fail(at, "constructor requires the existing required-parameter source capability")
		}
		typ, err := p.annotation(original.Type)
		if err != nil {
			return empty, err
		}
		if !p.check.IsTypeAssignableTo(p.check.GetTypeOfSymbolAtLocation(param, at), p.actual[typ]) {
			return empty, p.fail(at, "actual constructor parameter differs from original logical annotation")
		}
		result.Parameters = append(result.Parameters, typ)
	}
	return result, nil
}

func (p *arrayGenericProjection) constructorForwardCompatibility(source, target *ast.Symbol, typ string, node *ast.Node) (classCompatibility, error) {
	view, err := p.classViews()
	if err != nil {
		return classCompatibility{}, err
	}
	from, to := p.check.GetTypeOfSymbolAtLocation(source, node), p.check.GetTypeOfSymbolAtLocation(target, node)
	view.official.types[from], view.official.types[to] = typ, typ
	for _, entry := range []struct {
		owner *ast.Node
		id    string
	}{{p.arrayNode, p.payload.ArrayLength.ID}, {p.viewNode, p.payload.ReadSpanLength.ID}} {
		if err := p.annotationWork(); err != nil {
			return classCompatibility{}, err
		}
		member := entry.owner.AsInterfaceDeclaration().Members.Nodes[1]
		symbol := p.check.GetSymbolAtLocation(member.Name())
		if symbol == nil {
			return classCompatibility{}, p.fail(member, "owned constructor forwarding length symbol required")
		}
		view.official.bindings[symbol] = entry.id
	}
	return view.official.compatibility(from, to, node)
}
