// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func validNumericContract(p *arrayGenericPayload, source string, at map[string]span) error {
	fail := func(s string) error { return fmt.Errorf("original numeric source facts differ: %s", s) }
	if p.SourceRoot != "/src" || p.RootFile != "/src/main.ts" || p.PositionUnit != "utf16" || len(p.Files) != 2 || len(p.Functions) != 5 || len(p.Exports) != 4 || len(p.Diagnostics) != 0 {
		return fail("complete source closure")
	}
	found := false
	for _, f := range p.Files {
		if f.Path == "/src/main.ts" {
			found = f.Content == source && f.ByteLength == 595 && f.UTF16Length == 595 && f.SHA256 == arrayFactsSHA([]byte(source))
		}
	}
	if !found {
		return fail("complete original UTF16 source")
	}
	functions := map[string]*arrayGenericFunction{}
	for i := range p.Functions {
		f := &p.Functions[i]
		if functions[f.Name] != nil {
			return fail("duplicate function")
		}
		functions[f.Name] = f
	}
	bindings := map[string]arrayFactsBinding{}
	for _, name := range []string{"keep", "binary", "unary", "short", "numeric"} {
		f := functions[name]
		want := "i32"
		if name == "short" {
			want = "bool"
		}
		if f == nil || f.SourcePath != "/src/main.ts" || f.DeclarationRange != at[name] || f.NameRange != at[name+".name"] || f.ReturnType != want || f.ReturnAnnotationRange == nil || *f.ReturnAnnotationRange != at[name+".return.type"] {
			return fail("function signature " + name)
		}
		for _, b := range append(append([]arrayFactsBinding{}, f.Parameters...), f.Bindings...) {
			typ := "i32"
			if b.Name == "a" {
				typ = "array<union<i32,undefined>>"
			} else if b.Name == "i" {
				typ = "u32"
			} else if b.Name == "use" {
				typ = "bool"
			}
			if b.ID == "" || b.DeclarationRange != at[name+"."+b.Name] || b.DeclaredType != typ {
				return fail("original binding " + name + "." + b.Name)
			}
			bindings[name+"."+b.Name] = b
		}
	}
	keep := functions["keep"]
	if len(keep.Parameters) != 2 || len(keep.Bindings) != 2 || len(keep.Body) != 3 {
		return fail("keep arity/body")
	}
	exported := map[string]bool{}
	for _, e := range p.Exports {
		if exported[e.FunctionID] || e.FunctionID == keep.ID {
			return fail("export closure")
		}
		exported[e.FunctionID] = true
	}
	for _, name := range []string{"binary", "unary", "short", "numeric"} {
		if !exported[functions[name].ID] {
			return fail("missing export")
		}
	}
	ex := func(e *arrayGenericExpression, kind, typ, key string) bool {
		return e != nil && e.Kind == kind && e.Type == typ && e.FlowType == typ && e.Range == at[key]
	}
	ref := func(e *arrayGenericExpression, name, b, key string) bool {
		v := bindings[name+"."+b]
		return ex(e, "bindingReference", v.DeclaredType, key) && e.Binding == v.ID
	}
	lit := func(e *arrayGenericExpression, typ, key, token string) bool {
		return ex(e, "numericLiteral", typ, key) && e.Token == token && e.Decimal == token
	}
	op := func(e *arrayGenericExpression, kind, typ, key, token string) bool {
		return ex(e, kind, typ, key) && e.Operator == token && e.OperatorRange != nil && *e.OperatorRange == at[key+".operator"]
	}
	local := func(f *arrayGenericFunction, index int, name string) bool {
		b := bindings[f.Name+"."+name]
		s := f.Body[index]
		return s.Kind == "constDeclaration" && s.Range == b.DeclarationRange && s.Binding == b.ID && !b.Writable && s.Initializer != nil
	}
	cast := keep.Body[0].Initializer
	if !local(keep, 0, "x") || !local(keep, 1, "alias") || !ex(cast, "as", "i32", "keep.as") || cast.AnnotationRange == nil || *cast.AnnotationRange != at["keep.target"] || cast.OperatorRange == nil || *cast.OperatorRange != at["keep.operator"] {
		return fail("keep assertion/static target")
	}
	access := cast.Expression
	if !ex(access, "checkedElementAccess", "union<i32,undefined>", "keep.access") || !ref(access.Receiver, "keep", "a", "keep.array") || !ref(access.Index, "keep", "i", "keep.index") || !ref(keep.Body[1].Initializer, "keep", "x", "keep.alias.value") || keep.Body[2].Kind != "return" || keep.Body[2].Range != at["keep.return"] || !ref(keep.Body[2].Value, "keep", "alias", "keep.return.value") {
		return fail("keep read/alias/return symbols")
	}
	call := func(e *arrayGenericExpression, name string) bool {
		if !ex(e, "call", "i32", name+".call") || e.Callee != keep.ID || e.CalleeRange == nil || *e.CalleeRange != at[name+".callee"] || e.Signature == nil || e.Signature.DeclarationRange != at["keep"] || e.Signature.SourcePath != "/src/main.ts" || e.Signature.ReturnType != "i32" || len(e.Signature.Parameters) != 2 || e.Signature.Parameters[0] != "array<union<i32,undefined>>" || e.Signature.Parameters[1] != "u32" || e.Arguments == nil || len(*e.Arguments) != 2 {
			return false
		}
		return ref(&(*e.Arguments)[0], name, "a", name+".call.a") && lit(&(*e.Arguments)[1], "u32", name+".call.i", "0")
	}
	for _, name := range []string{"binary", "unary", "short", "numeric"} {
		f := functions[name]
		expectedBody, expectedBindings, expectedParams := 4, 2, 1
		if name == "short" || name == "numeric" {
			expectedBody, expectedBindings = 2, 1
		}
		if name == "numeric" {
			expectedParams = 0
		}
		if len(f.Body) != expectedBody || len(f.Bindings) != expectedBindings || len(f.Parameters) != expectedParams || !local(f, 0, "a") {
			return fail("original body/bindings " + name)
		}
		a := f.Body[0].Initializer
		if !ex(a, "arrayLiteral", "array<union<i32,undefined>>", name+".array") || a.Elements == nil || len(*a.Elements) != 1 {
			return fail("original array literal " + name)
		}
		element := &(*a.Elements)[0]
		if name == "numeric" {
			if !lit(element, "i32", name+".element", "1") {
				return fail("numeric source positive literal")
			}
		} else if !ex(element, "undefinedLiteral", "undefined", name+".element") {
			return fail("source undefined element")
		}
		if name == "binary" || name == "unary" {
			if !local(f, 1, "x") || !call(f.Body[1].Initializer, name) {
				return fail("original keep result local")
			}
			s := f.Body[2]
			if s.Kind != "if" || s.Range != at[name+".if"] || !ref(s.Condition, name, "use", name+".condition") || s.Then == nil || len(*s.Then) != 1 || s.Else == nil || len(*s.Else) != 0 {
				return fail("original if branch")
			}
			ret := (*s.Then)[0]
			if ret.Kind != "return" || ret.Range != at[name+".then.return"] {
				return fail("original then return")
			}
			v := ret.Value
			if name == "binary" {
				if !op(v, "binary", "i32", name+".operation", "+") || !ref(v.Left, name, "x", name+".operand") || !lit(v.Right, "i32", name+".one", "1") {
					return fail("binary source operation")
				}
			} else if !op(v, "unary", "i32", name+".operation", "-") || !ref(v.Expression, name, "x", name+".operand") {
				return fail("unary source operation")
			}
			if f.Body[3].Kind != "return" || f.Body[3].Range != at[name+".return"] || !lit(f.Body[3].Value, "i32", name+".fallback", "7") {
				return fail("original fallback")
			}
		} else {
			if f.Body[1].Kind != "return" || f.Body[1].Range != at[name+".return"] {
				return fail("original final return")
			}
			v := f.Body[1].Value
			if name == "short" {
				if !op(v, "binary", "bool", "short.logical", "&&") || !ref(v.Left, name, "use", "short.condition") || !ex(v.Right, "parentheses", "bool", "short.parentheses") {
					return fail("short source structure")
				}
				compare := v.Right.Expression
				if !op(compare, "binary", "bool", "short.compare", ">") || !call(compare.Left, name) || !lit(compare.Right, "i32", "short.zero", "0") {
					return fail("short RHS comparison")
				}
			} else if !op(v, "binary", "i32", "numeric.operation", "+") || !call(v.Left, name) || !lit(v.Right, "i32", "numeric.one", "1") {
				return fail("numeric positive source operation")
			}
		}
	}
	return nil
}

func TestArrayGenericV8ValidUndefinedNumericFacts(t *testing.T) {
	raw, err := os.ReadFile("testdata/array-facts-v8/valid-undefined-numeric.ts")
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 595 || arrayFactsSHA(raw) != "fa359e55b9a6c435329f6ff155ea6a36f8d3771c1569cbce22d355f4be1e36c9" {
		t.Fatal("frozen complete original source")
	}
	source := string(raw)
	at := validNumericOriginal(t, source)
	request := genericArrayRequest25f(t, source)
	var out bytes.Buffer
	if err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(request), &out); err != nil {
		t.Fatal(err)
	}
	genericArrayCapture25f(t, request, source, out.Bytes())
	var packet arrayGenericPackage
	if err := json.Unmarshal(out.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	if packet.Family != "tsfg.array-facts-package-candidate" || packet.Version != 8 || packet.RequestID == "" || packet.PackageID == "" {
		t.Fatal("public package identity")
	}
	if err := validNumericContract(&packet.Payload, source, at); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{"operand-type", "as-target", "alias-symbol", "return-symbol", "selected-callee", "call-index", "if-condition", "binary-op", "unary-op", "short-op", "short-argument", "numeric-literal"} {
		t.Run(mutation, func(t *testing.T) {
			var p arrayGenericPackage
			if err := json.Unmarshal(out.Bytes(), &p); err != nil {
				t.Fatal(err)
			}
			f := map[string]*arrayGenericFunction{}
			for i := range p.Payload.Functions {
				f[p.Payload.Functions[i].Name] = &p.Payload.Functions[i]
			}
			switch mutation {
			case "operand-type":
				f["keep"].Body[0].Initializer.Expression.Type = "i32"
			case "as-target":
				f["keep"].Body[0].Initializer.Type = "undefined"
			case "alias-symbol":
				f["keep"].Body[1].Initializer.Binding = f["keep"].Parameters[0].ID
			case "return-symbol":
				f["keep"].Body[2].Value.Binding = f["keep"].Bindings[0].ID
			case "selected-callee":
				f["binary"].Body[1].Initializer.Callee = f["unary"].ID
			case "call-index":
				(*f["binary"].Body[1].Initializer.Arguments)[1].Token = "1"
			case "if-condition":
				f["binary"].Body[2].Condition.Binding = f["binary"].Bindings[1].ID
			case "binary-op":
				(*f["binary"].Body[2].Then)[0].Value.Operator = "-"
			case "unary-op":
				(*f["unary"].Body[2].Then)[0].Value.Operator = "+"
			case "short-op":
				f["short"].Body[1].Value.Operator = "||"
			case "short-argument":
				(*f["short"].Body[1].Value.Right.Expression.Left.Arguments)[0].Binding = f["short"].Parameters[0].ID
			case "numeric-literal":
				(*f["numeric"].Body[0].Initializer.Elements)[0].Token = "2"
			}
			if validNumericContract(&p.Payload, source, at) == nil {
				t.Fatal("copied source-fact mutation accepted")
			}
		})
	}
}
