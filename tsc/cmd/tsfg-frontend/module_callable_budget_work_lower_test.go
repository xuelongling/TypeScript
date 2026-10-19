// SPDX-License-Identifier: Apache-2.0
package main

import "testing"

// The while body has an official block node, charged on all 256 iterations.
// This is the one new lower Source input; the qualified 1568 packet is reused.
func TestModuleCallableFactsBudgetSolverWorkLower(t *testing.T) {
	mcBudgetCapture23d(t, mcBudgetSolverWork23d(1567))
}
