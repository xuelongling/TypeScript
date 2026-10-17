// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
)

// These test-local JSON views retain literal presence as well as its value.
// Shared historical test DTOs stay unchanged and cannot erase false into zero.
type closureFlowWireExpression struct {
	Kind, Type, Operator, Symbol string
	Target                       *string
	Boolean                      *bool
	Signature                    *functionValueWireSignature
	Callee, Left, Right, Operand *closureFlowWireExpression
	Arguments                    []closureFlowWireExpression
	Range                        span
}
type closureFlowWireStatement struct {
	Kind                                              string
	Symbol                                            *functionValueWireSymbol
	Initializer, Expression, Target, Value, Condition *closureFlowWireExpression
	Body, ThenBody, ElseBody                          []closureFlowWireStatement
	Range                                             span
}
type closureFlowWireFunction struct {
	Name, Symbol, ReturnType, LexicalParent string
	ReturnSignature                         *functionValueWireSignature
	Parameters                              []functionValueWireSymbol
	Body                                    []closureFlowWireStatement
	Captures                                []struct {
		Symbol      string
		Read, Write *bool
	}
	Range span
}
type closureFlowResponse struct {
	Status      string
	Inputs      []string
	Functions   []closureFlowWireFunction
	Symbols     []functionValueWireSymbol
	Diagnostics []json.RawMessage
}

func TestFrontendCommandClosureFlowFacts(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"sharedCallableWrites", "export function entry(seed: bool): bool { function first(): bool { return seed; } function other(): bool { return false; } let selected: () => bool = first; function read(): bool { return selected(); } function write(next: () => bool): bool { selected = next; return selected(); } const alias = selected; write(other); return read() && alias(); }"},
		{"branchAndLoop", "export function entry(seed: bool): bool { function first(): bool { return seed; } function other(): bool { return false; } let selected: () => bool = first; if (seed) { selected = other; } else { selected = first; } const before = selected(); let again: bool = seed; while (again) { selected = other; again = false; } return selected() && before; }"},
		{"calleeBeforeArgumentRebind", "export function entry(seed: bool): bool { function first(leftValue: bool): bool { return leftValue; } function other(rightValue: bool): bool { return !rightValue; } let selected: (argument: bool) => bool = first; return selected((selected = other)(seed)); }"},
		{"mutualHigherOrder", "export function entry(seed: bool): bool { function leaf(): bool { return seed; } function left(fn: () => bool, again: bool): () => bool { if (again) { return right(fn, false); } return fn; } function right(next: () => bool, again: bool): () => bool { if (again) { return left(next, false); } return next; } const selected = left(leaf, seed); return selected(); }"},
		{"externalAndFactoryActivations", "export function entry(external: () => bool, seed: bool): bool { function first(): bool { return seed; } function other(): bool { return false; } function make(fn: () => bool): () => bool { function invoke(): bool { return fn(); } return invoke; } const left = make(first); const right = make(other); return external() && left() && right(); }"},
		{"transitiveExternalRootEnvironment", "export function entry(seed: bool): bool { function first(): bool { return seed; } function other(): bool { return false; } let selected: () => bool = first; function middle(): bool { function helper(): bool { return selected(); } return helper(); } return middle(); }"},
		{"capturedBlockLocal", "export function entry(seed: bool): bool { function fallback(): bool { return false; } let selected: () => bool = fallback; { const value: bool = seed; function read(): bool { return value; } selected = read; } return selected(); }"},
		{"capturedLoopLocal", "export function entry(seed: bool): bool { function fallback(): bool { return false; } let selected: () => bool = fallback; let again: bool = seed; while (again) { const value: bool = again; function read(): bool { return value; } selected = read; again = false; } return selected(); }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := integerRequest(tc.source)
			var oracle closureFlowOracle
			if !t.Run("officialDiagnosticsSymbolsASTAndSignatures", func(t *testing.T) {
				oracle = closureFlowOfficial(t, r)
			}) {
				t.Fatal("independent official oracle failed; owned command was not attempted")
			}
			t.Run("ownedProjection", func(t *testing.T) {
				input, err := json.Marshal(r)
				if err != nil {
					t.Fatal(err)
				}
				original, err := json.Marshal(map[string]any{"rootFile": r.RootFile, "files": r.Files})
				if err != nil {
					t.Fatal(err)
				}
				var output bytes.Buffer
				commandErr := runFrontendCommand(t.Context(), nil, bytes.NewReader(input), &output)
				t.Logf("TSFG_CLOSURE_FLOW_REQUEST_V1 %s", base64.StdEncoding.EncodeToString(input))
				t.Logf("TSFG_CLOSURE_FLOW_RESPONSE_V1 %s", base64.StdEncoding.EncodeToString(output.Bytes()))
				t.Logf("TSFG_CLOSURE_FLOW_ORIGINAL_V1 %s", base64.StdEncoding.EncodeToString(original))
				if commandErr != nil {
					t.Fatalf("public command failed: %v; raw=%s", commandErr, output.Bytes())
				}
				var result closureFlowResponse
				if err := json.Unmarshal(output.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if result.Status != "ok" || len(result.Diagnostics) != 0 {
					t.Fatalf("officially valid closure flow was rejected: status=%s diagnostics=%s", result.Status, result.Diagnostics)
				}
				if !reflect.DeepEqual(result.Inputs, []string{r.RootFile, r.Declarations.Paths[0]}) {
					t.Fatal("original input closure changed")
				}
				closureFlowOwned(t, tc.source, oracle, result)
				closureFlowRejectBooleanMutants(t, oracle, output.Bytes())
			})
		})
	}
}

func closureFlowOwned(t *testing.T, source string, oracle closureFlowOracle, result closureFlowResponse) {
	t.Helper()
	if err := closureFlowBooleanFacts(oracle, result); err != nil {
		t.Fatal(err)
	}
	if len(result.Functions) != len(oracle.functions) || len(result.Symbols) != len(oracle.declarations) {
		t.Fatal("owned definition counts differ from official AST")
	}
	definitions := map[string]functionValueWireSymbol{}
	for _, symbol := range result.Symbols {
		definitions[symbol.ID] = symbol
	}
	ids := map[*ast.Symbol]string{}
	seen := map[string]bool{}
	for _, expected := range oracle.declarations {
		var found *functionValueWireSymbol
		for i := range result.Symbols {
			got := &result.Symbols[i]
			if got.Name == expected.name && got.Kind == expected.kind && got.Range == expected.anchor {
				if found != nil {
					t.Fatal("ambiguous original declaration anchor")
				}
				found = got
			}
		}
		if found == nil || found.ID == "" || seen[found.ID] || found.Type != expected.typ || found.Writable != expected.writable || !reflect.DeepEqual(found.Signature, expected.signature) {
			t.Fatalf("owned original symbol changed: %s", expected.name)
		}
		ids[expected.symbol], seen[found.ID] = found.ID, true
	}
	for _, expected := range oracle.declarations {
		if definitions[ids[expected.symbol]].LexicalOwner != ids[expected.owner] {
			t.Fatalf("original owner changed: %s", expected.name)
		}
	}
	assertDefinition := func(got *functionValueWireSymbol, symbol *ast.Symbol) {
		if got == nil || got.ID != ids[symbol] {
			t.Fatal("inline binding lost its official declaration or position")
		}
		definition, found := definitions[got.ID]
		if !found || !reflect.DeepEqual(*got, definition) {
			t.Fatal("inline binding differs from its owned definition")
		}
	}
	checkRange := func(got, expected span) {
		if got.End != expected.End || got.Start < expected.Start || got.Start > expected.End {
			t.Fatalf("source end/anchor changed: got=%+v official=%+v", got, expected)
		}
		prefix := source[expected.Start:got.Start]
		if strings.TrimSpace(prefix) != "" {
			t.Fatalf("owned anchor skipped an original source token: %q", prefix)
		}
	}
	var expression func(*closureFlowWireExpression, *closureFlowExpr)
	expression = func(got *closureFlowWireExpression, expected *closureFlowExpr) {
		if got == nil || expected == nil {
			if (got == nil) != (expected == nil) {
				t.Fatal("owned expression omitted or invented")
			}
			return
		}
		checkRange(got.Range, expected.anchor)
		if got.Kind != expected.kind || got.Type != expected.typ || got.Operator != expected.operator || !reflect.DeepEqual(got.Signature, expected.signature) {
			t.Fatalf("typed original expression changed: source=%q got=%+v wantKind=%s", sourceText(t, source, got.Range), got, expected.kind)
		}
		if expected.kind == "reference" {
			if got.Symbol != ids[expected.symbol] || got.Target != nil {
				t.Fatal("reference lost official Symbol identity")
			}
		} else if expected.kind == "call" || expected.kind == "functionReference" {
			if got.Target == nil || *got.Target != ids[expected.symbol] || got.Symbol != "" {
				t.Fatal("named target lost official declaration")
			}
		} else if got.Symbol != "" || got.Target != nil {
			t.Fatal("non-reference gained a guessed target")
		}
		expression(got.Callee, expected.callee)
		expression(got.Left, expected.left)
		expression(got.Right, expected.right)
		expression(got.Operand, expected.operand)
		if len(got.Arguments) != len(expected.arguments) {
			t.Fatal("argument count changed")
		}
		for i := range expected.arguments {
			expression(&got.Arguments[i], expected.arguments[i])
		}
	}
	var statements func([]closureFlowWireStatement, []*closureFlowStmt)
	statements = func(got []closureFlowWireStatement, expected []*closureFlowStmt) {
		if len(got) != len(expected) {
			t.Fatal("ordered original statements omitted or invented")
		}
		for i, want := range expected {
			item := got[i]
			checkRange(item.Range, want.anchor)
			if item.Kind != want.kind {
				t.Fatalf("statement kind changed: got=%s want=%s", item.Kind, want.kind)
			}
			if want.symbol == nil {
				if item.Symbol != nil {
					t.Fatal("statement invented a binding")
				}
			} else {
				assertDefinition(item.Symbol, want.symbol)
			}
			expression(item.Initializer, want.initializer)
			expression(item.Expression, want.expression)
			expression(item.Target, want.target)
			expression(item.Value, want.value)
			expression(item.Condition, want.condition)
			statements(item.Body, want.body)
			statements(item.ThenBody, want.thenBody)
			statements(item.ElseBody, want.elseBody)
		}
	}
	for i, expected := range oracle.functions {
		got := result.Functions[i]
		if got.Name != expected.name || got.Symbol != ids[expected.symbol] || got.LexicalParent != ids[expected.parent] || got.Range != expected.anchor || got.ReturnType != expected.signature.ReturnType || !reflect.DeepEqual(got.ReturnSignature, expected.signature.ReturnSignature) {
			t.Fatalf("original function order, parent, anchor or return changed: %s", expected.name)
		}
		if len(got.Parameters) != len(expected.parameters) || len(got.Captures) != len(expected.captures) {
			t.Fatalf("parameter or direct capture count changed: %s", expected.name)
		}
		for i, symbol := range expected.parameters {
			assertDefinition(&got.Parameters[i], symbol)
		}
		captures := map[string]bool{}
		for _, capture := range got.Captures {
			if captures[capture.Symbol] || capture.Read == nil || capture.Write == nil {
				t.Fatal("duplicate or omitted explicit capture access flags")
			}
			captures[capture.Symbol] = true
			found := false
			for symbol, flags := range expected.captures {
				if ids[symbol] == capture.Symbol && flags == [2]bool{*capture.Read, *capture.Write} {
					found = true
				}
			}
			if !found {
				t.Fatalf("capture copied or changed original owner/read/write: %s", expected.name)
			}
		}
		statements(got.Body, expected.body)
	}
}

// This pure check compares all literal payloads with fresh official AST values.
// It returns errors so rejection-only corruptions never rely on t.Fatal control flow.
func closureFlowBooleanFacts(oracle closureFlowOracle, result closureFlowResponse) error {
	expected := map[int]bool{}
	for anchor, value := range oracle.booleans {
		expected[anchor.End] = value
	}
	seen := map[int]bool{}
	var expression func(*closureFlowWireExpression) error
	expression = func(value *closureFlowWireExpression) error {
		if value == nil {
			return nil
		}
		if value.Kind == "boolean" {
			want, found := expected[value.Range.End]
			if !found || seen[value.Range.End] || value.Type != "bool" || value.Boolean == nil || *value.Boolean != want {
				return fmt.Errorf("literal payload differs from official AST at %+v", value.Range)
			}
			seen[value.Range.End] = true
		} else if value.Boolean != nil {
			return fmt.Errorf("nonliteral carries a boolean payload at %+v", value.Range)
		}
		for _, child := range []*closureFlowWireExpression{value.Callee, value.Left, value.Right, value.Operand} {
			if err := expression(child); err != nil {
				return err
			}
		}
		for i := range value.Arguments {
			if err := expression(&value.Arguments[i]); err != nil {
				return err
			}
		}
		return nil
	}
	var statements func([]closureFlowWireStatement) error
	statements = func(body []closureFlowWireStatement) error {
		for _, item := range body {
			for _, value := range []*closureFlowWireExpression{item.Initializer, item.Expression, item.Target, item.Value, item.Condition} {
				if err := expression(value); err != nil {
					return err
				}
			}
			for _, nested := range [][]closureFlowWireStatement{item.Body, item.ThenBody, item.ElseBody} {
				if err := statements(nested); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, fn := range result.Functions {
		if err := statements(fn.Body); err != nil {
			return err
		}
	}
	if len(seen) != len(expected) {
		return fmt.Errorf("literal count differs from official AST: got=%d want=%d", len(seen), len(expected))
	}
	return nil
}

func closureFlowRejectBooleanMutants(t *testing.T, oracle closureFlowOracle, actual []byte) {
	t.Helper()
	ends := []int{}
	for anchor := range oracle.booleans {
		ends = append(ends, anchor.End)
	}
	sort.Ints(ends)
	for _, end := range ends {
		for _, mode := range []string{"flipFalseToTrue", "omit"} {
			var tree any
			if err := json.Unmarshal(actual, &tree); err != nil {
				t.Fatal(err)
			}
			var changed bool
			var visit func(any)
			visit = func(value any) {
				switch node := value.(type) {
				case map[string]any:
					if node["kind"] == "boolean" {
						anchor, ok := node["range"].(map[string]any)
						if ok && anchor["end"] == float64(end) {
							original, present := node["boolean"].(bool)
							if changed || !present {
								t.Fatal("actual frame lost its unique explicit literal")
							}
							if mode == "flipFalseToTrue" {
								if original {
									t.Fatal("this corruption requires an actual false literal")
								}
								node["boolean"] = true
							} else {
								delete(node, "boolean")
							}
							changed = true
						}
					}
					for _, child := range node {
						visit(child)
					}
				case []any:
					for _, child := range node {
						visit(child)
					}
				}
			}
			visit(tree)
			if !changed {
				t.Fatal("literal mutation did not address the actual response")
			}
			bytes, err := json.Marshal(tree)
			if err != nil {
				t.Fatal(err)
			}
			var corrupted closureFlowResponse
			if err := json.Unmarshal(bytes, &corrupted); err != nil {
				t.Fatal(err)
			}
			if err := closureFlowBooleanFacts(oracle, corrupted); err == nil {
				t.Fatalf("literal %s corruption at %d was accepted", mode, end)
			}
		}
	}
}
