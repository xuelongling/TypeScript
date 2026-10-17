// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"testing"
)

// Local wire views compile before the new owned production fields exist.
// They describe Source facts, not environment instances or stable FunctionIds.
type functionValueWireSignature struct {
	Parameters          []string
	ParameterSignatures []*functionValueWireSignature
	ReturnType          string
	ReturnSignature     *functionValueWireSignature
}

type functionValueWireSymbol struct {
	ID           string
	Name         string
	Kind         string
	Type         string
	Writable     bool
	Range        span
	LexicalOwner string
	Signature    *functionValueWireSignature
}

type functionValueWireExpression struct {
	Kind      string
	Type      string
	Symbol    string
	Target    *string
	Callee    *functionValueWireExpression
	Signature *functionValueWireSignature
	Arguments []functionValueWireExpression
	Range     span
}

type functionValueWireStatement struct {
	Kind        string
	Symbol      *functionValueWireSymbol
	Initializer *functionValueWireExpression
	Expression  *functionValueWireExpression
	Range       span
}

type functionValueWireFunction struct {
	Name            string
	Symbol          string
	Parameters      []functionValueWireSymbol
	ReturnType      string
	ReturnSignature *functionValueWireSignature
	Body            []functionValueWireStatement
	Range           span
	LexicalParent   string
	Captures        []struct {
		Symbol string
		Read   *bool
		Write  *bool
	}
}

func TestFrontendCommandReturnedClosureAndCallableParameterOwnedFacts(t *testing.T) {
	const source = "export function entry(seed: bool): bool { function make(value: bool): () => bool { function read(): bool { return value; } return read; } function invoke(fn: () => bool): bool { return fn(); } const selected = make(seed); const alias = selected; return invoke(alias); }"
	r := integerRequest(source)
	var anchors map[string]span
	if !t.Run("officialDiagnosticsSymbolsAndResolvedSignatures", func(t *testing.T) {
		anchors = functionValuesOfficialOracle(t, r)
	}) {
		t.Fatal("official oracle failed; owned projection was not attempted")
	}
	t.Run("ownedProjection", func(t *testing.T) {
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
		t.Logf("TSFG_FUNCTION_VALUES_REQUEST_V1 %s", base64.StdEncoding.EncodeToString(input))
		t.Logf("TSFG_FUNCTION_VALUES_RESPONSE_V1 %s", base64.StdEncoding.EncodeToString(output.Bytes()))
		t.Logf("TSFG_FUNCTION_VALUES_ORIGINAL_V1 %s", base64.StdEncoding.EncodeToString(original))
		if commandErr != nil {
			t.Fatalf("public frontend command failed: %v; rawResponse=%s", commandErr, output.Bytes())
		}
		var result struct {
			Status      string
			Inputs      []string
			Functions   []functionValueWireFunction
			Symbols     []functionValueWireSymbol
			Diagnostics []json.RawMessage
		}
		if err := json.Unmarshal(output.Bytes(), &result); err != nil {
			t.Fatalf("public response is not JSON: %v; rawResponse=%s", err, output.Bytes())
		}
		if result.Status != "ok" {
			t.Fatalf("officially valid returned closure and callable parameter must project owned facts: diagnostics=%s; rawResponse=%s", result.Diagnostics, output.Bytes())
		}
		if len(result.Diagnostics) != 0 || len(result.Functions) != 4 || len(result.Symbols) != 9 || !reflect.DeepEqual(result.Inputs, []string{"/src/main.ts", "/src/minimal-integers-v3.d.ts"}) {
			t.Fatalf("incomplete owned function-value definitions or original inputs: %+v", result)
		}
		leaf := &functionValueWireSignature{Parameters: []string{}, ReturnType: "bool"}
		factorySig := &functionValueWireSignature{Parameters: []string{"bool"}, ReturnType: "function", ReturnSignature: leaf}
		consumerSig := &functionValueWireSignature{Parameters: []string{"function"}, ParameterSignatures: []*functionValueWireSignature{leaf}, ReturnType: "bool"}
		checkSignature := func(got, want *functionValueWireSignature) {
			t.Helper()
			// These exact shapes describe this fixture only. They do not impose
			// signature equality as an assignment rule on other TS programs.
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("owned callable signature lost its aligned parameter or return description: got=%+v want=%+v", got, want)
			}
		}
		checkRange := func(got span, label string) {
			t.Helper()
			want, ok := anchors[label]
			if !ok || got != want {
				t.Fatalf("owned %s source range differs from the independent official AST: got=%+v want=%+v", label, got, want)
			}
		}
		definitions := map[string]functionValueWireSymbol{}
		for _, s := range result.Symbols {
			if s.ID == "" || definitions[s.ID].ID != "" {
				t.Fatalf("missing or duplicate Snapshot-local symbol: %+v", s)
			}
			definitions[s.ID] = s
		}
		entry, makeFn, read, invoke := result.Functions[0], result.Functions[1], result.Functions[2], result.Functions[3]
		for index, want := range []struct {
			name, parent, returns string
			parameters, body      int
			returnSignature       *functionValueWireSignature
		}{
			{"entry", "", "bool", 1, 5, nil},
			{"make", entry.Symbol, "function", 1, 2, leaf},
			{"read", makeFn.Symbol, "bool", 0, 1, nil},
			{"invoke", entry.Symbol, "bool", 1, 1, nil},
		} {
			fn := result.Functions[index]
			if fn.Name != want.name || fn.ReturnType != want.returns || fn.LexicalParent != want.parent || len(fn.Parameters) != want.parameters || len(fn.Body) != want.body {
				t.Fatalf("ordered function lost its original declaration, parent, parameters or body: %+v", fn)
			}
			checkSignature(fn.ReturnSignature, want.returnSignature)
			checkRange(fn.Range, want.name)
			definition := definitions[fn.Symbol]
			if definition.ID == "" || definition.Name != fn.Name || definition.Kind != "function" || definition.Type != fn.ReturnType || definition.Writable || definition.LexicalOwner != want.parent {
				t.Fatalf("function declaration lost its original owned symbol: %+v", definition)
			}
			checkRange(definition.Range, want.name+"Definition")
			if fn.Name != "read" && len(fn.Captures) != 0 {
				t.Fatalf("function without direct outer access gained transitive captures: %+v", fn)
			}
		}
		checkSymbol := func(s functionValueWireSymbol, name, kind, typ, owner string, writable bool, sig *functionValueWireSignature) {
			t.Helper()
			if s.ID == "" || s.Name != name || s.Kind != kind || s.Type != typ || s.LexicalOwner != owner || s.Writable != writable || !reflect.DeepEqual(definitions[s.ID], s) {
				t.Fatalf("binding lost its independent identity, role, type or owner: %+v", s)
			}
			checkSignature(s.Signature, sig)
			checkRange(s.Range, name)
		}
		seed, value, fn := entry.Parameters[0], makeFn.Parameters[0], invoke.Parameters[0]
		checkSymbol(seed, "seed", "parameter", "bool", entry.Symbol, true, nil)
		checkSymbol(value, "value", "parameter", "bool", makeFn.Symbol, true, nil)
		checkSymbol(fn, "fn", "parameter", "function", invoke.Symbol, true, leaf)
		if len(read.Captures) != 1 || read.Captures[0].Symbol != value.ID || read.Captures[0].Read == nil || !*read.Captures[0].Read || read.Captures[0].Write == nil || *read.Captures[0].Write {
			t.Fatalf("returned reader lost its original captured parameter and explicit direct access flags: %+v", read)
		}
		for _, item := range []struct {
			statement functionValueWireStatement
			fn        functionValueWireFunction
		}{
			{entry.Body[0], makeFn}, {entry.Body[1], invoke}, {makeFn.Body[0], read},
		} {
			if item.statement.Kind != "functionDeclaration" || item.statement.Symbol == nil || !reflect.DeepEqual(*item.statement.Symbol, definitions[item.fn.Symbol]) || item.statement.Range != item.fn.Range {
				t.Fatalf("parent body lost its ordered original function declaration: %+v", item.statement)
			}
		}
		selectedStatement, aliasStatement := entry.Body[2], entry.Body[3]
		if selectedStatement.Kind != "variable" || selectedStatement.Symbol == nil || selectedStatement.Initializer == nil || aliasStatement.Kind != "variable" || aliasStatement.Symbol == nil || aliasStatement.Initializer == nil {
			t.Fatal("entry lost its two original function-value variable declarations")
		}
		selected, alias := *selectedStatement.Symbol, *aliasStatement.Symbol
		checkSymbol(selected, "selected", "local", "function", entry.Symbol, false, leaf)
		checkSymbol(alias, "alias", "local", "function", entry.Symbol, false, leaf)
		checkRange(selectedStatement.Range, "selectedStatement")
		checkRange(aliasStatement.Range, "aliasStatement")
		if selected.ID == alias.ID || selected.ID == read.Symbol || alias.ID == read.Symbol || fn.ID == selected.ID || fn.ID == alias.ID {
			t.Fatal("same-signature callable bindings were merged with an alias or declaration")
		}
		checkReference := func(e *functionValueWireExpression, binding functionValueWireSymbol, label string) {
			t.Helper()
			if e == nil || e.Kind != "reference" || e.Type != binding.Type || e.Symbol != binding.ID || e.Target != nil || e.Callee != nil {
				t.Fatalf("reference lost its actual independent binding: %+v", e)
			}
			checkSignature(e.Signature, binding.Signature)
			checkRange(e.Range, label)
		}
		checkReference(aliasStatement.Initializer, selected, "aliasInitializer")
		checkDirectCall := func(e *functionValueWireExpression, target, typ, label string, sig *functionValueWireSignature) {
			t.Helper()
			if e == nil || e.Kind != "call" || e.Type != typ || e.Target == nil || *e.Target != target || e.Callee != nil || len(e.Arguments) != 1 {
				t.Fatalf("direct call lost its real declaration or ordered argument: %+v", e)
			}
			checkSignature(e.Signature, sig)
			checkRange(e.Range, label)
		}
		factoryCall := selectedStatement.Initializer
		checkDirectCall(factoryCall, makeFn.Symbol, "function", "makeCall", factorySig)
		checkReference(&factoryCall.Arguments[0], seed, "makeArgument")
		for _, returned := range []struct {
			statement functionValueWireStatement
			label     string
		}{
			{entry.Body[4], "entryReturn"}, {makeFn.Body[1], "makeReturn"}, {read.Body[0], "readReturn"}, {invoke.Body[0], "invokeReturn"},
		} {
			if returned.statement.Kind != "return" || returned.statement.Expression == nil {
				t.Fatalf("function lost its original return expression: %+v", returned.statement)
			}
			checkRange(returned.statement.Range, returned.label)
		}
		entryCall := entry.Body[4].Expression
		checkDirectCall(entryCall, invoke.Symbol, "bool", "invokeCall", consumerSig)
		checkReference(&entryCall.Arguments[0], alias, "invokeArgument")
		returnedFunction := makeFn.Body[1].Expression
		if returnedFunction.Kind != "functionReference" || returnedFunction.Type != "function" || returnedFunction.Target == nil || *returnedFunction.Target != read.Symbol || returnedFunction.Callee != nil || returnedFunction.Symbol != "" {
			t.Fatalf("factory return lost its original named function value: %+v", returnedFunction)
		}
		checkSignature(returnedFunction.Signature, leaf)
		checkRange(returnedFunction.Range, "returnedFunction")
		checkReference(read.Body[0].Expression, value, "captureReference")
		indirect := invoke.Body[0].Expression
		if indirect.Kind != "indirectCall" || indirect.Type != "bool" || indirect.Target != nil || indirect.Symbol != "" || indirect.Callee == nil || len(indirect.Arguments) != 0 {
			t.Fatalf("callback call gained a guessed target or lost its actual callee: %+v", indirect)
		}
		checkSignature(indirect.Signature, leaf)
		checkRange(indirect.Range, "fnCall")
		checkReference(indirect.Callee, fn, "indirectCallee")
		// No target-set, environment lifetime, escape safety or GC execution
		// conclusion follows from this owned Source projection.
	})
}
