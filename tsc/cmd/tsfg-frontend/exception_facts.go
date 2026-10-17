// SPDX-License-Identifier: Apache-2.0

package main

// Exception regions own their original source structure. These facts do not
// describe Runtime unwinding, request termination or an exception payload ABI.
type exceptionFact struct {
	TryBlock     exceptionBlockFact  `json:"tryBlock"`
	CatchClause  *exceptionCatchFact `json:"catchClause,omitempty"`
	FinallyBlock *exceptionBlockFact `json:"finallyBlock,omitempty"`
}

// Body is explicit even for an empty block; optional region pointers distinguish
// a missing catch/finally from a present region whose body is empty.
type exceptionBlockFact struct {
	Body  []statement `json:"body"`
	Range span        `json:"range"`
}

type exceptionCatchFact struct {
	Binding          *symbol            `json:"binding,omitempty"`
	DeclarationRange *span              `json:"declarationRange,omitempty"`
	AnnotationRange  *span              `json:"annotationRange,omitempty"`
	Block            exceptionBlockFact `json:"block"`
	Range            span               `json:"range"`
}

// A typeof guard keeps its compile-time tag and both original syntax anchors.
type typeTestFact struct {
	Tag         string `json:"tag"`
	TypeofRange span   `json:"typeofRange"`
	TagRange    span   `json:"tagRange"`
}
