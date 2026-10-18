// SPDX-License-Identifier: Apache-2.0
// Private string-facts v1 owns its exact DTO and failures; no legacy DTO extension.
package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/microsoft/typescript-go/internal/json"
)

const stringFactsMaxBytes = 2097152
const stringFactsSchema = "sha256:6dfed46326c15a81aeb6a1ea9b5ed2fc10db7bb1bf27733ac3df530d78d5cc62"

//go:embed testdata/string-facts-v1/schema.json
var stringFactsSchemaBytes []byte

type stringFactsConfig struct {
	Strict  bool `json:"strict"`
	NoEmit  bool `json:"noEmit"`
	AllowJS bool `json:"allowJs"`
}
type stringFactsDeclarations struct {
	Version string   `json:"version"`
	Paths   []string `json:"paths"`
}
type stringFactsInputFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	SHA256  string `json:"sha256"`
}
type stringFactsInput struct {
	SourceRoot   string                  `json:"sourceRoot"`
	RootFile     string                  `json:"rootFile"`
	Files        []stringFactsInputFile  `json:"files"`
	Declarations stringFactsDeclarations `json:"declarations"`
	Config       stringFactsConfig       `json:"config"`
}
type stringFactsRequest struct {
	Family  string           `json:"family"`
	Version int              `json:"version"`
	Input   stringFactsInput `json:"input"`
}
type stringFactsOwnedFile struct {
	Path        string `json:"path"`
	Content     string `json:"content"`
	SHA256      string `json:"sha256"`
	ByteLength  int    `json:"byteLength"`
	UTF16Length int    `json:"utf16Length"`
}
type stringFactsValue struct {
	Kind      string  `json:"kind"`
	Type      string  `json:"type"`
	Range     span    `json:"range"`
	CodeUnits []int32 `json:"codeUnits"`
}
type stringFactsStatement struct {
	Kind  string           `json:"kind"`
	Range span             `json:"range"`
	Value stringFactsValue `json:"value"`
}
type stringFactsFunction struct {
	Name             string                 `json:"name"`
	SourcePath       string                 `json:"sourcePath"`
	DeclarationRange span                   `json:"declarationRange"`
	Parameters       []string               `json:"parameters"`
	ReturnType       string                 `json:"returnType"`
	Body             []stringFactsStatement `json:"body"`
}
type stringFactsPayload struct {
	SourceRoot   string                  `json:"sourceRoot"`
	RootFile     string                  `json:"rootFile"`
	PositionUnit string                  `json:"positionUnit"`
	Config       stringFactsConfig       `json:"config"`
	Declarations stringFactsDeclarations `json:"declarations"`
	Files        []stringFactsOwnedFile  `json:"files"`
	Functions    []stringFactsFunction   `json:"functions"`
}
type stringFactsPackage struct {
	Family     string             `json:"family"`
	Version    int                `json:"version"`
	SchemaHash string             `json:"schemaHash"`
	RequestID  string             `json:"requestId"`
	PackageID  string             `json:"packageId"`
	Payload    stringFactsPayload `json:"payload"`
}

type stringFactsFailure struct {
	Kind        string       `json:"kind"`
	Code        string       `json:"code"`
	RuleID      string       `json:"ruleId"`
	Phase       string       `json:"phase"`
	Detail      string       `json:"detail"`
	Diagnostics []diagnostic `json:"diagnostics,omitempty"`
	Cause       error        `json:"-"`
}

func (e *stringFactsFailure) Error() string { return fmt.Sprintf("[%s] %s", e.Code, e.Detail) }
func (e *stringFactsFailure) Unwrap() error { return e.Cause }
func stringFactsError(kind, detail string, cause error) *stringFactsFailure {
	return &stringFactsFailure{Kind: kind, Code: "r03-string-facts-" + kind, RuleID: "r01-input-boundary", Phase: "input", Detail: detail, Cause: cause}
}
func stringFactsJSONError(err error) error {
	var failure *genericFailure
	if errors.As(err, &failure) {
		return stringFactsError(strings.TrimPrefix(failure.Kind, "package-"), failure.Detail, err)
	}
	return stringFactsError("json", "strict JSON boundary", err)
}
func stringFactsPolicy(allowNull bool) genericJSONPolicy {
	return genericJSONPolicy{maxBytes: stringFactsMaxBytes, maxDepth: 64, maxTokens: 200000, maxString: 1048576, allowNUL: true, allowNull: allowNull}
}
func stringFactsCanonical(value any) ([]byte, error) {
	data, err := genericJSONCanonical(value, stringFactsPolicy(false))
	if err != nil {
		return nil, stringFactsJSONError(err)
	}
	return data, nil
}
func stringFactsSHA(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func decodeStringFactsRequest(data []byte) (stringFactsRequest, error) {
	return decodeStringFactsVersionRequest(data, 1, stringFactsSchema, stringFactsSchemaBytes)
}

// Both explicit versions share the same closed input DTO and strict decoding rules.
func decodeStringFactsVersionRequest(data []byte, version int, schema string, schemaBytes []byte) (stringFactsRequest, error) {
	var r stringFactsRequest
	if err := genericJSONPreflight(data, stringFactsPolicy(false)); err != nil {
		return r, stringFactsJSONError(err)
	}
	if err := stringFactsRequestArrays(data); err != nil {
		return r, err
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		return r, stringFactsError("canonical", "request requires JCS plus one LF", nil)
	}
	raw := data[:len(data)-1]
	canonical := json.Value(bytes.Clone(raw))
	if err := canonical.Canonicalize(); err != nil || !bytes.Equal(canonical, raw) {
		return r, stringFactsError("canonical", "request requires exact JCS plus one LF", err)
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return r, stringFactsError("schema", "request DTO", err)
	}
	encoded, err := stringFactsCanonical(r)
	if err != nil {
		return r, err
	}
	if !bytes.Equal(encoded, raw) {
		return r, stringFactsError("schema", "request exact fields and types", nil)
	}
	if r.Family != "tsfg.string-facts-request-candidate" || r.Version != version || "sha256:"+stringFactsSHA(schemaBytes) != schema {
		return r, stringFactsError("schema", "request family/version or embedded schema", nil)
	}
	return r, nil
}

// All legal request arrays are the two input files or the one declaration path.
// Count their immediate values with the official token decoder before DTO allocation.
func stringFactsRequestArrays(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	containers := []int{}
	for {
		token, err := decoder.ReadToken()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return stringFactsError("json", "request array token boundary", err)
		}
		kind := token.Kind()
		last := len(containers) - 1
		if kind != '}' && kind != ']' && last >= 0 && containers[last] >= 0 {
			containers[last]++
			if containers[last] > 2 {
				return stringFactsError("limit", "closed request array cardinality", nil)
			}
		}
		switch kind {
		case '[':
			containers = append(containers, 0)
		case '{':
			containers = append(containers, -1)
		case ']', '}':
			if last < 0 {
				return stringFactsError("json", "request array container boundary", nil)
			}
			containers = containers[:last]
		}
	}
}
