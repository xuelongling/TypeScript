// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/json"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

func TestNullableFactsCommandNullishCoalesce(t *testing.T) {
	const source = "function fallback(value: i32): i32 { return value; }\nfunction coalesce(value: i32 | null, replacement: i32): i32 { return value ?? fallback(replacement); }\nexport function entry(value: i32): i32 { return coalesce(value, value); }"
	const schema = "sha256:badc9425050a8f538fcb7b9397e5ef31fbde96391989dfeebb8cab6a122783bd"
	digest := func(data []byte) string {
		sum := sha256.Sum256(data)
		return "sha256:" + hex.EncodeToString(sum[:])
	}
	if len(source) != 229 || len(utf16.Encode([]rune(source))) != 229 || digest([]byte(source)) != "sha256:7a250e785ef05abe46da6f8fe4953c9f50305c6d8090086f3af02f8a71552d94" {
		t.Fatal("frozen original 229-byte source changed")
	}
	// Official JSON canonicalization is independent of private package helpers.
	canonical := func(value any) []byte {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		result := json.Value(data)
		if err := result.Canonicalize(); err != nil {
			t.Fatal(err)
		}
		return result
	}
	r := integerRequest(source)
	originalInput := canonical(r)
	defer func() {
		if !bytes.Equal(canonical(r), originalInput) {
			t.Fatal("official/public/private acquisition modified the original request")
		}
	}()
	if !t.Run("officialDiagnostics", func(t *testing.T) { nullableBoundaryOfficialForTest(t, r, 0) }) {
		return
	}
	anchors := map[string]span{}
	if !t.Run("officialOracle", func(t *testing.T) {
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
		if file == nil || len(file.Statements.Nodes) != 3 {
			t.Fatal("fresh official Program omitted original functions")
		}
		check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
		defer release()
		positions := file.GetPositionMap()
		remember := func(label string, node *ast.Node, token bool) {
			t.Helper()
			if node == nil {
				t.Fatalf("official node missing: %s", label)
			}
			start := node.Pos()
			if token {
				for start < node.End() && (source[start] == ' ' || source[start] == '\n') {
					start++
				}
			}
			anchors[label] = span{positions.UTF8ToUTF16(start), positions.UTF8ToUTF16(node.End())}
		}
		names := []string{"fallback", "coalesce", "entry"}
		decls := []*ast.FunctionDeclaration{}
		returns := []*ast.Node{}
		bindings := map[*ast.Symbol]bool{}
		for index, node := range file.Statements.Nodes {
			if node.Kind != ast.KindFunctionDeclaration || node.Name() == nil {
				t.Fatal("official top level is not three named original functions")
			}
			decl := node.AsFunctionDeclaration()
			if decl.Body == nil || decl.Body.Kind != ast.KindBlock || len(decl.Body.AsBlock().Statements.Nodes) != 1 || decl.Body.AsBlock().Statements.Nodes[0].Kind != ast.KindReturnStatement {
				t.Fatal("official function body is not one original return")
			}
			for _, defining := range append([]*ast.Node{decl.Name()}, func() []*ast.Node {
				result := []*ast.Node{}
				for _, p := range decl.Parameters.Nodes {
					result = append(result, p.Name())
				}
				return result
			}()...) {
				binding := check.GetSymbolAtLocation(defining)
				if binding == nil || bindings[binding] {
					t.Fatal("official declarations merged distinct original symbols")
				}
				bindings[binding] = true
			}
			remember(names[index], node, false)
			remember(names[index]+"Definition", decl.Name(), false)
			for position, p := range decl.Parameters.Nodes {
				label := names[index] + "Parameter"
				if position == 1 {
					label += "2"
				}
				remember(label, p.Name(), false)
			}
			remember(names[index]+"Return", decl.Body.AsBlock().Statements.Nodes[0], false)
			decls = append(decls, decl)
			returns = append(returns, decl.Body.AsBlock().Statements.Nodes[0].AsReturnStatement().Expression)
		}
		if len(bindings) != 7 || len(decls[0].Parameters.Nodes) != 1 || len(decls[1].Parameters.Nodes) != 2 || len(decls[2].Parameters.Nodes) != 1 || returns[0].Kind != ast.KindIdentifier || returns[1].Kind != ast.KindBinaryExpression || returns[2].Kind != ast.KindCallExpression {
			t.Fatal("official parameter or return structure changed")
		}
		binary := returns[1].AsBinaryExpression()
		if binary.OperatorToken.Kind != ast.KindQuestionQuestionToken || binary.Left.Kind != ast.KindIdentifier || binary.Right.Kind != ast.KindCallExpression {
			t.Fatal("official coalesce must retain one lhs and a lazy RHS call")
		}
		rhs, entryCall := binary.Right.AsCallExpression(), returns[2].AsCallExpression()
		if len(rhs.Arguments.Nodes) != 1 || len(entryCall.Arguments.Nodes) != 2 || check.GetResolvedSignature(binary.Right) == nil || check.GetResolvedSignature(binary.Right).Declaration() != file.Statements.Nodes[0] || check.GetResolvedSignature(returns[2]) == nil || check.GetResolvedSignature(returns[2]).Declaration() != file.Statements.Nodes[1] {
			t.Fatal("official calls lost actual declaration signatures")
		}
		if check.GetSymbolAtLocation(rhs.Expression) != check.GetSymbolAtLocation(decls[0].Name()) || check.GetSymbolAtLocation(entryCall.Expression) != check.GetSymbolAtLocation(decls[1].Name()) || check.GetSymbolAtLocation(returns[0]) != check.GetSymbolAtLocation(decls[0].Parameters.Nodes[0].Name()) || check.GetSymbolAtLocation(binary.Left) != check.GetSymbolAtLocation(decls[1].Parameters.Nodes[0].Name()) || check.GetSymbolAtLocation(rhs.Arguments.Nodes[0]) != check.GetSymbolAtLocation(decls[1].Parameters.Nodes[1].Name()) {
			t.Fatal("official calls/references lost original callee/caller bindings")
		}
		for _, argument := range entryCall.Arguments.Nodes {
			if argument.Kind != ast.KindIdentifier || check.GetSymbolAtLocation(argument) != check.GetSymbolAtLocation(decls[2].Parameters.Nodes[0].Name()) {
				t.Fatal("official entry arguments do not retain the one original scalar binding")
			}
		}
		leftType, rightType, resultType := check.GetTypeAtLocation(binary.Left), check.GetTypeAtLocation(binary.Right), check.GetTypeAtLocation(returns[1])
		if leftType == nil || rightType == nil || !leftType.IsUnion() || resultType != rightType || rightType != check.GetTypeFromTypeNode(decls[0].Type) || rightType.Flags()&(checker.TypeFlagsNull|checker.TypeFlagsUndefined) != 0 {
			t.Fatal("official coalesce must share the actual scalar i32 RHS/result type")
		}
		payload, null := 0, 0
		for _, member := range leftType.Types() {
			if member == rightType {
				payload++
			} else if member.Flags()&checker.TypeFlagsNull != 0 {
				null++
			} else {
				t.Fatal("official lhs has a foreign or undefined member")
			}
		}
		if payload != 1 || null != 1 || len(leftType.Types()) != 2 {
			t.Fatal("official lhs is not exactly the actual i32 payload plus null")
		}
		exports := check.GetExportsOfModule(file.AsNode().Symbol())
		if len(exports) != 1 || exports[0].Name != "entry" || len(exports[0].Declarations) != 1 || exports[0].Declarations[0] != file.Statements.Nodes[2] || ast.GetSourceFileOfNode(exports[0].Declarations[0]) != file {
			t.Fatal("complete official export set is not the actual root scalar entry")
		}
		remember("whole", returns[1], true)
		remember("left", binary.Left, true)
		remember("operator", binary.OperatorToken, true)
		remember("rhsToken", binary.Right, true)
		remember("rhs", binary.Right, false)
		remember("replacement", rhs.Arguments.Nodes[0], false)
		remember("entryCall", returns[2], false)
		remember("entryArgument", entryCall.Arguments.Nodes[0], false)
		remember("entryArgument2", entryCall.Arguments.Nodes[1], false)
		for key, want := range map[string]span{
			"fallback": {0, 52}, "coalesce": {52, 155}, "entry": {155, 229},
			"whole": {122, 152}, "left": {122, 127}, "operator": {128, 130}, "rhsToken": {131, 152}, "rhs": {130, 152}, "replacement": {140, 151},
		} {
			if anchors[key] != want {
				t.Fatalf("official fixed %s range differs: got=%+v want=%+v", key, anchors[key], want)
			}
		}
		t.Logf("TSFG_NULLABLE_FACTS_OFFICIAL_ORACLE_V2 %s", base64.StdEncoding.EncodeToString(canonical(map[string]any{
			"kind": "test-only-official-coalesce-oracle", "sourcePath": r.RootFile, "anchors": anchors,
			"exports": []map[string]any{{"name": "entry", "range": anchors["entry"], "definition": anchors["entryDefinition"]}},
		})))
	}) {
		return
	}

	var legacy response
	var legacyRaw []byte
	if !t.Run("legacyFacts", func(t *testing.T) {
		legacy, legacyRaw = nullableBoundaryCommandForTest(t, r)
		if legacy.Status != "ok" || len(legacy.Diagnostics) != 0 || len(legacy.Functions) != 3 || len(legacy.Symbols) != 7 {
			t.Fatalf("legacy public coalesce failed before private boundary: %+v; raw=%s", legacy, legacyRaw)
		}
		definitions := map[string]symbol{}
		for _, s := range legacy.Symbols {
			if s.ID == "" || definitions[s.ID].ID != "" {
				t.Fatal("owned identities are missing or duplicated")
			}
			definitions[s.ID] = s
		}
		for index, name := range []string{"fallback", "coalesce", "entry"} {
			fn := legacy.Functions[index]
			definition := definitions[fn.Symbol]
			if fn.Name != name || fn.Range != anchors[name] || fn.ReturnType != "i32" || fn.ReturnPresence != nil || len(fn.Body) != 1 || fn.Body[0].Kind != "return" || fn.Body[0].Range != anchors[name+"Return"] || definition.Kind != "function" || definition.Name != name || definition.Range != anchors[name+"Definition"] || definition.Writable {
				t.Fatalf("owned function differs from original official declaration: %+v", fn)
			}
			for position, p := range fn.Parameters {
				label := name + "Parameter"
				if position == 1 {
					label += "2"
				}
				if !reflect.DeepEqual(p, definitions[p.ID]) || p.Type != "i32" || p.Kind != "parameter" || !p.Writable || p.Range != anchors[label] || p.Presence == nil || p.Presence.Undefined || p.Presence.Optional || p.Presence.Default || p.Presence.Null != (name == "coalesce" && position == 0) {
					t.Fatalf("owned parameter differs from original defining symbol/presence: %+v", p)
				}
			}
		}
		fallback, coalesce, entry := legacy.Functions[0], legacy.Functions[1], legacy.Functions[2]
		if len(fallback.Parameters) != 1 || len(coalesce.Parameters) != 2 || len(entry.Parameters) != 1 || fallback.Body[0].Expression == nil || fallback.Body[0].Expression.Kind != "reference" || fallback.Body[0].Expression.Symbol != fallback.Parameters[0].ID {
			t.Fatal("legacy original function parameter/reference closure changed")
		}
		value := coalesce.Body[0].Expression
		if value == nil || value.Kind != "shortCircuit" || value.Operator != "??" || value.Type != "i32" || value.Range != anchors["whole"] || value.Left == nil || value.Right == nil || value.PresenceFlow == nil || value.PresenceFlow.Undefined || value.PresenceFlow.Null {
			t.Fatalf("legacy lost one scalar lazy ?? node: %+v", value)
		}
		left, right := value.Left, value.Right
		if left.Kind != "reference" || left.Type != "i32" || left.Symbol != coalesce.Parameters[0].ID || left.Range != anchors["left"] || left.PresenceFlow == nil || left.PresenceFlow.Undefined || !left.PresenceFlow.Null || right.Kind != "call" || right.Type != "i32" || right.Target != fallback.Symbol || right.Range != anchors["rhs"] || right.PresenceFlow != nil || right.Arguments == nil || len(*right.Arguments) != 1 || right.Signature == nil || right.Signature.ReturnType != "i32" || !reflect.DeepEqual(right.Signature.Parameters, []string{"i32"}) {
			t.Fatalf("legacy operands lost original bindings/ranges/scalar signature: %+v", value)
		}
		argument := (*right.Arguments)[0]
		call := entry.Body[0].Expression
		if argument.Kind != "reference" || argument.Type != "i32" || argument.Symbol != coalesce.Parameters[1].ID || argument.Range != anchors["replacement"] || argument.PresenceFlow != nil || call == nil || call.Kind != "call" || call.Target != coalesce.Symbol || call.Type != "i32" || call.Range != anchors["entryCall"] || call.Arguments == nil || len(*call.Arguments) != 2 || call.Signature == nil || !reflect.DeepEqual(call.Signature.Parameters, []string{"i32", "i32"}) || call.Signature.ReturnType != "i32" || call.Signature.ReturnPresence != nil || len(call.Signature.Presence) != 2 || !reflect.DeepEqual(call.Signature.Presence[0], coalesce.Parameters[0].Presence) || !reflect.DeepEqual(call.Signature.Presence[1], coalesce.Parameters[1].Presence) {
			t.Fatal("legacy RHS/entry call lost actual signature or caller argument binding")
		}
		for index, a := range *call.Arguments {
			label := "entryArgument"
			if index == 1 {
				label += "2"
			}
			if a.Kind != "reference" || a.Type != "i32" || a.Symbol != entry.Parameters[0].ID || a.Range != anchors[label] || a.PresenceFlow != nil {
				t.Fatal("entry arguments were rebound or lost original order/ranges")
			}
		}
		var tree map[string]json.Value
		if err := json.Unmarshal(canonical(value), &tree); err != nil || len(tree) != 7 || string(tree["operator"]) != "\"??\"" {
			t.Fatal("legacy ?? was expanded, duplicated or rewritten as a write")
		}
		t.Logf("TSFG_NULLABLE_FACTS_LEGACY_RESPONSE_V2 %s", base64.StdEncoding.EncodeToString(legacyRaw))
	}) {
		return
	}

	t.Run("privateCommand", func(t *testing.T) {
		requestBytes := canonical(map[string]any{"family": "tsfg.nullable-facts-request-candidate", "version": 2, "schemaHash": schema, "input": json.Value(originalInput)})
		before := bytes.Clone(requestBytes)
		defer func() {
			if !bytes.Equal(before, requestBytes) {
				t.Fatal("private v2 command mutated original request bytes")
			}
		}()
		t.Logf("TSFG_NULLABLE_FACTS_REQUEST_V2 %s", base64.StdEncoding.EncodeToString(requestBytes))
		t.Logf("TSFG_NULLABLE_FACTS_ORIGINAL_V2 %s", base64.StdEncoding.EncodeToString(canonical(map[string]any{"rootFile": r.RootFile, "files": r.Files})))
		t.Logf("TSFG_NULLABLE_FACTS_MAIN_V2 %s", base64.StdEncoding.EncodeToString([]byte(source)))
		t.Logf("TSFG_NULLABLE_FACTS_DTS_V2 %s", base64.StdEncoding.EncodeToString([]byte(r.Files[1].Content)))
		var output bytes.Buffer
		if err := runFrontendCommand(t.Context(), []string{"--nullable-facts-io-v2"}, bytes.NewReader(requestBytes), &output); err != nil {
			if output.Len() != 0 {
				t.Fatalf("failed private command published partial output: err=%v raw=%s", err, output.Bytes())
			}
			t.Fatalf("official/legacy passed; private v2 command refused: %v", err)
		}
		raw := bytes.Clone(output.Bytes())
		t.Logf("TSFG_NULLABLE_FACTS_RESPONSE_V2 %s", base64.StdEncoding.EncodeToString(raw))
		fields := func(data []byte, keys ...string) map[string]json.Value {
			t.Helper()
			var value map[string]json.Value
			if err := json.Unmarshal(data, &value); err != nil || len(value) != len(keys) || !bytes.Equal(canonical(value), data) {
				t.Fatalf("private value is not exact canonical JSON: %v raw=%s", err, data)
			}
			for _, key := range keys {
				if _, ok := value[key]; !ok {
					t.Fatalf("private object omitted %s", key)
				}
			}
			return value
		}
		packet := fields(raw, "family", "version", "schemaHash", "requestId", "packageId", "payload")
		requestID := digest(append([]byte("tsfg.nullable-facts.request.v2\x00"), requestBytes...))
		if string(packet["family"]) != "\"tsfg.nullable-facts-package-candidate\"" || string(packet["version"]) != "2" || !bytes.Equal(packet["schemaHash"], canonical(schema)) || !bytes.Equal(packet["requestId"], canonical(requestID)) {
			t.Fatal("private family/version/schema/request identity differs")
		}
		payload := fields(packet["payload"], "inputIdentity", "files", "facts", "exports")
		if !bytes.Equal(packet["packageId"], canonical(digest(append([]byte("tsfg.nullable-facts.package.v2\x00"+schema+"\x00"), packet["payload"]...)))) {
			t.Fatal("package ID failed independent complete payload preimage")
		}
		var identity map[string]json.Value
		if err := json.Unmarshal(originalInput, &identity); err != nil {
			t.Fatal(err)
		}
		files := slices.Clone(r.Files)
		slices.SortFunc(files, func(a, b inputFile) int { return strings.Compare(a.Path, b.Path) })
		identityFiles, fileTable := []map[string]any{}, []map[string]any{}
		for _, file := range files {
			identityFiles = append(identityFiles, map[string]any{"path": file.Path, "content": file.Content})
			fileTable = append(fileTable, map[string]any{"path": file.Path, "digest": digest([]byte(file.Content)), "byteLength": len(file.Content), "length": len(utf16.Encode([]rune(file.Content))), "positionUnit": "utf16"})
		}
		identity["files"] = json.Value(canonical(identityFiles))
		legacyCanonical := json.Value(bytes.Clone(legacyRaw))
		if err := legacyCanonical.Canonicalize(); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(payload["inputIdentity"], canonical(digest(canonical(identity)))) || !bytes.Equal(payload["files"], canonical(fileTable)) || !bytes.Equal(payload["facts"], legacyCanonical) {
			t.Fatal("private package changed complete input/file/legacy facts identities")
		}
		wantExports := []map[string]any{{"name": "entry", "target": legacy.Functions[2].Symbol, "sourcePath": r.RootFile, "range": anchors["entry"], "definition": anchors["entryDefinition"]}}
		if !bytes.Equal(payload["exports"], canonical(wantExports)) {
			t.Fatal("private exports differ from full official actual-declaration oracle")
		}
		t.Logf("TSFG_NULLABLE_FACTS_CAPTURE_HASHES_V2 %s", base64.StdEncoding.EncodeToString(canonical(map[string]any{"kind": "test-only-capture-hashes", "requestSha256": digest(requestBytes), "responseSha256": digest(raw)})))
	})
}
