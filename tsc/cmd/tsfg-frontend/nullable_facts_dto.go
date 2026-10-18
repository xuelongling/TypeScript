// SPDX-License-Identifier: Apache-2.0

// The private nullable boundary owns its fields, limits and identity domains.
// Legacy response structs are embedded as facts, never extended or retagged.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/microsoft/typescript-go/internal/json"
)

const nullableFactsSchema = "sha256:24a04b0b595cae59ccf52a939f4671cea0369c46bd705f56726d028ab1df5a71"
const nullableFactsMaxBytes = 8388608

type nullableFactsInput struct {
	Target       *integerTarget `json:"target"`
	SourceRoot   string         `json:"sourceRoot"`
	RootFile     string         `json:"rootFile"`
	Files        []inputFile    `json:"files"`
	Declarations struct {
		Version string   `json:"version"`
		Paths   []string `json:"paths"`
	} `json:"declarations"`
	Config struct {
		Strict  bool `json:"strict"`
		NoEmit  bool `json:"noEmit"`
		AllowJS bool `json:"allowJs"`
	} `json:"config"`
}

type nullableFactsRequest struct {
	Family     string             `json:"family"`
	Version    int                `json:"version"`
	SchemaHash string             `json:"schemaHash"`
	Input      nullableFactsInput `json:"input"`
}

type nullableFactsFile struct {
	Path         string `json:"path"`
	Digest       string `json:"digest"`
	ByteLength   int    `json:"byteLength"`
	Length       int    `json:"length"`
	PositionUnit string `json:"positionUnit"`
}

type nullableFactsExport struct {
	Name       string `json:"name"`
	Target     string `json:"target"`
	SourcePath string `json:"sourcePath"`
	Range      span   `json:"range"`
	Definition span   `json:"definition"`
}

type nullableFactsPayload struct {
	InputIdentity string                `json:"inputIdentity"`
	Files         []nullableFactsFile   `json:"files"`
	Facts         response              `json:"facts"`
	Exports       []nullableFactsExport `json:"exports"`
}

type nullableFactsPackage struct {
	Family     string               `json:"family"`
	Version    int                  `json:"version"`
	SchemaHash string               `json:"schemaHash"`
	RequestID  string               `json:"requestId"`
	PackageID  string               `json:"packageId"`
	Payload    nullableFactsPayload `json:"payload"`
}

func nullableFactsError(kind, detail string) error {
	return fmt.Errorf("[r03-nullable-facts-%s] %s", kind, detail)
}

func nullableFactsJSON(data []byte, allowNull bool, version nullableFactsVersion) error {
	policy := genericJSONPolicy{maxBytes: nullableFactsMaxBytes, maxDepth: 128,
		maxTokens: 200000, maxString: 1048576, allowNUL: true, allowNull: allowNull}
	if err := genericJSONPreflight(data, policy); err != nil {
		var failure *genericFailure
		if errors.As(err, &failure) {
			return version.failure(strings.TrimPrefix(failure.Kind, "package-"), failure.Detail, err)
		}
		return version.failure("json", "strict token preflight", err)
	}
	return nil
}

func nullableFactsCanonical(value any, version nullableFactsVersion) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, version.failure("json", "DTO encoding", err)
	}
	if err := nullableFactsJSON(data, true, version); err != nil {
		return nil, err
	}
	canonical := json.Value(data)
	if err := canonical.Canonicalize(); err != nil {
		return nil, version.failure("canonical", "JCS encoding", err)
	}
	return canonical, nil
}

func nullableFactsDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func decodeNullableFactsRequest(data []byte, version nullableFactsVersion) (nullableFactsRequest, error) {
	var r nullableFactsRequest
	if err := nullableFactsJSON(data, false, version); err != nil {
		return r, err
	}
	canonical := json.Value(bytes.Clone(data))
	if err := canonical.Canonicalize(); err != nil || !bytes.Equal(canonical, data) {
		return r, version.failure("canonical", "request must be exact JCS", err)
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return r, version.failure("schema", "request DTO", err)
	}
	encoded, err := nullableFactsCanonical(r, version)
	if err != nil {
		return r, err
	}
	// Explicit false/empty fields survive DTO encoding. This rejects absent,
	// unknown, case-variant and wrong-typed fields at every nested boundary.
	if !bytes.Equal(encoded, data) {
		return r, version.failure("schema", "request exact fields and types")
	}
	if r.Family != "tsfg.nullable-facts-request-candidate" || r.Version != int(version) || r.SchemaHash != version.schemaHash() {
		return r, version.failure("schema", "request family/version/schema")
	}
	return r, nil
}

func nullableFactsInputs(input nullableFactsInput, version nullableFactsVersion) (request, string, []nullableFactsFile, error) {
	// The legacy analyzer changes Target.floats. Decode a complete owned clone;
	// neither that pointer nor sorted file slices may alias the incoming DTO.
	data, err := nullableFactsCanonical(input, version)
	if err != nil {
		return request{}, "", nil, err
	}
	var owned request
	if err := json.Unmarshal(data, &owned); err != nil {
		return request{}, "", nil, version.failure("schema", "legacy input conversion", err)
	}
	if !valid(owned) || input.Declarations.Version != "minimal-integers-v3" ||
		len(input.Files) != len(input.Declarations.Paths)+1 || input.Declarations.Paths[0] != input.SourceRoot+"/minimal-integers-v3.d.ts" {
		return request{}, "", nil, version.failure("input", "required integer target/config/source/declaration closure")
	}
	identity := input
	identity.Files = slices.Clone(input.Files)
	slices.SortFunc(identity.Files, func(a, b inputFile) int { return strings.Compare(a.Path, b.Path) })
	files := make([]nullableFactsFile, 0, len(identity.Files))
	declared := map[string]bool{input.RootFile: true}
	for _, name := range input.Declarations.Paths {
		declared[name] = true
	}
	for i, file := range identity.Files {
		digest := nullableFactsDigest([]byte(file.Content))
		if !declared[file.Path] || file.SHA256 != "" && file.SHA256 != digest {
			return request{}, "", nil, version.failure("identity", "source/declaration closure or content checksum")
		}
		identity.Files[i].SHA256 = ""
		files = append(files, nullableFactsFile{Path: file.Path, Digest: digest, ByteLength: len(file.Content),
			Length: len(utf16.Encode([]rune(file.Content))), PositionUnit: "utf16"})
	}
	identityBytes, err := nullableFactsCanonical(identity, version)
	if err != nil {
		return request{}, "", nil, err
	}
	return owned, nullableFactsDigest(identityBytes), files, nil
}
