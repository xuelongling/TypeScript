// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

type closureBoundarySymbol struct {
	ID           string
	Name         string
	Kind         string
	Type         string
	Writable     bool
	Range        span
	LexicalOwner string
}

type closureBoundaryExpression struct {
	Kind      string
	Type      string
	Symbol    string
	Target    string
	Arguments []closureBoundaryExpression
	Range     span
}

type closureBoundaryStatement struct {
	Kind       string
	Symbol     *closureBoundarySymbol
	Expression *closureBoundaryExpression
	Range      span
}

type closureBoundaryCapture struct {
	Symbol string
	Read   *bool
	Write  *bool
}

type closureBoundaryFunction struct {
	Name          string
	Symbol        string
	ReturnType    string
	Parameters    []closureBoundarySymbol
	Body          []closureBoundaryStatement
	Range         span
	LexicalParent string
	Captures      []closureBoundaryCapture
}

type closureBoundaryResponse struct {
	Status      string
	Functions   []closureBoundaryFunction
	Symbols     []closureBoundarySymbol
	Diagnostics []struct {
		Code  string
		Range span
	}
}

// Groups enumerate identifier occurrences in original source order. Each group
// must resolve to one official Symbol, and different groups must be distinct.
// Expectations come from the fixtures, never from owned producer IDs or maps.
type closureBoundaryIdentity struct {
	Name   string
	Groups [][]int
}

func closureBoundaryOfficial(t *testing.T, r request, identities []closureBoundaryIdentity) {
	t.Helper()
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
	if file == nil {
		t.Fatal("fresh official program omitted the original source")
	}
	if diagnostics := program.GetSyntacticDiagnostics(t.Context(), nil); len(diagnostics) != 0 {
		t.Fatalf("official syntax diagnostics: %+v", diagnostics)
	}
	if diagnostics := program.GetSemanticDiagnostics(t.Context(), nil); len(diagnostics) != 0 {
		t.Fatalf("official semantic diagnostics: %+v", diagnostics)
	}
	if diagnostics := program.GetProgramDiagnostics(); len(diagnostics) != 0 {
		t.Fatalf("official program diagnostics: %+v", diagnostics)
	}
	if diagnostics := program.GetGlobalDiagnostics(t.Context()); len(diagnostics) != 0 {
		t.Fatalf("official global diagnostics: %+v", diagnostics)
	}
	check, release := program.GetTypeCheckerForFileExclusive(t.Context(), file)
	defer release()
	for _, identity := range identities {
		var occurrences []*ast.Node
		var visit func(*ast.Node) bool
		visit = func(node *ast.Node) bool {
			if node.Kind == ast.KindIdentifier && node.Text() == identity.Name {
				occurrences = append(occurrences, node)
			}
			node.ForEachChild(visit)
			return false
		}
		file.ForEachChild(visit)
		seen := map[int]bool{}
		var groupSymbols []*ast.Symbol
		for _, group := range identity.Groups {
			var expected *ast.Symbol
			for _, index := range group {
				if index < 0 || index >= len(occurrences) || seen[index] {
					t.Fatalf("invalid source oracle occurrence for %s: %d of %d", identity.Name, index, len(occurrences))
				}
				seen[index] = true
				resolved := check.GetSymbolAtLocation(occurrences[index])
				if resolved == nil {
					t.Fatalf("official checker omitted %s occurrence %d", identity.Name, index)
				}
				if expected == nil {
					expected = resolved
				} else if resolved != expected {
					t.Fatalf("official %s occurrences do not share their declared binding: %+v", identity.Name, group)
				}
			}
			for _, previous := range groupSymbols {
				if previous == expected {
					t.Fatalf("official checker merged distinct shadow bindings named %s", identity.Name)
				}
			}
			groupSymbols = append(groupSymbols, expected)
		}
		if len(seen) != len(occurrences) {
			t.Fatalf("source oracle omitted %s occurrences: checked=%d actual=%d", identity.Name, len(seen), len(occurrences))
		}
	}
}

func closureBoundaryCommand(t *testing.T, r request) closureBoundaryResponse {
	t.Helper()
	input, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	commandErr := runFrontendCommand(t.Context(), nil, bytes.NewReader(input), &output)
	original, err := json.Marshal(map[string]any{"rootFile": r.RootFile, "files": r.Files})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("TSFG_CAPTURE_BOUNDARY_REQUEST_V1 %s", base64.StdEncoding.EncodeToString(input))
	t.Logf("TSFG_CAPTURE_BOUNDARY_RESPONSE_V1 %s", base64.StdEncoding.EncodeToString(output.Bytes()))
	t.Logf("TSFG_CAPTURE_BOUNDARY_ORIGINAL_V1 %s", base64.StdEncoding.EncodeToString(original))
	if commandErr != nil {
		t.Fatalf("public frontend command failed: %v; rawResponse=%s", commandErr, output.Bytes())
	}
	var result closureBoundaryResponse
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("public response is not JSON: %v; rawResponse=%s", err, output.Bytes())
	}
	return result
}

type closureBoundaryBinding struct {
	Name     string
	Owner    string
	Kind     string
	Writable bool
}

type closureBoundaryCaptureWant struct {
	Name  string
	Owner string
	Read  bool
	Write bool
}

type closureBoundaryFunctionWant struct {
	Name            string
	Parent          string
	Text            string
	ReturnBinding   string
	BindingOwner    string
	ReturnTarget    string
	ReturnText      string
	ArgumentBinding string
	ArgumentOwner   string
	Capture         *closureBoundaryCaptureWant
}

func closureBoundaryFacts(t *testing.T, source, payload string, result closureBoundaryResponse, functions []closureBoundaryFunctionWant, bindings []closureBoundaryBinding) {
	t.Helper()
	if result.Status != "ok" || len(result.Diagnostics) != 0 || len(result.Functions) != len(functions) || len(result.Symbols) != len(functions)+len(bindings) {
		t.Fatalf("incomplete owned boundary facts: %+v", result)
	}
	definitions := map[string]closureBoundarySymbol{}
	for _, owned := range result.Symbols {
		if owned.ID == "" || definitions[owned.ID].ID != "" {
			t.Fatalf("missing or duplicate owned identity: %+v", owned)
		}
		definitions[owned.ID] = owned
	}
	byName := map[string]closureBoundaryFunction{}
	for index, want := range functions {
		fn := result.Functions[index]
		if fn.Name != want.Name || fn.ReturnType != payload || len(fn.Body) == 0 || byName[fn.Name].Symbol != "" || sourceText(t, source, fn.Range) != want.Text {
			t.Fatalf("ordered function lost its full source body or checked type: %+v", fn)
		}
		byName[fn.Name] = fn
	}
	findBinding := func(name, owner string) closureBoundarySymbol {
		t.Helper()
		var found closureBoundarySymbol
		for _, owned := range result.Symbols {
			if owned.Name == name && owned.LexicalOwner == byName[owner].Symbol {
				if found.ID != "" {
					t.Fatalf("owned response duplicated binding %s in %s", name, owner)
				}
				found = owned
			}
		}
		if found.ID == "" {
			t.Fatalf("owned response omitted binding %s in %s", name, owner)
		}
		return found
	}
	for _, want := range bindings {
		owned := findBinding(want.Name, want.Owner)
		if owned.Kind != want.Kind || owned.Type != payload || owned.Writable != want.Writable || sourceText(t, source, owned.Range) != want.Name {
			t.Fatalf("binding lost its owner, mutability, type or source anchor: %+v", owned)
		}
	}
	for _, want := range functions {
		fn := byName[want.Name]
		definition := definitions[fn.Symbol]
		parent := byName[want.Parent]
		if definition.ID == "" || definition.Name != want.Name || definition.Kind != "function" || definition.Type != payload || definition.Writable || definition.LexicalOwner != parent.Symbol || fn.LexicalParent != parent.Symbol || sourceText(t, source, definition.Range) != want.Name {
			t.Fatalf("function lost its lexical parent or original definition: %+v", fn)
		}
		parameterCount := 0
		for _, binding := range bindings {
			if binding.Owner == want.Name && binding.Kind == "parameter" {
				parameterCount++
			}
		}
		if len(fn.Parameters) != parameterCount {
			t.Fatalf("function lost its original parameters: %+v", fn)
		}
		for _, parameter := range fn.Parameters {
			if definitions[parameter.ID] != parameter || parameter.LexicalOwner != fn.Symbol || parameter.Kind != "parameter" || parameter.Type != payload {
				t.Fatalf("parameter lost its independent owned definition: %+v", parameter)
			}
		}
		for _, item := range fn.Body {
			if item.Symbol != nil && definitions[item.Symbol.ID] != *item.Symbol {
				t.Fatalf("body binding disagrees with its original definition: %+v", item)
			}
		}
		if parent.Symbol != "" {
			declarations := 0
			for _, item := range parent.Body {
				if item.Kind == "functionDeclaration" && item.Symbol != nil && item.Symbol.ID == fn.Symbol {
					declarations++
					if item.Range != fn.Range || *item.Symbol != definition {
						t.Fatalf("nested declaration lost its full anchor or shared definition: %+v", item)
					}
				}
			}
			if declarations != 1 {
				t.Fatalf("parent must retain one original declaration for %s: %d", fn.Name, declarations)
			}
		}
		returned := fn.Body[len(fn.Body)-1]
		if returned.Kind != "return" || returned.Expression == nil || returned.Expression.Type != payload {
			t.Fatalf("function lost its original return: %+v", fn)
		}
		if want.ReturnTarget != "" {
			returnText := want.ReturnText
			if returnText == "" {
				returnText = "return " + want.ReturnTarget + "();"
			}
			if returned.Expression.Kind != "call" || returned.Expression.Target != byName[want.ReturnTarget].Symbol || sourceText(t, source, returned.Range) != returnText {
				t.Fatalf("return lost its original direct target: %+v", returned)
			}
			arguments := returned.Expression.Arguments
			if want.ArgumentBinding == "" {
				if len(arguments) != 0 {
					t.Fatalf("direct call gained arguments: %+v", arguments)
				}
			} else {
				binding := findBinding(want.ArgumentBinding, want.ArgumentOwner)
				if len(arguments) != 1 || arguments[0].Kind != "reference" || arguments[0].Type != payload || arguments[0].Symbol != binding.ID || sourceText(t, source, arguments[0].Range) != want.ArgumentBinding {
					t.Fatalf("direct call lost its outer argument binding: %+v", arguments)
				}
			}
		} else {
			binding := findBinding(want.ReturnBinding, want.BindingOwner)
			if returned.Expression.Kind != "reference" || returned.Expression.Symbol != binding.ID || sourceText(t, source, returned.Expression.Range) != want.ReturnBinding || sourceText(t, source, returned.Range) != "return "+want.ReturnBinding+";" {
				t.Fatalf("return lost its checked lexical binding: %+v", returned)
			}
		}
		if want.Capture == nil {
			if len(fn.Captures) != 0 {
				t.Fatalf("function without direct outer access gained captures: %+v", fn)
			}
			continue
		}
		binding := findBinding(want.Capture.Name, want.Capture.Owner)
		if len(fn.Captures) != 1 || fn.Captures[0].Symbol != binding.ID || fn.Captures[0].Read == nil || *fn.Captures[0].Read != want.Capture.Read || fn.Captures[0].Write == nil || *fn.Captures[0].Write != want.Capture.Write {
			t.Fatalf("direct capture lost its original binding or explicit access flags: %+v", fn)
		}
	}
}
