// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/microsoft/typescript-go/internal/json"
)

func TestGenericPackageIOBoundedPolicies(t *testing.T) {
	t.Parallel()
	inner, outer := genericPackageJSONPolicy(), genericPackageIOPolicy()
	for _, data := range [][]byte{[]byte("null"), []byte(`"\u0000"`), []byte("\"" + strings.Repeat("x", genericPackageMaxString+1) + "\"")} {
		if genericJSONPreflight(data, inner) == nil || genericJSONPreflight(data, outer) != nil {
			t.Fatal("outer allowances changed inner policy or remained narrowed")
		}
	}
	for _, test := range []struct {
		policy genericJSONPolicy
		depth  int
	}{{inner, 160}, {outer, 162}} {
		valid := []byte(strings.Repeat("[", test.depth) + "0" + strings.Repeat("]", test.depth))
		if err := genericJSONPreflight(valid, test.policy); err != nil {
			t.Fatal(err)
		}
		bad := []byte(strings.Repeat("[", test.depth+1) + "0" + strings.Repeat("]", test.depth+1))
		if genericJSONPreflight(bad, test.policy) == nil {
			t.Fatal("JSON depth was not bounded")
		}
	}
	tokens := []byte("[" + strings.Repeat("0,", outer.maxTokens-2) + "0]")
	if genericJSONPreflight(tokens, outer) == nil {
		t.Fatal("outer token budget was not enforced")
	}
	tooLarge := []byte("\"" + strings.Repeat("x", (3<<20)+1) + "\"")
	if genericJSONPreflight(tooLarge, outer) == nil {
		t.Fatal("outer source string cap was not enforced")
	}
}

type genericIOFailReader struct{ err error }

func (r genericIOFailReader) Read([]byte) (int, error) { return 0, r.err }

type genericIOFailWriter struct {
	err   error
	short bool
}

func (w genericIOFailWriter) Write(p []byte) (int, error) {
	if w.short {
		return len(p) - 1, nil
	}
	return 0, w.err
}

func TestGenericPackageIOCancellationAndIOFailures(t *testing.T) {
	t.Parallel()
	request := genericIOCanonicalForTest(t, genericIOExportInputForTest())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if result, err := handleGenericPackageCandidate(ctx, request); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal("cancellation became a user error response")
	}
	deadline, stop := context.WithDeadline(t.Context(), time.Unix(0, 0))
	defer stop()
	if result, err := handleGenericPackageCandidate(deadline, request); !errors.Is(err, context.DeadlineExceeded) || result != nil {
		t.Fatal("deadline became a user error response")
	}
	inner, stop := context.WithCancel(t.Context())
	defer stop()
	controlled := &genericCancelOnPoll{Context: inner, cancel: stop}
	controlled.remaining.Store(4)
	if result, err := handleGenericPackageCandidate(controlled, request); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal("controlled producer cancellation was swallowed")
	}
	sentinel := errors.New("owned test IO failure")
	var output bytes.Buffer
	if err := runGenericPackageCandidate(t.Context(), genericIOFailReader{err: sentinel}, &output); !errors.Is(err, sentinel) || output.Len() != 0 {
		t.Fatal("read failure emitted output")
	}
	if err := runGenericPackageCandidate(t.Context(), bytes.NewReader(request), genericIOFailWriter{err: sentinel}); !errors.Is(err, sentinel) {
		t.Fatal("write failure was swallowed")
	}
	if err := runGenericPackageCandidate(t.Context(), bytes.NewReader(request), genericIOFailWriter{short: true}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal("short write was swallowed")
	}
	want, err := handleGenericPackageCandidate(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if err := runGenericPackageCandidate(t.Context(), bytes.NewReader(request), &output); err != nil || !bytes.Equal(output.Bytes(), want) {
		t.Fatal("wrapper changed canonical single-response bytes")
	}
}

func TestGenericPackageIORejectsVFSFileDirectoryConflicts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, source string
		declarations []string
	}{
		{"source parent", "/original/main.ts", []string{"/original/main.ts/lib.d.ts"}},
		{"declaration parent", "/decls/core.d.ts/main.ts", []string{"/decls/core.d.ts"}},
		{"env parent first", "/original/main.ts", []string{"/decls/core.d.ts", "/decls/core.d.ts/nested.d.ts"}},
		{"env child first", "/original/main.ts", []string{"/decls/core.d.ts/nested.d.ts", "/decls/core.d.ts"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			r := genericIOExportInputForTest()
			r.Source.Path = test.source
			r.Environment.Files = make([]genericPackageDeclarationDTO, len(test.declarations))
			for i, path := range test.declarations {
				r.Environment.Files[i] = genericPackageDeclarationDTO{Path: path, Content: scalarDeclarations, Digest: genericTextDigest(scalarDeclarations)}
			}
			d := genericIODiagnosticForTest(t, genericIOInvokeForTest(t, genericIOCanonicalForTest(t, r), "error"))
			if d.Kind != "package-input" || d.Detail != "VFS file/directory conflict" || d.Primary != nil {
				t.Fatalf("VFS conflict was not rejected before construction: %+v", d)
			}
		})
	}
	_, _, packageBytes := genericIOProducedPackageForTest(t)
	g, err := decodeGenericPackage(packageBytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/original/main.ts/nested.d.ts"} {
		wire := genericPackageWireForTest(t, packageBytes)
		wire.Payload.Environment.Files[0].Path = path
		bad := genericPackageSignedBytesForTest(t, wire)
		request := genericIOCanonicalForTest(t, genericPackageIOInstantiateRequest{Family: genericPackageIOFamily, Version: 1, Operation: "instantiate", Package: bad, Roots: []genericPackageRoot{genericPackageRootForTest(t, g, "route", "boolean")}})
		d := genericIODiagnosticForTest(t, genericIOInvokeForTest(t, request, "error"))
		if d.Kind != "package-input" || d.Detail != "VFS file/directory conflict" {
			t.Fatal("fresh factory file/directory conflict was accepted")
		}
	}
}

type genericIOCancelWriter struct {
	cancel context.CancelFunc
	err    error
}

func (w genericIOCancelWriter) Write(p []byte) (int, error) { w.cancel(); return len(p), w.err }

func TestGenericPackageIOCancellationDuringWrite(t *testing.T) {
	t.Parallel()
	request := genericIOCanonicalForTest(t, genericIOExportInputForTest())
	for _, ioErr := range []error{nil, errors.New("owned writer failure")} {
		ctx, cancel := context.WithCancel(t.Context())
		err := runGenericPackageCandidate(ctx, bytes.NewReader(request), genericIOCancelWriter{cancel: cancel, err: ioErr})
		cancel()
		want := ioErr
		if want == nil {
			want = context.Canceled
		}
		if !errors.Is(err, want) {
			t.Fatalf("writer cancellation/IO priority changed: %v", err)
		}
	}
}

func TestGenericPackageIOFactoryASTDoesNotOccupyVFSPath(t *testing.T) {
	t.Parallel()
	r := genericIOExportInputForTest()
	r.Environment.Files[0].Path = "/owned/definitions.ts/extra.d.ts"
	response := genericIOInvokeForTest(t, genericIOCanonicalForTest(t, r), "ok")
	packageBytes := bytes.Clone(genericIOFieldsForTest(t, response)["value"])
	g, err := decodeGenericPackage(packageBytes)
	if err != nil {
		t.Fatal(err)
	}
	roots := []genericPackageRoot{genericPackageRootForTest(t, g, "route", "boolean"), genericPackageRootForTest(t, g, "route", "number")}
	request := genericIOCanonicalForTest(t, genericPackageIOInstantiateRequest{Family: genericPackageIOFamily, Version: 1, Operation: "instantiate", Package: packageBytes, Roots: roots})
	response = genericIOInvokeForTest(t, request, "ok")
	var facts genericPackageFacts
	if err := json.Unmarshal(genericIOFieldsForTest(t, response)["value"], &facts); err != nil {
		t.Fatal(err)
	}
	if len(facts.Instances) != 4 || genericPackageFactForTest(t, facts, facts.Roots[0]).ReturnType != "bool" || genericPackageFactForTest(t, facts, facts.Roots[1]).ReturnType != "f64" {
		t.Fatal("factory AST depended on a placeholder VFS source file")
	}
	if g.Environment.Files[0].Path != r.Environment.Files[0].Path {
		t.Fatal("legal environment namespace was rewritten")
	}
}

func TestGenericPackageIOFreshPathSelectionIsDeterministic(t *testing.T) {
	t.Parallel()
	r := genericIOExportInputForTest()
	r.Source.Path = "/owned/definitions.ts"
	r.Environment.Files = make([]genericPackageDeclarationDTO, 8)
	for i := range r.Environment.Files {
		content := "// explicit declaration input\n"
		if i == 0 {
			content = scalarDeclarations
		}
		r.Environment.Files[i] = genericPackageDeclarationDTO{Path: "/owned/definitions-" + strconv.Itoa(i+1) + ".ts/extra.d.ts", Content: content, Digest: genericTextDigest(content)}
	}
	response := genericIOInvokeForTest(t, genericIOCanonicalForTest(t, r), "ok")
	packageBytes := bytes.Clone(genericIOFieldsForTest(t, response)["value"])
	g, err := decodeGenericPackage(packageBytes)
	if err != nil {
		t.Fatal(err)
	}
	request := g.request(genericDefinitionNamed(t, g, "route").Key, genericType{Kind: "boolean"})
	tree := genericTreeForTest(t, g, request)
	if tree.fileName != "/owned/definitions-9.ts" {
		t.Fatalf("did not choose the first conflict-free candidate: %s", tree.fileName)
	}
	for i, j := 0, len(g.Environment.Files)-1; i < j; i, j = i+1, j-1 {
		g.Environment.Files[i], g.Environment.Files[j] = g.Environment.Files[j], g.Environment.Files[i]
	}
	other := genericTreeForTest(t, g, request)
	if tree.fileName != other.fileName {
		t.Fatal("declaration order changed the internal source path")
	}
	program := newGenericProgram(g, tree, tree.text.String())
	if text, ok := program.Host().FS().ReadFile(tree.fileName); !ok || text != "" {
		t.Fatal("internal FileExists placeholder carried source text")
	}
	if program.Host().FS().FileExists(g.Source.Path) {
		t.Fatal("fresh VFS retained the original definition source")
	}
	if program.GetSourceFile(tree.fileName) == nil {
		t.Fatal("official root loader did not consume the factory AST")
	}
	roots := []genericPackageRoot{genericPackageRootForTest(t, g, "route", "boolean"), genericPackageRootForTest(t, g, "route", "number")}
	input := genericIOCanonicalForTest(t, genericPackageIOInstantiateRequest{Family: genericPackageIOFamily, Version: 1, Operation: "instantiate", Package: packageBytes, Roots: roots})
	response = genericIOInvokeForTest(t, input, "ok")
	var facts genericPackageFacts
	if err := json.Unmarshal(genericIOFieldsForTest(t, response)["value"], &facts); err != nil {
		t.Fatal(err)
	}
	if len(facts.Instances) != 4 || genericPackageFactForTest(t, facts, facts.Roots[0]).ReturnType != "bool" || genericPackageFactForTest(t, facts, facts.Roots[1]).ReturnType != "f64" {
		t.Fatal("fresh path selection changed official instance facts")
	}
}
