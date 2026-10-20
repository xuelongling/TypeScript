// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/json"
)

const classReferenceIdentitySource = `export class Left { value: number = 1; }
export class Right { value: number = 2; }
export class Compare {
  same(left: Left, right: Left): boolean { return left === right; }
  different(left: Left, right: Left): boolean { return left !== right; }
  across(left: Left, right: Right): boolean { return left === right; }
}`

// Candidate reference-identity expressions preserve operands and the official
// boolean result. Distinct allocation keys never decide source comparability or
// precompute the result of comparing two references.
func TestOwnedClassSourceReferenceIdentityPreservesOperandsAcrossClassViews(t *testing.T) {
	t.Parallel()
	graph := classSourceForTest(t, classReferenceIdentitySource)
	bytes, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}
	var view map[string]any
	if err := json.Unmarshal(bytes, &view); err != nil {
		t.Fatal(err)
	}
	classes := classOwnedArray(t, view["classes"])
	if len(classes) != 3 {
		t.Fatalf("complete identity comparison closure differs: %+v", classes)
	}
	left, right, compare := classOwnedNamed(t, classes, "Left"), classOwnedNamed(t, classes, "Right"), classOwnedNamed(t, classes, "Compare")
	if left["runtimeKey"] == "" || left["runtimeKey"] == nil || left["runtimeKey"] == right["runtimeKey"] ||
		left["instanceType"] == right["instanceType"] {
		t.Fatalf("identity fixture lost the two distinct actual classes: %+v %+v", left, right)
	}
	if len(classOwnedArray(t, compare["methods"])) != 3 {
		t.Fatalf("complete identity method set differs: %+v", compare)
	}
	symbols := map[any]bool{}
	for _, test := range []struct{ name, operator, rightClass string }{
		{"same", "===", "Left"},
		{"different", "!==", "Left"},
		{"across", "===", "Right"},
	} {
		method := classOwnedNamed(t, compare["methods"], test.name)
		signature := classOwnedObject(t, method["signature"])
		parameters := classOwnedArray(t, signature["parameters"])
		if signature["return"] != "t:boolean" || len(parameters) != 2 {
			t.Fatalf("official comparison method signature differs: %+v", method)
		}
		leftParameter, rightParameter := classOwnedObject(t, parameters[0]), classOwnedObject(t, parameters[1])
		rightClass := classOwnedNamed(t, classes, test.rightClass)
		if leftParameter["type"] != left["instanceType"] || rightParameter["type"] != rightClass["instanceType"] {
			t.Fatalf("official comparison operand types changed: %+v", parameters)
		}
		for _, parameter := range []map[string]any{leftParameter, rightParameter} {
			if parameter["symbol"] == "" || parameter["symbol"] == nil || symbols[parameter["symbol"]] {
				t.Fatalf("distinct actual comparison parameters share a binding: %+v", parameters)
			}
			symbols[parameter["symbol"]] = true
		}
		expression := classOwnedReturnedExpression(t, method)
		if expression["kind"] != "reference-identity" || expression["operator"] != test.operator || expression["type"] != "t:boolean" {
			t.Fatalf("reference identity was dropped or evaluated using static class identities: %+v", expression)
		}
		actualLeft, actualRight := classOwnedObject(t, expression["left"]), classOwnedObject(t, expression["right"])
		if actualLeft["kind"] != "reference" || actualRight["kind"] != "reference" ||
			actualLeft["symbol"] != leftParameter["symbol"] || actualRight["symbol"] != rightParameter["symbol"] ||
			actualLeft["type"] != leftParameter["type"] || actualRight["type"] != rightParameter["type"] {
			t.Fatalf("reference identity operands lost their actual checked parameter bindings: %+v", expression)
		}
		methodStart := strings.Index(classReferenceIdentitySource, test.name+"(")
		token := "left " + test.operator + " right"
		start := methodStart + strings.Index(classReferenceIdentitySource[methodStart:], token)
		origin := classOwnedObject(t, expression["origin"])
		if origin["source"] != "/original/main.ts" || origin["start"] != float64(start) || origin["end"] != float64(start+len(token)) {
			t.Fatalf("comparison original expression anchor differs: %+v", origin)
		}
		statement := classOwnedObject(t, classOwnedArray(t, method["body"])[0])
		witness := classOwnedObject(t, statement["compatibility"])
		if witness["sourceType"] != "t:boolean" || witness["targetType"] != "t:boolean" ||
			witness["assignable"] != true || len(classOwnedArray(t, witness["members"])) != 0 {
			t.Fatalf("comparison typed return lost its official boolean relation: %+v", witness)
		}
	}
}
