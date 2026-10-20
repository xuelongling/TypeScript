// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0

// Official streaming JSON keeps tagged options and MarshalerTo behavior intact.
package main

import (
	"bytes"
	"errors"
	"io"
	"unicode/utf8"

	"github.com/microsoft/typescript-go/internal/json"
)

// This wrapper deliberately is not a *bytes.Buffer: every encoder flush must
// pass through Write, including its final buffered bytes.
type classJSONWriter struct {
	resources *classResources
	phase     string
	origin    classAnchor
	limit     int
	data      []byte
}

func (w *classJSONWriter) Write(data []byte) (int, error) {
	if err := w.resources.ctx.Err(); err != nil {
		return 0, err
	}
	if len(data) > w.limit-len(w.data) {
		return 0, classError("limit", w.phase+" byte budget", w.origin)
	}
	w.data = append(w.data, data...)
	return len(data), nil
}

func (r *classResources) canonicalJSON(value any, phase string, limit int, origin classAnchor) ([]byte, error) {
	if err := r.ctx.Err(); err != nil {
		return nil, err
	}
	writer := &classJSONWriter{resources: r, phase: phase, origin: origin, limit: limit}
	strings := json.MarshalToFunc(func(enc *json.Encoder, value string) error {
		if err := r.ctx.Err(); err != nil {
			return err
		}
		// OutputOffset includes buffered bytes. This lower bound rejects a huge
		// token before official quoting. Escaping may use a constant multiple
		// of this bounded token's length; Write enforces the exact total limit.
		remaining := int64(limit) - enc.OutputOffset()
		if remaining < 2 || int64(len(value)) > remaining-2 {
			return classError("limit", phase+" byte budget before string encoding", origin)
		}
		if !utf8.ValidString(value) {
			return classError("input", phase+" contains invalid UTF-8", origin)
		}
		// The locked API requires no encoder mutation before this fallback.
		return errors.ErrUnsupported
	})
	err := json.MarshalWrite(writer, value, json.AllowInvalidUTF8(false), json.WithMarshalers(strings))
	if err := r.ctx.Err(); err != nil {
		return nil, err
	}
	if err != nil {
		var failure *classFailure
		if errors.As(err, &failure) {
			return nil, failure
		}
		return nil, classError("input", phase+" official JSON encoding: "+err.Error(), origin)
	}
	if err := r.jsonShape(writer.data, phase, origin); err != nil {
		return nil, err
	}
	canonical := json.Value(writer.data)
	if err := canonical.Canonicalize(); err != nil {
		return nil, classError("input", phase+" official JSON canonicalization: "+err.Error(), origin)
	}
	if err := r.ctx.Err(); err != nil {
		return nil, err
	}
	if len(canonical) > limit {
		return nil, classError("limit", phase+" canonical byte budget", origin)
	}
	return canonical, nil
}

// The complete input here is already byte bounded. Count real official tokens,
// including object names and delimiters, before invoking JCS on nested data.
func (r *classResources) jsonShape(data []byte, phase string, origin classAnchor) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	depth, tokens := 0, 0
	for {
		if err := r.ctx.Err(); err != nil {
			return err
		}
		token, err := decoder.ReadToken()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return classError("input", phase+" official JSON tokenization: "+err.Error(), origin)
		}
		tokens++
		if tokens > classJSONTokenLimit {
			return classError("limit", phase+" token budget", origin)
		}
		switch token.Kind() {
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		}
		if depth > classJSONDepthLimit {
			return classError("limit", phase+" depth budget", origin)
		}
	}
}
