// SPDX-License-Identifier: Apache-2.0
// Both versions share official checking and owner joins; their value DTOs remain distinct.
package main

import (
	"context"
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"slices"
	"strings"
)

type stringFactsPending struct {
	fact  stringFactsFunction
	value string
}

// Borrowed nodes never leave the caller's exclusive checker scope or enter a DTO.
type stringFactsOwner struct {
	name             string
	symbol           *ast.Symbol
	value            *ast.Node
	declarationRange span
	returnRange      span
}

func stringFactsCheckedProgram(ctx context.Context, r request, input stringFactsInput) (*compiler.Program, *ast.SourceFile, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, stringFactsError("canceled", "before Program creation", err)
	}
	program, file, consumed, failure := newOwnedProgram(r)
	if failure != nil {
		return nil, nil, stringFactsDiagnostics("input", "official Program source boundary", failure.Diagnostics)
	}
	if err := stringFactsOfficialDiagnostics(ctx, program, r.RootFile); err != nil {
		return nil, nil, err
	}
	if len(consumed) != len(input.Files) {
		return nil, nil, stringFactsError("input", "complete actual consumed source closure", nil)
	}
	for i, name := range consumed {
		if name != input.Files[i].Path {
			return nil, nil, stringFactsError("input", "actual consumed source identity", nil)
		}
	}
	return program, file, nil
}

func stringFactsModuleShape(file *ast.SourceFile) error {
	if !ast.IsExternalModule(file) {
		return stringFactsCapability(file, file.AsNode(), "explicit module required")
	}
	// Unsupported declarations stay capability errors, even at the function limit.
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Name() == nil || node.Name().Kind != ast.KindIdentifier {
			return stringFactsCapability(file, node, "only named exported functions")
		}
	}
	if len(file.Statements.Nodes) > 64 {
		return stringFactsError("limit", "module functions", nil)
	}
	return nil
}

func stringFactsCheckedFunction(file *ast.SourceFile, check *checker.Checker, node *ast.Node, names map[string]bool, bodyDetail string) (stringFactsOwner, error) {
	var empty stringFactsOwner
	fn := node.AsFunctionDeclaration()
	modifiers := node.ModifierFlags()
	if modifiers&ast.ModifierFlagsExport == 0 || modifiers & ^(ast.ModifierFlagsExport|ast.ModifierFlagsDefault) != 0 ||
		fn.AsteriskToken != nil || fn.TypeParameters != nil && len(fn.TypeParameters.Nodes) != 0 || fn.Parameters == nil || len(fn.Parameters.Nodes) != 0 ||
		fn.Type == nil || fn.FullSignature != nil || fn.Body == nil || fn.Body.Kind != ast.KindBlock {
		return empty, stringFactsCapability(file, node, "closed zero-parameter function declaration")
	}
	name := fn.Name().Text()
	if names[name] {
		return empty, stringFactsCapability(file, node, "unique function declaration owner")
	}
	actual := check.GetSymbolAtLocation(fn.Name())
	if actual == nil || len(actual.Declarations) != 1 || actual.Declarations[0] != node || ast.GetSourceFileOfNode(node) != file {
		return empty, stringFactsCapability(file, node, "unique actual function binding")
	}
	signatures := check.GetSignaturesOfType(check.GetTypeOfSymbol(actual), checker.SignatureKindCall)
	declared := check.GetTypeFromTypeNode(fn.Type)
	if declared == nil || declared.Flags() != checker.TypeFlagsString || len(signatures) != 1 || signatures[0].Declaration() != node ||
		len(signatures[0].Parameters()) != 0 || len(signatures[0].TypeParameters()) != 0 || check.GetReturnTypeOfSignature(signatures[0]) != declared {
		return empty, stringFactsCapability(file, fn.Type, "official non-generic string return signature")
	}
	body := fn.Body.AsBlock().Statements.Nodes
	if len(body) != 1 || body[0].Kind != ast.KindReturnStatement || body[0].AsReturnStatement().Expression == nil {
		return empty, stringFactsCapability(file, fn.Body, bodyDetail)
	}
	return stringFactsOwner{name: name, symbol: actual, value: body[0].AsReturnStatement().Expression,
		declarationRange: expressionOffset(file, node), returnRange: expressionOffset(file, body[0])}, nil
}

func stringFactsLiteralValue(file *ast.SourceFile, check *checker.Checker, literal *ast.Node) (string, error) {
	if literal.Kind != ast.KindStringLiteral {
		return "", stringFactsCapability(file, literal, "ordinary quoted string literal required")
	}
	typ := check.GetTypeAtLocation(literal)
	if typ == nil || !typ.IsStringLiteral() {
		return "", stringFactsCapability(file, literal, "official StringLiteral type required")
	}
	value, ok := typ.AsLiteralType().Value().(string)
	if !ok || value != literal.Text() {
		return "", stringFactsCapability(file, literal, "official literal value/AST identity")
	}
	return value, nil
}

func stringFactsFunctionExports(file *ast.SourceFile, check *checker.Checker, bindings map[*ast.Node]*ast.Symbol) error {
	if len(bindings) == 0 {
		return stringFactsCapability(file, file.AsNode(), "at least one owned function required")
	}
	moduleSymbol := file.AsNode().Symbol()
	if moduleSymbol == nil {
		return stringFactsCapability(file, file.AsNode(), "actual module symbol required")
	}
	exports := check.GetExportsOfModule(moduleSymbol)
	if len(exports) != len(bindings) {
		return stringFactsCapability(file, file.AsNode(), "complete function exports required")
	}
	exportedDeclarations := map[*ast.Node]bool{}
	for _, exported := range exports {
		if exported == nil || exported.Flags&ast.SymbolFlagsAlias != 0 || len(exported.Declarations) != 1 {
			return stringFactsCapability(file, file.AsNode(), "direct unambiguous exports required")
		}
		node := exported.Declarations[0]
		actual := bindings[node]
		if actual == nil || ast.GetSourceFileOfNode(node) != file || exportedDeclarations[node] || exported != actual && actual.ExportSymbol != exported {
			return stringFactsCapability(file, file.AsNode(), "export must join its actual owned declaration")
		}
		exportedDeclarations[node] = true
	}
	return nil
}

func stringFactsOwnedFiles(program *compiler.Program, input stringFactsInput) ([]stringFactsOwnedFile, error) {
	files := make([]stringFactsOwnedFile, 0, len(input.Files))
	for _, entry := range input.Files {
		actual := program.GetSourceFile(entry.Path)
		if actual == nil || actual.Text() != entry.Content {
			return nil, stringFactsError("input", "actual consumed file content", nil)
		}
		files = append(files, stringFactsOwnedFile{Path: strings.Clone(entry.Path), Content: strings.Clone(entry.Content), SHA256: strings.Clone(entry.SHA256), ByteLength: len(entry.Content), UTF16Length: actual.GetPositionMap().UTF8ToUTF16(len(entry.Content))})
	}
	return files, nil
}

func projectStringFacts(ctx context.Context, r request, input stringFactsInput) (stringFactsPayload, error) {
	var empty stringFactsPayload
	program, file, err := stringFactsCheckedProgram(ctx, r, input)
	if err != nil {
		return empty, err
	}
	check, release := program.GetTypeCheckerForFileExclusive(ctx, file)
	defer release()
	if err := stringFactsModuleShape(file); err != nil {
		return empty, err
	}
	pending := make([]stringFactsPending, 0, len(file.Statements.Nodes))
	bindings := map[*ast.Node]*ast.Symbol{}
	names := map[string]bool{}
	for _, node := range file.Statements.Nodes {
		if err := ctx.Err(); err != nil {
			return empty, stringFactsError("canceled", "projection", err)
		}
		owner, err := stringFactsCheckedFunction(file, check, node, names, "complete body must be one literal return")
		if err != nil {
			return empty, err
		}
		value, err := stringFactsLiteralValue(file, check, owner.value)
		if err != nil {
			return empty, err
		}
		fact := stringFactsFunction{Name: strings.Clone(owner.name), SourcePath: strings.Clone(file.FileName()), DeclarationRange: owner.declarationRange, Parameters: []string{}, ReturnType: "string",
			Body: []stringFactsStatement{{Kind: "return", Range: owner.returnRange, Value: stringFactsValue{Kind: "stringLiteral", Type: "string", Range: expressionOffset(file, owner.value)}}}}
		pending = append(pending, stringFactsPending{fact, value})
		bindings[node] = owner.symbol
		names[owner.name] = true
	}
	if err := stringFactsFunctionExports(file, check, bindings); err != nil {
		return empty, err
	}
	// Check every literal and aggregate count before allocating any unit array.
	values := make([]string, 0, len(pending))
	for _, item := range pending {
		values = append(values, item.value)
	}
	if _, err := stringFactsTotalUnits(values); err != nil {
		return empty, err
	}
	functions := make([]stringFactsFunction, 0, len(pending))
	for _, item := range pending {
		units, err := stringFactsValueUnits(item.value)
		if err != nil {
			return empty, err
		}
		item.fact.Body[0].Value.CodeUnits = units
		functions = append(functions, item.fact)
	}
	files, err := stringFactsOwnedFiles(program, input)
	if err != nil {
		return empty, err
	}
	return stringFactsPayload{SourceRoot: strings.Clone(input.SourceRoot), RootFile: strings.Clone(input.RootFile), PositionUnit: "utf16", Config: input.Config,
		Declarations: stringFactsDeclarations{Version: strings.Clone(input.Declarations.Version), Paths: slices.Clone(input.Declarations.Paths)}, Files: files, Functions: functions}, nil
}
