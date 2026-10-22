// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	tsjson "github.com/microsoft/typescript-go/internal/json"
)

func TestArrayGenericV8ConstructorControlFlow25f(t *testing.T) {
	for _, test := range []struct {
		name, source string
		count        int
	}{
		{"branches", "class Base{constructor(public n:i32){}} class D extends Base{constructor(c:bool,n:i32){if(c){super(n);}else{super(n);}}} export function make(c:bool,n:i32):D{return new D(c,n);}", 2},
		{"bare-return", "class Base{n:i32=1;} class D extends Base{constructor(){super();return;}} export function make():D{return new D();}", 1},
		{"protected-readonly", "class Base{protected constructor(public readonly n:i32){}} class D extends Base{readonly x:i32;constructor(a:i32[],i:u32){super(a[i]);this.x=a[i];}} export function make(a:i32[],i:u32):D{return new D(a,i);}", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			types := genericConstructionTypes25f(t, genericArrayPublic25f(t, test.source))
			derived, base := types["D"], types["Base"]
			count := 0
			var visit func([]arrayGenericStatement)
			visit = func(statements []arrayGenericStatement) {
				for _, statement := range statements {
					if statement.Value != nil && statement.Value.Kind == "superConstruct" {
						genericSuperCall25f(t, statement.Value, base, test.source)
						count++
					}
					for _, children := range []*[]arrayGenericStatement{statement.Then, statement.Else, statement.Body} {
						if children != nil {
							visit(*children)
						}
					}
				}
			}
			visit(derived.Constructor.Body)
			if count != test.count {
				t.Fatal("actual super control-flow locations changed")
			}
			if test.name == "branches" && derived.Constructor.Body[0].Kind != "if" {
				t.Fatal("valid branch-local super was moved or collapsed")
			}
			if test.name == "protected-readonly" {
				if base.Construction.Access != "protected" || (*derived.Constructor.Body[0].Value.Arguments)[0].Kind != "checkedElementAccess" {
					t.Fatal("constructor access or checked super argument lost")
				}
			}
		})
	}
}

func TestArrayGenericV8ConstructorGenericForward25f(t *testing.T) {
	source := "@tsfgValue class V{n:i32=1;} class Box{n:i32=2;} class Base{constructor(public a:(i32|undefined)[],public b:Box|undefined,public v:V,public s:ReadSpan<string>){}} class Mid extends Base{} class Leaf extends Mid{} export function make(a:(i32|undefined)[],b:Box|undefined,v:V,s:ReadSpan<string>):Leaf{return new Leaf(a,b,v,s);}"
	types := genericConstructionTypes25f(t, genericArrayPublic25f(t, source))
	genericForward25f(t, types["Mid"], types["Base"])
	genericForward25f(t, types["Leaf"], types["Mid"])
	for _, name := range []string{"V", "Box"} {
		if types[name].Constructor != nil || types[name].Construction.Kind != "implicit-base" || types[name].Construction.ImplicitForward != nil {
			t.Fatal("default base must not invent an implementation")
		}
	}
	for _, proof := range types["Mid"].Construction.ImplicitForward.Parameters {
		if strings.HasPrefix(proof.Compatibility.SourceType, "array<") || strings.HasPrefix(proof.Compatibility.SourceType, "readSpan<") {
			if len(proof.Compatibility.Members) != 1 || proof.Compatibility.Members[0].Source != proof.Compatibility.Members[0].Target {
				t.Fatal("actual owned length member identity lost")
			}
		}
	}
}

func TestArrayGenericV8ConstructorMultiSource25f(t *testing.T) {
	source := "/*🧩*/ import { Base } from './base'; class D extends Base{constructor(n:i32){super(n);}} export function make(n:i32):D{return new D(n);}"
	base := "/*🧩*/ export class Base{constructor(public n:i32){}}"
	var request arrayFactsRequest
	if err := json.Unmarshal(genericArrayRequest25f(t, source), &request); err != nil {
		t.Fatal(err)
	}
	request.Input.Files = append([]arrayFactsInputFile{{Path: "/src/base.ts", Content: base, SHA256: arrayFactsSHA([]byte(base))}}, request.Input.Files...)
	input, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	raw := tsjson.Value(input)
	if err := raw.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	payload := genericArrayPublicRequest25f(t, append(raw, '\n'), source)
	types := genericConstructionTypes25f(t, payload)
	super := types["D"].Constructor.Body[0].Value
	genericSuperCall25f(t, super, types["Base"], source)
	if len(payload["files"].([]any)) != 3 || super.Signature.SourcePath != "/src/base.ts" || super.Signature.DeclarationRange.Start != 25 || types["Base"].Construction.Parameters[0].Origin.Source != "/src/base.ts" {
		t.Fatalf("original cross-source UTF16 constructor signature lost: %+v", super.Signature)
	}
}

func TestArrayGenericV8ConstructorBoundaries25f(t *testing.T) {
	for _, test := range []struct {
		name, source string
		official     bool
	}{
		{"typed-number", "class Base{constructor(public n:i32){}} class D extends Base{constructor(n:f64){super(n);}} export function f(n:f64):D{return new D(n);}", false},
		{"missing-super", "class Base{n:i32=1;} class D extends Base{constructor(){}} export function f():D{return new D();}", true},
		{"early-this", "class Base{n:i32=1;} class D extends Base{constructor(){const x:i32=this.n;super();}} export function f():D{return new D();}", true},
		{"private-base", "class Base{private constructor(public n:i32){}} class D extends Base{constructor(n:i32){super(n);}} export function f(n:i32):D{return new D(n);}", true},
		{"void-return", "class Base{constructor(public n:i32){}} class D extends Base{constructor(n:i32){return super(n);}} export function make(n:i32):D{return new D(n);}", true},
		{"native-before-parameter", "class C{copy:i32=this.n;constructor(public n:i32){}} export function make(n:i32):C{return new C(n);}", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(genericArrayRequest25f(t, test.source)), &out)
			var failure *arrayFactsFailure
			if !errors.As(err, &failure) || out.Len() != 0 {
				t.Fatalf("invalid constructor accepted: %v", err)
			}
			if test.official {
				if len(failure.Diagnostics) == 0 || failure.Diagnostics[0].UpstreamCode == 0 {
					t.Fatalf("official constructor diagnostic swallowed: %v", err)
				}
			} else if failure.Code != "r01-d-scalar-type" {
				t.Fatalf("typed numeric constructor argument converted: %v", err)
			}
		})
	}
	t.Run("bounded-chain", func(t *testing.T) {
		source := "class C0{constructor(n:i32){}} "
		for i := 1; i <= 65; i++ {
			source += fmt.Sprintf("class C%d extends C%d{} ", i, i-1)
		}
		source += "export function f(a:C65):void{}"
		genericArrayRelationReject25f(t, source, "r03-array-facts-limit")
	})
	for _, canceled := range []bool{false, true} {
		t.Run(fmt.Sprintf("memo-boundary-%v", canceled), func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if canceled {
				cancel()
			}
			p := &arrayGenericProjection{ctx: ctx, annotationCalls: 100000, types: map[string]arrayGenericType{"R": {Construction: &arrayGenericConstruction{}}}}
			err := p.constructorMetadata("R", nil, 0)
			var failure *arrayFactsFailure
			if canceled {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("memoized constructor ignored cancel: %v", err)
				}
			} else if !errors.As(err, &failure) || failure.Code != "r03-array-facts-limit" {
				t.Fatalf("memoized constructor ignored work: %v", err)
			}
		})
	}
}
