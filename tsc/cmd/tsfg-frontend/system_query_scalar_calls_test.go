// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"fmt"
	"github.com/microsoft/typescript-go/internal/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func systemScalarRequest38g(t *testing.T) request {
	t.Helper()
	r := systemRequest38c(t)
	source, e := os.ReadFile("testdata/system-query-v3/scalar-helper.ts")
	if e != nil {
		t.Fatal(e)
	}
	r.Files[0].Content = string(source)
	return r
}
func systemScalarRun38g(t *testing.T, r request, name string) (systemQueryPackage, error, int) {
	t.Helper()
	raw := systemPublicRequest38c(t, r)
	capture := os.Getenv("TSFG_QUERY38_CAPTURE")
	if capture != "" {
		if e := os.MkdirAll(capture, 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(capture, name+"-request.json"), raw, 0600); e != nil {
			t.Fatal(e)
		}
	}
	var output bytes.Buffer
	if e := runFrontendCommand(t.Context(), []string{"--system-query-facts-io-v3"}, bytes.NewReader(raw), &output); e != nil {
		return systemQueryPackage{}, e, output.Len()
	}
	var packet systemQueryPackage
	if e := json.Unmarshal(output.Bytes(), &packet); e != nil {
		t.Fatal(e)
	}
	if capture != "" {
		if e := os.WriteFile(filepath.Join(capture, name+"-facts.json"), output.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
	}
	return packet, nil, output.Len()
}
func TestSystemQueryScalarHelperPublic38g(t *testing.T) {
	for _, name := range []string{"direct", "imported"} {
		t.Run(name, func(t *testing.T) {
			r := systemScalarRequest38g(t)
			expectedHelpers := 1
			if name == "imported" {
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "function scaled(v:f32,dt:f32):f32{return v*dt;}\n", "", 1)
				r.Files[0].Content = "import {scaled as gain} from './helpers';\n" + strings.Replace(r.Files[0].Content, "scaled(v.x, dt)", "gain(v.x, dt)", 1)
				helper, e := os.ReadFile("testdata/system-query-v3/scalar-imported.ts")
				if e != nil {
					t.Fatal(e)
				}
				r.Files = append(r.Files, inputFile{Path: "/src/helpers.ts", Content: string(helper)})
				expectedHelpers = 2
			}
			official, _, _, failure := newOwnedSystemProgram(r)
			if failure != nil {
				t.Fatal(failure)
			}
			if codes := systemDiagnosticCodes38c(t, official); len(codes) != 0 {
				t.Fatal("complete helper Source diagnostics", codes)
			}
			packet, e, _ := systemScalarRun38g(t, r, name)
			if e != nil {
				t.Fatal(e)
			}
			if len(packet.Payload.ScalarHelpers) != expectedHelpers {
				t.Fatal("reachable helper inventory differs")
			}
			if len(packet.Payload.Program.Files) != len(r.Files) {
				t.Fatal("whole original Program was pruned")
			}
			call := packet.Payload.Systems[0].Loops[0].Statements[0].Value.Value
			if call.Kind != "call" || call.Type != "f32" || call.Arguments == nil || len(*call.Arguments) != 2 || (*call.Arguments)[0].Kind != "fieldAccess" || (*call.Arguments)[1].Binding != packet.Payload.Systems[0].Parameters[1].Binding.ID {
				t.Fatal("original argument order or scalar call identity lost")
			}
			for index, helper := range packet.Payload.ScalarHelpers {
				if helper.Declaration.Name == "unused" || helper.ReturnType != "f32" || index > 0 && packet.Payload.ScalarHelpers[index-1].ID >= helper.ID {
					t.Fatal("selected helper closure includes unrelated function or unsorted duplicate")
				}
				if helper.ID == call.Callee && helper.Signature.ID != call.Signature.ID {
					t.Fatal("call selected signature differs")
				}
			}
			t.Logf("actual whole publicv3 helpers=%d files=%d nodes=%d", len(packet.Payload.ScalarHelpers), len(packet.Payload.Program.Files), len(packet.Payload.Nodes))
		})
	}
}
func TestSystemQueryScalarHelperNegatives38g(t *testing.T) {
	for _, name := range []string{"captured-global", "global-mutation", "reference-parameter", "i32-parameter", "f64-return", "indirect", "allocation", "throw", "recursion", "cached-depth33", "inventory65", "arguments33"} {
		t.Run(name, func(t *testing.T) {
			r := systemScalarRequest38g(t)
			original := "function scaled(v:f32,dt:f32):f32{return v*dt;}"
			replacement := original
			expected := ""
			switch name {
			case "captured-global":
				replacement = "let global:f32=1;function scaled(v:f32,dt:f32):f32{return v*global;}"
				expected = "original loop binding or f32 parameter"
			case "global-mutation":
				replacement = "let global:f32=1;function scaled(v:f32,dt:f32):f32{global+=v;return v*dt;}"
				expected = "one original pure scalar return"
			case "reference-parameter":
				replacement = "function scaled(v:Position,dt:f32):f32{return v.x*dt;}"
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "scaled(v.x, dt)", "scaled(p, dt)", 1)
				expected = "native f32 annotation"
			case "i32-parameter":
				replacement = "function scaled(v:i32,dt:f32):f32{return v*dt;}"
				expected = "native f32 annotation"
			case "f64-return":
				replacement = "function scaled(v:f32,dt:f32):f64{return v*dt;}"
				expected = "native f32 return"
			case "indirect":
				replacement = original + "const redirect=scaled;"
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "scaled(v.x, dt)", "redirect(v.x, dt)", 1)
				expected = "original named helper declaration"
			case "allocation":
				replacement = "function scaled(v:f32,dt:f32):f32{return (new Box()).value*dt;}"
				expected = "direct Query component field access"
			case "throw":
				replacement = "function scaled(v:f32,dt:f32):f32{throw v;}"
				expected = "one original pure scalar return"
			case "recursion":
				replacement = "function scaled(v:f32,dt:f32):f32{return scaled(v,dt);}"
				expected = "recursive Query scalar helper graph"
			case "cached-depth33":
				replacement = "function f0(v:f32):f32{return v;}\n"
				for i := 1; i <= 31; i++ {
					replacement += fmt.Sprintf("function f%d(v:f32):f32{return f%d(v);}\n", i, i-1)
				}
				replacement += "function scaled(v:f32,dt:f32):f32{return f31(v)*dt;}"
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "p.y += v.y * dt;", "p.y += f0(v.y);", 1)
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "p.x += scaled(v.x, dt);", "p.x += f0(v.x); p.x += scaled(v.x, dt);", 1)
				expected = "exceeds32"
			case "inventory65":
				terms := []string{}
				replacement = ""
				for i := 0; i < 64; i++ {
					replacement += fmt.Sprintf("function leaf%d(v:f32):f32{return v;}\n", i)
					terms = append(terms, fmt.Sprintf("leaf%d(v)", i))
				}
				for len(terms) > 1 {
					next := []string{}
					for i := 0; i < len(terms); i += 2 {
						if i+1 < len(terms) {
							next = append(next, "("+terms[i]+"+"+terms[i+1]+")")
						} else {
							next = append(next, terms[i])
						}
					}
					terms = next
				}
				replacement += "function scaled(v:f32,dt:f32):f32{return " + terms[0] + ";}"
				expected = "inventory/depth limit"
			case "arguments33":
				params, args := []string{}, []string{}
				for i := 0; i < 33; i++ {
					params = append(params, fmt.Sprintf("v%d:f32", i))
					args = append(args, "v.x")
				}
				replacement = "function scaled(" + strings.Join(params, ",") + "):f32{return v0;}"
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "scaled(v.x, dt)", "scaled("+strings.Join(args, ",")+")", 1)
				expected = "at most32"
			}
			r.Files[0].Content = strings.Replace(r.Files[0].Content, original, replacement, 1)
			official, _, _, failure := newOwnedSystemProgram(r)
			if failure != nil {
				t.Fatal(failure)
			}
			if codes := systemDiagnosticCodes38c(t, official); len(codes) != 0 {
				t.Fatal("boundary must use checker-valid original Source", codes)
			}
			_, e, partial := systemScalarRun38g(t, r, "negative-"+name)
			if e == nil || partial != 0 || !strings.Contains(e.Error(), expected) {
				t.Fatal("specific atomic capability refusal absent", expected, e, partial)
			}
			t.Logf("actual capability refusal: %v", e)
		})
	}
}
