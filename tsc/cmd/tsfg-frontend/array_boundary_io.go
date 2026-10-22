// SPDX-License-Identifier: Apache-2.0
// Closed path/cardinality checks and cancellation-aware input for Array v8.
package main

import (
	"bytes"
	"context"
	"errors"
	"github.com/microsoft/typescript-go/internal/json"
	"io"
	"strings"
)

func arrayFactsRequestArrayLimit(data []byte, maximum int) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	containers := []int{}
	for {
		token, err := decoder.ReadToken()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return arrayFactsError("json", "request array boundary", err)
		}
		kind, last := token.Kind(), len(containers)-1
		if kind != '}' && kind != ']' && last >= 0 && containers[last] >= 0 {
			containers[last]++
			if containers[last] > maximum {
				return arrayFactsError("limit", "closed request array cardinality", nil)
			}
		}
		switch kind {
		case '[':
			containers = append(containers, 0)
		case '{':
			containers = append(containers, -1)
		case ']', '}':
			if last < 0 {
				return arrayFactsError("json", "request container boundary", nil)
			}
			containers = containers[:last]
		}
	}
}
func arrayFactsPath(path string) bool {
	return strings.HasPrefix(path, "/") && path != "/" && canonicalProjectPath(path) == path && !strings.ContainsAny(path, "\x00\\")
}

type arrayFactsReader struct {
	ctx   context.Context
	input io.Reader
}

func (r arrayFactsReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.input.Read(p)
	if canceled := r.ctx.Err(); canceled != nil {
		return n, errors.Join(err, canceled)
	}
	return n, err
}
