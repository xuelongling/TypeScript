// SPDX-License-Identifier: Apache-2.0
// Preserve the checked binary operation; neither operand is combined or evaluated here.
package main

import (
	"context"
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"slices"
	"strings"
)

type stringFactsConcatPending struct {
	fact  stringFactsConcatFunction
	left  string
	right string
}

func stringFactsConcatValueOf(file *ast.SourceFile, check *checker.Checker, value *ast.Node) (stringFactsConcatValue, string, string, error) {
	var empty stringFactsConcatValue
	if value.Kind != ast.KindBinaryExpression {
		return empty, "", "", stringFactsCapability(file, value, "binary string concat required")
	}
	binary := value.AsBinaryExpression()
	if binary.Type != nil || value.ModifierFlags() != 0 || binary.OperatorToken == nil || binary.Left == nil || binary.Right == nil {
		return empty, "", "", stringFactsCapability(file, value, "ordinary complete binary expression required")
	}
	if binary.OperatorToken.Kind != ast.KindPlusToken {
		return empty, "", "", stringFactsCapability(file, binary.OperatorToken, "string concat plus required")
	}
	left, err := stringFactsLiteralValue(file, check, binary.Left)
	if err != nil {
		return empty, "", "", err
	}
	right, err := stringFactsLiteralValue(file, check, binary.Right)
	if err != nil {
		return empty, "", "", err
	}
	typ := check.GetTypeAtLocation(value)
	if typ == nil || typ.Flags() != checker.TypeFlagsString {
		return empty, "", "", stringFactsCapability(file, value, "official primitive string concat result required")
	}
	fact := stringFactsConcatValue{Kind: "stringConcat", Type: "string", Range: expressionOffset(file, value), OperatorRange: expressionOffset(file, binary.OperatorToken),
		Left:  stringFactsValue{Kind: "stringLiteral", Type: "string", Range: expressionOffset(file, binary.Left)},
		Right: stringFactsValue{Kind: "stringLiteral", Type: "string", Range: expressionOffset(file, binary.Right)}}
	if fact.Range.Start != fact.Left.Range.Start || fact.Range.End != fact.Right.Range.End ||
		fact.Left.Range.End > fact.OperatorRange.Start || fact.OperatorRange.End != fact.OperatorRange.Start+1 || fact.OperatorRange.End > fact.Right.Range.Start {
		return empty, "", "", stringFactsCapability(file, value, "official concat token ranges required")
	}
	return fact, left, right, nil
}

func projectStringFactsConcat(ctx context.Context, r request, input stringFactsInput) (stringFactsConcatPayload, error) {
	var empty stringFactsConcatPayload
	program, file, err := stringFactsCheckedProgram(ctx, r, input)
	if err != nil {
		return empty, err
	}
	check, release := program.GetTypeCheckerForFileExclusive(ctx, file)
	defer release()
	if err := stringFactsModuleShape(file); err != nil {
		return empty, err
	}
	pending := make([]stringFactsConcatPending, 0, len(file.Statements.Nodes))
	bindings := map[*ast.Node]*ast.Symbol{}
	names := map[string]bool{}
	for _, node := range file.Statements.Nodes {
		if err := ctx.Err(); err != nil {
			return empty, stringFactsError("canceled", "projection", err)
		}
		owner, err := stringFactsCheckedFunction(file, check, node, names, "complete body must be one concat return")
		if err != nil {
			return empty, err
		}
		value, left, right, err := stringFactsConcatValueOf(file, check, owner.value)
		if err != nil {
			return empty, err
		}
		fact := stringFactsConcatFunction{Name: strings.Clone(owner.name), SourcePath: strings.Clone(file.FileName()), DeclarationRange: owner.declarationRange, Parameters: []string{}, ReturnType: "string",
			Body: []stringFactsConcatStatement{{Kind: "return", Range: owner.returnRange, Value: value}}}
		pending = append(pending, stringFactsConcatPending{fact, left, right})
		bindings[node] = owner.symbol
		names[owner.name] = true
	}
	if err := stringFactsFunctionExports(file, check, bindings); err != nil {
		return empty, err
	}
	// Both leaves count independently, before unit arrays; their sum is at most 32768.
	values := make([]string, 0, 2*len(pending))
	for _, item := range pending {
		values = append(values, item.left, item.right)
	}
	if _, err := stringFactsTotalUnits(values); err != nil {
		return empty, err
	}
	functions := make([]stringFactsConcatFunction, 0, len(pending))
	for _, item := range pending {
		left, err := stringFactsValueUnits(item.left)
		if err != nil {
			return empty, err
		}
		right, err := stringFactsValueUnits(item.right)
		if err != nil {
			return empty, err
		}
		item.fact.Body[0].Value.Left.CodeUnits = left
		item.fact.Body[0].Value.Right.CodeUnits = right
		functions = append(functions, item.fact)
	}
	files, err := stringFactsOwnedFiles(program, input)
	if err != nil {
		return empty, err
	}
	return stringFactsConcatPayload{SourceRoot: strings.Clone(input.SourceRoot), RootFile: strings.Clone(input.RootFile), PositionUnit: "utf16", Config: input.Config,
		Declarations: stringFactsDeclarations{Version: strings.Clone(input.Declarations.Version), Paths: slices.Clone(input.Declarations.Paths)}, Files: files, Functions: functions}, nil
}
