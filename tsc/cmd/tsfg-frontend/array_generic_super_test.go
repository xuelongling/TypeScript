// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"unicode/utf16"
)

func genericConstructionTypes25f(t *testing.T, payload map[string]any) map[string]arrayGenericType {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var typed arrayGenericPayload
	if err := json.Unmarshal(raw, &typed); err != nil {
		t.Fatal(err)
	}
	types := map[string]arrayGenericType{}
	for _, typ := range typed.Types {
		if typ.Kind != "reference" && typ.Kind != "value" {
			continue
		}
		types[typ.Name] = typ
		meta := typ.Construction
		if meta == nil || meta.ID != typ.ID+".ctor" || meta.Signature.ReturnType != typ.ID || meta.Origin.Source != typ.SourcePath {
			t.Fatalf("constructor metadata must join original owner: %+v", typ)
		}
		if len(meta.Parameters) != len(meta.Signature.Parameters) {
			t.Fatal("construct signature arity lost")
		}
		for i, param := range meta.Parameters {
			if param.Type != meta.Signature.Parameters[i] || param.Origin.Source == "" || param.Origin.End <= param.Origin.Start {
				t.Fatal("original constructor parameter provenance lost")
			}
			if typ.Constructor != nil && (param.Symbol != typ.Constructor.Parameters[i].ID || param.InheritedFrom != "") {
				t.Fatal("explicit parameter binding changed")
			}
		}
		if typ.Constructor != nil && (meta.Kind != "explicit" || meta.ImplicitForward != nil || meta.Signature.DeclarationRange != typ.Constructor.DeclarationRange) {
			t.Fatal("explicit signature must retain its actual declaration")
		}
		if len(meta.InitializationFields) != len(*typ.Fields) {
			t.Fatal("own field initialization coverage lost")
		}
		seen := map[string]bool{}
		for _, id := range meta.InitializationFields {
			if seen[id] {
				t.Fatal("own field initialized twice")
			}
			seen[id] = true
		}
		for _, field := range *typ.Fields {
			if !seen[field.ID] {
				t.Fatal("inherited or missing own initializer")
			}
		}
		wantPhase := "before-body"
		if typ.Base != "" {
			wantPhase = "after-successful-super"
		}
		if meta.InitializationPhase != wantPhase {
			t.Fatal("base/derived initialization phase changed")
		}
	}
	return types
}

func genericSuperCall25f(t *testing.T, value *arrayGenericExpression, base arrayGenericType, source string) {
	t.Helper()
	if value == nil || value.Kind != "superConstruct" || value.Callee != base.ID || value.Type != "void" || value.FlowType != "void" || value.Receiver != nil || value.AllocationSite != "" || !reflect.DeepEqual(value.Signature, &base.Construction.Signature) || value.Arguments == nil {
		t.Fatalf("super must use direct base/current receiver/void result: %+v", value)
	}
	units := utf16.Encode([]rune(source))
	if value.CalleeRange == nil || string(utf16.Decode(units[value.CalleeRange.Start:value.CalleeRange.End])) != "super" {
		t.Fatal("super UTF16 anchor lost")
	}
}

func genericForward25f(t *testing.T, derived, base arrayGenericType) {
	t.Helper()
	meta, parent := derived.Construction, base.Construction
	forward := meta.ImplicitForward
	if derived.Constructor != nil || meta.Kind != "implicit-derived" || forward == nil || forward.Target != parent.ID || forward.BaseSignature.Return != base.ID || !reflect.DeepEqual(forward.BaseSignature.Parameters, parent.Parameters) || len(forward.Parameters) != len(parent.Parameters) || forward.Origin != meta.Origin {
		t.Fatal("implicit forwarding must retain actual parent signature without fake AST")
	}
	for i, param := range meta.Parameters {
		edge := forward.Parameters[i]
		original := parent.Parameters[i]
		if param.Origin != original.Origin || param.InheritedFrom != original.Symbol || edge.Source != param.Symbol || edge.Target != original.Symbol || !edge.Compatibility.Assignable || edge.Compatibility.SourceType != param.Type || edge.Compatibility.TargetType != original.Type || edge.Compatibility.Origin != meta.Origin {
			t.Fatalf("actual inherited parameter edge lost: %+v", edge)
		}
	}
}

func TestArrayGenericV8SuperConstructPublic25f(t *testing.T) {
	for _, test := range []struct{ name, source string }{
		{"original", "class Base{constructor(public n:i32){}} class Left extends Base{constructor(n:i32,public l:i32){super(n);}} class Right extends Base{constructor(n:i32,public r:i32){super(n);}} export function f(x:Left|Right,i:u32):Base{const a:Base[]=[x];return a[i];}"},
		{"order", "function tick(a:i32[],n:i32):i32{a[0]+=n;return a[0];} class Base{b:i32=1;constructor(public log:i32[],public n:i32){tick(log,10);}} class Derived extends Base{d:i32=tick(this.log,100);constructor(log:i32[],public tag:i32){const before:i32=tick(log,1);super(log,tick(log,2));tick(log,1000);}} export function make(log:i32[]):Derived{return new Derived(log,3);}"},
		{"inherited", "class Root{constructor(public n:i32){}} class Mid extends Root{} class Leaf extends Mid{constructor(n:i32){super(n);}} export function make(n:i32):Leaf{return new Leaf(n);}"},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := genericArrayRequest25f(t, test.source)
			if root := os.Getenv("TSFG_ARRAY_V8_CAPTURE_ROOT"); root != "" {
				if err := os.MkdirAll(root, 0700); err != nil {
					t.Fatal(err)
				}
				for name, data := range map[string][]byte{"source.ts": []byte(test.source), "request.json": input} {
					if err := os.WriteFile(filepath.Join(root, test.name+"-"+name), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			types := genericConstructionTypes25f(t, genericArrayPublicRequest25f(t, input, test.source))
			switch test.name {
			case "original":
				for _, name := range []string{"Left", "Right"} {
					genericSuperCall25f(t, types[name].Constructor.Body[0].Value, types["Base"], test.source)
				}
			case "order":
				base, derived := types["Base"], types["Derived"]
				for _, typ := range []arrayGenericType{base, derived} {
					want := []string{}
					// owned_program pins ESNext: native ordinary initializers run
					// before the body assignments for parameter-properties.
					for _, parameter := range []bool{false, true} {
						for _, field := range *typ.Fields {
							if (field.ParameterBinding != "") == parameter {
								want = append(want, field.ID)
							}
						}
					}
					if !reflect.DeepEqual(typ.Construction.InitializationFields, want) {
						t.Fatal("ESNext native ordinary initializers must precede parameter-property body assignments")
					}
				}
				body := derived.Constructor.Body
				if len(body) != 3 || body[0].Initializer.Kind != "call" || (*body[0].Initializer.Arguments)[1].Decimal != "1" || (*body[2].Value.Arguments)[1].Decimal != "1000" {
					t.Fatal("pre-super/post-super statement order lost")
				}
				genericSuperCall25f(t, body[1].Value, base, test.source)
				args := *body[1].Value.Arguments
				if len(args) != 2 || args[0].Kind != "bindingReference" || args[1].Kind != "call" || (*args[1].Arguments)[1].Decimal != "2" {
					t.Fatal("super argument evaluation tree/order changed")
				}
				if (*(*derived.Fields)[0].Initializer.Arguments)[1].Decimal != "100" {
					t.Fatal("post-super field initializer disappeared")
				}
			case "inherited":
				genericForward25f(t, types["Mid"], types["Root"])
				genericSuperCall25f(t, types["Leaf"].Constructor.Body[0].Value, types["Mid"], test.source)
			}
		})
	}
}
