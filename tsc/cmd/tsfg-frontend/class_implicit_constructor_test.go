// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/json"
)

const classImplicitConstructorSource = `export class Base {
  value: number = 1;
  constructor(seed: number) { this.value = seed; }
}
export class Derived extends Base {
  extra: number = 2;
}`

// Candidate implicit construction is a separate owned fact, not fabricated AST:
// inheritedFrom identifies the actual base parameter; implicitForward owns the
// direct base target/signature and derived-parameter to base-parameter mapping.
// Its origin is the real Derived declaration, and its body has no fake super.
func TestOwnedClassSourceImplicitDerivedConstructorOwnsForwardingWithoutRebindingBase(t *testing.T) {
	t.Parallel()
	graph := classSourceForTest(t, classImplicitConstructorSource)
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
		t.Fatalf("complete implicit-constructor closure differs: %+v", classes)
	}
	base, derived := classOwnedNamed(t, classes, "Base"), classOwnedNamed(t, classes, "Derived")
	if derived["base"] != base["id"] {
		t.Fatalf("official base edge missing: %+v", derived)
	}
	baseConstructor, derivedConstructor := classOwnedObject(t, base["constructor"]), classOwnedObject(t, derived["constructor"])
	baseSignature, derivedSignature := classOwnedObject(t, baseConstructor["signature"]), classOwnedObject(t, derivedConstructor["signature"])
	baseParameters, derivedParameters := classOwnedArray(t, baseSignature["parameters"]), classOwnedArray(t, derivedSignature["parameters"])
	if len(baseParameters) != 1 || len(derivedParameters) != 1 || baseSignature["return"] != base["instanceType"] || derivedSignature["return"] != derived["instanceType"] {
		t.Fatalf("official inherited constructor signatures differ: base=%+v derived=%+v", baseSignature, derivedSignature)
	}
	baseParameter, derivedParameter := classOwnedObject(t, baseParameters[0]), classOwnedObject(t, derivedParameters[0])
	if baseParameter["type"] != "t:number" || derivedParameter["type"] != "t:number" ||
		baseParameter["symbol"] == derivedParameter["symbol"] || derivedParameter["symbol"] == "" ||
		derivedParameter["inheritedFrom"] != baseParameter["symbol"] {
		t.Fatalf("implicit derived parameter reused/replaced the base binding: base=%+v derived=%+v", baseParameter, derivedParameter)
	}
	found := false
	for _, item := range classOwnedArray(t, view["symbols"]) {
		symbol := classOwnedObject(t, item)
		if symbol["id"] == derivedParameter["symbol"] {
			found = true
			if symbol["kind"] != "implicit-parameter" || symbol["name"] != "seed" || symbol["type"] != "t:number" {
				t.Fatalf("implicit logical parameter lost official provenance: %+v", symbol)
			}
		}
	}
	if !found {
		t.Fatal("implicit derived logical parameter is not closed in the owned symbol table")
	}
	baseBody := classOwnedArray(t, baseConstructor["body"])
	if len(baseBody) != 1 {
		t.Fatalf("complete Base constructor lost: %+v", baseConstructor)
	}
	store := classOwnedExpression(t, baseBody[0])
	if classOwnedObject(t, store["right"])["symbol"] != baseParameter["symbol"] {
		t.Fatalf("registering Derived changed the real Base seed binding: %+v", store)
	}
	if derivedConstructor["kind"] != "implicit-derived" || len(classOwnedArray(t, derivedConstructor["body"])) != 0 ||
		derivedConstructor["superCall"] != "" || derivedConstructor["initializationPhase"] != "after-successful-super" {
		t.Fatalf("implicit constructor fabricated source body/super or lost initialization ordering: %+v", derivedConstructor)
	}
	forward := classOwnedObject(t, derivedConstructor["implicitForward"])
	if forward["target"] != baseConstructor["id"] {
		t.Fatalf("implicit forward did not retain the actual direct base target: %+v", forward)
	}
	origin, declaration := classOwnedObject(t, forward["origin"]), classOwnedObject(t, derived["origin"])
	if origin["source"] != declaration["source"] || origin["start"] != declaration["start"] || origin["end"] != declaration["end"] {
		t.Fatalf("implicit forwarding does not point to the real Derived declaration: %+v", origin)
	}
	forwardSignature := classOwnedObject(t, forward["baseSignature"])
	forwardParameters := classOwnedArray(t, forwardSignature["parameters"])
	if forwardSignature["return"] != base["instanceType"] || len(forwardParameters) != 1 || classOwnedObject(t, forwardParameters[0])["symbol"] != baseParameter["symbol"] {
		t.Fatalf("actual official base signature missing from forwarding facts: %+v", forwardSignature)
	}
	mapping := classOwnedArray(t, forward["parameters"])
	if len(mapping) != 1 {
		t.Fatalf("implicit forwarding argument map differs: %+v", mapping)
	}
	edge := classOwnedObject(t, mapping[0])
	if edge["source"] != derivedParameter["symbol"] || edge["target"] != baseParameter["symbol"] {
		t.Fatalf("implicit forwarding confused derived/base parameter identities: %+v", edge)
	}
	witness := classOwnedObject(t, edge["compatibility"])
	if witness["assignable"] != true || witness["sourceType"] != "t:number" || witness["targetType"] != "t:number" || len(classOwnedArray(t, witness["members"])) != 0 {
		t.Fatalf("implicit forwarding lacks the official parameter type witness: %+v", witness)
	}
	extra := classOwnedNamed(t, derived["fields"], "extra")
	initializers := classOwnedArray(t, derivedConstructor["initializationFields"])
	if len(initializers) != 1 || initializers[0] != extra["id"] || classOwnedObject(t, extra["initializer"])["token"] != "2" {
		t.Fatalf("Derived own initialization did not follow the base forwarding boundary: %+v", derivedConstructor)
	}
}
