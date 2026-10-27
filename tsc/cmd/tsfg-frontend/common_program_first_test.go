// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tsjson "github.com/microsoft/typescript-go/internal/json"
)

// The old exact protocol remains a refusal control after the causal RED.
// Original desired-capability assertions and raw failure are retained outside.
func TestCommonProgramLegacyBoundary37n(t *testing.T) {
	for _, name := range []string{"positive", "reachable-managed", "reference-value"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("testdata", "common-program-v1", name+".ts"))
			if err != nil {
				t.Fatal(err)
			}
			input := genericArrayRequest25f(t, string(source))
			if root := os.Getenv("TSFG_COMMON_PROGRAM_CAPTURE_ROOT"); root != "" {
				if err := os.MkdirAll(root, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, name+"-legacy-request.json"), input, 0600); err != nil {
					t.Fatal(err)
				}
			}
			var output bytes.Buffer
			if err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(input), &output); err == nil || output.Len() != 0 {
				t.Fatal("old v8 must reject the unsupported complete source without a partial packet")
			}
		})
	}
}

func commonProgramRequest37n(t *testing.T, name string) []byte {
	t.Helper()
	file := filepath.Join("testdata", "common-program-v1", name+".ts")
	if name == "mixed" {
		file = filepath.Join("testdata", "common-program-v1", "mixed", "main.ts")
	}
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var request map[string]any
	if err := json.Unmarshal(genericArrayRequest25f(t, string(source)), &request); err != nil {
		t.Fatal(err)
	}
	schema, err := os.ReadFile("testdata/common-program-v2/schema.json")
	if err != nil {
		t.Fatal(err)
	}
	request["family"], request["version"], request["schemaHash"] = "tsfg.program-facts-request-candidate", 2, "sha256:"+arrayFactsSHA(schema)
	input := request["input"].(map[string]any)
	if name == "mixed" {
		files := []arrayFactsInputFile{}
		for _, name := range []string{"barrel", "data", "main", "state"} {
			data, err := os.ReadFile(filepath.Join("testdata", "common-program-v1", "mixed", name+".ts"))
			if err != nil {
				t.Fatal(err)
			}
			files = append(files, arrayFactsInputFile{Path: "/src/" + name + ".ts", Content: string(data), SHA256: arrayFactsSHA(data)})
		}
		dts, err := os.ReadFile("testdata/array-facts-v8/minimal-arrays-v8.d.ts")
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, arrayFactsInputFile{Path: "/src/minimal-arrays-v8.d.ts", Content: string(dts), SHA256: arrayFactsSHA(dts)})
		slices.SortFunc(files, func(a, b arrayFactsInputFile) int { return strings.Compare(a.Path, b.Path) })
		input["files"] = files
	}
	input["hostImports"] = []commonProgramHostImport{}
	input["target"] = map[string]string{"target": "windows-x86_64-msvc", "triple": "x86_64-pc-windows-msvc", "pointerBits": "64", "llvmDataLayout": "e-m:w-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128"}
	root := func(kind, name string) map[string]string {
		return map[string]string{"kind": kind, "sourcePath": "/src/main.ts", "name": name}
	}
	input["compilationClosure"] = map[string]any{"roots": []any{root("application", "ordinary"), root("kernel", "left"), root("kernel", "right"), root("module-initialization", "$moduleInit")}, "requiredRoots": []any{root("kernel", "left"), root("kernel", "right")}}
	if name == "mixed" {
		roots := []any{root("application", "ordinary"), root("application", "view"), root("kernel", "left"), root("kernel", "right")}
		for _, name := range []string{"barrel", "data", "main", "state"} {
			roots = append(roots, map[string]string{"kind": "module-initialization", "sourcePath": "/src/" + name + ".ts", "name": "$moduleInit"})
		}
		input["compilationClosure"].(map[string]any)["roots"] = roots
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	canonical := tsjson.Value(encoded)
	if err := canonical.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	return append(canonical, '\n')
}

func TestCommonProgramMixedPublic37n(t *testing.T) {
	input := commonProgramRequest37n(t, "mixed")
	var output bytes.Buffer
	if err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	if root := os.Getenv("TSFG_COMMON_PROGRAM_CAPTURE_ROOT"); root != "" {
		for suffix, data := range map[string][]byte{"request.json": input, "package.json": output.Bytes()} {
			if err := os.WriteFile(filepath.Join(root, "mixed-"+suffix), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	var packet commonProgramPackage
	if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	p := packet.Payload
	if len(p.Files) != 5 || len(p.Modules) != 4 || len(p.Functions) != 7 || len(p.Globals) != 2 || len(p.Closure.Roots) != 8 {
		t.Fatal("the complete mixed source inventory must remain intact")
	}
	var state *commonProgramModule
	var ordinary, run *arrayGenericFunction
	for i := range p.Modules {
		if p.Modules[i].SourcePath == "/src/state.ts" {
			state = &p.Modules[i]
		}
	}
	for i := range p.Functions {
		if p.Functions[i].Name == "ordinary" {
			ordinary = &p.Functions[i]
		}
		if p.Functions[i].Name == "run" {
			run = &p.Functions[i]
		}
	}
	if state == nil || len(state.Initialization) != 2 || state.Initialization[1].Initializer.Left.Binding != p.Globals[0].Binding.ID {
		t.Fatal("initialization order and its actual prior global read must survive")
	}
	if ordinary == nil || ordinary.Body[1].FinallyBlock == nil || ordinary.Body[1].FinallyBlock.Body[0].Value.Kind != "call" {
		t.Fatal("the ordinary finally region and original namespace call were dropped")
	}
	if run == nil || len(run.Body) != 3 || run.Body[0].Initializer.Kind != "arrayLiteral" || run.Body[1].Initializer.Kind != "call" || run.Body[2].Value.Kind != "checkedElementAccess" {
		t.Fatal("the actual original Array/ReadSpan function must survive together with the other domains")
	}
}

func TestCommonProgramFullPublic37n(t *testing.T) {
	for _, name := range []string{"positive", "reachable-managed", "reference-value"} {
		t.Run(name, func(t *testing.T) {
			input := commonProgramRequest37n(t, name)
			var output bytes.Buffer
			if err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(input), &output); err != nil {
				t.Fatalf("complete official common Program: %v", err)
			}
			if root := os.Getenv("TSFG_COMMON_PROGRAM_CAPTURE_ROOT"); root != "" {
				if err := os.MkdirAll(root, 0700); err != nil {
					t.Fatal(err)
				}
				for suffix, data := range map[string][]byte{"request.json": input, "package.json": output.Bytes()} {
					if err := os.WriteFile(filepath.Join(root, name+"-"+suffix), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			var packet commonProgramPackage
			if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}
			p := packet.Payload
			if packet.Family != "tsfg.program-facts-package-candidate" || packet.Version != 2 || len(p.Functions) != 4 || len(p.Globals) != 2 || len(p.Modules) != 1 || len(p.Closure.Roots) != 4 {
				t.Fatal("complete source inventory was pruned or split")
			}
			if p.Globals[0].Binding.Name != "leftTotal" || p.Globals[1].Binding.Name != "rightTotal" || len(p.Modules[0].Initialization) != 2 || p.Modules[0].Initialization[0].Binding != p.Globals[0].Binding.ID || p.Modules[0].Initialization[1].Binding != p.Globals[1].Binding.ID {
				t.Fatal("ordered actual global initialization or original binding identity differs")
			}
			ordinary := p.Functions[3]
			if ordinary.Name != "ordinary" || len(ordinary.Body) != 2 || ordinary.Body[0].Initializer.Kind != "new" || ordinary.Body[1].Kind != "try" || ordinary.Body[1].TryBlock.Body[0].Kind != "throw" || ordinary.Body[1].CatchClause.Binding.DeclaredType != "unknown" || ordinary.Body[1].CatchClause.Block.Body[0].Value.Value.Kind != "as" {
				t.Fatal("ordinary managed allocation, throw, unknown catch or assertion was omitted")
			}
		})
	}
}
