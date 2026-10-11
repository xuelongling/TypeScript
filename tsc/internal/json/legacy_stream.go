// Copyright xuelongling. Licensed under the Apache License, Version 2.0.
// SPDX-License-Identifier: Apache-2.0
//
//nolint:depguard // This internal JSON boundary alone selects the v1 compatibility profile for existing command protocols.
package json

import (
	"io"

	jsonv1 "github.com/go-json-experiment/json/v1"
)

// NewLegacyDecoder preserves the existing encoding/json command wire semantics,
// including DisallowUnknownFields, number decoding and one-value stream reads.
func NewLegacyDecoder(input io.Reader) *jsonv1.Decoder { return jsonv1.NewDecoder(input) }

// NewLegacyEncoder preserves omitempty, HTML escaping and the stream newline.
func NewLegacyEncoder(output io.Writer) *jsonv1.Encoder { return jsonv1.NewEncoder(output) }
