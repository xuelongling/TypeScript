// SPDX-License-Identifier: Apache-2.0
// One typed Program directory; Source never publishes effects or kernel permission.
package main

import (
	_ "embed"

	"github.com/microsoft/typescript-go/internal/ast"
)

//go:embed testdata/common-program-v2/schema.json
var commonProgramSchema []byte

//go:embed testdata/common-program-v3/schema.json
var commonProgramConstructorSchema []byte

type commonProgramParameterStep struct {
	Binding      string                  `json:"binding"`
	ArgumentType string                  `json:"argumentType"`
	Range        span                    `json:"range"`
	Initializer  *arrayGenericExpression `json:"initializer,omitempty"`
}
type commonProgramParameterInitialization struct {
	MinimumArgumentCount    int                          `json:"minimumArgumentCount"`
	HasParameterExpressions bool                         `json:"hasParameterExpressions"`
	Steps                   []commonProgramParameterStep `json:"steps"`
}

type commonProgramCaptureOccurrence struct {
	Range span `json:"range"`
	Read  bool `json:"read"`
	Write bool `json:"write"`
}
type commonProgramCapture struct {
	Binding     string                           `json:"binding"`
	Read        bool                             `json:"read"`
	Write       bool                             `json:"write"`
	Occurrences []commonProgramCaptureOccurrence `json:"occurrences"`
}
type commonProgramTypeAlias struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	SourcePath       string `json:"sourcePath"`
	DeclarationRange span   `json:"declarationRange"`
	NameRange        span   `json:"nameRange"`
	AnnotationRange  span   `json:"annotationRange"`
	Type             string `json:"type"`
}

type commonProgramHostImport struct {
	SourcePath     string   `json:"sourcePath"`
	Name           string   `json:"name"`
	CapabilityID   int      `json:"capabilityId"`
	ParameterTypes []string `json:"parameterTypes"`
	ReturnType     string   `json:"returnType"`
}
type commonProgramHost struct {
	ID               string                `json:"id"`
	SourcePath       string                `json:"sourcePath"`
	Name             string                `json:"name"`
	CapabilityID     int                   `json:"capabilityId"`
	ParameterTypes   []string              `json:"parameterTypes"`
	ReturnType       string                `json:"returnType"`
	DeclarationRange span                  `json:"declarationRange"`
	NameRange        span                  `json:"nameRange"`
	Signature        arrayGenericSignature `json:"signature"`
}
type commonProgramInput struct {
	HostImports        []commonProgramHostImport `json:"hostImports"`
	SourceRoot         string                    `json:"sourceRoot"`
	RootFile           string                    `json:"rootFile"`
	Files              []arrayFactsInputFile     `json:"files"`
	Declarations       arrayFactsDeclarations    `json:"declarations"`
	Config             arrayFactsConfig          `json:"config"`
	Target             *commonProgramTarget      `json:"target"`
	CompilationClosure *closureConfiguration     `json:"compilationClosure"`
}
type commonProgramTarget struct {
	Target         string `json:"target"`
	Triple         string `json:"triple"`
	LLVMDataLayout string `json:"llvmDataLayout"`
	PointerBits    string `json:"pointerBits"`
}
type commonProgramRequest struct {
	Family     string             `json:"family"`
	Version    int                `json:"version"`
	SchemaHash string             `json:"schemaHash"`
	Input      commonProgramInput `json:"input"`
}
type commonProgramGlobal struct {
	SourcePath  string            `json:"sourcePath"`
	BindingKind string            `json:"bindingKind"`
	Binding     arrayFactsBinding `json:"binding"`
}
type commonProgramAlias struct {
	Kind             string  `json:"kind"`
	Name             string  `json:"name"`
	Imported         string  `json:"imported"`
	LocalBinding     *string `json:"localBinding"`
	NameRange        *span   `json:"nameRange"`
	Target           string  `json:"target"`
	TargetSourcePath string  `json:"targetSourcePath"`
	Range            span    `json:"range"`
}
type commonProgramImport struct {
	Kind       string               `json:"kind"`
	Specifier  string               `json:"specifier"`
	SourcePath string               `json:"sourcePath"`
	Range      span                 `json:"range"`
	Bindings   []commonProgramAlias `json:"bindings"`
}
type commonProgramExport struct {
	Name             string `json:"name"`
	Kind             string `json:"kind"`
	Target           string `json:"target"`
	TargetSourcePath string `json:"targetSourcePath"`
	Range            span   `json:"range"`
	DefinitionRange  span   `json:"definitionRange"`
}
type commonProgramModule struct {
	SourcePath     string                  `json:"sourcePath"`
	External       bool                    `json:"external"`
	Imports        []commonProgramImport   `json:"imports"`
	Exports        []commonProgramExport   `json:"exports"`
	Globals        []string                `json:"globals"`
	Initialization []arrayGenericStatement `json:"initialization"`
}
type commonProgramRoot struct {
	Kind       string `json:"kind"`
	SourcePath string `json:"sourcePath"`
	Name       string `json:"name"`
	Target     string `json:"target"`
	Range      span   `json:"range"`
}
type commonProgramClosure struct {
	Roots         []commonProgramRoot `json:"roots"`
	RequiredRoots []commonProgramRoot `json:"requiredRoots"`
}
type commonProgramBlock struct {
	Range span                    `json:"range"`
	Body  []arrayGenericStatement `json:"body"`
}
type commonProgramCatch struct {
	Range            span               `json:"range"`
	DeclarationRange *span              `json:"declarationRange"`
	Binding          *arrayFactsBinding `json:"binding"`
	Block            commonProgramBlock `json:"block"`
}
type commonProgramPayload struct {
	HostImports             []commonProgramHost                   `json:"hostImports"`
	TypeAliases             []commonProgramTypeAlias              `json:"typeAliases"`
	CompatibilityWitnesses  []arrayGenericCompatibilityWitness    `json:"compatibilityWitnesses"`
	EntryCompatibilityRoots []arrayGenericEntryCompatibilityRoots `json:"entryCompatibilityRoots"`
	SourceRoot              string                                `json:"sourceRoot"`
	RootFile                string                                `json:"rootFile"`
	PositionUnit            string                                `json:"positionUnit"`
	Config                  arrayFactsConfig                      `json:"config"`
	Declarations            arrayFactsDeclarations                `json:"declarations"`
	Files                   []arrayFactsOwnedFile                 `json:"files"`
	Types                   []arrayGenericType                    `json:"types"`
	ArrayLength             arrayFactsLengthProperty              `json:"arrayLength"`
	ReadSpanLength          arrayFactsReadSpanLength              `json:"readSpanLength"`
	ReadSpanIntrinsic       arrayFactsIntrinsic                   `json:"readSpanIntrinsic"`
	Functions               []arrayGenericFunction                `json:"functions"`
	Exports                 []arrayFactsExport                    `json:"exports"`
	Diagnostics             []diagnostic                          `json:"diagnostics"`
	Target                  commonProgramTarget                   `json:"target"`
	Modules                 []commonProgramModule                 `json:"modules"`
	Globals                 []commonProgramGlobal                 `json:"globals"`
	Closure                 commonProgramClosure                  `json:"closure"`
}

func (input commonProgramInput) arrayInput() arrayFactsInput {
	return arrayFactsInput{SourceRoot: input.SourceRoot, RootFile: input.RootFile, Files: input.Files,
		Declarations: input.Declarations, Config: input.Config}
}
func commonProgramPayloadFrom(payload arrayGenericPayload, selected *commonProgramProjection) commonProgramPayload {
	return commonProgramPayload{CompatibilityWitnesses: payload.CompatibilityWitnesses, EntryCompatibilityRoots: payload.EntryCompatibilityRoots,
		SourceRoot: payload.SourceRoot, RootFile: payload.RootFile, PositionUnit: payload.PositionUnit, Config: payload.Config,
		Declarations: payload.Declarations, Files: payload.Files, Types: payload.Types, ArrayLength: payload.ArrayLength,
		ReadSpanLength: payload.ReadSpanLength, ReadSpanIntrinsic: payload.ReadSpanIntrinsic, Functions: payload.Functions,
		Exports: payload.Exports, Diagnostics: payload.Diagnostics, Target: *selected.input.Target,
		Modules: selected.modules, Globals: selected.globalRows, Closure: selected.closure, TypeAliases: selected.typeAliases, HostImports: selected.hostImports}
}

type commonProgramPackage struct {
	Family     string               `json:"family"`
	Version    int                  `json:"version"`
	SchemaHash string               `json:"schemaHash"`
	RequestID  string               `json:"requestId"`
	PackageID  string               `json:"packageId"`
	Payload    commonProgramPayload `json:"payload"`
}

// Private projection state lives only inside the one official checker lease.
// Its AST/Symbol keys never enter the returned DTO.
type commonProgramProjection struct {
	version      int
	hosts        map[*ast.Symbol]commonProgramHost
	hostNodes    map[*ast.Symbol]*ast.Node
	hostImports  []commonProgramHost
	lexical      map[*ast.Node]*closureProjection
	bindings     map[string]arrayFactsBinding
	typeAliases  []commonProgramTypeAlias
	input        commonProgramInput
	globals      map[*ast.Symbol]arrayFactsBinding
	globalNodes  map[*ast.Symbol]*ast.Node
	declarations map[*ast.Node]arrayFactsBinding
	globalRows   []commonProgramGlobal
	modules      []commonProgramModule
	closure      commonProgramClosure
}
