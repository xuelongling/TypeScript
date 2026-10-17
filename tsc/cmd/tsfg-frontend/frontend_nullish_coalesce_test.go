// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

func TestFrontendCommandNullishCoalesceOwnedFacts(t *testing.T) {
	const source = "export function coalesce(value: i32 | null | undefined, fallback: i32): i32 { return value ?? fallback; }"
	sum := sha256.Sum256([]byte(source))
	if len(source) != 105 || len(utf16.Encode([]rune(source))) != 105 || hex.EncodeToString(sum[:]) != "55c38b3ae6da9a7dd05ce2faf7bf10e7af114459b4ad7a2eaceed3505084cc2c" {
		t.Fatal("frozen original source literal changed")
	}
	r := integerRequest(source)
	original, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		after, err := json.Marshal(r)
		if err != nil || !bytes.Equal(after, original) {
			t.Fatal("public command or oracle mutated the original request")
		}
	}()
	if !t.Run("officialDiagnostics", func(t *testing.T) { nullableBoundaryOfficialForTest(t, r, 0) }) {
		return
	}

	anchors := map[string]span{}
	if !t.Run("officialOracle", func(t *testing.T) {
		// A separate fresh Program obtains original AST/symbol/type objects.
		// Do not borrow finitePresence, scalar, offset or owned projection.
		contents := map[string]string{}
		for _, file := range r.Files {
			contents[file.Path] = file.Content
		}
		program := compiler.NewProgram(compiler.ProgramOptions{
			Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
				FileNames: append([]string{r.RootFile}, r.Declarations.Paths...),
				CompilerOptions: &core.CompilerOptions{
					NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext,
				},
			}},
			Host:           compiler.NewCompilerHost(r.SourceRoot, vfstest.FromMap(contents, true), r.SourceRoot, nil, nil),
			SingleThreaded: core.TSTrue,
		})
		file := program.GetSourceFile(r.RootFile)
		if file == nil || len(file.Statements.Nodes) != 1 || file.Statements.Nodes[0].Kind != ast.KindFunctionDeclaration {
			t.Fatal("official AST lost the one original function")
		}
		check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
		defer release()
		fnNode := file.Statements.Nodes[0]
		decl := fnNode.AsFunctionDeclaration()
		if len(decl.Parameters.Nodes) != 2 || decl.Body == nil || decl.Body.Kind != ast.KindBlock {
			t.Fatal("official declaration lost required parameters or original body")
		}
		body := decl.Body.AsBlock().Statements.Nodes
		if len(body) != 1 || body[0].Kind != ast.KindReturnStatement {
			t.Fatal("official body is not the single original return")
		}
		whole := body[0].AsReturnStatement().Expression
		if whole == nil || whole.Kind != ast.KindBinaryExpression {
			t.Fatal("official return lost the original binary AST")
		}
		binary := whole.AsBinaryExpression()
		if binary.OperatorToken.Kind != ast.KindQuestionQuestionToken || binary.Left.Kind != ast.KindIdentifier || binary.Right.Kind != ast.KindIdentifier || binary.Left == binary.Right {
			t.Fatal("official AST must retain one distinct identifier on each side of ??")
		}
		leftBinding := check.GetSymbolAtLocation(decl.Parameters.Nodes[0].Name())
		rightBinding := check.GetSymbolAtLocation(decl.Parameters.Nodes[1].Name())
		fnBinding := check.GetSymbolAtLocation(decl.Name())
		if leftBinding == nil || rightBinding == nil || fnBinding == nil || leftBinding == rightBinding || leftBinding == fnBinding || rightBinding == fnBinding || check.GetSymbolAtLocation(binary.Left) != leftBinding || check.GetSymbolAtLocation(binary.Right) != rightBinding {
			t.Fatal("official operand bindings differ from the two original parameters")
		}
		leftType := check.GetTypeAtLocation(binary.Left)
		rightType := check.GetTypeAtLocation(binary.Right)
		resultType := check.GetTypeAtLocation(whole)
		annotationType := check.GetTypeFromTypeNode(decl.Parameters.Nodes[1].AsParameterDeclaration().Type)
		if leftType == nil || rightType == nil || resultType == nil || !leftType.IsUnion() || rightType != annotationType || resultType != rightType || rightType.Flags()&(checker.TypeFlagsNull|checker.TypeFlagsUndefined) != 0 {
			t.Fatal("official ?? types do not retain the same declared scalar i32 payload/result")
		}
		payload, null, undefined := 0, 0, 0
		for _, member := range leftType.Types() {
			switch {
			case member.Flags()&checker.TypeFlagsNull != 0:
				null++
			case member.Flags()&checker.TypeFlagsUndefined != 0:
				undefined++
			case member == rightType:
				payload++
			default:
				t.Fatal("official lhs union contains a foreign payload")
			}
		}
		if payload != 1 || null != 1 || undefined != 1 || len(leftType.Types()) != 3 {
			t.Fatal("official lhs must contain exactly the i32, null and undefined members")
		}
		positions := file.GetPositionMap()
		for label, node := range map[string]*ast.Node{
			"function": fnNode, "definition": decl.Name(), "leftParameter": decl.Parameters.Nodes[0].Name(),
			"rightParameter": decl.Parameters.Nodes[1].Name(), "return": body[0],
		} {
			anchors[label] = span{Start: positions.UTF8ToUTF16(node.Pos()), End: positions.UTF8ToUTF16(node.End())}
		}
		for label, node := range map[string]*ast.Node{"whole": whole, "left": binary.Left, "right": binary.Right, "operator": binary.OperatorToken} {
			start := node.Pos()
			for start < node.End() && source[start] == ' ' {
				start++
			}
			anchors[label] = span{Start: positions.UTF8ToUTF16(start), End: positions.UTF8ToUTF16(node.End())}
		}
		want := map[string]span{
			"function": {0, 105}, "definition": {15, 24}, "leftParameter": {25, 30}, "rightParameter": {55, 64},
			"return": {77, 103}, "whole": {85, 102}, "left": {85, 90}, "right": {94, 102}, "operator": {91, 93},
		}
		if !reflect.DeepEqual(anchors, want) {
			t.Fatalf("official original UTF16 anchors changed: got=%+v want=%+v", anchors, want)
		}
		oracle, err := json.Marshal(map[string]any{
			"kind": "test-only-official-nullish-oracle", "sourcePath": r.RootFile, "anchors": anchors,
			"leftMembers": []string{"i32", "null", "undefined"}, "right": "same-declared-i32", "result": "same-declared-i32",
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("TSFG_NULLISH_COALESCE_OFFICIAL_ORACLE_V1 %s", base64.StdEncoding.EncodeToString(oracle))
	}) {
		return
	}

	t.Run("ownedProjection", func(t *testing.T) {
		t.Logf("TSFG_NULLISH_COALESCE_REQUEST_V1 %s", base64.StdEncoding.EncodeToString(original))
		result, raw := nullableBoundaryCommandForTest(t, r)
		if result.Status != "ok" || len(result.Diagnostics) != 0 || len(result.Functions) != 1 || len(result.Symbols) != 3 {
			t.Fatalf("official ?? oracle passed, but public owned projection rejected or omitted facts: %+v; rawResponse=%s", result, raw)
		}
		fn := result.Functions[0]
		if fn.Name != "coalesce" || fn.Symbol != "s1" || fn.ReturnType != "i32" || fn.ReturnPresence != nil || fn.Range != anchors["function"] || len(fn.Parameters) != 2 || len(fn.Body) != 1 || len(fn.ParameterInitializers) != 2 || fn.ParameterInitializers[0] != nil || fn.ParameterInitializers[1] != nil {
			t.Fatalf("original function signature/body/identity changed: %+v", fn)
		}
		definitions := map[string]symbol{}
		for index, definition := range result.Symbols {
			if definition.ID == "" || definitions[definition.ID].ID != "" {
				t.Fatal("owned symbols are missing or duplicated")
			}
			if index == 0 && (definition.ID != fn.Symbol || definition.Kind != "function" || definition.Name != fn.Name || definition.Range != anchors["definition"] || definition.Type != "i32" || definition.Writable || definition.Presence != nil || definition.ReturnPresence != nil) {
				t.Fatalf("function defining symbol differs from official AST: %+v", definition)
			}
			definitions[definition.ID] = definition
		}
		for index, expected := range []struct {
			id, name                string
			definition              span
			declaration, annotation span
			absent                  bool
		}{
			{"s2", "value", anchors["leftParameter"], span{25, 54}, span{32, 54}, true},
			{"s3", "fallback", anchors["rightParameter"], span{55, 69}, span{66, 69}, false},
		} {
			parameter := fn.Parameters[index]
			shape := parameter.Presence
			if parameter.ID != expected.id || parameter.Name != expected.name || parameter.Kind != "parameter" || parameter.Type != "i32" || !parameter.Writable || parameter.Range != expected.definition || !reflect.DeepEqual(definitions[parameter.ID], parameter) || shape == nil || shape.Null != expected.absent || shape.Undefined != expected.absent || shape.Optional || shape.Default || shape.Range != expected.declaration || shape.Annotation != expected.annotation {
				t.Fatalf("parameter declaration/binding/presence changed: %+v", parameter)
			}
		}
		returned := fn.Body[0]
		value := returned.Expression
		if returned.Kind != "return" || returned.Range != anchors["return"] || value == nil || value.Kind != "shortCircuit" || value.Operator != "??" || value.Type != "i32" || value.Range != anchors["whole"] || value.Left == nil || value.Right == nil || value.Left == value.Right || value.PresenceFlow == nil || value.PresenceFlow.Null || value.PresenceFlow.Undefined {
			t.Fatalf("?? lost scalar result, laziness structure or original range: %+v", returned)
		}
		for index, operand := range []*expression{value.Left, value.Right} {
			where := anchors["left"]
			if index == 1 {
				where = anchors["right"]
			}
			if operand.Kind != "reference" || operand.Type != "i32" || operand.Symbol != fn.Parameters[index].ID || operand.Range != where || operand.Left != nil || operand.Right != nil || operand.Operand != nil || operand.Callee != nil || operand.Receiver != nil || operand.Arguments != nil || operand.Target != "" || operand.Operator != "" {
				t.Fatalf("operand %d was duplicated, rewritten or rebound: %+v", index, operand)
			}
		}
		if value.Left.PresenceFlow == nil || !value.Left.PresenceFlow.Null || !value.Left.PresenceFlow.Undefined || value.Right.PresenceFlow != nil {
			t.Fatal("official nullable lhs or scalar fallback flow changed")
		}
		record := nullableBoundaryRecordForTest(t, raw, "functions", "name", fn.Name)
		returns := nullableBoundaryReturnsForTest(t, record["body"])
		if len(returns) != 1 || len(returns[0]) != 7 {
			t.Fatal("raw ?? tree must have only kind/type/operator/range/left/right/presenceFlow")
		}
		var flow map[string]json.RawMessage
		if err := json.Unmarshal(returns[0]["presenceFlow"], &flow); err != nil || len(flow) != 2 {
			t.Fatalf("scalar result omitted exact official flow flags: %v", err)
		}
		nullableBoundaryFlagsForTest(t, flow, false, false)
		for key, count := range map[string]int{"left": 5, "right": 4} {
			var leaf map[string]json.RawMessage
			if err := json.Unmarshal(returns[0][key], &leaf); err != nil || len(leaf) != count {
				t.Fatalf("%s is not one original reference-only operand: %v", key, err)
			}
		}
		t.Logf("TSFG_NULLISH_COALESCE_RESPONSE_V1 %s", base64.StdEncoding.EncodeToString(raw))
	})
}

// These are in-process public facts, not execution or side-effect-laziness
// evidence. Each valid-TS negative first passes all four official diagnostics.
func TestFrontendCommandNullishCoalesceBoundaries(t *testing.T) {
	acquire := func(t *testing.T, source string) (response, []byte) {
		t.Helper()
		r := integerRequest(source)
		before, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			after, err := json.Marshal(r)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("public nullish boundary changed its original request")
			}
		}()
		if !t.Run("officialDiagnostics", func(t *testing.T) { nullableBoundaryOfficialForTest(t, r, 0) }) {
			t.Fatal("official diagnostics failed; no owned boundary credit")
		}
		return nullableBoundaryCommandForTest(t, r)
	}
	positive := []struct {
		name, source, expression string
		undefined, null, call    bool
	}{
		{"nullOnly", "export function coalesce(value: i32 | null, fallback: i32): i32 { return value ?? fallback; }", "value ?? fallback", false, true, false},
		{"undefinedOnly", "export function coalesce(value: i32 | undefined, fallback: i32): i32 { return value ?? fallback; }", "value ?? fallback", true, false, false},
		{"scalarCallFallback", "function fallback(value: i32): i32 { return value; }\nexport function coalesce(value: i32 | null, other: i32): i32 { return value ?? fallback(other); }", "value ?? fallback(other)", false, true, true},
	}
	for _, tc := range positive {
		// Every positive is a prerequisite; missing support cannot make all
		// negative cases look green through a shared early capability failure.
		if !t.Run(tc.name, func(t *testing.T) {
			result, raw := acquire(t, tc.source)
			wantFunctions, wantSymbols := 1, 3
			if tc.call {
				wantFunctions, wantSymbols = 2, 5
			}
			if result.Status != "ok" || len(result.Diagnostics) != 0 || len(result.Functions) != wantFunctions || len(result.Symbols) != wantSymbols {
				t.Fatalf("positive public nullish facts rejected or incomplete: %+v; raw=%s", result, raw)
			}
			var fn, fallback function
			definitions := map[string]symbol{}
			for _, owned := range result.Symbols {
				if owned.ID == "" || definitions[owned.ID].ID != "" {
					t.Fatal("missing or duplicate original binding identity")
				}
				definitions[owned.ID] = owned
			}
			for _, candidate := range result.Functions {
				if candidate.Name == "coalesce" {
					fn = candidate
				} else if candidate.Name == "fallback" {
					fallback = candidate
				}
			}
			if fn.Symbol == "" || fn.ReturnType != "i32" || fn.ReturnPresence != nil || len(fn.Parameters) != 2 || len(fn.Body) != 1 || fn.Body[0].Kind != "return" || fn.Parameters[0].ID == fn.Parameters[1].ID {
				t.Fatalf("positive lost its original scalar-return declaration: %+v", fn)
			}
			for _, p := range fn.Parameters {
				if p.Type != "i32" || p.Kind != "parameter" || !p.Writable || !reflect.DeepEqual(definitions[p.ID], p) || p.Presence == nil || p.Presence.Optional || p.Presence.Default {
					t.Fatalf("required parameter or owned defining symbol changed: %+v", p)
				}
			}
			leftShape, rightShape := fn.Parameters[0].Presence, fn.Parameters[1].Presence
			if leftShape.Undefined != tc.undefined || leftShape.Null != tc.null || rightShape.Undefined || rightShape.Null {
				t.Fatal("declared absence leaked between lhs and scalar fallback")
			}
			value := fn.Body[0].Expression
			start := bytes.Index([]byte(tc.source), []byte(tc.expression))
			if start < 0 || value == nil || value.Kind != "shortCircuit" || value.Operator != "??" || value.Type != "i32" || value.Range != (span{start, start + len(tc.expression)}) || value.Left == nil || value.Right == nil || value.PresenceFlow == nil || value.PresenceFlow.Null || value.PresenceFlow.Undefined {
				t.Fatalf("positive lost one scalar shortCircuit and its original expression range: %+v", value)
			}
			left := value.Left
			if left.Kind != "reference" || left.Type != "i32" || left.Symbol != fn.Parameters[0].ID || left.Range != (span{start, start + 5}) || left.PresenceFlow == nil || left.PresenceFlow.Undefined != tc.undefined || left.PresenceFlow.Null != tc.null {
				t.Fatalf("lhs lost original binding, range or official absence: %+v", left)
			}
			right := value.Right
			if right.Type != "i32" || right.PresenceFlow != nil {
				t.Fatalf("scalar fallback acquired absence: %+v", right)
			}
			if tc.call {
				if fallback.Symbol == "" || fallback.Symbol == fn.Symbol || len(fallback.Parameters) != 1 || right.Kind != "call" || right.Target != fallback.Symbol || right.Signature == nil || right.Signature.ReturnType != "i32" || !reflect.DeepEqual(right.Signature.Parameters, []string{"i32"}) || right.Signature.ReturnPresence != nil || right.Arguments == nil || len(*right.Arguments) != 1 || right.Range != (span{start + 8, start + len(tc.expression)}) {
					t.Fatalf("fallback call was hoisted, replaced or lost its exact target/signature/raw range: %+v", right)
				}
				argument := (*right.Arguments)[0]
				if argument.Kind != "reference" || argument.Type != "i32" || argument.Symbol != fn.Parameters[1].ID || argument.Symbol == fallback.Parameters[0].ID || argument.Range != (span{start + 18, start + 23}) || argument.PresenceFlow != nil {
					t.Fatalf("call argument lost caller binding/order or original range: %+v", argument)
				}
			} else if right.Kind != "reference" || right.Symbol != fn.Parameters[1].ID || right.Range != (span{start + 9, start + len(tc.expression)}) {
				t.Fatalf("fallback reference lost its original binding/range: %+v", right)
			}
			counts := map[string]int{}
			var visit func(*expression)
			visit = func(e *expression) {
				if e == nil {
					return
				}
				counts[e.Kind]++
				visit(e.Left)
				visit(e.Right)
				visit(e.Operand)
				visit(e.Callee)
				visit(e.Receiver)
				if e.Arguments != nil {
					for index := range *e.Arguments {
						visit(&(*e.Arguments)[index])
					}
				}
			}
			visit(value)
			want := map[string]int{"shortCircuit": 1, "reference": 2}
			if tc.call {
				want["call"] = 1
			}
			if !reflect.DeepEqual(counts, want) {
				t.Fatalf("operands were duplicated, eagerly rewritten or made writable: got=%v want=%v", counts, want)
			}
		}) {
			return
		}
	}
	for _, tc := range []struct{ name, source string }{
		{"f64Payload", "export function reject(value: f64 | null, fallback: f64): f64 { return value ?? fallback; }"},
		{"nonnullableLhs", "export function reject(value: i32, fallback: i32): i32 { return value ?? fallback; }"},
		{"nullFallbackAndResult", "export function reject(value: i32 | null, fallback: i32 | null): i32 | null { return value ?? fallback; }"},
		{"undefinedFallbackAndResult", "export function reject(value: i32 | undefined, fallback: i32 | undefined): i32 | undefined { return value ?? fallback; }"},
		{"bothAbsencesFallbackAndResult", "export function reject(value: i32 | null | undefined, fallback: i32 | null | undefined): i32 | null | undefined { return value ?? fallback; }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, raw := acquire(t, tc.source)
			if result.Status != "error" || len(result.Functions) != 0 || len(result.Diagnostics) != 1 {
				t.Fatalf("valid TS outside the finite ?? slice was admitted or lost its rejection: %+v; raw=%s", result, raw)
			}
			d := result.Diagnostics[0]
			if d.Code != "r02b-d-unsupported" || d.Category != "unsupported" || d.Phase != "capability" || d.Severity != "error" || d.UpstreamCode != 0 || d.File != "/src/main.ts" || d.Primary.SourceID != d.File || d.Primary.Start != d.Range.Start || d.Primary.End != d.Range.End || sourceText(t, tc.source, d.Range) != "value ?? fallback" {
				t.Fatalf("rejection did not reach the original whole ?? capability gate: %+v; raw=%s", d, raw)
			}
		})
	}
}
