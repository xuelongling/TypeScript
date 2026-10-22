// SPDX-License-Identifier: Apache-2.0
// Shared owned Array boundary values; logical v8 payloads use these exact fields.
package main

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

type arrayFactsLengthProperty struct {
	ID               string `json:"id"`
	Kind             string `json:"kind"`
	Name             string `json:"name"`
	OwnerType        string `json:"ownerType"`
	OfficialType     string `json:"officialType"`
	WriteType        string `json:"writeType"`
	SourcePath       string `json:"sourcePath"`
	DeclarationRange span   `json:"declarationRange"`
	NameRange        span   `json:"nameRange"`
	TypeRange        span   `json:"typeRange"`
}

type arrayFactsReadSpanLength struct {
	ID               string `json:"id"`
	Kind             string `json:"kind"`
	Name             string `json:"name"`
	OwnerType        string `json:"ownerType"`
	OfficialType     string `json:"officialType"`
	ReadType         string `json:"readType"`
	Readonly         bool   `json:"readonly"`
	SourcePath       string `json:"sourcePath"`
	DeclarationRange span   `json:"declarationRange"`
	NameRange        span   `json:"nameRange"`
	TypeRange        span   `json:"typeRange"`
}

type arrayFactsIntrinsicParameter struct {
	Name             string `json:"name"`
	DeclarationRange span   `json:"declarationRange"`
	NameRange        span   `json:"nameRange"`
	TypeRange        span   `json:"typeRange"`
}

type arrayFactsIntrinsic struct {
	ID                 string                         `json:"id"`
	Kind               string                         `json:"kind"`
	Name               string                         `json:"name"`
	SourcePath         string                         `json:"sourcePath"`
	DeclarationRange   span                           `json:"declarationRange"`
	NameRange          span                           `json:"nameRange"`
	TypeParameterRange span                           `json:"typeParameterRange"`
	Parameters         []arrayFactsIntrinsicParameter `json:"parameters"`
	ReturnTypeRange    span                           `json:"returnTypeRange"`
}

type arrayFactsExport struct {
	Name       string `json:"name"`
	FunctionID string `json:"functionId"`
}
