// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Command selection precedes request decoding and keeps the legacy wire boundary.
package main

import (
	"context"
	"errors"
	"io"
)

func runFrontendCommand(ctx context.Context, args []string, input io.Reader, output io.Writer) error {
	if len(args) == 1 && args[0] == "--string-facts-io-v1" {
		return runStringFactsCandidate(ctx, input, output)
	}
	if len(args) == 1 && args[0] == "--generic-package-io-v1" {
		return runGenericPackageCandidate(ctx, input, output)
	}
	if len(args) == 1 && args[0] == "--exception-facts-io-v1" {
		return runExceptionFactsCandidate(ctx, input, output)
	}
	if len(args) == 1 && args[0] == "--nullable-facts-io-v3" {
		return runNullableFactsCandidate(ctx, input, output)
	}
	if len(args) != 0 {
		return errors.New("tsfg-frontend accepts one JSON request on stdin")
	}
	return runCommand(input, output)
}
