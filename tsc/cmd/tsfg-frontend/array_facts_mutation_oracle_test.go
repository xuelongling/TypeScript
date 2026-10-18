// SPDX-License-Identifier: Apache-2.0
// Independent official-object proofs for required scalar parameters and mutations.
package main

import (
	"strconv"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/jsnum"
	"github.com/microsoft/typescript-go/internal/scanner"
)

type arrayOfficialScope25 struct {
	file        *ast.SourceFile
	check       *checker.Checker
	arraySymbol *ast.Symbol
	scalars     map[string]*ast.Symbol
	indexNode   *ast.Node
	lengthNode  *ast.Node
	property    arrayFirstProperty25
	symbols     map[string]*ast.Symbol
	types       map[string]string
	sites       map[string]bool
}

func (s *arrayOfficialScope25) parameters(t *testing.T, nodes []*ast.Node, actual []*ast.Symbol, expected []arrayFirstParameter25) {
	t.Helper()
	if len(nodes) != len(expected) || len(actual) != len(expected) || len(nodes) > 8 {
		t.Fatal("complete original scalar parameter order")
	}
	for i, want := range expected {
		node := nodes[i]
		if node.Kind != ast.KindParameter || node.Name() == nil || node.Name().Kind != ast.KindIdentifier || node.Name().Text() != want.Name || want.Kind != "parameter" || !want.Writable || s.scalars[want.DeclaredType] == nil {
			t.Fatal("original required scalar parameter declaration")
		}
		p := node.AsParameterDeclaration()
		if p.Type == nil || p.Type.Kind != ast.KindTypeReference || p.Initializer != nil || p.QuestionToken != nil || p.DotDotDotToken != nil || node.ModifierFlags() != 0 {
			t.Fatal("plain explicit required parameter, no default/optional/rest/modifier")
		}
		annotation := p.Type.AsTypeReferenceNode()
		if annotation.TypeName == nil || annotation.TypeName.Kind != ast.KindIdentifier || annotation.TypeName.Text() != want.DeclaredType || annotation.TypeArguments != nil || s.check.GetSymbolAtLocation(annotation.TypeName) != s.scalars[want.DeclaredType] || s.check.GetTypeFromTypeNode(p.Type) != s.check.GetNumberType() {
			t.Fatal("parameter annotation joins original pinned scalar alias")
		}
		symbol := s.check.GetSymbolAtLocation(node.Name())
		if symbol == nil || symbol != actual[i] || len(symbol.Declarations) != 1 || symbol.Declarations[0] != node || s.check.GetTypeOfSymbol(symbol) != s.check.GetNumberType() || s.symbols[want.ID] != nil {
			t.Fatal("actual ordered signature parameter Symbol/type")
		}
		arrayFirstAnchor25(t, s.file, node, want.DeclarationRange)
		arrayFirstAnchor25(t, s.file, node.Name(), want.NameRange)
		arrayFirstAnchor25(t, s.file, p.Type, want.AnnotationRange)
		if want.ID != arrayFirstBindingID25(t, s.file.FileName(), want.NameRange) {
			t.Fatal("independent scalar parameter binding preimage")
		}
		s.symbols[want.ID], s.types[want.ID] = symbol, want.DeclaredType
	}
}

func (s *arrayOfficialScope25) local(t *testing.T, statement *ast.Node, fact arrayFirstStatement25, expected arrayFirstBinding25) {
	t.Helper()
	if statement.Kind != ast.KindVariableStatement || fact.Kind != "variable" || fact.Binding != expected.ID || fact.Initializer == nil {
		t.Fatal("complete original const Array statement")
	}
	list := statement.AsVariableStatement().DeclarationList
	if list.Flags&ast.NodeFlagsConst == 0 || len(list.AsVariableDeclarationList().Declarations.Nodes) != 1 {
		t.Fatal("one original const declarator")
	}
	decl := list.AsVariableDeclarationList().Declarations.Nodes[0]
	local := decl.AsVariableDeclaration()
	if decl.Name() == nil || decl.Name().Kind != ast.KindIdentifier || local.Initializer == nil || decl.Name().Text() != expected.Name || expected.Kind != "const" || expected.DeclaredType != "array.f64" || expected.Writable {
		t.Fatal("complete original const Array binding")
	}
	symbol := s.check.GetSymbolAtLocation(decl.Name())
	if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != decl || s.symbols[expected.ID] != nil {
		t.Fatal("unique original local Symbol")
	}
	arrayFirstAnchor25(t, s.file, statement, fact.Range)
	arrayFirstAnchor25(t, s.file, decl, expected.DeclarationRange)
	arrayFirstAnchor25(t, s.file, decl.Name(), expected.NameRange)
	if expected.ID != arrayFirstBindingID25(t, s.file.FileName(), expected.NameRange) {
		t.Fatal("independent original local binding preimage")
	}
	arrayFirstArrayType25(t, s.check, s.check.GetTypeOfSymbol(symbol), s.arraySymbol, s.indexNode, false)
	if local.Type == nil {
		if expected.AnnotationRange != nil || expected.ElementAnnotationRange != nil || fact.Initializer.Kind != "reference" {
			t.Fatal("only original alias may omit annotation")
		}
	} else {
		if local.Type.Kind != ast.KindArrayType || expected.AnnotationRange == nil || expected.ElementAnnotationRange == nil {
			t.Fatal("explicit original f64 Array annotation")
		}
		arrayFirstAnchor25(t, s.file, local.Type, *expected.AnnotationRange)
		element := local.Type.AsArrayTypeNode().ElementType
		arrayFirstAnchor25(t, s.file, element, *expected.ElementAnnotationRange)
		arrayFirstF64Annotation25(t, s.check, element, s.scalars["f64"])
		arrayFirstArrayType25(t, s.check, s.check.GetTypeFromTypeNode(local.Type), s.arraySymbol, s.indexNode, false)
	}
	init := local.Initializer
	if fact.Initializer.Kind == "arrayLiteral" {
		if local.Type == nil || init.Kind != ast.KindArrayLiteralExpression || fact.Initializer.Type != "array.f64" {
			t.Fatal("explicit f64 Array literal root")
		}
		arrayFirstAnchor25(t, s.file, init, fact.Initializer.Range)
		elements := init.AsArrayLiteralExpression().Elements.Nodes
		if len(elements) != len(fact.Initializer.Elements) {
			t.Fatal("complete original dense element list")
		}
		arrayFirstArrayType25(t, s.check, s.check.GetTypeAtLocation(init), s.arraySymbol, s.indexNode, len(elements) == 0)
		if !s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(init), s.check.GetTypeOfSymbol(symbol)) {
			t.Fatal("official literal/context assignment")
		}
		site := "sha256:" + arrayFirstHash25(append([]byte("tsfg.array-facts.allocation-site.v1\x00"), arrayFirstCanonical25(t, map[string]any{"sourcePath": s.file.FileName(), "range": fact.Initializer.Range})...))
		if site != fact.Initializer.AllocationSite || s.sites[site] {
			t.Fatal("distinct original allocation site preimage")
		}
		s.sites[site] = true
		for n, element := range elements {
			arrayFirstLiteral25(t, s.file, s.check, element, fact.Initializer.Elements[n])
		}
	} else {
		arrayFirstReference25(t, s.file, s.check, init, fact.Initializer, s.symbols, s.arraySymbol, s.indexNode)
	}
	s.symbols[expected.ID], s.types[expected.ID] = symbol, "array.f64"
}

func (s *arrayOfficialScope25) value(t *testing.T, node *ast.Node, want *arrayFirstValue25) {
	t.Helper()
	if want == nil || node == nil {
		t.Fatal("complete original value")
	}
	arrayFirstAnchor25(t, s.file, node, want.Range)
	switch want.Kind {
	case "reference":
		if node.Kind != ast.KindIdentifier || s.symbols[want.Binding] == nil || s.types[want.Binding] != want.FlowType || s.check.GetSymbolAtLocation(node) != s.symbols[want.Binding] || s.check.GetTypeAtLocation(node) != s.check.GetNumberType() || want.FlowType != "u32" && want.FlowType != "f64" {
			t.Fatal("original same-scope scalar parameter reference")
		}
	case "numericLiteral":
		arrayFirstLiteral25(t, s.file, s.check, node, *want)
	case "indexLiteral", "lengthLiteral":
		if node.Kind != ast.KindNumericLiteral || want.Type != "u32" || s.file.Text()[scanner.SkipTrivia(s.file.Text(), node.Pos()):node.End()] != want.Token {
			t.Fatal("original scalar literal spelling/type")
		}
		actual := s.check.GetTypeAtLocation(node)
		if actual == nil || !actual.IsNumberLiteral() {
			t.Fatal("official NumberLiteral index or length")
		}
		number, ok := actual.AsLiteralType().Value().(jsnum.Number)
		decimal, err := strconv.ParseUint(want.Decimal, 10, 32)
		if !ok || err != nil || want.Kind == "indexLiteral" && decimal > 4294967294 || float64(number) != float64(decimal) {
			t.Fatal("independent original exact index/length mathematical value")
		}
	case "checkedElementAccess":
		if node.Kind != ast.KindElementAccessExpression || want.Type != "f64" || s.check.GetTypeAtLocation(node) != s.check.GetNumberType() {
			t.Fatal("official Number checked element read")
		}
		access := node.AsElementAccessExpression()
		if access.QuestionDotToken != nil {
			t.Fatal("ordinary element read")
		}
		arrayFirstReference25(t, s.file, s.check, access.Expression, want.Array, s.symbols, s.arraySymbol, s.indexNode)
		s.value(t, access.ArgumentExpression, want.Index)
	default:
		t.Fatalf("unhandled independent value kind: %s", want.Kind)
	}
}

func (s *arrayOfficialScope25) mutation(t *testing.T, statement *ast.Node, want arrayFirstStatement25) {
	t.Helper()
	if statement.Kind != ast.KindExpressionStatement || want.Target == nil || want.Value == nil {
		t.Fatal("complete original mutation statement")
	}
	arrayFirstAnchor25(t, s.file, statement, want.Range)
	expr := statement.AsExpressionStatement().Expression
	if expr == nil || expr.Kind != ast.KindBinaryExpression {
		t.Fatal("original assignment expression")
	}
	binary := expr.AsBinaryExpression()
	if binary.OperatorToken.Kind != ast.KindEqualsToken {
		t.Fatal("ordinary equals assignment operator")
	}
	arrayFirstAnchor25(t, s.file, expr, want.ExpressionRange)
	arrayFirstAnchor25(t, s.file, binary.OperatorToken, want.OperatorRange)
	arrayFirstAnchor25(t, s.file, binary.Left, want.Target.Range)
	if want.Kind == "elementAssignment" {
		if binary.Left.Kind != ast.KindElementAccessExpression || want.Target.Kind != "elementTarget" || want.Target.Type != "f64" {
			t.Fatal("ordinary original element assignment target")
		}
		access := binary.Left.AsElementAccessExpression()
		if access.QuestionDotToken != nil {
			t.Fatal("ordinary element target")
		}
		arrayFirstReference25(t, s.file, s.check, access.Expression, want.Target.Array, s.symbols, s.arraySymbol, s.indexNode)
		s.value(t, access.ArgumentExpression, want.Target.Index)
	} else {
		if want.Kind != "lengthAssignment" || binary.Left.Kind != ast.KindPropertyAccessExpression || want.Target.Kind != "lengthTarget" || want.Target.Type != "u32" || want.Target.Property != s.property.ID {
			t.Fatal("ordinary pinned Array length assignment target")
		}
		access := binary.Left.AsPropertyAccessExpression()
		if access.QuestionDotToken != nil || access.Name() == nil || access.Name().Text() != "length" {
			t.Fatal("original ordinary length property name")
		}
		arrayFirstReference25(t, s.file, s.check, access.Expression, want.Target.Array, s.symbols, s.arraySymbol, s.indexNode)
		arrayFirstAnchor25(t, s.file, access.Name(), want.Target.NameRange)
		property := s.check.GetSymbolAtLocation(access.Name())
		if property == nil || len(property.Declarations) != 1 || property.Declarations[0] != s.lengthNode || s.check.GetPropertyOfType(s.check.GetTypeAtLocation(access.Expression), "length") != property || s.check.GetTypeAtLocation(access.Name()) != s.check.GetNumberType() || s.check.GetTypeOfSymbolAtLocation(property, access.Name()) != s.check.GetNumberType() {
			t.Fatal("original used length Symbol/type joins pinned declaration")
		}
	}
	s.value(t, binary.Right, want.Value)
	if s.check.GetTypeAtLocation(binary.Left) != s.check.GetNumberType() || !s.check.IsTypeAssignableTo(s.check.GetTypeAtLocation(binary.Right), s.check.GetTypeAtLocation(binary.Left)) {
		t.Fatal("official assignment target and RHS assignability")
	}
}
