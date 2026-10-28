// SPDX-License-Identifier: Apache-2.0
package main

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
)

// Observe the original official Program before projection. A const binding is
// not a readonly field; field and element writes retain their original targets.
func commonConstValueFieldsOfficial19v(t *testing.T, file *ast.SourceFile, check *checker.Checker, anchor func(*ast.Node) span) func(commonProgramPayload) {
	t.Helper()
	var held, reader *ast.Node
	writes := []*ast.Node{}
	var visit func(*ast.Node)
	visit = func(n *ast.Node) {
		if n.Kind == ast.KindVariableDeclaration && n.Name().Text() == "held" {
			held = n
			if n.Parent.Flags&ast.NodeFlagsConst == 0 {
				t.Fatal("original captured value must be const")
			}
		}
		if n.Kind == ast.KindFunctionDeclaration && n.Name().Text() == "readOwned" {
			reader = n
			for _, statement := range n.Body().AsBlock().Statements.Nodes {
				if statement.Kind != ast.KindExpressionStatement {
					continue
				}
				binary := statement.AsExpressionStatement().Expression
				if binary.Kind != ast.KindBinaryExpression || binary.AsBinaryExpression().OperatorToken.Kind != ast.KindPlusEqualsToken {
					t.Fatal("original closure write must retain compound assignment")
				}
				left := binary.AsBinaryExpression().Left
				if left.Kind == ast.KindPropertyAccessExpression {
					field := check.GetSymbolAtLocation(left.Name())
					if field == nil || len(field.Declarations) != 1 || field.Declarations[0].ModifierFlags()&ast.ModifierFlagsReadonly != 0 {
						t.Fatal("original field must be writable independently of const binding")
					}
				} else if left.Kind != ast.KindElementAccessExpression || !check.IsArrayType(check.GetTypeAtLocation(left.AsElementAccessExpression().Expression)) {
					t.Fatal("original managed element target must be an Array")
				}
				writes = append(writes, binary)
			}
		}
		n.ForEachChild(func(child *ast.Node) bool { visit(child); return false })
	}
	visit(file.AsNode())
	if held == nil || reader == nil || len(writes) == 0 {
		t.Fatal("original const field-write oracle is empty")
	}
	return func(payload commonProgramPayload) {
		t.Helper()
		var binding string
		for _, fn := range payload.Functions {
			for _, b := range fn.Bindings {
				if b.DeclarationRange == anchor(held) {
					if b.Writable || b.Name != "held" {
						t.Fatal("original const value binding became writable")
					}
					binding = b.ID
				}
			}
		}
		if binding == "" {
			t.Fatal("original const binding missing")
		}
		// Join each original property Symbol through its declaration, never its
		// spelling alone; Held.n and Managed.n are distinct fields.
		fieldID := func(property *ast.Node) string {
			symbol := check.GetSymbolAtLocation(property.Name())
			if symbol == nil || len(symbol.Declarations) != 1 {
				t.Fatal("original field Symbol missing")
			}
			declaration := symbol.Declarations[0]
			location := anchor(declaration)
			for _, typ := range payload.Types {
				if typ.Fields == nil {
					continue
				}
				for _, field := range *typ.Fields {
					if field.Origin.Source == ast.GetSourceFileOfNode(declaration).FileName() && field.Origin.Start == location.Start && field.Origin.End == location.End && field.Name == property.Name().Text() && field.Readonly == (declaration.ModifierFlags()&ast.ModifierFlagsReadonly != 0) {
						return field.ID
					}
				}
			}
			t.Fatal("original field declaration missing")
			return ""
		}
		for _, fn := range payload.Functions {
			if fn.DeclarationRange != anchor(reader) {
				continue
			}
			captureFound, observed := false, 0
			for _, capture := range *fn.Captures {
				if capture.Binding == binding {
					if !capture.Read || capture.Write {
						t.Fatal("field writes must not rebind the captured const slot")
					}
					captureFound = true
				}
			}
			for _, row := range fn.Body {
				if row.Kind != "expression" || row.Value == nil {
					continue
				}
				if observed >= len(writes) {
					t.Fatal("extra closure write published")
				}
				original, value := writes[observed].AsBinaryExpression(), row.Value
				left, target := original.Left, value.Target
				if value.Range != anchor(writes[observed]) || value.Operator != "+=" || value.OperatorRange == nil || *value.OperatorRange != anchor(original.OperatorToken) || value.Type != "i32" || target == nil || target.Range != anchor(left) {
					t.Fatal("original closure write/operator/target occurrence differs")
				}
				var receiver *ast.Node
				if left.Kind == ast.KindPropertyAccessExpression {
					if value.Kind != "fieldAssignment" || target.Kind != "fieldTarget" || target.NameRange == nil || *target.NameRange != anchor(left.Name()) || target.Property != fieldID(left) {
						t.Fatal("original writable field target differs")
					}
					receiver = left.AsPropertyAccessExpression().Expression
				} else {
					if value.Kind != "elementCompoundAssignment" || target.Kind != "elementTarget" || target.Index == nil || target.Index.Range != anchor(left.AsElementAccessExpression().ArgumentExpression) {
						t.Fatal("original managed Array element target differs")
					}
					receiver = left.AsElementAccessExpression().Expression
				}
				projected := target.Receiver
				for receiver.Kind == ast.KindPropertyAccessExpression {
					if projected == nil || projected.Kind != "fieldAccess" || projected.Range != anchor(receiver) || projected.Property != fieldID(receiver) || projected.NameRange == nil || *projected.NameRange != anchor(receiver.Name()) {
						t.Fatal("original nested managed field occurrence differs")
					}
					receiver, projected = receiver.AsPropertyAccessExpression().Expression, projected.Receiver
				}
				if receiver.Kind != ast.KindIdentifier || check.GetSymbolAtLocation(receiver) != check.GetSymbolAtLocation(held.Name()) || projected == nil || projected.Kind != "bindingReference" || projected.Binding != binding || projected.Range != anchor(receiver) {
					t.Fatal("field write lost its original captured const binding")
				}
				observed++
			}
			if !captureFound || observed != len(writes) {
				t.Fatal("original const capture or write occurrence missing")
			}
			return
		}
		t.Fatal("original writing closure missing")
	}
}
