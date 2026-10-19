// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"io"
	"strings"

	"github.com/microsoft/typescript-go/internal/json"
)

const moduleCallableFactsSchema = "sha256:8d66ba7464ea05c317fc413cf61dbc906c34c1ef38f4c7fcc916038e7dbe388e"
const moduleCallableMaxBytes = 8388608

//go:embed testdata/module-callable-facts-v1/schema.json
var moduleCallableSchemaBytes []byte

type moduleCallableInput struct {
	SourceRoot   string      `json:"sourceRoot"`
	RootFile     string      `json:"rootFile"`
	Files        []inputFile `json:"files"`
	Declarations struct {
		Version string   `json:"version"`
		Paths   []string `json:"paths"`
	} `json:"declarations"`
	Config struct {
		Strict  bool `json:"strict"`
		NoEmit  bool `json:"noEmit"`
		AllowJS bool `json:"allowJs"`
	} `json:"config"`
	Target             *integerTarget        `json:"target"`
	CompilationClosure *closureConfiguration `json:"compilationClosure"`
	HostImports        []hostImport          `json:"hostImports"`
}

type moduleCallableRequest struct {
	Family     string              `json:"family"`
	Version    int                 `json:"version"`
	SchemaHash string              `json:"schemaHash"`
	Input      moduleCallableInput `json:"input"`
}

type moduleCallableFile struct {
	Path         string `json:"path"`
	Digest       string `json:"digest"`
	ByteLength   int    `json:"byteLength"`
	Length       int    `json:"length"`
	PositionUnit string `json:"positionUnit"`
}

type moduleCallableParameterAnnotation struct {
	Symbol           string `json:"symbol"`
	DeclarationRange span   `json:"declarationRange"`
	NameRange        span   `json:"nameRange"`
	AnnotationRange  span   `json:"annotationRange"`
}

type moduleCallableAnnotation struct {
	Symbol                string                              `json:"symbol"`
	SourcePath            string                              `json:"sourcePath"`
	DeclarationRange      span                                `json:"declarationRange"`
	NameRange             span                                `json:"nameRange"`
	ReturnAnnotationRange span                                `json:"returnAnnotationRange"`
	Parameters            []moduleCallableParameterAnnotation `json:"parameters"`
}

type moduleCallableExport struct {
	Name             string `json:"name"`
	Target           string `json:"target"`
	Range            span   `json:"range"`
	OriginSourcePath string `json:"originSourcePath"`
	DefinitionRange  span   `json:"definitionRange"`
	NameRange        span   `json:"nameRange"`
}

type moduleCallableExportGroup struct {
	SourcePath string                 `json:"sourcePath"`
	External   bool                   `json:"external"`
	Bindings   []moduleCallableExport `json:"bindings"`
}

type moduleCallablePayload struct {
	InputIdentity string                      `json:"inputIdentity"`
	Files         []moduleCallableFile        `json:"files"`
	Facts         response                    `json:"facts"`
	Annotations   []moduleCallableAnnotation  `json:"annotations"`
	Exports       []moduleCallableExportGroup `json:"exports"`
}

type moduleCallablePacket struct {
	Family     string                `json:"family"`
	Version    int                   `json:"version"`
	SchemaHash string                `json:"schemaHash"`
	RequestID  string                `json:"requestId"`
	PackageID  string                `json:"packageId,omitempty"`
	Payload    moduleCallablePayload `json:"payload"`
}

func moduleCallableError(kind string) error {
	messages := map[string]string{
		"input": "invalid module callable candidate", "identity": "module callable candidate identity differs",
		"source": "module callable source facts differ", "canonical": "module callable candidate must use canonical JSON plus LF",
		"limit": "module callable candidate budget exceeded",
	}
	return errors.New("r03-module-callable-" + kind + ": " + messages[kind])
}

func moduleCallableHash(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func moduleCallablePolicy() genericJSONPolicy {
	return genericJSONPolicy{maxBytes: moduleCallableMaxBytes, maxDepth: 128, maxTokens: 200000, maxString: 1048576, allowNUL: true, allowNull: true, allowRaw: true}
}

func moduleCallableCanonical(value any) ([]byte, error) {
	if err := moduleCallableWireBudget(value); err != nil {
		return nil, err
	}
	var encoded bytes.Buffer
	if err := json.NewLegacyEncoder(&encoded).Encode(value); err != nil {
		return nil, moduleCallableError("input")
	}
	canonical := json.Value(bytes.TrimSuffix(encoded.Bytes(), []byte("\n")))
	if err := canonical.Canonicalize(); err != nil {
		return nil, moduleCallableError("canonical")
	}
	if _, err := inspectGenericJSON(canonical, moduleCallablePolicy()); err != nil {
		return nil, moduleCallableJSONError(err)
	}
	return canonical, nil
}

func moduleCallableJSONError(err error) error {
	if strings.Contains(err.Error(), "limit") {
		return moduleCallableError("limit")
	}
	return moduleCallableError("input")
}

func moduleCallableDigest(domain string, value any) (string, error) {
	data, err := moduleCallableCanonical(value)
	if err != nil {
		return "", err
	}
	return moduleCallableHash(append([]byte(domain+"\x00"), data...)), nil
}

func decodeModuleCallableRequest(data []byte) (moduleCallableRequest, error) {
	var r moduleCallableRequest
	if len(data) > moduleCallableMaxBytes {
		return r, moduleCallableError("limit")
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		return r, moduleCallableError("canonical")
	}
	raw := data[:len(data)-1]
	if _, err := inspectGenericJSON(raw, moduleCallablePolicy()); err != nil {
		return r, moduleCallableJSONError(err)
	}
	decoder := json.NewLegacyDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&r); err != nil {
		return r, moduleCallableError("input")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return r, moduleCallableError("canonical")
	}
	encoded, err := moduleCallableCanonical(r)
	if err != nil {
		return r, err
	}
	if !bytes.Equal(raw, encoded) {
		return r, moduleCallableError("canonical")
	}
	if r.Family != "tsfg.module-callable-facts-request-candidate" || r.Version != 2 {
		return r, moduleCallableError("input")
	}
	if r.SchemaHash != moduleCallableFactsSchema || moduleCallableHash(moduleCallableSchemaBytes) != moduleCallableFactsSchema {
		return r, moduleCallableError("identity")
	}
	return r, nil
}
