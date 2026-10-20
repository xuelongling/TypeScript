// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/json"
)

const classConstructSource = classCounterSource + `
export class Factory {
  fresh: Counter = new Counter(5);
  make(seed: number): Counter { return new Counter(seed); }
}`

// Candidate construct expressions retain the actual constructor value and the
// official selected declaration/signature. Allocation is a distinct operation:
// field initialization and typed return retain their own compatibility witnesses.
func TestOwnedClassSourceConstructOwnsResolvedTargetAndArgumentWitnesses(t *testing.T) {
	t.Parallel()
	graph := classSourceForTest(t, classConstructSource)
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
		t.Fatalf("complete Counter/Factory closure differs: %+v", classes)
	}
	counter, factory := classOwnedNamed(t, classes, "Counter"), classOwnedNamed(t, classes, "Factory")
	counterFields, factoryFields := classOwnedArray(t, counter["fields"]), classOwnedArray(t, factory["fields"])
	if len(counterFields) != 1 || len(factoryFields) != 1 || len(classOwnedArray(t, factory["methods"])) != 1 {
		t.Fatalf("construction fixture lost own fields/methods: %+v %+v", counter, factory)
	}
	value, fresh := classOwnedObject(t, counterFields[0]), classOwnedObject(t, factoryFields[0])
	constructor := classOwnedObject(t, counter["constructor"])
	signature := classOwnedObject(t, constructor["signature"])
	constructorParameters := classOwnedArray(t, signature["parameters"])
	if len(constructorParameters) != 1 || signature["return"] != counter["instanceType"] {
		t.Fatalf("actual Counter constructor signature missing: %+v", signature)
	}
	parameter := classOwnedObject(t, constructorParameters[0])
	makeMethod := classOwnedNamed(t, factory["methods"], "make")
	makeParameters := classOwnedArray(t, classOwnedObject(t, makeMethod["signature"])["parameters"])
	if len(makeParameters) != 1 {
		t.Fatalf("Factory.make seed binding missing: %+v", makeMethod)
	}
	makeParameter := classOwnedObject(t, makeParameters[0])
	makeBody := classOwnedArray(t, makeMethod["body"])
	if len(makeBody) != 1 || classOwnedObject(t, makeBody[0])["kind"] != "return" {
		t.Fatalf("complete Factory.make return missing: %+v", makeMethod)
	}
	initialized, returned := classOwnedObject(t, fresh["initializer"]), classOwnedExpression(t, makeBody[0])
	if fresh["name"] != "fresh" || fresh["type"] != counter["instanceType"] ||
		initialized["id"] == "" || initialized["id"] == nil || returned["id"] == "" || returned["id"] == nil ||
		initialized["id"] == returned["id"] {
		t.Fatalf("distinct construct occurrences/field type were lost: %+v %+v", fresh, returned)
	}
	for index, expression := range []map[string]any{initialized, returned} {
		if expression["kind"] != "construct" || expression["dispatch"] != "direct-constructor" ||
			expression["staticTarget"] != constructor["id"] || expression["type"] != counter["instanceType"] {
			t.Fatalf("actual constructor target/allocation result missing: %+v", expression)
		}
		receiver := classOwnedObject(t, expression["receiver"])
		if receiver["kind"] != "reference" || receiver["symbol"] != counter["symbol"] || receiver["type"] != counter["constructorType"] {
			t.Fatalf("construct lost its official constructor-value receiver: %+v", expression)
		}
		resolved := classOwnedObject(t, expression["resolvedSignature"])
		parameters := classOwnedArray(t, resolved["parameters"])
		if resolved["return"] != counter["instanceType"] || len(parameters) != 1 ||
			classOwnedObject(t, parameters[0])["symbol"] != parameter["symbol"] || classOwnedObject(t, parameters[0])["type"] != "t:number" {
			t.Fatalf("official selected constructor signature differs: %+v", resolved)
		}
		arguments, witnesses := classOwnedArray(t, expression["arguments"]), classOwnedArray(t, expression["argumentCompatibility"])
		if len(arguments) != 1 || len(witnesses) != 1 {
			t.Fatalf("construct argument/official witness missing: %+v", expression)
		}
		argument, witness := classOwnedObject(t, arguments[0]), classOwnedObject(t, witnesses[0])
		token := "new Counter(5)"
		if index == 0 {
			if argument["kind"] != "number" || argument["token"] != "5" {
				t.Fatalf("fresh construction lost its real numeric argument: %+v", argument)
			}
			// The literal retains its official narrow identity, independently of
			// declared number=f64 and its pending R01 contextual interpretation.
			literal := classOwnedObject(t, argument["numericLiteral"])
			var narrow map[string]any
			for _, item := range classOwnedArray(t, view["types"]) {
				record := classOwnedObject(t, item)
				if record["id"] == argument["type"] {
					narrow = record
				}
			}
			if narrow == nil || narrow["kind"] != "number-literal" || narrow["scalar"] != "" || narrow["literal"] != "5" ||
				literal["officialType"] != argument["type"] || literal["state"] != "unresolved" ||
				literal["defaultScalar"] != "i32" || literal["contextType"] != parameter["type"] {
				t.Fatalf("numeric argument lost its untyped narrow identity or actual context: type=%+v literal=%+v", narrow, literal)
			}
		} else {
			token = "new Counter(seed)"
			if argument["kind"] != "reference" || argument["symbol"] != makeParameter["symbol"] {
				t.Fatalf("make construction confused caller/callee seed bindings: %+v", argument)
			}
		}
		if witness["assignable"] != true || witness["sourceType"] != argument["type"] || witness["targetType"] != parameter["type"] ||
			len(classOwnedArray(t, witness["members"])) != 0 {
			t.Fatalf("constructor argument relation differs from official scalar types: %+v", witness)
		}
		origin := classOwnedObject(t, expression["origin"])
		start := strings.Index(classConstructSource, token)
		if origin["source"] != "/original/main.ts" || origin["start"] != float64(start) || origin["end"] != float64(start+len(token)) {
			t.Fatalf("construct original expression anchor differs: %+v", origin)
		}
	}
	classReadonlyWitness(t, fresh["initializerCompatibility"], counter["instanceType"], counter["instanceType"], value["symbol"], value["symbol"])
	classReadonlyWitness(t, classOwnedObject(t, makeBody[0])["compatibility"], counter["instanceType"], counter["instanceType"], value["symbol"], value["symbol"])
	factoryConstructor := classOwnedObject(t, factory["constructor"])
	initializers := classOwnedArray(t, factoryConstructor["initializationFields"])
	if factoryConstructor["kind"] != "implicit-base" || factoryConstructor["initializationPhase"] != "before-body" ||
		len(initializers) != 1 || initializers[0] != fresh["id"] {
		t.Fatalf("Factory own allocation initializer ordering was lost: %+v", factoryConstructor)
	}
}
