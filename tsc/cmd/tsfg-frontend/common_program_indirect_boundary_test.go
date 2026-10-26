// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
)

func TestCommonProgramIndirectBoundary37s(t *testing.T) {
	original, err := os.ReadFile("testdata/common-program-v2/common-indirect-valid.ts")
	if err != nil {
		t.Fatal(err)
	}
	for _, sample := range []struct{ name, old, new string }{
		{"arrow", "const f: Unary = leafLeft;", "const f: Unary = (x: i32): i32 => x;"},
		{"block-declaration", "function bump(x: i32): i32 { n += x; return n; }\n  return bump;", "{ function bump(x: i32): i32 { n += x; return n; } return bump; }"},
		{"parameter-capture", "n += x; return n;", "return seed + x;"},
		{"uninitialized-capture", "let n: i32 = seed;", "let n: i32; n = seed;"},
	} {
		t.Run(sample.name, func(t *testing.T) {
			source := strings.Replace(string(original), sample.old, sample.new, 1)
			if source == string(original) {
				t.Fatal("negative Source mutation did not apply")
			}
			// Each remains a complete, independently well-typed Program; rejection
			// must be the documented bounded capability, never a TS diagnostic.
			commonMethodProgram37q(t, source)
			var output bytes.Buffer
			err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(commonIndirectRequest37s(t, []byte(source))), &output)
			var failure *arrayFactsFailure
			if !errors.As(err, &failure) || failure.Code != "r03-array-facts-capability" || output.Len() != 0 {
				t.Fatalf("bounded capability must reject atomically, got %v and %d bytes", err, output.Len())
			}
		})
	}
	t.Run("old-public-flag", func(t *testing.T) {
		var output bytes.Buffer
		if err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v1"}, bytes.NewReader(commonIndirectRequest37s(t, original)), &output); err == nil || output.Len() != 0 {
			t.Fatal("old public common route must reject rather than reheader")
		}
	})
}

// Private low-work seam: three original parent binding identities are inherited
// before the child body. Counts, not map iteration order, define these bounds.
func TestCommonProgramInheritedWork37s(t *testing.T) {
	for _, sample := range []struct {
		name       string
		remaining  int
		cancel     bool
		wantWrites int
		wantError  bool
	}{
		{"exact", 3, false, 3, false}, {"short-one", 2, false, 2, true}, {"exhausted", 0, false, 0, true}, {"canceled", 3, true, 0, true},
	} {
		t.Run(sample.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if sample.cancel {
				cancel()
			}
			parent, child := &ast.Node{}, &ast.Node{}
			a, b, c := &ast.Symbol{}, &ast.Symbol{}, &ast.Symbol{}
			original := map[*ast.Symbol]symbol{a: {ID: "a"}, b: {ID: "b"}, c: {ID: "c"}}
			lexical := &closureProjection{parents: map[*ast.Node]*ast.Node{child: parent}, members: map[*ast.Node]bool{child: true}, environments: map[*ast.Node]map[*ast.Symbol]symbol{parent: original}}
			p := &arrayGenericProjection{ctx: ctx, annotationCalls: 100000 - sample.remaining, common: &commonProgramProjection{lexical: map[*ast.Node]*closureProjection{child: lexical}, bindings: map[string]arrayFactsBinding{"a": {ID: "a"}, "b": {ID: "b"}, "c": {ID: "c"}}}}
			state := &arrayGenericState{bindings: map[*ast.Symbol]arrayFactsBinding{}}
			err := p.commonEnterBindings(child, state)
			if sample.wantError {
				if sample.cancel {
					if !errors.Is(err, context.Canceled) {
						t.Fatal("canceled inheritance accepted", err)
					}
				} else {
					var failure *arrayFactsFailure
					if !errors.As(err, &failure) || failure.Code != "r03-array-facts-limit" {
						t.Fatal("unpaid inheritance accepted", err)
					}
				}
			} else if err != nil {
				t.Fatal("exact inherited work rejected", err)
			}
			if len(state.bindings) != sample.wantWrites || len(original) != 3 {
				t.Fatal("inheritance did work after the bounded stop or changed parent", len(state.bindings))
			}
			if !sample.cancel && p.annotationCalls != 100000 {
				t.Fatal("inherited work did not consume its exact shared budget", p.annotationCalls)
			}
			for key, value := range state.bindings {
				if value.ID != original[key].ID {
					t.Fatal("original binding identity changed")
				}
			}
		})
	}
}

func TestCommonProgramInheritedPackageBytes37s(t *testing.T) {
	// Byte-equivalence pins from the original successful public projections;
	// independent Source semantics remain in commonIndirectOfficial37s.
	for _, sample := range []struct{ name, sha string }{
		{"common-indirect-valid", "93923499b87af414b477d5eb5910b6c4fabd88b6c25cdbad5855ffb49da5bf8c"},
		{"common-indirect-broad-valid", "0740bc010e8ffd3e3b7b08fb38740b526fb4dccc7feb7f3b6c2355e7f7f6f5ae"},
		{"common-indirect-shadow-valid", "df46372997d2c7709270007beb722812d7d4e5beeb1ef07a69d9c16d644a32db"},
	} {
		t.Run(sample.name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/common-program-v2/" + sample.name + ".ts")
			if err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(commonIndirectRequest37s(t, source)), &output); err != nil {
				t.Fatal(err)
			}
			if arrayFactsSHA(output.Bytes()) != sample.sha {
				t.Fatal("inheritance resource repair changed original complete facts")
			}
		})
	}
}

func TestCommonProgramNullableOwnerWire37s(t *testing.T) {
	type ownerDTO struct {
		Owner **string `json:"owner,omitzero"`
	}
	for _, sample := range []struct {
		name     string
		value    ownerDTO
		expected string
	}{
		{"old-v8-absent", ownerDTO{}, "{}"},
		{"common-global-null", ownerDTO{commonNullableOwner37s("")}, `{"owner":null}`},
		{"common-function-owned", ownerDTO{commonNullableOwner37s("fn")}, `{"owner":"fn"}`},
	} {
		t.Run(sample.name, func(t *testing.T) {
			wire, err := arrayFactsCanonical(sample.value)
			if err != nil || string(wire) != sample.expected {
				t.Fatalf("optional-nullable wire %q/%q: %v", wire, sample.expected, err)
			}
			measured, err := arrayBudgetWalk26(sample.value, arrayFactsPolicy(true), 0)
			if err != nil || measured.bytes != len(wire) {
				t.Fatalf("nullable leaf preflight differs: %v", err)
			}
		})
	}
	var unsupported ***string
	if wire, err := arrayFactsCanonical(struct {
		Owner ***string `json:"owner,omitzero"`
	}{unsupported}); err == nil || wire != nil {
		t.Fatal("unbounded pointer chains must remain rejected")
	}
}

func TestCommonProgramCallableWork37s(t *testing.T) {
	for _, operation := range []string{"function-type", "type-alias", "binding-owner", "lexical-header", "capture-scan"} {
		for _, cause := range []string{"work", "cancel"} {
			t.Run(operation+"-"+cause, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				p := &arrayGenericProjection{ctx: ctx, common: &commonProgramProjection{bindings: map[string]arrayFactsBinding{}}, types: map[string]arrayGenericType{}}
				if cause == "work" {
					p.annotationCalls = 100000
				} else {
					cancel()
				}
				var err error
				switch operation {
				case "function-type":
					_, err = p.commonCallableType([]string{"i32"}, "i32", nil)
				case "type-alias":
					err = p.commonTypeAlias(nil)
				case "binding-owner":
					err = p.commonBinding(nil, &arrayFactsBinding{})
				case "lexical-header":
					err = p.commonFunctionHeader(nil, &arrayGenericFunction{})
				case "capture-scan":
					err = p.commonFunctionComplete(nil, &arrayGenericFunction{}, &arrayGenericState{bindings: map[*ast.Symbol]arrayFactsBinding{{}: {ID: "original"}}})
				}
				if cause == "cancel" {
					if !errors.Is(err, context.Canceled) || p.annotationCalls != 0 {
						t.Fatal("cancel did not stop new Source work before dereference", err)
					}
				} else {
					var failure *arrayFactsFailure
					if !errors.As(err, &failure) || failure.Code != "r03-array-facts-limit" || p.annotationCalls != 100000 {
						t.Fatal("shared projection budget was bypassed", err)
					}
				}
				if len(p.types) != 0 || len(p.common.bindings) != 0 || len(p.common.typeAliases) != 0 {
					t.Fatal("failed work published partial fact catalogs")
				}
			})
		}
	}
}
