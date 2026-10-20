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

// Every unsupported form below is an officially valid Program. Capability
// rejection must retain its own cause/occurrence and expose no partial graph.
func TestOwnedClassSourceRejectsUnsupportedTerminalIfForms(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, member, token, detail string
	}{
		{"missing-else", "choose(flag: bool): void { if (flag) { return; } }",
			"if (flag) { return; }", "terminal if requires explicit then and else blocks"},
		{"unbraced", "choose(flag: bool, other: Box): Box { if (flag) return this; else return other; }",
			"if (flag) return this; else return other;", "terminal if requires explicit then and else blocks"},
		{"extra-statement", "choose(flag: bool, other: Box): Box { other; if (flag) { return this; } else { return other; } }",
			"if (flag) { return this; } else { return other; }", "terminal if must be the sole method statement"},
		{"multiple-branch-statements", "choose(flag: bool, other: Box): Box { if (flag) { other; return this; } else { return other; } }",
			"{ other; return this; }", "terminal if block requires exactly one reference return"},
		{"nested-if", "choose(flag: bool, other: Box): Box { if (flag) { if (flag) { return this; } else { return other; } } else { return other; } }",
			"{ if (flag) { return this; } else { return other; } }", "terminal if block requires exactly one reference return"},
		{"number-truthiness", "choose(flag: number, other: Box): Box { if (flag) { return this; } else { return other; } }",
			"flag)", "terminal if condition requires official BooleanLike type"},
		{"scalar-result", "choose(flag: bool, other: number): number { if (flag) { return other; } else { return other; } }",
			"if (flag) { return other; } else { return other; }", "terminal if requires a reference method return"},
		{"constructor", "constructor(flag: bool) { if (flag) { return; } else { return; } }",
			"if (flag) { return; } else { return; }", "class body statement kind: KindIfStatement"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			source := "// 😀\nexport class Box { " + test.member + " }"
			program, _ := genericProgramForTest(source)
			if err := checkClassDiagnostics(t.Context(), program); err != nil {
				t.Fatalf("terminal if boundary fixture is not officially valid: %v", err)
			}
			inputs := []classInput{}
			for _, file := range program.GetSourceFiles() {
				inputs = append(inputs, classInput{Path: file.FileName(), SHA256: classTextSHA(file.Text()), Declaration: file.IsDeclarationFile})
			}
			graph, err := exportClassGraph(t.Context(), program, []*ast.SourceFile{program.GetSourceFile("/original/main.ts")}, classExportOptions{
				Scope: "class:terminal-if-boundary", Inputs: inputs, MaxNodes: 4096, MaxDepth: 64,
			})
			var failure *classFailure
			if !errors.As(err, &failure) || failure.Kind != "unsupported" || failure.Detail != test.detail {
				t.Fatalf("terminal if boundary lost its precise capability cause: %v", err)
			}
			start := strings.Index(source, test.token)
			if start < 0 {
				t.Fatal("boundary fixture anchor missing")
			}
			end := start + len(test.token)
			if test.name == "number-truthiness" {
				end-- // Only the condition identifier, excluding its close parenthesis.
			}
			if failure.Origin.Source != "/original/main.ts" ||
				failure.Origin.Start != len(utf16.Encode([]rune(source[:start]))) ||
				failure.Origin.End != len(utf16.Encode([]rune(source[:end]))) {
				t.Fatalf("terminal if boundary lost its exact UTF16 occurrence: %+v", failure.Origin)
			}
			if !reflect.DeepEqual(graph, classGraph{}) {
				t.Fatal("unsupported terminal if exposed a partial graph")
			}
		})
	}
}

// A nonterminating branch in this annotated method is an official error before
// owned projection; it must not be reclassified as a successful branch fact.
func TestOwnedClassSourceKeepsOfficialTerminalIfMissingReturnFailure(t *testing.T) {
	t.Parallel()
	source := "// 😀\nexport class Box { choose(flag: bool, other: Box): Box { if (flag) { other; } else { return other; } } }"
	graph, err := classIdentityExport(t, source, nil)
	var failure *classFailure
	if !errors.As(err, &failure) || failure.Kind != "official" || failure.Code != 2366 ||
		failure.Origin.Source != "/original/main.ts" {
		t.Fatalf("nonterminating branch lost the real official return diagnostic: %v", err)
	}
	if !reflect.DeepEqual(graph, classGraph{}) {
		t.Fatal("official missing return exposed a partial owned graph")
	}
}
