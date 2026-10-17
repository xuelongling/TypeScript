// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

type functionValueBoundaryExpression struct {
	Kind, Type, Operator, Symbol string
	Target                       *string
	Signature                    *functionValueWireSignature
	Callee, Left, Right, Operand *functionValueBoundaryExpression
	Arguments                    []functionValueBoundaryExpression
	Range                        span
}

type functionValueBoundaryStatement struct {
	Kind                     string
	Symbol                   *functionValueWireSymbol
	Initializer, Expression  *functionValueBoundaryExpression
	Target, Value, Condition *functionValueBoundaryExpression
	Body, ThenBody, ElseBody []functionValueBoundaryStatement
	Range                    span
}

type functionValueBoundaryFunction struct {
	Name, Symbol, ReturnType, LexicalParent string
	ReturnSignature                         *functionValueWireSignature
	Parameters                              []functionValueWireSymbol
	Body                                    []functionValueBoundaryStatement
	Captures                                []struct {
		Symbol      string
		Read, Write *bool
	}
}

type functionValueBoundaryResponse struct {
	Status      string
	Functions   []functionValueBoundaryFunction
	Symbols     []functionValueWireSymbol
	Diagnostics []struct {
		Code, RuleID, Phase, Category, Severity string
		UpstreamCode                            int32
		Range                                   span
	}
}

type functionValueCallWant struct {
	Count, Parameters int
	ReturnType        string
	Declaration       string
}

// This is a fresh official Program, without producer contexts or owned maps.
// Binding groups come from source occurrences; every call independently checks
// its official callee type and resolved signature, not a guessed target set.
func functionValueBoundaryOfficial(t *testing.T, r request, identities []closureBoundaryIdentity, calls map[string]functionValueCallWant, upstream int32) {
	t.Helper()
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
	if file == nil {
		t.Fatal("fresh official program omitted original source")
	}
	if diagnostics := program.GetSyntacticDiagnostics(t.Context(), nil); len(diagnostics) != 0 {
		t.Fatalf("official syntax diagnostics: %+v", diagnostics)
	}
	semantic := program.GetSemanticDiagnostics(t.Context(), nil)
	if upstream == 0 && len(semantic) != 0 || upstream != 0 && (len(semantic) != 1 || semantic[0].Code() != upstream) {
		t.Fatalf("unexpected official semantic diagnostics: %+v; wantCode=%d", semantic, upstream)
	}
	if diagnostics := program.GetProgramDiagnostics(); len(diagnostics) != 0 {
		t.Fatalf("official program diagnostics: %+v", diagnostics)
	}
	if diagnostics := program.GetGlobalDiagnostics(t.Context()); len(diagnostics) != 0 {
		t.Fatalf("official global diagnostics: %+v", diagnostics)
	}
	if upstream != 0 {
		return
	}
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	occurrences := map[string][]*ast.Node{}
	callSites := map[string][]*ast.Node{}
	var visit func(*ast.Node) bool
	visit = func(node *ast.Node) bool {
		if node.Kind == ast.KindIdentifier {
			occurrences[node.Text()] = append(occurrences[node.Text()], node)
		}
		if node.Kind == ast.KindCallExpression {
			text := strings.TrimSpace(file.Text()[node.Pos():node.End()])
			callSites[text] = append(callSites[text], node)
			resolved := check.GetResolvedSignature(node)
			signatures := check.GetSignaturesOfType(check.GetTypeAtLocation(node.AsCallExpression().Expression), checker.SignatureKindCall)
			if resolved == nil || resolved.Declaration() == nil || len(signatures) != 1 || signatures[0].Declaration() != resolved.Declaration() {
				t.Fatalf("official call lost its unique callee type or resolved signature: %s", text)
			}
		}
		node.ForEachChild(visit)
		return false
	}
	file.ForEachChild(visit)
	for _, identity := range identities {
		seenIndices := map[int]bool{}
		seenBindings := map[*ast.Symbol]bool{}
		for _, group := range identity.Groups {
			var expected *ast.Symbol
			for _, index := range group {
				if index < 0 || index >= len(occurrences[identity.Name]) || seenIndices[index] {
					t.Fatalf("invalid source-selected %s occurrence %d", identity.Name, index)
				}
				seenIndices[index] = true
				binding := check.GetSymbolAtLocation(occurrences[identity.Name][index])
				if binding == nil || expected != nil && binding != expected {
					t.Fatalf("official %s source uses lost their binding", identity.Name)
				}
				expected = binding
			}
			if expected == nil || seenBindings[expected] {
				t.Fatalf("official %s shadow bindings were merged", identity.Name)
			}
			seenBindings[expected] = true
		}
		if len(seenIndices) != len(occurrences[identity.Name]) {
			t.Fatalf("source oracle omitted %s occurrences", identity.Name)
		}
	}
	for text, want := range calls {
		sites := callSites[text]
		if len(sites) != want.Count {
			t.Fatalf("official call count for %s: got=%d want=%d", text, len(sites), want.Count)
		}
		for _, node := range sites {
			resolved := check.GetResolvedSignature(node)
			returned := check.GetReturnTypeOfSignature(resolved)
			validReturn := returned.IsBooleanLike() && want.ReturnType == "bool" || returned.Alias() != nil && returned.Alias().Symbol().Name == want.ReturnType
			if want.ReturnType == "function" {
				validReturn = len(check.GetSignaturesOfType(returned, checker.SignatureKindCall)) == 1
			}
			if want.ReturnType == "f64" && returned.Flags()&checker.TypeFlagsNumber != 0 {
				validReturn = true
			}
			if len(resolved.Parameters()) != want.Parameters || !validReturn {
				t.Fatalf("official call shape differs from fixture expectation: %s", text)
			}
			if want.Declaration != "" {
				declaration := resolved.Declaration()
				if strings.TrimSpace(file.Text()[declaration.Pos():declaration.End()]) != want.Declaration {
					t.Fatalf("official call signature declaration differs from source oracle: %s", text)
				}
			}
		}
	}
}

func functionValueBoundaryCommand(t *testing.T, r request) functionValueBoundaryResponse {
	t.Helper()
	input, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	original, err := json.Marshal(map[string]any{"rootFile": r.RootFile, "files": r.Files})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	commandErr := runFrontendCommand(t.Context(), nil, bytes.NewReader(input), &output)
	t.Logf("TSFG_FUNCTION_VALUE_BOUNDARY_REQUEST_V1 %s", base64.StdEncoding.EncodeToString(input))
	t.Logf("TSFG_FUNCTION_VALUE_BOUNDARY_RESPONSE_V1 %s", base64.StdEncoding.EncodeToString(output.Bytes()))
	t.Logf("TSFG_FUNCTION_VALUE_BOUNDARY_ORIGINAL_V1 %s", base64.StdEncoding.EncodeToString(original))
	if commandErr != nil {
		t.Fatalf("public command failed: %v; raw=%s", commandErr, output.Bytes())
	}
	var result functionValueBoundaryResponse
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("invalid public JSON: %v; raw=%s", err, output.Bytes())
	}
	return result
}

func functionValueBoundaryOK(t *testing.T, result functionValueBoundaryResponse) {
	t.Helper()
	if result.Status != "ok" || len(result.Diagnostics) != 0 {
		t.Fatalf("officially valid function-value boundary was rejected: %+v", result)
	}
}

func functionValueBoundaryLeaf(kind string, parameters ...string) *functionValueWireSignature {
	return &functionValueWireSignature{Parameters: append([]string{}, parameters...), ReturnType: kind}
}

func functionValueBoundarySignature(t *testing.T, got, want *functionValueWireSignature) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fixture callable description differs: got=%+v want=%+v", got, want)
	}
}

func functionValueBoundaryFunctionNamed(t *testing.T, result functionValueBoundaryResponse, name string) functionValueBoundaryFunction {
	t.Helper()
	var found *functionValueBoundaryFunction
	for index := range result.Functions {
		if result.Functions[index].Name == name {
			if found != nil {
				t.Fatalf("ambiguous function name %s", name)
			}
			found = &result.Functions[index]
		}
	}
	if found == nil {
		t.Fatalf("missing function %s", name)
	}
	return *found
}

func functionValueBoundaryBinding(t *testing.T, result functionValueBoundaryResponse, name, owner string) functionValueWireSymbol {
	t.Helper()
	var found *functionValueWireSymbol
	for index := range result.Symbols {
		s := &result.Symbols[index]
		if s.Name == name && s.LexicalOwner == owner {
			if found != nil {
				t.Fatalf("ambiguous binding %s in %s", name, owner)
			}
			found = s
		}
	}
	if found == nil {
		t.Fatalf("missing binding %s in %s", name, owner)
	}
	return *found
}

func functionValueBoundaryExpressions(result functionValueBoundaryResponse) []*functionValueBoundaryExpression {
	var expressions []*functionValueBoundaryExpression
	var expression func(*functionValueBoundaryExpression)
	expression = func(e *functionValueBoundaryExpression) {
		if e == nil {
			return
		}
		expressions = append(expressions, e)
		for _, child := range []*functionValueBoundaryExpression{e.Callee, e.Left, e.Right, e.Operand} {
			expression(child)
		}
		for index := range e.Arguments {
			expression(&e.Arguments[index])
		}
	}
	var statements func([]functionValueBoundaryStatement)
	statements = func(body []functionValueBoundaryStatement) {
		for index := range body {
			s := &body[index]
			for _, e := range []*functionValueBoundaryExpression{s.Initializer, s.Expression, s.Target, s.Value, s.Condition} {
				expression(e)
			}
			statements(s.Body)
			statements(s.ThenBody)
			statements(s.ElseBody)
		}
	}
	for index := range result.Functions {
		statements(result.Functions[index].Body)
	}
	return expressions
}

func functionValueBoundaryExpressionAt(t *testing.T, source string, result functionValueBoundaryResponse, kind, text string) *functionValueBoundaryExpression {
	t.Helper()
	var found *functionValueBoundaryExpression
	for _, e := range functionValueBoundaryExpressions(result) {
		if e.Kind == kind && sourceText(t, source, e.Range) == text {
			if found != nil {
				t.Fatalf("ambiguous owned expression %s %s", kind, text)
			}
			found = e
		}
	}
	if found == nil {
		t.Fatalf("missing source-anchored owned expression %s %s", kind, text)
	}
	return found
}

func functionValueBoundaryIndirect(t *testing.T, call *functionValueBoundaryExpression, binding string) {
	t.Helper()
	if call == nil || call.Kind != "indirectCall" || call.Target != nil || call.Callee == nil || call.Callee.Kind != "reference" || call.Callee.Symbol != binding {
		t.Fatalf("indirect call lost independent callee or gained target: %+v", call)
	}
}

func functionValueBoundaryCapture(t *testing.T, fn functionValueBoundaryFunction, ids ...string) {
	t.Helper()
	if len(fn.Captures) != len(ids) {
		t.Fatalf("unexpected direct capture count for %s: %+v", fn.Name, fn.Captures)
	}
	seen := map[string]bool{}
	for _, capture := range fn.Captures {
		if seen[capture.Symbol] || capture.Read == nil || !*capture.Read || capture.Write == nil || *capture.Write {
			t.Fatalf("invalid direct read capture in %s: %+v", fn.Name, capture)
		}
		seen[capture.Symbol] = true
	}
	for _, id := range ids {
		if !seen[id] {
			t.Fatalf("missing original captured binding %s in %s", id, fn.Name)
		}
	}
}
