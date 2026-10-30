// SPDX-License-Identifier: Apache-2.0
package main

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

// Original checker objects select constituents and nominal declarations. The
// same boundary serves the union and original-block oracles, including bool's
// two literal constituents sharing one logical bool payload.
func captureTypeMatcherOfficial19ai(t *testing.T, check *checker.Checker, types map[string]arrayGenericType, anchor func(*ast.Node) span) func(string, *checker.Type) bool {
	t.Helper()
	var match func(string, *checker.Type) bool
	match = func(id string, actual *checker.Type) bool {
		typ := types[id]
		if actual.IsUnion() {
			members := []string{id}
			if typ.Kind == "union" {
				members = typ.Members
			}
			for _, original := range actual.Types() {
				found := false
				for _, member := range members {
					found = found || match(member, original)
				}
				if !found {
					return false
				}
			}
			for _, member := range members {
				found := false
				for _, original := range actual.Types() {
					found = found || match(member, original)
				}
				if !found {
					return false
				}
			}
			return true
		}
		switch {
		case actual.Flags()&checker.TypeFlagsNull != 0:
			return typ.Kind == "null"
		case actual.Flags()&checker.TypeFlagsUndefined != 0:
			return typ.Kind == "undefined"
		case actual.Flags()&checker.TypeFlagsStringLike != 0:
			return typ.Kind == "string"
		case actual.Flags()&checker.TypeFlagsNumberLike != 0:
			return typ.Kind == "scalar" && typ.ID == "i32"
		case actual.Flags()&checker.TypeFlagsBooleanLike != 0:
			return typ.Kind == "scalar" && typ.ID == "bool"
		case check.IsArrayType(actual):
			arguments := check.GetTypeArguments(actual)
			return typ.Kind == "array" && len(arguments) == 1 && match(typ.ElementType, arguments[0])
		}
		if signatures := check.GetSignaturesOfType(actual, checker.SignatureKindCall); len(signatures) == 1 {
			if typ.Kind != "function" || typ.Parameters == nil || len(*typ.Parameters) != len(signatures[0].Parameters()) || !match(typ.ReturnType, check.GetReturnTypeOfSignature(signatures[0])) {
				return false
			}
			for index, parameter := range signatures[0].Parameters() {
				if !match((*typ.Parameters)[index], check.GetTypeOfSymbol(parameter)) {
					return false
				}
			}
			return true
		}
		symbol := actual.Symbol()
		if symbol == nil || len(symbol.Declarations) != 1 {
			return false
		}
		declaration := symbol.Declarations[0]
		kind := "reference"
		if declaration.Modifiers() != nil {
			for _, modifier := range declaration.Modifiers().Nodes {
				if modifier.Kind == ast.KindDecorator {
					marker := check.GetSymbolAtLocation(modifier.AsDecorator().Expression)
					if marker == nil || marker.Name != "tsfgValue" || len(marker.Declarations) != 1 || ast.GetSourceFileOfNode(marker.Declarations[0]).FileName() != "/src/minimal-arrays-v8.d.ts" {
						t.Fatal("original capture nominal marker differs")
					}
					kind = "value"
				}
			}
		}
		return typ.Kind == kind && typ.SourcePath == ast.GetSourceFileOfNode(declaration).FileName() && typ.DeclarationRange != nil && *typ.DeclarationRange == anchor(declaration)
	}
	return match
}
