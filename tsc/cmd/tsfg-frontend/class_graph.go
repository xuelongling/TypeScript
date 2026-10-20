// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Candidate owned class facts. This is not an adopted RPC or runtime TypeId.
package main

import "fmt"

type classAnchor struct {
	Source string `json:"source"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
}

type classInput struct {
	Path        string `json:"path"`
	SHA256      string `json:"sha256"`
	Declaration bool   `json:"declaration"`
}

type classSource struct {
	classInput
	UTF16Length     int   `json:"utf16Length"`
	SurrogateStarts []int `json:"surrogateStarts"`
}

type classExportOptions struct {
	Scope    string
	Inputs   []classInput
	MaxNodes int
	MaxDepth int
}

type classStrictOptions struct {
	StrictNullChecks             bool `json:"strictNullChecks"`
	StrictFunctionTypes          bool `json:"strictFunctionTypes"`
	StrictBindCallApply          bool `json:"strictBindCallApply"`
	StrictPropertyInitialization bool `json:"strictPropertyInitialization"`
	StrictBuiltinIteratorReturn  bool `json:"strictBuiltinIteratorReturn"`
	NoImplicitAny                bool `json:"noImplicitAny"`
	NoImplicitThis               bool `json:"noImplicitThis"`
	UseUnknownInCatchVariables   bool `json:"useUnknownInCatchVariables"`
}

type classCompilerOptions struct {
	EffectiveStrict         classStrictOptions `json:"effectiveStrict"`
	OfficialOptions         string             `json:"officialOptions"`
	OfficialOptionsSHA256   string             `json:"officialOptionsSha256"`
	Target                  int32              `json:"target"`
	Module                  int32              `json:"module"`
	UseDefineForClassFields bool               `json:"useDefineForClassFields"`
}

type classType struct {
	Scalar       string              `json:"scalar"`
	Nominal      *classScalarNominal `json:"nominal"`
	Literal      string              `json:"literal"`
	Target       string              `json:"target"`
	ThisArgument string              `json:"thisArgument"`
	ID           string              `json:"id"`
	Kind         string              `json:"kind"`
	Declaration  string              `json:"declaration"`
}

type classSymbol struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Kind        string      `json:"kind"`
	Type        string      `json:"type"`
	Declaration classAnchor `json:"declaration"`
}

type classPermission struct {
	Readable bool `json:"readable"`
	Writable bool `json:"writable"`
	Readonly bool `json:"readonly"`
	WriteUse bool `json:"writeUse"`
}

type classMemberProjection struct {
	Target string `json:"target"`
	Source string `json:"source"`
}

type classCompatibility struct {
	SourceType string                  `json:"sourceType"`
	TargetType string                  `json:"targetType"`
	Assignable bool                    `json:"assignable"`
	Members    []classMemberProjection `json:"members"`
	Origin     classAnchor             `json:"origin"`
}

type classExpression struct {
	NumericLiteral        *classNumericLiteral `json:"numericLiteral"`
	Compatibility         *classCompatibility  `json:"compatibility"`
	ID                    string               `json:"id"`
	Dispatch              string               `json:"dispatch"`
	StaticTarget          string               `json:"staticTarget"`
	Arguments             []*classExpression   `json:"arguments"`
	ArgumentCompatibility []classCompatibility `json:"argumentCompatibility"`
	ResolvedSignature     *classSignature      `json:"resolvedSignature"`
	Kind                  string               `json:"kind"`
	Type                  string               `json:"type"`
	Symbol                string               `json:"symbol"`
	Token                 string               `json:"token"`
	Operator              string               `json:"operator"`
	Receiver              *classExpression     `json:"receiver"`
	Left                  *classExpression     `json:"left"`
	Right                 *classExpression     `json:"right"`
	Permission            *classPermission     `json:"permission"`
	Origin                classAnchor          `json:"origin"`
}

type classStatementBlock struct {
	Body   []classStatement `json:"body"`
	Origin classAnchor      `json:"origin"`
}

// This private profile admits only a method-level terminal if with two returns.
type classConditional struct {
	Condition              *classExpression    `json:"condition"`
	ConditionCompatibility classCompatibility  `json:"conditionCompatibility"`
	Then                   classStatementBlock `json:"then"`
	Else                   classStatementBlock `json:"else"`
}

type classStatement struct {
	Branch        *classConditional    `json:"branch,omitempty"`
	Local         *classLocalStatement `json:"local"`
	Compatibility *classCompatibility  `json:"compatibility"`
	Kind          string               `json:"kind"`
	Expression    *classExpression     `json:"expression"`
	Origin        classAnchor          `json:"origin"`
}

type classParameter struct {
	InheritedFrom string      `json:"inheritedFrom"`
	Symbol        string      `json:"symbol"`
	Type          string      `json:"type"`
	Origin        classAnchor `json:"origin"`
}

type classSignature struct {
	Parameters []classParameter `json:"parameters"`
	Return     string           `json:"return"`
}

type classField struct {
	InitializerCompatibility *classCompatibility `json:"initializerCompatibility"`
	ID                       string              `json:"id"`
	Symbol                   string              `json:"symbol"`
	Name                     string              `json:"name"`
	Ordinal                  int                 `json:"ordinal"`
	Type                     string              `json:"type"`
	Access                   string              `json:"access"`
	Readonly                 bool                `json:"readonly"`
	DefiniteAssertion        bool                `json:"definiteAssertion"`
	Initializer              *classExpression    `json:"initializer"`
	Origin                   classAnchor         `json:"origin"`
}

type classImplicitParameter struct {
	Source        string             `json:"source"`
	Target        string             `json:"target"`
	Compatibility classCompatibility `json:"compatibility"`
}

type classImplicitForward struct {
	Target        string                   `json:"target"`
	BaseSignature classSignature           `json:"baseSignature"`
	Parameters    []classImplicitParameter `json:"parameters"`
	Origin        classAnchor              `json:"origin"`
}

type classConstructor struct {
	Access               string                `json:"access"`
	ImplicitForward      *classImplicitForward `json:"implicitForward"`
	ID                   string                `json:"id"`
	SuperCall            string                `json:"superCall"`
	Kind                 string                `json:"kind"`
	Signature            classSignature        `json:"signature"`
	Body                 []classStatement      `json:"body"`
	InitializationPhase  string                `json:"initializationPhase"`
	InitializationFields []string              `json:"initializationFields"`
	Origin               classAnchor           `json:"origin"`
}

type classMethod struct {
	OverrideTarget string           `json:"overrideTarget"`
	Access         string           `json:"access"`
	ID             string           `json:"id"`
	Symbol         string           `json:"symbol"`
	Name           string           `json:"name"`
	Signature      classSignature   `json:"signature"`
	Body           []classStatement `json:"body"`
	Origin         classAnchor      `json:"origin"`
}

type classDeclaration struct {
	Base            string           `json:"base"`
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Symbol          string           `json:"symbol"`
	RuntimeKey      string           `json:"runtimeKey"`
	InstanceType    string           `json:"instanceType"`
	ConstructorType string           `json:"constructorType"`
	ThisType        string           `json:"thisType"`
	Fields          []classField     `json:"fields"`
	Constructor     classConstructor `json:"constructor"`
	Methods         []classMethod    `json:"methods"`
	Origin          classAnchor      `json:"origin"`
}

type classGraph struct {
	SemanticProfile string               `json:"semanticProfile"`
	Scope           string               `json:"scope"`
	Sources         []classSource        `json:"sources"`
	Options         classCompilerOptions `json:"options"`
	Types           []classType          `json:"types"`
	Symbols         []classSymbol        `json:"symbols"`
	Classes         []classDeclaration   `json:"classes"`
}

type classFailure struct {
	Kind   string
	Detail string
	Code   int32
	Origin classAnchor
}

func (e *classFailure) Error() string {
	return fmt.Sprintf("owned class %s (%d) at %s:%d-%d: %s", e.Kind, e.Code,
		e.Origin.Source, e.Origin.Start, e.Origin.End, e.Detail)
}

func classError(kind, detail string, origin classAnchor) error {
	return &classFailure{Kind: kind, Detail: detail, Origin: origin}
}
