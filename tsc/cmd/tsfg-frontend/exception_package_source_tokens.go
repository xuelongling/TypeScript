// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/scanner"
)

const exceptionFactsSourceTokenLimit = 50000

// Count official non-trivia tokens without constructing a Program or checker.
// The caller only needs the exact count through the first over-limit token.
func exceptionFactsSourceTokenCount(source string) int {
	tokens := scanner.NewScanner()
	tokens.SetText(source)
	count := 0
	for tokens.Scan() != ast.KindEndOfFile {
		count++
		if count > exceptionFactsSourceTokenLimit {
			return count
		}
	}
	return count
}
