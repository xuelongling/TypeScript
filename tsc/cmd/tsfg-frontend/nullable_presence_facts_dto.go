// SPDX-License-Identifier: Apache-2.0

// Two actual command callers select a protocol before any request is decoded.
package main

import (
	"fmt"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
)

const nullablePresenceFactsSchema = "sha256:4205bc18e817d983dd0f91d5b1c1d92b99cff5f6e32eca419d22530bfdcf41ac"

type nullableFactsVersion uint8

const (
	nullableFactsV3 nullableFactsVersion = 3
	nullableFactsV4 nullableFactsVersion = 4
)

func (v nullableFactsVersion) schemaHash() string {
	if v == nullableFactsV4 {
		return nullablePresenceFactsSchema
	}
	return nullableFactsSchema
}

func (v nullableFactsVersion) requestDomain() string {
	if v == nullableFactsV4 {
		return "tsfg.nullable-facts.request.v4\x00"
	}
	return "tsfg.nullable-facts.request.v3\x00"
}

func (v nullableFactsVersion) packageDomain() string {
	if v == nullableFactsV4 {
		return "tsfg.nullable-facts.package.v4\x00"
	}
	return "tsfg.nullable-facts.package.v3\x00"
}

// This error is never an output packet. Cause preserves the real boundary error.
type nullablePresenceFactsError struct {
	Kind   string
	Detail string
	File   string
	Range  span
	Cause  error
}

func (e *nullablePresenceFactsError) Error() string {
	message := fmt.Sprintf("[r03-nullable-facts-%s] %s", e.Kind, e.Detail)
	if e.File != "" {
		message += fmt.Sprintf(" at %s:%d:%d", e.File, e.Range.Start, e.Range.End)
	}
	return message
}

func (e *nullablePresenceFactsError) Unwrap() error { return e.Cause }

func (v nullableFactsVersion) failure(kind, detail string, causes ...error) error {
	if v == nullableFactsV3 {
		return nullableFactsError(kind, detail)
	}
	// V3's historical unicode category is retained only on its own route.
	if kind == "unicode" {
		kind = "json"
	}
	var cause error
	if len(causes) != 0 {
		cause = causes[0]
	}
	return &nullablePresenceFactsError{Kind: kind, Detail: detail, Cause: cause}
}

func (v nullableFactsVersion) at(kind, detail string, file *ast.SourceFile, node *ast.Node, causes ...error) error {
	err := v.failure(kind, detail, causes...)
	if located, ok := err.(*nullablePresenceFactsError); ok && file != nil && node != nil {
		located.File, located.Range = file.FileName(), offset(file, node)
	}
	return err
}

// Only private expression errors reach this adapter, never caller IO errors.
func (v nullableFactsVersion) expressionFailure(err error, file *ast.SourceFile, node *ast.Node) error {
	if v == nullableFactsV3 {
		return err
	}
	kind := "capability"
	if strings.HasPrefix(err.Error(), "[r03-nullable-facts-limit]") {
		kind = "limit"
	}
	return v.at(kind, "typed private expression", file, node, err)
}
