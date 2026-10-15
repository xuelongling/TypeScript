// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Validates owned request inputs and configures one official checker for all supplied source.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/tspath"
	"maps"
	"strings"
)

func canonicalProjectPath(name string) string {
	parts := []string{"/"}
	for part := range strings.SplitSeq(name, "/") {
		switch part {
		case "", ".":
			continue
		case "..":
			if len(parts) > 1 {
				parts = parts[:len(parts)-1]
			}
		default:
			parts = append(parts, part)
		}
	}
	return tspath.GetPathFromPathComponents(parts)
}

func valid(r request) bool {
	if r.Declarations.MarkerProfile != nil && (r.Declarations.Version != conversionVersion || r.Config.DecoratorMode != "standard" || !validMarkerProfile(r)) || r.Declarations.MarkerProfile == nil && r.Config.DecoratorMode != "" {
		return false
	}
	if (r.Declarations.Version == "minimal-integers-v3" || r.Declarations.Version == "minimal-floats-v4" || r.Declarations.Version == conversionVersion) && !validIntegerTarget(r.Target) {
		return false
	}
	if r.SourceRoot == "" || !strings.HasPrefix(r.SourceRoot, "/") || canonicalProjectPath(r.SourceRoot) != r.SourceRoot || r.RootFile == "" ||
		(r.Declarations.Version != "minimal-bool-v1" && r.Declarations.Version != "minimal-scalars-v2" && r.Declarations.Version != "minimal-integers-v3" && r.Declarations.Version != "minimal-floats-v4" && r.Declarations.Version != conversionVersion) || !r.Config.Strict || !r.Config.NoEmit || r.Config.AllowJS || len(r.Files) == 0 || len(r.Declarations.Paths) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, file := range r.Files {
		if file.Path != canonicalProjectPath(file.Path) || !strings.HasPrefix(file.Path, r.SourceRoot+"/") || seen[file.Path] ||
			(file.Path != r.RootFile && !strings.HasSuffix(file.Path, ".d.ts") && (r.CompilationClosure == nil || !strings.HasSuffix(file.Path, ".ts"))) {
			return false
		}
		seen[file.Path] = true
	}
	if !seen[r.RootFile] || !strings.HasSuffix(r.RootFile, ".ts") || strings.HasSuffix(r.RootFile, ".d.ts") {
		return false
	}
	declarations := map[string]bool{}
	for _, name := range r.Declarations.Paths {
		if !seen[name] || !strings.HasSuffix(name, ".d.ts") || declarations[name] {
			return false
		}
		declarations[name] = true
	}
	return true
}

// The configured checker remains the sole authority for all supplied source files.
// Per-file context maps are joined before semantic checking; no upstream session
// object is exported in the owned response.
func programScalarAssignmentContexts(r request, program *compiler.Program, check *checker.Checker, root *ast.SourceFile, functions map[*ast.Symbol]functionInfo) map[*ast.Node]*checker.Type {
	if r.CompilationClosure == nil {
		return scalarAssignmentContexts(root, check, functions, r.Target)
	}
	contexts := map[*ast.Node]*checker.Type{}
	for _, file := range program.GetSourceFiles() {
		if !strings.HasSuffix(file.FileName(), ".d.ts") {
			maps.Copy(contexts, scalarAssignmentContexts(file, check, functions, r.Target))
		}
	}
	return contexts
}

func verifyProgramConversionNominals(r request, program *compiler.Program, check *checker.Checker, root *ast.SourceFile, facts *conversionFacts) *response {
	if r.CompilationClosure == nil {
		return verifyConversionNominals(root, check, facts)
	}
	for _, file := range program.GetSourceFiles() {
		if !strings.HasSuffix(file.FileName(), ".d.ts") {
			if failure := verifyConversionNominals(file, check, facts); failure != nil {
				return failure
			}
		}
	}
	return nil
}
