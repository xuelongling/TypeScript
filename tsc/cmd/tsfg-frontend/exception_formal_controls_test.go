// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/scanner"
)

type exceptionFormalOracle struct {
	ranges                                                      map[span]bool
	failParameterName, failParameter, failAnnotation, failValue span
}

// This oracle owns a fresh official Program. It does not use projection helpers
// or Product HIR to manufacture facts, call targets, completion or execution.
func exceptionFormalOriginal(t *testing.T, r request, indirect bool) exceptionFormalOracle {
	t.Helper()
	oracle := exceptionFormalOracle{}
	program := throwBoundaryProgramForTest(t, r)
	throwBoundaryDiagnosticsForTest(t, program)
	files := map[string]*ast.SourceFile{}
	for _, f := range program.GetSourceFiles() {
		files[f.FileName()] = f
	}
	if len(files) != len(r.Files) {
		t.Fatal("complete official input closure")
	}
	for _, input := range r.Files {
		if files[input.Path] == nil || files[input.Path].Text() != input.Content {
			t.Fatal("original input bytes")
		}
	}
	file := files[r.RootFile]
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	names := []string{"loop"}
	if indirect {
		names = []string{"fail", "entry"}
	}
	if len(file.Statements.Nodes) != len(names) {
		t.Fatal("complete original functions")
	}
	for i, fn := range file.Statements.Nodes {
		if fn.Kind != ast.KindFunctionDeclaration || fn.Name().Text() != names[i] {
			t.Fatal("original function declaration order")
		}
		for _, p := range fn.AsFunctionDeclaration().Parameters.Nodes {
			annotation := p.AsParameterDeclaration().Type
			if annotation == nil || check.GetTypeAtLocation(p.Name()) != check.GetTypeFromTypeNode(annotation) {
				t.Fatal("original annotated parameter checker type")
			}
		}
	}
	is := func(n, declaration *ast.Node) bool {
		s := check.GetSymbolAtLocation(n)
		return s != nil && len(s.Declarations) == 1 && s.Declarations[0] == declaration
	}
	if indirect {
		fail, entry := file.Statements.Nodes[0], file.Statements.Nodes[1]
		body := entry.Body().AsBlock().Statements.Nodes
		if len(body) != 2 || body[0].Kind != ast.KindVariableStatement || body[1].Kind != ast.KindTryStatement {
			t.Fatal("original callable and try order")
		}
		decl := body[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0]
		if decl.Name().Text() != "selected" || !is(decl.AsVariableDeclaration().Initializer, fail) {
			t.Fatal("original function value initializer")
		}
		attempt := body[1].AsTryStatement()
		call := attempt.TryBlock.AsBlock().Statements.Nodes[0].AsReturnStatement().Expression
		if call.Kind != ast.KindCallExpression || !is(call.AsCallExpression().Expression, decl) || len(call.AsCallExpression().Arguments.Nodes) != 1 || !is(call.AsCallExpression().Arguments.Nodes[0], entry.AsFunctionDeclaration().Parameters.Nodes[0]) {
			t.Fatal("original actual indirect invocation")
		}
		sig := check.GetResolvedSignature(call)
		if sig == nil || sig.Declaration() != decl.AsVariableDeclaration().Type || ast.GetSourceFileOfNode(sig.Declaration()) != file || check.GetTypeAtLocation(call) != check.GetTypeFromTypeNode(entry.AsFunctionDeclaration().Type) {
			t.Fatal("original resolved callable annotation/signature")
		}
		caught := attempt.CatchClause.AsCatchClause()
		thrown := caught.Block.AsBlock().Statements.Nodes[0]
		if thrown.Kind != ast.KindThrowStatement || !is(thrown.AsThrowStatement().Expression, caught.VariableDeclaration) || check.GetTypeAtLocation(thrown.AsThrowStatement().Expression) != check.GetUnknownType() {
			t.Fatal("original current catch rethrow")
		}
		failBody := fail.Body().AsBlock().Statements.Nodes
		if len(failBody) != 1 || failBody[0].Kind != ast.KindThrowStatement || len(fail.AsFunctionDeclaration().Parameters.Nodes) != 1 {
			t.Fatal("complete throwing callee")
		}
		parameter := fail.AsFunctionDeclaration().Parameters.Nodes[0]
		value := failBody[0].AsThrowStatement().Expression
		annotation := parameter.AsParameterDeclaration().Type
		if value == nil || value.Kind != ast.KindIdentifier || !is(value, parameter) || annotation == nil || check.GetTypeAtLocation(value) != check.GetTypeFromTypeNode(annotation) {
			t.Fatal("original fail throw resolves its own annotated parameter declaration")
		}
		oracle.failParameterName = throwBoundaryNodeForTest(t, file, parameter.Name())
		oracle.failParameter = throwBoundaryNodeForTest(t, file, parameter)
		oracle.failAnnotation = throwBoundaryNodeForTest(t, file, annotation)
		oracle.failValue = throwBoundaryNodeForTest(t, file, value)
	} else {
		fn := file.Statements.Nodes[0]
		body := fn.Body().AsBlock().Statements.Nodes
		if len(body) != 3 || body[0].Kind != ast.KindVariableStatement || body[1].Kind != ast.KindWhileStatement || body[2].Kind != ast.KindReturnStatement {
			t.Fatal("original local/while/return order")
		}
		decl := body[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0]
		loop := body[1].AsWhileStatement()
		if loop.Expression.Kind != ast.KindIdentifier || !is(loop.Expression, decl) || decl.Name().Text() != "pending" || !is(decl.AsVariableDeclaration().Initializer, fn.AsFunctionDeclaration().Parameters.Nodes[0]) {
			t.Fatal("original dynamic while condition")
		}
		attempt := loop.Statement.AsBlock().Statements.Nodes[0].AsTryStatement()
		if attempt.CatchClause != nil || attempt.FinallyBlock == nil {
			t.Fatal("original try/finally")
		}
		tryBody, finalBody := attempt.TryBlock.AsBlock().Statements.Nodes, attempt.FinallyBlock.AsBlock().Statements.Nodes
		if len(tryBody) != 1 || tryBody[0].Kind != ast.KindExpressionStatement || len(finalBody) != 1 || finalBody[0].Kind != ast.KindVariableStatement {
			t.Fatal("original normal try/finally completion permits a real backedge")
		}
		assignment := tryBody[0].AsExpressionStatement().Expression.AsBinaryExpression()
		copied := finalBody[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0]
		if assignment.OperatorToken.Kind != ast.KindEqualsToken || !is(assignment.Left, decl) || assignment.Right.Kind != ast.KindFalseKeyword || copied.Name().Text() != "copied" || !is(copied.AsVariableDeclaration().Initializer, fn.AsFunctionDeclaration().Parameters.Nodes[1]) {
			t.Fatal("original bool write and finally alias in their real scopes")
		}
	}
	ranges := map[span]bool{}
	for _, original := range files {
		var visit func(*ast.Node) bool
		visit = func(n *ast.Node) bool {
			if n.Pos() >= 0 && n.End() >= n.Pos() {
				raw := throwBoundaryNodeForTest(t, original, n)
				trim := throwBoundarySpanForTest(original.Text(), scanner.SkipTrivia(original.Text(), n.Pos()), n.End())
				if original == file {
					ranges[raw], ranges[trim] = true, true
				}
			}
			n.ForEachChild(visit)
			return false
		}
		visit(original.AsNode())
	}
	t.Log("fresh official Program: four diagnostic groups0; complete declarations/types/symbols/signatures/current-catch and per-file UTF16; no execution claim")
	oracle.ranges = ranges
	return oracle
}

// Same spelling/type in another function cannot substitute this declaration.
func exceptionFormalFailBinding(f function, original exceptionFormalOracle) bool {
	if f.Name != "fail" || len(f.Parameters) != 1 || len(f.Body) != 1 || f.Body[0].Kind != "throw" {
		return false
	}
	p, e := f.Parameters[0], f.Body[0].Expression
	return p.ID != "" && p.Name == "value" && p.Kind == "parameter" && p.Type == "i32" && p.Writable &&
		p.Range == original.failParameterName && p.Presence != nil && p.Presence.Range == original.failParameter && p.Presence.Annotation == original.failAnnotation &&
		!p.Presence.Undefined && !p.Presence.Null && !p.Presence.Optional && !p.Presence.Default &&
		e != nil && e.Kind == "reference" && e.Type == p.Type && e.Symbol == p.ID && e.Range == original.failValue
}

func exceptionFormalFacts(t *testing.T, facts response, indirect bool, original exceptionFormalOracle) {
	t.Helper()
	ranges := original.ranges
	names := []string{"loop"}
	if indirect {
		names = []string{"fail", "entry"}
	}
	if facts.Status != "ok" || len(facts.Diagnostics) != 0 || len(facts.Inputs) != 2 || len(facts.Functions) != len(names) {
		t.Fatalf("complete official facts: %+v", facts)
	}
	symbols := map[string]symbol{}
	for _, s := range facts.Symbols {
		if s.ID == "" || !ranges[s.Range] {
			t.Fatal("original symbol identity/range")
		}
		symbols[s.ID] = s
	}
	for i, f := range facts.Functions {
		if f.Name != names[i] || f.ReturnType != "i32" || !ranges[f.Range] || symbols[f.Symbol].Name != f.Name {
			t.Fatal("complete original function identity")
		}
	}
	ref := func(e *expression, name string) bool {
		return e != nil && e.Kind == "reference" && symbols[e.Symbol].Name == name && ranges[e.Range]
	}
	if indirect {
		fail, entry := facts.Functions[0], facts.Functions[1]
		if !exceptionFormalFailBinding(fail, original) || len(entry.Body) != 2 || entry.Body[0].Kind != "variable" || entry.Body[0].Symbol == nil {
			t.Fatal("complete throw/callable declarations")
		}
		selected := entry.Body[0]
		if selected.Symbol.Name != "selected" || selected.Symbol.Type != "function" || selected.Initializer == nil || selected.Initializer.Kind != "functionReference" || selected.Initializer.Target != fail.Symbol {
			t.Fatal("actual selected initializer")
		}
		attempt := entry.Body[1].Exception
		if entry.Body[1].Kind != "try" || attempt == nil || len(attempt.TryBlock.Body) != 1 || attempt.CatchClause == nil || attempt.FinallyBlock != nil {
			t.Fatal("original try/catch")
		}
		call := attempt.TryBlock.Body[0].Expression
		if attempt.TryBlock.Body[0].Kind != "return" || call == nil || call.Kind != "indirectCall" || call.Target != "" || !ref(call.Callee, "selected") || call.Callee.Symbol != selected.Symbol.ID || call.Arguments == nil || len(*call.Arguments) != 1 || !ref(&(*call.Arguments)[0], "value") || (*call.Arguments)[0].Symbol != entry.Parameters[0].ID {
			t.Fatal("actual independent callee/argument relationship")
		}
		for _, sig := range []*signature{call.Signature, call.Callee.Signature, selected.Symbol.Signature, selected.Initializer.Signature} {
			if sig == nil || !reflect.DeepEqual(sig.Parameters, []string{"i32"}) || sig.ReturnType != "i32" {
				t.Fatal("original callable signature")
			}
		}
		caught := attempt.CatchClause
		if caught.Binding == nil || caught.Binding.Name != "caught" || caught.Binding.Type != "unknown" || len(caught.Block.Body) != 1 || caught.Block.Body[0].Kind != "throw" || !ref(caught.Block.Body[0].Expression, "caught") || caught.Block.Body[0].Expression.Symbol != caught.Binding.ID {
			t.Fatal("current catch binding/rethrow preserved")
		}
	} else {
		f := facts.Functions[0]
		if len(f.Parameters) != 2 || len(f.Body) != 3 || f.Body[0].Kind != "variable" || f.Body[0].Symbol == nil || f.Body[0].Symbol.Name != "pending" || f.Body[0].Symbol.Type != "bool" || !f.Body[0].Symbol.Writable || !ref(f.Body[0].Initializer, "again") {
			t.Fatal("original mutable loop local")
		}
		loop := f.Body[1]
		if loop.Kind != "while" || !ref(loop.Condition, "pending") || loop.Condition.Symbol != f.Body[0].Symbol.ID || len(loop.Body) != 1 || loop.Body[0].Kind != "block" || len(loop.Body[0].Body) != 1 {
			t.Fatal("dynamic condition and real lexical loop block")
		}
		attempt := loop.Body[0].Body[0].Exception
		if loop.Body[0].Body[0].Kind != "try" || attempt == nil || attempt.CatchClause != nil || attempt.FinallyBlock == nil || len(attempt.TryBlock.Body) != 1 || len(attempt.FinallyBlock.Body) != 1 {
			t.Fatal("loop original try/finally regions")
		}
		write, copied := attempt.TryBlock.Body[0], attempt.FinallyBlock.Body[0]
		if write.Kind != "assignment" || !ref(write.Target, "pending") || write.Target.Symbol != f.Body[0].Symbol.ID || write.Value == nil || write.Value.Kind != "boolean" || write.Value.Boolean == nil || *write.Value.Boolean {
			t.Fatal("normal try bool write before actual backedge")
		}
		if copied.Kind != "variable" || copied.Symbol == nil || copied.Symbol.Name != "copied" || copied.Symbol.Writable || !ref(copied.Initializer, "value") || copied.Initializer.Symbol != f.Parameters[1].ID {
			t.Fatal("normal finally local alias in original scope")
		}
		if f.Body[2].Kind != "return" || !ref(f.Body[2].Expression, "value") {
			t.Fatal("normal loop exit")
		}
	}
	// Every serialized range must still join an actual original AST node. This
	// walk checks declarations/expressions/regions, without reusing projection.
	raw, err := json.Marshal(facts)
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	exceptionCorpusWalkForTest(value, "/src/main.ts", func(row map[string]any, _ string) {
		if r, ok := row["range"].(map[string]any); ok {
			where := span{int(r["start"].(float64)), int(r["end"].(float64))}
			if !ranges[where] {
				t.Fatalf("range not in independent original AST: %+v", where)
			}
		}
	})
}

func TestExceptionPackageV1FormalControlInputs(t *testing.T) {
	for _, row := range []struct {
		name, source string
		indirect     bool
	}{
		{"while-finally", "// 😀\r\nexport function loop(again:bool,value:i32):i32{let pending:bool=again;while(pending){try{pending=false;}finally{const copied:i32=value;}}return value;}", false},
		{"indirect-current-rethrow", "// 😀\r\nfunction fail(value:i32):i32{throw value;}\r\nexport function entry(value:i32):i32{const selected:(value:i32)=>i32=fail;try{return selected(value);}catch(caught){throw caught;}}", true},
	} {
		t.Run(row.name, func(t *testing.T) {
			r := integerRequest(row.source)
			ranges := exceptionFormalOriginal(t, r, row.indirect)
			input := exceptionPackageExportsRequestForTest(t, row.name, r)
			var public bytes.Buffer
			err := runFrontendCommand(t.Context(), nil, bytes.NewReader(input), &public)
			if err != nil {
				t.Fatalf("public source command: %v %s", err, public.Bytes())
			}
			private := nullableFactsBoundaryCanonical(t, map[string]any{"family": "tsfg.exception-facts-request-candidate", "version": 1, "schemaHash": exceptionFactsSchema, "input": r})
			var output bytes.Buffer
			err = runFrontendCommand(t.Context(), []string{"--exception-facts-io-v1"}, bytes.NewReader(private), &output)
			if root := os.Getenv("TSFG_EXCEPTION_FORMAL_CAPTURE_ROOT"); root != "" {
				directory := filepath.Join(root, row.name)
				if err := os.MkdirAll(directory, 0755); err != nil {
					t.Fatal(err)
				}
				for name, data := range map[string][]byte{"source.ts": []byte(row.source), "request.json": input, "response.json": public.Bytes(), "exception-request.json": private, "package.json": output.Bytes()} {
					if err := os.WriteFile(filepath.Join(directory, name), data, 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err != nil {
				t.Fatalf("public exception package: %v %s", err, output.Bytes())
			}
			var packet struct {
				Family                           string
				Version                          int
				SchemaHash, RequestID, PackageID string
				Payload                          struct {
					Files   []exceptionFactsFile
					Facts   json.RawMessage
					Exports []exceptionFactsExportGroup
				}
			}
			if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}
			if packet.Family != "tsfg.exception-facts-package-candidate" || packet.Version != 1 || packet.SchemaHash != exceptionFactsSchema || packet.RequestID == "" || packet.PackageID == "" || len(packet.Payload.Files) != len(r.Files) || len(packet.Payload.Exports) != 1 || len(packet.Payload.Exports[0].Bindings) != 1 {
				t.Fatal("complete package/export identity")
			}
			for _, file := range packet.Payload.Files {
				found := false
				for _, original := range r.Files {
					if original.Path == file.Path {
						found = file.Digest == exceptionPackageExportsDigestForTest([]byte(original.Content)) && file.ByteLength == len(original.Content) && file.Length == len(utf16.Encode([]rune(original.Content))) && file.PositionUnit == "utf16"
					}
				}
				if !found {
					t.Fatal("original file manifest")
				}
			}
			var publicValue any
			if err := json.Unmarshal(public.Bytes(), &publicValue); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(packet.Payload.Facts, nullableFactsBoundaryCanonical(t, publicValue)) {
				t.Fatal("exception package retains complete same-input public facts")
			}
			var facts response
			if err := json.Unmarshal(packet.Payload.Facts, &facts); err != nil {
				t.Fatal(err)
			}
			exceptionFormalFacts(t, facts, row.indirect, ranges)
			if row.indirect {
				t.Run("same-name-foreign-parameter", func(t *testing.T) {
					// Mutate only an in-memory copy of the actual full response. The
					// captured official packet/response and source remain unchanged.
					var corrupted response
					if err := json.Unmarshal(public.Bytes(), &corrupted); err != nil {
						t.Fatal(err)
					}
					corrupted.Functions[0].Body[0].Expression.Symbol = corrupted.Functions[1].Parameters[0].ID
					if exceptionFormalFailBinding(corrupted.Functions[0], ranges) {
						t.Fatal("same-name foreign parameter accepted as fail's own binding")
					}
				})
			}
		})
	}
}
