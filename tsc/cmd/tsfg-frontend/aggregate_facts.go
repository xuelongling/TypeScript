// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

package main

const markerVersion = "minimal-aggregates-v1"

// These declarations are compile-time markers. Their ordinary standard
// decorator signatures are checked by the official frontend; no decorator is
// executed and no replacement constructor is accepted.
const markerDeclarations = `// SPDX-License-Identifier: MIT
interface ClassDecoratorContext<C> {
  readonly kind: "class";
  readonly name: string | undefined;
  addInitializer(initializer: (this: C) => void): void;
}
declare function tsfgValue<C>(target: C, context: ClassDecoratorContext<C>): void;
declare function tsfgComponent<C>(target: C, context: ClassDecoratorContext<C>): void;
`

type markerEntry struct {
	Name     string `json:"name"`
	Category string `json:"category"`
}

type markerProfile struct {
	Version      string        `json:"version"`
	Path         string        `json:"path"`
	SHA256       string        `json:"sha256"`
	ContextName  string        `json:"contextName"`
	ContextArity int           `json:"contextArity"`
	Mode         string        `json:"mode"`
	Entries      []markerEntry `json:"entries"`
}

type markerOrigin struct {
	DeclarationPath   string `json:"declarationPath"`
	DeclarationRange  span   `json:"declarationRange"`
	DeclarationSHA256 string `json:"declarationSha256"`
	EntryID           string `json:"entryId"`
	Version           string `json:"version"`
	ContextName       string `json:"contextName"`
	ContextArity      int    `json:"contextArity"`
	Mode              string `json:"mode"`
	Range             span   `json:"range"`
}

type nominalKey struct {
	SourcePath      string   `json:"sourcePath"`
	DeclarationPath []string `json:"declarationPath"`
	Category        string   `json:"category"`
}

type sourceCoverage struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Symbol    string `json:"symbol,omitempty"`
	Range     span   `json:"range"`
	BodyRange *span  `json:"bodyRange"`
}

type fieldPermission struct {
	Readable        bool   `json:"readable"`
	Writable        bool   `json:"writable"`
	ConstructorSelf bool   `json:"constructorSelf"`
	ClassRef        string `json:"classRef"`
}

type aggregateField struct {
	ID                      string      `json:"id"`
	Ordinal                 int         `json:"ordinal"`
	Name                    string      `json:"name"`
	Symbol                  string      `json:"symbol"`
	Type                    string      `json:"type"`
	Readonly                bool        `json:"readonly"`
	Access                  string      `json:"access"`
	OptionalSyntax          bool        `json:"optionalSyntax"`
	DefiniteAssertionSyntax bool        `json:"definiteAssertionSyntax"`
	Initializer             *expression `json:"initializer"`
	Range                   span        `json:"range"`
}

type aggregateConstructor struct {
	ID                    string        `json:"id"`
	Kind                  string        `json:"kind"`
	ClassRef              string        `json:"classRef"`
	SignatureDeclaration  *span         `json:"signatureDeclaration"`
	Implementation        *span         `json:"implementation"`
	Parameters            []symbol      `json:"parameters"`
	Signature             signature     `json:"signature"`
	ResultType            string        `json:"resultType"`
	Body                  []statement   `json:"body"`
	ParameterInitializers []*expression `json:"parameterInitializers"`
	Range                 span          `json:"range"`
	BodyRange             *span         `json:"bodyRange"`
}

type aggregateDeclaration struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Symbol          string                 `json:"symbol"`
	Category        string                 `json:"category"`
	NominalKey      nominalKey             `json:"nominalKey"`
	MarkerOrigin    markerOrigin           `json:"markerOrigin"`
	InstanceType    string                 `json:"instanceType"`
	ConstructorType string                 `json:"constructorType"`
	Fields          []aggregateField       `json:"fields"`
	Constructors    []aggregateConstructor `json:"constructors"`
	MemberCoverage  []sourceCoverage       `json:"memberCoverage"`
	Range           span                   `json:"range"`
	BodyRange       span                   `json:"bodyRange"`
	NoInheritance   bool                   `json:"noInheritance"`
}

type aggregateFacts struct {
	Profile        string                 `json:"profile"`
	Declarations   []aggregateDeclaration `json:"declarations"`
	SourceCoverage []sourceCoverage       `json:"sourceCoverage"`
}
