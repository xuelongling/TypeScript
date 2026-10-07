// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"strings"
	"testing"
)

func conversionRequest(t *testing.T, source string) request {
	t.Helper()
	bytes, err := os.ReadFile("testdata/minimal-conversions-v5.d.ts")
	if err != nil {
		t.Fatal(err)
	}
	r := integerRequest(source)
	r.Files[1] = inputFile{Path: "/src/minimal-conversions-v5.d.ts", Content: string(bytes)}
	r.Declarations.Version = conversionVersion
	r.Declarations.Paths = []string{r.Files[1].Path}
	return r
}

func TestAdmittedConversionHasOwnedOriginAndOneOperand(t *testing.T) {
	source := "export function entry(value: f64): i8 { return checked_i8_from_f64(value); }"
	r := conversionRequest(t, source)
	got := analyze(r)
	if got.Status != "ok" {
		t.Fatalf("conversion rejected: %+v", got.Diagnostics)
	}
	e := got.Functions[0].Body[0].Expression
	if e.Kind != "conversion" || e.Type != "i8" || e.Operand == nil || e.Operand.Type != "f64" || e.Target != "" || e.Arguments != nil || e.Signature != nil || e.Conversion == nil {
		t.Fatalf("not an owned unary scalar operation: %+v", e)
	}
	origin := e.Conversion
	if origin.DeclarationSHA256 != conversionDeclarationHash || origin.Version != conversionVersion || origin.DeclarationPath != r.Files[1].Path || !strings.HasPrefix(origin.EntryID, "sha256:") || sourceText(t, r.Files[1].Content, origin.DeclarationRange) != "declare function checked_i8_from_f64(value: f64): i8;" {
		t.Fatalf("incorrect declaration ownership: %+v", origin)
	}
}

func TestConversionSpellingDoesNotOwnOrdinaryFunction(t *testing.T) {
	source := "function checked_i8_from_f64(value: f64): i8 { return 42; } export function entry(value: f64): i8 { return checked_i8_from_f64(value); }"
	got := analyze(conversionRequest(t, source))
	if got.Status != "ok" {
		t.Fatalf("valid ordinary body rejected: %+v", got.Diagnostics)
	}
	e := got.Functions[1].Body[0].Expression
	if e.Kind != "call" || e.Conversion != nil || e.Target != got.Functions[0].Symbol {
		t.Fatalf("ordinary function became intrinsic: %+v", e)
	}
}

func TestGlobalConversionSpellingWithBodyRemainsOrdinary(t *testing.T) {
	source := "function checked_i8_from_f64(value: f64): i8 { return 42; } function entry(value: f64): i8 { return checked_i8_from_f64(value); }"
	got := analyze(conversionRequest(t, source))
	if got.Status != "ok" {
		t.Fatalf("valid global ordinary body rejected: %+v", got.Diagnostics)
	}
	e := got.Functions[1].Body[0].Expression
	if e.Kind != "call" || e.Conversion != nil || e.Target != got.Functions[0].Symbol {
		t.Fatalf("global user body became intrinsic: %+v", e)
	}
}

func TestConversionRejectsUnownedAndInvalidNominalInputs(t *testing.T) {
	for _, source := range []string{
		"export function entry(x: f64): i8 { return checked_i8_from_f64(); }",
		"export function entry(x: f64): i8 { return checked_i8_from_f64(x,x); }",
		"export function entry(x: f64): i8 { return checked_i8_from_f64<f64>(x); }",
		"export function entry(x: i16): i8 { return checked_i8_from_f64(x); }",
		"export function entry(x: f64): i8 { return reinterpret_i8_from_f64(x); }",
		"declare const brand: unique symbol; type i8 = number & { readonly [brand]: true }; export function entry(x: f64): i8 { return checked_i8_from_f64(x); }",
		"declare const brand: unique symbol; type f64 = number & { readonly [brand]: true }; export function entry(x: f64): i8 { return checked_i8_from_f64(x); }",
	} {
		got := analyze(conversionRequest(t, source))
		if got.Status != "error" || len(got.Diagnostics) == 0 {
			t.Fatalf("invalid input accepted: %s %+v", source, got)
		}
		if strings.HasPrefix(got.Diagnostics[0].Code, "r01-d-conversion-") &&
			(got.Diagnostics[0].RuleID != "r01-conversions" || got.Diagnostics[0].Phase != "types") {
			t.Fatalf("conversion diagnostic machine identity differs: %+v", got.Diagnostics[0])
		}
	}
	r := conversionRequest(t, "export function entry(x: f64): i8 { return checked_i8_from_f64(x); }")
	r.Files[1].Content += "\n"
	if got := analyze(r); got.Status != "error" {
		t.Fatalf("changed official declaration accepted: %+v", got)
	}
}
