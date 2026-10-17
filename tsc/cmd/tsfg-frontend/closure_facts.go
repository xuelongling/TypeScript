// SPDX-License-Identifier: Apache-2.0

package main

// closureCapture refers to the original Snapshot-local binding. Read and Write
// describe this function's direct source accesses only, not accesses in a child
// function or a callee. LexicalParent preserves the ancestry needed by a future
// consumer to calculate environment forwarding; these facts are not GC roots,
// environment instances, or stable FunctionIds.
type closureCapture struct {
	Symbol string `json:"symbol"`
	Read   bool   `json:"read"`
	Write  bool   `json:"write"`
}

func directClosureCaptures(fn *function, bindings map[string]symbol) []closureCapture {
	var captures []closureCapture
	positions := map[string]int{}
	add := func(id string, read, write bool) {
		binding, ok := bindings[id]
		if !ok || binding.LexicalOwner == "" || binding.LexicalOwner == fn.Symbol {
			return
		}
		index, exists := positions[id]
		if !exists {
			index = len(captures)
			positions[id] = index
			captures = append(captures, closureCapture{Symbol: id})
		}
		captures[index].Read = captures[index].Read || read
		captures[index].Write = captures[index].Write || write
	}
	var expressionAccess func(*expression, bool, bool)
	expressionAccess = func(value *expression, read, write bool) {
		if value == nil {
			return
		}
		switch value.Kind {
		case "reference":
			add(value.Symbol, read, write)
			return
		case "functionReference":
			add(value.Target, read, write)
			return
		case "group":
			expressionAccess(value.Operand, read, write)
			return
		case "write":
			expressionAccess(value.Left, value.Operator != "=", true)
			expressionAccess(value.Right, true, false)
			return
		case "update":
			expressionAccess(value.Operand, true, true)
			return
		}
		expressionAccess(value.Left, true, false)
		expressionAccess(value.Right, true, false)
		expressionAccess(value.Operand, true, false)
		expressionAccess(value.Receiver, true, false)
		expressionAccess(value.Callee, true, false)
		if value.Arguments != nil {
			for index := range *value.Arguments {
				expressionAccess(&(*value.Arguments)[index], true, false)
			}
		}
	}
	for _, initializer := range fn.ParameterInitializers {
		expressionAccess(initializer, true, false)
	}
	walkOwnedStatements(fn.Body, func(item *statement) {
		if item.Kind == "functionDeclaration" {
			// Child bodies live in the Functions table and have their own
			// direct capture facts; declaration does not execute that body.
			return
		}
		if item.Kind == "assignment" {
			expressionAccess(item.Target, false, true)
			expressionAccess(item.Value, true, false)
		} else {
			expressionAccess(item.Target, true, false)
			expressionAccess(item.Value, true, false)
		}
		expressionAccess(item.Initializer, true, false)
		expressionAccess(item.Expression, true, false)
		expressionAccess(item.Condition, true, false)
	})
	return captures
}
