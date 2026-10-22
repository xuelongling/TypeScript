// SPDX-License-Identifier: Apache-2.0
// Strict canonical JSON and bounded identities for the owned Array boundary.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/microsoft/typescript-go/internal/json"
	"strings"
)

const arrayFactsMaxBytes = 2097152

type arrayFactsFailure struct {
	Kind        string       `json:"kind"`
	Code        string       `json:"code"`
	RuleID      string       `json:"ruleId"`
	Phase       string       `json:"phase"`
	Detail      string       `json:"detail"`
	Diagnostics []diagnostic `json:"diagnostics,omitempty"`
	Cause       error        `json:"-"`
}

func (e *arrayFactsFailure) Error() string { return fmt.Sprintf("[%s] %s", e.Code, e.Detail) }
func (e *arrayFactsFailure) Unwrap() error { return e.Cause }
func arrayFactsError(kind, detail string, cause error) *arrayFactsFailure {
	return &arrayFactsFailure{Kind: kind, Code: "r03-array-facts-" + kind, RuleID: "r01-input-boundary", Phase: "input", Detail: detail, Cause: cause}
}
func arrayFactsPolicy(allowNull bool) genericJSONPolicy {
	return genericJSONPolicy{maxBytes: arrayFactsMaxBytes, maxDepth: 64, maxTokens: 200000, maxString: 1048576, allowNUL: true, allowNull: allowNull}
}
func arrayFactsJSONError(err error) error {
	var failure *genericFailure
	if errors.As(err, &failure) {
		return arrayFactsError(strings.TrimPrefix(failure.Kind, "package-"), failure.Detail, err)
	}
	return arrayFactsError("json", "strict JSON boundary", err)
}
func arrayFactsCanonical(value any) ([]byte, error) {
	if err := arrayFactsDTOBudget(value, 0); err != nil {
		return nil, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, arrayFactsError("json", "DTO encoding", err)
	}
	canonical := json.Value(data)
	if err := canonical.Canonicalize(); err != nil {
		return nil, arrayFactsError("json", "JCS encoding", err)
	}
	return []byte(canonical), nil
}
func arrayFactsSHA(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func arrayFactsID(domain string, value any) (string, error) {
	data, err := arrayFactsCanonical(value)
	if err != nil {
		return "", err
	}
	return "sha256:" + arrayFactsSHA(append([]byte(domain), data...)), nil
}
