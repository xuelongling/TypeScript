// SPDX-License-Identifier: Apache-2.0
package main

import (
	"strings"
	"testing"
	"unicode/utf16"
)

func TestSystemQueryFieldCompound38m(t *testing.T) {
	r := systemViewRequest38h(t)
	const original = "p.x = update(p,v,dt);"
	const replacement = "p.x *= update(p,v,dt);"
	if strings.Count(r.Files[0].Content, original) != 1 {
		t.Fatal("complete original Query statement differs")
	}
	r.Files[0].Content = strings.Replace(r.Files[0].Content, original, replacement, 1)
	official, _, consumed, failure := newOwnedSystemProgram(r)
	if failure != nil || len(consumed) != len(r.Files) {
		t.Fatal("whole original Program consumption differs", failure, consumed)
	}
	if codes := systemDiagnosticCodes38c(t, official); len(codes) != 0 {
		t.Fatal("original field compound requires official diagnostics0", codes)
	}
	packet, e, partial := systemScalarRun38g(t, r, "compound-multiply")
	if e != nil {
		t.Fatal("public v4 field compound rejected", e, partial)
	}
	if packet.Version != 4 || len(packet.Payload.Program.Files) != len(r.Files) || len(packet.Payload.ScalarHelpers) != 3 || len(packet.Payload.Systems) != 1 {
		t.Fatal("complete original Program or reachable helper inventory differs")
	}
	for _, file := range r.Files {
		found := false
		for _, actual := range packet.Payload.Program.Files {
			found = found || actual.Path == file.Path && actual.Content == file.Content
		}
		if !found {
			t.Fatal("whole original Source file bytes lost", file.Path)
		}
	}
	loop := packet.Payload.Systems[0].Loops[0]
	value := loop.Statements[0].Value
	if len(loop.Bindings) != 2 || len(loop.Statements) != 2 || value.Kind != "fieldAssignment" || value.Operator != "*=" || value.Type != "f32" || value.Target == nil || value.Target.Receiver.Binding != loop.Bindings[0].Declaration.ID || value.Value.Kind != "call" || value.Value.Arguments == nil || len(*value.Value.Arguments) != 3 {
		t.Fatal("original compound target, operator or ordered helper arguments differ")
	}
	source := r.Files[0].Content
	start := len(utf16.Encode([]rune(source[:strings.Index(source, replacement)])))
	if value.Range != (span{start, start + len(replacement) - 1}) || value.OperatorRange == nil || *value.OperatorRange != (span{start + 4, start + 6}) || value.Target.Range != (span{start, start + 3}) {
		t.Fatal("original UTF16 compound/operator/target anchors differ", value)
	}
	arguments := *value.Value.Arguments
	if arguments[0].Binding != loop.Bindings[0].Declaration.ID || arguments[1].Binding != loop.Bindings[1].Declaration.ID || arguments[2].Binding != packet.Payload.Systems[0].Parameters[1].Binding.ID {
		t.Fatal("ordered original helper arguments differ")
	}
	t.Logf("actual publicv4 whole files=%d nodes=%d helpers=%d compound=%s originalUnusedAndBox=true", len(packet.Payload.Program.Files), len(packet.Payload.Nodes), len(packet.Payload.ScalarHelpers), value.Operator)
}

func TestSystemQueryRemainingFieldCompounds38m(t *testing.T) {
	for _, c := range []struct {
		name, operator string
		helper, nested bool
	}{
		{name: "compound-subtract", operator: "-="},
		{name: "compound-divide", operator: "/="},
		{name: "helper-subtract", operator: "-=", helper: true},
		{name: "helper-multiply", operator: "*=", helper: true},
		{name: "helper-divide", operator: "/=", helper: true},
		{name: "compound-nested-rhs", operator: "*=", nested: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := systemViewRequest38h(t)
			original, replacement := "p.x = update(p,v,dt)", "p.x "+c.operator+" update(p,v,dt)"
			expectedHelpers := 3
			if c.helper {
				original, replacement = "p.x += v.x*dt", "p.x "+c.operator+" v.x*dt"
			}
			if c.nested {
				replacement, expectedHelpers = "p.x *= (p.x = dt)", 2
			}
			if strings.Count(r.Files[0].Content, original) != 1 {
				t.Fatal("complete original field expression differs", original)
			}
			r.Files[0].Content = strings.Replace(r.Files[0].Content, original, replacement, 1)
			official, _, consumed, failure := newOwnedSystemProgram(r)
			if failure != nil || len(consumed) != len(r.Files) {
				t.Fatal("whole original Program consumption differs", failure, consumed)
			}
			if codes := systemDiagnosticCodes38c(t, official); len(codes) != 0 {
				t.Fatal("original compound requires official diagnostics0", codes)
			}
			packet, e, partial := systemScalarRun38g(t, r, c.name)
			if e != nil {
				t.Fatal("public v4 compound rejected", e, partial)
			}
			if packet.Version != 4 || len(packet.Payload.Program.Files) != len(r.Files) || len(packet.Payload.ScalarHelpers) != expectedHelpers || len(packet.Payload.Systems) != 1 {
				t.Fatal("complete original Program or selected helper inventory differs")
			}
			for _, file := range r.Files {
				found := false
				for _, actual := range packet.Payload.Program.Files {
					found = found || actual.Path == file.Path && actual.Content == file.Content
				}
				if !found {
					t.Fatal("complete unused/Box/profile Source bytes differ", file.Path)
				}
			}
			loop := packet.Payload.Systems[0].Loops[0]
			value := loop.Statements[0].Value
			binding := loop.Bindings[0].Declaration.ID
			if c.helper {
				for _, helper := range packet.Payload.ScalarHelpers {
					if helper.Declaration.Name == "update" {
						value, binding = helper.Value, helper.Parameters[0].Binding.ID
					}
				}
			}
			if value.Kind != "fieldAssignment" || value.Operator != c.operator || value.Type != "f32" || value.Target == nil || value.Target.Receiver.Binding != binding || value.Value == nil || loop.Statements[1].Value.Operator != "+=" {
				t.Fatal("original target binding/operator or old += changed")
			}
			start := len(utf16.Encode([]rune(r.Files[0].Content[:strings.Index(r.Files[0].Content, replacement)])))
			if value.Range != (span{start, start + len(replacement)}) || value.OperatorRange == nil || *value.OperatorRange != (span{start + 4, start + 6}) || value.Target.Range != (span{start, start + 3}) {
				t.Fatal("original UTF16 operator/expression/target differs", value)
			}
			if c.nested {
				inner := value.Value.Expression
				if value.Value.Kind != "parentheses" || inner == nil || inner.Kind != "fieldAssignment" || inner.Operator != "=" || inner.Target.Receiver.Binding != binding || inner.Value.Binding != packet.Payload.Systems[0].Parameters[1].Binding.ID {
					t.Fatal("original nested RHS assignment/order structure lost")
				}
			}
			t.Logf("actual complete publicv4 name=%s files=%d nodes=%d helpers=%d operator=%s", c.name, len(packet.Payload.Program.Files), len(packet.Payload.Nodes), len(packet.Payload.ScalarHelpers), c.operator)
		})
	}
}

func TestSystemQueryCompoundBoundaries38m(t *testing.T) {
	for _, c := range []struct {
		name, expression string
		code             int
	}{
		{"read-subtract", "v.x -= dt", 2540},
		{"read-multiply", "v.x *= dt", 2540},
		{"read-divide", "v.x /= dt", 2540},
		{"wrong-type", "p.x *= true", 2363},
		{"wrong-field", "p.z *= dt", 2339},
		{"declared-readonly", "p.x *= dt", 2540},
		{"unsupported-remainder", "p.x %= dt", 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := systemViewRequest38h(t)
			r.Files[0].Content = strings.Replace(r.Files[0].Content, "p.x = update(p,v,dt)", c.expression, 1)
			if c.name == "declared-readonly" {
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "class Position { x:f32=0;", "class Position { readonly x:f32=0;", 1)
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "return p.x += v.x*dt;", "return v.x*dt;", 1)
			}
			official, _, _, failure := newOwnedSystemProgram(r)
			if failure != nil {
				t.Fatal(failure)
			}
			codes := systemDiagnosticCodes38c(t, official)
			found := c.code == 0 && len(codes) == 0
			for _, code := range codes {
				found = found || code == c.code
			}
			if !found {
				t.Fatal("official compound boundary differs", c.code, codes)
			}
			_, e, partial := systemScalarRun38g(t, r, "negative-"+c.name)
			if e == nil || partial != 0 || c.code == 0 && !strings.Contains(e.Error(), "Query field assignment requires") {
				t.Fatal("original invalid compound emitted partial facts or wrong refusal", e, partial)
			}
			t.Logf("actual original compound refusal: %v", e)
		})
	}
}
