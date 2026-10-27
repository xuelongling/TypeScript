// SPDX-License-Identifier: Apache-2.0
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"reflect"
	"strings"
	"testing"
)

// This original Program runs before public projection and supplies parameter
// identity/type/access expectations independently of every produced DTO.
func scalarParameterOriginal19m(t *testing.T, source string, sample scalarParameterCase19m) *compiler.Program {
	t.Helper()
	program := commonMethodProgram37q(t, source)
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	functions := map[string]*ast.Node{}
	classes := 0
	var walk func(*ast.Node, string)
	walk = func(n *ast.Node, parent string) {
		if n.Kind == ast.KindFunctionDeclaration {
			if parent != "" {
				parent += "."
			}
			parent += n.Name().Text()
			functions[parent] = n
		}
		if n.Kind == ast.KindClassDeclaration {
			classes++
		}
		n.ForEachChild(func(c *ast.Node) bool { walk(c, parent); return false })
	}
	walk(file.AsNode(), "")
	owner, reader := functions["makeOwnedReader"], functions[sample.reader]
	if len(functions) != sample.functions || classes != 3 || owner == nil || reader == nil {
		t.Fatal("independent complete original declaration inventory differs")
	}
	params := owner.AsFunctionDeclaration().Parameters.Nodes
	if len(params) != 1 || params[0].Kind != ast.KindParameter || params[0].Parent != owner || params[0].Name().Text() != "seed" || params[0].AsParameterDeclaration().Initializer != nil {
		t.Fatal("original incoming parameter declaration differs")
	}
	param := params[0]
	symbol := check.GetSymbolAtLocation(param.Name())
	if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != param || strings.TrimSpace(source[param.AsParameterDeclaration().Type.Pos():param.AsParameterDeclaration().Type.End()]) != "i32" {
		t.Fatal("unique original scalar parameter Symbol/type differs")
	}
	uses := 0
	var useWalk func(*ast.Node)
	useWalk = func(n *ast.Node) {
		if n.Kind == ast.KindIdentifier && n.Text() == "seed" && n.Parent.Kind != ast.KindVariableDeclaration {
			same := check.GetSymbolAtLocation(n) == symbol
			if sample.shadow == same {
				t.Fatal("shadow/ancestor original Symbol relation differs")
			}
			if !sample.shadow && check.GetTypeAtLocation(n) != check.GetTypeAtLocation(param.Name()) {
				t.Fatal("original parameter use type differs")
			}
			uses++
		}
		n.ForEachChild(func(c *ast.Node) bool { useWalk(c); return false })
	}
	useWalk(reader.Body())
	expected := 1
	if sample.mutation {
		expected = 2
	}
	if uses != expected {
		t.Fatal("complete parameter use directory differs", uses)
	}
	t.Log("pre-producer independent Program: four diagnostic categories zero; full declarations, unique incoming parameter/owner/type/Symbol and shadow relation verified")
	return program
}

func scalarParameterOfficial19m(t *testing.T, source string, program *compiler.Program, p commonProgramPayload, sample scalarParameterCase19m) {
	t.Helper()
	file := program.GetSourceFile("/src/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	byID, facts := map[string]arrayGenericFunction{}, map[string]arrayGenericFunction{}
	types, named := map[string]arrayGenericType{}, map[string]arrayGenericType{}
	for _, f := range p.Functions {
		byID[f.ID] = f
	}
	for _, f := range p.Functions {
		name, parent := f.Name, f.LexicalParent
		for parent != nil && *parent != nil {
			owner, ok := byID[**parent]
			if !ok {
				t.Fatal("original parent absent")
			}
			name = owner.Name + "." + name
			parent = owner.LexicalParent
		}
		facts[name] = f
	}
	functionTypes := 0
	for _, ty := range p.Types {
		types[ty.ID] = ty
		if ty.Name != "" {
			named[ty.Name] = ty
		}
		if ty.Kind == "function" {
			functionTypes++
		}
	}
	index := commonMethodIndex37q(t, source, file, check, facts, named)
	rangeOf := index.rangeOf
	if len(index.declarations) != sample.functions || len(index.classes) != 3 || len(p.Functions) != sample.functions || len(p.Globals) != 2 || len(p.Modules) != 1 || len(p.Modules[0].Exports) != 8 || len(p.Closure.Roots) != 9 || len(p.Closure.RequiredRoots) != 2 || len(p.EntryCompatibilityRoots) != 8 || len(p.TypeAliases) != 1 || functionTypes != 2 {
		t.Fatal("complete original inventory cropped")
	}
	for name, node := range index.declarations {
		fn := facts[name]
		if fn.Captures == nil || fn.DeclarationScope == nil {
			t.Fatal("original lexical facts absent", name)
		}
		parent := commonLexicalOwnerForOracle26i(node)
		if parent == nil {
			if fn.LexicalParent != nil || len(*fn.Captures) != 0 || *fn.DeclarationScope != (span{0, file.GetPositionMap().UTF8ToUTF16(len(source))}) {
				t.Fatal("original top-level scope differs", name)
			}
		} else if fn.LexicalParent == nil || *fn.LexicalParent == nil || byID[**fn.LexicalParent].DeclarationRange != rangeOf(parent) || *fn.DeclarationScope != rangeOf(parent.Body()) {
			t.Fatal("original nested parent/scope differs", name)
		}
		for _, b := range append(append([]arrayFactsBinding{}, fn.Parameters...), fn.Bindings...) {
			if b.LexicalOwner == nil || *b.LexicalOwner == nil || **b.LexicalOwner != fn.ID {
				t.Fatal("original binding owner differs", name, b.Name)
			}
		}
	}
	if facts["ordinary"].Body[1].CatchClause == nil || facts["arrayOrdinary"].Body[0].Initializer == nil || len(*facts["factory.bump"].Captures) != 1 || !(*facts["factory.bump"].Captures)[0].Write || len(*named["Delta"].Fields) != 1 || len(*named["Box"].Fields) != 1 || len(*named["ReaderBox"].Fields) != 1 {
		t.Fatal("unrelated original business lost")
	}
	globals := map[string]bool{}
	for _, g := range p.Globals {
		if g.Binding.LexicalOwner != nil {
			t.Fatal("global acquired lexical owner")
		}
		globals[g.Binding.Name] = true
	}
	if !globals["leftTotal"] || !globals["rightTotal"] {
		t.Fatal("original globals lost")
	}
	owner, reader := facts["makeOwnedReader"], facts[sample.reader]
	ownerNode, readerNode := index.declarations["makeOwnedReader"], index.declarations[sample.reader]
	originalParam := ownerNode.AsFunctionDeclaration().Parameters.Nodes[0]
	seed := owner.Parameters[0]
	if len(owner.Parameters) != 1 || seed.Name != "seed" || seed.Kind != "parameter" || !seed.Writable || seed.DeclaredType != "i32" || types[seed.DeclaredType].Kind != "scalar" || seed.DeclarationRange != rangeOf(originalParam) || seed.NameRange != rangeOf(originalParam.Name()) || seed.LexicalOwner == nil || *seed.LexicalOwner == nil || **seed.LexicalOwner != owner.ID {
		t.Fatal("incoming parameter DTO identity/type/writability differs")
	}
	stmts := readerNode.Body().AsBlock().Statements.Nodes
	last := stmts[len(stmts)-1]
	if last.Kind != ast.KindReturnStatement {
		t.Fatal("reader return absent")
	}
	sum := last.AsReturnStatement().Expression.AsBinaryExpression()
	ar := sum.Left.AsBinaryExpression()
	aUse, rUse, seedUse := ar.Left.AsElementAccessExpression().Expression, ar.Right.AsElementAccessExpression().Expression, sum.Right
	var a, r arrayFactsBinding
	for _, b := range owner.Bindings {
		if b.Name == "a" {
			a = b
		}
		if b.Name == "r" {
			r = b
		}
	}
	if a.ID == "" || r.ID == "" || a.Writable || r.Writable || types[a.DeclaredType].Kind != "array" || types[r.DeclaredType].Kind != "readSpan" {
		t.Fatal("original readonly Array/ReadSpan owners differ")
	}
	expected := []commonProgramCapture{}
	capture := func(binding arrayFactsBinding, nodes []*ast.Node, reads, writes []bool) {
		c := commonProgramCapture{Binding: binding.ID, Occurrences: []commonProgramCaptureOccurrence{}}
		for i, n := range nodes {
			c.Read = c.Read || reads[i]
			c.Write = c.Write || writes[i]
			c.Occurrences = append(c.Occurrences, commonProgramCaptureOccurrence{Range: rangeOf(n), Read: reads[i], Write: writes[i]})
		}
		expected = append(expected, c)
	}
	if sample.mutation {
		first := stmts[0].AsExpressionStatement().Expression.AsBinaryExpression()
		e := reader.Body[0].Value
		if first.OperatorToken.Kind != ast.KindPlusEqualsToken || first.Right.Text() != "1" || e.Kind != "bindingAssignment" || e.Operator != "+=" || e.Target.Binding != seed.ID || e.Value.Decimal != "1" {
			t.Fatal("incoming shared parameter write lost")
		}
		capture(seed, []*ast.Node{first.Left, seedUse}, []bool{true, true}, []bool{true, false})
	}
	capture(a, []*ast.Node{aUse}, []bool{true}, []bool{false})
	capture(r, []*ast.Node{rUse}, []bool{true}, []bool{false})
	if !sample.mutation && !sample.shadow {
		capture(seed, []*ast.Node{seedUse}, []bool{true}, []bool{false})
	}
	if !reflect.DeepEqual(*reader.Captures, expected) {
		t.Fatal("whole original capture/access/occurrence order differs", *reader.Captures, expected)
	}
	value := reader.Body[len(reader.Body)-1].Value
	if value.Kind != "binary" || value.Operator != "+" || value.Left.Left.Receiver.Binding != a.ID || value.Left.Right.Receiver.Binding != r.ID || value.Right.Kind != "bindingReference" {
		t.Fatal("complete Array/ReadSpan/parameter read tree differs")
	}
	if !sample.shadow && value.Right.Binding != seed.ID {
		t.Fatal("direct/ancestor parameter identity replaced")
	}
	if sample.shadow {
		var local arrayFactsBinding
		for _, b := range reader.Bindings {
			if b.Name == "seed" {
				local = b
			}
		}
		if local.ID == "" || value.Right.Binding != local.ID || local.ID == seed.ID || local.Writable {
			t.Fatal("shadow local incorrectly captured parameter")
		}
	}
	loop, loopNode := facts["loopRead"], index.declarations["loopRead"]
	final := loopNode.Body().AsBlock().Statements.Nodes
	ret := final[len(final)-1].AsReturnStatement().Expression
	dto := loop.Body[len(loop.Body)-1].Value
	var join func(*ast.Node, *arrayGenericExpression)
	join = func(n *ast.Node, e *arrayGenericExpression) {
		if n.Kind == ast.KindBinaryExpression {
			v := n.AsBinaryExpression()
			if v.OperatorToken.Kind != ast.KindPlusToken || e.Kind != "binary" || e.Operator != "+" {
				t.Fatal("ordered final sum differs")
			}
			join(v.Left, e.Left)
			join(v.Right, e.Right)
			return
		}
		if n.Kind != ast.KindCallExpression || e.Kind != "indirectCall" || e.Range != rangeOf(n) || e.Signature == nil || e.Signature.DeclarationRange != rangeOf(check.GetResolvedSignature(n).Declaration()) || len(n.AsCallExpression().Arguments.Nodes) != 0 {
			t.Fatal("original ordered indirect signature differs")
		}
	}
	join(ret, dto)
	t.Log("complete original AST/checker/UTF16/parameter declaration/owner/type/writable, all captures/occurrences, inventory and selected indirect signatures verified; no environment/GC/execution claim")
}
