// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/typescript-go/internal/vfs/iovfs"
)

// main is the native source-intake command host. The probe receives only this
// cwd-rooted FS and the normalized source name, retaining its approved input set.
//
//nolint:forbidigo // Native command host must inspect argv/cwd, mount DirFS, and report via stdout/stderr/exit; filepath handles native argv paths.
func main() {
	if len(os.Args) != 2 || filepath.Ext(os.Args[1]) != ".ts" {
		fmt.Fprintln(os.Stderr, "expected one TypeScript source file")
		os.Exit(2)
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	relative, err := filepath.Rel(cwd, os.Args[1])
	if err != nil {
		panic(err)
	}
	name := "/" + filepath.ToSlash(relative)
	if err := runProbe(iovfs.From(os.DirFS(cwd), true), name, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
