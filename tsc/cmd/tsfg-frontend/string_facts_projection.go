// SPDX-License-Identifier: Apache-2.0
// The exclusive official checker proves the complete closed module before owned copying.
package main

import (
	"context"
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"slices"
	"strings"
)

type stringFactsPending struct {
	fact  stringFactsFunction
	value string
}

func projectStringFacts(ctx context.Context, r request, input stringFactsInput) (stringFactsPayload, error) {
	var empty stringFactsPayload
	if err := ctx.Err(); err != nil {
		return empty, stringFactsError("canceled", "before Program creation", err)
	}
	program, file, consumed, failure := newOwnedProgram(r)
	if failure != nil {
		return empty, stringFactsDiagnostics("input", "official Program source boundary", failure.Diagnostics)
	}
	if err := stringFactsOfficialDiagnostics(ctx, program, r.RootFile); err != nil {
		return empty, err
	}
	if len(consumed) != len(input.Files) {
		return empty, stringFactsError("input", "complete actual consumed source closure", nil)
	}
	for i, name := range consumed {
		if name != input.Files[i].Path {
			return empty, stringFactsError("input", "actual consumed source identity", nil)
		}
	}
	check, release := program.GetTypeCheckerForFileExclusive(ctx, file)
	defer release()
	if !ast.IsExternalModule(file) {
		return empty, stringFactsCapability(file, file.AsNode(), "explicit module required")
	}
	// A legal unsupported declaration is capability, not another function.
	// Validate the complete top-level shape before applying the function budget.
	for _, node := range file.Statements.Nodes {
		if node.Kind != ast.KindFunctionDeclaration || node.Name() == nil || node.Name().Kind != ast.KindIdentifier {
			return empty, stringFactsCapability(file, node, "only named exported functions")
		}
	}
	if len(file.Statements.Nodes) > 64 {
		return empty, stringFactsError("limit", "module functions", nil)
	}
	pending := make([]stringFactsPending, 0, len(file.Statements.Nodes))
	bindings := map[*ast.Node]*ast.Symbol{}
	names := map[string]bool{}
	for _, node := range file.Statements.Nodes {
		if err := ctx.Err(); err != nil {
			return empty, stringFactsError("canceled", "projection", err)
		}
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
			return empty, stringFactsCapability(file, fn.Body, "complete body must be one literal return")
		}
		literal := body[0].AsReturnStatement().Expression
		if literal.Kind != ast.KindStringLiteral {
			return empty, stringFactsCapability(file, literal, "ordinary quoted string literal required")
		}
		typ := check.GetTypeAtLocation(literal)
		if typ == nil || !typ.IsStringLiteral() {
			return empty, stringFactsCapability(file, literal, "official StringLiteral type required")
		}
		value, ok := typ.AsLiteralType().Value().(string)
		if !ok || value != literal.Text() {
			return empty, stringFactsCapability(file, literal, "official literal value/AST identity")
		}
		fact := stringFactsFunction{Name: strings.Clone(name), SourcePath: strings.Clone(file.FileName()), DeclarationRange: expressionOffset(file, node), Parameters: []string{}, ReturnType: "string",
			Body: []stringFactsStatement{{Kind: "return", Range: expressionOffset(file, body[0]), Value: stringFactsValue{Kind: "stringLiteral", Type: "string", Range: expressionOffset(file, literal)}}}}
		pending = append(pending, stringFactsPending{fact, value})
		bindings[node] = actual
		names[name] = true
	}
	if len(pending) == 0 {
		return empty, stringFactsCapability(file, file.AsNode(), "at least one owned function required")
	}
	moduleSymbol := file.AsNode().Symbol()
	if moduleSymbol == nil {
		return empty, stringFactsCapability(file, file.AsNode(), "actual module symbol required")
	}
	exports := check.GetExportsOfModule(moduleSymbol)
	if len(exports) != len(pending) {
		return empty, stringFactsCapability(file, file.AsNode(), "complete function exports required")
	}
	exportedDeclarations := map[*ast.Node]bool{}
	for _, exported := range exports {
		if exported == nil || exported.Flags&ast.SymbolFlagsAlias != 0 || len(exported.Declarations) != 1 {
			return empty, stringFactsCapability(file, file.AsNode(), "direct unambiguous exports required")
		}
		node := exported.Declarations[0]
		actual := bindings[node]
		if actual == nil || ast.GetSourceFileOfNode(node) != file || exportedDeclarations[node] || exported != actual && actual.ExportSymbol != exported {
			return empty, stringFactsCapability(file, file.AsNode(), "export must join its actual owned declaration")
		}
		exportedDeclarations[node] = true
	}
	// Check every literal and the aggregate unit count before allocating any unit array.
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
	files := make([]stringFactsOwnedFile, 0, len(input.Files))
	for _, entry := range input.Files {
		actual := program.GetSourceFile(entry.Path)
		if actual == nil || actual.Text() != entry.Content {
			return empty, stringFactsError("input", "actual consumed file content", nil)
		}
		files = append(files, stringFactsOwnedFile{Path: strings.Clone(entry.Path), Content: strings.Clone(entry.Content), SHA256: strings.Clone(entry.SHA256), ByteLength: len(entry.Content), UTF16Length: actual.GetPositionMap().UTF8ToUTF16(len(entry.Content))})
	}
	return stringFactsPayload{SourceRoot: strings.Clone(input.SourceRoot), RootFile: strings.Clone(input.RootFile), PositionUnit: "utf16", Config: input.Config,
		Declarations: stringFactsDeclarations{Version: strings.Clone(input.Declarations.Version), Paths: slices.Clone(input.Declarations.Paths)}, Files: files, Functions: functions}, nil
}
