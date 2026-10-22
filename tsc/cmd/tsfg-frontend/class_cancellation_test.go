// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/json"
)

func TestOwnedClassSourceCancellationPreservesErrorsAndOwnedOutput(t *testing.T) {
	t.Parallel()
	program, environment := genericProgramForTest(classCounterSource)
	files := []*ast.SourceFile{program.GetSourceFile("/original/main.ts")}
	inputs := []classInput{{Path: "/original/main.ts", SHA256: classTextSHA(classCounterSource)}}
	for _, declaration := range environment.Files {
		inputs = append(inputs, classInput{Path: declaration.Path, SHA256: "sha256:" + declaration.Digest, Declaration: true})
	}
	options := classExportOptions{Scope: "class:cancellation", Inputs: inputs, MaxNodes: 4096, MaxDepth: 64}
	active := func(t *testing.T) classGraph {
		t.Helper()
		graph, err := exportClassGraph(t.Context(), program, files, options)
		if err != nil {
			t.Fatalf("active official class Program did not project: %v", err)
		}
		if len(graph.Classes) != 1 || graph.Classes[0].Name != "Counter" ||
			graph.Classes[0].Origin.Source != "/original/main.ts" || len(graph.Classes[0].Fields) != 1 {
			t.Fatal("active projection did not own the actual Counter class and field")
		}
		counter := graph.Classes[0]
		field := counter.Fields[0]
		if field.Name != "value" || field.Type != "t:number" || field.Initializer == nil ||
			field.Initializer.Kind != "number" || field.Initializer.Token != "1" ||
			counter.Constructor.Kind != "explicit" || len(counter.Constructor.Body) != 1 {
			t.Fatal("active projection lost the actual field initializer or constructor body")
		}
		return graph
	}
	canonical := func(t *testing.T, graph classGraph) []byte {
		t.Helper()
		data, err := json.Marshal(graph)
		if err != nil {
			t.Fatal(err)
		}
		value := json.Value(data)
		if err := value.Canonicalize(); err != nil {
			t.Fatal(err)
		}
		return value
	}
	before := canonical(t, active(t))
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	// An already-expired deadline is synchronous; no timer race or sleep is
	// needed to exercise the distinct original DeadlineExceeded error.
	expired, expire := context.WithDeadline(t.Context(), time.Time{})
	defer expire()
	for _, test := range []struct {
		name string
		ctx  context.Context
		want error
	}{
		{"pre-canceled", canceled, context.Canceled},
		{"expired-deadline", expired, context.DeadlineExceeded},
	} {
		t.Run(test.name, func(t *testing.T) {
			// These cases deliberately reuse the same Program and options in
			// sequence so every rejected call has a subsequent active control.
			if err := test.ctx.Err(); err != test.want {
				t.Fatalf("context fixture is not already terminated: got %v, want %v", err, test.want)
			}
			graph, err := exportClassGraph(test.ctx, program, files, options)
			if err != test.want {
				t.Fatalf("public export lost the original context error: got %v, want %v", err, test.want)
			}
			if !reflect.DeepEqual(graph, classGraph{}) {
				t.Fatal("terminated context exposed a nonzero or partial owned class graph")
			}
			if after := canonical(t, active(t)); !bytes.Equal(before, after) {
				t.Fatal("terminated export changed the subsequent active owned canonical result")
			}
		})
	}
}
