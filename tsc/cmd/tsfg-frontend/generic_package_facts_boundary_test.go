// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestOwnedGenericPackageFactsRejectsInvalidRoots(t *testing.T) {
	t.Parallel()
	data := genericPackageFactsBytesFromSource(t, "export function entry<T>(value: T): T { return hidden<T>(value); }\n"+
		"function hidden<U>(value: U): U { return value; }")
	g, err := decodeGenericPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	base := genericPackageRootForTest(t, g, "entry", "boolean")
	for _, test := range []struct {
		name, kind string
		roots      []genericPackageRoot
	}{
		{"empty", "package-limit", nil},
		{"too many", "package-limit", make([]genericPackageRoot, 257)},
		{"empty definition", "package-reference", []genericPackageRoot{{TypeArguments: base.TypeArguments}}},
		{"foreign definition", "package-reference", []genericPackageRoot{{Definition: strings.Repeat("0", 64), TypeArguments: base.TypeArguments}}},
		{"private definition", "package-reference", []genericPackageRoot{genericPackageRootForTest(t, g, "hidden", "boolean")}},
		{"missing type arguments", "package-reference", []genericPackageRoot{{Definition: base.Definition}}},
		{"extra type arguments", "package-reference", []genericPackageRoot{genericPackageRootForTest(t, g, "entry", "boolean", "number")}},
		{"uninstantiated binder", "package-reference", []genericPackageRoot{genericPackageRootForTest(t, g, "entry", "parameter")}},
		{"unknown type", "package-reference", []genericPackageRoot{genericPackageRootForTest(t, g, "entry", "unknown")}},
		{"nonzero scalar index", "package-reference", []genericPackageRoot{{Definition: base.Definition, TypeArguments: []genericPackageTypeDTO{{Kind: "boolean", Parameter: 1}}}}},
		{"type argument limit", "package-limit", []genericPackageRoot{{Definition: base.Definition, TypeArguments: make([]genericPackageTypeDTO, 17)}}},
		{"invalid UTF8", "package-unicode", []genericPackageRoot{{Definition: "bad\xff", TypeArguments: base.TypeArguments}}},
		{"NUL", "package-unicode", []genericPackageRoot{{Definition: "bad\x00", TypeArguments: base.TypeArguments}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := instantiateGenericPackageFacts(t.Context(), data, test.roots)
			requireGenericPackageFailureKind(t, err, test.kind)
			if !reflect.DeepEqual(result, genericPackageFacts{}) {
				t.Fatal("invalid root exposed partial facts")
			}
		})
	}
}

func TestOwnedGenericPackageFactsRejectsPackageErrors(t *testing.T) {
	t.Parallel()
	data := genericPackageBytesForTest(t)
	g, err := decodeGenericPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	roots := []genericPackageRoot{genericPackageRootForTest(t, g, "route", "boolean")}
	wire := genericPackageWireForTest(t, data)
	wire.SchemaHash = "different"
	wrongSchema := genericPackageSignedBytesForTest(t, wire)
	wire = genericPackageWireForTest(t, data)
	wire.PackageID = "sha256:" + strings.Repeat("0", 64)
	wrongID, err := genericPackageCanonical(wire)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, kind string
		data       []byte
	}{
		{"empty", "package-limit", nil},
		{"JSON", "package-json", []byte("{")},
		{"canonical", "package-canonical", append(bytes.Clone(data), '\n')},
		{"schema", "package-schema", wrongSchema},
		{"identity", "package-identity", wrongID},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := instantiateGenericPackageFacts(t.Context(), test.data, roots)
			requireGenericPackageFailureKind(t, err, test.kind)
			if !reflect.DeepEqual(result, genericPackageFacts{}) {
				t.Fatal("invalid package exposed partial facts")
			}
		})
	}
}

func TestOwnedGenericPackageFactsCancellation(t *testing.T) {
	t.Parallel()
	data := genericPackageBytesForTest(t)
	g, err := decodeGenericPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	roots := []genericPackageRoot{genericPackageRootForTest(t, g, "route", "boolean")}
	t.Run("pre-canceled", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		result, err := instantiateGenericPackageFacts(ctx, nil, nil)
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, genericPackageFacts{}) {
			t.Fatalf("pre-canceled adapter returned partial data or wrong error: %v", err)
		}
	})
	t.Run("deadline", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithDeadline(t.Context(), time.Unix(0, 0))
		defer cancel()
		result, err := instantiateGenericPackageFacts(ctx, data, roots)
		if !errors.Is(err, context.DeadlineExceeded) || !reflect.DeepEqual(result, genericPackageFacts{}) {
			t.Fatalf("expired adapter returned partial data or wrong error: %v", err)
		}
	})
	t.Run("controlled poll", func(t *testing.T) {
		t.Parallel()
		inner, cancel := context.WithCancel(t.Context())
		defer cancel()
		ctx := &genericCancelOnPoll{Context: inner, cancel: cancel}
		ctx.remaining.Store(12)
		result, err := instantiateGenericPackageFacts(ctx, data, roots)
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, genericPackageFacts{}) {
			t.Fatalf("cancellation during import/checking returned partial data or wrong error: %v", err)
		}
	})
}

func TestOwnedGenericPackageFactsPreservesOfficialDiagnostic(t *testing.T) {
	t.Parallel()
	g, err := decodeGenericPackage(genericPackageBytesForTest(t))
	if err != nil {
		t.Fatal(err)
	}
	bad := genericPackageDefinitionForMutation(t, &g, "unused")
	bad.Return = genericType{Kind: "number"}
	key, origin := bad.Key, bad.Origin
	g.Digest = genericGraphDigest(g)
	data, err := encodeGenericPackage(g)
	if err != nil {
		t.Fatal(err)
	}
	result, err := instantiateGenericPackageFacts(t.Context(), data, []genericPackageRoot{{Definition: key,
		TypeArguments: []genericPackageTypeDTO{{Kind: "boolean"}}}})
	var failure *genericFailure
	if !errors.As(err, &failure) || failure.Kind != "official-type" || failure.Code != 2322 ||
		!reflect.DeepEqual(failure.Source, g.Source) || failure.Origin.Start < origin.Start ||
		failure.Origin.End > origin.End || failure.Origin.End <= failure.Origin.Start || !reflect.DeepEqual(result, genericPackageFacts{}) {
		t.Fatalf("adapter changed the official machine code/original UTF16 origin: %v", err)
	}
}

func genericPackageFactRecursiveBody(depth int) string {
	if depth == 0 {
		return "value"
	}
	child := genericPackageFactRecursiveBody(depth - 1)
	return "wide<T0,T1,T2,T3,T4,T5,T6>(" + child + "," + child + ")"
}

func genericPackageFactPhantomRoots(t *testing.T, g genericGraph, name string, count, arguments int) []genericPackageRoot {
	t.Helper()
	roots := make([]genericPackageRoot, count)
	for i := range roots {
		kinds := make([]string, arguments)
		for j := range kinds {
			kinds[j] = "boolean"
			if j > 0 && i&(1<<(j-1)) != 0 {
				kinds[j] = "number"
			}
		}
		roots[i] = genericPackageRootForTest(t, g, name, kinds...)
	}
	return roots
}

func TestOwnedGenericPackageFactsBoundsResult(t *testing.T) {
	t.Parallel()
	t.Run("total nodes", func(t *testing.T) {
		t.Parallel()
		data := genericPackageFactsBytesFromSource(t, "export function wide<T0,T1,T2,T3,T4,T5,T6>(value: T0, other: T0): T0 { return "+
			genericPackageFactRecursiveBody(9)+"; }")
		g, err := decodeGenericPackage(data)
		if err != nil {
			t.Fatal(err)
		}
		roots := genericPackageFactPhantomRoots(t, g, "wide", 33, 7)
		result, err := instantiateGenericPackageFacts(t.Context(), data, roots)
		var failure *genericFailure
		if !errors.As(err, &failure) || failure.Kind != "package-limit" || failure.Detail != "result expression depth/count" ||
			!reflect.DeepEqual(result, genericPackageFacts{}) {
			t.Fatalf("33 real instances of 1023 nodes did not respect the result node bound: %v", err)
		}
	})
	t.Run("canonical bytes before serialization", func(t *testing.T) {
		t.Parallel()
		parameters := make([]string, 8)
		first := strings.Repeat("p", 64<<10) + "0"
		for i := range parameters {
			parameters[i] = strings.Repeat("p", 64<<10) + strconv.Itoa(i) + ": T0"
		}
		data := genericPackageFactsBytesFromSource(t, "export function large<T0,T1,T2,T3,T4,T5>("+
			strings.Join(parameters, ",")+"): T0 { return "+first+"; }")
		g, err := decodeGenericPackage(data)
		if err != nil {
			t.Fatal(err)
		}
		roots := genericPackageFactPhantomRoots(t, g, "large", 17, 6)
		result, err := instantiateGenericPackageFacts(t.Context(), data, roots)
		var failure *genericFailure
		if !errors.As(err, &failure) || failure.Kind != "package-limit" || failure.Detail != "canonical bytes" ||
			!reflect.DeepEqual(result, genericPackageFacts{}) {
			t.Fatalf("real instance metadata exceeded cumulative bytes before serialization: %v", err)
		}
	})
}
