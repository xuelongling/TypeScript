// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Candidate package bytes do not add or alter any public frontend RPC.
package main

import (
	"bytes"
	"context"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/json"
)

func encodeGenericPackage(g genericGraph) ([]byte, error) {
	if err := g.validate(); err != nil {
		return nil, err
	}
	payload := genericPackagePayload(g)
	if err := validateGenericPackagePayload(payload); err != nil {
		return nil, err
	}
	wire := genericPackageDTO{Family: genericPackageFamily, Version: genericPackageVersion, SchemaHash: genericPackageSchema,
		PackageID: "sha256:" + genericPackageIdentity(genericPackageDomain, payload), Payload: payload}
	data, err := genericPackageCanonical(wire)
	if err != nil {
		return nil, err
	}
	if err := genericJSONPreflight(data, genericPackageJSONPolicy()); err != nil {
		return nil, err
	}
	return data, nil
}

func decodeGenericPackage(data []byte) (genericGraph, error) {
	if len(data) == 0 || len(data) > genericPackageMaxBytes {
		return genericGraph{}, genericPackageError("limit", "package bytes")
	}
	data = bytes.Clone(data)
	if err := genericJSONPreflight(data, genericPackageJSONPolicy()); err != nil {
		return genericGraph{}, err
	}
	var wire genericPackageDTO
	if err := json.Unmarshal(data, &wire); err != nil {
		return genericGraph{}, genericPackageError("json", "typed DTO decoding")
	}
	if wire.Family != genericPackageFamily || wire.Version != genericPackageVersion || wire.SchemaHash != genericPackageSchema {
		return genericGraph{}, genericPackageError("schema", "family/version/schema identity")
	}
	if err := validateGenericPackagePayload(wire.Payload); err != nil {
		return genericGraph{}, err
	}
	if err := genericPackageBudget(wire.Payload); err != nil {
		return genericGraph{}, err
	}
	if wire.PackageID != "sha256:"+genericPackageIdentity(genericPackageDomain, wire.Payload) {
		return genericGraph{}, genericPackageError("identity", "packageId")
	}
	g := genericPackageOwnedGraph(wire.Payload)
	if err := g.validate(); err != nil {
		return genericGraph{}, err
	}
	canonical, err := encodeGenericPackage(g)
	if err != nil {
		return genericGraph{}, err
	}
	// Exact canonical comparison rejects unknown/missing/case-variant fields,
	// null arrays, alternate escaping and numeric forms without permissive shims.
	if !bytes.Equal(data, canonical) {
		return genericGraph{}, genericPackageError("canonical", "noncanonical or unknown/missing fields")
	}
	return g, nil
}

func exportGenericPackage(ctx context.Context, program *compiler.Program, file *ast.SourceFile, scope string, environment genericDeclarationEnvironment) ([]byte, error) {
	g, err := exportGenericGraph(ctx, program, file, scope, environment)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := encodeGenericPackage(g)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return data, nil
}
