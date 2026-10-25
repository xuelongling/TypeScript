// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"errors"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/parser"
)

// Exercise the existing low-work projection seam instead of generating a large
// source fixture just to exhaust the shared catalog/edge budget.
func TestCommonProgramSharedWork37n(t *testing.T) {
	file := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: "/src/work.ts"},
		"try {} catch {}\ntry {} finally {}", core.ScriptKindTS)
	for _, operation := range []string{"globals", "catch", "lengths"} {
		for _, cause := range []string{"work", "cancel"} {
			t.Run(operation+"-"+cause, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				p := &arrayGenericProjection{ctx: ctx, common: &commonProgramProjection{}}
				state := &arrayGenericState{commonSeeded: operation != "globals"}
				if cause == "work" {
					p.annotationCalls = 99999
				} else {
					cancel()
				}
				bindings := map[*ast.Symbol]arrayFactsBinding{
					{}: {ID: "first"}, {}: {ID: "second"},
				}
				var err error
				switch operation {
				case "globals":
					p.common.globals = bindings
					// The exhausted seed must fail before inspecting an expression.
					_, err = p.expression(nil, "", state, 0)
				case "catch":
					state.bindings = bindings
					_, err = p.commonException(file.Statements.Nodes[0], state, &arrayGenericFunction{}, 0)
				case "lengths":
					state.lengths = map[string]*arrayFactsLengthState{"first": {known: true}, "second": {known: true}}
					_, err = p.commonException(file.Statements.Nodes[1], state, &arrayGenericFunction{}, 0)
				}
				if cause == "cancel" {
					if !errors.Is(err, context.Canceled) || p.annotationCalls != 0 {
						t.Fatalf("canceled %s work was not stopped before expansion: %v", operation, err)
					}
				} else {
					var failure *arrayFactsFailure
					if !errors.As(err, &failure) || failure.Code != "r03-array-facts-limit" || p.annotationCalls != 100000 {
						t.Fatalf("%s expansion must consume only the remaining shared work: %v", operation, err)
					}
				}
				maximumWrites := 1
				if cause == "cancel" {
					maximumWrites = 0
				}
				if operation == "globals" && (state.commonSeeded || len(state.bindings) > maximumWrites) {
					t.Fatal("incomplete global seed was published or grew beyond charged work")
				}
				invalidated := 0
				for _, length := range state.lengths {
					if !length.known {
						invalidated++
					}
				}
				if invalidated > maximumWrites {
					t.Fatal("length invalidation grew beyond charged work")
				}
			})
		}
	}
}
