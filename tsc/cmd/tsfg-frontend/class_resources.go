// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// One owner accounts for explicit projection work and fixed candidate wire limits.
package main

import (
	"context"

	"github.com/microsoft/typescript-go/internal/ast"
)

const (
	classOptionsByteLimit = 1 << 20
	classGraphByteLimit   = 8 << 20
	classJSONDepthLimit   = 128
	classJSONTokenLimit   = 200000
)

type classResources struct {
	ctx      context.Context
	maxNodes int
	maxDepth int
	nodes    int
}

func (r *classResources) inputNode(node *ast.Node, depth int) error {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	if r.nodes >= r.maxNodes || depth > r.maxDepth {
		return classError("limit", "official input node/depth budget", classNodeAnchor(node))
	}
	r.nodes++
	return nil
}

func (r *classResources) memberEdges(count int, origin classAnchor) error {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	if count > r.maxNodes-r.nodes {
		return classError("limit", "owned member projection node budget", origin)
	}
	r.nodes += count
	return nil
}
