// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

func mcBudgetRequest23d(t *testing.T, tc mcBudgetCase23d) mcObject23d {
	r := mcMutableRequest23d(t, "first-rr")
	input := mcInputObject23d(r)
	files := []any{}
	for _, item := range input["files"].([]any) {
		row := item.(map[string]any)
		path := row["path"].(string)
		if strings.HasSuffix(path, ".d.ts") {
			files = append(files, row)
		} else if path == "/src/app.ts" {
			files = append(files, mcObject23d{"path": path, "content": tc.source, "sha256": strings.TrimPrefix(mcHash23d([]byte(tc.source)), "sha256:")})
		}
	}
	input["files"] = files
	sort.Slice(tc.roots, func(i, j int) bool {
		return moduleCallableBudgetRootKey23d(tc.roots[i]) < moduleCallableBudgetRootKey23d(tc.roots[j])
	})
	roots := []any{}
	for _, root := range tc.roots {
		roots = append(roots, mcObject23d{"kind": root.Kind, "sourcePath": root.SourcePath, "name": root.Name})
	}
	input["compilationClosure"] = mcObject23d{"roots": roots, "requiredRoots": []any{}}
	return r
}
func moduleCallableBudgetRootKey23d(root closureRoot) string {
	return root.Kind + "\x00" + root.SourcePath + "\x00" + root.Name
}
func mcBudgetCapture23d(t *testing.T, tc mcBudgetCase23d) {
	r := mcBudgetRequest23d(t, tc)
	input := mcRequestBytes23d(t, r)
	official := mcBudgetOfficial23d(t, input)
	if official["functions"] != tc.functions || official["globals"] != tc.globals || official["ast"] > 16384 || official["calls"] > 16384 {
		t.Fatalf("independent official structural control differs: %+v", official)
	}
	output := &mcPublicWriter23d{}
	before := bytes.Clone(input)
	if err := runFrontendCommand(t.Context(), []string{"--module-callable-facts-io-v2"}, bytes.NewReader(input), output); err != nil {
		t.Fatalf("independently legal budget Source must publish complete packet, not an earlier guard: %v official=%+v", err, official)
	}
	if output.writes != 1 || !bytes.Equal(input, before) {
		t.Fatal("budget capture requires one complete write and unchanged input")
	}
	var packet mcObject23d
	if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), append(mcCanonical23d(t, packet), '\n')) || packet["requestId"] != mcDomain23d(t, "tsfg.module-callable-facts.request.v2", r) || packet["schemaHash"] != mcHash23d(moduleCallableSchema23d) {
		t.Fatal("budget complete packet/request identity differs")
	}
	packageID := packet["packageId"]
	delete(packet, "packageId")
	if packageID != mcDomain23d(t, "tsfg.module-callable-facts.package.v2", packet) {
		t.Fatal("budget packet complete digest differs")
	}
	packet["packageId"] = packageID
	payload := packet["payload"].(map[string]any)
	if payload["inputIdentity"] != mcDomain23d(t, "tsfg.module-callable-facts.input.v2", r["input"]) {
		t.Fatal("budget input identity differs")
	}
	facts := payload["facts"].(map[string]any)
	if facts["status"] != "ok" || len(facts["functions"].([]any)) != tc.functions || len(facts["symbols"].([]any)) != official["symbols"] || len(facts["modules"].([]any)) != 1 || len(payload["annotations"].([]any)) != tc.functions {
		t.Fatal("budget complete declaration/facts coverage differs")
	}
	bundle := mcObject23d{"name": tc.name, "kind": tc.kind, "scale": tc.scale, "official": official, "request": r, "packet": packet}
	raw := mcCanonical23d(t, bundle)
	tokens, depth, maxString := mcBudgetJsonMetrics23d(packet, 1)
	t.Logf("TSFG_MODULE_CALLABLE_23D_BUDGET_CONTROL name=%s functions=%d globals=%d ast=%d calls=%d symbols=%d packetBytes=%d packetTokens=%d packetDepth=%d maxStringBytes=%d", tc.name, tc.functions, tc.globals, official["ast"], official["calls"], official["symbols"], output.Len(), tokens, depth, maxString)
	t.Logf("TSFG_MODULE_CALLABLE_23D_BUDGET_CAPTURE name=%s bytes=%d digest=%s base64=%s", tc.name, len(raw), mcHash23d(raw), base64.StdEncoding.EncodeToString(raw))
}

// Count complete official nodes and declarations, independent of producer facts.
func mcBudgetOfficial23d(t *testing.T, input []byte) map[string]int {
	var envelope struct{ Input mcInput23d }
	if err := json.Unmarshal(input, &envelope); err != nil {
		t.Fatal(err)
	}
	contents := map[string]string{}
	names := []string{}
	for _, file := range envelope.Input.Files {
		contents[file.Path] = file.Content
		names = append(names, file.Path)
	}
	program := compiler.NewProgram(compiler.ProgramOptions{Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{FileNames: names, CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext}}}, Host: compiler.NewCompilerHost(envelope.Input.SourceRoot, vfstest.FromMap(contents, true), envelope.Input.SourceRoot, nil, nil), SingleThreaded: core.TSTrue})
	if ds := program.GetSyntacticDiagnostics(t.Context(), nil); len(ds) != 0 {
		t.Fatalf("budget independent official syntax: %+v", ds)
	}
	if ds := program.GetSemanticDiagnostics(t.Context(), nil); len(ds) != 0 {
		t.Fatalf("budget independent official semantic: %+v", ds)
	}
	if ds := program.GetProgramDiagnostics(); len(ds) != 0 {
		t.Fatalf("budget independent official program: %+v", ds)
	}
	if ds := program.GetGlobalDiagnostics(t.Context()); len(ds) != 0 {
		t.Fatalf("budget independent official global: %+v", ds)
	}
	counts := map[string]int{"functions": 0, "globals": 0, "symbols": 0, "ast": 0, "calls": 0}
	consumed := map[string]bool{}
	for _, file := range program.GetSourceFiles() {
		if contents[file.FileName()] != file.Text() || consumed[file.FileName()] {
			t.Fatal("budget official original source closure differs")
		}
		consumed[file.FileName()] = true
		if strings.HasSuffix(file.FileName(), ".d.ts") {
			continue
		}
		for _, node := range file.Statements.Nodes {
			if node.Kind == ast.KindFunctionDeclaration {
				counts["functions"]++
				counts["symbols"] += 1 + len(node.AsFunctionDeclaration().Parameters.Nodes)
			} else if node.Kind == ast.KindVariableStatement {
				n := len(node.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes)
				counts["globals"] += n
				counts["symbols"] += n
			}
		}
		var visit func(*ast.Node) bool
		visit = func(node *ast.Node) bool {
			if ast.IsStatement(node) || ast.IsExpression(node) {
				counts["ast"]++
			}
			if node.Kind == ast.KindCallExpression {
				counts["calls"]++
			}
			if node.Kind == ast.KindVariableDeclaration && node.Parent != nil && node.Parent.Parent != nil && node.Parent.Parent.Parent != file.AsNode() {
				counts["symbols"]++
			}
			node.ForEachChild(visit)
			return false
		}
		file.ForEachChild(visit)
	}
	if len(consumed) != len(contents) {
		t.Fatal("budget official consumed closure incomplete")
	}
	return counts
}
func mcBudgetJsonMetrics23d(v any, level int) (tokens, depth, maxString int) {
	depth = level
	switch x := v.(type) {
	case map[string]any:
		tokens = 2
		for key, value := range x {
			tokens++
			if len([]byte(key)) > maxString {
				maxString = len([]byte(key))
			}
			n, d, s := mcBudgetJsonMetrics23d(value, level+1)
			tokens += n
			if d > depth {
				depth = d
			}
			if s > maxString {
				maxString = s
			}
		}
	case []any:
		tokens = 2
		for _, value := range x {
			n, d, s := mcBudgetJsonMetrics23d(value, level+1)
			tokens += n
			if d > depth {
				depth = d
			}
			if s > maxString {
				maxString = s
			}
		}
	case string:
		tokens = 1
		maxString = len([]byte(x))
	default:
		tokens = 1
	}
	return
}
