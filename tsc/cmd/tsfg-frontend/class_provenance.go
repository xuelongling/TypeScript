// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Private v2 provenance hashes the complete environment once per projection.
package main

import (
	"crypto/sha256"
	"fmt"
)

const (
	classEnvironmentDomain = "tsfg.candidate.class-environment.v2\x00"
	classAllocationDomain  = "tsfg.candidate.class-allocation-provenance.v2\x00"
)

func classDomainDigest(domain string, canonical []byte) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte(domain))
	_, _ = digest.Write(canonical)
	return fmt.Sprintf("sha256:%x", digest.Sum(nil))
}

func (p *classProjection) initializeEnvironment() error {
	payload := struct {
		Profile string               `json:"profile"`
		Scope   string               `json:"scope"`
		Sources []classSource        `json:"sources"`
		Options classCompilerOptions `json:"options"`
	}{Profile: p.graph.SemanticProfile, Scope: p.graph.Scope, Sources: p.graph.Sources, Options: p.graph.Options}
	canonical, err := p.resources.canonicalJSON(&payload, "owned environment", classGraphByteLimit, classAnchor{})
	if err != nil {
		return err
	}
	p.environmentDigest = classDomainDigest(classEnvironmentDomain, canonical)
	return nil
}

func (p *classProjection) runtimeKey(declaration classAnchor) (string, error) {
	if p.environmentDigest == "" {
		return "", classError("identity", "candidate environment digest missing", declaration)
	}
	// The owned metadata lets a cold consumer calculate E once. Each class
	// binds only E and its declaration, never re-encodes options or all sources.
	// This allocation provenance never decides ordinary assignability.
	payload := struct {
		EnvironmentDigest string      `json:"environmentDigest"`
		Declaration       classAnchor `json:"declaration"`
	}{EnvironmentDigest: p.environmentDigest, Declaration: declaration}
	canonical, err := p.resources.canonicalJSON(&payload, "owned allocation provenance", classGraphByteLimit, declaration)
	if err != nil {
		return "", err
	}
	return classDomainDigest(classAllocationDomain, canonical), nil
}
