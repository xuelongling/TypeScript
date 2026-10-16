// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"strings"
	"testing"
)

func requireGenericPackageFailureKind(t *testing.T, err error, kind string) {
	t.Helper()
	var failure *genericFailure
	if !errors.As(err, &failure) || failure.Kind != kind || failure.Code != 0 {
		t.Fatalf("expected structured %s boundary failure, got %v", kind, err)
	}
}

func genericPackageDefinitionForMutation(t *testing.T, g *genericGraph, name string) *genericDefinition {
	t.Helper()
	for i := range g.Definitions {
		if g.Definitions[i].Name == name {
			return &g.Definitions[i]
		}
	}
	t.Fatalf("missing definition %q", name)
	return nil
}

func TestOwnedGenericPackageClassifiesIdentityAndReferenceFailures(t *testing.T) {
	t.Parallel()
	data := genericPackageBytesForTest(t)
	for _, test := range []struct {
		name   string
		kind   string
		mutate func(*testing.T, *genericGraph)
	}{
		{"definition identity", "package-identity", func(t *testing.T, g *genericGraph) {
			genericPackageDefinitionForMutation(t, g, "identity").Key = strings.Repeat("0", 64)
		}},
		{"declaration digest", "package-identity", func(t *testing.T, g *genericGraph) {
			g.Environment.Files[0].Digest = strings.Repeat("0", 64)
		}},
		{"declaration content", "package-identity", func(t *testing.T, g *genericGraph) {
			g.Environment.Files[0].Content += "\n// altered content"
		}},
		{"source digest format", "package-identity", func(t *testing.T, g *genericGraph) {
			g.Source.Digest = "invalid"
		}},
		{"missing definition", "package-reference", func(t *testing.T, g *genericGraph) {
			for i, d := range g.Definitions {
				if d.Name == "identity" {
					g.Definitions = append(g.Definitions[:i], g.Definitions[i+1:]...)
					return
				}
			}
			t.Fatal("missing identity definition before mutation")
		}},
		{"foreign definition reference", "package-reference", func(t *testing.T, g *genericGraph) {
			genericPackageDefinitionForMutation(t, g, "route").Body.Definition = strings.Repeat("0", 64)
		}},
		{"return binder", "package-reference", func(t *testing.T, g *genericGraph) {
			genericPackageDefinitionForMutation(t, g, "identity").Return.Parameter = 1
		}},
		{"parameter type binder", "package-reference", func(t *testing.T, g *genericGraph) {
			genericPackageDefinitionForMutation(t, g, "identity").Parameters[0].Type.Parameter = 1
		}},
		{"body value binder", "package-reference", func(t *testing.T, g *genericGraph) {
			genericPackageDefinitionForMutation(t, g, "identity").Body.Parameter = 1
		}},
		{"call type binder", "package-reference", func(t *testing.T, g *genericGraph) {
			genericPackageDefinitionForMutation(t, g, "route").Body.TypeArguments[0].Parameter = 1
		}},
		{"call type arity", "package-reference", func(t *testing.T, g *genericGraph) {
			genericPackageDefinitionForMutation(t, g, "route").Body.TypeArguments = nil
		}},
		{"call value arity", "package-reference", func(t *testing.T, g *genericGraph) {
			genericPackageDefinitionForMutation(t, g, "route").Body.Arguments = nil
		}},
		{"call target shadowed", "package-reference", func(t *testing.T, g *genericGraph) {
			genericPackageDefinitionForMutation(t, g, "route").Parameters[0].Name = "identity"
		}},
		{"type representation", "input", func(t *testing.T, g *genericGraph) {
			genericPackageDefinitionForMutation(t, g, "identity").Return.Kind = "unknown"
		}},
		{"definition metadata", "input", func(t *testing.T, g *genericGraph) {
			genericPackageDefinitionForMutation(t, g, "identity").Origin.End = g.Source.Length + 1
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g, err := decodeGenericPackage(data)
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(t, &g)
			g.Digest = genericGraphDigest(g)
			t.Run("encode", func(t *testing.T) {
				_, err := encodeGenericPackage(g)
				requireGenericPackageFailureKind(t, err, test.kind)
			})
			t.Run("decode", func(t *testing.T) {
				wire := genericPackageWireForTest(t, data)
				wire.Payload = genericPackagePayload(g)
				// Re-sign the outer identity so inner identity and reference
				// violations must be distinguished by their own validation branch.
				_, err := decodeGenericPackage(genericPackageSignedBytesForTest(t, wire))
				requireGenericPackageFailureKind(t, err, test.kind)
			})
		})
	}
	t.Run("graph digest encode", func(t *testing.T) {
		t.Parallel()
		g, err := decodeGenericPackage(data)
		if err != nil {
			t.Fatal(err)
		}
		g.Digest = strings.Repeat("0", 64)
		_, err = encodeGenericPackage(g)
		requireGenericPackageFailureKind(t, err, "package-identity")
	})
	t.Run("package digest decode", func(t *testing.T) {
		t.Parallel()
		wire := genericPackageWireForTest(t, data)
		wire.PackageID = "sha256:" + strings.Repeat("0", 64)
		bad, err := genericPackageCanonical(wire)
		if err != nil {
			t.Fatal(err)
		}
		_, err = decodeGenericPackage(bad)
		requireGenericPackageFailureKind(t, err, "package-identity")
	})
}
