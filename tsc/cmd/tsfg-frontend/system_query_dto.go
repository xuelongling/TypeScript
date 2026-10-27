// SPDX-License-Identifier: Apache-2.0
package main

type systemQueryOrigin struct {
	ID         string `json:"id"`
	SourcePath string `json:"sourcePath"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Range      span   `json:"range"`
	NameRange  span   `json:"nameRange"`
}
type systemQueryNode struct {
	ID         string `json:"id"`
	Parent     string `json:"parent"`
	SourcePath string `json:"sourcePath"`
	Kind       string `json:"kind"`
	Range      span   `json:"range"`
	Name       string `json:"name"`
	NameRange  *span  `json:"nameRange"`
}
type systemQueryMarkerAlias struct {
	UseSourcePath string            `json:"useSourcePath"`
	UseRange      span              `json:"useRange"`
	Declaration   systemQueryOrigin `json:"declaration"`
	Target        systemQueryOrigin `json:"target"`
}
type systemQueryAccess struct {
	ComponentSourcePath string            `json:"componentSourcePath"`
	Index               int               `json:"index"`
	Mode                string            `json:"mode"`
	Range               span              `json:"range"`
	Wrapper             systemQueryOrigin `json:"wrapper"`
	ComponentType       string            `json:"componentType"`
	Component           systemQueryOrigin `json:"component"`
	ComponentRange      span              `json:"componentRange"`
}
type systemQueryAlias struct {
	Target        systemQueryOrigin `json:"target"`
	UseSourcePath string            `json:"useSourcePath"`
	UseRange      span              `json:"useRange"`
	Declaration   systemQueryOrigin `json:"declaration"`
}
type systemQueryParameter struct {
	Index           int                         `json:"index"`
	Binding         systemQueryOrigin           `json:"binding"`
	AnnotationRange span                        `json:"annotationRange"`
	LogicalType     string                      `json:"logicalType"`
	Query           *systemQueryParameterAccess `json:"query"`
}
type systemQueryParameterAccess struct {
	Declaration     systemQueryOrigin   `json:"declaration"`
	TupleSourcePath string              `json:"tupleSourcePath"`
	TupleRange      span                `json:"tupleRange"`
	Aliases         []systemQueryAlias  `json:"aliases"`
	Items           []systemQueryAccess `json:"items"`
}
type systemQueryFunction struct {
	Declaration systemQueryOrigin      `json:"declaration"`
	MarkerRange span                   `json:"markerRange"`
	Marker      systemQueryOrigin      `json:"marker"`
	Signature   systemQueryOrigin      `json:"signature"`
	Parameters  []systemQueryParameter `json:"parameters"`
	ReturnType  string                 `json:"returnType"`
}
type systemQueryProgram struct {
	SourceRoot   string                 `json:"sourceRoot"`
	RootFile     string                 `json:"rootFile"`
	PositionUnit string                 `json:"positionUnit"`
	Config       arrayFactsConfig       `json:"config"`
	Declarations arrayFactsDeclarations `json:"declarations"`
	Target       *commonProgramTarget   `json:"target"`
	Files        []arrayFactsOwnedFile  `json:"files"`
	Diagnostics  []diagnostic           `json:"diagnostics"`
}
type systemQueryPayload struct {
	Program       systemQueryProgram       `json:"program"`
	ProfilePath   string                   `json:"profilePath"`
	ProfileSHA256 string                   `json:"profileSHA256"`
	Builtins      []systemQueryOrigin      `json:"builtins"`
	Nodes         []systemQueryNode        `json:"nodes"`
	Types         []arrayGenericType       `json:"types"`
	Functions     []arrayGenericFunction   `json:"functions"`
	MarkerAliases []systemQueryMarkerAlias `json:"markerAliases"`
	Systems       []systemQueryFunction    `json:"systems"`
}
