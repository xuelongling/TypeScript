// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

func TestFrontendCommandNullableScalarEntryOwnedFacts(t *testing.T) {
	const source = "// 😀\nfunction echo(value: i32 | null): i32 | null { return value; }\nexport function entry(value: i32): i32 { const result: i32 | null = echo(value); return value; }"
	if len(source) != 167 || len(utf16.Encode([]rune(source))) != 165 {
		t.Fatal("original emoji-prefixed literal lost its UTF8/UTF16 distinction")
	}
	r := integerRequest(source)
	originalRequest, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if !t.Run("officialDiagnostics", func(t *testing.T) { nullableBoundaryOfficialForTest(t, r, 0) }) {
		return
	}

	// This fresh official oracle is test evidence, not a production package or
	// an export flag inferred from source text or from the owned response.
	contents := map[string]string{}
	for _, file := range r.Files {
		contents[file.Path] = file.Content
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
	file := program.GetSourceFile(r.RootFile)
	if file == nil || len(file.Statements.Nodes) != 2 {
		t.Fatal("official program omitted the two original declarations")
	}
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	echoNode, entryNode := file.Statements.Nodes[0], file.Statements.Nodes[1]
	if echoNode.Kind != ast.KindFunctionDeclaration || entryNode.Kind != ast.KindFunctionDeclaration {
		t.Fatal("official declarations are not the original two functions")
	}
	echoDecl, entryDecl := echoNode.AsFunctionDeclaration(), entryNode.AsFunctionDeclaration()
	exports := check.GetExportsOfModule(file.AsNode().Symbol())
	entryBinding := check.GetSymbolAtLocation(entryDecl.Name())
	echoBinding := check.GetSymbolAtLocation(echoDecl.Name())
	if len(exports) != 1 || exports[0].Name != "entry" || entryBinding == nil || echoBinding == nil || entryBinding == echoBinding {
		t.Fatal("official export set must contain exactly the distinct scalar entry binding")
	}
	if len(exports[0].Declarations) != 1 || exports[0].Declarations[0] != entryNode {
		t.Fatal("official export binding does not point to the actual entry declaration")
	}
	if len(echoDecl.Parameters.Nodes) != 1 || len(entryDecl.Parameters.Nodes) != 1 || echoDecl.Body == nil || entryDecl.Body == nil {
		t.Fatal("official function parameters or bodies changed")
	}
	echoBody := echoDecl.Body.AsBlock().Statements.Nodes
	entryBody := entryDecl.Body.AsBlock().Statements.Nodes
	if len(echoBody) != 1 || echoBody[0].Kind != ast.KindReturnStatement || len(entryBody) != 2 || entryBody[0].Kind != ast.KindVariableStatement || entryBody[1].Kind != ast.KindReturnStatement {
		t.Fatal("official source lost the saved call or original returns")
	}
	locals := entryBody[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
	if len(locals) != 1 || locals[0].Kind != ast.KindVariableDeclaration {
		t.Fatal("official source lost the single nullable local")
	}
	localNode := locals[0]
	callNode := localNode.AsVariableDeclaration().Initializer
	if callNode == nil || callNode.Kind != ast.KindCallExpression || len(callNode.AsCallExpression().Arguments.Nodes) != 1 {
		t.Fatal("official initializer lost its ordered call argument")
	}
	callAst := callNode.AsCallExpression()
	resolved := check.GetResolvedSignature(callNode)
	if resolved == nil || resolved.Declaration() != echoNode || check.GetSymbolAtLocation(callAst.Expression) != echoBinding || len(resolved.Parameters()) != 1 {
		t.Fatal("official call signature or target is not the actual echo declaration")
	}
	echoParameter := check.GetSymbolAtLocation(echoDecl.Parameters.Nodes[0].Name())
	entryParameter := check.GetSymbolAtLocation(entryDecl.Parameters.Nodes[0].Name())
	localBinding := check.GetSymbolAtLocation(localNode.Name())
	if echoParameter == nil || entryParameter == nil || localBinding == nil || echoParameter == entryParameter || localBinding == entryParameter || localBinding == echoParameter || check.GetSymbolAtLocation(echoBody[0].AsReturnStatement().Expression) != echoParameter || check.GetSymbolAtLocation(callAst.Arguments.Nodes[0]) != entryParameter || check.GetSymbolAtLocation(entryBody[1].AsReturnStatement().Expression) != entryParameter {
		t.Fatal("official uses lost their original distinct binding identities")
	}
	positions := file.GetPositionMap()
	anchors := map[string]span{}
	for label, node := range map[string]*ast.Node{
		"echo": echoNode, "entry": entryNode, "echoDefinition": echoDecl.Name(), "entryDefinition": entryDecl.Name(),
		"echoParameter": echoDecl.Parameters.Nodes[0].Name(), "entryParameter": entryDecl.Parameters.Nodes[0].Name(),
		"local": localNode.Name(), "localDeclaration": localNode, "call": callNode, "argument": callAst.Arguments.Nodes[0],
		"echoReturn": echoBody[0], "echoReference": echoBody[0].AsReturnStatement().Expression,
		"entryReturn": entryBody[1], "entryReference": entryBody[1].AsReturnStatement().Expression,
	} {
		anchors[label] = span{Start: positions.UTF8ToUTF16(node.Pos()), End: positions.UTF8ToUTF16(node.End())}
	}
	wantAnchors := map[string]span{
		"echo": {0, 68}, "entry": {68, 165}, "echoDefinition": {14, 19}, "entryDefinition": {84, 90},
		"echoParameter": {20, 25}, "entryParameter": {91, 96}, "local": {115, 122}, "localDeclaration": {115, 148},
		"call": {136, 148}, "argument": {142, 147}, "echoReturn": {52, 66}, "echoReference": {59, 65},
		"entryReturn": {149, 163}, "entryReference": {156, 162},
	}
	if !reflect.DeepEqual(anchors, wantAnchors) {
		t.Fatalf("official original UTF16 anchors differ: got=%+v want=%+v", anchors, wantAnchors)
	}

	t.Run("ownedProjection", func(t *testing.T) {
		result, raw := nullableBoundaryCommandForTest(t, r)
		if result.Status != "ok" || len(result.Diagnostics) != 0 || len(result.Functions) != 2 || len(result.Symbols) != 5 {
			t.Fatalf("existing scalar-entry nullable facts were lost: %+v; rawResponse=%s", result, raw)
		}
		if len(r.Files) != 2 || r.Files[0].Path != r.RootFile || r.Files[0].Content != source || r.Files[1].Path != r.Declarations.Paths[0] || r.Files[1].Content == "" {
			t.Fatal("request changed the original main/declaration files")
		}
		afterRequest, err := json.Marshal(r)
		if err != nil || string(afterRequest) != string(originalRequest) {
			t.Fatal("public command changed the original request")
		}
		echo, entry := result.Functions[0], result.Functions[1]
		if echo.Name != "echo" || entry.Name != exports[0].Name || echo.Symbol == "" || entry.Symbol == "" || echo.Symbol == entry.Symbol || echo.Range != anchors["echo"] || entry.Range != anchors["entry"] || echo.ReturnType != "i32" || entry.ReturnType != "i32" || echo.ReturnPresence == nil || entry.ReturnPresence != nil || len(echo.Parameters) != 1 || len(entry.Parameters) != 1 || len(echo.Body) != 1 || len(entry.Body) != 2 {
			t.Fatalf("owned declarations differ from the independent official oracle: %+v", result.Functions)
		}
		symbols := map[string]symbol{}
		for _, owned := range result.Symbols {
			if owned.ID == "" || symbols[owned.ID].ID != "" {
				t.Fatalf("missing or duplicate owned identity: %+v", owned)
			}
			symbols[owned.ID] = owned
		}
		for _, item := range []struct {
			fn   function
			name string
		}{
			{echo, "echo"},
			{entry, "entry"},
		} {
			definition := symbols[item.fn.Symbol]
			if definition.Name != item.name || definition.Kind != "function" || definition.Type != "i32" || definition.Writable || definition.Range != anchors[item.name+"Definition"] || !reflect.DeepEqual(definition.ReturnPresence, item.fn.ReturnPresence) {
				t.Fatalf("function defining symbol disagrees with official declaration: %+v", definition)
			}
			parameter := item.fn.Parameters[0]
			if parameter.ID == "" || parameter.Name != "value" || parameter.Kind != "parameter" || parameter.Type != "i32" || !parameter.Writable || parameter.Range != anchors[item.name+"Parameter"] || !reflect.DeepEqual(symbols[parameter.ID], parameter) {
				t.Fatalf("parameter lost its owned defining identity: %+v", parameter)
			}
		}
		if echo.Parameters[0].ID == entry.Parameters[0].ID {
			t.Fatal("same-spelled parameters in distinct functions were merged")
		}
		checkPresence := func(shape *parameterPresence, declaration, annotation span, nullable bool) {
			t.Helper()
			if shape == nil || shape.Undefined || shape.Null != nullable || shape.Optional || shape.Default || shape.Range != declaration || shape.Annotation != annotation {
				t.Fatalf("declaration presence lost original members/UTF16 anchors: %+v", shape)
			}
		}
		checkPresence(echo.Parameters[0].Presence, span{20, 37}, span{27, 37}, true)
		checkPresence(entry.Parameters[0].Presence, span{91, 101}, span{98, 101}, false)
		if len(echo.ParameterInitializers) != 1 || echo.ParameterInitializers[0] != nil || len(entry.ParameterInitializers) != 1 || entry.ParameterInitializers[0] != nil {
			t.Fatal("required parameters acquired a default initializer")
		}
		checkReference := func(value *expression, binding string, where span, nullable bool) {
			t.Helper()
			if value == nil || value.Kind != "reference" || value.Type != "i32" || value.Symbol != binding || value.Range != where || value.Target != "" {
				t.Fatalf("reference lost original binding/UTF16 anchor: %+v", value)
			}
			if nullable {
				if value.PresenceFlow == nil || value.PresenceFlow.Undefined || !value.PresenceFlow.Null {
					t.Fatalf("nullable reference lost official use-site flow: %+v", value)
				}
			} else if value.PresenceFlow != nil {
				t.Fatalf("ordinary scalar reference acquired nullable flow: %+v", value)
			}
		}
		if echo.Body[0].Kind != "return" || echo.Body[0].Range != anchors["echoReturn"] || entry.Body[1].Kind != "return" || entry.Body[1].Range != anchors["entryReturn"] {
			t.Fatal("return statements lost original source order or anchors")
		}
		checkReference(echo.Body[0].Expression, echo.Parameters[0].ID, anchors["echoReference"], true)
		checkReference(entry.Body[1].Expression, entry.Parameters[0].ID, anchors["entryReference"], false)
		variable := entry.Body[0]
		if variable.Kind != "variable" || variable.Range != anchors["localDeclaration"] || variable.Symbol == nil || variable.Initializer == nil {
			t.Fatalf("unused nullable local or initializer call was silently dropped: %+v", variable)
		}
		local := variable.Symbol
		if local.ID == "" || local.Name != "result" || local.Kind != "local" || local.Type != "i32" || local.Writable || local.Range != anchors["local"] || !reflect.DeepEqual(symbols[local.ID], *local) {
			t.Fatalf("const local lost its distinct declaration: %+v", local)
		}
		checkPresence(local.Presence, span{115, 148}, span{124, 134}, true)
		call := variable.Initializer
		if call.Kind != "call" || call.Type != "i32" || call.Target != echo.Symbol || call.Range != anchors["call"] || call.Arguments == nil || len(*call.Arguments) != 1 || call.Signature == nil || call.PresenceFlow == nil || call.PresenceFlow.Undefined || !call.PresenceFlow.Null {
			t.Fatalf("saved call lost callee/signature/independent nullable flow: %+v", call)
		}
		argument := (*call.Arguments)[0]
		checkReference(&argument, entry.Parameters[0].ID, anchors["argument"], false)
		sig := call.Signature
		if len(sig.Parameters) != 1 || sig.Parameters[0] != "i32" || sig.ReturnType != "i32" || len(sig.Presence) != 1 || !reflect.DeepEqual(sig.Presence[0], echo.Parameters[0].Presence) || !reflect.DeepEqual(sig.ReturnPresence, echo.ReturnPresence) || sig.ReturnSignature != nil || len(sig.ParameterSignatures) != 0 {
			t.Fatalf("resolved call signature differs from the actual echo declaration: %+v", sig)
		}
		// Check exact returnPresence JSON fields, including explicit false.
		echoRecord := nullableBoundaryRecordForTest(t, raw, "functions", "name", "echo")
		entryRecord := nullableBoundaryRecordForTest(t, raw, "functions", "name", "entry")
		nullableBoundaryReturnShapeForTest(t, echoRecord["returnPresence"], source, false, true, span{0, 68}, span{40, 50}, "i32 | null", "// 😀\nfunction echo(value: i32 | null): i32 | null { return value; }")
		echoSymbol := nullableBoundaryRecordForTest(t, raw, "symbols", "id", echo.Symbol)
		if string(echoSymbol["returnPresence"]) != string(echoRecord["returnPresence"]) || len(entryRecord["returnPresence"]) != 0 {
			t.Fatal("function return presence leaked onto scalar entry or lost defining symbol equality")
		}
		var statements []struct {
			Initializer struct {
				Signature struct {
					ReturnPresence json.RawMessage
				}
			}
		}
		if err := json.Unmarshal(entryRecord["body"], &statements); err != nil || len(statements) != 2 {
			t.Fatalf("raw body lost saved call: %v", err)
		}
		nullableBoundaryReturnShapeForTest(t, statements[0].Initializer.Signature.ReturnPresence, source, false, true, span{0, 68}, span{40, 50}, "i32 | null", "// 😀\nfunction echo(value: i32 | null): i32 | null { return value; }")
		oracle, err := json.Marshal(map[string]any{
			"kind": "test-only-official-export-oracle", "sourcePath": r.RootFile,
			"exports": []map[string]any{{"name": exports[0].Name, "targetOwnedSymbol": entry.Symbol, "declaration": anchors["entry"], "definition": anchors["entryDefinition"]}},
			"anchors": anchors,
		})
		if err != nil {
			t.Fatal(err)
		}
		original, err := json.Marshal(map[string]any{"rootFile": r.RootFile, "files": r.Files})
		if err != nil {
			t.Fatal(err)
		}
		// These records acquire existing public facts only. No private-package,
		// executable, Core consumer or runtime result is claimed by this test.
		t.Logf("TSFG_NULLABLE_SCALAR_ENTRY_REQUEST_V1 %s", base64.StdEncoding.EncodeToString(originalRequest))
		t.Logf("TSFG_NULLABLE_SCALAR_ENTRY_RESPONSE_V1 %s", base64.StdEncoding.EncodeToString(raw))
		t.Logf("TSFG_NULLABLE_SCALAR_ENTRY_ORIGINAL_V1 %s", base64.StdEncoding.EncodeToString(original))
		t.Logf("TSFG_NULLABLE_SCALAR_ENTRY_MAIN_V1 %s", base64.StdEncoding.EncodeToString([]byte(r.Files[0].Content)))
		t.Logf("TSFG_NULLABLE_SCALAR_ENTRY_DTS_V1 %s", base64.StdEncoding.EncodeToString([]byte(r.Files[1].Content)))
		t.Logf("TSFG_NULLABLE_SCALAR_ENTRY_OFFICIAL_EXPORT_ORACLE_V1 %s", base64.StdEncoding.EncodeToString(oracle))
	})
}
