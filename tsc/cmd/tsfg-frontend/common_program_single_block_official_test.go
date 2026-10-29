// SPDX-License-Identifier: Apache-2.0
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	tsjson "github.com/microsoft/typescript-go/internal/json"
)

// Source ranges and Symbols, including repeated spellings, establish this oracle
// before projection. No producer scope, binding or environment selects an owner.
type blockCaptureExpectation struct {
	Entry, Binding            string
	Cells, CapturingFunctions int
	Repeated, Scalar          bool
}

func singleBlockCaptureOfficial19ae(t *testing.T, source string, expected blockCaptureExpectation) func(commonProgramPayload) {
	t.Helper()
	program := commonMethodProgram37q(t, source)
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	t.Cleanup(release)
	rangeOf := func(n *ast.Node) span {
		start := n.Pos()
		for start < n.End() && strings.ContainsRune(" \r\n\t", rune(source[start])) {
			start++
		}
		return span{len(utf16.Encode([]rune(source[:start]))), len(utf16.Encode([]rune(source[:n.End()])))}
	}
	owner := func(n *ast.Node) *ast.Node {
		for p := n.Parent; p != nil; p = p.Parent {
			if p.Kind == ast.KindFunctionDeclaration {
				return p
			}
		}
		return nil
	}
	scope := func(n *ast.Node) *ast.Node {
		for p := n.Parent; p != nil; p = p.Parent {
			if p.Kind == ast.KindBlock {
				return p
			}
		}
		return nil
	}
	identity := func(domain string, b []byte) string {
		canonical := tsjson.Value(b)
		if err := canonical.Canonicalize(); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(append([]byte(domain+"\x00"), canonical...))
		return "sha256:" + hex.EncodeToString(sum[:])
	}
	functions := []*ast.Node{}
	nodes := map[span]*ast.Node{}
	var entry *ast.Node
	var visit func(*ast.Node)
	visit = func(n *ast.Node) {
		nodes[rangeOf(n)] = n
		if n.Kind == ast.KindFunctionDeclaration {
			functions = append(functions, n)
			if n.Name().Text() == expected.Entry && owner(n) == nil {
				if entry != nil {
					t.Fatal("original block entry duplicated")
				}
				entry = n
			}
			if signatures := check.GetSignaturesOfType(check.GetTypeAtLocation(n.Name()), checker.SignatureKindCall); len(signatures) != 1 || signatures[0].Declaration() != n {
				t.Fatal("original function signature differs")
			}
		}
		n.ForEachChild(func(c *ast.Node) bool { visit(c); return false })
	}
	visit(file.AsNode())
	if entry == nil {
		t.Fatal("complete original block entry absent")
	}
	inEntry := func(n *ast.Node) bool {
		for p := n; p != nil; p = p.Parent {
			if p == entry {
				return true
			}
		}
		return false
	}
	functionIDs := map[*ast.Node]string{}
	for _, n := range functions {
		b, err := json.Marshal(struct {
			SourcePath       string `json:"sourcePath"`
			DeclarationRange span   `json:"declarationRange"`
		}{"/src/main.ts", rangeOf(n)})
		if err != nil {
			t.Fatal(err)
		}
		functionIDs[n] = identity("tsfg.array-facts.function.v8", b)
	}
	declarations := map[*ast.Symbol]*ast.Node{}
	bindingIDs := map[*ast.Node]string{}
	delegatedUnions := map[*ast.Node]bool{}
	loopAncestor, allScalar, blockCells := false, true, 0
	visit = func(n *ast.Node) {
		if n.Kind == ast.KindVariableDeclaration && inEntry(n) && n.Name().Text() == expected.Binding {
			symbol := check.GetSymbolAtLocation(n.Name())
			if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != n || n.Type() == nil || scope(n) == nil {
				t.Fatal("unique original block binding Symbol/type/scope absent")
			}
			declarations[symbol] = n
			actual := check.GetTypeFromTypeNode(n.Type())
			delegatedUnions[n] = unionCaptureAnnotation19x(n.Type(), actual)
			members := []*checker.Type{actual}
			if actual.IsUnion() {
				members = actual.Types()
			}
			for _, member := range members {
				allScalar = allScalar && member.Flags()&(checker.TypeFlagsNumberLike|checker.TypeFlagsBooleanLike) != 0
			}
			if scope(n) != owner(n).Body() {
				blockCells++
			}
			for p := n.Parent; p != nil && p != owner(n); p = p.Parent {
				loopAncestor = loopAncestor || p.Kind == ast.KindWhileStatement || p.Kind == ast.KindForStatement || p.Kind == ast.KindForInStatement || p.Kind == ast.KindForOfStatement || p.Kind == ast.KindDoStatement
			}
			b, err := json.Marshal(struct {
				SourcePath string `json:"sourcePath"`
				NameRange  span   `json:"nameRange"`
			}{"/src/main.ts", rangeOf(n.Name())})
			if err != nil {
				t.Fatal(err)
			}
			bindingIDs[n] = identity("tsfg.array-facts.binding.v1", b)
		}
		n.ForEachChild(func(c *ast.Node) bool { visit(c); return false })
	}
	visit(entry)
	uses := map[*ast.Node]map[*ast.Node][]commonProgramCaptureOccurrence{}
	originalUses := map[span]*ast.Node{}
	flows := map[span]string{}
	calls := map[span]*ast.Node{}
	visit = func(n *ast.Node) {
		if n.Kind == ast.KindCallExpression {
			signature := check.GetResolvedSignature(n)
			if signature == nil || signature.Declaration() == nil {
				t.Fatal("original selected block call signature absent")
			}
			calls[rangeOf(n)] = signature.Declaration()
		}
		if n.Kind == ast.KindIdentifier {
			if declaration := declarations[check.GetSymbolAtLocation(n)]; declaration != nil && n != declaration.Name() {
				originalUses[rangeOf(n)] = declaration
				actual := check.GetTypeAtLocation(n)
				members := []*checker.Type{actual}
				if actual.IsUnion() {
					members = actual.Types()
				}
				numbers, booleans := true, true
				for _, member := range members {
					numbers = numbers && member.Flags()&checker.TypeFlagsNumberLike != 0
					booleans = booleans && member.Flags()&checker.TypeFlagsBooleanLike != 0
				}
				if numbers {
					flows[rangeOf(n)] = "i32"
				} else if booleans {
					flows[rangeOf(n)] = "bool"
				}
				fn := owner(n)
				if fn != owner(declaration) {
					if uses[fn] == nil {
						uses[fn] = map[*ast.Node][]commonProgramCaptureOccurrence{}
					}
					write := n.Parent.Kind == ast.KindBinaryExpression && n.Parent.AsBinaryExpression().Left == n && ast.IsAssignmentOperator(n.Parent.AsBinaryExpression().OperatorToken.Kind)
					read := !write || n.Parent.AsBinaryExpression().OperatorToken.Kind != ast.KindEqualsToken
					uses[fn][declaration] = append(uses[fn][declaration], commonProgramCaptureOccurrence{Range: rangeOf(n), Read: read, Write: write})
				}
			}
		}
		n.ForEachChild(func(c *ast.Node) bool { visit(c); return false })
	}
	visit(entry)
	if len(declarations) != expected.Cells || len(uses) != expected.CapturingFunctions || blockCells == 0 || loopAncestor != expected.Repeated || allScalar != expected.Scalar {
		t.Fatalf("independent block/Symbol/loop inventory differs: cells=%d captures=%d blocks=%d repeated=%v scalar=%v", len(declarations), len(uses), blockCells, loopAncestor, allScalar)
	}
	var compareUnion func(commonProgramPayload)
	for _, union := range delegatedUnions {
		if union {
			// The existing official union oracle owns exact constituent identities
			// and checker narrowing; this oracle owns original block placement.
			compareUnion = unionCaptureOfficial19x(t, source, true)
			break
		}
	}
	t.Log("pre-producer official complete Program diagnostics, Symbols, original blocks, loop ancestry, types, checker flow, signatures and UTF16 accesses verified")
	return func(payload commonProgramPayload) {
		t.Helper()
		if compareUnion != nil {
			compareUnion(payload)
		}
		byID := map[string]arrayGenericFunction{}
		for _, fn := range payload.Functions {
			byID[fn.ID] = fn
		}
		if len(byID) != len(functions) {
			t.Fatal("complete original function inventory cropped or duplicated")
		}
		for _, n := range functions {
			fn, found := byID[functionIDs[n]]
			if !found || fn.DeclarationRange != rangeOf(n) || fn.NameRange != rangeOf(n.Name()) || fn.Captures == nil || fn.DeclarationScope == nil {
				t.Fatal("original function identity/range missing")
			}
			if parent := owner(n); parent != nil {
				if fn.LexicalParent == nil || *fn.LexicalParent == nil || **fn.LexicalParent != functionIDs[parent] || *fn.DeclarationScope != rangeOf(scope(n)) {
					t.Fatal("original function block scope or lexical owner replaced")
				}
			} else if fn.LexicalParent != nil || *fn.DeclarationScope != (span{0, len(utf16.Encode([]rune(source)))}) {
				t.Fatal("top-level original scope replaced")
			}
		}
		bindings := map[string]arrayFactsBinding{}
		for _, fn := range payload.Functions {
			for _, b := range fn.Bindings {
				bindings[b.ID] = b
			}
		}
		for _, n := range declarations {
			b, found := bindings[bindingIDs[n]]
			typeName := strings.TrimSpace(source[n.Type().Pos():n.Type().End()])
			if !found || b.Name != expected.Binding || b.Kind != "local" || !delegatedUnions[n] && b.DeclaredType != typeName || b.NameRange != rangeOf(n.Name()) || b.DeclarationRange != rangeOf(n) || b.Writable != (n.Parent.Flags&ast.NodeFlagsConst == 0) || b.LexicalOwner == nil || *b.LexicalOwner == nil || **b.LexicalOwner != functionIDs[owner(n)] {
				t.Fatal("original scalar block binding identity/owner/const differs")
			}
		}
		seenUses, seenCalls, seenDeclarations := map[span]bool{}, map[span]bool{}, map[string]bool{}
		var expression func(*arrayGenericExpression)
		expression = func(v *arrayGenericExpression) {
			if v == nil {
				return
			}
			if n := originalUses[v.Range]; n != nil && v.Kind == "bindingReference" {
				if v.Binding != bindingIDs[n] || v.Type != bindings[bindingIDs[n]].DeclaredType || !delegatedUnions[n] && v.FlowType != flows[v.Range] {
					t.Fatal("original scalar read/flow changed binding")
				}
				seenUses[v.Range] = true
			}
			if v.Target != nil {
				if n := originalUses[v.Target.Range]; n != nil {
					if v.Target.Binding != bindingIDs[n] {
						t.Fatal("original scalar write changed block binding")
					}
					seenUses[v.Target.Range] = true
				}
				expression(v.Target.Receiver)
				expression(v.Target.Index)
			}
			if declaration := calls[v.Range]; declaration != nil {
				if v.Signature == nil || v.Signature.SourcePath != "/src/main.ts" || v.Signature.DeclarationRange != rangeOf(declaration) {
					t.Fatal("original selected block call signature differs")
				}
				seenCalls[v.Range] = true
			}
			for _, child := range []*arrayGenericExpression{v.Receiver, v.Index, v.Expression, v.Value, v.Left, v.Right} {
				expression(child)
			}
			for _, children := range []*[]arrayGenericExpression{v.Arguments, v.Elements} {
				if children != nil {
					for _, child := range *children {
						expression(&child)
					}
				}
			}
		}
		var statements func([]arrayGenericStatement, span)
		statements = func(rows []arrayGenericStatement, currentScope span) {
			for _, row := range rows {
				for n, id := range bindingIDs {
					if row.Binding == id {
						kind := "constDeclaration"
						if bindings[id].Writable {
							kind = "letDeclaration"
						}
						if currentScope != rangeOf(scope(n)) || row.Range != rangeOf(n) || row.Initializer == nil || row.Kind != kind {
							t.Fatal("block binding declaration was hoisted or lost initialization")
						}
						seenDeclarations[id] = true
					}
				}
				if row.Kind == "functionDeclaration" {
					fn := byID[row.FunctionID]
					if fn.DeclarationScope == nil || currentScope != *fn.DeclarationScope || row.Range != fn.DeclarationRange {
						t.Fatal("named function declaration moved out of its original block")
					}
				}
				expression(row.Initializer)
				expression(row.Value)
				expression(row.Condition)
				n := nodes[row.Range]
				if row.Kind == "if" {
					if n == nil || n.Kind != ast.KindIfStatement || row.Then == nil || row.Else == nil {
						t.Fatal("original conditional block tree differs")
					}
					statements(*row.Then, rangeOf(n.AsIfStatement().ThenStatement))
					if n.AsIfStatement().ElseStatement != nil {
						statements(*row.Else, rangeOf(n.AsIfStatement().ElseStatement))
					} else if len(*row.Else) != 0 {
						t.Fatal("extra conditional scope without an original block")
					}
				}
				if row.Kind == "block" || row.Kind == "while" {
					if n == nil || row.Body == nil || row.Kind == "block" && n.Kind != ast.KindBlock || row.Kind == "while" && n.Kind != ast.KindWhileStatement {
						t.Fatal("original block/loop tree differs")
					}
					block := n
					if row.Kind == "while" {
						block = n.AsWhileStatement().Statement
					}
					statements(*row.Body, rangeOf(block))
				}
			}
		}
		for _, n := range functions {
			if !inEntry(n) {
				continue
			}
			fn := byID[functionIDs[n]]
			expected := []commonProgramCapture{}
			for declaration, occurrences := range uses[n] {
				capture := commonProgramCapture{Binding: bindingIDs[declaration], Occurrences: occurrences}
				for _, access := range occurrences {
					capture.Read = capture.Read || access.Read
					capture.Write = capture.Write || access.Write
				}
				expected = append(expected, capture)
			}
			sort.Slice(expected, func(i, j int) bool {
				return expected[i].Occurrences[0].Range.Start < expected[j].Occurrences[0].Range.Start
			})
			if !reflect.DeepEqual(*fn.Captures, expected) {
				t.Fatal("original direct capture Symbol/access directory differs")
			}
			statements(fn.Body, rangeOf(n.Body()))
		}
		if len(seenUses) != len(originalUses) || len(seenCalls) != len(calls) || len(seenDeclarations) != len(bindingIDs) {
			t.Fatal("original complete block accesses/calls/declarations cropped")
		}
	}
}
