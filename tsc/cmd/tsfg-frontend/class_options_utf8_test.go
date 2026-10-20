// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"reflect"
	"testing"

	"github.com/microsoft/typescript-go/internal/collections"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/json"
)

// Paths uses the official custom JSON marshaler. Its raw configuration must not
// silently turn invalid UTF8 into the same owned identity as genuine U+FFFD.
// Every case builds a real fresh Program before the public projection call.
func TestOwnedClassSourcePathsRejectsInvalidUTF8WithoutRejectingReplacementRune(t *testing.T) {
	t.Parallel()
	invalid := string([]byte{0xff})
	for _, test := range []struct {
		name, key, value string
		reject           bool
	}{
		{"invalid-key", "alias" + invalid, "./module", true},
		{"invalid-value", "alias", "./" + invalid, true},
		{"replacement-key", "alias\ufffd", "./module", false},
		{"replacement-value", "alias", "./\ufffd", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			graph, err := classIdentityExport(t, classCounterSource, func(options *core.CompilerOptions) {
				options.Paths = &collections.OrderedMap[string, []string]{}
				options.Paths.Set(test.key, []string{test.value})
			})
			if test.reject {
				var failure *classFailure
				if !errors.As(err, &failure) || failure.Kind != "input" {
					t.Fatalf("invalid Paths UTF8 must be rejected before producing an ambiguous identity: %v", err)
				}
				if !reflect.DeepEqual(graph, classGraph{}) {
					t.Fatalf("rejected options exposed a partial owned graph: %+v", graph)
				}
				return
			}
			if err != nil {
				t.Fatalf("valid replacement-rune Paths option was rejected: %v", err)
			}
			if len(graph.Classes) != 1 || graph.Classes[0].Name != "Counter" || graph.Classes[0].RuntimeKey == "" {
				t.Fatalf("valid actual Paths option did not produce the real complete class: %+v", graph.Classes)
			}
			var options map[string]any
			if err := json.Unmarshal([]byte(graph.Options.OfficialOptions), &options); err != nil {
				t.Fatal(err)
			}
			paths := classOwnedObject(t, options["paths"])
			values := classOwnedArray(t, paths[test.key])
			if len(paths) != 1 || len(values) != 1 || values[0] != test.value {
				t.Fatalf("valid U+FFFD Paths strings were altered in the owned actual configuration: %+v", paths)
			}
		})
	}
}
