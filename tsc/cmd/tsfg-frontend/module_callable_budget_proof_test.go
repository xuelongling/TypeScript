// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"testing"
)

// One independently checked Source packet serves both fixed-upper proof cases.
// Unexported declarations are intentional: roots can name local module functions.
func TestModuleCallableFactsBudgetProofWork(t *testing.T) {
	source := mcBudgetGlobals23d(1466)
	roots := []closureRoot{}
	for i := 0; i < 32; i++ {
		name := fmt.Sprintf("p%03d", i)
		prefix := ""
		if i == 0 {
			prefix = "export "
		}
		source += prefix + "function " + name + "(): bool { return true; }\n"
		roots = append(roots, mcBudgetRoot23d(name))
	}
	mcBudgetCapture23d(t, mcBudgetCase23d{name: "proof-work-1466", kind: "proof-work", source: source, roots: roots, functions: 32, globals: 1466, scale: 1466})
}
