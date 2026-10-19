// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

//go:embed testdata/module-callable-immutable-alias/first-whole-oracle.json
var immutableAliasWhole23g []byte

type immutableAliasAST23g struct {
	Kind               string
	Start, End, Parent int
}

type immutableAliasWholeFixture23g struct {
	Input    request
	Manifest []struct {
		Path, Digest, PositionUnit string
		ByteLength, Length         int
	}
	Official struct {
		AppAST, DeclarationAST                   []immutableAliasAST23g
		AppSymbolGroups, DeclarationSymbolGroups map[string][]int
	}
	ExpectedFacts response
}

// Independent official Program only: no owned projection, target adapter or
// producer identity helper constructs this AST/symbol/signature oracle.
func TestModuleCallableImmutableAliasOfficialWholePrior(t *testing.T) {
	before := bytes.Clone(immutableAliasWhole23g)
	var fixture immutableAliasWholeFixture23g
	if err := json.Unmarshal(immutableAliasWhole23g, &fixture); err != nil {
		t.Fatal(err)
	}
	r := fixture.Input
	contents := map[string]string{}
	names := []string{}
	for i, file := range r.Files {
		if i >= len(fixture.Manifest) {
			t.Fatal("extra original input")
		}
		manifest := fixture.Manifest[i]
		digest := sha256.Sum256([]byte(file.Content))
		if manifest.Path != file.Path || manifest.Digest != "sha256:"+file.SHA256 ||
			manifest.ByteLength != len(file.Content) || manifest.Length != len(utf16.Encode([]rune(file.Content))) ||
			manifest.PositionUnit != "utf16" || file.SHA256 != hex.EncodeToString(digest[:]) {
			t.Fatal("complete source/DTS byte and UTF16 manifest differs")
		}
		contents[file.Path] = file.Content
		names = append(names, file.Path)
	}
	if len(names) != 2 || len(fixture.Manifest) != 2 {
		t.Fatal("exact original TS/DTS closure required")
	}
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
			FileNames: names,
			CompilerOptions: &core.CompilerOptions{
				NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext,
			},
		}},
		Host:           compiler.NewCompilerHost(r.SourceRoot, vfstest.FromMap(contents, true), r.SourceRoot, nil, nil),
		SingleThreaded: core.TSTrue,
	})
	if ds := program.GetSyntacticDiagnostics(t.Context(), nil); len(ds) != 0 {
		t.Fatalf("official syntax: %+v", ds)
	}
	if ds := program.GetSemanticDiagnostics(t.Context(), nil); len(ds) != 0 {
		t.Fatalf("official semantic: %+v", ds)
	}
	if ds := program.GetProgramDiagnostics(); len(ds) != 0 {
		t.Fatalf("official program: %+v", ds)
	}
	if ds := program.GetGlobalDiagnostics(t.Context()); len(ds) != 0 {
		t.Fatalf("official global: %+v", ds)
	}
	if len(program.GetSourceFiles()) != 2 {
		t.Fatal("official program consumed an extra or missing source")
	}
	app, dts := program.GetSourceFile(r.RootFile), program.GetSourceFile(r.Declarations.Paths[0])
	if app == nil || dts == nil || app.Text() != contents[r.RootFile] || dts.Text() != contents[r.Declarations.Paths[0]] {
		t.Fatal("official complete source text differs")
	}
	if len(app.Statements.Nodes) != 5 || len(dts.Statements.Nodes) != 32 {
		t.Fatal("official complete top-level statements differ")
	}
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), app)
	defer release()
	files := []*ast.SourceFile{app, dts}
	wants := [][]immutableAliasAST23g{fixture.Official.AppAST, fixture.Official.DeclarationAST}
	groups := []map[string][]int{fixture.Official.AppSymbolGroups, fixture.Official.DeclarationSymbolGroups}
	walks := make([][]*ast.Node, 2)
	bindings := []map[string]*ast.Symbol{{}, {}}
	for f, file := range files {
		rows := []immutableAliasAST23g{}
		var walk func(*ast.Node, int)
		walk = func(node *ast.Node, parent int) {
			index := len(rows)
			positions := file.GetPositionMap()
			rows = append(rows, immutableAliasAST23g{strings.TrimPrefix(node.Kind.String(), "Kind"), positions.UTF8ToUTF16(node.Pos()), positions.UTF8ToUTF16(node.End()), parent})
			walks[f] = append(walks[f], node)
			node.ForEachChild(func(child *ast.Node) bool { walk(child, index); return false })
		}
		walk(file.AsNode(), -1)
		if !reflect.DeepEqual(rows, wants[f]) {
			actual, _ := json.Marshal(rows)
			t.Logf("TSFG_M23G_OFFICIAL_AST_%d=%s", f, actual)
			t.Fatalf("complete literal AST differs for %s", file.FileName())
		}
		seen := map[*ast.Symbol]string{}
		for label, indices := range groups[f] {
			var original *ast.Symbol
			for _, index := range indices {
				if index < 0 || index >= len(walks[f]) || walks[f][index].Kind != ast.KindIdentifier {
					t.Fatal("literal original symbol site is not an identifier")
				}
				actual := check.GetSymbolAtLocation(walks[f][index])
				if actual == nil || actual.Flags&ast.SymbolFlagsAlias != 0 {
					t.Fatal("original binding missing or confused with TS alias flag")
				}
				if original == nil {
					original = actual
				} else if actual != original {
					t.Fatalf("original symbol relation differs: %s", label)
				}
			}
			if previous, exists := seen[original]; exists {
				t.Fatalf("independent original symbols merged: %s and %s", previous, label)
			}
			seen[original] = label
			bindings[f][label] = original
		}
	}
	for label, index := range map[string]int{"s1": 3, "s2": 8, "s3": 24, "s4": 27, "s5": 41, "s6": 10, "s7": 30, "s8": 44} {
		declarations := bindings[0][label].Declarations
		if len(declarations) != 1 || declarations[0] != walks[0][index] {
			t.Fatalf("original declaration node differs: %s", label)
		}
	}
	if bindings[0]["type.bool"] != bindings[1]["type.bool"] {
		t.Fatal("every app bool annotation must use the original DTS declaration")
	}
	nodes := walks[0]
	leaf, op, entry, peer := nodes[8], nodes[24], nodes[27], nodes[41]
	if leaf.Name().Text() != "leaf" || op.Name().Text() != "op" || entry.Name().Text() != "entry" || peer.Name().Text() != "peer" ||
		ast.HasSyntacticModifier(leaf, ast.ModifierFlagsExport) || !ast.HasSyntacticModifier(entry, ast.ModifierFlagsExport) ||
		!ast.HasSyntacticModifier(peer, ast.ModifierFlagsExport) || ast.HasSyntacticModifier(nodes[22], ast.ModifierFlagsExport) ||
		nodes[23].Flags&ast.NodeFlagsConst == 0 || op.AsVariableDeclaration().Type != nil || op.AsVariableDeclaration().Initializer != nodes[26] {
		t.Fatal("unique nonexport const initializer or original function declarations differ")
	}
	if bindings[0]["s2"] == bindings[0]["s3"] || nodes[26].Text() != "leaf" || nodes[39].Text() != "op" {
		t.Fatal("initializer leaf and actual op callee must remain distinct original symbols")
	}
	for _, fn := range []*ast.Node{leaf, entry, peer} {
		decl := fn.AsFunctionDeclaration()
		if decl.TypeParameters != nil || len(decl.Parameters.Nodes) != 1 || decl.Type == nil || decl.Body == nil ||
			decl.Body.Kind != ast.KindBlock || len(decl.Body.AsBlock().Statements.Nodes) != 1 {
			t.Fatal("ordinary complete scalar function shape differs")
		}
		parameter := decl.Parameters.Nodes[0].AsParameterDeclaration()
		if parameter.Initializer != nil || parameter.DotDotDotToken != nil || parameter.QuestionToken != nil || parameter.Type == nil ||
			!check.GetTypeAtLocation(decl.Parameters.Nodes[0].Name()).IsBooleanLike() || !check.GetTypeFromTypeNode(decl.Type).IsBooleanLike() {
			t.Fatal("one required original bool parameter and bool return required")
		}
	}
	for _, index := range []int{9, 25, 26, 39} {
		sigs := check.GetSignaturesOfType(check.GetTypeAtLocation(nodes[index]), checker.SignatureKindCall)
		if len(sigs) != 1 || sigs[0].Declaration() != leaf || len(sigs[0].Parameters()) != 1 ||
			sigs[0].Parameters()[0] != bindings[0]["s6"] || sigs[0].MinArgumentCount() != 1 ||
			len(sigs[0].TypeParameters()) != 0 || sigs[0].ThisParameter() != nil || sigs[0].HasRestParameter() ||
			!check.GetTypeOfSymbol(sigs[0].Parameters()[0]).IsBooleanLike() || !check.GetReturnTypeOfSignature(sigs[0]).IsBooleanLike() {
			t.Fatal("original leaf/op declaration and use signatures differ")
		}
	}
	call := nodes[38].AsCallExpression()
	resolved := check.GetResolvedSignature(nodes[38])
	if call.Expression != nodes[39] || len(call.Arguments.Nodes) != 1 || call.Arguments.Nodes[0] != nodes[40] ||
		call.QuestionDotToken != nil || call.TypeArguments != nil || resolved == nil || resolved.Declaration() != leaf ||
		len(resolved.Parameters()) != 1 || resolved.Parameters()[0] != bindings[0]["s6"] || resolved.MinArgumentCount() != 1 ||
		!check.GetReturnTypeOfSignature(resolved).IsBooleanLike() || bindings[0]["s7"] != check.GetSymbolAtLocation(nodes[40]) {
		t.Fatal("actual op call resolved declaration/parameter order differs")
	}
	if !bytes.Equal(immutableAliasWhole23g, before) {
		t.Fatal("independent data changed")
	}
	t.Log("TSFG_M23G_OFFICIAL_WHOLE_PASS diagnostics=0 app57 dts210 declarations5 originalSymbols8 resolvedSignature=leaf DTScomplete=true")
}
