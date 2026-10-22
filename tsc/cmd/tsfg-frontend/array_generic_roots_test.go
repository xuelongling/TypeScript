// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArrayGenericV8MultipleRoots25f(t *testing.T) {
	for _, test := range []struct{ name, source string }{
		{"union-slot", "class Base{n:i32=1;} class Left extends Base{l:i32=2;} class Right extends Base{r:i32=3;} export function f(x:Left|Right,i:u32):Base{const a:Base[]=[x];return a[i];}"},
		{"parentheses-slot", "class Base{n:i32=1;} class Left extends Base{l:i32=2;} class Right extends Base{r:i32=3;} export function f(x:Left|Right,i:u32):Base{const a:Base[]=[(x)];return a[i];}"},
		{"presence-only", "class A{n:i32=1;} class B{m:i32=2;} export function f(a:(A|B|undefined)[],i:u32):A|B{return (a as(A|B)[])[i];}"},
		{"identity-subset", "class A{n:i32=1;} class B{m:i32=2;} export function f(a:(A|B|undefined)[],i:u32):A{return (a as A[])[i];}"},
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
			p := genericArrayPublicRequest25f(t, input, test.source)
			if test.name == "union-slot" || test.name == "parentheses-slot" {
				fn := genericFindFunction25f(t, p, "f")
				element := fn["body"].([]any)[0].(map[string]any)["initializer"].(map[string]any)["elements"].([]any)[0].(map[string]any)
				if len(element["compatibilityRoots"].([]any)) != 2 {
					t.Fatal("ordinary union source lost an authoritative leaf root")
				}
				if test.name == "parentheses-slot" {
					inner := element["expression"].(map[string]any)
					if len(inner["compatibilityRoots"].([]any)) != 2 || len(p["compatibilityWitnesses"].([]any)) != 4 {
						t.Fatal("parentheses inner and outer require independent actual occurrence roots")
					}
				}
			} else if len(p["compatibilityWitnesses"].([]any)) != 0 {
				t.Fatal("identity selection invented structural proofs")
			}
		})
	}
}

func TestArrayGenericV8RootContexts25f(t *testing.T) {
	prefix := "class Base{n:i32=1;} class Left extends Base{l:i32=2;} class Right extends Base{r:i32=3;} "
	for _, source := range []string{
		prefix + "export function f(x:Left|Right):Base{return (x);}",
		prefix + "function pass(x:Base):Base{return x;} export function f(x:Left|Right):Base{return pass(x);}",
		prefix + "export function f(a:(Left|Right)[]):Base[]{return a;}",
		"class A{n:i32=1;} class B{m:i32=2;} class C{k:i32=3;} export function f(a:A|B):A|B|C{return a as A|B|C;}",
		"class A{n:i32=1;} class B{m:i32=2;} export function f(a:A|B):A{return a as A;}",
		"@tsfgValue class V{n:i32=1;} @tsfgValue class W{m:i32=2;} export function f(a:(V|W|undefined)[],i:u32):V{return (a as V[])[i];}",
		"class A{n:i32=1;} class B{n:i32=2;} class C{n:i32=3;} export function f(a:A[],i:u32):C{return a[i] as B;}",
	} {
		p := genericArrayPublic25f(t, source)
		if strings.Contains(source, "return a[i] as B;") {
			cast := genericReturn25f(t, p, "f")
			if len(cast["compatibilityRoots"].([]any)) != 2 || len(p["compatibilityWitnesses"].([]any)) != 2 {
				t.Fatal("as and contextual views must both be explicit at their actual node")
			}
		}
	}
}

func TestArrayGenericV8RootTargetCandidate25f(t *testing.T) {
	for _, field := range []string{"l", "r"} {
		t.Run(field, func(t *testing.T) {
			prefix := "class Base{n:i32=1;} class Left extends Base{l:i32=2;} class Right extends Base{r:i32=3;} class OnlyLeftView{" + field + ":i32=4;} "
			seed := "Left      "
			if field == "r" {
				seed = "Right     "
			}
			for _, sourceType := range []string{"Left|Right", seed} {
				p := genericArrayPublic25f(t, prefix+"export function f(a:("+sourceType+")[]):OnlyLeftView[]|Base[]{return a;}")
				types, ids := map[string]string{}, map[string]string{}
				for _, raw := range p["types"].([]any) {
					typ := raw.(map[string]any)
					if name, ok := typ["name"].(string); ok {
						types[typ["id"].(string)], ids[name] = name, typ["id"].(string)
					}
				}
				if ids["Right"] >= ids["Left"] || ids["OnlyLeftView"] >= ids["Base"] {
					t.Fatal("original candidate/source traversal order changed")
				}
				roots := genericReturn25f(t, p, "f")["compatibilityRoots"].([]any)
				wantTarget, wantCount := "Base", 2
				if sourceType == seed {
					wantTarget, wantCount = "OnlyLeftView", 1
				}
				if len(roots) != wantCount {
					t.Fatal("missing actual authoritative selected root")
				}
				for _, raw := range roots {
					pair := raw.(map[string]any)
					if types[pair["targetType"].(string)] != wantTarget {
						t.Fatal("failed whole-array union candidate leaked a partial root")
					}
				}
			}
		})
	}
}

func genericRootProof25f(t *testing.T, p, expression map[string]any) map[string]any {
	t.Helper()
	roots, ok := expression["compatibilityRoots"].([]any)
	if !ok || len(roots) != 1 {
		t.Fatalf("expected one actual explicit root: %v", expression)
	}
	root := roots[0].(map[string]any)
	where := expression["range"].(map[string]any)
	for _, raw := range p["compatibilityWitnesses"].([]any) {
		proof := raw.(map[string]any)["compatibility"].(map[string]any)
		origin := proof["origin"].(map[string]any)
		if proof["sourceType"] == root["sourceType"] && proof["targetType"] == root["targetType"] && origin["source"] == "/src/main.ts" && origin["start"] == where["start"] && origin["end"] == where["end"] {
			return proof
		}
	}
	t.Fatal("explicit expression root has no same-origin directory record")
	return nil
}

func TestArrayGenericV8InheritedReceiverRoots25f(t *testing.T) {
	p := genericArrayPublic25f(t, "class Base{m():i32{return 1;}} class D extends Base{x:i32=2;} export function f(a:D[],i:u32):i32{return a[i].m();}")
	call := genericReturn25f(t, p, "f")
	receiver := call["receiver"].(map[string]any)
	proof := genericRootProof25f(t, p, receiver)
	if receiver["kind"] != "checkedElementAccess" || proof["sourceType"] != receiver["flowType"] {
		t.Fatal("inherited receiver lost checked source or original relation")
	}
	for _, raw := range p["functions"].([]any) {
		fn := raw.(map[string]any)
		if fn["id"] == call["callee"] && fn["ownerType"] != proof["targetType"] {
			t.Fatal("receiver proof must join actual resolved method owner")
		}
	}
}
