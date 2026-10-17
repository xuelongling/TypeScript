// SPDX-License-Identifier: Apache-2.0

// The exception transport owns its schema, identity domains and exact input DTO.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/microsoft/typescript-go/internal/json"
)

const exceptionFactsSchema = "sha256:5221b50822965c633418462b28a9b58bfbc41b01e0fa4b6d31c50ab83805424d"
const exceptionFactsMaxBytes = 8388608
const exceptionFactsMaxArrayItems = 4096

type exceptionFactsInput struct {
	CompilationClosure *closureConfiguration `json:"compilationClosure,omitempty"`
	Target             *integerTarget        `json:"target,omitempty"`
	SourceRoot         string                `json:"sourceRoot"`
	RootFile           string                `json:"rootFile"`
	Files              []inputFile           `json:"files"`
	Declarations       struct {
		MarkerProfile *markerProfile `json:"markerProfile,omitempty"`
		Version       string         `json:"version"`
		Paths         []string       `json:"paths"`
	} `json:"declarations"`
	Config struct {
		DecoratorMode string `json:"decoratorMode,omitempty"`
		Strict        bool   `json:"strict"`
		NoEmit        bool   `json:"noEmit"`
		AllowJS       bool   `json:"allowJs"`
	} `json:"config"`
}

type exceptionFactsRequest struct {
	Family     string              `json:"family"`
	Version    int                 `json:"version"`
	SchemaHash string              `json:"schemaHash"`
	Input      exceptionFactsInput `json:"input"`
}

type exceptionFactsFile struct {
	Path         string `json:"path"`
	Digest       string `json:"digest"`
	ByteLength   int    `json:"byteLength"`
	Length       int    `json:"length"`
	PositionUnit string `json:"positionUnit"`
}

type exceptionFactsExport struct {
	Name       string `json:"name"`
	Target     string `json:"target"`
	SourcePath string `json:"sourcePath"`
	Range      span   `json:"range"`
	Definition span   `json:"definition"`
}

type exceptionFactsExportGroup struct {
	SourcePath string                 `json:"sourcePath"`
	External   bool                   `json:"external"`
	Bindings   []exceptionFactsExport `json:"bindings"`
}

type exceptionFactsPayload struct {
	InputIdentity string                      `json:"inputIdentity"`
	Files         []exceptionFactsFile        `json:"files"`
	Facts         json.Value                  `json:"facts"`
	Exports       []exceptionFactsExportGroup `json:"exports"`
}

type exceptionFactsPackage struct {
	Family     string                `json:"family"`
	Version    int                   `json:"version"`
	SchemaHash string                `json:"schemaHash"`
	RequestID  string                `json:"requestId"`
	PackageID  string                `json:"packageId"`
	Payload    exceptionFactsPayload `json:"payload"`
}

func exceptionFactsError(kind, detail string) error {
	return fmt.Errorf("[r03-exception-facts-%s] %s", kind, detail)
}

func exceptionFactsJSON(data []byte, allowNull bool) error {
	policy := genericJSONPolicy{maxBytes: exceptionFactsMaxBytes, maxDepth: 128,
		maxTokens: 200000, maxString: 1048576, allowNUL: true, allowNull: allowNull}
	if err := genericJSONPreflight(data, policy); err != nil {
		var failure *genericFailure
		if errors.As(err, &failure) {
			return exceptionFactsError(strings.TrimPrefix(failure.Kind, "package-"), failure.Detail)
		}
		return exceptionFactsError("json", "strict token preflight")
	}
	return exceptionFactsArrayBounds(data)
}

// The official decoder owns JSON syntax and container boundaries. After the
// shared strict preflight, count each array's immediate values, including a
// nested object/array as one value. Keys and closing delimiters are not items.
func exceptionFactsArrayBounds(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	containers := []int{}
	for {
		token, err := decoder.ReadToken()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return exceptionFactsError("json", "strict array token preflight")
		}
		kind := token.Kind()
		last := len(containers) - 1
		if kind != '}' && kind != ']' && last >= 0 && containers[last] >= 0 {
			containers[last]++
			if containers[last] > exceptionFactsMaxArrayItems {
				return exceptionFactsError("limit", "array exceeds 4096 items")
			}
		}
		switch kind {
		case '[':
			containers = append(containers, 0)
		case '{':
			containers = append(containers, -1)
		case ']', '}':
			if last < 0 {
				return exceptionFactsError("json", "array container preflight")
			}
			containers = containers[:last]
		}
	}
}

// Encode the current owned response with the public DTO's serializer. Its
// pointer-to-slice fields preserve required empty call/new arguments; v2
// omitempty would otherwise omit those present empty arrays.
func exceptionFactsEncodeResponse(facts response) (json.Value, error) {
	var encoded bytes.Buffer
	if err := json.NewLegacyEncoder(&encoded).Encode(facts); err != nil {
		return nil, exceptionFactsError("json", "owned response DTO encoding")
	}
	if err := exceptionFactsJSON(encoded.Bytes(), true); err != nil {
		return nil, err
	}
	canonical := json.Value(encoded.Bytes())
	if err := canonical.Canonicalize(); err != nil {
		return nil, exceptionFactsError("canonical", "owned response JCS encoding")
	}
	return canonical, nil
}

func exceptionFactsCanonical(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, exceptionFactsError("json", "DTO encoding")
	}
	if err := exceptionFactsJSON(data, true); err != nil {
		return nil, err
	}
	canonical := json.Value(data)
	if err := canonical.Canonicalize(); err != nil {
		return nil, exceptionFactsError("canonical", "JCS encoding")
	}
	return canonical, nil
}

func exceptionFactsDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func decodeExceptionFactsRequest(data []byte) (exceptionFactsRequest, error) {
	var r exceptionFactsRequest
	if err := exceptionFactsJSON(data, false); err != nil {
		return r, err
	}
	canonical := json.Value(bytes.Clone(data))
	if err := canonical.Canonicalize(); err != nil || !bytes.Equal(canonical, data) {
		return r, exceptionFactsError("canonical", "request must be exact JCS")
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return r, exceptionFactsError("schema", "request DTO")
	}
	encoded, err := exceptionFactsCanonical(r)
	if err != nil {
		return r, err
	}
	// Complete typed re-encoding preserves false/empty required fields and
	// rejects missing, unknown, case-variant and explicit null optional members.
	if !bytes.Equal(encoded, data) {
		return r, exceptionFactsError("schema", "request exact fields and types")
	}
	if r.Family != "tsfg.exception-facts-request-candidate" || r.Version != 1 || r.SchemaHash != exceptionFactsSchema {
		return r, exceptionFactsError("schema", "request family/version/schema")
	}
	return r, nil
}
