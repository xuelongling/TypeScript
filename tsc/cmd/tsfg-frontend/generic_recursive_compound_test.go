// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/json"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

// Literal graph topology is independent of the owned graph and projector.
var recursiveCompoundNames = []string{"leaf", "mid", "route", "self", "left", "right"}
var recursiveCompoundTargets = []string{"", "leaf", "mid", "self", "right", "left"}

func recursiveCompoundSource(signature string) string {
	var source strings.Builder
	source.WriteString("// 😀 original recursive compound signatures\n")
	for i, name := range recursiveCompoundNames {
		body := "value"
		if recursiveCompoundTargets[i] != "" {
			body = recursiveCompoundTargets[i] + "<T>(value)"
		}
		fmt.Fprintf(&source, "export function %s<T>(value: %s): %s { return %s; }\n", name, signature, signature, body)
	}
	return source.String()
}

// Fresh official Program and checker only: this oracle never exports or decodes
// owned data, and resolves original symbols plus concrete probe signatures.
func recursiveCompoundOfficial(t *testing.T, source string) {
	t.Helper()
	var probe strings.Builder
	fmt.Fprintf(&probe, "import { %s } from './main.js';\n", strings.Join(recursiveCompoundNames, ", "))
	for _, name := range recursiveCompoundNames {
		fmt.Fprintf(&probe, "%s<boolean>(true); %s<number>(1);\n", name, name)
	}
	inputs := map[string]string{"/original/main.ts": source, "/original/probe.ts": probe.String(), "/original/scalars.d.ts": scalarDeclarations}
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{FileNames: []string{"/original/main.ts", "/original/probe.ts", "/original/scalars.d.ts"},
			CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext, Module: core.ModuleKindESNext}}},
		Host: compiler.NewCompilerHost("/original", vfstest.FromMap(inputs, true), "/original", nil, nil), SingleThreaded: core.TSTrue})
	for _, file := range program.GetSourceFiles() {
		if ds := program.GetSyntacticDiagnostics(t.Context(), file); len(ds) != 0 {
			t.Fatalf("official syntax: %v", ds)
		}
		if ds := program.GetSemanticDiagnostics(t.Context(), file); len(ds) != 0 {
			t.Fatalf("official semantics: %v", ds)
		}
	}
	if len(program.GetProgramDiagnostics()) != 0 || len(program.GetGlobalDiagnostics(t.Context())) != 0 {
		t.Fatal("official program/global diagnostics")
	}
	main := program.GetSourceFile("/original/main.ts")
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), main)
	defer release()
	declarations := map[string]*ast.Node{}
	for _, node := range main.Statements.Nodes {
		declarations[node.Name().Text()] = node
	}
	for i, name := range recursiveCompoundNames {
		node := declarations[name]
		d := node.AsFunctionDeclaration()
		body := d.Body.AsBlock().Statements.Nodes[0].AsReturnStatement().Expression
		value := check.GetSymbolAtLocation(d.Parameters.Nodes[0].Name())
		if recursiveCompoundTargets[i] == "" {
			if body.Kind != ast.KindIdentifier || check.GetSymbolAtLocation(body) != value {
				t.Fatal("official leaf symbol")
			}
		} else {
			call := body.AsCallExpression()
			target := declarations[recursiveCompoundTargets[i]]
			sig := check.GetResolvedSignature(body)
			if sig == nil || sig.Declaration() != target || check.GetSymbolAtLocation(call.Expression) != check.GetSymbolAtLocation(target.Name()) ||
				len(call.TypeArguments.Nodes) != 1 || len(call.Arguments.Nodes) != 1 || check.GetSymbolAtLocation(call.Arguments.Nodes[0]) != value ||
				check.GetSymbolAtLocation(call.TypeArguments.Nodes[0].AsTypeReferenceNode().TypeName) != check.GetSymbolAtLocation(d.TypeParameters.Nodes[0].Name()) {
				t.Fatal("official full call symbol/type/value binding")
			}
		}
	}
	var calls []*ast.Node
	var visit func(*ast.Node) bool
	visit = func(node *ast.Node) bool {
		if node.Kind == ast.KindCallExpression {
			calls = append(calls, node)
		}
		node.ForEachChild(visit)
		return false
	}
	program.GetSourceFile("/original/probe.ts").AsNode().ForEachChild(visit)
	if len(calls) != 12 {
		t.Fatal("official concrete probe count")
	}
	for i, node := range calls {
		expected := check.GetBooleanType()
		if i%2 == 1 {
			expected = check.GetNumberType()
		}
		sig := check.GetResolvedSignature(node)
		if sig == nil || sig.Declaration() != declarations[recursiveCompoundNames[i/2]] || len(sig.Parameters()) != 1 ||
			check.GetTypeOfSymbol(sig.Parameters()[0]) != expected || check.GetReturnTypeOfSignature(sig) != expected || check.GetTypeAtLocation(node) != expected {
			t.Fatal("independent concrete probe signature")
		}
	}
	t.Log("independent official original symbols and twelve bool/f64 signatures PASS")
}

func TestOwnedGenericRecursiveCompoundColdClosure(t *testing.T) {
	for _, family := range []struct{ name, signature string }{
		{"conditional", "T extends boolean ? boolean : number"},
		{"mapped", "{ [K in 'value']: T }['value']"},
	} {
		t.Run(family.name, func(t *testing.T) {
			source := recursiveCompoundSource(family.signature)
			recursiveCompoundOfficial(t, source)
			// This helper destroys all producer VFS inputs before returning bytes.
			data := genericMappedColdSourceForTest(t, source)
			g, err := decodeGenericPackage(data)
			if err != nil {
				t.Fatal(err)
			}
			again, err := encodeGenericPackage(g)
			if err != nil || !bytes.Equal(data, again) {
				t.Fatalf("cold canonical package: %v", err)
			}
			requests := []genericPackageRoot{}
			for _, name := range []string{"route", "self", "left"} {
				for _, kind := range []string{"boolean", "number"} {
					requests = append(requests, genericPackageRootForTest(t, g, name, kind))
				}
			}
			requests = append(requests, requests[0])
			before := bytes.Clone(data)
			boolOnly, err := instantiateGenericPackageFacts(t.Context(), data, []genericPackageRoot{requests[0], requests[2], requests[4]})
			if err != nil || len(boolOnly.Instances) != 6 {
				t.Fatalf("initial bool-only cold closure: %v", err)
			}
			for _, instance := range boolOnly.Instances {
				if instance.ReturnType != "bool" || instance.TypeArguments[0].Kind != "boolean" {
					t.Fatal("initial cold closure already contained a number instance")
				}
			}
			// The number instantiations below did not exist in the preceding
			// request; only canonical package bytes connect the fresh requests.
			facts, err := instantiateGenericPackageFacts(t.Context(), data, requests)
			if err != nil {
				t.Fatal(err)
			}
			oracleBytes, err := os.ReadFile("testdata/generic-recursive/" + family.name + "-oracle.json")
			if err != nil {
				t.Fatal(err)
			}
			var oracle struct {
				Package  json.Value `json:"package"`
				Requests json.Value `json:"requests"`
				Result   json.Value `json:"result"`
			}
			if err := json.Unmarshal(oracleBytes, &oracle); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(data, mappedCallJSON14(t, oracle.Package)) || !bytes.Equal(mappedCallJSON14(t, requests), mappedCallJSON14(t, oracle.Requests)) ||
				!bytes.Equal(mappedCallJSON14(t, facts), mappedCallJSON14(t, oracle.Result)) {
				t.Fatal("entire cold package/request/result differs from independent literal source/topology oracle")
			}
			if len(facts.Instances) != 12 || len(facts.Roots) != 7 || facts.Roots[0] != facts.Roots[6] {
				t.Fatal("finite same-instance closure/reuse")
			}
			for _, instance := range facts.Instances {
				definition := ""
				for _, d := range g.Definitions {
					if d.Key == instance.Definition {
						definition = d.Name
					}
				}
				index := -1
				for i, name := range recursiveCompoundNames {
					if name == definition {
						index = i
					}
				}
				if index < 0 {
					t.Fatal("unexpected definition")
				}
				want := "bool"
				if instance.TypeArguments[0].Kind == "number" {
					want = "f64"
				}
				if instance.ReturnType != want || instance.Parameters[0].Type != want || instance.Body.Type != want {
					t.Fatal("independent official scalar facts")
				}
				target := recursiveCompoundTargets[index]
				if target == "" {
					if instance.Body.Kind != "parameter" || instance.Body.Parameter != 0 {
						t.Fatal("leaf fact")
					}
				} else {
					callee := genericPackageFactForTest(t, facts, instance.Body.Callee)
					if instance.Body.Kind != "call" || len(instance.Body.Arguments) != 1 || instance.Body.Arguments[0].Kind != "parameter" || instance.Body.Arguments[0].Parameter != 0 ||
						instance.Body.Arguments[0].Type != want || callee.Definition != genericDefinitionNamed(t, g, target).Key || callee.ReturnType != want {
						t.Fatal("complete recursive edge facts")
					}
				}
			}
			// New cold request, absent from the original root set, resolves through the
			// same finite closure without the original checker or its mapped instances.
			fresh, err := instantiateGenericPackageFacts(t.Context(), data, []genericPackageRoot{genericPackageRootForTest(t, g, "right", "number")})
			if err != nil || len(fresh.Instances) != 2 {
				t.Fatalf("cold new instantiation: %v", err)
			}
			if !bytes.Equal(data, before) {
				t.Fatal("caller bytes mutated")
			}
			record := mappedCallJSON14(t, struct {
				Requests []genericPackageRoot `json:"requests"`
				Result   genericPackageFacts  `json:"result"`
			}{requests, facts})
			t.Logf("TSFG_RECURSIVE_%s_PACKAGE %s", family.name, base64.StdEncoding.EncodeToString(data))
			t.Logf("TSFG_RECURSIVE_%s_RECORD %s", family.name, base64.StdEncoding.EncodeToString(record))
		})
	}
}

func TestOwnedGenericRecursiveDescriptorIdentity(t *testing.T) {
	descriptor, err := os.ReadFile("testdata/generic-recursive/descriptor-v7.json")
	if err != nil {
		t.Fatal(err)
	}
	if genericTextDigest(string(descriptor)) != genericPackageSchema {
		t.Fatal("private7 complete descriptor identity")
	}
}
