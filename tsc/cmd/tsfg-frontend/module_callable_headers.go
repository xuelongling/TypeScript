// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
)

// Only this private request profile reaches callable module headers. Its source
// identities remain snapshot-local; neither structural types nor names select
// indirect runtime targets here.
func moduleCallableHeader(file *ast.SourceFile, check *checker.Checker, node *ast.Node, result *response) (functionInfo, response) {
	bad := func(at *ast.Node) (functionInfo, response) {
		return functionInfo{}, failAt("r02b-d-unsupported", "unsupported", file, at)
	}
	declaration := node.AsFunctionDeclaration()
	actual := check.GetSymbolAtLocation(node.Name())
	if actual == nil {
		return bad(node)
	}
	signatures := check.GetSignaturesOfType(check.GetTypeOfSymbol(actual), checker.SignatureKindCall)
	if len(signatures) != 1 || signatures[0].Declaration() != node {
		return bad(node)
	}
	owned, ok := functionValueSignature(check, signatures[0], map[*checker.Type]bool{})
	if !ok {
		return bad(node)
	}
	presence := make([]*parameterPresence, len(declaration.Parameters.Nodes))
	hasPresence := false
	for i, parameter := range declaration.Parameters.Nodes {
		if check.GetSymbolAtLocation(parameter.Name()) != signatures[0].Parameters()[i] {
			return bad(parameter)
		}
		if owned.Parameters[i] != "function" {
			scalarType, shape, valid := checkedParameter(file, check, parameter)
			if !valid || scalarType != owned.Parameters[i] {
				return bad(parameter)
			}
			presence[i] = shape
			hasPresence = true
		}
	}
	if hasPresence {
		owned.Presence = presence
	}
	binding := symbol{SourcePath: file.FileName(), ID: fmt.Sprintf("s%d", len(result.Symbols)+1), Name: node.Name().Text(), Kind: "function", Type: owned.ReturnType, Range: offset(file, node.Name())}
	result.Symbols = append(result.Symbols, binding)
	return functionInfo{ID: binding.ID, Declaration: node, Signature: *owned, ModuleCallable: true}, response{}
}

func moduleCallableProfile(program *compiler.Program, check *checker.Checker) response {
	// Reuse the module boundary's default integer classification. The official
	// number-like type alone does not encode R01's unannotated i32 literal rule.
	globals := moduleGlobals(program, check)
	for _, file := range program.GetSourceFiles() {
		if strings.HasSuffix(file.FileName(), ".d.ts") {
			continue
		}
		bad := func(node *ast.Node) response { return failAt("r02b-d-unsupported", "unsupported", file, node) }
		for _, node := range file.Statements.Nodes {
			if ast.HasSyntacticModifier(node, ast.ModifierFlagsDefault|ast.ModifierFlagsAmbient|ast.ModifierFlagsAsync|ast.ModifierFlagsDecorator) {
				return bad(node)
			}
			switch node.Kind {
			case ast.KindFunctionDeclaration:
				declaration := node.AsFunctionDeclaration()
				if node.Name() == nil || node.Name().Kind != ast.KindIdentifier || declaration.Body == nil || declaration.Body.Kind != ast.KindBlock || declaration.Type == nil || declaration.TypeParameters != nil {
					return bad(node)
				}
			case ast.KindVariableStatement:
				if node.AsVariableStatement().DeclarationList.Flags&(ast.NodeFlagsLet|ast.NodeFlagsConst) == 0 {
					return bad(node)
				}
				for _, declaration := range localDeclarations(node) {
					if declaration.Name() == nil || declaration.Name().Kind != ast.KindIdentifier || declaration.AsVariableDeclaration().Initializer == nil {
						return bad(declaration)
					}
					typ := globals[check.GetSymbolAtLocation(declaration.Name())].Type
					if typ != "bool" && typ != "i32" {
						return bad(declaration)
					}
				}
			case ast.KindImportDeclaration:
				clause := node.AsImportDeclaration().ImportClause
				if clause != nil && ast.IsTypeOnlyImportDeclaration(clause) {
					return bad(node)
				}
				if clause != nil && clause.AsImportClause().NamedBindings != nil && clause.AsImportClause().NamedBindings.Kind == ast.KindNamedImports {
					for _, element := range clause.AsImportClause().NamedBindings.AsNamedImports().Elements.Nodes {
						if element.AsImportSpecifier().IsTypeOnly {
							return bad(element)
						}
					}
				}
			case ast.KindExportDeclaration:
				declaration := node.AsExportDeclaration()
				if declaration.IsTypeOnly || declaration.ExportClause != nil && declaration.ExportClause.Kind != ast.KindNamedExports {
					return bad(node)
				}
				if declaration.ExportClause != nil {
					for _, element := range declaration.ExportClause.AsNamedExports().Elements.Nodes {
						if element.AsExportSpecifier().IsTypeOnly {
							return bad(element)
						}
					}
				}
			case ast.KindExpressionStatement, ast.KindBlock, ast.KindIfStatement, ast.KindWhileStatement:
				// The existing module initializer owns these ordinary statements.
			default:
				return bad(node)
			}
		}
	}
	return response{}
}
