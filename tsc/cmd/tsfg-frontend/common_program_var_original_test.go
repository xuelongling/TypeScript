// SPDX-License-Identifier: Apache-2.0
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	tsjson "github.com/microsoft/typescript-go/internal/json"
	"github.com/microsoft/typescript-go/internal/scanner"
)

// A separate official Program owns the expected Symbol, canonical declaration,
// every redeclaration and occurrence. The producer never selects these facts.
func varBindingOfficial19am(t *testing.T, source, entryName, bindingName string) func(commonProgramPayload) {
	t.Helper()
	program := commonMethodProgram37q(t, source)
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	t.Cleanup(release)
	rangeOf := func(node *ast.Node) span {
		return span{file.GetPositionMap().UTF8ToUTF16(scanner.SkipTrivia(source, node.Pos())), file.GetPositionMap().UTF8ToUTF16(node.End())}
	}
	owner := func(node *ast.Node) *ast.Node {
		for parent := node.Parent; parent != nil; parent = parent.Parent {
			if parent.Kind == ast.KindFunctionDeclaration {
				return parent
			}
		}
		return nil
	}
	identity := func(domain string, raw []byte) string {
		canonical := tsjson.Value(raw)
		if err := canonical.Canonicalize(); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(append([]byte(domain+"\x00"), canonical...))
		return "sha256:" + hex.EncodeToString(sum[:])
	}
	functions := map[*ast.Node]string{}
	nodes := map[span]*ast.Node{}
	var entry *ast.Node
	var original *ast.Symbol
	declarations := map[span]*ast.Node{}
	var walk func(*ast.Node)
	walk = func(node *ast.Node) {
		nodes[rangeOf(node)] = node
		if node.Kind == ast.KindFunctionDeclaration {
			raw, err := json.Marshal(struct {
				SourcePath       string `json:"sourcePath"`
				DeclarationRange span   `json:"declarationRange"`
			}{file.FileName(), rangeOf(node)})
			if err != nil {
				t.Fatal(err)
			}
			functions[node] = identity("tsfg.array-facts.function.v8", raw)
			if node.Name().Text() == entryName && owner(node) == nil {
				if entry != nil {
					t.Fatal("original var entry duplicated")
				}
				entry = node
			}
		}
		node.ForEachChild(func(child *ast.Node) bool { walk(child); return false })
	}
	walk(file.AsNode())
	if entry == nil {
		t.Fatal("original var entry absent")
	}
	walk = func(node *ast.Node) {
		if node.Kind == ast.KindVariableDeclaration && node.Name().Text() == bindingName && owner(node) == entry {
			if node.Parent.Kind != ast.KindVariableDeclarationList || node.Parent.Flags&ast.NodeFlagsBlockScoped != 0 {
				t.Fatal("var fixture must retain its original function-scoped declaration flag")
			}
			symbol := check.GetSymbolAtLocation(node.Name())
			if symbol == nil || symbol.Flags&ast.SymbolFlagsFunctionScopedVariable == 0 || original != nil && original != symbol {
				t.Fatal("original var declarations do not share their original Symbol")
			}
			original = symbol
			declarations[rangeOf(node)] = node
		}
		node.ForEachChild(func(child *ast.Node) bool { walk(child); return false })
	}
	walk(entry)
	if original == nil || original.ValueDeclaration == nil || len(declarations) == 0 {
		t.Fatal("original var Symbol/value declaration absent")
	}
	canonical := original.ValueDeclaration
	if owner(canonical) != entry {
		t.Fatal("original var canonical owner differs")
	}
	raw, err := json.Marshal(struct {
		SourcePath string `json:"sourcePath"`
		NameRange  span   `json:"nameRange"`
	}{file.FileName(), rangeOf(canonical.Name())})
	if err != nil {
		t.Fatal(err)
	}
	bindingID := identity("tsfg.array-facts.binding.v1", raw)
	declared := check.GetTypeOfSymbol(original)
	uses := map[span]*ast.Node{}
	captures := map[*ast.Node][]commonProgramCaptureOccurrence{}
	walk = func(node *ast.Node) {
		if node.Kind == ast.KindIdentifier && check.GetSymbolAtLocation(node) == original {
			isName := node.Parent.Name() == node && (node.Parent.Kind == ast.KindVariableDeclaration || node.Parent.Kind == ast.KindParameter)
			if !isName {
				uses[rangeOf(node)] = node
				if fn := owner(node); fn != entry {
					write := node.Parent.Kind == ast.KindBinaryExpression && node.Parent.AsBinaryExpression().Left == node && ast.IsAssignmentOperator(node.Parent.AsBinaryExpression().OperatorToken.Kind)
					read := !write || node.Parent.AsBinaryExpression().OperatorToken.Kind != ast.KindEqualsToken
					captures[fn] = append(captures[fn], commonProgramCaptureOccurrence{Range: rangeOf(node), Read: read, Write: write})
				}
			}
		}
		node.ForEachChild(func(child *ast.Node) bool { walk(child); return false })
	}
	walk(entry)
	if len(uses) == 0 || len(captures) != 2 {
		t.Fatal("original shared var read/write inventory differs")
	}
	t.Logf("pre-producer original var Symbol: declarations=%d canonical=%v type=%s flags=%d", len(declarations), rangeOf(canonical), check.TypeToString(declared), declared.Flags())
	return func(payload commonProgramPayload) {
		t.Helper()
		byID := map[string]arrayGenericFunction{}
		types := map[string]arrayGenericType{}
		for _, fn := range payload.Functions {
			byID[fn.ID] = fn
		}
		for _, typ := range payload.Types {
			types[typ.ID] = typ
		}
		if len(byID) != len(functions) {
			t.Fatal("complete original var function directory differs")
		}
		for node, id := range functions {
			fn, found := byID[id]
			if !found || fn.Name != node.Name().Text() || fn.DeclarationRange != rangeOf(node) || fn.NameRange != rangeOf(node.Name()) || fn.DeclarationScope == nil {
				t.Fatal("original var function identity/range differs")
			}
			if parent := owner(node); parent != nil {
				block := node.Parent
				for block != nil && block.Kind != ast.KindBlock {
					block = block.Parent
				}
				if block == nil || fn.LexicalParent == nil || *fn.LexicalParent == nil || **fn.LexicalParent != functions[parent] || *fn.DeclarationScope != rangeOf(block) {
					t.Fatal("original function lexical owner/block scope differs")
				}
			} else if fn.LexicalParent != nil || *fn.DeclarationScope != (span{0, file.GetPositionMap().UTF8ToUTF16(len(source))}) {
				t.Fatal("original top-level function scope differs")
			}
		}
		fn := byID[functions[entry]]
		var binding arrayFactsBinding
		matches := 0
		for _, b := range append(append([]arrayFactsBinding{}, fn.Parameters...), fn.Bindings...) {
			if b.ID == bindingID {
				binding = b
				matches++
			}
		}
		kind := "local"
		if canonical.Kind == ast.KindParameter {
			kind = "parameter"
		}
		match := captureTypeMatcherOfficial19ai(t, check, types, rangeOf)
		if matches != 1 || binding.Name != bindingName || binding.Kind != kind || !binding.Writable || binding.DeclarationRange != rangeOf(canonical) || binding.NameRange != rangeOf(canonical.Name()) || binding.LexicalOwner == nil || *binding.LexicalOwner == nil || **binding.LexicalOwner != functions[entry] || !match(binding.DeclaredType, declared) {
			t.Fatal("original shared var binding/type/parameter/owner differs")
		}
		for node, id := range functions {
			var got []commonProgramCapture
			if byID[id].Captures == nil {
				t.Fatal("original function capture directory absent")
			}
			for _, capture := range *byID[id].Captures {
				if capture.Binding == bindingID {
					got = append(got, capture)
				}
			}
			want := []commonProgramCapture{}
			if occurrences := captures[node]; len(occurrences) != 0 {
				capture := commonProgramCapture{Binding: bindingID, Occurrences: occurrences}
				for _, use := range occurrences {
					capture.Read = capture.Read || use.Read
					capture.Write = capture.Write || use.Write
				}
				want = append(want, capture)
			}
			if len(got) == 0 {
				got = []commonProgramCapture{}
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatal("original shared var capture directory differs")
			}
		}
		seenUses, seenDeclarations := map[span]bool{}, map[span]bool{}
		var expression func(*arrayGenericExpression)
		expression = func(value *arrayGenericExpression) {
			if value == nil {
				return
			}
			if use := uses[value.Range]; use != nil && value.Kind == "bindingReference" {
				if value.Binding != bindingID || value.Type != binding.DeclaredType || !match(value.FlowType, check.GetTypeAtLocation(use)) {
					t.Fatal("original var reference/flow differs")
				}
				seenUses[value.Range] = true
			}
			if value.Target != nil {
				if uses[value.Target.Range] != nil {
					if value.Target.Binding != bindingID || value.Target.Type != binding.DeclaredType {
						t.Fatal("original var assignment changed binding/type")
					}
					seenUses[value.Target.Range] = true
				}
				expression(value.Target.Receiver)
				expression(value.Target.Index)
			}
			for _, child := range []*arrayGenericExpression{value.Expression, value.Receiver, value.Index, value.Value, value.Left, value.Right} {
				expression(child)
			}
			for _, children := range []*[]arrayGenericExpression{value.Arguments, value.Elements} {
				if children != nil {
					for i := range *children {
						expression(&(*children)[i])
					}
				}
			}
		}
		var statements func([]arrayGenericStatement, span)
		statements = func(rows []arrayGenericStatement, scope span) {
			for _, row := range rows {
				if node := declarations[row.Range]; node != nil {
					if seenDeclarations[row.Range] || row.Kind != "varDeclaration" || row.Binding != bindingID || row.NameRange == nil || *row.NameRange != rangeOf(node.Name()) || scope != rangeOf(node.Parent.Parent.Parent) {
						t.Fatal("original var declaration was moved, duplicated, lost or changed to let")
					}
					if node.Type() == nil && row.AnnotationRange != nil || node.Type() != nil && (row.AnnotationRange == nil || *row.AnnotationRange != rangeOf(node.Type())) {
						t.Fatal("original var annotation differs")
					}
					if node.Initializer() == nil && row.Initializer != nil || node.Initializer() != nil && (row.Initializer == nil || row.Initializer.Range != rangeOf(node.Initializer())) {
						t.Fatal("var initializer moved, vanished or was synthesized")
					}
					seenDeclarations[row.Range] = true
				}
				expression(row.Initializer)
				expression(row.Value)
				expression(row.Condition)
				node := nodes[row.Range]
				if row.Kind == "block" || row.Kind == "while" {
					if node == nil || row.Body == nil {
						t.Fatal("original var statement region absent")
					}
					block := node
					if row.Kind == "while" {
						if node.Kind != ast.KindWhileStatement {
							t.Fatal("original while statement kind differs")
						}
						block = node.AsWhileStatement().Statement
					} else if node.Kind != ast.KindBlock {
						t.Fatal("original block statement kind differs")
					}
					statements(*row.Body, rangeOf(block))
				}
				if row.Kind == "if" {
					if node == nil || node.Kind != ast.KindIfStatement || row.Then == nil || row.Else == nil {
						t.Fatal("original conditional region absent")
					}
					statements(*row.Then, rangeOf(node.AsIfStatement().ThenStatement))
					if node.AsIfStatement().ElseStatement != nil {
						statements(*row.Else, rangeOf(node.AsIfStatement().ElseStatement))
					}
				}
				if row.TryBlock != nil {
					statements(row.TryBlock.Body, row.TryBlock.Range)
				}
				if row.CatchClause != nil {
					statements(row.CatchClause.Block.Body, row.CatchClause.Block.Range)
				}
				if row.FinallyBlock != nil {
					statements(row.FinallyBlock.Body, row.FinallyBlock.Range)
				}
			}
		}
		for node, id := range functions {
			statements(byID[id].Body, rangeOf(node.Body()))
		}
		if len(seenUses) != len(uses) || len(seenDeclarations) != len(declarations) {
			t.Fatal("original var references/declarations cropped")
		}
	}
}
