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

// One frozen public tracer separates argv/schema refusal from typed grammar RED.
// The mask2 source is unchanged from the actual v2 reference-RHS capture. None
// of these owned facts is Source executable or runtime laziness evidence.
func TestNullableFactsCommandAbsenceMasksV3OwnedFacts(t *testing.T) {
	const schema = "sha256:24a04b0b595cae59ccf52a939f4671cea0369c46bd705f56726d028ab1df5a71"
	const entry = "\nexport function entry(value: i32): i32 { return coalesce(value, value); }"
	cases := []struct {
		name, source, hash string
		mask, whole, entry int
	}{
		{"mask2", "function coalesce(value: i32 | null, replacement: i32): i32 { return value ?? replacement; }" + entry, "0c517833dcec4fe92acd123b9969459008dedc3be959ebc0a6bb9feba8e472e3", 2, 69, 92},
		{"mask1", "function coalesce(value: i32 | undefined, replacement: i32): i32 { return value ?? replacement; }" + entry, "d0c9f55832571e9916b8acdcca4023caedaa9126014115d1bbc2ab9582335363", 1, 74, 97},
		{"mask3", "function coalesce(value: i32 | null | undefined, replacement: i32): i32 { return value ?? replacement; }" + entry, "9fb373bcd1ce3afddd204ae5c7012610956c9f03827e6bf7b9ae8e550f75ee57", 3, 81, 104},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			canonical := func(value any) []byte { return nullableFactsBoundaryCanonical(t, value) }
			digest := func(value []byte) string {
				sum := sha256.Sum256(value)
				return "sha256:" + hex.EncodeToString(sum[:])
			}
			if digest([]byte(tc.source)) != "sha256:"+tc.hash || len(tc.source) != tc.entry+74 || len(utf16.Encode([]rune(tc.source))) != len(tc.source) || strings.HasSuffix(tc.source, "\n") {
				t.Fatal("frozen ASCII/no-final-LF source changed")
			}
			r := integerRequest(tc.source)
			original := canonical(r)
			defer func() {
				if !bytes.Equal(original, canonical(r)) {
					t.Fatal("official/legacy/private acquisition mutated original input")
				}
			}()
			if !t.Run("officialDiagnostics", func(t *testing.T) { nullableBoundaryOfficialForTest(t, r, 0) }) {
				return
			}
			anchors := map[string]span{}
			if !t.Run("officialOracle", func(t *testing.T) {
				// This fresh official checker never calls finitePresence, scalar,
				// producer projection, private hashing or private shape validators.
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
					t.Fatal("fresh Program lost the two original root functions")
				}
				check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
				defer release()
				positions := file.GetPositionMap()
				remember := func(label string, node *ast.Node, token bool) {
					t.Helper()
					if node == nil {
						t.Fatalf("missing official node %s", label)
					}
					start := node.Pos()
					if token {
						for start < node.End() && (tc.source[start] == ' ' || tc.source[start] == '\n') {
							start++
						}
					}
					anchors[label] = span{positions.UTF8ToUTF16(start), positions.UTF8ToUTF16(node.End())}
				}
				decls, returns := []*ast.FunctionDeclaration{}, []*ast.Node{}
				bindings := map[*ast.Symbol]bool{}
				for index, name := range []string{"coalesce", "entry"} {
					node := file.Statements.Nodes[index]
					if node.Kind != ast.KindFunctionDeclaration || node.Name() == nil || node.Name().Text() != name {
						t.Fatal("official declaration order/name changed")
					}
					decl := node.AsFunctionDeclaration()
					if decl.Type == nil || decl.TypeParameters != nil || decl.Body == nil || decl.Body.Kind != ast.KindBlock || len(decl.Body.AsBlock().Statements.Nodes) != 1 || decl.Body.AsBlock().Statements.Nodes[0].Kind != ast.KindReturnStatement {
						t.Fatal("official body must be one original typed return")
					}
					remember(name, node, false)
					remember(name+"Definition", node.Name(), false)
					remember(name+"Return", decl.Body.AsBlock().Statements.Nodes[0], false)
					binding := check.GetSymbolAtLocation(node.Name())
					if binding == nil || bindings[binding] {
						t.Fatal("official function symbol is absent or merged")
					}
					bindings[binding] = true
					for position, p := range decl.Parameters.Nodes {
						parameter := p.AsParameterDeclaration()
						if parameter.Type == nil || parameter.QuestionToken != nil || parameter.Initializer != nil || parameter.DotDotDotToken != nil {
							t.Fatal("official parameter ceased to be required and explicitly typed")
						}
						label := name + "Parameter"
						if position == 1 {
							label += "2"
						}
						remember(label, p.Name(), false)
						remember(label+"Declaration", p, false)
						remember(label+"Annotation", parameter.Type, true)
						binding := check.GetSymbolAtLocation(p.Name())
						if binding == nil || bindings[binding] {
							t.Fatal("official defining parameters share an identity")
						}
						bindings[binding] = true
					}
					decls = append(decls, decl)
					returns = append(returns, decl.Body.AsBlock().Statements.Nodes[0].AsReturnStatement().Expression)
				}
				if len(bindings) != 5 || len(decls[0].Parameters.Nodes) != 2 || len(decls[1].Parameters.Nodes) != 1 || returns[0].Kind != ast.KindBinaryExpression || returns[1].Kind != ast.KindCallExpression {
					t.Fatal("official function/parameter/return structure changed")
				}
				binary, call := returns[0].AsBinaryExpression(), returns[1].AsCallExpression()
				if binary.OperatorToken.Kind != ast.KindQuestionQuestionToken || binary.Left.Kind != ast.KindIdentifier || binary.Right.Kind != ast.KindIdentifier || len(call.Arguments.Nodes) != 2 || call.Expression.Kind != ast.KindIdentifier {
					t.Fatal("official ?? must retain two references and one ordinary entry call")
				}
				if check.GetSymbolAtLocation(binary.Left) != check.GetSymbolAtLocation(decls[0].Parameters.Nodes[0].Name()) || check.GetSymbolAtLocation(binary.Right) != check.GetSymbolAtLocation(decls[0].Parameters.Nodes[1].Name()) || check.GetSymbolAtLocation(call.Expression) != check.GetSymbolAtLocation(decls[0].Name()) {
					t.Fatal("official operands/callee lost actual binding identity")
				}
				resolved := check.GetResolvedSignature(returns[1])
				if resolved == nil || resolved.Declaration() != file.Statements.Nodes[0] {
					t.Fatal("official entry call did not resolve the original coalesce declaration")
				}
				leftType, rightType, resultType := check.GetTypeAtLocation(binary.Left), check.GetTypeAtLocation(binary.Right), check.GetTypeAtLocation(returns[0])
				payload := check.GetTypeFromTypeNode(decls[0].Parameters.Nodes[1].AsParameterDeclaration().Type)
				if leftType == nil || !leftType.IsUnion() || rightType != payload || resultType != payload || check.GetTypeFromTypeNode(decls[0].Type) != payload || check.GetTypeFromTypeNode(decls[1].Type) != payload || check.GetReturnTypeOfSignature(resolved) != payload || payload.Flags()&(checker.TypeFlagsNull|checker.TypeFlagsUndefined) != 0 {
					t.Fatal("three official use-site types and resolved scalar return disagree")
				}
				checkMembers := func(value *checker.Type) {
					t.Helper()
					if value == nil || !value.IsUnion() {
						t.Fatal("official lhs annotation/use-site is not a union")
					}
					payloadCount, nullCount, undefinedCount := 0, 0, 0
					for _, member := range value.Types() {
						switch {
						case member == payload:
							payloadCount++
						case member.Flags()&checker.TypeFlagsNull != 0:
							nullCount++
						case member.Flags()&checker.TypeFlagsUndefined != 0:
							undefinedCount++
						default:
							t.Fatal("official lhs contains a foreign payload/member")
						}
					}
					wantNull, wantUndefined := 0, 0
					if tc.mask&2 != 0 {
						wantNull = 1
					}
					if tc.mask&1 != 0 {
						wantUndefined = 1
					}
					if payloadCount != 1 || nullCount != wantNull || undefinedCount != wantUndefined || len(value.Types()) != 1+wantNull+wantUndefined {
						t.Fatal("official union did not retain the exact selected mask and same payload object")
					}
				}
				checkMembers(leftType)
				checkMembers(check.GetTypeFromTypeNode(decls[0].Parameters.Nodes[0].AsParameterDeclaration().Type))
				for _, argument := range call.Arguments.Nodes {
					if argument.Kind != ast.KindIdentifier || check.GetSymbolAtLocation(argument) != check.GetSymbolAtLocation(decls[1].Parameters.Nodes[0].Name()) || check.GetTypeAtLocation(argument) != payload {
						t.Fatal("official entry call lost its original scalar argument binding/type")
					}
				}
				exports := check.GetExportsOfModule(file.AsNode().Symbol())
				if len(exports) != 1 || exports[0].Name != "entry" || exports[0].Flags&ast.SymbolFlagsAlias != 0 || len(exports[0].Declarations) != 1 || exports[0].Declarations[0] != file.Statements.Nodes[1] || ast.GetSourceFileOfNode(exports[0].Declarations[0]) != file {
					t.Fatal("complete official exports differ from the one actual root entry")
				}
				remember("whole", returns[0], true)
				remember("operator", binary.OperatorToken, true)
				remember("left", binary.Left, true)
				remember("right", binary.Right, true)
				remember("entryCall", returns[1], false)
				remember("entryArgument", call.Arguments.Nodes[0], false)
				remember("entryArgument2", call.Arguments.Nodes[1], false)
				for name, want := range map[string]span{
					"coalesce": {0, tc.entry}, "entry": {tc.entry, len(tc.source)},
					"whole": {tc.whole, tc.whole + 20}, "left": {tc.whole, tc.whole + 5},
					"operator": {tc.whole + 6, tc.whole + 8}, "right": {tc.whole + 9, tc.whole + 20},
				} {
					if anchors[name] != want {
						t.Fatalf("official original UTF16 %s changed: got=%+v want=%+v", name, anchors[name], want)
					}
				}
				t.Logf("TSFG_ABSENCE_MASKS_OFFICIAL_ORACLE_V3 %s %s", tc.name, base64.StdEncoding.EncodeToString(canonical(map[string]any{"mask": tc.mask, "anchors": anchors, "sourceSha256": digest([]byte(tc.source))})))
			}) {
				return
			}
			var legacy response
			var legacyRaw []byte
			if !t.Run("legacyFacts", func(t *testing.T) {
				legacy, legacyRaw = nullableBoundaryCommandForTest(t, r)
				if legacy.Status != "ok" || len(legacy.Diagnostics) != 0 || len(legacy.Functions) != 2 || len(legacy.Symbols) != 5 {
					t.Fatalf("legacy failed before v3 boundary: %+v raw=%s", legacy, legacyRaw)
				}
				definitions := map[string]symbol{}
				for index, definition := range legacy.Symbols {
					wantID := []string{"s1", "s2", "s3", "s4", "s5"}[index]
					if definition.ID != wantID || definitions[definition.ID].ID != "" {
						t.Fatal("complete owned symbols lost original unique order/IDs")
					}
					definitions[definition.ID] = definition
				}
				for index, name := range []string{"coalesce", "entry"} {
					fn := legacy.Functions[index]
					def := definitions[fn.Symbol]
					wantCount := 2 - index
					if fn.Name != name || fn.Symbol != []string{"s1", "s2"}[index] || fn.Range != anchors[name] || fn.ReturnType != "i32" || fn.ReturnPresence != nil || def.Kind != "function" || def.Name != name || def.Range != anchors[name+"Definition"] || def.Writable || def.Type != "i32" || def.Presence != nil || def.ReturnPresence != nil || len(fn.Parameters) != wantCount || len(fn.Body) != 1 || fn.Body[0].Kind != "return" || fn.Body[0].Range != anchors[name+"Return"] || len(fn.ParameterInitializers) != wantCount {
						t.Fatalf("owned function differs from actual official scalar declaration: %+v", fn)
					}
					for position, p := range fn.Parameters {
						label := name + "Parameter"
						if position == 1 {
							label += "2"
						}
						shape := p.Presence
						nullable := name == "coalesce" && position == 0
						if !reflect.DeepEqual(p, definitions[p.ID]) || p.Type != "i32" || p.Kind != "parameter" || !p.Writable || p.Range != anchors[label] || shape == nil || shape.Null != (nullable && tc.mask&2 != 0) || shape.Undefined != (nullable && tc.mask&1 != 0) || shape.Optional || shape.Default || shape.Range != anchors[label+"Declaration"] || shape.Annotation != anchors[label+"Annotation"] || fn.ParameterInitializers[position] != nil {
							t.Fatalf("owned defining parameter lost exact declared mask/anchors: %+v", p)
						}
					}
				}
				fn, exported := legacy.Functions[0], legacy.Functions[1]
				value := fn.Body[0].Expression
				if value == nil || value.Kind != "shortCircuit" || value.Operator != "??" || value.Type != "i32" || value.Range != anchors["whole"] || value.PresenceFlow == nil || value.PresenceFlow.Null || value.PresenceFlow.Undefined || value.Left == nil || value.Right == nil {
					t.Fatal("owned scalar lazy ?? result changed")
				}
				for index, operand := range []*expression{value.Left, value.Right} {
					label := []string{"left", "right"}[index]
					if operand.Kind != "reference" || operand.Type != "i32" || operand.Symbol != fn.Parameters[index].ID || operand.Range != anchors[label] || operand.Left != nil || operand.Right != nil || operand.Operand != nil || operand.Arguments != nil || operand.Target != "" || operand.Operator != "" || operand.Callee != nil || operand.Receiver != nil {
						t.Fatal("owned operand duplicated, rebound, written or expanded")
					}
				}
				if value.Left.PresenceFlow == nil || value.Left.PresenceFlow.Null != (tc.mask&2 != 0) || value.Left.PresenceFlow.Undefined != (tc.mask&1 != 0) || value.Right.PresenceFlow != nil {
					t.Fatal("owned use-site flow lost its exact selected absence mask")
				}
				call := exported.Body[0].Expression
				if call == nil || call.Kind != "call" || call.Target != fn.Symbol || call.Type != "i32" || call.Range != anchors["entryCall"] || call.Arguments == nil || len(*call.Arguments) != 2 || call.Signature == nil || !reflect.DeepEqual(call.Signature.Parameters, []string{"i32", "i32"}) || call.Signature.ReturnType != "i32" || call.Signature.ReturnPresence != nil || len(call.Signature.Presence) != 2 || !reflect.DeepEqual(call.Signature.Presence[0], fn.Parameters[0].Presence) || !reflect.DeepEqual(call.Signature.Presence[1], fn.Parameters[1].Presence) {
					t.Fatal("owned caller signature differs from actual declaration-scoped complete presence")
				}
				for index, argument := range *call.Arguments {
					label := []string{"entryArgument", "entryArgument2"}[index]
					if argument.Kind != "reference" || argument.Symbol != exported.Parameters[0].ID || argument.Type != "i32" || argument.PresenceFlow != nil || argument.Range != anchors[label] {
						t.Fatal("owned call arguments lost scalar caller binding/order/ranges")
					}
				}
				var node map[string]json.Value
				if err := json.Unmarshal(canonical(value), &node); err != nil || len(node) != 7 {
					t.Fatal("owned ?? tree gained synthetic/eager/write fields")
				}
				t.Logf("TSFG_ABSENCE_MASKS_LEGACY_RESPONSE_V3 %s %s", tc.name, base64.StdEncoding.EncodeToString(legacyRaw))
			}) {
				return
			}
			t.Run("privateCommand", func(t *testing.T) {
				input := canonical(map[string]any{"family": "tsfg.nullable-facts-request-candidate", "version": 3, "schemaHash": schema, "input": json.Value(original)})
				before := bytes.Clone(input)
				defer func() {
					if !bytes.Equal(before, input) {
						t.Fatal("private command mutated caller request bytes")
					}
				}()
				t.Logf("TSFG_ABSENCE_MASKS_REQUEST_V3 %s %s", tc.name, base64.StdEncoding.EncodeToString(input))
				t.Logf("TSFG_ABSENCE_MASKS_ORIGINAL_V3 %s %s", tc.name, base64.StdEncoding.EncodeToString(canonical(map[string]any{"rootFile": r.RootFile, "files": r.Files})))
				var output bytes.Buffer
				if err := runFrontendCommand(t.Context(), []string{"--nullable-facts-io-v3"}, bytes.NewReader(input), &output); err != nil {
					if output.Len() != 0 {
						t.Fatalf("failed v3 command published output: err=%v raw=%s", err, output.Bytes())
					}
					t.Fatalf("official/legacy passed; private v3 refused: %v", err)
				}
				raw := bytes.Clone(output.Bytes())
				fields := func(data []byte, keys ...string) map[string]json.Value {
					t.Helper()
					var value map[string]json.Value
					if err := json.Unmarshal(data, &value); err != nil || len(value) != len(keys) || !bytes.Equal(canonical(value), data) {
						t.Fatalf("not exact JCS object: %v raw=%s", err, data)
					}
					for _, key := range keys {
						if _, ok := value[key]; !ok {
							t.Fatalf("missing object key %s", key)
						}
					}
					return value
				}
				packet := fields(raw, "family", "version", "schemaHash", "requestId", "packageId", "payload")
				if string(packet["family"]) != "\"tsfg.nullable-facts-package-candidate\"" || string(packet["version"]) != "3" || !bytes.Equal(packet["schemaHash"], canonical(schema)) || !bytes.Equal(packet["requestId"], canonical(digest(append([]byte("tsfg.nullable-facts.request.v3\x00"), input...)))) {
					t.Fatal("v3 family/version/schema/request ID differs")
				}
				payload := fields(packet["payload"], "inputIdentity", "files", "facts", "exports")
				if !bytes.Equal(packet["packageId"], canonical(digest(append([]byte("tsfg.nullable-facts.package.v3\x00"+schema+"\x00"), packet["payload"]...)))) {
					t.Fatal("package ID does not bind complete v3 payload")
				}
				var identity map[string]json.Value
				if err := json.Unmarshal(original, &identity); err != nil {
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
					t.Fatal("package filtered/changed complete input/files/legacy facts")
				}
				wantExports := []map[string]any{{"name": "entry", "target": legacy.Functions[1].Symbol, "sourcePath": r.RootFile, "range": anchors["entry"], "definition": anchors["entryDefinition"]}}
				if !bytes.Equal(payload["exports"], canonical(wantExports)) {
					t.Fatal("package exports differ from complete actual official declaration oracle")
				}
				t.Logf("TSFG_ABSENCE_MASKS_RESPONSE_V3 %s %s", tc.name, base64.StdEncoding.EncodeToString(raw))
				t.Logf("TSFG_ABSENCE_MASKS_CAPTURE_HASHES_V3 %s %s", tc.name, base64.StdEncoding.EncodeToString(canonical(map[string]any{"requestSha256": digest(input), "responseSha256": digest(raw)})))
			})
		})
	}
}
