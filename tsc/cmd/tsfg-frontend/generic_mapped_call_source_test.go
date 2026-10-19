// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"
)

// Build source syntax only; fresh official checkers determine all result types.
func mappedCallSourceForTest14(keys, index, value string) string {
	leafValue, routeValue := value, value
	if value == "parameter" {
		leafValue, routeValue = "U", "T"
	}
	leaf := "{ [K in " + keys + "]: " + leafValue + " }[" + index + "]"
	route := "{ [K in " + keys + "]: " + routeValue + " }[" + index + "]"
	return "// 😀\nexport function leaf<U>(value: " + leaf + "): " + leaf + " { return value; }\n" +
		"export function route<T>(value: " + route + "): " + route + " { return leaf<T>(value); }"
}

func mappedCallDeclarationOrigin14(source, name string) genericOrigin {
	start := strings.Index(source, "export function "+name+"<")
	end := len(source)
	if next := strings.IndexByte(source[start:], '\n'); next >= 0 {
		end = start + next
	}
	return genericOrigin{Start: len(utf16.Encode([]rune(source[:start]))), End: len(utf16.Encode([]rune(source[:end])))}
}

func TestOwnedGenericMappedCallRejectsUnsupportedOfficialBodies(t *testing.T) {
	t.Parallel()
	base := mappedCallSourceForTest14("'value'", "'value'", "parameter")
	leafLine := strings.Split(base, "\n")[1]
	uniform := mappedCallSourceForTest14("'value'", "'value'", "boolean")
	for _, test := range []struct{ name, source, declaration, expression string }{
		{"self", strings.Replace(base, "return leaf<T>(value);", "return route<T>(value);", 1), "route", ""},
		{"mutual", strings.Replace(base, "return value;", "return route<U>(value);", 1), "leaf", ""},
		{"chain", strings.Replace(base, "return value;", "return bottom<U>(value);", 1) + "\n" + strings.Replace(leafLine, "leaf<U>", "bottom<U>", 1), "route", ""},
		{"nested", strings.Replace(base, "leaf<T>(value)", "leaf<T>(leaf<T>(value))", 1), "route", ""},
		{"inferred", strings.Replace(base, "leaf<T>(value)", "leaf(value)", 1), "", "leaf(value)"},
		{"concrete-argument", strings.Replace(uniform, "leaf<T>(value)", "leaf<boolean>(value)", 1), "route", ""},
		{"literal-argument", strings.Replace(uniform, "leaf<T>(value)", "leaf<T>(false)", 1), "", "false"},
		{"binder-text", strings.Replace(base, "[K in", "[J in", 2), "route", ""},
		{"key-order", strings.Replace(mappedCallSourceForTest14("'a' | 'b'", "'a'", "parameter"), "'a' | 'b'", "'b' | 'a'", 2), "route", ""},
		{"key-occurrences", strings.Replace(mappedCallSourceForTest14("'a' | 'b'", "'a'", "parameter"), "'a' | 'b'", "'a' | 'b' | 'a'", 2), "route", ""},
		{"index-order", strings.Replace(mappedCallSourceForTest14("'a' | 'b'", "'a' | 'b'", "parameter"), "}['a' | 'b']", "}['b' | 'a']", 2), "route", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			program, environment := genericProgramForTest(test.source)
			if err := checkGenericDiagnostics(t.Context(), program, nil); err != nil {
				t.Fatalf("owned-negative fixture is not officially valid: %v", err)
			}
			data, err := exportGenericPackage(t.Context(), program, program.GetSourceFile("/original/main.ts"), "package:mapped-call-unsupported", environment)
			requireGenericPackageFailureKind(t, err, "unsupported")
			want := genericOrigin{}
			if test.declaration != "" {
				want = mappedCallDeclarationOrigin14(test.source, test.declaration)
			} else {
				start := strings.LastIndex(test.source, test.expression)
				want = genericOrigin{Start: len(utf16.Encode([]rune(test.source[:start]))), End: len(utf16.Encode([]rune(test.source[:start+len(test.expression)])))}
			}
			var failure *genericFailure
			if !errors.As(err, &failure) || failure.Code != 0 || failure.Origin != want || len(data) != 0 {
				t.Fatalf("owned unsupported lost exact original anchor: %v", err)
			}
		})
	}
}

func TestOwnedGenericMappedCallKeepsOfficialTypeFailures(t *testing.T) {
	t.Parallel()
	base := mappedCallSourceForTest14("'value'", "'value'", "parameter")
	source := strings.Replace(base, "return value;", "return false;", 1)
	program, environment := genericProgramForTest(source)
	data, err := exportGenericPackage(t.Context(), program, program.GetSourceFile("/original/main.ts"), "package:mapped-call-official-error", environment)
	var failure *genericFailure
	if !errors.As(err, &failure) || failure.Kind != "official-type" || failure.Code != 2322 || len(data) != 0 ||
		failure.Source.Path != "/original/main.ts" || failure.Source.Digest != genericTextDigest(source) || failure.Source.Length != len(utf16.Encode([]rune(source))) ||
		failure.Origin.Start < 6 || failure.Origin.End <= failure.Origin.Start || failure.Origin.End > 118 {
		t.Fatalf("official TS2322 was replaced by an owned refusal: %v", err)
	}
}

func TestOwnedGenericMappedCallFiniteKeysAndPhantomInstances(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, keys, index, value string
		keyValues, indexValues   []string
		want                     [2]string
	}{
		{"former-conditional-negative", "'value'", "'value'", "parameter", []string{"value"}, []string{"value"}, [2]string{"bool", "f64"}},
		{"ordered-duplicates", "'z' | 'a' | 'z'", "'a' | 'z' | 'a'", "parameter", []string{"z", "a", "z"}, []string{"a", "z", "a"}, [2]string{"bool", "f64"}},
		{"separate-index-subset", "'z' | 'a' | 'b'", "'b' | 'a'", "parameter", []string{"z", "a", "b"}, []string{"b", "a"}, [2]string{"bool", "f64"}},
		{"phantom-boolean", "'left' | 'right'", "'right'", "boolean", []string{"left", "right"}, []string{"right"}, [2]string{"bool", "bool"}},
		{"phantom-number", "'left' | 'right'", "'left' | 'right'", "number", []string{"left", "right"}, []string{"left", "right"}, [2]string{"f64", "f64"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := mappedCallSourceForTest14(test.keys, test.index, test.value)
			if test.name == "former-conditional-negative" && genericTextDigest(source) != "b3474cc1136403c513c6651792bdb42c20e579fd71e7afd43547597d75db55d4" {
				t.Fatal("the formerly rejected original source was not retained")
			}
			data := genericMappedColdSourceForTest(t, source)
			g, err := decodeGenericPackage(data)
			if err != nil {
				t.Fatal(err)
			}
			again, err := encodeGenericPackage(g)
			if err != nil || !bytes.Equal(data, again) {
				t.Fatalf("complete finite syntax changed on cold roundtrip: %v", err)
			}
			for _, d := range g.Definitions {
				mapped := d.Return.Indexed.Object.Mapped
				if !reflect.DeepEqual(d.Parameters[0].Type, d.Return) || mapped.Binder != "K" || !reflect.DeepEqual(mapped.Keys.Literals, test.keyValues) || !reflect.DeepEqual(d.Return.Indexed.Index.Literals, test.indexValues) {
					t.Fatal("complete binder, raw key occurrences/order or independent index syntax changed")
				}
			}
			requests := []genericPackageRoot{genericPackageRootForTest(t, g, "route", "boolean"), genericPackageRootForTest(t, g, "route", "number")}
			facts, err := instantiateGenericPackageFacts(t.Context(), data, requests)
			if err != nil {
				t.Fatal(err)
			}
			if len(facts.Roots) != 2 || len(facts.Instances) != 4 || facts.Roots[0] == facts.Roots[1] {
				t.Fatal("full concrete/phantom closure identity collapsed")
			}
			for i, want := range test.want {
				caller := genericPackageFactForTest(t, facts, facts.Roots[i])
				callee := genericPackageFactForTest(t, facts, caller.Body.Callee)
				if caller.ReturnType != want || caller.Parameters[0].Type != want || caller.Body.Kind != "call" || caller.Body.Type != want || caller.Body.Arguments[0].Type != want || callee.ReturnType != want || callee.Body.Kind != "parameter" || callee.Body.Parameter != 0 || !reflect.DeepEqual(caller.TypeArguments, requests[i].TypeArguments) {
					t.Fatal("fresh official caller/callee facts or complete outer arguments changed")
				}
			}
		})
	}
}

// Run this named case separately: it performs 512 fresh official instantiations.
// A 129th pair exceeds 256 definitions first; 513 closure and expression depth 64
// are not isolated by this legal one-hop shape. Existing independent limits stay.
func TestOwnedGenericMappedCallFull512InstanceBoundary(t *testing.T) {
	var source strings.Builder
	source.WriteString("// 😀\n")
	for i := 0; i < 128; i++ {
		pair := strings.TrimPrefix(mappedCallSourceForTest14("'value'", "'value'", "parameter"), "// 😀\n")
		pair = strings.ReplaceAll(pair, "leaf<", "leaf"+strconv.Itoa(i)+"<")
		pair = strings.ReplaceAll(pair, "route<", "route"+strconv.Itoa(i)+"<")
		source.WriteString(pair + "\n")
	}
	data := genericMappedColdSourceForTest(t, source.String())
	g, err := decodeGenericPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Definitions) != 256 {
		t.Fatal("full one-hop budget source lost definitions")
	}
	requests := []genericPackageRoot{}
	for i := 0; i < 128; i++ {
		for _, kind := range []string{"boolean", "number"} {
			requests = append(requests, genericPackageRootForTest(t, g, "route"+strconv.Itoa(i), kind))
		}
	}
	before, requestBefore := bytes.Clone(data), mappedCallJSON14(t, requests)
	facts, err := instantiateGenericPackageFacts(t.Context(), data, requests)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.Roots) != 256 || len(facts.Instances) != 512 {
		t.Fatal("exact256 definitions/roots must produce512 distinct complete instances")
	}
	for i, request := range requests {
		caller := genericPackageFactForTest(t, facts, facts.Roots[i])
		callee := genericPackageFactForTest(t, facts, caller.Body.Callee)
		want := []string{"bool", "f64"}[i%2]
		leaf := genericDefinitionNamed(t, g, "leaf"+strconv.Itoa(i/2))
		if caller.Definition != request.Definition || !reflect.DeepEqual(caller.TypeArguments, request.TypeArguments) || caller.ReturnType != want || caller.Body.Kind != "call" || caller.Body.Type != want || len(caller.Body.Arguments) != 1 || caller.Body.Arguments[0].Parameter != 0 || caller.Body.Arguments[0].Type != want || callee.Definition != leaf.Key || callee.ReturnType != want || callee.Body.Kind != "parameter" || callee.Body.Parameter != 0 {
			t.Fatalf("full budget closure lost real official forwarding at root%d", i)
		}
	}
	for i := 1; i < len(facts.Instances); i++ {
		if facts.Instances[i-1].Key >= facts.Instances[i].Key {
			t.Fatal("full closure is not independently keyed and sorted")
		}
	}
	if !bytes.Equal(before, data) || !bytes.Equal(requestBefore, mappedCallJSON14(t, requests)) {
		t.Fatal("full closure mutated caller-owned input")
	}
}
