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

	"github.com/microsoft/typescript-go/internal/ast"
)

func genericMappedTypeSourceForTest(typeText string) string {
	return "// 😀\nexport function mapped<T>(value: " + typeText + "): " + typeText + " { return value; }"
}

// Return bytes only after real diagnostics and export; clear every original VFS
// input so subsequent package requests cannot consult the source Program.
func genericMappedColdSourceForTest(t *testing.T, source string) []byte {
	t.Helper()
	program, environment := genericProgramForTest(source)
	if err := checkGenericDiagnostics(t.Context(), program, nil); err != nil {
		t.Fatalf("mapped fixture is not officially valid: %v", err)
	}
	data, err := exportGenericPackage(t.Context(), program, program.GetSourceFile("/original/main.ts"), "package:mapped-boundaries", environment)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range program.GetSourceFiles() {
		if err := program.Host().FS().WriteFile(file.FileName(), ""); err != nil {
			t.Fatal(err)
		}
		if text, ok := program.Host().FS().ReadFile(file.FileName()); !ok || text != "" {
			t.Fatal("original mapped input was not cleared")
		}
	}
	return data
}

func TestOwnedGenericMappedSignatureFiniteKeysColdInstances(t *testing.T) {
	t.Parallel()
	sixteen, sixteenText := []string{}, []string{}
	for i := 0; i < 16; i++ {
		key := "key" + strconv.Itoa(i)
		sixteen = append(sixteen, key)
		sixteenText = append(sixteenText, "'"+key+"'")
	}
	maxKey := strings.Repeat("é", 128)
	for _, test := range []struct {
		name, keys, index, value string
		keyKind, indexKind       string
		keyValues, indexValues   []string
		want                     [2]string
	}{
		{"ordered-duplicates", "'z' | 'a' | 'z'", "'z' | 'a' | 'z'", "T", "union", "union", []string{"z", "a", "z"}, []string{"z", "a", "z"}, [2]string{"bool", "f64"}},
		{"reordered-subset", "'z' | 'a' | 'b'", "'b' | 'a'", "T", "union", "union", []string{"z", "a", "b"}, []string{"b", "a"}, [2]string{"bool", "f64"}},
		{"empty-key", "''", "''", "T", "string-literal", "string-literal", []string{""}, []string{""}, [2]string{"bool", "f64"}},
		{"unicode-no-normalization", "'😀' | 'é' | 'e\\u0301' | '\\u{10000}'", "'e\\u0301' | '😀'", "T", "union", "union", []string{"😀", "é", "e\u0301", "𐀀"}, []string{"e\u0301", "😀"}, [2]string{"bool", "f64"}},
		{"escaped-newline", "'line\\nvalue'", "'line\\nvalue'", "T", "string-literal", "string-literal", []string{"line\nvalue"}, []string{"line\nvalue"}, [2]string{"bool", "f64"}},
		{"key-256-utf8-bytes", "'" + maxKey + "'", "'" + maxKey + "'", "T", "string-literal", "string-literal", []string{maxKey}, []string{maxKey}, [2]string{"bool", "f64"}},
		{"sixteen-domain-and-index-occurrences", strings.Join(sixteenText, " | "), strings.Join(sixteenText, " | "), "T", "union", "union", sixteen, sixteen, [2]string{"bool", "f64"}},
		{"sixteen-duplicate-occurrences", strings.Repeat("'value' | ", 15) + "'value'", strings.Repeat("'value' | ", 15) + "'value'", "T", "union", "union", strings.Split(strings.Repeat("value,", 15)+"value", ","), strings.Split(strings.Repeat("value,", 15)+"value", ","), [2]string{"bool", "f64"}},
		{"uniform-boolean", "'left' | 'right'", "'right'", "boolean", "union", "string-literal", []string{"left", "right"}, []string{"right"}, [2]string{"bool", "bool"}},
		{"uniform-number", "'left' | 'right'", "'left' | 'right'", "number", "union", "union", []string{"left", "right"}, []string{"left", "right"}, [2]string{"f64", "f64"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			typeText := "{ [K in " + test.keys + "]: " + test.value + " }[" + test.index + "]"
			source := genericMappedTypeSourceForTest(typeText)
			data := genericMappedColdSourceForTest(t, source)
			g, err := decodeGenericPackage(data)
			if err != nil {
				t.Fatal(err)
			}
			again, err := encodeGenericPackage(g)
			if err != nil || !bytes.Equal(data, again) {
				t.Fatalf("finite mapped syntax changed on cold roundtrip: %v", err)
			}
			d := genericDefinitionNamed(t, g, "mapped")
			if d.Return.Indexed == nil || d.Return.Indexed.Object.Mapped == nil || !reflect.DeepEqual(d.Parameters[0].Type, d.Return) {
				t.Fatal("complete indexed/mapped signature was lost")
			}
			mapped := d.Return.Indexed.Object.Mapped
			wantValue := genericType{Kind: test.value}
			if test.value == "T" {
				wantValue = genericType{Kind: "parameter"}
			}
			if mapped.Binder != "K" || mapped.Value != wantValue ||
				!reflect.DeepEqual(mapped.Keys, genericKeyType{Kind: test.keyKind, Literals: test.keyValues}) ||
				!reflect.DeepEqual(d.Return.Indexed.Index, genericKeyType{Kind: test.indexKind, Literals: test.indexValues}) ||
				g.Source.Digest != genericTextDigest(source) || g.Source.Length != len(utf16.Encode([]rune(source))) {
				t.Fatal("keys were normalized, deduplicated, selected, reordered or confused with the outer binder")
			}
			requests := []genericPackageRoot{genericPackageRootForTest(t, g, "mapped", "boolean"), genericPackageRootForTest(t, g, "mapped", "number")}
			facts, err := instantiateGenericPackageFacts(t.Context(), data, requests)
			if err != nil {
				t.Fatal(err)
			}
			if len(facts.Instances) != 2 || len(facts.Roots) != 2 || facts.Roots[0] == facts.Roots[1] {
				t.Fatal("concrete or phantom outer arguments lost distinct instance identities")
			}
			for i, want := range test.want {
				instance := genericPackageFactForTest(t, facts, facts.Roots[i])
				if instance.ReturnType != want || len(instance.Parameters) != 1 || instance.Parameters[0].Type != want ||
					instance.Body.Kind != "parameter" || instance.Body.Parameter != 0 || instance.Body.Type != want ||
					instance.Body.Origin != genericPackageOrigin(d.Body.Origin) || !reflect.DeepEqual(instance.TypeArguments, requests[i].TypeArguments) {
					t.Fatalf("fresh official scalar facts differ for request %d: %+v", i, instance)
				}
			}
		})
	}
	// The inner binder name is independent of the sole outer symbol even when
	// that outer symbol uses the familiar mapped-key spelling K.
	source := strings.Replace(genericMappedTypeSourceForTest("{ [J in 'value']: K }['value']"), "mapped<T>", "mapped<K>", 1)
	data := genericMappedColdSourceForTest(t, source)
	g, err := decodeGenericPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	d := genericDefinitionNamed(t, g, "mapped")
	if !reflect.DeepEqual(d.TypeParameters, []string{"K"}) || d.Return.Indexed.Object.Mapped.Binder != "J" ||
		d.Return.Indexed.Object.Mapped.Value != (genericType{Kind: "parameter"}) {
		t.Fatal("inner binder and real outer type symbol were confused")
	}
	facts, err := instantiateGenericPackageFacts(t.Context(), data, []genericPackageRoot{genericPackageRootForTest(t, g, "mapped", "boolean"), genericPackageRootForTest(t, g, "mapped", "number")})
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.Roots) != 2 || genericPackageFactForTest(t, facts, facts.Roots[0]).ReturnType != "bool" || genericPackageFactForTest(t, facts, facts.Roots[1]).ReturnType != "f64" {
		t.Fatalf("independently named binders changed official facts: %v", err)
	}
}

func TestOwnedGenericMappedSignatureRejectsUnsupportedOfficialSource(t *testing.T) {
	t.Parallel()
	base := "{ [K in 'value']: T }['value']"
	for _, test := range []struct {
		name, typeText, rejected string
		width                    int
	}{
		{"readonly", "{ readonly [K in 'value']: T }['value']", "{ readonly [K in 'value']: T }", 0},
		{"optional", "{ [K in 'value']?: T }['value']", "{ [K in 'value']?: T }", 0},
		{"remapping", "{ [K in 'value' as 'renamed']: T }['renamed']", "{ [K in 'value' as 'renamed']: T }", 0},
		{"inner-symbol-value", "{ [K in 'value']: K }['value']", "K }", 1},
		{"same-name-binder", "{ [T in 'value']: boolean }['value']", "T in 'value'", 0},
		{"keyof", "{ [K in keyof { value: T }]: T }['value']", "keyof { value: T }", 0},
		{"arbitrary-object", "{ value: T }['value']", "{ value: T }['value']", 0},
		{"infinite-string-key", "{ [K in string]: T }['value']", "string", 0},
		{"conditional-value", "{ [K in 'value']: T extends boolean ? boolean : number }['value']", "T extends boolean ? boolean : number", 0},
		{"numeric-key", "{ [K in 0]: T }[0]", "0", 1},
		{"nested-mapped-value", "{ [K in 'value']: { [L in 'inner']: T }['inner'] }['value']", "{ [L in 'inner']: T }['inner']", 0},
		{"direct-mapped", "{ [K in 'value']: T }", "{ [K in 'value']: T }", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			source := genericMappedTypeSourceForTest(test.typeText)
			program, environment := genericProgramForTest(source)
			if err := checkGenericDiagnostics(t.Context(), program, nil); err != nil {
				t.Fatalf("unsupported mapped fixture is not officially valid: %v", err)
			}
			data, err := exportGenericPackage(t.Context(), program, program.GetSourceFile("/original/main.ts"), "package:mapped-unsupported", environment)
			requireGenericPackageFailureKind(t, err, "unsupported")
			start, width := strings.Index(source, test.rejected), test.width
			if width == 0 {
				width = len(test.rejected)
			}
			var failure *genericFailure
			if start < 0 || !errors.As(err, &failure) || len(data) != 0 ||
				failure.Origin != (genericOrigin{Start: len(utf16.Encode([]rune(source[:start]))), End: len(utf16.Encode([]rune(source[:start+width])))}) {
				t.Fatalf("unsupported real syntax lost its exact UTF16 anchor: %v", err)
			}
		})
	}
	for _, source := range []string{
		strings.Replace(genericMappedTypeSourceForTest(base), "mapped<T>", "mapped<T, U>", 1),
		"// 😀\nfunction identity<U>(value: U): U { return value; }\n" + strings.TrimPrefix(strings.Replace(genericMappedTypeSourceForTest(base), "return value;", "return identity<"+base+">(value);", 1), "// 😀\n"),
	} {
		program, environment := genericProgramForTest(source)
		if err := checkGenericDiagnostics(t.Context(), program, nil); err != nil {
			t.Fatal(err)
		}
		data, err := exportGenericPackage(t.Context(), program, program.GetSourceFile("/original/main.ts"), "package:mapped-role", environment)
		requireGenericPackageFailureKind(t, err, "unsupported")
		if len(data) != 0 {
			t.Fatal("unsupported signature/body/call role exposed bytes")
		}
	}
}

func TestOwnedGenericMappedSignatureKeepsOfficialFailures(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, source string
		code         int32
	}{
		{"wrong-body", strings.Replace(genericMappedSignatureSource, "return value;", "return false;", 1), 2322},
		{"unknown-index", genericMappedTypeSourceForTest("{ [K in 'value']: T }['missing']"), 2339},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			program, environment := genericProgramForTest(test.source)
			data, err := exportGenericPackage(t.Context(), program, program.GetSourceFile("/original/main.ts"), "package:mapped-official-error", environment)
			var failure *genericFailure
			if !errors.As(err, &failure) || failure.Kind != "official-type" || failure.Code != test.code || len(data) != 0 ||
				failure.Source.Path != "/original/main.ts" || failure.Source.Digest != genericTextDigest(test.source) ||
				failure.Source.Length != len(utf16.Encode([]rune(test.source))) || failure.Origin.Start < 6 ||
				failure.Origin.End <= failure.Origin.Start || failure.Origin.End > failure.Source.Length {
				t.Fatalf("real mapped official diagnostic lost code, source identity or UTF16 anchor: %v", err)
			}
		})
	}
}

func TestOwnedGenericMappedSignatureFreshCheckerIgnoresDiagnosticText(t *testing.T) {
	t.Parallel()
	data := genericMappedColdSourceForTest(t, genericMappedTypeSourceForTest("{ [K in 'z' | 'a' | 'z']: T }['a' | 'z']"))
	g, err := decodeGenericPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	d := genericDefinitionNamed(t, g, "mapped")
	for i, kind := range []string{"boolean", "number"} {
		request := g.request(d.Key, genericType{Kind: kind})
		a, b := genericTreeForTest(t, g, request), genericTreeForTest(t, g, request)
		counts := map[ast.Kind]int{}
		for _, fact := range genericSyntaxFacts(a) {
			counts[fact.Kind]++
		}
		if counts[ast.KindMappedType] != 3 || counts[ast.KindIndexedAccessType] != 3 || counts[ast.KindUnionType] != 6 || counts[ast.KindStringLiteral] != 15 {
			t.Fatal("factory or driver erased compound or repeated key syntax")
		}
		normal, blank := newGenericProgram(g, a, a.text.String()), newGenericProgram(g, b, strings.Repeat(" ", b.text.Len()))
		if err := checkGenericProgram(t.Context(), g, normal, a); err != nil {
			t.Fatal(err)
		}
		if err := checkGenericProgram(t.Context(), g, blank, b); err != nil {
			t.Fatal(err)
		}
		x, xc, err := projectCheckedGenericInstance(t.Context(), g, request, a, normal)
		if err != nil {
			t.Fatal(err)
		}
		y, yc, err := projectCheckedGenericInstance(t.Context(), g, request, b, blank)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"bool", "f64"}[i]
		if !reflect.DeepEqual(x, y) || len(xc) != 0 || len(yc) != 0 || x.Return != want || x.Body.Type != want ||
			!reflect.DeepEqual(x.Parameters, []string{want}) || x.Body.Kind != "parameter" || x.Body.Parameter != 0 {
			t.Fatal("diagnostic text changed fresh official mapped facts")
		}
	}
}
