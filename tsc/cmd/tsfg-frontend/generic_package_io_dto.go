// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Candidate single-request IO remains separate from the default native command.
package main

import "github.com/microsoft/typescript-go/internal/json"

const genericPackageIOFamily = "tsfg.generic-package-io-candidate"
const genericPackageIORequestDomain = "tsfg:generic-package-io-request:v1\n"
const genericPackageIOMaxBytes = 16 << 20

func genericPackageIOPolicy() genericJSONPolicy {
	return genericJSONPolicy{maxBytes: genericPackageIOMaxBytes, maxDepth: 162, maxTokens: 1_032_768,
		maxString: 3 << 20, allowNUL: true, allowNull: true, allowRaw: true}
}

type genericPackageIOSource struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Digest  string `json:"digest"`
}

type genericPackageIOExportRequest struct {
	Family      string                       `json:"family"`
	Version     int                          `json:"version"`
	Operation   string                       `json:"operation"`
	Scope       string                       `json:"scope"`
	Source      genericPackageIOSource       `json:"source"`
	Environment genericPackageEnvironmentDTO `json:"environment"`
}

type genericPackageIOInstantiateRequest struct {
	Family    string               `json:"family"`
	Version   int                  `json:"version"`
	Operation string               `json:"operation"`
	Package   json.Value           `json:"package"`
	Roots     []genericPackageRoot `json:"roots"`
}

type genericPackageIOOK struct {
	Family    string     `json:"family"`
	Version   int        `json:"version"`
	Operation string     `json:"operation"`
	RequestID string     `json:"requestId"`
	Status    string     `json:"status"`
	Value     json.Value `json:"value"`
}

type genericPackageIOError struct {
	Family     string                     `json:"family"`
	Version    int                        `json:"version"`
	Operation  string                     `json:"operation"`
	RequestID  string                     `json:"requestId"`
	Status     string                     `json:"status"`
	Diagnostic genericPackageIODiagnostic `json:"diagnostic"`
}

type genericPackageIODiagnostic struct {
	Kind    string                   `json:"kind"`
	Code    int32                    `json:"code"`
	Detail  string                   `json:"detail"`
	Primary *genericPackageIOPrimary `json:"primary"`
}

type genericPackageIOPrimary struct {
	Source genericPackageSourceDTO `json:"source"`
	Origin genericPackageOriginDTO `json:"origin"`
}
