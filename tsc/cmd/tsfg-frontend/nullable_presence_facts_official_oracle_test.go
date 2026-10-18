// SPDX-License-Identifier: Apache-2.0
// Fresh official observations compare independently frozen literals, ranges and nominal members.
package main

import (
	"fmt"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/scanner"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

func nullablePresenceFirstProgram(t *testing.T, r request) *compiler.Program {
	t.Helper()
	contents := map[string]string{}
	for _, file := range r.Files {
		contents[file.Path] = file.Content
	}
	return compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
			FileNames:       append([]string{r.RootFile}, r.Declarations.Paths...),
			CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext},
		}},
		Host:           compiler.NewCompilerHost(r.SourceRoot, vfstest.FromMap(contents, true), r.SourceRoot, nil, nil),
		SingleThreaded: core.TSTrue,
	})
}

func nullablePresenceFirstDiagnostics(t *testing.T, program *compiler.Program) {
	t.Helper()
	if ds := program.GetSyntacticDiagnostics(t.Context(), nil); len(ds) != 0 {
		t.Fatalf("complete official syntax diagnostics: %+v", ds)
	}
	if ds := program.GetSemanticDiagnostics(t.Context(), nil); len(ds) != 0 {
		t.Fatalf("complete official semantic diagnostics: %+v", ds)
	}
	if ds := program.GetProgramDiagnostics(); len(ds) != 0 {
		t.Fatalf("complete official program diagnostics: %+v", ds)
	}
	if ds := program.GetGlobalDiagnostics(t.Context()); len(ds) != 0 {
		t.Fatalf("complete official global diagnostics: %+v", ds)
	}
}

func nullablePresenceFirstOfficialMask(t *testing.T, typ, payload *checker.Type, mask int, label string) {
	t.Helper()
	if typ == nil || payload == nil {
		t.Fatalf("official type absent at %s", label)
	}
	if mask == 0 {
		if typ != payload {
			t.Fatalf("official scalar payload object changed at %s", label)
		}
		return
	}
	if !typ.IsUnion() {
		t.Fatalf("official declared/use-site union absent at %s mask=%d", label, mask)
	}
	p, u, n := 0, 0, 0
	for _, member := range typ.Types() {
		switch {
		case member == payload:
			p++
		case member.Flags()&checker.TypeFlagsUndefined != 0:
			u++
		case member.Flags()&checker.TypeFlagsNull != 0:
			n++
		default:
			t.Fatalf("foreign official member at %s", label)
		}
	}
	wantU, wantN := 0, 0
	if mask&1 != 0 {
		wantU = 1
	}
	if mask&2 != 0 {
		wantN = 1
	}
	if p != 1 || u != wantU || n != wantN || len(typ.Types()) != 1+wantU+wantN {
		t.Fatalf("official members differ at %s: payload=%d undefined=%d null=%d mask=%d", label, p, u, n, mask)
	}
}

// The official scanner observes actual token starts only. Expected numeric intervals/text
// remain the frozen independent table; no product offset/scalar/finitePresence helper is used.
func nullablePresenceFirstObserve(t *testing.T, file *ast.SourceFile, source string, table map[string]nullablePresenceFirstAnchor, seen map[string]bool, label string, node *ast.Node, token bool) {
	t.Helper()
	want, exists := table[label]
	if !exists || seen[label] || node == nil || ast.GetSourceFileOfNode(node) != file {
		t.Fatalf("missing, duplicated or foreign official anchor %s", label)
	}
	start := node.Pos()
	if token {
		start = scanner.SkipTrivia(file.Text(), start)
	}
	positions := file.GetPositionMap()
	actual := [2]int{positions.UTF8ToUTF16(start), positions.UTF8ToUTF16(node.End())}
	if actual != want.Range || nullablePresenceFirstSourceSlice(t, source, actual) != want.Text {
		t.Fatalf("complete official UTF16 anchor %s differs: actual=%+v want=%+v text=%q", label, actual, want.Range, want.Text)
	}
	seen[label] = true
}

func nullablePresenceFirstOfficial(t *testing.T, program *compiler.Program, f nullablePresenceFirstFixture) int {
	t.Helper()
	file := program.GetSourceFile(f.r.RootFile)
	if file == nil || len(file.Statements.Nodes) != 2 {
		t.Fatal("fresh official Program lost original two-function source")
	}
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	chooseNode, entryNode := file.Statements.Nodes[0], file.Statements.Nodes[1]
	if chooseNode.Kind != ast.KindFunctionDeclaration || entryNode.Kind != ast.KindFunctionDeclaration || chooseNode.Name() == nil || entryNode.Name() == nil || chooseNode.Name().Text() != "choose" || entryNode.Name().Text() != "entry" {
		t.Fatal("official declaration kind/order/name changed")
	}
	choose, entry := chooseNode.AsFunctionDeclaration(), entryNode.AsFunctionDeclaration()
	if choose.TypeParameters != nil || entry.TypeParameters != nil || choose.Type == nil || entry.Type == nil || choose.Body == nil || entry.Body == nil || choose.Body.Kind != ast.KindBlock || entry.Body.Kind != ast.KindBlock || len(choose.Parameters.Nodes) != 2 || len(entry.Parameters.Nodes) != 1 {
		t.Fatal("official ordinary required function shape changed")
	}
	parameters := []*ast.Node{choose.Parameters.Nodes[0], choose.Parameters.Nodes[1], entry.Parameters.Nodes[0]}
	for _, node := range parameters {
		p := node.AsParameterDeclaration()
		if node.Kind != ast.KindParameter || node.Name() == nil || node.Name().Kind != ast.KindIdentifier || p.Type == nil || p.QuestionToken != nil || p.Initializer != nil || p.DotDotDotToken != nil {
			t.Fatal("official parameter ceased to be required explicitly typed identifier")
		}
	}
	if parameters[0].Name().Text() != "value" || parameters[1].Name().Text() != "fallback" || parameters[2].Name().Text() != "value" {
		t.Fatal("original parameter names changed")
	}
	chooseBinding, entryBinding := check.GetSymbolAtLocation(chooseNode.Name()), check.GetSymbolAtLocation(entryNode.Name())
	valueBinding, fallbackBinding, entryValueBinding := check.GetSymbolAtLocation(parameters[0].Name()), check.GetSymbolAtLocation(parameters[1].Name()), check.GetSymbolAtLocation(parameters[2].Name())
	unique := map[*ast.Symbol]bool{}
	for _, binding := range []*ast.Symbol{chooseBinding, entryBinding, valueBinding, fallbackBinding, entryValueBinding} {
		if binding == nil || unique[binding] {
			t.Fatal("five original defining Symbol identities absent or merged")
		}
		unique[binding] = true
	}
	payload := check.GetTypeFromTypeNode(parameters[1].AsParameterDeclaration().Type)
	if payload == nil || payload.Flags()&(checker.TypeFlagsUndefined|checker.TypeFlagsNull) != 0 {
		t.Fatal("official nominal i32 payload missing")
	}
	masks := []int{f.ranges.Mask, 0, 0}
	for i, node := range parameters {
		nullablePresenceFirstOfficialMask(t, check.GetTypeFromTypeNode(node.AsParameterDeclaration().Type), payload, masks[i], fmt.Sprintf("parameter%d annotation", i))
		nullablePresenceFirstOfficialMask(t, check.GetTypeOfSymbol(check.GetSymbolAtLocation(node.Name())), payload, masks[i], fmt.Sprintf("parameter%d symbol", i))
	}
	for i, node := range []*ast.Node{chooseNode, entryNode} {
		sigs := check.GetSignaturesOfType(check.GetTypeOfSymbol(check.GetSymbolAtLocation(node.Name())), checker.SignatureKindCall)
		count := 2 - i
		if len(sigs) != 1 || sigs[0].Declaration() != node || len(sigs[0].Parameters()) != count {
			t.Fatal("official complete source signature changed")
		}
		nullablePresenceFirstOfficialMask(t, check.GetTypeFromTypeNode(node.AsFunctionDeclaration().Type), payload, 0, "return annotation")
		nullablePresenceFirstOfficialMask(t, check.GetReturnTypeOfSignature(sigs[0]), payload, 0, "signature return")
		for j, parameter := range sigs[0].Parameters() {
			original := node.AsFunctionDeclaration().Parameters.Nodes[j]
			if parameter != check.GetSymbolAtLocation(original.Name()) {
				t.Fatal("signature parameter no longer names its original defining symbol")
			}
			mask := 0
			if i == 0 && j == 0 {
				mask = f.ranges.Mask
			}
			nullablePresenceFirstOfficialMask(t, check.GetTypeOfSymbol(parameter), payload, mask, "signature parameter")
		}
	}
	chooseBody, entryBody := choose.Body.AsBlock().Statements.Nodes, entry.Body.AsBlock().Statements.Nodes
	if len(chooseBody) != len(f.ranges.Guards)+1 || len(entryBody) != 1 || chooseBody[len(chooseBody)-1].Kind != ast.KindReturnStatement || entryBody[0].Kind != ast.KindReturnStatement {
		t.Fatal("original guard/early-return statement sequence changed")
	}
	tail, entryReturn := chooseBody[len(chooseBody)-1], entryBody[0]
	tailRef, callNode := tail.AsReturnStatement().Expression, entryReturn.AsReturnStatement().Expression
	if tailRef == nil || tailRef.Kind != ast.KindIdentifier || check.GetSymbolAtLocation(tailRef) != valueBinding || callNode == nil || callNode.Kind != ast.KindCallExpression {
		t.Fatal("final same-symbol narrowed reference or entry call lost")
	}
	nullablePresenceFirstOfficialMask(t, check.GetTypeAtLocation(tailRef), payload, 0, "tail reference")
	call := callNode.AsCallExpression()
	if call.Expression.Kind != ast.KindIdentifier || call.TypeArguments != nil || call.QuestionDotToken != nil || len(call.Arguments.Nodes) != 2 || check.GetSymbolAtLocation(call.Expression) != chooseBinding {
		t.Fatal("entry call lost original ordered callee/arguments")
	}
	resolved := check.GetResolvedSignature(callNode)
	if resolved == nil || resolved.Declaration() != chooseNode || len(resolved.Parameters()) != 2 {
		t.Fatal("actual call did not resolve original choose signature")
	}
	nullablePresenceFirstOfficialMask(t, check.GetReturnTypeOfSignature(resolved), payload, 0, "resolved call return")
	nullablePresenceFirstOfficialMask(t, check.GetTypeAtLocation(callNode), payload, 0, "call use-site")
	for j, parameter := range resolved.Parameters() {
		if parameter != check.GetSymbolAtLocation(choose.Parameters.Nodes[j].Name()) {
			t.Fatal("resolved signature parameter identity changed")
		}
		mask := 0
		if j == 0 {
			mask = f.ranges.Mask
		}
		nullablePresenceFirstOfficialMask(t, check.GetTypeOfSymbol(parameter), payload, mask, "resolved parameter")
	}
	for _, argument := range call.Arguments.Nodes {
		if argument.Kind != ast.KindIdentifier || check.GetSymbolAtLocation(argument) != entryValueBinding {
			t.Fatal("call arguments no longer reference original scalar caller binding")
		}
		nullablePresenceFirstOfficialMask(t, check.GetTypeAtLocation(argument), payload, 0, "call argument")
	}
	if !ast.IsExternalModule(file) || file.AsNode().Symbol() == nil {
		t.Fatal("official source has no real external module symbol")
	}
	exports := check.GetExportsOfModule(file.AsNode().Symbol())
	if len(exports) != 1 || exports[0] == nil || exports[0].Name != "entry" || exports[0].Flags&ast.SymbolFlagsAlias != 0 || len(exports[0].Declarations) != 1 || exports[0].Declarations[0] != entryNode || ast.GetSourceFileOfNode(exports[0].Declarations[0]) != file || check.GetSymbolAtLocation(exports[0].Declarations[0].Name()) != entryBinding {
		t.Fatal("complete official exports differ from the original sole scalar entry")
	}

	seen := map[string]bool{}
	note := func(raw, token string, node *ast.Node) {
		nullablePresenceFirstObserve(t, file, f.r.Files[0].Content, f.ranges.Anchors, seen, raw, node, false)
		if token != "" {
			nullablePresenceFirstObserve(t, file, f.r.Files[0].Content, f.ranges.Anchors, seen, token, node, true)
		}
	}
	note("chooseRange", "chooseToken", chooseNode)
	note("chooseDefinition", "chooseDefinitionToken", chooseNode.Name())
	note("valueDefinition", "", parameters[0].Name())
	note("valueParameter", "", parameters[0])
	note("valueAnnotationRaw", "valueAnnotation", parameters[0].AsParameterDeclaration().Type)
	note("fallbackDefinition", "fallbackDefinitionToken", parameters[1].Name())
	note("fallbackParameter", "", parameters[1])
	note("fallbackAnnotationRaw", "fallbackAnnotation", parameters[1].AsParameterDeclaration().Type)
	note("chooseReturnAnnotationRaw", "chooseReturnAnnotation", choose.Type)
	note("chooseBody", "chooseBodyToken", choose.Body)
	note("tailReturn", "tailReturnToken", tail)
	note("tailReference", "tailReferenceToken", tailRef)
	note("entryRange", "entryToken", entryNode)
	note("entryDefinition", "entryDefinitionToken", entryNode.Name())
	note("entryValueDefinition", "", parameters[2].Name())
	note("entryValueParameter", "", parameters[2])
	note("entryValueAnnotationRaw", "entryValueAnnotation", parameters[2].AsParameterDeclaration().Type)
	note("entryReturnAnnotationRaw", "entryReturnAnnotation", entry.Type)
	note("entryBody", "entryBodyToken", entry.Body)
	note("entryReturn", "entryReturnToken", entryReturn)
	note("entryCall", "entryCallToken", callNode)
	note("entryCallee", "entryCalleeToken", call.Expression)
	note("entryArgument1", "entryArgument1Token", call.Arguments.Nodes[0])
	note("entryArgument2", "entryArgument2Token", call.Arguments.Nodes[1])
	if len(seen) != len(f.ranges.Anchors) {
		t.Fatal("official oracle omitted an independent common range")
	}
	count := len(seen)
	for i, g := range f.ranges.Guards {
		if g.UseMask != f.ranges.GuardMasks[i] || len(g.Anchors) != 14 || chooseBody[i].Kind != ast.KindIfStatement {
			t.Fatal("original guard order/use-site constants changed")
		}
		guardNode := chooseBody[i]
		guard := guardNode.AsIfStatement()
		if guard.ElseStatement != nil || guard.Expression == nil || guard.Expression.Kind != ast.KindBinaryExpression || guard.ThenStatement == nil || guard.ThenStatement.Kind != ast.KindBlock {
			t.Fatal("official guard gained else/group or lost its block")
		}
		binary := guard.Expression.AsBinaryExpression()
		if binary.OperatorToken.Kind != ast.KindEqualsEqualsEqualsToken || binary.Left.Kind != ast.KindIdentifier || check.GetSymbolAtLocation(binary.Left) != valueBinding {
			t.Fatal("official guard changed operator or original left parameter symbol")
		}
		nullablePresenceFirstOfficialMask(t, check.GetTypeAtLocation(binary.Left), payload, g.UseMask, "guard left use-site")
		if check.GetTypeAtLocation(guard.Expression) != check.GetBooleanType() {
			t.Fatal("actual presence comparison result is not official boolean")
		}
		rightType := check.GetTypeAtLocation(binary.Right)
		if rightType == nil {
			t.Fatal("absence operand has no official type")
		}
		if g.Absence == "undefined" {
			binding := check.GetSymbolAtLocation(binary.Right)
			if binary.Right.Kind != ast.KindIdentifier || binding == nil || binding != check.GetUndefinedSymbol() || len(binding.Declarations) != 0 || rightType.Flags()&checker.TypeFlagsUndefined == 0 {
				t.Fatal("undefined is not the real official unshadowed builtin symbol/type")
			}
		} else if g.Absence != "null" || binary.Right.Kind != ast.KindNullKeyword || rightType.Flags()&checker.TypeFlagsNull == 0 {
			t.Fatal("null guard lost its original official keyword/type")
		}
		then := guard.ThenStatement.AsBlock().Statements.Nodes
		if len(then) != 1 || then[0].Kind != ast.KindReturnStatement {
			t.Fatal("then block must retain one original early return")
		}
		fallback := then[0].AsReturnStatement().Expression
		if fallback == nil || fallback.Kind != ast.KindIdentifier || check.GetSymbolAtLocation(fallback) != fallbackBinding {
			t.Fatal("early return no longer uses distinct scalar fallback parameter")
		}
		nullablePresenceFirstOfficialMask(t, check.GetTypeAtLocation(fallback), payload, 0, "fallback use-site")
		guardSeen := map[string]bool{}
		observe := func(raw, token string, node *ast.Node) {
			nullablePresenceFirstObserve(t, file, f.r.Files[0].Content, g.Anchors, guardSeen, raw, node, false)
			if token != "" {
				nullablePresenceFirstObserve(t, file, f.r.Files[0].Content, g.Anchors, guardSeen, token, node, true)
			}
		}
		observe("ifRaw", "ifRange", guardNode)
		observe("condition", "", guard.Expression)
		observe("left", "", binary.Left)
		observe("operatorRaw", "operator", binary.OperatorToken)
		observe("rightRaw", "right", binary.Right)
		observe("block", "blockToken", guard.ThenStatement)
		observe("returned", "returnToken", then[0])
		observe("reference", "referenceToken", fallback)
		if len(guardSeen) != len(g.Anchors) {
			t.Fatal("official oracle omitted a complete guard range")
		}
		count += len(guardSeen)
	}
	t.Logf("TSFG_20K_FIRST_OFFICIAL %s utf16Ranges=%d declaredMask=%d guardMasks=%v tailMask=0 symbols=5 exports=entry", f.name, count, f.ranges.Mask, f.ranges.GuardMasks)
	return count
}
