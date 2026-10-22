// SPDX-License-Identifier: Apache-2.0
// Array v8 describes logical checked source; consumers own storage and effects.
package main

import _ "embed"

//go:embed testdata/array-facts-v8/schema.json
var arrayGenericSchema []byte

//go:embed testdata/array-facts-v8/minimal-arrays-v8.d.ts
var arrayGenericDeclarations []byte

type arrayGenericType struct {
	ID                  string                    `json:"id"`
	Kind                string                    `json:"kind"`
	Name                string                    `json:"name,omitempty"`
	ElementType         string                    `json:"elementType,omitempty"`
	Members             []string                  `json:"members,omitempty"`
	SourcePath          string                    `json:"sourcePath,omitempty"`
	DeclarationRange    *span                     `json:"declarationRange,omitempty"`
	NameRange           *span                     `json:"nameRange,omitempty"`
	IndexSignatureRange *span                     `json:"indexSignatureRange,omitempty"`
	NominalKey          *nominalKey               `json:"nominalKey,omitempty"`
	MarkerOrigin        *markerOrigin             `json:"markerOrigin,omitempty"`
	RuntimeKey          string                    `json:"runtimeKey,omitempty"`
	Base                string                    `json:"base,omitempty"`
	Methods             []string                  `json:"methods,omitempty"`
	Fields              *[]arrayGenericField      `json:"fields,omitzero"`
	Constructor         *arrayGenericFunction     `json:"constructor"`
	Construction        *arrayGenericConstruction `json:"construction,omitempty"`
}
type arrayGenericConstruction struct {
	ID                   string                `json:"id"`
	Kind                 string                `json:"kind"`
	Access               string                `json:"access"`
	Signature            arrayGenericSignature `json:"signature"`
	Parameters           []classParameter      `json:"parameters"`
	ImplicitForward      *classImplicitForward `json:"implicitForward"`
	InitializationPhase  string                `json:"initializationPhase"`
	InitializationFields []string              `json:"initializationFields"`
	Origin               classAnchor           `json:"origin"`
}
type arrayGenericField struct {
	ParameterBinding string                  `json:"parameterBinding,omitempty"`
	ID               string                  `json:"id"`
	Name             string                  `json:"name"`
	Type             string                  `json:"type"`
	Readonly         bool                    `json:"readonly"`
	Origin           classAnchor             `json:"origin"`
	Initializer      *arrayGenericExpression `json:"initializer,omitempty"`
}
type arrayGenericSignature struct {
	ID               string   `json:"id"`
	SourcePath       string   `json:"sourcePath"`
	DeclarationRange span     `json:"declarationRange"`
	Parameters       []string `json:"parameters"`
	ReturnType       string   `json:"returnType"`
}
type arrayGenericCompatibilityDependency struct {
	SourceType string `json:"sourceType"`
	TargetType string `json:"targetType"`
}
type arrayGenericCompatibilityWitness struct {
	Compatibility classCompatibility                    `json:"compatibility"`
	Dependencies  []arrayGenericCompatibilityDependency `json:"dependencies"`
}
type arrayGenericTarget struct {
	Kind      string                  `json:"kind"`
	Type      string                  `json:"type"`
	Range     span                    `json:"range"`
	Receiver  *arrayGenericExpression `json:"receiver,omitempty"`
	Index     *arrayGenericExpression `json:"index,omitempty"`
	Binding   string                  `json:"binding,omitempty"`
	Property  string                  `json:"property,omitempty"`
	NameRange *span                   `json:"nameRange,omitempty"`
}
type arrayGenericExpression struct {
	CompatibilityRoots []arrayGenericCompatibilityDependency `json:"compatibilityRoots,omitempty"`
	Kind               string                                `json:"kind"`
	Type               string                                `json:"type"`
	FlowType           string                                `json:"flowType"`
	Range              span                                  `json:"range"`
	Token              string                                `json:"token,omitempty"`
	Decimal            string                                `json:"decimal,omitempty"`
	Bits               string                                `json:"bits,omitempty"`
	CodeUnits          *[]int32                              `json:"codeUnits,omitzero"`
	Binding            string                                `json:"binding,omitempty"`
	Callee             string                                `json:"callee,omitempty"`
	CalleeRange        *span                                 `json:"calleeRange,omitempty"`
	Signature          *arrayGenericSignature                `json:"signature,omitempty"`
	Arguments          *[]arrayGenericExpression             `json:"arguments,omitzero"`
	AllocationSite     string                                `json:"allocationSite,omitempty"`
	Elements           *[]arrayGenericExpression             `json:"elements,omitzero"`
	Receiver           *arrayGenericExpression               `json:"receiver,omitempty"`
	Index              *arrayGenericExpression               `json:"index,omitempty"`
	Expression         *arrayGenericExpression               `json:"expression,omitempty"`
	AnnotationRange    *span                                 `json:"annotationRange,omitempty"`
	Operator           string                                `json:"operator,omitempty"`
	OperatorRange      *span                                 `json:"operatorRange,omitempty"`
	Prefix             *bool                                 `json:"prefix,omitempty"`
	Target             *arrayGenericTarget                   `json:"target,omitempty"`
	Value              *arrayGenericExpression               `json:"value,omitempty"`
	Left               *arrayGenericExpression               `json:"left,omitempty"`
	Right              *arrayGenericExpression               `json:"right,omitempty"`
	Property           string                                `json:"property,omitempty"`
	NameRange          *span                                 `json:"nameRange,omitempty"`
}
type arrayGenericStatement struct {
	Kind        string                   `json:"kind"`
	Range       span                     `json:"range"`
	Binding     string                   `json:"binding,omitempty"`
	Initializer *arrayGenericExpression  `json:"initializer,omitempty"`
	Value       *arrayGenericExpression  `json:"value,omitempty"`
	Condition   *arrayGenericExpression  `json:"condition,omitempty"`
	Then        *[]arrayGenericStatement `json:"then,omitzero"`
	Else        *[]arrayGenericStatement `json:"else,omitzero"`
	Body        *[]arrayGenericStatement `json:"body,omitzero"`
}
type arrayGenericFunction struct {
	OwnerType             string                  `json:"ownerType,omitempty"`
	OverrideTarget        string                  `json:"overrideTarget,omitempty"`
	ID                    string                  `json:"id"`
	Name                  string                  `json:"name"`
	SourcePath            string                  `json:"sourcePath"`
	DeclarationRange      span                    `json:"declarationRange"`
	NameRange             span                    `json:"nameRange"`
	ReturnType            string                  `json:"returnType"`
	ReturnAnnotationRange *span                   `json:"returnAnnotationRange"`
	Parameters            []arrayFactsBinding     `json:"parameters"`
	Bindings              []arrayFactsBinding     `json:"bindings"`
	Body                  []arrayGenericStatement `json:"body"`
}
type arrayGenericPayload struct {
	CompatibilityWitnesses []arrayGenericCompatibilityWitness `json:"compatibilityWitnesses"`
	SourceRoot             string                             `json:"sourceRoot"`
	RootFile               string                             `json:"rootFile"`
	PositionUnit           string                             `json:"positionUnit"`
	Config                 arrayFactsConfig                   `json:"config"`
	Declarations           arrayFactsDeclarations             `json:"declarations"`
	Files                  []arrayFactsOwnedFile              `json:"files"`
	Types                  []arrayGenericType                 `json:"types"`
	ArrayLength            arrayFactsLengthProperty           `json:"arrayLength"`
	ReadSpanLength         arrayFactsReadSpanLength           `json:"readSpanLength"`
	ReadSpanIntrinsic      arrayFactsIntrinsic                `json:"readSpanIntrinsic"`
	Functions              []arrayGenericFunction             `json:"functions"`
	Exports                []arrayFactsExport                 `json:"exports"`
	Diagnostics            []diagnostic                       `json:"diagnostics"`
}
type arrayGenericPackage struct {
	Family     string              `json:"family"`
	Version    int                 `json:"version"`
	SchemaHash string              `json:"schemaHash"`
	RequestID  string              `json:"requestId"`
	PackageID  string              `json:"packageId"`
	Payload    arrayGenericPayload `json:"payload"`
}
