// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/json"
)

const classReadonlySource = `export class ReadView { readonly value: number = 0; }
export class Mutable {
  padding: boolean = true;
  value: number = 1;
  view: ReadView = this;
  asView(): ReadView { return this; }
}
export class Holder {
  view: ReadView;
  constructor(source: Mutable) { this.view = source; source.value = 2; }
  accept(value: ReadView): ReadView { return value; }
  transfer(source: Mutable): ReadView { return this.accept(source); }
  read(): number { return this.view.value; }
}`

func classReadonlyWitness(t *testing.T, value any, sourceType, targetType, sourceMember, targetMember any) {
	t.Helper()
	witness := classOwnedObject(t, value)
	if witness["assignable"] != true || witness["sourceType"] != sourceType || witness["targetType"] != targetType {
		t.Fatalf("owned official reference conversion differs: %+v", witness)
	}
	members := classOwnedArray(t, witness["members"])
	if len(members) != 1 {
		t.Fatalf("complete ReadView projection must have exactly one member: %+v", witness)
	}
	edge := classOwnedObject(t, members[0])
	if edge["source"] != sourceMember || edge["target"] != targetMember {
		t.Fatalf("structural view used a name/offset instead of the actual official member symbols: %+v", edge)
	}
	if classOwnedObject(t, witness["origin"])["source"] != "/original/main.ts" {
		t.Fatalf("conversion lost its original source: %+v", witness)
	}
}

func TestOwnedClassSourceOwnsStructuralReadonlyConversionsAndAliasPermissions(t *testing.T) {
	t.Parallel()
	graph := classSourceForTest(t, classReadonlySource)
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
		t.Fatalf("complete ordinary class closure differs: %+v", classes)
	}
	readView, mutable, holder := classOwnedNamed(t, classes, "ReadView"), classOwnedNamed(t, classes, "Mutable"), classOwnedNamed(t, classes, "Holder")
	if readView["runtimeKey"] == mutable["runtimeKey"] || readView["instanceType"] == mutable["instanceType"] {
		t.Fatal("the structural compatibility fixture did not retain two distinct actual classes")
	}
	target := classOwnedNamed(t, readView["fields"], "value")
	actual := classOwnedNamed(t, mutable["fields"], "value")
	if target["ordinal"] != float64(0) || actual["ordinal"] != float64(1) || target["readonly"] != true || actual["readonly"] != false {
		t.Fatalf("fixture must project readonly ordinal0 onto mutable ordinal1: %+v %+v", target, actual)
	}
	alias := classOwnedNamed(t, mutable["fields"], "view")
	initializer := classOwnedObject(t, alias["initializer"])
	if initializer["kind"] != "this" || initializer["symbol"] != mutable["symbol"] {
		t.Fatalf("field initializer replaced the same reference: %+v", initializer)
	}
	// Candidate keys: a field owns initializerCompatibility, a typed return
	// statement owns compatibility, and a reference assignment expression owns
	// compatibility. Each is an official relation plus actual member projections.
	classReadonlyWitness(t, alias["initializerCompatibility"], mutable["thisType"], readView["instanceType"], actual["symbol"], target["symbol"])
	asView := classOwnedNamed(t, mutable["methods"], "asView")
	asViewBody := classOwnedArray(t, asView["body"])
	if len(asViewBody) != 1 || classOwnedObject(t, asViewBody[0])["kind"] != "return" {
		t.Fatalf("complete asView return missing: %+v", asView)
	}
	returned := classOwnedExpression(t, asViewBody[0])
	if returned["kind"] != "this" || returned["symbol"] != mutable["symbol"] {
		t.Fatalf("return copied/replaced the same reference: %+v", returned)
	}
	classReadonlyWitness(t, classOwnedObject(t, asViewBody[0])["compatibility"], mutable["thisType"], readView["instanceType"], actual["symbol"], target["symbol"])
	constructor := classOwnedObject(t, holder["constructor"])
	parameters := classOwnedArray(t, classOwnedObject(t, constructor["signature"])["parameters"])
	body := classOwnedArray(t, constructor["body"])
	if len(parameters) != 1 || len(body) != 2 {
		t.Fatalf("complete Holder constructor missing: %+v", constructor)
	}
	parameter := classOwnedObject(t, parameters[0])
	store, mutate := classOwnedExpression(t, body[0]), classOwnedExpression(t, body[1])
	stored := classOwnedObject(t, store["right"])
	if stored["kind"] != "reference" || stored["symbol"] != parameter["symbol"] || stored["type"] != mutable["instanceType"] {
		t.Fatalf("assignment replaced the incoming object identity: %+v", stored)
	}
	classReadonlyWitness(t, store["compatibility"], mutable["instanceType"], readView["instanceType"], actual["symbol"], target["symbol"])
	write := classOwnedObject(t, mutate["left"])
	permission := classOwnedObject(t, write["permission"])
	if write["symbol"] != actual["symbol"] || classOwnedObject(t, write["receiver"])["symbol"] != parameter["symbol"] ||
		permission["writable"] != true || permission["readonly"] != false || permission["writeUse"] != true {
		t.Fatalf("a readonly view incorrectly froze the writable alias: %+v", write)
	}
	transfer := classOwnedNamed(t, holder["methods"], "transfer")
	transferBody := classOwnedArray(t, transfer["body"])
	if len(transferBody) != 1 {
		t.Fatalf("complete transfer body missing: %+v", transfer)
	}
	call := classOwnedExpression(t, transferBody[0])
	arguments, witnesses := classOwnedArray(t, call["arguments"]), classOwnedArray(t, call["argumentCompatibility"])
	if call["kind"] != "method-call" || len(arguments) != 1 || len(witnesses) != 1 {
		t.Fatalf("actual checked call/argument lost: %+v", call)
	}
	classReadonlyWitness(t, witnesses[0], mutable["instanceType"], readView["instanceType"], actual["symbol"], target["symbol"])
	classReadonlyWitness(t, classOwnedObject(t, transferBody[0])["compatibility"], readView["instanceType"], readView["instanceType"], target["symbol"], target["symbol"])
	read := classOwnedReturnedExpression(t, classOwnedNamed(t, holder["methods"], "read"))
	readPermission := classOwnedObject(t, read["permission"])
	if read["symbol"] != target["symbol"] || readPermission["readable"] != true || readPermission["readonly"] != true ||
		readPermission["writable"] != false || readPermission["writeUse"] != false {
		t.Fatalf("readonly static view permission was lost: %+v", read)
	}
}

func TestOwnedClassSourcePreservesOfficialAccessAndPrivateOriginFailures(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, source, token string
		code                int32
		exact               bool
	}{
		{"readonly", "export class R { readonly value: number = 0; write(other: R): void { other.value = 1; } }", "other.value", 2540, true},
		{"private-access", "export class Hidden { private value: number = 0; } export class Other { read(other: Hidden): number { return other.value; } }", "other.value", 2341, true},
		{"protected-access", "export class Hidden { protected value: number = 0; } export class Other { read(other: Hidden): number { return other.value; } }", "other.value", 2445, true},
		{"private-origin", "export class Left { private value: number = 0; } export class Right { private value: number = 0; } export class Other { from(other: Left): Right { return other; } }", "return other;", 2322, false},
		{"protected-origin", "export class Left { protected value: number = 0; } export class Right { protected value: number = 0; } export class Other { from(other: Left): Right { return other; } }", "return other;", 2322, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			// The non-BMP prefix distinguishes original UTF16 source positions
			// from UTF8 byte offsets in every official negative case.
			source := "// 😀\n" + test.source
			_, err := classIdentityExport(t, source, nil)
			var failure *classFailure
			if !errors.As(err, &failure) || failure.Kind != "official" || failure.Code != test.code {
				t.Fatalf("official %s diagnostic identity was replaced: %v", test.name, err)
			}
			byteStart := strings.LastIndex(source, test.token)
			byteEnd := byteStart + len(test.token)
			if test.exact {
				byteStart += len("other.")
			}
			start, end := len(utf16.Encode([]rune(source[:byteStart]))), len(utf16.Encode([]rune(source[:byteEnd])))
			if failure.Origin.Source != "/original/main.ts" || failure.Origin.Start < start || failure.Origin.End > end || failure.Origin.Start >= failure.Origin.End ||
				test.exact && (failure.Origin.Start != start || failure.Origin.End != end) {
				t.Fatalf("official %s UTF16 source anchor differs: got %+v, expected within %d-%d", test.name, failure.Origin, start, end)
			}
		})
	}
}
