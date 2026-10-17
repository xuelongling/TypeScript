// SPDX-License-Identifier: Apache-2.0

package main

// Owned consumers visit statements and their structural regions in source order.
// Function declarations are visited, while their separately owned bodies remain
// in the Functions table.
func walkOwnedStatements(body []statement, visit func(*statement)) {
	for index := range body {
		item := &body[index]
		visit(item)
		if item.Kind == "functionDeclaration" {
			continue
		}
		walkOwnedStatements(item.Body, visit)
		walkOwnedStatements(item.ThenBody, visit)
		walkOwnedStatements(item.ElseBody, visit)
		if regions := item.Exception; regions != nil {
			walkOwnedStatements(regions.TryBlock.Body, visit)
			if regions.CatchClause != nil {
				walkOwnedStatements(regions.CatchClause.Block.Body, visit)
			}
			if regions.FinallyBlock != nil {
				walkOwnedStatements(regions.FinallyBlock.Body, visit)
			}
		}
	}
}
