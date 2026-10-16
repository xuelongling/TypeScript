// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Owned request/response data and the stdin/stdout adapter used by native.go.
package main

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/json"
	"io"
)

type inputFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	SHA256  string `json:"sha256,omitempty"`
}

type request struct {
	CompilationClosure *closureConfiguration `json:"compilationClosure,omitempty"`
	HostImports        []hostImport          `json:"hostImports,omitempty"`
	Target             *integerTarget        `json:"target,omitempty"`
	SourceRoot         string                `json:"sourceRoot"`
	RootFile           string                `json:"rootFile"`
	Files              []inputFile           `json:"files"`
	Declarations       struct {
		MarkerProfile *markerProfile `json:"markerProfile,omitempty"`
		Version       string         `json:"version"`
		Paths         []string       `json:"paths"`
	} `json:"declarations"`
	Config struct {
		DecoratorMode string `json:"decoratorMode,omitempty"`
		Strict        bool   `json:"strict"`
		NoEmit        bool   `json:"noEmit"`
		AllowJS       bool   `json:"allowJs"`
	} `json:"config"`
}

type span struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type diagnostic struct {
	Code         string   `json:"code"`
	RuleID       string   `json:"ruleId"`
	RuleIDs      []string `json:"ruleIds"`
	Category     string   `json:"category"`
	Phase        string   `json:"phase"`
	Severity     string   `json:"severity"`
	File         string   `json:"file"`
	Range        span     `json:"range"`
	Primary      primary  `json:"primary"`
	UpstreamCode int32    `json:"upstreamCode,omitempty"`
}

type primary struct {
	SourceID    string `json:"sourceId"`
	Start       int    `json:"start"`
	End         int    `json:"end"`
	StartLine   int    `json:"startLine"`
	StartColumn int    `json:"startColumn"`
	EndLine     int    `json:"endLine"`
	EndColumn   int    `json:"endColumn"`
}

type symbol struct {
	SourcePath     string             `json:"sourcePath,omitempty"`
	BindingKind    string             `json:"bindingKind,omitempty"`
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Kind           string             `json:"kind"`
	Type           string             `json:"type"`
	Writable       bool               `json:"writable"`
	Range          span               `json:"range"`
	Presence       *parameterPresence `json:"presence,omitempty"`
	ReturnPresence *returnPresence    `json:"returnPresence,omitempty"`
}

type expression struct {
	Name         string            `json:"name,omitempty"`
	Receiver     *expression       `json:"receiver,omitempty"`
	FieldPath    []string          `json:"fieldPath,omitempty"`
	OwnerType    string            `json:"ownerType,omitempty"`
	Permission   *fieldPermission  `json:"permission,omitempty"`
	Conversion   *conversionOrigin `json:"conversion,omitempty"`
	Kind         string            `json:"kind"`
	Type         string            `json:"type"`
	Operator     string            `json:"operator,omitempty"`
	Prefix       *bool             `json:"prefix,omitempty"`
	Absence      string            `json:"absence,omitempty"`
	PresenceFlow *presenceFlow     `json:"presenceFlow,omitempty"`
	Left         *expression       `json:"left,omitempty"`
	Right        *expression       `json:"right,omitempty"`
	Operand      *expression       `json:"operand,omitempty"`
	Symbol       string            `json:"symbol,omitempty"`
	Target       string            `json:"target,omitempty"`
	Arguments    *[]expression     `json:"arguments,omitempty"`
	Signature    *signature        `json:"signature,omitempty"`
	Boolean      *bool             `json:"boolean,omitempty"`
	Literal      *integerLiteral   `json:"literal,omitempty"`
	Float        *floatLiteral     `json:"float,omitempty"`
	Range        span              `json:"range"`
}

type signature struct {
	Parameters     []string             `json:"parameters"`
	ReturnType     string               `json:"returnType"`
	Presence       []*parameterPresence `json:"presence,omitempty"`
	ReturnPresence *returnPresence      `json:"returnPresence,omitempty"`
}

type integerLiteral struct {
	Decimal string `json:"decimal"`
	Token   string `json:"token"`
}

type statement struct {
	Kind        string      `json:"kind"`
	Expression  *expression `json:"expression,omitempty"`
	Condition   *expression `json:"condition,omitempty"`
	Symbol      *symbol     `json:"symbol,omitempty"`
	Initializer *expression `json:"initializer,omitempty"`
	Target      *expression `json:"target,omitempty"`
	Value       *expression `json:"value,omitempty"`
	Body        []statement `json:"body,omitempty"`
	ThenBody    []statement `json:"thenBody,omitempty"`
	ElseBody    []statement `json:"elseBody,omitempty"`
	Range       span        `json:"range"`
}

type function struct {
	SourcePath            string          `json:"sourcePath,omitempty"`
	Name                  string          `json:"name"`
	Symbol                string          `json:"symbol"`
	Parameters            []symbol        `json:"parameters"`
	ReturnType            string          `json:"returnType"`
	ReturnPresence        *returnPresence `json:"returnPresence,omitempty"`
	Body                  []statement     `json:"body"`
	Range                 span            `json:"range"`
	ParameterInitializers []*expression   `json:"parameterInitializers,omitempty"`
}

type functionInfo struct {
	Declaration *ast.Node
	Conversion  *conversionOrigin
	ID          string
	Signature   signature
}

type response struct {
	Modules        []moduleFact     `json:"modules,omitempty"`
	HostImports    []hostImportFact `json:"hostImports,omitempty"`
	AggregateFacts *aggregateFacts  `json:"aggregateFacts,omitempty"`
	Status         string           `json:"status"`
	Inputs         []string         `json:"inputs,omitempty"`
	Functions      []function       `json:"functions,omitempty"`
	Symbols        []symbol         `json:"symbols"`
	Diagnostics    []diagnostic     `json:"diagnostics,omitempty"`
}

func runCommand(input io.Reader, output io.Writer) error {
	decoder := json.NewLegacyDecoder(input)
	decoder.DisallowUnknownFields()
	var r request
	if err := decoder.Decode(&r); err != nil {
		return err
	}
	return json.NewLegacyEncoder(output).Encode(analyze(r))
}
