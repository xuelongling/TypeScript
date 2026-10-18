// SPDX-License-Identifier: Apache-2.0
// V2 keeps the common IO rules and binds only its own schema and identity domains.
package main

import (
	"context"
	"io"
)

func runStringFactsConcatCandidate(ctx context.Context, input io.Reader, output io.Writer) error {
	data, err := readStringFactsCommand(ctx, input)
	if err != nil {
		return err
	}
	r, err := decodeStringFactsConcatRequest(data)
	if err != nil {
		return err
	}
	owned, err := stringFactsInputs(r.Input)
	if err != nil {
		return err
	}
	payload, err := projectStringFactsConcat(ctx, owned, r.Input)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return stringFactsError("canceled", "after checking", err)
	}
	payloadBytes, err := stringFactsCanonical(payload)
	if err != nil {
		return err
	}
	packet := stringFactsConcatPackage{Family: "tsfg.string-facts-package-candidate", Version: 2, SchemaHash: stringFactsConcatSchema,
		RequestID: "sha256:" + stringFactsSHA(append([]byte("tsfg.string-facts.request.v2\x00"), data...)),
		PackageID: "sha256:" + stringFactsSHA(append([]byte("tsfg.string-facts.package.v2\x00"+stringFactsConcatSchema+"\x00"), payloadBytes...)), Payload: payload}
	return publishStringFactsPacket(ctx, output, packet)
}
