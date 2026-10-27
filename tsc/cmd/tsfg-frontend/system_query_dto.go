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
	View            *systemQueryView            `json:"view"`
}
type systemQueryView struct {
	Aliases []systemQueryAlias `json:"aliases"`
	Access  systemQueryAccess  `json:"access"`
}
type systemQueryParameterAccess struct {
	Declaration     systemQueryOrigin   `json:"declaration"`
	TupleSourcePath string              `json:"tupleSourcePath"`
	TupleRange      span                `json:"tupleRange"`
	Aliases         []systemQueryAlias  `json:"aliases"`
	Items           []systemQueryAccess `json:"items"`
}
type systemQueryLoopBinding struct {
	Index         int               `json:"index"`
	Declaration   systemQueryOrigin `json:"declaration"`
	ComponentType string            `json:"componentType"`
	Mode          string            `json:"mode"`
}
type systemQueryLoop struct {
	Range        span                     `json:"range"`
	QueryBinding string                   `json:"queryBinding"`
	QueryRange   span                     `json:"queryRange"`
	BindingRange span                     `json:"bindingRange"`
	BodyRange    span                     `json:"bodyRange"`
	Bindings     []systemQueryLoopBinding `json:"bindings"`
	Statements   []arrayGenericStatement  `json:"statements"`
}
type systemQueryFunction struct {
	BodyRange   span                   `json:"bodyRange"`
	Loops       []systemQueryLoop      `json:"loops"`
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
type systemQueryScalarHelper struct {
	ID                    string                  `json:"id"`
	Declaration           systemQueryOrigin       `json:"declaration"`
	Signature             arrayGenericSignature   `json:"signature"`
	Parameters            []systemQueryParameter  `json:"parameters"`
	ReturnType            string                  `json:"returnType"`
	ReturnAnnotationRange span                    `json:"returnAnnotationRange"`
	BodyRange             span                    `json:"bodyRange"`
	ReturnRange           span                    `json:"returnRange"`
	Value                 *arrayGenericExpression `json:"value"`
}
type systemQueryPayload struct {
	ScalarHelpers []systemQueryScalarHelper `json:"scalarHelpers"`
	Program       systemQueryProgram        `json:"program"`
	ProfilePath   string                    `json:"profilePath"`
	ProfileSHA256 string                    `json:"profileSHA256"`
	Builtins      []systemQueryOrigin       `json:"builtins"`
	Nodes         []systemQueryNode         `json:"nodes"`
	Types         []arrayGenericType        `json:"types"`
	Functions     []arrayGenericFunction    `json:"functions"`
	MarkerAliases []systemQueryMarkerAlias  `json:"markerAliases"`
	Systems       []systemQueryFunction     `json:"systems"`
}
