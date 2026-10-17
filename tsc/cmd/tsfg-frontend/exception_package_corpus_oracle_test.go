// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/scanner"
)

// Golden JSON is only a join expectation: all declaration symbols, types,
// expression sites and export targets below come from a fresh official Program.
func exceptionCorpusWalkForTest(value any, owner string, visit func(map[string]any, string)) {
	switch value := value.(type) {
	case []any:
		for _, child := range value {
			exceptionCorpusWalkForTest(child, owner, visit)
		}
	case map[string]any:
		for _, key := range []string{"sourcePath", "path"} {
			if path, ok := value[key].(string); ok && strings.HasPrefix(path, "/src/") {
				owner = path
			}
		}
		visit(value, owner)
		for key, child := range value {
			next := owner
			if key == "markerDeclarationRange" {
				if path, ok := value["markerDeclarationPath"].(string); ok {
					next = path
				}
			}
			exceptionCorpusWalkForTest(child, next, visit)
		}
	}
}

func exceptionCorpusOfficialForTest(t *testing.T, name string, r request, golden response, facts, prior any) []any {
	t.Helper()
	program := throwBoundaryProgramForTest(t, r)
	throwBoundaryDiagnosticsForTest(t, program)
	files := map[string]*ast.SourceFile{}
	consumed, sources := []string{}, []*ast.SourceFile{}
	for _, file := range program.GetSourceFiles() {
		files[file.FileName()] = file
		consumed = append(consumed, file.FileName())
		if !strings.HasSuffix(file.FileName(), ".d.ts") {
			sources = append(sources, file)
		}
	}
	slices.Sort(consumed)
	slices.SortFunc(sources, func(a, b *ast.SourceFile) int { return strings.Compare(a.FileName(), b.FileName()) })
	if !reflect.DeepEqual(consumed, golden.Inputs) || len(consumed) != len(r.Files) {
		t.Fatal("complete actual consumed files changed")
	}
	for _, input := range r.Files {
		if files[input.Path] == nil || files[input.Path].Text() != input.Content {
			t.Fatal("fresh Program changed original input bytes")
		}
	}
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), files[r.RootFile])
	defer release()
	ranges := map[string]map[span]bool{}
	nodes := map[string]map[span][]*ast.Node{}
	declarations := []*ast.Node{}
	for _, file := range program.GetSourceFiles() {
		ranges[file.FileName()], nodes[file.FileName()] = map[span]bool{}, map[span][]*ast.Node{}
		add := func(where span, node *ast.Node) {
			ranges[file.FileName()][where] = true
			nodes[file.FileName()][where] = append(nodes[file.FileName()][where], node)
		}
		var visit func(*ast.Node) bool
		visit = func(node *ast.Node) bool {
			if node.Pos() >= 0 && node.End() >= node.Pos() && node.End() <= len(file.Text()) {
				add(throwBoundaryNodeForTest(t, file, node), node)
				start := scanner.SkipTrivia(file.Text(), node.Pos())
				if start <= node.End() {
					add(throwBoundarySpanForTest(file.Text(), start, node.End()), node)
				}
			}
			if !strings.HasSuffix(file.FileName(), ".d.ts") {
				switch node.Kind {
				case ast.KindClassDeclaration, ast.KindPropertyDeclaration, ast.KindFunctionDeclaration, ast.KindParameter, ast.KindVariableDeclaration:
					declarations = append(declarations, node)
				}
				if node.Kind == ast.KindClassDeclaration {
					start := node.AsClassDeclaration().Members.Pos() - 1
					for start >= node.Pos() && file.Text()[start] != '{' {
						start--
					}
					if start < node.Pos() {
						t.Fatal("official class body preimage missing")
					}
					add(throwBoundarySpanForTest(file.Text(), start, node.End()), node)
				}
			}
			node.ForEachChild(visit)
			return false
		}
		visit(file.AsNode())
	}
	owner := func(path string) string {
		if path == "" {
			return r.RootFile
		}
		return path
	}
	kind := func(node *ast.Node) string {
		switch node.Kind {
		case ast.KindClassDeclaration:
			return "class"
		case ast.KindPropertyDeclaration:
			return "field"
		case ast.KindFunctionDeclaration:
			return "function"
		case ast.KindParameter:
			return "parameter"
		}
		return "local"
	}
	match := func(node *ast.Node) symbol {
		file := ast.GetSourceFileOfNode(node)
		where := throwBoundaryNodeForTest(t, file, node.Name())
		matches := []symbol{}
		for _, s := range golden.Symbols {
			if owner(s.SourcePath) == file.FileName() && s.Name == node.Name().Text() && s.Kind == kind(node) && s.Range == where {
				matches = append(matches, s)
			}
		}
		if len(matches) != 1 {
			t.Fatalf("expected one golden binding at actual %s/%s/%+v, got %d", file.FileName(), node.Name().Text(), where, len(matches))
		}
		return matches[0]
	}
	classes := map[*checker.Type]string{}
	for _, node := range declarations {
		if node.Kind == ast.KindClassDeclaration {
			classes[check.GetDeclaredTypeOfSymbol(check.GetSymbolAtLocation(node.Name()))] = match(node).Type
		}
	}
	var typeOf func(*checker.Type) (string, bool, bool)
	typeOf = func(value *checker.Type) (string, bool, bool) {
		if value == nil {
			t.Fatal("official type missing")
		}
		if value == check.GetUnknownType() {
			return "unknown", false, false
		}
		if value == check.GetBooleanType() || value.IsBooleanLike() {
			return "bool", false, false
		}
		if id := classes[value]; id != "" {
			return id, false, false
		}
		if value.IsUnion() {
			payload, null, undefined := "", false, false
			for _, member := range value.Types() {
				switch {
				case member.Flags()&checker.TypeFlagsNull != 0:
					null = true
				case member.Flags()&checker.TypeFlagsUndefined != 0:
					undefined = true
				default:
					part, u, n := typeOf(member)
					if payload != "" || u || n {
						t.Fatal("original union payload identity changed")
					}
					payload = part
				}
			}
			if payload == "" {
				t.Fatal("original typed union lost its nominal payload")
			}
			return payload, undefined, null
		}
		if value.Alias() != nil && value.Alias().Symbol() != nil {
			switch value.Alias().Symbol().Name {
			case "i8", "i16", "i32", "i64", "u8", "u16", "u32", "u64", "isize", "usize", "f32", "f64":
				return value.Alias().Symbol().Name, false, false
			}
		}
		if len(check.GetSignaturesOfType(value, checker.SignatureKindCall)) == 1 {
			return "function", false, false
		}
		t.Fatal("original official type is outside this finite corpus")
		return "", false, false
	}
	ids, defining := map[*ast.Symbol]string{}, map[string]*ast.Node{}
	for _, node := range declarations {
		s := match(node)
		actual := exceptionTraversalIdentityForTest(t, check, node)
		if ids[actual] != "" || defining[s.ID] != nil {
			t.Fatal("distinct actual/golden defining identities merged")
		}
		ids[actual], defining[s.ID] = s.ID, node
		if node.Kind == ast.KindClassDeclaration {
			continue
		}
		actualType := check.GetTypeOfSymbol(actual)
		if node.Kind == ast.KindFunctionDeclaration {
			actualType = check.GetTypeFromTypeNode(node.AsFunctionDeclaration().Type)
		}
		typ, undefined, null := typeOf(actualType)
		if typ != s.Type {
			t.Fatalf("official declared type differs for %s: got=%s want=%s", s.Name, typ, s.Type)
		}
		if node.Kind != ast.KindFunctionDeclaration {
			if s.Presence == nil {
				if undefined || null {
					t.Fatal("original declaration presence disappeared")
				}
			} else if s.Presence.Undefined != undefined || s.Presence.Null != null {
				t.Fatal("official declaration members differ from independent golden")
			}
		}
	}
	if len(defining) != len(golden.Symbols) {
		t.Fatal("complete independent symbol registry did not join actual declarations")
	}
	for _, s := range golden.Symbols {
		if s.LexicalOwner != "" {
			parent := ast.FindAncestor(defining[s.ID].Parent, ast.IsFunctionLike)
			if parent == nil || ids[check.GetSymbolAtLocation(parent.Name())] != s.LexicalOwner {
				t.Fatal("actual lexical owner differs from independent golden")
			}
		}
	}
	functionNodes := map[string]*ast.Node{}
	for _, fn := range golden.Functions {
		node := defining[fn.Symbol]
		if node == nil || node.Kind != ast.KindFunctionDeclaration || fn.Range != throwBoundaryNodeForTest(t, files[owner(fn.SourcePath)], node) || node.Name().Text() != fn.Name {
			t.Fatal("function full range/source/actual declaration join failed")
		}
		decl := node.AsFunctionDeclaration()
		signatures := check.GetSignaturesOfType(check.GetTypeOfSymbol(check.GetSymbolAtLocation(node.Name())), checker.SignatureKindCall)
		if len(signatures) != 1 || signatures[0].Declaration() != node || len(signatures[0].Parameters()) != len(fn.Parameters) || len(decl.Parameters.Nodes) != len(fn.Parameters) || check.GetReturnTypeOfSignature(signatures[0]) != check.GetTypeFromTypeNode(decl.Type) {
			t.Fatal("complete official function signature changed")
		}
		for i, p := range decl.Parameters.Nodes {
			if signatures[0].Parameters()[i] != check.GetSymbolAtLocation(p.Name()) || ids[signatures[0].Parameters()[i]] != fn.Parameters[i].ID {
				t.Fatal("actual parameter signature identity changed")
			}
		}
		_, u, n := typeOf(check.GetTypeFromTypeNode(decl.Type))
		if fn.ReturnPresence == nil {
			if u || n {
				t.Fatal("nullable return declaration lost presence")
			}
		} else if fn.ReturnPresence.Undefined != u || fn.ReturnPresence.Null != n {
			t.Fatal("official return members differ from independent golden")
		}
		parent := ast.FindAncestor(node.Parent, ast.IsFunctionLike)
		parentID := ""
		if parent != nil {
			parentID = ids[check.GetSymbolAtLocation(parent.Name())]
		}
		if fn.LexicalParent != parentID {
			t.Fatal("actual lexical parent differs from independent golden")
		}
		functionNodes[fn.Symbol] = node
	}
	for _, node := range declarations {
		if node.Kind == ast.KindFunctionDeclaration && functionNodes[ids[check.GetSymbolAtLocation(node.Name())]] != node {
			t.Fatal("fresh function declaration omitted by golden")
		}
	}
	exports, freshNames := []any{}, map[string][]map[string]any{}
	for _, file := range sources {
		bindings := []map[string]any{}
		if ast.IsExternalModule(file) {
			for _, exported := range check.GetExportsOfModule(file.AsNode().Symbol()) {
				if exported.Flags&ast.SymbolFlagsAlias != 0 || len(exported.Declarations) != 1 {
					t.Fatal("original corpus must have only direct actual function exports")
				}
				node := exported.Declarations[0]
				if node.Kind != ast.KindFunctionDeclaration || node.Parent != file.AsNode() || ast.GetSourceFileOfNode(node) != file {
					t.Fatal("official export is not an actual top-level defining function")
				}
				id := ids[check.GetSymbolAtLocation(node.Name())]
				if id == "" || functionNodes[id] != node {
					t.Fatal("official export did not join full actual declaration to independent owned ID")
				}
				bindings = append(bindings, map[string]any{"name": exported.Name, "target": id, "sourcePath": file.FileName(), "range": throwBoundaryNodeForTest(t, file, node), "definition": throwBoundaryNodeForTest(t, file, node.Name())})
			}
		}
		slices.SortFunc(bindings, func(a, b map[string]any) int { return strings.Compare(a["name"].(string), b["name"].(string)) })
		freshNames[file.FileName()] = bindings
		exports = append(exports, map[string]any{"sourcePath": file.FileName(), "external": ast.IsExternalModule(file), "bindings": bindings})
	}
	exceptionCorpusWalkForTest(facts, r.RootFile, func(value map[string]any, path string) {
		k, ok := value["kind"].(string)
		if !ok || value["id"] != nil || value["type"] == nil || value["range"] == nil {
			return
		}
		var where span
		if err := json.Unmarshal(nullableFactsBoundaryCanonical(t, value["range"]), &where); err != nil {
			t.Fatal(err)
		}
		var chosen *ast.Node
		for _, node := range nodes[path][where] {
			fits := k == "reference" && node.Kind == ast.KindIdentifier || k == "boolean" && (node.Kind == ast.KindTrueKeyword || node.Kind == ast.KindFalseKeyword) || (k == "binary" || k == "type-test" || k == "write") && node.Kind == ast.KindBinaryExpression || k == "group" && node.Kind == ast.KindParenthesizedExpression || k == "field" && node.Kind == ast.KindPropertyAccessExpression || k == "call" && node.Kind == ast.KindCallExpression || k == "construct" && node.Kind == ast.KindNewExpression
			if fits {
				if chosen != nil && chosen != node {
					t.Fatal("ambiguous original expression range")
				}
				chosen = node
			}
		}
		if chosen == nil {
			t.Fatalf("independent %s expression has no original official AST at %s/%+v", k, path, where)
		}
		typ, u, n := typeOf(check.GetTypeAtLocation(chosen))
		if typ != value["type"] {
			t.Fatalf("fresh use-site type differs: kind=%s got=%s want=%v", k, typ, value["type"])
		}
		if flow, ok := value["presenceFlow"].(map[string]any); ok {
			if flow["undefined"] != u || flow["null"] != n {
				t.Fatal("official occurrence presence differs from original independent expected")
			}
		} else if u || n {
			t.Fatal("official occurrence presence was erased")
		}
		if k == "reference" && ids[check.GetSymbolAtLocation(chosen)] != value["symbol"] {
			t.Fatal("original reference rebound actual symbol")
		}
		if k == "field" && ids[check.GetSymbolAtLocation(chosen.Name())] != value["symbol"] {
			t.Fatal("original field rebound actual symbol")
		}
		if k == "call" {
			actual := check.GetSymbolAtLocation(chosen.AsCallExpression().Expression)
			for i := 0; actual != nil && actual.Flags&ast.SymbolFlagsAlias != 0 && i < 4; i++ {
				actual = check.GetImmediateAliasedSymbol(actual)
			}
			resolved := check.GetResolvedSignature(chosen)
			id, _ := value["target"].(string)
			if actual == nil || ids[actual] != id || resolved == nil || resolved.Declaration() != functionNodes[id] {
				t.Fatal("actual original resolved call target differs")
			}
		}
	})
	exceptionCorpusWalkForTest(prior, r.RootFile, func(value map[string]any, path string) {
		if start, ok := value["start"].(float64); ok {
			if end, ok := value["end"].(float64); !ok || start != float64(int(start)) || end != float64(int(end)) || !ranges[path][span{int(start), int(end)}] {
				t.Fatalf("prior oracle range contradicts fresh official AST: %s/%v", path, value)
			}
		}
		if digest, ok := value["sourceSha256"].(string); ok && (files[path] == nil || exceptionPackageExportsDigestForTest([]byte(files[path].Text())) != digest) {
			t.Fatal("prior oracle original source digest changed")
		}
		if digest, ok := value["markerSha256"].(string); ok {
			p, _ := value["markerDeclarationPath"].(string)
			if files[p] == nil || exceptionPackageExportsDigestForTest([]byte(files[p].Text())) != digest {
				t.Fatal("prior marker declaration identity changed")
			}
		}
		if old, ok := value["exports"].([]any); ok {
			bindings := freshNames[path]
			if len(old) != len(bindings) {
				t.Fatal("complete fresh export count contradicts original oracle")
			}
			for _, item := range old {
				name, fields := "", map[string]any{}
				if text, ok := item.(string); ok {
					name = text
				} else {
					fields = item.(map[string]any)
					name, _ = fields["name"].(string)
				}
				found := false
				for _, binding := range bindings {
					if binding["name"] == name {
						found = true
						for _, key := range []string{"range", "definition"} {
							if fields[key] != nil && !bytes.Equal(nullableFactsBoundaryCanonical(t, fields[key]), nullableFactsBoundaryCanonical(t, binding[key])) {
								t.Fatal("original export raw anchor contradicts fresh actual declaration")
							}
						}
					}
				}
				if !found {
					t.Fatal("original official export name missing from fresh complete table")
				}
			}
		}
	})
	oracle, err := json.Marshal(map[string]any{"case": name, "kind": "test-only-fresh-official-corpus-oracle", "consumed": consumed, "joinedSymbols": len(defining), "joinedFunctions": len(functionNodes), "exports": exports, "priorOracleRangesChecked": true, "goldenUseSiteTypesChecked": true})
	t.Logf("TSFG_EXCEPTION_CORPUS_FRESH_ORACLE_V1 case=%s %s", name, base64.StdEncoding.EncodeToString(oracle))
	if err != nil {
		t.Fatal(err)
	}
	return exports
}
