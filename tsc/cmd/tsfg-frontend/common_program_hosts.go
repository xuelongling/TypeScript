// SPDX-License-Identifier: Apache-2.0
// Shared original Counter declaration boundary; Common publishes facts, not trust.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"strings"
)

// The existing module and Common routes join the same original ambient node.
func counterHostDeclaration(file *ast.SourceFile, name string) *ast.Node {
	if file == nil {
		return nil
	}
	for _, node := range file.Statements.Nodes {
		if node.Kind == ast.KindFunctionDeclaration && node.Name() != nil && node.Name().Text() == name {
			if node.Body() != nil {
				return nil
			}
			return node
		}
	}
	return nil
}
func commonHostPaths(input commonProgramInput) ([]string, error) {
	paths := []string{}
	if len(input.HostImports) > 1 {
		return nil, arrayFactsError("input", "only the existing Counter31 Host declaration is supported", nil)
	}
	for _, host := range input.HostImports {
		if !arrayFactsPath(host.SourcePath) || !strings.HasPrefix(host.SourcePath, input.SourceRoot+"/") || !strings.HasSuffix(host.SourcePath, ".d.ts") || host.SourcePath == input.SourceRoot+"/minimal-arrays-v8.d.ts" || host.Name == "" || host.CapabilityID != 31 || len(host.ParameterTypes) != 1 || host.ParameterTypes[0] != "i32" || host.ReturnType != "i32" {
			return nil, arrayFactsError("input", "original Counter31 Host declaration/signature required", nil)
		}
		paths = append(paths, host.SourcePath)
	}
	return paths, nil
}
func (p *arrayGenericProjection) commonHostHeaders(files []*ast.SourceFile) error {
	c := p.common
	c.hosts = map[*ast.Symbol]commonProgramHost{}
	c.hostNodes = map[*ast.Symbol]*ast.Node{}
	c.hostImports = []commonProgramHost{}
	if len(files) != len(c.input.HostImports) {
		return arrayFactsError("input", "complete Counter declaration source closure required", nil)
	}
	for i, host := range c.input.HostImports {
		if err := p.annotationWork(); err != nil {
			return err
		}
		file := files[i]
		for range file.Statements.Nodes {
			if err := p.annotationWork(); err != nil {
				return err
			}
		}
		found := counterHostDeclaration(file, host.Name)
		if file.FileName() != host.SourcePath || len(file.Statements.Nodes) != 1 || found == nil {
			return p.fail(file.AsNode(), "unique original ambient Counter declaration required")
		}
		symbol := p.check.GetSymbolAtLocation(found.Name())
		decl := found.AsFunctionDeclaration()
		if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != found || decl.TypeParameters != nil || decl.Type == nil || len(decl.Parameters.Nodes) != 1 {
			return p.fail(found, "unique Counter Symbol and non-generic signature required")
		}
		parameter := decl.Parameters.Nodes[0].AsParameterDeclaration()
		if parameter.Type == nil || parameter.QuestionToken != nil || parameter.Initializer != nil || parameter.DotDotDotToken != nil {
			return p.fail(found, "Counter requires one explicit required parameter")
		}
		param, err := p.annotation(parameter.Type)
		if err != nil {
			return err
		}
		result, err := p.annotation(decl.Type)
		if err != nil {
			return err
		}
		signatures := p.check.GetSignaturesOfType(p.check.GetTypeOfSymbol(symbol), checker.SignatureKindCall)
		if param != "i32" || result != "i32" || len(signatures) != 1 || signatures[0].Declaration() != found || len(signatures[0].Parameters()) != 1 {
			return p.fail(found, "Counter requires original pinned i32 parameter and result")
		}
		id, err := arrayFactsBindingID(file.FileName(), *genericSpan(found.Name()))
		if err != nil {
			return err
		}
		signature, err := genericNodeID("tsfg.array-facts.signature.v8\x00", found)
		if err != nil {
			return err
		}
		row := commonProgramHost{ID: id, SourcePath: file.FileName(), Name: found.Name().Text(), CapabilityID: 31, ParameterTypes: []string{param}, ReturnType: result,
			DeclarationRange: *genericSpan(found), NameRange: *genericSpan(found.Name()), Signature: arrayGenericSignature{ID: signature, SourcePath: file.FileName(), DeclarationRange: *genericSpan(found), Parameters: []string{param}, ReturnType: result}}
		c.hosts[symbol], c.hostNodes[symbol] = row, found
		c.hostImports = append(c.hostImports, row)
	}
	return nil
}
