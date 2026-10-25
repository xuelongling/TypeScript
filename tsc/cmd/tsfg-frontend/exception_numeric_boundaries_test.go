// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExceptionNumericFactsDescriptorTarget(t *testing.T) {
	var descriptor struct {
		Defs struct {
			Input struct {
				Required []string `json:"required"`
			} `json:"input"`
			Declarations struct {
				Properties struct {
					Version struct {
						Enum []string `json:"enum"`
					} `json:"version"`
				} `json:"properties"`
			} `json:"declarations"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(exceptionNumericDescriptor, &descriptor); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, key := range descriptor.Defs.Input.Required {
		found = found || key == "target"
	}
	if !found {
		t.Fatal("v2 descriptor must require its explicit scalar target")
	}
	if !reflect.DeepEqual(descriptor.Defs.Declarations.Properties.Version.Enum, []string{"minimal-integers-v3", "minimal-floats-v4", "minimal-conversions-v5"}) {
		t.Fatal("v2 descriptor must not advertise the target-free legacy profile")
	}
	r := scalarRequest("export function entry(value:i32):i32{return value;}")
	input := nullableFactsBoundaryCanonical(t, map[string]any{"family": "tsfg.exception-facts-request-candidate", "version": 2, "schemaHash": exceptionNumericFactsSchema, "input": r})
	var out bytes.Buffer
	err := runFrontendCommand(t.Context(), []string{"--exception-facts-io-v2"}, bytes.NewReader(input), &out)
	if err == nil || !strings.Contains(err.Error(), "[r03-exception-facts-input]") || out.Len() != 0 {
		t.Fatalf("target-free legacy profile must reject before output: %v", err)
	}
}

func TestExceptionNumericFactsTypedAbsence(t *testing.T) {
	for _, row := range []struct{ name, source string }{
		{"return", "export function entry():i32|undefined{return undefined;}"},
		{"argument", "function accept(value:i32|undefined):i32|undefined{return value;}export function entry():i32|undefined{return accept(undefined);}"},
		{"initializer", "export function entry():i32|undefined{const value:i32|undefined=undefined;return value;}"},
	} {
		t.Run(row.name, func(t *testing.T) {
			r := integerRequest(row.source)
			p := throwBoundaryProgramForTest(t, r)
			throwBoundaryDiagnosticsForTest(t, p)
			file := p.GetSourceFile(r.RootFile)
			c, release := p.GetTypeCheckerForFileExclusive(t.Context(), file)
			defer release()
			var leaf *ast.Node
			last := file.Statements.Nodes[len(file.Statements.Nodes)-1].AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
			if row.name == "initializer" {
				leaf = last[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0].AsVariableDeclaration().Initializer
			} else {
				leaf = last[0].AsReturnStatement().Expression
				if row.name == "argument" {
					leaf = leaf.AsCallExpression().Arguments.Nodes[0]
				}
			}
			actual := c.GetSymbolAtLocation(leaf)
			if leaf.Kind != ast.KindIdentifier || leaf.Text() != "undefined" || actual == nil || len(actual.Declarations) != 0 || c.GetTypeAtLocation(leaf).Flags()&checker.TypeFlagsUndefined == 0 {
				t.Fatal("official built-in typed absence")
			}
			var first []byte
			for _, version := range []int{1, 2} {
				schema, flag := exceptionFactsSchema, "--exception-facts-io-v1"
				if version == 2 {
					schema, flag = exceptionNumericFactsSchema, "--exception-facts-io-v2"
				}
				input := nullableFactsBoundaryCanonical(t, map[string]any{"family": "tsfg.exception-facts-request-candidate", "version": version, "schemaHash": schema, "input": r})
				var out bytes.Buffer
				err := runFrontendCommand(t.Context(), []string{flag}, bytes.NewReader(input), &out)
				if root := os.Getenv("TSFG_EXCEPTION_NUMERIC_CAPTURE_ROOT"); root != "" {
					dir := filepath.Join(root, "typed-absence", row.name, flag[2:])
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					for name, data := range map[string][]byte{"exception-request.json": input, "package.json": out.Bytes()} {
						if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
							t.Fatal(err)
						}
					}
				}
				if err != nil {
					t.Fatalf("v%d existing typed absence: %v", version, err)
				}
				var packet exceptionFactsPackage
				var facts response
				if err := json.Unmarshal(out.Bytes(), &packet); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(packet.Payload.Facts, &facts); err != nil {
					t.Fatal(err)
				}
				body := facts.Functions[len(facts.Functions)-1].Body
				var value *expression
				if row.name == "initializer" {
					value = body[0].Initializer
				} else {
					value = body[0].Expression
					if row.name == "argument" {
						value = &(*value.Arguments)[0]
					}
				}
				if value.Kind != "absence" || value.Type != "i32" || value.Absence != "undefined" || value.Symbol != "" || value.Range != throwBoundaryNodeForTest(t, file, leaf) {
					t.Fatal("original typed-absence payload and UTF16")
				}
				if version == 1 {
					first = bytes.Clone(packet.Payload.Facts)
				} else if !bytes.Equal(first, packet.Payload.Facts) {
					t.Fatal("existing complete facts changed across v1/v2")
				}
			}
		})
	}
}

func TestExceptionNumericFactsCurrentCatchAndOperators(t *testing.T) {
	for _, row := range []struct{ name, source string }{
		{"current-catch", "// 😀\r\nexport function rewrite(value:i32):i32{try{throw undefined;}catch(caught){caught=value;const viewed:i32=caught as i32;return viewed;}}\r\nexport function erase(value:i32):i32{try{throw value;}catch(caught){caught=undefined;throw caught;}}"},
		{"shadow-undefined", "// 😀\r\nexport function parameter(undefined:i32):i32{throw undefined;}\r\nexport function local(value:i32):i32{const undefined:i32=value;throw undefined;}"},
		{"numeric-families", "// 😀\r\nexport function operations(left:i32,right:i32):i32{try{left+right;left-right;left*right;left/right;left%right;left&right;left|right;left^right;left<<right;left>>right;left>>>right;left<right;left<=right;left>right;left>=right;left===right;left!==right;+left;-left;~left;throw undefined;}catch(caught){const viewed:i32=caught as i32;const alias:i32=viewed;alias+right;return alias;}finally{const done:bool=true;}}"},
	} {
		t.Run(row.name, func(t *testing.T) {
			_, facts := exceptionNumericPacketForTest(t, row.name, integerRequest(row.source))
			if row.name == "shadow-undefined" {
				for _, f := range facts.Functions {
					last := f.Body[len(f.Body)-1].Expression
					if last.Kind != "reference" || last.Type != "i32" || last.Symbol == "" {
						t.Fatal("declared same-spelled undefined cannot become the built-in singleton")
					}
				}
			}
			if row.name == "current-catch" {
				changed := facts
				data, err := json.Marshal(facts)
				if err != nil {
					t.Fatal(err)
				}
				if err = json.Unmarshal(data, &changed); err != nil {
					t.Fatal(err)
				}
				// A stale original throw must not replace the current catch cell.
				changed.Functions[1].Body[0].Exception.CatchClause.Block.Body[1].Expression.Symbol = changed.Functions[1].Parameters[0].ID
				if exceptionNumericFactsOracle(t, integerRequest(row.source), changed) == nil {
					t.Fatal("stale thrown operand accepted")
				}
			}
		})
	}
}

func TestExceptionNumericFactsVersionAndRefusal(t *testing.T) {
	for _, name := range []string{"v1-new-source", "v1-with-v2-schema", "v2-with-v1-schema", "old-v2-schema", "wrong-target", "wrong-profile", "no-target", "unsupported-assertion", "illegal-operator", "unknown-name", "readonly-write", "over-byte-budget"} {
		t.Run(name, func(t *testing.T) {
			r := integerRequest(exceptionNumericSource)
			version := 2
			schema := exceptionNumericFactsSchema
			flag := "--exception-facts-io-v2"
			category := "schema"
			switch name {
			case "v1-new-source":
				version, schema, flag, category = 1, exceptionFactsSchema, "--exception-facts-io-v1", "source"
			case "v1-with-v2-schema":
				version, flag = 1, "--exception-facts-io-v1"
			case "v2-with-v1-schema":
				schema = exceptionFactsSchema
			case "old-v2-schema":
				schema = "sha256:29a53c8f370274148e0dbd39d8505ad7b436a8ca711d0439cac5558f77c80f39"
			case "wrong-target":
				r.Target.Triple = "unknown"
				category = "input"
			case "wrong-profile":
				r.Declarations.Version = "unknown"
				category = "input"
			case "no-target":
				r.Target = nil
				category = "input"
			case "unsupported-assertion":
				r = integerRequest("export function entry(value:i32):i32{try{throw undefined;}catch(caught){const text=caught as string;return value;}}")
				category = "source"
			case "illegal-operator":
				r = integerRequest("export function entry(value:i32):i32{try{throw undefined;}catch(caught){true+false;return value;}}")
				category = "source"
			case "unknown-name":
				r = integerRequest("export function entry():i32{return absent as i32;}")
				category = "source"
			case "readonly-write":
				r = integerRequest("export function entry(value:i32):i32{const copy:i32=value;copy=value;return copy;}")
				category = "source"
			case "over-byte-budget":
				category = "limit"
			}
			input := nullableFactsBoundaryCanonical(t, map[string]any{"family": "tsfg.exception-facts-request-candidate", "version": version, "schemaHash": schema, "input": r})
			if name == "over-byte-budget" {
				input = bytes.Repeat([]byte(" "), exceptionFactsMaxBytes+1)
			}
			var out bytes.Buffer
			err := runFrontendCommand(t.Context(), []string{flag}, bytes.NewReader(input), &out)
			if err == nil || !strings.Contains(err.Error(), "[r03-exception-facts-"+category+"]") || out.Len() != 0 {
				t.Fatalf("correct-layer refusal/no partial: %v %s", err, out.Bytes())
			}
			t.Logf("%s actual %v, output0", name, err)
		})
	}
}

// Version selection owns vocabulary only. Existing complete multi-module facts
// are preserved byte-for-byte, including initializers, imports and all functions.
func TestExceptionNumericFactsCompleteModules(t *testing.T) {
	r, expected, prior := exceptionFinallyLegacy(t, "09")
	var original response
	if err := json.Unmarshal(nullableFactsBoundaryCanonical(t, expected), &original); err != nil {
		t.Fatal(err)
	}
	exports := exceptionCorpusOfficialForTest(t, "module-initialization", r, original, expected, prior)
	var first []byte
	for _, version := range []int{1, 2} {
		schema, flag := exceptionFactsSchema, "--exception-facts-io-v1"
		if version == 2 {
			schema, flag = exceptionNumericFactsSchema, "--exception-facts-io-v2"
		}
		input := nullableFactsBoundaryCanonical(t, map[string]any{"family": "tsfg.exception-facts-request-candidate", "version": version, "schemaHash": schema, "input": r})
		var out bytes.Buffer
		if err := runFrontendCommand(t.Context(), []string{flag}, bytes.NewReader(input), &out); err != nil {
			t.Fatal(err)
		}
		var packet exceptionFactsPackage
		if err := json.Unmarshal(out.Bytes(), &packet); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(packet.Payload.Facts, nullableFactsBoundaryCanonical(t, expected)) || !bytes.Equal(nullableFactsBoundaryCanonical(t, packet.Payload.Exports), nullableFactsBoundaryCanonical(t, exports)) {
			t.Fatal("complete independent multi-module facts/exports")
		}
		if version == 1 {
			first = nullableFactsBoundaryCanonical(t, packet.Payload)
		} else if !reflect.DeepEqual(first, nullableFactsBoundaryCanonical(t, packet.Payload)) {
			t.Fatal("version must not crop modules/functions/initialization")
		}
		if root := os.Getenv("TSFG_EXCEPTION_NUMERIC_CAPTURE_ROOT"); root != "" {
			directory := filepath.Join(root, "complete-modules", schema[len("sha256:"):])
			if err := os.MkdirAll(directory, 0755); err != nil {
				t.Fatal(err)
			}
			for name, data := range map[string][]byte{"exception-request.json": input, "package.json": out.Bytes(), "tested-input-set.json": nullableFactsBoundaryCanonical(t, map[string]any{"rootFile": r.RootFile, "files": r.Files})} {
				if err := os.WriteFile(filepath.Join(directory, name), data, 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}
