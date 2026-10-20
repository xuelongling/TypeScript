// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"reflect"
	"testing"

	"github.com/microsoft/typescript-go/internal/json"
)

const classDefaultMiddleSource = "export class Base { value: number = 1; constructor(seed: number) { this.value = seed; } } " +
	"export class Middle extends Base { extra: number = 2; } " +
	"export class Leaf extends Middle { constructor(seed: number) { super(seed); } make(seed: number): Middle { return new Middle(seed); } }"

// The public seam builds a fresh official Program and returns only owned facts.
func classDefaultConstructView(t *testing.T, source string) map[string]any {
	t.Helper()
	graph := classSourceForTest(t, source)
	data, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}
	var view map[string]any
	if err := json.Unmarshal(data, &view); err != nil {
		t.Fatal(err)
	}
	return view
}

func classDefaultConstructSelection(t *testing.T, expression, target map[string]any, super bool) {
	t.Helper()
	constructor := classOwnedObject(t, target["constructor"])
	kind, dispatch, result := "construct", "direct-constructor", target["instanceType"]
	if super {
		kind, dispatch, result = "super-construct", "direct-base-constructor", "t:void"
	}
	if expression["kind"] != kind || expression["dispatch"] != dispatch ||
		expression["staticTarget"] != constructor["id"] || expression["type"] != result ||
		!reflect.DeepEqual(expression["resolvedSignature"], constructor["signature"]) {
		t.Fatalf("default constructor lost the direct logical target/signature: expression=%+v constructor=%+v", expression, constructor)
	}
	receiver := classOwnedObject(t, expression["receiver"])
	receiverKind := "reference"
	if super {
		receiverKind = "super"
	}
	if receiver["kind"] != receiverKind || receiver["symbol"] != target["symbol"] || receiver["type"] != target["constructorType"] {
		t.Fatalf("default constructor lost its real constructor receiver: %+v", receiver)
	}
}

func TestOwnedClassSourceDefaultConstructOwnsImplicitRootAllocation(t *testing.T) {
	t.Parallel()
	view := classDefaultConstructView(t, "export class Root { value: number = 1; } export class Factory { fresh: Root = new Root(); }")
	classes := classOwnedArray(t, view["classes"])
	root, factory := classOwnedNamed(t, classes, "Root"), classOwnedNamed(t, classes, "Factory")
	constructor := classOwnedObject(t, root["constructor"])
	if constructor["kind"] != "implicit-base" || constructor["implicitForward"] != nil ||
		len(classOwnedArray(t, constructor["body"])) != 0 || constructor["initializationPhase"] != "before-body" {
		t.Fatalf("root default constructor fabricated a body or forwarding: %+v", constructor)
	}
	fresh := classOwnedNamed(t, factory["fields"], "fresh")
	allocation := classOwnedObject(t, fresh["initializer"])
	classDefaultConstructSelection(t, allocation, root, false)
	if len(classOwnedArray(t, allocation["arguments"])) != 0 || len(classOwnedArray(t, allocation["argumentCompatibility"])) != 0 {
		t.Fatalf("zero-argument allocation invented parameters: %+v", allocation)
	}
	field := classOwnedNamed(t, root["fields"], "value")
	classReadonlyWitness(t, fresh["initializerCompatibility"], root["instanceType"], root["instanceType"], field["symbol"], field["symbol"])
}

func TestOwnedClassSourceDefaultConstructOwnsImplicitRootSuper(t *testing.T) {
	t.Parallel()
	view := classDefaultConstructView(t, "export class Root { value: number = 1; } export class Leaf extends Root { constructor() { super(); } }")
	classes := classOwnedArray(t, view["classes"])
	root, leaf := classOwnedNamed(t, classes, "Root"), classOwnedNamed(t, classes, "Leaf")
	constructor := classOwnedObject(t, leaf["constructor"])
	body := classOwnedArray(t, constructor["body"])
	if len(body) != 1 {
		t.Fatalf("explicit default-base super body was lost: %+v", constructor)
	}
	call := classOwnedExpression(t, body[0])
	classDefaultConstructSelection(t, call, root, true)
	if constructor["superCall"] != call["id"] || constructor["initializationPhase"] != "after-successful-super" ||
		len(classOwnedArray(t, call["arguments"])) != 0 || len(classOwnedArray(t, call["argumentCompatibility"])) != 0 {
		t.Fatalf("implicit-base super lost its successful direct initialization boundary: %+v", constructor)
	}
}

func TestOwnedClassSourceDefaultConstructOwnsImplicitMiddleSuper(t *testing.T) {
	t.Parallel()
	view := classDefaultConstructView(t, classDefaultMiddleSource)
	classes := classOwnedArray(t, view["classes"])
	base, middle, leaf := classOwnedNamed(t, classes, "Base"), classOwnedNamed(t, classes, "Middle"), classOwnedNamed(t, classes, "Leaf")
	baseConstructor, middleConstructor, leafConstructor := classOwnedObject(t, base["constructor"]), classOwnedObject(t, middle["constructor"]), classOwnedObject(t, leaf["constructor"])
	baseParameter := classOwnedObject(t, classOwnedArray(t, classOwnedObject(t, baseConstructor["signature"])["parameters"])[0])
	middleParameter := classOwnedObject(t, classOwnedArray(t, classOwnedObject(t, middleConstructor["signature"])["parameters"])[0])
	leafParameter := classOwnedObject(t, classOwnedArray(t, classOwnedObject(t, leafConstructor["signature"])["parameters"])[0])
	if middleConstructor["kind"] != "implicit-derived" || middleParameter["symbol"] == baseParameter["symbol"] ||
		middleParameter["inheritedFrom"] != baseParameter["symbol"] || !reflect.DeepEqual(middleParameter["origin"], baseParameter["origin"]) {
		t.Fatalf("Middle lost its distinct logical parameter and real Base provenance: %+v", middleConstructor)
	}
	forward := classOwnedObject(t, middleConstructor["implicitForward"])
	if forward["target"] != baseConstructor["id"] || !reflect.DeepEqual(forward["baseSignature"], baseConstructor["signature"]) ||
		!reflect.DeepEqual(forward["origin"], middle["origin"]) {
		t.Fatalf("Middle forwarding skipped its direct Base signature/origin: %+v", forward)
	}
	mapping := classOwnedArray(t, forward["parameters"])
	if len(mapping) != 1 {
		t.Fatalf("Middle forwarding parameter closure differs: %+v", forward)
	}
	edge := classOwnedObject(t, mapping[0])
	witness := classOwnedObject(t, edge["compatibility"])
	if edge["source"] != middleParameter["symbol"] || edge["target"] != baseParameter["symbol"] ||
		witness["assignable"] != true || witness["sourceType"] != middleParameter["type"] ||
		witness["targetType"] != baseParameter["type"] || len(classOwnedArray(t, witness["members"])) != 0 {
		t.Fatalf("Middle logical forwarding lost its official type witness: %+v", edge)
	}
	baseStore := classOwnedExpression(t, classOwnedArray(t, baseConstructor["body"])[0])
	if classOwnedObject(t, baseStore["right"])["symbol"] != baseParameter["symbol"] {
		t.Fatal("Middle forwarding rebound Base's source parameter")
	}
	body := classOwnedArray(t, leafConstructor["body"])
	if len(body) != 1 {
		t.Fatalf("Leaf's complete constructor body differs: %+v", leafConstructor)
	}
	call := classOwnedExpression(t, body[0])
	classDefaultConstructSelection(t, call, middle, true)
	if call["staticTarget"] == baseConstructor["id"] || leafConstructor["superCall"] != call["id"] ||
		leafConstructor["initializationPhase"] != "after-successful-super" {
		t.Fatalf("Leaf super skipped Middle's logical constructor boundary: %+v", leafConstructor)
	}
	argument := classOwnedObject(t, classOwnedArray(t, call["arguments"])[0])
	argumentWitness := classOwnedObject(t, classOwnedArray(t, call["argumentCompatibility"])[0])
	if argument["symbol"] != leafParameter["symbol"] || argumentWitness["assignable"] != true ||
		argumentWitness["sourceType"] != argument["type"] || argumentWitness["targetType"] != middleParameter["type"] ||
		len(classOwnedArray(t, argumentWitness["members"])) != 0 {
		t.Fatalf("Leaf super confused caller/inherited logical bindings: %+v", call)
	}
	makeMethod := classOwnedNamed(t, leaf["methods"], "make")
	allocation := classOwnedReturnedExpression(t, makeMethod)
	classDefaultConstructSelection(t, allocation, middle, false)
	makeParameter := classOwnedObject(t, classOwnedArray(t, classOwnedObject(t, makeMethod["signature"])["parameters"])[0])
	if classOwnedObject(t, classOwnedArray(t, allocation["arguments"])[0])["symbol"] != makeParameter["symbol"] {
		t.Fatalf("new Middle lost its actual caller parameter: %+v", allocation)
	}
}

func TestOwnedClassSourceDefaultConstructRejectsUnsupportedShapesWithoutIdentityFailure(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		"export class Root { constructor(seed: number = 1) {} } export class Factory { fresh: Root = new Root(); }",
		"export class Root {} export class Factory { fresh: Root = new (Root)(); }",
	} {
		graph, err := classIdentityExport(t, source, nil)
		var failure *classFailure
		if !errors.As(err, &failure) || failure.Kind != "unsupported" || !reflect.DeepEqual(graph, classGraph{}) {
			t.Fatalf("legal unsupported construction must have a capability diagnostic and zero graph: %v", err)
		}
	}
}
