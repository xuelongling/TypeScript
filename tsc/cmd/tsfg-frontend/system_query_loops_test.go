// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"github.com/microsoft/typescript-go/internal/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func systemLoopRequest38d(t *testing.T) request {
	t.Helper()
	r := systemRequest38c(t)
	source, e := os.ReadFile("testdata/system-query-v2/architecture.ts")
	if e != nil {
		t.Fatal(e)
	}
	r.Files[0].Content = string(source)
	return r
}
func TestSystemQueryLoopArchitecture38d(t *testing.T) {
	r := systemLoopRequest38d(t)
	p, _, _, failure := newOwnedSystemProgram(r)
	if failure != nil {
		t.Fatal(failure)
	}
	if codes := systemDiagnosticCodes38c(t, p); len(codes) != 0 {
		t.Fatal("complete architecture loop requires official diagnostics0", codes)
	}
}
func TestSystemQueryLoopTyping38d(t *testing.T) {
	for _, name := range []string{"readonly-write", "wrong-field", "ordinary", "ordinary-marker", "shadow-query", "duplicate-binding", "nested-readonly"} {
		t.Run(name, func(t *testing.T) {
			r := systemLoopRequest38d(t)
			code := 0
			switch name {
			case "readonly-write":
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "p.x += v.x * dt", "v.x += p.x * dt", 1)
				code = 2540
			case "wrong-field":
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "p.x +=", "p.z +=", 1)
				code = 2339
			case "ordinary":
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "@system\n", "", 1)
			case "ordinary-marker":
				code = 1206
			case "shadow-query":
				r.Files[0].Content = "interface Query<T extends unknown[]>{readonly __query:T;}\n" + r.Files[0].Content
				code = 2488
			case "duplicate-binding":
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "[p,v]", "[p,p]", 1)
				code = 2451
			case "nested-readonly":
				r.Files[0].Content = "@tsfgValue class Vec {x:f32=0;}\n" + strings.Replace(r.Files[0].Content, "class Velocity { x: f32 = 0; y: f32 = 0; }", "class Velocity { x: f32 = 0; y: f32 = 0; declare nested:Vec; }", 1)
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "p.x += v.x * dt", "v.nested.x += p.x * dt", 1)
				code = 2540
			}
			p, _, _, failure := newOwnedProgram(r)
			if name != "ordinary" && name != "ordinary-marker" {
				p, _, _, failure = newOwnedSystemProgram(r)
			}
			if failure != nil {
				t.Fatal(failure)
			}
			codes := systemDiagnosticCodes38c(t, p)
			if code == 0 {
				if len(codes) != 0 {
					t.Fatal("ordinary TS body changed", codes)
				}
			} else {
				found := false
				for _, actual := range codes {
					if actual == code {
						found = true
					}
				}
				if !found {
					t.Fatal("official rejection absent", code, codes)
				}
			}
		})
	}
}
func TestSystemQueryLoopPublicFacts38d(t *testing.T) {
	for _, name := range []string{"loop-architecture", "loop-aliases", "loop-multiple", "loop-shared-component"} {
		t.Run(name, func(t *testing.T) {
			r := systemLoopRequest38d(t)
			switch name {
			case "loop-aliases":
				r.Files[0].Content = "import type {Q,Modes} from './aliases';\n" + strings.Replace(r.Files[0].Content, "Query<[Write<Position>, Read<Velocity>]>", "Q<Modes<Position>>", 1)
				r.Files = append(r.Files, inputFile{Path: "/src/aliases.ts", Content: "import type {Velocity} from './main';\nexport type W<T> = Write<T>;\nexport type R<T> = Read<T>;\nexport type Modes<T> = [W<T>,R<Velocity>];\nexport type Q<T extends unknown[]> = Query<T>;\n"})
			case "loop-shared-component":
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "Read<Velocity>", "Read<Position>", 1)
			case "loop-multiple":
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "[Write<Position>, Read<Velocity>]", "[Write<Position>, Read<Velocity>, Read<Acceleration>]", 1)
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "[p,v]", "[p,v,a]", 1)
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "p.y += v.y", "p.y += a.y", 1)
				r.Files[0].Content += "\n@component class Acceleration {x:f32=0;y:f32=0;}\n"
			}
			raw := systemPublicRequest38c(t, r)
			var out bytes.Buffer
			if e := runFrontendCommand(t.Context(), []string{"--system-query-facts-io-v3"}, bytes.NewReader(raw), &out); e != nil {
				t.Fatal(e)
			}
			var packet systemQueryPackage
			if e := json.Unmarshal(out.Bytes(), &packet); e != nil {
				t.Fatal(e)
			}
			if len(packet.Payload.Systems) != 1 || len(packet.Payload.Systems[0].Loops) != 1 {
				t.Fatal("whole system loop inventory differs")
			}
			loop := packet.Payload.Systems[0].Loops[0]
			expectedBindings := 2
			if name == "loop-multiple" {
				expectedBindings = 3
			}
			if len(loop.Bindings) != expectedBindings || len(loop.Statements) != 2 {
				t.Fatal("actual tuple/statement inventory differs")
			}
			for _, statement := range loop.Statements {
				value := statement.Value
				if statement.Kind != "expression" || value.Kind != "fieldAssignment" || value.Type != "f32" || value.Operator != "+=" || value.Value.Kind != "binary" || value.Value.Operator != "*" {
					t.Fatal("actual field/f32 arithmetic facts differ")
				}
			}
			if capture := os.Getenv("TSFG_QUERY38_CAPTURE"); capture != "" {
				if e := os.MkdirAll(capture, 0700); e != nil {
					t.Fatal(e)
				}
				if e := os.WriteFile(filepath.Join(capture, name+"-request.json"), raw, 0600); e != nil {
					t.Fatal(e)
				}
				if e := os.WriteFile(filepath.Join(capture, name+"-facts.json"), out.Bytes(), 0600); e != nil {
					t.Fatal(e)
				}
			}
			t.Logf("whole original Program bytes=%d nodes=%d types=%d loopBindings=%d statements=%d", out.Len(), len(packet.Payload.Nodes), len(packet.Payload.Types), len(loop.Bindings), len(loop.Statements))
		})
	}
}
func TestSystemQueryLoopBoundary38d(t *testing.T) {
	for _, name := range []string{"foreign-iterator", "foreign-wrapper", "incomplete-tuple", "shadowed-loop", "readonly-write", "wrong-field"} {
		t.Run(name, func(t *testing.T) {
			r := systemLoopRequest38d(t)
			switch name {
			case "foreign-iterator":
				r.Files[0].Content = "interface Query<T extends unknown[]>{readonly __query:T;[Symbol.iterator]():SystemQueryIterator<[Position,Velocity]>;}\n" + r.Files[0].Content
			case "foreign-wrapper":
				r.Files[0].Content = "interface Read<T>{readonly __read:T;}\n" + r.Files[0].Content
			case "incomplete-tuple":
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "[p,v]", "[p]", 1)
				r.Files[0].Content = strings.ReplaceAll(r.Files[0].Content, "v.x", "p.x")
				r.Files[0].Content = strings.ReplaceAll(r.Files[0].Content, "v.y", "p.y")
			case "shadowed-loop":
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "for(const [p,v] of q)", "for(const [q,v] of q)", 1)
			case "readonly-write":
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "p.x += v.x * dt", "v.x += p.x * dt", 1)
			case "wrong-field":
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "p.x +=", "p.z +=", 1)
			}
			raw := systemPublicRequest38c(t, r)
			var out bytes.Buffer
			e := runFrontendCommand(t.Context(), []string{"--system-query-facts-io-v3"}, bytes.NewReader(raw), &out)
			if e == nil || out.Len() != 0 {
				t.Fatal("invalid loop emitted facts", e, out.Len())
			}
			t.Logf("original publicIO rejection: %v", e)
		})
	}
}
