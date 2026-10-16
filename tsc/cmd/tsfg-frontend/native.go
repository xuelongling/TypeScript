// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"fmt"
	"os"
)

// main is the native stdin/stdout command host. Analysis receives an owned JSON
// request and cannot access these process-global inputs or streams.
//
//nolint:forbidigo // Native command host must read argv/stdin and report status via stderr/exit.
func main() {
	if err := runFrontendCommand(context.Background(), os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
