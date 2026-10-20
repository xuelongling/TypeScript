// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/json"
)

const classLiteralFactsSource = `export class Literals {
  integer: number = 1;
  decimal: number = 1.0;
  exponent: number = 1e0;
  hexadecimal: number = 0x1;
  hexadecimalE: number = 0xDEAD;
  exactLarge: number = 9007199254740992;
  roundedLarge: number = 9007199254740993;
}`

// Official TS narrow identity may share values that have different source
// tokens and R01 defaults. These facts remain unresolved: no settled value,
// range qualification or RNE result is inferred from the official display.
func TestOwnedClassSourceLiteralFactsPreserveOccurrencesDefaultsAndContext(t *testing.T) {
	t.Parallel()
	graph := classSourceForTest(t, classLiteralFactsSource)
	data, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}
	var view map[string]any
	if err := json.Unmarshal(data, &view); err != nil {
		t.Fatal(err)
	}
	classes := classOwnedArray(t, view["classes"])
	if len(classes) != 1 {
		t.Fatalf("complete literal class closure differs: %+v", classes)
	}
	owner := classOwnedNamed(t, classes, "Literals")
	fields := classOwnedArray(t, owner["fields"])
	if len(fields) != 7 || len(classOwnedArray(t, owner["methods"])) != 0 {
		t.Fatalf("complete literal fields differ: %+v", owner)
	}
	types := map[any]map[string]any{}
	for _, item := range classOwnedArray(t, view["types"]) {
		record := classOwnedObject(t, item)
		types[record["id"]] = record
	}
	seen := map[any]bool{}
	var oneType any
	for index, expected := range []struct{ name, token, defaultScalar string }{
		{"integer", "1", "i32"},
		{"decimal", "1.0", "f64"},
		{"exponent", "1e0", "f64"},
		{"hexadecimal", "0x1", "i32"},
		{"hexadecimalE", "0xDEAD", "i32"},
		{"exactLarge", "9007199254740992", "i32"},
		{"roundedLarge", "9007199254740993", "i32"},
	} {
		field := classOwnedNamed(t, fields, expected.name)
		expression := classOwnedObject(t, field["initializer"])
		literal := classOwnedObject(t, expression["numericLiteral"])
		narrow, context := types[expression["type"]], types[field["type"]]
		if field["ordinal"] != float64(index) || expression["id"] == nil || expression["id"] == "" || seen[expression["id"]] ||
			expression["kind"] != "number" || expression["token"] != expected.token {
			t.Fatalf("literal occurrence/token was lost or merged: %+v", expression)
		}
		seen[expression["id"]] = true
		if narrow == nil || narrow["kind"] != "number-literal" || narrow["scalar"] != "" ||
			context == nil || context["kind"] != "number" || context["scalar"] != "f64" ||
			literal["state"] != "unresolved" || literal["officialType"] != expression["type"] ||
			literal["defaultScalar"] != expected.defaultScalar || literal["contextType"] != field["type"] ||
			expression["type"] == field["type"] {
			t.Fatalf("literal narrow/default/actual slot context differs: expression=%+v narrow=%+v context=%+v", expression, narrow, context)
		}
		if index < 4 {
			if index == 0 {
				oneType = expression["type"]
			}
			if expression["type"] != oneType || narrow["literal"] != "1" {
				t.Fatalf("the same official narrow value lost its shared identity: %+v", narrow)
			}
		}
		origin := classOwnedObject(t, expression["origin"])
		start := strings.Index(classLiteralFactsSource, "= "+expected.token+";") + len("= ")
		if origin["source"] != "/original/main.ts" || origin["start"] != float64(start) ||
			origin["end"] != float64(start+len(expected.token)) {
			t.Fatalf("exact literal occurrence origin differs: token=%s origin=%+v", expected.token, origin)
		}
		witness := classOwnedObject(t, field["initializerCompatibility"])
		witnessOrigin := classOwnedObject(t, witness["origin"])
		if witness["assignable"] != true || witness["sourceType"] != expression["type"] ||
			witness["targetType"] != field["type"] || len(classOwnedArray(t, witness["members"])) != 0 ||
			witnessOrigin["source"] != origin["source"] || witnessOrigin["start"] != origin["start"] || witnessOrigin["end"] != origin["end"] {
			t.Fatalf("literal initializer lost its actual narrow-to-slot official witness: %+v", witness)
		}
	}
	if len(seen) != 7 {
		t.Fatal("large or equal-valued literal occurrences were merged")
	}
}
