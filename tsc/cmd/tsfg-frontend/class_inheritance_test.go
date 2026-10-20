// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/json"
)

const classInheritanceSource = `export class Base {
  value: number = 1;
  constructor(seed: number) {
    this.value = seed;
    this.observe(this);
    this.touch();
  }
  observe(other: Base): number { return other.value; }
  touch(): number { return this.value; }
}
export class Derived extends Base {
  extra: number = 2;
  constructor(seed: number) {
    super(seed);
    this.extra = this.touch();
  }
  override touch(): number { return this.value; }
  baseTouch(): number { return super.touch(); }
}`

// Read the owned output as a consumer. No missing Go DTO field can replace the
// intended real Source capability RED with a compiler error.
// Agreed candidate keys: base references the direct base class; overrideTarget
// references the overridden method; dispatch separates virtual and direct base
// calls; staticTarget is the official resolved method/constructor declaration.
// superCall references the explicit super expression that precedes own field
// initialization; argumentCompatibility owns official source/target type
// witnesses and the corresponding member-symbol projections.
func classOwnedObject(t *testing.T, value any) map[string]any {
	t.Helper()
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("owned object missing: %#v", value)
	}
	return object
}

func classOwnedArray(t *testing.T, value any) []any {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("owned array missing: %#v", value)
	}
	return items
}

func classOwnedNamed(t *testing.T, value any, name string) map[string]any {
	t.Helper()
	for _, item := range classOwnedArray(t, value) {
		object := classOwnedObject(t, item)
		if object["name"] == name {
			return object
		}
	}
	t.Fatalf("owned declaration %q missing", name)
	return nil
}

func classOwnedExpression(t *testing.T, statement any) map[string]any {
	t.Helper()
	return classOwnedObject(t, classOwnedObject(t, statement)["expression"])
}

func classOwnedReturnedExpression(t *testing.T, method map[string]any) map[string]any {
	t.Helper()
	body := classOwnedArray(t, method["body"])
	if len(body) != 1 || classOwnedObject(t, body[0])["kind"] != "return" {
		t.Fatalf("complete single return method body differs: %+v", method)
	}
	return classOwnedExpression(t, body[0])
}

func TestOwnedClassSourceInheritanceOwnsDispatchAndSuperInitialization(t *testing.T) {
	t.Parallel()
	graph := classSourceForTest(t, classInheritanceSource)
	bytes, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}
	var view map[string]any
	if err := json.Unmarshal(bytes, &view); err != nil {
		t.Fatal(err)
	}
	classes := classOwnedArray(t, view["classes"])
	if len(classes) != 2 {
		t.Fatalf("want Base and Derived, got %+v", classes)
	}
	base := classOwnedNamed(t, classes, "Base")
	derived := classOwnedNamed(t, classes, "Derived")
	if derived["base"] != base["id"] || base["runtimeKey"] == derived["runtimeKey"] ||
		base["instanceType"] == derived["instanceType"] {
		t.Fatalf("official base edge or distinct actual classes missing: base=%+v derived=%+v", base, derived)
	}
	baseFields, derivedFields := classOwnedArray(t, base["fields"]), classOwnedArray(t, derived["fields"])
	if len(baseFields) != 1 || len(derivedFields) != 1 {
		t.Fatalf("own fields mixed with inherited layout: %+v %+v", baseFields, derivedFields)
	}
	value, extra := classOwnedObject(t, baseFields[0]), classOwnedObject(t, derivedFields[0])
	if value["name"] != "value" || extra["name"] != "extra" || value["ordinal"] != float64(0) || extra["ordinal"] != float64(0) || value["id"] == extra["id"] {
		t.Fatalf("own field order/identity differs: %+v %+v", value, extra)
	}
	if len(classOwnedArray(t, base["methods"])) != 2 || len(classOwnedArray(t, derived["methods"])) != 2 {
		t.Fatalf("complete own method sets differ: %+v %+v", base["methods"], derived["methods"])
	}
	baseTouch := classOwnedNamed(t, base["methods"], "touch")
	observe := classOwnedNamed(t, base["methods"], "observe")
	derivedTouch := classOwnedNamed(t, derived["methods"], "touch")
	if derivedTouch["overrideTarget"] != baseTouch["id"] {
		t.Fatalf("official override target missing: %+v", derivedTouch)
	}
	derivedReturn := classOwnedReturnedExpression(t, derivedTouch)
	if derivedReturn["symbol"] != value["symbol"] || classOwnedObject(t, derivedReturn["receiver"])["symbol"] != derived["symbol"] {
		t.Fatalf("Derived this.value did not resolve inherited Base.value: %+v", derivedReturn)
	}
	baseReturn := classOwnedReturnedExpression(t, baseTouch)
	if baseReturn["symbol"] != value["symbol"] || classOwnedObject(t, baseReturn["receiver"])["symbol"] != base["symbol"] {
		t.Fatalf("Base touch resolution differs: %+v", baseReturn)
	}
	observeParameters := classOwnedArray(t, classOwnedObject(t, observe["signature"])["parameters"])
	if len(observeParameters) != 1 || classOwnedObject(t, observeParameters[0])["type"] != base["instanceType"] {
		t.Fatalf("observe Base parameter differs: %+v", observeParameters)
	}
	observed := classOwnedReturnedExpression(t, observe)
	if observed["symbol"] != value["symbol"] || classOwnedObject(t, observed["receiver"])["symbol"] != classOwnedObject(t, observeParameters[0])["symbol"] {
		t.Fatalf("transferred this view lost parameter/field resolution: %+v", observed)
	}
	baseConstructor := classOwnedObject(t, base["constructor"])
	baseBody := classOwnedArray(t, baseConstructor["body"])
	if len(baseBody) != 3 || baseConstructor["initializationPhase"] != "before-body" {
		t.Fatalf("complete Base constructor lost: %+v", baseConstructor)
	}
	transfer, virtualTouch := classOwnedExpression(t, baseBody[1]), classOwnedExpression(t, baseBody[2])
	if transfer["kind"] != "method-call" || transfer["dispatch"] != "virtual" || transfer["staticTarget"] != observe["id"] ||
		virtualTouch["kind"] != "method-call" || virtualTouch["dispatch"] != "virtual" || virtualTouch["staticTarget"] != baseTouch["id"] ||
		classOwnedObject(t, transfer["receiver"])["symbol"] != base["symbol"] || classOwnedObject(t, virtualTouch["receiver"])["symbol"] != base["symbol"] {
		t.Fatalf("constructor virtual calls were fixed to an implementation or discarded: %+v %+v", transfer, virtualTouch)
	}
	arguments := classOwnedArray(t, transfer["arguments"])
	if len(arguments) != 1 {
		t.Fatalf("constructor this argument missing: %+v", transfer)
	}
	self := classOwnedObject(t, arguments[0])
	if self["kind"] != "this" || self["symbol"] != base["symbol"] || self["type"] != base["thisType"] {
		t.Fatalf("constructor this transfer lost the same reference: %+v", self)
	}
	witnesses := classOwnedArray(t, transfer["argumentCompatibility"])
	if len(witnesses) != 1 {
		t.Fatalf("official argument compatibility witness missing: %+v", transfer)
	}
	witness := classOwnedObject(t, witnesses[0])
	if witness["sourceType"] != base["thisType"] || witness["targetType"] != base["instanceType"] || witness["assignable"] != true {
		t.Fatalf("constructor this-to-Base official compatibility differs: %+v", witness)
	}
	projection := classOwnedArray(t, witness["members"])
	if len(projection) != 3 {
		t.Fatalf("complete Base member projection differs: %+v", projection)
	}
	for _, member := range []any{value["symbol"], observe["symbol"], baseTouch["symbol"]} {
		found := false
		for _, item := range projection {
			edge := classOwnedObject(t, item)
			if edge["target"] == member && edge["source"] == member {
				found = true
			}
		}
		if !found {
			t.Fatalf("official this-to-Base member projection missing %v: %+v", member, projection)
		}
	}
	derivedConstructor := classOwnedObject(t, derived["constructor"])
	derivedBody := classOwnedArray(t, derivedConstructor["body"])
	if len(derivedBody) != 2 {
		t.Fatalf("complete Derived constructor lost: %+v", derivedConstructor)
	}
	super := classOwnedExpression(t, derivedBody[0])
	origin := classOwnedObject(t, super["origin"])
	start := strings.Index(classInheritanceSource, "super(seed)")
	if super["kind"] != "super-construct" || super["dispatch"] != "direct-base-constructor" ||
		super["staticTarget"] != baseConstructor["id"] || super["id"] == "" || super["id"] == nil ||
		origin["source"] != "/original/main.ts" || origin["start"] != float64(start) || origin["end"] != float64(start+len("super(seed)")) ||
		derivedConstructor["superCall"] != super["id"] || derivedConstructor["initializationPhase"] != "after-successful-super" {
		t.Fatalf("explicit super target/anchor and initialization boundary missing: super=%+v ctor=%+v", super, derivedConstructor)
	}
	initializers := classOwnedArray(t, derivedConstructor["initializationFields"])
	if len(initializers) != 1 || initializers[0] != extra["id"] {
		t.Fatalf("Derived own initializers differ: %+v", initializers)
	}
	store := classOwnedExpression(t, derivedBody[1])
	call := classOwnedObject(t, store["right"])
	if classOwnedObject(t, store["left"])["symbol"] != extra["symbol"] || call["dispatch"] != "virtual" || call["staticTarget"] != derivedTouch["id"] {
		t.Fatalf("Derived constructor did not keep own field/virtual static target: %+v", store)
	}
	direct := classOwnedNamed(t, derived["methods"], "baseTouch")
	directCall := classOwnedReturnedExpression(t, direct)
	if directCall["kind"] != "method-call" || directCall["dispatch"] != "direct-base-method" || directCall["staticTarget"] != baseTouch["id"] ||
		classOwnedObject(t, directCall["receiver"])["kind"] != "super" {
		t.Fatalf("super.touch was not a direct base call: %+v", directCall)
	}
}
