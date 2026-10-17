// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
)

// Exact qualified private4 package, SHA256 435032b9aa3305df439dd11e9e639122f9a97c7f303aa75dd454992d1ae2a249.
// It remains a predecessor negative, not a newly executed Source5 result.
const genericConditionalCallPredecessor4 = "eyJmYW1pbHkiOiJ0c2ZnLW93bmVkLWdlbmVyaWMtcGFja2FnZSIsInBhY2thZ2VJZCI6InNoYTI1NjoyNGE0YTM4MTkxZjBhNzIzOWE3YWY3MTA1YWU3YTJlMjc5OTExNTdjZGEyNzlhOGI4NjVjZTdmMTAwMTVlOGFkIiwicGF5bG9hZCI6eyJjYXBhYmlsaXRpZXMiOlsiY29uZGl0aW9uYWwtc2lnbmF0dXJlIiwiZXhwbGljaXQtdHlwZS1hcmd1bWVudHMiLCJtYXBwZWQtaW5kZXhlZC1zaWduYXR1cmUiLCJudW1iZXItYWRkIiwic2NhbGFyLWdlbmVyaWNzIiwic2luZ2xlLW1vZHVsZSJdLCJkZWZpbml0aW9ucyI6W3siYm9keSI6eyJhcmd1bWVudHMiOltdLCJkZWZpbml0aW9uIjoiIiwia2luZCI6InBhcmFtZXRlciIsIm9yaWdpbiI6eyJlbmQiOjc2LCJzdGFydCI6NzF9LCJwYXJhbWV0ZXIiOjAsInR5cGVBcmd1bWVudHMiOltdfSwiZXhwb3J0ZWQiOnRydWUsImtleSI6IjIwZmM1MjA5N2Q2N2E0ZTE5Y2E4Yjg3MWIzNTkyMTkyN2Y4ZTcxOWQ3ZjJlZmZjOTBkMTQ1NzY2ZGNkZDAxOWIiLCJuYW1lIjoiaWRlbnRpdHkiLCJvcmlnaW4iOnsiZW5kIjo3OSwic3RhcnQiOjIxfSwicGFyYW1ldGVycyI6W3sibmFtZSI6InZhbHVlIiwib3JpZ2luIjp7ImVuZCI6NTcsInN0YXJ0Ijo0OX0sInR5cGUiOnsia2luZCI6InBhcmFtZXRlciIsInBhcmFtZXRlciI6MH19XSwicmV0dXJuVHlwZSI6eyJraW5kIjoicGFyYW1ldGVyIiwicGFyYW1ldGVyIjowfSwidHlwZVBhcmFtZXRlcnMiOlsiVCJdfSx7ImJvZHkiOnsiYXJndW1lbnRzIjpbXSwiZGVmaW5pdGlvbiI6IiIsImtpbmQiOiJwYXJhbWV0ZXIiLCJvcmlnaW4iOnsiZW5kIjoyMTIsInN0YXJ0IjoyMDh9LCJwYXJhbWV0ZXIiOjAsInR5cGVBcmd1bWVudHMiOltdfSwiZXhwb3J0ZWQiOnRydWUsImtleSI6IjM4NzJjMmE5NjMyMDVjOTBhNzA0MmFlMGZkMWJjZWNhNTMyOThmMzA3OTc0Mjc1ZjM4NGFlYTgzMWY3OWNhNTYiLCJuYW1lIjoidW51c2VkIiwib3JpZ2luIjp7ImVuZCI6MjE1LCJzdGFydCI6MTQ5fSwicGFyYW1ldGVycyI6W3sibmFtZSI6ImZsYWciLCJvcmlnaW4iOnsiZW5kIjoxODgsInN0YXJ0IjoxNzV9LCJ0eXBlIjp7ImtpbmQiOiJib29sZWFuIiwicGFyYW1ldGVyIjowfX1dLCJyZXR1cm5UeXBlIjp7ImtpbmQiOiJib29sZWFuIiwicGFyYW1ldGVyIjowfSwidHlwZVBhcmFtZXRlcnMiOlsiViJdfSx7ImJvZHkiOnsiYXJndW1lbnRzIjpbeyJhcmd1bWVudHMiOltdLCJkZWZpbml0aW9uIjoiIiwia2luZCI6InBhcmFtZXRlciIsIm9yaWdpbiI6eyJlbmQiOjE0NCwic3RhcnQiOjEzOX0sInBhcmFtZXRlciI6MCwidHlwZUFyZ3VtZW50cyI6W119XSwiZGVmaW5pdGlvbiI6IjIwZmM1MjA5N2Q2N2E0ZTE5Y2E4Yjg3MWIzNTkyMTkyN2Y4ZTcxOWQ3ZjJlZmZjOTBkMTQ1NzY2ZGNkZDAxOWIiLCJraW5kIjoiY2FsbCIsIm9yaWdpbiI6eyJlbmQiOjE0NSwic3RhcnQiOjEyN30sInBhcmFtZXRlciI6MCwidHlwZUFyZ3VtZW50cyI6W3sia2luZCI6InBhcmFtZXRlciIsInBhcmFtZXRlciI6MH1dfSwiZXhwb3J0ZWQiOnRydWUsImtleSI6ImEzOWZmNDYxNWE2N2FhNGQ5OGY4N2U0MjRjMGUxODZkOThlMzdjZGQ3ZjE4MmIzYWZkMDU5NDJjZWM3ODhjOTIiLCJuYW1lIjoicm91dGUiLCJvcmlnaW4iOnsiZW5kIjoxNDgsInN0YXJ0Ijo4MH0sInBhcmFtZXRlcnMiOlt7Im5hbWUiOiJpbnB1dCIsIm9yaWdpbiI6eyJlbmQiOjExMywic3RhcnQiOjEwNX0sInR5cGUiOnsia2luZCI6InBhcmFtZXRlciIsInBhcmFtZXRlciI6MH19XSwicmV0dXJuVHlwZSI6eyJraW5kIjoicGFyYW1ldGVyIiwicGFyYW1ldGVyIjowfSwidHlwZVBhcmFtZXRlcnMiOlsiVSJdfV0sImVudmlyb25tZW50Ijp7ImZpbGVzIjpbeyJjb250ZW50IjoiaW50ZXJmYWNlIE9iamVjdCB7fVxuaW50ZXJmYWNlIEZ1bmN0aW9uIHt9XG5pbnRlcmZhY2UgQ2FsbGFibGVGdW5jdGlvbiB7fVxuaW50ZXJmYWNlIE5ld2FibGVGdW5jdGlvbiB7fVxuaW50ZXJmYWNlIElBcmd1bWVudHMge31cbmludGVyZmFjZSBTdHJpbmcge31cbmludGVyZmFjZSBOdW1iZXIge31cbmludGVyZmFjZSBCb29sZWFuIHt9XG5pbnRlcmZhY2UgUmVnRXhwIHt9XG5pbnRlcmZhY2UgQXJyYXk8VD4ge31cbnR5cGUgYm9vbCA9IGJvb2xlYW47XG5kZWNsYXJlIGNvbnN0IF9fdHNmZ19pMzI6IHVuaXF1ZSBzeW1ib2w7XG50eXBlIGkzMiA9IG51bWJlciAmIHsgcmVhZG9ubHkgW19fdHNmZ19pMzJdOiB0cnVlIH07XG50eXBlIGY2NCA9IG51bWJlcjtcbiIsImRpZ2VzdCI6ImQ2MjkwNTlmNTE3NWE1ODBhOGE3YjE5NTMxYzdkOGNmZjFiZmE3YjYxMzIwZjY2MzhjZjRlZTI1ZmJmNWExNDEiLCJwYXRoIjoiL29yaWdpbmFsL21pbmltYWwtc2NhbGFycy12Mi5kLnRzIn1dLCJwcm9maWxlIjoibWluaW1hbC1zY2FsYXJzLXYyIn0sInNjb3BlIjoicGFja2FnZTp3aXJlLWNhbmRpZGF0ZSIsInNlbWFudGljUHJvZmlsZSI6InRzZmctb3duZWQtc2NhbGFyLWdlbmVyaWNzLWhvc3QtdjQiLCJzb3VyY2UiOnsiZGlnZXN0IjoiMWY3NTRiNDY4ZjUxOWY3YjYzMzFiOTUyNTE4ZGE4Nzc3Y2MxNDkzNzdmYzgxYjEzM2FmN2UzMTcyNTQ4NzkzMyIsImxlbmd0aCI6MjE2LCJwYXRoIjoiL29yaWdpbmFsL21haW4udHMiLCJwb3NpdGlvblVuaXQiOiJ1dGYxNiIsInN1cnJvZ2F0ZVN0YXJ0cyI6WzhdfX0sInNjaGVtYUhhc2giOiJlYzA1NjMxYzE1ZjI4MGY1NDI3OWJhOTM1M2ZkOGZjM2ZmNzc5MzQ5YTFmNTZmN2NkYjAwZWJhMjJjNzY3ZTIwIiwidmVyc2lvbiI6NH0="

func genericConditionalCallDefinitionsForTest(t *testing.T, wire *genericPackageDTO) (*genericPackageDefinitionDTO, *genericPackageDefinitionDTO) {
	t.Helper()
	var leaf, route *genericPackageDefinitionDTO
	for i := range wire.Payload.Definitions {
		d := &wire.Payload.Definitions[i]
		switch d.Name {
		case "leaf":
			leaf = d
		case "route":
			route = d
		}
	}
	if leaf == nil || route == nil {
		t.Fatal("actual conditional-call package lacks leaf or route")
	}
	return leaf, route
}

func genericConditionalCallRejectForTest(t *testing.T, bad []byte, requests []genericPackageRoot, kind string, origin genericOrigin) {
	t.Helper()
	graph, err := decodeGenericPackage(bad)
	requireGenericPackageFailureKind(t, err, kind)
	var failure *genericFailure
	if !errors.As(err, &failure) || failure.Origin != origin || !reflect.DeepEqual(graph, genericGraph{}) {
		t.Fatalf("cold conditional-call rejection lost its exact anchor or exposed a graph: %v", err)
	}
	facts, err := instantiateGenericPackageFacts(t.Context(), bad, requests)
	requireGenericPackageFailureKind(t, err, kind)
	if !errors.As(err, &failure) || failure.Origin != origin || !reflect.DeepEqual(facts, genericPackageFacts{}) {
		t.Fatalf("rejected conditional-call package exposed facts or changed its diagnostic: %v", err)
	}
}

func TestOwnedGenericConditionalCallRejectsOldAndMixedContracts(t *testing.T) {
	t.Parallel()
	data := genericConditionalCallColdBytes(t)
	wire := genericPackageWireForTest(t, data)
	wantCaps := []string{"conditional-generic-call", "conditional-signature", "explicit-type-arguments", "mapped-indexed-signature", "number-add", "scalar-generics", "single-module"}
	if wire.Version != 5 || wire.SchemaHash != "da7eb72ae306eb88863a44c945df77d572a96443a005fefbd787267fe90658bf" ||
		wire.Payload.SemanticProfile != "tsfg-owned-scalar-generics-host-v5" || !reflect.DeepEqual(wire.Payload.Capabilities, wantCaps) {
		t.Fatal("actual conditional-call package differs from the independently pinned private5 descriptor")
	}
	_, route := genericConditionalCallDefinitionsForTest(t, &wire)
	requests := []genericPackageRoot{{Definition: route.Key, TypeArguments: []genericPackageTypeDTO{{Kind: "boolean"}}}}
	for _, test := range []struct {
		name   string
		mutate func(*genericPackageDTO)
	}{
		{"old-version4", func(w *genericPackageDTO) { w.Version = 4 }},
		{"old-schema4", func(w *genericPackageDTO) {
			w.SchemaHash = "ec05631c15f280f54279ba9353fd8fc3ff779349a1f56f7cdb00eba22c767e20"
		}},
		{"old-profile4", func(w *genericPackageDTO) { w.Payload.SemanticProfile = "tsfg-owned-scalar-generics-host-v4" }},
		{"old-capabilities4", func(w *genericPackageDTO) { w.Payload.Capabilities = append([]string{}, wantCaps[1:]...) }},
		{"duplicate-capability", func(w *genericPackageDTO) { w.Payload.Capabilities[1] = w.Payload.Capabilities[0] }},
		{"reordered-capability", func(w *genericPackageDTO) {
			w.Payload.Capabilities[0], w.Payload.Capabilities[1] = w.Payload.Capabilities[1], w.Payload.Capabilities[0]
		}},
		{"extra-capability", func(w *genericPackageDTO) {
			w.Payload.Capabilities = append(w.Payload.Capabilities, "recursive-conditional")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			w := genericPackageWireForTest(t, data)
			test.mutate(&w)
			genericConditionalCallRejectForTest(t, genericPackageSignedBytesForTest(t, w), requests, "package-schema", genericOrigin{})
		})
	}
	t.Run("exact-qualified-predecessor4", func(t *testing.T) {
		t.Parallel()
		old, err := base64.StdEncoding.DecodeString(genericConditionalCallPredecessor4)
		if err != nil || len(old) != 2741 || genericTextDigest(string(old)) != "435032b9aa3305df439dd11e9e639122f9a97c7f303aa75dd454992d1ae2a249" {
			t.Fatal("qualified predecessor4 witness bytes changed")
		}
		genericConditionalCallRejectForTest(t, old, requests, "package-schema", genericOrigin{})
	})
	t.Run("old-package-domain4", func(t *testing.T) {
		t.Parallel()
		w := genericPackageWireForTest(t, data)
		w.PackageID = "sha256:" + genericPackageIdentity("tsfg-owned-generic-package/v4\n", w.Payload)
		bad, err := genericPackageCanonical(w)
		if err != nil {
			t.Fatal(err)
		}
		genericConditionalCallRejectForTest(t, bad, requests, "package-identity", genericOrigin{})
	})
}

// Cold negatives mutate only definitions from a real public export. They are
// never successful facts; re-signing forces the semantic gate to do the work.
func TestOwnedGenericConditionalCallRejectsMalformedColdBodies(t *testing.T) {
	t.Parallel()
	data := genericConditionalCallColdBytes(t)
	for _, test := range []struct {
		name, kind string
		origin     genericOrigin
		mutate     func(*genericPackageDefinitionDTO, *genericPackageDefinitionDTO)
	}{
		{"callee-parameter-signature", "unsupported", genericOrigin{Start: 6, End: 130}, func(leaf, route *genericPackageDefinitionDTO) {
			leaf.Parameters[0].Type = genericPackageTypeDTO{Kind: "boolean"}
		}},
		{"callee-leaf-signature", "unsupported", genericOrigin{Start: 131, End: 265}, func(leaf, route *genericPackageDefinitionDTO) {
			leaf.Parameters[0].Type = genericPackageTypeDTO{Kind: "parameter"}
			leaf.ReturnType = genericPackageTypeDTO{Kind: "parameter"}
		}},
		{"callee-wrong-parameter-body", "unsupported", genericOrigin{Start: 6, End: 130}, func(leaf, route *genericPackageDefinitionDTO) {
			leaf.Body.Parameter = 1
		}},
		{"conditional-chain", "unsupported", genericOrigin{Start: 131, End: 265}, func(leaf, route *genericPackageDefinitionDTO) {
			argument := leaf.Body
			leaf.Body = genericPackageExpressionDTO{Kind: "call", Definition: leaf.Key, Origin: argument.Origin,
				TypeArguments: []genericPackageTypeDTO{{Kind: "parameter"}}, Arguments: []genericPackageExpressionDTO{argument}}
		}},
		{"conditional-self", "unsupported", genericOrigin{Start: 131, End: 265}, func(leaf, route *genericPackageDefinitionDTO) {
			route.Body.Definition = route.Key
		}},
		{"concrete-type-argument", "unsupported", genericOrigin{Start: 131, End: 265}, func(leaf, route *genericPackageDefinitionDTO) {
			route.Body.TypeArguments[0] = genericPackageTypeDTO{Kind: "boolean"}
		}},
		{"concrete-number-type-argument", "unsupported", genericOrigin{Start: 131, End: 265}, func(leaf, route *genericPackageDefinitionDTO) {
			route.Body.TypeArguments[0] = genericPackageTypeDTO{Kind: "number"}
		}},
		{"unbound-type-argument", "package-reference", genericOrigin{Start: 248, End: 262}, func(leaf, route *genericPackageDefinitionDTO) {
			route.Body.TypeArguments[0].Parameter = 1
		}},
		{"wrong-value-argument", "unsupported", genericOrigin{Start: 131, End: 265}, func(leaf, route *genericPackageDefinitionDTO) {
			route.Body.Arguments[0].Parameter = 1
		}},
		{"nested-call-argument", "unsupported", genericOrigin{Start: 131, End: 265}, func(leaf, route *genericPackageDefinitionDTO) {
			argument := route.Body.Arguments[0]
			route.Body.Arguments[0] = genericPackageExpressionDTO{Kind: "call", Definition: leaf.Key, Origin: argument.Origin,
				TypeArguments: []genericPackageTypeDTO{{Kind: "parameter"}}, Arguments: []genericPackageExpressionDTO{argument}}
		}},
		{"missing-target", "package-reference", genericOrigin{Start: 248, End: 262}, func(leaf, route *genericPackageDefinitionDTO) {
			route.Body.Definition = strings.Repeat("0", 64)
		}},
		{"call-metadata", "input", genericOrigin{}, func(leaf, route *genericPackageDefinitionDTO) {
			route.Body.Parameter = 1
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			wire := genericPackageWireForTest(t, data)
			leaf, route := genericConditionalCallDefinitionsForTest(t, &wire)
			requests := []genericPackageRoot{{Definition: route.Key, TypeArguments: []genericPackageTypeDTO{{Kind: "boolean"}}}}
			test.mutate(leaf, route)
			bad := genericPackageSignedBytesForTest(t, wire)
			if bytes.Equal(bad, data) {
				t.Fatal("conditional-call negative did not change the real package")
			}
			genericConditionalCallRejectForTest(t, bad, requests, test.kind, test.origin)
		})
	}
}

func TestOwnedGenericConditionalCallRejectsUnsupportedOfficialBodies(t *testing.T) {
	t.Parallel()
	mapped := "// 😀\nexport function leaf<U>(value: { [K in 'value']: U }['value']): { [K in 'value']: U }['value'] { return value; }\n" +
		"export function route<T>(value: { [K in 'value']: T }['value']): { [K in 'value']: T }['value'] { return leaf<T>(value); }"
	for _, test := range []struct {
		name, source, rejected string
	}{
		{"self", strings.Replace(genericConditionalCallSource, "return leaf<T>(value);", "return route<T>(value);", 1), "route"},
		{"mutual-chain", strings.Replace(genericConditionalCallSource, "return value;", "return route<U>(value);", 1), "leaf"},
		{"nested-argument", strings.Replace(genericConditionalCallSource, "return leaf<T>(value);", "return leaf<T>(leaf<T>(value));", 1), "route"},
		{"mapped-call", mapped, "route"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			program, environment := genericProgramForTest(test.source)
			if err := checkGenericDiagnostics(t.Context(), program, nil); err != nil {
				t.Fatalf("unsupported call fixture is not officially valid: %v", err)
			}
			data, err := exportGenericPackage(t.Context(), program, program.GetSourceFile("/original/main.ts"), "package:conditional-call-boundary", environment)
			requireGenericPackageFailureKind(t, err, "unsupported")
			start := strings.Index(test.source, "export function "+test.rejected+"<")
			if start < 0 {
				t.Fatal("expected rejected function is absent from the actual source")
			}
			end := len(test.source)
			if next := strings.IndexByte(test.source[start:], '\n'); next >= 0 {
				end = start + next
			}
			want := genericOrigin{Start: len(utf16.Encode([]rune(test.source[:start]))), End: len(utf16.Encode([]rune(test.source[:end])))}
			var failure *genericFailure
			if !errors.As(err, &failure) || failure.Origin != want || len(data) != 0 {
				t.Fatalf("unsupported official call lost its exact declaration anchor or exposed bytes: %v", err)
			}
		})
	}
}

func TestOwnedGenericConditionalCallKeepsDefinitionScopedAnchors(t *testing.T) {
	t.Parallel()
	data := genericConditionalCallColdBytes(t)
	original := bytes.Clone(data)
	wire := genericPackageWireForTest(t, data)
	_, route := genericConditionalCallDefinitionsForTest(t, &wire)
	// Existing call origins are independent source occurrences within the
	// definition; a child is not required to fit inside the call's anchor.
	route.Body.Origin = genericPackageOriginDTO{Start: 248, End: 252}
	control := genericPackageSignedBytesForTest(t, wire)
	g, err := decodeGenericPackage(control)
	if err != nil {
		t.Fatal(err)
	}
	again, err := encodeGenericPackage(g)
	if err != nil || !bytes.Equal(control, again) || bytes.Equal(data, control) {
		t.Fatalf("definition-scoped call anchor did not survive cold roundtrip: %v", err)
	}
	requests := []genericPackageRoot{
		genericPackageRootForTest(t, g, "route", "boolean"), genericPackageRootForTest(t, g, "route", "number"),
	}
	facts, err := instantiateGenericPackageFacts(t.Context(), control, requests)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.Roots) != 2 || len(facts.Instances) != 4 || facts.PackageID != "sha256:"+g.Digest {
		t.Fatal("definition-scoped anchors changed the real instance closure")
	}
	leaf := genericDefinitionNamed(t, g, "leaf")
	for i, scalar := range []string{"bool", "f64"} {
		instance := genericPackageFactForTest(t, facts, facts.Roots[i])
		callee := genericInstanceKey(g.request(leaf.Key, genericType{Kind: requests[i].TypeArguments[0].Kind}))
		if instance.Body.Kind != "call" || instance.Body.Callee != callee || instance.Body.Type != scalar ||
			instance.Body.Origin != (genericPackageOriginDTO{Start: 248, End: 252}) || len(instance.Body.Arguments) != 1 {
			t.Fatal("fresh official checker replaced the independent call anchor or callee")
		}
		argument := instance.Body.Arguments[0]
		if argument.Kind != "parameter" || argument.Parameter != 0 || argument.Type != scalar ||
			argument.Origin != (genericPackageOriginDTO{Start: 256, End: 261}) || argument.Origin.Start <= instance.Body.Origin.End {
			t.Fatal("fresh official parameter argument was clamped into its call anchor")
		}
	}
	if !bytes.Equal(data, original) || !bytes.Equal(control, again) {
		t.Fatal("cold anchor control mutated the actual exported input")
	}
}
