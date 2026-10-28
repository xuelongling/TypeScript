// SPDX-License-Identifier: Apache-2.0
package main

import (
	"strings"
	"testing"
	"unicode/utf16"
)

func systemF64Request38n(t *testing.T, expression string) request {
	t.Helper()
	r := systemScalarRequest38g(t)
	source := strings.ReplaceAll(r.Files[0].Content, ": f32", ": f64")
	const original = "p.x += scaled(v.x, dt)"
	if strings.Count(source, original) != 1 {
		t.Fatal("complete original Query Source differs")
	}
	r.Files[0].Content = strings.Replace(source, original, expression, 1)
	return r
}

func TestSystemQueryF64Addition38n(t *testing.T) {
	const expression = "p.x += v.x * dt"
	r := systemF64Request38n(t, expression)
	official, _, consumed, failure := newOwnedSystemProgram(r)
	if failure != nil || len(consumed) != len(r.Files) {
		t.Fatal("complete original Program consumption differs", failure, consumed)
	}
	if codes := systemDiagnosticCodes38c(t, official); len(codes) != 0 {
		t.Fatal("official f64 Query diagnostics must be0", codes)
	}
	packet, e, partial := systemScalarRun38g(t, r, "f64-add")
	if e != nil {
		t.Fatal("public v5 f64 Query capability missing", e, partial)
	}
	if packet.Version != 5 || len(packet.Payload.Program.Files) != len(r.Files) || len(packet.Payload.Systems) != 1 || len(packet.Payload.ScalarHelpers) != 0 {
		t.Fatal("whole original Program or reachable helper inventory differs")
	}
	for _, file := range r.Files {
		found := false
		for _, actual := range packet.Payload.Program.Files {
			found = found || file.Path == actual.Path && file.Content == actual.Content
		}
		if !found {
			t.Fatal("complete original Source bytes lost", file.Path)
		}
	}
	system := packet.Payload.Systems[0]
	loop := system.Loops[0]
	value := loop.Statements[0].Value
	if system.Parameters[1].LogicalType != "f64" || len(loop.Bindings) != 2 || len(loop.Statements) != 2 || value.Kind != "fieldAssignment" || value.Operator != "+=" || value.Type != "f64" || value.FlowType != "f64" || value.Target == nil || value.Target.Type != "f64" || value.Target.Receiver.Binding != loop.Bindings[0].Declaration.ID {
		t.Fatal("original f64 field, parameter, binding or assignment differs")
	}
	rhs := value.Value
	if rhs.Kind != "binary" || rhs.Type != "f64" || rhs.Operator != "*" || rhs.Left.Kind != "fieldAccess" || rhs.Left.Type != "f64" || rhs.Left.Receiver.Binding != loop.Bindings[1].Declaration.ID || rhs.Right.Binding != system.Parameters[1].Binding.ID || rhs.Right.Type != "f64" {
		t.Fatal("original same-width f64 operands or source order differs")
	}
	start := len(utf16.Encode([]rune(r.Files[0].Content[:strings.Index(r.Files[0].Content, expression)])))
	if value.Range != (span{start, start + len(expression)}) || value.OperatorRange == nil || *value.OperatorRange != (span{start + 4, start + 6}) || value.Target.Range != (span{start, start + 3}) {
		t.Fatal("original UTF16 assignment/operator/target anchors differ", value)
	}
	t.Logf("actual complete publicv5 f64 files=%d nodes=%d helpers=%d officialDiagnostics0", len(packet.Payload.Program.Files), len(packet.Payload.Nodes), len(packet.Payload.ScalarHelpers))
}

func TestSystemQueryF64Remaining38n(t *testing.T) {
	for _, c := range []struct{ name, operator, rhs string }{
		{"f64-assign", "=", "v.x * dt"},
		{"f64-subtract", "-=", "v.x + dt"},
		{"f64-multiply", "*=", "v.x - dt"},
		{"f64-divide", "/=", "v.x / dt"},
		{"f64-nested-rhs", "*=", "(p.x = dt)"},
		{"independent-widths", "+=", "v.x * dt"},
	} {
		t.Run(c.name, func(t *testing.T) {
			expression := "p.x " + c.operator + " " + c.rhs
			r := systemF64Request38n(t, expression)
			if c.name == "independent-widths" {
				r.Files[0].Content = strings.ReplaceAll(r.Files[0].Content, "y: f64", "y: f32")
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "dt: f64): void", "dt: f64, narrow: f32): void", 1)
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "p.y += v.y * dt", "p.y += v.y * narrow", 1)
			}
			official, _, consumed, failure := newOwnedSystemProgram(r)
			if failure != nil || len(consumed) != len(r.Files) {
				t.Fatal("complete original Program consumption differs", failure, consumed)
			}
			if codes := systemDiagnosticCodes38c(t, official); len(codes) != 0 {
				t.Fatal("official f64 Query diagnostics must be0", codes)
			}
			packet, e, partial := systemScalarRun38g(t, r, c.name)
			if e != nil {
				t.Fatal("public v5 same-width Query rejected", e, partial)
			}
			if packet.Version != 5 || len(packet.Payload.Program.Files) != len(r.Files) || len(packet.Payload.ScalarHelpers) != 0 || len(packet.Payload.Systems) != 1 || !strings.Contains(packet.Payload.Program.Files[0].Content, "class Box") || !strings.Contains(packet.Payload.Program.Files[0].Content, "function unused(v:i32)") {
				t.Fatal("whole original Source or selected helper inventory differs")
			}
			system := packet.Payload.Systems[0]
			loop := system.Loops[0]
			value := loop.Statements[0].Value
			if value.Kind != "fieldAssignment" || value.Type != "f64" || value.FlowType != "f64" || value.Operator != c.operator || value.Target == nil || value.Target.Type != "f64" || value.Target.Receiver.Binding != loop.Bindings[0].Declaration.ID {
				t.Fatal("original same-width assignment or target binding differs")
			}
			matchedField := false
			for _, typ := range packet.Payload.Types {
				if typ.ID == loop.Bindings[0].ComponentType && typ.Fields != nil {
					for _, field := range *typ.Fields {
						matchedField = matchedField || field.Name == "x" && field.Type == "f64" && field.ID == value.Target.Property
					}
				}
			}
			if !matchedField {
				t.Fatal("assignment must retain its original owning component field")
			}
			start := len(utf16.Encode([]rune(r.Files[0].Content[:strings.Index(r.Files[0].Content, expression)])))
			if value.Range != (span{start, start + len(expression)}) || value.OperatorRange == nil || *value.OperatorRange != (span{start + 4, start + 4 + len(c.operator)}) {
				t.Fatal("original UTF16 expression/operator anchors differ")
			}
			if c.name == "f64-nested-rhs" {
				group := value.Value
				if group.Kind != "parentheses" || group.Expression == nil || group.Expression.Kind != "fieldAssignment" || group.Expression.Type != "f64" || group.Expression.Operator != "=" || group.Expression.Target.Property != value.Target.Property || group.Expression.Target.Receiver.Binding != loop.Bindings[0].Declaration.ID || group.Expression.Value.Binding != system.Parameters[1].Binding.ID {
					t.Fatal("old target before nested RHS assignment structure lost")
				}
			} else if value.Value.Kind != "binary" || value.Value.Type != "f64" || value.Value.Left.Type != "f64" || value.Value.Right.Type != "f64" {
				t.Fatal("same-width arithmetic operands lost")
			}
			if c.name == "independent-widths" && (len(system.Parameters) != 3 || system.Parameters[1].LogicalType != "f64" || system.Parameters[2].LogicalType != "f32" || loop.Statements[1].Value.Type != "f32" || loop.Statements[1].Value.Value.Right.Binding != system.Parameters[2].Binding.ID) {
				t.Fatal("independent f32 and f64 fields/parameters were globally coerced")
			}
			t.Logf("actual complete publicv5 name=%s files=%d nodes=%d sameWidth=f64", c.name, len(packet.Payload.Program.Files), len(packet.Payload.Nodes))
		})
	}
}

func TestSystemQueryF64Boundaries38n(t *testing.T) {
	for _, name := range []string{"mixed-parameter", "mixed-field", "mixed-assignment", "read-write", "wrong-field", "wrong-type"} {
		t.Run(name, func(t *testing.T) {
			expression := "p.x += v.x * dt"
			if name == "mixed-assignment" {
				expression = "p.x = dt"
			}
			if name == "read-write" {
				expression = "v.x += dt"
			}
			if name == "wrong-field" {
				expression = "p.missing += dt"
			}
			if name == "wrong-type" {
				expression = "p.x *= true"
			}
			r := systemF64Request38n(t, expression)
			expected, code := "same-width f32/f64 operands", 0
			switch name {
			case "mixed-parameter", "mixed-assignment":
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "dt: f64): void", "dt: f32): void", 1)
			case "mixed-field":
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "class Velocity { x: f64", "class Velocity { x: f32", 1)
			case "read-write":
				code = 2540
			case "wrong-field":
				code = 2339
			case "wrong-type":
				code = 2363
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
				t.Fatal("original official f64 diagnostic boundary differs", code, codes)
			}
			_, e, partial := systemScalarRun38g(t, r, "negative-"+name)
			if e == nil || partial != 0 || code == 0 && !strings.Contains(e.Error(), expected) {
				t.Fatal("f64 boundary emitted partial facts or wrong capability", e, partial)
			}
			t.Logf("actual original f64 refusal: %v", e)
		})
	}
}
