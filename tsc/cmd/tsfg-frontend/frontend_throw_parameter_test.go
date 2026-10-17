// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/scanner"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

// This first tracer observes owned Source facts only. It grants no catch/finally,
// unknown narrowing, exception transport, consumer or execution support.
// Raw Node.Pos ranges deliberately differ from token/diagnostic starts.
func TestFrontendThrowI32ParameterOwnedFacts(t *testing.T) {
	const source = "// 😀\nexport function entry(value: i32): i32 { throw value; }"
	const sourceSHA = "68df2dc0be8305b9038b69214f03fb51dce957699806e629b5bbff02765311a7"
	sum := sha256.Sum256([]byte(source))
	if len(source) != 63 || len(utf16.Encode([]rune(source))) != 61 || hex.EncodeToString(sum[:]) != sourceSHA {
		t.Fatal("frozen UTF8/UTF16 source identity changed")
	}
	r := integerRequest(source)
	if r.RootFile != "/src/main.ts" || len(r.Files) != 2 || r.Files[0].Content != source ||
		r.Declarations.Version != "minimal-integers-v3" || len(r.Declarations.Paths) != 1 ||
		r.Declarations.Paths[0] != "/src/minimal-integers-v3.d.ts" {
		t.Fatal("fixed original root/declaration request changed")
	}
	original, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		after, err := json.Marshal(r)
		if err != nil || !bytes.Equal(original, after) {
			t.Fatal("official/public acquisition mutated the original request")
		}
	}()

	if !t.Run("officialOracle", func(t *testing.T) {
		contents := map[string]string{}
		for _, input := range r.Files {
			contents[input.Path] = input.Content
		}
		program := compiler.NewProgram(compiler.ProgramOptions{
			Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
				FileNames: append([]string{r.RootFile}, r.Declarations.Paths...),
				CompilerOptions: &core.CompilerOptions{
					NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext,
				},
			}},
			Host:           compiler.NewCompilerHost(r.SourceRoot, vfstest.FromMap(contents, true), r.SourceRoot, nil, nil),
			SingleThreaded: core.TSTrue,
		})
		for _, group := range []struct {
			name  string
			items []*ast.Diagnostic
		}{
			{"syntactic", program.GetSyntacticDiagnostics(t.Context(), nil)},
			{"semantic", program.GetSemanticDiagnostics(t.Context(), nil)},
			{"program", program.GetProgramDiagnostics()},
			{"global", program.GetGlobalDiagnostics(t.Context())},
		} {
			if len(group.items) != 0 {
				t.Fatalf("official %s diagnostics must pass before projection credit: %+v", group.name, group.items)
			}
		}
		file := program.GetSourceFile(r.RootFile)
		if file == nil || file.Text() != source || len(file.Statements.Nodes) != 1 {
			t.Fatal("fresh official Program lost the one original function")
		}
		check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
		defer release()
		fnNode := file.Statements.Nodes[0]
		if fnNode.Kind != ast.KindFunctionDeclaration || fnNode.Name() == nil || fnNode.Name().Text() != "entry" {
			t.Fatal("original official entry declaration changed")
		}
		fn := fnNode.AsFunctionDeclaration()
		if fn.Type == nil || fn.TypeParameters != nil || len(fn.Parameters.Nodes) != 1 ||
			fn.Body == nil || fn.Body.Kind != ast.KindBlock || len(fn.Body.AsBlock().Statements.Nodes) != 1 {
			t.Fatal("official required typed function/body shape changed")
		}
		parameterNode := fn.Parameters.Nodes[0]
		parameter := parameterNode.AsParameterDeclaration()
		if parameterNode.Name() == nil || parameterNode.Name().Kind != ast.KindIdentifier ||
			parameterNode.Name().Text() != "value" || parameter.Type == nil || parameter.QuestionToken != nil ||
			parameter.Initializer != nil || parameter.DotDotDotToken != nil {
			t.Fatal("original parameter must be one required scalar i32 binding")
		}
		thrown := fn.Body.AsBlock().Statements.Nodes[0]
		if thrown.Kind != ast.KindThrowStatement {
			t.Fatal("official body is not the one original throw statement")
		}
		payload := thrown.AsThrowStatement().Expression
		if payload == nil || payload.Kind != ast.KindIdentifier || payload.Text() != "value" {
			t.Fatal("official payload is not the one original parameter reference")
		}
		functionSymbol := check.GetSymbolAtLocation(fnNode.Name())
		parameterSymbol := check.GetSymbolAtLocation(parameterNode.Name())
		if functionSymbol == nil || parameterSymbol == nil || functionSymbol == parameterSymbol ||
			len(functionSymbol.Declarations) != 1 || functionSymbol.Declarations[0] != fnNode ||
			len(parameterSymbol.Declarations) != 1 || parameterSymbol.Declarations[0] != parameterNode ||
			check.GetSymbolAtLocation(payload) != parameterSymbol {
			t.Fatal("official declaration/payload lost its distinct actual symbol identity")
		}
		declared := check.GetTypeFromTypeNode(parameter.Type)
		if declared == nil || declared.Alias() == nil || declared.Alias().Symbol() == nil ||
			declared.Alias().Symbol().Name != "i32" || check.GetTypeFromTypeNode(fn.Type) != declared ||
			check.GetTypeOfSymbol(parameterSymbol) != declared || check.GetTypeAtLocation(parameterNode.Name()) != declared ||
			check.GetTypeAtLocation(payload) != declared {
			t.Fatal("official payload, parameter, annotation and result must share the declared nominal i32 type object")
		}
		signatures := check.GetSignaturesOfType(check.GetTypeOfSymbol(functionSymbol), checker.SignatureKindCall)
		if len(signatures) != 1 || signatures[0].Declaration() != fnNode || len(signatures[0].Parameters()) != 1 ||
			signatures[0].Parameters()[0] != parameterSymbol || check.GetReturnTypeOfSignature(signatures[0]) != declared {
			t.Fatal("official complete signature differs from the original typed declaration")
		}
		exports := check.GetExportsOfModule(file.AsNode().Symbol())
		if len(exports) != 1 || exports[0].Name != "entry" || exports[0].Flags&ast.SymbolFlagsAlias != 0 ||
			len(exports[0].Declarations) != 1 || exports[0].Declarations[0] != fnNode ||
			ast.GetSourceFileOfNode(exports[0].Declarations[0]) != file {
			t.Fatal("complete official exports must contain exactly the actual root entry")
		}
		positions := file.GetPositionMap()
		for _, anchor := range []struct {
			name       string
			node       *ast.Node
			raw, token span
		}{
			{"function", fnNode, span{0, 61}, span{6, 61}},
			{"definition", fnNode.Name(), span{21, 27}, span{22, 27}},
			{"parameter", parameterNode, span{28, 38}, span{28, 38}},
			{"parameterName", parameterNode.Name(), span{28, 33}, span{28, 33}},
			{"parameterAnnotation", parameter.Type, span{34, 38}, span{35, 38}},
			{"resultAnnotation", fn.Type, span{40, 44}, span{41, 44}},
			{"throw", thrown, span{46, 59}, span{47, 59}},
			{"payload", payload, span{52, 58}, span{53, 58}},
		} {
			raw := span{positions.UTF8ToUTF16(anchor.node.Pos()), positions.UTF8ToUTF16(anchor.node.End())}
			token := span{positions.UTF8ToUTF16(scanner.SkipTrivia(file.Text(), anchor.node.Pos())), raw.End}
			if raw != anchor.raw || token != anchor.token {
				t.Fatalf("independent original UTF16 %s differs: raw=%+v want=%+v token=%+v want=%+v",
					anchor.name, raw, anchor.raw, token, anchor.token)
			}
		}
		if string(utf16.Decode(utf16.Encode([]rune(source))[47:52])) != "throw" {
			t.Fatal("independent throw keyword token anchor changed")
		}
		t.Log("official four diagnostic groups, same-checker AST/symbol/signature/type objects, full exports and raw/token UTF16 anchors passed")
	}) {
		return
	}

	t.Run("publicProjection", func(t *testing.T) {
		t.Logf("TSFG_THROW_PARAMETER_REQUEST_V1 %s", base64.StdEncoding.EncodeToString(original))
		t.Logf("TSFG_THROW_PARAMETER_SOURCE_V1 %s", base64.StdEncoding.EncodeToString([]byte(source)))
		var output bytes.Buffer
		commandErr := runFrontendCommand(t.Context(), nil, bytes.NewReader(original), &output)
		raw := bytes.Clone(output.Bytes())
		// Always retain the complete actual public response before assertions.
		t.Logf("TSFG_THROW_PARAMETER_RESPONSE_V1 %s", base64.StdEncoding.EncodeToString(raw))
		if commandErr != nil {
			t.Fatalf("public command failed after official oracle PASS: %v; rawResponse=%s", commandErr, raw)
		}
		where := func(start, end int) map[string]int { return map[string]int{"start": start, "end": end} }
		parameter := map[string]any{
			"id": "s2", "name": "value", "kind": "parameter", "type": "i32", "writable": true, "range": where(28, 33),
			"presence": map[string]any{
				"undefined": false, "null": false, "optional": false, "default": false,
				"range": where(28, 38), "annotation": where(35, 38),
			},
		}
		definition := map[string]any{
			"id": "s1", "name": "entry", "kind": "function", "type": "i32", "writable": false, "range": where(21, 27),
		}
		expected := map[string]any{
			"status": "ok", "inputs": []string{"/src/main.ts", "/src/minimal-integers-v3.d.ts"},
			"symbols": []any{definition, parameter},
			"functions": []any{map[string]any{
				"name": "entry", "symbol": "s1", "returnType": "i32", "range": where(0, 61),
				"parameters": []any{parameter}, "parameterInitializers": []any{nil},
				"body": []any{map[string]any{
					"kind": "throw", "range": where(46, 59),
					"expression": map[string]any{"kind": "reference", "type": "i32", "symbol": "s2", "range": where(52, 58)},
				}},
			}},
		}
		expectedBytes, err := json.Marshal(expected)
		if err != nil {
			t.Fatal(err)
		}
		var got, want any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("public response is not JSON: %v; rawResponse=%s", err, raw)
		}
		if err := json.Unmarshal(expectedBytes, &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("official oracle PASS; public owned facts differ from the independently frozen complete throw/reference facts:\nwant=%s\nrawResponse=%s", expectedBytes, raw)
		}
		t.Log("one owned typed throw, exactly one original parameter payload subtree, complete symbols/inputs and raw source anchors passed")
	})
}
