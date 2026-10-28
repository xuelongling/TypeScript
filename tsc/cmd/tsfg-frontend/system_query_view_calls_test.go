// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"github.com/microsoft/typescript-go/internal/json"
	"os"
	"strings"
	"testing"
)

func systemViewRequest38h(t *testing.T) request {
	t.Helper()
	r := systemRequest38c(t)
	source, e := os.ReadFile("testdata/system-query-v4/view-helper.ts")
	if e != nil {
		t.Fatal(e)
	}
	r.Files[0].Content = string(source)
	return r
}
func TestSystemQueryViewHelperPublic38h(t *testing.T) {
	for _, name := range []string{"direct", "imported", "aliases", "same-owner"} {
		t.Run(name, func(t *testing.T) {
			r := systemViewRequest38h(t)
			if name == "imported" {
				first := strings.Index(r.Files[0].Content, "function readX(")
				last := strings.Index(r.Files[0].Content, "@system")
				helpers := r.Files[0].Content[first:last]
				r.Files[0].Content = "import {update as advance,scaledX as sample} from './helpers';\n" + r.Files[0].Content[:first] + r.Files[0].Content[last:]
				r.Files[0].Content = strings.ReplaceAll(strings.ReplaceAll(r.Files[0].Content, "update(p,v,dt)", "advance(p,v,dt)"), "scaledX(p,dt)", "sample(p,dt)")
				helpers = strings.ReplaceAll(strings.ReplaceAll(helpers, "function scaledX(", "export function scaledX("), "function update(", "export function update(")
				r.Files = append(r.Files, inputFile{Path: "/src/helpers.ts", Content: "import type {Position,Velocity} from './main';\n" + helpers})
			}
			if name == "aliases" {
				r.Files[0].Content = "type R<T> = Read<T>;\ntype W = Write<Position>;\n" + strings.ReplaceAll(strings.ReplaceAll(r.Files[0].Content, "p:Read<Position>", "p:R<Position>"), "p:Write<Position>", "p:W")
			}
			if name == "same-owner" {
				r.Files[0].Content = strings.ReplaceAll(strings.ReplaceAll(r.Files[0].Content, "v:Read<Velocity>", "v:Read<Position>"), "update(p,v,dt)", "update(p,p,dt)")
			}
			official, _, consumed, failure := newOwnedSystemProgram(r)
			if failure != nil {
				t.Fatal(failure)
			}
			if len(consumed) != len(r.Files) {
				t.Fatal("complete original Program consumed paths differ")
			}
			if codes := systemDiagnosticCodes38c(t, official); len(codes) != 0 {
				t.Fatal("complete view helper Source diagnostics", codes)
			}
			packet, e, partial := systemScalarRun38g(t, r, "view-"+name)
			if e != nil {
				t.Fatalf("actual public view helper refusal output=%d: %v", partial, e)
			}
			if len(packet.Payload.ScalarHelpers) != 3 || len(packet.Payload.Program.Files) != len(r.Files) {
				t.Fatal("reachable helpers/full original Program inventory differs")
			}
			foundAlias := false
			for _, helper := range packet.Payload.ScalarHelpers {
				for index, parameter := range helper.Parameters {
					if parameter.Query != nil || parameter.Binding.Kind != "KindParameter" || parameter.Index != index || helper.Signature.Parameters[index] != parameter.LogicalType {
						t.Fatal("actual helper parameter/signature provenance differs")
					}
					if parameter.View != nil {
						access := parameter.View.Access
						if access.ComponentType != parameter.LogicalType || access.Index != 0 || access.Wrapper.SourcePath != systemQueryProfilePath || access.Component.Kind != "KindClassDeclaration" {
							t.Fatal("owned original view/component identity differs")
						}
						if len(parameter.View.Aliases) > 0 {
							foundAlias = true
						}
					} else if parameter.LogicalType != "f32" {
						t.Fatal("non-view helper parameter is not native f32")
					}
				}
				if helper.Declaration.Name == "readX" && (helper.Value.Kind != "fieldAccess" || helper.Value.Receiver.Binding != helper.Parameters[0].Binding.ID) {
					t.Fatal("read field must reference original formal view binding")
				}
				if helper.Declaration.Name == "update" && (helper.Value.Kind != "fieldAssignment" || helper.Value.Target.Receiver.Binding != helper.Parameters[0].Binding.ID || helper.Parameters[0].View.Access.Mode != "write" || helper.Parameters[1].View.Access.Mode != "read") {
					t.Fatal("original write/read views or field assignment differ")
				}
			}
			if name == "aliases" && !foundAlias {
				t.Fatal("original formal alias/substitution trace absent")
			}
			t.Logf("actual public view helpers=%d files=%d nodes=%d", len(packet.Payload.ScalarHelpers), len(packet.Payload.Program.Files), len(packet.Payload.Nodes))
		})
	}
}

func TestSystemQueryViewHelperNegatives38h(t *testing.T) {
	for _, name := range []string{"read-to-write", "component", "plain-formal", "foreign-wrapper", "global-capture"} {
		t.Run(name, func(t *testing.T) {
			r := systemViewRequest38h(t)
			expected := ""
			switch name {
			case "read-to-write":
				r.Files[0].Content = strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(r.Files[0].Content, "[Write<Position>,Read<Velocity>]", "[Write<Position>,Read<Position>,Read<Velocity>]"), "[p,v]", "[p,r,v]"), "update(p,v,dt)", "update(r,v,dt)")
				expected = "Read-to-Write upgrade"
			case "component":
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "update(p,v,dt)", "update(v,v,dt)", 1)
				expected = "original parameter and component"
			case "plain-formal":
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "p:Write<Position>", "p:Position", 1)
				expected = "native f32 annotation or owned Read/Write view"
			case "foreign-wrapper":
				r.Files[0].Content = "interface Read<T> {readonly x:f32;readonly y:f32;}\n" + r.Files[0].Content
				expected = "owned Read or Write"
			case "global-capture":
				r.Files[0].Content = strings.Replace(strings.Replace(r.Files[0].Content, "function readX(", "const captured:Position=new Position();\nfunction readX(", 1), "return p.x;", "return captured.x;", 1)
				expected = "original loop binding or f32/f64 parameter"
			}
			official, _, _, failure := newOwnedSystemProgram(r)
			if failure != nil {
				t.Fatal(failure)
			}
			if codes := systemDiagnosticCodes38c(t, official); len(codes) != 0 {
				t.Fatal("view boundary must use checker-valid original Source", codes)
			}
			_, e, partial := systemScalarRun38g(t, r, "negative-view-"+name)
			if e == nil || partial != 0 || !strings.Contains(e.Error(), expected) {
				t.Fatal("specific atomic view capability refusal absent", expected, e, partial)
			}
			t.Logf("actual view capability: %v", e)
		})
	}
}

func TestSystemQueryViewReadonlyOfficial38h(t *testing.T) {
	r := systemViewRequest38h(t)
	r.Files[0].Content = strings.Replace(r.Files[0].Content, "return p.x;", "return p.x += p.y;", 1)
	official, _, _, failure := newOwnedSystemProgram(r)
	if failure != nil {
		t.Fatal(failure)
	}
	codes := systemDiagnosticCodes38c(t, official)
	found := false
	for _, code := range codes {
		found = found || code == 2540
	}
	if !found {
		t.Fatal("original Read formal write requires actual TS2540", codes)
	}
	_, e, partial := systemScalarRun38g(t, r, "negative-view-readonly-official")
	if e == nil || partial != 0 {
		t.Fatal("original readonly error emitted partial facts", e, partial)
	}
}

func TestSystemQueryViewCalleeReassignmentOfficial38h(t *testing.T) {
	r := systemViewRequest38h(t)
	r.Files[0].Content += "\nfunction other(p:Read<Position>):f32{return p.y;}\nreadX=other;\n"
	official, _, _, failure := newOwnedSystemProgram(r)
	if failure != nil {
		t.Fatal(failure)
	}
	codes := systemDiagnosticCodes38c(t, official)
	found := false
	for _, code := range codes {
		found = found || code == 2630
	}
	if !found {
		t.Fatal("original helper reassignment requires actual TS2630", codes)
	}
	_, e, partial := systemScalarRun38g(t, r, "negative-view-callee-reassignment")
	if e == nil || partial != 0 {
		t.Fatal("original callee binding error emitted partial facts", e, partial)
	}
}

func TestSystemQueryViewOldVersion38h(t *testing.T) {
	r := systemViewRequest38h(t)
	raw := systemPublicRequest38c(t, r)
	var out bytes.Buffer
	if e := runFrontendCommand(t.Context(), []string{"--system-query-facts-io-v3"}, bytes.NewReader(raw), &out); e == nil || out.Len() != 0 {
		t.Fatal("retired private command was accepted", e, out.Len())
	}
	var prior systemQueryRequest
	if e := json.Unmarshal(raw, &prior); e != nil {
		t.Fatal(e)
	}
	prior.Version = 3
	old, e := arrayFactsCanonical(prior)
	if e != nil {
		t.Fatal(e)
	}
	old = append(old, '\n')
	if e := runFrontendCommand(t.Context(), []string{"--system-query-facts-io-v4"}, bytes.NewReader(old), &out); e == nil || out.Len() != 0 {
		t.Fatal("retired private version was accepted", e, out.Len())
	}
}
