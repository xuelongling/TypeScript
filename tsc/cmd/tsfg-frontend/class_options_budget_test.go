// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/collections"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/json"
)

// Options rejection already existed after an unbounded Marshal. These are
// public regression controls for the bounded official encoder; the earlier
// wire-expansion test supplies this slice's new behavior RED.
func TestOwnedClassSourceBoundsOfficialOptionsEncoding(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		configure func(*core.CompilerOptions)
	}{
		{"oversized-string", func(options *core.CompilerOptions) {
			options.Paths = &collections.OrderedMap[string, []string]{}
			options.Paths.Set("unused/*", []string{"/original/" + strings.Repeat("s", 1<<20) + "/*"})
		}},
		{"escaped-string-expansion", func(options *core.CompilerOptions) {
			options.Paths = &collections.OrderedMap[string, []string]{}
			options.Paths.Set("unused/*", []string{"/original/" + strings.Repeat("\\", 600000) + "target"})
		}},
		{"many-short-paths", func(options *core.CompilerOptions) {
			options.Paths = &collections.OrderedMap[string, []string]{}
			for index := range 8192 {
				options.Paths.Set(fmt.Sprintf("unused%d/*", index), []string{"/original/" + strings.Repeat("s", 128) + "/*"})
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			graph, err := classIdentityExport(t, classCounterSource, test.configure)
			var failure *classFailure
			if !errors.As(err, &failure) || failure.Kind != "limit" ||
				!strings.Contains(failure.Detail, "official options") || !strings.Contains(failure.Detail, "byte") {
				t.Fatal("official options exceeded the byte budget without its controlled limit")
			}
			if !reflect.DeepEqual(graph, classGraph{}) {
				t.Fatal("exhausted options budget exposed a partial graph")
			}
		})
	}
	t.Run("complete-small-paths", func(t *testing.T) {
		t.Parallel()
		target := "/original/" + strings.Repeat("segment/", 64) + "\uFFFD/*"
		graph, err := classIdentityExport(t, classCounterSource, func(options *core.CompilerOptions) {
			options.Paths = &collections.OrderedMap[string, []string]{}
			options.Paths.Set("unused/*", []string{target})
		})
		if err != nil {
			t.Fatal(err)
		}
		var options map[string]any
		if err := json.Unmarshal([]byte(graph.Options.OfficialOptions), &options); err != nil {
			t.Fatal(err)
		}
		values := classOwnedArray(t, classOwnedObject(t, options["paths"])["unused/*"])
		if len(graph.Classes) != 1 || len(values) != 1 || values[0] != target {
			t.Fatal("within-budget official Paths were truncated or changed")
		}
	})
}
