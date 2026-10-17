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

const absenceMasksV3SchemaForTest = "sha256:24a04b0b595cae59ccf52a939f4671cea0369c46bd705f56726d028ab1df5a71"

// An independent test oracle follows the frozen first's complete preimages,
// but accepts a different actual source. Never calls producer identity helpers.
func absenceMasksV3PackageForTest(t *testing.T, r request, legacy response, input, raw, legacyRaw []byte, anchors map[string]span) {
	t.Helper()
	canonical := func(value any) []byte { return nullableFactsBoundaryCanonical(t, value) }
	digest := func(value []byte) string { sum := sha256.Sum256(value); return "sha256:" + hex.EncodeToString(sum[:]) }
	fields := func(data []byte, keys ...string) map[string]json.Value {
		t.Helper()
		var value map[string]json.Value
		if err := json.Unmarshal(data, &value); err != nil || len(value) != len(keys) || !bytes.Equal(canonical(value), data) {
			t.Fatalf("not exact JCS object: %v raw=%s", err, data)
		}
		for _, key := range keys {
			if _, ok := value[key]; !ok {
				t.Fatalf("missing key %s", key)
			}
		}
		return value
	}
	packet := fields(raw, "family", "version", "schemaHash", "requestId", "packageId", "payload")
	if string(packet["family"]) != "\"tsfg.nullable-facts-package-candidate\"" || string(packet["version"]) != "3" || !bytes.Equal(packet["schemaHash"], canonical(absenceMasksV3SchemaForTest)) || !bytes.Equal(packet["requestId"], canonical(digest(append([]byte("tsfg.nullable-facts.request.v3\x00"), input...)))) {
		t.Fatal("v3 package family/request identity differs")
	}
	payload := fields(packet["payload"], "inputIdentity", "files", "facts", "exports")
	if !bytes.Equal(packet["packageId"], canonical(digest(append([]byte("tsfg.nullable-facts.package.v3\x00"+absenceMasksV3SchemaForTest+"\x00"), packet["payload"]...)))) {
		t.Fatal("package ID does not bind full payload")
	}
	var identity map[string]json.Value
	if err := json.Unmarshal(canonical(r), &identity); err != nil {
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
	facts := json.Value(bytes.Clone(legacyRaw))
	if err := facts.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(payload["inputIdentity"], canonical(digest(canonical(identity)))) || !bytes.Equal(payload["files"], canonical(fileTable)) || !bytes.Equal(payload["facts"], facts) {
		t.Fatal("package filtered/changed complete inputs or legacy facts")
	}
	entry := legacy.Functions[len(legacy.Functions)-1]
	if entry.Name != "entry" || entry.ReturnType != "i32" || entry.ReturnPresence != nil || len(entry.Parameters) != 1 || entry.Parameters[0].Presence == nil || entry.Parameters[0].Presence.Null || entry.Parameters[0].Presence.Undefined || entry.Parameters[0].Presence.Optional || entry.Parameters[0].Presence.Default {
		t.Fatal("positive entry is not the original scalar ABI")
	}
	want := []map[string]any{{"name": "entry", "target": entry.Symbol, "sourcePath": r.RootFile, "range": anchors["entry"], "definition": anchors["entryDefinition"]}}
	if !bytes.Equal(payload["exports"], canonical(want)) {
		t.Fatal("exports differ from complete official actual declaration")
	}
}

// Same Program/checker holds every AST, symbol, annotation and use-site fact.
// Expected masks come from the source case, never from the producer response.
func absenceMasksV3OfficialForTest(t *testing.T, r request, mask int) map[string]span {
	t.Helper()
	source := r.Files[0].Content
	contents := map[string]string{}
	for _, file := range r.Files {
		contents[file.Path] = file.Content
	}
	program := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
			FileNames:       append([]string{r.RootFile}, r.Declarations.Paths...),
			CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext},
		}},
		Host:           compiler.NewCompilerHost(r.SourceRoot, vfstest.FromMap(contents, true), r.SourceRoot, nil, nil),
		SingleThreaded: core.TSTrue,
	})
	file := program.GetSourceFile(r.RootFile)
	if file == nil || len(file.Statements.Nodes) != 3 {
		t.Fatal("fresh official Program lost three original functions")
	}
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	positions, anchors := file.GetPositionMap(), map[string]span{}
	remember := func(label string, node *ast.Node, token bool) {
		t.Helper()
		if node == nil {
			t.Fatalf("missing original node %s", label)
		}
		start := node.Pos()
		if token {
			for start < node.End() && (source[start] == ' ' || source[start] == '\n') {
				start++
			}
		}
		anchors[label] = span{positions.UTF8ToUTF16(start), positions.UTF8ToUTF16(node.End())}
	}
	decls := []*ast.FunctionDeclaration{}
	unique := map[*ast.Symbol]bool{}
	define := func(node *ast.Node) {
		t.Helper()
		s := check.GetSymbolAtLocation(node)
		if s == nil || unique[s] {
			t.Fatal("official defining symbols merged")
		}
		unique[s] = true
	}
	for index, name := range []string{"echo", "coalesce", "entry"} {
		node := file.Statements.Nodes[index]
		if node.Kind != ast.KindFunctionDeclaration || node.Name() == nil || node.Name().Text() != name {
			t.Fatal("official function order/name changed")
		}
		decl := node.AsFunctionDeclaration()
		if decl.Type == nil || decl.Body == nil || decl.Body.Kind != ast.KindBlock {
			t.Fatal("official function lost typed body")
		}
		define(node.Name())
		remember(name, node, false)
		remember(name+"Definition", node.Name(), false)
		remember(name+"ResultAnnotation", decl.Type, true)
		for position, p := range decl.Parameters.Nodes {
			label := name + "Parameter"
			if position == 1 {
				label += "2"
			}
			define(p.Name())
			remember(label, p.Name(), false)
			remember(label+"Declaration", p, false)
			remember(label+"Annotation", p.AsParameterDeclaration().Type, true)
		}
		decls = append(decls, decl)
	}
	if len(decls[0].Parameters.Nodes) != 1 || len(decls[1].Parameters.Nodes) != 2 || len(decls[2].Parameters.Nodes) != 1 {
		t.Fatal("official required arities changed")
	}
	payload := check.GetTypeFromTypeNode(decls[1].Parameters.Nodes[1].AsParameterDeclaration().Type)
	members := func(typ *checker.Type, want int) {
		t.Helper()
		if want == 0 {
			if typ != payload {
				t.Fatal("official scalar is not the same declared i32 payload")
			}
			return
		}
		if typ == nil || !typ.IsUnion() {
			t.Fatal("official nullable type is not a union")
		}
		p, n, u := 0, 0, 0
		for _, member := range typ.Types() {
			switch {
			case member == payload:
				p++
			case member.Flags()&checker.TypeFlagsNull != 0:
				n++
			case member.Flags()&checker.TypeFlagsUndefined != 0:
				u++
			default:
				t.Fatal("foreign official union member")
			}
		}
		wn, wu := 0, 0
		if want&2 != 0 {
			wn = 1
		}
		if want&1 != 0 {
			wu = 1
		}
		if p != 1 || n != wn || u != wu || len(typ.Types()) != 1+wn+wu {
			t.Fatal("official exact mask/payload changed")
		}
	}
	for index, decl := range decls {
		resultMask := 0
		if index == 0 {
			resultMask = mask
		}
		members(check.GetTypeFromTypeNode(decl.Type), resultMask)
		for position, p := range decl.Parameters.Nodes {
			want := 0
			if index < 2 && position == 0 {
				want = mask
			}
			members(check.GetTypeFromTypeNode(p.AsParameterDeclaration().Type), want)
		}
	}
	echoBody, body, entryBody := decls[0].Body.AsBlock().Statements.Nodes, decls[1].Body.AsBlock().Statements.Nodes, decls[2].Body.AsBlock().Statements.Nodes
	if len(echoBody) != 1 || len(body) != 3 || len(entryBody) != 1 || echoBody[0].Kind != ast.KindReturnStatement || body[0].Kind != ast.KindVariableStatement || body[1].Kind != ast.KindVariableStatement || body[2].Kind != ast.KindReturnStatement || entryBody[0].Kind != ast.KindReturnStatement {
		t.Fatal("official const/return grammar changed")
	}
	locals := []*ast.Node{}
	for index, name := range []string{"original", "result"} {
		list := body[index].AsVariableStatement().DeclarationList
		if list.Flags&ast.NodeFlagsConst == 0 || len(list.AsVariableDeclarationList().Declarations.Nodes) != 1 {
			t.Fatal("official immutable local missing")
		}
		node := list.AsVariableDeclarationList().Declarations.Nodes[0]
		local := node.AsVariableDeclaration()
		if node.Name() == nil || node.Name().Text() != name || local.Type == nil || local.Initializer == nil {
			t.Fatal("official local name/type/initializer changed")
		}
		define(node.Name())
		remember(name, node, false)
		remember(name+"Definition", node.Name(), false)
		remember(name+"Annotation", local.Type, true)
		members(check.GetTypeFromTypeNode(local.Type), mask)
		members(check.GetTypeAtLocation(local.Initializer), mask)
		locals = append(locals, node)
	}
	root := body[2].AsReturnStatement().Expression
	if root == nil || root.Kind != ast.KindBinaryExpression {
		t.Fatal("official coalesce lost binary node")
	}
	binary := root.AsBinaryExpression()
	if binary.OperatorToken.Kind != ast.KindQuestionQuestionToken || binary.Left.Kind != ast.KindCallExpression || binary.Right.Kind != ast.KindIdentifier {
		t.Fatal("official lazy call-LHS ?? shape changed")
	}
	members(check.GetTypeAtLocation(binary.Left), mask)
	members(check.GetTypeAtLocation(binary.Right), 0)
	members(check.GetTypeAtLocation(root), 0)
	remember("whole", root, true)
	remember("left", binary.Left, false)
	remember("right", binary.Right, true)
	remember("operator", binary.OperatorToken, true)
	ref := func(node, definition *ast.Node) {
		t.Helper()
		if node == nil || node.Kind != ast.KindIdentifier || check.GetSymbolAtLocation(node) != check.GetSymbolAtLocation(definition) {
			t.Fatal("official original reference rebound")
		}
	}
	ref(echoBody[0].AsReturnStatement().Expression, decls[0].Parameters.Nodes[0].Name())
	ref(locals[0].AsVariableDeclaration().Initializer, decls[1].Parameters.Nodes[0].Name())
	ref(binary.Right, decls[1].Parameters.Nodes[1].Name())
	for index, node := range []*ast.Node{locals[1].AsVariableDeclaration().Initializer, binary.Left} {
		if node.Kind != ast.KindCallExpression {
			t.Fatal("official nullable call lost original node")
		}
		call := node.AsCallExpression()
		resolved := check.GetResolvedSignature(node)
		if resolved == nil || resolved.Declaration() != file.Statements.Nodes[0] || len(call.Arguments.Nodes) != 1 {
			t.Fatal("official call lost actual echo declaration signature")
		}
		ref(call.Expression, decls[0].Name())
		ref(call.Arguments.Nodes[0], locals[index].Name())
		members(check.GetReturnTypeOfSignature(resolved), mask)
	}
	entryCallNode := entryBody[0].AsReturnStatement().Expression
	if entryCallNode == nil || entryCallNode.Kind != ast.KindCallExpression {
		t.Fatal("official entry call missing")
	}
	entryCall := entryCallNode.AsCallExpression()
	resolved := check.GetResolvedSignature(entryCallNode)
	if resolved == nil || resolved.Declaration() != file.Statements.Nodes[1] || len(entryCall.Arguments.Nodes) != 2 {
		t.Fatal("official entry signature differs")
	}
	ref(entryCall.Expression, decls[1].Name())
	for _, argument := range entryCall.Arguments.Nodes {
		ref(argument, decls[2].Parameters.Nodes[0].Name())
		members(check.GetTypeAtLocation(argument), 0)
	}
	exports := check.GetExportsOfModule(file.AsNode().Symbol())
	if len(unique) != 9 || len(exports) != 1 || exports[0].Name != "entry" || exports[0].Flags&ast.SymbolFlagsAlias != 0 || len(exports[0].Declarations) != 1 || exports[0].Declarations[0] != file.Statements.Nodes[2] || ast.GetSourceFileOfNode(exports[0].Declarations[0]) != file {
		t.Fatal("official unique symbols/complete direct export changed")
	}
	return anchors
}

func TestNullableFactsCommandAbsenceMasksV3Boundaries(t *testing.T) {
	canonical := func(value any) []byte { return nullableFactsBoundaryCanonical(t, value) }
	inputFor := func(r request) []byte {
		return canonical(map[string]any{"family": "tsfg.nullable-facts-request-candidate", "version": 3, "schemaHash": absenceMasksV3SchemaForTest, "input": r})
	}
	acquire := func(t *testing.T, source string) (request, response, []byte, []byte) {
		t.Helper()
		r := integerRequest(source)
		before := canonical(r)
		if !t.Run("officialDiagnostics", func(t *testing.T) { nullableBoundaryOfficialForTest(t, r, 0) }) {
			t.Fatal("official diagnostics failed before claimed layer")
		}
		legacy, raw := nullableBoundaryCommandForTest(t, r)
		if !bytes.Equal(before, canonical(r)) {
			t.Fatal("official/legacy changed request")
		}
		return r, legacy, raw, inputFor(r)
	}
	run := func(t *testing.T, input []byte) ([]byte, error) {
		t.Helper()
		before := bytes.Clone(input)
		out, err := nullableFactsBoundaryRun(t.Context(), []string{"--nullable-facts-io-v3"}, input)
		if !bytes.Equal(before, input) {
			t.Fatal("private command changed request bytes")
		}
		if err != nil && len(out) != 0 {
			t.Fatal("failed command published partial package")
		}
		return out, err
	}
	for _, tc := range []struct {
		name, annotation string
		mask             int
	}{{"mask2", "i32 | null", 2}, {"mask1", "i32 | undefined", 1}, {"mask3", "i32 | null | undefined", 3}} {
		if !t.Run(tc.name+"ReturnCallConst", func(t *testing.T) {
			source := "function echo(value: " + tc.annotation + "): " + tc.annotation + " { return value; }\nfunction coalesce(value: " + tc.annotation + ", replacement: i32): i32 { const original: " + tc.annotation + " = value; const result: " + tc.annotation + " = echo(original); return echo(result) ?? replacement; }\nexport function entry(value: i32): i32 { return coalesce(value, value); }"
			r := integerRequest(source)
			before := canonical(r)
			defer func() {
				if !bytes.Equal(before, canonical(r)) {
					t.Fatal("positive changed original request")
				}
			}()
			if !t.Run("officialDiagnostics", func(t *testing.T) { nullableBoundaryOfficialForTest(t, r, 0) }) {
				return
			}
			var anchors map[string]span
			if !t.Run("officialOracle", func(t *testing.T) { anchors = absenceMasksV3OfficialForTest(t, r, tc.mask) }) {
				return
			}
			legacy, raw := nullableBoundaryCommandForTest(t, r)
			if legacy.Status != "ok" || len(legacy.Diagnostics) != 0 || len(legacy.Functions) != 3 || len(legacy.Symbols) != 9 {
				t.Fatalf("positive legacy failed: %+v raw=%s", legacy, raw)
			}
			defs := map[string]symbol{}
			for _, s := range legacy.Symbols {
				if s.ID == "" || defs[s.ID].ID != "" {
					t.Fatal("duplicate/missing owned symbol")
				}
				defs[s.ID] = s
			}
			shape := func(s *parameterPresence, want int) {
				t.Helper()
				if s == nil || s.Null != (want&2 != 0) || s.Undefined != (want&1 != 0) || s.Optional || s.Default {
					t.Fatal("owned declaration exact mask changed")
				}
			}
			flow := func(e *expression, want int) {
				t.Helper()
				if e == nil || e.PresenceFlow == nil || e.PresenceFlow.Null != (want&2 != 0) || e.PresenceFlow.Undefined != (want&1 != 0) {
					t.Fatal("owned use-site exact flow changed")
				}
			}
			for index, name := range []string{"echo", "coalesce", "entry"} {
				fn := legacy.Functions[index]
				def := defs[fn.Symbol]
				if fn.Name != name || fn.Range != anchors[name] || fn.ReturnType != "i32" || def.Kind != "function" || def.Name != name || def.Range != anchors[name+"Definition"] || !reflect.DeepEqual(def.ReturnPresence, fn.ReturnPresence) {
					t.Fatal("owned declaration differs from official original")
				}
				if index == 0 {
					if fn.ReturnPresence == nil || fn.ReturnPresence.Null != (tc.mask&2 != 0) || fn.ReturnPresence.Undefined != (tc.mask&1 != 0) || fn.ReturnPresence.Range != anchors[name] || fn.ReturnPresence.Annotation != anchors[name+"ResultAnnotation"] {
						t.Fatal("nullable return mask/anchors changed")
					}
				} else if fn.ReturnPresence != nil {
					t.Fatal("scalar return became nullable")
				}
				for position, p := range fn.Parameters {
					label := name + "Parameter"
					if position == 1 {
						label += "2"
					}
					want := 0
					if index < 2 && position == 0 {
						want = tc.mask
					}
					shape(p.Presence, want)
					if !reflect.DeepEqual(p, defs[p.ID]) || p.Type != "i32" || p.Range != anchors[label] || p.Presence.Range != anchors[label+"Declaration"] || p.Presence.Annotation != anchors[label+"Annotation"] {
						t.Fatal("owned parameter differs from original symbol/annotation")
					}
				}
			}
			echo, fn := legacy.Functions[0], legacy.Functions[1]
			if len(echo.Body) != 1 || echo.Body[0].Expression == nil || echo.Body[0].Expression.Symbol != echo.Parameters[0].ID || len(fn.Body) != 3 {
				t.Fatal("owned nullable return/const body changed")
			}
			flow(echo.Body[0].Expression, tc.mask)
			for index, name := range []string{"original", "result"} {
				local := fn.Body[index]
				if local.Kind != "variable" || local.Symbol == nil || local.Symbol.Name != name || local.Symbol.Writable || local.Range != anchors[name] || local.Symbol.Range != anchors[name+"Definition"] || !reflect.DeepEqual(*local.Symbol, defs[local.Symbol.ID]) {
					t.Fatal("owned const identity/mutability changed")
				}
				shape(local.Symbol.Presence, tc.mask)
				if local.Symbol.Presence.Range != anchors[name] || local.Symbol.Presence.Annotation != anchors[name+"Annotation"] {
					t.Fatal("const declaration presence lost original range")
				}
				flow(local.Initializer, tc.mask)
			}
			if fn.Body[0].Initializer.Kind != "reference" || fn.Body[0].Initializer.Symbol != fn.Parameters[0].ID {
				t.Fatal("const initializer lost caller binding")
			}
			root := fn.Body[2].Expression
			if root == nil || root.Kind != "shortCircuit" || root.Operator != "??" || root.Type != "i32" || root.Range != anchors["whole"] || root.Right == nil || root.Right.Kind != "reference" || root.Right.Symbol != fn.Parameters[1].ID || root.Right.PresenceFlow != nil || root.Right.Range != anchors["right"] {
				t.Fatal("owned lazy scalar coalesce changed")
			}
			flow(root, 0)
			flow(root.Left, tc.mask)
			for index, call := range []*expression{fn.Body[1].Initializer, root.Left} {
				if call.Kind != "call" || call.Target != echo.Symbol || call.Arguments == nil || len(*call.Arguments) != 1 || (*call.Arguments)[0].Symbol != fn.Body[index].Symbol.ID || call.Signature == nil || !reflect.DeepEqual(call.Signature.ReturnPresence, echo.ReturnPresence) || len(call.Signature.Presence) != 1 || !reflect.DeepEqual(call.Signature.Presence[0], echo.Parameters[0].Presence) {
					t.Fatal("owned nullable call lost exact declaration-scoped signature/binding")
				}
			}
			input := inputFor(r)
			out, err := run(t, input)
			if err != nil {
				t.Fatalf("positive private failed after official+legacy PASS: %v", err)
			}
			absenceMasksV3PackageForTest(t, r, legacy, input, out, raw, anchors)
			t.Logf("TSFG_ABSENCE_MASKS_BOUNDARY_REQUEST_V3 %s %s", tc.name, base64.StdEncoding.EncodeToString(input))
			t.Logf("TSFG_ABSENCE_MASKS_BOUNDARY_RESPONSE_V3 %s %s", tc.name, base64.StdEncoding.EncodeToString(out))
			t.Logf("TSFG_ABSENCE_MASKS_BOUNDARY_LEGACY_V3 %s %s", tc.name, base64.StdEncoding.EncodeToString(raw))
		}) {
			return
		}
	}
	const scalarEntry = "\nexport function entry(value: i32): i32 { return value; }"
	negatives := []struct{ name, source, layer string }{
		{"mixedPayloadSource", "function bad(value: i32 | u32 | null): i32 | u32 | null { return value; }" + scalarEntry, "source"},
		{"absenceOnlySource", "function bad(value: null | undefined): null | undefined { return value; }" + scalarEntry, "source"},
		{"optionalGrammar", "function bad(replacement: i32, value?: i32): i32 { return value ?? replacement; }" + scalarEntry, "capability"},
		{"defaultGrammar", "function bad(replacement: i32, value: i32 = replacement): i32 { return value; }" + scalarEntry, "capability"},
		{"assignmentGrammar", "function bad(value: i32 | null | undefined, replacement: i32): i32 { return value ??= replacement; }" + scalarEntry, "capability"},
		{"nullableRhsResultSource", "function bad(value: i32 | null | undefined, replacement: i32 | null | undefined): i32 | null | undefined { return value ?? replacement; }" + scalarEntry, "source"},
		{"mask1To3Call", "function target(value: i32 | null | undefined): i32 | null | undefined { return value; }\nfunction bad(value: i32 | undefined): i32 | null | undefined { return target(value); }" + scalarEntry, "capability"},
		{"mask2To3Call", "function target(value: i32 | null | undefined): i32 | null | undefined { return value; }\nfunction bad(value: i32 | null): i32 | null | undefined { return target(value); }" + scalarEntry, "capability"},
		{"mask1To3Return", "function bad(value: i32 | undefined): i32 | null | undefined { return value; }" + scalarEntry, "capability"},
		{"mask2To3Return", "function bad(value: i32 | null): i32 | null | undefined { return value; }" + scalarEntry, "capability"},
	}
	for _, tc := range negatives {
		t.Run(tc.name, func(t *testing.T) {
			_, legacy, raw, input := acquire(t, tc.source)
			if tc.layer == "source" {
				if legacy.Status != "error" || len(legacy.Diagnostics) != 1 || len(legacy.Functions) != 0 {
					t.Fatalf("expected early legacy Source rejection: %+v raw=%s", legacy, raw)
				}
			} else if legacy.Status != "ok" || len(legacy.Diagnostics) != 0 || len(legacy.Functions) == 0 {
				t.Fatalf("private witness failed before its claimed gate: %+v raw=%s", legacy, raw)
			}
			out, err := run(t, input)
			if err == nil || len(out) != 0 || !strings.HasPrefix(err.Error(), "[r03-nullable-facts-"+tc.layer+"] ") {
				t.Fatalf("wrong rejection layer %s: err=%v raw=%s", tc.layer, err, out)
			}
			if tc.layer == "source" && !strings.Contains(err.Error(), legacy.Diagnostics[0].Code+" at /src/main.ts") {
				t.Fatal("Source adapter lost actual legacy diagnostic identity")
			}
			if tc.name == "assignmentGrammar" && !strings.Contains(err.Error(), "only ordinary nullish coalesce operators") {
				t.Fatal("assignment did not reach unsupported operator gate")
			}
			t.Logf("actual layer=%s: %v", tc.layer, err)
		})
	}
	t.Run("oldVersionAndSchema", func(t *testing.T) {
		r, legacy, raw, input := acquire(t, "function bad(value: i32 | undefined, replacement: i32): i32 { return value ?? replacement; }"+scalarEntry)
		if legacy.Status != "ok" || len(legacy.Diagnostics) != 0 {
			t.Fatalf("identity control prerequisite failed: %s", raw)
		}
		if out, err := run(t, input); err != nil || len(out) == 0 {
			t.Fatalf("current identity control failed: %v", err)
		}
		for _, tc := range []struct {
			name    string
			version int
			schema  string
		}{{"oldVersion", 2, absenceMasksV3SchemaForTest}, {"oldSchema", 3, "sha256:badc9425050a8f538fcb7b9397e5ef31fbde96391989dfeebb8cab6a122783bd"}} {
			t.Run(tc.name, func(t *testing.T) {
				changed := canonical(map[string]any{"family": "tsfg.nullable-facts-request-candidate", "version": tc.version, "schemaHash": tc.schema, "input": r})
				if bytes.Equal(input, changed) {
					t.Fatal("identity mutation was a no-op")
				}
				out, err := run(t, changed)
				if err == nil || len(out) != 0 || !strings.HasPrefix(err.Error(), "[r03-nullable-facts-") {
					t.Fatalf("wrong strict identity rejection: err=%v raw=%s", err, out)
				}
			})
		}
	})
}
