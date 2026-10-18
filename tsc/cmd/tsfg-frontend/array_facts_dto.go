// SPDX-License-Identifier: Apache-2.0
// Private ArrayFacts v1 owns its complete wire values; no legacy DTO changes.
package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/microsoft/typescript-go/internal/json"
)

const arrayFactsMaxBytes = 2097152
const arrayFactsSchema = "sha256:fbde82e304c30461ab69a2bb0c4d488fdeea0500c8d7d3f5dcda3764b03ec47e"
const arrayFactsDtsSHA = "04a97584458d9212250e94d4fc517264c277a9ce635fbb14de11452194fd76b2"

//go:embed testdata/array-facts-v1/schema.json
var arrayFactsSchemaBytes []byte

//go:embed testdata/array-facts-v1/minimal-arrays-v1.d.ts
var arrayFactsDtsBytes []byte

type arrayFactsConfig struct {
	Strict  bool `json:"strict"`
	NoEmit  bool `json:"noEmit"`
	AllowJS bool `json:"allowJs"`
}
type arrayFactsDeclarations struct {
	Version string   `json:"version"`
	Paths   []string `json:"paths"`
}
type arrayFactsInputFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	SHA256  string `json:"sha256"`
}
type arrayFactsInput struct {
	SourceRoot   string                 `json:"sourceRoot"`
	RootFile     string                 `json:"rootFile"`
	Files        []arrayFactsInputFile  `json:"files"`
	Declarations arrayFactsDeclarations `json:"declarations"`
	Config       arrayFactsConfig       `json:"config"`
}
type arrayFactsRequest struct {
	Family  string          `json:"family"`
	Version int             `json:"version"`
	Input   arrayFactsInput `json:"input"`
}
type arrayFactsOwnedFile struct {
	Path        string `json:"path"`
	Content     string `json:"content"`
	SHA256      string `json:"sha256"`
	ByteLength  int    `json:"byteLength"`
	UTF16Length int    `json:"utf16Length"`
}
type arrayFactsType struct {
	ID                  string `json:"id"`
	Kind                string `json:"kind"`
	ElementType         string `json:"elementType,omitempty"`
	SourcePath          string `json:"sourcePath"`
	DeclarationRange    span   `json:"declarationRange"`
	NameRange           span   `json:"nameRange"`
	IndexSignatureRange *span  `json:"indexSignatureRange,omitempty"`
}
type arrayFactsBinding struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	Kind                   string `json:"kind"`
	DeclaredType           string `json:"declaredType"`
	Writable               bool   `json:"writable"`
	DeclarationRange       span   `json:"declarationRange"`
	NameRange              span   `json:"nameRange"`
	AnnotationRange        *span  `json:"annotationRange"`
	ElementAnnotationRange *span  `json:"elementAnnotationRange"`
}
type arrayFactsValue struct {
	Kind           string             `json:"kind"`
	Type           string             `json:"type,omitempty"`
	Range          span               `json:"range"`
	Token          string             `json:"token,omitempty"`
	Bits           string             `json:"bits,omitempty"`
	Binding        string             `json:"binding,omitempty"`
	FlowType       string             `json:"flowType,omitempty"`
	Decimal        string             `json:"decimal,omitempty"`
	AllocationSite string             `json:"allocationSite,omitempty"`
	Elements       *[]arrayFactsValue `json:"elements,omitzero"`
	Array          *arrayFactsValue   `json:"array,omitempty"`
	Index          *arrayFactsValue   `json:"index,omitempty"`
}
type arrayFactsStatement struct {
	Kind        string           `json:"kind"`
	Range       span             `json:"range"`
	Binding     string           `json:"binding,omitempty"`
	Initializer *arrayFactsValue `json:"initializer,omitempty"`
	Value       *arrayFactsValue `json:"value,omitempty"`
}
type arrayFactsFunction struct {
	ID                    string                `json:"id"`
	Name                  string                `json:"name"`
	SourcePath            string                `json:"sourcePath"`
	DeclarationRange      span                  `json:"declarationRange"`
	NameRange             span                  `json:"nameRange"`
	ReturnType            string                `json:"returnType"`
	ReturnAnnotationRange span                  `json:"returnAnnotationRange"`
	Parameters            []string              `json:"parameters"`
	Bindings              []arrayFactsBinding   `json:"bindings"`
	Body                  []arrayFactsStatement `json:"body"`
}
type arrayFactsExport struct {
	Name       string `json:"name"`
	FunctionID string `json:"functionId"`
}
type arrayFactsPayload struct {
	SourceRoot   string                 `json:"sourceRoot"`
	RootFile     string                 `json:"rootFile"`
	PositionUnit string                 `json:"positionUnit"`
	Config       arrayFactsConfig       `json:"config"`
	Declarations arrayFactsDeclarations `json:"declarations"`
	Files        []arrayFactsOwnedFile  `json:"files"`
	Types        []arrayFactsType       `json:"types"`
	Functions    []arrayFactsFunction   `json:"functions"`
	Exports      []arrayFactsExport     `json:"exports"`
	Diagnostics  []diagnostic           `json:"diagnostics"`
}
type arrayFactsPackage struct {
	Family     string            `json:"family"`
	Version    int               `json:"version"`
	SchemaHash string            `json:"schemaHash"`
	RequestID  string            `json:"requestId"`
	PackageID  string            `json:"packageId"`
	Payload    arrayFactsPayload `json:"payload"`
}
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
	data, err := genericJSONCanonical(value, arrayFactsPolicy(true))
	if err != nil {
		return nil, arrayFactsJSONError(err)
	}
	return data, nil
}
func arrayFactsSHA(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func arrayFactsID(domain string, value any) (string, error) {
	data, err := arrayFactsCanonical(value)
	if err != nil {
		return "", err
	}
	return "sha256:" + arrayFactsSHA(append([]byte(domain), data...)), nil
}
func decodeArrayFactsRequest(data []byte) (arrayFactsRequest, error) {
	var r arrayFactsRequest
	if err := genericJSONPreflight(data, arrayFactsPolicy(false)); err != nil {
		return r, arrayFactsJSONError(err)
	}
	if err := arrayFactsRequestArrays(data); err != nil {
		return r, err
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		return r, arrayFactsError("canonical", "request requires JCS plus one LF", nil)
	}
	raw := data[:len(data)-1]
	canonical := json.Value(bytes.Clone(raw))
	if err := canonical.Canonicalize(); err != nil || !bytes.Equal(canonical, raw) {
		return r, arrayFactsError("canonical", "request requires exact JCS plus one LF", err)
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return r, arrayFactsError("schema", "request DTO", err)
	}
	encoded, err := arrayFactsCanonical(r)
	if err != nil {
		return r, err
	}
	if !bytes.Equal(encoded, raw) || r.Family != "tsfg.array-facts-request-candidate" || r.Version != 1 || "sha256:"+arrayFactsSHA(arrayFactsSchemaBytes) != arrayFactsSchema {
		return r, arrayFactsError("schema", "exact fields/types/family/version/embedded schema", nil)
	}
	return r, nil
}
