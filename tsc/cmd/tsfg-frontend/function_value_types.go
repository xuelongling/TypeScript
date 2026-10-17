// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

// Callable descriptions are owned structural facts from the official checker.
// They never decide assignment compatibility or identify runtime call targets.
// A cycle in the official type graph is a current representation capability
// limit, not a source-language error or permission to construct a host union.
func functionValueType(check *checker.Checker, typ *checker.Type) (*signature, bool) {
	_, owned, ok := functionValuePayload(check, typ, map[*checker.Type]bool{})
	return owned, ok && owned != nil
}

func functionValuePayload(check *checker.Checker, typ *checker.Type, active map[*checker.Type]bool) (string, *signature, bool) {
	if typ == nil {
		return "", nil, false
	}
	if kind := scalar(typ); kind != "" {
		return kind, nil, true
	}
	if typ.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown|checker.TypeFlagsUnion|checker.TypeFlagsIntersection) != 0 || active[typ] {
		return "", nil, false
	}
	active[typ] = true
	defer delete(active, typ)
	signatures := check.GetSignaturesOfType(typ, checker.SignatureKindCall)
	if len(signatures) != 1 {
		return "", nil, false
	}
	owned, ok := functionValueSignature(check, signatures[0], active)
	if !ok {
		return "", nil, false
	}
	return "function", owned, true
}

func functionValueSignature(check *checker.Checker, resolved *checker.Signature, active map[*checker.Type]bool) (*signature, bool) {
	if resolved == nil || resolved.Declaration() == nil || len(resolved.TypeParameters()) != 0 || resolved.ThisParameter() != nil || resolved.HasRestParameter() || resolved.MinArgumentCount() != len(resolved.Parameters()) {
		return nil, false
	}
	declaration := resolved.Declaration()
	if declaration.Kind == ast.KindFunctionDeclaration && declaration.AsFunctionDeclaration().TypeParameters != nil {
		return nil, false
	}
	owned := &signature{Parameters: make([]string, 0, len(resolved.Parameters()))}
	parameterSignatures := make([]*signature, len(resolved.Parameters()))
	hasCallable := false
	for index, parameter := range resolved.Parameters() {
		kind, callable, valid := functionValuePayload(check, check.GetTypeOfSymbol(parameter), active)
		if !valid {
			return nil, false
		}
		owned.Parameters = append(owned.Parameters, kind)
		parameterSignatures[index] = callable
		hasCallable = hasCallable || callable != nil
	}
	if hasCallable {
		owned.ParameterSignatures = parameterSignatures
	}
	var valid bool
	owned.ReturnType, owned.ReturnSignature, valid = functionValuePayload(check, check.GetReturnTypeOfSignature(resolved), active)
	return owned, valid
}

func functionValueParameter(check *checker.Checker, node *ast.Node) (*signature, bool) {
	parameter := node.AsParameterDeclaration()
	if node.Name() == nil || node.Name().Kind != ast.KindIdentifier || parameter.Type == nil || parameter.QuestionToken != nil || parameter.Initializer != nil || parameter.DotDotDotToken != nil {
		return nil, false
	}
	return functionValueType(check, check.GetTypeFromTypeNode(parameter.Type))
}

func signatureHasCallable(sig signature) bool {
	return sig.ReturnSignature != nil || len(sig.ParameterSignatures) != 0
}
