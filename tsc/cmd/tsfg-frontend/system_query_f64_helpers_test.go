// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/json"
)

func systemF64ScalarRequest38n(t *testing.T) request {
	t.Helper()
	r := systemF64Request38n(t, "p.x += scaled(v.x, dt)")
	r.Files[0].Content = strings.Replace(r.Files[0].Content, "function scaled(v:f32,dt:f32):f32", "function scaled(v:f64,dt:f64):f64", 1)
	return r
}

func systemF64ViewRequest38n(t *testing.T, imported bool) request {
	t.Helper()
	r := systemViewRequest38h(t)
	r.Files[0].Content = strings.ReplaceAll(r.Files[0].Content, ":f32", ":f64")
	r.Files[0].Content = strings.Replace(r.Files[0].Content, "return readX(p)*dt;", "return readX(p)-dt;", 1)
	r.Files[0].Content = strings.Replace(r.Files[0].Content, "return p.x += v.x*dt;", "return p.x += v.x/dt;", 1)
	if imported {
		first := strings.Index(r.Files[0].Content, "function readX(")
		last := strings.Index(r.Files[0].Content, "@system")
		helpers := r.Files[0].Content[first:last]
		r.Files[0].Content = "import {update as advance,scaledX as sample} from './helpers';\n" + r.Files[0].Content[:first] + r.Files[0].Content[last:]
		r.Files[0].Content = strings.ReplaceAll(strings.ReplaceAll(r.Files[0].Content, "update(p,v,dt)", "advance(p,v,dt)"), "scaledX(p,dt)", "sample(p,dt)")
		helpers = strings.ReplaceAll(strings.ReplaceAll(helpers, "function scaledX(", "export function scaledX("), "function update(", "export function update(")
		r.Files = append(r.Files, inputFile{Path: "/src/helpers.ts", Content: "import type {Position,Velocity} from './main';\n" + helpers})
	}
	return r
}

func TestSystemQueryF64ViewHelpers38n(t *testing.T) {
	for _, name := range []string{"f64-view-helpers", "f64-view-imported"} {
		t.Run(name, func(t *testing.T) {
			r := systemF64ViewRequest38n(t, name == "f64-view-imported")
			official, _, consumed, failure := newOwnedSystemProgram(r)
			if failure != nil || len(consumed) != len(r.Files) {
				t.Fatal("complete original view Program differs", failure, consumed)
			}
			if codes := systemDiagnosticCodes38c(t, official); len(codes) != 0 {
				t.Fatal("official f64 view helper diagnostics", codes)
			}
			packet, e, partial := systemScalarRun38g(t, r, name)
			if e != nil {
				t.Fatal("actual v5 f64 view helper rejected", e, partial)
			}
			if packet.Version != 5 || len(packet.Payload.Program.Files) != len(r.Files) || len(packet.Payload.ScalarHelpers) != 3 {
				t.Fatal("whole Program/selected helper inventory differs")
			}
			helperIDs := map[string]string{}
			for _, helper := range packet.Payload.ScalarHelpers {
				helperIDs[helper.Declaration.Name] = helper.ID
			}
			for _, helper := range packet.Payload.ScalarHelpers {
				if helper.ReturnType != "f64" || helper.Signature.ReturnType != "f64" || helper.Value.Type != "f64" {
					t.Fatal("original f64 signature/result differs")
				}
				for index, parameter := range helper.Parameters {
					if parameter.Index != index || parameter.Binding.Kind != "KindParameter" || parameter.LogicalType != helper.Signature.Parameters[index] || parameter.View == nil && parameter.LogicalType != "f64" {
						t.Fatal("original ordered formal parameter differs")
					}
				}
				if name == "f64-view-imported" && helper.Declaration.SourcePath != "/src/helpers.ts" {
					t.Fatal("named import resolved wrong original declaration")
				}
				if helper.Declaration.Name == "readX" && (helper.Value.Kind != "fieldAccess" || helper.Value.Receiver.Binding != helper.Parameters[0].Binding.ID || helper.Parameters[0].View.Access.Mode != "read") {
					t.Fatal("original Read field binding differs")
				}
				if helper.Declaration.Name == "scaledX" && (helper.Value.Kind != "binary" || helper.Value.Operator != "-" || helper.Value.Left.Kind != "call" || helper.Value.Left.Type != "f64" || helper.Value.Left.Callee != helperIDs["readX"] || helper.Value.Right.Binding != helper.Parameters[1].Binding.ID) {
					t.Fatal("original nested call and arithmetic order differs")
				}
				if helper.Declaration.Name == "update" && (helper.Value.Kind != "fieldAssignment" || helper.Value.Operator != "+=" || helper.Value.Target.Receiver.Binding != helper.Parameters[0].Binding.ID || helper.Parameters[0].View.Access.Mode != "write" || helper.Parameters[1].View.Access.Mode != "read" || helper.Value.Value.Operator != "/" || helper.Value.Value.Left.Receiver.Binding != helper.Parameters[1].Binding.ID) {
					t.Fatal("original Write/Read field effect bindings differ")
				}
			}
			loop := packet.Payload.Systems[0].Loops[0]
			call := loop.Statements[0].Value.Value
			if call.Callee != helperIDs["update"] || call.Type != "f64" || call.Arguments == nil || len(*call.Arguments) != 3 || (*call.Arguments)[0].Binding != loop.Bindings[0].Declaration.ID || (*call.Arguments)[1].Binding != loop.Bindings[1].Declaration.ID {
				t.Fatal("original caller argument order/callee differs")
			}
			t.Logf("actual whole publicv5 f64 view helpers=%d files=%d nodes=%d", len(packet.Payload.ScalarHelpers), len(packet.Payload.Program.Files), len(packet.Payload.Nodes))
		})
	}
}

func TestSystemQueryF64HelperBoundaries38n(t *testing.T) {
	for _, name := range []string{"argument-width", "return-width", "arithmetic-width", "read-view-write", "read-to-write"} {
		t.Run(name, func(t *testing.T) {
			r := systemF64ScalarRequest38n(t)
			expected, code := "", 0
			switch name {
			case "argument-width":
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "dt: f64): void", "dt: f32): void", 1)
				expected = "argument must join original parameter"
			case "return-width":
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "function scaled(v:f64,dt:f64):f64", "function scaled(v:f64,dt:f64):f32", 1)
				expected = "return must preserve declared f32/f64 type"
			case "arithmetic-width":
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "function scaled(v:f64,dt:f64):f64", "function scaled(v:f64,dt:f32):f64", 1)
				expected = "same-width f32/f64 operands"
			case "read-view-write":
				r = systemF64ViewRequest38n(t, false)
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "function update(p:Write<Position>", "function update(p:Read<Position>", 1)
				code = 2540
			case "read-to-write":
				r = systemF64ViewRequest38n(t, false)
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "[Write<Position>,Read<Velocity>]", "[Write<Position>,Read<Position>,Read<Velocity>]", 1)
				r.Files[0].Content = strings.ReplaceAll(strings.ReplaceAll(r.Files[0].Content, "[p,v]", "[p,r,v]"), "update(p,v,dt)", "update(r,v,dt)")
				expected = "without Read-to-Write upgrade"
			}
			official, _, _, failure := newOwnedSystemProgram(r)
			if failure != nil {
				t.Fatal(failure)
			}
			codes := systemDiagnosticCodes38c(t, official)
			found := code == 0 && len(codes) == 0
			for _, actual := range codes {
				found = found || actual == code
			}
			if !found {
				t.Fatal("official helper diagnostic boundary differs", code, codes)
			}
			_, e, partial := systemScalarRun38g(t, r, "f64-helper-negative-"+name)
			if e == nil || partial != 0 || code == 0 && !strings.Contains(e.Error(), expected) {
				t.Fatal("helper width/permission boundary emitted wrong or partial facts", e, partial)
			}
			t.Logf("actual f64 helper refusal: %v", e)
		})
	}
}

func TestSystemQueryRetiredV4Rejected38n(t *testing.T) {
	raw := systemPublicRequest38c(t, systemF64ScalarRequest38n(t))
	var output bytes.Buffer
	if e := runFrontendCommand(t.Context(), []string{"--system-query-facts-io-v4"}, bytes.NewReader(raw), &output); e == nil || output.Len() != 0 {
		t.Fatal("retired v4 command accepted", e, output.Len())
	}
	var prior systemQueryRequest
	if e := json.Unmarshal(raw, &prior); e != nil {
		t.Fatal(e)
	}
	prior.Version = 4
	old, e := arrayFactsCanonical(prior)
	if e != nil {
		t.Fatal(e)
	}
	if e := runFrontendCommand(t.Context(), []string{"--system-query-facts-io-v5"}, bytes.NewReader(append(old, '\n')), &output); e == nil || output.Len() != 0 {
		t.Fatal("retired v4 request accepted", e, output.Len())
	}
}

func TestSystemQueryF64ScalarHelper38n(t *testing.T) {
	r := systemF64ScalarRequest38n(t)
	official, _, consumed, failure := newOwnedSystemProgram(r)
	if failure != nil || len(consumed) != len(r.Files) {
		t.Fatal("whole original helper Program differs", failure, consumed)
	}
	if codes := systemDiagnosticCodes38c(t, official); len(codes) != 0 {
		t.Fatal("original f64 helper requires official diagnostics0", codes)
	}
	packet, e, partial := systemScalarRun38g(t, r, "f64-scalar-helper")
	if e != nil {
		t.Fatal("public v5 f64 scalar helper rejected", e, partial)
	}
	if packet.Version != 5 || len(packet.Payload.Program.Files) != len(r.Files) || len(packet.Payload.ScalarHelpers) != 1 || len(packet.Payload.Systems) != 1 {
		t.Fatal("whole original Program or selected helper inventory differs")
	}
	helper := packet.Payload.ScalarHelpers[0]
	if helper.Declaration.Name != "scaled" || helper.ReturnType != "f64" || helper.Signature.ReturnType != "f64" || len(helper.Parameters) != 2 || len(helper.Signature.Parameters) != 2 || helper.Value.Kind != "binary" || helper.Value.Operator != "*" || helper.Value.Type != "f64" {
		t.Fatal("actual f64 signature/return expression differs")
	}
	for index, parameter := range helper.Parameters {
		if parameter.LogicalType != "f64" || helper.Signature.Parameters[index] != "f64" || parameter.View != nil || parameter.Index != index || parameter.Binding.Kind != "KindParameter" {
			t.Fatal("original ordered f64 helper parameters differ")
		}
	}
	if helper.Value.Left.Binding != helper.Parameters[0].Binding.ID || helper.Value.Right.Binding != helper.Parameters[1].Binding.ID {
		t.Fatal("original parameter Symbol bindings differ")
	}
	system := packet.Payload.Systems[0]
	loop := system.Loops[0]
	call := loop.Statements[0].Value.Value
	if call.Kind != "call" || call.Type != "f64" || call.FlowType != "f64" || call.Callee != helper.ID || call.Signature == nil || call.Signature.ID != helper.Signature.ID || call.Signature.ReturnType != "f64" || call.Arguments == nil || len(*call.Arguments) != 2 {
		t.Fatal("actual resolved helper callee/signature/result differs")
	}
	arguments := *call.Arguments
	if arguments[0].Kind != "fieldAccess" || arguments[0].Type != "f64" || arguments[0].Receiver.Binding != loop.Bindings[1].Declaration.ID || arguments[1].Binding != system.Parameters[1].Binding.ID || arguments[1].Type != "f64" {
		t.Fatal("original ordered call arguments differ")
	}
	source := r.Files[0].Content
	start := len(utf16.Encode([]rune(source[:strings.Index(source, "scaled(v.x, dt)")])))
	if call.Range != (span{start, start + len("scaled(v.x, dt)")}) || call.CalleeRange == nil || *call.CalleeRange != (span{start, start + len("scaled")}) || !strings.Contains(source, "function unused(v:i32)") || !strings.Contains(source, "class Box") {
		t.Fatal("original UTF16 callee or complete unused context differs")
	}
	t.Logf("actual whole publicv5 f64 scalar helper files=%d nodes=%d helper=%s return=%s", len(packet.Payload.Program.Files), len(packet.Payload.Nodes), helper.Declaration.Name, helper.ReturnType)
}
