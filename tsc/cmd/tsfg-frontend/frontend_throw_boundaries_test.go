// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
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

// This oracle owns a fresh official Program. It never uses the producer's scalar,
// presence, offset or expression projection helpers to derive an expectation.
func throwBoundaryProgramForTest(t *testing.T, r request) *compiler.Program {
	t.Helper()
	contents := map[string]string{}
	for _, f := range r.Files {
		contents[f.Path] = f.Content
	}
	p := compiler.NewProgram(compiler.ProgramOptions{
		Config: &tsoptions.ParsedCommandLine{ParsedConfig: &core.ParsedOptions{
			FileNames:       append([]string{r.RootFile}, r.Declarations.Paths...),
			CompilerOptions: &core.CompilerOptions{NoLib: core.TSTrue, NoEmit: core.TSTrue, Strict: core.TSTrue, Target: core.ScriptTargetESNext},
		}},
		Host:           compiler.NewCompilerHost(r.SourceRoot, vfstest.FromMap(contents, true), r.SourceRoot, nil, nil),
		SingleThreaded: core.TSTrue,
	})
	if p.GetSourceFile(r.RootFile) == nil {
		t.Fatal("official root source missing")
	}
	return p
}

func throwBoundarySpanForTest(source string, start, end int) span {
	return span{len(utf16.Encode([]rune(source[:start]))), len(utf16.Encode([]rune(source[:end])))}
}

func throwBoundaryNodeForTest(t *testing.T, file *ast.SourceFile, node *ast.Node) span {
	t.Helper()
	if node == nil {
		t.Fatal("official node missing")
	}
	want := throwBoundarySpanForTest(file.Text(), node.Pos(), node.End())
	positions := file.GetPositionMap()
	if want != (span{positions.UTF8ToUTF16(node.Pos()), positions.UTF8ToUTF16(node.End())}) {
		t.Fatal("independent UTF16 preimage disagrees with official position map")
	}
	return want
}

func throwBoundaryDiagnosticsForTest(t *testing.T, p *compiler.Program) {
	t.Helper()
	for _, group := range [][]*ast.Diagnostic{
		p.GetSyntacticDiagnostics(t.Context(), nil), p.GetSemanticDiagnostics(t.Context(), nil),
		p.GetProgramDiagnostics(), p.GetGlobalDiagnostics(t.Context()),
	} {
		if len(group) != 0 {
			t.Fatalf("official diagnostic prerequisite failed: %+v", group)
		}
	}
}

func throwBoundaryCommandForTest(t *testing.T, r request) (response, []byte) {
	t.Helper()
	before, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("TSFG_THROW_BOUNDARY_REQUEST_V1 %s", base64.StdEncoding.EncodeToString(before))
	result, raw := nullableBoundaryCommandForTest(t, r)
	t.Logf("TSFG_THROW_BOUNDARY_RESPONSE_V1 %s", base64.StdEncoding.EncodeToString(raw))
	after, err := json.Marshal(r)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("public command changed original request")
	}
	return result, raw
}

func throwBoundaryCapabilityForTest(t *testing.T, source string, r request, want span) {
	t.Helper()
	result, raw := throwBoundaryCommandForTest(t, r)
	if result.Status != "error" || len(result.Functions) != 0 || len(result.Diagnostics) != 1 {
		t.Fatalf("finite throw boundary admitted or lost diagnostic: %+v; raw=%s", result, raw)
	}
	d := result.Diagnostics[0]
	if d.Code != "r02b-d-unsupported" || d.Category != "unsupported" || d.Phase != "capability" ||
		d.Severity != "error" || d.UpstreamCode != 0 || d.RuleID != "r01-ts-baseline" ||
		d.File != r.RootFile || d.Primary.SourceID != r.RootFile || d.Range != want ||
		d.Primary.Start != want.Start || d.Primary.End != want.End {
		t.Fatalf("did not reach original whole throw/try capability gate: %+v; raw=%s", d, raw)
	}
	if text := sourceText(t, source, want); !strings.HasPrefix(text, "throw ") && !strings.HasPrefix(text, "try ") {
		t.Fatalf("boundary anchor is not original throw/try: %q", text)
	}
}

func TestFrontendThrowI32OwnedBoundaries(t *testing.T) {
	positives := []struct {
		name, source, payload, returnType string
		local, call                       bool
	}{
		{"explicitConstCRLFNonBMP", "// 😀\r\nexport function fail(value: i32): i32 {\r\n const payload: i32 = value;\r\n throw payload;\r\n}", "payload", "i32", true, false},
		{"directCall", "function echo(value: i32): i32 { return value; }\r\n/* 😀 */ export function fail(value: i32): i32 { throw echo(value); }", "echo(value)", "i32", false, true},
		{"boolReturnDoesNotHintPayload", "/* 😀 */ export function fail(value: i32): bool { throw value; }", "value", "bool", false, false},
	}
	for _, tc := range positives {
		if !t.Run(tc.name, func(t *testing.T) {
			r := integerRequest(tc.source)
			p := throwBoundaryProgramForTest(t, r)
			if !t.Run("officialDiagnostics", func(t *testing.T) { throwBoundaryDiagnosticsForTest(t, p) }) {
				return
			}
			file := p.GetSourceFile(r.RootFile)
			anchors := map[string]span{}
			var expectedSignature *signature
			if !t.Run("officialPayloadOracle", func(t *testing.T) {
				index := 0
				if tc.call {
					index = 1
				}
				if len(file.Statements.Nodes) != index+1 {
					t.Fatal("unexpected official top-level declarations")
				}
				node := file.Statements.Nodes[index]
				if node.Kind != ast.KindFunctionDeclaration {
					t.Fatal("official function missing")
				}
				decl := node.AsFunctionDeclaration()
				if decl.Name().Text() != "fail" || len(decl.Parameters.Nodes) != 1 || decl.Body == nil || decl.Body.Kind != ast.KindBlock {
					t.Fatal("official fail shape changed")
				}
				body := decl.Body.AsBlock().Statements.Nodes
				expectedBody := 1
				if tc.local {
					expectedBody++
				}
				if len(body) != expectedBody || body[len(body)-1].Kind != ast.KindThrowStatement {
					t.Fatal("official body lost one final throw")
				}
				thrown := body[len(body)-1]
				payload := thrown.AsThrowStatement().Expression
				if payload == nil {
					t.Fatal("throw lacks one expression")
				}
				check, release := p.GetTypeCheckerForFileExclusive(t.Context(), file)
				defer release()
				parameter := decl.Parameters.Nodes[0]
				declared := check.GetTypeFromTypeNode(parameter.AsParameterDeclaration().Type)
				if declared == nil || check.GetTypeAtLocation(parameter.Name()) != declared || check.GetTypeAtLocation(payload) != declared ||
					declared.Flags()&(checker.TypeFlagsNull|checker.TypeFlagsUndefined) != 0 {
					t.Fatal("official payload is not exactly the declared nonnullable i32")
				}
				if tc.returnType == "bool" && check.GetTypeFromTypeNode(decl.Type) == declared {
					t.Fatal("bool return oracle cannot expose a wrong payload hint")
				}
				anchors["function"] = throwBoundaryNodeForTest(t, file, node)
				anchors["definition"] = throwBoundaryNodeForTest(t, file, decl.Name())
				anchors["parameter"] = throwBoundaryNodeForTest(t, file, parameter.Name())
				anchors["throw"] = throwBoundaryNodeForTest(t, file, thrown)
				anchors["payload"] = throwBoundaryNodeForTest(t, file, payload)
				if sourceText(t, tc.source, anchors["payload"]) != tc.payload || sourceText(t, tc.source, anchors["throw"]) != "throw "+tc.payload+";" {
					t.Fatal("official original spelling changed")
				}
				binding := check.GetSymbolAtLocation(parameter.Name())
				if binding == nil {
					t.Fatal("official parameter symbol missing")
				}
				switch {
				case tc.local:
					if body[0].Kind != ast.KindVariableStatement {
						t.Fatal("official explicit const missing")
					}
					list := body[0].AsVariableStatement().DeclarationList
					if list.Flags&ast.NodeFlagsConst == 0 || len(list.AsVariableDeclarationList().Declarations.Nodes) != 1 {
						t.Fatal("local must be one const declaration")
					}
					local := list.AsVariableDeclarationList().Declarations.Nodes[0]
					value := local.AsVariableDeclaration()
					localSymbol := check.GetSymbolAtLocation(local.Name())
					if local.Name().Text() != "payload" || value.Type == nil || value.Initializer == nil || localSymbol == nil || localSymbol == binding ||
						check.GetTypeFromTypeNode(value.Type) != declared || check.GetTypeAtLocation(value.Initializer) != declared ||
						check.GetSymbolAtLocation(value.Initializer) != binding || check.GetSymbolAtLocation(payload) != localSymbol {
						t.Fatal("const payload lost its explicit type or distinct official binding")
					}
					anchors["local"] = throwBoundaryNodeForTest(t, file, local.Name())
					anchors["initializer"] = throwBoundaryNodeForTest(t, file, value.Initializer)
				case tc.call:
					if payload.Kind != ast.KindCallExpression {
						t.Fatal("one direct-call payload required")
					}
					call := payload.AsCallExpression()
					calleeNode := file.Statements.Nodes[0]
					callee := calleeNode.AsFunctionDeclaration()
					resolved := check.GetResolvedSignature(payload)
					if callee.Name().Text() != "echo" || len(callee.Parameters.Nodes) != 1 || call.Expression.Kind != ast.KindIdentifier || len(call.Arguments.Nodes) != 1 ||
						resolved == nil || resolved.Declaration() != calleeNode || check.GetReturnTypeOfSignature(resolved) != declared ||
						check.GetSymbolAtLocation(call.Expression) != check.GetSymbolAtLocation(callee.Name()) ||
						check.GetSymbolAtLocation(call.Arguments.Nodes[0]) != binding || check.GetTypeAtLocation(call.Arguments.Nodes[0]) != declared {
						t.Fatal("official direct call lost its exact callee/signature/caller argument")
					}
					anchors["callee"] = throwBoundaryNodeForTest(t, file, callee.Name())
					anchors["argument"] = throwBoundaryNodeForTest(t, file, call.Arguments.Nodes[0])
					param := callee.Parameters.Nodes[0]
					typeNode := param.AsParameterDeclaration().Type
					typeStart := typeNode.Pos()
					for typeStart < typeNode.End() && (tc.source[typeStart] == ' ' || tc.source[typeStart] == '\t') {
						typeStart++
					}
					expectedSignature = &signature{Parameters: []string{"i32"}, ReturnType: "i32", Presence: []*parameterPresence{{Range: throwBoundaryNodeForTest(t, file, param), Annotation: throwBoundarySpanForTest(tc.source, typeStart, typeNode.End())}}}
				default:
					if payload.Kind != ast.KindIdentifier || check.GetSymbolAtLocation(payload) != binding {
						t.Fatal("official throw reference rebound")
					}
				}
				oracle, err := json.Marshal(anchors)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("TSFG_THROW_BOUNDARY_OFFICIAL_ORACLE_V1 %s", base64.StdEncoding.EncodeToString(oracle))
			}) {
				return
			}
			t.Run("ownedProjection", func(t *testing.T) {
				result, raw := throwBoundaryCommandForTest(t, r)
				expectedFunctions, expectedSymbols := 1, 2
				if tc.local {
					expectedSymbols++
				}
				if tc.call {
					expectedFunctions++
					expectedSymbols += 2
				}
				if result.Status != "ok" || len(result.Diagnostics) != 0 || len(result.Functions) != expectedFunctions || len(result.Symbols) != expectedSymbols {
					t.Fatalf("official throw oracle passed, owned facts rejected: %+v; raw=%s", result, raw)
				}
				defs := map[string]symbol{}
				for _, s := range result.Symbols {
					if s.ID == "" || defs[s.ID].ID != "" {
						t.Fatal("missing/duplicate owned symbol")
					}
					defs[s.ID] = s
				}
				var fn, callee function
				for _, f := range result.Functions {
					if f.Name == "fail" {
						fn = f
					} else if f.Name == "echo" {
						callee = f
					}
				}
				expectedBody := 1
				if tc.local {
					expectedBody++
				}
				if fn.Symbol == "" || fn.ReturnType != tc.returnType || fn.ReturnPresence != nil || fn.Range != anchors["function"] || len(fn.Parameters) != 1 || len(fn.Body) != expectedBody {
					t.Fatalf("throw function shape changed: %+v", fn)
				}
				definition := defs[fn.Symbol]
				if definition.Name != "fail" || definition.Kind != "function" || definition.Type != tc.returnType || definition.Range != anchors["definition"] {
					t.Fatal("function identity differs from original declaration")
				}
				param := fn.Parameters[0]
				if param.ID == "" || param.Name != "value" || param.Kind != "parameter" || param.Type != "i32" || !param.Writable || param.Range != anchors["parameter"] || !reflect.DeepEqual(param, defs[param.ID]) {
					t.Fatal("caller parameter binding changed")
				}
				thrown := fn.Body[len(fn.Body)-1]
				if thrown.Kind != "throw" || thrown.Range != anchors["throw"] || thrown.Expression == nil {
					t.Fatalf("one original throw subtree missing: %+v", thrown)
				}
				expected := &expression{Kind: "reference", Type: "i32", Symbol: param.ID, Range: anchors["payload"]}
				if tc.local {
					local := fn.Body[0]
					if local.Kind != "variable" || local.Symbol == nil || local.Symbol.Name != "payload" || local.Symbol.Kind != "local" || local.Symbol.Type != "i32" || local.Symbol.Writable || local.Symbol.ID == param.ID || local.Symbol.Range != anchors["local"] || !reflect.DeepEqual(*local.Symbol, defs[local.Symbol.ID]) {
						t.Fatal("explicit const lost its distinct immutable owned binding")
					}
					if !reflect.DeepEqual(local.Initializer, &expression{Kind: "reference", Type: "i32", Symbol: param.ID, Range: anchors["initializer"]}) {
						t.Fatal("const initializer was rewritten or rebound")
					}
					expected.Symbol = local.Symbol.ID
				}
				if tc.call {
					if callee.Symbol == "" || callee.Symbol == fn.Symbol || callee.ReturnType != "i32" || len(callee.Parameters) != 1 || defs[callee.Symbol].Range != anchors["callee"] {
						t.Fatal("owned callee identity missing")
					}
					args := []expression{{Kind: "reference", Type: "i32", Symbol: param.ID, Range: anchors["argument"]}}
					expected = &expression{Kind: "call", Type: "i32", Target: callee.Symbol, Signature: expectedSignature, Arguments: &args, Range: anchors["payload"]}
				}
				if !reflect.DeepEqual(thrown.Expression, expected) {
					t.Fatalf("payload subtree duplicated/retyped/rebound: got=%+v want=%+v", thrown.Expression, expected)
				}
				record := nullableBoundaryRecordForTest(t, raw, "functions", "name", "fail")
				var body []map[string]json.RawMessage
				if err := json.Unmarshal(record["body"], &body); err != nil {
					t.Fatal(err)
				}
				actual := body[len(body)-1]
				if len(actual) != 3 || string(actual["kind"]) != "\"throw\"" || actual["expression"] == nil || actual["range"] == nil {
					t.Fatalf("throw must serialize only one expression and original range: %s", record["body"])
				}
			})
		}) {
			return
		}
	}

	// Identical return controls prove the non-i32 declaration/expression is already
	// projectable; an early parameter failure cannot impersonate the throw gate.
	for _, typ := range []string{"bool", "i16", "i32 | null", "i32 | undefined"} {
		t.Run("finitePayload/"+typ, func(t *testing.T) {
			source := "/* 😀 */ export function fail(value: " + typ + "): " + typ + " { throw value; }"
			control := strings.Replace(source, "throw value;", "return value;", 1)
			controlRequest := integerRequest(control)
			throwBoundaryDiagnosticsForTest(t, throwBoundaryProgramForTest(t, controlRequest))
			old, raw := throwBoundaryCommandForTest(t, controlRequest)
			if old.Status != "ok" || len(old.Functions) != 1 || len(old.Diagnostics) != 0 || len(old.Functions[0].Body) != 1 || old.Functions[0].Body[0].Expression == nil {
				t.Fatalf("existing payload control missing: %+v; raw=%s", old, raw)
			}
			r := integerRequest(source)
			p := throwBoundaryProgramForTest(t, r)
			throwBoundaryDiagnosticsForTest(t, p)
			file := p.GetSourceFile(r.RootFile)
			fn := file.Statements.Nodes[0].AsFunctionDeclaration()
			node := fn.Body.AsBlock().Statements.Nodes[0]
			if node.Kind != ast.KindThrowStatement || node.AsThrowStatement().Expression == nil {
				t.Fatal("original throw missing")
			}
			check, release := p.GetTypeCheckerForFileExclusive(t.Context(), file)
			declared := check.GetTypeFromTypeNode(fn.Parameters.Nodes[0].AsParameterDeclaration().Type)
			sameType := declared != nil && check.GetTypeAtLocation(node.AsThrowStatement().Expression) == declared
			release()
			if !sameType {
				t.Fatal("official rejected-payload precondition lost declared type identity")
			}
			start := strings.Index(source, "throw value;")
			want := throwBoundarySpanForTest(source, start, start+len("throw value;"))
			throwBoundaryCapabilityForTest(t, source, r, want)
		})
	}
	for _, body := range []string{
		"try { throw value; } catch { return value; }",
		"try { throw value; } finally { const copy: i32 = value; }",
		"try { throw value; } catch { return value; } finally { const copy: i32 = value; }",
	} {
		t.Run(body, func(t *testing.T) {
			source := "export function fail(value: i32): i32 { " + body + " }"
			r := integerRequest(source)
			p := throwBoundaryProgramForTest(t, r)
			throwBoundaryDiagnosticsForTest(t, p)
			file := p.GetSourceFile(r.RootFile)
			node := file.Statements.Nodes[0].AsFunctionDeclaration().Body.AsBlock().Statements.Nodes[0]
			if node.Kind != ast.KindTryStatement || sourceText(t, source, throwBoundaryNodeForTest(t, file, node)) != body {
				t.Fatal("official try/catch/finally shape changed")
			}
			start := strings.Index(source, body)
			throwBoundaryCapabilityForTest(t, source, r, throwBoundarySpanForTest(source, start, start+len(body)))
		})
	}
	for _, tc := range []struct {
		name, source, code, phase string
		upstream                  int32
	}{
		{"unresolvedName", "// 😀\r\nexport function fail(value: i32): i32 { throw missingPayload; }", "r01-d-unresolved-name", "types", 2304},
		{"throwLineBreak", "export function fail(value: i32): i32 { throw\r\nvalue; }", "r02b-d-type", "types", 1142},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := integerRequest(tc.source)
			p := throwBoundaryProgramForTest(t, r)
			file := p.GetSourceFile(r.RootFile)
			syntax := p.GetSyntacticDiagnostics(t.Context(), nil)
			semantic := p.GetSemanticDiagnostics(t.Context(), nil)
			if len(p.GetProgramDiagnostics()) != 0 || len(p.GetGlobalDiagnostics(t.Context())) != 0 {
				t.Fatal("unexpected official global/program diagnostic")
			}
			if len(syntax) != 0 {
				t.Fatalf("semantic diagnostic fixture has syntax errors: %+v", syntax)
			}
			official := semantic
			if len(official) != 1 || official[0].Code() != tc.upstream || official[0].File() != file {
				t.Fatalf("official diagnostic precondition changed: %+v", official)
			}
			where := throwBoundarySpanForTest(tc.source, official[0].Pos(), official[0].End())
			if tc.upstream == 2304 && sourceText(t, tc.source, where) != "missingPayload" {
				t.Fatal("official name diagnostic lost original token")
			}
			result, raw := throwBoundaryCommandForTest(t, r)
			if result.Status != "error" || len(result.Functions) != 0 || len(result.Diagnostics) != 1 {
				t.Fatalf("official error did not survive: %+v; raw=%s", result, raw)
			}
			d := result.Diagnostics[0]
			if d.Code != tc.code || d.UpstreamCode != tc.upstream || d.Phase != tc.phase || d.RuleID != "r01-ts-baseline" || d.Category != "error" || d.Severity != "error" || d.File != r.RootFile || d.Range != where || d.Primary.SourceID != r.RootFile || d.Primary.Start != where.Start || d.Primary.End != where.End {
				t.Fatalf("official diagnostic was masked/relabelled/relocated: %+v; raw=%s", d, raw)
			}
		})
	}
}
