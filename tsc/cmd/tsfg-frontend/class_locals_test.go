// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/json"
)

const classLocalSourceStart = `// 😀
export class Box {
  value: number;
  constructor(seed: number) { this.value = seed; }
`

const classLocalSourceEnd = `  keep(other: Box): Box { const alias: Box = other; return alias; }
  passScalar(seed: number): number { const alias: number = seed; return alias; }
}`

const classLocalConstSource = classLocalSourceStart + `  share(other: Box): Box {
    const alias: Box = other;
    alias.value = this.value;
    return alias;
  }
` + classLocalSourceEnd

const classLocalLetSource = classLocalSourceStart + `  share(other: Box, replacement: Box): Box {
    let alias: Box = other;
    alias = replacement;
    alias.value = this.value;
    return alias;
  }
` + classLocalSourceEnd

func classLocalOrigin(t *testing.T, value any, source, token string, from int) {
	t.Helper()
	origin := classOwnedObject(t, value)
	offset := strings.Index(source[from:], token)
	if offset < 0 {
		t.Fatalf("fixture token missing: %q", token)
	}
	start := from + offset
	end := start + len(token)
	if origin["source"] != "/original/main.ts" ||
		origin["start"] != float64(len(utf16.Encode([]rune(source[:start])))) ||
		origin["end"] != float64(len(utf16.Encode([]rune(source[:end])))) {
		t.Fatalf("local fact lost its exact original UTF16 occurrence: token=%q origin=%+v", token, origin)
	}
}

func classLocalWitness(t *testing.T, value any, sourceType, targetType any, members map[any]bool) {
	t.Helper()
	witness := classOwnedObject(t, value)
	projections := classOwnedArray(t, witness["members"])
	if witness["assignable"] != true || witness["sourceType"] != sourceType || witness["targetType"] != targetType ||
		len(projections) != len(members) {
		t.Fatalf("local relation lost its complete official type/member witness: %+v", witness)
	}
	seen := map[any]bool{}
	for _, item := range projections {
		edge := classOwnedObject(t, item)
		if !members[edge["target"]] || edge["source"] != edge["target"] || seen[edge["target"]] {
			t.Fatalf("local relation guessed/duplicated an official member: %+v", edge)
		}
		seen[edge["target"]] = true
	}
}

// Candidate v5: statement.kind=local owns local:{kind:const|let,bindings:[
// {symbol,type,initializer,compatibility,origin}]}. It has no expression or
// statement compatibility. Local symbols use the real declaration identifier.
// A let rebind is a real expression.kind=local-assignment, not a declaration.
func TestOwnedClassSourceOwnsInitializedLocalsAndAliasIdentity(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ kind, source string }{
		{"const", classLocalConstSource},
		{"let", classLocalLetSource},
	} {
		t.Run(test.kind, func(t *testing.T) {
			t.Parallel()
			graph := classSourceForTest(t, test.source)
			data, err := json.Marshal(graph)
			if err != nil {
				t.Fatal(err)
			}
			var view map[string]any
			if err := json.Unmarshal(data, &view); err != nil {
				t.Fatal(err)
			}
			if view["semanticProfile"] != "tsfg-owned-class-facts-candidate-v5" {
				t.Fatal("new local facts were mixed into the previous candidate profile")
			}
			classes := classOwnedArray(t, view["classes"])
			if len(classes) != 1 {
				t.Fatal("complete Box closure differs")
			}
			box := classOwnedNamed(t, classes, "Box")
			fields, methods := classOwnedArray(t, box["fields"]), classOwnedArray(t, box["methods"])
			if len(fields) != 1 || len(methods) != 3 || classOwnedObject(t, fields[0])["initializer"] != nil {
				t.Fatal("fixture lost its uninitialized declared field or complete typed methods")
			}
			field := classOwnedObject(t, fields[0])
			members := map[any]bool{field["symbol"]: true}
			for _, item := range methods {
				members[classOwnedObject(t, item)["symbol"]] = true
			}
			symbols := map[any]map[string]any{}
			localCount := 0
			for _, item := range classOwnedArray(t, view["symbols"]) {
				symbol := classOwnedObject(t, item)
				if symbol["id"] == nil || symbol["id"] == "" || symbols[symbol["id"]] != nil {
					t.Fatal("owned symbols collided")
				}
				symbols[symbol["id"]] = symbol
				if symbol["kind"] == "local" {
					localCount++
				}
			}
			if localCount != 3 {
				t.Fatalf("expected three independent actual local bindings, got %d", localCount)
			}
			locals := map[any]bool{}
			for _, methodName := range []string{"share", "keep", "passScalar"} {
				method := classOwnedNamed(t, methods, methodName)
				signature := classOwnedObject(t, method["signature"])
				parameters := classOwnedArray(t, signature["parameters"])
				body := classOwnedArray(t, method["body"])
				expectedKind, expectedType, expectedLength := "const", box["instanceType"], 2
				expectedMembers := members
				parameterCount := 1
				if methodName == "share" {
					expectedKind, expectedLength = test.kind, 3
					if test.kind == "let" {
						expectedLength, parameterCount = 4, 2
					}
				} else if methodName == "passScalar" {
					expectedType, expectedMembers = "t:number", nil
				}
				if len(parameters) != parameterCount || len(body) != expectedLength || signature["return"] != expectedType {
					t.Fatalf("complete local method signature/body differs: %s", methodName)
				}
				first := classOwnedObject(t, body[0])
				if first["kind"] != "local" || first["expression"] != nil || first["compatibility"] != nil {
					t.Fatalf("local declaration was dropped or disguised as an expression: %+v", first)
				}
				local := classOwnedObject(t, first["local"])
				bindings := classOwnedArray(t, local["bindings"])
				if local["kind"] != expectedKind || len(bindings) != 1 {
					t.Fatalf("actual const/let declaration kind/order missing: %+v", local)
				}
				binding := classOwnedObject(t, bindings[0])
				symbol := symbols[binding["symbol"]]
				if symbol == nil || symbol["kind"] != "local" || symbol["name"] != "alias" ||
					symbol["type"] != expectedType || binding["type"] != expectedType || locals[binding["symbol"]] {
					t.Fatalf("same-spelled locals lost official symbol/type or method isolation: %+v", binding)
				}
				locals[binding["symbol"]] = true
				parameter := classOwnedObject(t, parameters[0])
				initializer := classOwnedObject(t, binding["initializer"])
				if initializer["kind"] != "reference" || initializer["symbol"] != parameter["symbol"] ||
					initializer["type"] != expectedType || parameter["type"] != expectedType {
					t.Fatalf("local initializer copied/replaced or renamed the incoming reference: %+v", initializer)
				}
				classLocalWitness(t, binding["compatibility"], expectedType, expectedType, expectedMembers)
				from := strings.Index(test.source, methodName+"(")
				declaration, input := "alias: Box = other", "other"
				if methodName == "passScalar" {
					declaration, input = "alias: number = seed", "seed"
				}
				declStart := from + strings.Index(test.source[from:], declaration)
				classLocalOrigin(t, first["origin"], test.source, expectedKind+" "+declaration+";", from)
				classLocalOrigin(t, binding["origin"], test.source, declaration, from)
				classLocalOrigin(t, symbol["declaration"], test.source, "alias", declStart)
				classLocalOrigin(t, initializer["origin"], test.source, input, declStart+strings.Index(declaration, "= ")+2)
				returnedStatement := classOwnedObject(t, body[len(body)-1])
				returned := classOwnedObject(t, returnedStatement["expression"])
				if returnedStatement["kind"] != "return" || returned["kind"] != "reference" ||
					returned["symbol"] != binding["symbol"] || returned["type"] != expectedType {
					t.Fatalf("local return lost the actual alias binding: %+v", returned)
				}
				classLocalWitness(t, returnedStatement["compatibility"], expectedType, expectedType, expectedMembers)
				if methodName != "share" {
					continue
				}
				storeIndex := 1
				if test.kind == "let" {
					rebind := classOwnedExpression(t, body[1])
					left, right := classOwnedObject(t, rebind["left"]), classOwnedObject(t, rebind["right"])
					if rebind["kind"] != "local-assignment" || rebind["operator"] != "=" ||
						left["kind"] != "reference" || left["symbol"] != binding["symbol"] || left["type"] != expectedType ||
						right["kind"] != "reference" || right["symbol"] != classOwnedObject(t, parameters[1])["symbol"] || right["type"] != expectedType {
						t.Fatalf("let rebind confused the binding with object mutation: %+v", rebind)
					}
					classLocalWitness(t, rebind["compatibility"], expectedType, expectedType, members)
					classLocalOrigin(t, rebind["origin"], test.source, "alias = replacement", from)
					storeIndex = 2
				}
				store := classOwnedExpression(t, body[storeIndex])
				left, right := classOwnedObject(t, store["left"]), classOwnedObject(t, store["right"])
				receiver, permission := classOwnedObject(t, left["receiver"]), classOwnedObject(t, left["permission"])
				if store["kind"] != "assignment" || left["kind"] != "field" || left["symbol"] != field["symbol"] ||
					receiver["kind"] != "reference" || receiver["symbol"] != binding["symbol"] || receiver["type"] != expectedType ||
					permission["writable"] != true || permission["writeUse"] != true || permission["readonly"] != false ||
					right["kind"] != "field" || right["symbol"] != field["symbol"] ||
					classOwnedObject(t, right["receiver"])["kind"] != "this" {
					t.Fatalf("const/let alias incorrectly froze the object or replaced its field receiver: %+v", store)
				}
				classLocalWitness(t, store["compatibility"], "t:number", "t:number", nil)
			}
		})
	}
}

func TestOwnedClassSourcePreservesOfficialConstRebindingFailure(t *testing.T) {
	t.Parallel()
	source := strings.Replace(classLocalLetSource, "let alias: Box = other;", "const alias: Box = other;", 1)
	graph, err := classIdentityExport(t, source, nil)
	var failure *classFailure
	if !errors.As(err, &failure) || failure.Kind != "official" || failure.Code != 2588 {
		t.Fatalf("const rebind lost the real official diagnostic: %v", err)
	}
	start := strings.Index(source, "alias = replacement;")
	if failure.Origin.Source != "/original/main.ts" ||
		failure.Origin.Start != len(utf16.Encode([]rune(source[:start]))) ||
		failure.Origin.End != len(utf16.Encode([]rune(source[:start+len("alias")]))) {
		t.Fatal("const rebind lost the exact original UTF16 binding anchor")
	}
	if !reflect.DeepEqual(graph, classGraph{}) {
		t.Fatal("official const rebind failure exposed a partial graph")
	}
}
