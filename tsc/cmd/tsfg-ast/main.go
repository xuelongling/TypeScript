// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// tsfg-ast is a compiler-side source intake probe, not a compiler or JS runner.
package main

import (
	"encoding/json"
	"fmt"
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/parser"
	"os"
	"path/filepath"
)

type statement struct {
	Kind  string `json:"kind"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

func main() {
	if len(os.Args) != 2 || filepath.Ext(os.Args[1]) != ".ts" {
		fmt.Fprintln(os.Stderr, "expected one TypeScript source file")
		os.Exit(2)
	}
	text, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	file := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: filepath.Base(os.Args[1])}, string(text), core.ScriptKindTS)
	statements := []statement{}
	for _, node := range file.Statements.Nodes {
		statements = append(statements, statement{node.Kind.String(), node.Pos(), node.End()})
	}
	identifiers := []string{}
	var visit ast.Visitor
	visit = func(node *ast.Node) bool {
		if node.Kind == ast.KindIdentifier {
			identifiers = append(identifiers, node.Text())
		}
		node.ForEachChild(visit)
		return false
	}
	file.ForEachChild(visit)
	result := struct {
		Kind                 string      `json:"kind"`
		Statements           []statement `json:"statements"`
		Identifiers          []string    `json:"identifiers"`
		ParseDiagnosticCount int         `json:"parseDiagnosticCount"`
	}{file.AsNode().Kind.String(), statements, identifiers, len(file.Diagnostics())}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
