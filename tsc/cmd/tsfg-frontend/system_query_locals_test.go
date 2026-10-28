// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemQueryLocals38o(t *testing.T) {
	for _, width := range []string{"f32", "f64"} {
		t.Run(width, func(t *testing.T) {
			r := systemScalarRequest38g(t)
			r.Files[0].Content = strings.ReplaceAll(r.Files[0].Content, "f32", width)
			r.Files[0].Content = strings.Replace(r.Files[0].Content, "return v*dt;", "const product = v*dt; let current:"+width+" = product; current += dt; current = current - product; return current;", 1)
			r.Files[0].Content = strings.Replace(r.Files[0].Content, "p.x += scaled(v.x, dt)", "const snapshot = p.x; let delta:"+width+" = scaled(v.x, dt); p.x = dt; delta += snapshot; delta = delta - dt; p.x += delta; p.y = snapshot", 1)
			official, _, consumed, failure := newOwnedSystemProgram(r)
			if failure != nil || len(consumed) != len(r.Files) {
				t.Fatal(failure, consumed)
			}
			if codes := systemDiagnosticCodes38c(t, official); len(codes) != 0 {
				t.Fatal("official local Source diagnostics", codes)
			}
			file := official.GetSourceFile("/src/main.ts")
			check, release := official.GetTypeCheckerForFileExclusive(t.Context(), file)
			declarations := map[*ast.Symbol]*ast.Node{}
			uses := map[span]*ast.Node{}
			var walk func(*ast.Node)
			walk = func(node *ast.Node) {
				if node.Kind == ast.KindVariableDeclaration && node.Name().Kind == ast.KindIdentifier {
					symbol := check.GetSymbolAtLocation(node.Name())
					if symbol == nil || len(symbol.Declarations) != 1 || symbol.Declarations[0] != node || check.GetTypeOfSymbolAtLocation(symbol, node).Flags()&checker.TypeFlagsNumberLike == 0 {
						t.Fatal("official original local Symbol/type missing")
					}
					declarations[symbol] = node
				}
				node.ForEachChild(func(child *ast.Node) bool { walk(child); return false })
			}
			walk(file.AsNode())
			walk = func(node *ast.Node) {
				if node.Kind == ast.KindIdentifier {
					if declaration := declarations[check.GetSymbolAtLocation(node)]; declaration != nil && node != declaration.Name() {
						uses[*genericSpan(node)] = declaration
					}
				}
				node.ForEachChild(func(child *ast.Node) bool { walk(child); return false })
			}
			walk(file.AsNode())
			release()
			packet, e, partial := systemScalarRun38g(t, r, "locals-"+width)
			if e != nil {
				t.Fatal("original checked local Query rejected", e, partial)
			}
			if len(packet.Payload.Systems[0].Loops[0].Statements) != 8 || len(packet.Payload.ScalarHelpers) != 1 {
				t.Fatal("complete local statement inventory lost")
			}
			loop := packet.Payload.Systems[0].Loops[0]
			helper := packet.Payload.ScalarHelpers[0]
			if len(declarations) != 4 || len(loop.LocalBindings) != 2 || len(helper.Bindings) != 2 || len(helper.Statements) != 5 {
				t.Fatal("official local directory differs")
			}
			bindings := map[string]arrayFactsBinding{}
			for _, binding := range append(loop.LocalBindings, helper.Bindings...) {
				bindings[binding.ID] = binding
				if binding.LexicalOwner == nil || *binding.LexicalOwner == nil || binding.DeclaredType != width {
					t.Fatal("original binding owner/width missing")
				}
			}
			seen := 0
			var expression func(*arrayGenericExpression)
			expression = func(value *arrayGenericExpression) {
				if value == nil {
					return
				}
				if binding, ok := bindings[value.Binding]; ok {
					declaration := uses[value.Range]
					if declaration == nil || binding.NameRange != *genericSpan(declaration.Name()) || binding.DeclarationRange != *genericSpan(declaration) {
						t.Fatal("local reference differs from independent original Symbol", value.Range)
					}
					seen++
				}
				if value.Target != nil {
					if binding, ok := bindings[value.Target.Binding]; ok {
						declaration := uses[value.Target.Range]
						if declaration == nil || binding.NameRange != *genericSpan(declaration.Name()) || !binding.Writable {
							t.Fatal("original local write Symbol differs")
						}
						seen++
					}
				}
				for _, child := range []*arrayGenericExpression{value.Left, value.Right, value.Value, value.Expression, value.Receiver} {
					expression(child)
				}
				if value.Arguments != nil {
					for index := range *value.Arguments {
						expression(&(*value.Arguments)[index])
					}
				}
			}
			for _, statement := range append(loop.Statements, helper.Statements...) {
				expression(statement.Initializer)
				expression(statement.Value)
			}
			if seen != len(uses) {
				t.Fatal("complete original local use inventory differs", seen, len(uses))
			}
		})
	}
}

func TestSystemQueryLocalFamily38o(t *testing.T) {
	for _, width := range []string{"f32", "f64"} {
		for _, name := range []string{"literal", "view", "imported", "same-owner"} {
			t.Run(width+"-"+name, func(t *testing.T) {
				r := systemScalarRequest38g(t)
				r.Files[0].Content = strings.ReplaceAll(r.Files[0].Content, "f32", width)
				largest, tiny := "3.4028234663852886e38", "1.401298464324817e-45"
				if width == "f64" {
					largest, tiny = "1.7976931348623157e308", "4.9406564584124654e-324"
				}
				replacement := "const one:Scalar=1, negative:Scalar=-0; let amount:Scalar=1.25e-2; const largest:Scalar=" + largest + "; const tiny:Scalar=" + tiny + "; const inferred=1.25e-2; amount*=one; amount/=dt; amount-=negative; amount=literalGain(amount); p.x=amount"
				if name == "literal" {
					r.Files[0].Content = "type Scalar=" + width + ";\n" + r.Files[0].Content + "\nfunction literalGain(v:Scalar):Scalar{const unit:Scalar=1; let current:Scalar=-0; current+=v; return current;}\n"
				}
				if name != "literal" {
					replacement = "const before=p.x; const result=update(p,v,dt); p.x=dt; p.x=result+result; p.y=before"
					helper := "function update(p:Write<Position>,v:Read<Velocity>,dt:" + width + "):" + width + "{const before=p.x; let delta:" + width + "=v.x*dt; delta+=before; p.x=delta; return before;}\n"
					if name == "imported" {
						r.Files[0].Content = "import {update as change} from './helpers';\n" + r.Files[0].Content
						helper = "import {Position,Velocity} from './main';\nexport " + helper
						r.Files = append(r.Files, inputFile{Path: "/src/helpers.ts", Content: helper})
						replacement = strings.Replace(replacement, "update(p,v,dt)", "change(p,v,dt)", 1)
					} else {
						r.Files[0].Content += helper
					}
					if name == "same-owner" {
						replacement = strings.Replace(replacement, "update(p,v,dt)", "update(p,p,dt)", 1)
						r.Files[0].Content = strings.Replace(r.Files[0].Content, "v:Read<Velocity>", "v:Read<Position>", 1)
					}
				}
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "p.x += scaled(v.x, dt)", replacement, 1)
				official, _, _, failure := newOwnedSystemProgram(r)
				if failure != nil {
					t.Fatal(failure)
				}
				if codes := systemDiagnosticCodes38c(t, official); len(codes) != 0 {
					t.Fatal("official family diagnostics", codes)
				}
				packet, e, partial := systemScalarRun38g(t, r, "locals-"+width+"-"+name)
				if e != nil {
					t.Fatal(e, partial)
				}
				if len(packet.Payload.Systems[0].Loops[0].LocalBindings) != map[bool]int{true: 6, false: 2}[name == "literal"] {
					t.Fatal("complete local family directory differs")
				}
				if name == "literal" {
					if packet.Payload.Systems[0].Loops[0].LocalBindings[5].DeclaredType != "f64" || packet.Payload.Systems[0].Loops[0].Statements[5].Initializer.Type != "f64" {
						t.Fatal("inferred fractional default must retain f64")
					}
					expected := []string{"3f800000", "80000000", "3c4ccccd", "7f7fffff", "00000001"}
					if width == "f64" {
						expected = []string{"3ff0000000000000", "8000000000000000", "3f8999999999999a", "7fefffffffffffff", "0000000000000001"}
					}
					for index, bits := range expected {
						if packet.Payload.Systems[0].Loops[0].Statements[index].Initializer.Bits != bits {
							t.Fatal("independent fixed IEEE literal oracle differs", width, index, bits)
						}
					}
				}
			})
		}
	}
}

func TestSystemQueryLocalNegatives38o(t *testing.T) {
	cases := map[string]string{"const-write": "const x:f32=dt; x=dt; p.x=x", "uninitialized": "let x:f32; p.x=x", "scope": "{const x:f32=dt;} p.x=x", "width": "const x:f64=dt; p.x=x", "gc": "const x=new Box(); p.x=x.value", "capture": "const x=()=>dt; p.x=x()", "borrow": "const x=p; p.x=x.x", "overflow": "const x:f32=1e40; p.x=x", "f64-overflow": "const x:f64=1e400; p.x=dt", "inferred-narrowing": "const x=1.25; p.x=x", "tdz": "p.x=x; const x:f32=dt"}
	for name, replacement := range cases {
		t.Run(name, func(t *testing.T) {
			r := systemScalarRequest38g(t)
			r.Files[0].Content = strings.Replace(r.Files[0].Content, "p.x += scaled(v.x, dt)", replacement, 1)
			official, _, _, failure := newOwnedSystemProgram(r)
			if failure != nil {
				t.Fatal(failure)
			}
			codes := systemDiagnosticCodes38c(t, official)
			code := map[string]int{"const-write": 2588, "uninitialized": 2454, "scope": 2304, "tdz": 2448}[name]
			found := code == 0 && len(codes) == 0
			for _, actual := range codes {
				found = found || actual == code
			}
			if !found {
				t.Fatal("official local refusal diagnostic differs", name, code, codes)
			}
			_, e, partial := systemScalarRun38g(t, r, "locals-negative-"+name)
			if e == nil || partial != 0 {
				t.Fatal("unsupported local emitted facts", e, partial)
			}
		})
	}
}

func TestSystemQueryRetiredV5Rejected38o(t *testing.T) {
	raw := systemPublicRequest38c(t, systemScalarRequest38g(t))
	var output bytes.Buffer
	if e := runFrontendCommand(t.Context(), []string{"--system-query-facts-io-v5"}, bytes.NewReader(raw), &output); e == nil || output.Len() != 0 {
		t.Fatal("retired v5 entry point accepted", e)
	}
	var request systemQueryRequest
	if e := json.Unmarshal(raw, &request); e != nil {
		t.Fatal(e)
	}
	request.Version = 5
	old, e := arrayFactsCanonical(request)
	if e != nil {
		t.Fatal(e)
	}
	if e := runFrontendCommand(t.Context(), []string{"--system-query-facts-io-v6"}, bytes.NewReader(append(old, '\n')), &output); e == nil || output.Len() != 0 {
		t.Fatal("retired v5 request accepted", e)
	}
}

func TestSystemQueryRefreshFixtures38o(t *testing.T) {
	root := os.Getenv("TSFG_QUERY38_REPLAY")
	if root == "" {
		t.Skip("fixture refresh not requested")
	}
	schema, e := systemQuerySchemaID()
	if e != nil {
		t.Fatal(e)
	}
	dirs, e := os.ReadDir(root)
	if e != nil {
		t.Fatal(e)
	}
	for _, dir := range dirs {
		if !dir.IsDir() || !strings.HasPrefix(dir.Name(), "system-query") {
			continue
		}
		paths, e := filepath.Glob(filepath.Join(root, dir.Name(), "*-request.json"))
		if e != nil {
			t.Fatal(e)
		}
		for _, path := range paths {
			if strings.Contains(path, "private-import-malformed") {
				continue
			}
			data, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			var request systemQueryRequest
			if e = json.Unmarshal(data, &request); e != nil {
				t.Fatal(e)
			}
			request.Version, request.SchemaHash = 6, schema
			raw, e := arrayFactsCanonical(request)
			if e != nil {
				t.Fatal(e)
			}
			raw = append(raw, '\n')
			var output bytes.Buffer
			if e = runFrontendCommand(t.Context(), []string{"--system-query-facts-io-v6"}, bytes.NewReader(raw), &output); e != nil {
				t.Fatal(path, e)
			}
			if e = os.WriteFile(path, raw, 0600); e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(strings.TrimSuffix(path, "-request.json")+"-facts.json", output.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			t.Log("actual Source fixture refreshed", path)
		}
	}
}
