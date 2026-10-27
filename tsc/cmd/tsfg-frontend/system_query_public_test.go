// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"github.com/microsoft/typescript-go/internal/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func systemPublicRequest38c(t *testing.T, r request) []byte {
	t.Helper()
	schema, e := systemQuerySchemaID()
	if e != nil {
		t.Fatal(e)
	}
	in := systemQueryInput{SourceRoot: r.SourceRoot, RootFile: r.RootFile, Declarations: arrayFactsDeclarations{Version: r.Declarations.Version, Paths: append([]string{}, r.Declarations.Paths...)}, Config: arrayFactsConfig{Strict: true, NoEmit: true, AllowJS: false}, Target: &commonProgramTarget{Target: "windows-x86_64-msvc", Triple: "x86_64-pc-windows-msvc", LLVMDataLayout: "e-m:w-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128", PointerBits: "64"}, Files: []arrayFactsInputFile{}}
	for _, file := range r.Files {
		in.Files = append(in.Files, arrayFactsInputFile{Path: file.Path, Content: file.Content, SHA256: arrayFactsSHA([]byte(file.Content))})
	}
	slices.SortFunc(in.Files, func(a, b arrayFactsInputFile) int { return strings.Compare(a.Path, b.Path) })
	raw, e := arrayFactsCanonical(systemQueryRequest{Family: "tsfg.system-query-facts-request-candidate", Version: 3, SchemaHash: schema, Input: in})
	if e != nil {
		t.Fatal(e)
	}
	return append(raw, '\n')
}
func TestSystemQueryPublicFacts38c(t *testing.T) {
	for _, name := range []string{"architecture", "aliases", "duplicate", "complete", "wrapper-alias"} {
		t.Run(name, func(t *testing.T) {
			r := systemRequest38c(t)
			switch name {
			case "aliases":
				r.Files[0].Content = "import type {Q,Modes} from './aliases';\n" + strings.Replace(r.Files[0].Content, "Query<[Write<Position>, Read<Velocity>]>", "Q<Modes<Position>>", 1)
				r.Files = append(r.Files, inputFile{Path: "/src/aliases.ts", Content: "import type {Velocity} from './main';\nexport type W<T> = Write<T>;\nexport type R<T> = Read<T>;\nexport type Modes<T> = [W<T>,R<Velocity>];\nexport type Q<T extends unknown[]> = Query<T>;\n"})
			case "wrapper-alias":
				r.Files[0].Content = "type WPos = Write<Position>;\n" + strings.Replace(r.Files[0].Content, "Write<Position>", "WPos", 1)
			case "duplicate":
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "[Write<Position>, Read<Velocity>]", "[Read<Position>, Write<Position>, Read<Velocity>]", 1)
			case "complete":
				r.Files[0].Content += "\nclass Box { value:i32=1; }\nexport function unrelated(v:i32):i32{return v;}\n"
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
			if len(packet.Payload.Systems) != 1 {
				t.Fatal("original system inventory differs")
			}
			items := packet.Payload.Systems[0].Parameters[0].Query.Items
			if name == "duplicate" {
				if len(items) != 3 || items[0].Mode != "read" || items[1].Mode != "write" || items[0].ComponentType != items[1].ComponentType {
					t.Fatal("source tuple duplicates/order were changed")
				}
			} else if len(items) != 2 || items[0].Mode != "write" || items[1].Mode != "read" {
				t.Fatal("original access order differs")
			}
			if name == "aliases" {
				aliases := packet.Payload.Systems[0].Parameters[0].Query.Aliases
				if len(aliases) < 4 || items[0].ComponentSourcePath != "/src/main.ts" || packet.Payload.Systems[0].Parameters[0].Query.TupleSourcePath != "/src/aliases.ts" {
					t.Fatal("original cross-file substitution anchors lost", aliases)
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
			t.Logf("actual full original Program packet bytes=%d nodes=%d types=%d", out.Len(), len(packet.Payload.Nodes), len(packet.Payload.Types))
		})
	}
}
func TestSystemQueryCapabilityNegatives38c(t *testing.T) {
	for _, name := range []string{"noncomponent", "GC-component", "foreign-Read", "foreign-component", "profile-context", "request-schema"} {
		t.Run(name, func(t *testing.T) {
			r := systemRequest38c(t)
			switch name {
			case "noncomponent":
				r.Files[0].Content += "\nclass Box { value:i32=1; }\n"
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "Write<Position>", "Read<Box>", 1)
			case "GC-component":
				r.Files[0].Content += "\nclass Box {value:i32=1;}\n@component class Bad {declare payload:Box;}\n"
				r.Files[0].Content = strings.Replace(r.Files[0].Content, "Write<Position>", "Read<Bad>", 1)
			case "foreign-Read":
				r.Files[0].Content = "interface Read<T>{readonly __read:T;}\n" + r.Files[0].Content
			case "foreign-component":
				r.Files[0].Content = "function component(value: unknown, context: ClassDecoratorContext<unknown>): void {}\n" + r.Files[0].Content
			case "profile-context":
				r.Files[2].Content += "declare const lookalike:unknown;"
			}
			raw := systemPublicRequest38c(t, r)
			if name == "request-schema" {
				raw = bytes.Replace(raw, []byte("sha256:"), []byte("SHA256:"), 1)
			}
			var out bytes.Buffer
			e := runFrontendCommand(t.Context(), []string{"--system-query-facts-io-v3"}, bytes.NewReader(raw), &out)
			if e == nil || out.Len() != 0 {
				t.Fatal("invalid boundary emitted partial facts", e, out.Len())
			}
			t.Logf("original refusal: %v", e)
		})
	}
}

// The checker erases never, but the original alias syntax remains a union.
// This boundary must return a capability error without a partial packet.
func TestSystemQueryUnionNever38c(t *testing.T) {
	r := systemRequest38c(t)
	r.Files[0].Content = "type P = Position | never;\n" + strings.Replace(r.Files[0].Content, "Write<Position>", "Write<P>", 1)
	p, _, _, failure := newOwnedSystemProgram(r)
	if failure != nil {
		t.Fatal(failure)
	}
	if codes := systemDiagnosticCodes38c(t, p); len(codes) != 0 {
		t.Fatal("original union/never Source requires diagnostics0", codes)
	}
	raw := systemPublicRequest38c(t, r)
	if capture := os.Getenv("TSFG_QUERY38_CAPTURE"); capture != "" {
		if e := os.MkdirAll(capture, 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(capture, "union-never-request.json"), raw, 0600); e != nil {
			t.Fatal(e)
		}
	}
	var out bytes.Buffer
	e := runFrontendCommand(t.Context(), []string{"--system-query-facts-io-v3"}, bytes.NewReader(raw), &out)
	if e == nil || out.Len() != 0 || !strings.Contains(e.Error(), "original concrete component reference required") {
		t.Fatal("union/never must fail precisely without partial facts", e, out.Len())
	}
	t.Logf("original refusal: %v", e)
}
