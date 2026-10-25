// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tsjson "github.com/microsoft/typescript-go/internal/json"
)

// This boundary preserves original syntax, symbols and order. Copying, GC and
// kernel obligations belong to the independent common Core consumer.
func TestCommonProgramHiddenValuePublic37p(t *testing.T) {
	var request map[string]any
	if err := json.Unmarshal(commonProgramRequest37n(t, "hidden-value-copy"), &request); err != nil {
		t.Fatal(err)
	}
	root := func(kind, name string) closureRoot {
		return closureRoot{Kind: kind, SourcePath: "/src/main.ts", Name: name}
	}
	request["input"].(map[string]any)["compilationClosure"] = map[string]any{
		"roots":         []closureRoot{root("application", "ordinary"), root("application", "relayHidden"), root("kernel", "hiddenKernel"), root("kernel", "left"), root("kernel", "right"), root("module-initialization", "$moduleInit")},
		"requiredRoots": []closureRoot{root("kernel", "left"), root("kernel", "right")},
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	canonical := tsjson.Value(encoded)
	if err := canonical.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	input := append(canonical, '\n')
	var output bytes.Buffer
	if err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v1"}, bytes.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	if capture := os.Getenv("TSFG_COMMON_PROGRAM_CAPTURE_ROOT"); capture != "" {
		if err := os.MkdirAll(capture, 0700); err != nil {
			t.Fatal(err)
		}
		for suffix, data := range map[string][]byte{"request.json": input, "package.json": output.Bytes()} {
			if err := os.WriteFile(filepath.Join(capture, "hidden-value-copy-"+suffix), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	var packet commonProgramPackage
	if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
		t.Fatal(err)
	}
	p := packet.Payload
	if packet.Family != "tsfg.program-facts-package-candidate" || packet.Version != 1 || len(p.Files) != 2 || len(p.Functions) != 7 || len(p.Globals) != 2 || len(p.Modules) != 1 || len(p.Modules[0].Initialization) != 2 || len(p.Closure.Roots) != 6 || len(p.Closure.RequiredRoots) != 2 {
		t.Fatal("complete original hidden-value Program inventory changed")
	}
	if p.Files[0].Path != "/src/main.ts" || p.Files[0].SHA256 != "f6ed9fd4c21e61b90b1ae2b22fa22a4e65a51a5da5b013ac254f3ae73b8f94d1" || p.Files[0].UTF16Length != 975 || p.Globals[0].Binding.Name != "leftTotal" || p.Globals[1].Binding.Name != "rightTotal" {
		t.Fatal("original source identity, UTF16 length or global directory changed")
	}
	functions, names := map[string]arrayGenericFunction{}, []string{}
	for _, fn := range p.Functions {
		functions[fn.Name], names = fn, append(names, fn.Name)
	}
	if !slices.Equal(names, []string{"add", "left", "right", "ordinary", "id", "relayHidden", "hiddenKernel"}) {
		t.Fatal("original function order changed")
	}
	for i, name := range []string{"ordinary", "relayHidden", "hiddenKernel", "left", "right", "$moduleInit"} {
		selected := p.Closure.Roots[i]
		kind := "kernel"
		if i < 2 {
			kind = "application"
		} else if i == 5 {
			kind = "module-initialization"
		}
		if selected.Kind != kind || selected.Name != name || selected.SourcePath != "/src/main.ts" || i < 5 && selected.Target != functions[name].ID || i == 5 && selected.Target != "/src/main.ts" {
			t.Fatal("root must join its original function or module without a safety verdict")
		}
	}
	for i, name := range []string{"left", "right"} {
		if p.Closure.RequiredRoots[i].Kind != "kernel" || p.Closure.RequiredRoots[i].Name != name || p.Closure.RequiredRoots[i].Target != functions[name].ID {
			t.Fatal("required roots changed")
		}
	}
	types, classes := map[string]arrayGenericType{}, []string{}
	var carrier, box arrayGenericType
	for _, typ := range p.Types {
		types[typ.ID] = typ
		if typ.Kind == "value" || typ.Kind == "reference" {
			classes = append(classes, typ.Name)
		}
		if typ.Name == "Carrier" {
			carrier = typ
		} else if typ.Name == "Box" {
			box = typ
		}
	}
	slices.Sort(classes)
	if !slices.Equal(classes, []string{"Box", "Carrier", "Delta"}) || carrier.Kind != "value" || carrier.Fields == nil || len(*carrier.Fields) != 2 || box.Kind != "reference" {
		t.Fatal("original value Carrier and managed Box declarations changed")
	}
	n, ref := (*carrier.Fields)[0], (*carrier.Fields)[1]
	if n.Name != "n" || n.Type != "i32" || ref.Name != "box" || types[ref.Type].Kind != "union" || len(types[ref.Type].Members) != 2 || !slices.Contains(types[ref.Type].Members, box.ID) || !slices.Contains(types[ref.Type].Members, "undefined") {
		t.Fatal("Carrier fields must retain original order and Box|undefined type")
	}
	id, relay, hidden := functions["id"], functions["relayHidden"], functions["hiddenKernel"]
	if len(id.Parameters) != 1 || id.Parameters[0].DeclaredType != "i32" || id.ReturnType != "i32" || len(id.Body) != 1 || id.Body[0].Value == nil || id.Body[0].Value.Binding != id.Parameters[0].ID || id.Body[0].Value.Range != (span{666, 667}) || len(relay.Body) != 7 {
		t.Fatal("id must preserve its original scalar signature, parameter and return")
	}
	body := relay.Body
	if body[0].Initializer == nil || body[0].Initializer.Kind != "new" || body[0].Initializer.Type != carrier.ID || body[0].Initializer.Range != (span{730, 743}) {
		t.Fatal("original Carrier allocation changed")
	}
	for i, expected := range []struct {
		index int
		value string
		where span
	}{{1, "5", span{747, 754}}, {4, "7", span{828, 835}}} {
		store := body[expected.index].Value
		if store == nil || store.Kind != "fieldAssignment" || store.Range != expected.where || store.Target == nil || store.Target.Property != n.ID || store.Target.Receiver == nil || store.Target.Receiver.Binding != body[0].Binding || store.Value == nil || store.Value.Decimal != expected.value {
			t.Fatalf("original Carrier write %d moved or changed", i)
		}
	}
	store := body[2].Value
	if store == nil || store.Target == nil || store.Target.Property != ref.ID || store.Target.Receiver == nil || store.Target.Receiver.Binding != body[0].Binding || store.Value == nil || store.Value.Kind != "new" || store.Value.Type != box.ID || store.Value.Range != (span{766, 775}) {
		t.Fatal("original Box allocation and Carrier.box store changed")
	}
	call := body[3].Initializer
	if call == nil || call.Kind != "call" || call.Callee != id.ID || call.Range != (span{801, 824}) || call.Arguments == nil || len(*call.Arguments) != 1 {
		t.Fatal("original scalar-view call to id changed")
	}
	checkAs := func(value *arrayGenericExpression, typ, binding string, outer, inner span) {
		t.Helper()
		if value == nil || value.Kind != "as" || value.Type != typ || value.Range != outer || value.Expression == nil || value.Expression.Kind != "as" || value.Expression.Type != "unknown" || value.Expression.Range != inner || value.Expression.Expression == nil || value.Expression.Expression.Kind != "bindingReference" || value.Expression.Expression.Binding != binding {
			t.Fatal("nested pure assertions must retain their original binding and UTF16 anchors")
		}
	}
	argument := (*call.Arguments)[0]
	checkAs(&argument, "i32", body[0].Binding, span{804, 823}, span{804, 816})
	checkAs(body[5].Initializer, carrier.ID, body[3].Binding, span{861, 891}, span{861, 880})
	returned := body[6].Value
	if returned == nil || returned.Kind != "fieldAccess" || returned.Property != n.ID || returned.Receiver == nil || returned.Receiver.Binding != body[5].Binding || returned.Range != (span{902, 908}) {
		t.Fatal("original back.n return must join the returned-value binding")
	}
	if len(hidden.Parameters) != 1 || len(hidden.Body) != 1 || hidden.Body[0].Value == nil || hidden.Body[0].Value.Callee != id.ID || hidden.Body[0].Value.Range != (span{966, 971}) || hidden.Body[0].Value.Arguments == nil || len(*hidden.Body[0].Value.Arguments) != 1 || (*hidden.Body[0].Value.Arguments)[0].Binding != hidden.Parameters[0].ID {
		t.Fatal("declared hiddenKernel call must retain its real id target and parameter")
	}
}

// Exercise the actual legacy annotation/class-view boundaries with no module
// globals, so the unrelated old top-level coverage gate cannot satisfy this.
func TestCommonProgramHiddenLegacyBoundaries37p(t *testing.T) {
	for _, tc := range []struct {
		source, detail, token string
	}{
		{"@tsfgValue class V { n:i32=0; } export function f(x:V):i32 { return x as unknown as i32; }", "type must join its original pinned scalar, absence, union, generic owner or nominal declaration", "unknown"},
		{"@tsfgValue class A { n:i32=0; } @tsfgValue class B { n:i32=0; } export function f(x:A):B { return x as B; }", "value assertion requires its original nominal identity", "x as B"},
	} {
		var output bytes.Buffer
		err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(genericArrayRequest25f(t, tc.source)), &output)
		var failure *arrayFactsFailure
		start := strings.Index(tc.source, tc.token)
		if !errors.As(err, &failure) || failure.Code != "r03-array-facts-capability" || failure.Detail != tc.detail || output.Len() != 0 || len(failure.Diagnostics) != 1 || failure.Diagnostics[0].Range != (span{start, start + len(tc.token)}) {
			t.Fatalf("legacy type/view boundary expanded or failed at an unrelated guard: %v", err)
		}
	}
}
