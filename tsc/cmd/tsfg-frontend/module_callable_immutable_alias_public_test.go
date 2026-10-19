// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

//go:embed testdata/module-callable-immutable-alias/public-and-boundary-oracle.json
var immutableAliasPublicWhole23g []byte

type immutableAliasPublicData23g struct {
	Command     string
	PublicCases []struct {
		Name                                                   string
		Input                                                  request
		Facts                                                  response
		RequestBytes, PacketBytes, RequestSHA256, PacketSHA256 string
	}
	BoundaryCases []struct {
		Name           string
		Input          request
		ExpectedAnchor struct {
			Kind, Spelling string
			Start, End     int
		}
		ExpectedResponse      response
		ExpectedCallbackCount int
	}
}

// Complete literals were independently worked before observing this public
// producer. Origin here means all annotation/export source records in payload;
// no executable provenance, Source admission, Core or VM execution is claimed.
func TestModuleCallableImmutableAliasPublicWhole(t *testing.T) {
	var data immutableAliasPublicData23g
	if err := json.Unmarshal(immutableAliasPublicWhole23g, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.PublicCases) != 2 || data.Command != "--module-callable-facts-io-v2" {
		t.Fatal("exact independently frozen private2 bool/i32 cases required")
	}
	for _, tc := range data.PublicCases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Name == "bool-original-shared" {
				var first immutableAliasWholeFixture23g
				if err := json.Unmarshal(immutableAliasWhole23g, &first); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(tc.Input, first.Input) || !reflect.DeepEqual(tc.Facts, first.ExpectedFacts) {
					t.Fatal("original bool typed input and complete raw whole must remain unchanged")
				}
			}
			input, expected := []byte(tc.RequestBytes), []byte(tc.PacketBytes)
			requestSHA, packetSHA := sha256.Sum256(input), sha256.Sum256(expected)
			if hex.EncodeToString(requestSHA[:]) != tc.RequestSHA256 || hex.EncodeToString(packetSHA[:]) != tc.PacketSHA256 ||
				len(input) == 0 || input[len(input)-1] != 0x0a || len(expected) == 0 || expected[len(expected)-1] != 0x0a {
				t.Fatal("complete canonical plus-LF request and package literals changed")
			}
			before := bytes.Clone(input)
			output := &mcPublicWriter23d{}
			if err := runFrontendCommand(t.Context(), []string{data.Command}, bytes.NewReader(input), output); err != nil {
				t.Fatalf("official-valid immutable alias must publish complete private2 facts: %v", err)
			}
			t.Logf("TSFG_M23G_PUBLIC_WHOLE_ACTUAL name=%s base64=%s writes=%d", tc.Name, base64.StdEncoding.EncodeToString(output.Bytes()), output.writes)
			if output.writes != 1 || !bytes.Equal(output.Bytes(), expected) || !bytes.Equal(input, before) {
				t.Fatalf("complete prewritten public packet or exact one-write boundary differs for %s", tc.Name)
			}
			var packet moduleCallablePacket
			if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(packet.Payload.Facts, tc.Facts) {
				t.Fatal("public packet raw facts differ from the independent typed whole")
			}
		})
	}
}

// The nominal i32 positive intentionally has no scalar global initializer.
// It shares alias/required-signature restrictions with bool, not bool's shared
// effect scene. No owned scalar assignment or projection helper is used here.
func TestModuleCallableImmutableAliasI32OfficialPrior(t *testing.T) {
	var data immutableAliasPublicData23g
	if err := json.Unmarshal(immutableAliasPublicWhole23g, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.PublicCases) != 2 || data.PublicCases[1].Name != "i32-nominal-forwarding" {
		t.Fatal("independent i32 case missing")
	}
	r := data.PublicCases[1].Input
	contents := map[string]string{}
	fileNames := []string{}
	for _, file := range r.Files {
		contents[file.Path] = file.Content
		fileNames = append(fileNames, file.Path)
	}
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
			FileNames:       fileNames,
			CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext},
		}},
		Host: compiler.NewCompilerHost(r.SourceRoot, vfstest.FromMap(contents, true), r.SourceRoot, nil, nil), SingleThreaded: core.TSTrue,
	})
	if ds := program.GetSyntacticDiagnostics(t.Context(), nil); len(ds) != 0 {
		t.Fatalf("official i32 syntax: %+v", ds)
	}
	if ds := program.GetSemanticDiagnostics(t.Context(), nil); len(ds) != 0 {
		t.Fatalf("official i32 semantic: %+v", ds)
	}
	if ds := program.GetProgramDiagnostics(); len(ds) != 0 {
		t.Fatalf("official i32 program: %+v", ds)
	}
	if ds := program.GetGlobalDiagnostics(t.Context()); len(ds) != 0 {
		t.Fatalf("official i32 global: %+v", ds)
	}
	app, dts := program.GetSourceFile(r.RootFile), program.GetSourceFile(r.Declarations.Paths[0])
	if app == nil || dts == nil || len(program.GetSourceFiles()) != 2 || app.Text() != r.Files[0].Content || dts.Text() != r.Files[1].Content || len(app.Statements.Nodes) != 4 {
		t.Fatal("complete original nominal source/DTS closure differs")
	}
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), app)
	defer release()
	var nominal *checker.Type
	for _, declaration := range dts.Statements.Nodes {
		if declaration.Kind == ast.KindTypeAliasDeclaration && declaration.Name().Text() == "i32" {
			if nominal != nil {
				t.Fatal("repeated original i32 declaration")
			}
			nominal = check.GetDeclaredTypeOfSymbol(check.GetSymbolAtLocation(declaration.Name()))
		}
	}
	if nominal == nil || nominal.IsBooleanLike() {
		t.Fatal("original nominal i32 type missing")
	}
	nodes := app.Statements.Nodes
	if nodes[0].Kind != ast.KindFunctionDeclaration || nodes[1].Kind != ast.KindVariableStatement || nodes[2].Kind != ast.KindFunctionDeclaration || nodes[3].Kind != ast.KindFunctionDeclaration {
		t.Fatal("complete four-declaration i32 shape differs")
	}
	leaf, entry, peer := nodes[0], nodes[2], nodes[3]
	decls := nodes[1].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
	if len(decls) != 1 || decls[0].Name().Text() != "op" || nodes[1].AsVariableStatement().DeclarationList.Flags&ast.NodeFlagsConst == 0 {
		t.Fatal("single original const op required")
	}
	op := decls[0]
	if check.GetSymbolAtLocation(op.AsVariableDeclaration().Initializer) != check.GetSymbolAtLocation(leaf.Name()) || check.GetSymbolAtLocation(op.Name()) == check.GetSymbolAtLocation(leaf.Name()) {
		t.Fatal("op and initializer original leaf symbols differ from oracle")
	}
	for _, node := range []*ast.Node{leaf, entry, peer} {
		decl := node.AsFunctionDeclaration()
		sigs := check.GetSignaturesOfType(check.GetTypeAtLocation(decl.Name()), checker.SignatureKindCall)
		if len(decl.Parameters.Nodes) != 1 || len(sigs) != 1 || sigs[0].Declaration() != node || len(sigs[0].Parameters()) != 1 || sigs[0].MinArgumentCount() != 1 || sigs[0].HasRestParameter() || sigs[0].ThisParameter() != nil || len(sigs[0].TypeParameters()) != 0 || check.GetTypeOfSymbol(sigs[0].Parameters()[0]) != nominal || check.GetReturnTypeOfSignature(sigs[0]) != nominal {
			t.Fatal("complete original single-required nominal signature differs")
		}
	}
	leafReturn := leaf.Body().AsBlock().Statements.Nodes[0].AsReturnStatement().Expression
	peerReturn := peer.Body().AsBlock().Statements.Nodes[0].AsReturnStatement().Expression
	callNode := entry.Body().AsBlock().Statements.Nodes[0].AsReturnStatement().Expression
	if callNode.Kind != ast.KindCallExpression {
		t.Fatal("actual original op call missing")
	}
	call := callNode.AsCallExpression()
	resolved := check.GetResolvedSignature(callNode)
	if len(call.Arguments.Nodes) != 1 || call.TypeArguments != nil || call.QuestionDotToken != nil || resolved == nil || resolved.Declaration() != leaf || check.GetReturnTypeOfSignature(resolved) != nominal || check.GetSymbolAtLocation(call.Expression) != check.GetSymbolAtLocation(op.Name()) || check.GetSymbolAtLocation(call.Arguments.Nodes[0]) != check.GetSymbolAtLocation(entry.AsFunctionDeclaration().Parameters.Nodes[0].Name()) || check.GetSymbolAtLocation(leafReturn) != check.GetSymbolAtLocation(leaf.AsFunctionDeclaration().Parameters.Nodes[0].Name()) || check.GetSymbolAtLocation(peerReturn) != check.GetSymbolAtLocation(peer.AsFunctionDeclaration().Parameters.Nodes[0].Name()) {
		t.Fatal("complete original i32 alias argument and returns differ")
	}
	t.Log("TSFG_M23G_I32_OFFICIAL_PASS fourDiagnostics0 nominalDTSOriginal=true originalResolvedLeaf=true")
}
