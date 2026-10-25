// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
)

// The original user programs and independent expectations remain immutable.
// Only the supported declaration profile/target and its input identity change.
func exceptionFinallyLegacy(t *testing.T, id string) (request, any, any) {
	t.Helper()
	const directory = "testdata/exception-package-corpus-v1/"
	manifest, err := os.ReadFile(directory + "manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct{ Records json.RawMessage }
	if err := json.Unmarshal(manifest, &table); err != nil {
		t.Fatal(err)
	}
	var records []exceptionCorpusRecordForTest
	var values any
	if err := json.Unmarshal(table.Records, &values); err != nil {
		t.Fatal(err)
	}
	if exceptionPackageExportsDigestForTest(nullableFactsBoundaryCanonical(t, values)) != "sha256:db7a05d8575154025f75ef4464b5082bb4add44bdd8e736edc28d42d04dd8990" {
		t.Fatal("original corpus identity")
	}
	if err := json.Unmarshal(table.Records, &records); err != nil {
		t.Fatal(err)
	}
	data := map[string][]byte{}
	for _, record := range records {
		if record.ID == id {
			for _, file := range record.Files {
				value, err := os.ReadFile(directory + file.Path)
				if err != nil || len(value) != file.Bytes || exceptionPackageExportsDigestForTest(value) != file.SHA256 {
					t.Fatalf("original %s: %v", file.Role, err)
				}
				data[file.Role] = value
			}
		}
	}
	var old request
	if err := json.Unmarshal(data["REQUEST"], &old); err != nil {
		t.Fatal(err)
	}
	if old.Declarations.Version != "minimal-scalars-v2" || old.Target != nil || !reflect.DeepEqual(old.Declarations.Paths, []string{"/src/minimal-scalars-v2.d.ts"}) {
		t.Fatal("original absent-target profile")
	}
	r := old
	profile := integerRequest("")
	r.Declarations, r.Target = profile.Declarations, profile.Target
	r.Files = nil
	for _, file := range old.Files {
		if file.Path != old.Declarations.Paths[0] {
			r.Files = append(r.Files, file)
		}
	}
	r.Files = append(r.Files, profile.Files[1])
	var expected map[string]any
	var prior any
	if err := json.Unmarshal(data["EXPECTED"], &expected); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data["OFFICIAL_ORACLE"], &prior); err != nil {
		t.Fatal(err)
	}
	paths := []string{}
	for _, file := range r.Files {
		paths = append(paths, file.Path)
	}
	slices.Sort(paths)
	expected["inputs"] = paths
	return r, expected, prior
}

// A fresh official Program establishes each declaration, parameter owner and
// complete nested region. No projected completion or Product IR is an oracle.
func exceptionFinallyOriginal(t *testing.T, r request, facts response) []any {
	t.Helper()
	program := throwBoundaryProgramForTest(t, r)
	throwBoundaryDiagnosticsForTest(t, program)
	if len(program.GetSourceFiles()) != len(r.Files) {
		t.Fatal("complete original input closure")
	}
	paths := []string{}
	for _, input := range r.Files {
		file := program.GetSourceFile(input.Path)
		if file == nil || file.Text() != input.Content {
			t.Fatal("original file bytes")
		}
		paths = append(paths, input.Path)
	}
	slices.Sort(paths)
	if !reflect.DeepEqual(facts.Inputs, paths) {
		t.Fatal("complete public input directory")
	}
	file := program.GetSourceFile(r.RootFile)
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	if !ast.IsExternalModule(file) || len(file.Statements.Nodes) != 2 || len(facts.Functions) != 2 || len(facts.Symbols) != 7 {
		t.Fatal("complete two-function original")
	}
	bindings := []map[string]any{}
	ids := map[*ast.Symbol]string{}
	seen := map[string]bool{}
	for i, name := range []string{"override", "nested"} {
		node, f := file.Statements.Nodes[i], facts.Functions[i]
		if node.Kind != ast.KindFunctionDeclaration || node.Name().Text() != name || f.Name != name || f.ReturnType != "i32" || f.Range != throwBoundaryNodeForTest(t, file, node) || (f.SourcePath != "" && f.SourcePath != r.RootFile) {
			t.Fatal("original function order and range")
		}
		fn := node.AsFunctionDeclaration()
		result := check.GetTypeFromTypeNode(fn.Type)
		if len(fn.Parameters.Nodes) != i+2 || len(f.Parameters) != i+2 || result == nil || result.Alias() == nil || result.Alias().Symbol() == nil || result.Alias().Symbol().Name != "i32" {
			t.Fatal("original i32 signature")
		}
		join := func(actual *ast.Node, id, kind string, writable bool) {
			t.Helper()
			symbol := check.GetSymbolAtLocation(actual.Name())
			if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != actual || ids[symbol] != "" || id == "" || seen[id] {
				t.Fatal("distinct original defining symbols")
			}
			found := false
			for _, s := range facts.Symbols {
				if s.ID == id {
					found = s.Name == actual.Name().Text() && s.Kind == kind && s.Type == "i32" && s.Writable == writable && s.Range == throwBoundaryNodeForTest(t, file, actual.Name()) && (s.SourcePath == "" || s.SourcePath == r.RootFile)
				}
			}
			if !found {
				t.Fatal("original defining symbol/type/range")
			}
			ids[symbol], seen[id] = id, true
		}
		join(node, f.Symbol, "function", false)
		for p, actual := range fn.Parameters.Nodes {
			parameter := f.Parameters[p]
			join(actual, parameter.ID, "parameter", true)
			annotation := actual.AsParameterDeclaration().Type
			if parameter.Name != actual.Name().Text() || parameter.Type != "i32" || parameter.Kind != "parameter" || !parameter.Writable || parameter.Range != throwBoundaryNodeForTest(t, file, actual.Name()) || parameter.Presence == nil || parameter.Presence.Range != throwBoundaryNodeForTest(t, file, actual) || parameter.Presence.Annotation != throwBoundaryNodeForTest(t, file, annotation) || parameter.Presence.Undefined || parameter.Presence.Null || parameter.Presence.Optional || parameter.Presence.Default || check.GetTypeAtLocation(actual.Name()) != check.GetTypeFromTypeNode(annotation) {
				t.Fatal("own required annotated parameter")
			}
		}
		var block func(*ast.Node, []statement)
		block = func(original *ast.Node, owned []statement) {
			t.Helper()
			if original.Kind != ast.KindBlock || len(original.AsBlock().Statements.Nodes) != len(owned) {
				t.Fatal("complete ordered region")
			}
			for j, source := range original.AsBlock().Statements.Nodes {
				actual := owned[j]
				if actual.Range != throwBoundaryNodeForTest(t, file, source) {
					t.Fatal("original statement UTF16")
				}
				switch source.Kind {
				case ast.KindTryStatement:
					attempt := source.AsTryStatement()
					if actual.Kind != "try" || actual.Exception == nil || attempt.CatchClause != nil || actual.Exception.CatchClause != nil || attempt.FinallyBlock == nil || actual.Exception.FinallyBlock == nil || actual.Exception.TryBlock.Range != throwBoundaryNodeForTest(t, file, attempt.TryBlock) || actual.Exception.FinallyBlock.Range != throwBoundaryNodeForTest(t, file, attempt.FinallyBlock) {
						t.Fatal("original present try/finally without invented catch")
					}
					block(attempt.TryBlock, actual.Exception.TryBlock.Body)
					block(attempt.FinallyBlock, actual.Exception.FinallyBlock.Body)
				case ast.KindReturnStatement, ast.KindThrowStatement:
					kind := "return"
					var value *ast.Node
					if source.Kind == ast.KindThrowStatement {
						kind, value = "throw", source.AsThrowStatement().Expression
					} else {
						value = source.AsReturnStatement().Expression
					}
					if actual.Kind != kind || value == nil || value.Kind != ast.KindIdentifier || actual.Expression == nil || actual.Expression.Kind != "reference" || actual.Expression.Type != "i32" || actual.Expression.Symbol != ids[check.GetSymbolAtLocation(value)] || actual.Expression.Symbol == "" || actual.Expression.Range != throwBoundaryNodeForTest(t, file, value) || check.GetTypeAtLocation(value) != check.GetTypeFromTypeNode(fn.Type) {
						t.Fatal("original return/throw uses its own actual parameter")
					}
				default:
					t.Fatal("unexpected original statement")
				}
			}
		}
		block(fn.Body, f.Body)
		bindings = append(bindings, map[string]any{"name": name, "target": f.Symbol, "sourcePath": r.RootFile, "range": throwBoundaryNodeForTest(t, file, node), "definition": throwBoundaryNodeForTest(t, file, node.Name())})
	}
	if len(ids) != 7 || len(check.GetExportsOfModule(file.AsNode().Symbol())) != 2 {
		t.Fatal("complete original declarations/exports")
	}
	slices.SortFunc(bindings, func(a, b map[string]any) int { return strings.Compare(a["name"].(string), b["name"].(string)) })
	t.Log("fresh official Program: both complete finally overrides, own parameter declarations/types/symbols, UTF16 and complete exports; no execution claim")
	return []any{map[string]any{"sourcePath": r.RootFile, "external": true, "bindings": bindings}}
}

// This fifth real input isolates a literal-true loop with no normal exit. The
// Source oracle supplies original syntax/bindings, not a canComplete assertion.
func exceptionForeverOriginal(t *testing.T, r request, facts response) []any {
	t.Helper()
	program := throwBoundaryProgramForTest(t, r)
	throwBoundaryDiagnosticsForTest(t, program)
	paths := []string{}
	for _, input := range r.Files {
		file := program.GetSourceFile(input.Path)
		if file == nil || file.Text() != input.Content {
			t.Fatal("forever original input bytes")
		}
		paths = append(paths, input.Path)
	}
	slices.Sort(paths)
	if len(program.GetSourceFiles()) != len(paths) || !reflect.DeepEqual(facts.Inputs, paths) {
		t.Fatal("forever complete official input directory")
	}
	file := program.GetSourceFile(r.RootFile)
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	if len(file.Statements.Nodes) != 1 || file.Statements.Nodes[0].Kind != ast.KindFunctionDeclaration || !ast.IsExternalModule(file) || len(facts.Functions) != 1 || len(facts.Symbols) != 2 {
		t.Fatal("forever complete function set")
	}
	node, f := file.Statements.Nodes[0], facts.Functions[0]
	fn := node.AsFunctionDeclaration()
	if node.Name().Text() != "forever" || f.Name != "forever" || f.ReturnType != "i32" || f.Range != throwBoundaryNodeForTest(t, file, node) || (f.SourcePath != "" && f.SourcePath != r.RootFile) || len(fn.Parameters.Nodes) != 1 || len(f.Parameters) != 1 || len(fn.Body.AsBlock().Statements.Nodes) != 1 || len(f.Body) != 1 {
		t.Fatal("forever complete original function")
	}
	parameter, p := fn.Parameters.Nodes[0], f.Parameters[0]
	annotation := parameter.AsParameterDeclaration().Type
	actualFunction, actualParameter := check.GetSymbolAtLocation(node.Name()), check.GetSymbolAtLocation(parameter.Name())
	if actualFunction == nil || actualParameter == nil || actualFunction == actualParameter || len(actualFunction.Declarations) != 1 || actualFunction.Declarations[0] != node || len(actualParameter.Declarations) != 1 || actualParameter.Declarations[0] != parameter || annotation == nil || check.GetTypeAtLocation(parameter.Name()) != check.GetTypeFromTypeNode(annotation) || check.GetTypeFromTypeNode(fn.Type) != check.GetTypeFromTypeNode(annotation) {
		t.Fatal("forever original i32 declaration and own parameter")
	}
	if p.ID == "" || p.ID == f.Symbol || p.Name != "value" || p.Type != "i32" || p.Kind != "parameter" || !p.Writable || p.Range != throwBoundaryNodeForTest(t, file, parameter.Name()) || p.Presence == nil || p.Presence.Range != throwBoundaryNodeForTest(t, file, parameter) || p.Presence.Annotation != throwBoundaryNodeForTest(t, file, annotation) || p.Presence.Undefined || p.Presence.Null || p.Presence.Optional || p.Presence.Default {
		t.Fatal("forever owned annotated parameter")
	}
	seen := map[string]bool{}
	for _, s := range facts.Symbols {
		decl, kind, writable := node, "function", false
		if s.ID == p.ID {
			decl, kind, writable = parameter, "parameter", true
		} else if s.ID != f.Symbol {
			t.Fatal("forever foreign symbol")
		}
		if s.ID == "" || seen[s.ID] || s.Name != decl.Name().Text() || s.Kind != kind || s.Writable != writable || s.Type != "i32" || s.Range != throwBoundaryNodeForTest(t, file, decl.Name()) || (s.SourcePath != "" && s.SourcePath != r.RootFile) {
			t.Fatal("forever exact symbol declaration")
		}
		seen[s.ID] = true
	}
	loopNode, loopFact := fn.Body.AsBlock().Statements.Nodes[0], f.Body[0]
	if loopNode.Kind != ast.KindWhileStatement || loopFact.Kind != "while" || loopFact.Range != throwBoundaryNodeForTest(t, file, loopNode) {
		t.Fatal("forever actual while statement")
	}
	loop := loopNode.AsWhileStatement()
	if loop.Expression.Kind != ast.KindTrueKeyword || !check.GetTypeAtLocation(loop.Expression).IsBooleanLike() || loop.Statement.Kind != ast.KindBlock || len(loop.Statement.AsBlock().Statements.Nodes) != 1 || loopFact.Condition == nil || loopFact.Condition.Kind != "boolean" || loopFact.Condition.Type != "bool" || loopFact.Condition.Boolean == nil || !*loopFact.Condition.Boolean || loopFact.Condition.Range != throwBoundaryNodeForTest(t, file, loop.Expression) || len(loopFact.Body) != 1 || loopFact.Body[0].Kind != "block" || loopFact.Body[0].Range != throwBoundaryNodeForTest(t, file, loop.Statement) || len(loopFact.Body[0].Body) != 1 {
		t.Fatal("forever original literal condition and complete body")
	}
	thrown, owned := loop.Statement.AsBlock().Statements.Nodes[0], loopFact.Body[0].Body[0]
	if thrown.Kind != ast.KindThrowStatement || owned.Kind != "throw" || owned.Range != throwBoundaryNodeForTest(t, file, thrown) {
		t.Fatal("forever original terminal throw")
	}
	value := thrown.AsThrowStatement().Expression
	if value == nil || value.Kind != ast.KindIdentifier || check.GetSymbolAtLocation(value) != actualParameter || check.GetTypeAtLocation(value) != check.GetTypeFromTypeNode(annotation) || owned.Expression == nil || owned.Expression.Kind != "reference" || owned.Expression.Type != "i32" || owned.Expression.Symbol != p.ID || owned.Expression.Range != throwBoundaryNodeForTest(t, file, value) {
		t.Fatal("forever throw uses its own actual parameter")
	}
	exported := check.GetExportsOfModule(file.AsNode().Symbol())
	if len(exported) != 1 || exported[0] != actualFunction {
		t.Fatal("forever complete original export")
	}
	t.Log("fresh official Program: literal-true while and complete throwing body/own i32 Binding/UTF16; no execution or completion claim")
	return []any{map[string]any{"sourcePath": r.RootFile, "external": true, "bindings": []any{map[string]any{"name": "forever", "target": f.Symbol, "sourcePath": r.RootFile, "range": throwBoundaryNodeForTest(t, file, node), "definition": throwBoundaryNodeForTest(t, file, node.Name())}}}}
}

func TestExceptionPackageV1FinallyModuleInputs(t *testing.T) {
	const source = "// 😀\r\nexport function override(value:i32,replacement:i32):i32{try{return value;}finally{throw replacement;}}\r\nexport function nested(value:i32,replacement:i32,last:i32):i32{try{try{return value;}finally{throw replacement;}}finally{return last;}}"
	for _, row := range []struct{ name, legacy string }{{"finally-overrides", ""}, {"catch-write-statement", "04"}, {"catch-write-nested", "06"}, {"module-initialization", "09"}, {"literal-true-while", ""}} {
		t.Run(row.name, func(t *testing.T) {
			r := integerRequest(source)
			if row.name == "literal-true-while" {
				r = integerRequest("// 😀\r\nexport function forever(value:i32):i32{while(true){throw value;}}")
			}
			var expected, prior any
			if row.legacy != "" {
				r, expected, prior = exceptionFinallyLegacy(t, row.legacy)
			}
			input := exceptionPackageExportsRequestForTest(t, row.name, r)
			var public, output bytes.Buffer
			publicError := runFrontendCommand(t.Context(), nil, bytes.NewReader(input), &public)
			private := nullableFactsBoundaryCanonical(t, map[string]any{"family": "tsfg.exception-facts-request-candidate", "version": 1, "schemaHash": exceptionFactsSchema, "input": r})
			packageError := runFrontendCommand(t.Context(), []string{"--exception-facts-io-v1"}, bytes.NewReader(private), &output)
			// Preserve actual streams even when a later independent assertion fails.
			if root := os.Getenv("TSFG_EXCEPTION_FINALLY_CAPTURE_ROOT"); root != "" {
				directory := filepath.Join(root, row.name)
				if err := os.MkdirAll(directory, 0755); err != nil {
					t.Fatal(err)
				}
				data := map[string][]byte{"request.json": input, "response.json": public.Bytes(), "exception-request.json": private, "package.json": output.Bytes(), "tested-input-set.json": nullableFactsBoundaryCanonical(t, map[string]any{"rootFile": r.RootFile, "files": r.Files})}
				for _, file := range r.Files {
					data["sources/"+strings.TrimPrefix(file.Path, "/")] = []byte(file.Content)
				}
				for name, bytes := range data {
					target := filepath.Join(directory, filepath.FromSlash(name))
					if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(target, bytes, 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			if publicError != nil || packageError != nil {
				t.Fatalf("actual public entry: facts=%v package=%v public=%s packet=%s", publicError, packageError, public.Bytes(), output.Bytes())
			}
			var value any
			var facts response
			if err := json.Unmarshal(public.Bytes(), &value); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(public.Bytes(), &facts); err != nil {
				t.Fatal(err)
			}
			if facts.Status != "ok" || len(facts.Diagnostics) != 0 {
				t.Fatal("public facts success")
			}
			var exports []any
			if row.name == "literal-true-while" {
				exports = exceptionForeverOriginal(t, r, facts)
			} else if row.legacy == "" {
				exports = exceptionFinallyOriginal(t, r, facts)
			} else {
				if !bytes.Equal(nullableFactsBoundaryCanonical(t, value), nullableFactsBoundaryCanonical(t, expected)) {
					t.Fatal("original complete expected facts changed beyond the declared input profile")
				}
				var golden response
				if err := json.Unmarshal(nullableFactsBoundaryCanonical(t, expected), &golden); err != nil {
					t.Fatal(err)
				}
				exports = exceptionCorpusOfficialForTest(t, row.name, r, golden, expected, prior)
			}
			var packet struct {
				Family                           string
				Version                          int
				SchemaHash, RequestID, PackageID string
				Payload                          struct {
					Files   []exceptionFactsFile
					Facts   json.RawMessage
					Exports json.RawMessage
				}
			}
			if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}
			if packet.Family != "tsfg.exception-facts-package-candidate" || packet.Version != 1 || packet.SchemaHash != exceptionFactsSchema || packet.RequestID == "" || packet.PackageID == "" || len(packet.Payload.Files) != len(r.Files) || !bytes.Equal(packet.Payload.Facts, nullableFactsBoundaryCanonical(t, value)) || !bytes.Equal(packet.Payload.Exports, nullableFactsBoundaryCanonical(t, exports)) {
				t.Fatal("complete same-input package/facts/independent exports")
			}
			seen := map[string]bool{}
			for _, file := range packet.Payload.Files {
				found := false
				for _, original := range r.Files {
					if original.Path == file.Path {
						found = file.Digest == exceptionPackageExportsDigestForTest([]byte(original.Content)) && file.ByteLength == len(original.Content) && file.Length == len(utf16.Encode([]rune(original.Content))) && file.PositionUnit == "utf16"
					}
				}
				if !found || seen[file.Path] {
					t.Fatal("complete original per-file manifest")
				}
				seen[file.Path] = true
			}
		})
	}
}
