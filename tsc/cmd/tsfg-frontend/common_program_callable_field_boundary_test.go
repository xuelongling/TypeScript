// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCommonProgramCallableFieldBoundary26n(t *testing.T) {
	original := string(callableFieldSource26n(t))
	for _, sample := range []struct{ name, old, replacement string }{
		{"optional-call", "fieldReceiver().apply(fieldArgument())", "fieldReceiver().apply?.(fieldArgument())"},
		{"optional-receiver", "fieldReceiver().apply(fieldArgument())", "fieldReceiver()?.apply(fieldArgument())"},
		{"bound-method-value", "return method.m(direct);", "const bound: (x: i32) => i32 = method.m; return bound(direct);"},
	} {
		t.Run(sample.name, func(t *testing.T) {
			source := strings.Replace(original, sample.old, sample.replacement, 1)
			if source == original {
				t.Fatal("independent original source boundary mutation missing")
			}
			commonMethodProgram37q(t, source)
			var output bytes.Buffer
			err := runFrontendCommand(t.Context(), []string{"--program-facts-io-v2"}, bytes.NewReader(readSpanCaptureRequest26i(t, []byte(source))), &output)
			var failure *arrayFactsFailure
			if !errors.As(err, &failure) || failure.Code != "r03-array-facts-capability" || output.Len() != 0 {
				t.Fatal("unchanged bounded capability must reject atomically", err, output.Len())
			}
		})
	}
	t.Run("canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		var output bytes.Buffer
		if err := runFrontendCommand(ctx, []string{"--program-facts-io-v2"}, bytes.NewReader(readSpanCaptureRequest26i(t, []byte(original))), &output); err == nil || output.Len() != 0 {
			t.Fatal("canceled field projection must not publish partial facts", err)
		}
	})
	t.Run("legacy-v8", func(t *testing.T) {
		var output bytes.Buffer
		if err := runFrontendCommand(t.Context(), []string{"--array-facts-io-v8"}, bytes.NewReader(genericArrayRequest25f(t, original)), &output); err == nil || output.Len() != 0 {
			t.Fatal("old array contract must not acquire Common callable values", err)
		}
	})
}
