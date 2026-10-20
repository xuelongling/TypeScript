// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Constructor signatures follow actual values and owned logical parameters.
// Official default signatures may have no declaration or inherit an ancestor's.
package main

import (
	"slices"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

func (p *classProjection) constructorSignature(signature *checker.Signature, static *checker.Type, target classDeclaration, node *ast.Node) (classSignature, error) {
	if signature == nil || static == nil || p.types[static] != target.ConstructorType {
		return classSignature{}, classError("identity", "official constructor value differs from its owned class", classNodeAnchor(node))
	}
	signatures := p.check.GetSignaturesOfType(static, checker.SignatureKindConstruct)
	if len(signatures) != 1 || signatures[0] != signature {
		return classSignature{}, classError("identity", "official selected signature differs from its actual constructor value", classNodeAnchor(node))
	}
	result, err := p.typeRef(p.check.GetReturnTypeOfSignature(signature), node)
	if err != nil {
		return classSignature{}, err
	}
	if target.Constructor.ID == "" || result != target.InstanceType ||
		len(signature.Parameters()) != len(target.Constructor.Signature.Parameters) {
		return classSignature{}, classError("identity", "official constructor result/parameter closure differs from its owned header", classNodeAnchor(node))
	}
	// The header already records actual inherited parameter provenance and its
	// independent allocation-specific symbols. Never rebind the source symbols.
	selected := classSignature{Parameters: slices.Clone(target.Constructor.Signature.Parameters), Return: result}
	for ordinal, parameter := range signature.Parameters() {
		if err := p.ctx.Err(); err != nil {
			return classSignature{}, err
		}
		parameterRef, err := p.typeRef(p.check.GetTypeOfSymbolAtLocation(parameter, node), node)
		if err != nil {
			return classSignature{}, err
		}
		if selected.Parameters[ordinal].Type != parameterRef {
			return classSignature{}, classError("identity", "official constructor parameter differs from its owned logical signature", classNodeAnchor(node))
		}
	}
	return selected, nil
}
