// SPDX-License-Identifier: Apache-2.0
// Compare private preflight with the fixed official JSON writer and streaming reader.
package main

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/json"
)

func arrayBudgetWalk26(value any, policy genericJSONPolicy, framing int) (arrayFactsJSONBudget, error) {
	budget := arrayFactsJSONBudget{policy: policy, bytes: framing,
		types: map[reflect.Type]bool{}, fields: map[reflect.Type][]arrayFactsJSONField{}}
	actual := reflect.ValueOf(value)
	if err := budget.trusted(actual.Type()); err != nil {
		return budget, err
	}
	err := budget.walk(actual, 0)
	return budget, err
}
func arrayBudgetOfficialWire26(t *testing.T, value any) ([]byte, genericJSONStats) {
	t.Helper()
	// Only small bounded test fixtures reach this independent writer.
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	canonical := json.Value(encoded)
	if err := canonical.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	stats, err := inspectGenericJSON(canonical, arrayFactsPolicy(true))
	if err != nil {
		t.Fatal(err)
	}
	return canonical, stats
}
func TestArrayBoundaryBudget25fMatchesOfficialWire(t *testing.T) {
	emptyValues := []arrayGenericExpression{}
	emptyInts := []int{}
	var nilInts []int
	value := struct {
		False       bool     `json:"false,omitempty"`
		Zero        int      `json:"zero,omitempty"`
		Text        string   `json:"text"`
		Empty       string   `json:"empty,omitempty"`
		EmptyObject struct{} `json:"object,omitempty"`
		EmptyArray  []int    `json:"emptyArray,omitempty"`
		NilArray    []int    `json:"nilArray"`
		Pointer     *[]int   `json:"pointer,omitzero"`
		Unset       *[]int   `json:"unset,omitzero"`
	}{Text: "\x00é😀", EmptyArray: emptyInts, NilArray: nilInts, Pointer: &nilInts}
	for _, item := range []struct {
		name  string
		value any
	}{
		{"json-v2-empty-shapes", value},
		{"empty-arguments", arrayGenericExpression{Kind: "call", Arguments: &emptyValues}},
		{"empty-elements", arrayGenericExpression{Kind: "arrayLiteral", Elements: &emptyValues}},
		{"omitted-methods", arrayGenericType{ID: "f64", Kind: "scalar"}},
		{"readonly-length", arrayFactsReadSpanLength{Readonly: true}},
		{"pointer", &arrayGenericExpression{Kind: "numericLiteral", Type: "f64", Range: span{Start: 0, End: 1}, Token: "0", Bits: "0000000000000000"}},
	} {
		t.Run(item.name, func(t *testing.T) {
			expected, stats := arrayBudgetOfficialWire26(t, item.value)
			for _, framing := range []int{0, 1} {
				measured, err := arrayBudgetWalk26(item.value, arrayFactsPolicy(true), framing)
				if err != nil || measured.bytes != len(expected)+framing || measured.tokens != stats.tokens {
					t.Fatalf("complete wire budget mismatch bytes=%d/%d tokens=%d/%d err=%v", measured.bytes, len(expected)+framing, measured.tokens, stats.tokens, err)
				}
			}
			actual, err := arrayFactsCanonical(item.value)
			if err != nil || !bytes.Equal(actual, expected) {
				t.Fatalf("preflight changed official canonical output: %s %v", actual, err)
			}
			if item.name == "json-v2-empty-shapes" && (!bytes.Contains(actual, []byte("\"false\":false")) || !bytes.Contains(actual, []byte("\"zero\":0")) || !bytes.Contains(actual, []byte("\"nilArray\":[]")) || !bytes.Contains(actual, []byte("\"pointer\":[]"))) {
				t.Fatal("JSONv2 false/zero/nil-slice/nonzero-pointer semantics lost")
			}
			if item.name == "empty-arguments" && !bytes.Contains(actual, []byte("\"arguments\":[]")) {
				t.Fatal("empty required arguments dropped")
			}
			if item.name == "empty-elements" && !bytes.Contains(actual, []byte("\"elements\":[]")) {
				t.Fatal("empty required elements dropped")
			}
			if item.name == "omitted-methods" && bytes.Contains(actual, []byte("methods")) {
				t.Fatal("omitted methods were charged/encoded")
			}
		})
	}
}
func TestArrayBoundaryBudget25fInclusiveWireBoundaries(t *testing.T) {
	value := struct {
		Data []string `json:"data"`
	}{Data: []string{"\x00é😀"}}
	wire, stats := arrayBudgetOfficialWire26(t, value)
	for _, name := range []string{"bytes", "tokens", "depth", "string"} {
		for _, delta := range []int{0, -1} {
			label := name + "/at"
			if delta < 0 {
				label = name + "/below"
			}
			t.Run(label, func(t *testing.T) {
				policy := arrayFactsPolicy(true)
				switch name {
				case "bytes":
					policy.maxBytes = len(wire) + delta
				case "tokens":
					policy.maxTokens = stats.tokens + delta
				case "depth":
					policy.maxDepth = stats.depth + delta
				case "string":
					policy.maxString = len("\x00é😀") + delta
				}
				_, err := arrayBudgetWalk26(value, policy, 0)
				if delta == 0 {
					if err != nil {
						t.Fatalf("inclusive exact wire boundary failed: %v", err)
					}
					return
				}
				failure := arrayBudgetFailure26(t, err)
				if failure.Code != "r03-array-facts-limit" {
					t.Fatalf("wire bound lost limit classification: %+v", failure)
				}
			})
		}
	}
}

type arrayBudgetNode26 struct {
	Value string             `json:"value"`
	Next  *arrayBudgetNode26 `json:"next,omitempty"`
}

func TestArrayBoundaryBudget25fDepthAndCycles(t *testing.T) {
	root := &arrayBudgetNode26{Value: "x"}
	tail := root
	for i := 1; i < 64; i++ {
		tail.Next = &arrayBudgetNode26{Value: "x"}
		tail = tail.Next
	}
	expected, stats := arrayBudgetOfficialWire26(t, root)
	actual, err := arrayFactsCanonical(root)
	if stats.depth != 64 || err != nil || !bytes.Equal(actual, expected) {
		t.Fatalf("64 actual JSON objects through pointers: %v", err)
	}
	tail.Next = &arrayBudgetNode26{Value: "x"}
	if _, err := arrayFactsCanonical(root); arrayBudgetFailure26(t, err).Code != "r03-array-facts-limit" {
		t.Fatal("65 objects passed JSON depth")
	}
	root.Next = root
	if _, err := arrayFactsCanonical(root); arrayBudgetFailure26(t, err).Detail != "JSON depth" {
		t.Fatal("cycle did not fail at bounded JSON depth")
	}
}

var arrayBudgetMethodCalls26 int

type arrayBudgetCustomZero26 int

func (arrayBudgetCustomZero26) IsZero() bool { arrayBudgetMethodCalls26++; return true }

type arrayBudgetCustomJSON26 struct{}

func (arrayBudgetCustomJSON26) MarshalJSON() ([]byte, error) {
	arrayBudgetMethodCalls26++
	return []byte("null"), nil
}
func TestArrayBoundaryBudget25fRejectsUnsafeTypesAndUnicode(t *testing.T) {
	arrayBudgetMethodCalls26 = 0
	for _, value := range []any{
		arrayBudgetCustomZero26(0), &arrayBudgetCustomJSON26{},
		struct {
			Hidden *arrayBudgetCustomZero26 `json:"hidden,omitzero"`
		}{},
		func() {}, map[string]int{}, struct {
			Dynamic any `json:"dynamic"`
		}{Dynamic: "x"},
		struct {
			Number int `json:"number,string"`
		}{},
	} {
		encoded, err := arrayFactsCanonical(value)
		failure := arrayBudgetFailure26(t, err)
		if failure.Code != "r03-array-facts-json" || encoded != nil || arrayBudgetMethodCalls26 != 0 {
			t.Fatalf("unsafe/custom type reached encoding: %T %v calls=%d", value, err, arrayBudgetMethodCalls26)
		}
	}
	for _, value := range []struct {
		Data string `json:"data"`
	}{
		{Data: string([]byte{0xff})}, {Data: strings.Repeat("a", 1048577)},
	} {
		encoded, err := arrayFactsCanonical(value)
		failure := arrayBudgetFailure26(t, err)
		if encoded != nil || failure.Code != "r03-array-facts-unicode" && failure.Code != "r03-array-facts-limit" {
			t.Fatalf("invalid UTF8/string limit reached writer: %v", err)
		}
	}
}

func arrayBudgetFailure26(t *testing.T, err error) *arrayFactsFailure {
	t.Helper()
	var failure *arrayFactsFailure
	if !errors.As(err, &failure) {
		t.Fatalf("expected owned Array boundary failure: %v", err)
	}
	return failure
}
