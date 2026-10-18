// SPDX-License-Identifier: Apache-2.0
// Same string-facts family, explicit v2: a typed operation retains both literal operands.
package main

import _ "embed"

const stringFactsConcatSchema = "sha256:79d8f807552e301b606aacdee7b81fcee83d8996ff2043e58c435e5f2b7b519c"

//go:embed testdata/string-facts-v2/schema.json
var stringFactsConcatSchemaBytes []byte

type stringFactsConcatValue struct {
	Kind          string           `json:"kind"`
	Type          string           `json:"type"`
	Range         span             `json:"range"`
	OperatorRange span             `json:"operatorRange"`
	Left          stringFactsValue `json:"left"`
	Right         stringFactsValue `json:"right"`
}
type stringFactsConcatStatement struct {
	Kind  string                 `json:"kind"`
	Range span                   `json:"range"`
	Value stringFactsConcatValue `json:"value"`
}
type stringFactsConcatFunction struct {
	Name             string                       `json:"name"`
	SourcePath       string                       `json:"sourcePath"`
	DeclarationRange span                         `json:"declarationRange"`
	Parameters       []string                     `json:"parameters"`
	ReturnType       string                       `json:"returnType"`
	Body             []stringFactsConcatStatement `json:"body"`
}
type stringFactsConcatPayload struct {
	SourceRoot   string                      `json:"sourceRoot"`
	RootFile     string                      `json:"rootFile"`
	PositionUnit string                      `json:"positionUnit"`
	Config       stringFactsConfig           `json:"config"`
	Declarations stringFactsDeclarations     `json:"declarations"`
	Files        []stringFactsOwnedFile      `json:"files"`
	Functions    []stringFactsConcatFunction `json:"functions"`
}
type stringFactsConcatPackage struct {
	Family     string                   `json:"family"`
	Version    int                      `json:"version"`
	SchemaHash string                   `json:"schemaHash"`
	RequestID  string                   `json:"requestId"`
	PackageID  string                   `json:"packageId"`
	Payload    stringFactsConcatPayload `json:"payload"`
}

func decodeStringFactsConcatRequest(data []byte) (stringFactsRequest, error) {
	return decodeStringFactsVersionRequest(data, 2, stringFactsConcatSchema, stringFactsConcatSchemaBytes)
}
