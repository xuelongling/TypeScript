// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
)

// Candidate v5 owns explicit public constructor access. Its public-constructor
// capability rejects nonpublic construction semantics until those are represented.
// Nonpublic construction semantics remain a separate unsupported capability.
func TestOwnedClassSourceRejectsUnrepresentedConstructorVisibility(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, source, constructor, kind string
		unsupported                     bool
	}{
		{"public-explicit", "// 😀\nexport class Open { public constructor() {} }", "public constructor() {}", "explicit", false},
		{"public-implicit", "// 😀\nexport class Open {}", "", "implicit-base", false},
		{"private", "// 😀\nexport class Closed { private constructor() {} }", "private constructor() {}", "", true},
		{"protected", "// 😀\nexport class Guarded { protected constructor() {} }", "protected constructor() {}", "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			program, _ := genericProgramForTest(test.source)
			// These declarations are legal official Programs. An official syntax,
			// access or signature failure must never substitute for capability RED.
			if err := checkClassDiagnostics(t.Context(), program); err != nil {
				t.Fatalf("constructor visibility fixture is not an officially valid Program: %v", err)
			}
			inputs := []classInput{}
			for _, file := range program.GetSourceFiles() {
				inputs = append(inputs, classInput{Path: file.FileName(), SHA256: classTextSHA(file.Text()), Declaration: file.IsDeclarationFile})
			}
			graph, err := exportClassGraph(t.Context(), program, []*ast.SourceFile{program.GetSourceFile("/original/main.ts")}, classExportOptions{
				Scope: "class:constructor-visibility", Inputs: inputs, MaxNodes: 4096, MaxDepth: 64,
			})
			for _, input := range inputs {
				if writeErr := program.Host().FS().WriteFile(input.Path, ""); writeErr != nil {
					t.Fatal(writeErr)
				}
			}
			if test.unsupported {
				var failure *classFailure
				if !errors.As(err, &failure) || failure.Kind != "unsupported" || !strings.Contains(failure.Detail, "constructor visibility") {
					t.Fatalf("unrepresented nonpublic constructor lost its explicit capability diagnostic: %v", err)
				}
				start := strings.Index(test.source, test.constructor)
				if start < 0 {
					t.Fatal("fixture constructor anchor is missing")
				}
				end := start + len(test.constructor)
				if failure.Origin.Source != "/original/main.ts" ||
					failure.Origin.Start != len(utf16.Encode([]rune(test.source[:start]))) ||
					failure.Origin.End != len(utf16.Encode([]rune(test.source[:end]))) {
					t.Fatalf("unsupported constructor lost its complete modifier/body UTF16 anchor: %+v", failure.Origin)
				}
				if !reflect.DeepEqual(graph, classGraph{}) {
					t.Fatal("unsupported constructor visibility exposed a partial owned graph")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if graph.SemanticProfile != "tsfg-owned-class-facts-candidate-v5" || len(graph.Classes) != 1 {
				t.Fatal("public constructor changed the existing candidate profile or class closure")
			}
			declaration := graph.Classes[0]
			constructor := declaration.Constructor
			if constructor.Kind != test.kind || constructor.Signature.Return != declaration.InstanceType ||
				len(constructor.Signature.Parameters) != 0 || len(constructor.Body) != 0 ||
				constructor.ImplicitForward != nil || constructor.SuperCall != "" ||
				constructor.InitializationPhase != "before-body" || len(constructor.InitializationFields) != 0 ||
				len(declaration.Fields) != 0 || len(declaration.Methods) != 0 {
				t.Fatalf("supported empty public constructor was dropped or changed: %+v", constructor)
			}
		})
	}
}
