// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Candidate wire DTOs are explicit and separate from the owned domain.
package main

const genericPackageFamily = "tsfg-owned-generic-package"
const genericPackageVersion = 7
const genericPackageSchema = "2e9d1e07016b6f8ce5f48bf8a3508491eaf1ce7e6fd43bf497c133f65c8dff94"
const genericPackageDomain = "tsfg-owned-generic-package/v7\n"
const genericDefinitionDomain = "tsfg:generic-definition:v1\n"
const genericInstanceDomain = "tsfg:generic-instance:v1\n"
const genericPackageMaxBytes = 8 << 20
const genericPackageMaxDepth = 160
const genericPackageMaxTokens = 1_000_000
const genericPackageMaxString = 1 << 20
const genericPackageMaxExpressions = 32768
const genericPackageMaxKeyBytes = 256
const genericPackageMaxKeyMembers = 16

// Return owned storage so callers cannot mutate protocol capabilities globally.
func genericPackageCapabilities() []string {
	return []string{"conditional-generic-call", "conditional-signature", "explicit-type-arguments", "mapped-indexed-generic-call", "mapped-indexed-signature", "number-add", "scalar-generics", "single-module"}
}

type genericPackageDTO struct {
	Family     string                   `json:"family"`
	PackageID  string                   `json:"packageId"`
	Payload    genericPackagePayloadDTO `json:"payload"`
	SchemaHash string                   `json:"schemaHash"`
	Version    int                      `json:"version"`
}

type genericPackagePayloadDTO struct {
	Capabilities    []string                      `json:"capabilities"`
	Definitions     []genericPackageDefinitionDTO `json:"definitions"`
	Environment     genericPackageEnvironmentDTO  `json:"environment"`
	Scope           string                        `json:"scope"`
	SemanticProfile string                        `json:"semanticProfile"`
	Source          genericPackageSourceDTO       `json:"source"`
}

type genericPackageSourceDTO struct {
	Digest          string `json:"digest"`
	Length          int    `json:"length"`
	Path            string `json:"path"`
	PositionUnit    string `json:"positionUnit"`
	SurrogateStarts []int  `json:"surrogateStarts"`
}

type genericPackageOriginDTO struct {
	End   int `json:"end"`
	Start int `json:"start"`
}

type genericPackageEnvironmentDTO struct {
	Files   []genericPackageDeclarationDTO `json:"files"`
	Profile string                         `json:"profile"`
}

type genericPackageDeclarationDTO struct {
	Content string `json:"content"`
	Digest  string `json:"digest"`
	Path    string `json:"path"`
}

type genericPackageDefinitionDTO struct {
	Body           genericPackageExpressionDTO  `json:"body"`
	Exported       bool                         `json:"exported"`
	Key            string                       `json:"key"`
	Name           string                       `json:"name"`
	Origin         genericPackageOriginDTO      `json:"origin"`
	Parameters     []genericPackageParameterDTO `json:"parameters"`
	ReturnType     genericPackageTypeDTO        `json:"returnType"`
	TypeParameters []string                     `json:"typeParameters"`
}

type genericPackageParameterDTO struct {
	Name   string                  `json:"name"`
	Origin genericPackageOriginDTO `json:"origin"`
	Type   genericPackageTypeDTO   `json:"type"`
}

type genericPackageTypeDTO struct {
	Kind        string                        `json:"kind"`
	Parameter   int                           `json:"parameter"`
	CheckType   *genericPackageTypeDTO        `json:"checkType,omitempty"`
	ExtendsType *genericPackageTypeDTO        `json:"extendsType,omitempty"`
	TrueType    *genericPackageTypeDTO        `json:"trueType,omitempty"`
	FalseType   *genericPackageTypeDTO        `json:"falseType,omitempty"`
	Mapped      *genericPackageMappedTypeDTO  `json:"mapped,omitempty"`
	Indexed     *genericPackageIndexedTypeDTO `json:"indexed,omitempty"`
}

type genericPackageKeyTypeDTO struct {
	Kind     string   `json:"kind"`
	Literals []string `json:"literals"`
}

type genericPackageMappedTypeDTO struct {
	Binder string                   `json:"binder"`
	Keys   genericPackageKeyTypeDTO `json:"keys"`
	Value  genericPackageTypeDTO    `json:"value"`
}

type genericPackageIndexedTypeDTO struct {
	Object genericPackageTypeDTO    `json:"object"`
	Index  genericPackageKeyTypeDTO `json:"index"`
}

type genericPackageExpressionDTO struct {
	Arguments     []genericPackageExpressionDTO `json:"arguments"`
	Definition    string                        `json:"definition"`
	Kind          string                        `json:"kind"`
	Origin        genericPackageOriginDTO       `json:"origin"`
	Parameter     int                           `json:"parameter"`
	TypeArguments []genericPackageTypeDTO       `json:"typeArguments"`
}

type genericPackageDefinitionIdentityDTO struct {
	Name   string                  `json:"name"`
	Origin genericPackageOriginDTO `json:"origin"`
	Scope  string                  `json:"scope"`
	Source genericPackageSourceDTO `json:"source"`
}

type genericPackageInstanceIdentityDTO struct {
	Definition      string                  `json:"definition"`
	GraphDigest     string                  `json:"graphDigest"`
	Scope           string                  `json:"scope"`
	SemanticProfile string                  `json:"semanticProfile"`
	TypeArguments   []genericPackageTypeDTO `json:"typeArguments"`
}
