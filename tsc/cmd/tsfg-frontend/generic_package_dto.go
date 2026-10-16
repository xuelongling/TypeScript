// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Candidate wire DTOs are explicit and separate from the owned domain.
package main

const genericPackageFamily = "tsfg-owned-generic-package"
const genericPackageVersion = 1
const genericPackageSchema = "5f3e8a6e8e3b49dcbdf507db643c538a5810cc3a09f75ea7dc4db560086e9e4b"
const genericPackageDomain = "tsfg-owned-generic-package/v1\n"
const genericDefinitionDomain = "tsfg:generic-definition:v1\n"
const genericInstanceDomain = "tsfg:generic-instance:v1\n"
const genericPackageMaxBytes = 8 << 20
const genericPackageMaxDepth = 160
const genericPackageMaxTokens = 1_000_000
const genericPackageMaxString = 1 << 20
const genericPackageMaxExpressions = 32768

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
	Kind      string `json:"kind"`
	Parameter int    `json:"parameter"`
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
