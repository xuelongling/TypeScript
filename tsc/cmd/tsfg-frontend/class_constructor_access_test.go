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

// Read constructor access as a cold consumer. Missing owned facts must fail as
// behavior before a new profile or Go DTO field can hide that capability gap.
func TestOwnedClassSourceOwnsExplicitPublicConstructorAccess(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, source, unsupportedConstructor string
		kinds, bases                         map[string]string
	}{
		{"explicit-public", "// 😀\nexport class Root { public constructor() {} }", "",
			map[string]string{"Root": "explicit"}, map[string]string{}},
		{"explicit-default", "// 😀\nexport class Root { constructor() {} }", "",
			map[string]string{"Root": "explicit"}, map[string]string{}},
		{"implicit-root", "// 😀\nexport class Root {}", "",
			map[string]string{"Root": "implicit-base"}, map[string]string{}},
		{"implicit-public-chain", "// 😀\nexport class Root { public constructor() {} }\nexport class Middle extends Root {}\nexport class Leaf extends Middle {}", "",
			map[string]string{"Root": "explicit", "Middle": "implicit-derived", "Leaf": "implicit-derived"},
			map[string]string{"Middle": "Root", "Leaf": "Middle"}},
		{"protected-base-implicit-child", "// 😀\nexport class Base { protected constructor() {} }\nexport class Child extends Base {}", "protected constructor() {}",
			nil, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			program, _ := genericProgramForTest(test.source)
			// A protected constructor may have a legally declared subclass. Check
			// official legality first; an access diagnostic is not this capability.
			if err := checkClassDiagnostics(t.Context(), program); err != nil {
				t.Fatalf("constructor access fixture is not an officially valid Program: %v", err)
			}
			inputs := []classInput{}
			for _, file := range program.GetSourceFiles() {
				inputs = append(inputs, classInput{Path: file.FileName(), SHA256: classTextSHA(file.Text()), Declaration: file.IsDeclarationFile})
			}
			graph, err := exportClassGraph(t.Context(), program, []*ast.SourceFile{program.GetSourceFile("/original/main.ts")}, classExportOptions{
				Scope: "class:constructor-access", Inputs: inputs, MaxNodes: 4096, MaxDepth: 64,
			})
			for _, input := range inputs {
				if writeErr := program.Host().FS().WriteFile(input.Path, ""); writeErr != nil {
					t.Fatal(writeErr)
				}
			}
			if test.unsupportedConstructor != "" {
				var failure *classFailure
				if !errors.As(err, &failure) || failure.Kind != "unsupported" || !strings.Contains(failure.Detail, "constructor visibility") {
					t.Fatalf("unrepresented protected base lost its controlled visibility capability: %v", err)
				}
				start := strings.Index(test.source, test.unsupportedConstructor)
				if start < 0 {
					t.Fatal("protected constructor fixture anchor is missing")
				}
				end := start + len(test.unsupportedConstructor)
				if failure.Origin.Source != "/original/main.ts" ||
					failure.Origin.Start != len(utf16.Encode([]rune(test.source[:start]))) ||
					failure.Origin.End != len(utf16.Encode([]rune(test.source[:end]))) {
					t.Fatalf("protected constructor lost its complete original UTF16 anchor: %+v", failure.Origin)
				}
				if !reflect.DeepEqual(graph, classGraph{}) {
					t.Fatal("implicit child exposed a partial graph from an unsupported base")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			view := classMultiView(t, graph)
			classes := classOwnedArray(t, view["classes"])
			if len(classes) != len(test.kinds) {
				t.Fatal("explicit original constructor class closure differs")
			}
			// This is deliberately the first new assertion: actual output must
			// own public access for every constructor, not imply it from absence.
			for _, item := range classes {
				declaration := classOwnedObject(t, item)
				constructor := classOwnedObject(t, declaration["constructor"])
				if constructor["access"] != "public" {
					t.Fatalf("official %s constructor has no explicit owned public access fact: %#v", declaration["name"], constructor["access"])
				}
			}
			if view["semanticProfile"] != "tsfg-owned-class-facts-candidate-v5" {
				t.Fatal("constructor access facts require the exact new candidate profile")
			}
			for name, kind := range test.kinds {
				declaration := classOwnedNamed(t, classes, name)
				constructor := classOwnedObject(t, declaration["constructor"])
				signature := classOwnedObject(t, constructor["signature"])
				if constructor["kind"] != kind || signature["return"] != declaration["instanceType"] ||
					len(classOwnedArray(t, signature["parameters"])) != 0 || len(classOwnedArray(t, constructor["body"])) != 0 {
					t.Fatal("public access changed the actual constructor signature or fabricated a body")
				}
				if kind == "explicit" {
					token := "constructor() {}"
					if strings.Contains(test.source, "public "+token) {
						token = "public " + token
					}
					classLocalOrigin(t, constructor["origin"], test.source, token, strings.Index(test.source, "class "+name))
				} else if !reflect.DeepEqual(constructor["origin"], declaration["origin"]) {
					t.Fatal("implicit access fact fabricated a constructor occurrence")
				}
				if declaration["runtimeKey"] != classMultiExpectedKey(t, view, declaration) {
					t.Fatal("public access profile lost independently checked complete v2 allocation provenance")
				}
				baseName := test.bases[name]
				if baseName == "" {
					if declaration["base"] != "" || constructor["implicitForward"] != nil || constructor["initializationPhase"] != "before-body" {
						t.Fatal("root public constructor acquired an invented base or forwarding")
					}
				} else {
					base := classOwnedNamed(t, classes, baseName)
					baseConstructor := classOwnedObject(t, base["constructor"])
					forward := classOwnedObject(t, constructor["implicitForward"])
					if declaration["base"] != base["id"] || baseConstructor["access"] != "public" ||
						forward["target"] != baseConstructor["id"] || constructor["initializationPhase"] != "after-successful-super" ||
						len(classOwnedArray(t, forward["parameters"])) != 0 || constructor["superCall"] != "" {
						t.Fatal("implicit constructor public access lacks its complete actual public base chain")
					}
				}
			}
		})
	}
}
