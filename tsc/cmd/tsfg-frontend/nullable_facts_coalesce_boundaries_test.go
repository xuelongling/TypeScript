// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/json"
)

func TestNullableFactsCommandNullishCoalesceBoundaries(t *testing.T) {
	const schema = "sha256:24a04b0b595cae59ccf52a939f4671cea0369c46bd705f56726d028ab1df5a71"
	const entry = "\nexport function entry(value: i32): i32 { return coalesce(value, value); }"
	const fallback = "function fallback(value: i32): i32 { return value; }\n"
	canonical := func(value any) []byte { return nullableFactsBoundaryCanonical(t, value) }
	digest := func(value []byte) string {
		sum := sha256.Sum256(value)
		return "sha256:" + hex.EncodeToString(sum[:])
	}
	acquire := func(t *testing.T, source string) (request, response, []byte, []byte) {
		t.Helper()
		r := integerRequest(source)
		before := canonical(r)
		if !t.Run("officialDiagnostics", func(t *testing.T) { nullableBoundaryOfficialForTest(t, r, 0) }) {
			t.Fatal("official diagnostics failed; no later grammar/export credit")
		}
		legacy, raw := nullableBoundaryCommandForTest(t, r)
		if !bytes.Equal(before, canonical(r)) {
			t.Fatal("official/legacy command changed original request")
		}
		input := canonical(map[string]any{"family": "tsfg.nullable-facts-request-candidate", "version": 3, "schemaHash": schema, "input": r})
		return r, legacy, raw, input
	}
	private := func(t *testing.T, input []byte, flag string) ([]byte, error) {
		t.Helper()
		before := bytes.Clone(input)
		output, err := nullableFactsBoundaryRun(t.Context(), []string{flag}, input)
		if !bytes.Equal(input, before) {
			t.Fatal("private command modified caller request bytes")
		}
		if err != nil && len(output) != 0 {
			t.Fatalf("failed private command published partial package: %v raw=%s", err, output)
		}
		return output, err
	}
	fields := func(t *testing.T, raw []byte, keys ...string) map[string]json.Value {
		t.Helper()
		var value map[string]json.Value
		if err := json.Unmarshal(raw, &value); err != nil || len(value) != len(keys) || !bytes.Equal(canonical(value), raw) {
			t.Fatalf("package object is not exact JCS: %v raw=%s", err, raw)
		}
		for _, key := range keys {
			if _, ok := value[key]; !ok {
				t.Fatalf("package object omitted %s", key)
			}
		}
		return value
	}
	checkPackage := func(t *testing.T, r request, legacyRaw, input, output []byte) {
		t.Helper()
		packet := fields(t, output, "family", "version", "schemaHash", "requestId", "packageId", "payload")
		if string(packet["family"]) != "\"tsfg.nullable-facts-package-candidate\"" || string(packet["version"]) != "3" || !bytes.Equal(packet["schemaHash"], canonical(schema)) || !bytes.Equal(packet["requestId"], canonical(digest(append([]byte("tsfg.nullable-facts.request.v3\x00"), input...)))) {
			t.Fatal("private family/version/schema/request identity changed")
		}
		payload := fields(t, packet["payload"], "inputIdentity", "files", "facts", "exports")
		if !bytes.Equal(packet["packageId"], canonical(digest(append([]byte("tsfg.nullable-facts.package.v3\x00"+schema+"\x00"), packet["payload"]...)))) {
			t.Fatal("package does not bind complete canonical payload")
		}
		var identity map[string]json.Value
		if err := json.Unmarshal(canonical(r), &identity); err != nil {
			t.Fatal(err)
		}
		files := slices.Clone(r.Files)
		slices.SortFunc(files, func(a, b inputFile) int { return strings.Compare(a.Path, b.Path) })
		identityFiles, fileTable := []map[string]any{}, []map[string]any{}
		for _, file := range files {
			identityFiles = append(identityFiles, map[string]any{"path": file.Path, "content": file.Content})
			fileTable = append(fileTable, map[string]any{"path": file.Path, "digest": digest([]byte(file.Content)), "byteLength": len(file.Content), "length": len(utf16.Encode([]rune(file.Content))), "positionUnit": "utf16"})
		}
		identity["files"] = json.Value(canonical(identityFiles))
		legacy := json.Value(bytes.Clone(legacyRaw))
		if err := legacy.Canonicalize(); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(payload["inputIdentity"], canonical(digest(canonical(identity)))) || !bytes.Equal(payload["files"], canonical(fileTable)) || !bytes.Equal(payload["facts"], legacy) {
			t.Fatal("private package filtered/changed original complete files or legacy facts")
		}
		var facts response
		if err := json.Unmarshal(legacyRaw, &facts); err != nil {
			t.Fatal(err)
		}
		var exported function
		for _, fn := range facts.Functions {
			if fn.Name == "entry" {
				exported = fn
			}
		}
		source := r.Files[0].Content
		start := strings.Index(source, "export function entry")
		if start < 0 || exported.Symbol == "" || exported.ReturnType != "i32" || exported.ReturnPresence != nil {
			t.Fatal("positive lost original scalar entry")
		}
		declStart := start
		if start > 0 && source[start-1] == '\n' {
			declStart--
		}
		for _, p := range exported.Parameters {
			if p.Type != "i32" || p.Presence == nil || p.Presence.Null || p.Presence.Undefined || p.Presence.Optional || p.Presence.Default {
				t.Fatal("positive entry acquired a nullable/optional ABI")
			}
		}
		wantExports := []map[string]any{{"name": "entry", "target": exported.Symbol, "sourcePath": r.RootFile, "range": span{declStart, len(source)}, "definition": span{start + 15, start + 21}}}
		if !bytes.Equal(payload["exports"], canonical(wantExports)) {
			t.Fatal("private export table changed original entry binding/UTF16 range")
		}
	}
	positives := []struct{ name, source, expression string }{
		{"referenceRhs", "function coalesce(value: i32 | null, replacement: i32): i32 { return value ?? replacement; }" + entry, "value ?? replacement"},
		{"nullableCallLhs", "function echo(value: i32 | null): i32 | null { return value; }\nfunction coalesce(value: i32 | null, replacement: i32): i32 { return echo(value) ?? replacement; }" + entry, "echo(value) ?? replacement"},
		{"nestedCallArgument", fallback + "function coalesce(value: i32 | null, other: i32 | null, replacement: i32): i32 { return value ?? fallback(other ?? replacement); }\nexport function entry(value: i32): i32 { return coalesce(value, value, value); }", "value ?? fallback(other ?? replacement)"},
	}
	for _, tc := range positives {
		// All three are positive controls before any rejection is credited.
		if !t.Run(tc.name, func(t *testing.T) {
			r, legacy, legacyRaw, input := acquire(t, tc.source)
			before := canonical(r)
			if legacy.Status != "ok" || len(legacy.Diagnostics) != 0 {
				t.Fatalf("positive legacy prerequisite failed: %+v raw=%s", legacy, legacyRaw)
			}
			functions, names, definitions := map[string]function{}, map[string]function{}, map[string]symbol{}
			for _, fn := range legacy.Functions {
				functions[fn.Symbol], names[fn.Name] = fn, fn
			}
			for _, s := range legacy.Symbols {
				if s.ID == "" || definitions[s.ID].ID != "" {
					t.Fatal("positive definitions lost unique original IDs")
				}
				definitions[s.ID] = s
			}
			fn := names["coalesce"]
			if fn.Symbol == "" || len(fn.Parameters) < 2 || len(fn.Body) != 1 || fn.Body[0].Kind != "return" || fn.Body[0].Expression == nil {
				t.Fatal("positive coalesce declaration/return is incomplete")
			}
			for _, p := range fn.Parameters {
				if !reflect.DeepEqual(definitions[p.ID], p) {
					t.Fatal("parameter and complete owned symbol table differ")
				}
			}
			root := fn.Body[0].Expression
			if root.Kind != "shortCircuit" || root.Operator != "??" || sourceText(t, tc.source, root.Range) != tc.expression {
				t.Fatalf("positive lost exact original ?? structure/range: %+v", root)
			}
			counts := map[string]int{}
			var visit func(*expression)
			visit = func(e *expression) {
				if e == nil || e.Type != "i32" || e.Range.Start < 0 || e.Range.End > len(tc.source) || e.Range.Start >= e.Range.End || e.Operand != nil || e.Receiver != nil || e.Callee != nil {
					t.Fatal("positive expression lost original bounded typed shape")
				}
				counts[e.Kind]++
				switch e.Kind {
				case "shortCircuit":
					if e.Operator != "??" || e.Left == nil || e.Right == nil || e.Left == e.Right || e.PresenceFlow == nil || e.PresenceFlow.Null || e.PresenceFlow.Undefined || e.Left.PresenceFlow == nil || !e.Left.PresenceFlow.Null || e.Left.PresenceFlow.Undefined || e.Right.PresenceFlow != nil && (e.Right.PresenceFlow.Null || e.Right.PresenceFlow.Undefined) {
						t.Fatal("coalesce operands/result lost exact independent official flow")
					}
					visit(e.Left)
					visit(e.Right)
				case "reference":
					def := definitions[e.Symbol]
					if def.Kind != "parameter" || sourceText(t, tc.source, e.Range) != def.Name || e.Left != nil || e.Right != nil || e.Arguments != nil {
						t.Fatal("positive reference was rebound or expanded")
					}
				case "call":
					target := functions[e.Target]
					if target.Symbol == "" || e.Signature == nil || e.Arguments == nil || len(*e.Arguments) != len(target.Parameters) || !strings.HasPrefix(sourceText(t, tc.source, e.Range), target.Name+"(") {
						t.Fatal("positive call lost original declaration/arguments/range")
					}
					want := signature{ReturnType: target.ReturnType, ReturnPresence: target.ReturnPresence}
					for _, p := range target.Parameters {
						want.Parameters = append(want.Parameters, p.Type)
						want.Presence = append(want.Presence, p.Presence)
					}
					if !reflect.DeepEqual(e.Signature, &want) {
						t.Fatal("positive call signature differs from actual complete target")
					}
					for index := range *e.Arguments {
						visit(&(*e.Arguments)[index])
					}
				default:
					t.Fatal("positive coalesce was rewritten into an eager/write/group expression")
				}
			}
			visit(root)
			wantCounts := map[string]int{"shortCircuit": 1, "reference": 2}
			switch tc.name {
			case "referenceRhs":
				if root.Left.Symbol != fn.Parameters[0].ID || root.Right.Symbol != fn.Parameters[1].ID {
					t.Fatal("reference RHS lost original caller bindings")
				}
			case "nullableCallLhs":
				wantCounts["call"] = 1
				if root.Left.Kind != "call" || root.Left.Target != names["echo"].Symbol || (*root.Left.Arguments)[0].Symbol != fn.Parameters[0].ID || root.Right.Symbol != fn.Parameters[1].ID {
					t.Fatal("nullable call LHS was hoisted/rebound")
				}
			case "nestedCallArgument":
				wantCounts["shortCircuit"], wantCounts["reference"], wantCounts["call"] = 2, 3, 1
				if len(fn.Parameters) != 3 || root.Left.Symbol != fn.Parameters[0].ID || root.Right.Kind != "call" || root.Right.Target != names["fallback"].Symbol || root.Right.Arguments == nil || len(*root.Right.Arguments) != 1 {
					t.Fatal("nested RHS call lost original outer binding/order")
				}
				inner := (*root.Right.Arguments)[0]
				if inner.Kind != "shortCircuit" || inner.Left.Symbol != fn.Parameters[1].ID || inner.Right.Symbol != fn.Parameters[2].ID || sourceText(t, tc.source, inner.Range) != "other ?? replacement" {
					t.Fatal("recursive argument coalesce lost original bindings/range")
				}
			}
			if !reflect.DeepEqual(counts, wantCounts) {
				t.Fatalf("operands duplicated or eagerly rewritten: got=%v want=%v", counts, wantCounts)
			}
			output, err := private(t, input, "--nullable-facts-io-v3")
			if err != nil {
				t.Fatalf("positive private v3 failed after official/legacy PASS: %v", err)
			}
			checkPackage(t, r, legacyRaw, input, output)
			t.Logf("TSFG_NULLABLE_COALESCE_POSITIVE_REQUEST_V3 %s %s", tc.name, base64.StdEncoding.EncodeToString(input))
			t.Logf("TSFG_NULLABLE_COALESCE_POSITIVE_RESPONSE_V3 %s %s", tc.name, base64.StdEncoding.EncodeToString(output))
			if !bytes.Equal(before, canonical(r)) {
				t.Fatal("private acquisition changed original request DTO")
			}
		}) {
			return
		}
	}
	boundaryCases := []struct{ name, source, layer string }{
		{"mask1Supported", "function coalesce(value: i32 | undefined, replacement: i32): i32 { return value ?? replacement; }" + entry, "success"},
		{"mask3Supported", "function coalesce(value: i32 | null | undefined, replacement: i32): i32 { return value ?? replacement; }" + entry, "success"},
		{"nullableExport", "export function entry(value: i32 | null, replacement: i32): i32 { return value ?? replacement; }", "export"},
		{"parenthesizedRhsGrammar", "function coalesce(value: i32 | null, replacement: i32): i32 { return value ?? (replacement); }" + entry, "capability"},
		{"assignmentOperatorGrammar", "function coalesce(value: i32 | null, replacement: i32): i32 { return value ??= replacement; }" + entry, "capability"},
		{"nonnullableLhsSource", "function coalesce(value: i32, replacement: i32): i32 { return value ?? replacement; }" + entry, "source"},
		{"nullableRhsResultSource", "function coalesce(value: i32 | null, replacement: i32 | null): i32 | null { return value ?? replacement; }\nexport function entry(value: i32): i32 { return value; }", "source"},
	}
	for _, tc := range boundaryCases {
		t.Run(tc.name, func(t *testing.T) {
			r, legacy, legacyRaw, input := acquire(t, tc.source)
			if tc.layer == "success" {
				if legacy.Status != "ok" || len(legacy.Diagnostics) != 0 || len(legacy.Functions) != 2 || len(legacy.Symbols) != 5 {
					t.Fatalf("supported mask failed official/legacy prerequisite: %+v raw=%s", legacy, legacyRaw)
				}
				fn := legacy.Functions[0]
				root := fn.Body[0].Expression
				wantNull := tc.name == "mask3Supported"
				if len(fn.Parameters) != 2 || fn.Parameters[0].Presence == nil || !fn.Parameters[0].Presence.Undefined || fn.Parameters[0].Presence.Null != wantNull || fn.Parameters[0].Presence.Optional || fn.Parameters[0].Presence.Default || root == nil || root.Kind != "shortCircuit" || root.Operator != "??" || root.PresenceFlow == nil || root.PresenceFlow.Null || root.PresenceFlow.Undefined || root.Left == nil || root.Right == nil || root.Left.Kind != "reference" || root.Left.Symbol != fn.Parameters[0].ID || root.Left.PresenceFlow == nil || !root.Left.PresenceFlow.Undefined || root.Left.PresenceFlow.Null != wantNull || root.Right.Kind != "reference" || root.Right.Symbol != fn.Parameters[1].ID || root.Right.PresenceFlow != nil || sourceText(t, tc.source, root.Range) != "value ?? replacement" {
					t.Fatal("supported mask lost exact owned declaration/flow/lazy reference structure")
				}
				output, err := private(t, input, "--nullable-facts-io-v3")
				if err != nil {
					t.Fatalf("supported mask private v3 refused after official/legacy PASS: %v", err)
				}
				checkPackage(t, r, legacyRaw, input, output)
				t.Logf("TSFG_NULLABLE_COALESCE_POSITIVE_REQUEST_V3 %s %s", tc.name, base64.StdEncoding.EncodeToString(input))
				t.Logf("TSFG_NULLABLE_COALESCE_POSITIVE_RESPONSE_V3 %s %s", tc.name, base64.StdEncoding.EncodeToString(output))
				t.Logf("TSFG_NULLABLE_COALESCE_POSITIVE_LEGACY_V3 %s %s", tc.name, base64.StdEncoding.EncodeToString(legacyRaw))
				return
			}
			if tc.layer == "source" {
				if legacy.Status != "error" || len(legacy.Diagnostics) != 1 || len(legacy.Functions) != 0 {
					t.Fatalf("Source control did not reject at legacy layer: %+v raw=%s", legacy, legacyRaw)
				}
				d := legacy.Diagnostics[0]
				if d.Code != "r02b-d-unsupported" || d.Category != "unsupported" || d.Phase != "capability" || d.UpstreamCode != 0 || d.File != "/src/main.ts" || sourceText(t, tc.source, d.Range) != "value ?? replacement" {
					t.Fatalf("wrong original legacy Source rejection: %+v", d)
				}
			} else if legacy.Status != "ok" || len(legacy.Diagnostics) != 0 || len(legacy.Functions) == 0 {
				t.Fatalf("private negative failed before its claimed gate: %+v raw=%s", legacy, legacyRaw)
			}
			output, err := private(t, input, "--nullable-facts-io-v3")
			if err == nil || len(output) != 0 || !strings.HasPrefix(err.Error(), "[r03-nullable-facts-"+tc.layer+"] ") {
				t.Fatalf("wrong private rejection layer: want=%s err=%v raw=%s", tc.layer, err, output)
			}
			if tc.name == "assignmentOperatorGrammar" && !strings.Contains(err.Error(), "only ordinary nullish coalesce operators") {
				t.Fatal("assignment negative did not reach the unsupported operator grammar gate")
			}
			if tc.layer == "source" && !strings.Contains(err.Error(), "r02b-d-unsupported at /src/main.ts") {
				t.Fatal("private Source adapter lost the actual legacy diagnostic identity")
			}
			// Groups test group rejection, never binary/operator-specific credit.
			t.Logf("actual rejection layer=%s: %v", tc.layer, err)
		})
	}
	t.Run("removedV2Api", func(t *testing.T) {
		_, legacy, raw, input := acquire(t, positives[0].source)
		if legacy.Status != "ok" || len(legacy.Diagnostics) != 0 {
			t.Fatalf("old-API control lost current valid legacy input: %s", raw)
		}
		output, err := private(t, input, "--nullable-facts-io-v2")
		if err == nil || len(output) != 0 || !strings.Contains(err.Error(), "accepts one JSON request on stdin") || strings.Contains(err.Error(), "[r03-nullable-facts-") {
			t.Fatalf("removed v2 flag did not fail at argv API before package decoding: err=%v raw=%s", err, output)
		}
	})
}
