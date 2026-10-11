// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"testing"

	"github.com/microsoft/typescript-go/internal/json"
)

func aggregateRequest(t *testing.T, source string) request {
	t.Helper()
	r := conversionRequest(t, source)
	r.Files = append(r.Files, inputFile{Path: "/src/minimal-aggregates-v1.d.ts", Content: markerDeclarations})
	r.Declarations.Paths = append(r.Declarations.Paths, "/src/minimal-aggregates-v1.d.ts")
	r.Declarations.MarkerProfile = &markerProfile{Version: markerVersion, Path: "/src/minimal-aggregates-v1.d.ts", SHA256: markerHash(), ContextName: "ClassDecoratorContext", ContextArity: 1, Mode: "standard", Entries: []markerEntry{{"tsfgValue", "value"}, {"tsfgComponent", "component"}}}
	r.Config.DecoratorMode = "standard"
	return r
}

func TestAggregateScalarOperandSpansKeepTokenStartsAndContainment(t *testing.T) {
	t.Parallel()
	const observed = "@tsfgValue class Pair { n: i64 = 7; }\nexport function answer(): i64 { const pair = new Pair(); const count: i64 = 1; return count + pair.n; }"
	original := analyze(aggregateRequest(t, observed))
	if original.Status != "ok" {
		t.Fatalf("original observed scalar-left source rejected: %+v", original.Diagnostics)
	}
	observedValue := original.Functions[0].Body[2].Expression
	if observedValue.Range != (span{124, 138}) || observedValue.Left.Range != (span{124, 129}) || observedValue.Right.Range != (span{132, 138}) {
		t.Fatalf("original observed spans were not corrected from actual AST token starts: %+v", observedValue)
	}
	const header = "/*😀*/ @tsfgValue class Pair { n: i64 = 7; }\n"
	const ordinary = "export function ordinary(n: i64): i64 { return n; }\n"
	pure := analyze(conversionRequest(t, ordinary))
	if pure.Status != "ok" {
		t.Fatalf("ordinary scalar baseline rejected: %+v", pure.Diagnostics)
	}
	pureRangeText := sourceText(t, ordinary, pure.Functions[0].Body[0].Expression.Range)
	for _, item := range []struct{ name, operand, rangeText string }{
		{"observed", "count + pair.n", "count + pair.n"},
		{"leading-trivia", " \t /* lead */ count + pair.n", "count + pair.n"},
		{"parentheses", "( count ) + pair.n", "( count ) + pair.n"},
		{"multiple-scalar-subexpressions", "count + count + pair.n", "count + count + pair.n"},
		{"nested-groups", "((count + count)) + (pair.n + count)", "((count + count)) + (pair.n + count)"},
		{"scalar-call-operand", "ordinary( count + count ) + pair.n", "ordinary( count + count ) + pair.n"},
	} {
		t.Run(item.name, func(t *testing.T) {
			source := header + ordinary + "export function answer(): i64 { const pair = new Pair(); const count: i64 = 1; return " + item.operand + "; }"
			result := analyze(aggregateRequest(t, source))
			if result.Status != "ok" || len(result.Functions) != 2 {
				t.Fatalf("typed aggregate scalar operand rejected: %+v", result)
			}
			if sourceText(t, source, result.Functions[0].Body[0].Expression.Range) != pureRangeText {
				t.Fatal("pure-scalar declaration projection changed under aggregate profile")
			}
			value := result.Functions[1].Body[2].Expression
			if value.Kind != "binary" || value.Type != "i64" || sourceText(t, source, value.Range) != item.rangeText {
				t.Fatalf("aggregate parent lost its actual lexical token span: %+v", value)
			}
			var check func(*expression)
			check = func(parent *expression) {
				if parent == nil {
					return
				}
				text := sourceText(t, source, parent.Range)
				if len(text) == 0 || text[0] == ' ' || text[0] == '\t' || text[0] == '\n' || text[0] == '\r' {
					t.Fatalf("aggregate-owned operand retained leading trivia: %+v", parent)
				}
				children := []*expression{parent.Left, parent.Right, parent.Operand, parent.Receiver}
				if parent.Arguments != nil {
					for index := range *parent.Arguments {
						children = append(children, &(*parent.Arguments)[index])
					}
				}
				for _, child := range children {
					if child == nil {
						continue
					}
					if child.Range.Start < parent.Range.Start || child.Range.End > parent.Range.End {
						t.Fatalf("actual Source child span escapes owned parent: parent=%+v child=%+v", parent.Range, child.Range)
					}
					check(child)
				}
			}
			check(value)
			if value.Left.Kind == "reference" && sourceText(t, source, value.Left.Range) != "count" {
				t.Fatalf("observed ordinary scalar reference did not keep its exact token identity: %+v", value.Left)
			}
		})
	}
}

func TestAggregateScalarLeavesKeepEvaluationAndTypeRules(t *testing.T) {
	t.Parallel()
	const header = `@tsfgValue class Pair { n: i64 = 7; flag: bool = false; small: u8 = 1; fraction: f32 = 1.5; }
`
	for _, item := range []struct{ name, source, kind, operandType string }{
		{"literal-left", `export function answer(): i64 { const pair: Pair = new Pair(); return 1 + pair.n; }`, "binary", "i64"},
		{"unary-field", `export function answer(): i64 { const pair: Pair = new Pair(); return -pair.n; }`, "unary", "i64"},
		{"short-circuit", `export function answer(): bool { const pair: Pair = new Pair(); return pair.flag && (pair.flag = true); }`, "shortCircuit", "bool"},
		{"shift", `export function answer(): i64 { const pair: Pair = new Pair(); return pair.n << 1; }`, "binary", "i64"},
	} {
		t.Run(item.name, func(t *testing.T) {
			result := analyze(aggregateRequest(t, header+item.source))
			if result.Status != "ok" {
				t.Fatalf("scalar field rejected: %+v", result.Diagnostics)
			}
			value := result.Functions[0].Body[1].Expression
			if value.Kind != item.kind {
				t.Fatalf("evaluation mode lost: %+v", value)
			}
			operand := value.Left
			if operand == nil {
				operand = value.Operand
			}
			if operand.Type != item.operandType {
				t.Fatalf("nominal scalar context lost: %+v", value)
			}
			if item.name == "shift" && value.Right.Type != "i32" {
				t.Fatalf("shift count context lost: %+v", value)
			}
		})
	}
	for _, item := range []struct{ name, source, code string }{
		{"shift-count", `export function answer(): i64 { const pair: Pair = new Pair(); return pair.n << pair.small; }`, "r01-d-shift-count-type"},
		{"numeric-truthiness", `export function answer(): i64 { const pair: Pair = new Pair(); return pair.n || pair.n; }`, "r02b-d-unsupported"},
		{"unary-truthiness", `export function answer(): bool { const pair: Pair = new Pair(); return !pair.n; }`, "r02b-d-unsupported"},
		{"float-bitwise", `export function answer(): f32 { const pair: Pair = new Pair(); return pair.fraction | pair.fraction; }`, "r01-d-scalar-type"},
	} {
		t.Run(item.name, func(t *testing.T) {
			result := analyze(aggregateRequest(t, header+item.source))
			if result.Status == "ok" || len(result.Diagnostics) == 0 || result.Diagnostics[0].Code != item.code {
				t.Fatalf("invalid field operator admitted or relabeled: %+v", result)
			}
		})
	}
}

func TestAggregateWholeCoverageAndOfficialIdentityControls(t *testing.T) {
	t.Parallel()
	for _, item := range []struct{ name, source, code string }{
		{"unmarked-unused-class", `class Hidden { n: i64 = 7; } export function answer(): bool { return true; }`, "r01-d-aggregate-marker"},
		{"unused-method-body", `@tsfgValue class Pair { n: i64 = 7; unused(): bool { return true; } } export function answer(): bool { return true; }`, "r02b-d-unsupported"},
		{"static-field", `@tsfgValue class Pair { static n: i64 = 7; } export function answer(): bool { return true; }`, "r02b-d-unsupported"},
		{"constructor-alias", `@tsfgValue class Pair { n: i64 = 7; } export function answer(): Pair { const Alias = Pair; return new Alias(); }`, "r02b-d-unsupported"},
		{"generic-class", `@tsfgValue class Pair<T> { n: i64 = 7; } export function answer(): bool { return true; }`, "r02b-d-unsupported"},
		{"nominal-equality", `@tsfgValue class A { n: i64 = 7; } @tsfgValue class B { n: i64 = 7; } export function answer(): bool { return new A() === new B(); }`, "r01-d-aggregate-identity"},
	} {
		t.Run(item.name, func(t *testing.T) {
			result := analyze(aggregateRequest(t, item.source))
			if result.Status == "ok" || len(result.Diagnostics) == 0 || result.Diagnostics[0].Code != item.code {
				t.Fatalf("whole source/official identity guard lost: %+v", result)
			}
		})
	}
}

func TestAggregateConstructorReturnsKeepActualSelfAndControlFacts(t *testing.T) {
	t.Parallel()
	const source = `@tsfgValue class Pair { n!: i64; constructor(flag: bool) { if (flag) { this.n = 7; return; } this.n = 8; return this; } }
export function answer(): i64 { const pair = new Pair(false); return pair.n; }`
	result := analyze(aggregateRequest(t, source))
	if result.Status != "ok" {
		t.Fatalf("official constructor return rejected: %+v", result.Diagnostics)
	}
	decl := result.AggregateFacts.Declarations[0]
	body := decl.Constructors[0].Body
	if len(body) != 3 || body[0].Kind != "if" || len(body[0].ThenBody) != 1 || body[0].ThenBody[0].Kind != "block" {
		t.Fatalf("constructor control facts lost: %+v", body)
	}
	then := body[0].ThenBody[0].Body
	if len(then) != 2 || then[1].Kind != "return" || then[1].Expression != nil {
		t.Fatalf("bare constructor return invented a value: %+v", then)
	}
	if body[2].Kind != "return" || body[2].Expression == nil || body[2].Expression.Kind != "constructionReceiver" || body[2].Expression.Type != decl.ID || !decl.Fields[0].DefiniteAssertionSyntax || decl.Fields[0].Initializer != nil {
		t.Fatalf("self return/init syntax confused: %+v", body)
	}
}

func TestAggregateFieldCompoundUpdateCopyPreservesTypedMutation(t *testing.T) {
	t.Parallel()
	const source = `@tsfgValue class Pair { n: i64 = 7; }
export function answer(): i64 { const pair = new Pair(); pair.n += 2; pair.n++; const copy: Pair = pair; pair.n = 40; return copy.n; }`
	result := analyze(aggregateRequest(t, source))
	if result.Status != "ok" || len(result.Functions) != 1 {
		t.Fatalf("typed compound mutation falsely rejected: %+v", result)
	}
	body := result.Functions[0].Body
	if len(body) != 6 {
		t.Fatalf("mutation/copy statements were swallowed: %+v", body)
	}
	compound, update := body[1].Expression, body[2].Expression
	if compound.Kind != "fieldWrite" || compound.Operator != "+=" || compound.Type != "i64" || compound.Right.Type != "i64" || compound.Right.Literal == nil || compound.Right.Literal.Decimal != "2" || !compound.Left.Permission.Writable || compound.Left.Permission.ConstructorSelf {
		t.Fatalf("compound lost typed ordinary field permission or literal: %+v", compound)
	}
	if update.Kind != "fieldUpdate" || update.Operator != "++" || update.Prefix == nil || *update.Prefix || update.Type != "i64" || !update.Operand.Permission.Writable || update.Operand.Symbol != compound.Left.Symbol {
		t.Fatalf("postfix update lost actual field identity: %+v", update)
	}
	if body[3].Initializer.Kind != "reference" || body[3].Initializer.Type != result.AggregateFacts.Declarations[0].ID || body[4].Expression.Kind != "fieldWrite" || body[4].Expression.Operator != "=" || body[5].Expression.Kind != "field" || body[5].Expression.Receiver.Symbol != body[3].Symbol.ID {
		t.Fatalf("ordinary copy/store/read ownership lost: %+v", body)
	}
	for _, item := range []struct {
		name, source, code string
		upstream           int32
	}{
		{"readonly compound", `@tsfgValue class Pair { readonly n: i64 = 7; } export function answer(): i64 { const pair = new Pair(); pair.n += 2; return pair.n; }`, "r02b-d-type", 2540},
		{"readonly update", `@tsfgValue class Pair { readonly n: i64 = 7; } export function answer(): i64 { const pair = new Pair(); pair.n++; return pair.n; }`, "r02b-d-type", 2540},
		{"wrong scalar", `@tsfgValue class Pair { n: i64 = 7; } export function answer(): i64 { const pair = new Pair(); const wrong: u8 = 1; pair.n += wrong; return pair.n; }`, "r01-d-scalar-type", 2322},
	} {
		t.Run(item.name, func(t *testing.T) {
			result := analyze(aggregateRequest(t, item.source))
			found := false
			for _, d := range result.Diagnostics {
				if d.Code == item.code && d.UpstreamCode == item.upstream {
					found = true
				}
			}
			if result.Status != "error" || !found {
				t.Fatalf("mutation gate swallowed readonly/type diagnostic: %+v", result)
			}
		})
	}
}

func TestAggregateMixedCallSignaturesKeepNumericLiteralContexts(t *testing.T) {
	t.Parallel()
	const header = `@tsfgValue class Pair { n: u8; constructor(n: u8) { this.n = n; } }
`
	for _, item := range []struct{ name, source string }{
		{"aggregate return", `function make(n: u8): Pair { return new Pair(n); }
export function answer(): u8 { const pair = make(1); return pair.n; }`},
		{"aggregate parameter", `function take(p: Pair, n: u8): u8 { return p.n + n; }
export function answer(): u8 { const pair = new Pair(1); return take(pair, 1); }`},
	} {
		t.Run(item.name, func(t *testing.T) {
			result := analyze(aggregateRequest(t, header+item.source))
			if result.Status != "ok" || len(result.Functions) != 2 || result.AggregateFacts == nil || len(result.AggregateFacts.SourceCoverage) != 3 {
				t.Fatalf("mixed signature falsely rejected or source coverage lost: %+v", result)
			}
			owner := result.AggregateFacts.Declarations[0].ID
			callee, caller := result.Functions[0], result.Functions[1]
			var call *expression
			if item.name == "aggregate return" {
				if callee.ReturnType != owner || callee.Parameters[0].Type != "u8" {
					t.Fatalf("mixed return signature lost nominal/scalar identity: %+v", callee)
				}
				call = caller.Body[0].Initializer
			} else {
				if callee.ReturnType != "u8" || len(callee.Parameters) != 2 || callee.Parameters[0].Type != owner || callee.Parameters[1].Type != "u8" {
					t.Fatalf("mixed parameter signature lost nominal/scalar identity: %+v", callee)
				}
				call = caller.Body[1].Expression
			}
			if call.Kind != "call" || call.Target != callee.Symbol || call.Arguments == nil {
				t.Fatalf("actual checked ordinary call identity lost: %+v", call)
			}
			argument := (*call.Arguments)[len(*call.Arguments)-1]
			if argument.Type != "u8" || argument.Literal == nil || argument.Literal.Decimal != "1" {
				t.Fatalf("mixed call literal context lost: %+v", argument)
			}
		})
	}
	for _, source := range []string{
		`function make(n: u8): Pair { return new Pair(n); }
export function answer(): u8 { const pair = make(256); return pair.n; }`,
		`function take(p: Pair, n: u8): u8 { return p.n + n; }
export function answer(): u8 { const pair = new Pair(1); return take(pair, 256); }`,
	} {
		result := analyze(aggregateRequest(t, header+source))
		if result.Status != "error" {
			t.Fatalf("mixed signature numeric overflow admitted: %+v", result)
		}
		found := false
		for _, d := range result.Diagnostics {
			if d.Code == "r01-d-integer-range" && d.UpstreamCode == 2345 && sourceText(t, header+source, d.Range) == "256" {
				found = true
			}
		}
		if !found {
			t.Fatalf("mixed overflow lost owned range or genuine official identity: %+v", result.Diagnostics)
		}
	}
}

func TestAggregateFieldShiftKeepsOwnedDefaultI32Local(t *testing.T) {
	t.Parallel()
	const source = `@tsfgValue class Pair { n: i64 = 7; }
export function answer(): i64 { const pair = new Pair(); const count = 1; return pair.n << count; }
export function ordinary(): i32 { const count = 1; return count << 1; }`
	result := analyze(aggregateRequest(t, source))
	if result.Status != "ok" {
		t.Fatalf("owned default i32 local rejected by field diagnostic qualification: %+v", result.Diagnostics)
	}
	if len(result.Functions) != 2 {
		t.Fatalf("ordinary source coverage lost: %+v", result.Functions)
	}
	for index, function := range result.Functions {
		local := function.Body[len(function.Body)-2].Symbol
		shift := function.Body[len(function.Body)-1].Expression
		if local == nil || local.Type != "i32" || shift.Kind != "binary" || shift.Operator != "<<" || shift.Right.Type != "i32" {
			t.Fatalf("default scalar binding or shift context changed: %+v", function)
		}
		if index == 0 && (shift.Type != "i64" || shift.Left.Kind != "field" || shift.Right.Symbol != local.ID) {
			t.Fatalf("typed field shift lost owned operand identity: %+v", shift)
		}
	}
}

func TestAggregateTypedConstructorCopyFieldFacts(t *testing.T) {
	t.Parallel()
	source := `/*😀*/ @tsfgValue class Pair { readonly small: u8; large: i64; constructor(small: u8, large: i64) { this.small = small; this.small = small; this.large = large; } }
export function answer(): i64 { const pair = new Pair(255, -9223372036854775808); const copy: Pair = pair; return copy.large; }`
	result := analyze(aggregateRequest(t, source))
	if result.Status != "ok" {
		t.Fatalf("typed constructor rejected: %+v", result.Diagnostics)
	}
	if result.AggregateFacts == nil || result.AggregateFacts.Profile != markerVersion || len(result.AggregateFacts.Declarations) != 1 || len(result.Functions) != 1 || len(result.AggregateFacts.SourceCoverage) != 2 {
		t.Fatalf("missing full coverage: %+v", result)
	}
	decl := result.AggregateFacts.Declarations[0]
	if decl.Category != "value" || decl.Name != "Pair" || decl.InstanceType == decl.ConstructorType || len(decl.Fields) != 2 || len(decl.MemberCoverage) != 3 || len(decl.Constructors) != 1 {
		t.Fatalf("incorrect class facts: %+v", decl)
	}
	ctor := decl.Constructors[0]
	if ctor.Kind != "explicit" || ctor.SignatureDeclaration == nil || ctor.Implementation == nil || ctor.ResultType != decl.ID || len(ctor.Body) != 3 {
		t.Fatalf("lost constructor origin: %+v", ctor)
	}
	for _, item := range ctor.Body {
		if item.Expression == nil || item.Expression.Kind != "fieldWrite" || item.Expression.Left == nil || !item.Expression.Left.Permission.ConstructorSelf || !item.Expression.Left.Permission.Writable {
			t.Fatalf("constructor ownership lost: %+v", item)
		}
	}
	if !decl.Fields[0].Readonly || decl.Fields[0].DefiniteAssertionSyntax || decl.Fields[0].Initializer != nil || decl.Fields[0].Type != "u8" || decl.Fields[1].Type != "i64" {
		t.Fatalf("field source/init facts wrong: %+v", decl.Fields)
	}
	fn := result.Functions[0]
	if result.AggregateFacts.SourceCoverage[1].Range != fn.Range {
		t.Fatalf("function declaration owners disagree: coverage %+v, ordinary %+v", result.AggregateFacts.SourceCoverage[1].Range, fn.Range)
	}
	construct := fn.Body[0].Initializer
	if construct.Kind != "construct" || construct.Target != ctor.ID || len(*construct.Arguments) != 2 || (*construct.Arguments)[0].Literal.Decimal != "255" || (*construct.Arguments)[1].Literal.Decimal != "-9223372036854775808" {
		t.Fatalf("literal/new facts wrong: %+v", construct)
	}
	if fn.Body[1].Initializer.Kind != "reference" || fn.Body[1].Initializer.Type != decl.ID || fn.Body[2].Expression.Kind != "field" || fn.Body[2].Expression.FieldPath[0] != decl.Fields[1].ID {
		t.Fatalf("copy/read facts lost: %+v", fn.Body)
	}
	read := fn.Body[2].Expression
	if read.Receiver.Range.Start < read.Range.Start || read.Receiver.Range.End > read.Range.End || sourceText(t, source, read.Receiver.Range) != "copy" {
		t.Fatalf("official aggregate receiver span escapes field parent: %+v", read)
	}
	if sourceText(t, source, decl.Range)[0] != '@' || sourceText(t, source, *ctor.Implementation)[0:11] != "constructor" || sourceText(t, source, decl.BodyRange)[0] != '{' {
		t.Fatalf("incorrect owned class/body spans: %+v", decl)
	}
}

func TestAggregateImplicitConstructorNestedComponentFacts(t *testing.T) {
	t.Parallel()
	source := `@tsfgValue class Inner { readonly n: u8 = 7; }
@tsfgComponent class Outer { value: Inner = new Inner(); pending!: i64; }
export function answer(): u8 { const outer = new Outer(); return outer.value.n; }`
	result := analyze(aggregateRequest(t, source))
	if result.Status != "ok" {
		t.Fatalf("nested/default constructor rejected: %+v", result.Diagnostics)
	}
	decls := result.AggregateFacts.Declarations
	if len(decls) != 2 || decls[1].Category != "component" || decls[0].NominalKey.Category == decls[1].NominalKey.Category {
		t.Fatalf("component identity lost: %+v", decls)
	}
	for _, decl := range decls {
		ctor := decl.Constructors[0]
		if ctor.Kind != "implicitDefault" || ctor.SignatureDeclaration != nil || ctor.Implementation != nil || len(ctor.Parameters) != 0 || ctor.ResultType != decl.ID {
			t.Fatalf("invented default source signature: %+v", ctor)
		}
	}
	if !decls[1].Fields[1].DefiniteAssertionSyntax || decls[1].Fields[1].Initializer != nil {
		t.Fatalf("assertion confused with physical initialization: %+v", decls[1].Fields[1])
	}
	field := result.Functions[0].Body[1].Expression
	if len(field.FieldPath) != 2 || field.FieldPath[0] != decls[1].Fields[0].ID || field.FieldPath[1] != decls[0].Fields[0].ID {
		t.Fatalf("nested symbol path lost: %+v", field)
	}
}

func TestAggregateNominalAndOfficialNegativeControls(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, source, code string
		upstream           int32
	}{
		{"range", `@tsfgValue class Pair { n: u8; constructor(n:u8) { this.n=n; } } export function answer(): u8 { return new Pair(256).n; }`, "r01-d-integer-range", 2345},
		{"readonly alias", `@tsfgValue class Pair { readonly n: u8; constructor(n:u8) { this.n=n; const alias=this; alias.n=n; } } export function answer(): u8 { return new Pair(1).n; }`, "r02b-d-type", 2540},
		{"readonly foreign constructor", `@tsfgValue class Pair { readonly n:u8=1; } @tsfgValue class Other { p:Pair; constructor(p:Pair) { this.p=p; p.n=2; } } export function answer(): u8 { return new Pair().n; }`, "r02b-d-type", 2540},
		{"missing initialization", `@tsfgValue class Pair { n:u8; } export function answer(): u8 { return new Pair().n; }`, "r02b-d-type", 2564},
		{"nominal shape", `@tsfgValue class A { n:u8=1; } @tsfgValue class B { n:u8=1; } export function answer(): u8 { const a:A=new B(); return a.n; }`, "r01-d-aggregate-identity", 0},
		{"inheritance", `@tsfgValue class A { n:u8=1; } @tsfgValue class B extends A { } export function answer(): u8 { return new A().n; }`, "r01-d-aggregate-inheritance", 0},
		{"cycle", `@tsfgValue class A { self!:A; } export function answer(): u8 { return 1; }`, "r01-d-aggregate-cycle", 0},
		{"marker shadow", `export function tsfgValue(target: i32, context: i32): i32 { return target; } @tsfgValue class A { n:u8=1; } export function answer(): u8 { return new A().n; }`, "r01-d-aggregate-marker", 0},
		{"method explicitly unsupported", `@tsfgValue class A { n:u8=1; method():u8{return this.n;} } export function answer():u8{return new A().n;}`, "r02b-d-unsupported", 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := analyze(aggregateRequest(t, test.source))
			if result.Status != "error" {
				t.Fatalf("negative accepted: %+v", result)
			}
			found := false
			for _, d := range result.Diagnostics {
				if d.Code == test.code && (test.upstream == 0 || d.UpstreamCode == test.upstream) {
					found = true
				}
			}
			if !found {
				t.Fatalf("wrong diagnostic: %+v", result.Diagnostics)
			}
		})
	}
}

func TestAggregateMarkerBytesAndCoverageOrdinaryConversions(t *testing.T) {
	t.Parallel()
	r := aggregateRequest(t, `export function checked_u8_from_i32(x:i32):u8 { return checked_u8_from_i32(x); }
@tsfgValue class Pair { n:u8=1; }
export function answer(x:i32):u8 { return checked_u8_from_i32(x); }`)
	result := analyze(r)
	if result.Status != "ok" {
		t.Fatalf("ordinary shadow function rejected: %+v", result.Diagnostics)
	}
	if len(result.Functions) != 2 || len(result.AggregateFacts.SourceCoverage) != 3 || result.AggregateFacts.SourceCoverage[0].Symbol != result.Functions[0].Symbol || result.AggregateFacts.SourceCoverage[2].Symbol != result.Functions[1].Symbol || result.Functions[1].Body[0].Expression.Kind != "call" {
		t.Fatalf("ordinary conversion shadow was swallowed: %+v", result)
	}
	r.Files[len(r.Files)-1].Content += "\n"
	if got := analyze(r); got.Status != "error" || got.Diagnostics[0].Code != "r02b-d-input" {
		t.Fatalf("marker byte tamper accepted: %+v", got)
	}
}

func TestAggregateFieldPreservesAdmittedConversionOperand(t *testing.T) {
	t.Parallel()
	result := analyze(aggregateRequest(t, `@tsfgValue class Pair { n:u8=255; } export function answer():i64 { return checked_i64_from_u8(new Pair().n); }`))
	if result.Status != "ok" {
		t.Fatalf("field conversion rejected: %+v", result.Diagnostics)
	}
	value := result.Functions[0].Body[0].Expression
	if value.Kind != "conversion" || value.Conversion == nil || value.Operand == nil || value.Operand.Kind != "field" || value.Operand.Type != "u8" || value.Type != "i64" || value.Arguments != nil || value.Signature != nil || value.Target != "" {
		t.Fatalf("field changed admitted conversion shape: %+v", value)
	}
}

func TestAggregateNativeProtocolOwnedFacts(t *testing.T) {
	t.Parallel()
	request := aggregateRequest(t, `@tsfgValue class Pair { first!:u8; second:u8; constructor(second:u8) { this.second=second; this.first=this.second; } } export function answer():u8 { return new Pair(7).first; }`)
	var input, output bytes.Buffer
	if err := json.NewLegacyEncoder(&input).Encode(request); err != nil {
		t.Fatal(err)
	}
	if err := runCommand(&input, &output); err != nil {
		t.Fatal(err)
	}
	var result response
	if err := json.NewLegacyDecoder(&output).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" || result.AggregateFacts == nil || len(result.AggregateFacts.Declarations) != 1 {
		t.Fatalf("owned protocol rejected: %+v", result)
	}
	decl := result.AggregateFacts.Declarations[0]
	if decl.NominalKey.SourcePath != "/src/main.ts" || !decl.Fields[0].DefiniteAssertionSyntax || decl.Fields[0].Initializer != nil {
		t.Fatalf("identity or physical initialization fact changed: %+v", decl)
	}
	read := decl.Constructors[0].Body[1].Expression.Right
	if read.Kind != "field" || read.Receiver.Kind != "constructionReceiver" || read.FieldPath[0] != decl.Fields[1].ID || !read.Permission.ConstructorSelf {
		t.Fatalf("selected self leaf turned into whole-self copy: %+v", read)
	}
}

func TestAggregateMalformedSyntaxKeepsOfficialPublicDiagnostics(t *testing.T) {
	t.Parallel()
	const malformed = "@tsfgValue class Pair { n: i64 = 7;"
	tests := []struct {
		name, prefix string
		end, line    int
	}{
		{"missing class brace", "", 35, 0},
		{"UTF16 prefix", "// 💡\n", 41, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			r := aggregateRequest(t, test.prefix+malformed)
			var input, output bytes.Buffer
			if err := json.NewLegacyEncoder(&input).Encode(r); err != nil {
				t.Fatal(err)
			}
			if err := runCommand(&input, &output); err != nil {
				t.Fatal(err)
			}
			var result response
			if err := json.NewLegacyDecoder(&output).Decode(&result); err != nil {
				t.Fatal(err)
			}
			if result.Status != "error" || result.AggregateFacts != nil || len(result.Functions) != 0 {
				t.Fatalf("malformed source acquired projected facts: %+v", result)
			}
			found := false
			for _, d := range result.Diagnostics {
				if d.Code == "r01-d-aggregate-marker" {
					t.Fatalf("custom admission replaced official syntax: %+v", result.Diagnostics)
				}
				if d.Code != "r02b-d-syntax" || d.UpstreamCode != 1005 {
					continue
				}
				expected := primary{SourceID: "/src/main.ts", Start: test.end, End: test.end, StartLine: test.line, EndLine: test.line, StartColumn: len(malformed), EndColumn: len(malformed)}
				if d.File != "/src/main.ts" || d.Range != (span{test.end, test.end}) || d.Primary != expected || d.RuleID != "r01-ts-baseline" || len(d.RuleIDs) != 1 || d.RuleIDs[0] != d.RuleID || d.Category != "error" || d.Phase != "syntax" || d.Severity != "error" {
					t.Fatalf("official syntax identity or UTF16 anchor changed: %+v", d)
				}
				found = true
			}
			if !found {
				t.Fatalf("missing official closing-brace diagnostic: %+v", result.Diagnostics)
			}
		})
	}
}

func TestAggregateUnsynchronizedPresenceUsesPublicCapabilityDiagnostics(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, source, unsupportedText string
	}{
		{"default aggregate parameter", `@tsfgValue class Pair { n: i64 = 7; } function make(p: Pair = new Pair()): Pair { return p; } export function answer(): i64 { return make().n; }`, "p: Pair = new Pair()"},
		{"optional aggregate parameter", `@tsfgValue class Pair { n: i64 = 7; } function make(p?: Pair): Pair { if (p === undefined) { return new Pair(); } return p; } export function answer(): i64 { return make().n; }`, "p?: Pair"},
		{"default aggregate constructor parameter", `@tsfgValue class Pair { n: i64 = 7; } @tsfgValue class Holder { pair: Pair; constructor(pair: Pair = new Pair()) { this.pair = pair; } } export function answer(): i64 { return new Holder().pair.n; }`, "pair: Pair = new Pair()"},
		{"optional aggregate constructor parameter", `@tsfgValue class Pair { n: i64 = 7; } @tsfgValue class Holder { pair!: Pair; constructor(pair?: Pair) { if (pair !== undefined) { this.pair = pair; } } } export function answer(): i64 { return new Holder().pair.n; }`, "pair?: Pair"},
		{"aggregate payload nullish field assignment", `@tsfgValue class I { n: u8 = 1; } @tsfgValue class O { i: I = new I(); } export function answer(): u8 { const o = new O(); o.i ??= new I(); return o.i.n; }`, "o.i ??= new I()"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			r := aggregateRequest(t, test.source)
			var input, output bytes.Buffer
			if err := json.NewLegacyEncoder(&input).Encode(r); err != nil {
				t.Fatal(err)
			}
			if err := runCommand(&input, &output); err != nil {
				t.Fatal(err)
			}
			var result response
			if err := json.NewLegacyDecoder(&output).Decode(&result); err != nil {
				t.Fatal(err)
			}
			if result.Status != "error" || result.AggregateFacts != nil || len(result.Functions) != 0 {
				t.Fatalf("unsupported profile emitted incompatible facts: %+v", result)
			}
			start := bytes.Index([]byte(test.source), []byte(test.unsupportedText))
			if start < 0 {
				t.Fatal("missing independent source anchor")
			}
			end := start + len(test.unsupportedText)
			expected := primary{SourceID: "/src/main.ts", Start: start, End: end, StartLine: 0, EndLine: 0, StartColumn: start, EndColumn: end}
			found := false
			for _, d := range result.Diagnostics {
				if d.Code != "r02b-d-unsupported" {
					continue
				}
				if d.Category != "unsupported" || d.Phase != "capability" || d.Severity != "error" || d.RuleID != "r01-ts-baseline" || len(d.RuleIDs) != 1 || d.RuleIDs[0] != d.RuleID || d.UpstreamCode != 0 || d.File != "/src/main.ts" || d.Range != (span{start, end}) || d.Primary != expected {
					t.Fatalf("capability diagnostic lost its source identity: %+v", d)
				}
				found = true
			}
			if !found {
				t.Fatalf("missing public capability refusal: %+v", result.Diagnostics)
			}
		})
	}
}

func TestAggregateCapabilityBoundaryKeepsSupportedPublicFacts(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, source string }{
		{"required aggregate parameter", `@tsfgValue class Pair { n: i64 = 7; } function make(p: Pair): Pair { return p; } export function answer(): i64 { return make(new Pair()).n; }`},
		{"default scalar constructor parameter", `@tsfgValue class Pair { n: i64; constructor(n: i64 = 7) { this.n = n; } } export function answer(): i64 { return new Pair().n; }`},
		{"default scalar with aggregate parameter", `@tsfgValue class Pair { n: i64 = 7; } function take(p: Pair, n: i64 = 7): i64 { return p.n + n; } export function answer(): i64 { return take(new Pair()); }`},
		{"optional ordinary scalar parameter", `@tsfgValue class Pair { n: i64 = 7; } function take(n?: i64): i64 { if (n === undefined) { return 7; } return n; } export function answer(): i64 { const pair = new Pair(); return take(); }`},
		{"ordinary scalar nullish assignment", `@tsfgValue class Pair { n: i64 = 7; } export function answer(): i64 { const pair = new Pair(); let value: i64 | undefined = undefined; value ??= 7; return value; }`},
		{"scalar payload field nullish assignment", `@tsfgValue class Pair { n: i64 = 7; } export function answer(): i64 { const pair = new Pair(); pair.n ??= 7; return pair.n; }`},
		{"plain aggregate payload field assignment", `@tsfgValue class I { n: u8 = 1; } @tsfgValue class O { i: I = new I(); } export function answer(): u8 { const o = new O(); o.i = new I(); return o.i.n; }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			r := aggregateRequest(t, test.source)
			var input, output bytes.Buffer
			if err := json.NewLegacyEncoder(&input).Encode(r); err != nil {
				t.Fatal(err)
			}
			if err := runCommand(&input, &output); err != nil {
				t.Fatal(err)
			}
			var result response
			if err := json.NewLegacyDecoder(&output).Decode(&result); err != nil {
				t.Fatal(err)
			}
			if result.Status != "ok" || result.AggregateFacts == nil || len(result.Functions) == 0 || len(result.Diagnostics) != 0 {
				t.Fatalf("supported public facts rejected by aggregate capability boundary: %+v", result)
			}
		})
	}
}

func TestAggregateLocalRebindingPreservesOwnedPublicFacts(t *testing.T) {
	const header = `@tsfgValue class Pair { n:i64; m:i64; constructor(n:i64,m:i64) { this.n=n; this.m=m; } } `
	tests := []struct {
		name, source, code string
		upstream           int32
	}{
		{"captured receiver", header + `export function answer():i64 { let p=new Pair(10,20); p.n += (p=new Pair(100,200)).n-95; return p.n*1000+p.m; }`, "", 0},
		{"plain rebinding", header + `export function answer():i64 { let p=new Pair(10,20); p=new Pair(100,200); return p.n; }`, "", 0},
		{"immutable binding", header + `export function answer():i64 { const p=new Pair(10,20); p=new Pair(100,200); return p.n; }`, "", 2588},
		{"foreign nominal", header + `@tsfgValue class Other { n:i64=100; m:i64=200; } export function answer():i64 { let p=new Pair(10,20); p=new Other(); return p.n; }`, "r01-d-aggregate-identity", 0},
		{"constructor argument overflow", `@tsfgValue class Pair { n:u8; constructor(n:u8) { this.n=n; } } export function answer():u8 { let p=new Pair(1); p=new Pair(256); return p.n; }`, "r01-d-integer-range", 0},
	}
	for _, item := range tests {
		t.Run(item.name, func(t *testing.T) {
			r := aggregateRequest(t, item.source)
			var input, output bytes.Buffer
			if err := json.NewLegacyEncoder(&input).Encode(r); err != nil {
				t.Fatal(err)
			}
			if err := runCommand(&input, &output); err != nil {
				t.Fatal(err)
			}
			var result response
			if err := json.NewLegacyDecoder(&output).Decode(&result); err != nil {
				t.Fatal(err)
			}
			if item.code != "" || item.upstream != 0 {
				if result.Status != "error" {
					t.Fatalf("invalid nominal write admitted: %+v", result)
				}
				found := false
				for _, d := range result.Diagnostics {
					if item.code != "" && d.Code == item.code || item.upstream != 0 && d.UpstreamCode == item.upstream {
						found = true
					}
				}
				if !found {
					t.Fatalf("negative identity changed: %+v", result.Diagnostics)
				}
				return
			}
			if result.Status != "ok" || result.AggregateFacts == nil || len(result.Functions) != 1 || len(result.Diagnostics) != 0 {
				t.Fatalf("valid local rebinding rejected: %+v", result)
			}
			if item.name == "plain rebinding" {
				assignment := result.Functions[0].Body[1]
				if assignment.Kind != "assignment" || assignment.Target.Type != result.AggregateFacts.Declarations[0].ID || assignment.Value.Kind != "construct" || len(*assignment.Value.Arguments) != 2 {
					t.Fatalf("ordinary assignment facts changed: %+v", assignment)
				}
				return
			}
			write := result.Functions[0].Body[1].Expression
			if item.name == "captured receiver" {
				if write.Kind != "fieldWrite" || write.Operator != "+=" || write.Right.Kind != "binary" || write.Right.Left.Kind != "field" {
					t.Fatalf("ordered field-write facts changed: %+v", write)
				}
				write = write.Right.Left.Receiver.Operand
			}
			if write == nil || write.Kind != "write" || write.Operator != "=" || write.Type != result.AggregateFacts.Declarations[0].ID || write.Left.Kind != "reference" || write.Right.Kind != "construct" || len(*write.Right.Arguments) != 2 || write.Right.Signature.Parameters[0] != "i64" || write.Right.Signature.Parameters[1] != "i64" {
				t.Fatalf("nominal write/constructor facts lost: %+v", write)
			}
		})
	}
}

func TestAggregateScalarWritesConsumeFieldRHS(t *testing.T) {
	const header = `@tsfgValue class Pair { n:i64=7; } `
	for _, item := range []struct{ name, body, operator string }{
		{"compound", `const pair=new Pair(); let x:i64=2; x += pair.n; return x;`, "+="},
		{"assignment expression", `const pair=new Pair(); let x:i64=2; return (x = pair.n);`, "="},
	} {
		t.Run(item.name, func(t *testing.T) {
			source := header + "export function answer():i64 { " + item.body + " }"
			r := aggregateRequest(t, source)
			var input, output bytes.Buffer
			if err := json.NewLegacyEncoder(&input).Encode(r); err != nil {
				t.Fatal(err)
			}
			if err := runCommand(&input, &output); err != nil {
				t.Fatal(err)
			}
			var result response
			if err := json.NewLegacyDecoder(&output).Decode(&result); err != nil {
				t.Fatal(err)
			}
			if result.Status != "ok" || len(result.Diagnostics) != 0 || result.AggregateFacts == nil || len(result.Functions) != 1 {
				t.Fatalf("scalar write with field RHS rejected: %+v", result)
			}
			write := result.Functions[0].Body[2].Expression
			if write.Kind == "group" {
				write = write.Operand
			}
			if write.Kind != "write" || write.Operator != item.operator || write.Type != "i64" || write.Left.Kind != "reference" || write.Right.Kind != "field" || write.Right.Type != "i64" {
				t.Fatalf("owned scalar/field write facts changed: %+v", write)
			}
			if sourceText(t, source, write.Right.Range) != "pair.n" || write.Left.Range.Start < write.Range.Start || write.Right.Range.End > write.Range.End {
				t.Fatalf("write operands lost official source containment: %+v", write)
			}
		})
	}
}

func TestAggregateScalarWriteKeepsOrdinaryBoundaries(t *testing.T) {
	const header = `@tsfgValue class Pair { n:i64=7; count:i32=1; flag:bool=true; f:f64=1; } `
	tests := []struct {
		name, body, code string
		upstream         int32
	}{
		{"plain statement", `const pair=new Pair(); let x:i64=2; x=pair.n; return x;`, "", 0},
		{"shift count", `const pair=new Pair(); let x:i64=2; return (x <<= pair.count);`, "", 0},
		{"nullable nullish assignment", `const pair=new Pair(); let x:i64|undefined=undefined; return (x ??= pair.n);`, "", 0},
		{"nullable plain assignment", `const pair=new Pair(); let x:i64|undefined=undefined; return (x = pair.n);`, "", 0},
		{"boolean logical assignment", `const pair=new Pair(); let x:bool=true; return (x &&= pair.flag);`, "", 0},
		{"float compound", `const pair=new Pair(); let x:f64=2; return (x += pair.f);`, "", 0},
		{"immutable binding", `const pair=new Pair(); const x:i64=2; return (x = pair.n);`, "", 2588},
		{"wrong scalar payload", `const pair=new Pair(); let x:u8=2; return (x = pair.n);`, "r01-d-scalar-type", 0},
		{"wrong shift count", `const pair=new Pair(); let x:i64=2; return (x <<= pair.n);`, "r01-d-scalar-type", 0},
		{"unproven nullable compound", `const pair=new Pair(); let x:i64|undefined=undefined; return (x += pair.n);`, "", 18048},
	}
	for _, item := range tests {
		t.Run(item.name, func(t *testing.T) {
			resultType := "i64"
			if item.name == "boolean logical assignment" {
				resultType = "bool"
			}
			if item.name == "float compound" {
				resultType = "f64"
			}
			if item.name == "wrong scalar payload" {
				resultType = "u8"
			}
			source := header + "export function answer():" + resultType + " { " + item.body + " }"
			r := aggregateRequest(t, source)
			var input, output bytes.Buffer
			if err := json.NewLegacyEncoder(&input).Encode(r); err != nil {
				t.Fatal(err)
			}
			if err := runCommand(&input, &output); err != nil {
				t.Fatal(err)
			}
			var result response
			if err := json.NewLegacyDecoder(&output).Decode(&result); err != nil {
				t.Fatal(err)
			}
			if item.code != "" || item.upstream != 0 {
				if result.Status != "error" {
					t.Fatalf("invalid scalar write admitted: %+v", result)
				}
				found := false
				for _, d := range result.Diagnostics {
					if item.code != "" && d.Code == item.code || item.upstream != 0 && d.UpstreamCode == item.upstream {
						found = true
					}
				}
				if !found {
					t.Fatalf("ordinary write diagnostic changed: %+v", result.Diagnostics)
				}
				return
			}
			if result.Status != "ok" || len(result.Diagnostics) != 0 || result.AggregateFacts == nil {
				t.Fatalf("valid scalar write rejected: %+v", result)
			}
			if item.name == "plain statement" {
				return
			}
			write := result.Functions[0].Body[2].Expression
			if write.Kind == "group" {
				write = write.Operand
			}
			if write.Kind != "write" || write.Left.Kind != "reference" || write.Right.Kind != "field" || write.Type != resultType || !permittedPresence(write, nil) {
				t.Fatalf("ordinary scalar write/presence facts lost: %+v", write)
			}
		})
	}
}
