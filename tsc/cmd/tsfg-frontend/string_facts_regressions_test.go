// SPDX-License-Identifier: Apache-2.0
// Three distinct public regressions: simultaneous errors, path closure, and function-only cardinality.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/microsoft/typescript-go/internal/ast"
	"strings"
	"testing"
)

type stringCancelErrorReader24 struct {
	cancel  context.CancelFunc
	failure error
	calls   int
}

func (r *stringCancelErrorReader24) Read(_ []byte) (int, error) {
	r.calls++
	r.cancel()
	return 0, r.failure
}

func TestStringFactsReadCancelPreservesConcurrentIOError(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	sentinel := errors.New("simultaneous reader IO failure")
	input := &stringCancelErrorReader24{cancel: cancel, failure: sentinel}
	output := &stringWriterProbe24{}
	err := runFrontendCommand(ctx, []string{"--string-facts-io-v1"}, input, output)
	if !errors.Is(err, context.Canceled) || !errors.Is(err, sentinel) || input.calls != 1 || output.calls != 0 {
		t.Fatalf("both causal errors and zero writes required: canceled=%v io=%v reads=%d writes=%d err=%v", errors.Is(err, context.Canceled), errors.Is(err, sentinel), input.calls, output.calls, err)
	}
}

func TestStringFactsRejectsDeclarationAncestorPath(t *testing.T) {
	const source = "export function entry(): string { return 'x'; }"
	input := stringBoundaryRequest24(t, source)
	input = stringBoundaryMutate24(t, input, func(r map[string]any) {
		body := r["input"].(map[string]any)
		body["rootFile"] = "/owned/minimal-scalars-v2.d.ts/main.ts"
		body["files"].([]any)[1].(map[string]any)["path"] = body["rootFile"]
	})
	// Prove this is a fully canonical, hash-correct request, not an identity rejection.
	var decoded struct {
		Input struct {
			RootFile string                                   `json:"rootFile"`
			Files    []struct{ Path, Content, SHA256 string } `json:"files"`
		} `json:"input"`
	}
	if err := json.Unmarshal(input, &decoded); err != nil {
		t.Fatal(err)
	}
	var graph map[string]any
	if err := json.Unmarshal(input, &graph); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(append(stringOracleCanonical24(t, graph), '\n'), input) || len(decoded.Input.Files) != 2 {
		t.Fatal("independent canonical/closure control")
	}
	for _, file := range decoded.Input.Files {
		if fmt.Sprintf("%x", sha256.Sum256([]byte(file.Content))) != file.SHA256 {
			t.Fatal("independent complete file hash")
		}
	}
	if decoded.Input.Files[0].Path != "/owned/minimal-scalars-v2.d.ts" || decoded.Input.Files[1].Path != decoded.Input.RootFile || !strings.HasPrefix(decoded.Input.RootFile, decoded.Input.Files[0].Path+"/") {
		t.Fatal("manual ancestor conflict absent")
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("invalid file ancestry panicked instead of input failure: %v", recovered)
		}
	}()
	failure := stringBoundaryFailure24(t, input, "input")
	if failure.Code != "r03-string-facts-input" || failure.Phase != "input" {
		t.Fatalf("private input boundary: %+v", failure)
	}
}

func TestStringFactsUnsupportedDeclarationAtFunctionLimit(t *testing.T) {
	var source strings.Builder
	for i := 0; i < 64; i++ {
		fmt.Fprintf(&source, "export function bounded%02d(): string { return ''; }\n", i)
	}
	const extra = "const unselected = 'extra';"
	source.WriteString(extra)
	if source.Len() > 4096 {
		t.Fatal("bounded Source premise exceeded")
	}
	input := stringBoundaryRequest24(t, source.String())
	var r struct{ Input request }
	if err := json.Unmarshal(input, &r); err != nil {
		t.Fatal(err)
	}
	official := throwBoundaryProgramForTest(t, r.Input)
	throwBoundaryDiagnosticsForTest(t, official)
	file := official.GetSourceFile(r.Input.RootFile)
	if len(file.Statements.Nodes) != 65 {
		t.Fatal("official complete statement cardinality")
	}
	for i, node := range file.Statements.Nodes {
		if i < 64 {
			if node.Kind != ast.KindFunctionDeclaration || len(node.AsFunctionDeclaration().Parameters.Nodes) != 0 {
				t.Fatal("official function cardinality differs from 64")
			}
		} else if node.Kind != ast.KindVariableStatement {
			t.Fatal("official legal extra const declaration absent")
		}
	}
	check, release := official.GetTypeCheckerForFileExclusive(t.Context(), file)
	exportCount := len(check.GetExportsOfModule(file.AsNode().Symbol()))
	release()
	if exportCount != 64 {
		t.Fatal("official full exports must be exactly the 64 functions")
	}
	failure := stringBoundaryFailure24(t, input, "capability")
	if failure.Code != "r03-string-facts-capability" || failure.RuleID != "r01-ts-baseline" || failure.Phase != "capability" || len(failure.Diagnostics) != 1 {
		t.Fatalf("unselected declaration must remain capability: %+v", failure)
	}
	at := strings.Index(source.String(), extra)
	if failure.Diagnostics[0].Range.Start != at || failure.Diagnostics[0].Range.End != at+len(extra) {
		t.Fatalf("capability must anchor the complete unselected const: %+v", failure.Diagnostics)
	}
}
