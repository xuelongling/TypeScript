// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// The official checker keeps an explicit this parameter outside ordinary
// runtime parameters. Unsupported syntax must not be called broken identity.
func TestOwnedClassSourceRejectsExplicitThisParameterWithCapabilityDiagnostic(t *testing.T) {
	t.Parallel()
	t.Run("ordinary-reference-parameter", func(t *testing.T) {
		t.Parallel()
		graph := classSourceForTest(t, "export class A { m(self: A): void {} }")
		if len(graph.Classes) != 1 || len(graph.Classes[0].Methods) != 1 {
			t.Fatal("ordinary class method was not completely projected")
		}
		owner := graph.Classes[0]
		method := owner.Methods[0]
		if method.Name != "m" || len(method.Signature.Parameters) != 1 ||
			method.Signature.Parameters[0].Type != owner.InstanceType || len(method.Body) != 0 {
			t.Fatal("ordinary reference parameter lost its signature or method body")
		}
	})
	t.Run("explicit-this-capability-diagnostic", func(t *testing.T) {
		t.Parallel()
		const source = "export class A { m(this: A): void {} }"
		graph, err := classIdentityExport(t, source, nil)
		var failure *classFailure
		if !errors.As(err, &failure) || failure.Kind != "unsupported" ||
			!strings.Contains(failure.Detail, "this") || !strings.Contains(failure.Detail, "parameter") {
			t.Fatalf("legal explicit this parameter needs a specific capability diagnostic: %v", err)
		}
		start := strings.Index(source, "this: A")
		expected := classAnchor{Source: "/original/main.ts", Start: start, End: start + len("this: A")}
		if failure.Origin != expected || !reflect.DeepEqual(graph, classGraph{}) {
			t.Fatal("unsupported this parameter lost its exact source anchor or exposed a partial graph")
		}
	})
}
