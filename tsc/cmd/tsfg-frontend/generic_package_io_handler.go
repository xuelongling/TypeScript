// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Strict candidate framing precedes operation-specific domain validation.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/microsoft/typescript-go/internal/json"
)

func genericPackageIORequestID(data []byte) string {
	hash := sha256.New()
	hash.Write([]byte(genericPackageIORequestDomain))
	hash.Write(data)
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

func genericPackageIOFrame(data []byte) (string, error) {
	if err := genericJSONPreflight(data, genericPackageIOPolicy()); err != nil {
		return "", err
	}
	canonical := json.Value(bytes.Clone(data))
	if err := canonical.Canonicalize(); err != nil || !bytes.Equal(canonical, data) {
		return "", genericPackageError("canonical", "candidate request must be JCS")
	}
	var fields map[string]json.Value
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return "", genericPackageError("schema", "candidate request object")
	}
	var family, operation string
	var version int
	if json.Unmarshal(fields["family"], &family) != nil || json.Unmarshal(fields["version"], &version) != nil ||
		json.Unmarshal(fields["operation"], &operation) != nil || family != genericPackageIOFamily || version != 1 {
		return "", genericPackageError("schema", "candidate family/version/operation")
	}
	keys := []string{"family", "version", "operation"}
	switch operation {
	case "export":
		keys = append(keys, "scope", "source", "environment")
	case "instantiate":
		keys = append(keys, "package", "roots")
	default:
		return "", genericPackageError("schema", "candidate operation")
	}
	if len(fields) != len(keys) {
		return "", genericPackageError("schema", "candidate top-level fields")
	}
	for _, key := range keys {
		if _, found := fields[key]; !found {
			return "", genericPackageError("schema", "candidate top-level fields")
		}
	}
	return operation, nil
}

func decodeGenericPackageIOPayload(data []byte, target any) error {
	if err := json.Unmarshal(data, target); err != nil {
		return genericPackageError("json", "candidate payload DTO")
	}
	canonical, err := genericJSONCanonical(target, genericPackageIOPolicy())
	if err != nil {
		return err
	}
	if !bytes.Equal(data, canonical) {
		return genericPackageError("schema", "candidate payload exact fields/types")
	}
	return nil
}

func handleGenericPackageCandidate(ctx context.Context, data []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	operation, err := genericPackageIOFrame(data)
	if contextErr := ctx.Err(); contextErr != nil {
		return nil, contextErr
	}
	if err != nil {
		return nil, err
	}
	requestID := genericPackageIORequestID(data)
	var value []byte
	switch operation {
	case "export":
		var r genericPackageIOExportRequest
		err = decodeGenericPackageIOPayload(data, &r)
		if err == nil {
			value, err = exportGenericPackageFromInputs(ctx, r)
		}
	case "instantiate":
		var r genericPackageIOInstantiateRequest
		err = decodeGenericPackageIOPayload(data, &r)
		if err == nil && (len(r.Package) == 0 || r.Package[0] != '{' || r.Roots == nil) {
			err = genericPackageError("schema", "package object and roots array required")
		}
		if err == nil {
			var facts genericPackageFacts
			facts, err = instantiateGenericPackageFacts(ctx, r.Package, r.Roots)
			if err == nil {
				value, err = genericPackageCanonical(facts)
			}
		}
	}
	if contextErr := ctx.Err(); contextErr != nil {
		return nil, contextErr
	}
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		var failure *genericFailure
		if !errors.As(err, &failure) {
			return nil, err
		}
		diagnostic := genericPackageIODiagnostic{Kind: failure.Kind, Code: failure.Code, Detail: failure.Detail}
		if failure.Source.Path != "" {
			diagnostic.Primary = &genericPackageIOPrimary{Source: genericPackageSource(failure.Source), Origin: genericPackageOrigin(failure.Origin)}
		}
		return encodeGenericPackageIOResponse(ctx, genericPackageIOError{Family: genericPackageIOFamily, Version: 1, Operation: operation,
			RequestID: requestID, Status: "error", Diagnostic: diagnostic})
	}
	return encodeGenericPackageIOResponse(ctx, genericPackageIOOK{Family: genericPackageIOFamily, Version: 1, Operation: operation,
		RequestID: requestID, Status: "ok", Value: json.Value(value)})
}

func encodeGenericPackageIOResponse(ctx context.Context, value any) ([]byte, error) {
	data, err := genericJSONCanonical(value, genericPackageIOPolicy())
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return data, nil
}
